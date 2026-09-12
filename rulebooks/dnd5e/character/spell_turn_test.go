// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

func (s *TurnRefreshTestSuite) TestSpellPaymentPersistsAndRestrictsBothOrders() {
	bonus := SpellPayment{Turn: "encounter/1/cleric",
		Casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction},
		Price:   &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}},
	}
	standard := SpellPayment{Turn: bonus.Turn,
		Casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction},
		Price:   &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
	}
	for _, pair := range [][2]SpellPayment{{bonus, standard}, {standard, bonus}} {
		char, err := Load(s.ctx, s.sheet())
		s.Require().NoError(err)
		_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
		s.Require().NoError(err)
		markSaved(char)
		s.Require().NoError(char.CanPaySpell(pair[0]))
		s.False(char.IsDirty())
		s.Require().NoError(char.PaySpell(pair[0]))
		s.True(char.IsDirty())
		encoded, err := json.Marshal(char.ToData())
		s.Require().NoError(err)
		var data Data
		s.Require().NoError(json.Unmarshal(encoded, &data))
		loaded, err := Load(s.ctx, &data)
		s.Require().NoError(err)
		before := loaded.ToData().ActionEconomy
		s.ErrorIs(loaded.CanPaySpell(pair[1]), combat.ErrBonusActionSpell)
		s.ErrorIs(loaded.PaySpell(pair[1]), combat.ErrBonusActionSpell)
		s.Equal(before, loaded.ToData().ActionEconomy)
		s.False(loaded.IsDirty())
	}
}

func (s *TurnRefreshTestSuite) TestSpellPaymentFailureDoesNotRecordOrSpend() {
	char, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)
	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	markSaved(char)
	input := SpellPayment{Turn: "encounter/1/cleric",
		Casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction},
		Price: &combat.SpendProfile{
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
			Pools: map[coreResources.ResourceKey]int{resources.SpellSlotLevel1: 1},
		},
	}
	before := char.ToData().ActionEconomy
	s.Error(char.CanPaySpell(input))
	s.Error(char.PaySpell(input))
	s.Equal(before, char.ToData().ActionEconomy)
	s.False(char.IsDirty())
}

func (s *TurnRefreshTestSuite) TestSpellTurnChangesDoNotRefillEconomy() {
	char, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)
	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	input := SpellPayment{Turn: "encounter/1/cleric",
		Casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction}}
	s.Require().NoError(char.PaySpell(input))
	s.True(char.IsDirty(), "even a free cast records history")
	// An economy refresh is not permission to forget casting on the same turn.
	_, err = char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 30})
	s.Require().NoError(err)
	input.Casting.Time = combat.SpellCastingReaction
	s.ErrorIs(char.PaySpell(input), combat.ErrBonusActionSpell)
	input.Turn = "encounter/1/goblin"
	input.Price = &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionReaction: 1}}
	s.Require().NoError(char.PaySpell(input))
	s.Equal(0, char.SlotsLeft(coreCombat.ActionReaction))
	input.Turn = "encounter/1/ogre"
	s.Error(char.PaySpell(input), "a new spell turn must not replenish the reaction")
	s.Equal("encounter/1/goblin", char.GetActionEconomy().Spellcasting.Turn)
	_, err = char.ExitCombat(s.ctx, &ExitCombatInput{})
	s.Require().NoError(err)
	s.Error(char.PaySpell(input))
}
