package actions_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
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

func (s *InformationSuite) TestBaseFactsRemainSeparateAndDoNotMutateDefinition() {
	definition := s.attack()
	before, err := json.Marshal(definition)
	s.Require().NoError(err)
	out, err := actions.Describe(&actions.DescribeInput{Definition: definition})
	s.Require().NoError(err)
	s.Equal(definition.Description, out.Information.Description)
	s.Contains(out.Information.Details, actions.InformationDetail{Label: "Base damage", Value: "1d8 + STR modifier (+3) · Bludgeoning"})
	out.Information.Details[0].Value = "changed output"
	after, err := json.Marshal(definition)
	s.Require().NoError(err)
	s.Equal(before, after)
}

func (s *InformationSuite) TestOffHandUsesBasePolicyWithoutRestoringAnEffectContribution() {
	for _, tc := range []struct {
		modifier int
		value    string
	}{{3, "1d8 · Bludgeoning"}, {0, "1d8 · Bludgeoning"}, {-2, "1d8 + STR modifier (-2) · Bludgeoning"}} {
		s.Run(tc.value, func() {
			definition := s.attack()
			definition.Attack.IsOffHandAttack = true
			definition.Attack.Ability.Modifier = tc.modifier
			out, err := actions.Describe(&actions.DescribeInput{Definition: definition})
			s.Require().NoError(err)
			s.Contains(out.Information.Details, actions.InformationDetail{Label: "Base damage", Value: tc.value})
		})
	}
}

func (s *InformationSuite) TestZeroIsARealIncludedModifierAndPoolsStaySeparate() {
	definition := s.attack()
	definition.Attack.Ability.Modifier = 0
	definition.Attack.Damage[0].FlatBonus = 2
	definition.Attack.Damage = append(definition.Attack.Damage, damage.Damage{Dice: "1d4", Type: damage.Fire})
	out, err := actions.Describe(&actions.DescribeInput{Definition: definition})
	s.Require().NoError(err)
	s.Equal(actions.InformationDetail{Label: "Base damage", Value: "1d8 +2 + STR modifier (+0) · Bludgeoning"}, out.Information.Details[0])
	s.Equal(actions.InformationDetail{Label: "Base damage", Value: "1d4 · Fire"}, out.Information.Details[1])
}

func (s *InformationSuite) TestActualWeaponAssemblyOwnsGripAndAbility() {
	for _, tc := range []struct {
		id        weapons.WeaponID
		twoHanded bool
		damage    string
	}{
		{weapons.Warhammer, false, "1d8 + STR modifier (+3) · Bludgeoning"},
		{weapons.Warhammer, true, "1d10 + STR modifier (+3) · Bludgeoning"},
		{weapons.Rapier, false, "1d8 + DEX modifier (+4) · Piercing"},
	} {
		s.Run(tc.damage, func() {
			weapon, err := weapons.GetByID(tc.id)
			s.Require().NoError(err)
			definition, err := weaponattack.Assemble(&weaponattack.Input{Weapon: &weapon, Wielder: informationWielder{}, TwoHanded: tc.twoHanded})
			s.Require().NoError(err)
			out, err := actions.Describe(&actions.DescribeInput{Definition: definition})
			s.Require().NoError(err)
			s.NotEmpty(out.Information.Description)
			s.Contains(out.Information.Details, actions.InformationDetail{Label: "Base damage", Value: tc.damage})
		})
	}
}

func (s *InformationSuite) TestSpellDescriptionReusesCatalogueWithoutAnEffectRequirement() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13})
	s.Require().NotNil(definition)
	out, err := actions.Describe(&actions.DescribeInput{Definition: *definition})
	s.Require().NoError(err)
	s.Equal(spells.GetData(spells.Bane).Description, out.Information.Description)
	s.Contains(out.Information.Description, "Charisma save")
}

func (s *InformationSuite) TestOnHitOnlyAndMissingDescriptionDoNotInventDamageOrMeaning() {
	definition := s.attack()
	definition.Description = ""
	definition.Attack.Damage = nil
	definition.Attack.Ability = nil
	definition.Attack.OnHit = []actions.ConditionApplication{{Ref: core.Ref{Module: "dnd5e", Type: "conditions", ID: "prone"}}}
	out, err := actions.Describe(&actions.DescribeInput{Definition: definition})
	s.Require().NoError(err)
	s.Empty(out.Information.Description)
	for _, detail := range out.Information.Details {
		s.NotEqual("Base damage", detail.Label)
	}
}

func (s *InformationSuite) TestBasicActionsAreContentAndUnknownKindsStayAbsent() {
	for _, kind := range []actions.BasicActionKind{actions.BasicMove, actions.BasicEndTurn, actions.BasicDeathSave, actions.BasicIntimidate, actions.BasicPersuade} {
		info := actions.BasicInformation(actions.BasicInformationInput{Kind: kind})
		s.NotEmpty(info.Description, string(kind))
		s.Empty(info.Details)
	}
	info := actions.BasicInformation(actions.BasicInformationInput{Kind: "not-a-known-action"})
	s.Empty(info.Description)
	s.Empty(info.Details)
}

func (s *InformationSuite) TestNilAndInvalidInputReturnErrorsRatherThanMadeUpFacts() {
	_, err := actions.Describe(nil)
	s.Require().Error(err)
	_, err = actions.Describe(&actions.DescribeInput{})
	s.Require().Error(err)
}

var _ weaponattack.Wielder = informationWielder{}
