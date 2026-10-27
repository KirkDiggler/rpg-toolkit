// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// dash_test.go is the encounter's half of the dash (rpg-project#671, R17,
// R18): what a `dash` row costs, how far it goes, and the two facts it is
// judged by — what this turn's movement buys against the foe, and who else
// could reach the cell the run lands on.
//
// Everybody walks thirty feet (six cells) and the goblin swings at five feet
// (one cell), so the bands are the design's own arithmetic: a gap of 1 is
// reach, 2..7 a walk, 8..13 a dash, 14 and on far.

type DashSuite struct {
	suite.Suite
}

func TestDashSuite(t *testing.T) { suite.Run(t, new(DashSuite)) }

// dashSheets is thirty feet for everybody and the goblin's five-foot swing.
var dashSheets = sheetFacts{
	alice:  {SpeedFeet: 30},
	bob:    {SpeedFeet: 30},
	goblin: {SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}}},
}

// hall is a long open room, the members a test names, and a striker that
// records every swing. Seeing each other starts the fight, the members in
// the order given.
func (s *DashSuite) hall(striker *swingRecorder, members ...encounter.MemberInput) *encounter.Encounter {
	s.T().Helper()
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
			Sheets:        dashSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
			Roller:        rollsLowest{},
			Actors: encounter.Actors{
				Striker:   striker,
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

// goblinsTurn ends each player's turn until the goblin's has been driven,
// inside the call that ended the player's before it.
func (s *DashSuite) goblinsTurn(enc *encounter.Encounter) {
	s.T().Helper()
	for range 3 {
		on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
		s.Require().NoError(err)
		s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: seeing each other started the fight")
		_, err = enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
		s.Require().NoError(err)
		if len(s.beats(enc, encounter.BeatAnswered, goblin)) > 0 {
			return
		}
	}
	s.Require().Fail("the goblin never had its turn")
}

// beats is every story beat of one kind naming one creature, decoded, in
// order — under `member` or, for an `answered` pick, `creature`.
func (s *DashSuite) beats(enc *encounter.Encounter, kind string, who encounter.MemberID) []map[string]any {
	s.T().Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	var out []map[string]any
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind && (beat["member"] == string(who) || beat["creature"] == string(who)) {
			out = append(out, beat)
		}
	}

	return out
}

// verdict is row i's verdict on one `answered` beat.
func verdict(pick map[string]any, i int) map[string]any {
	return pick["rows"].([]any)[i].(map[string]any)
}

// onRow is a cell on the hall's first row, which is a straight hex line.
func onRow(x int) spatial.Position { return spatial.Position{X: float64(x), Y: 1} }

// enemyIs is a row that holds under one `enemy` word and stands.
func enemyIs(word encounter.EnemyWord) encounter.Answer {
	return encounter.Answer{Weight: 1, When: []encounter.When{{Enemy: word}}, Hold: true}
}

// dasher is the dasher bundle written out in Go: swing in reach, walk when a
// walk gets there, run when only a run does, otherwise stand.
func dasher() encounter.Table {
	enemy := &encounter.Selector{Word: encounter.SelectorEnemy}

	return encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}}, Attack: enemy},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyWalk}}, Toward: enemy},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyDash}}, Dash: enemy},
		{Weight: 1, Hold: true},
	}}
}

// TestTheMoveWordsHoldInTheirBands pins `enemy: walk | dash | far` at every
// boundary, reach included: speed six cells and reach one, so a walk of g-1
// cells closes a gap of g.
func (s *DashSuite) TestTheMoveWordsHoldInTheirBands() {
	bands := []encounter.EnemyWord{encounter.EnemyReach, encounter.EnemyWalk, encounter.EnemyDash, encounter.EnemyFar}
	for gap := 1; gap <= 15; gap++ {
		want := encounter.EnemyFar
		switch {
		case gap == 1:
			want = encounter.EnemyReach
		case gap <= 7:
			want = encounter.EnemyWalk
		case gap <= 13:
			want = encounter.EnemyDash
		}
		enc := s.hall(&swingRecorder{},
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: onRow(1)},
			encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: onRow(1 + gap),
				Table: encounter.Table{encounter.AnswerTime: {
					enemyIs(encounter.EnemyReach), enemyIs(encounter.EnemyWalk),
					enemyIs(encounter.EnemyDash), enemyIs(encounter.EnemyFar),
				}}},
		)
		s.goblinsTurn(enc)

		picks := s.beats(enc, encounter.BeatAnswered, goblin)
		s.Require().NotEmpty(picks, "gap %d: the goblin was asked", gap)
		for i, band := range bands {
			s.Equal(band == want, verdict(picks[0], i)["held"], "gap %d: `enemy: %s`", gap, band)
		}
	}
}

// TestADashRunsTwiceTheCellsAndDoesNotSwing: thirteen cells away, a walk
// cannot reach her and a run can. The goblin runs twelve cells — twice its
// six — lands beside her, and does not swing: the run was the swing.
func (s *DashSuite) TestADashRunsTwiceTheCellsAndDoesNotSwing() {
	striker := &swingRecorder{}
	enc := s.hall(striker,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: onRow(1)},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: onRow(14), Table: dasher()},
	)
	s.goblinsTurn(enc)

	s.Equal(onRow(2), whereIs(s.T(), enc, goblin), "it ran the whole gap")
	moved := s.beats(enc, encounter.BeatMoved, goblin)
	s.Require().Len(moved, 12, "twelve cells: twice what thirty feet walks")
	for _, beat := range moved {
		s.Equal("encounter:table:dash", beat["cause"], "every step of the run names it")
	}
	s.Empty(striker.swings, "it ran and did not swing")

	picks := s.beats(enc, encounter.BeatAnswered, goblin)
	s.Require().NotEmpty(picks)
	s.Equal("dash", picks[0]["word"], "the record names the row's own word")
	s.Equal("enemy", picks[0]["selector"])
	s.Equal(string(alice), picks[0]["target"])
	s.Equal(true, verdict(picks[0], 2)["held"], "the dash row held")
	s.Equal(false, verdict(picks[0], 1)["held"], "and the walk row did not")
}

