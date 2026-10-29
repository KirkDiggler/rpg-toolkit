// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// leave_test.go is THE RUN ENDS AT THE DOOR (rpg-project#716, ruled
// 2026-10-29): "walk out the front door to end. king taken care of? no?
// failure, yes? success." and "last one out ends it."

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const (
	leaveDoor = encounter.ExitID("front-door")
	leaveKing = core.EntityID("king")
)

var (
	leaveDoorCell = spatial.Position{X: 0, Y: 1}
	besideTheDoor = spatial.Position{X: 1, Y: 1}
)

// leaveEndings is the operator's two endings at one door.
func leaveEndings() []encounter.EndingInput {
	return []encounter.EndingInput{
		{Key: "king-slain", Trigger: encounter.TriggerLeft{Exit: leaveDoor},
			If: encounter.TriggerMemberDown{Member: leaveKing}, Result: encounter.ResultSuccess},
		{Key: "king-lives", Trigger: encounter.TriggerLeft{Exit: leaveDoor}, Result: encounter.ResultFailure},
	}
}

type LeaveSuite struct {
	suite.Suite

	standing *nobodyIsInContact
}

func TestLeaveSuite(t *testing.T) {
	suite.Run(t, new(LeaveSuite))
}

// field is one room with the front door at its west end.
func (s *LeaveSuite) field() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 3)},
		Exits:   []encounter.FieldExit{{ID: leaveDoor, At: leaveDoorCell}},
	}
}

// capabilities is a host in which nobody fights and the test says who is down.
func (s *LeaveSuite) capabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: s.standing,
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

// scene seats alice on the door, bob beside it, and the king across the hall.
func (s *LeaveSuite) scene(endings []encounter.EndingInput) (*encounter.Encounter, error) {
	s.standing = &nobodyIsInContact{}

	return encounter.NewEncounter(&encounter.SetupInput{
		Field: s.field(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: leaveDoorCell},
			{ID: bob, Kind: encounter.KindPlayer, Position: besideTheDoor},
			{ID: leaveKing, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 1}},
		},
		Endings:      endings,
		Retention:    encounter.RetentionUnbounded,
		Capabilities: s.capabilities(),
	})
}

// leave has alice, then bob, walk out the front door: bob steps onto it
// once alice has gone. It returns what each departure closed.
func (s *LeaveSuite) leave(enc *encounter.Encounter) (first, last *encounter.Outcome) {
	out, err := enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)
	first = out.Closed
	_, err = enc.Step(&encounter.StepInput{Member: bob, To: leaveDoorCell, EndWalk: true})
	s.Require().NoError(err)
	out, err = enc.Exit(&encounter.ExitInput{Member: bob})
	s.Require().NoError(err)

	return first, out.Closed
}

// TestTheLastOneOutEndsIt: alice leaving by the door ends nothing while bob
// is still inside; bob leaving after her ends the run, on the ending whose
// `if` holds, with its outcome.
func (s *LeaveSuite) TestTheLastOneOutEndsIt() {
	enc, err := s.scene(leaveEndings())
	s.Require().NoError(err)
	s.standing.down = []encounter.MemberID{leaveKing}

	first, last := s.leave(enc)

	s.Nil(first, "the first one out ends nothing: bob is still playing")
	s.Require().NotNil(last, "the last one out ends it")
	s.Equal("king-slain", last.Ending)
	s.Equal(encounter.ResultSuccess, last.Result, "the king was down when they left")
	status, err := enc.Status()
	s.Require().NoError(err)
	s.Equal(encounter.ResultSuccess, status.Outcome.Result)
}

// TestAnIfThatDoesNotHoldFallsThroughToTheNextEnding: the king is standing
// when the party leaves, so the first ending's `if` fails and the plain
// `left` after it fires.
func (s *LeaveSuite) TestAnIfThatDoesNotHoldFallsThroughToTheNextEnding() {
	enc, err := s.scene(leaveEndings())
	s.Require().NoError(err)

	_, last := s.leave(enc)

	s.Require().NotNil(last)
	s.Equal("king-lives", last.Ending)
	s.Equal(encounter.ResultFailure, last.Result)
}

// TestADownMemberDoesNotKeepTheRunOpen: bob is down, so alice is the last
// one standing, and her leaving ends the run.
func (s *LeaveSuite) TestADownMemberDoesNotKeepTheRunOpen() {
	enc, err := s.scene(leaveEndings())
	s.Require().NoError(err)
	s.standing.down = []encounter.MemberID{bob}

	out, err := enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)

	s.Require().NotNil(out.Closed, "a body left behind is not somebody still playing")
	s.Equal("king-lives", out.Closed.Ending)
}

// TestLeavingByAnotherWayEndsNothing: the last one out who leaves from any
// cell but the door's leaves the run open for the monsters' sake, as today.
func (s *LeaveSuite) TestLeavingByAnotherWayEndsNothing() {
	enc, err := s.scene(leaveEndings())
	s.Require().NoError(err)

	_, err = enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)
	out, err := enc.Exit(&encounter.ExitInput{Member: bob})
	s.Require().NoError(err)

	s.Nil(out.Closed, "bob left from beside the door, not through it")
}

