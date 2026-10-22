// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestAPausedWalkTowardASoundKeepsItsCell pins the pause's half of
// rpg-toolkit#2076: a walk toward a heard sound that stops for a reaction
// window keeps the sound's cell through the blob, so the resumed walk still
// ends facing where it rang. A blob naming a member AND a sound to face is
// one no build writes.
func TestAPausedWalkTowardASoundKeepsItsCell(t *testing.T) {
	clang := spatial.Position{X: 4, Y: 2}
	p := &pause{
		kind: PauseTurn, member: "goblin", act: "move:01HF7YAT003RVQKEBZ99Y1AF3E",
		from: spatial.Position{X: 0, Y: 0}, to: spatial.Position{X: 1, Y: 0},
		remaining: []spatial.Position{{X: 1, Y: 0}},
		turn: &turnPause{
			round: 1, intent: 0, bound: 4, terminal: true,
			budget: TurnBudget{AttacksLeft: 1, MovementFeet: 25},
			face:   walkFace{heard: &clang},
		},
	}

	d := pauseDataFrom(p)
	require.NotNil(t, d.Turn.FaceHeard)
	require.Empty(t, d.Turn.Face)
	members := map[core.EntityID]struct{}{"goblin": {}}
	require.NoError(t, validatePause(d, members))

	back, err := pauseFrom(d)
	require.NoError(t, err)
	require.NotNil(t, back.turn.face.heard)
	require.Equal(t, clang, *back.turn.face.heard)

	d.Turn.Face = "alice"
	require.ErrorIs(t, validatePause(d, members), ErrInvalidData)
}
