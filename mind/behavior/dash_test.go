// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DashSuite is the `dash` word and the move words beside it (rpg-project#671,
// R17 and R18): a row that spends the turn running at its target, and the
// facts that say what a walk or a dash buys this turn.
type DashSuite struct {
	suite.Suite
}

func TestDashSuite(t *testing.T) { suite.Run(t, new(DashSuite)) }

func (s *DashSuite) pick(table behavior.Table, facts behavior.Facts) *behavior.PickOutput {
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	return out
}

// dasher is the dasher bundle's rows written out: swing in reach, walk when a
// walk reaches, run when only a run does, and stand when not even that.
func dasher() behavior.Table {
	enemy := func() *behavior.Selector { return &behavior.Selector{Word: behavior.SelectorEnemy} }

	return behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyReach}}, Attack: enemy()},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyWalk}}, Toward: enemy()},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyDash}}, Dash: enemy()},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyFar}}, Hold: true},
	}}
}

// TestDashTakesTheSelectorsTowardTakes: `dash` reads a word, a cell, a route
// and a sound the way `toward` does, and refuses what `toward` refuses, in
// the same sentence with its own name in it.
func (s *DashSuite) TestDashTakesTheSelectorsTowardTakes() {
	route := []spatial.Position{{X: 2, Y: 3}, {X: 6, Y: 3}}
	heard := []behavior.When{{Heard: &behavior.HeardWhen{Kind: lit("bell"), Within: 2}}}
	for name, row := range map[string]behavior.Answer{
		"dash: enemy":    {Weight: 1, Dash: &behavior.Selector{Word: behavior.SelectorEnemy}},
		"dash: attacker": {Weight: 1, Dash: &behavior.Selector{Word: behavior.SelectorAttacker}},
		"dash: { at }":   {Weight: 1, Dash: &behavior.Selector{At: &someCell}},
		"dash: { path }": {Weight: 1, Dash: &behavior.Selector{Path: route}},
		"dash: sound":    {Weight: 1, When: heard, Dash: &behavior.Selector{Word: behavior.SelectorSound}},
	} {
		s.NoError(row.Validate(), name)
	}

	for name, tc := range map[string]struct {
		sel  behavior.Selector
		says string
	}{
		"no target": {behavior.Selector{},
			"`dash` names no target; give it one of somebody, a cell (`at`) or a route (`path`): " +
				"behavior: selector cannot be read"},
		"a cell and a path": {behavior.Selector{At: &someCell, Path: route},
			"`dash` names a cell (`at`) and a route (`path`); a target is exactly one of " +
				"somebody, a cell or a route: behavior: selector cannot be read"},
		"a sound with nothing listened for": {behavior.Selector{Word: behavior.SelectorSound},
			"`sound` needs a `heard` condition on the row: the sound it walks to is the one the row " +
				"listened for: behavior: selector cannot be read"},
	} {
		toward := behavior.Answer{Weight: 1, Toward: &tc.sel}
		s.Require().ErrorIs(toward.Validate(), behavior.ErrBadSelector, "toward refuses %s", name)
		dash := behavior.Answer{Weight: 1, Dash: &tc.sel}
		err := dash.Validate()
		s.Require().ErrorIs(err, behavior.ErrBadSelector, name)
		s.EqualError(err, tc.says, name)
	}
}

// TestADasherPicksTheRunOnlyWhenOnlyARunReaches is R18 read by a table: the
// move words are facts the host computed, and the row whose `when` names the
// one that holds is the only candidate.
func (s *DashSuite) TestADasherPicksTheRunOnlyWhenOnlyARunReaches() {
	seen := []core.EntityID{"fighter"}
	for _, tc := range []struct {
		name  string
		facts behavior.Facts
		entry int
	}{
		{name: "in reach", entry: 0, facts: behavior.Facts{CanAttack: true, CanMove: true, CanDash: true,
			EnemyInReach: true, SeenFoes: seen, FoesInReach: seen}},
		{name: "a walk reaches", entry: 1, facts: behavior.Facts{CanAttack: true, CanMove: true, CanDash: true,
			EnemySeen: true, EnemyWalk: true, SeenFoes: seen}},
		{name: "only a dash reaches", entry: 2, facts: behavior.Facts{CanAttack: true, CanMove: true, CanDash: true,
			EnemySeen: true, EnemyDash: true, SeenFoes: seen}},
		{name: "remembered, only a dash reaches", entry: 2, facts: behavior.Facts{CanAttack: true, CanMove: true,
			CanDash:         true,
			EnemyRemembered: true, EnemyDash: true, RememberedFoes: seen}},
		{name: "not even a dash reaches", entry: 3, facts: behavior.Facts{CanAttack: true, CanMove: true, CanDash: true,
			EnemySeen: true, EnemyFar: true, SeenFoes: seen}},
	} {
		s.Run(tc.name, func() {
			out := s.pick(dasher(), tc.facts)
			s.Require().Len(out.Candidates, 1, "the move words are exclusive, so one row is on the table")
			s.Equal(tc.entry, out.Entry)
		})
	}

	out := s.pick(dasher(), behavior.Facts{CanAttack: true, CanMove: true, CanDash: true,
		EnemySeen: true, EnemyDash: true, SeenFoes: seen})
	s.Require().NotNil(out.Answer.Dash)
	s.Equal(behavior.SelectorEnemy, out.Answer.Dash.Word, "the row that fired carries its own word")

	unset := s.pick(dasher(), behavior.Facts{CanAttack: true, CanMove: true, CanDash: true,
		EnemySeen: true, SeenFoes: seen})
	s.Equal(-1, unset.Entry, "a host that computed no move word holds none of them: nothing is on the table")
}

