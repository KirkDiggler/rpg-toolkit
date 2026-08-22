// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FightingStyleDuelingTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *FightingStyleDuelingTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestFightingStyleDuelingSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleDuelingTestSuite))
}

func (s *FightingStyleDuelingTestSuite) TestNewFightingStyleDuelingCondition() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	s.NotNil(dueling)
	s.False(dueling.IsApplied())
}

func (s *FightingStyleDuelingTestSuite) TestApplyAndRemove() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(dueling.IsApplied())

	err = dueling.Apply(s.ctx, s.bus)
	s.Error(err)

	err = dueling.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(dueling.IsApplied())
}

// TestAddsDamageWithOneHandedWeapon pins rpg-toolkit#1178's fix: eligibility
// now reads entirely off the folded event — no gamectx.GameContext is
// installed anywhere in this test, which is the point being pinned (the
// old version of this test built one via gamectx.NewGameContext).
func (s *FightingStyleDuelingTestSuite) TestAddsDamageWithOneHandedWeapon() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	// A one-handed melee weapon, no off-hand weapon: eligible.
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:       "fighter-1",
		TargetID:         "goblin-1",
		WeaponRef:        refs.Weapons.Longsword(),
		WeaponDamageType: damage.Fire,
		IsMelee:          true,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				Properties:        []damage.Property{damage.AddsAttackAbilityModifier},
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				FlatBonus:         3,
				DamageType:        damage.Slashing,
			},
		},
		IsCritical: true,
	}

	// Execute through damage chain — plain context, no gamectx installed.
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should have 2 components: weapon + dueling bonus
	s.Len(finalEvent.Components, 2)
	s.Equal(2, finalEvent.Components[1].FlatBonus)
	s.Equal(damage.Fire, finalEvent.Components[1].DamageType)
	s.False(finalEvent.Components[1].IsCritical, "flat dueling damage is not doubled")
}

func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithTwoHandedWeapon() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	// A two-handed grip disqualifies Dueling regardless of the weapon.
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		WeaponRef:  refs.Weapons.Greatsword(),
		IsMelee:    true,
		TwoHanded:  true,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				Properties:        []damage.Property{damage.AddsAttackAbilityModifier},
				OriginalDiceRolls: []int{6, 6},
				FinalDiceRolls:    []int{6, 6},
				FlatBonus:         4,
				DamageType:        damage.Slashing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should only have 1 component (no dueling bonus for two-handed)
	s.Len(finalEvent.Components, 1)
}

func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithOffHandWeapon() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	// A weapon in the off hand (dual wielding) disqualifies Dueling.
	offHandRef := refs.Weapons.Shortsword()
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:       "fighter-1",
		TargetID:         "goblin-1",
		WeaponRef:        refs.Weapons.Shortsword(),
		IsMelee:          true,
		OffHandWeaponRef: offHandRef,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				Properties:        []damage.Property{damage.AddsAttackAbilityModifier},
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				FlatBonus:         3,
				DamageType:        damage.Piercing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should only have 1 component (no dueling bonus when dual wielding)
	s.Len(finalEvent.Components, 1)
}

// TestDoesNotAddWithShieldInOffHand pins the "shields are OK" half: a shield
// in the off hand is not a weapon, so OffHandWeaponRef stays nil and Dueling
// still applies — this is precisely the sheet shape (longsword + shield)
// that reproduced rpg-toolkit#1178 live.
func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithShieldInOffHand() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		WeaponRef:  refs.Weapons.Longsword(),
		IsMelee:    true,
		// OffHandWeaponRef intentionally nil — a shield is not a weapon.
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				Properties:        []damage.Property{damage.AddsAttackAbilityModifier},
				OriginalDiceRolls: []int{8},
				FinalDiceRolls:    []int{8},
				FlatBonus:         3,
				DamageType:        damage.Slashing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	s.Len(finalEvent.Components, 2, "a shield in the off hand still leaves Dueling eligible")
	s.Equal(2, finalEvent.Components[1].FlatBonus)
}

// TestDoesNotAddWithUnarmedStrike pins Copilot's finding on PR #1179: moving
// eligibility off a live registry silently dropped an exclusion the OLD
// gamectx.CharacterRegistry never had to state, because it never listed the
// catalog's unarmed strike as a "weapon" to begin with. Dueling requires
// wielding an actual melee weapon (rpg-toolkit#1168's unarmed strike is a
// rule substitute for one, not one) — IsMelee alone cannot tell the two
// apart, since an unarmed swing compiles as melee too.
func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithUnarmedStrike() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		WeaponRef:  refs.Weapons.UnarmedStrike(),
		IsMelee:    true,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				Properties:        []damage.Property{damage.AddsAttackAbilityModifier},
				OriginalDiceRolls: []int{1},
				FinalDiceRolls:    []int{1},
				FlatBonus:         3,
				DamageType:        damage.Bludgeoning,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	s.Len(finalEvent.Components, 1, "no Dueling bonus for a bare-fisted swing")
}

// TestDoesNotAddWithNoWeaponRef pins the nil half of the same exclusion — a
// swing that names no weapon at all is at least as unarmed as one that
// names the catalog's unarmed strike explicitly, and must not default to
// eligible.
func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithNoWeaponRef() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		WeaponRef:  nil,
		IsMelee:    true,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				Properties:        []damage.Property{damage.AddsAttackAbilityModifier},
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				FlatBonus:         3,
				DamageType:        damage.Slashing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	s.Len(finalEvent.Components, 1, "no weapon named means no Dueling bonus")
}

func (s *FightingStyleDuelingTestSuite) TestToJSON() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	jsonData, err := dueling.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleDueling().ID)
	s.Contains(string(jsonData), "fighter-1")
}
