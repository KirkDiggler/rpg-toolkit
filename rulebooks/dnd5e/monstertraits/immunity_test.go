// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Trait tests follow same event-driven pattern with different conditions
package monstertraits

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type ImmunityTestSuite struct {
	suite.Suite
	bus      events.EventBus
	ctx      context.Context
	immunity *immunityCondition
}

func TestImmunityTestSuite(t *testing.T) {
	suite.Run(t, new(ImmunityTestSuite))
}

func (s *ImmunityTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.immunity = nil // Will be created in each test
}

// hit is 3+4+2 = 9 of damageType from weaponRef, dealt to targetID.
func (s *ImmunityTestSuite) hit(
	targetID string, weaponRef *core.Ref, damageType damage.Type,
) (*dnd5eEvents.IncomingDamageEvent, int) {
	folded, settled, err := foldIncoming(s.ctx, s.bus, "pc-1", targetID, []dnd5eEvents.DamageComponent{{
		Source: dnd5eEvents.DamageSourceWeapon,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: weaponRef, Name: weaponRef.ID},
			Dice:     testDiceTrace(6, 3, 4),
			Modifier: intPtr(2),
		},
		DamageType: damageType,
	}})
	s.Require().NoError(err)
	_, total := settled.FinalDamage()
	return folded, total
}

// The trait answers on the incoming fold, as its holder's own answer, and
// leaves the dealt damage as it was dealt.
func (s *ImmunityTestSuite) TestImmunityAnswersOnTheIncomingFold() {
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)
	s.Require().NoError(s.immunity.Apply(s.ctx, s.bus))

	folded, total := s.hit("monster-1", refs.Weapons.Dagger(), damage.Poison)

	dealt := folded.Dealt()
	s.Require().Len(dealt, 1)
	s.Equal(9, dealt[0].Total(), "the dealt damage is untouched")
	s.Equal([]dnd5eEvents.DamageMultiplier{{
		Category:   dnd5eEvents.DamageSourceMonsterTrait,
		Source:     dnd5eEvents.RollSource{Ref: refs.MonsterTraits.Immunity(), Name: "Immunity"},
		DamageType: damage.Poison,
		Factor:     dnd5eEvents.DamageFactorImmunity,
	}}, folded.Multipliers)
	s.Equal(0, total)
}

func (s *ImmunityTestSuite) TestImmunityDoesNotAffectOtherDamageTypes() {
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)
	s.Require().NoError(s.immunity.Apply(s.ctx, s.bus))

	folded, total := s.hit("monster-1", refs.Weapons.Longsword(), damage.Slashing)

	s.Empty(folded.Multipliers)
	s.Equal(9, total)
}

func (s *ImmunityTestSuite) TestImmunityIgnoresOtherTargets() {
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)
	s.Require().NoError(s.immunity.Apply(s.ctx, s.bus))

	folded, total := s.hit("monster-2", refs.Weapons.Dagger(), damage.Poison)

	s.Empty(folded.Multipliers)
	s.Equal(9, total)
}

func (s *ImmunityTestSuite) TestImmunityCanBeRemoved() {
	s.immunity = Immunity("monster-1", damage.Poison).(*immunityCondition)
	s.Require().NoError(s.immunity.Apply(s.ctx, s.bus))
	s.True(s.immunity.IsApplied())

	s.Require().NoError(s.immunity.Remove(s.ctx, s.bus))
	s.False(s.immunity.IsApplied())

	folded, total := s.hit("monster-1", refs.Weapons.Dagger(), damage.Poison)
	s.Empty(folded.Multipliers, "a removed trait answers nothing")
	s.Equal(9, total)
}
