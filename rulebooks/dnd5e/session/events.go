// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EventStream delivers events to the host, which routes them to clients.
//
// Required. A verb's response describes the caller's own action; everything
// else that happens — monsters acting, the clock advancing, another member
// crossing a doorway — reaches a client only through here. That is as true with
// one player as with four.
//
// Use DiscardEvents when there is genuinely nothing to inform.
//
// Events are already projected per audience when they arrive here — who may
// see what is a rule, decided inside this package where perception lives, not
// a delivery concern the host is expected to re-derive. A host that filtered
// events itself would be reimplementing visibility, and its first mistake
// would leak something a player has not perceived.
//
// Publishing is best-effort by contract: a failure here is reported but does
// not fail the verb, because the story log remains the source of truth and a
// client that misses an event can notice the gap and re-query. Implementations
// should therefore not block indefinitely.
type EventStream interface {
	// Publish delivers a batch of already-projected events.
	Publish(ctx context.Context, events []Event) error
}

// DiscardEvents is an EventStream that delivers nothing.
//
// For tests, headless simulation, and analysis runs — anything with no client
// to inform. It exists so that "no delivery" is a stated decision rather than a
// nil nobody noticed: a host that genuinely wants a silent session says so, and
// a host that simply forgot to wire a stream is refused at construction.
//
//	mgr, err := session.NewManager(&session.Config{
//	    Sessions: repo, Encounters: enc, Characters: chars,
//	    Events: session.DiscardEvents{}, Dice: roller,
//	    PresentationIDs: presentationIDs, TurnDriver: session.Pass{},
//	})
type DiscardEvents struct{}

// Publish discards the batch and reports success.
func (DiscardEvents) Publish(_ context.Context, _ []Event) error { return nil }

// DeliveryReport says what reached the event stream.
//
// Separate from SaveReport because they answer different questions and have
// different consequences. A failed save means the world did not change; a
// failed delivery means it did, and some clients have not heard yet. Conflating
// them would leave a host unable to tell "retry this" from "this happened, tell
// them again."
type DeliveryReport struct {
	// Events is how many per-recipient events were handed to the stream.
	Events int `json:"events"`

	// Failed reports that delivery did not succeed. The verb still succeeded:
	// the story log is the source of truth, sequences are gapless, and a client
	// that misses an event notices the hole and re-queries.
	Failed bool `json:"failed,omitempty"`
}

// deliver hands an already-built batch to the stream.
//
// Called only AFTER the save has landed (S9). Announcing a fact that failed to
// persist is the one ordering mistake that cannot be recovered from: a client
// told the ogre died, and a world in which it did not, and no sequence gap to
// betray the difference. The batch itself is BUILT BEFORE the save
// ([Manager.commit], from the pure view and the live Story), because the
// save-point ToData trims the log — a batch projected afterwards would
// silently drop whatever a big verb's delta lost to the trim. Building
// early and delivering late keeps both laws.
//
// Delivery is best effort (S10). A failure is reported and the verb still
// succeeds, because the log remains the truth and per-recipient dense
// sequences let a client detect what it missed. Failing the verb instead
// would roll back nothing — the world has already changed — and would turn a
// transient stream outage into a spurious error the host must decide how to
// interpret.
func (m *Manager) deliver(ctx context.Context, events []Event) DeliveryReport {
	if len(events) == 0 {
		return DeliveryReport{}
	}

	if err := m.events.Publish(ctx, events); err != nil {
		return DeliveryReport{Events: len(events), Failed: true}
	}
	return DeliveryReport{Events: len(events)}
}

// projectEvents turns the beats a verb recorded into one event per recipient.
//
// It reads the LIVE story — called before the save-point ToData, while the
// log still holds the verb's whole delta (see [Manager.deliver]).
//
// The audience question is answered by asking the composition, not by us: for
// each member, its own Story is queried from the baseline, and whatever it
// returns is what that member may know. Filtering a shared list here would mean
// reimplementing visibility outside the module that owns perception, and the
// first mistake would leak something a player has not perceived.
//
// The roster comes from EverMembers rather than current members, so a member
// who left during this very verb still receives the beat describing their
// departure. Using the live roster would silently drop the one event the
// departing player most needs.
func (m *Manager) projectEvents(
	scope *writeScope, snapshot *encounter.EncounterData,
) []Event {
	var events []Event

	for _, member := range snapshot.EverMembers {
		entries, err := scope.enc.Story(&encounter.StoryInput{
			Audience: member,
			AfterSeq: scope.baseline,
		})
		if err != nil {
			// A member whose story cannot be read gets no events rather than
			// failing the fan-out for everyone else. Delivery is best effort by
			// contract, and one unreadable recipient must not silence the rest
			// of the table.
			continue
		}

		for _, entry := range entries {
			events = append(events, projectEntry(scope.session, string(member), entry,
				scope.deliveredSeq(string(member), entry.Seq)))
		}
	}

	return events
}

// projectEntry turns one story-log entry into the Event it becomes for one
// recipient — the SAME projection whether the entry reaches a client live
// (projectEvents, moments after Record) or later, on catch-up ([Manager.Story]).
//
// One function, two call sites, is the whole mechanism behind rpg-api-protos
// #239's ruling: live delivery and a Story catch-up are byte-equal for the
// same seq because nothing else in this package is allowed to build an Event
// any other way. Before this, Story returned a thinner StoryEntry with the
// raw payload and no typed Kind or Body, so a client that noticed a gap and
// re-queried Story got a shape it had to decode a second, different way —
// exactly the drift #239 found live in the debug feed (kind=UNKNOWN
// body=null on every caught-up entry).
// seq is the RECIPIENT's own number for this entry, computed by the caller
// from the one persisted cursor (stream.go) — never the record's global
// sequence, which stays internal to the seam.
func projectEntry(session, recipient string, e record.Entry, seq uint64) Event {
	kind, body := decodeBeat(e.Payload)

	var tags map[string]string
	if len(e.Tags) > 0 {
		tags = make(map[string]string, len(e.Tags))
		for k, v := range e.Tags {
			tags[k] = v
		}
	}

	return Event{
		Session:     session,
		Seq:         seq,
		At:          e.At,
		Correlation: e.Correlation,
		Tags:        tags,
		Recipient:   recipient,
		Kind:        kind,
		Payload:     append([]byte(nil), e.Payload...),
		Body:        body,
	}
}

