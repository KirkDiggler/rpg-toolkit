// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// someCell is a cell these tests name when a selector has to carry one; which
// cell it is never matters, only that it is one. This module carries a
// position and never reads it, which is the whole of what the test needs.
var someCell = spatial.Position{X: 3, Y: 4}

// lit is a literal [behavior.Name]: the plain string a `fact` or a `heard`
// kind was before patterns existed, and still is when no pattern is written.
func lit(name string) behavior.Name { return behavior.Name{Literal: name} }

// table_test.go is THE CREATURE'S TABLE, pinned (rpg-project#465): the
// layering, the conditions, the loaded die, and the deal.
//
// EVERY CLAIM HERE IS A UNIT TEST WITH A SEEDED DIE, which is the rule this
// slice was briefed under: behaviour is proven by unit tests, and a
// probability is a unit test with a die you control. The rulebook's own scene
// tests prove the WIRING — that a verb pays a round, that a cowed goblin runs
// — and say nothing about what a coward does over a thousand rolls, which is
// the one thing a walk could never show.
//
// THEY RUN WITH NO BOARD, NO CLOCK AND NO RULEBOOK, which is the claim the
// move itself makes: the evaluator needs a table, a temperament, some facts
// and a die, and that is all it has ever needed.

// --- the dice these tests roll -----------------------------------------------

// facedDie answers a face the test named, and records the die it was asked
// for so a test can assert the loaded total without reading it off the beat.
type facedDie struct {
	face int
	of   int
}

func (r *facedDie) Roll(_ context.Context, size int) (int, error) {
	r.of = size
	if r.face > size {
		return size, nil
	}

	return r.face, nil
}

func (r *facedDie) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		face, err := r.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		out[i] = face
	}

	return out, nil
}

// seededDie is a real distribution from a fixed seed: the only honest way
// to claim "a coward runs more often than a soldier does" is to roll it many
// times, and the only way to claim it REPEATABLY is to say which seed.
type seededDie struct{ r *rand.Rand }

func newSeededDie(seed int64) *seededDie {
	return &seededDie{r: rand.New(rand.NewSource(seed))} //nolint:gosec // a test's die, not a secret
}

func (s *seededDie) Roll(_ context.Context, size int) (int, error) {
	return s.r.Intn(size) + 1, nil
}

func (s *seededDie) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		face, err := s.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		out[i] = face
	}

	return out, nil
}

// --- the temperaments, as the rulebook ships them ----------------------------

// The three profiles, in percent — the numbers rulebooks/dnd5e holds as
// content, restated here so this module's own arithmetic can be pinned without
// importing a rulebook it is not allowed to import (C1). A test in the session
// module is where the two lists are checked against each other.
var (
	soldierProfile    = behavior.TemperProfile{Attack: 100, Toward: 100, Away: 100, Flee: 100, Hold: 100}
	cowardProfile     = behavior.TemperProfile{Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100}
	aggressiveProfile = behavior.TemperProfile{Attack: 300, Toward: 300, Away: 25, Flee: 25, Hold: 50}
)

type TableSuite struct {
	suite.Suite
}

func TestTableSuite(t *testing.T) { suite.Run(t, new(TableSuite)) }

// canAct is a creature with a whole turn in front of it: one attack and some
// movement. AFFORDABILITY IS ELIGIBILITY, and the zero value is "cannot" — so
// a fixture about what a table CHOOSES has to say the creature could pay for
// any of it, or it is quietly a fixture about a creature with nothing left.
var canAct = behavior.Facts{CanAttack: true, CanMove: true}

// anEventKey and anotherEventKey stand for whatever a rulebook calls the
// events its own verbs settle. THIS MODULE NAMES ONLY [behavior.KeyTime]: a
// creature having time is the one trigger every game has, and the rest are
// the caller's to name, to seal and to refuse.
const (
	anEventKey      = behavior.AnswerKey("intimidated")
	anotherEventKey = behavior.AnswerKey("persuaded")
)

// --- Layer -------------------------------------------------------------------

// TestLayerTakesTheNearestKeyWholesale is design §1: three layers supply
// tables and for each key the nearest one that names it wins ENTIRELY. There
// is no merging of entry lists, so an author never has to reason about what
// was added to what — and the cost of overriding a key is that the layer
// underneath's entries for it are gone, visibly.
func (s *TableSuite) TestLayerTakesTheNearestKeyWholesale() {
	base := behavior.Table{
		behavior.KeyTime: {
			{Weight: 3, When: []behavior.When{{Enemy: behavior.EnemySeen}},
				Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
			{Weight: 1, Hold: true},
		},
		anEventKey: {{Weight: 1, Say: "the rulebook's line"}},
	}
	over := behavior.Table{behavior.KeyTime: {{Weight: 1, Hold: true}}}

	laid := behavior.Layer(base, over)

	s.Require().Len(laid[behavior.KeyTime], 1,
		"the nearer layer's `time` key replaced the default's two entries entirely")
	s.True(laid[behavior.KeyTime][0].Hold)
	s.Equal(base[anEventKey], laid[anEventKey],
		"a key the nearer layer is silent about is the further layer's, untouched")
}

// TestLayerMutatesNeitherInput is why a rulebook's default table can be one
// value shared by every creature of its kind: a layering that wrote through to
// it would give one goblin's orders to every goblin in the game.
func (s *TableSuite) TestLayerMutatesNeitherInput() {
	base := behavior.Table{behavior.KeyTime: {{Weight: 1, Hold: true}}}
	over := behavior.Table{anEventKey: {{Weight: 1, Say: "hi"}}}

	laid := behavior.Layer(base, over)
	laid[behavior.KeyTime][0].Weight = 99
	laid[anEventKey] = nil
	laid[anotherEventKey] = []behavior.Answer{{Weight: 1}}

	s.Equal(1, base[behavior.KeyTime][0].Weight, "the base's own entry is unchanged")
	s.Len(base, 1, "and nothing was added to it")
	s.Len(over, 1, "nor to the layer laid over it")
	s.NotEmpty(over[anEventKey])
}

// TestLayerOnNothingIsNothing: a creature with no table anywhere answers
// nothing and rolls nothing, and nil is how that is said.
func (s *TableSuite) TestLayerOnNothingIsNothing() {
	s.Nil(behavior.Layer(nil, nil))
	s.Nil(behavior.Layer(behavior.Table{}, behavior.Table{}))

	over := behavior.Table{behavior.KeyTime: {{Weight: 1, Hold: true}}}
	s.Equal(over, behavior.Layer(nil, over), "an empty base is the layer laid over it")
}

// --- when --------------------------------------------------------------------

// TestTheFourEnemyBandsAreExclusive pins the whole ladder, one entry per band:
// EXACTLY ONE is on the table at any moment, which is what lets an author
// write one row for each and know which fires.
//
// `reach` IS THE BAND THAT MADE THE DEFAULT WORK (ruled on a session-builder
// finding). `enemy: seen` used to mean "I can see them", so the shipped thug
// swung at somebody across the room and passed, every round, forever. It means
// "I can see them and cannot touch them" now, and `reach` is where the swing
// belongs.
//
// `lost` IS THE FIFTH BAND (rpg-project#668 R3): the decider has been to the
// remembered cell and found nobody. Without it a creature standing on its own
// stale memory reads `remembered` forever and has nowhere left to walk.
func (s *TableSuite) TestTheFiveEnemyBandsAreExclusive() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyReach}},
			Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemySeen}},
			Toward: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyRemembered}},
			Toward: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyLost}},
			Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingWent}},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyNone}}, Hold: true},
	}}

	for _, tc := range []struct {
		name  string
		facts behavior.Facts
		entry int
	}{
		{name: "within reach", facts: behavior.Facts{EnemyInReach: true, CanAttack: true, CanMove: true}, entry: 0},
		{name: "in sight, out of reach", facts: behavior.Facts{EnemySeen: true, CanAttack: true, CanMove: true}, entry: 1},
		{name: "lost sight of", facts: behavior.Facts{EnemyRemembered: true, CanAttack: true, CanMove: true}, entry: 2},
		{name: "searched the last place", facts: behavior.Facts{EnemyLost: true, CanAttack: true, CanMove: true}, entry: 3},
		{name: "never seen", facts: behavior.Facts{CanAttack: true, CanMove: true}, entry: 4},
	} {
		s.Run(tc.name, func() {
			roller := &facedDie{face: 1}
			chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
				Key: behavior.KeyTime, Table: table, Facts: tc.facts, Die: roller})
			s.Require().NoError(err)
			s.Require().NotNil(chosen)
			s.Require().Len(chosen.Candidates, 1,
				"exactly one band holds, so exactly one entry is on the table")
			s.Equal(tc.entry, chosen.Entry)
		})
	}
}

