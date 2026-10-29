// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

// fell_test.go is the `fell` deed (rpg-project#716, lane rpg-project#722 D1):
// a member going down is a deed its witnesses hold, read like `attacked` and
// `fled`, and the one deed whose condition may name WHO it was done to.
//
// This module knows `fell` is a deed word and nothing about dying. Whether a
// fall happened, who saw it and who caused it are the rulebook's; here the
// tests prove the grammar reads the testimony it is handed, and only that.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

type FellSuite struct {
	suite.Suite
}

func TestFellSuite(t *testing.T) { suite.Run(t, new(FellSuite)) }

// fellRow is a table whose ONLY eligible entry is the `fell` row, so the
// chosen index says whether it held: 0 when it did, 1 (the hold) when not.
func fellRow(when behavior.When) behavior.Table {
	return behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{when}, Away: &behavior.Selector{Word: behavior.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
}

// pick rolls `time` with a die that lands on the first candidate.
func (s *FellSuite) pick(table behavior.Table, facts behavior.Facts) int {
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(chosen)

	return chosen.Entry
}

// kingFell is the king's fall as a witness holds it: the barbarian dropped
// him at 10.
var kingFell = behavior.HeldDeed{Kind: behavior.VerbFell, Actor: "barbarian", Target: "king", At: 10}

func (s *FellSuite) TestFellIsAWhenWordForItsOwnVerb() {
	s.Contains(behavior.WhenDeeds, "fell", "the dialect's refusal lists it beside the other deeds")
	s.Equal(behavior.VerbFell, behavior.DeedVerbFor("fell"),
		"the author's word and the store's verb are spelled alike, and still meet in the one mapping")
	s.Equal([]string{"fell"}, behavior.WhoDeeds, "`fell` is the one deed that takes `who`")
}

func (s *FellSuite) TestValidateReadsFellWithAndWithoutWho() {
	for _, tc := range []struct {
		name string
		when behavior.When
	}{
		{name: "fell within 3", when: behavior.When{Deed: "fell", Within: 3}},
		{name: "fell within 3, who: king", when: behavior.When{Deed: "fell", Within: 3, Who: "king"}},
		{name: "fell on an ally, who: king",
			when: behavior.When{Deed: "fell", Within: 3, Who: "king", Scope: behavior.ScopeAlly}},
		{name: "fell as the actor", when: behavior.When{Deed: "fell", Within: 3, Scope: behavior.ScopeActor}},
		{name: "not: fell, who: king",
			when: behavior.When{Not: &behavior.When{Deed: "fell", Within: 3, Who: "king"}}},
	} {
		s.Run(tc.name, func() {
			s.NoError((&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{tc.when}}).Validate())
		})
	}
}

// TestWhoIsRefusedOnEveryOtherDeedByName: `who` names the one who fell, and
// a deed that is not a fall has nobody to name that way. Refused in a
// sentence that names the deed it was written on.
func (s *FellSuite) TestWhoIsRefusedOnEveryOtherDeedByName() {
	for _, word := range []string{"attacked", "intimidated", "persuaded", "fled"} {
		s.Run(word, func() {
			err := (&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{
				{Deed: word, Within: 3, Who: "x"},
			}}).Validate()
			s.Require().ErrorIs(err, behavior.ErrBadWhen)
			s.EqualError(err, "`who` names the one who fell, and only `fell` takes it; `"+word+
				"` does not: behavior: when cannot be read")
		})
	}

	s.Run("beside a condition that is not a deed", func() {
		err := (&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{
			{Fact: lit("alarm-raised"), Who: "king"},
		}}).Validate()
		s.Require().ErrorIs(err, behavior.ErrBadWhen)
		s.EqualError(err, "a `when` entry names `fact` with `who`; `who` belongs only beside `fell`: "+
			"behavior: when cannot be read")
	})

	s.Run("inside a not", func() {
		err := (&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{
			{Not: &behavior.When{Deed: "attacked", Within: 3, Who: "x"}},
		}}).Validate()
		s.Require().ErrorIs(err, behavior.ErrBadWhen)
		s.EqualError(err, "inside `not`: `who` names the one who fell, and only `fell` takes it; "+
			"`attacked` does not: behavior: when cannot be read")
	})
}

