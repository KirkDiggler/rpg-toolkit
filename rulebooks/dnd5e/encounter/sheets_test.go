// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sheets_test.go pins the Sheets capability (rpg-project#538): a member's
// speed, actions and targeting are the sheet's, asked at the moment they are
// used, and this composition keeps no copy of them anywhere.

type SheetsTestSuite struct {
	suite.Suite
}

func TestSheetsSuite(t *testing.T) { suite.Run(t, new(SheetsTestSuite)) }

// glaive is a ten-foot reach: two cells, where the shortsword reaches one.
var glaive = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "glaive"}

// hallSetup is a long open room with alice in it, on the world clock — nobody
// opposed to her, so no fight forms and every step she takes is paced.
func hallSetup(sheets encounter.Sheets, extra ...encounter.MemberInput) *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
		},
		Members: append([]encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		}, extra...),
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     sheets,
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	}
}

// walkAlice steps alice cell by cell along row 1, starting from column from.
func (s *SheetsTestSuite) walkAlice(enc *encounter.Encounter, from, cells int) {
	s.T().Helper()
	for i := 1; i <= cells; i++ {
		_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(from+i, 1)})
		s.Require().NoError(err, "step %d", i)
	}
}

// blob is the encounter's persisted form as the host stores it.
func (s *SheetsTestSuite) blob(enc *encounter.Encounter) []byte {
	s.T().Helper()
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	return raw
}

// TestAWalkIsPacedAtTheSpeedTheSheetAnswersNow: the speed is asked at each
// step, so a sheet whose speed changed between two walks paces the second at
// the new speed — and nothing was written to the encounter for it to happen.
func (s *SheetsTestSuite) TestAWalkIsPacedAtTheSpeedTheSheetAnswersNow() {
	sheets := sheetFacts{alice: {SpeedFeet: 30}}
	enc, err := encounter.NewEncounter(hallSetup(sheets))
	s.Require().NoError(err)

	s.walkAlice(enc, 1, 6)
	s.Require().Equal(1, enc.ToData().Clock.HighWater, "precondition: six cells of thirty feet is one round")

	before := s.blob(enc)
	sheets[alice] = encounter.SheetFacts{SpeedFeet: 10}
	s.Equal(string(before), string(s.blob(enc)), "the sheet changed; the encounter did not, and needs not")

	s.walkAlice(enc, 7, 2)
	s.Equal(2, enc.ToData().Clock.HighWater,
		"two cells at ten feet is a round — a speed remembered from the first walk would want six")
}