// TestASpanIsCountedFromOneAndEndsWhenItSaysSo is the `within` boundary, both
// sides of it. A deed landed N units ago still counts; N+1 does not.
//
// THE UNIT IS THE CALLER'S. This module subtracts a deed's At from
// [behavior.Facts.Now] and compares; whether that difference is rounds, turns
// or days is a question it never asks.
func (s *TableSuite) TestASpanIsCountedFromOneAndEndsWhenItSaysSo() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "fled", Within: 3}},
			Away: &behavior.Selector{Word: behavior.SelectorActor}},
		{Weight: 1, Hold: true},
	}}

	for _, tc := range []struct {
		name  string
		now   uint64
		entry int
	}{
		{name: "the unit it landed on", now: 10, entry: 0},
		{name: "one later", now: 11, entry: 0},
		{name: "three later, the last that counts", now: 13, entry: 0},
		{name: "four later, the span has run out", now: 14, entry: 1},
	} {
		s.Run(tc.name, func() {
			facts := behavior.Facts{
				Now:     tc.now,
				CanMove: true,
				Deeds:   []behavior.HeldDeed{{Kind: behavior.VerbFled, Actor: "alice", At: 10}},
			}
			chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
				Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
			s.Require().NoError(err)
			s.Require().NotNil(chosen)
			s.Equal(tc.entry, chosen.Entry)
		})
	}
}

// TestADeedOfAnotherKindIsNotThisCondition: the conditions are per verb, which
// is why `fled` and `attacked` are separate words. A creature that was
// attacked is not a creature that ran.
func (s *TableSuite) TestADeedOfAnotherKindIsNotThisCondition() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "fled", Within: 3}},
			Away: &behavior.Selector{Word: behavior.SelectorActor}},
		{Weight: 1, Hold: true},
	}}
	facts := behavior.Facts{
		Now:     11,
		CanMove: true,
		Deeds:   []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "alice", At: 10}},
	}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.Equal(1, chosen.Entry, "the `fled` condition does not read an `attack` deed")
}

// TestTheAuthorsPastTenseNamesTheStoresVerb is the one place the two
// vocabularies meet. A condition is written from the CREATURE's side ("I was
// attacked"); a deed is recorded from the WITNESS's ("somebody attacked").
func (s *TableSuite) TestTheAuthorsPastTenseNamesTheStoresVerb() {
	s.Equal([]string{"attacked", "intimidated", "persuaded", "fled"}, behavior.WhenDeeds)

	s.Equal(behavior.VerbAttack, behavior.DeedVerbFor("attacked"))
	s.Equal(behavior.VerbIntimidate, behavior.DeedVerbFor("intimidated"))
	s.Equal(behavior.VerbPersuade, behavior.DeedVerbFor("persuaded"))
	s.Equal(behavior.VerbFled, behavior.DeedVerbFor("fled"))

	s.Empty(behavior.DeedVerbFor("insulted"),
		"a word nobody mapped matches no held deed, and the refusal for one lives at the caller's own door")
	s.Empty(behavior.DeedVerbFor("attack"),
		"and the store's own verb is not a condition word — that is the whole point of the mapping")
}

// --- affordability -----------------------------------------------------------

// TestWhatACreatureCannotPayForIsNotOnTheTable is the api builder's finding,
// ruled: affordability is part of eligibility, the same way a `when` is.
//
// A PICK THAT CANNOT ACT IS NOISE ON THE LOG. A caller that asks again after a
// swing was handed `attack` a second time, published a second identical
// account of a roll, and watched nothing happen. An entry the creature cannot
// pay for is simply not a candidate.
func (s *TableSuite) TestWhatACreatureCannotPayForIsNotOnTheTable() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Toward: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Away: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}

	for _, tc := range []struct {
		name    string
		facts   behavior.Facts
		entries []int
	}{
		{
			name:    "a whole turn in front of it",
			facts:   behavior.Facts{CanAttack: true, CanMove: true},
			entries: []int{0, 1, 2, 3},
		},
		{
			name:    "the swing is spent, so the walks and the hold are left",
			facts:   behavior.Facts{CanMove: true},
			entries: []int{1, 2, 3},
		},
		{
			name:    "the movement is spent, so the swing and the hold are left",
			facts:   behavior.Facts{CanAttack: true},
			entries: []int{0, 3},
		},
		{
			name:    "nothing left to spend, and holding still costs nothing",
			facts:   behavior.Facts{},
			entries: []int{3},
		},
	} {
		s.Run(tc.name, func() {
			chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
				Key: behavior.KeyTime, Table: table, Facts: tc.facts, Die: &facedDie{face: 1}})
			s.Require().NoError(err)
			s.Require().NotNil(chosen)

			var on []int
			for _, c := range chosen.Candidates {
				on = append(on, c.Entry)
			}
			s.Equal(tc.entries, on, "exactly what the creature could have paid for")
		})
	}
}

// TestNothingAffordableIsAHoldWithNoCandidates: a table whose every entry
// costs something the creature has spent answers the same shape as a table
// whose every condition is false — the creature had its turn and did nothing,
// and the caller is told so rather than left to infer it from silence.
func (s *TableSuite) TestNothingAffordableIsAHoldWithNoCandidates() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Toward: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.True(chosen.Answer.Hold)
	s.Empty(chosen.Candidates)
	s.Equal(-1, chosen.Entry)
	s.Zero(chosen.Roll, "and nothing was rolled, because there was nothing to roll on")
}

// TestHoldIsAffordableWhateverTheBudgetSays: `hold` is the word that lets a
// creature with nothing left still have HAD its turn, so it is never gated.
func (s *TableSuite) TestHoldIsAffordableWhateverTheBudgetSays() {
	table := behavior.Table{behavior.KeyTime: {{Weight: 1, Hold: true}}}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.Require().Len(chosen.Candidates, 1, "an authored hold is a real pick, not the empty one")
	s.Equal(0, chosen.Entry)
}

// TestAnEventsWordsCostNothingATurnCanRunOutOf: `fact`, `flee` and a bare line
// are answers to something that just happened, not spends out of a turn — so
// affordability never gates them.
func (s *TableSuite) TestAnEventsWordsCostNothingATurnCanRunOutOf() {
	table := behavior.Table{anEventKey: {
		{Weight: 1, Fact: "camp-cowed"},
		{Weight: 1, Flee: true},
		{Weight: 1, Say: "nothing doing"},
	}}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: anEventKey, Table: table, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.Len(chosen.Candidates, 3, "all three, on a creature with nothing left to spend")
}

// TestEveryRowGetsAVerdict is rpg-project#674: a pick names a verdict for
// EVERY row of the key in authored order — the row whose condition failed,
// the row the creature could not pay for, and the candidate with its load —
// so a reader can say why each row did or did not fire without the file.
func (s *TableSuite) TestEveryRowGetsAVerdict() {
	table := behavior.Table{behavior.KeyTime: {
		{
			Weight: 4, When: []behavior.When{{Enemy: behavior.EnemySeen}},
			Toward: &behavior.Selector{Word: behavior.SelectorEnemy},
		},
		{Weight: 2, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 3, Away: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}
	temper := behavior.Temper{Profile: cowardProfile}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Temper: temper,
		Facts: behavior.Facts{CanMove: true}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)

	s.Equal([]behavior.RowVerdict{
		{Entry: 0, Held: false, Affordable: true},
		{Entry: 1, Held: true, Affordable: false},
		{Entry: 2, Held: true, Affordable: true, Weight: 3, Percent: 300, Loaded: 900},
	}, chosen.Rows, "the failed condition, the unaffordable swing, and the candidate with its load")
	s.Require().Len(chosen.Candidates, 1, "the candidates stay the eligible subset")
	s.Equal(2, chosen.Candidates[0].Entry)
}

// TestAHoldStillSaysWhyEveryRowWasOut: a `time` pick with nothing eligible is
// the one most in need of explaining, so the hold carries its verdicts too.
func (s *TableSuite) TestAHoldStillSaysWhyEveryRowWasOut() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemySeen}}, Hold: true},
		{Weight: 1, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.Equal(-1, chosen.Entry)
	s.Equal([]behavior.RowVerdict{
		{Entry: 0, Held: false, Affordable: true},
		{Entry: 1, Held: true, Affordable: false},
	}, chosen.Rows)
}

