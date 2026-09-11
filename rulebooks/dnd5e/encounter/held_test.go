// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// held_test.go is a DIRECTED walk stopping to ask, which is the half
// rpg-project#316 rung 3 left for the day a directive provoked. A pushed
// creature never opened a window — a forced step suppresses the swing — so
// Direct refused a pause outright. Dissonant Whispers sends a creature
// fleeing and it IS struck on the way out, so the remainder needs somewhere
// to live, and it lives beside the held turn.
//
// The scene is the pause suite's: a ten-by-ten room, alice at [2,2] and the
// goblin at [6,2], with the goblin routed AWAY along row 2. Nothing here
// drives a turn — a directive is nobody's turn — so the driver passes.
type HeldTestSuite struct {
	suite.Suite
}

func TestHeldSuite(t *testing.T) {
	suite.Run(t, new(HeldTestSuite))
}

// whispersRef is the cause every held directive in this file names.
var whispersRef = core.Ref{Module: "dnd5e", Type: "spells", ID: "dissonant-whispers"}

// fleeRoute is the three cells the goblin is sent down, away from alice.
func fleeRoute() []spatial.Position {
	return []spatial.Position{cellAt(7, 2), cellAt(8, 2), cellAt(9, 2)}
}

func (s *HeldTestSuite) scene(mover encounter.Mover, standing encounter.Standing) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: mover, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{
					{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
				},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// flee sends the goblin down fleeRoute as a provoking directive.
func (s *HeldTestSuite) flee(enc *encounter.Encounter) (encounter.DirectOutput, error) {
	return enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Cause: whispersRef, Route: fleeRoute(), Provokes: true,
	})
}

func (s *HeldTestSuite) positionOf(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", "%s is not on the roster", id)
	return spatial.Position{}
}

// beatsOf decodes every beat one member can hear, in story order.
func (s *HeldTestSuite) beatsOf(enc *encounter.Encounter, audience encounter.MemberID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

func (s *HeldTestSuite) beatsNamed(enc *encounter.Encounter, audience encounter.MemberID, kind string) []map[string]any {
	var out []map[string]any
	for _, beat := range s.beatsOf(enc, audience) {
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}
	return out
}

// loadInput is the reload every test here uses, with the data swapped in.
func loadInput(data encounter.EncounterData, mover encounter.Mover, standing encounter.Standing,
) *encounter.LoadEncounterInput {
	return &encounter.LoadEncounterInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: mover, Announcer: quietAnnouncer{},
		Data: data,
	}
}

// TestAProvokingDirectiveThatPausesIsHeldNotRefused is the refusal Direct's
// own doc promised would have to be answered "the day a directive provokes".
//
// It is not an error, nothing is lost, and the table is told it is waiting —
// the same three things a paused turn reports, through a directive's own
// output.
func (s *HeldTestSuite) TestAProvokingDirectiveThatPausesIsHeldNotRefused() {
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.scene(mover, &downList{})

	out, err := s.flee(enc)
	s.Require().NoError(err, "a window opening is news, not a malfunction")
	s.True(out.Paused, "the caller is told the walk is held")
	s.Equal(0, out.Moved, "the first cell was announced and not taken")

	s.True(enc.Paused())
	s.True(enc.HeldDirective(), "and it is a directive, not a turn")
	s.Equal(encounter.MemberID(goblin), enc.PausedMember())
	s.Equal(cellAt(6, 2), s.positionOf(enc, goblin), "the mover has not moved")

	windows := s.beatsNamed(enc, alice, encounter.BeatWindowOpened)
	s.Require().Len(windows, 1)
	s.Equal(string(goblin), windows[0]["member"])
	s.Equal(whispersRef.String(), windows[0]["cause"],
		"a held directive's window names what is moving them; a held turn's has no cause to name")

	data := enc.ToData()
	s.Require().NotNil(data.HeldDirective, "the hold is in the blob")
	s.Equal(encounter.MemberID(goblin), data.HeldDirective.Member)
	s.Equal(whispersRef.String(), data.HeldDirective.Cause)
	s.Equal([]encounter.PositionData{posData(cellAt(7, 2)), posData(cellAt(8, 2)), posData(cellAt(9, 2))},
		data.HeldDirective.Remaining, "the announced cell comes first")

	loaded, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
	s.Require().NoError(lerr)
	s.True(loaded.Paused(), "a restart between the question and the answer is a non-event")
	s.True(loaded.HeldDirective())
	s.Equal(encounter.MemberID(goblin), loaded.PausedMember())

	wantJSON, err := json.Marshal(data)
	s.Require().NoError(err)
	gotJSON, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	s.JSONEq(string(wantJSON), string(gotJSON), "ToData -> Load -> ToData is the identity")
}

