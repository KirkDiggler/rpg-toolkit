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

func (m *pausingMover) Move(_ context.Context, _ *encounter.Encounter, step encounter.MoveStep) error {
	n := len(m.calls)
	m.calls = append(m.calls, announcedOf(step))
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
	ctx context.Context, enc *encounter.Encounter, step encounter.MoveStep,
) error {
	err := m.pausingMover.Move(ctx, enc, step)
	if len(m.calls)-1 == m.dropAt {
		m.standing.down = append(m.standing.down, step.Mover)
	}
	return err
}

// walkingScene builds a one-room fight where the goblin is asked to walk a
// three-cell path away from alice — the shape every scene in this suite
// needs, differing only in its Mover and its Standing.
func (s *PauseTestSuite) walkingScene(mover encounter.Mover, standing encounter.StandingWithParticipation) *encounter.Encounter {
	return s.sceneWithPath(mover, standing, []spatial.Position{cellAt(5, 2), cellAt(4, 2), cellAt(3, 2)})
}

func (s *PauseTestSuite) sceneWithPath(
	mover encounter.Mover, standing encounter.StandingWithParticipation, path []spatial.Position,
) *encounter.Encounter {
	return s.sceneWithDriver(mover, standing, &scriptedDriver{
		intents: []encounter.TurnIntent{encounter.Move{Path: path}},
	})
}

// pauseShortsword is what every driven monster in this file can do: walk thirty
// feet and swing a shortsword at the closest enemy.
var pauseShortsword = encounter.SheetFacts{SpeedFeet: 30, Actions: []encounter.ActionView{
	{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
}, Targeting: "closest"}

// pauseSheets answers the goblin and, where a scene has one, the skeleton —
// on the scene's Setup and on every reload of it, since the sheets are the
// host's and nothing in the blob carries them.
var pauseSheets = sheetFacts{goblin: pauseShortsword, "skeleton": pauseShortsword, alice: {}}

func (s *PauseTestSuite) sceneWithDriver(
	mover encounter.Mover, standing encounter.StandingWithParticipation, driver encounter.Driver,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 2},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   standing,
			Initiative: orderAsGiven{},
			Driver:     driver,
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
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
		kind := beat["beat"].(string)
		// What HAPPENED, not what this member could see — the split
		// standing_test.go's beatKindsOf states in full.
		if recipientScopedKinds[kind] {
			continue
		}
		kinds = append(kinds, kind)
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
func (s *PauseTestSuite) reload(enc *encounter.Encounter, mover encounter.Mover, standing encounter.StandingWithParticipation) *encounter.Encounter {
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(),
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   standing,
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
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
	// THE PAYLOAD IS THE ONE IT ALWAYS WAS. A directed walk's hold writes
	// this same beat with a "cause" added (held.go), and the guard that
	// keeps that additive is only honest if the turn side is pinned too —
	// two repos already decode this beat kind.
	s.NotContains(beat, "cause", "a turn's walk has no cause to name")
	windows, ok := beat["windows"].([]any)
	s.Require().True(ok)
	s.Require().Len(windows, 1)
	window := windows[0].(map[string]any)
	s.Equal(string(alice), window["audience"])
	s.Equal(testOpportunityAttack.Ref, window["reaction"].(map[string]any)["ref"])
}

// TestATurnPauseSurvivesASaveAndLoad is the restart-changes-nothing half of
// the design's done-when: rpg-api going down between the question and the
// answer must not lose the turn it interrupted.
func (s *PauseTestSuite) TestATurnPauseSurvivesASaveAndLoad() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	before := enc.ToData()
	s.Require().NotNil(before.Pause, "the pause is in the blob")
	s.Equal(encounter.MemberID(goblin), before.Pause.Member)
	s.Equal([]encounter.PositionData{posData(cellAt(4, 2)), posData(cellAt(3, 2))}, before.Pause.Remaining,
		"the announced cell comes first, then the cells nobody has walked")
	s.Equal(25, before.Pause.Turn.Budget.MovementFeet,
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

	out, err := enc.Resume(context.Background())
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
		// The resumed turn is the last of the order, so the round wraps on it
		// — and a fight round wrapping is the world getting one (design §5).
		"tick",
	}, s.beats(enc, alice))
}

