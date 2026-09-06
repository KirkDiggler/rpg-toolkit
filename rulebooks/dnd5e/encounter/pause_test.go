// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// PauseTestSuite covers a driven turn stopping mid-walk to ask a player
// whether they react, and continuing once they have answered
// (rpg-project#316 rung 3).
type PauseTestSuite struct {
	suite.Suite
}

func TestPauseSuite(t *testing.T) {
	suite.Run(t, new(PauseTestSuite))
}

var testOpportunityAttack = encounter.ReactionIdentity{
	Ref:  "dnd5e:conditions:opportunity_attack",
	Name: "Opportunity Attack",
}

// pausingMover is a Mover that poses a window on chosen steps instead of
// letting them happen: the composition-side stand-in for the session seam
// asking a player whether they take their opportunity attack.
//
// pauseAt is keyed on the Move-call ORDINAL rather than on a cell, and that
// is the point: a resumed step is never announced a second time, so the
// ordinals of a whole walk are unique and a test can name exactly which step
// pauses, including a second one after a resume.
type pausingMover struct {
	pauseAt map[int]bool
	calls   []announcedStep
}

func (m *pausingMover) Move(
	_ context.Context, _ *encounter.Encounter, mover encounter.MemberID,
	from, to spatial.Position,
) error {
	n := len(m.calls)
	m.calls = append(m.calls, announcedStep{Mover: mover, From: from, To: to})
	if m.pauseAt[n] {
		return &encounter.StepPausedError{Windows: []encounter.PausedWindow{
			{Audience: encounter.MemberID(alice), Reaction: testOpportunityAttack},
		}}
	}
	return nil
}

// pausingThenDroppingMover pauses on a chosen step and, on the way out of
// that pause, reports the mover down — the reaction landed hard enough while
// the player was being asked.
type pausingThenDroppingMover struct {
	pausingMover
	standing *downList
	dropAt   int
}

func (m *pausingThenDroppingMover) Move(
	ctx context.Context, enc *encounter.Encounter, mover encounter.MemberID,
	from, to spatial.Position,
) error {
	err := m.pausingMover.Move(ctx, enc, mover, from, to)
	if len(m.calls)-1 == m.dropAt {
		m.standing.down = append(m.standing.down, mover)
	}
	return err
}

// walkingScene builds a one-room fight where the goblin is asked to walk a
// three-cell path away from alice — the shape every scene in this suite
// needs, differing only in its Mover and its Standing.
func (s *PauseTestSuite) walkingScene(mover encounter.Mover, standing encounter.Standing) *encounter.Encounter {
	return s.sceneWithPath(mover, standing, []spatial.Position{cellAt(5, 2), cellAt(4, 2), cellAt(3, 2)})
}

func (s *PauseTestSuite) sceneWithPath(
	mover encounter.Mover, standing encounter.Standing, path []spatial.Position,
) *encounter.Encounter {
	return s.sceneWithDriver(mover, standing, &scriptedDriver{
		intents: []encounter.TurnIntent{encounter.Move{Path: path}},
	})
}

func (s *PauseTestSuite) sceneWithDriver(
	mover encounter.Mover, standing encounter.Standing, driver encounter.TurnDriver,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: driver,
		Striker:    passStriker{}, Mover: mover, Announcer: quietAnnouncer{},
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

// posData is one cell in the shape the blob stores it in.
func posData(p spatial.Position) encounter.PositionData {
	return encounter.PositionData{X: p.X, Y: p.Y}
}

// positionOf reads one member's cell through the public seam.
func (s *PauseTestSuite) positionOf(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Failf("not placed", "member %q has no cell", id)
	return spatial.Position{}
}

// beats reads one audience's whole story as its list of beat kinds.
func (s *PauseTestSuite) beats(enc *encounter.Encounter, audience encounter.MemberID) []string {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	kinds := make([]string, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	return kinds
}

// windowBeat returns the decoded payload of the one window-opened beat.
func (s *PauseTestSuite) windowBeat(enc *encounter.Encounter, audience encounter.MemberID) map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatWindowOpened {
			return beat
		}
	}
	s.Require().Fail("no window-opened beat in the story")
	return nil
}

// reload round-trips an encounter through the storage boundary.
func (s *PauseTestSuite) reload(enc *encounter.Encounter, mover encounter.Mover, standing encounter.Standing) *encounter.Encounter {
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: mover, Announcer: quietAnnouncer{},
		Data: enc.ToData(),
	})
	s.Require().NoError(err)
	return loaded
}