// TestResumeDirectiveStepsTheAnnouncedCellWithoutASecondAnnounceAndWalksTheRest
// is the continuation, and it copies the paused turn's exactly: every reactor
// for the announced cell was already asked, so asking again would pose the
// same window twice and — for a reaction the host has since spent — refuse it
// the second time and silently lose the swing.
func (s *HeldTestSuite) TestResumeDirectiveStepsTheAnnouncedCellWithoutASecondAnnounceAndWalksTheRest() {
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.scene(mover, &downList{})
	_, err := s.flee(enc)
	s.Require().NoError(err)

	out, rerr := enc.ResumeDirective(context.Background())
	s.Require().NoError(rerr)
	s.False(out.Paused)
	s.Equal(len(fleeRoute()), out.Moved, "the whole route, counted across the hold")
	s.Empty(out.StoppedBy)

	s.False(enc.Paused(), "the resume cleared the hold")
	s.False(enc.HeldDirective())
	s.Equal(encounter.MemberID(""), enc.PausedMember())
	s.Equal(cellAt(9, 2), s.positionOf(enc, goblin))
	s.Nil(enc.ToData().HeldDirective, "and the blob no longer carries one")

	announced := 0
	for _, call := range mover.calls {
		if call.To == cellAt(7, 2) {
			announced++
		}
	}
	s.Equal(1, announced, "the announced cell is stepped, not announced a second time")
	for _, call := range mover.calls {
		s.Equal(whispersRef, call.Cause, "every announced cell of a flee names the spell")
		s.False(call.Forced, "and a flee provokes, so it is not forced")
	}

	moved := s.beatsNamed(enc, alice, "moved")
	s.Require().NotEmpty(moved)
	for _, beat := range moved {
		s.Equal(whispersRef.String(), beat["cause"],
			"including the cell stepped by hand: a turn's walk has no cause, a directive's does")
	}
}

// TestResumeDirectiveOnADroppedMoverClearsTheHold is ruling R6 through the
// front door: the reaction landed hard enough, so the announced step never
// happens and the body stays in the cell it was leaving. The hold is still
// cleared — a stale one would freeze a table that is running again.
func (s *HeldTestSuite) TestResumeDirectiveOnADroppedMoverClearsTheHold() {
	standing := &downList{}
	mover := &pausingThenDroppingMover{
		pausingMover: pausingMover{pauseAt: map[int]bool{0: true}},
		standing:     standing,
		dropAt:       0,
	}
	enc := s.scene(mover, standing)
	_, err := s.flee(enc)
	s.Require().NoError(err)
	s.Require().True(enc.Paused())

	before := len(mover.calls)
	out, rerr := enc.ResumeDirective(context.Background())
	s.Require().NoError(rerr)
	s.Equal(0, out.Moved)
	s.False(out.Paused)
	s.False(enc.Paused(), "the hold is cleared even though nothing was walked")
	s.False(enc.HeldDirective())
	s.Equal(cellAt(6, 2), s.positionOf(enc, goblin), "the body is in the cell it was leaving")
	s.Equal(before, len(mover.calls), "a dropped mover announces nothing further")

	_, aerr := enc.ResumeDirective(context.Background())
	s.Require().ErrorIs(aerr, encounter.ErrNotPaused, "and there is nothing left to resume")
}

// TestASecondPauseAccumulatesMoved is the pack passing the line: a route is
// several windows, one per step, and the count of cells walked has to survive
// every one of them or the cast beat reports a flee shorter than it was.
func (s *HeldTestSuite) TestASecondPauseAccumulatesMoved() {
	// THE FIRST HOLD MUST HAVE WALKED SOMETHING, or the accumulation is
	// zero plus something and the test proves nothing. So the first cell
	// goes through and the SECOND one asks.
	mover := &pausingMover{pauseAt: map[int]bool{1: true, 2: true}}
	enc := s.scene(mover, &downList{})
	out, err := s.flee(enc)
	s.Require().NoError(err)
	s.True(out.Paused)
	s.Equal(1, out.Moved, "one cell walked before the first window opened")
	s.Equal(cellAt(7, 2), s.positionOf(enc, goblin))

	// The announced cell is stepped by hand, and the last one asks again.
	again, rerr := enc.ResumeDirective(context.Background())
	s.Require().NoError(rerr)
	s.True(again.Paused, "it is waiting on somebody a second time")
	s.Equal(2, again.Moved, "two cells, counted across the first hold")
	s.True(enc.HeldDirective())
	s.Equal(cellAt(8, 2), s.positionOf(enc, goblin))

	data := enc.ToData()
	s.Require().NotNil(data.HeldDirective)
	s.Equal(2, data.HeldDirective.Moved, "the second hold carries what the first one walked")
	s.Equal([]encounter.PositionData{posData(cellAt(9, 2))}, data.HeldDirective.Remaining)

	last, lerr := enc.ResumeDirective(context.Background())
	s.Require().NoError(lerr)
	s.False(last.Paused)
	s.Equal(len(fleeRoute()), last.Moved, "the whole route, counted across both holds")
	s.Equal(cellAt(9, 2), s.positionOf(enc, goblin))

	s.Len(s.beatsNamed(enc, alice, encounter.BeatWindowOpened), 2, "one window beat per pause")
}

