// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// spatialOrigin is a cell these tests name when a selector has to carry one;
// which cell it is never matters, only that it is one.
var spatialOrigin = spatial.Position{X: 3, Y: 4}

// table_internal_test.go is THE CREATURE'S TABLE, pinned (rpg-project#465):
// the layering, the conditions, the loaded die, and the deal.
//
// EVERY CLAIM HERE IS A UNIT TEST WITH A SEEDED ROLLER, which is the rule this
// slice was briefed under: behaviour is proven by unit tests, and a
// probability is a unit test with a die you control. The scene tests next door
// prove the WIRING — that a verb pays a round, that the world thinks on it,
// that a cowed goblin runs — and say nothing about what a coward does over a
// thousand rolls, which is the one thing a walk could never show.

// --- the dice these tests roll -----------------------------------------------

// facedRoller answers a face the test named, and records the die it was asked
// for so a test can assert the loaded total without reading it off the beat.
type facedRoller struct {
	face int
	of   int
}

func (r *facedRoller) Roll(_ context.Context, size int) (int, error) {
	r.of = size
	if r.face > size {
		return size, nil
	}

	return r.face, nil
}

func (r *facedRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
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

// seededRoller is a real distribution from a fixed seed: the only honest way
// to claim "a coward runs more often than a soldier does" is to roll it many
// times, and the only way to claim it REPEATABLY is to say which seed.
type seededRoller struct{ r *rand.Rand }

func newSeededRoller(seed int64) *seededRoller {
	return &seededRoller{r: rand.New(rand.NewSource(seed))} //nolint:gosec // a test's die, not a secret
}

func (s *seededRoller) Roll(_ context.Context, size int) (int, error) {
	return s.r.Intn(size) + 1, nil
}

func (s *seededRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
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
	soldierProfile    = TemperProfile{Attack: 100, Toward: 100, Away: 100, Flee: 100, Hold: 100}
	cowardProfile     = TemperProfile{Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100}
	aggressiveProfile = TemperProfile{Attack: 300, Toward: 300, Away: 25, Flee: 25, Hold: 50}
)

type TableSuite struct {
	suite.Suite
}

func TestTableSuite(t *testing.T) { suite.Run(t, new(TableSuite)) }

// --- Layer -------------------------------------------------------------------

// TestLayerTakesTheNearestKeyWholesale is design §1: three layers supply
// tables and for each key the nearest one that names it wins ENTIRELY. There
// is no merging of entry lists, so an author never has to reason about what
// was added to what — and the cost of overriding a key is that the layer
// underneath's entries for it are gone, visibly.
func (s *TableSuite) TestLayerTakesTheNearestKeyWholesale() {
	base := Table{
		AnswerTime: {
			{Weight: 3, When: &When{Enemy: EnemySeen}, Attack: &Selector{Word: SelectorEnemy}},
			{Weight: 1, Hold: true},
		},
		AnswerIntimidated: {{Weight: 1, Say: "the rulebook's line"}},
	}
	over := Table{AnswerTime: {{Weight: 1, Hold: true}}}

	laid := Layer(base, over)

	s.Require().Len(laid[AnswerTime], 1,
		"the nearer layer's `time` key replaced the default's two entries entirely")
	s.True(laid[AnswerTime][0].Hold)
	s.Equal(base[AnswerIntimidated], laid[AnswerIntimidated],
		"a key the nearer layer is silent about is the further layer's, untouched")
}

// TestLayerMutatesNeitherInput is why a rulebook's default table can be one
// value shared by every creature of its kind: a layering that wrote through to
// it would give one goblin's orders to every goblin in the game.
func (s *TableSuite) TestLayerMutatesNeitherInput() {
	base := Table{AnswerTime: {{Weight: 1, Hold: true}}}
	over := Table{AnswerIntimidated: {{Weight: 1, Say: "hi"}}}

	laid := Layer(base, over)
	laid[AnswerTime][0].Weight = 99
	laid[AnswerIntimidated] = nil
	laid[AnswerPersuaded] = []Answer{{Weight: 1}}

	s.Equal(1, base[AnswerTime][0].Weight, "the base's own entry is unchanged")
	s.Len(base, 1, "and nothing was added to it")
	s.Len(over, 1, "nor to the layer laid over it")
	s.NotEmpty(over[AnswerIntimidated])
}

// TestLayerOnNothingIsNothing: a creature with no table anywhere answers
// nothing and rolls nothing, and nil is how that is said.
func (s *TableSuite) TestLayerOnNothingIsNothing() {
	s.Nil(Layer(nil, nil))
	s.Nil(Layer(Table{}, Table{}))

	over := Table{AnswerTime: {{Weight: 1, Hold: true}}}
	s.Equal(over, Layer(nil, over), "an empty base is the layer laid over it")
}

// --- when --------------------------------------------------------------------

// TestEnemyConditionsReadTheTwoBooleans pins all three values of `enemy:` —
// and that `seen` and `remembered` are EXCLUSIVE, which is what makes an
// author able to write one entry for each and know exactly one is on the
// table.
func (s *TableSuite) TestEnemyConditionsReadTheTwoBooleans() {
	table := Table{AnswerTime: {
		{Weight: 1, When: &When{Enemy: EnemySeen}, Attack: &Selector{Word: SelectorEnemy}},
		{Weight: 1, When: &When{Enemy: EnemyRemembered}, Toward: &Selector{Word: SelectorEnemy}},
		{Weight: 1, When: &When{Enemy: EnemyNone}, Hold: true},
	}}

	for _, tc := range []struct {
		name  string
		facts Facts
		entry int
	}{
		{name: "in sight", facts: Facts{EnemySeen: true}, entry: 0},
		{name: "lost sight of", facts: Facts{EnemyRemembered: true}, entry: 1},
		{name: "never seen", facts: Facts{}, entry: 2},
	} {
		s.Run(tc.name, func() {
			roller := &facedRoller{face: 1}
			chosen, err := pick(context.Background(), AnswerTime, table, Temper{}, tc.facts, roller)
			s.Require().NoError(err)
			s.Require().NotNil(chosen)
			s.Require().Len(chosen.Candidates, 1, "exactly one condition holds, so exactly one entry is on the table")
			s.Equal(tc.entry, chosen.Entry)
		})
	}
}

// TestASpanIsCountedFromOneAndEndsWhenItSaysSo is the `within` boundary, both
// sides of it. A deed landed N rounds ago still counts; N+1 does not.
func (s *TableSuite) TestASpanIsCountedFromOneAndEndsWhenItSaysSo() {
	table := Table{AnswerTime: {
		{Weight: 1, When: &When{Deed: "fled", Within: 3}, Away: &Selector{Word: SelectorActor}},
		{Weight: 1, Hold: true},
	}}

	for _, tc := range []struct {
		name  string
		now   uint64
		entry int
	}{
		{name: "the round it landed", now: 10, entry: 0},
		{name: "one round later", now: 11, entry: 0},
		{name: "three rounds later, the last that counts", now: 13, entry: 0},
		{name: "four rounds later, the span has run out", now: 14, entry: 1},
	} {
		s.Run(tc.name, func() {
			facts := Facts{
				Now:   tc.now,
				Deeds: []HeldDeed{{Kind: DeedFled, Actor: "alice", At: 10}},
			}
			roller := &facedRoller{face: 1}
			chosen, err := pick(context.Background(), AnswerTime, table, Temper{}, facts, roller)
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
	table := Table{AnswerTime: {
		{Weight: 1, When: &When{Deed: "fled", Within: 3}, Away: &Selector{Word: SelectorActor}},
		{Weight: 1, Hold: true},
	}}
	facts := Facts{Now: 11, Deeds: []HeldDeed{{Kind: DeedAttack, Actor: "alice", At: 10}}}

	chosen, err := pick(context.Background(), AnswerTime, table, Temper{}, facts, &facedRoller{face: 1})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)
	s.Equal(1, chosen.Entry, "the `fled` condition does not read an `attack` deed")
}

// --- the loaded die ----------------------------------------------------------

// TestTheBeatsArithmeticAddsUp is R7: every candidate carries its authored
// weight, its temperament's factor and their product, and the products sum to
// the die that was rolled. A reader has to be able to add it up.
func (s *TableSuite) TestTheBeatsArithmeticAddsUp() {
	table := Table{AnswerTime: {
		{Weight: 3, Attack: &Selector{Word: SelectorEnemy}},
		{Weight: 1, Away: &Selector{Word: SelectorEnemy}},
	}}
	temper := Temper{Word: "coward", Profile: cowardProfile}

	roller := &facedRoller{face: 1}
	chosen, err := pick(context.Background(), AnswerTime, table, temper, Facts{}, roller)
	s.Require().NoError(err)
	s.Require().NotNil(chosen)

	s.Require().Len(chosen.Candidates, 2)
	s.Equal(Candidate{Entry: 0, Weight: 3, Percent: 50, Loaded: 150}, chosen.Candidates[0])
	s.Equal(Candidate{Entry: 1, Weight: 1, Percent: 300, Loaded: 300}, chosen.Candidates[1])

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
	table := Table{AnswerTime: {
		{Weight: 3, Attack: &Selector{Word: SelectorEnemy}},
		{Weight: 1, Away: &Selector{Word: SelectorEnemy}},
	}}

	none, err := pick(context.Background(), AnswerTime, table, Temper{}, Facts{}, &facedRoller{face: 1})
	s.Require().NoError(err)
	soldier, err := pick(context.Background(), AnswerTime, table,
		Temper{Profile: soldierProfile}, Facts{}, &facedRoller{face: 1})
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
	table := Table{AnswerTime: {
		{Weight: 70, Hold: true},
		{Weight: 30, Away: &Selector{Word: SelectorEnemy}},
	}}

	const rolls = 1000
	for _, tc := range []struct {
		name      string
		temper    Temper
		of        int
		wantRuns  float64
		tolerance float64
	}{
		{
			// 70×100 hold against 30×300 away: 7000 and 9000 of 16000.
			name:   "coward",
			temper: Temper{Word: "coward", Profile: cowardProfile},
			of:     16000, wantRuns: 56.25, tolerance: 5,
		},
		{
			// 70×50 hold against 30×25 away: 3500 and 750 of 4250.
			name:   "aggressive",
			temper: Temper{Word: "aggressive", Profile: aggressiveProfile},
			of:     4250, wantRuns: 17.6, tolerance: 5,
		},
		{
			// Unloaded: the author's own 70/30.
			name:   "soldier",
			temper: Temper{Word: "soldier", Profile: soldierProfile},
			of:     10000, wantRuns: 30, tolerance: 5,
		},
	} {
		s.Run(tc.name, func() {
			roller := newSeededRoller(20260918)
			ran := 0
			for i := 0; i < rolls; i++ {
				chosen, err := pick(context.Background(), AnswerTime, table, tc.temper, Facts{}, roller)
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
	table := Table{AnswerTime: {
		{Weight: 1, When: &When{Enemy: EnemySeen}, Attack: &Selector{Word: SelectorEnemy}},
	}}

	chosen, err := pick(context.Background(), AnswerTime, table, Temper{}, Facts{}, &facedRoller{face: 1})
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
	chosen, err := pick(context.Background(), AnswerIntimidated, Table{}, Temper{}, Facts{}, &facedRoller{face: 1})
	s.Require().NoError(err)
	s.Nil(chosen)
}

// TestAPickWithNoDieIsRefusedByName: the roller is optional at construction
// because a scene with no table rolls nothing. The moment something does, its
// absence is [ErrNoRoller] rather than a silent default.
func (s *TableSuite) TestAPickWithNoDieIsRefusedByName() {
	table := Table{AnswerTime: {{Weight: 1, Hold: true}}}

	_, err := pick(context.Background(), AnswerTime, table, Temper{}, Facts{}, nil)
	s.Require().Error(err)
	s.ErrorIs(err, ErrNoRoller)
}

// --- the deal ----------------------------------------------------------------

// TestAMixDealsEveryWordItNames is design §3: four goblins, one table, four
// behaviours. The mix is walked in sorted order so the same seed deals the same
// word every run (C8), and over enough rolls every word in it comes up.
func (s *TableSuite) TestAMixDealsEveryWordItNames() {
	mix := Temper{
		Mix: map[string]int{"coward": 1, "soldier": 2, "aggressive": 1},
		Profiles: map[string]TemperProfile{
			"coward": cowardProfile, "soldier": soldierProfile, "aggressive": aggressiveProfile,
		},
	}

	roller := newSeededRoller(20260918)
	seen := map[string]int{}
	for i := 0; i < 400; i++ {
		dealt, roll, of, err := dealTemper(context.Background(), mix, roller)
		s.Require().NoError(err)
		s.Equal(4, of, "the die is the sum of the shares the author wrote")
		s.GreaterOrEqual(roll, 1)
		s.LessOrEqual(roll, of)
		s.Empty(dealt.Mix, "what comes out of a deal is a creature, not a spread")
		seen[dealt.Word]++
	}

	s.Len(seen, 3, "every word in the mix was dealt at least once: %v", seen)
	s.Greater(seen["soldier"], seen["coward"], "a share of two comes up more often than a share of one")
}

// TestADealtWordCarriesItsProfile: the caller supplies what each word MEANS,
// and the deal hands back the one that was chosen — because this module cannot
// import the rulebook the numbers live in (C1).
func (s *TableSuite) TestADealtWordCarriesItsProfile() {
	mix := Temper{
		Mix:      map[string]int{"coward": 1},
		Profiles: map[string]TemperProfile{"coward": cowardProfile},
	}

	dealt, roll, of, err := dealTemper(context.Background(), mix, &facedRoller{face: 1})
	s.Require().NoError(err)
	s.Equal("coward", dealt.Word)
	s.Equal(cowardProfile, dealt.Profile)
	s.Equal(1, roll)
	s.Equal(1, of)
}

// TestAMixNamingAWordNothingDefinesIsRefused is fail-closed at the door: a
// temperament dealt with no profile behind it would silently apply as a
// soldier, and the creature would look like an authoring success.
func (s *TableSuite) TestAMixNamingAWordNothingDefinesIsRefused() {
	mix := Temper{Mix: map[string]int{"coward": 1}}

	_, _, _, err := dealTemper(context.Background(), mix, &facedRoller{face: 1})
	s.Require().Error(err)
	s.ErrorIs(err, ErrBadTemper)
	s.Contains(err.Error(), "coward")
}

// TestAShareBelowOneCanNeverBeDealt: the same rule a weight keeps, one layer
// up. A zero share is a temperament sitting in a file looking like a
// possibility.
func (s *TableSuite) TestAShareBelowOneCanNeverBeDealt() {
	mix := Temper{
		Mix:      map[string]int{"coward": 0},
		Profiles: map[string]TemperProfile{"coward": cowardProfile},
	}

	_, _, _, err := dealTemper(context.Background(), mix, &facedRoller{face: 1})
	s.Require().Error(err)
	s.ErrorIs(err, ErrBadTemper)
}

// TestTheTemperWordsAreSealedHereToo: the words live twice on purpose — the
// profiles are rulebook content and this module cannot import a rulebook, but
// it still has to refuse `temper: brave` on the author's form rather than at
// the table. A test in the session module is where the two lists are checked
// against each other.
func (s *TableSuite) TestTheTemperWordsAreSealedHereToo() {
	s.Equal([]string{"soldier", "coward", "aggressive"}, TemperWords)
	for _, word := range TemperWords {
		s.True(ValidTemperWord(word))
	}
	s.False(ValidTemperWord("brave"))
	s.False(ValidTemperWord(""))
}

// --- validation --------------------------------------------------------------

// TestATableThisBuildCannotRollIsRefusedAtTheDoor pins every refusal
// [validateTable] makes, which is every refusal a persisted blob somebody
// edited has to meet as well as an authored file.
func (s *TableSuite) TestATableThisBuildCannotRollIsRefusedAtTheDoor() {
	for _, tc := range []struct {
		name  string
		table Table
		says  string
	}{
		{
			name:  "a trigger nobody designed",
			table: Table{"bribed": {{Weight: 1, Say: "fine"}}},
			says:  "not a trigger this build rolls",
		},
		{
			name:  "a weight that can never be rolled",
			table: Table{AnswerTime: {{Weight: 0, Hold: true}}},
			says:  "weighs 0",
		},
		{
			name:  "a time word under a social key",
			table: Table{AnswerIntimidated: {{Weight: 1, Hold: true}}},
			says:  "what a creature does with time",
		},
		{
			name:  "a social word under time",
			table: Table{AnswerTime: {{Weight: 1, Flee: true}}},
			says:  "answers a social verdict",
		},
		{
			name:  "a condition under a social key",
			table: Table{AnswerIntimidated: {{Weight: 1, Say: "hi", When: &When{Enemy: EnemySeen}}}},
			says:  "already the condition",
		},
		{
			name:  "a `when` that names two things",
			table: Table{AnswerTime: {{Weight: 1, Hold: true, When: &When{Enemy: EnemySeen, Deed: "fled", Within: 1}}}},
			says:  "one condition, and this is two",
		},
		{
			name:  "a `when` that names nothing",
			table: Table{AnswerTime: {{Weight: 1, Hold: true, When: &When{}}}},
			says:  "omit it",
		},
		{
			name:  "an enemy word this build does not read",
			table: Table{AnswerTime: {{Weight: 1, Hold: true, When: &When{Enemy: "nearby"}}}},
			says:  "not a condition this build reads",
		},
		{
			name:  "a deed this build does not hold",
			table: Table{AnswerTime: {{Weight: 1, Hold: true, When: &When{Deed: "insulted", Within: 1}}}},
			says:  "not a deed this build holds",
		},
		{
			name:  "a span counted from zero",
			table: Table{AnswerTime: {{Weight: 1, Hold: true, When: &When{Deed: "fled", Within: 0}}}},
			says:  "counted from 1",
		},
		{
			name: "`actor` with no deed to have been the actor of",
			table: Table{AnswerTime: {{Weight: 1, Away: &Selector{Word: SelectorActor},
				When: &When{Enemy: EnemySeen}}}},
			says: "this entry names none",
		},
		{
			name:  "a cell on a word that acts on a creature",
			table: Table{AnswerTime: {{Weight: 1, Attack: &Selector{At: &spatialOrigin}}}},
			says:  "somewhere to walk toward",
		},
	} {
		s.Run(tc.name, func() {
			err := validateTable(tc.table)
			s.Require().Error(err)
			s.ErrorIs(err, ErrBadAnswer)
			s.Contains(err.Error(), tc.says)
		})
	}
}

// TestAWholeLegalTableIsAccepted is the control the refusals above need: every
// word, every condition and every selector this build ships, in one table that
// passes.
func (s *TableSuite) TestAWholeLegalTableIsAccepted() {
	cell := spatialOrigin
	require.NoError(s.T(), validateTable(Table{
		AnswerIntimidated:      {{Weight: 70, Say: "Fine, fine!", Fact: "camp-cowed"}, {Weight: 30, Flee: true}},
		AnswerIntimidateFailed: {{Weight: 1, Say: "Big talk."}},
		AnswerPersuaded:        {{Weight: 1, Fact: "camp-cowed"}},
		AnswerPersuadeFailed:   {{Weight: 1, Say: "Nothing down there."}},
		AnswerTime: {
			{Weight: 3, When: &When{Deed: "attacked", Within: 3}, Attack: &Selector{Word: SelectorAttacker}},
			{Weight: 1, When: &When{Enemy: EnemySeen}, Attack: &Selector{Word: SelectorEnemy}},
			{Weight: 1, When: &When{Enemy: EnemyRemembered}, Toward: &Selector{Word: SelectorEnemy}},
			{Weight: 1, When: &When{Enemy: EnemyNone}, Toward: &Selector{At: &cell}},
			{Weight: 5, When: &When{Deed: "fled", Within: 3}, Away: &Selector{Word: SelectorActor}},
			{Weight: 1, Hold: true},
		},
	}))
}