// --- the loaded die ----------------------------------------------------------

// TestTheBeatsArithmeticAddsUp is R7: every candidate carries its authored
// weight, its temperament's factor and their product, and the products sum to
// the die that was rolled. A reader has to be able to add it up.
func (s *TableSuite) TestTheBeatsArithmeticAddsUp() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 3, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Away: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}
	temper := behavior.Temper{Word: "coward", Profile: cowardProfile}

	roller := &facedDie{face: 1}
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Temper: temper, Facts: canAct, Die: roller})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)

	s.Require().Len(chosen.Candidates, 2)
	s.Equal(behavior.Candidate{Entry: 0, Weight: 3, Percent: 50, Loaded: 150}, chosen.Candidates[0])
	s.Equal(behavior.Candidate{Entry: 1, Weight: 1, Percent: 300, Loaded: 300}, chosen.Candidates[1])

	sum := 0
	for _, c := range chosen.Candidates {
		sum += c.Loaded
	}
	s.Equal(chosen.Of, sum, "the die is the sum of the loaded shares and nothing else")
	s.Equal(450, roller.of, "and it is the die that was actually rolled")
	s.Equal("coward", chosen.Temper, "the beat says WHY the shares are what they are")
}

// TestASoldierIsNoTemperAtAll: the zero profile and the all-100 profile are
// one creature, which is what "no temper = soldier" means and why a soldier
// stores nothing.
func (s *TableSuite) TestASoldierIsNoTemperAtAll() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 3, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Away: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}

	none, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: canAct, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	soldier, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: canAct,
		Temper: behavior.Temper{Profile: soldierProfile}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)

	s.Equal(none.Candidates, soldier.Candidates)
	s.Equal(none.Of, soldier.Of)
}

// TestTemperamentChangesTheSharesAndTheOdds is design §3, and the only claim
// in this file that needs a thousand rolls: the same table under `coward` and
// `aggressive` gives measurably different distributions, in the direction the
// content says.
//
// A SEEDED DIE AND A STATED TOLERANCE, because a distribution asserted exactly
// is a test that fails on a fair die. Five points either side of the arithmetic
// share, over a thousand rolls, is comfortably outside the noise and
// comfortably inside what a real shift looks like.
func (s *TableSuite) TestTemperamentChangesTheSharesAndTheOdds() {
	// The goblin's authored answer from the front room: 70 cower, 30 run.
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 70, Hold: true},
		{Weight: 30, Away: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}

	const rolls = 1000
	for _, tc := range []struct {
		name      string
		temper    behavior.Temper
		of        int
		wantRuns  float64
		tolerance float64
	}{
		{
			// 70×100 hold against 30×300 away: 7000 and 9000 of 16000.
			name:   "coward",
			temper: behavior.Temper{Word: "coward", Profile: cowardProfile},
			of:     16000, wantRuns: 56.25, tolerance: 5,
		},
		{
			// 70×50 hold against 30×25 away: 3500 and 750 of 4250.
			name:   "aggressive",
			temper: behavior.Temper{Word: "aggressive", Profile: aggressiveProfile},
			of:     4250, wantRuns: 17.6, tolerance: 5,
		},
		{
			// Unloaded: the author's own 70/30.
			name:   "soldier",
			temper: behavior.Temper{Word: "soldier", Profile: soldierProfile},
			of:     10000, wantRuns: 30, tolerance: 5,
		},
	} {
		s.Run(tc.name, func() {
			roller := newSeededDie(20260918)
			ran := 0
			for i := 0; i < rolls; i++ {
				chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
					Key: behavior.KeyTime, Table: table, Temper: tc.temper, Facts: canAct, Die: roller})
				s.Require().NoError(err)
				s.Require().Equal(tc.of, chosen.Of, "the loaded die is the same size every roll")
				if chosen.Answer.Away != nil {
					ran++
				}
			}
			share := float64(ran) / rolls * 100
			s.InDelta(tc.wantRuns, share, tc.tolerance,
				"%s ran %d times in %d — the arithmetic says about %.1f%%", tc.name, ran, rolls, tc.wantRuns)
		})
	}
}

// TestZeroEligibleUnderTimeIsAHoldThatSaysSo is "fail closed loudly": a
// creature that was given time and whose every entry's condition is false
// still HAD its time, and the beat records that it was asked. A silent nothing
// would be indistinguishable from never being consulted.
func (s *TableSuite) TestZeroEligibleUnderTimeIsAHoldThatSaysSo() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemySeen}},
			Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: canAct, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen, "the creature was asked, so there is a pick")
	s.True(chosen.Answer.Hold)
	s.Empty(chosen.Candidates, "and nothing was on the table")
	s.Equal(-1, chosen.Entry, "so no entry fired")
	s.Zero(chosen.Roll, "and nothing was rolled")
}

// TestZeroEligibleUnderASocialKeyIsSilence keeps the shipped answer: a
// placement that authored nothing for this outcome produces no beat at all,
// which is distinguishable from an entry that fired and did nothing.
func (s *TableSuite) TestZeroEligibleUnderASocialKeyIsSilence() {
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: anEventKey, Table: behavior.Table{}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Nil(chosen)
}

// TestAPickWithNoDieIsRefusedByName: the roller is optional at construction
// because a scene with no table rolls nothing. The moment something does, its
// absence is [behavior.ErrNoDie] rather than a silent default.
func (s *TableSuite) TestAPickWithNoDieIsRefusedByName() {
	table := behavior.Table{behavior.KeyTime: {{Weight: 1, Hold: true}}}

	_, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table})
	s.Require().Error(err)
	s.ErrorIs(err, behavior.ErrNoDie)
}

// TestACellSelectorIsCarriedAndNeverRead is the one thing this module does
// with a cell: hand it back. tools/spatial is as rulebook-neutral as this
// module is, and a position is the shape every board in this repo already
// speaks — but nothing here reads one, and a caller that resolves `at` to a
// walk is the only thing that ever will.
func (s *TableSuite) TestACellSelectorIsCarriedAndNeverRead() {
	cell := someCell
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Toward: &behavior.Selector{At: &cell}},
	}}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: canAct, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.Require().NotNil(chosen.Answer.Toward)
	s.Equal(&cell, chosen.Answer.Toward.At, "the cell came back exactly as it went in")
	s.Empty(chosen.Answer.Toward.Word, "and a cell selector names no word")
}

// --- the deal ----------------------------------------------------------------

// TestAMixDealsEveryWordItNames is design §3: four goblins, one table, four
// behaviours. The mix is walked in sorted order so the same seed deals the same
// word every run (C8), and over enough rolls every word in it comes up.
func (s *TableSuite) TestAMixDealsEveryWordItNames() {
	mix := behavior.Temper{
		Mix: map[string]int{"coward": 1, "soldier": 2, "aggressive": 1},
		Profiles: map[string]behavior.TemperProfile{
			"coward": cowardProfile, "soldier": soldierProfile, "aggressive": aggressiveProfile,
		},
	}

	roller := newSeededDie(20260918)
	seen := map[string]int{}
	for i := 0; i < 400; i++ {
		out, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: mix, Die: roller})
		s.Require().NoError(err)
		s.Equal(4, out.Of, "the die is the sum of the shares the author wrote")
		s.GreaterOrEqual(out.Roll, 1)
		s.LessOrEqual(out.Roll, out.Of)
		s.Empty(out.Temper.Mix, "what comes out of a deal is a creature, not a spread")
		seen[out.Temper.Word]++
	}

	s.Len(seen, 3, "every word in the mix was dealt at least once: %v", seen)
	s.Greater(seen["soldier"], seen["coward"], "a share of two comes up more often than a share of one")
}

// TestADealtWordCarriesItsProfile: the caller supplies what each word MEANS,
// and the deal hands back the one that was chosen — this module never asks
// what "coward" is.
func (s *TableSuite) TestADealtWordCarriesItsProfile() {
	mix := behavior.Temper{
		Mix:      map[string]int{"coward": 1},
		Profiles: map[string]behavior.TemperProfile{"coward": cowardProfile},
	}

	out, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: mix, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal("coward", out.Temper.Word)
	s.Equal(cowardProfile, out.Temper.Profile)
	s.Equal(1, out.Roll)
	s.Equal(1, out.Of)
}

