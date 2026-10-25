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

// target_internal_test.go is `target: attacker | closest` at the driver
// (rpg-project#669): which member an `enemy` names, read from the decider's
// own testimony and nothing else, and which policy named it.

// spear is the one action the views below strike with.
var targetSpear = core.Ref{Type: "weapons", ID: "spear"}

// twoEnemies is a view with a fighter in reach and a bard four cells off,
// both opposed and standing, and whatever deeds the caller lands on it.
func twoEnemies(deeds ...HeldDeed) MonsterView {
	return MonsterView{
		Self:    "captain",
		Actions: []ActionView{{Ref: targetSpear, RangeFeet: 5}},
		Seen: []SeenMember{
			{ID: "fighter", Opposed: true, Standing: true, DistanceCells: 1,
				InReach: map[core.Ref]bool{targetSpear: true}},
			{ID: "bard", Opposed: true, Standing: true, DistanceCells: 4,
				InReach: map[core.Ref]bool{targetSpear: false}, Path: []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}}},
		},
		Deeds:  deeds,
		Budget: TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// strikes and walks are the two rows the policy stands on.
func strikes(p TargetPolicy) Answer {
	return Answer{Weight: 1, Attack: &Selector{Word: SelectorEnemy, Target: p}}
}

func walks(p TargetPolicy) Answer {
	return Answer{Weight: 1, Toward: &Selector{Word: SelectorEnemy, Target: p}}
}

// TestAttackerNamesWhoeverHitTheDeciderLast: the bard hit the captain, the
// fighter is nearer. `attacker` names the bard and says so; `closest` names
// the fighter, as `enemy` always did.
func TestAttackerNamesWhoeverHitTheDeciderLast(t *testing.T) {
	d := TableDriver{}
	view := twoEnemies(HeldDeed{Kind: DeedAttack, Actor: "bard", At: 3})

	intent, targeted, err := d.intentFor(view, strikes(TargetAttacker))
	require.NoError(t, err)
	require.Equal(t, Attack{Target: "bard", Action: targetSpear}, intent)
	require.Equal(t, TargetAttacker, targeted)

	intent, targeted, err = d.intentFor(view, walks(TargetAttacker))
	require.NoError(t, err)
	require.Equal(t, MemberID("bard"), intent.(Move).Target, "toward walks to the one who hit it")
	require.Equal(t, TargetAttacker, targeted)

	intent, targeted, err = d.intentFor(view, strikes(TargetClosest))
	require.NoError(t, err)
	require.Equal(t, Attack{Target: "fighter", Action: targetSpear}, intent)
	require.Equal(t, TargetClosest, targeted)
}

// TestAttackerFallsBackToTheClosestAndSaysSo: nobody hit the captain, or the
// one who did is out of his sight, or down: the closest, named as the closest.
func TestAttackerFallsBackToTheClosestAndSaysSo(t *testing.T) {
	d := TableDriver{}

	for name, view := range map[string]MonsterView{
		"nobody hit him":               twoEnemies(),
		"his attacker is out of sight": twoEnemies(HeldDeed{Kind: DeedAttack, Actor: "rogue", At: 3}),
		"his attacker is down": func() MonsterView {
			v := twoEnemies(HeldDeed{Kind: DeedAttack, Actor: "bard", At: 3})
			v.Seen[1].Standing = false
			return v
		}(),
		"his attacker is no longer opposed": func() MonsterView {
			v := twoEnemies(HeldDeed{Kind: DeedAttack, Actor: "bard", At: 3})
			v.Seen[1].Opposed = false
			return v
		}(),
		// THE MOST RECENT OR NONE: the rogue hit him last and is gone; the
		// bard hit him earlier and is in sight. The author wrote "the one
		// who hit him", so it is the closest, not the bard.
		"the latest attacker is out of sight, an earlier one is not": twoEnemies(
			HeldDeed{Kind: DeedAttack, Actor: "bard", At: 2}, HeldDeed{Kind: DeedAttack, Actor: "rogue", At: 5}),
	} {
		intent, targeted, err := d.intentFor(view, strikes(TargetAttacker))
		require.NoError(t, err, name)
		require.Equal(t, Attack{Target: "fighter", Action: targetSpear}, intent, name)
		require.Equal(t, TargetClosest, targeted, name)
	}
}