// TestWhoIsTheTruthTableOfWhoFell is `who` against the testimony: the fall
// of the member it names holds, the fall of anybody else does not, and a fall
// whose subject the witness could not see names nobody, so a `who` row
// cannot read it, while a row with no `who` reads every fall.
func (s *FellSuite) TestWhoIsTheTruthTableOfWhoFell() {
	cookFell := behavior.HeldDeed{Kind: behavior.VerbFell, Actor: "barbarian", Target: "cook", At: 10}
	unseenFell := behavior.HeldDeed{Kind: behavior.VerbFell, Actor: "barbarian", At: 10}

	for _, tc := range []struct {
		name  string
		who   core.EntityID
		held  []behavior.HeldDeed
		holds bool
	}{
		{name: "who names the one who fell", who: "king", held: []behavior.HeldDeed{kingFell}, holds: true},
		{name: "who names somebody else", who: "king", held: []behavior.HeldDeed{cookFell}, holds: false},
		{name: "the witness could not see who fell", who: "king",
			held: []behavior.HeldDeed{unseenFell}, holds: false},
		{name: "the named fall among others", who: "king",
			held: []behavior.HeldDeed{cookFell, kingFell}, holds: true},
		{name: "no who reads any fall", held: []behavior.HeldDeed{cookFell}, holds: true},
		{name: "no who reads an unseen fall too", held: []behavior.HeldDeed{unseenFell}, holds: true},
		{name: "nothing fell", who: "king", holds: false},
	} {
		s.Run(tc.name, func() {
			table := fellRow(behavior.When{Deed: "fell", Within: 3, Who: tc.who, Scope: behavior.ScopeAlly})
			facts := behavior.Facts{Now: 11, CanMove: true, AllyDeeds: tc.held}
			want := 1
			if tc.holds {
				want = 0
			}
			s.Equal(want, s.pick(table, facts))
		})
	}
}

// TestWhoComparesTheDeedsTargetAndNothingElse: `who` is the one who fell,
// never the one who dropped them, so naming the actor matches nothing.
func (s *FellSuite) TestWhoComparesTheDeedsTargetAndNothingElse() {
	facts := behavior.Facts{Now: 11, CanMove: true, AllyDeeds: []behavior.HeldDeed{kingFell}}
	s.Equal(1, s.pick(fellRow(behavior.When{Deed: "fell", Within: 3, Who: "barbarian", Scope: behavior.ScopeAlly}),
		facts), "the one who dropped him is the actor, not who fell")
}

// TestFellReadsEachScopeLikeTheOtherDeeds: the three readings are the same
// three lists every deed is read from. `on: ally` is a fall on my side,
// `as: actor` is a fall I caused, and the default is my own fall, which a
// creature back on its feet can still remember.
func (s *FellSuite) TestFellReadsEachScopeLikeTheOtherDeeds() {
	mine := behavior.HeldDeed{Kind: behavior.VerbFell, Actor: "barbarian", Target: "me", At: 10}
	dropped := behavior.HeldDeed{Kind: behavior.VerbFell, Actor: "me", Target: "rogue", At: 10}

	for _, tc := range []struct {
		name  string
		scope behavior.DeedScope
		facts behavior.Facts
		holds bool
	}{
		{name: "self reads my own fall", scope: behavior.ScopeSelf,
			facts: behavior.Facts{Deeds: []behavior.HeldDeed{mine}}, holds: true},
		{name: "self does not read an ally's", scope: behavior.ScopeSelf,
			facts: behavior.Facts{AllyDeeds: []behavior.HeldDeed{kingFell}}, holds: false},
		{name: "ally reads an ally's fall", scope: behavior.ScopeAlly,
			facts: behavior.Facts{AllyDeeds: []behavior.HeldDeed{kingFell}}, holds: true},
		{name: "ally does not read my own", scope: behavior.ScopeAlly,
			facts: behavior.Facts{Deeds: []behavior.HeldDeed{mine}}, holds: false},
		{name: "actor reads a fall I caused", scope: behavior.ScopeActor,
			facts: behavior.Facts{OwnDeeds: []behavior.HeldDeed{dropped}}, holds: true},
		{name: "actor does not read a fall I watched", scope: behavior.ScopeActor,
			facts: behavior.Facts{AllyDeeds: []behavior.HeldDeed{kingFell}}, holds: false},
	} {
		s.Run(tc.name, func() {
			tc.facts.Now, tc.facts.CanMove = 11, true
			want := 1
			if tc.holds {
				want = 0
			}
			s.Equal(want, s.pick(fellRow(behavior.When{Deed: "fell", Within: 3, Scope: tc.scope}), tc.facts))
		})
	}
}

// TestAFallAgesOutOfItsSpan: the deed span's rule, for a fall. Seen at 10,
// `within: 3` holds through 13 and not at 14.
func (s *FellSuite) TestAFallAgesOutOfItsSpan() {
	table := fellRow(behavior.When{Deed: "fell", Within: 3, Who: "king", Scope: behavior.ScopeAlly})
	s.Equal(0, s.pick(table, behavior.Facts{Now: 13, CanMove: true, AllyDeeds: []behavior.HeldDeed{kingFell}}))
	s.Equal(1, s.pick(table, behavior.Facts{Now: 14, CanMove: true, AllyDeeds: []behavior.HeldDeed{kingFell}}))
}

// TestAnEmptyWhoIsOmittedWhenMarshalled: a condition with no `who` writes no
// key, so every committed golden stays byte-identical.
func (s *FellSuite) TestAnEmptyWhoIsOmittedWhenMarshalled() {
	plain, err := json.Marshal(behavior.When{Deed: "attacked", Within: 3})
	s.Require().NoError(err)
	s.NotContains(string(plain), "Who")

	named, err := json.Marshal(behavior.When{Deed: "fell", Within: 3, Who: "king"})
	s.Require().NoError(err)
	s.Contains(string(named), `"Who":"king"`)
}