// TestAnyWordWithAProfileIsDealt: the vocabulary is the CALLER'S. This module
// seals no list — a rulebook that ships `reckless` and says what it means
// deals `reckless`, and refusing an unsealed word on an author's form is the
// dialect's job, not this one's.
func (s *TableSuite) TestAnyWordWithAProfileIsDealt() {
	mix := behavior.Temper{
		Mix:      map[string]int{"reckless": 1},
		Profiles: map[string]behavior.TemperProfile{"reckless": {Attack: 500, Hold: 10}},
	}

	out, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: mix, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal("reckless", out.Temper.Word)
	s.Equal(500, out.Temper.Profile.Attack)
}

// TestAMixNamingAWordNothingDefinesIsRefused is fail-closed at the door: a
// temperament dealt with no profile behind it would silently apply as a
// soldier, and the creature would look like an authoring success.
func (s *TableSuite) TestAMixNamingAWordNothingDefinesIsRefused() {
	mix := behavior.Temper{Mix: map[string]int{"coward": 1}}

	_, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: mix, Die: &facedDie{face: 1}})
	s.Require().Error(err)
	s.ErrorIs(err, behavior.ErrBadMix)
	s.Contains(err.Error(), "coward")
}

// TestAShareBelowOneCanNeverBeDealt: the same rule a weight keeps, one layer
// up. A zero share is a temperament sitting in a file looking like a
// possibility.
func (s *TableSuite) TestAShareBelowOneCanNeverBeDealt() {
	mix := behavior.Temper{
		Mix:      map[string]int{"coward": 0},
		Profiles: map[string]behavior.TemperProfile{"coward": cowardProfile},
	}

	_, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: mix, Die: &facedDie{face: 1}})
	s.Require().Error(err)
	s.ErrorIs(err, behavior.ErrBadMix)
}

// ---------------------------------------------------------------------------
// The three readings of a deed condition (rpg-toolkit#1883, rpg-project#498)
// ---------------------------------------------------------------------------
//
// A `when` on a deed gained a SCOPE: whose deed it is about. The default is
// unchanged — a deed against the creature itself — so every test above still
// proves what it always proved. These prove the two readings beside it, and
// that the default did not move.

// deedReadings builds a table whose ONLY eligible entry is the scoped one, so
// the chosen index says which reading held.
func deedReadings(scope behavior.DeedScope) behavior.Table {
	return behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "attacked", Within: 3, Scope: scope}},
			Attack: &behavior.Selector{Word: behavior.SelectorAttacker}},
		{Weight: 1, Hold: true},
	}}
}

func (s *TableSuite) TestADeedConditionDefaultsToAgainstTheCreatureItself() {
	// THE COMPATIBILITY CLAIM: a document that authors no scope means what it
	// always meant. An empty scope reads `Deeds`, so the row that fired for a
	// blow to the creature still fires.
	facts := behavior.Facts{
		Now: 11, CanAttack: true,
		Deeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "alice", At: 10}},
	}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: deedReadings(behavior.ScopeSelf),
		Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(0, chosen.Entry, "a blow to me still reads the default scope")
}

func (s *TableSuite) TestAllyScopeReadsABlowToMySideNotToMe() {
	table := deedReadings(behavior.ScopeAlly)

	// A blow to an ALLY holds, even though nothing was done to this creature.
	toAlly := behavior.Facts{
		Now: 11, CanAttack: true,
		AllyDeeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "alice", At: 10}},
	}
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: toAlly, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(0, chosen.Entry, "an ally's wound is the fact this condition asks for")

	// A blow to ME does NOT hold under `ally`, which is the whole point of the
	// two readings being separate.
	toMe := behavior.Facts{
		Now: 11, CanAttack: true,
		Deeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "alice", At: 10}},
	}
	chosen, err = behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: toMe, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(1, chosen.Entry, "my own wound is not the ally reading")
}

func (s *TableSuite) TestActorScopeIsThePauseReadsWhatIDid() {
	// THE PAUSE, with no separate concept in the grammar: "after I strike,
	// stand still for two rounds" is a deed the creature DID, recently.
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "attacked", Within: 2, Scope: behavior.ScopeActor}},
			Hold: true},
		{Weight: 1, Attack: &behavior.Selector{Word: behavior.SelectorEnemy}},
	}}

	struck := behavior.Facts{
		Now: 11, CanAttack: true,
		OwnDeeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "me", At: 10}},
	}
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: struck, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(0, chosen.Entry, "the round after I struck, the pause holds")

	// Two rounds later the span has run out and it acts again.
	fresh := behavior.Facts{
		Now: 13, CanAttack: true,
		OwnDeeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "me", At: 10}},
	}
	chosen, err = behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: fresh, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(1, chosen.Entry, "the pause ends when its span does")

	// AND A BLOW TO ME IS NOT A THING I DID: the actor reading does not fire
	// on the creature's own Deeds, which is what keeps the two apart.
	wasHit := behavior.Facts{
		Now: 11, CanAttack: true,
		Deeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "alice", At: 10}},
	}
	chosen, err = behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: wasHit, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(1, chosen.Entry, "being hit is not doing it")
}

func (s *TableSuite) TestAnUnsetAllyReadingAuthorsNothing() {
	// THE ZERO VALUE TELLS THE TRUTH, the same rule the rest of this package
	// keeps: a caller that has not thought about allies hands `nil`, and a
	// table that authors no `ally` condition behaves exactly as before.
	facts := behavior.Facts{Now: 11, CanAttack: true}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: deedReadings(behavior.ScopeAlly),
		Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(1, chosen.Entry, "no ally deeds means the ally row is not on the table")
}

// TestAnUnknownScopeMatchesNothing is the fail-closed guard (found in review,
// rpg-toolkit#1884 thread 1): a scope that is neither the default nor one of
// the two readings must make the row ABSENT, not quietly read as "against me".
//
// The dialect refuses an unknown scope at decode, so this arm is for a struct
// built in Go — a test, a future rulebook, a consumer pinned one tag behind —
// which never passes through that door. Failing open there would fire a row on
// the wrong facts, which is worse than a row that is visibly dead.
func (s *TableSuite) TestAnUnknownScopeMatchesNothing() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "attacked", Within: 3, Scope: behavior.DeedScope("allie")}},
			Attack: &behavior.Selector{Word: behavior.SelectorAttacker}},
		{Weight: 1, Hold: true},
	}}

	// The creature WAS attacked, so a fail-open reading would fire entry 0.
	facts := behavior.Facts{
		Now: 11, CanAttack: true,
		Deeds: []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "alice", At: 10}},
	}

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(1, chosen.Entry, "a typo'd scope is a dead row, not the default reading")
}

// TestAnEmptyScopeIsOmittedWhenMarshalled pins the claim the field doc makes
// (found in review, rpg-toolkit#1884 thread 3). The consumer's goldens caught
// the defect this prevents, but the proof belongs where the field lives: a
// future serializer change should fail HERE first, not three modules away.
func (s *TableSuite) TestAnEmptyScopeIsOmittedWhenMarshalled() {
	unscoped, err := json.Marshal(behavior.When{Deed: "attacked", Within: 3})
	s.Require().NoError(err)
	s.NotContains(string(unscoped), "Scope",
		"the pre-scope reading writes no key, which is what keeps every committed golden byte-identical")

	scoped, err := json.Marshal(behavior.When{Deed: "attacked", Within: 3, Scope: behavior.ScopeAlly})
	s.Require().NoError(err)
	s.Contains(string(scoped), `"Scope":"ally"`, "and a real scope does carry")
}

// --- heard, fact, not, the list, and path (rpg-project#569) ------------------

// pickTime rolls the `time` key with a die that always lands on the first
// candidate, so the entry that fired is the first one eligible.
func (s *TableSuite) pickTime(table behavior.Table, facts behavior.Facts) int {
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)

	return chosen.Entry
}