// decodeBeat determines a beat's wire EventKind and, for the kinds this
// version types, its typed EventBody — DELETING kindOf's own JSON sniffing
// (rpg-toolkit#941). It is still the one place this package interprets a
// payload rather than passing it through: the composition's own "beat" key
// is a DECLARED kind (every append site in encounter names it explicitly),
// not content this function guesses at, the same peek-then-dispatch pattern
// LoadJSON's own routing already uses for conditions and features elsewhere
// in this codebase.
//
// KIND AND BODY ARE TWO SEPARATE QUESTIONS, on purpose. kindFor answers the
// first from "beat" alone and is total over every string the composition
// can write there; bodyFor attempts the second, and a payload whose other
// fields do not match (a hand-built fixture missing "targets", a future
// composition version that renamed one) yields a nil Body without ALSO
// demoting a correctly-identified Kind to EventUnknown. Conflating the two
// would mean a single missing field anywhere in a payload could make an
// otherwise-ordinary struck beat unrecognisable, which is a worse failure
// than the one #941 set out to fix.
//
// An unrecognised beat becomes EventUnknown rather than being dropped. A
// client that receives something it cannot interpret still learns its
// sequence advanced, which keeps gap-detection working; dropping it would
// manufacture a hole and send every client into a resync it did not need.
// It also means a newer composition can add beats without older clients
// losing their place.
func decodeBeat(payload []byte) (EventKind, EventBody) {
	var peek struct {
		Beat string `json:"beat"`
	}
	if err := json.Unmarshal(payload, &peek); err != nil {
		return EventUnknown, nil
	}

	kind := kindFor(peek.Beat)
	if kind == EventUnknown {
		return EventUnknown, nil
	}
	return kind, bodyFor(kind, payload)
}

// kindFor maps the composition's declared "beat" string onto the wire enum.
// Total: every string reaches an arm, and the ones this build does not
// recognise fall to EventUnknown alongside an empty or unparseable one.
func kindFor(beat string) EventKind {
	switch beat {
	case "moved":
		return EventMoved
	case "joined":
		return EventJoined
	case "exited":
		return EventExited
	case "ended":
		return EventEnded
	case "scene-opened":
		return EventSceneOpened
	case "tick":
		return EventTick
	case "turn-ended":
		return EventTurnEnded
	case "bubble-formed":
		return EventFightStarted
	case "bubble-dissolved":
		return EventFightEnded
	// The explicit outcome beats. Unlike every other case here, these strings are
	// not the composition's own vocabulary for a verb it ran — they are the
	// OutcomeKind a rulebook handed it (encounter's Record), which is why the
	// mapping is worth a word: adding an outcome kind upstream means adding a
	// case here, or the new outcome goes out unnamed.
	case "struck":
		return EventStruck
	case "missed":
		return EventMissed
	case "death_save":
		return EventDeathSave
	case "activated":
		return EventActivated
	case "activation-result":
		return EventActivationResult
	// The third outcome beat, and the one nobody pushed. "down" is an
	// OutcomeKind like the two above, but no caller can hand it to Record —
	// the composition refuses that deliberately (rpg-toolkit#1077) and writes
	// the beat itself when it notices somebody at zero. It is in this family
	// rather than beside the clock beats because it carries the same tag, has
	// the same audience, and is read by a client in the same breath as the
	// strike that caused it.
	//
	// THE WORD CHANGES HERE, ON PURPOSE. The composition says "down" and this
	// seam says "downed", and that asymmetry is Kirk's ruling
	// (rpg-toolkit#1084): a bare "down" also reads as PRONE, so the vocabulary
	// that LEAVES the session is the unambiguous one, while the composition's
	// own kind — which is persisted in every stored world — is left alone. A
	// rename there would be a migration; a translation here is a line.
	case "down":
		return EventDowned
	case "door":
		return EventDoor
	case "door_revealed":
		return EventDoorRevealed
	case "region_revealed":
		return EventRegionRevealed
	// The holdings verbs, named by what the record says (rpg-project#368
	// §4.1). "looted", "held" and "dropped" are the composition's own words
	// for what it did, so they cross unchanged — unlike "down"/"downed"
	// above, which is a translation and says why. NOTHING HERE SAYS "took":
	// Take is reserved for the act that lands a thing in inventory (R10),
	// and no beat this seam publishes may claim it.
	case "looted":
		return EventLooted
	case "held":
		return EventHeld
	case "dropped":
		return EventDropped
	// THE WORD CHANGES HERE, like "down"/"downed" above: the composition
	// names the noun ("stance", the thing that changed) and the wire names
	// the event, beside "fight_ended" and "door_revealed". A stance turning
	// is truth grain and goes to everyone (rpg-project#375, design §6).
	case "stance":
		return EventStanceChanged
	// A reserved placement entering the run (rpg-project#375, R6): the
	// composition's own word for what it did, crossing unchanged like
	// "held" and "dropped".
	case "arrived":
		return EventArrived
	default:
		return EventUnknown
	}
}

