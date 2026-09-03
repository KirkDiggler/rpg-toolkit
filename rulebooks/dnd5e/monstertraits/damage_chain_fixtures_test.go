// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// intPtr returns a pointer to v, so a present zero modifier stays present.
func intPtr(v int) *int { return &v }

// testDiceTrace builds a self-consistent dice trace for one pool of faces.
func testDiceTrace(dieSize int, faces ...int) *dnd5eEvents.DiceTrace {
	subtotal := 0
	for _, face := range faces {
		subtotal += face
	}
	return &dnd5eEvents.DiceTrace{
		Notation:      dice.SimplePool(len(faces), dieSize, 0).Notation(),
		DieSize:       dieSize,
		OriginalRolls: faces,
		FinalRolls:    slices.Clone(faces),
		Subtotal:      subtotal,
	}
}
