// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// testFrame is a valid attack frame with every field a marshalled frame would
// betray.
func testFrame() contributions.Frame {
	return contributions.Frame{
		Actor:    "rogue",
		Target:   contributions.Known("goblin"),
		Action:   contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)},
		Complete: true,
	}
}

// requireNoFrame marshals event, fails if the frame reached the blob, and
// returns the blob for a round trip.
func requireNoFrame(t *testing.T, event any) []byte {
	t.Helper()
	blob, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), "Frame") || strings.Contains(string(blob), "Complete") {
		t.Fatalf("the frame was persisted: %s", blob)
	}
	return blob
}

// TestAttackChainEventNeverPersistsItsFrame: a frozen fold marshals the event,
// and a frame would come back hollow, so it is left out entirely.
func TestAttackChainEventNeverPersistsItsFrame(t *testing.T) {
	blob := requireNoFrame(t, AttackChainEvent{AttackerID: "rogue", TargetID: "goblin", Frame: testFrame()})
	var back AttackChainEvent
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Frame.Actor != "" || back.Frame.Complete {
		t.Fatalf("a restored event carries a frame: %+v", back.Frame)
	}
}

// TestDamageAndOfferEventsNeverPersistTheirFrames: the damage fold and the
// post-roll offers read the same execution frame, and neither event carries
// it into JSON.
func TestDamageAndOfferEventsNeverPersistTheirFrames(t *testing.T) {
	requireNoFrame(t, DamageChainEvent{AttackerID: "rogue", TargetID: "goblin", Frame: testFrame()})
	requireNoFrame(t, PostRollOfferEvent{AttackerID: "rogue", TargetID: "goblin", Frame: testFrame()})
}
