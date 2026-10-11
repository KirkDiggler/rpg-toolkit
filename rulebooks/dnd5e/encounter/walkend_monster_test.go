// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestMonsterWalkNeverEndsOnAnAlly pins the monster side of the passage law
// the player already meets twice (rpg-toolkit#2030): a cell with a
// non-hostile creature on it may be crossed but never stopped on. A driver's
// path is truncated to its movement budget, so the last cell it hands the
// encounter can be an ally's; the walk must stop short of that cell rather
// than stack two bodies on it — and must not count on a cell it will never
// reach to be its destination.
//
// The goblin starts at (2,1) and alice, hostile, stands at (1,1); the ogre is
// its ally and stands where each case puts it.
func TestMonsterWalkNeverEndsOnAnAlly(t *testing.T) {
	goblinStart := cellAt(2, 1)

	cases := map[string]struct {
		ogre, aliceAt spatial.Position
		path          []spatial.Position
		goblinEnds    spatial.Position
	}{
		"last cell is the ally": {
			ogre: cellAt(3, 1), aliceAt: cellAt(1, 1),
			path:       []spatial.Position{cellAt(3, 1)},
			goblinEnds: goblinStart,
		},
		"ally mid-path, standable end": {
			ogre: cellAt(3, 1), aliceAt: cellAt(1, 1),
			path:       []spatial.Position{cellAt(3, 1), cellAt(4, 1)},
			goblinEnds: cellAt(4, 1),
		},
		"floor then ally": {
			ogre: cellAt(4, 1), aliceAt: cellAt(1, 1),
			path:       []spatial.Position{cellAt(3, 1), cellAt(4, 1)},
			goblinEnds: cellAt(3, 1),
		},
		// The gate's probe: floor past a hostile the step will refuse is not
		// a destination, so the ally before it is not a cell to stop on either.
		"ally, then a hostile, then floor": {
			ogre: cellAt(3, 1), aliceAt: cellAt(4, 1),
			path:       []spatial.Position{cellAt(3, 1), cellAt(4, 1), cellAt(5, 1)},
			goblinEnds: goblinStart,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			driver := &scriptedDriver{intents: []encounter.TurnIntent{
				encounter.Move{Path: tc.path},
				encounter.Pass{},
			}}
			enc, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: tc.aliceAt},
					{ID: goblin, Kind: encounter.KindMonster, Position: goblinStart},
					{ID: "ogre", Kind: encounter.KindMonster, Position: tc.ogre},
				},
				Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
				Capabilities: encounter.Capabilities{
					Sight:         everyoneSeesTheWholeMap{},
					Equipment:     encounter.UnobservedEquipment{},
					Conditions:    encounter.UnobservedEquipment{},
					Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, "ogre": {SpeedFeet: 30}, alice: {}},
					Participation: everyoneStanding{},
					Initiative:    arrivalOrder{},
					Driver:        driver,
					Actors: encounter.Actors{
						Striker:   &scriptedStriker{kind: encounter.OutcomeMissed},
						Mover:     quietMover{},
						Announcer: quietAnnouncer{},
					},
				},
			})
			require.NoError(t, err)

			_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			require.NoError(t, err)

			where := positionsOf(t, enc)
			require.Equal(t, tc.ogre, where["ogre"], "the ogre did not move")
			require.NotEqual(t, tc.ogre, where[goblin], "the goblin ended its walk on the ogre's cell")
			require.Equal(t, tc.goblinEnds, where[goblin])
		})
	}
}

// TestWorldWalkNeverEndsOnANonHostile is the same law on the world clock,
// reached through the shipped table driver rather than a scripted path: the
// goblin is told to walk to an authored cell, the only route runs through a
// vendor it is not hostile to, and its one cell of movement this round would
// land exactly on the vendor. It stays put instead.
func TestWorldWalkNeverEndsOnANonHostile(t *testing.T) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: worldField(),
		Members: []encounter.MemberInput{
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(0, 0), Table: walksTo(cellAt(2, 0))},
			{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 5}, "vendor": {}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	_, err = aRound(enc)
	require.NoError(t, err)

	where := positionsOf(t, enc)
	require.Equal(t, cellAt(1, 0), where["vendor"])
	require.NotEqual(t, cellAt(1, 0), where[goblin], "the goblin ended its round on the vendor's cell")
	require.Equal(t, cellAt(0, 0), where[goblin])
}

func positionsOf(t *testing.T, enc *encounter.Encounter) map[encounter.MemberID]spatial.Position {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	where := map[encounter.MemberID]spatial.Position{}
	for _, m := range members {
		where[m.ID] = m.Position
	}
	return where
}