// TestAPausedStepLeavesTheMoverOnTheCellBefore is the whole rung in one
// scene: the mover walks the first cell, the second is announced and NOT
// taken, and the fight stops there waiting on alice.
func (s *PauseTestSuite) TestAPausedStepLeavesTheMoverOnTheCellBefore() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})

	out, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err, "a window opening is news, not a malfunction")

	s.True(out.Paused, "the caller is told the fight is waiting")
	s.True(enc.Paused())
	s.Equal(encounter.MemberID(goblin), enc.PausedMember())
	s.Equal(encounter.MemberID(goblin), out.Next,
		"the clock has not advanced past the member whose turn is paused")

	s.Require().Len(mover.calls, 2, "the third cell is never announced")
	s.Equal(cellAt(5, 2), s.positionOf(enc, goblin),
		"the mover stands on the cell BEFORE the announced one")
	s.Equal(cellAt(5, 2), mover.calls[1].From)
	s.Equal(cellAt(4, 2), mover.calls[1].To)

	s.Equal([]string{"scene-opened", "bubble-formed", "turn-ended", "moved", encounter.BeatWindowOpened},
		s.beats(enc, alice), "the walked cell is narrated, then the window")

	beat := s.windowBeat(enc, alice)
	s.Equal(string(goblin), beat["member"])
	windows, ok := beat["windows"].([]any)
	s.Require().True(ok)
	s.Require().Len(windows, 1)
	window := windows[0].(map[string]any)
	s.Equal(string(alice), window["audience"])
	s.Equal(testOpportunityAttack.Ref, window["reaction"].(map[string]any)["ref"])
}

// TestAPausedTurnSurvivesASaveAndLoad is the restart-changes-nothing half of
// the design's done-when: rpg-api going down between the question and the
// answer must not lose the turn it interrupted.
func (s *PauseTestSuite) TestAPausedTurnSurvivesASaveAndLoad() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	before := enc.ToData()
	s.Require().NotNil(before.PausedTurn, "the pause is in the blob")
	s.Equal(encounter.MemberID(goblin), before.PausedTurn.Member)
	s.Equal([]encounter.PositionData{posData(cellAt(4, 2)), posData(cellAt(3, 2))}, before.PausedTurn.Remaining,
		"the announced cell comes first, then the cells nobody has walked")
	s.Equal(25, before.PausedTurn.Budget.MovementFeet,
		"the one cell already walked is charged before the blob is written")

	loaded := s.reload(enc, &pausingMover{}, &downList{})
	s.True(loaded.Paused(), "a reloaded encounter is still waiting")

	wantJSON, err := json.Marshal(before)
	s.Require().NoError(err)
	gotJSON, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	s.JSONEq(string(wantJSON), string(gotJSON), "ToData -> Load -> ToData is the identity")
}

// TestResumingFinishesTheWalkAndTheTurn is the answer arriving: the
// announced step is taken without being announced again, the rest of the
// walk happens normally, and the turn ends.
func (s *PauseTestSuite) TestResumingFinishesTheWalkAndTheTurn() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	out, err := enc.ResumeTurn(context.Background())
	s.Require().NoError(err)
	s.False(out.Paused, "the walk finished")
	s.False(enc.Paused())
	s.Equal(encounter.MemberID(alice), out.Next, "the turn came back to the player")

	s.Equal(cellAt(3, 2), s.positionOf(enc, goblin), "the whole path was walked")
	s.Require().Len(mover.calls, 3,
		"the paused cell is NOT announced a second time: two before, one after")
	s.Equal(cellAt(3, 2), mover.calls[2].To)

	s.Equal([]string{
		"scene-opened", "bubble-formed", "turn-ended",
		"moved", encounter.BeatWindowOpened, "moved", "moved", "turn-ended",
	}, s.beats(enc, alice))
}

// TestAMoverDroppedBeforeTheResumeEndsInTheLeavingCell is ruling R6 across a
// pause. The strike the player chose landed while the window was open; the
// body is in the cell it was leaving, the announced step never happens, and
// the fight carries on with whoever is left.
func (s *PauseTestSuite) TestAMoverDroppedBeforeTheResumeEndsInTheLeavingCell() {
	standing := &downList{}
	mover := &pausingThenDroppingMover{
		pausingMover: pausingMover{pauseAt: map[int]bool{1: true}},
		standing:     standing,
		dropAt:       1,
	}
	// A second monster, so the fight outlives the mover and there is a next
	// turn for the resume to drive.
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: &scriptedDriver{intents: []encounter.TurnIntent{
			encounter.Move{Path: []spatial.Position{cellAt(5, 2), cellAt(4, 2), cellAt(3, 2)}},
		}},
		Striker: passStriker{}, Mover: mover, Announcer: quietAnnouncer{},
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
			{
				ID: "skeleton", Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{
					{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
				},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())

	out, err := enc.ResumeTurn(context.Background())
	s.Require().NoError(err)
	s.False(out.Paused)
	s.False(enc.Paused())
	s.Equal(cellAt(5, 2), s.positionOf(enc, goblin),
		"the mover fell in the cell it was LEAVING, and the announced step never happened")
	s.Require().Len(mover.calls, 2, "no cell after the pause is announced either")
	s.Equal(encounter.MemberID(alice), out.Next,
		"the fight carried on past the body and handed the turn back to the player")
}

// TestASecondPauseOnALaterCellWorks: one walk can ask twice. The pack
// passing the line is several windows, one per step.
func (s *PauseTestSuite) TestASecondPauseOnALaterCellWorks() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true, 2: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	first, err := enc.ResumeTurn(context.Background())
	s.Require().NoError(err)
	s.True(first.Paused, "the third cell asked too")
	s.Equal(cellAt(4, 2), s.positionOf(enc, goblin), "the second cell WAS taken")

	data := enc.ToData()
	s.Require().NotNil(data.PausedTurn)
	s.Equal([]encounter.PositionData{posData(cellAt(3, 2))}, data.PausedTurn.Remaining)
	s.Equal(20, data.PausedTurn.Budget.MovementFeet, "two cells charged, not one, not three")

	second, err := enc.ResumeTurn(context.Background())
	s.Require().NoError(err)
	s.False(second.Paused)
	s.Equal(cellAt(3, 2), s.positionOf(enc, goblin))
	s.Equal(encounter.MemberID(alice), second.Next)
}

