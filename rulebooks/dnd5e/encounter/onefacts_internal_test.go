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
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestASocialVerdictAndADrivenTurnAgreeOnEnemyInReach(t *testing.T) {
	rulebook := &oneDown{}
	club := ActionView{Ref: core.Ref{Module: "test", Type: "actions", ID: "club"}, Name: "Club", RangeFeet: 5}
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 12, 12)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 3, Y: 2}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  UnobservedEquipment{},
			Conditions: UnobservedEquipment{},
			Sheets: sheetFacts{
				"alice":  {SpeedFeet: 30},
				"goblin": {SpeedFeet: 30, Actions: []ActionView{club}},
			},
			Participation: rulebook,
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
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

// TestASocialVerdictAndADrivenTurnAgreeOnEnemyLost: a lost memory
// (rpg-project#668 R3) reads `enemy: lost` through both doors, and never as
// remembered.
func TestASocialVerdictAndADrivenTurnAgreeOnEnemyLost(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 12, 12)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 8, Y: 8}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight: propagationSight{}, Equipment: UnobservedEquipment{}, Conditions: UnobservedEquipment{},
			Sheets:        sheetFacts{"alice": {SpeedFeet: 30}, "goblin": {SpeedFeet: 30}},
			Participation: &oneDown{}, Initiative: orderAsGiven{}, Driver: passDriver{},
			Actors: Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)
	payload, err := EncodeSightTestimony(SightTestimony{State: LocationLost, Position: spatial.Position{X: 4, Y: 4}})
	require.NoError(t, err)
	_, err = enc.intelLog.Report(perception.ReportInput{
		Observer: sightMember("goblin"), Channel: perception.Sight,
		Reports: []perception.Presence{{ID: sightMember("alice"), Payload: payload}},
	})
	require.NoError(t, err)

	social, err := enc.factsFor("goblin")
	require.NoError(t, err)
	goblin := enc.members["goblin"]
	view, err := enc.buildMonsterView(goblin, TurnBudget{MovementFeet: 30}, 1)
	require.NoError(t, err)
	driven := factsFromView(view)

	require.True(t, social.EnemyLost, "a verdict reads the lost memory")
	require.True(t, driven.EnemyLost, "and so does a driven turn")
	require.False(t, social.EnemyRemembered)
	require.False(t, driven.EnemyRemembered)
}