// bodyFor decodes payload into the typed body kind names, or nil if the
// payload does not match what that kind's body needs — a best-effort
// second step that never changes the Kind decodeBeat already settled on.
func bodyFor(kind EventKind, payload []byte) EventBody {
	switch kind {
	case EventMoved:
		var p struct {
			Member   string           `json:"member"`
			Position spatial.Position `json:"position"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" {
			return nil
		}
		return MovedBody{Member: p.Member, To: p.Position}
	case EventJoined:
		var p struct {
			Member string `json:"member"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" {
			return nil
		}
		return JoinedBody{Member: p.Member}
	case EventExited:
		var p struct {
			Member  string   `json:"member"`
			Holding []string `json:"holding"`
			Exit    string   `json:"exit"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" {
			return nil
		}
		// Neither new field gates the body. A departure carrying nothing
		// through no authored exit is the ORDINARY case and must still
		// decode; requiring either would demote every ordinary exit to an
		// untyped payload, which is the failure this function's own doc
		// warns kind-and-body conflation causes.
		return ExitedBody{Member: p.Member, Holding: p.Holding, Exit: p.Exit}
	case EventEnded:
		var p struct {
			Ending string `json:"ending"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Ending == "" {
			return nil
		}
		return EndedBody{Ending: p.Ending}
	case EventDoor:
		var p struct {
			Door   string `json:"door"`
			State  string `json:"state"`
			Actor  string `json:"actor"`
			DC     int    `json:"dc"`
			Total  int    `json:"total"`
			Beaten bool   `json:"beaten"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Door == "" || p.State == "" {
			return nil
		}
		return DoorBody{Door: p.Door, State: p.State, Actor: p.Actor, DC: p.DC, Total: p.Total, Beaten: p.Beaten}
	case EventTurnEnded:
		var p struct {
			Member string `json:"member"`
			Next   string `json:"next"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" || p.Next == "" {
			return nil
		}
		return TurnEndedBody{Member: p.Member, Next: p.Next}
	case EventFightStarted:
		var p struct {
			Order []string `json:"order"`
		}
		if json.Unmarshal(payload, &p) != nil || len(p.Order) == 0 {
			return nil
		}
		return FightStartedBody{Members: p.Order}
	case EventFightEnded:
		var p struct {
			Cause string `json:"cause"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Cause == "" {
			return nil
		}
		return FightEndedBody{Cause: DissolveKind(p.Cause)}
	case EventStanceChanged:
		var p struct {
			Between []string `json:"between"`
			Stance  string   `json:"stance"`
		}
		// A stance is between exactly two named factions and is a word: a
		// beat naming one faction, three, an empty id, or no stance has no
		// lawful reading, and is left untyped rather than narrated as a pair
		// with a hole in it.
		if json.Unmarshal(payload, &p) != nil || len(p.Between) != 2 ||
			p.Between[0] == "" || p.Between[1] == "" || p.Stance == "" {
			return nil
		}
		return StanceChangedBody{Between: p.Between, Stance: p.Stance}
	case EventArrived:
		var p struct {
			ID   string           `json:"id"`
			Kind string           `json:"kind"`
			Cell spatial.Position `json:"cell"`
		}
		// An arrival names a thing and says which kind of thing it is; a
		// kind outside the closed set is a composition newer than this build,
		// and a client that cannot tell a monster from a prop cannot narrate
		// it, so the body stays untyped rather than guessing. The cell is
		// always a real cell and carries no absence to check for.
		if json.Unmarshal(payload, &p) != nil || p.ID == "" ||
			(p.Kind != string(PlacementMonster) && p.Kind != string(PlacementProp)) {
			return nil
		}
		return ArrivedBody{ID: p.ID, Kind: PlacementKind(p.Kind), Cell: p.Cell}
	case EventStruck:
		return structBody(payload, true)
	case EventMissed:
		return structBody(payload, false)
	case EventDeathSave:
		return deathSaveEventBody(payload)
	case EventActivated:
		var p struct {
			Actor   string `json:"actor"`
			Ability struct {
				Ref  string `json:"ref"`
				Name string `json:"name"`
			} `json:"ability"`
			Target string `json:"target"`
		}
		if json.Unmarshal(payload, &p) != nil ||
			p.Actor == "" || p.Ability.Ref == "" || p.Ability.Name == "" {
			return nil
		}
		return ActivatedBody{
			Actor: p.Actor,
			Ability: AbilityRef{
				Ref: p.Ability.Ref, Name: p.Ability.Name,
			},
			Target: p.Target,
		}
	case EventActivationResult:
		return activationResultBody(payload)
	case EventDowned:
		var p struct {
			Member string `json:"member"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" {
			return nil
		}
		return DownedBody{Member: p.Member}
	case EventLooted:
		var p struct {
			Member string `json:"member"`
			Target string `json:"target"`
		}
		// THE COMPOSITION'S KEYS ARE member/target AND THE BODY'S ARE
		// looter/body. The beat is written by a verb whose input names an
		// actor and a target; the body is read by a client narrating a
		// sentence, and "looter" and "body" are the words that sentence uses.
		// The rename is one line here rather than a second vocabulary
		// upstream — the same move kindFor makes for "down"/"downed".
		if json.Unmarshal(payload, &p) != nil || p.Member == "" || p.Target == "" {
			return nil
		}
		return LootedBody{Looter: p.Member, Body: p.Target}
	case EventHeld:
		var p struct {
			Holder string `json:"holder"`
			Prop   string `json:"prop"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Holder == "" || p.Prop == "" {
			return nil
		}
		return HeldBody{Holder: p.Holder, Prop: p.Prop}
	case EventDropped:
		var p struct {
			Member   string           `json:"member"`
			Prop     string           `json:"prop"`
			Position spatial.Position `json:"position"`
		}
		// "position" in, At out — the composition's own key for a cell, read
		// into the word a client narrating a drop uses, exactly as EventMoved
		// reads "position" into MovedBody.To one arm above.
		if json.Unmarshal(payload, &p) != nil || p.Member == "" || p.Prop == "" {
			return nil
		}
		return DroppedBody{Member: p.Member, Prop: p.Prop, At: p.Position}
	case EventDoorRevealed:
		// The composition writes doorways as bare from/to pairs; the body
		// re-carries them as [AtlasDoorway] entries with Door filled, so
		// the patch appends to the cached atlas's own list without reshaping.
		var p struct {
			Door       string         `json:"door"`
			State      string         `json:"state"`
			Doorways   []AtlasDoorway `json:"doorways"`
			Approaches []DoorApproach `json:"approaches"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Door == "" || p.State == "" {
			return nil
		}
		for i := range p.Doorways {
			p.Doorways[i].Door = p.Door
		}
		return DoorRevealedBody{
			Door: p.Door, State: p.State,
			Doorways: p.Doorways, Approaches: p.Approaches,
		}
	case EventRegionRevealed:
		// The payload's region, props, boundaries, segments and sealed cells
		// carry exactly this package's atlas field names — the beat is the
		// recipient's own atlas answer, sliced — so the types decode directly.
		var p struct {
			Region     AtlasRegion        `json:"region"`
			Props      []AtlasProp        `json:"props"`
			Boundaries []AtlasBoundary    `json:"boundaries"`
			Segments   []AtlasSegment     `json:"segments"`
			Sealed     []spatial.Position `json:"sealed"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Region.ID == "" {
			return nil
		}
		return RegionRevealedBody{
			Region: p.Region, Props: p.Props, Boundaries: p.Boundaries,
			Segments: p.Segments, Sealed: p.Sealed,
		}
	default:
		// EventSceneOpened, EventTick: no body member exists for these — see
		// EventBody's own doc.
		return nil
	}
}

// activationResultBody decodes one result payload and rejects fields belonging
// to a second result shape. Raw field presence is checked separately from typed
// values: JSON null is still present, while a required numeric zero remains a
// valid, present value. The strict object scan rejects duplicate keys at every
// depth it walks, and the healing's roll representation is checked whole: a
// payload carrying both the legacy scalars and a calculation, a calculation
// whose arithmetic or pairing does not hold, or a forbidden null is refused
// rather than guessed at.
func activationResultBody(payload []byte) EventBody {
	outer, ok := strictJSONObject(payload)
	if !ok {
		return nil
	}
	resultPayload, ok := outer["result"]
	if !ok {
		return nil
	}

	var p struct {
		Actor string `json:"actor"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Actor == "" {
		return nil
	}

	fields, ok := strictJSONObject(resultPayload)
	if !ok {
		return nil
	}

	var result struct {
		Kind   encounter.ActivationResultKind `json:"kind"`
		Target string                         `json:"target"`

		Ref  *string `json:"ref"`
		Name *string `json:"name"`

		Amount    *int `json:"amount"`
		Requested *int `json:"requested"`
		// Roll and Modifier are the legacy scalar representation, readable only
		// from payloads written before roll traces.
		Roll     *int `json:"roll"`
		Modifier *int `json:"modifier"`
		Before   *int `json:"before"`
		After    *int `json:"after"`

		Description *string `json:"description"`
		Reason      *string `json:"reason"`
	}
	if json.Unmarshal(resultPayload, &result) != nil || result.Target == "" {
		return nil
	}

	_, refPresent := fields["ref"]
	_, namePresent := fields["name"]
	_, amountPresent := fields["amount"]
	_, requestedPresent := fields["requested"]
	_, rollPresent := fields["roll"]
	_, modifierPresent := fields["modifier"]
	_, beforePresent := fields["before"]
	_, afterPresent := fields["after"]
	_, descriptionPresent := fields["description"]
	_, reasonPresent := fields["reason"]
	calculationRaw, calculationPresent := fields["calculation"]

	healingNumericsPresent := amountPresent && requestedPresent && beforePresent && afterPresent &&
		result.Amount != nil && result.Requested != nil && result.Before != nil && result.After != nil
	identityPresent := refPresent && namePresent &&
		result.Ref != nil && *result.Ref != "" && result.Name != nil && *result.Name != ""
	// Every healing-shaped numeric, legacy scalars included: a non-healing
	// kind may carry none of them, and null presence still counts as
	// presence (the raw key was written).
	numericPresent := amountPresent || requestedPresent || rollPresent ||
		modifierPresent || beforePresent || afterPresent

	body := ActivationResultBody{Actor: p.Actor}
	switch result.Kind {
	case encounter.ResultHealingApplied:
		if !identityPresent || !healingNumericsPresent || descriptionPresent || reasonPresent {
			return nil
		}
		if calculationPresent {
			// The NEW representation: the trace IS the roll record, and it
			// must carry the roll alone — a payload that also carries the
			// legacy scalars is ambiguous and refused.
			if rollPresent || modifierPresent {
				return nil
			}
			calculation, ok := decodeRollCalculation(calculationRaw)
			if !ok || calculation.Total != *result.Requested {
				return nil
			}
			body.HealingApplied = &HealingAppliedBody{
				Target: result.Target, Amount: *result.Amount, Requested: *result.Requested,
				SourceRef: *result.Ref, SourceName: *result.Name,
				HPBefore: *result.Before, HPAfter: *result.After,
				Calculation: calculation,
			}
			return body
		}
		// The legacy representation: scalar roll facts, read as written and
		// never refabricated into a trace.
		if !rollPresent || !modifierPresent || result.Roll == nil || result.Modifier == nil {
			return nil
		}
		body.HealingApplied = &HealingAppliedBody{
			Target: result.Target, Amount: *result.Amount, Requested: *result.Requested,
			Roll: *result.Roll, Modifier: *result.Modifier,
			SourceRef: *result.Ref, SourceName: *result.Name,
			HPBefore: *result.Before, HPAfter: *result.After,
		}
		return body
	case encounter.ResultConditionApplied:
		if !identityPresent || calculationPresent || numericPresent || descriptionPresent || reasonPresent {
			return nil
		}
		body.ConditionApplied = &ConditionAppliedBody{
			Target: result.Target, Ref: *result.Ref, Name: *result.Name,
		}
	case encounter.ResultConditionRemoved:
		if !identityPresent || calculationPresent || numericPresent || descriptionPresent || !reasonPresent ||
			result.Reason == nil || *result.Reason == "" {
			return nil
		}
		body.ConditionRemoved = &ConditionRemovedBody{
			Target: result.Target, Ref: *result.Ref, Name: *result.Name, Reason: *result.Reason,
		}
	case encounter.ResultCapacityGranted:
		if refPresent || namePresent || reasonPresent || calculationPresent || !descriptionPresent ||
			result.Description == nil || *result.Description == "" {
			return nil
		}
		if numericPresent {
			return nil
		}
		body.CapacityGranted = &CapacityGrantedBody{
			Member: result.Target, Description: *result.Description,
		}
	default:
		return nil
	}

	return body
}

func deathSaveEventBody(payload []byte) EventBody {
	outer, ok := strictJSONObject(payload)
	if !ok {
		return nil
	}
	for key := range outer {
		switch key {
		case "beat", "actor", "death_save":
		default:
			return nil
		}
	}
	for _, key := range []string{"beat", "actor", "death_save"} {
		if _, present := outer[key]; !present || isJSONNull(outer[key]) {
			return nil
		}
	}
	var beat, actor string
	if json.Unmarshal(outer["beat"], &beat) != nil || beat != "death_save" ||
		json.Unmarshal(outer["actor"], &actor) != nil || actor == "" {
		return nil
	}

	detail, ok := strictJSONObject(outer["death_save"])
	if !ok {
		return nil
	}
	required := []string{
		"roll", "outcome", "successes_added", "failures_added", "successes", "failures",
		"successes_needed", "failures_remaining", "stabilized", "dead", "recovered",
		"hp_restored", "continuation", "presentation_id",
	}
	if len(detail) != len(required) {
		return nil
	}
	for _, key := range required {
		if _, present := detail[key]; !present || isJSONNull(detail[key]) {
			return nil
		}
	}

	var decoded struct {
		Roll              int                   `json:"roll"`
		Outcome           DeathSaveOutcome      `json:"outcome"`
		SuccessesAdded    int                   `json:"successes_added"`
		FailuresAdded     int                   `json:"failures_added"`
		Successes         int                   `json:"successes"`
		Failures          int                   `json:"failures"`
		SuccessesNeeded   int                   `json:"successes_needed"`
		FailuresRemaining int                   `json:"failures_remaining"`
		Stabilized        bool                  `json:"stabilized"`
		Dead              bool                  `json:"dead"`
		Recovered         bool                  `json:"recovered"`
		HPRestored        int                   `json:"hp_restored"`
		Continuation      DeathSaveContinuation `json:"continuation"`
		PresentationID    string                `json:"presentation_id"`
	}
	if json.Unmarshal(outer["death_save"], &decoded) != nil || decoded.Outcome == "" ||
		decoded.Continuation == "" || decoded.PresentationID == "" {
		return nil
	}
	return DeathSaveBody{
		Actor: actor, Roll: decoded.Roll, Outcome: decoded.Outcome,
		SuccessesAdded: decoded.SuccessesAdded, FailuresAdded: decoded.FailuresAdded,
		Successes: decoded.Successes, Failures: decoded.Failures,
		SuccessesNeeded: decoded.SuccessesNeeded, FailuresRemaining: decoded.FailuresRemaining,
		Stabilized: decoded.Stabilized, Dead: decoded.Dead,
		Recovered: decoded.Recovered, HPRestored: decoded.HPRestored,
		Continuation: decoded.Continuation, PresentationID: decoded.PresentationID,
	}
}

// strictJSONObject walks one JSON object key by key, retaining each field's
// raw value and refusing the ambiguities encoding/json resolves silently:
// a repeated key at the depth it walks, a payload that is not exactly one
// JSON object, and any trailing content after it. It runs at the OUTER beat
// payload boundary and over the nested bodies this package routes through
// strict decoding — activation results, damage components, and their roll
// graphs. Bodies no decoder routes through it (the shared attack-scalar
// pass, advantage/disadvantage source entries) keep encoding/json's tolerant
// decoding, because a body that says a field twice has no lawful reading
// only where this package vouches for the shape.
func strictJSONObject(payload []byte) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, false
	}

	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return nil, false
		}
		key, isString := token.(string)
		if !isString {
			return nil, false
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, false
		}

		var value json.RawMessage
		if decodeErr := decoder.Decode(&value); decodeErr != nil {
			return nil, false
		}
		fields[key] = value
	}

	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return fields, true
}

