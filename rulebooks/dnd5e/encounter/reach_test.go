// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// reach_test.go is INTERACTION BY REACH (rpg-project#753, S2 T1): the oracle
// that answers "can this member be shown this" without projecting an atlas,
// the placement index it reads, the list of what stands within reach and is
// seen now, and the Speak door's reach.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// speakReach is the reach every line in this package's scenes is said
// within: the session's interaction reach, three hexes (rpg-project#753, R4).
const speakReach = 3

// requireShownToMatchesAtlasFor holds the oracle to the projection it
// replaces, for every member of a run: the member's mask to the whole-floor
// walk it replaced, and then every placement and every cell prop
// the field holds, and the floor's cells on their own, is shown exactly
// when the member's own atlas lists one of its cells (every cellStride-th
// cell, so a garrisoned floor stays inside the package's -race budget; the
// hand-built boundary scenes ask every cell). It returns how many
// (member, placement) pairs were NOT shown, so a caller can require the
// fixture actually withholds something.
func requireShownToMatchesAtlasFor(t *testing.T, enc *encounter.Encounter, cellStride int) int {
	t.Helper()
	full, err := enc.Atlas()
	require.NoError(t, err)
	members, err := enc.Members()
	require.NoError(t, err)
	require.NotEmpty(t, members)
	withheld := 0
	for _, m := range members {
		require.NoError(t, enc.ExportedUndiscoveredAgrees(m.ID), "the mask is the walk it replaced")
		atlas, err := enc.AtlasFor(m.ID)
		require.NoError(t, err)
		listed := make(map[spatial.Position]bool, len(atlas.Cells))
		for _, c := range atlas.Cells {
			listed[c] = true
		}
		anyListed := func(cells []spatial.Position) bool {
			for _, c := range cells {
				if listed[c] {
					return true
				}
			}
			return false
		}
		for _, p := range full.Placed {
			want := anyListed(p.Cells)
			require.Equal(t, want, enc.ExportedShownTo(m.ID, p.Cells), "%s: placement %q", m.ID, p.ID)
			if !want {
				withheld++
			}
		}
		for _, p := range full.Props {
			cells := []spatial.Position{p.At}
			require.Equal(t, anyListed(cells), enc.ExportedShownTo(m.ID, cells), "%s: prop %q", m.ID, p.ID)
		}
		for i := 0; i < len(full.Cells); i += cellStride {
			c := full.Cells[i]
			require.Equal(t, listed[c], enc.ExportedShownTo(m.ID, []spatial.Position{c}), "%s: cell %v", m.ID, c)
		}
	}

	return withheld
}

