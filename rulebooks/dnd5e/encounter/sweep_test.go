// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// sweep_test.go is A SWEEP COVERS WHAT CAN BE SEEN (rpg-project#753, S3): the
// discovery sweep walks the floor within sight + 1 of the observer and learns
// what the whole-floor walk learned; a monster learns like any observer, keeps
// it across a reload, and is written no cells-revealed beat.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestDiscoverCells_EqualsFullSweep: the bounded sweep answers the same cells
// as the whole-floor walk, for every observer it is asked of.
//
//   - The castle with its garrison, every member at its post (cold); then the
//     castle's party alone, after a player walks a few hexes (after a step).
//   - The 10k keep, both shared players at their seats and at a sample of
//     floor cells across the keep.
//   - A rim: sight three, a pillar's centre exactly four hexes off. No ray
//     reaches it; its face does, from the neighbour three hexes off — the
//     cell the sweep's "+ 1" exists for.
func TestDiscoverCells_EqualsFullSweep(t *testing.T) {
	t.Run("castle", func(t *testing.T) {
		castle, _ := openSite(t, 0, true)
		roster := castle.ExportedRosterIDs()
		got, err := castle.ExportedSweepAgrees(roster, nil)
		require.NoError(t, err, "cold")
		require.Equal(t, len(roster), got.Sweeps, "every member is placed and swept")
		require.Positive(t, got.Learned)

		// A step, on the castle with nobody to fight: the first player walks
		// a few hexes to floor it sees.
		quiet, _ := openSite(t, 0, false)
		stepAway(t, quiet, keepPlayers[0])
		_, err = quiet.ExportedSweepAgrees(quiet.ExportedRosterIDs(), nil)
		require.NoError(t, err, "after a step")
	})

	t.Run("keep", func(t *testing.T) {
		keep := sharedKeep10k(t)
		full, err := keep.Atlas()
		require.NoError(t, err)
		got, err := keep.ExportedSweepAgrees(keepPlayers[:2], nil)
		require.NoError(t, err, "at the seats")
		require.Equal(t, 2, got.Sweeps)
		// Elsewhere on the keep: six floor cells spread over the field.
		stride := len(full.Cells) / 6
		for i := 0; i < 6; i++ {
			at := map[encounter.MemberID]spatial.Position{keepPlayers[0]: full.Cells[i*stride+stride/2]}
			_, err := keep.ExportedSweepAgrees(keepPlayers[:1], at)
			require.NoError(t, err, "from %v", at[keepPlayers[0]])
		}
	})

	t.Run("rim", func(t *testing.T) {
		const hero = encounter.MemberID("hero")
		pillarAt := cellAt(6, 12)
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{
				Canvas:  openAir(),
				Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 24, 24)},
				Placed: []encounter.PlacedPropInput{{
					ID:                "pillar",
					Placement:         spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: 3, D: 3}}, Origin: centreOf(pillarAt)},
					BlocksMovement:    true,
					BlocksLineOfSight: true,
				}},
			},
			Members:      []encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 12}}},
			Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
			Capabilities: knowledgeCapabilities(&sightList{fallback: 3}),
		})
		require.NoError(t, err)
		at := positionOfMember(t, enc, hero)
		require.InDelta(t, 4, enc.Distance(at, pillarAt), 0, "precondition: the pillar stands one past sight")
		got, err := enc.ExportedSweepAgrees([]encounter.MemberID{hero}, nil)
		require.NoError(t, err)
		require.Positive(t, got.Rim, "the pillar's face is learned from one past sight")

		atlas, err := enc.AtlasFor(hero)
		require.NoError(t, err)
		require.Contains(t, atlas.Cells, pillarAt, "and the launch's own sweep learned it")
	})
}

