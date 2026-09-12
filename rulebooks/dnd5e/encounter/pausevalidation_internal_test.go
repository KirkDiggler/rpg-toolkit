// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// pausevalidation_internal_test.go asks validatePausedTurn directly, which a
// test outside the package cannot.
//
// TWO ARMS ENFORCE THE SAME RULE and that is deliberate: validatePausedTurn is
// the trust boundary and runs BEFORE anything is constructed (R5), while
// pausedTurnFrom parses the cause again afterwards and keeps its error arm so
// neither half can start lying by silence. From outside, a bad blob is refused
// either way and the two are indistinguishable — which is what makes deleting
// the validator invisible to every behavioural test in the suite. So this one
// calls it by name.

// aPausedTurnBlob is the smallest shape validatePausedTurn accepts, so each
// case below changes exactly one thing.
func aPausedTurnBlob() *PausedTurnData {
	cell := PositionData{X: 1, Y: 0}
	return &PausedTurnData{
		Member:    "goblin",
		Round:     1,
		From:      PositionData{X: 0, Y: 0},
		To:        cell,
		Remaining: []PositionData{cell},
		Budget:    TurnBudgetData{AttacksLeft: 1, MovementFeet: 25},
		Intent:    0,
		Bound:     8,
	}
}

func TestValidatePausedTurnReadsTheCauseBeforeAnythingIsBuilt(t *testing.T) {
	members := map[core.EntityID]struct{}{"goblin": {}}

	t.Run("no cause at all is a walk the creature chose", func(t *testing.T) {
		require.NoError(t, validatePausedTurn(aPausedTurnBlob(), members),
			"a Move's pause names no cause, and the empty string is how it says so")
	})

	t.Run("a cause that is a ref", func(t *testing.T) {
		blob := aPausedTurnBlob()
		blob.Cause = "dnd5e:conditions:commanded"
		require.NoError(t, validatePausedTurn(blob, members))
	})

	t.Run("a cause that is not a ref", func(t *testing.T) {
		blob := aPausedTurnBlob()
		blob.Cause = "not-a-ref"
		require.ErrorIs(t, validatePausedTurn(blob, members), ErrInvalidData,
			"a compelled walk names its cause, and a resumed one still has to")
	})
}