// TestShownTo_MatchesAtlasFor: the oracle is the projection's answer, on the
// castle (garrison and all), on the castle with a room hidden under a
// concealment so a prop stands on concealed floor, on the tombs whose walls
// bring footing and whose secrets are authored, and on the 10k keep.
func TestShownTo_MatchesAtlasFor(t *testing.T) {
	t.Run("castle", func(t *testing.T) {
		enc, _ := openSite(t, 0, true)
		requireShownToMatchesAtlasFor(t, enc, 5)
	})

	t.Run("castle with a prop on concealed floor", func(t *testing.T) {
		compiled := compiledSite(t, 0)
		plain := openCompiled(t, compiled, seatedParty(compiled), false, findsNothing{})
		atlas, err := plain.Atlas()
		require.NoError(t, err)
		// The region one placed prop stands in, away from the party's own.
		start, ok := plain.RegionAt(positionOfMember(t, plain, keepPlayers[0]))
		require.True(t, ok)
		var hide *encounter.RegionInput
		for _, p := range atlas.Placed {
			region, ok := plain.RegionAt(p.Cells[0])
			if !ok || region == start {
				continue
			}
			for i := range compiled.Field.Regions {
				if compiled.Field.Regions[i].ID == region {
					hide = &compiled.Field.Regions[i]
				}
			}
			break
		}
		require.NotNil(t, hide, "precondition: a placed prop stands outside the party's region")
		compiled.Field.Concealments = append(compiled.Field.Concealments, encounter.ConcealmentInput{
			ID:     "under-the-floor",
			Checks: []encounter.CheckApproach{{Ability: "perception", DC: 30}},
			Cells:  hide.Cells,
		})
		// The party alone: the garrison's own masks are the plain castle's
		// case above, and opening it twice is most of this test's budget.
		enc := openCompiled(t, compiled, seatedParty(compiled), false, findsNothing{})
		require.Positive(t, requireShownToMatchesAtlasFor(t, enc, 1),
			"precondition: the concealment withholds a placement from somebody")
	})

	for _, path := range []string{
		"dungeonspec/testdata/reference-tomb-heirloom.yaml",
		"dungeonspec/testdata/world-builder-v4-tomb-vault.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			compiled := compiledFile(t, path)
			enc := openCompiled(t, compiled, seatedParty(compiled), true, findsNothing{})
			requireShownToMatchesAtlasFor(t, enc, 1)
		})
	}

	// THE BOUNDARY RULES, which neither the castle nor the keep exercises:
	// unowned scenery bordering learned floor, an authored wall's unowned
	// footing, and a concealment over both. Sight is three cells, so the far
	// edge of what a member learned borders scenery they never saw.
	t.Run("scenery, footing and a secret on the boundary", func(t *testing.T) {
		var scenery []spatial.Position
		for r := 0; r < 10; r++ {
			scenery = append(scenery, spatial.Position{X: 8, Y: float64(r)})
		}
		scenery = append(scenery, spatial.Position{X: 9, Y: 0})
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{
				Canvas:  openAir(),
				Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 10)},
				Scenery: scenery,
				// The rim's wall, and one between two scenery cells whose near
				// end borders learned floor and whose far end nobody saw.
				Walls: append(seamWallRows(7, 0, 5), wall(8, 0, 9, 0)),
				Concealments: []encounter.ConcealmentInput{{
					ID: "the-niche", Checks: []encounter.CheckApproach{{Ability: "perception", DC: 30}},
					Cells: []spatial.Position{{X: 8, Y: 7}, {X: 8, Y: 3}, {X: 2, Y: 8}},
				}},
			},
			Members: []encounter.MemberInput{
				{ID: "east", Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 5}},
				{ID: "west", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
				// Beside the secret on the open rim, and beside the one on the
				// walled rim: the two places the boundary rules disagree.
				{ID: "rim", Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 6}},
				{ID: "gate", Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 3}},
				// Short-sighted, so the scenery beyond the floor they learned
				// is floor they never saw.
				{ID: "north", Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 1}},
				{ID: "niche", Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 7}},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
			Capabilities: encounter.Capabilities{
				Sight:         &sightList{fallback: 3, reach: map[encounter.MemberID]int{"north": 1, "niche": 1}},
				Equipment:     encounter.UnobservedEquipment{},
				Conditions:    encounter.UnobservedEquipment{},
				Sheets:        zeroSheets{},
				Participation: everyoneStanding{},
				Initiative:    orderAsGiven{},
				Driver:        passDriver{},
				CheckResolver: findsNothing{},
				Witness:       nobodyPerceives{},
				Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
			},
		})
		require.NoError(t, err)
		requireShownToMatchesAtlasFor(t, enc, 1)
	})

	t.Run("keep 10k", func(t *testing.T) {
		// Every placement, and every 37th cell of the floor on its own: the
		// whole floor cell by cell is the castle's job, not the keep's.
		require.Positive(t, requireShownToMatchesAtlasFor(t, sharedKeep10k(t), 37),
			"precondition: the party has not seen the whole keep")
	})
}

// TestPlacementIndex_AgreesWithTheWalk: the bucketed reads answer what the
// whole-floor walk answered, for authored placements and for each one dropped
// across the floor: every placement on the castle; every wall and a seeded
// sample of the rest on the 10k keep.
func TestPlacementIndex_AgreesWithTheWalk(t *testing.T) {
	castle, _ := openSite(t, 0, false)
	require.NoError(t, castle.ExportedPlacementIndexAgrees(0, 8, 1), "every castle placement")
	require.NoError(t, sharedKeep10k(t).ExportedPlacementIndexAgrees(64, 4, 1),
		"every keep wall and 64 seeded placements")
}