// stepAway walks a member to the first cell it sees, three to five hexes off,
// that the pathfinder reaches and it may stand on.
func stepAway(t *testing.T, enc *encounter.Encounter, who encounter.MemberID) {
	t.Helper()
	from := positionOfMember(t, enc, who)
	visible, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: who})
	require.NoError(t, err)
	for _, to := range visible.Cells {
		if d := enc.Distance(from, to); d < 3 || d > 5 {
			continue
		}
		route, err := enc.Route(encounter.RouteInput{Mover: who, Policy: encounter.MoveToward, Anchor: to, Budget: 12})
		if err != nil || route.StoppedBy != "" || len(route.Path) == 0 {
			continue
		}
		path := route.Path
		if path[len(path)-1] != to {
			path = append(path, to) // a walk toward a cell stops beside it
		}
		for i, cell := range path {
			_, err := enc.Step(&encounter.StepInput{Member: who, To: cell, EndWalk: i == len(path)-1})
			require.NoError(t, err)
		}
		require.NotEqual(t, from, positionOfMember(t, enc, who), "the member moved")
		return
	}
	require.Fail(t, "no reachable cell three to five hexes off", "%s at %v", who, from)
}

// knowledgeCapabilities is the quiet host the knowledge tests run under,
// with this sight.
func knowledgeCapabilities(sight encounter.Sight) encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         sight,
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		CheckResolver: findsNothing{},
		Witness:       nobodyPerceives{},
		Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

// Three rooms in a row joined by shut doors on row 1 (the cell knowledge
// fixture): the hero at the entry door, a monster at the far one.
const (
	sweepHero    = encounter.MemberID("a")
	sweepMonster = encounter.MemberID("m")
)

func monsterRooms(t *testing.T) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: (&CellKnowledgeSuite{}).field(),
		Members: []encounter.MemberInput{
			{ID: sweepHero, Kind: encounter.KindPlayer, Position: cellAt(2, 1)},
			{ID: sweepMonster, Kind: encounter.KindMonster, Position: cellAt(7, 1)},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: knowledgeCapabilities(&sightList{fallback: 20}),
	})
	require.NoError(t, err)
	return enc
}

func knownTo(t *testing.T, enc *encounter.Encounter, member encounter.MemberID) map[spatial.Position]bool {
	t.Helper()
	atlas, err := enc.AtlasFor(member)
	require.NoError(t, err)
	return cellSet(atlas.Cells)
}

func revealBeats(t *testing.T, enc *encounter.Encounter, member encounter.MemberID) int {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(t, err)
	n := 0
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatCellsRevealed {
			n++
		}
	}
	return n
}

// roomCell is a cell of the middle room on the doors' row, behind both shut
// doors and in line with both.
var roomCell = cellAt(4, 1)

// TestMonster_WritesNoRevealBeat: the far door opens and the monster sees the
// middle room. It learns the room, as a seat would, and is written no
// cells-revealed beat; the hero, whose door opens next, learns the room and
// is written one.
func TestMonster_WritesNoRevealBeat(t *testing.T) {
	enc := monsterRooms(t)
	require.False(t, knownTo(t, enc, sweepMonster)[roomCell], "precondition: the room is behind the far door")

	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "far-door"})
	require.NoError(t, err)
	require.True(t, knownTo(t, enc, sweepMonster)[roomCell], "the monster learns the room it now sees")
	require.Zero(t, revealBeats(t, enc, sweepMonster), "and is written no cells-revealed beat")

	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"})
	require.NoError(t, err)
	require.True(t, knownTo(t, enc, sweepHero)[roomCell], "the hero learns the room")
	require.Positive(t, revealBeats(t, enc, sweepHero), "a seat's learning is delivered as a beat")
}

// TestMonster_ReloadKeepsKnowledge: what a monster learned after the scene
// opened survives a save and a reload, and the reloaded run's next look
// neither loses nor relearns it.
func TestMonster_ReloadKeepsKnowledge(t *testing.T) {
	enc := monsterRooms(t)
	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "far-door"})
	require.NoError(t, err)
	before := knownTo(t, enc, sweepMonster)
	require.True(t, before[roomCell], "precondition: the monster learned the room")

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	// The far door shuts in the saved run, so the reloaded monster cannot
	// simply see the room again: what it knows of it is what it kept.
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: knowledgeCapabilities(&sightList{fallback: 20}),
	})
	require.NoError(t, err)
	_, err = reloaded.CloseDoor(&encounter.CloseDoorInput{Door: "far-door"})
	require.NoError(t, err)
	_, err = reloaded.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{sweepMonster}})
	require.NoError(t, err)
	require.Equal(t, before, knownTo(t, reloaded, sweepMonster), "the reloaded monster knows exactly what it knew")
}
