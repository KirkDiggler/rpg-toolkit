// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// BardCantripsSuite is rpg-project#405's creation half: a bard chooses exactly
// two cantrips, and the caster answers its own spell save DC.
type BardCantripsSuite struct {
	suite.Suite
	bus events.EventBus
}

func TestBardCantripsSuite(t *testing.T) {
	suite.Run(t, new(BardCantripsSuite))
}

func (s *BardCantripsSuite) SetupTest() { s.bus = events.NewEventBus() }

// bardDraft builds a level-1 bard whose only variable is its cantrip
// selection and its Charisma.
func (s *BardCantripsSuite) bardDraft(charisma int, cantrips []shared.SelectionID) *Draft {
	draft, err := NewDraft(&DraftConfig{ID: "bard-draft", PlayerID: "player-1"})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Scanlan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Bard,
		Choices: ClassChoices{
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
			Tools:    []shared.SelectionID{"lute", "flute", "drum"},
			Cantrips: cantrips,
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BardWeaponsPrimary, OptionID: choices.BardWeaponRapier},
				{ChoiceID: choices.BardPack, OptionID: choices.BardPackDiplomat},
				{ChoiceID: choices.BardInstrument, OptionID: choices.BardInstrumentLute},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: charisma,
		},
		Method: "standard-array",
	}))

	return draft
}

// fighterDraft is the control: a class with no spellcasting ability at all.
func (s *BardCantripsSuite) fighterDraft() *Draft {
	draft, err := NewDraft(&DraftConfig{ID: "fighter-draft", PlayerID: "player-2"})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Grog"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills:        []skills.Skill{skills.Athletics, skills.Intimidation},
			FightingStyle: "defense",
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 12, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	return draft
}

func (s *BardCantripsSuite) TestABardFinalizesChoosingExactlyTwo() {
	draft := s.bardDraft(16, []shared.SelectionID{spells.TrueStrike, spells.ViciousMockery})

	s.Require().NoError(draft.ValidateChoices())

	char, err := draft.ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	known := char.KnownCantrips()
	s.Require().Len(known, 2)
	s.Equal(refs.Spells.TrueStrike().String(), known[0].String())
	s.Equal(refs.Spells.ViciousMockery().String(), known[1].String())
}

func (s *BardCantripsSuite) TestOneCantripIsRefused() {
	draft := s.bardDraft(16, []shared.SelectionID{spells.ViciousMockery})

	s.Require().ErrorContains(draft.ValidateChoices(), "Must choose exactly 2 cantrips")

	_, err := draft.ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().Error(err, "and the draft does not finalize either")
}

func (s *BardCantripsSuite) TestThreeCantripsAreRefused() {
	draft := s.bardDraft(16, []shared.SelectionID{
		spells.TrueStrike, spells.ViciousMockery, spells.MageHand,
	})

	s.Require().ErrorContains(draft.ValidateChoices(), "Must choose exactly 2 cantrips")
}

func (s *BardCantripsSuite) TestACantripOutsideTheOfferedListIsRefused() {
	draft := s.bardDraft(16, []shared.SelectionID{spells.TrueStrike, spells.MageHand})

	s.Require().Error(draft.ValidateChoices(),
		"Mage Hand is a bard cantrip this build cannot cast, so it is not offered")
}

func (s *BardCantripsSuite) TestTheSpellSaveDCIsEightPlusProficiencyPlusCharisma() {
	s.Run("Charisma 16", func() {
		char, err := s.bardDraft(16, []shared.SelectionID{
			spells.TrueStrike, spells.ViciousMockery,
		}).ToCharacter(context.Background(), "bard-1", events.NewEventBus())
		s.Require().NoError(err)

		s.Equal(2, char.ProficiencyBonus())
		s.Equal(3, char.GetAbilityModifier(abilities.CHA))
		s.Equal(13, char.SpellSaveDC(), "8 + 2 + 3")
	})

	s.Run("Charisma 14", func() {
		char, err := s.bardDraft(14, []shared.SelectionID{
			spells.TrueStrike, spells.ViciousMockery,
		}).ToCharacter(context.Background(), "bard-2", events.NewEventBus())
		s.Require().NoError(err)

		s.Equal(12, char.SpellSaveDC(), "8 + 2 + 2")
	})
}

func (s *BardCantripsSuite) TestAFighterIsUnchanged() {
	requirements := choices.GetClassRequirements(classes.Fighter)
	s.Require().NotNil(requirements)
	s.Nil(requirements.Cantrips, "a fighter is asked nothing about cantrips")

	char, err := s.fighterDraft().ToCharacter(context.Background(), "fighter-1", s.bus)
	s.Require().NoError(err)

	s.Nil(char.KnownCantrips())
	s.Zero(char.SpellSaveDC(), "and a class with no spellcasting ability has no DC")
}
