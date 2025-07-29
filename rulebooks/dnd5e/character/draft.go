// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Draft represents a character in progress
type Draft struct {
	ID       string
	PlayerID string
	Name     string
	Progress DraftProgress

	// Explicit typed choices - compile-time safe!
	RaceChoice          RaceChoice           `json:"race_choice"`
	ClassChoice         string               `json:"class_choice"`
	BackgroundChoice    string               `json:"background_choice"`
	AbilityScoreChoice  shared.AbilityScores `json:"ability_score_choice"`
	SkillChoices        []string             `json:"skill_choices"`
	LanguageChoices     []string             `json:"language_choices"`
	FightingStyleChoice string               `json:"fighting_style_choice,omitempty"`
	SpellChoices        []string             `json:"spell_choices,omitempty"`
	CantripChoices      []string             `json:"cantrip_choices,omitempty"`
	EquipmentChoices    []string             `json:"equipment_choices,omitempty"`
	FeatChoices         []string             `json:"feat_choices,omitempty"` // For future feat system

	CreatedAt time.Time
	UpdatedAt time.Time
}

// DraftProgress tracks completion of character creation steps
type DraftProgress struct {
	flags uint32
}

// ToCharacter converts a completed draft into a playable character
// This method validates the draft is complete and creates a fully initialized character
func (d *Draft) ToCharacter(raceData *race.Data, classData *class.Data,
	backgroundData *shared.Background) (*Character, error) {
	// Validate we have all required data
	if raceData == nil || classData == nil || backgroundData == nil {
		return nil, errors.New("race, class, and background data are required")
	}

	// Check if draft is complete enough to build
	if !d.isComplete() {
		return nil, errors.New("draft is incomplete - missing required choices")
	}

	// Validate the draft with external data
	validator := NewValidator()
	errors := validator.ValidateDraft(d, raceData, classData, backgroundData)
	if len(errors) > 0 {
		return nil, fmt.Errorf("validation failed: %v", errors)
	}

	// Compile the character using the same logic as builder
	return d.compileCharacter(raceData, classData, backgroundData)
}

// isComplete checks if the draft has all required fields to create a character
func (d *Draft) isComplete() bool {
	required := ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores
	return d.Progress.flags&required == required
}

