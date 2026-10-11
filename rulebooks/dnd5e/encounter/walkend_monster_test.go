// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestMonsterWalkNeverEndsOnAnAlly pins the monster side of the passage law (rpg-toolkit#2030)
// the player already meets twice: a cell with a non-hostile creature on it
// may be crossed but never stopped on. A driver's path is truncated to its
// movement budget, so the last cell it hands the encounter can be an ally's;
// the walk must stop short of that cell rather than stack two bodies on it.
func TestMonsterWalkNeverEndsOnAnAlly(t *testing.T) {
	goblinStart, ogreCell, beyond := cellAt(2, 1), cellAt(3, 1), cellAt(4, 1)

	cases := map[string][]spatial.Position{
		// The truncated path: one cell, and it is the ogre's.
		"last cell is the ally":        {ogreCell},
		"ally mid-path, standable end": {ogreCell, beyond},
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			driver := &scriptedDriver{intents: []encounter.TurnIntent{
				encounter.Move{Path: path},
				encounter.Pass{},
			}}
			enc, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(1, 1)},
					{ID: goblin, Kind: encounter.KindMonster, Position: goblinStart},
					{ID: "ogre", Kind: encounter.KindMonster, Position: ogreCell},
				},
				Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
				Capabilities: encounter.Capabilities{
					Sight:         everyoneSeesTheWholeMap{},
					Equipment:     encounter.UnobservedEquipment{},
					Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, "ogre": {SpeedFeet: 30}, alice: {}},
					Conditions:    encounter.UnobservedEquipment{},
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

			members, err := enc.Members()
			require.NoError(t, err)
			where := map[encounter.MemberID]spatial.Position{}
			for _, m := range members {
				where[m.ID] = m.Position
			}
			require.Equal(t, ogreCell, where["ogre"], "the ogre did not move")
			require.NotEqual(t, ogreCell, where[goblin], "the goblin ended its walk on the ogre's cell")
			if len(path) > 1 {
				require.Equal(t, beyond, where[goblin], "with a standable end the goblin walks through the ogre to it")
			} else {
				require.Equal(t, goblinStart, where[goblin], "with nowhere to stand the goblin stays put")
			}
		})
	}
}
