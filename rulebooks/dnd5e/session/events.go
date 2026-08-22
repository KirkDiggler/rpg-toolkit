// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
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

// publish fans out everything recorded at or after the baseline sequence.
//
// Called only AFTER the save has landed (S9). Announcing a fact that failed to
// persist is the one ordering mistake that cannot be recovered from: a client
// told the ogre died, and a world in which it did not, and no sequence gap to
// betray the difference.
//
// Delivery is best effort (S10). A failure is reported and the verb still
// succeeds, because the log remains the truth and gapless sequences let a
// client detect what it missed. Failing the verb instead would roll back
// nothing — the world has already changed — and would turn a transient stream
// outage into a spurious error the host must decide how to interpret.
func (m *Manager) publish(
	ctx context.Context, scope *writeScope, snapshot *encounter.EncounterData,
) DeliveryReport {
	events := m.projectEvents(scope, snapshot)
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
			kind, body := decodeBeat(entry.Payload)
			events = append(events, Event{
				Session:     scope.session,
				Seq:         entry.Seq,
				At:          entry.At,
				Correlation: entry.Correlation,
				Recipient:   string(member),
				Kind:        kind,
				Payload:     append([]byte(nil), entry.Payload...),
				Body:        body,
			})
		}
	}

	return events
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
// An unrecognised beat, or one whose declared kind decodes to a shape this
// build's struct does not match, becomes (EventUnknown, nil) rather than
// being dropped. A client that receives something it cannot interpret still
// learns its sequence advanced, which keeps gap-detection working; dropping
// it would manufacture a hole and send every client into a resync it did
// not need. It also means a newer composition can add beats without older
// clients losing their place.
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
	case EventDowned:
		var p struct {
			Member string `json:"member"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Member == "" {
			return nil
		}
		return DownedBody{Member: p.Member}
	default:
		// EventJoined, EventExited, EventEnded, EventSceneOpened, EventTick:
		// no body member exists for these — see EventBody's own doc.
		return nil
	}
}

// beatAttack is the wire shape Record's payload writes an attack identity
// as (attack.go's recordFor) — decoded here rather than re-derived, since
// this package is the one that wrote it in the first place.
// beatAttack is the wire shape Record's payload writes an attack identity
// as (attack.go's recordFor). Ref here is the FULL module:type:id catalog
// ref encounter.Record validates (rpg-toolkit#1172) — "dnd5e:weapons:
// longsword" — NOT the bare id the wire's own AttackRef.ref speaks; toRef
// is where that gets derived, with the SAME parser Record itself validated
// it with.
type beatAttack struct {
	Ref        string `json:"ref"`
	Name       string `json:"name"`
	DamageType string `json:"damage_type"`
}

// toRef derives the wire's AttackRef — bare id, display name, damage type
// — from the beat's own full ref. A ref that fails to parse (unreachable
// from a beat this package itself wrote, since Record already validated
// it at write time; reachable only from a hand-built or corrupted payload)
// leaves the wire Ref empty rather than failing the whole body: Attacker,
// Target, Roll and the rest are still meaningful even when the weapon
// identity did not decode, matching bodyFor's own best-effort contract.
func (a beatAttack) toRef() AttackRef {
	ref := AttackRef{Name: a.Name, DamageType: DamageType(a.DamageType)}
	if parsed, err := core.ParseString(a.Ref); err == nil {
		ref.Ref = string(parsed.ID)
	}
	return ref
}

// structBody decodes a struck or missed outcome beat's shared fields.
// wantAmount distinguishes the two: a struck beat carries Amount and
// Critical, a missed one carries neither, matching StruckBody/MissedBody's
// own shapes. targets is always exactly one member for an attack
// (recordFor's own shape); a beat with none does not type.
func structBody(payload []byte, wantAmount bool) EventBody {
	var p struct {
		Actor    string     `json:"actor"`
		Targets  []string   `json:"targets"`
		Roll     int        `json:"roll"`
		Total    int        `json:"total"`
		Against  int        `json:"against"`
		Amount   int        `json:"amount"`
		Critical bool       `json:"critical"`
		Attack   beatAttack `json:"attack"`
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
		}
	}
	return MissedBody{
		Attacker: p.Actor, Target: p.Targets[0],
		Roll: p.Roll, Total: p.Total, Against: p.Against, Attack: p.Attack.toRef(),
	}
}