// TestOptionsFor_NoProjection: the option reads and the reach read project no
// atlas, on the 10k keep — every placed prop's options asked, every member's.
func TestOptionsFor_NoProjection(t *testing.T) {
	enc := sharedKeep10k(t)
	full, err := enc.Atlas()
	require.NoError(t, err)

	before := enc.ExportedAtlasProjections()
	_, err = enc.OptionsFor(keepPlayers[0], keepPlayers[1])
	require.NoError(t, err)
	for _, p := range full.Placed {
		_, err := enc.PropOptionsFor(keepPlayers[0], p.ID)
		if err != nil {
			require.ErrorIs(t, err, encounter.ErrNoProp)
		}
	}
	_, _, err = enc.WithinReach(keepPlayers[0], speakReach)
	require.NoError(t, err)
	require.Equal(t, before, enc.ExportedAtlasProjections(), "no atlas is projected to answer them")
}

// Ring scene members and props.
const (
	ringHero   = encounter.MemberID("hero")
	ringNear   = encounter.MemberID("near")   // adjacent, in view
	ringEdge   = encounter.MemberID("edge")   // three hexes off, in view
	ringFar    = encounter.MemberID("far")    // four hexes off, in view
	ringBehind = encounter.MemberID("behind") // within reach, behind the wall
	ringLong   = encounter.PropID("long")     // a table whose one end is within reach
	ringLedger = encounter.PropID("ledger")   // within reach, behind the wall, known
	ringStool  = encounter.PropID("stool")    // four hexes off, in view
)

// ringScene is an open hall split by a wall between columns 11 and 12 over
// rows 4–17. The hero starts east of it, beside the ledger, and walks round
// the wall's south end to [10,10]: the ledger, seen on the way, is now behind
// the wall three hexes off.
func ringScene(t *testing.T) *encounter.Encounter {
	t.Helper()
	hall := rectRegion("hall", 0, 0, 24, 24)
	players := map[encounter.MemberID]spatial.Position{
		ringHero: {X: 15, Y: 10}, ringNear: {X: 9, Y: 10}, ringEdge: {X: 10, Y: 13},
		ringFar: {X: 10, Y: 14}, ringBehind: {X: 12, Y: 9},
	}
	var members []encounter.MemberInput
	for _, id := range []encounter.MemberID{ringHero, ringNear, ringEdge, ringFar, ringBehind} {
		members = append(members, encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: players[id]})
	}
	box := func(side, depth float64, cell spatial.Position) spatial.FootprintPlacement {
		return spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: side, D: depth}}, Origin: centreOf(cell)}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{hall},
			Walls:   seamWallRows(11, 4, 14),
			Placed: []encounter.PlacedPropInput{
				{ID: ringLong, Placement: box(5, 15, cellAt(6, 10))},
				{ID: ringLedger, Placement: box(1, 1, cellAt(12, 11))},
				{ID: ringStool, Placement: box(1, 1, cellAt(10, 14))},
			},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: 24},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)

	to := cellAt(10, 10)
	route, err := enc.Route(encounter.RouteInput{Mover: ringHero, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(t, err)
	require.Empty(t, route.StoppedBy, "the hero has a way round the wall")
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != to {
		path = append(path, to) // a walk toward a cell stops beside it
	}
	for i, cell := range path {
		_, err := enc.Step(&encounter.StepInput{Member: ringHero, To: cell, EndWalk: i == len(path)-1})
		require.NoError(t, err)
	}
	require.Equal(t, to, positionOfMember(t, enc, ringHero))

	return enc
}

// TestWithinReach_Ring: what stands within reach and is seen now.
func TestWithinReach_Ring(t *testing.T) {
	enc := ringScene(t)
	at := positionOfMember(t, enc, ringHero)
	dist := func(id encounter.MemberID) float64 { return enc.Distance(at, positionOfMember(t, enc, id)) }

	// The scene's preconditions, measured rather than trusted.
	require.InDelta(t, 1, dist(ringNear), 0)
	require.InDelta(t, 3, dist(ringEdge), 0)
	require.InDelta(t, 4, dist(ringFar), 0)
	require.LessOrEqual(t, dist(ringBehind), 3.0)
	require.InDelta(t, 3, enc.Distance(at, cellAt(7, 10)), 0, "the table's near end is three hexes off")
	require.InDelta(t, 3, enc.Distance(at, cellAt(12, 11)), 0, "the ledger is three hexes off")
	require.InDelta(t, 4, enc.Distance(at, cellAt(10, 14)), 0, "the stool is four hexes off")
	atlas, err := enc.AtlasFor(ringHero)
	require.NoError(t, err)
	require.Contains(t, atlas.Cells, cellAt(12, 11), "the hero saw the ledger's cell on the way round")
	require.Len(t, enc.ExportedCellsWithin(at, 3), 37, "a ring of three is 37 cells")
	require.Len(t, enc.ExportedCellsWithin(at, 1), 7)

	members, props, err := enc.WithinReach(ringHero, speakReach)
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{ringEdge, ringNear}, members,
		"the creatures within three in view; the one behind the wall is not sighted and is absent, the far one is out of reach")
	require.Equal(t, []encounter.PropID{ringLong}, props,
		"the table counts by its one cell within reach; the ledger behind the wall is known but not seen; the stool is four off")

	members, props, err = enc.WithinReach(ringHero, 1)
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{ringNear}, members, "adjacent only at reach 1")
	require.Empty(t, props)

	_, _, err = enc.WithinReach("nobody", speakReach)
	require.ErrorIs(t, err, encounter.ErrNotMember)
}

