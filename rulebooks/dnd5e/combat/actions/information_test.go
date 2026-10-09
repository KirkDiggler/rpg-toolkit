package actions_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

type InformationSuite struct{ suite.Suite }

func TestInformationSuite(t *testing.T) { suite.Run(t, new(InformationSuite)) }

type informationWielder struct{}

func (informationWielder) GetAbilityModifier(a abilities.Ability) int {
	if a == abilities.DEX {
		return 4
	}
	return 3
}
func (informationWielder) ProficiencyBonus() int                 { return 2 }
func (informationWielder) IsProficientWith(*weapons.Weapon) bool { return true }

func (s *InformationSuite) attack() actions.Definition {
	return actions.Definition{
		Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "warhammer"}, Name: "Warhammer", Description: "Make a melee attack against a creature in reach.",
		Attack: &actions.AttackProfile{
			Category: actions.AttackCategoryWeapon,
			Delivery: actions.AttackDelivery{Melee: &actions.MeleeDelivery{ReachFeet: 5}},
			Ability:  &actions.AbilityContribution{Ability: abilities.STR, Modifier: 3},
			Weapon:   &actions.WeaponContext{},
			Damage:   []damage.Damage{{Dice: "1d8", Type: damage.Bludgeoning, Properties: []damage.Property{damage.AddsAttackAbilityModifier}}},
		},
	}
}

func (s *InformationSuite) describe(definition actions.Definition) *actions.DescribeOutput {
	out, err := actions.Describe(&actions.DescribeInput{Definition: definition})
	s.Require().NoError(err)
	return out
}

func (s *InformationSuite) assembled(id weapons.WeaponID, twoHanded bool) actions.Definition {
	weapon, err := weapons.GetByID(id)
	s.Require().NoError(err)
	definition, err := weaponattack.Assemble(&weaponattack.Input{Weapon: &weapon, Wielder: informationWielder{}, TwoHanded: twoHanded})
	s.Require().NoError(err)
	return definition
}

func (s *InformationSuite) TestWarhammerGripDice() {
	one := s.describe(s.assembled(weapons.Warhammer, false))
	s.Require().Len(one.Facts.Damage, 1)
	s.Equal("1d8", one.Facts.Damage[0].Dice)
	s.Equal(damage.Bludgeoning, one.Facts.Damage[0].Type)
	s.Equal(actions.GripOneHanded, one.Facts.Grip)
	s.NotEmpty(one.Description)

	two := s.describe(s.assembled(weapons.Warhammer, true))
	s.Require().Len(two.Facts.Damage, 1)
	s.Equal("1d10", two.Facts.Damage[0].Dice)
	s.Equal(actions.GripTwoHanded, two.Facts.Grip)
}

func (s *InformationSuite) TestFinesseStatesDex() {
	out := s.describe(s.assembled(weapons.Rapier, false))
	s.Require().Len(out.Facts.Damage, 1)
	s.Require().NotNil(out.Facts.Damage[0].Ability)
	s.Equal(actions.AbilityFact{Ability: abilities.DEX, Modifier: 4, Participates: true}, *out.Facts.Damage[0].Ability)
	s.Equal(damage.Piercing, out.Facts.Damage[0].Type)
}

func (s *InformationSuite) TestOffHandParticipation() {
	for _, tc := range []struct {
		modifier     int
		participates bool
	}{{3, false}, {0, false}, {-2, true}} {
		s.Run(fmt.Sprintf("modifier %d", tc.modifier), func() {
			definition := s.attack()
			definition.Attack.IsOffHandAttack = true
			definition.Attack.Ability.Modifier = tc.modifier
			out := s.describe(definition)
			s.Equal(actions.GripOffHand, out.Facts.Grip)
			s.Require().Len(out.Facts.Damage, 1)
			s.Require().NotNil(out.Facts.Damage[0].Ability)
			s.Equal(tc.modifier, out.Facts.Damage[0].Ability.Modifier)
			s.Equal(tc.participates, out.Facts.Damage[0].Ability.Participates)
		})
	}
}

