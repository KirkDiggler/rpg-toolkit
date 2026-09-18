// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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