// isJSONNull reports whether a retained raw value is the null literal —
// present, but carrying no value. Inside a decoded roll body null is never an
// answer: the composition writes absence by omitting a key, so a null is a
// malformed payload, not a zero.
func isJSONNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}

// decodeRollCalculation decodes one persisted roll calculation into the SDK's
// string-ref shape, refusing unknown keys, nulls, duplicate keys at every
// nested depth, and any structural or arithmetic inconsistency the
// composition's own validator replays: face ranges, ordered rerolls, kept
// indices, subtotals, and the total.
func decodeRollCalculation(raw json.RawMessage) (*RollCalculation, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return nil, false
	}
	for key := range fields {
		switch key {
		case "components", "total":
		default:
			return nil, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return nil, false
		}
	}
	componentsRaw, componentsPresent := fields["components"]
	totalRaw, totalPresent := fields["total"]
	if !componentsPresent || !totalPresent {
		return nil, false
	}

	var elements []json.RawMessage
	if json.Unmarshal(componentsRaw, &elements) != nil {
		return nil, false
	}
	calculation := &RollCalculation{}
	for _, element := range elements {
		component, ok := decodeRollComponent(element)
		if !ok {
			return nil, false
		}
		calculation.Components = append(calculation.Components, component)
	}
	if json.Unmarshal(totalRaw, &calculation.Total) != nil {
		return nil, false
	}
	if encounter.ValidateRollCalculation(validationCalculationFor(calculation)) != nil {
		return nil, false
	}
	return calculation, true
}

