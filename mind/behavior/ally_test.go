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
)

// AllySuite is the `ally` selector (rpg-project#717, lane D2): the nearest
// allied member a creature holds, somebody to walk with and never somebody to
// strike. `{ when: { enemy: none }, toward: ally }` is "follow".
type AllySuite struct {
	suite.Suite
}

func TestAllySuite(t *testing.T) { suite.Run(t, new(AllySuite)) }

func ally() *behavior.Selector { return &behavior.Selector{Word: behavior.SelectorAlly} }

func (s *AllySuite) pick(table behavior.Table, facts behavior.Facts) *behavior.PickOutput {
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	return out
}

// follower is a companion's table: strike a foe in reach, walk at one in
// sight, and with no foe follow the party; hold when there is nobody to
// follow either.
func follower() behavior.Table {
	enemy := &behavior.Selector{Word: behavior.SelectorEnemy}

	return behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyReach}}, Attack: enemy},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemySeen}}, Toward: enemy},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyNone}}, Toward: ally()},
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyNone}}, Hold: true},
	}}
}

// TestAllyIsAWordForTheWalkingWords: `toward`, `dash` and `away` take `ally`,
// and the word is one of the sealed selector words the dialect reads.
func (s *AllySuite) TestAllyIsAWordForTheWalkingWords() {
	for name, row := range map[string]behavior.Answer{
		"toward: ally": {Weight: 1, Toward: ally()},
		"dash: ally":   {Weight: 1, Dash: ally()},
		"away: ally":   {Weight: 1, Away: ally()},
	} {
		s.NoError(row.Validate(), name)
	}
	s.Contains(behavior.SelectorWords, behavior.SelectorAlly)
	s.Equal(behavior.SelectorWord("ally"), behavior.SelectorAlly)
}

// TestAttackCannotTakeAnAlly: a mind is never handed an ally to strike, so
// `attack: ally` is refused at the door with a sentence for the form, inside
// a group as well as on a row.
func (s *AllySuite) TestAttackCannotTakeAnAlly() {
	row := behavior.Answer{Weight: 1, Attack: ally()}
	err := row.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadSelector)
	s.Equal("`attack` cannot take `ally`: a mind is never handed an ally to strike, "+
		"only one to walk toward or away from: behavior: selector cannot be read", err.Error())

	group := behavior.Answer{Weight: 1, Rows: []behavior.Answer{{Weight: 1, Hold: true}, row}}
	err = group.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadSelector)
	s.Contains(err.Error(), "row 2 of the group: `attack` cannot take `ally`")
}

// TestFollowIsOnTheTableWhileAnAllyIsHeld: with no foe, the follow row is a
// candidate when an ally is in sight, and when one is only remembered — a
// companion follows where it believes the party went.
func (s *AllySuite) TestFollowIsOnTheTableWhileAnAllyIsHeld() {
	for name, facts := range map[string]behavior.Facts{
		"in sight":   {CanMove: true, SeenAllies: []core.EntityID{"hero"}},
		"remembered": {CanMove: true, RememberedAllies: []core.EntityID{"hero"}},
	} {
		out := s.pick(follower(), facts)
		s.Require().Len(out.Candidates, 2, name)
		s.Equal(2, out.Candidates[0].Entry, "%s: the follow row is a candidate", name)
		s.Equal(behavior.SelectorAlly, out.Answer.Toward.Word, "%s: and the die lands on it", name)
	}
}

// TestFollowIsAbsentWithNobodyToFollow: with no ally held the follow row is
// not a candidate (absent, not a pass), and the author's hold is what fires.
// A foe the creature holds is not an ally, so it does not put the row back.
func (s *AllySuite) TestFollowIsAbsentWithNobodyToFollow() {
	for name, facts := range map[string]behavior.Facts{
		"nobody held":     {CanMove: true},
		"only a foe held": {CanMove: true, RememberedFoes: []core.EntityID{"thug"}, SeenFoes: []core.EntityID{"thug2"}},
	} {
		out := s.pick(follower(), facts)
		s.Require().Len(out.Candidates, 1, name)
		s.Equal(3, out.Candidates[0].Entry, name)
		s.True(out.Answer.Hold, name)
		s.False(out.Rows[2].Affordable, "%s: the follow row's verdict says it could not be paid for", name)
		s.True(out.Rows[2].Held, "%s: its when held; nobody to follow is what kept it off", name)
	}
}

// TestAnAllyIsNotAFoe: an ally held does not make `attacker` or `enemy`
// rows candidates on its account — the ally lists are read by `ally` only.
func (s *AllySuite) TestAnAllyIsNotAFoe() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Deed: "attacked", Within: 3}},
			Toward: &behavior.Selector{Word: behavior.SelectorAttacker}},
		{Weight: 1, Hold: true},
	}}
	facts := behavior.Facts{
		CanMove: true, Now: 5,
		Deeds:      []behavior.HeldDeed{{Kind: behavior.VerbAttack, Actor: "hero", At: 4}},
		SeenAllies: []core.EntityID{"hero"},
	}
	out := s.pick(table, facts)
	s.Require().Len(out.Candidates, 1)
	s.True(out.Answer.Hold, "the one who hit it is held only as an ally, so there is no attacker to walk at")
}

// TestAFollowerNeedsLegsAndARunNeedsTheAction: `ally` is gated by the same
// budget the other walking words are — no movement, no follow; no action to
// spend, no run.
func (s *AllySuite) TestAFollowerNeedsLegsAndARunNeedsTheAction() {
	held := []core.EntityID{"hero"}
	for name, tc := range map[string]struct {
		row   behavior.Answer
		facts behavior.Facts
	}{
		"toward with no movement": {behavior.Answer{Weight: 1, Toward: ally()},
			behavior.Facts{SeenAllies: held, CanDash: true}},
		"away with no movement": {behavior.Answer{Weight: 1, Away: ally()},
			behavior.Facts{SeenAllies: held}},
		"dash with no action": {behavior.Answer{Weight: 1, Dash: ally()},
			behavior.Facts{SeenAllies: held, CanMove: true}},
	} {
		table := behavior.Table{behavior.KeyTime: {tc.row, {Weight: 1, Hold: true}}}
		out := s.pick(table, tc.facts)
		s.Require().Len(out.Candidates, 1, name)
		s.True(out.Answer.Hold, name)
	}

	table := behavior.Table{behavior.KeyTime: {{Weight: 1, Dash: ally()}, {Weight: 1, Hold: true}}}
	out := s.pick(table, behavior.Facts{SeenAllies: held, CanMove: true, CanDash: true})
	s.Require().Len(out.Candidates, 2, "with legs and the action the run is on the table")
	s.Equal(behavior.SelectorAlly, out.Answer.Dash.Word)
}

// TestAnAllyRowMarshalsByItsWord: a row that follows persists as the word,
// and the facts a host never filled add nothing to a marshalled projection.
func (s *AllySuite) TestAnAllyRowMarshalsByItsWord() {
	raw, err := json.Marshal(behavior.Answer{Weight: 1, Toward: ally()})
	s.Require().NoError(err)
	var back behavior.Answer
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Equal(behavior.SelectorAlly, back.Toward.Word)

	facts, err := json.Marshal(behavior.Facts{})
	s.Require().NoError(err)
	s.NotContains(string(facts), "Allies")
}
