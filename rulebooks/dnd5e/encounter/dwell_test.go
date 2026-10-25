// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// dwell_test.go pins the dwell's reaction-window rule (rpg-project#624,
// design R18): a walk to a waypoint with a wait that stops for a reaction
// window and resumes still arrives with the whole wait ahead of it — the
// resumed turn is the same turn, and spends none of it.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// waypointWalkDriver walks one member two cells east on its first turn, to a
// waypoint whose pause states a wait of two and a south facing, and passes
// after.
type waypointWalkDriver struct {
	member encounter.MemberID
	used   bool
}

func (d *waypointWalkDriver) Act(view encounter.MonsterView) (encounter.Decision, error) {
	if view.Self != d.member || d.used {
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	}
	d.used = true
	first := spatial.Position{X: view.Position.X + 1, Y: view.Position.Y}
	waypoint := spatial.Position{X: view.Position.X + 2, Y: view.Position.Y}
	south := 180.0

	return encounter.Decision{Intent: encounter.Move{
		Path:  []spatial.Position{first, waypoint},
		Pause: &encounter.WaypointPause{Cell: waypoint, Facing: &south, Wait: 2},
	}}, nil
}

// TestAReactionWindowSpendsNoDwell: the goblin's walk to the waypoint stops
// for alice's window on its first step; resumed, it ends on the waypoint
// facing the stated south with both of the wait's rounds still ahead.
func TestAReactionWindowSpendsNoDwell(t *testing.T) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := facingFight(t, members, &waypointWalkDriver{member: goblin}, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
	start := memberOf(t, enc, goblin).Position

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.True(t, enc.Paused(), "precondition: the walk stopped for a window")

	_, err = enc.Resume(context.Background())
	require.NoError(t, err)
	require.False(t, enc.Paused())

	waypoint := spatial.Position{X: start.X + 2, Y: start.Y}
	require.Equal(t, waypoint, memberOf(t, enc, goblin).Position, "the resumed walk ended on the waypoint")
	require.InDelta(t, 180, memberOf(t, enc, goblin).Facing, 1e-9, "standing there it faces the stated south")
	var dwell *encounter.DwellData
	for _, m := range enc.ToData().Members {
		if m.ID == goblin {
			dwell = m.Dwell
		}
	}
	require.NotNil(t, dwell, "the walk that ended on the waypoint began a dwell")
	require.Equal(t, 2, dwell.Rounds, "the window and the resumed turn spent none of the wait")
}

// TestADwellReloadedWhilePausedSpendsNone is the reaction-window rule across
// the storage boundary: the walk to the waiting waypoint stops for alice's
// window, the run is saved and loaded while paused, and the loaded run's
// resumed walk still arrives with the whole wait ahead. The pause's own
// record of the waypoint is what carried it; a stored pause facing or wait
// this build would never write is refused at load.
func TestADwellReloadedWhilePausedSpendsNone(t *testing.T) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := facingFight(t, members, &waypointWalkDriver{member: goblin}, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
	start := memberOf(t, enc, goblin).Position

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.True(t, enc.Paused(), "precondition: the walk stopped for a window")

	data := enc.ToData()
	require.NotNil(t, data.Pause.Turn.FacePause, "the paused walk keeps the waypoint it walks to")
	require.Equal(t, 2, data.Pause.Turn.FacePause.Wait)

	loaded := reloadDwellFight(t, data, mover)
	_, err = loaded.Resume(context.Background())
	require.NoError(t, err)

	waypoint := spatial.Position{X: start.X + 2, Y: start.Y}
	require.Equal(t, waypoint, memberOf(t, loaded, goblin).Position, "the loaded run's walk ended on the waypoint")
	require.InDelta(t, 180, memberOf(t, loaded, goblin).Facing, 1e-9, "standing there it faces the stated south")
	require.Equal(t, 2, dwellIn(t, loaded, goblin).Rounds, "the window and the reload spent none of the wait")

	for name, spoil := range map[string]func(d *encounter.EncounterData){
		"a pause facing of 360": func(d *encounter.EncounterData) {
			full := 360.0
			d.Pause.Turn.FacePause.Facing = &full
		},
		"a negative pause wait": func(d *encounter.EncounterData) { d.Pause.Turn.FacePause.Wait = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			spoilt := enc.ToData()
			spoil(&spoilt)
			_, lerr := encounter.LoadEncounter(dwellLoadInput(spoilt, mover))
			require.ErrorIs(t, lerr, encounter.ErrInvalidData)
		})
	}
}

// TestANegativeDwellIsRefusedAtLoad: a stored dwell with negative rounds left
// is one no build writes.
func TestANegativeDwellIsRefusedAtLoad(t *testing.T) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}
	enc := facingFight(t, members, &waypointWalkDriver{member: goblin}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	data := enc.ToData()
	stood := false
	for i := range data.Members {
		if data.Members[i].ID == goblin {
			require.NotNil(t, data.Members[i].Dwell, "precondition: the goblin stands at its waypoint")
			data.Members[i].Dwell.Rounds = -1
			stood = true
		}
	}
	require.True(t, stood)
	_, err = encounter.LoadEncounter(dwellLoadInput(data, quietMover{}))
	require.ErrorIs(t, err, encounter.ErrInvalidData)
}