// Speak scene members and props.
const (
	spkHero   = encounter.MemberID("hero")
	spkThree  = encounter.MemberID("three")
	spkFour   = encounter.MemberID("four")
	spkAltar  = encounter.PropID("altar")
	spkSecret = encounter.PropID("secret")
)

// TestSpeak_OutOfRange: the door measures reach after it has decided the
// target is shown and witnesses the speaker, so a line out of reach is
// refused ErrOutOfRange and a hidden thing far off still names nothing.
func TestSpeak_OutOfRange(t *testing.T) {
	hail := encounter.Option{ID: "hail", Say: "Hail.", Check: []encounter.CheckApproach{{Ability: "persuasion", DC: 10}}}
	members := []encounter.MemberInput{
		{ID: spkHero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: spkThree, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 2}, Options: []encounter.Option{hail}},
		{ID: spkFour, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}, Options: []encounter.Option{hail}},
	}
	box := func(cell spatial.Position) spatial.FootprintPlacement {
		return spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 1}}, Origin: centreOf(cell)}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 5)},
			Placed: []encounter.PlacedPropInput{
				{ID: spkAltar, Placement: box(cellAt(12, 2)), Options: []encounter.Option{hail}},
				{ID: spkSecret, Placement: box(cellAt(12, 3)), Options: []encounter.Option{hail}},
			},
			Concealments: []encounter.ConcealmentInput{{
				ID: "the-secret", Checks: []encounter.CheckApproach{{Ability: "perception", DC: 30}}, Props: []encounter.PropID{spkSecret},
			}},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: 24},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)
	at := positionOfMember(t, enc, spkHero)
	require.InDelta(t, 3, enc.Distance(at, positionOfMember(t, enc, spkThree)), 0)
	require.InDelta(t, 4, enc.Distance(at, positionOfMember(t, enc, spkFour)), 0)
	require.InDelta(t, 10, enc.Distance(at, cellAt(12, 2)), 0)

	speak := func(target encounter.MemberID, prop encounter.PropID) error {
		_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
			Range: speakReach, Actor: spkHero, Target: target, Prop: prop, OptionID: "hail", Skill: "persuasion",
			Beaten: true, DC: 10, Total: 15, Roller: rollsLowest{},
		})
		return err
	}
	require.ErrorIs(t, speak(spkFour, ""), encounter.ErrOutOfRange, "four hexes off is out of reach")
	require.NoError(t, speak(spkThree, ""), "three hexes off is within it")
	require.ErrorIs(t, speak("", spkAltar), encounter.ErrOutOfRange, "the altar ten hexes off is shown and out of reach")
	err = speak("", spkSecret)
	require.ErrorIs(t, err, encounter.ErrNoProp, "a hidden prop ten hexes off names nothing")
	require.NotErrorIs(t, err, encounter.ErrOutOfRange, "and never says it is out of range")
}

// openHall opens a 24 x 24 open hall with these members, placed props and
// cell props, the hero's discovery checks answered by resolver.
func openHall(
	t *testing.T, members []encounter.MemberInput, placed []encounter.PlacedPropInput,
	props []encounter.PropInput, concealments []encounter.ConcealmentInput, resolver encounter.CheckResolver,
) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:       openAir(),
			Regions:      []encounter.RegionInput{rectRegion("hall", 0, 0, 24, 24)},
			Placed:       placed,
			Props:        props,
			Concealments: concealments,
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: 24},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: resolver,
			Witness:       nobodyPerceives{},
			Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)

	return enc
}