// TestAttackerReadsOnlyTheDecidersOwnTestimony (R8): a blow the captain's
// side took, and a blow the captain dealt, are not a blow on the captain; a
// remembered attacker is somewhere to walk, not somebody to strike.
func TestAttackerReadsOnlyTheDecidersOwnTestimony(t *testing.T) {
	d := TableDriver{}

	view := twoEnemies()
	view.AllyDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "bard", At: 3}}
	view.OwnDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "captain", At: 4}}
	intent, targeted, err := d.intentFor(view, strikes(TargetAttacker))
	require.NoError(t, err)
	require.Equal(t, Attack{Target: "fighter", Action: targetSpear}, intent, "an ally's blow is not the captain's")
	require.Equal(t, TargetClosest, targeted)

	remembered := twoEnemies(HeldDeed{Kind: DeedAttack, Actor: "rogue", At: 3})
	remembered.Seen = remembered.Seen[:1]
	remembered.Seen[0].InReach = map[core.Ref]bool{targetSpear: false}
	remembered.Seen[0].Path = []spatial.Position{{X: 0, Y: 1}}
	remembered.Remembered = []RememberedMember{{ID: "rogue", Opposed: true, DistanceCells: 5,
		Path: []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}}}}

	intent, targeted, err = d.intentFor(remembered, walks(TargetAttacker))
	require.NoError(t, err)
	require.Equal(t, MemberID("rogue"), intent.(Move).Target, "toward walks to where it last saw its attacker")
	require.Equal(t, TargetAttacker, targeted)

	intent, targeted, err = d.intentFor(remembered, strikes(TargetAttacker))
	require.NoError(t, err)
	require.Equal(t, MemberID("fighter"), intent.(Attack).Target, "a remembered attacker cannot be struck")
	require.Equal(t, TargetClosest, targeted)
}

// TestATargetPolicySurvivesSaveAndLoad is the S1 lesson for this field: the
// table is saved the way a host saves it (through JSON), loaded back, and
// the loaded row still names its attacker, passes the door and is used.
func TestATargetPolicySurvivesSaveAndLoad(t *testing.T) {
	table := Table{AnswerTime: {strikes(TargetAttacker), walks(TargetClosest)}}

	blob, err := json.Marshal(tableDataFrom(table))
	require.NoError(t, err)
	var data map[string][]AnswerData
	require.NoError(t, json.Unmarshal(blob, &data))
	loaded := tableFromData(data)

	require.NoError(t, validateTable(loaded), "a loaded enemy still names its policy")
	require.Equal(t, TargetAttacker, loaded[AnswerTime][0].Attack.Target)
	require.Equal(t, TargetClosest, loaded[AnswerTime][1].Toward.Target)

	intent, targeted, err := TableDriver{}.intentFor(
		twoEnemies(HeldDeed{Kind: DeedAttack, Actor: "bard", At: 3}), loaded[AnswerTime][0])
	require.NoError(t, err)
	require.Equal(t, MemberID("bard"), intent.(Attack).Target, "and is used")
	require.Equal(t, TargetAttacker, targeted)
}

// TestAnEnemyThatLoadsWithoutAPolicyIsRefused: a saved row that dropped the
// word comes back refused at the door, never as a closest nobody wrote.
func TestAnEnemyThatLoadsWithoutAPolicyIsRefused(t *testing.T) {
	loaded := tableFromData(map[string][]AnswerData{
		string(AnswerTime): {{Weight: 1, Attack: &SelectorData{Word: string(SelectorEnemy)}}},
	})
	require.ErrorIs(t, validateTable(loaded), ErrBadAnswer)
}
