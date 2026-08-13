// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

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
	if m.events == nil {
		return DeliveryReport{}
	}

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
// Read from the payload's own "beat" discriminator rather than the tag,
// because the composition's tags are coarser than its beats: "membership"
// covers both a join and an exit, and "movement" covers both a step and a
// doorway crossing. A host switching on kind wants those apart — arriving and
// leaving are opposite events at a table — so the finer discriminator is the
// honest source.
//
// An unrecognised beat becomes EventUnknown rather than being dropped. A client
// that receives something it cannot interpret still learns its sequence
// advanced, which keeps gap-detection working; dropping it would manufacture a
// hole and send every client into a resync it did not need.
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
	case "traversed":
		return EventTraversed
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
	default:
		return EventUnknown
	}
}
