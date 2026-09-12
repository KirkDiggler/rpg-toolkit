// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/stretchr/testify/suite"
)

type SpellTurnSuite struct{ suite.Suite }

func TestSpellTurnSuite(t *testing.T) { suite.Run(t, new(SpellTurnSuite)) }

func (s *SpellTurnSuite) TestBothCastingOrders() {
	bonus := combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction}
	cases := []struct {
		name    string
		other   combat.SpellCasting
		allowed bool
	}{
		{"one-action cantrip", combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction}, true},
		{"leveled action", combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction}, false},
		{"reaction", combat.SpellCasting{Level: 1, Time: combat.SpellCastingReaction}, false},
		{"reaction cantrip", combat.SpellCasting{Level: 0, Time: combat.SpellCastingReaction}, false},
		{"bonus cantrip", combat.SpellCasting{Level: 0, Time: combat.SpellCastingBonusAction}, false},
		{"longer cantrip", combat.SpellCasting{Level: 0, Time: combat.SpellCastingOther}, false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			for _, pair := range [][2]combat.SpellCasting{{bonus, tc.other}, {tc.other, bonus}} {
				first, err := (combat.SpellTurnState{}).AfterCast("encounter-a/round-1/cleric", pair[0])
				s.Require().NoError(err)
				second, err := first.AfterCast(first.Turn, pair[1])
				if tc.allowed {
					s.Require().NoError(err)
					s.True(second.BonusActionSpell)
				} else {
					s.ErrorIs(err, combat.ErrBonusActionSpell)
					s.Equal(first, second, "refusal must preserve the input history")
				}
			}
		})
	}
}

func (s *SpellTurnSuite) TestDoesNotImposeOneLeveledSpellPerTurn() {
	state := combat.SpellTurnState{}
	for _, cast := range []combat.SpellCasting{
		{Level: 1, Time: combat.SpellCastingAction},
		{Level: 1, Time: combat.SpellCastingReaction},
		{Level: 3, Time: combat.SpellCastingAction},
	} {
		next, err := state.AfterCast("same-turn", cast)
		s.Require().NoError(err)
		state = next
	}
}

func (s *SpellTurnSuite) TestPersistedHistoryExpiresOnExplicitTurnChange() {
	state, err := (combat.SpellTurnState{}).AfterCast("encounter-a/1/cleric",
		combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction})
	s.Require().NoError(err)
	encoded, err := json.Marshal(state)
	s.Require().NoError(err)
	var loaded combat.SpellTurnState
	s.Require().NoError(json.Unmarshal(encoded, &loaded))
	reaction := combat.SpellCasting{Level: 1, Time: combat.SpellCastingReaction}
	_, err = loaded.AfterCast(state.Turn, reaction)
	s.ErrorIs(err, combat.ErrBonusActionSpell)
	for _, turn := range []string{"encounter-a/1/goblin", "encounter-a/2/cleric", "encounter-b/1/cleric"} {
		next, nextErr := loaded.AfterCast(turn, reaction)
		s.Require().NoError(nextErr)
		s.False(next.BonusActionSpell)
		s.True(next.OtherSpell)
	}
}

func (s *SpellTurnSuite) TestRejectsIncompleteDeclarationsWithoutChangingHistory() {
	state := combat.SpellTurnState{Turn: "previous", OtherSpell: true}
	for _, cast := range []combat.SpellCasting{
		{}, {Level: -1, Time: combat.SpellCastingAction},
		{Level: 10, Time: combat.SpellCastingAction}, {Time: "unknown"},
	} {
		next, err := state.AfterCast("next", cast)
		s.Error(err)
		s.Equal(state, next)
	}
	next, err := state.AfterCast(" ", combat.SpellCasting{Time: combat.SpellCastingAction})
	s.Error(err)
	s.Equal(state, next)
}
