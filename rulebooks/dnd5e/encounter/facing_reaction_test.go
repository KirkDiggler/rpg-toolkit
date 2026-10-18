// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// facing_reaction_test.go pins the two facing gaps the C-facing walk found
// (rpg-toolkit#2076, sneaking design R13): an opportunity attack turns its
// striker toward the cell it HOLDS for the mover, through the same recorded
// train every attack lands by, and a walk toward a heard sound ends facing
// the cell the sound rang at.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// opportunityMover is the session's Mover in miniature: the first step the
// mover takes provokes the striker, whose reaction is recorded through
// [encounter.Encounter.RecordTrain] with the mover still on the cell it is
// leaving — exactly where the session's movement fold records one.
type opportunityMover struct {
	mover, striker encounter.MemberID
	from           *spatial.Position
}

func (o *opportunityMover) Move(_ context.Context, enc *encounter.Encounter, step encounter.MoveStep) error {
	if o.from != nil || step.Mover != o.mover {
		return nil
	}
	from := step.From
	o.from = &from
	reaction := testOpportunityAttack
	_, err := enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: o.striker, Targets: []encounter.MemberID{o.mover}, Reaction: &reaction,
	}}}})

	return err
}

// reactionFight is facingFight with the sight capability supplied, so a
// scene can blind the striker.
func reactionFight(
	t *testing.T, sight encounter.Sight, driver encounter.Driver, mover encounter.Mover,
) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
		},
		Endings:   []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         sight,
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        adjacentSkeletonSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
			Actors: encounter.Actors{
				Striker:   &scriptedStriker{kind: encounter.OutcomeMissed},
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc
}

// TestAnOpportunityAttackTurnsTheStriker: the goblin walks east, out of
// alice's reach, and provokes. Her reaction is recorded while the goblin
// still stands on the cell it is leaving — the cell she holds it at — and it
// turns her toward that cell, 90, from the unstated 180. A striker who holds
// no sighting of the mover places nobody, and does not turn.
func TestAnOpportunityAttackTurnsTheStriker(t *testing.T) {
	run := func(t *testing.T, sight encounter.Sight) (*encounter.Encounter, *opportunityMover) {
		t.Helper()
		mover := &opportunityMover{mover: goblin, striker: alice}
		driver := &lazyStepDriver{member: goblin, dx: 1}
		enc := reactionFight(t, sight, driver, mover)
		require.Equal(t, encounter.UnstatedFacing, memberOf(t, enc, alice).Facing)
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		require.NoError(t, err)
		require.NotNil(t, mover.from, "the goblin's step provoked")

		return enc, mover
	}

	t.Run("toward the held cell of the mover", func(t *testing.T) {
		enc, mover := run(t, everyoneSeesTheWholeMap{})
		here := memberOf(t, enc, alice).Position
		require.NotEqual(t, *mover.from, memberOf(t, enc, goblin).Position, "precondition: the goblin moved on")
		require.InDelta(t, stepBearing(t, here, *mover.from), memberOf(t, enc, alice).Facing, 1e-9,
			"the striker turned toward the cell it held the mover at")
	})

	t.Run("no sighting turns nobody", func(t *testing.T) {
		blind := &sightList{reach: map[encounter.MemberID]int{alice: 0}, fallback: unlimitedSight}
		enc, _ := run(t, blind)
		require.Equal(t, encounter.UnstatedFacing, memberOf(t, enc, alice).Facing,
			"a striker that holds no sighting of the mover does not turn")
	})
}

// lazyStepDriver walks one member one cell, dx columns over, on its first
// turn, and passes every other: the path is read off the view, so the scene's
// region offset never has to be known.
type lazyStepDriver struct {
	member encounter.MemberID
	dx     float64
	used   bool
}

func (d *lazyStepDriver) Act(view encounter.MonsterView) (encounter.Decision, error) {
	if view.Self != d.member || d.used {
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	}
	d.used = true
	to := spatial.Position{X: view.Position.X + d.dx, Y: view.Position.Y}

	return encounter.Decision{Intent: encounter.Move{Path: []spatial.Position{to}}}, nil
}

// TestTowardSoundEndsFacingTheClang: a guard who heard steel walks toward
// where it rang and stops beside it, facing the clang's cell — the sound is
// the walk's target — rather than along its last step.
func TestTowardSoundEndsFacingTheClang(t *testing.T) {
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		tabled(noiseKeen, 18, 1, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{Kind: encounter.SoundClang, Within: 6}}},
				Toward: &encounter.Selector{Word: encounter.SelectorSound}},
			{Weight: 1, Hold: true},
		}}),
	)
	clangAt := cellAt(2, 5)

	aliceStrikesTheThug(t, enc)
	for range 4 {
		_, err := aRound(enc)
		require.NoError(t, err)
	}

	end := whereIs(t, enc, noiseKeen)
	require.LessOrEqual(t, enc.Distance(end, clangAt), 1.0, "precondition: it stopped beside the clang")
	require.InDelta(t, stepBearing(t, end, clangAt), memberOf(t, enc, noiseKeen).Facing, 1e-9,
		"the walk toward the sound ended facing where it rang")
}
