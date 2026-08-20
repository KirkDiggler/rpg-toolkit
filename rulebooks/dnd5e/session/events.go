// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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
			events = append(events, Event{
				Session:     scope.session,
				Seq:         entry.Seq,
				At:          entry.At,
				Correlation: entry.Correlation,
				Recipient:   string(member),
				Kind:        kindOf(entry.Payload),
				Payload:     append([]byte(nil), entry.Payload...),
			})
		}
	}

	return events
}

// kindOf maps a beat onto the wire enum.
//
// TEMPORARY SHAPE, pending toolkit#941. This is the one place the package
// interprets a payload rather than passing it through, and that is a coupling
// worth being uncomfortable about: if the composition changes its payload
// shape, kind detection fails SILENTLY — every event degrades to EventUnknown,
// nothing errors, no test fails, and the symptom is a table where nothing
// narrates correctly.
//
// It is here because the composition's tags are currently coarser than its
// beats: "membership" covers both a join and an exit, "movement" covers both a
// step and a doorway crossing. Four tags stand in front of seven beats and two
// of those groupings pair opposites, so the tag cannot answer "what happened"
// — and a client cannot narrate an arrival and a departure the same way.
//
// When #941 enriches the tags, this reads declared metadata instead of parsed
// content and the coupling disappears.
//
// An unrecognised beat becomes EventUnknown rather than being dropped. A client
// that receives something it cannot interpret still learns its sequence
// advanced, which keeps gap-detection working; dropping it would manufacture a
// hole and send every client into a resync it did not need. It also means a
// newer composition can add beats without older clients losing their place.
func kindOf(payload []byte) EventKind {
	var beat struct {
		Beat string `json:"beat"`
	}
	if err := json.Unmarshal(payload, &beat); err != nil {
		return EventUnknown
	}

	switch beat.Beat {
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
	//
	// This is therefore the one case in this function where the two strings
	// deliberately differ, which is also why its pin is built from
	// encounter.OutcomeDown rather than from a literal: the composition's
	// string is the half that must not drift, and nothing else would notice.
	case "down":
		return EventDowned
	default:
		return EventUnknown
	}
}