// TestAPausedTurnRoundTripsThroughOnePause is the turn half of ruling E2: the
// paused turn is written as the one Pause, kind turn, at PauseVersion, and a
// reload resumed through the one Resume finishes the walk exactly as the
// in-memory resume above does.
func (s *PauseTestSuite) TestAPausedTurnRoundTripsThroughOnePause() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.Equal(encounter.PauseVersion, data.Pause.Version)
	s.Equal(encounter.PauseTurn, data.Pause.Kind)
	s.Require().NotNil(data.Pause.Turn, "a turn pause carries its turn")
	s.False(data.Pause.Forced)

	loaded := s.reload(enc, mover, &downList{})
	kind, ok := loaded.PauseKind()
	s.Require().True(ok)
	s.Equal(encounter.PauseTurn, kind)

	out, err := loaded.Resume(context.Background())
	s.Require().NoError(err)
	s.Equal(encounter.PauseTurn, out.Kind)
	s.False(out.Paused, "the walk finished")
	s.False(loaded.Paused())
	_, ok = loaded.PauseKind()
	s.False(ok)
	s.Equal(encounter.MemberID(alice), out.Next, "the turn came back to the player")

	s.Equal(cellAt(3, 2), s.positionOf(loaded, goblin), "the whole path was walked")
	s.Require().Len(mover.calls, 3,
		"the paused cell is NOT announced a second time: two before, one after")
	s.Equal(cellAt(3, 2), mover.calls[2].To)

	s.Equal([]string{
		"scene-opened", "bubble-formed", "turn-ended",
		"moved", encounter.BeatWindowOpened, "moved", "moved", "turn-ended",
		"tick",
	}, s.beats(loaded, alice))
}

// TestAStalePauseVersionIsRefused is ruling E5 at the encounter's door: a
// pause another build wrote is refused by name, before anything is built.
func (s *PauseTestSuite) TestAStalePauseVersionIsRefused() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	for _, version := range []int{0, encounter.PauseVersion + 1} {
		data := enc.ToData()
		data.Pause.Version = version
		_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
		s.Require().ErrorIs(lerr, encounter.ErrStalePause, "version %d", version)
	}

	// THE VERSION IS ASKED FIRST. A stale pause that is also malformed is
	// refused as stale, not as whichever field happened to disagree.
	data := enc.ToData()
	data.Pause.Version = encounter.PauseVersion + 1
	data.Pause.Kind = "cast"
	_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
	s.Require().ErrorIs(lerr, encounter.ErrStalePause, "the version wins over every other refusal")
}

// TestAPauseWithTheWrongArmIsRefused — the kind decides the arm. A turn
// carries its turn and is never forced; a directive carries no turn and
// always names its cause; and a kind this build does not know is no pause.
func (s *PauseTestSuite) TestAPauseWithTheWrongArmIsRefused() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	cases := []struct {
		name   string
		mutate func(*encounter.PauseData)
	}{
		{"a turn without its turn", func(p *encounter.PauseData) { p.Turn = nil }},
		{"a directive with a turn", func(p *encounter.PauseData) {
			p.Kind = encounter.PauseDirective
			p.Cause = "dnd5e:spells:dissonant-whispers"
		}},
		{"a turn that is forced", func(p *encounter.PauseData) { p.Forced = true }},
		{"a kind nobody wrote", func(p *encounter.PauseData) { p.Kind = "sideways" }},
		{"a directive with no cause", func(p *encounter.PauseData) {
			p.Kind = encounter.PauseDirective
			p.Turn = nil
		}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := enc.ToData()
			tc.mutate(data.Pause)
			_, lerr := encounter.LoadEncounter(loadInput(data, &pausingMover{}, &downList{}))
			s.Require().ErrorIs(lerr, encounter.ErrInvalidData)
		})
	}
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
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 2},
			},
			{
				ID: "skeleton", Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 2},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   standing,
			Initiative: orderAsGiven{},
			Driver: &scriptedDriver{intents: []encounter.TurnIntent{
				encounter.Move{Path: []spatial.Position{cellAt(5, 2), cellAt(4, 2), cellAt(3, 2)}},
			}},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())

	out, err := enc.Resume(context.Background())
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

	first, err := enc.Resume(context.Background())
	s.Require().NoError(err)
	s.True(first.Paused, "the third cell asked too")
	s.Equal(cellAt(4, 2), s.positionOf(enc, goblin), "the second cell WAS taken")

	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.Equal([]encounter.PositionData{posData(cellAt(3, 2))}, data.Pause.Remaining)
	s.Equal(20, data.Pause.Turn.Budget.MovementFeet, "two cells charged, not one, not three")

	second, err := enc.Resume(context.Background())
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
		_, rerr := enc.Resume(context.Background())
		s.Require().NoError(rerr)
	}

	// AN ALLOWANCE, NOT A COUNT. 2 + 30/5 is what the whole turn may spend on
	// intents, pauses included; a resume that restarted the inner loop would
	// hand out a second allowance and land above it. How many of the
	// allowance the turn actually uses is not the guarantee and is free to
	// change — it dropped by one when the drive loop stopped asking a paused
	// member for a turn they were already taking.
	s.LessOrEqual(len(driver.calls), 8,
		"never a fresh allowance of intents per window")
	s.Less(asked, len(driver.calls), "and the resume genuinely kept asking")
}