// TestADashAtAFoeInReachIsAbsent is the engine's one floor (R17): a run at
// somebody the creature could strike is off the table, not a pass, so the
// author's sibling row wins the roll. Every other threshold is the row's own
// `when`.
func (s *DashSuite) TestADashAtAFoeInReachIsAbsent() {
	enemyRun := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Dash: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
	seen := []core.EntityID{"fighter"}
	inReach := s.pick(enemyRun, behavior.Facts{CanMove: true, CanDash: true,
		EnemyInReach: true, SeenFoes: seen, FoesInReach: seen})
	s.False(inReach.Rows[0].Affordable, "a dash at an enemy in reach is absent")
	s.Equal(1, inReach.Entry, "the sibling row wins")

	outOfReach := s.pick(enemyRun, behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, SeenFoes: seen})
	s.True(outOfReach.Rows[0].Affordable)
	s.Equal(0, outOfReach.Entry)

	legless := s.pick(enemyRun, behavior.Facts{CanAttack: true, EnemySeen: true, SeenFoes: seen})
	s.False(legless.Rows[0].Affordable, "a dash is a walk, and a creature with no movement cannot pay for it")

	attackerRun := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "attacked", Within: 2}},
			Dash: &behavior.Selector{Word: behavior.SelectorAttacker}},
		{Weight: 1, Hold: true},
	}}
	hit := []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "bard", At: 9}}
	for name, tc := range map[string]struct {
		facts     behavior.Facts
		candidate bool
	}{
		"the bard in reach": {behavior.Facts{CanMove: true, CanDash: true, Now: 10, Deeds: hit, EnemyInReach: true,
			SeenFoes: []core.EntityID{"bard"}, FoesInReach: []core.EntityID{"bard"}}, false},
		"the bard in sight, the fighter in reach": {behavior.Facts{CanMove: true, CanDash: true, Now: 10, Deeds: hit,
			EnemyInReach: true, SeenFoes: []core.EntityID{"fighter", "bard"},
			FoesInReach: []core.EntityID{"fighter"}}, true},
		"the bard remembered": {behavior.Facts{CanMove: true, CanDash: true, Now: 10, Deeds: hit,
			RememberedFoes: []core.EntityID{"bard"}}, true},
		"the bard held nowhere": {behavior.Facts{CanMove: true, CanDash: true, Now: 10, Deeds: hit}, false},
	} {
		out := s.pick(attackerRun, tc.facts)
		s.Equal(tc.candidate, out.Rows[0].Affordable, name)
	}

	cell := behavior.Table{behavior.KeyTime: {{Weight: 1, Dash: &behavior.Selector{At: &someCell}}}}
	s.Equal(0, s.pick(cell, behavior.Facts{CanMove: true, CanDash: true, EnemyInReach: true}).Entry,
		"a cell is a place, not a foe: a foe in reach does not take a run to it off the table")
}

// TestADashIsLoadedAsAWalkToward: a run is the creature closing, so it
// carries the `toward` factor, as `search` does.
func (s *DashSuite) TestADashIsLoadedAsAWalkToward() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Dash: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Die: &facedDie{face: 1},
		Facts:  behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true},
		Temper: behavior.Temper{Word: "coward", Profile: cowardProfile}})
	s.Require().NoError(err)
	s.Require().Len(out.Candidates, 2)
	s.Equal(cowardProfile.Toward, out.Candidates[0].Percent)
}

