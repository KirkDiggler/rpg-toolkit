// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/internal/turntest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// turnfacts_test.go pins rpg-project#1131 slice 1d-3: the encounter reads what
// is left of a driven creature's turn from its SHEET ([encounter.SheetFacts.Turn])
// and keeps no count of its own. A run is paid through [encounter.Dasher]; a
// round of the world is as it was.
//
// Every scene is one long hall, alice at the west end and the goblin a named
// distance east of her, the goblin second in initiative so alice's EndTurn
// drives its turn.

type TurnFactsSuite struct {
	suite.Suite
}

func TestTurnFactsSuite(t *testing.T) { suite.Run(t, new(TurnFactsSuite)) }

// goblinSheet is thirty feet, a five-foot swing and the nimble trait.
func goblinSheet() encounter.SheetFacts {
	return encounter.SheetFacts{
		SpeedFeet:      30,
		Actions:        []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}},
		BonusDisengage: nimbleEscape,
	}
}

// turnScene is one hall and what stands in it.
type turnScene struct {
	// goblinAt and the members besides alice and the goblin.
	goblinAt int
	others   []encounter.MemberInput

	// sheets is the scene's own host, before any turn host wraps it. Nil means
	// the goblin's sheet and nothing else.
	sheets encounter.Sheets

	driver encounter.Driver
	actors encounter.Actors

	// table is the goblin's own behaviour, for a scene the table drives.
	table encounter.Table
}

func (s *TurnFactsSuite) build(sc turnScene) *encounter.Encounter {
	s.T().Helper()
	at := sc.goblinAt
	if at == 0 {
		at = 2
	}
	sheets := sc.sheets
	if sheets == nil {
		sheets = sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}
	}
	members := append([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: onRow(1)},
		{ID: goblin, Kind: encounter.KindMonster, Position: onRow(at), Behaviors: ordinaryBehavior(sc.table)},
	}, sc.others...)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 60, 8)},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: everyoneStanding{},
			Initiative:    arrivalOrder{},
			Driver:        sc.driver,
			Roller:        rollsLowest{},
			Actors:        sc.actors,
		},
	})
	s.Require().NoError(err)

	return enc
}

// plainActors is a scene whose host keeps no turn: a striker that records, a
// quiet mover and announcer, and whatever else a test names.
func plainActors(striker encounter.Striker) encounter.Actors {
	return encounter.Actors{Striker: striker, Mover: quietMover{}, Announcer: quietAnnouncer{}}
}

func (s *TurnFactsSuite) endTurn(enc *encounter.Encounter, who encounter.MemberID) {
	s.T().Helper()
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: who})
	s.Require().NoError(err)
}

// beats is every story beat of a kind naming a member, decoded, in order.
func (s *TurnFactsSuite) beats(enc *encounter.Encounter, kind string, who encounter.MemberID) []map[string]any {
	s.T().Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	var out []map[string]any
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind && (beat["member"] == string(who) || beat["creature"] == string(who) || beat["actor"] == string(who)) {
			out = append(out, beat)
		}
	}

	return out
}

// attacksSeen and bonusSeen are what each of a driver's views showed.
func attacksSeen(d *scriptedDriver) []int {
	var out []int
	for _, v := range d.calls {
		out = append(out, v.Budget.AttacksLeft)
	}

	return out
}

func swings(n int) []encounter.TurnIntent {
	out := make([]encounter.TurnIntent, n)
	for i := range out {
		out[i] = encounter.Attack{Target: alice, Action: testMeleeAction}
	}

	return out
}

// --- 1 ----------------------------------------------------------------------

// TestASilentHostHasAFullTurn: a host that keeps no account says nothing, and
// that creature has one action and one bonus action every time it is asked.
func (s *TurnFactsSuite) TestASilentHostHasAFullTurn() {
	driver := &scriptedDriver{}
	enc := s.build(turnScene{driver: driver, actors: plainActors(&swingRecorder{})})

	s.endTurn(enc, alice)

	s.Require().NotEmpty(driver.calls)
	s.Equal(1, driver.calls[0].Budget.AttacksLeft, "one action")
	s.Equal(1, driver.calls[0].Budget.BonusActions, "and one bonus action")
	s.Equal(30, driver.calls[0].Budget.MovementFeet)
}

// --- 2 ----------------------------------------------------------------------

type noActionLeft struct{ inner sheetFacts }

