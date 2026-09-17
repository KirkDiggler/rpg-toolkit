// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// The carrier is nil-able so a beat written before the field existed still
// decodes; these pin the other half of that bargain. A verb in THIS build that
// rolled a check and then reached the beat with nothing behind its total is
// refused loudly, because the leniency downstream is exactly what would make
// the omission invisible: a missing optional field on a new beat looks
// identical to an old beat, and no test downstream can tell them apart.
func TestRequireCalculationRefusesASilentlyLostRoll(t *testing.T) {
	err := requireCalculation("intimidate", "alice", nil)

	require.ErrorIs(t, err, ErrNoCalculation)
	require.Contains(t, err.Error(), "intimidate", "the message names the verb that lost it")
	require.Contains(t, err.Error(), `"alice"`, "and who it was rolled for")
}

func TestRequireCalculationAcceptsARollThatIsThere(t *testing.T) {
	require.NoError(t, requireCalculation("intimidate", "alice", &encounter.RollCalculation{}))
}

// The posed sibling guards the window that ASKS, which is where a roll is
// first seen — the player is about to decide with it in front of them (R5).
func TestRequirePosedCalculationRefusesAQuestionWithNoRollBehindIt(t *testing.T) {
	err := requirePosedCalculation("unlock", "alice", nil)

	require.ErrorIs(t, err, ErrNoCalculation)
	require.Contains(t, err.Error(), "unlock")
	require.Contains(t, err.Error(), "paused", "and says it was the question, not the verdict")
}

func TestRequirePosedCalculationAcceptsAQuestionThatCarriesOne(t *testing.T) {
	require.NoError(t, requirePosedCalculation("unlock", "alice", &dnd5eEvents.RollCalculation{}))
}