// TestHeardHoldsInsideItsWindowAndForItsKind is `when: { heard: { kind,
// within } }`: a sound of that kind the creature holds, no older than the
// span. The boundary is the deed condition's own — N units later still counts,
// N+1 does not — and a sound of another kind is not this condition.
func (s *TableSuite) TestHeardHoldsInsideItsWindowAndForItsKind() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 2}}},
			Toward: &behavior.Selector{At: &someCell}},
		{Weight: 1, Hold: true},
	}}

	for _, tc := range []struct {
		name  string
		heard sound.Sound
		now   uint64
		entry int
	}{
		{name: "the unit it was heard on", heard: sound.Sound{Kind: "clang", At: 10}, now: 10, entry: 0},
		{name: "two later, the last that counts", heard: sound.Sound{Kind: "clang", At: 10}, now: 12, entry: 0},
		{name: "three later, the window has closed", heard: sound.Sound{Kind: "clang", At: 10}, now: 13, entry: 1},
		{name: "a sound of another kind", heard: sound.Sound{Kind: "voice", At: 10}, now: 10, entry: 1},
		{name: "a sound stamped ahead of now is wrong, not fresh",
			heard: sound.Sound{Kind: "clang", At: 11}, now: 10, entry: 1},
	} {
		s.Run(tc.name, func() {
			facts := behavior.Facts{Now: tc.now, CanMove: true, Sounds: []sound.Sound{tc.heard}}
			s.Equal(tc.entry, s.pickTime(table, facts))
		})
	}
}

// TestFactHoldsWhenTheCreatureKnowsIt is `when: { fact: <id> }`: the id is
// among the facts this creature holds, and nothing else answers it.
func (s *TableSuite) TestFactHoldsWhenTheCreatureKnowsIt() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Fact: lit("alarm-raised")}}, Toward: &behavior.Selector{At: &someCell}},
		{Weight: 1, Hold: true},
	}}

	s.Equal(0, s.pickTime(table, behavior.Facts{CanMove: true, Known: []string{"guards-turned", "alarm-raised"}}),
		"the creature holds the fact")
	s.Equal(1, s.pickTime(table, behavior.Facts{CanMove: true, Known: []string{"guards-turned"}}),
		"another fact is not this one")
	s.Equal(1, s.pickTime(table, behavior.Facts{CanMove: true}), "a creature that knows nothing")
}

// TestEveryWhenInTheListMustHold is R8: a row's `when` is a list and the row
// is eligible only when every entry holds. This is the storm-the-keep row —
// turn on the fact, but not while a loyalist is in sight.
func (s *TableSuite) TestEveryWhenInTheListMustHold() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{
			{Fact: lit("guards-turned")},
			{Not: &behavior.When{Enemy: behavior.EnemySeen}},
		}, Toward: &behavior.Selector{At: &someCell}},
		{Weight: 1, Hold: true},
	}}
	turned := []string{"guards-turned"}

	s.Equal(0, s.pickTime(table, behavior.Facts{CanMove: true, Known: turned}), "both hold")
	s.Equal(1, s.pickTime(table, behavior.Facts{CanMove: true, Known: turned, EnemySeen: true}),
		"the fact holds and an enemy is in sight: one of two is not enough")
	s.Equal(1, s.pickTime(table, behavior.Facts{CanMove: true}),
		"no enemy in sight and no fact: one of two is not enough")
}

// TestAnEmptyListIsAStandingOrder: no conditions is all of them holding,
// which is the meaning a row with no `when` always had.
func (s *TableSuite) TestAnEmptyListIsAStandingOrder() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{}, Toward: &behavior.Selector{At: &someCell}},
	}}

	s.Equal(0, s.pickTime(table, behavior.Facts{CanMove: true}))
}

// TestNotHoldsWhenItsConditionDoesNot is `not: { … }`: the negative of one
// condition, wrapping any of them, another `not` included.
func (s *TableSuite) TestNotHoldsWhenItsConditionDoesNot() {
	row := func(w behavior.When) behavior.Table {
		return behavior.Table{behavior.KeyTime: {
			{Weight: 1, When: []behavior.When{w}, Toward: &behavior.Selector{At: &someCell}},
			{Weight: 1, Hold: true},
		}}
	}
	notSeen := row(behavior.When{Not: &behavior.When{Enemy: behavior.EnemySeen}})
	notHeard := row(behavior.When{Not: &behavior.When{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 2}}})
	notNotSeen := row(behavior.When{Not: &behavior.When{Not: &behavior.When{Enemy: behavior.EnemySeen}}})
	clang := []sound.Sound{{Kind: "clang", At: 5}}

	s.Equal(0, s.pickTime(notSeen, behavior.Facts{CanMove: true}), "no enemy in sight")
	s.Equal(1, s.pickTime(notSeen, behavior.Facts{CanMove: true, EnemySeen: true}), "an enemy in sight")
	s.Equal(0, s.pickTime(notHeard, behavior.Facts{CanMove: true, Now: 5}), "heard nothing")
	s.Equal(1, s.pickTime(notHeard, behavior.Facts{CanMove: true, Now: 5, Sounds: clang}), "heard the clang")
	s.Equal(0, s.pickTime(notHeard, behavior.Facts{CanMove: true, Now: 9, Sounds: clang}),
		"the clang is outside the window, so it was not heard")
	s.Equal(0, s.pickTime(notNotSeen, behavior.Facts{CanMove: true, EnemySeen: true}),
		"two negatives are the condition itself")
}

// TestAWhenNamesExactlyOneCondition is the refusal, in the sentence the
// form-filler reads: two conditions in one `when` entry are two entries of
// the list, and an entry naming nothing is a row that meant something else.
func (s *TableSuite) TestAWhenNamesExactlyOneCondition() {
	for _, tc := range []struct {
		name string
		when behavior.When
		says string
	}{
		{name: "two conditions",
			when: behavior.When{Enemy: behavior.EnemySeen, Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 2}},
			says: "a `when` entry names `enemy` and `heard`; give each its own entry in the list, " +
				"and every entry must hold: behavior: when cannot be read"},
		{name: "three conditions",
			when: behavior.When{
				Deed: "attacked", Within: 2, Fact: lit("alarm-raised"),
				Not: &behavior.When{Enemy: behavior.EnemySeen},
			},
			says: "a `when` entry names `attacked`, `fact` and `not`; give each its own entry in the list, " +
				"and every entry must hold: behavior: when cannot be read"},
		{name: "no condition",
			when: behavior.When{},
			says: "a `when` entry names no condition; give it one of `enemy`, `attacked`, `intimidated`, " +
				"`persuaded`, `fled`, `heard`, `fact` or `not`: behavior: when cannot be read"},
		{name: "two conditions inside a not",
			when: behavior.When{Not: &behavior.When{Enemy: behavior.EnemySeen, Fact: lit("alarm-raised")}},
			says: "inside `not`: a `when` entry names `enemy` and `fact`; give each its own entry in the list, " +
				"and every entry must hold: behavior: when cannot be read"},
		{name: "heard with no kind",
			when: behavior.When{Heard: &behavior.HeardWhen{Within: 2}},
			says: "`heard` needs a `kind`, the sound it listens for: " +
				"behavior: when cannot be read"},
		{name: "heard with a span of zero",
			when: behavior.When{Heard: &behavior.HeardWhen{Kind: lit("clang")}},
			says: "`heard` within 0: a span is counted from 1: behavior: when cannot be read"},
	} {
		s.Run(tc.name, func() {
			entry := behavior.Answer{Weight: 1, When: []behavior.When{{Fact: lit("fine")}, tc.when}, Hold: true}
			err := entry.Validate()
			s.Require().ErrorIs(err, behavior.ErrBadWhen)
			s.EqualError(err, tc.says)
		})
	}

	ok := behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{
		{Enemy: behavior.EnemyNone},
		{Deed: "attacked", Within: 2},
		{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 2}},
		{Fact: lit("alarm-raised")},
		{Not: &behavior.When{Not: &behavior.When{Enemy: behavior.EnemySeen}}},
	}}
	s.NoError(ok.Validate(), "one condition per entry, however many entries")
}

