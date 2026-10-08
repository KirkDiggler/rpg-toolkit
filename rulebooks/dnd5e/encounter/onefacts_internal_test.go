// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// onefacts_internal_test.go holds the two doors into a creature's facts to one
// projection (rpg-project#539, "What an observer believes and reaches"): a
// social verdict ([Encounter.factsFor]) and a driven turn ([factsFromView] over
// [Encounter.buildMonsterView]) read the same holdings and must agree on
// "enemy in reach" — and a downed subject is in reach of neither.
//
// White-box for bothways_internal_test.go's reason: a Driver is handed a view
// and nothing else, so neither door is reachable from outside.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestASocialVerdictAndADrivenTurnAgreeOnEnemyInReach(t *testing.T) {
	rulebook := &oneDown{}
	club := ActionView{Ref: core.Ref{Module: "test", Type: "actions", ID: "club"}, Name: "Club", RangeFeet: 5}
	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: UnobservedEquipment{},
		Sheets: sheetFacts{
			"alice":  {SpeedFeet: 30},
			"goblin": {SpeedFeet: 30, Actions: []ActionView{club}},
		},
		Standing: rulebook, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 12, 12)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 3, Y: 2}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	both := func() (social, driven Facts) {
		t.Helper()
		social, err := enc.factsFor("goblin")
		require.NoError(t, err)
		view, err := enc.buildMonsterView(enc.members["goblin"], TurnBudget{AttacksLeft: 1, MovementFeet: 30}, 1)
		require.NoError(t, err)
		return social, factsFromView(view)
	}

	social, driven := both()
	require.True(t, social.EnemyInReach, "a standing enemy a club's length away is in reach of a verdict")
	require.True(t, driven.EnemyInReach, "and of a driven turn")

	// The same holdings, the enemy now down: still seen by both, in reach of
	// neither — a body is not an enemy in reach.
	rulebook.who = "alice"
	social, driven = both()
	require.False(t, social.EnemyInReach, "a downed enemy is not in reach of a verdict")
	require.False(t, driven.EnemyInReach, "nor of a driven turn")
	require.True(t, social.EnemySeen, "the body is still seen")
	require.Equal(t, social.EnemySeen, driven.EnemySeen)
	require.Equal(t, social.EnemyRemembered, driven.EnemyRemembered)
}
