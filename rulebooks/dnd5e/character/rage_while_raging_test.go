// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// RageWhileRagingSuite activates Rage through a loaded barbarian's own door,
// the path session's Activate drives, and reads availability the way Afford
// does.
type RageWhileRagingSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRageWhileRagingSuite(t *testing.T) { suite.Run(t, new(RageWhileRagingSuite)) }

func (s *RageWhileRagingSuite) SetupTest() { s.ctx = context.Background() }

// barbarian is a level-3 barbarian with two of three Rage charges, in combat
// with two bonus actions so the second activation is refused by Rage alone.
func (s *RageWhileRagingSuite) barbarian() *Character {
	data := fullSheet(&s.Suite)
	data.Conditions = nil
	data.ActionEconomy.BonusActionsRemaining = 2
	char, err := LoadFromData(s.ctx, data, events.NewEventBus())
	s.Require().NoError(err)
	return char
}

func (s *RageWhileRagingSuite) ragingCount(char *Character) int {
	count := 0
	for _, condition := range char.GetConditions() {
		if condition.Ref().String() == refs.Conditions.Raging().String() {
			count++
		}
	}
	return count
}

func (s *RageWhileRagingSuite) rageAvailability(char *Character) AvailableAbility {
	for _, ability := range char.buildAvailableAbilities() {
		if ability.Ref.ID == refs.Features.Rage().ID {
			return ability
		}
	}
	s.FailNow("rage is not among the barbarian's abilities")
	return AvailableAbility{}
}

func (s *RageWhileRagingSuite) TestActivatingRageWhileRagingIsRefused() {
	char := s.barbarian()

	first, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.Rage()})
	s.Require().NoError(err)
	s.Require().True(first.Success, first.Error)
	s.Require().Equal(1, s.ragingCount(char))
	charges := char.GetResource(resources.RageCharges).Current()
	bonus := char.GetActionEconomy().BonusActionsRemaining

	second, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.Rage()})

	s.Require().NoError(err)
	s.False(second.Success)
	s.Contains(second.Error, "already raging")
	s.Equal(1, s.ragingCount(char), "one Raging condition on the sheet")
	s.Equal(charges, char.GetResource(resources.RageCharges).Current(), "no charge spent")
	s.Equal(bonus, char.GetActionEconomy().BonusActionsRemaining, "no bonus action spent")
}

func (s *RageWhileRagingSuite) TestRageReadsUnavailableWhileRaging() {
	char := s.barbarian()
	s.True(s.rageAvailability(char).CanUse, "a barbarian not raging may rage")

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.Rage()})
	s.Require().NoError(err)
	s.Require().True(out.Success, out.Error)

	rage := s.rageAvailability(char)
	s.False(rage.CanUse, "Rage is not offered while raging")
	s.Contains(rage.Reason, "already raging")
	s.Greater(char.GetResource(resources.RageCharges).Current(), 0, "unavailable for the rage, not for charges")
}

func (s *RageWhileRagingSuite) TestRecklessAttackTwiceLeavesOneCondition() {
	char := s.barbarian()
	reckless, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref: refs.Features.RecklessAttack().String(), Config: json.RawMessage(`{}`), CharacterID: char.GetID(),
	})
	s.Require().NoError(err)
	char.features = append(char.features, reckless.Feature)

	first, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.RecklessAttack()})
	s.Require().NoError(err)
	s.Require().True(first.Success, first.Error)

	second, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.RecklessAttack()})
	s.Require().NoError(err)
	s.False(second.Success)
	s.Contains(second.Error, "already attacking recklessly")

	count := 0
	for _, condition := range char.GetConditions() {
		if condition.Ref().String() == refs.Conditions.RecklessAttack().String() {
			count++
		}
	}
	s.Equal(1, count)
}