// compileCharacter creates a character from the draft data
func (d *Draft) compileCharacter(raceData *race.Data, classData *class.Data,
	backgroundData *shared.Background) (*Character, error) {
	// Start with base character data
	charData := Data{
		ID:           d.ID,
		PlayerID:     d.PlayerID,
		Name:         d.Name,
		Level:        1, // Starting level
		RaceID:       raceData.ID,
		ClassID:      classData.ID,
		BackgroundID: backgroundData.ID,
	}

	// Set subrace ID if present
	if d.RaceChoice.SubraceID != "" {
		charData.SubraceID = d.RaceChoice.SubraceID
	}

	// Set ability scores from explicit field
	charData.AbilityScores = d.AbilityScoreChoice

	// Apply racial ability score improvements
	applyAbilityScoreIncreases(charData.AbilityScores, raceData.AbilityScoreIncreases)

	// Apply subrace ability score improvements if applicable
	if d.RaceChoice.SubraceID != "" {
		// Find the subrace data
		for _, subrace := range raceData.Subraces {
			if subrace.ID == d.RaceChoice.SubraceID {
				applyAbilityScoreIncreases(charData.AbilityScores, subrace.AbilityScoreIncreases)
				break
			}
		}
	}

	// Calculate HP
	charData.MaxHitPoints = classData.HitDice + charData.AbilityScores.Modifier(constants.CON)
	charData.HitPoints = charData.MaxHitPoints

	// Physical characteristics from race
	charData.Speed = raceData.Speed
	charData.Size = raceData.Size

	// Skills
	charData.Skills = make(map[string]int)
	// Add chosen skills
	for _, skill := range d.SkillChoices {
		charData.Skills[skill] = int(shared.Proficient)
	}
	// Add background skills
	// Note: If a skill is already proficient (e.g., Half-Orc gets Intimidation,
	// player chooses Intimidation from Fighter), this is fine - you just don't
	// get double proficiency. The map structure naturally handles this.
	for _, skill := range backgroundData.SkillProficiencies {
		charData.Skills[skill] = int(shared.Proficient)
	}

	// Languages
	// Start with ensuring Common is always included
	languageSet := make(map[string]bool)
	languageSet["common"] = true

	// Add race languages
	for _, lang := range raceData.Languages {
		languageSet[lang] = true
	}

	// Add background languages
	for _, lang := range backgroundData.Languages {
		languageSet[lang] = true
	}

	// Add language choices
	for _, lang := range d.LanguageChoices {
		languageSet[lang] = true
	}

	// Convert set to slice
	charData.Languages = make([]string, 0, len(languageSet))
	for lang := range languageSet {
		charData.Languages = append(charData.Languages, lang)
	}

	// Proficiencies
	charData.Proficiencies = shared.Proficiencies{
		Armor:   classData.ArmorProficiencies,
		Weapons: append(classData.WeaponProficiencies, raceData.WeaponProficiencies...),
		Tools:   append(classData.ToolProficiencies, backgroundData.ToolProficiencies...),
	}

	// Saving throws
	charData.SavingThrows = make(map[string]int)
	for _, save := range classData.SavingThrows {
		charData.SavingThrows[save] = int(shared.Proficient)
	}

	// Store choices made - now using explicit typed fields!
	charData.Choices = []ChoiceData{}

	// Store name choice
	charData.Choices = append(charData.Choices, ChoiceData{
		Category:  string(shared.ChoiceName),
		Source:    "player",
		Selection: d.Name,
	})

	// Store race choice
	charData.Choices = append(charData.Choices, ChoiceData{
		Category:  string(shared.ChoiceRace),
		Source:    "race",
		Selection: d.RaceChoice,
	})

	// Store class choice
	charData.Choices = append(charData.Choices, ChoiceData{
		Category:  string(shared.ChoiceClass),
		Source:    "class",
		Selection: d.ClassChoice,
	})

	// Store background choice
	charData.Choices = append(charData.Choices, ChoiceData{
		Category:  string(shared.ChoiceBackground),
		Source:    "background",
		Selection: d.BackgroundChoice,
	})

	// Store ability scores
	charData.Choices = append(charData.Choices, ChoiceData{
		Category:  string(shared.ChoiceAbilityScores),
		Source:    "player",
		Selection: d.AbilityScoreChoice,
	})

	// Store skill choices (if any)
	if len(d.SkillChoices) > 0 {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(shared.ChoiceSkills),
			Source:    "class",
			Selection: d.SkillChoices,
		})
	}

	// Store language choices (if any)
	if len(d.LanguageChoices) > 0 {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(shared.ChoiceLanguages),
			Source:    "race",
			Selection: d.LanguageChoices,
		})
	}

	// Store fighting style choice (if any)
	if d.FightingStyleChoice != "" {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(shared.ChoiceFightingStyle),
			Source:    "class",
			Selection: d.FightingStyleChoice,
		})
	}

	// Store spell choices (if any)
	if len(d.SpellChoices) > 0 {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(shared.ChoiceSpells),
			Source:    "class",
			Selection: d.SpellChoices,
		})
	}

	// Store cantrip choices (if any)
	if len(d.CantripChoices) > 0 {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(shared.ChoiceCantrips),
			Source:    "class",
			Selection: d.CantripChoices,
		})
	}

	// Store equipment choices (if any)
	if len(d.EquipmentChoices) > 0 {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(shared.ChoiceEquipment),
			Source:    "class",
			Selection: d.EquipmentChoices,
		})
	}

	// Store feat choices (if any) - for future use
	if len(d.FeatChoices) > 0 {
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  "feats", // New category for feats
			Source:    "player",
			Selection: d.FeatChoices,
		})
	}

	// Process equipment choices
	equipment := make([]string, 0)

	// Add starting equipment from class
	for _, eq := range classData.StartingEquipment {
		equipment = append(equipment, formatEquipmentWithQuantity(eq.ItemID, eq.Quantity))
	}

	// Add equipment from background
	equipment = append(equipment, backgroundData.Equipment...)

	// Process equipment choices (expanding bundles)
	if len(d.EquipmentChoices) > 0 {
		chosenEquipment := processEquipmentChoices(d.EquipmentChoices)
		equipment = append(equipment, chosenEquipment...)
	}

	charData.Equipment = equipment

	// Initialize class resources
	classResources := initializeClassResources(classData, 1, charData.AbilityScores)
	resourcesData := make(map[string]ResourceData)
	for name, res := range classResources {
		resourcesData[name] = ResourceData{
			Name:    res.Name,
			Max:     res.Max,
			Current: res.Current,
			Resets:  string(res.Resets),
		}
	}
	charData.ClassResources = resourcesData

	// Initialize spell slots
	spellSlots := initializeSpellSlots(classData, 1)
	charData.SpellSlots = spellSlots

	charData.CreatedAt = time.Now()
	charData.UpdatedAt = time.Now()

	// Create the character domain object
	return LoadCharacterFromData(charData, raceData, classData, backgroundData)
}

