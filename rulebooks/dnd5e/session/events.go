// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"

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
//	    Sessions: repo, Encounters: enc, Events: session.DiscardEvents{},
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
	// The two outcome beats. Unlike every other case here, these strings are
	// not the composition's own vocabulary for a verb it ran — they are the
	// OutcomeKind a rulebook handed it (encounter's Record), which is why the
	// mapping is worth a word: adding an outcome kind upstream means adding a
	// case here, or the new outcome goes out unnamed.
	case "struck":
		return EventStruck
	case "missed":
		return EventMissed
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
			Member string `json:"member"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" {
			return nil
		}
		return ExitedBody{Member: p.Member}
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
	case EventStruck:
		return structBody(payload, true)
	case EventMissed:
		return structBody(payload, false)
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
		// The payload's region, props and boundaries carry exactly this
		// package's atlas field names — the beat is the recipient's own
		// atlas answer, sliced — so the types decode directly.
		var p struct {
			Region     AtlasRegion     `json:"region"`
			Props      []AtlasProp     `json:"props"`
			Boundaries []AtlasBoundary `json:"boundaries"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Region.ID == "" {
			return nil
		}
		return RegionRevealedBody{
			Region: p.Region, Props: p.Props, Boundaries: p.Boundaries,
		}
	default:
		// EventSceneOpened, EventTick: no body member exists for these — see
		// EventBody's own doc.
		return nil
	}
}

// activationResultBody decodes one result payload and rejects fields belonging
// to a second result shape. Pointer fields retain JSON presence, so required
// numeric zeroes remain distinguishable from omitted facts.
func activationResultBody(payload []byte) EventBody {
	var p struct {
		Actor  string          `json:"actor"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Actor == "" || len(p.Result) == 0 {
		return nil
	}

	var result struct {
		Kind   encounter.ActivationResultKind `json:"kind"`
		Target string                         `json:"target"`

		Ref  *string `json:"ref"`
		Name *string `json:"name"`

		Amount    *int `json:"amount"`
		Requested *int `json:"requested"`
		Roll      *int `json:"roll"`
		Modifier  *int `json:"modifier"`
		Before    *int `json:"before"`
		After     *int `json:"after"`

		Description *string `json:"description"`
		Reason      *string `json:"reason"`
	}
	if json.Unmarshal(p.Result, &result) != nil || result.Target == "" {
		return nil
	}

	numericPresent := result.Amount != nil || result.Requested != nil ||
		result.Roll != nil || result.Modifier != nil || result.Before != nil || result.After != nil
	allNumericPresent := result.Amount != nil && result.Requested != nil &&
		result.Roll != nil && result.Modifier != nil && result.Before != nil && result.After != nil
	identityPresent := result.Ref != nil && *result.Ref != "" && result.Name != nil && *result.Name != ""

	body := ActivationResultBody{Actor: p.Actor}
	switch result.Kind {
	case encounter.ResultHealingApplied:
		if !identityPresent || !allNumericPresent || result.Description != nil || result.Reason != nil {
			return nil
		}
		body.HealingApplied = &HealingAppliedBody{
			Target: result.Target, Amount: *result.Amount, Requested: *result.Requested,
			Roll: *result.Roll, Modifier: *result.Modifier,
			SourceRef: *result.Ref, SourceName: *result.Name,
			HPBefore: *result.Before, HPAfter: *result.After,
		}
	case encounter.ResultConditionApplied:
		if !identityPresent || numericPresent || result.Description != nil || result.Reason != nil {
			return nil
		}
		body.ConditionApplied = &ConditionAppliedBody{
			Target: result.Target, Ref: *result.Ref, Name: *result.Name,
		}
	case encounter.ResultConditionRemoved:
		if !identityPresent || numericPresent || result.Description != nil ||
			result.Reason == nil || *result.Reason == "" {
			return nil
		}
		body.ConditionRemoved = &ConditionRemovedBody{
			Target: result.Target, Ref: *result.Ref, Name: *result.Name, Reason: *result.Reason,
		}
	case encounter.ResultCapacityGranted:
		if result.Ref != nil || result.Name != nil || numericPresent || result.Reason != nil ||
			result.Description == nil || *result.Description == "" {
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
func structBody(payload []byte, wantAmount bool) EventBody {
	var p struct {
		Actor               string                 `json:"actor"`
		Targets             []string               `json:"targets"`
		Roll                int                    `json:"roll"`
		Total               int                    `json:"total"`
		Against             int                    `json:"against"`
		Amount              int                    `json:"amount"`
		Critical            bool                   `json:"critical"`
		Attack              beatAttack             `json:"attack"`
		DamageComponents    []DamageComponent      `json:"damage_components"`
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
	if wantAmount {
		return StruckBody{
			Attacker: p.Actor, Target: p.Targets[0],
			Roll: p.Roll, Total: p.Total, Against: p.Against, Damage: p.Amount,
			Attack: p.Attack.toRef(), Critical: p.Critical,
			DamageComponents: p.DamageComponents,
			AdvantageSources: p.AdvantageSources, DisadvantageSources: p.DisadvantageSources,
		}
	}
	return MissedBody{
		Attacker: p.Actor, Target: p.Targets[0],
		Roll: p.Roll, Total: p.Total, Against: p.Against, Attack: p.Attack.toRef(),
	}
}