// spinningWalker asks for one cell, forever — the misbehaving driver the
// inner bound exists to stop.
type spinningWalker struct {
	calls []encounter.MonsterView
}

func (d *spinningWalker) Act(view encounter.MonsterView) (encounter.Decision, error) {
	d.calls = append(d.calls, view)
	return encounter.Decision{Intent: encounter.Move{Path: []spatial.Position{cellAt(5, 2)}}}, nil
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

// TestResumeWithNothingPausedIsRefused. Resuming nothing is a caller that has
// lost track of which half of the pose/answer pair it is in.
func (s *PauseTestSuite) TestResumeWithNothingPausedIsRefused() {
	enc := s.walkingScene(&pausingMover{}, &downList{})

	_, err := enc.Resume(context.Background())
	s.Require().ErrorIs(err, encounter.ErrNotPaused)
}

// TestATurnPauseLoadedWithoutItsMemberIsRefused is the trust boundary:
// reject, never crash, on bytes no version of this module wrote.
func (s *PauseTestSuite) TestATurnPauseLoadedWithoutItsMemberIsRefused() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := s.walkingScene(mover, &downList{})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	data := enc.ToData()
	data.Pause.Member = "nobody"
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   &downList{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)

	data = enc.ToData()
	data.Pause.Remaining = nil
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   &downList{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData, "a pause with nothing left to walk is not a pause")

	data = enc.ToData()
	data.Pause.Turn.Intent = data.Pause.Turn.Bound
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   &downList{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData, "an intent outside the turn's bound is not resumable")

	// The cause a Routed pause carries is the newest field on this shape, and
	// it gets the twin of the held directive's own check (held_test.go): a
	// pause that names a cause the grammar cannot read is bytes no version of
	// this module wrote, and it is refused before anything is constructed.
	data = enc.ToData()
	data.Pause.Cause = "not-a-ref"
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   &downList{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData,
		"a compelled walk names its cause, and a resumed one still has to")
}

// TestTheLastMonsterDroppedInTheWindowReloadsAndResumesCleanly is the case
// that had the two halves of pause.go disagreeing: the strike a player chose
// through the window drops the LAST monster, so the fight dissolves and the
// body is spliced out of its bubble — all of it recorded before anybody
// resumes. The paused member is then on no clock, which is the ORDINARY
// consequence of the answer, and both the load and the resume have to say so.
func (s *PauseTestSuite) TestTheLastMonsterDroppedInTheWindowReloadsAndResumesCleanly() {
	standing := &downList{}
	mover := &pausingThenDroppingMover{
		pausingMover: pausingMover{pauseAt: map[int]bool{1: true}},
		standing:     standing,
		dropAt:       1,
	}
	enc := s.walkingScene(mover, standing)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())

	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.Require().Empty(data.Bubbles,
		"the last monster falling ended the fight while the window was open")

	// The host's own mid-verb reload: this used to be refused as corruption.
	loaded, lerr := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   standing,
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     &pausingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(lerr, "a paused member in no fight is legal, not corruption")
	s.Require().True(loaded.Paused())

	out, rerr := loaded.Resume(context.Background())
	s.Require().NoError(rerr)
	s.False(out.Paused)
	s.False(loaded.Paused(), "the resume cleared the pause")
	s.Equal(cellAt(5, 2), s.positionOf(loaded, goblin),
		"the body is still in the cell it was leaving: the announced step never happened")

	after, aerr := loaded.Resume(context.Background())
	s.Require().ErrorIs(aerr, encounter.ErrNotPaused, "and there is nothing left to resume")
	s.Nil(after)
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

	out, err := loaded.Resume(context.Background())
	s.Require().NoError(err)
	s.False(out.Paused)
	s.Equal(cellAt(3, 2), s.positionOf(loaded, goblin),
		"a restart between the question and the answer changed nothing")
	s.Require().Len(resumeMover.calls, 1,
		"only the cell after the announced one is announced by the reloaded encounter")
}

