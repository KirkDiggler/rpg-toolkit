// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// HitPointsSuite is the `hit_points` condition (rpg-project#1131, ruled
// 2026-11-28): a row asks whether its creature's own hit points are at most a
// percent of its maximum, both numbers the caller's.
type HitPointsSuite struct {
	suite.Suite
}

func TestHitPointsSuite(t *testing.T) { suite.Run(t, new(HitPointsSuite)) }

func (s *HitPointsSuite) pick(table behavior.Table, facts behavior.Facts) *behavior.PickOutput {
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	return out
}

func atMost(n int) behavior.When {
	return behavior.When{HitPoints: &behavior.HitPointsWhen{AtMost: n, Percent: true}}
}

// candidate reports whether row 0 of a table is a candidate with these facts.
func (s *HitPointsSuite) candidate(table behavior.Table, facts behavior.Facts) bool {
	return s.pick(table, facts).Entry == 0
}

func (s *HitPointsSuite) TestAtOrBelowTheShareAndNotAbove() {
	table := behavior.Table{behavior.KeyTime: {{Weight: 1, When: []behavior.When{atMost(30)}, Hold: true}}}
	for name, tc := range map[string]struct {
		hp, max int
		holds   bool
	}{
		"2 of 9 holds":               {2, 9, true},
		"3 of 9 does not":            {3, 9, false},
		"3 of 10 holds, boundary in": {3, 10, true},
		"31 of 100 does not":         {31, 100, false},
		"0 of 9 holds":               {0, 9, true},
		"9 of 9 does not":            {9, 9, false},
	} {
		s.Equal(tc.holds, s.candidate(table, behavior.Facts{HitPoints: tc.hp, MaxHitPoints: tc.max}), name)
	}
}

func (s *HitPointsSuite) TestNotToldHoldsNothing() {
	table := behavior.Table{behavior.KeyTime: {{Weight: 1, When: []behavior.When{atMost(30)}, Hold: true}}}
	s.False(s.candidate(table, behavior.Facts{}), "nothing told")
	s.False(s.candidate(table, behavior.Facts{HitPoints: 5}), "no maximum")
	s.False(s.candidate(table, behavior.Facts{HitPoints: 0, MaxHitPoints: -3}), "a negative maximum")
	s.False(s.candidate(table, behavior.Facts{HitPoints: -5, MaxHitPoints: -3}), "a negative maximum, negative hit points")
}

// TestNotIsPlainNegation: as around every condition, so a creature the host
// told nothing about holds `not` (threat's rule: the empty answer is "I did
// not judge it", and negation is plain).
func (s *HitPointsSuite) TestNotIsPlainNegation() {
	inner := atMost(30)
	table := behavior.Table{behavior.KeyTime: {{Weight: 1, When: []behavior.When{{Not: &inner}}, Hold: true}}}
	s.True(s.candidate(table, behavior.Facts{HitPoints: 5, MaxHitPoints: 9}))
	s.False(s.candidate(table, behavior.Facts{HitPoints: 2, MaxHitPoints: 9}))
	s.True(s.candidate(table, behavior.Facts{}), "nothing told: `not` of a condition that holds nothing holds")
}

func (s *HitPointsSuite) TestWithAnotherConditionBothMustHold() {
	table := behavior.Table{behavior.KeyTime: {{Weight: 1,
		When: []behavior.When{atMost(30), {Enemy: behavior.EnemyReach}}, Hold: true}}}
	low := behavior.Facts{HitPoints: 2, MaxHitPoints: 9}
	s.False(s.candidate(table, low), "low, no foe in reach")
	low.EnemyInReach = true
	s.True(s.candidate(table, low), "low and a foe in reach")
	high := behavior.Facts{HitPoints: 5, MaxHitPoints: 9, EnemyInReach: true}
	s.False(s.candidate(table, high), "a foe in reach but not low")
}

func (s *HitPointsSuite) TestWithTheRowItWasBuiltFor() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{atMost(30)}, Activate: "second wind"},
		{Weight: 1, Hold: true},
	}}
	both := s.pick(table, behavior.Facts{HitPoints: 2, MaxHitPoints: 9, CanActivate: []string{"second wind"}})
	s.Equal(0, both.Entry, "low and able: the ability row")

	notLow := s.pick(table, behavior.Facts{HitPoints: 5, MaxHitPoints: 9, CanActivate: []string{"second wind"}})
	s.Equal(1, notLow.Entry, "able but not low: the hold")

	cannot := s.pick(table, behavior.Facts{HitPoints: 2, MaxHitPoints: 9})
	s.Equal(1, cannot.Entry, "low but cannot activate: the hold")
}

func (s *HitPointsSuite) TestTheNumberIsJudged() {
	for _, n := range []int{0, 101, -5} {
		w := atMost(n)
		err := w.Validate()
		s.Require().ErrorIs(err, behavior.ErrBadWhen, "%d", n)
	}
	w := atMost(101)
	s.EqualError(w.Validate(), "`hit_points` at most 101%: a share of the creature's own maximum is a "+
		"whole percent from 1 to 100: behavior: when cannot be read")
	for _, n := range []int{1, 30, 100} {
		w := atMost(n)
		s.NoError(w.Validate(), "%d", n)
	}
}

func raw(n int) behavior.When {
	return behavior.When{HitPoints: &behavior.HitPointsWhen{AtMost: n}}
}

