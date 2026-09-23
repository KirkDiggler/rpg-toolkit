// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// table_internal_test.go is THIS RULEBOOK'S HALF of the creature's table
// (rpg-project#465): the triggers a D&D verb settles, the words that are legal
// under each, the temperament vocabulary this build ships, and the projection
// that keeps the enemy bands exclusive.
//
// THE EVALUATOR IS NOT TESTED HERE ANY MORE. Layer, the bands, the `within`
// boundary, the loaded arithmetic, the distributions and the deal moved to
// mind/behavior with the code, and they run there with no board, no clock and
// no rulebook. What is left is what this module actually decides.

// spatialOrigin is a cell these tests name when a selector has to carry one;
// which cell it is never matters, only that it is one.
var spatialOrigin = spatial.Position{X: 3, Y: 4}

type TableSuite struct {
	suite.Suite
}

func TestTableSuite(t *testing.T) { suite.Run(t, new(TableSuite)) }

// --- the triggers this rulebook names ----------------------------------------

// TestTheSocialKeysAreThisRulebooksAndTimeIsNot is the line the move drew.
// `intimidated` is a D&D verb settling and lives here with the validation that
// refuses everything else; a creature HAVING TIME is not a D&D idea, so
// mind/behavior names it and this build simply uses the name.
func (s *TableSuite) TestTheSocialKeysAreThisRulebooksAndTimeIsNot() {
	s.Equal([]AnswerKey{
		AnswerIntimidated, AnswerIntimidateFailed, AnswerPersuaded, AnswerPersuadeFailed,
	}, AnswerKeys, "each verb's success then its failure, in authored order")

	s.Equal(behavior.KeyTime, AnswerTime, "the one key that came from the module")
	s.Equal(append(append([]AnswerKey(nil), AnswerKeys...), AnswerTime), TableKeys)

	for _, key := range TableKeys {
		s.True(validTableKey(key), "%q is a key this build rolls", key)
	}
	s.False(validTableKey("bribed"), "and a key nobody designed is not")
}

// --- the bands, as this composition projects them -----------------------------

// TestTheProjectionIsWhatKeepsTheBandsExclusive: the flags are narrowed where
// they are built, not where they are read, so there is one place the
// --- the temperament vocabulary ---

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

// --- validation ---

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
			// THE SCOPE IS THE ONE FIELD OF THIS CONDITION THAT WAS NOT
			// REFUSED HERE (rpg-toolkit#1890 thread 1). This function is the
			// door for a hand-edited persisted blob, so an unknown scope must
			// be named rather than reaching the evaluator, where it matches
			// nothing and would silently revert an `on: ally` row.
			name:  "a scope this build does not read",
			table: Table{AnswerTime: {{Weight: 1, Hold: true, When: &When{Deed: "attacked", Within: 1, Scope: DeedScope("banana")}}}},
			says:  "not a scope this build reads",
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

// --- the cause each direction carries ------------------------------------------

// TestEachDirectionNamesItselfInTheCause pins the only thing on the wire that
// tells a table's two walks apart.
//
// BOTH LEAVE AS A [Routed] — a walk to an authored cell and a run from an
// enemy are each the creature obeying its own orders through the engine's
// router — and the `moved` and `stayed` beats carry the cause and nothing else
// about the intent. One ref for both read the bandits' walk to the front room
// as a rout: a story that answers wrongly, which is worse than one that does
// not answer.
func (s *TableSuite) TestEachDirectionNamesItselfInTheCause() {
	d := TableDriver{}
	cell := spatialOrigin
	budget := TurnBudget{MovementFeet: 30}

	toward, ok := d.towardIntent(
		MonsterView{Budget: budget},
		Answer{Toward: &Selector{At: &cell}},
	).(Routed)
	s.Require().True(ok, "an authored cell goes out routed")
	s.Equal("encounter:table:toward", toward.Cause.String(), "the word the author wrote")

	away, ok := d.awayIntent(
		MonsterView{
			Budget: budget,
			Seen: []SeenMember{{
				ID: "intruder", Opposed: true, Standing: true, DistanceCells: 2,
			}},
		},
		Answer{Away: &Selector{Word: SelectorEnemy}},
	).(Routed)
	s.Require().True(ok, "and so does a run")
	s.Equal("encounter:table:away", away.Cause.String(), "which is not the same word")

	s.NotEqual(tableCauseToward, tableCauseAway, "two directions, two causes")
}

// ---------------------------------------------------------------------------
// The pause, through the real driver (rpg-toolkit#1883, rpg-project#498)
// ---------------------------------------------------------------------------
//
// THE CLAIM THIS SLICE WAS RULED ON: "after I strike, stand still for two
// rounds" is expressible without a new concept in the grammar. It is
// `{ attacked: { within: 2, as: actor } }` — a deed the creature DID — read
// against OwnDeeds.
//
// These go through TableDriver.Act, which is the path a `time` pick actually
// takes, so what they prove is the ability to fire and not only that a
// projection exists. The driver is driven directly with a view, exactly as the
// other tests in this file do: no board and no clock are needed to ask what a
// creature does with its turn.