// dwellLoadInput is facingFight's capabilities over a stored run.
func dwellLoadInput(data encounter.EncounterData, mover encounter.Mover) *encounter.LoadEncounterInput {
	return &encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        adjacentSkeletonSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        &waypointWalkDriver{member: goblin, used: true},
			Actors: encounter.Actors{
				Striker:   &scriptedStriker{kind: encounter.OutcomeMissed},
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
	}
}

// reloadDwellFight loads a stored run with facingFight's capabilities.
func reloadDwellFight(t *testing.T, data encounter.EncounterData, mover encounter.Mover) *encounter.Encounter {
	t.Helper()
	loaded, err := encounter.LoadEncounter(dwellLoadInput(data, mover))
	require.NoError(t, err)

	return loaded
}

// dwellIn is a member's stored dwell; it must have one.
func dwellIn(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) *encounter.DwellData {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID == id {
			require.NotNil(t, m.Dwell, "%s stands at a waypoint", id)
			return m.Dwell
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return nil
}

// dwellOrNil is a member's stored dwell, or nil.
func dwellOrNil(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) *encounter.DwellData {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID == id {
			return m.Dwell
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return nil
}

// TestARowAboveThePatrolInterruptsTheDwellAndLeavingEndsIt: a guard paces a
// wide triangle — a loop longer than a round's walk — whose second waypoint
// states a wait of nine. While he stands there
// a clang rings and his `heard` row — above the patrol row — walks him toward
// it: the dwell is only the patrol's own leg. Found off the waypoint at his
// next turn, the dwell ends with rounds still left, and the patrol takes him
// on rather than holding him where the sound left him.
func TestARowAboveThePatrolInterruptsTheDwellAndLeavingEndsIt(t *testing.T) {
	a, b, c := cellAt(4, 2), cellAt(8, 2), cellAt(6, 8)
	south := 180.0
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		tabled(noiseKeen, 4, 2, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{Kind: behavior.Name{Literal: encounter.SoundClang}, Within: 2}}},
				Toward: &encounter.Selector{Word: encounter.SelectorSound}},
			{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyNone}},
				Toward: &encounter.Selector{Path: []spatial.Position{a, b, c},
					Facings: []*float64{nil, &south, nil}, Waits: []int{0, 9, 0}}},
		}}),
	)

	for i := 0; i < 6 && dwellOrNil(t, enc, noiseKeen) == nil; i++ {
		_, err := aRound(enc)
		require.NoError(t, err)
	}
	require.Equal(t, b, whereIs(t, enc, noiseKeen), "precondition: he walked to the waiting waypoint")
	require.Equal(t, 9, dwellIn(t, enc, noiseKeen).Rounds, "precondition: and stands there")
	_, err := aRound(enc)
	require.NoError(t, err)
	require.Equal(t, b, whereIs(t, enc, noiseKeen), "precondition: a round of standing")
	require.InDelta(t, 180, memberOf(t, enc, noiseKeen).Facing, 1e-9)

	aliceStrikesTheThug(t, enc)
	clang := whereIs(t, enc, noiseAlice)
	left := false
	for i := 0; i < 8 && !left; i++ {
		before := dwellIn(t, enc, noiseKeen)
		_, err = aRound(enc)
		require.NoError(t, err)
		if at := whereIs(t, enc, noiseKeen); at != b {
			left = true
			require.Greater(t, before.Rounds, 1, "he left with the dwell's rounds still ahead of him")
			require.Less(t, enc.Distance(at, clang), enc.Distance(b, clang), "toward the clang, not along his route")
		}
	}
	require.True(t, left, "the heard row above the patrol walked him off the waypoint, mid-dwell")

	ended := false
	for i := 0; i < 3 && !ended; i++ {
		_, err = aRound(enc)
		require.NoError(t, err)
		ended = dwellOrNil(t, enc, noiseKeen) == nil
	}
	require.True(t, ended, "found off the waypoint, his dwell ended with rounds still left")

	// The sound goes stale; with no dwell to hold him where it left him, the
	// patrol takes him back to his route, and arriving on the waiting
	// waypoint again begins a fresh dwell.
	back := false
	for i := 0; i < 12 && !back; i++ {
		_, err = aRound(enc)
		require.NoError(t, err)
		d := dwellOrNil(t, enc, noiseKeen)
		back = d != nil && whereIs(t, enc, noiseKeen) == b && d.Rounds == 9
	}
	require.True(t, back, "the patrol walked him back to the waypoint, where a fresh dwell began")
}