// TestTheIntentBoundHoldsAcrossAPause is finding 4 of the rung-3 survey: a
// resume that restarted the inner loop at zero would hand a driver that
// never passes a fresh allowance of intents for every window it opened.
//
// The driver here asks for one cell forever. Its member can afford six
// (30 feet), so the bound is eight — and eight is what it gets in total
// however many times the walk is paused and resumed.
func (s *PauseTestSuite) TestTheIntentBoundHoldsAcrossAPause() {
	driver := &spinningWalker{}
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.sceneWithDriver(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())
	asked := len(driver.calls)

	for enc.Paused() {
		_, rerr := enc.ResumeTurn(context.Background())
		s.Require().NoError(rerr)
	}

	s.Equal(8, len(driver.calls),
		"2 + 30/5 intents for the whole turn, pauses included — never a fresh allowance per window")
	s.Less(asked, len(driver.calls), "and the resume genuinely kept asking")
}

// spinningWalker asks for one cell, forever — the misbehaving driver the
// inner bound exists to stop.
type spinningWalker struct {
	calls []encounter.MonsterView
}

func (d *spinningWalker) Act(view encounter.MonsterView) (encounter.TurnIntent, error) {
	d.calls = append(d.calls, view)
	return encounter.Move{Path: []spatial.Position{cellAt(5, 2)}}, nil
}

// TestEveryDriveEntryIsANoOpWhilePaused: the fight has exactly one way
// forward while it is waiting, and nothing else may advance its clock.
func (s *PauseTestSuite) TestEveryDriveEntryIsANoOpWhilePaused() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())
	announced := len(mover.calls)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, encounter.ErrTurnPaused, "EndTurn refuses while a turn is paused")

	_, err = enc.Step(&encounter.StepInput{Member: goblin, To: cellAt(4, 2)})
	s.Require().ErrorIs(err, encounter.ErrTurnPaused, "the paused member cannot step out from under its window")

	// Transfer reaches driveIfStillRunning, one of the five drive entries.
	_, err = enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockWorld})
	s.Require().NoError(err)

	s.Equal(announced, len(mover.calls), "no drive entry announced another step")
	s.True(enc.Paused(), "and the pause still stands")
}

// TestResumeTurnRefusesWhenNothingIsPaused. Resuming nothing is a caller
// that has lost track of which half of the pose/answer pair it is in.
func (s *PauseTestSuite) TestResumeTurnRefusesWhenNothingIsPaused() {
	enc := s.walkingScene(&pausingMover{}, &downList{})

	_, err := enc.ResumeTurn(context.Background())
	s.Require().ErrorIs(err, encounter.ErrNotPaused)
}

// TestAPausedTurnLoadedWithoutItsMemberIsRefused is the trust boundary:
// reject, never crash, on bytes no version of this module wrote.
func (s *PauseTestSuite) TestAPausedTurnLoadedWithoutItsMemberIsRefused() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	data := enc.ToData()
	data.PausedTurn.Member = "nobody"
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: &downList{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Data: data,
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)

	data = enc.ToData()
	data.PausedTurn.Remaining = nil
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: &downList{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Data: data,
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData, "a pause with nothing left to walk is not a pause")

	data = enc.ToData()
	data.PausedTurn.Intent = data.PausedTurn.Bound
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: &downList{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Data: data,
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData, "an intent outside the turn's bound is not resumable")
}

// TestAResumedWalkContinuesFromTheReloadedTurn walks the whole done-when
// path through the storage boundary: pause, save, load, resume, finish.
func (s *PauseTestSuite) TestAResumedWalkContinuesFromTheReloadedTurn() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	resumeMover := &pausingMover{}
	loaded := s.reload(enc, resumeMover, &downList{})

	out, err := loaded.ResumeTurn(context.Background())
	s.Require().NoError(err)
	s.False(out.Paused)
	s.Equal(cellAt(3, 2), s.positionOf(loaded, goblin),
		"a restart between the question and the answer changed nothing")
	s.Require().Len(resumeMover.calls, 1,
		"only the cell after the announced one is announced by the reloaded encounter")
}