// drivenPair is alice and the goblin two cells apart in one room: the fight
// forms at first light, alice acts first, and the goblin's turn is driven by
// the returned recorder. Nobody moves or swings — the driver only passes — so
// each of the goblin's turns sees alice at exactly the same distance.
func (s *SheetsTestSuite) drivenPair(sheets encounter.Sheets) (*encounter.Encounter, *scriptedDriver) {
	s.T().Helper()
	driver := &scriptedDriver{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 4, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     sheets,
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     driver,
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc, driver
}

// TestADrivenTurnReadsTheSheetOfThatMoment: a driven member's movement budget,
// its actions, its reach and its targeting are the sheet's answer at the start
// of that turn — the second turn after a weapon swap reaches with the new one.
func (s *SheetsTestSuite) TestADrivenTurnReadsTheSheetOfThatMoment() {
	sheets := sheetFacts{goblin: {SpeedFeet: 30, Targeting: "closest", Actions: []encounter.ActionView{
		{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
	}}, alice: {}}
	enc, driver := s.drivenPair(sheets)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(driver.calls, 1, "precondition: the goblin was driven once")
	first := driver.calls[0]
	s.Equal(30, first.Budget.MovementFeet)
	s.Equal("closest", first.Targeting)
	s.Require().Len(first.Seen, 1)
	s.False(first.Seen[0].InReach[testMeleeAction], "two cells away is past a shortsword's five feet")

	sheets[goblin] = encounter.SheetFacts{SpeedFeet: 10, Targeting: "lowest-health", Actions: []encounter.ActionView{
		{Ref: glaive, Name: "Glaive", RangeFeet: 10, Kind: "melee"},
	}}

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(driver.calls, 2, "precondition: the goblin was driven again")
	second := driver.calls[1]
	s.Equal(10, second.Budget.MovementFeet, "the budget is the sheet's speed at the start of this turn")
	s.Equal("lowest-health", second.Targeting)
	s.Require().Len(second.Actions, 1)
	s.Equal(glaive, second.Actions[0].Ref, "the view carries the weapon the sheet holds now")
	s.Require().Len(second.Seen, 1)
	s.True(second.Seen[0].InReach[glaive], "and alice is inside its ten feet")
}

// TestTheBlobCarriesNoSheetFacts: nothing the module writes names a speed, a
// sight range, an action or a targeting word — for a member on the board or
// one waiting in reserve.
func (s *SheetsTestSuite) TestTheBlobCarriesNoSheetFacts() {
	sheets := sheetFacts{
		alice:      {SpeedFeet: 30},
		"reserved": {SpeedFeet: 30, Targeting: "closest", Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}}},
	}
	enc, err := encounter.NewEncounter(hallSetup(sheets, encounter.MemberInput{
		ID: "reserved", Kind: encounter.KindMonster, Position: spatial.Position{X: 20, Y: 5},
		Arrives: encounter.TriggerRound{Round: 99},
	}))
	s.Require().NoError(err)

	var doc struct {
		Members []map[string]json.RawMessage `json:"members"`
		Reserve []map[string]json.RawMessage `json:"reserve"`
	}
	s.Require().NoError(json.Unmarshal(s.blob(enc), &doc))
	s.Require().NotEmpty(doc.Members, "precondition: a roster was written")
	s.Require().NotEmpty(doc.Reserve, "precondition: a reserve was written")

	for _, rows := range [][]map[string]json.RawMessage{doc.Members, doc.Reserve} {
		for _, row := range rows {
			for _, key := range []string{"speed_feet", "sight_feet", "actions", "targeting"} {
				s.NotContains(row, key, "%s carries %q", row["id"], key)
			}
		}
	}
}

// TestABlobCarryingTheOldCopiesLoadsAndTheSheetWins: a blob written before
// rpg-project#538 still carries the copies. It loads, the copies are ignored —
// the walk is paced by the sheet, not by the blob's number — and the next save
// drops them.
func (s *SheetsTestSuite) TestABlobCarryingTheOldCopiesLoadsAndTheSheetWins() {
	sheets := sheetFacts{alice: {SpeedFeet: 30}}
	enc, err := encounter.NewEncounter(hallSetup(sheets, encounter.MemberInput{
		ID: "reserved", Kind: encounter.KindMonster, Position: spatial.Position{X: 20, Y: 5},
		Arrives: encounter.TriggerRound{Round: 99},
	}))
	s.Require().NoError(err)

	// Write the old keys back in, as a pre-#538 build would have: alice a
	// five-foot walker, a round per cell, if anybody still read the copy.
	var doc map[string]any
	s.Require().NoError(json.Unmarshal(s.blob(enc), &doc))
	oldKeys := map[string]any{
		"speed_feet": 5, "sight_feet": 60, "targeting": "closest",
		"actions": []any{map[string]any{"ref": map[string]any{"module": "dnd5e", "type": "weapons", "id": "club"}, "range_feet": 5}},
	}
	for _, list := range []string{"members", "reserve"} {
		rows, ok := doc[list].([]any)
		s.Require().True(ok, "precondition: %s present", list)
		for _, row := range rows {
			for k, v := range oldKeys {
				row.(map[string]any)[k] = v
			}
		}
	}
	old, err := json.Marshal(doc)
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(old, &data))

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     sheets,
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err, "a blob carrying the old copies loads")

	s.walkAlice(loaded, 1, 5)
	s.Zero(loaded.ToData().Clock.HighWater, "five cells at the sheet's thirty feet is no round; the blob's five feet would be five")

	resaved := string(s.blob(loaded))
	for k := range oldKeys {
		s.NotContains(resaved, `"`+k+`"`, "the next save drops %q", k)
	}
}