// decodeRollComponent decodes one persisted roll component, refusing unknown
// keys, nulls, and duplicate keys, and — when the component rolls dice — every
// structural and arithmetic inconsistency the composition's validator can
// replay. A component that contributes no dice and no modifier (the
// multiplier-only trait component's shape) carries only its sourced identity,
// and there is no arithmetic to replay.
func decodeRollComponent(raw json.RawMessage) (RollComponent, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return RollComponent{}, false
	}
	for key := range fields {
		switch key {
		case "source", "dice", "modifier":
		default:
			return RollComponent{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return RollComponent{}, false
		}
	}
	sourceRaw, sourcePresent := fields["source"]
	if !sourcePresent {
		return RollComponent{}, false
	}
	source, ok := decodeRollSource(sourceRaw)
	if !ok {
		return RollComponent{}, false
	}
	component := RollComponent{Source: source}
	if diceRaw, present := fields["dice"]; present {
		trace, ok := decodeDiceTrace(diceRaw)
		if !ok {
			return RollComponent{}, false
		}
		component.Dice = trace
	}
	if modifierRaw, present := fields["modifier"]; present {
		var modifier int
		if json.Unmarshal(modifierRaw, &modifier) != nil {
			return RollComponent{}, false
		}
		component.Modifier = &modifier
	}
	if component.Dice == nil && component.Modifier == nil {
		// No rollable fact: nothing to replay. The source identity is all this
		// decoder can vouch for, and the damage facts beside the roll — a
		// multiplier, on the damage carrier — say whether the component
		// contributed at all.
		return component, true
	}
	if encounter.ValidateRollCalculation(&encounter.RollCalculation{
		Components: []encounter.RollComponent{validationComponentFor(component)},
		Total:      rollComponentTotal(component),
	}) != nil {
		return RollComponent{}, false
	}
	return component, true
}