// lowestDie answers every roll with 1, so a weighted table picks the entry a
// test named without sampling. It is declared HERE rather than reached for
// from testroller_test.go because that file is the EXTERNAL test package and
// this one is internal — the boundary the toolkit keeps between tests that see
// internals and tests that do not.
type lowestDie struct{}

func (lowestDie) Roll(_ context.Context, _ int) (int, error) { return 1, nil }

func (lowestDie) RollN(_ context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = 1
	}

	return out, nil
}

// pauseTable is a guard that opens a fight, then stands still for two rounds
// before swinging again — and holds when there is nobody, so the table is
// never empty.
func pauseTable() Table {
	return Table{AnswerTime: {
		{Weight: 100, Hold: true,
			When: &When{Deed: "attacked", Within: 2, Scope: ScopeActor}},
		{Weight: 1, Attack: &Selector{Word: SelectorEnemy},
			When: &When{Enemy: EnemyReach}},
		{Weight: 1, Hold: true, When: &When{Enemy: EnemyNone}},
	}}
}

// pauseView is a guard with a weapon in hand and an enemy in reach, having
// acted `ago` rounds before `now`.
func pauseView(now uint64, ago int) MonsterView {
	return MonsterView{
		Self:    "guard",
		Actions: []ActionView{{Ref: core.Ref{Type: "weapons", ID: "spear"}, RangeFeet: 5}},
		Seen: []SeenMember{{
			ID: "intruder", Opposed: true, Standing: true, InReach: map[core.Ref]bool{{Type: "weapons", ID: "spear"}: true},
		}},
		Budget:   TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Table:    pauseTable(),
		At:       now,
		Round:    1,
		OwnDeeds: []HeldDeed{{Kind: DeedAttack, Actor: "guard", At: now - uint64(ago)}},
	}
}

func (s *TableSuite) TestAPauseHoldsTheRoundAfterIStruck() {
	// THE ROUND ITSELF: the creature struck at `now`, so the pause is on and
	// it stands there — even though an enemy is in reach and it could swing.
	d := TableDriver{Roller: lowestDie{}}
	decision, err := d.Act(pauseView(10, 0))
	s.Require().NoError(err)
	s.IsType(Pass{}, decision.Intent, "the round I struck, I stand still")

	// ONE ROUND LATER, still inside the span.
	decision, err = d.Act(pauseView(11, 1))
	s.Require().NoError(err)
	s.IsType(Pass{}, decision.Intent, "still inside the two-round span")

	// THE SPAN RUNS OUT and the guard is free to act again — which is what
	// makes this a pause rather than a creature that stopped fighting.
	decision, err = d.Act(pauseView(13, 3))
	s.Require().NoError(err)
	s.IsType(Attack{}, decision.Intent, "three rounds on, the pause is over and it swings")
}

func (s *TableSuite) TestBeingHitIsNotAPause() {
	// THE TWO READINGS STAY APART: a blow TO the guard is `Deeds`, which is
	// not what the pause asks for. Without this the guard would freeze every
	// time it was wounded instead of retaliating.
	d := TableDriver{Roller: lowestDie{}}
	view := pauseView(10, 0)
	view.OwnDeeds = nil
	view.Deeds = []HeldDeed{{Kind: DeedAttack, Actor: "intruder", At: 10}}

	decision, err := d.Act(view)
	s.Require().NoError(err)
	s.IsType(Attack{}, decision.Intent, "being struck does not silence the guard")
}

// TestTheAllyReadingFeedsARowThatFires is the second half of the objective:
// attrition morale. A creature whose SIDE has been hurt closes on the enemy
// rather than holding — the same machinery, the other reading.
func (s *TableSuite) TestTheAllyReadingFeedsARowThatFires() {
	table := Table{AnswerTime: {
		// "A friend has been struck in the last three rounds: avenge it."
		{Weight: 100, Attack: &Selector{Word: SelectorEnemy},
			When: &When{Deed: "attacked", Within: 3, Scope: ScopeAlly}},
		{Weight: 1, Hold: true},
	}}

	d := TableDriver{Roller: lowestDie{}}
	view := MonsterView{
		Self:    "goblin",
		Actions: []ActionView{{Ref: core.Ref{Type: "weapons", ID: "scimitar"}, RangeFeet: 5}},
		Seen: []SeenMember{{
			ID: "intruder", Opposed: true, Standing: true, InReach: map[core.Ref]bool{{Type: "weapons", ID: "scimitar"}: true},
		}},
		Budget: TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Table:  table,
		At:     10,
		Round:  1,
	}

	// Nothing has happened to its side: the row is not on the table.
	decision, err := d.Act(view)
	s.Require().NoError(err)
	s.IsType(Pass{}, decision.Intent, "nobody on my side has been hurt")

	// A friend was struck: the row fires and the goblin attacks.
	view.AllyDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "intruder", At: 10}}
	decision, err = d.Act(view)
	s.Require().NoError(err)
	s.IsType(Attack{}, decision.Intent, "an ally's wound puts the row on the table")

	// And it ages out like every other span.
	view.At = 20
	decision, err = d.Act(view)
	s.Require().NoError(err)
	s.IsType(Pass{}, decision.Intent, "the span is a span, whoever it was about")
}
