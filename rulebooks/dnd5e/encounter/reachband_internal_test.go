// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// reachband_internal_test.go is R11 (rpg-project#569, ruled on the castle
// walk): an enemy is "in reach" only when the creature can attack THIS PICK.
// Reach is a statement about striking. A crossbow bandit on the world clock
// — where no attack is ever affordable — with a thug ten cells off used to
// read `enemy: reach`, which cleared `seen`, and its only reach row was an
// attack it could not pay for: no eligible row at all, and the patrol froze.

var (
	reachCrossbow = core.Ref{Module: "dnd5e", Type: "weapons", ID: "light-crossbow"}
	reachScimitar = core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"}
)

// banditView is one opposed, standing sighting within the given action's
// range, with the budget given.
func banditView(action core.Ref, attacksLeft int) MonsterView {
	return MonsterView{
		Self:    "bandit",
		Actions: []ActionView{{Ref: action, RangeFeet: 80}},
		Budget:  TurnBudget{AttacksLeft: attacksLeft, MovementFeet: 30},
		Seen: []SeenMember{{
			ID: "thug", Opposed: true, Standing: true, DistanceCells: 10,
			InReach: map[core.Ref]bool{action: true},
		}},
	}
}

func TestOnTheWorldClockACrossbowSeesRatherThanReaches(t *testing.T) {
	facts := factsFromView(banditView(reachCrossbow, 0))

	require.False(t, facts.EnemyInReach, "no attack is affordable, so nothing is in reach")
	require.True(t, facts.EnemySeen, "the thug ten cells off is seen")
	require.False(t, facts.EnemyRemembered)
}

func TestWithAnAttackLeftTheSameCrossbowReaches(t *testing.T) {
	facts := factsFromView(banditView(reachCrossbow, 1))

	require.True(t, facts.EnemyInReach, "the turn clock's swing makes reach mean what it says")
	require.False(t, facts.EnemySeen, "and the bands stay exclusive")
}

func TestAnAdjacentMeleeCreatureOnTheWorldClockSees(t *testing.T) {
	view := banditView(reachScimitar, 0)
	view.Actions[0].RangeFeet = 5
	view.Seen[0].DistanceCells = 1

	facts := factsFromView(view)

	require.False(t, facts.EnemyInReach)
	require.True(t, facts.EnemySeen, "adjacent, and still only seen: it cannot swing this pick")
}