// walkMember walks a member to a cell along the pathfinder's route.
func walkMember(t *testing.T, enc *encounter.Encounter, who encounter.MemberID, to spatial.Position) {
	t.Helper()
	route, err := enc.Route(encounter.RouteInput{Mover: who, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(t, err)
	require.Empty(t, route.StoppedBy, "%s has a way to %v", who, to)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != to {
		path = append(path, to) // a walk toward a cell stops beside it
	}
	for i, cell := range path {
		_, err := enc.Step(&encounter.StepInput{Member: who, To: cell, EndWalk: i == len(path)-1})
		require.NoError(t, err)
	}
}

// TestWithinReach_MovedPropsFromTheirRecord: a prop that can move is offered
// where its record says it stands NOW, never at the cell it was authored on.
// Two porters take four holdable things — a placed footprint and a cell prop
// authored within the hero's reach, and one of each authored far off — and
// leave the run: the near pair is dropped far away, the far pair beside the
// hero. A reserved prop authored beside the hero, not arrived, is never
// offered.
func TestWithinReach_MovedPropsFromTheirRecord(t *testing.T) {
	const (
		hero    = encounter.MemberID("hero")
		porterA = encounter.MemberID("porter-a")
		porterB = encounter.MemberID("porter-b")
		candle  = encounter.PropID("candle") // placed, authored within reach
		key     = encounter.PropID("key")    // cell prop, authored within reach
		lamp    = encounter.PropID("lamp")   // placed, authored far off
		coin    = encounter.PropID("coin")   // cell prop, authored far off
		late    = encounter.PropID("late")   // placed, reserved, authored within reach
	)
	box := func(cell spatial.Position) spatial.FootprintPlacement {
		return spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 1}}, Origin: centreOf(cell)}
	}
	enc := openHall(t,
		[]encounter.MemberInput{
			{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 10}},
			{ID: porterA, Kind: encounter.KindPlayer, Position: spatial.Position{X: 9, Y: 11}},
			{ID: porterB, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 18}},
		},
		[]encounter.PlacedPropInput{
			{ID: candle, Placement: box(cellAt(9, 10)), Holdable: true},
			{ID: lamp, Placement: box(cellAt(10, 19)), Holdable: true},
			{ID: late, Placement: box(cellAt(11, 10)), Arrives: encounter.TriggerRound{Round: 99}},
		},
		[]encounter.PropInput{
			holdableProp(key, "test:props:key", spatial.Position{X: 10, Y: 11}),
			holdableProp(coin, "test:props:coin", spatial.Position{X: 11, Y: 19}),
		},
		nil, findsNothing{},
	)
	at := positionOfMember(t, enc, hero)
	for _, c := range []struct {
		cell spatial.Position
		near bool
	}{{cellAt(9, 10), true}, {cellAt(10, 11), true}, {cellAt(11, 10), true}, {cellAt(10, 19), false}, {cellAt(11, 19), false}} {
		require.Equal(t, c.near, enc.Distance(at, c.cell) <= speakReach, "precondition: %v within reach is %v", c.cell, c.near)
	}
	reach := func() []encounter.PropID {
		t.Helper()
		_, props, err := enc.WithinReach(hero, speakReach)
		require.NoError(t, err)
		return props
	}
	require.Equal(t, []encounter.PropID{candle, key}, reach(),
		"the near pair stands where it was authored; the reserved one has not arrived")

	take := func(who encounter.MemberID, what ...encounter.PropID) {
		t.Helper()
		for _, id := range what {
			_, err := enc.Hold(&encounter.HoldInput{Member: who, Target: id, Range: 2})
			require.NoError(t, err, "%s takes %s", who, id)
		}
	}
	take(porterA, candle, key)
	require.Empty(t, reach(), "a thing in somebody's hands stands nowhere")

	// Porter A leaves far off with the near pair; porter B beside the hero
	// with the far pair. Leaving drops what one holds where one stands.
	far, beside := cellAt(10, 16), cellAt(12, 10)
	require.Greater(t, enc.Distance(at, far), float64(speakReach))
	require.LessOrEqual(t, enc.Distance(at, beside), float64(speakReach))
	walkMember(t, enc, porterA, far)
	_, err := enc.Exit(&encounter.ExitInput{Member: porterA})
	require.NoError(t, err)
	require.Empty(t, reach(), "the near pair, dropped far off, is not offered at the cells it was authored on")

	take(porterB, lamp, coin)
	walkMember(t, enc, porterB, beside)
	_, err = enc.Exit(&encounter.ExitInput{Member: porterB})
	require.NoError(t, err)
	require.Equal(t, []encounter.PropID{coin, lamp}, reach(),
		"the far pair, dropped beside the hero, is offered where it lies now")
}