// A ROUTED TURN PAUSED MID-WALK is the case this suite grew for Command
// (rpg-project ideas/spells/command §5.4). Everything about the hold is the
// same as a Move's; what is new is that finishing the walk finishes the TURN,
// and that the flag saying so, and the cause the beats name, both have to
// survive the window and a restart.

// routedWalkingScene is walkingScene with a Routed intent instead of a Move:
// alice at [2,2] and the goblin at [6,2], compelled to approach her.
func (s *PauseTestSuite) routedWalkingScene(
	mover encounter.Mover, standing encounter.StandingWithParticipation, driver encounter.Driver,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 2},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     pauseSheets,
			Standing:   standing,
			Initiative: orderAsGiven{},
			Driver:     driver,
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// TestARoutedPauseCarriesTerminalAndItsCauseThroughASaveAndLoad. The window
// opened on the second cell of a compelled walk. Neither of the two facts the
// resume needs — that this turn ends with the walk, and what routed it — can
// be rebuilt from the cells that are left, so both are in the blob.
func (s *PauseTestSuite) TestARoutedPauseCarriesTerminalAndItsCauseThroughASaveAndLoad() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	driver := routedDriver(encounter.MoveToward, alice)
	enc := s.routedWalkingScene(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())

	before := enc.ToData()
	s.Require().NotNil(before.Pause)
	s.True(before.Pause.Turn.Terminal, "a routed turn ends when its walk does")
	s.Equal(commandedRef.String(), before.Pause.Cause, "and the beats after the window still say why")
	s.Equal(25, before.Pause.Turn.Budget.MovementFeet,
		"the one cell already walked is charged before the blob is written")

	loaded := s.reload(enc, &pausingMover{}, &downList{})
	s.True(loaded.Paused())

	wantJSON, err := json.Marshal(before)
	s.Require().NoError(err)
	gotJSON, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	s.JSONEq(string(wantJSON), string(gotJSON), "ToData -> Load -> ToData is the identity")
}

// TestAResumedRoutedTurnEndsWithoutAnotherAct is the terminal rule arriving
// through the resume: the rest of the walk happens, the turn ends, and the
// driver is never asked a second time.
//
// One Act call across a pause and a resume is the whole assertion. A resume
// that fell into runTurnIntents would ask again, and a commanded creature
// would get a turn its compulsion never gave it.
func (s *PauseTestSuite) TestAResumedRoutedTurnEndsWithoutAnotherAct() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	driver := routedDriver(encounter.MoveToward, alice)
	enc := s.routedWalkingScene(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())
	s.Require().Len(driver.calls, 1)

	out, err := enc.Resume(context.Background())
	s.Require().NoError(err)
	s.False(out.Paused, "the walk finished")
	s.False(enc.Paused())
	s.Len(driver.calls, 1, "the brain is asked once for the whole turn, window and all")
	s.Equal(encounter.MemberID(alice), out.Next, "the turn came back to the player")
	s.Equal(cellAt(3, 2), s.positionOf(enc, goblin), "and the whole route was walked")

	// The three cells and the ending, not the whole transcript: what this test
	// claims is that the walk finished and the turn ended, and pinning every
	// other beat in the story would also pin "nothing else happened", which it
	// does not claim and should not tax.
	s.Equal(3, len(movedBeatsOf(s.T(), enc, alice)), "one beat per cell of the route")
	s.True(turnEndedFor(s.T(), enc, goblin), "and the walk ended the turn")
}

// TestEveryCellOfAResumedRoutedWalkNamesItsCause. The cause survives the
// window, and that includes the hand-taken cell the resume steps without
// announcing — a single beat in three claiming the creature walked off on its
// own would be the story lying about a compulsion.
func (s *PauseTestSuite) TestEveryCellOfAResumedRoutedWalkNamesItsCause() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	driver := routedDriver(encounter.MoveToward, alice)
	enc := s.routedWalkingScene(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(commandedRef.String(), s.windowBeat(enc, alice)["cause"],
		"the window says what the interrupted walk was")

	_, err = enc.Resume(context.Background())
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	cells := 0
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] != "moved" {
			continue
		}
		cells++
		s.Equal(commandedRef.String(), beat["cause"], "every cell, the hand-taken one included")
	}
	s.Equal(3, cells)
}