// TestAPathIsARouteAndOnlyTowardWalksOne is `toward: { path: [...] }` (R7):
// a selector names exactly one target, and a route is a target only for the
// word that walks.
func (s *TableSuite) TestAPathIsARouteAndOnlyTowardWalksOne() {
	route := []spatial.Position{{X: 2, Y: 3}, {X: 6, Y: 3}, {X: 6, Y: 8}}

	s.NoError((&behavior.Answer{Weight: 1, Toward: &behavior.Selector{Path: route}}).Validate())

	for _, tc := range []struct {
		name  string
		entry behavior.Answer
		says  string
	}{
		{name: "a path on attack", entry: behavior.Answer{Weight: 1, Attack: &behavior.Selector{Path: route}},
			says: "`attack` cannot take a `path`: a route is walked, and only `toward` walks one: " +
				"behavior: selector cannot be read"},
		{name: "a path on away", entry: behavior.Answer{Weight: 1, Away: &behavior.Selector{Path: route}},
			says: "`away` cannot take a `path`: a route is walked, and only `toward` walks one: " +
				"behavior: selector cannot be read"},
		{name: "a cell and a path", entry: behavior.Answer{Weight: 1,
			Toward: &behavior.Selector{At: &someCell, Path: route}},
			says: "`toward` names a cell (`at`) and a route (`path`); a target is exactly one of " +
				"somebody, a cell or a route: behavior: selector cannot be read"},
		{name: "a word and a path", entry: behavior.Answer{Weight: 1,
			Toward: &behavior.Selector{Word: behavior.SelectorEnemy, Path: route}},
			says: "`toward` names `enemy` and a route (`path`); a target is exactly one of " +
				"somebody, a cell or a route: behavior: selector cannot be read"},
	} {
		s.Run(tc.name, func() {
			err := tc.entry.Validate()
			s.Require().ErrorIs(err, behavior.ErrBadSelector)
			s.EqualError(err, tc.says)
		})
	}

	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyNone}}, Toward: &behavior.Selector{Path: route}},
	}}
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: behavior.Facts{CanMove: true}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(route, chosen.Answer.Toward.Path, "the route is carried, and the caller walks it")
}

// TestTheNewFieldsAreOmittedWhenEmpty is the Scope rule carried to every
// field added beside it: a condition or selector that names none of them
// marshals exactly as it did before they existed.
func (s *TableSuite) TestTheNewFieldsAreOmittedWhenEmpty() {
	when, err := json.Marshal(behavior.When{Deed: "attacked", Within: 3})
	s.Require().NoError(err)
	s.NotContains(string(when), "Heard")
	s.NotContains(string(when), "Fact")
	s.NotContains(string(when), "Not")

	sel, err := json.Marshal(behavior.Selector{Word: behavior.SelectorEnemy})
	s.Require().NoError(err)
	s.NotContains(string(sel), "Path")

	route, err := json.Marshal(behavior.Selector{Path: []spatial.Position{{X: 1, Y: 2}}})
	s.Require().NoError(err)
	s.NotContains(string(route), "Facings")
	s.NotContains(string(route), "Waits")
}

// TestAWaypointMayCarryTheFacingOfItsPause is rpg-project#624: a route may
// state, per waypoint, the bearing faced and the rounds stood while the
// walker pauses there. The module carries both beside the cell and reads
// them no more than the cell; it refuses a list that cannot be matched to
// the waypoints, a facing outside [0, 360) and a negative wait.
func (s *TableSuite) TestAWaypointMayCarryTheFacingOfItsPause() {
	route := []spatial.Position{{X: 2, Y: 3}, {X: 6, Y: 3}, {X: 6, Y: 8}}
	south := 180.0
	faced := &behavior.Selector{Path: route, Facings: []*float64{nil, &south, nil}, Waits: []int{0, 2, 0}}

	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyNone}}, Toward: faced},
	}}
	s.Require().NoError((&behavior.Answer{Weight: 1, Toward: faced}).Validate())
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: behavior.Facts{CanMove: true}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().Len(chosen.Answer.Toward.Facings, len(route), "one facing slot per waypoint")
	s.Nil(chosen.Answer.Toward.Facings[0], "a waypoint with no facing carries none")
	s.Equal(&south, chosen.Answer.Toward.Facings[1], "the faced waypoint carries its bearing")

	s.Require().Len(chosen.Answer.Toward.Waits, len(route), "one wait slot per waypoint")
	s.Equal(0, chosen.Answer.Toward.Waits[0], "a waypoint with no wait carries none")
	s.Equal(2, chosen.Answer.Toward.Waits[1], "the waypoint that waits carries its rounds")

	north := 0.0
	facedNorth := &behavior.Selector{Path: route, Facings: []*float64{&north, nil, nil}}
	s.NoError((&behavior.Answer{Weight: 1, Toward: facedNorth}).Validate(), "a facing of exactly 0 is a bearing")

	full, wrapAround, notANumber := 360.0, -1.0, math.NaN()
	for _, tc := range []struct {
		name string
		sel  behavior.Selector
		says string
	}{
		{name: "too few facings", sel: behavior.Selector{Path: route, Facings: []*float64{&south}},
			says: "`toward` states 1 waypoint facings for a route of 3 waypoints; a facing belongs " +
				"to one waypoint, so a route states one per waypoint or none: behavior: selector cannot be read"},
		{name: "too few waits", sel: behavior.Selector{Path: route, Waits: []int{2}},
			says: "`toward` states 1 waypoint waits for a route of 3 waypoints; a wait belongs " +
				"to one waypoint, so a route states one per waypoint or none: behavior: selector cannot be read"},
		{name: "too many facings", sel: behavior.Selector{Path: route, Facings: []*float64{nil, &south, nil, nil}},
			says: "`toward` states 4 waypoint facings for a route of 3 waypoints; a facing belongs " +
				"to one waypoint, so a route states one per waypoint or none: behavior: selector cannot be read"},
		{name: "too many waits", sel: behavior.Selector{Path: route, Waits: []int{0, 2, 0, 1}},
			says: "`toward` states 4 waypoint waits for a route of 3 waypoints; a wait belongs " +
				"to one waypoint, so a route states one per waypoint or none: behavior: selector cannot be read"},
		{name: "a NaN facing", sel: behavior.Selector{Path: route, Facings: []*float64{nil, nil, &notANumber}},
			says: "`toward` waypoint 3 faces NaN, which is not a bearing in [0, 360): behavior: selector cannot be read"},
		{name: "a facing of 360", sel: behavior.Selector{Path: route, Facings: []*float64{nil, &full, nil}},
			says: "`toward` waypoint 2 faces 360, which is not a bearing in [0, 360): behavior: selector cannot be read"},
		{name: "a negative facing", sel: behavior.Selector{Path: route, Facings: []*float64{&wrapAround, nil, nil}},
			says: "`toward` waypoint 1 faces -1, which is not a bearing in [0, 360): behavior: selector cannot be read"},
		{name: "a negative wait", sel: behavior.Selector{Path: route, Waits: []int{0, 0, -1}},
			says: "`toward` waypoint 3 waits -1 rounds; a wait is never negative: behavior: selector cannot be read"},
	} {
		s.Run(tc.name, func() {
			sel := tc.sel
			err := (&behavior.Answer{Weight: 1, Toward: &sel}).Validate()
			s.Require().ErrorIs(err, behavior.ErrBadSelector)
			s.EqualError(err, tc.says)
		})
	}
}

// TestSoundIsAPlaceToWalkToOrFrom is the `sound` selector word: where the
// freshest held sound was made. It is sealed beside the other words, walks
// with `toward` and `away`, and is refused on `attack` — a sound cannot be
// struck.
func (s *TableSuite) TestSoundIsAPlaceToWalkToOrFrom() {
	s.Contains(behavior.SelectorWords, behavior.SelectorSound)
	s.Equal(behavior.SelectorWord("sound"), behavior.SelectorSound)

	heard := []behavior.When{{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 2}}}
	s.NoError((&behavior.Answer{Weight: 1, When: heard,
		Toward: &behavior.Selector{Word: behavior.SelectorSound}}).Validate())
	s.NoError((&behavior.Answer{Weight: 1, When: heard,
		Away: &behavior.Selector{Word: behavior.SelectorSound}}).Validate())

	err := (&behavior.Answer{Weight: 1, When: heard,
		Attack: &behavior.Selector{Word: behavior.SelectorSound}}).Validate()
	s.Require().ErrorIs(err, behavior.ErrBadSelector)
	s.EqualError(err, "`attack` cannot take `sound`: a sound cannot be struck, only walked toward "+
		"or away from: behavior: selector cannot be read")
}