func (h noActionLeft) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out, err := h.inner.Sheets(members)
	if err != nil {
		return nil, err
	}
	for id, facts := range out {
		facts.Turn = &encounter.TurnFacts{ActionsLeft: 0, BonusActionsLeft: 1}
		out[id] = facts
	}

	return out, nil
}

// TestTheSheetIsTheJudgeOfASwing: the host says no action is left, so the view
// shows no attack, `attack` is not on the table, an Attack intent is refused
// with the existing words, and the striker is never asked.
func (s *TurnFactsSuite) TestTheSheetIsTheJudgeOfASwing() {
	spent := noActionLeft{inner: sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}}

	s.Run("an Attack intent is refused", func() {
		striker := &swingRecorder{}
		driver := &scriptedDriver{intents: swings(1)}
		enc := s.build(turnScene{sheets: spent, driver: driver, actors: plainActors(striker)})

		s.endTurn(enc, alice)

		s.Require().NotEmpty(driver.calls)
		s.Equal(0, driver.calls[0].Budget.AttacksLeft, "the view shows no attack")
		s.Empty(striker.swings, "the striker is never asked")
		stayed := s.beats(enc, encounter.BeatStayed, goblin)
		s.Require().Len(stayed, 1)
		s.Equal("no attack left", stayed[0]["why"])
	})

	s.Run("attack is not on the table", func() {
		striker := &swingRecorder{}
		enc := s.build(turnScene{
			sheets: spent, driver: tableDriver(), actors: plainActors(striker),
			table: encounter.Table{encounter.AnswerTime: {
				{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
					Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
				{Weight: 1, Hold: true},
			}},
		})

		s.endTurn(enc, alice)

		picks := s.beats(enc, encounter.BeatAnswered, goblin)
		s.Require().NotEmpty(picks)
		s.Equal(false, verdict(picks[0], 0)["affordable"], "the sheet says it cannot pay for a swing")
		s.Equal("hold", picks[0]["word"], "so the roll landed on the hold")
		s.Empty(striker.swings)
	})
}

// --- 3 ----------------------------------------------------------------------

// TestThisModulesOwnOneIsGone: a host that says two actions a turn is a
// creature that swings twice, and a third is refused.
func (s *TurnFactsSuite) TestThisModulesOwnOneIsGone() {
	striker := &swingRecorder{}
	host := turntest.New(sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}, striker, nil)
	host.AllowPerTurn(goblin, 2, 1)
	driver := &scriptedDriver{intents: swings(3)}
	enc := s.build(turnScene{sheets: host, driver: driver, actors: host.Actors(quietMover{}, false)})

	s.endTurn(enc, alice)

	s.Equal([]int{2, 1, 0}, attacksSeen(driver), "the views count the sheet's own two down")
	s.Len(striker.swings, 2, "two swings landed")
	stayed := s.beats(enc, encounter.BeatStayed, goblin)
	s.Require().Len(stayed, 1, "and the third was refused, told")
	s.Equal("no attack left", stayed[0]["why"])
}

// --- 4 ----------------------------------------------------------------------

// TestOnceATurnAndAgainNextTurn: one swing, a second refused, and in the next
// round one swing again.
func (s *TurnFactsSuite) TestOnceATurnAndAgainNextTurn() {
	striker := &swingRecorder{}
	host := turntest.New(sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}, striker, nil)
	driver := &scriptedDriver{intents: swings(3)}
	enc := s.build(turnScene{sheets: host, driver: driver, actors: host.Actors(quietMover{}, false)})

	s.endTurn(enc, alice)
	s.Len(striker.swings, 1, "one swing in the first turn")
	s.Equal([]int{1, 0}, attacksSeen(driver), "the second swing was asked for and refused")

	s.endTurn(enc, alice)
	s.Len(striker.swings, 2, "one swing again in the next turn")
	s.Equal([]int{1, 0, 1, 0}, attacksSeen(driver)[:4])
}

// --- 5 ----------------------------------------------------------------------

// leavesAliceThenBob is the script of a goblin beside alice with bob east of it:
// out of alice's reach (a Disengage), into bob's, and out of his again.
func leavesAliceThenBob() []encounter.TurnIntent {
	return []encounter.TurnIntent{
		encounter.Move{Path: []spatial.Position{onRow(3)}},
		encounter.Move{Path: []spatial.Position{onRow(4)}},
		encounter.Move{Path: []spatial.Position{onRow(3)}},
		encounter.Pass{},
	}
}