// TestTheMoveWordsAreRead: `walk`, `dash` and `far` each hold off their own
// fact, the sealed list carries them after the bands, and a word this build
// does not read still holds nothing.
func (s *DashSuite) TestTheMoveWordsAreRead() {
	s.Equal(behavior.EnemyWord("walk"), behavior.EnemyWalk)
	s.Equal(behavior.EnemyWord("dash"), behavior.EnemyDash)
	s.Equal(behavior.EnemyWord("far"), behavior.EnemyFar)
	for word, facts := range map[behavior.EnemyWord]behavior.Facts{
		behavior.EnemyWalk: {EnemyWalk: true},
		behavior.EnemyDash: {EnemyDash: true},
		behavior.EnemyFar:  {EnemyFar: true},
	} {
		table := behavior.Table{behavior.KeyTime: {{Weight: 1, When: []behavior.When{{Enemy: word}}, Hold: true}}}
		s.Equal(0, s.pick(table, facts).Entry, string(word))
		s.Equal(-1, s.pick(table, behavior.Facts{EnemySeen: true}).Entry, "%s needs its own fact", word)
	}

	unknown := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: "sprint"}}, Hold: true},
	}}
	s.Equal(-1, s.pick(unknown, behavior.Facts{EnemyWalk: true, EnemyDash: true, EnemyFar: true}).Entry,
		"a word this build does not read holds nothing")
}

// TestTheRecordReadsDash is the persisted row: a dash rides the row's JSON
// under its own name and comes back unchanged, and a row that does not dash
// marshals exactly as it did before the word existed.
func (s *DashSuite) TestTheRecordReadsDash() {
	row := behavior.Answer{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyDash}},
		Dash: &behavior.Selector{Word: behavior.SelectorEnemy}}
	raw, err := json.Marshal(row)
	s.Require().NoError(err)
	s.Contains(string(raw), `"Dash":{"Word":"enemy","At":null}`)
	s.Contains(string(raw), `"Enemy":"dash"`)
	var back behavior.Answer
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Equal(row, back)

	plain, err := json.Marshal(behavior.Answer{Weight: 1, Toward: &behavior.Selector{Word: behavior.SelectorEnemy}})
	s.Require().NoError(err)
	s.NotContains(string(plain), "Dash", "a row that does not dash marshals as before")

	verdict := s.pick(dasher(), behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true})
	pick, err := json.Marshal(verdict)
	s.Require().NoError(err)
	s.Contains(string(pick), `"Answer":{"Weight":1,"Say":"","When":[{"Enemy":"dash","Deed":"","Within":0}]`)
	s.Contains(string(pick), `"Dash":{"Word":"enemy","At":null}`, "the record reads the row's own word")
}

// TestAGroupCarriesNoDash: `dash` is a word, so a group that carries one is
// refused like a group carrying `toward`.
func (s *DashSuite) TestAGroupCarriesNoDash() {
	group := behavior.Answer{Weight: 1, Dash: &behavior.Selector{Word: behavior.SelectorEnemy},
		Rows: []behavior.Answer{{Weight: 1, Hold: true}}}
	err := group.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadGroup)
	s.Contains(err.Error(), "`dash`")
}

// careful is the careful dasher (R18): run only when only a run reaches, and
// only when nobody else could reach where it lands; otherwise stand.
func careful() behavior.Table {
	return behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyDash}, {Threat: behavior.ThreatNone}},
			Dash: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
}

// threatFor is a host that judges every row's landing cell the same way.
func threatFor(word behavior.ThreatWord) func(behavior.Answer) behavior.ThreatWord {
	return func(behavior.Answer) behavior.ThreatWord { return word }
}

// TestTheCarefulDasherRunsOnlyWhenBothHold: the run is on the table only
// while a dash is what reaches AND nobody else threatens the landing cell.
func (s *DashSuite) TestTheCarefulDasherRunsOnlyWhenBothHold() {
	for _, tc := range []struct {
		name   string
		facts  behavior.Facts
		dashes bool
	}{
		{name: "only a dash reaches, nobody else threatens", dashes: true, facts: behavior.Facts{
			CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true, Threat: threatFor(behavior.ThreatNone)}},
		{name: "only a dash reaches, a walker threatens", facts: behavior.Facts{
			CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true, Threat: threatFor(behavior.ThreatWalk)}},
		{name: "only a dash reaches, a dasher threatens", facts: behavior.Facts{
			CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true, Threat: threatFor(behavior.ThreatDash)}},
		{name: "a walk reaches, nobody threatens", facts: behavior.Facts{
			CanMove: true, CanDash: true, EnemySeen: true, EnemyWalk: true, Threat: threatFor(behavior.ThreatNone)}},
		{name: "the host judged no threat", facts: behavior.Facts{
			CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true}},
		{name: "the host answered nothing", facts: behavior.Facts{
			CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true, Threat: threatFor("")}},
	} {
		s.Run(tc.name, func() {
			out := s.pick(careful(), tc.facts)
			s.Equal(tc.dashes, out.Rows[0].Held)
			if tc.dashes {
				s.Equal(0, out.Entry)

				return
			}
			s.Equal(1, out.Entry, "the careful dasher stands")
		})
	}
}