// TestTheDepartureIsToldBeforeTheEnding: the record reads "bob left by the
// front door" and then "ended", with the outcome on the ended beat.
func (s *LeaveSuite) TestTheDepartureIsToldBeforeTheEnding() {
	enc, err := s.scene(leaveEndings())
	s.Require().NoError(err)
	s.standing.down = []encounter.MemberID{leaveKing}
	s.leave(enc)

	story, err := enc.Story(&encounter.StoryInput{Audience: bob})
	s.Require().NoError(err)
	exited, ended := -1, -1
	var endedBody struct{ Ending, Outcome string }
	for i, entry := range story {
		var b struct{ Beat, Member, Exit, Ending, Outcome string }
		s.Require().NoError(json.Unmarshal(entry.Payload, &b))
		switch {
		case b.Beat == encounter.BeatExited && b.Member == string(bob):
			exited = i
			s.Equal(string(leaveDoor), b.Exit)
		case b.Beat == encounter.BeatEnded:
			ended = i
			endedBody.Ending, endedBody.Outcome = b.Ending, b.Outcome
		}
	}
	s.Require().GreaterOrEqual(exited, 0, "bob's departure is told")
	s.Greater(ended, exited, "and the ending after it")
	s.Equal("king-slain", endedBody.Ending)
	s.Equal("success", endedBody.Outcome, "the ended beat says what it meant")
}

// TestTheLeaveEndingsSurviveTheBlob: a run saved and loaded back keeps the
// `if` and the outcome, ends on the right one, and a closed run read back
// says the same result.
func (s *LeaveSuite) TestTheLeaveEndingsSurviveTheBlob() {
	enc, err := s.scene(leaveEndings())
	s.Require().NoError(err)
	reload := func(e *encounter.Encounter) *encounter.Encounter {
		blob, merr := json.Marshal(e.ToData())
		s.Require().NoError(merr)
		var data encounter.EncounterData
		s.Require().NoError(json.Unmarshal(blob, &data))
		back, lerr := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities()})
		s.Require().NoError(lerr)
		return back
	}
	enc = reload(enc)
	s.standing.down = []encounter.MemberID{leaveKing}

	_, last := s.leave(enc)
	s.Require().NotNil(last)
	s.Equal(encounter.ResultSuccess, last.Result)

	status, err := reload(enc).Status()
	s.Require().NoError(err)
	s.Equal("king-slain", status.Outcome.Ending)
	s.Equal(encounter.ResultSuccess, status.Outcome.Result, "the result is the declared ending's, read back")
}

// TestTheLeaveEndingIsRefusedWhereItCannotFire: every refusal at the door,
// each ErrNoEnding (or ErrNoFaction for an until).
func (s *LeaveSuite) TestTheLeaveEndingIsRefusedWhereItCannotFire() {
	for _, tc := range []struct {
		name    string
		endings []encounter.EndingInput
		says    string
	}{
		{name: "an exit nobody declared", endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerLeft{Exit: "back-door"}, Result: encounter.ResultFailure}},
			says: `waits for the party to leave by "back-door", which this field does not declare`},
		{name: "no outcome", endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerLeft{Exit: leaveDoor}}},
			says: "does not say whether that is success or failure"},
		{name: "an outcome that is neither", endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerLeft{Exit: leaveDoor}, Result: "draw"}},
			says: `says its outcome is "draw"`},
		{name: "an if off a leave ending", endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerRound{Round: 3}, If: encounter.TriggerMemberDown{Member: leaveKing}}},
			says: "only an ending that waits for the party to leave has one"},
		{name: "an if that waits instead of asks", endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerLeft{Exit: leaveDoor}, If: encounter.TriggerRound{Round: 2},
				Result: encounter.ResultSuccess}},
			says: "an if asks whether somebody is down"},
	} {
		s.Run(tc.name, func() {
			_, err := s.scene(tc.endings)
			s.Require().ErrorIs(err, encounter.ErrNoEnding)
			s.Contains(err.Error(), tc.says)
		})
	}

	s.Run("on a disposition's until", func() {
		s.standing = &nobodyIsInContact{}
		f := s.field()
		f.Factions = []encounter.FactionInput{{ID: "court"}}
		f.Dispositions = []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"court", encounter.FactionParty}, Stance: encounter.StanceNeutral,
			Until: encounter.TriggerLeft{Exit: leaveDoor},
		}}
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Field:        f,
			Members:      []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: leaveDoorCell}},
			Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
			Capabilities: s.capabilities(),
		})
		s.Require().Error(err)
		s.True(errors.Is(err, encounter.ErrNoFaction), "%v", err)
		s.Contains(err.Error(), "only an ending waits for that")
	})
}
