// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestAPausedTurnSavesItsMovementOnly pins that a turn paused mid-walk stores
// the movement it has left and nothing of its actions (rpg-project#1131): the
// creature's sheet says what it can still pay for, so a reload hands the
// resumed turn the same movement the in-memory pause would have and no count
// of attacks or bonus actions. A negative movement is refused at the door like
// any negative budget.
func TestAPausedTurnSavesItsMovementOnly(t *testing.T) {
	p := &pause{
		kind: PauseTurn, member: "goblin",
		from: spatial.Position{X: 0, Y: 0}, to: spatial.Position{X: 1, Y: 0},
		remaining: []spatial.Position{{X: 1, Y: 0}},
		turn:      &turnPause{round: 1, intent: 0, bound: 4, movementFeet: 25},
	}

	d := pauseDataFrom(p)
	require.Equal(t, 25, d.Turn.Budget.MovementFeet)
	raw, err := json.Marshal(d.Turn.Budget)
	require.NoError(t, err)
	require.JSONEq(t, `{"movement_feet":25}`, string(raw), "the saved budget is movement and nothing else")

	back, err := pauseFrom(d)
	require.NoError(t, err)
	require.Equal(t, 25, back.turn.movementFeet)

	d.Turn.Budget.MovementFeet = -5
	require.ErrorIs(t, validatePause(d, map[core.EntityID]struct{}{"goblin": {}}), ErrInvalidData)
}