// TestAMoveTurnPauseStillResumesIntoAnotherAct is the flag's other side,
// pinned so terminal cannot quietly become "every paused turn". A Move's walk
// is one intent of a turn that has more, so finishing it asks the driver
// again — and the blob says the turn was never terminal.
func (s *PauseTestSuite) TestAMoveTurnPauseStillResumesIntoAnotherAct() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Move{Path: []spatial.Position{cellAt(5, 2), cellAt(4, 2), cellAt(3, 2)}},
		encounter.Pass{},
	}}
	enc := s.sceneWithDriver(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())
	s.Require().Len(driver.calls, 1)

	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.False(data.Pause.Turn.Terminal, "a Move's pause is not terminal")
	s.Empty(data.Pause.Cause, "and a walk the creature chose has no cause to name")

	_, err = enc.Resume(context.Background())
	s.Require().NoError(err)
	s.Greater(len(driver.calls), 1, "the turn had intents left and the driver was asked for them")
}

// TestATurnPauseTakesNoSecondStepWhileItsWindowIsOpen is the regression for
// a bug older than the Routed intent that exposed it: the drive loop carried
// on past one of its own paused turns.
//
// The clock has not advanced while a window is open, so the paused member was
// still the active one. The next iteration of the loop built a fresh view for
// them, asked their driver for a turn they were already mid-way through, and
// took the answer: a SECOND step announced and walked while the reactor was
// still being asked about the first, charged against a fresh full budget. The
// stored pause was then a lie — it described a walk from a cell the mover had
// already left, still owing an announced step into the cell it was standing
// on.
//
// The teeth are the three facts a caller or a reload can see: one Move
// announcement, the mover still on the cell before the announced one, and a
// stored pause whose `from` is where the mover actually stands. A driver that
// keeps asking for cells is what makes the second step available to be taken.
func (s *PauseTestSuite) TestATurnPauseTakesNoSecondStepWhileItsWindowIsOpen() {
	driver := &spinningWalker{}
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := s.sceneWithDriver(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused())

	s.Len(mover.calls, 1, "exactly one step is announced, and it is not taken")
	s.Equal(cellAt(6, 2), s.positionOf(enc, goblin),
		"the mover has not moved: the announced step is what the window is about")

	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.Equal(posData(cellAt(6, 2)), data.Pause.From,
		"the stored pause says the mover stands where it actually stands")
	s.Equal(posData(cellAt(5, 2)), data.Pause.To)
	s.Equal(30, data.Pause.Turn.Budget.MovementFeet, "and owes its whole turn's movement")

	s.Equal([]string{"scene-opened", "bubble-formed", "turn-ended", encounter.BeatWindowOpened},
		s.beats(enc, alice), "no cell is narrated as walked, because none was")
}

// TestARoutedWalkPausedTwiceIsStillOneCompelledTurn is the case the terminal
// flag exists for, and the only one that exercises carrying it FORWARD.
//
// One compelled walk, two reactors. The first window is the fresh pause the
// intent stored; the second is a pause stored by the RESUME, out of
// finishPausedIntent rather than executeTurnIntent — a different constructor,
// and the one that has to copy both facts across. Drop `terminal` there and
// the second resume falls back into runTurnIntents and hands the compelled
// driver a second Act for a turn its compulsion never gave it. Drop `cause`
// and the last cells narrate a creature strolling off on its own.
func (s *PauseTestSuite) TestARoutedWalkPausedTwiceIsStillOneCompelledTurn() {
	mover := &pausingMover{pauseAt: map[int]bool{1: true, 2: true}}
	driver := routedDriver(encounter.MoveToward, alice)
	enc := s.routedWalkingScene(mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused(), "the first window")

	first, err := enc.Resume(context.Background())
	s.Require().NoError(err)
	s.Require().True(first.Paused, "a later cell of the same walk asked somebody else")
	s.Require().True(enc.Paused(), "the second window")

	second := enc.ToData()
	s.Require().NotNil(second.Pause)
	s.True(second.Pause.Turn.Terminal, "the re-pause still knows the turn ends with the walk")
	s.Equal(commandedRef.String(), second.Pause.Cause, "and still knows what routed it")

	out, err := enc.Resume(context.Background())
	s.Require().NoError(err)
	s.False(out.Paused, "the walk finished")
	s.False(enc.Paused())
	s.Len(driver.calls, 1, "one Act for the whole turn, both windows included")
	s.Equal(cellAt(3, 2), s.positionOf(enc, goblin), "and the whole route was walked")
	s.Equal(encounter.MemberID(alice), out.Next)

	moved := 0
	for _, beat := range storyBeats(s.T(), enc, alice) {
		if beat["beat"] != "moved" {
			continue
		}
		moved++
		s.Equal(commandedRef.String(), beat["cause"], "every cell, across both windows")
	}
	s.Equal(3, moved)
	s.True(turnEndedFor(s.T(), enc, goblin))
}