// decodeRollSource decodes one sourced identity, requiring the canonical ref
// and display name and refusing nulls, duplicates, and unknown keys.
func decodeRollSource(raw json.RawMessage) (RollSource, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return RollSource{}, false
	}
	for key := range fields {
		switch key {
		case "ref", "name", "label":
		default:
			return RollSource{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return RollSource{}, false
		}
	}
	var source RollSource
	refRaw, refPresent := fields["ref"]
	if !refPresent || json.Unmarshal(refRaw, &source.Ref) != nil || source.Ref == "" {
		return RollSource{}, false
	}
	// Canonical ref syntax, the same law the composition's own write-time
	// validation applies to every persisted roll source: a ref this package
	// cannot parse is a corrupted record, not a fact to pass through. The
	// dice-bearing paths replay it again through the composition's
	// validator; a multiplier-only roll's identity is checked HERE, because
	// there is no trace for that validator to reach it through.
	if _, err := core.ParseString(source.Ref); err != nil {
		return RollSource{}, false
	}
	nameRaw, namePresent := fields["name"]
	if !namePresent || json.Unmarshal(nameRaw, &source.Name) != nil || strings.TrimSpace(source.Name) == "" {
		return RollSource{}, false
	}
	if labelRaw, labelPresent := fields["label"]; labelPresent {
		if json.Unmarshal(labelRaw, &source.Label) != nil {
			return RollSource{}, false
		}
	}
	return source, true
}

// decodeDiceTrace decodes one persisted dice pool trace. Faces, rerolls, and
// kept indices are retained exactly as written; their arithmetic is replayed
// by decodeRollComponent's validator, which owns the refusal.
func decodeDiceTrace(raw json.RawMessage) (*DiceTrace, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return nil, false
	}
	for key := range fields {
		switch key {
		case "notation", "die_size", "original_rolls", "rerolls", "final_rolls", "kept_indices", "subtotal":
		default:
			return nil, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return nil, false
		}
	}
	trace := &DiceTrace{}
	notationRaw, notationPresent := fields["notation"]
	if !notationPresent || json.Unmarshal(notationRaw, &trace.Notation) != nil {
		return nil, false
	}
	dieSizeRaw, dieSizePresent := fields["die_size"]
	if !dieSizePresent || json.Unmarshal(dieSizeRaw, &trace.DieSize) != nil {
		return nil, false
	}
	originalRaw, originalPresent := fields["original_rolls"]
	if !originalPresent || json.Unmarshal(originalRaw, &trace.OriginalRolls) != nil ||
		len(trace.OriginalRolls) == 0 {
		return nil, false
	}
	finalRaw, finalPresent := fields["final_rolls"]
	if !finalPresent || json.Unmarshal(finalRaw, &trace.FinalRolls) != nil {
		return nil, false
	}
	subtotalRaw, subtotalPresent := fields["subtotal"]
	if !subtotalPresent || json.Unmarshal(subtotalRaw, &trace.Subtotal) != nil {
		return nil, false
	}
	if rerollsRaw, rerollsPresent := fields["rerolls"]; rerollsPresent {
		var elements []json.RawMessage
		if json.Unmarshal(rerollsRaw, &elements) != nil {
			return nil, false
		}
		for _, element := range elements {
			reroll, ok := decodeDiceReroll(element)
			if !ok {
				return nil, false
			}
			trace.Rerolls = append(trace.Rerolls, reroll)
		}
	}
	if keptRaw, keptPresent := fields["kept_indices"]; keptPresent {
		if json.Unmarshal(keptRaw, &trace.KeptIndices) != nil {
			return nil, false
		}
	}
	return trace, true
}