func (s *InformationSuite) TestZeroModifierIsPresent() {
	definition := s.attack()
	definition.Attack.Ability.Modifier = 0
	out := s.describe(definition)
	s.Require().NotNil(out.Facts.Damage[0].Ability)
	s.Equal(0, out.Facts.Damage[0].Ability.Modifier)
	s.True(out.Facts.Damage[0].Ability.Participates)
}

func (s *InformationSuite) TestPoolsStaySeparateWithFlatBonus() {
	definition := s.attack()
	definition.Attack.Damage[0].FlatBonus = 2
	definition.Attack.Damage = append(definition.Attack.Damage, damage.Damage{Dice: "1d4", Type: damage.Fire})
	out := s.describe(definition)
	s.Require().Len(out.Facts.Damage, 2)
	s.Equal("1d8", out.Facts.Damage[0].Dice)
	s.Equal(2, out.Facts.Damage[0].FlatBonus)
	s.Equal(damage.Bludgeoning, out.Facts.Damage[0].Type)
	s.NotNil(out.Facts.Damage[0].Ability)
	s.Equal(actions.DamageFact{Dice: "1d4", Type: damage.Fire}, out.Facts.Damage[1])
}

func (s *InformationSuite) TestOnHitOnlyStatesNoDamage() {
	definition := s.attack()
	definition.Description = ""
	definition.Attack.Damage = nil
	definition.Attack.Ability = nil
	definition.Attack.OnHit = []actions.ConditionApplication{{Ref: core.Ref{Module: "dnd5e", Type: "conditions", ID: "prone"}}}
	out := s.describe(definition)
	s.Empty(out.Description)
	s.Empty(out.Facts.Damage)
}

func (s *InformationSuite) TestReachAndRange() {
	melee := s.describe(s.attack())
	s.Require().NotNil(melee.Facts.Melee)
	s.Equal(5, melee.Facts.Melee.ReachFeet)
	s.Nil(melee.Facts.Ranged)
	s.Nil(melee.Facts.Cast)

	ranged := s.attack()
	ranged.Attack.Delivery = actions.AttackDelivery{Ranged: &actions.RangedDelivery{NormalFeet: 80, LongFeet: 320}}
	out := s.describe(ranged)
	s.Require().NotNil(out.Facts.Ranged)
	s.Equal(actions.RangedDelivery{NormalFeet: 80, LongFeet: 320}, *out.Facts.Ranged)
	s.Nil(out.Facts.Melee)

	// The facts are copies: changing one never reaches the definition.
	out.Facts.Ranged.NormalFeet = 1
	s.Equal(80, ranged.Attack.Delivery.Ranged.NormalFeet)
}

func (s *InformationSuite) TestDescribeDoesNotMutate() {
	for name, definition := range map[string]actions.Definition{
		"attack":      s.attack(),
		"bane":        *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13}),
		"thunderwave": *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Thunderwave, SpellSaveDC: 13}),
	} {
		s.Run(name, func() {
			before, err := json.Marshal(definition)
			s.Require().NoError(err)
			out := s.describe(definition)
			if len(out.Facts.Damage) > 0 {
				out.Facts.Damage[0].Dice = "changed output"
			}
			if out.Facts.Cast != nil {
				out.Facts.Cast.RangeFeet = 999
				if out.Facts.Cast.Save != nil {
					out.Facts.Cast.Save.Abilities[0] = abilities.STR
				}
			}
			after, err := json.Marshal(definition)
			s.Require().NoError(err)
			s.Equal(string(before), string(after))
		})
	}
}

func (s *InformationSuite) TestBaneReusesCatalogue() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13})
	s.Require().NotNil(definition)
	out := s.describe(*definition)
	s.Equal(spells.GetData(spells.Bane).Description, out.Description)
	s.Contains(out.Description, "Charisma save")
}

