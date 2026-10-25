// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// attacker_internal_test.go is `attacker` at the driver (rpg-project#669):
// the one who hit the creature, from its own held deeds, named only while
// its own sight testimony places them; otherwise the row is absent and the
// author's `enemy` row is the fallback through the roll.

var revengeSpear = core.Ref{Type: "weapons", ID: "spear"}

// revenge is the captain's table: strike whoever hit him, then strike the
// nearest. Rolled on the lowest face, the first candidate fires.
func revenge() Table {
	return Table{AnswerTime: {
		{Weight: 1, When: []When{{Deed: "attacked", Within: 2}}, Attack: &Selector{Word: SelectorAttacker}},
		{Weight: 1, Attack: &Selector{Word: SelectorEnemy}},
	}}
}

// revengeView is the captain with the fighter and the bard both in reach,
// the fighter nearer, and the deeds the caller lands on him.
func revengeView(deeds ...HeldDeed) MonsterView {
	reach := map[core.Ref]bool{revengeSpear: true}

	return MonsterView{
		Self:    "captain",
		Table:   revenge(),
		Actions: []ActionView{{Ref: revengeSpear, RangeFeet: 5}},
		Seen: []SeenMember{
			{ID: "fighter", Opposed: true, Standing: true, DistanceCells: 1, InReach: reach},
			{ID: "bard", Opposed: true, Standing: true, DistanceCells: 1.5, InReach: reach},
		},
		Deeds:  deeds,
		At:     5,
		Budget: TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// TestAWitnessedBlowNamesTheAttackerOverACloserEnemy: the bard hit the
// captain; the nearer fighter is passed over.
func TestAWitnessedBlowNamesTheAttackerOverACloserEnemy(t *testing.T) {
	decision, err := TableDriver{Roller: lowestDie{}}.Act(revengeView(HeldDeed{Kind: DeedAttack, Actor: "bard", At: 4}))
	require.NoError(t, err)
	require.Equal(t, 0, decision.Pick.Entry, "the attacker row fires")
	require.Equal(t, Attack{Target: "bard", Action: revengeSpear}, decision.Intent)
	require.Equal(t, MemberID("bard"), intentTarget(decision.Intent), "the record names whom it struck")
}

// TestAnAttackerOutOfSightLeavesTheRowOffTheTable: the rogue hit the captain
// and is nowhere he can see. The attacker row is absent, not a Pass, and
// the `enemy` row wins the roll.
func TestAnAttackerOutOfSightLeavesTheRowOffTheTable(t *testing.T) {
	decision, err := TableDriver{Roller: lowestDie{}}.Act(revengeView(HeldDeed{Kind: DeedAttack, Actor: "rogue", At: 4}))
	require.NoError(t, err)
	require.Len(t, decision.Pick.Candidates, 1, "only the enemy row is a candidate")
	require.Equal(t, 1, decision.Pick.Entry)
	require.Equal(t, Attack{Target: "fighter", Action: revengeSpear}, decision.Intent)

	// A BLOW ON AN ALLY IS NOT ONE ON HIM (R8).
	view := revengeView()
	view.AllyDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "bard", At: 4}}
	decision, err = TableDriver{Roller: lowestDie{}}.Act(view)
	require.NoError(t, err)
	require.Equal(t, 1, decision.Pick.Entry, "his attacker rows read blows on him")
}