// toggledFinder fails every discovery check until finds is set.
type toggledFinder struct{ finds bool }

// ResolveCheck answers finds.
func (f *toggledFinder) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: f.finds, Applied: in.Approaches[0]}, nil
}

// ResolveDiscoveryCheck answers finds.
func (f *toggledFinder) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return f.ResolveCheck(in)
}

// TestWithinReach_SecretUntilFound: proximity reveals nothing a map does not
// show. A prop an unfound concealment hides stands on floor the hero sees,
// two hexes off, and is not offered; once the hero finds it, it is.
func TestWithinReach_SecretUntilFound(t *testing.T) {
	const (
		hero  = encounter.MemberID("hero")
		relic = encounter.PropID("relic")
	)
	finder := &toggledFinder{}
	enc := openHall(t,
		[]encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 10}}},
		[]encounter.PlacedPropInput{{
			ID: relic, Placement: spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 1}}, Origin: centreOf(cellAt(12, 10)),
			},
		}},
		nil,
		[]encounter.ConcealmentInput{{
			ID: "the-relic", Checks: []encounter.CheckApproach{{Ability: "perception", DC: 15}}, Props: []encounter.PropID{relic},
		}},
		finder,
	)
	at := positionOfMember(t, enc, hero)
	require.InDelta(t, 2, enc.Distance(at, cellAt(12, 10)), 0)
	visible, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: hero})
	require.NoError(t, err)
	require.Contains(t, visible.Cells, cellAt(12, 10), "precondition: the hero sees the floor the relic stands on")

	_, props, err := enc.WithinReach(hero, speakReach)
	require.NoError(t, err)
	require.Empty(t, props, "an unfound secret within reach is not offered")

	// Walk up beside it with eyes that find: the automatic check fires.
	finder.finds = true
	walkMember(t, enc, hero, cellAt(11, 10))
	_, props, err = enc.WithinReach(hero, speakReach)
	require.NoError(t, err)
	require.Equal(t, []encounter.PropID{relic}, props, "found, it is offered")
}

// TestWithinReach_OwnCellUnderASecret: the seen-now test admits the cell a
// member stands on unconditionally, and a member can stand on floor a
// concealment hides. The offer and the Speak door still agree there: standing
// on the secret's floor shows it, so the urn on that cell is offered and the
// door admits the line said to it.
func TestWithinReach_OwnCellUnderASecret(t *testing.T) {
	const (
		hero = encounter.MemberID("hero")
		urn  = encounter.PropID("urn")
	)
	hail := encounter.Option{ID: "hail", Say: "Hail.", Check: []encounter.CheckApproach{{Ability: "persuasion", DC: 10}}}
	enc := openHall(t,
		[]encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 10}}},
		[]encounter.PlacedPropInput{{
			ID: urn, Options: []encounter.Option{hail}, Placement: spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 1}}, Origin: centreOf(cellAt(10, 10)),
			},
		}},
		nil,
		[]encounter.ConcealmentInput{{
			ID: "the-crypt", Checks: []encounter.CheckApproach{{Ability: "perception", DC: 30}},
			Cells: []spatial.Position{{X: 10, Y: 10}},
		}},
		findsNothing{},
	)
	_, props, err := enc.WithinReach(hero, speakReach)
	require.NoError(t, err)
	require.Equal(t, []encounter.PropID{urn}, props, "the urn on the hero's own cell is offered")
	_, err = enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Prop: urn, OptionID: "hail", Skill: "persuasion",
		Beaten: true, DC: 10, Total: 15, Roller: rollsLowest{},
	})
	require.NoError(t, err, "and the door admits the line the offer listed")
}