// TestEndTurnIsRefusedWhileADirectiveIsHeld. The table freezes while a player
// decides, and that is the right freeze for a directive too — even though the
// walk being held is nobody's turn, and even for the player whose turn it
// actually is.
func (s *HeldTestSuite) TestEndTurnIsRefusedWhileADirectiveIsHeld() {
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.scene(mover, &downList{})
	_, err := s.flee(enc)
	s.Require().NoError(err)

	_, eerr := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(eerr, encounter.ErrTurnPaused)

	_, rerr := enc.ResumeDirective(context.Background())
	s.Require().NoError(rerr)
	_, eerr = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(eerr, "and the table runs again once the hold is finished")
}

// TestASecondDirectiveIsRefusedWhileOneIsHeld. There is exactly one held
// walk, so a second would overwrite the first — and a blob carrying two is
// one this module refuses to read back.
func (s *HeldTestSuite) TestASecondDirectiveIsRefusedWhileOneIsHeld() {
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.scene(mover, &downList{})
	_, err := s.flee(enc)
	s.Require().NoError(err)

	_, derr := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: alice, Cause: whispersRef, Route: []spatial.Position{cellAt(2, 3)}, Provokes: false,
	})
	s.Require().ErrorIs(derr, encounter.ErrTurnPaused, "somebody else's push waits too")
	s.Equal(cellAt(2, 2), s.positionOf(enc, alice), "and moves nobody")
}

// TestResumeDirectiveWithNothingHeldIsNotPaused. Resuming nothing is a caller
// defect, not a no-op: a host that reaches it has lost track of which half of
// the pose/answer pair it is in.
func (s *HeldTestSuite) TestResumeDirectiveWithNothingHeldIsNotPaused() {
	enc := s.scene(&pausingMover{}, &downList{})

	_, err := enc.ResumeDirective(context.Background())
	s.Require().ErrorIs(err, encounter.ErrNotPaused)
}

// TestAHeldDirectiveOnAClosedEncounterIsRefusedAtLoad, and every other shape
// this build could not have written. The trust boundary is validateHeldDirective,
// and it mirrors validatePausedTurn's refusals — minus the turn coordinates a
// directive has none of, plus the cause it is refused without.
func (s *HeldTestSuite) TestAHeldDirectiveOnAClosedEncounterIsRefusedAtLoad() {
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.scene(mover, &downList{})
	_, err := s.flee(enc)
	s.Require().NoError(err)

	s.Run("a closed encounter", func() {
		data := enc.ToData()
		data.Outcome = &encounter.OutcomeData{Ending: "called"}
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData)
	})

	s.Run("a member who is not one", func() {
		data := enc.ToData()
		data.HeldDirective.Member = "nobody"
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData)
	})

	s.Run("nothing left to walk", func() {
		data := enc.ToData()
		data.HeldDirective.Remaining = nil
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData, "a hold with nothing left to walk is not a hold")
	})

	s.Run("an announced cell that is not the first one left", func() {
		data := enc.ToData()
		data.HeldDirective.To = posData(cellAt(8, 2))
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData)
	})

	s.Run("a cause that is not a ref", func() {
		data := enc.ToData()
		data.HeldDirective.Cause = "not-a-ref"
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData,
			"a directed move names its cause, and a resumed one still has to")
	})

	s.Run("negative progress", func() {
		data := enc.ToData()
		data.HeldDirective.Moved = -1
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData)
	})

	s.Run("a hold and a paused turn at once", func() {
		data := enc.ToData()
		data.PausedTurn = &encounter.PausedTurnData{
			Member:    goblin,
			To:        posData(cellAt(5, 2)),
			Remaining: []encounter.PositionData{posData(cellAt(5, 2))},
			Bound:     2,
		}
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData,
			"there is one held walk, and two would leave the resume verbs guessing")
	})
}

// TestAHeldPushKeepsItsStanceAcrossTheHold. A push is the forced directive and
// a rout is not, and the difference reaches the [Mover] as one flag whose ZERO
// VALUE is the permissive one. So a hold that dropped it would resume a shove
// as a stroll and hand out opportunity attacks nothing paid for — silently,
// and only for the cells after the window.
//
// Nothing poses a window on a forced step today, because a forced step
// suppresses the swing before anybody is asked. The Mover here does it anyway:
// the flag is persisted, and a persisted field that only one caller ever sets
// is exactly the one that rots.
func (s *HeldTestSuite) TestAHeldPushKeepsItsStanceAcrossTheHold() {
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.scene(mover, &downList{})

	out, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: goblin, Cause: thunderwaveRef, Route: fleeRoute(), Provokes: false,
	})
	s.Require().NoError(err)
	s.Require().True(out.Paused)

	reloaded, lerr := encounter.LoadEncounter(loadInput(enc.ToData(), mover, &downList{}))
	s.Require().NoError(lerr, "and it survives the restart, which is where a dropped flag would show")

	_, rerr := reloaded.ResumeDirective(context.Background())
	s.Require().NoError(rerr)

	s.Require().Greater(len(mover.calls), 1)
	for _, call := range mover.calls {
		s.True(call.Forced, "every cell of a push is forced, before and after the hold")
		s.Equal(thunderwaveRef, call.Cause)
	}
}