// TestAThreatIsJudgedAtEachRowsOwnLanding: the host is asked with the row
// being judged, so a walk and a run in one pick can read different threats,
// and a row whose `when` names no threat is never asked about.
func (s *DashSuite) TestAThreatIsJudgedAtEachRowsOwnLanding() {
	enemy := &behavior.Selector{Word: behavior.SelectorEnemy}
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Threat: behavior.ThreatNone}}, Toward: enemy},
		{Weight: 1, When: []behavior.When{{Threat: behavior.ThreatNone}}, Dash: enemy},
		{Weight: 1, When: []behavior.When{{Not: &behavior.When{Threat: behavior.ThreatWalk}}}, Hold: true},
		{Weight: 1, Away: enemy},
	}}
	var asked []string
	facts := behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true,
		Threat: func(row behavior.Answer) behavior.ThreatWord {
			switch {
			case row.Toward != nil:
				asked = append(asked, "toward")
				return behavior.ThreatWalk // the walk lands beside the archer
			case row.Dash != nil:
				asked = append(asked, "dash")
				return behavior.ThreatNone
			case row.Away != nil:
				asked = append(asked, "away")
			default:
				asked = append(asked, "stand")
			}

			return behavior.ThreatNone
		}}
	out := s.pick(table, facts)
	s.Equal([]string{"toward", "dash", "stand"}, asked, "each threat row asked once, the away row never")
	s.False(out.Rows[0].Held, "the walk lands threatened")
	s.True(out.Rows[1].Held, "the run lands clear")
	s.True(out.Rows[2].Held, "`not` reads the threat too: where it stands, no walker reaches")
}

// TestAThreatIsOneOfThreeWords: none, walk and dash are read; anything else,
// inside a `not` included, is refused by name, and a When naming a threat
// and an enemy word is two conditions, refused as every pair is: the list is
// the `and`.
func (s *DashSuite) TestAThreatIsOneOfThreeWords() {
	s.Equal([]behavior.ThreatWord{behavior.ThreatNone, behavior.ThreatWalk, behavior.ThreatDash},
		behavior.ThreatWords)
	for _, w := range behavior.ThreatWords {
		s.NoError((&behavior.When{Threat: w}).Validate(), string(w))
	}
	err := (&behavior.When{Threat: "far"}).Validate()
	s.Require().ErrorIs(err, behavior.ErrBadWhen)
	s.EqualError(err, "`threat: far`: a threat is none, walk or dash: behavior: when cannot be read")

	err = (&behavior.Answer{Weight: 1, Hold: true,
		When: []behavior.When{{Not: &behavior.When{Threat: "lots"}}}}).Validate()
	s.Require().ErrorIs(err, behavior.ErrBadWhen)
	s.Contains(err.Error(), "`threat: lots`")

	err = (&behavior.When{Enemy: behavior.EnemyDash, Threat: behavior.ThreatNone}).Validate()
	s.Require().ErrorIs(err, behavior.ErrBadWhen)
	s.Contains(err.Error(), "names `enemy` and `threat`")

	s.NoError((&behavior.Answer{Weight: 1, Dash: &behavior.Selector{Word: behavior.SelectorEnemy},
		When: []behavior.When{{Enemy: behavior.EnemyDash}, {Threat: behavior.ThreatNone}}}).Validate(),
		"the careful dasher's list is the `and`")

	unsealed := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Threat: "far"}}, Hold: true}}}
	s.Equal(-1, s.pick(unsealed, behavior.Facts{Threat: threatFor("far")}).Entry,
		"an unsealed threat holds nothing, even when the host echoes it")
}