// strangerSheets answers every member asked and one nobody asked about.
type strangerSheets struct{}

func (strangerSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := sheetFacts{}
	for _, id := range members {
		out[id] = encounter.SheetFacts{SpeedFeet: 30}
	}
	out["stranger"] = encounter.SheetFacts{SpeedFeet: 30}

	return out, nil
}

// skippingSheets answers every member asked except one.
type skippingSheets struct{ skip encounter.MemberID }

func (k skippingSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := sheetFacts{}
	for _, id := range members {
		if id == k.skip {
			continue
		}
		out[id] = encounter.SheetFacts{SpeedFeet: 30}
	}

	return out, nil
}

// failingSheets fails every ask.
type failingSheets struct{ err error }

func (f failingSheets) Sheets([]encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	return nil, f.err
}

// TestAnAnswerThatIsWrongAbortsTheVerb: a member missing from the answer, a
// stranger in it, a negative distance or the capability's own failure each
// fail the verb that asked, by name — never a speed or reach invented in its
// place. Each case is a walk on the world clock, which asks at every step.
func (s *SheetsTestSuite) TestAnAnswerThatIsWrongAbortsTheVerb() {
	broken := errors.New("sheet store unreachable")
	cases := []struct {
		name   string
		sheets encounter.Sheets
		want   error
	}{
		{"a member missing from the answer", skippingSheets{skip: alice}, encounter.ErrNoSheets},
		{"a stranger in the answer", strangerSheets{}, encounter.ErrNotMember},
		{"a negative speed", sheetFacts{alice: {SpeedFeet: -5}}, encounter.ErrInvalidData},
		{"a negative reach", sheetFacts{alice: {SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: -5}}}}, encounter.ErrInvalidData},
		{"the capability's own failure", failingSheets{err: broken}, broken},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			enc, err := encounter.NewEncounter(hallSetup(tc.sheets))
			s.Require().NoError(err, "nothing asks the sheets until somebody walks")

			_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 1)})
			s.Require().ErrorIs(err, tc.want)
		})
	}
}

// TestADrivenTurnWithAMissingSheetAbortsTheVerb: the same refusal from the
// driven turn — the verb that drove it fails rather than budgeting the turn at
// zero.
func (s *SheetsTestSuite) TestADrivenTurnWithAMissingSheetAbortsTheVerb() {
	enc, _ := s.drivenPair(skippingSheets{skip: goblin})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, encounter.ErrNoSheets)
}

// TestSetupAndLoadRefuseWithoutSheets: the capability is required at both
// doors and never defaulted.
func (s *SheetsTestSuite) TestSetupAndLoadRefuseWithoutSheets() {
	setup := hallSetup(zeroSheets{})
	built, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	setup.Sheets = nil
	_, err = encounter.NewEncounter(setup)
	s.Require().ErrorIs(err, encounter.ErrNoSheets)

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: built.ToData(),
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrNoSheets)
}

// TestARefusingWorldRefusesToPaceAMember: the stand-in a compiled world
// gets has no sheets behind anybody, so a walk in one is refused by name
// rather than paced at an invented speed — and a world with members still
// builds and loads, because neither asks.
func (s *SheetsTestSuite) TestARefusingWorldRefusesToPaceAMember() {
	setup := &encounter.SetupInput{Field: hallSetup(nil).Field, Endings: hallSetup(nil).Endings, Capabilities: encounter.RefusingCapabilities()}
	setup.Standing = everyoneStanding{}
	setup.Members = hallSetup(nil).Members
	built, err := encounter.NewEncounter(setup)
	s.Require().NoError(err, "building a world with a member asks no sheet")

	load := &encounter.LoadEncounterInput{Data: built.ToData(), Capabilities: encounter.RefusingCapabilities()}
	load.Standing = everyoneStanding{} // the one capability this test answers itself, so the walk reaches the pace
	loaded, err := encounter.LoadEncounter(load)
	s.Require().NoError(err, "nor does loading it")

	_, err = loaded.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 1)})
	s.Require().ErrorIs(err, encounter.ErrRefusingSheets)
}