// LoadDraftFromData creates a Draft from persistent data
func LoadDraftFromData(data DraftData) (*Draft, error) {
	if data.ID == "" {
		return nil, errors.New("draft ID is required")
	}

	return &Draft{
		ID:       data.ID,
		PlayerID: data.PlayerID,
		Name:     data.Name,
		Progress: DraftProgress{flags: data.ProgressFlags},

		// Load explicit choice fields
		RaceChoice:          data.RaceChoice,
		ClassChoice:         data.ClassChoice,
		BackgroundChoice:    data.BackgroundChoice,
		AbilityScoreChoice:  data.AbilityScoreChoice,
		SkillChoices:        data.SkillChoices,
		LanguageChoices:     data.LanguageChoices,
		FightingStyleChoice: data.FightingStyleChoice,
		SpellChoices:        data.SpellChoices,
		CantripChoices:      data.CantripChoices,
		EquipmentChoices:    data.EquipmentChoices,
		FeatChoices:         data.FeatChoices,

		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// ToData converts the draft to its persistent representation
func (d *Draft) ToData() DraftData {
	return DraftData{
		ID:            d.ID,
		PlayerID:      d.PlayerID,
		Name:          d.Name,
		ProgressFlags: d.Progress.flags,

		// Save explicit choice fields
		RaceChoice:          d.RaceChoice,
		ClassChoice:         d.ClassChoice,
		BackgroundChoice:    d.BackgroundChoice,
		AbilityScoreChoice:  d.AbilityScoreChoice,
		SkillChoices:        d.SkillChoices,
		LanguageChoices:     d.LanguageChoices,
		FightingStyleChoice: d.FightingStyleChoice,
		SpellChoices:        d.SpellChoices,
		CantripChoices:      d.CantripChoices,
		EquipmentChoices:    d.EquipmentChoices,
		FeatChoices:         d.FeatChoices,

		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// GetProgress returns information about the draft's completion status
func (d *Draft) GetProgress() DraftProgress {
	return d.Progress
}

// IsComplete returns true if the draft has all required fields to create a character
func (d *Draft) IsComplete() bool {
	return d.isComplete()
}

// Helper methods for DraftProgress

func (p *DraftProgress) setFlag(flag uint32) {
	p.flags |= flag
}

func (p *DraftProgress) hasFlag(flag uint32) bool {
	return p.flags&flag != 0
}

// applyAbilityScoreIncreases applies ability score increases to the given scores
func applyAbilityScoreIncreases(scores shared.AbilityScores, increases map[string]int) {
	// Convert string ability names to constants
	constIncreases := make(map[constants.Ability]int)
	for abilityStr, bonus := range increases {
		var ability constants.Ability
		switch abilityStr {
		case shared.AbilityStrength, "strength":
			ability = constants.STR
		case shared.AbilityDexterity, "dexterity":
			ability = constants.DEX
		case shared.AbilityConstitution, "constitution":
			ability = constants.CON
		case shared.AbilityIntelligence, "intelligence":
			ability = constants.INT
		case shared.AbilityWisdom, "wisdom":
			ability = constants.WIS
		case shared.AbilityCharisma, "charisma":
			ability = constants.CHA
		default:
			continue // Skip unknown abilities
		}
		constIncreases[ability] = bonus
	}

	// Apply the increases
	_ = scores.ApplyIncreases(constIncreases) // Ignore errors about exceeding 20 during creation
}
