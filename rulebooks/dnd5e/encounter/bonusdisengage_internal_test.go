// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestPausedTurnKeepsItsBonusAction pins that a turn paused mid-walk stores
// its bonus action with the rest of its budget (rpg-toolkit#2032): a reload
// must hand the resumed turn exactly what the in-memory pause would have,
// and a negative count is refused at the door like any negative budget.
func TestPausedTurnKeepsItsBonusAction(t *testing.T) {
	p := &pause{
		kind: PauseTurn, member: "goblin",
		from: spatial.Position{X: 0, Y: 0}, to: spatial.Position{X: 1, Y: 0},
		remaining: []spatial.Position{{X: 1, Y: 0}},
		turn: &turnPause{
			round: 1, intent: 0, bound: 4,
			budget: TurnBudget{AttacksLeft: 1, MovementFeet: 25, BonusActions: 1},
		},
	}

	d := pauseDataFrom(p)
	require.Equal(t, 1, d.Turn.Budget.BonusActions)

	back, err := pauseFrom(d)
	require.NoError(t, err)
	require.Equal(t, p.turn.budget, back.turn.budget)

	d.Turn.Budget.BonusActions = -1
	require.ErrorIs(t, validatePause(d, map[core.EntityID]struct{}{"goblin": {}}), ErrInvalidData)
}