// TestADashAfterTheSwingIsRefusedAndSaysSo: the goblin swings at alice beside
// it, then its table offers a run to the far end of the hall. The swing is
// spent, so the run is refused — told on the record and never walked.
func (s *DashSuite) TestADashAfterTheSwingIsRefusedAndSaysSo() {
	far := onRow(40)
	striker := &swingRecorder{}
	enc := s.hall(striker,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: onRow(1)},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: onRow(2),
			Table: encounter.Table{encounter.AnswerTime: {
				{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
					Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
				{Weight: 1, Dash: &encounter.Selector{At: &far}},
			}}},
	)
	s.goblinsTurn(enc)

	s.Require().Len(striker.swings, 1, "it swung first")
	s.Equal(onRow(2), whereIs(s.T(), enc, goblin), "and the run after it went nowhere")
	s.Empty(s.beats(enc, encounter.BeatMoved, goblin))

	stayed := s.beats(enc, encounter.BeatStayed, goblin)
	s.Require().Len(stayed, 1)
	s.Equal("encounter:table:dash", stayed[0]["cause"])
	s.Equal("its action is spent", stayed[0]["why"])

	picks := s.beats(enc, encounter.BeatAnswered, goblin)
	s.Require().Len(picks, 2)
	s.Equal("attack", picks[0]["word"])
	s.Equal("dash", picks[1]["word"])
}

// TestACarefulDasherRunsOnlyWhenNobodyElseCouldReachWhereItLands: the
// goblin's run at alice lands beside her. Bob is somewhere else in the hall,
// always farther from the goblin than she is, so she stays the target. Where
// bob could walk into reach of the landing cell the careful row stands and
// its verdict says `walk`; where only a dash would bring him it says `dash`;
// where he could not get there at all it runs, and says `none`. Alice is
// beside the landing in every scene and never counts: she is the row's own
// target.
func (s *DashSuite) TestACarefulDasherRunsOnlyWhenNobodyElseCouldReachWhereItLands() {
	enemy := &encounter.Selector{Word: encounter.SelectorEnemy}
	careful := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyDash}, {Threat: encounter.ThreatNone}}, Dash: enemy},
		{Weight: 1, Hold: true},
	}}
	// Alice at 10, the goblin at 23: a gap of 13, a run of twelve that lands
	// at 11.
	for _, tc := range []struct {
		name   string
		bob    int
		threat string
		ran    bool
	}{
		{name: "bob six from the landing walks into reach", bob: 5, threat: "walk", ran: false},
		{name: "bob eleven from the landing only dashes there", bob: 0, threat: "dash", ran: false},
		{name: "bob far down the hall cannot get there", bob: 50, threat: "none", ran: true},
	} {
		s.Run(tc.name, func() {
			enc := s.hall(&swingRecorder{},
				encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: onRow(10)},
				encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: onRow(23), Table: careful},
				encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: onRow(tc.bob)},
			)
			s.goblinsTurn(enc)

			picks := s.beats(enc, encounter.BeatAnswered, goblin)
			s.Require().NotEmpty(picks)
			s.Equal(tc.threat, verdict(picks[0], 0)["threat"], "the verdict records the judged threat")
			s.Equal(tc.ran, verdict(picks[0], 0)["held"])
			if tc.ran {
				s.Equal("dash", picks[0]["word"])
				s.Equal(onRow(11), whereIs(s.T(), enc, goblin), "it ran")

				return
			}
			s.Equal("hold", picks[0]["word"])
			s.Equal(onRow(23), whereIs(s.T(), enc, goblin), "it stood")
		})
	}
}

// TestADashOnTheWorldClockDoublesThePace: a goblin nobody is opposed to,
// told to go to the far end of the hall, covers six cells a round at a walk
// and twelve at a run. There is no swing to forfeit; the run is the pace.
func (s *DashSuite) TestADashOnTheWorldClockDoublesThePace() {
	far := spatial.Position{X: 28, Y: 6}
	for _, tc := range []struct {
		name  string
		row   encounter.Answer
		cells int
	}{
		{name: "a walk", row: encounter.Answer{Weight: 1, Toward: &encounter.Selector{At: &far}}, cells: 6},
		{name: "a run", row: encounter.Answer{Weight: 1, Dash: &encounter.Selector{At: &far}}, cells: 12},
	} {
		s.Run(tc.name, func() {
			world := new(WorldTimeSuite)
			world.SetT(s.T())
			enc := world.hallWith(
				[]encounter.DispositionInput{{
					Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
					Stance:  encounter.StanceNeutral,
				}},
				encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
				encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
					Position: spatial.Position{X: 2, Y: 6}, Table: encounter.Table{encounter.AnswerTime: {tc.row}}},
			)
			s.Require().NoError(encounter.ExportedSpendWorldAction(enc, alice))

			s.Len(s.beats(enc, encounter.BeatMoved, goblin), tc.cells,
				"%s covers %d cells in one round of the world", tc.name, tc.cells)
		})
	}
}