// TestTheRecordReadsThreat: the threat condition rides the row's JSON by its
// own name, the host's function never does, and a condition with no threat
// marshals as before.
func (s *DashSuite) TestTheRecordReadsThreat() {
	row := careful()[behavior.KeyTime][0]
	raw, err := json.Marshal(row)
	s.Require().NoError(err)
	s.Contains(string(raw), `{"Enemy":"","Deed":"","Within":0,"Threat":"none"}`)
	var back behavior.Answer
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Equal(row, back)

	plain, err := json.Marshal(behavior.When{Enemy: behavior.EnemyDash})
	s.Require().NoError(err)
	s.JSONEq(`{"Enemy":"dash","Deed":"","Within":0}`, string(plain), "no threat, no key")

	pick := s.pick(careful(), behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true,
		Threat: threatFor(behavior.ThreatNone)})
	out, err := json.Marshal(pick)
	s.Require().NoError(err)
	s.Contains(string(out), `"Threat":"none"`, "the record carries the row's threat condition")
}

// TestTheRecklessDasherRunsWhateverTheThreat: drop the threat condition and
// the run is on the table whenever only a run reaches, threatened or not,
// and the host is never asked.
func (s *DashSuite) TestTheRecklessDasherRunsWhateverTheThreat() {
	reckless := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyDash}},
			Dash: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
	for _, word := range []behavior.ThreatWord{behavior.ThreatNone, behavior.ThreatWalk, behavior.ThreatDash, ""} {
		asked := false
		out := s.pick(reckless, behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true,
			Threat: func(behavior.Answer) behavior.ThreatWord { asked = true; return word }})
		s.True(out.Rows[0].Held, "threat %q", word)
		s.True(out.Rows[0].Affordable, "threat %q", word)
		s.Equal(0, out.Entry, "the reckless dasher takes the first face under threat %q", word)
		s.False(asked, "a row that names no threat is never asked about one")
	}
	out := s.pick(reckless, behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true})
	s.Equal(0, out.Entry, "with no threat judged at all")
}

// TestTheVerdictRecordsTheJudgedThreat: the record carries the word the host
// answered for a row that asked, so a careful dasher that stood says why,
// and a row that never asked carries no threat key at all.
func (s *DashSuite) TestTheVerdictRecordsTheJudgedThreat() {
	out := s.pick(careful(), behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true,
		Threat: threatFor(behavior.ThreatWalk)})
	s.Equal(1, out.Entry, "the careful dasher stands")
	s.Equal(behavior.ThreatWalk, out.Rows[0].Threat)
	s.Empty(out.Rows[1].Threat, "the hold row never asked")

	raw, err := json.Marshal(out.Rows)
	s.Require().NoError(err)
	var rows []map[string]any
	s.Require().NoError(json.Unmarshal(raw, &rows))
	s.Equal("walk", rows[0]["Threat"], "the record carries the judged word")
	s.NotContains(rows[1], "Threat", "a row never asked carries no key")

	unjudged := s.pick(careful(), behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, EnemyDash: true,
		Threat: threatFor("")})
	raw, err = json.Marshal(unjudged.Rows[0])
	s.Require().NoError(err)
	s.NotContains(string(raw), "Threat", "an empty answer reads as absent, not as none")
}

// TestADashNeedsItsAction: a run spends the creature's action, so with
// CanDash false the `dash` row is ABSENT — not a candidate, never picked and
// refused — while the `toward` row beside it, which spends only movement, is
// still on the table. The zero value is "cannot".
func (s *DashSuite) TestADashNeedsItsAction() {
	enemy := &behavior.Selector{Word: behavior.SelectorEnemy}
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Dash: enemy},
		{Weight: 1, Toward: enemy},
	}}
	seen := []core.EntityID{"bard"}

	spent := s.pick(table, behavior.Facts{CanMove: true, EnemySeen: true, SeenFoes: seen})
	s.False(spent.Rows[0].Affordable, "with the action spent the run is not affordable")
	s.True(spent.Rows[1].Affordable, "and the walk beside it still is")
	s.Require().Len(spent.Candidates, 1, "the run is absent, not a candidate")
	s.Equal(1, spent.Entry, "so the walk is what the roll lands on")

	unspent := s.pick(table, behavior.Facts{CanMove: true, CanDash: true, EnemySeen: true, SeenFoes: seen})
	s.True(unspent.Rows[0].Affordable)
	s.Len(unspent.Candidates, 2)
	s.Equal(0, unspent.Entry, "with the action to spend, the run is the first candidate")

	noMove := s.pick(table, behavior.Facts{CanDash: true, EnemySeen: true, SeenFoes: seen})
	s.False(noMove.Rows[0].Affordable, "and a run still needs movement")
}
