// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestDungeonSetupConstructs is a smoke test, not a second scene (#929 T5
// trailing): it makes the workbench's demo fixture constructible BY TEST,
// so CI already covers it. Before this existed, the fixture crashed the
// workbench on startup for some span of this wave (both rooms defaulted
// to Origin (0,0), which W2 rejects as overlapping) and nothing caught
// it — six review passes read the library and never ran the binary. A
// future W-law addition that invalidates this fixture now fails this
// test instead of surfacing only when a human launches the workbench.
func TestDungeonSetupConstructs(t *testing.T) {
	enc, err := encounter.NewEncounter(dungeonSetup())
	if err != nil {
		t.Fatalf("the demo dungeon must construct: %v", err)
	}

	if _, err := enc.Atlas(); err != nil {
		t.Fatalf("the demo dungeon's Atlas must resolve: %v", err)
	}
}
