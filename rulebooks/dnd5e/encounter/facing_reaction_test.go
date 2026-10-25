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

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
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
			{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{Kind: behavior.Name{Literal: encounter.SoundClang}, Within: 6}}},
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

// TestAHeardRoutedNamingAMemberIsRefused: [encounter.Routed.Heard] says the
// anchor is a sound's cell, so a Heard Routed that names a member instead is
// a malformed intent — the turn ends unwalked, as Pass would.
func TestAHeardRoutedNamingAMemberIsRefused(t *testing.T) {
	mover := &recordingMover{}
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 0}},
	}, &oneMemberDriver{member: goblin, intent: encounter.Routed{
		Policy: encounter.MoveToward, Anchor: alice, Heard: true, Cause: facingRoutedCause,
	}}, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
	start := memberOf(t, enc, goblin).Position

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.Empty(t, mover.calls, "the refused walk announced no step")
	require.Equal(t, start, memberOf(t, enc, goblin).Position, "the goblin did not walk")
	require.Equal(t, encounter.UnstatedFacing, memberOf(t, enc, goblin).Facing, "nor turn")
}

// TestAPausedWalkTowardASoundResumesFacingIt: a walk toward a heard sound
// stops for a reaction window on its first step, and the resumed walk still
// ends facing the sound's cell. The clang rang on alice's cell, so the walk
// stops beside it; bob stands where an open approach would end, so the last
// step (210) does not already point at the clang (270).
func TestAPausedWalkTowardASoundResumesFacingIt(t *testing.T) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 0}},
		{ID: bob, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}, BlocksMovement: true},
	}
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	driver := &soundWalkDriver{member: goblin, sound: alice}
	enc := facingFight(t, members, driver, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
	clang := memberOf(t, enc, alice).Position

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.True(t, enc.Paused(), "precondition: the walk stopped for a window")

	_, err = enc.Resume(context.Background())
	require.NoError(t, err)
	require.False(t, enc.Paused())

	end := memberOf(t, enc, goblin).Position
	last := mover.calls[len(mover.calls)-1]
	toClang := stepBearing(t, end, clang)
	require.NotEqual(t, stepBearing(t, last.From, last.To), toClang,
		"precondition: the last step does not already point at the clang")
	require.InDelta(t, toClang, memberOf(t, enc, goblin).Facing, 1e-9,
		"the resumed walk ended facing where the sound rang")
}

// soundWalkDriver sends one member toward a heard sound that rang on another
// member's cell, read off the view, on its first turn, and passes after.
type soundWalkDriver struct {
	member, sound encounter.MemberID
	used          bool
}

func (d *soundWalkDriver) Act(view encounter.MonsterView) (encounter.Decision, error) {
	if view.Self != d.member || d.used {
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	}
	d.used = true
	for _, s := range view.Seen {
		if s.ID == d.sound {
			cell := s.Position

			return encounter.Decision{Intent: encounter.Routed{
				Policy: encounter.MoveToward, AnchorAt: &cell, Heard: true, Cause: facingRoutedCause,
			}}, nil
		}
	}

	return encounter.Decision{Intent: encounter.Pass{}}, nil
}

// TestAMoveNamingATargetEndsFacingIt pins the Move arm's walk-end turn
// ([encounter.Move.Target]): the goblin steps west, 270, to the cell beside
// alice, and ends facing her held cell, 210.
func TestAMoveNamingATargetEndsFacingIt(t *testing.T) {
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}},
	}, &targetedStepDriver{member: goblin, target: alice}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})
	start := memberOf(t, enc, goblin).Position

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	end := memberOf(t, enc, goblin).Position
	require.Equal(t, spatial.Position{X: start.X - 1, Y: start.Y}, end, "precondition: the goblin stepped west")
	toAlice := stepBearing(t, end, memberOf(t, enc, alice).Position)
	require.NotEqual(t, stepBearing(t, start, end), toAlice, "precondition: the step does not already point at alice")
	require.InDelta(t, toAlice, memberOf(t, enc, goblin).Facing, 1e-9, "the walk ended facing its target")
}

// targetedStepDriver walks one member one cell west, naming a target, on its
// first turn, and passes after.
type targetedStepDriver struct {
	member, target encounter.MemberID
	used           bool
}

func (d *targetedStepDriver) Act(view encounter.MonsterView) (encounter.Decision, error) {
	if view.Self != d.member || d.used {
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	}
	d.used = true
	to := spatial.Position{X: view.Position.X - 1, Y: view.Position.Y}

	return encounter.Decision{Intent: encounter.Move{Path: []spatial.Position{to}, Target: d.target}}, nil
}