// decodeDiceReroll decodes one ordered die replacement, requiring its index,
// faces, and source; the replay that checks the faces against the pool is
// decodeRollComponent's.
func decodeDiceReroll(raw json.RawMessage) (DiceReroll, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return DiceReroll{}, false
	}
	for key := range fields {
		switch key {
		case "die_index", "before", "after", "source":
		default:
			return DiceReroll{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return DiceReroll{}, false
		}
	}
	reroll := DiceReroll{}
	dieIndexRaw, dieIndexPresent := fields["die_index"]
	if !dieIndexPresent || json.Unmarshal(dieIndexRaw, &reroll.DieIndex) != nil {
		return DiceReroll{}, false
	}
	beforeRaw, beforePresent := fields["before"]
	if !beforePresent || json.Unmarshal(beforeRaw, &reroll.Before) != nil {
		return DiceReroll{}, false
	}
	afterRaw, afterPresent := fields["after"]
	if !afterPresent || json.Unmarshal(afterRaw, &reroll.After) != nil {
		return DiceReroll{}, false
	}
	sourceRaw, sourcePresent := fields["source"]
	if !sourcePresent {
		return DiceReroll{}, false
	}
	source, ok := decodeRollSource(sourceRaw)
	if !ok {
		return DiceReroll{}, false
	}
	reroll.Source = source
	return reroll, true
}

// rollComponentTotal totals one component's roll facts the way the
// composition's validator does: the dice trace's authoritative subtotal plus
// any present modifier. It never resumms recorded faces.
func rollComponentTotal(component RollComponent) int {
	total := 0
	if component.Dice != nil {
		total += component.Dice.Subtotal
	}
	if component.Modifier != nil {
		total += *component.Modifier
	}
	return total
}

// validationComponentFor projects the SDK's string-ref roll component onto the
// composition's validator shape. The two carry identical fields, so the
// projection is mechanical — and one-directional: it exists so replay
// arithmetic lives in exactly one module instead of a copy here.
func validationComponentFor(component RollComponent) encounter.RollComponent {
	return encounter.RollComponent{
		Source: encounter.RollSource{
			Ref: component.Source.Ref, Name: component.Source.Name, Label: component.Source.Label,
		},
		Dice:     validationDiceTraceFor(component.Dice),
		Modifier: cloneInt(component.Modifier),
	}
}

// validationCalculationFor projects the SDK's string-ref calculation onto the
// composition's validator shape, components in order.
func validationCalculationFor(calculation *RollCalculation) *encounter.RollCalculation {
	if calculation == nil {
		return nil
	}
	clone := &encounter.RollCalculation{Total: calculation.Total}
	if calculation.Components != nil {
		clone.Components = make([]encounter.RollComponent, len(calculation.Components))
		for i, component := range calculation.Components {
			clone.Components[i] = validationComponentFor(component)
		}
	}
	return clone
}

// validationDiceTraceFor projects the SDK's dice trace onto the composition's
// validator shape, or nil when there is none.
func validationDiceTraceFor(trace *DiceTrace) *encounter.DiceTrace {
	if trace == nil {
		return nil
	}
	clone := &encounter.DiceTrace{
		Notation:      trace.Notation,
		DieSize:       trace.DieSize,
		OriginalRolls: append([]int(nil), trace.OriginalRolls...),
		FinalRolls:    append([]int(nil), trace.FinalRolls...),
		KeptIndices:   append([]int(nil), trace.KeptIndices...),
		Subtotal:      trace.Subtotal,
	}
	if trace.Rerolls != nil {
		clone.Rerolls = make([]encounter.DiceReroll, len(trace.Rerolls))
		for i, reroll := range trace.Rerolls {
			clone.Rerolls[i] = encounter.DiceReroll{
				DieIndex: reroll.DieIndex, Before: reroll.Before, After: reroll.After,
				Source: encounter.RollSource{
					Ref: reroll.Source.Ref, Name: reroll.Source.Name, Label: reroll.Source.Label,
				},
			}
		}
	}
	return clone
}

// cloneInt returns an independently owned copy of value, nil-safe.
func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

// beatAttack is the wire shape Record's payload writes an attack identity
// as (attack.go's recordFor) — decoded here rather than re-derived, since
// this package is the one that wrote it in the first place.
type beatAttack struct {
	Ref        string `json:"ref"`
	Name       string `json:"name"`
	DamageType string `json:"damage_type"`
}

func (a beatAttack) toRef() AttackRef {
	return AttackRef{Ref: a.Ref, Name: a.Name, DamageType: DamageType(a.DamageType)}
}