func share(n int) behavior.When {
	return behavior.When{HitPoints: &behavior.HitPointsWhen{AtMost: n, Percent: true}}
}

func (s *HitPointsSuite) rowFor(w behavior.When) behavior.Table {
	return behavior.Table{behavior.KeyTime: {{Weight: 1, When: []behavior.When{w}, Hold: true}}}
}

func (s *HitPointsSuite) TestARawNumberIsHitPoints() {
	table := s.rowFor(raw(8))
	for name, tc := range map[string]struct {
		hp, max int
		holds   bool
	}{
		"8 of 27 holds":                 {8, 27, true},
		"9 of 27 does not":              {9, 27, false},
		"8 of 10 holds, max is ignored": {8, 10, true},
		"0 of 27 holds":                 {0, 27, true},
	} {
		s.Equal(tc.holds, s.candidate(table, behavior.Facts{HitPoints: tc.hp, MaxHitPoints: tc.max}), name)
	}
}

func (s *HitPointsSuite) TestAShareIsOfTheMaximum() {
	table := s.rowFor(share(30))
	for name, tc := range map[string]struct {
		hp, max int
		holds   bool
	}{
		"8 of 27 holds":    {8, 27, true},
		"9 of 27 does not": {9, 27, false},
		"3 of 10 holds":    {3, 10, true},
		"2 of 9 holds":     {2, 9, true},
		"3 of 9 does not":  {3, 9, false},
	} {
		s.Equal(tc.holds, s.candidate(table, behavior.Facts{HitPoints: tc.hp, MaxHitPoints: tc.max}), name)
	}
}

func (s *HitPointsSuite) TestTheSameNumberMeansDifferentThings() {
	full := behavior.Facts{HitPoints: 27, MaxHitPoints: 27}
	s.True(s.candidate(s.rowFor(raw(30)), full), "30 hit points or fewer: he has 27")
	s.False(s.candidate(s.rowFor(share(30)), full), "30 percent: he is at 100")
}

func (s *HitPointsSuite) TestNotToldHoldsNothingInEitherForm() {
	for name, w := range map[string]behavior.When{"raw": raw(8), "share": share(30)} {
		s.False(s.candidate(s.rowFor(w), behavior.Facts{}), name+": nothing told")
	}
	s.False(s.candidate(s.rowFor(raw(8)), behavior.Facts{HitPoints: 0, MaxHitPoints: 0}),
		"a raw number with no maximum told is not nearly dead")
}

func (s *HitPointsSuite) TestEachNumberIsJudgedInItsOwnUnit() {
	for _, n := range []int{0, -5} {
		w := raw(n)
		s.Require().ErrorIs(w.Validate(), behavior.ErrBadWhen, "raw %d", n)
	}
	w := raw(0)
	s.EqualError(w.Validate(), "`hit_points` at most 0: a number of hit points is 1 or more: "+
		"behavior: when cannot be read")
	for _, n := range []int{1, 5000} {
		w := raw(n)
		s.NoError(w.Validate(), "raw %d", n)
	}
	for _, n := range []int{0, 101, -5} {
		w := share(n)
		s.Require().ErrorIs(w.Validate(), behavior.ErrBadWhen, "share %d", n)
	}
	for _, n := range []int{1, 100} {
		w := share(n)
		s.NoError(w.Validate(), "share %d", n)
	}
}

func (s *HitPointsSuite) TestTheRecordReadsEachForm() {
	w := raw(8)
	b, err := json.Marshal(w)
	s.Require().NoError(err)
	s.Contains(string(b), `"HitPoints":{"AtMost":8}`)
	s.NotContains(string(b), "Percent")
	var back behavior.When
	s.Require().NoError(json.Unmarshal(b, &back))
	s.Equal(w, back)

	w = share(30)
	b, err = json.Marshal(w)
	s.Require().NoError(err)
	s.Contains(string(b), `"HitPoints":{"AtMost":30,"Percent":true}`)
	back = behavior.When{}
	s.Require().NoError(json.Unmarshal(b, &back))
	s.Equal(w, back)
}

func (s *HitPointsSuite) TestOneConditionPerEntry() {
	both := behavior.When{Enemy: behavior.EnemySeen, HitPoints: &behavior.HitPointsWhen{AtMost: 30, Percent: true}}
	err := both.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadWhen)
	s.Contains(err.Error(), "names `enemy` and `hit_points`")

	within := behavior.When{HitPoints: &behavior.HitPointsWhen{AtMost: 30, Percent: true}, Within: 2}
	err = within.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadWhen)
	s.Contains(err.Error(), "with a deed's `within` or scope")

	bad := atMost(0)
	inNot := behavior.When{Not: &bad}
	err = inNot.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadWhen)
	s.Contains(err.Error(), "inside `not`:")
}

func (s *HitPointsSuite) TestTheRecordReadsItAndAWhenWithoutItIsAsItWas() {
	w := atMost(30)
	raw, err := json.Marshal(w)
	s.Require().NoError(err)
	s.Contains(string(raw), `"HitPoints":{"AtMost":30,"Percent":true}`)
	var back behavior.When
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Equal(w, back)

	plain, err := json.Marshal(behavior.When{Enemy: behavior.EnemyDash})
	s.Require().NoError(err)
	s.JSONEq(`{"Enemy":"dash","Deed":"","Within":0}`, string(plain), "no hit points, no key")
}