// TestTheSheetIsTheJudgeOfADisengage: a host that says no bonus action is left
// is never asked to disengage; a host that keeps the turn is asked once a turn,
// and once again the next.
func (s *TurnFactsSuite) TestTheSheetIsTheJudgeOfADisengage() {
	bobAt := []encounter.MemberInput{{ID: bob, Kind: encounter.KindPlayer, Position: onRow(5)}}
	sheets := sheetFacts{alice: {SpeedFeet: 30}, bob: {}, goblin: goblinSheet()}

	s.Run("no bonus action left", func() {
		d := &recordingDisengager{}
		driver := &scriptedDriver{intents: leavesAliceThenBob()[:1]}
		actors := plainActors(&swingRecorder{})
		actors.Disengager = d
		enc := s.build(turnScene{
			sheets: noBonusLeft{inner: sheets}, driver: driver, actors: actors, others: bobAt,
		})

		s.endTurn(enc, alice)

		s.Empty(d.calls, "nobody was asked to disengage")
		s.Equal(onRow(3), whereIs(s.T(), enc, goblin), "the walk happened")
		s.Equal(0, driver.calls[0].Budget.BonusActions)
	})

	s.Run("one a turn, and again next turn", func() {
		d := &recordingDisengager{}
		host := turntest.New(sheets, &swingRecorder{}, nil).WithDisengager(d)
		next := []encounter.TurnIntent{
			encounter.Move{Path: []spatial.Position{onRow(4)}},
			encounter.Move{Path: []spatial.Position{onRow(3)}},
			encounter.Pass{},
		}
		driver := &scriptedDriver{intents: append(leavesAliceThenBob(), next...)}
		enc := s.build(turnScene{sheets: host, driver: driver, actors: host.Actors(quietMover{}, true), others: bobAt})

		s.endTurn(enc, alice)
		s.Equal([]encounter.MemberID{goblin}, host.Disengages, "the second leaving in one turn was not paid for")
		s.Equal([]encounter.MemberID{goblin}, d.calls)

		s.endTurn(enc, alice)
		s.Equal([]encounter.MemberID{goblin, goblin}, host.Disengages, "the next turn has a bonus action again")
	})
}

type noBonusLeft struct{ inner sheetFacts }

func (h noBonusLeft) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out, err := h.inner.Sheets(members)
	if err != nil {
		return nil, err
	}
	for id, facts := range out {
		facts.Turn = &encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 0}
		out[id] = facts
	}

	return out, nil
}

// --- 6 ----------------------------------------------------------------------

// runAtAlice is a run from thirteen cells away that lands beside her: twelve
// cells, twice what thirty feet walks.
func runAtAlice() encounter.Move {
	path := make([]spatial.Position, 0, 12)
	for x := 13; x >= 2; x-- {
		path = append(path, onRow(x))
	}

	return encounter.Move{Dash: true, Path: path}
}