func (s *InformationSuite) TestBaneCastFacts() {
	out := s.describe(*spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13}))
	cast := out.Facts.Cast
	s.Require().NotNil(cast)
	s.Require().NotNil(cast.Save)
	s.Equal([]abilities.Ability{abilities.CHA}, cast.Save.Abilities)
	s.Equal(13, cast.Save.DC)
	s.True(cast.Save.DCKnown)
	s.Equal(saves.Negated, cast.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, cast.Save.Recurrence)
	s.Equal([]actions.EffectFact{{Ref: *refs.Conditions.Baned(), Recipient: actions.CastRecipientTarget, OnFailedSave: true}}, cast.Effects)
	s.Equal(actions.TargetsFact{Rule: actions.CastTargetOneCreature, Min: 1, Max: 3}, cast.Targets)
	s.Equal(spells.BaneRangeFeet, cast.RangeFeet)
	s.Equal(30, cast.RangeFeet)
	s.NotNil(cast.Concentration)
	s.Nil(cast.Healing)
	s.Nil(cast.Area)
	s.Empty(out.Facts.Damage)
	s.Empty(cast.DamageIfInjured)
}

func (s *InformationSuite) TestThunderwaveCastFacts() {
	out := s.describe(*spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Thunderwave, SpellSaveDC: 13}))
	cast := out.Facts.Cast
	s.Require().NotNil(cast)
	s.Require().NotNil(cast.Save)
	s.Equal([]abilities.Ability{abilities.CON}, cast.Save.Abilities)
	s.Equal(saves.Half, cast.Save.OnSuccess)
	s.Equal(13, cast.Save.DC)
	s.Equal([]actions.DamageFact{{Dice: "2d8", Type: damage.Thunder}}, out.Facts.Damage)
	s.Nil(out.Facts.Damage[0].Ability)
	s.Require().NotNil(cast.Area)
	s.Equal(actions.Footprint{Shape: actions.AreaBox, SizeFeet: 15, Origin: actions.AreaOriginCasterEdge}, cast.Area.Footprint)
	s.Equal(actions.AreaCatchesOthers, cast.Area.Catches)
	s.Equal(actions.CastTargetArea, cast.Targets.Rule)
}

func (s *InformationSuite) TestCureWoundsCastFacts() {
	modifiers := []healing.Modifier{
		{Source: dnd5eEvents.RollSource{Ref: &core.Ref{Module: refs.Module, Type: refs.TypeAbilities, ID: "wis"}, Name: "Wisdom"}, Amount: 3},
		{Source: dnd5eEvents.RollSource{Ref: refs.Features.DiscipleOfLife(), Name: "Disciple of Life"}, Amount: 3},
	}
	out := s.describe(*spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.CureWounds, SpellSaveDC: 13, HealingModifiers: modifiers}))
	cast := out.Facts.Cast
	s.Require().NotNil(cast)
	s.Require().NotNil(cast.Healing)
	s.Equal("1d8", cast.Healing.Dice)
	s.Equal([]actions.ModifierFact{{Name: "Wisdom", Amount: 3}, {Name: "Disciple of Life", Amount: 3}}, cast.Healing.Modifiers)
	s.Nil(cast.Save)
	s.Empty(cast.Effects)
	s.Equal(actions.TargetsFact{Rule: actions.CastTargetTouch, Min: 1, Max: 1}, cast.Targets)
	s.Empty(out.Facts.Damage)
}

func (s *InformationSuite) TestNonStaticDCIsUnknown() {
	definition := *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13})
	definition.Cast.Save.DC = saves.DCFivePlusDamageTaken()
	out := s.describe(definition)
	s.Require().NotNil(out.Facts.Cast.Save)
	s.False(out.Facts.Cast.Save.DCKnown)
	s.Zero(out.Facts.Cast.Save.DC)
}

func (s *InformationSuite) TestNilAndInvalidInputRefused() {
	_, err := actions.Describe(nil)
	s.Require().Error(err)
	_, err = actions.Describe(&actions.DescribeInput{})
	s.Require().Error(err)
}

var _ weaponattack.Wielder = informationWielder{}