// structBody decodes a struck or missed outcome beat's shared fields.
// wantAmount distinguishes the two: a struck beat carries Amount and
// Critical, a missed one carries neither, matching StruckBody/MissedBody's
// own shapes. targets is always exactly one member for an attack
// (recordFor's own shape); a beat with none does not type.
//
// The outer object is scanned strictly — a duplicate key at the payload's
// own level has no lawful reading — and each damage component is decoded
// through its own strict scan, which also rejects a payload carrying both
// the legacy scalars and a roll trace, forbidden nulls, unknown keys inside
// known roll bodies, and rolls whose arithmetic does not replay. A missed
// beat carries no components by contract, so their raw presence is left to
// the scalar pass.
func structBody(payload []byte, wantAmount bool) EventBody {
	outer, ok := strictJSONObject(payload)
	if !ok {
		return nil
	}
	for key, value := range outer {
		switch key {
		case "beat", "actor", "targets", "roll", "total", "against", "amount", "critical",
			"attack", "damage_components", "advantage_sources", "disadvantage_sources":
			if isJSONNull(value) {
				return nil
			}
		}
	}

	var p struct {
		Actor               string                 `json:"actor"`
		Targets             []string               `json:"targets"`
		Roll                int                    `json:"roll"`
		Total               int                    `json:"total"`
		Against             int                    `json:"against"`
		Amount              int                    `json:"amount"`
		Critical            bool                   `json:"critical"`
		Attack              beatAttack             `json:"attack"`
		AdvantageSources    []AttackModifierSource `json:"advantage_sources"`
		DisadvantageSources []AttackModifierSource `json:"disadvantage_sources"`
	}
	if json.Unmarshal(payload, &p) != nil ||
		p.Actor == "" || len(p.Targets) != 1 || p.Attack.Ref == "" {
		// EXACTLY one target, not merely "at least one": recordFor's own
		// shape always writes one for an attack (attack.go), so a payload
		// naming zero or several is not a shape this decoder recognises
		// rather than an ambiguous one to guess at (Copilot, PR #1174).
		return nil
	}
	// A missed beat carries NO damage components by contract (MissedBody's own
	// doc): the composition only writes the key for a strike that landed. So
	// on a miss the key's mere presence — well-formed or malformed, decoded or
	// not — is a shape this decoder does not recognise, refused BEFORE any
	// decode rather than decoded and silently dropped.
	if !wantAmount {
		if _, present := outer["damage_components"]; present {
			return nil
		}
	}
	components, ok := decodeDamageComponents(outer["damage_components"])
	if !ok {
		return nil
	}
	if wantAmount {
		return StruckBody{
			Attacker: p.Actor, Target: p.Targets[0],
			Roll: p.Roll, Total: p.Total, Against: p.Against, Damage: p.Amount,
			Attack: p.Attack.toRef(), Critical: p.Critical,
			DamageComponents: components,
			AdvantageSources: p.AdvantageSources, DisadvantageSources: p.DisadvantageSources,
		}
	}
	return MissedBody{
		Attacker: p.Actor, Target: p.Targets[0],
		Roll: p.Roll, Total: p.Total, Against: p.Against, Attack: p.Attack.toRef(),
	}
}

// decodeDamageComponents decodes a struck beat's ordered damage components.
// nil means the key was absent — the composition omits it when a swing dealt
// no componented damage — and absence is legal.
func decodeDamageComponents(raw json.RawMessage) ([]DamageComponent, bool) {
	if raw == nil {
		return nil, true
	}
	var elements []json.RawMessage
	if json.Unmarshal(raw, &elements) != nil {
		return nil, false
	}
	if len(elements) == 0 {
		return nil, true
	}
	components := make([]DamageComponent, 0, len(elements))
	for _, element := range elements {
		component, ok := decodeDamageComponent(element)
		if !ok {
			return nil, false
		}
		components = append(components, component)
	}
	return components, true
}

// damageComponentKeys are the field names one persisted damage component may
// carry: the trace representation's four, the multiplier beside them, and the
// legacy scalars that a pre-trace payload reads back through.
var damageComponentKeys = map[string]struct{}{
	"source": {}, "roll": {}, "damage_type": {}, "multiplier": {},
	"source_ref": {}, "dice": {}, "final_rolls": {}, "flat_bonus": {},
}

// decodeDamageComponent decodes one damage component in exactly one of its two
// representations. A component carrying the roll trace AND any legacy scalar
// is mixing two shapes and is refused; so is one carrying neither. The legacy
// scalars are read as written — the decoder never fabricates a trace from
// them.
func decodeDamageComponent(raw json.RawMessage) (DamageComponent, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return DamageComponent{}, false
	}
	for key := range fields {
		if _, known := damageComponentKeys[key]; !known {
			return DamageComponent{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return DamageComponent{}, false
		}
	}

	_, rollPresent := fields["roll"]
	legacyPresent := false
	for _, key := range []string{"source_ref", "dice", "final_rolls", "flat_bonus", "multiplier"} {
		if _, present := fields[key]; present {
			legacyPresent = true
		}
	}
	if rollPresent {
		// The NEW representation. A multiplier rides BESIDE the roll as a
		// damage fact and the two are one shape together; the four scalar
		// roll facts are the OLD shape's representation of the same question
		// and cannot coexist with a trace.
		for _, key := range []string{"source_ref", "dice", "final_rolls", "flat_bonus"} {
			if _, present := fields[key]; present {
				return DamageComponent{}, false
			}
		}
	} else if !legacyPresent {
		// Neither representation — with no roll and no legacy scalar fact,
		// the multiplier among them, there is nothing to read.
		return DamageComponent{}, false
	}

	var scalar struct {
		Source     string   `json:"source"`
		SourceRef  string   `json:"source_ref"`
		Dice       string   `json:"dice"`
		FinalRolls []int    `json:"final_rolls"`
		FlatBonus  int      `json:"flat_bonus"`
		DamageType string   `json:"damage_type"`
		Multiplier *float64 `json:"multiplier"`
	}
	if json.Unmarshal(raw, &scalar) != nil || scalar.Source == "" {
		return DamageComponent{}, false
	}

	component := DamageComponent{
		Source: scalar.Source, DamageType: DamageType(scalar.DamageType), Multiplier: scalar.Multiplier,
	}
	if rollPresent {
		roll, ok := decodeRollComponent(fields["roll"])
		if !ok {
			return DamageComponent{}, false
		}
		component.Roll = roll
		// A sourced identity alone contributes nothing — the composition
		// refuses a component with neither dice, a modifier, nor a multiplier
		// at write time, and the multiplier-only shape is the one way a
		// component with neither rollable fact has a story to tell.
		if roll.Dice == nil && roll.Modifier == nil && component.Multiplier == nil {
			return DamageComponent{}, false
		}
		return component, true
	}
	component.SourceRef = scalar.SourceRef
	component.Dice = scalar.Dice
	component.FinalRolls = scalar.FinalRolls
	component.FlatBonus = scalar.FlatBonus
	return component, true
}