// TestARunOnATurnIsPaidThroughTheHost.
func (s *TurnFactsSuite) TestARunOnATurnIsPaidThroughTheHost() {
	sheets := sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}

	s.Run("paid, then the swing is spent", func() {
		striker := &swingRecorder{}
		host := turntest.New(sheets, striker, nil)
		driver := &scriptedDriver{intents: append([]encounter.TurnIntent{runAtAlice()}, swings(1)...)}
		enc := s.build(turnScene{goblinAt: 14, sheets: host, driver: driver, actors: host.Actors(quietMover{}, false)})

		s.endTurn(enc, alice)

		s.Equal([]encounter.MemberID{goblin}, host.Dashes, "the Dasher was asked once")
		s.Equal(onRow(2), whereIs(s.T(), enc, goblin), "the speed was added: it ran the whole gap")
		s.Len(s.beats(enc, encounter.BeatMoved, goblin), 12)
		s.Equal([]int{1, 0}, attacksSeen(driver), "the run spent the action the sheet pays")
		s.Empty(striker.swings, "a swing after the run is refused")
		stayed := s.beats(enc, encounter.BeatStayed, goblin)
		s.Require().Len(stayed, 1)
		s.Equal("no attack left", stayed[0]["why"])
	})

	s.Run("a run after a swing is refused and the Dasher is not asked", func() {
		striker := &swingRecorder{}
		host := turntest.New(sheets, striker, nil)
		driver := &scriptedDriver{intents: []encounter.TurnIntent{
			swings(1)[0], encounter.Move{Dash: true, Path: []spatial.Position{onRow(3)}},
		}}
		enc := s.build(turnScene{sheets: host, driver: driver, actors: host.Actors(quietMover{}, false)})

		s.endTurn(enc, alice)

		s.Len(striker.swings, 1)
		s.Empty(host.Dashes, "the sheet had no action left to pay with")
		s.Equal(onRow(2), whereIs(s.T(), enc, goblin), "it did not run")
		stayed := s.beats(enc, encounter.BeatStayed, goblin)
		s.Require().Len(stayed, 1)
		s.Equal("encounter:table:dash", stayed[0]["cause"])
		s.Equal("its action is spent", stayed[0]["why"])
	})

	s.Run("no Dasher wired", func() {
		actors := plainActors(&swingRecorder{})
		driver := &scriptedDriver{intents: []encounter.TurnIntent{runAtAlice(), encounter.Pass{}}}
		enc := s.build(turnScene{goblinAt: 14, sheets: sheets, driver: driver, actors: actors})

		s.endTurn(enc, alice)

		s.Equal(onRow(14), whereIs(s.T(), enc, goblin), "no speed was added: it did not move")
		s.Len(driver.calls, 1, "the turn ended")
		stayed := s.beats(enc, encounter.BeatStayed, goblin)
		s.Require().Len(stayed, 1)
		s.Equal("encounter:table:dash", stayed[0]["cause"])
		s.Equal("nobody can be paid for its run", stayed[0]["why"])
	})

	s.Run("a Dasher that fails aborts the verb", func() {
		host := turntest.New(sheets, &swingRecorder{}, nil)
		host.FailDash = errors.New("the sheet could not pay")
		driver := &scriptedDriver{intents: []encounter.TurnIntent{runAtAlice()}}
		enc := s.build(turnScene{goblinAt: 14, sheets: host, driver: driver, actors: host.Actors(quietMover{}, false)})

		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})

		s.Require().ErrorIs(err, host.FailDash)
		s.Equal(onRow(14), whereIs(s.T(), enc, goblin), "nothing added: it did not walk")
		s.Empty(s.beats(enc, encounter.BeatMoved, goblin))
		s.Empty(s.beats(enc, encounter.BeatStayed, goblin), "nothing told")
	})
}

// --- 7 ----------------------------------------------------------------------

// TestARoundOfTheWorldIsAsItWas: a host that says a full turn changes nothing
// on the world clock. The view shows no attack and no bonus action, a run adds
// its speed and asks no Dasher, and nobody is asked to disengage.
func (s *TurnFactsSuite) TestARoundOfTheWorldIsAsItWas() {
	host := turntest.New(sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}, &swingRecorder{}, nil)
	path := make([]spatial.Position, 0, 7)
	for x := 19; x >= 13; x-- {
		path = append(path, spatial.Position{X: float64(x), Y: 6})
	}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Move{Dash: true, Path: path}}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
			Factions: []encounter.FactionInput{{ID: "vendors", Mind: goblin}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors", Position: spatial.Position{X: 20, Y: 6},
				Behaviors: ordinaryBehavior(encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}})},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        host,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
			Roller:        rollsLowest{},
			Actors:        host.Actors(quietMover{}, true),
		},
	})
	s.Require().NoError(err)

	s.Require().NoError(encounter.ExportedSpendWorldAction(enc, alice))

	s.Require().GreaterOrEqual(len(driver.calls), 2, "the world woke the goblin and asked it again")
	first := driver.calls[0]
	s.Equal(encounter.ClockWorld, first.Clock)
	s.Equal(0, first.Budget.AttacksLeft, "a round of the world carries no attack")
	s.Equal(0, first.Budget.BonusActions, "and no bonus action, whatever the host says")
	s.Equal(30, first.Budget.MovementFeet)
	s.Equal(25, driver.calls[1].Budget.MovementFeet, "the run added its speed: 30 and 30 less seven cells")
	s.Equal(spatial.Position{X: 13, Y: 6}, whereIs(s.T(), enc, goblin))
	s.Empty(host.Dashes, "nobody is asked to be paid on the world clock")
	s.Empty(host.Disengages)
}

// --- 8 ----------------------------------------------------------------------

