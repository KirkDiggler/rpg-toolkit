// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// TestAttackChainEventNeverPersistsItsFrame: a frozen fold marshals the event,
// and a frame would come back hollow, so it is left out entirely.
func TestAttackChainEventNeverPersistsItsFrame(t *testing.T) {
	event := AttackChainEvent{
		AttackerID: "rogue",
		TargetID:   "goblin",
		Frame: contributions.Frame{
			Actor:    "rogue",
			Target:   contributions.Known("goblin"),
			Action:   contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)},
			Complete: true,
		},
	}

	blob, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), "Frame") || strings.Contains(string(blob), "Complete") {
		t.Fatalf("the frame was persisted: %s", blob)
	}
	var back AttackChainEvent
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Frame.Actor != "" || back.Frame.Complete {
		t.Fatalf("a restored event carries a frame: %+v", back.Frame)
	}
}
