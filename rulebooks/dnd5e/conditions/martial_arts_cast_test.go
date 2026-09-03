// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// MartialArtsCastSuite pins the cast read on BOTH chains this condition
// subscribes to, in both directions of the comparison it exists to make.
//
// The feature is "use DEX when DEX is higher", so a rule hardcoded to either
// ability is right half the time. Every fixture here is built so the wrong
// ability produces a DIFFERENT number: a mutant that always swaps and a mutant
// that never swaps must each fail something.
type MartialArtsCastSuite struct {
	suite.Suite
	bus events.EventBus
}

func (s *MartialArtsCastSuite) SetupTest() { s.bus = events.NewEventBus() }

func TestMartialArtsCastSuite(t *testing.T) { suite.Run(t, new(MartialArtsCastSuite)) }

// dexMonk has DEX +3 / STR +0 — the swap must fire.
func dexMonkScores() shared.AbilityScores {
	return shared.AbilityScores{
		abilities.STR: 10, abilities.DEX: 16, abilities.CON: 14,
		abilities.INT: 10, abilities.WIS: 15, abilities.CHA: 8,
	}
}

// strMonk has STR +3 / DEX +2 — the swap must NOT fire.
func strMonkScores() shared.AbilityScores {
	return shared.AbilityScores{
		abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
		abilities.INT: 10, abilities.WIS: 15, abilities.CHA: 8,
	}
}

// apply puts a level 1 Martial Arts condition for id on this suite's bus and
// unsubscribes it at test end. Nothing here holds the condition afterwards:
// every assertion below is about what the CHAIN came back with, which is the
// only thing a rule's caller ever sees.
func (s *MartialArtsCastSuite) apply(id string) {
	s.T().Helper()
	ma := NewMartialArtsCondition(MartialArtsInput{MemberID: id, MonkLevel: 1})
	s.Require().NoError(ma.Apply(context.Background(), s.bus))
	s.T().Cleanup(func() { _ = ma.Remove(context.Background(), s.bus) })
}

// attackBonusAfter folds an unarmed attack and reports the resulting bonus.
func (s *MartialArtsCastSuite) attackBonusAfter(ctx context.Context, id string, base int) int {
	s.T().Helper()
	event := dnd5eEvents.AttackChainEvent{
		AttackerID: id, TargetID: "target-1",
		WeaponRef: refs.Weapons.UnarmedStrike(), IsMelee: true,
		AttackBonus: base, TargetAC: 12,
	}
	chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(ctx, event, chain)
	s.Require().NoError(err)

	final, err := modified.Execute(ctx, event)
	s.Require().NoError(err,
		"a condition that cannot answer must leave the chain untouched, never error")

	return final.AttackBonus
}

// abilityBonusAfter folds an unarmed damage event and reports the ability
// component's flat bonus and the ability the event ended up crediting.
func (s *MartialArtsCastSuite) abilityBonusAfter(
	ctx context.Context, id string, seededSTR int,
) (int, abilities.Ability) {
	s.T().Helper()
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: id, TargetID: "target-1",
		WeaponRef: refs.Weapons.UnarmedStrike(),
		Components: []dnd5eEvents.DamageComponent{
			{
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(6, 1),
				},
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			},
			{
				Source: dnd5eEvents.DamageSourceAbility,
				Roll: dnd5eEvents.RollComponent{
					Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
					Modifier: intPtr(seededSTR),
				},
			},
		},
		AbilityUsed: abilities.STR,
	}
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(ctx, event, chain)
	s.Require().NoError(err)

	final, err := modified.Execute(ctx, event)
	s.Require().NoError(err,
		"a condition that cannot answer must leave the chain untouched, never error")

	for _, comp := range final.Components {
		if comp.Source == dnd5eEvents.DamageSourceAbility {
			return comp.Total(), final.AbilityUsed
		}
	}
	s.Require().Fail("no ability component survived the fold")

	return 0, ""
}

// DEX monk, damage chain: STR +0 is replaced by DEX +3.
func (s *MartialArtsCastSuite) TestDamageSwapsToDEXWhenDEXIsHigher() {
	s.apply("monk-dex")
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-dex", scores: dexMonkScores()})

	bonus, used := s.abilityBonusAfter(ctx, "monk-dex", 0)

	s.Equal(3, bonus, "DEX(+3) replaces the seeded STR(+0)")
	s.Equal(abilities.DEX, used, "and the event credits DEX, which is what the combat log reads")
}

// STR monk, damage chain: STR +3 stays. A rule that always swaps reports +2.
func (s *MartialArtsCastSuite) TestDamageKeepsSTRWhenSTRIsHigher() {
	s.apply("monk-str")
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-str", scores: strMonkScores()})

	bonus, used := s.abilityBonusAfter(ctx, "monk-str", 3)

	s.Equal(3, bonus, "STR(+3) stays; a rule that always swaps would report DEX(+2)")
	s.Equal(abilities.STR, used, "and the credited ability stays STR")
}

// DEX monk, attack chain: the bonus gains the DEX-minus-STR difference.
func (s *MartialArtsCastSuite) TestAttackSwapsToDEXWhenDEXIsHigher() {
	s.apply("monk-dex")
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-dex", scores: dexMonkScores()})

	// Seeded 2 = STR(+0) + proficiency(+2). DEX(+3) - STR(+0) = +3 → 5.
	s.Equal(5, s.attackBonusAfter(ctx, "monk-dex", 2),
		"attack and damage must agree on the governing ability (#709)")
}

// STR monk, attack chain: untouched. A rule that always swaps would SUBTRACT.
func (s *MartialArtsCastSuite) TestAttackKeepsSTRWhenSTRIsHigher() {
	s.apply("monk-str")
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-str", scores: strMonkScores()})

	// Seeded 5 = STR(+3) + proficiency(+2). Always-swap would give 5 + (2-3) = 4.
	s.Equal(5, s.attackBonusAfter(ctx, "monk-str", 5),
		"STR stays when DEX is not higher; an unconditional swap would report 4")
}

// NO CAST: both chains come back untouched, and neither errors.
//
// Both, in one test, because the two handlers read through the same helper and
// a migration that converted one and missed the other would otherwise leave a
// condition half on the new channel with nothing saying so.
func (s *MartialArtsCastSuite) TestNoCastLeavesBothChainsUntouched() {
	s.apply("monk-dex")
	bare := context.Background()

	bonus, used := s.abilityBonusAfter(bare, "monk-dex", 0)
	s.Equal(0, bonus, "no cast, no comparison — the seeded STR(+0) stands")
	s.Equal(abilities.STR, used)

	s.Equal(2, s.attackBonusAfter(bare, "monk-dex", 2), "and the attack bonus is untouched too")
}

// A cast that does not hold THIS monk answers the same as no cast — installed
// and answering, it simply cannot name this member.
func (s *MartialArtsCastSuite) TestACastWithoutThisMonkLeavesBothChainsUntouched() {
	s.apply("monk-dex")
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "somebody-else", scores: dexMonkScores()})

	bonus, _ := s.abilityBonusAfter(ctx, "monk-dex", 0)
	s.Equal(0, bonus, "another monk's dexterity is not this monk's")

	s.Equal(2, s.attackBonusAfter(ctx, "monk-dex", 2))
}