// TestAHeardSpanBelowOneHoldsNothing: Pick never validates, so a struct built
// in Go reaches the evaluator with whatever span it carries. Zero and below
// hold nothing — a negative span must not wrap to a window that never closes.
func (s *TableSuite) TestAHeardSpanBelowOneHoldsNothing() {
	for _, within := range []int{0, -1} {
		s.Run(fmt.Sprintf("within %d", within), func() {
			table := behavior.Table{behavior.KeyTime: {
				{Weight: 1, When: []behavior.When{{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: within}}},
					Toward: &behavior.Selector{At: &someCell}},
				{Weight: 1, Hold: true},
			}}
			for _, heardAt := range []uint64{10, 1} {
				facts := behavior.Facts{Now: 10, CanMove: true, Sounds: []sound.Sound{{Kind: "clang", At: heardAt}}}
				s.Equal(1, s.pickTime(table, facts), "heard at %d, now 10", heardAt)
			}
		})
	}
}

// TestADeedsSpanAndScopeBelongOnlyBesideADeed: `within`, `on` and `as` modify
// a deed condition, and dangling one off another condition is a shape fault.
// The deed span rule itself (below 1) stays the dialect's.
func (s *TableSuite) TestADeedsSpanAndScopeBelongOnlyBesideADeed() {
	for _, tc := range []struct {
		name string
		when behavior.When
		says string
	}{
		{name: "a span beside a fact", when: behavior.When{Fact: lit("alarm-raised"), Within: 3},
			says: "a `when` entry names `fact` with a deed's `within` or scope; " +
				"those belong only beside a deed: behavior: when cannot be read"},
		{name: "a scope beside an enemy band", when: behavior.When{Enemy: behavior.EnemySeen, Scope: behavior.ScopeAlly},
			says: "a `when` entry names `enemy` with a deed's `within` or scope; " +
				"those belong only beside a deed: behavior: when cannot be read"},
	} {
		s.Run(tc.name, func() {
			err := (&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{tc.when}}).Validate()
			s.Require().ErrorIs(err, behavior.ErrBadWhen)
			s.EqualError(err, tc.says)
		})
	}

	s.NoError((&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{
		{Deed: "attacked", Within: 3, Scope: behavior.ScopeAlly},
	}}).Validate(), "beside a deed they are what the deed means")
}

// TestASelectorNamingNoTargetIsRefused: exactly one of the three means not
// zero either, and an empty route is no route.
func (s *TableSuite) TestASelectorNamingNoTargetIsRefused() {
	for _, sel := range []*behavior.Selector{{}, {Path: []spatial.Position{}}} {
		err := (&behavior.Answer{Weight: 1, Toward: sel}).Validate()
		s.Require().ErrorIs(err, behavior.ErrBadSelector)
		s.EqualError(err, "`toward` names no target; give it one of somebody, a cell (`at`) or a route (`path`): "+
			"behavior: selector cannot be read")
	}
}

// TestSoundNeedsARowThatListened is R10: `sound` walks to the sound the row
// listened for, so a row with no `heard` condition has none to walk to — the
// way `actor` needs a deed. A `heard` inside a `not` is a row that heard
// nothing, and does not count.
func (s *TableSuite) TestSoundNeedsARowThatListened() {
	const says = "`sound` needs a `heard` condition on the row: " +
		"the sound it walks to is the one the row listened for: behavior: selector cannot be read"
	clang := &behavior.HeardWhen{Kind: lit("clang"), Within: 2}

	for _, tc := range []struct {
		name string
		when []behavior.When
	}{
		{name: "no when at all"},
		{name: "a fact and no heard", when: []behavior.When{{Fact: lit("alarm-raised")}}},
		{name: "a heard only inside a not", when: []behavior.When{{Not: &behavior.When{Heard: clang}}}},
	} {
		s.Run(tc.name, func() {
			for _, entry := range []behavior.Answer{
				{Weight: 1, When: tc.when, Toward: &behavior.Selector{Word: behavior.SelectorSound}},
				{Weight: 1, When: tc.when, Away: &behavior.Selector{Word: behavior.SelectorSound}},
			} {
				err := entry.Validate()
				s.Require().ErrorIs(err, behavior.ErrBadSelector)
				s.EqualError(err, says)
			}
		})
	}

	s.NoError((&behavior.Answer{Weight: 1,
		When:   []behavior.When{{Fact: lit("alarm-raised")}, {Heard: clang}},
		Toward: &behavior.Selector{Word: behavior.SelectorSound}}).Validate(),
		"a `heard` anywhere in the list is the row listening")
}

// TestAWalkToASoundIsAbsentWhenNoSoundIsHeld: a `sound` selector names where
// a held sound was made, so with no sound held there is nowhere to walk and
// the row is absent from the roll, like an unmet `when`. The second table
// has no `when` at all — a struct built in Go that never met Validate — so it
// is the affordability gate, and not the `heard` condition, being tested.
func (s *TableSuite) TestAWalkToASoundIsAbsentWhenNoSoundIsHeld() {
	listened := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 3}}},
			Toward: &behavior.Selector{Word: behavior.SelectorSound}},
		{Weight: 1, Hold: true},
	}}
	unvalidated := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Toward: &behavior.Selector{Word: behavior.SelectorSound}},
		{Weight: 1, Away: &behavior.Selector{Word: behavior.SelectorSound}},
		{Weight: 1, Hold: true},
	}}
	clang := []sound.Sound{{Kind: "clang", Where: someCell, At: 5}}

	s.Equal(1, s.pickTime(listened, behavior.Facts{Now: 5, CanMove: true}), "nothing held, nowhere to walk")
	s.Equal(0, s.pickTime(listened, behavior.Facts{Now: 5, CanMove: true, Sounds: clang}), "a clang is held")

	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: unvalidated, Facts: behavior.Facts{Now: 5, CanMove: true}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().Len(chosen.Candidates, 1, "toward and away on a sound are both absent; only hold is left")
	s.Equal(2, chosen.Entry)

	chosen, err = behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: unvalidated, Facts: behavior.Facts{Now: 5, CanMove: true, Sounds: clang},
		Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Equal(0, chosen.Entry, "with a sound held the walk is back on the table")
}

// TestLostIsSealedBetweenRememberedAndNone is the band's place in the sealed
// list a dialect refuses against: nearest first, so `lost` sits after
// `remembered` and before `none` (rpg-project#668 R3).
func (s *TableSuite) TestLostIsSealedBetweenRememberedAndNone() {
	s.Equal(behavior.EnemyWord("lost"), behavior.EnemyLost)
	s.Equal([]behavior.EnemyWord{
		behavior.EnemyReach, behavior.EnemySeen, behavior.EnemyRemembered, behavior.EnemyLost, behavior.EnemyNone,
	}, behavior.EnemyWords)
}

// TestSearchNeedsALostTrailAndLegs is the `search` word's eligibility
// (rpg-project#668 R4): it walks on from a lost cell, so with nothing lost
// there is nowhere to search from and the row is absent from the roll — the
// rule `sound` keeps for a walk with no sound held. And it is a walk, so a
// creature with no movement left cannot pay for it.
func (s *TableSuite) TestSearchNeedsALostTrailAndLegs() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingWent}},
	}}
	for _, tc := range []struct {
		name     string
		facts    behavior.Facts
		eligible bool
	}{
		{name: "lost, can move", facts: behavior.Facts{EnemyLost: true, CanMove: true}, eligible: true},
		{name: "lost, no movement left", facts: behavior.Facts{EnemyLost: true, CanAttack: true}},
		{name: "only remembered", facts: behavior.Facts{EnemyRemembered: true, CanMove: true}},
		{name: "nothing held", facts: behavior.Facts{CanMove: true}},
	} {
		s.Run(tc.name, func() {
			chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
				Key: behavior.KeyTime, Table: table, Facts: tc.facts, Die: &facedDie{face: 1}})
			s.Require().NoError(err)
			if tc.eligible {
				s.Equal(0, chosen.Entry)
				s.Require().NotNil(chosen.Answer.Search)
				s.Equal(6, chosen.Answer.Search.Distance)

				return
			}
			s.Equal(-1, chosen.Entry, "nothing eligible is a hold")
			s.True(chosen.Answer.Hold)
		})
	}
}

