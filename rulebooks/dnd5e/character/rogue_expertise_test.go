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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// RogueExpertiseSuite tests Rogue expertise feature
type RogueExpertiseSuite struct {
	suite.Suite
	eventBus events.EventBus
}

func TestRogueExpertiseSuite(t *testing.T) {
	suite.Run(t, new(RogueExpertiseSuite))
}

func (s *RogueExpertiseSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

// TestRogueExpertiseDoublesSkillBonus tests that expertise doubles the proficiency bonus
func (s *RogueExpertiseSuite) TestRogueExpertiseDoublesSkillBonus() {
	ctx := context.Background()

	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-rogue-expertise",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Shadowmere"})
	s.Require().NoError(err)

	// Set race (Human)
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	})
	s.Require().NoError(err)

	// Set class (Rogue with skills and expertise)
	// Rogue gets 4 skills: Stealth, Perception, Sleight of Hand, Deception
	// Expertise in 2 of them: Stealth and Sleight of Hand
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Rogue,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Stealth,
				skills.Perception,
				skills.SleightOfHand,
				skills.Deception,
			},
			Expertise: []skills.Skill{
				skills.Stealth,       // Will have double proficiency
				skills.SleightOfHand, // Will have double proficiency
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.RogueWeaponsPrimary, OptionID: choices.RogueWeaponRapier},
				{ChoiceID: choices.RogueWeaponsSecondary, OptionID: choices.RogueSecondaryShortbow},
				{ChoiceID: choices.RoguePack, OptionID: choices.RoguePackBurglar},
			},
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Criminal,
	})
	s.Require().NoError(err)

	// Set ability scores (DEX-focused Rogue)
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16, // +3 modifier
			abilities.CON: 12,
			abilities.INT: 14,
			abilities.WIS: 10,
			abilities.CHA: 14,
		},
		Method: "standard",
	})
	s.Require().NoError(err)

	// Convert to character
	char, err := draft.ToCharacter(ctx, "char-rogue-1", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Verify proficiency bonus at level 1 is +2
	s.Equal(2, char.GetProficiencyBonus())

	// DEX modifier is +3
	dexMod := char.GetAbilityModifier(abilities.DEX)
	s.Equal(3, dexMod)

	// Stealth (expertise): DEX (+3) + double proficiency (+4) = +7
	stealthMod := char.GetSkillModifier(skills.Stealth)
	s.Equal(7, stealthMod, "Stealth with expertise should be DEX (+3) + double prof (+4) = +7")

	// Sleight of Hand (expertise): DEX (+3) + double proficiency (+4) = +7
	sleightMod := char.GetSkillModifier(skills.SleightOfHand)
	s.Equal(7, sleightMod, "Sleight of Hand with expertise should be DEX (+3) + double prof (+4) = +7")

	// Perception (no expertise): WIS (+0) + proficiency (+2) = +2
	perceptionMod := char.GetSkillModifier(skills.Perception)
	s.Equal(2, perceptionMod, "Perception without expertise should be WIS (+0) + prof (+2) = +2")

	// Deception (no expertise): CHA (+2) + proficiency (+2) = +4
	deceptionMod := char.GetSkillModifier(skills.Deception)
	s.Equal(4, deceptionMod, "Deception without expertise should be CHA (+2) + prof (+2) = +4")
}

// TestRogueExpertiseCanUseRacialSkill tests that expertise can be applied to skills
// gained from race (not just class skills)
func (s *RogueExpertiseSuite) TestRogueExpertiseCanUseRacialSkill() {
	ctx := context.Background()

	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-rogue-racial-expertise",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Shadowmere"})
	s.Require().NoError(err)

	// Set race (Elf - grants Perception skill proficiency)
	err = draft.SetRace(&SetRaceInput{
		RaceID:    races.Elf,
		SubraceID: races.HighElf,
		Choices:   RaceChoices{
			// High Elf gets a cantrip choice, but we'll skip that for this test
		},
	})
	s.Require().NoError(err)

	// Set class (Rogue) - choose 4 skills that DON'T include Perception
	// Then set expertise in Perception (from race) - this should work!
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Rogue,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Stealth,
				skills.SleightOfHand,
				skills.Deception,
				skills.Acrobatics,
			},
			Expertise: []skills.Skill{
				skills.Stealth,    // From class
				skills.Perception, // From race (Elf) - should be valid!
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.RogueWeaponsPrimary, OptionID: choices.RogueWeaponRapier},
				{ChoiceID: choices.RogueWeaponsSecondary, OptionID: choices.RogueSecondaryShortbow},
				{ChoiceID: choices.RoguePack, OptionID: choices.RoguePackBurglar},
			},
		},
	})
	s.Require().NoError(err, "Expertise in racial skill (Perception from Elf) should be allowed")

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Criminal,
	})
	s.Require().NoError(err)

	// Set ability scores (base scores before racial bonuses)
	// Elf gets +2 DEX, so base 14 DEX becomes 16 DEX (+3 modifier)
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14, // +2 from Elf = 16 (+3 modifier)
			abilities.CON: 12,
			abilities.INT: 14, // +1 from High Elf = 15 (+2 modifier)
			abilities.WIS: 14, // +2 modifier for Perception
			abilities.CHA: 10,
		},
		Method: "standard",
	})
	s.Require().NoError(err)

	// Convert to character
	char, err := draft.ToCharacter(ctx, "char-elf-rogue", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Verify DEX is 16 after racial bonus (14 base + 2 Elf)
	s.Equal(16, char.GetAbilityScore(abilities.DEX), "DEX should be 16 (14 base + 2 Elf)")

	// Perception (expertise from racial skill): WIS (+2) + double proficiency (+4) = +6
	perceptionMod := char.GetSkillModifier(skills.Perception)
	s.Equal(6, perceptionMod, "Perception with expertise should be WIS (+2) + double prof (+4) = +6")

	// Stealth (expertise from class skill): DEX (+3) + double proficiency (+4) = +7
	stealthMod := char.GetSkillModifier(skills.Stealth)
	s.Equal(7, stealthMod, "Stealth with expertise should be DEX (+3) + double prof (+4) = +7")
}

// TestRogueExpertiseMustBeFromProficientSkills tests that expertise can only be applied
// to skills the character is proficient in (from any source)
func (s *RogueExpertiseSuite) TestRogueExpertiseMustBeFromProficientSkills() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-rogue-invalid-expertise",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)

	// Set name and race
	_ = draft.SetName(&SetNameInput{Name: "Shadowmere"})
	_ = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	})

	// Try to set expertise in a skill the Rogue is NOT proficient in
	// Rogue chooses Stealth, Perception, Sleight of Hand, Deception
	// But tries to get expertise in Athletics (not chosen)
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Rogue,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Stealth,
				skills.Perception,
				skills.SleightOfHand,
				skills.Deception,
			},
			Expertise: []skills.Skill{
				skills.Stealth,
				skills.Athletics, // Not proficient - should fail
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.RogueWeaponsPrimary, OptionID: choices.RogueWeaponRapier},
				{ChoiceID: choices.RogueWeaponsSecondary, OptionID: choices.RogueSecondaryShortbow},
				{ChoiceID: choices.RoguePack, OptionID: choices.RoguePackBurglar},
			},
		},
	})
	s.Error(err, "Setting expertise in a non-proficient skill should fail")
}
