// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// CastContentSuite covers supported cast profiles and unsupported catalog entries.
type CastContentSuite struct {
	suite.Suite
}

func TestCastContentSuite(t *testing.T) {
	suite.Run(t, new(CastContentSuite))
}

func (s *CastContentSuite) TestBaneCompilesItsCompleteLevelOneProfile() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(*refs.Spells.Bane(), definition.Ref)
	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Equal(1, definition.Cost.Pools[resources.SpellSlotLevel1])

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(30, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)
	s.Equal(1, profile.MinTargets)
	s.Equal(3, profile.MaxTargets)
	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.CHA}, profile.Save.Abilities)
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Negated, profile.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence)
	s.Require().Len(profile.Effects, 1)
	s.Equal(actions.CastRecipientTarget, profile.Effects[0].Recipient)
	s.Equal(*refs.Conditions.Baned(), profile.Effects[0].Ref)
	s.Require().NotNil(profile.Concentration)
	s.Equal(10, profile.Concentration.TurnEnds)
	s.True(profile.Concentration.SkipFirstTurnEnd)
	s.True(spells.HasCastProfile(spells.Bane))
	s.Equal([]spells.Spell{spells.Bane}, spells.Castable([]spells.Spell{spells.Bless, spells.Bane}))
}

func (s *CastContentSuite) TestSacredFlameCarriesOnlyItsSaveAndRadiantDamage() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.SacredFlame, SpellSaveDC: 14})
	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(refs.Spells.SacredFlame().String(), definition.Ref.String())
	s.Equal("Sacred Flame", definition.Name)
	s.Nil(definition.Attack)
	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Empty(definition.Cost.Pools, "cantrips spend no spell-slot pool")

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(60, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)
	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.DEX}, profile.Save.Abilities)
	s.Equal(14, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Negated, profile.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence)
	s.Require().Len(profile.Damage, 1)
	s.Equal("1d8", profile.Damage[0].Dice)
	s.Equal(damage.Radiant, profile.Damage[0].Type)
	s.Empty(profile.Effects, "Sacred Flame leaves no condition behind")
	s.Nil(profile.Concentration)
}

func (s *CastContentSuite) TestClericCastableSubsetDoesNotEnableOtherKnownCantrips() {
	s.Equal([]spells.Spell{spells.SacredFlame},
		spells.Castable([]spells.Spell{spells.Guidance, spells.SacredFlame, spells.Light}))
}

func (s *CastContentSuite) TestViciousMockeryCarriesItsSaveGateAndDamage() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(refs.Spells.ViciousMockery().String(), definition.Ref.String())
	s.Equal("Vicious Mockery", definition.Name)

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(spells.ViciousMockeryRangeFeet, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)

	s.Require().NotNil(profile.Save, "the target contests it")
	s.Equal([]abilities.Ability{abilities.WIS}, profile.Save.Abilities)
	s.Equal(saves.Negated, profile.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence)
	s.Equal(saves.DCKindStatic, profile.Save.DC.Kind())
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}), "the caster's own DC, written in")

	s.Require().Len(profile.Damage, 1)
	s.Equal(spells.ViciousMockeryDamage, profile.Damage[0].Dice)
	s.Equal(damage.Psychic, profile.Damage[0].Type)

	s.Require().Len(profile.Effects, 1)
	s.Equal(actions.CastRecipientTarget, profile.Effects[0].Recipient)
	s.Equal(refs.Conditions.ViciousMockery().String(), profile.Effects[0].Ref.String())
	s.Equal(spells.ViciousMockeryCasterParameter, profile.Effects[0].CounterpartKey)
}

func (s *CastContentSuite) TestTheDCIsTheCastersRatherThanTheSpells() {
	twelve := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 12})
	fifteen := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 15})

	s.Require().NotNil(twelve)
	s.Require().NotNil(fifteen)
	s.Equal(12, twelve.Cast.Save.DC.DC(saves.DCInput{}))
	s.Equal(15, fifteen.Cast.Save.DC.DC(saves.DCInput{}))
}

func (s *CastContentSuite) TestTrueStrikeCarriesNoSave() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(refs.Spells.TrueStrike().String(), definition.Ref.String())

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Nil(profile.Save, "nobody resists True Strike")
	s.Empty(profile.Damage, "and it deals none")
	s.Equal(spells.TrueStrikeRangeFeet, profile.RangeFeet)

	s.Require().Len(profile.Effects, 1)
	s.Equal(actions.CastRecipientCaster, profile.Effects[0].Recipient,
		"the advantage lands on the caster, not the creature named")
	s.Equal(refs.Conditions.TrueStrike().String(), profile.Effects[0].Ref.String())
	s.Equal(spells.TrueStrikeTargetParameter, profile.Effects[0].CounterpartKey)
}

// The retrofit (rpg-project#407 R8): True Strike IS a concentration cantrip,
// and the two turn ends it used to count for itself now live on the profile,
// which is where the owning condition reads them.
func (s *CastContentSuite) TestTrueStrikeDeclaresConcentration() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NotNil(definition.Cast.Concentration)
	s.Equal(spells.TrueStrikeTurnEnds, definition.Cast.Concentration.TurnEnds)
	s.Equal(2, spells.TrueStrikeTurnEnds,
		"the end of the casting turn, then the end of the next one")
}

func (s *CastContentSuite) TestViciousMockeryDeclaresNone() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Nil(definition.Cast.Concentration, "an insult that landed needs nobody to hold it")
}

func (s *CastContentSuite) TestACantripWithNoContentMintsNothing() {
	for _, id := range []spells.Spell{spells.MageHand, spells.Light, spells.Prestidigitation} {
		s.Nil(spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: 13}), "%s has no cast content in this build", id)
		s.False(spells.HasCastProfile(id))
	}
}

func (s *CastContentSuite) TestASpellThisBuildNeverHeardOfMintsNothing() {
	s.Nil(spells.CastDefinition(spells.CastDefinitionInput{Spell: "song-of-nothing", SpellSaveDC: 13}))
	s.False(spells.HasCastProfile("song-of-nothing"))
}

func (s *CastContentSuite) TestTheBardsListIsAllElevenCantrips() {
	s.Len(spells.BardCantrips, 11)
	s.Contains(spells.BardCantrips, spells.TrueStrike)
	s.Contains(spells.BardCantrips, spells.ViciousMockery)
	s.Contains(spells.BardCantrips, spells.MageHand)
}

func (s *CastContentSuite) TestCastableIsTheTwoThisBuildCanCast() {
	s.Equal([]spells.Spell{spells.TrueStrike, spells.ViciousMockery},
		spells.Castable(spells.BardCantrips))
}

func (s *CastContentSuite) TestEveryCastableCantripHasAValidDefinition() {
	for _, id := range spells.Castable(spells.BardCantrips) {
		definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: 13})
		s.Require().NotNil(definition, "%s", id)
		s.Require().NoError(definition.Validate(), "%s", id)
		s.Nil(definition.Attack, "%s declares a cast, not an attack", id)
		s.Require().NotNil(definition.Cost, "%s", id)
		s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard], "%s", id)
		s.Empty(definition.Cost.Pools, "%s is a cantrip", id)
	}
}