// TestSearchIsLoadedAsAWalkToward is the temperament on a search: it is the
// creature closing on where its quarry went, so it carries the `toward`
// factor. An aggressive creature searches at triple, a coward at half.
func (s *TableSuite) TestSearchIsLoadedAsAWalkToward() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingWent}},
		{Weight: 1, Hold: true},
	}}
	lost := behavior.Facts{EnemyLost: true, CanMove: true}
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: lost,
		Temper: behavior.Temper{Word: "aggressive", Profile: aggressiveProfile}, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().Len(chosen.Candidates, 2)
	s.Equal(aggressiveProfile.Toward, chosen.Candidates[0].Percent)
	s.Equal(aggressiveProfile.Hold, chosen.Candidates[1].Percent)
}

// TestASearchWalksAtLeastOneCell is the shape refusal: a distance below 1 is a
// search that goes nowhere. The dialect fills an omitted distance before this
// door, so a zero here is a struct built past it.
func (s *TableSuite) TestASearchWalksAtLeastOneCell() {
	lost := []behavior.When{{Enemy: behavior.EnemyLost}}
	s.NoError((&behavior.Answer{Weight: 1, When: lost,
		Search: &behavior.Search{Distance: 1, Heading: behavior.HeadingWent}}).Validate())
	for _, d := range []int{0, -3} {
		err := (&behavior.Answer{Weight: 1, When: lost,
			Search: &behavior.Search{Distance: d, Heading: behavior.HeadingWent}}).Validate()
		s.Require().ErrorIs(err, behavior.ErrBadSearch)
		s.Contains(err.Error(), "`search` distance")
	}
}

// TestARowThatDoesNotSearchMarshalsAsBefore is the persisted shape: a row
// with no `search` carries no Search key at all, so every compiled table
// written before the word existed is byte-identical after it.
func (s *TableSuite) TestARowThatDoesNotSearchMarshalsAsBefore() {
	plain, err := json.Marshal(behavior.Answer{Weight: 1, Hold: true})
	s.Require().NoError(err)
	s.NotContains(string(plain), "Search")

	searching, err := json.Marshal(behavior.Answer{Weight: 1,
		Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingWent}})
	s.Require().NoError(err)
	s.Contains(string(searching), `"Search":{"Distance":6,"Heading":"went"}`)
}

// TestASearchNamesOneOfFourHeadings is the sealed heading word
// (rpg-project#679, R10): `went`, `faced`, `left` and `right` are read; a
// fifth word, and an empty one, are refused. The dialect fills an omitted
// heading with `went` before this door, so an empty word here is a struct
// built past it, the same as a zero distance.
func (s *TableSuite) TestASearchNamesOneOfFourHeadings() {
	s.Equal([]behavior.SearchHeading{
		behavior.HeadingWent, behavior.HeadingFaced, behavior.HeadingLeft, behavior.HeadingRight,
	}, behavior.SearchHeadings)
	lost := []behavior.When{{Enemy: behavior.EnemyLost}}
	for _, h := range behavior.SearchHeadings {
		s.NoError((&behavior.Answer{Weight: 1, When: lost,
			Search: &behavior.Search{Distance: 6, Heading: h}}).Validate(), string(h))
	}
	for _, h := range []behavior.SearchHeading{"backwards", ""} {
		err := (&behavior.Answer{Weight: 1, When: lost,
			Search: &behavior.Search{Distance: 6, Heading: h}}).Validate()
		s.Require().ErrorIs(err, behavior.ErrBadSearch, string(h))
		s.Contains(err.Error(), "`search` heading")
	}
}

// TestASearchHeadingSurvivesTheMarshalledRow is the persisted row: the
// heading word rides the row's JSON beside the distance and comes back
// unchanged, so a table saved and loaded never becomes a wordless search.
func (s *TableSuite) TestASearchHeadingSurvivesTheMarshalledRow() {
	for _, h := range behavior.SearchHeadings {
		row := behavior.Answer{Weight: 2, When: []behavior.When{{Enemy: behavior.EnemyLost}},
			Search: &behavior.Search{Distance: 4, Heading: h}}
		raw, err := json.Marshal(row)
		s.Require().NoError(err)
		var back behavior.Answer
		s.Require().NoError(json.Unmarshal(raw, &back))
		s.Require().NotNil(back.Search)
		s.Equal(h, back.Search.Heading)
		s.Equal(4, back.Search.Distance)
	}
}

// TestAnEnemyNamesOneOfTwoTargetPolicies is the sealed target policy
// (rpg-project#669): `enemy` on `attack` and `toward` picks its member
// `closest` or by who hit it (`attacker`); a third word, and an empty one,
// are refused. The dialect fills an omitted policy with `closest` before
// this door, so an empty word here is a struct built past it, the same as a
// search with no heading.
func (s *TableSuite) TestAnEnemyNamesOneOfTwoTargetPolicies() {
	s.Equal([]behavior.TargetPolicy{behavior.TargetClosest, behavior.TargetAttacker}, behavior.TargetPolicies)
	for _, p := range behavior.TargetPolicies {
		s.NoError((&behavior.Answer{Weight: 1,
			Attack: &behavior.Selector{Word: behavior.SelectorEnemy, Target: p}}).Validate(), "attack "+string(p))
		s.NoError((&behavior.Answer{Weight: 1,
			Toward: &behavior.Selector{Word: behavior.SelectorEnemy, Target: p}}).Validate(), "toward "+string(p))
	}
	for _, p := range []behavior.TargetPolicy{"weakest", ""} {
		err := (&behavior.Answer{Weight: 1,
			Attack: &behavior.Selector{Word: behavior.SelectorEnemy, Target: p}}).Validate()
		s.Require().ErrorIs(err, behavior.ErrBadSelector, string(p))
		s.Contains(err.Error(), "`target`")
	}
}

// TestOnlyAnEnemyTakesATargetPolicy is where the policy may stand: it picks
// AMONG enemies, so a cell, a route, a sound or another word takes none, and
// `attacker` is a choice only `attack` and `toward` make (no use case has a
// creature run from the one who hit it rather than the nearest).
func (s *TableSuite) TestOnlyAnEnemyTakesATargetPolicy() {
	cell := spatial.Position{X: 1, Y: 1}
	heard := []behavior.When{{Heard: &behavior.HeardWhen{Kind: lit("clang"), Within: 2}}}
	for name, row := range map[string]behavior.Answer{
		"at":   {Weight: 1, Toward: &behavior.Selector{At: &cell, Target: behavior.TargetClosest}},
		"path": {Weight: 1, Toward: &behavior.Selector{Path: []spatial.Position{cell}, Target: behavior.TargetAttacker}},
		"sound": {Weight: 1, When: heard,
			Toward: &behavior.Selector{Word: behavior.SelectorSound, Target: behavior.TargetClosest}},
		"attacker": {Weight: 1, Attack: &behavior.Selector{Word: behavior.SelectorAttacker, Target: behavior.TargetAttacker}},
		"away from the attacker": {Weight: 1,
			Away: &behavior.Selector{Word: behavior.SelectorEnemy, Target: behavior.TargetAttacker}},
	} {
		err := row.Validate()
		s.Require().ErrorIs(err, behavior.ErrBadSelector, name)
		s.Contains(err.Error(), "`target`", name)
	}
	s.NoError((&behavior.Answer{Weight: 1,
		Away: &behavior.Selector{Word: behavior.SelectorEnemy, Target: behavior.TargetClosest}}).Validate(),
		"away runs from the closest, as it always did")
}

// TestATargetPolicySurvivesTheMarshalledRow is the persisted row: the policy
// rides the selector's JSON and comes back unchanged, so a table saved and
// loaded never becomes an `enemy` with no policy. A selector that is not an
// enemy carries no key for it, so those rows marshal as before.
func (s *TableSuite) TestATargetPolicySurvivesTheMarshalledRow() {
	for _, p := range behavior.TargetPolicies {
		row := behavior.Answer{Weight: 1, Attack: &behavior.Selector{Word: behavior.SelectorEnemy, Target: p}}
		raw, err := json.Marshal(row)
		s.Require().NoError(err)
		s.Contains(string(raw), `"Target":"`+string(p)+`"`)
		var back behavior.Answer
		s.Require().NoError(json.Unmarshal(raw, &back))
		s.Require().NotNil(back.Attack)
		s.Equal(p, back.Attack.Target)
		s.NoError(back.Validate())
	}
	route, err := json.Marshal(behavior.Selector{Path: []spatial.Position{{X: 1, Y: 2}}})
	s.Require().NoError(err)
	s.NotContains(string(route), "Target")
}