// TestAStrikeThatPausesSavesNoCount: the saved pause has no key for attacks or
// bonus actions; the resumed creature is not asked to swing again that turn,
// because the host says it has spent its action.
func (s *TurnFactsSuite) TestAStrikeThatPausesSavesNoCount() {
	sheets := sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}
	first := &pausingStriker{pause: true}
	host := turntest.New(sheets, first, nil)
	driver := &scriptedDriver{intents: swings(1)}
	enc := s.build(turnScene{sheets: host, driver: driver, actors: host.Actors(quietMover{}, false)})

	s.endTurn(enc, alice)
	s.Require().True(enc.Paused())
	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.Require().True(data.Pause.Turn.AfterStrike)

	raw, err := json.Marshal(data.Pause)
	s.Require().NoError(err)
	s.NotContains(string(raw), "attacks_left")
	s.NotContains(string(raw), "bonus_actions")

	resumed := &pausingStriker{}
	host.Striker = resumed
	after := &scriptedDriver{intents: swings(2)}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        host,
			Participation: everyoneStanding{},
			Initiative:    arrivalOrder{},
			Driver:        after,
			Actors:        host.Actors(quietMover{}, false),
		},
	})
	s.Require().NoError(err)
	_, err = loaded.Resume(context.Background())
	s.Require().NoError(err)

	s.Equal(1, first.calls)
	s.Equal(0, resumed.calls, "the resumed creature is not asked to swing again that turn")
}

// --- 9 ----------------------------------------------------------------------

// TestAWalkThatPausesSavesItsMovementOnly: a walk paused mid-route saves its
// movement; what the host says is left changes between the pause and the
// resume, and the resumed view shows the movement as saved and the host's new
// numbers.
func (s *TurnFactsSuite) TestAWalkThatPausesSavesItsMovementOnly() {
	sheets := sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}
	host := turntest.New(sheets, &swingRecorder{}, nil)
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Move{Path: []spatial.Position{onRow(7), onRow(6), onRow(5)}},
	}}
	enc := s.build(turnScene{goblinAt: 8, sheets: host, driver: driver, actors: host.Actors(mover, false)})

	s.endTurn(enc, alice)
	s.Require().True(enc.Paused())
	data := enc.ToData()
	s.Equal(25, data.Pause.Turn.Budget.MovementFeet, "one cell was walked before the pause")
	raw, err := json.Marshal(data.Pause)
	s.Require().NoError(err)
	s.NotContains(string(raw), "attacks_left")
	s.NotContains(string(raw), "bonus_actions")

	host.Leave(goblin, 0, 0)
	after := &scriptedDriver{}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        host,
			Participation: everyoneStanding{},
			Initiative:    arrivalOrder{},
			Driver:        after,
			Actors:        host.Actors(&pausingMover{}, false),
		},
	})
	s.Require().NoError(err)
	_, err = loaded.Resume(context.Background())
	s.Require().NoError(err)

	s.Require().NotEmpty(after.calls)
	view := after.calls[0]
	s.Equal(15, view.Budget.MovementFeet, "movement as saved, less the two cells the resume walked")
	s.Equal(0, view.Budget.AttacksLeft, "the host's new numbers")
	s.Equal(0, view.Budget.BonusActions)
}

// --- 10 ---------------------------------------------------------------------

type negativeTurn struct {
	inner sheetFacts
	turn  encounter.TurnFacts
}

func (h negativeTurn) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out, err := h.inner.Sheets(members)
	if err != nil {
		return nil, err
	}
	for id, facts := range out {
		turn := h.turn
		facts.Turn = &turn
		out[id] = facts
	}

	return out, nil
}

// TestANegativeCountIsRefused: a host answering Turn with a negative number is
// ErrInvalidData.
func (s *TurnFactsSuite) TestANegativeCountIsRefused() {
	for name, turn := range map[string]encounter.TurnFacts{
		"actions":       {ActionsLeft: -1, BonusActionsLeft: 1},
		"bonus actions": {ActionsLeft: 1, BonusActionsLeft: -1},
	} {
		s.Run(name, func() {
			enc := s.build(turnScene{
				sheets: negativeTurn{inner: sheetFacts{alice: {SpeedFeet: 30}, goblin: goblinSheet()}, turn: turn},
				driver: &scriptedDriver{}, actors: plainActors(&swingRecorder{}),
			})

			_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})

			s.ErrorIs(err, encounter.ErrInvalidData)
		})
	}
}
