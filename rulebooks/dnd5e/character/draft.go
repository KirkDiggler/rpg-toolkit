// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Draft represents a character in progress
type Draft struct {
	ID       string
	PlayerID string
	Name     string
	Progress DraftProgress

	// Core identity choices that need special handling
	RaceChoice         RaceChoice             `json:"race_choice"`
	ClassChoice        ClassChoice            `json:"class_choice"`
	BackgroundChoice   backgrounds.Background `json:"background_choice"`
	AbilityScoreChoice shared.AbilityScores   `json:"ability_score_choice"`

	// All choices with source tracking
	Choices []ChoiceData `json:"choices"`

	// Validation state - populated by ValidateChoices()
	ValidationWarnings []string `json:"validation_warnings,omitempty"`
	ValidationErrors   []string `json:"validation_errors,omitempty"`
	CanFinalize        bool     `json:"can_finalize"`

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
	charData := d.buildBaseCharacterData()

	// Apply ability score improvements
	d.applyAbilityScoreImprovements(&charData, raceData)

	// Calculate derived stats
	charData.MaxHitPoints = classData.HitDice + charData.AbilityScores.Modifier(abilities.CON)
	charData.HitPoints = charData.MaxHitPoints
	charData.Speed = raceData.Speed
	charData.Size = raceData.Size

	// Compile skills
	charData.Skills = d.compileSkills(raceData, backgroundData)

	// Compile languages
	charData.Languages = d.compileLanguages(raceData, backgroundData)

	// Proficiencies
	charData.Proficiencies = shared.Proficiencies{
		Armor:   classData.ArmorProficiencies,
		Weapons: append(classData.WeaponProficiencies, raceData.WeaponProficiencies...),
		Tools:   append(classData.ToolProficiencies, backgroundData.ToolProficiencies...),
	}

	// Saving throws
	charData.SavingThrows = make(map[abilities.Ability]shared.ProficiencyLevel)
	for _, save := range classData.SavingThrows {
		charData.SavingThrows[save] = shared.Proficient
	}

	// Compile all choices (fundamental and player-made)
	charData.Choices = d.compileChoices()

	// Compile equipment
	charData.Equipment = d.compileEquipment(classData, backgroundData)

	// Initialize class resources
	classResources := initializeClassResources(classData, 1, charData.AbilityScores)
	resourcesData := make(map[shared.ClassResourceType]ResourceData)
	for resType, res := range classResources {
		resourcesData[resType] = ResourceData{
			Type:    resType,
			Name:    res.Name,
			Max:     res.Max,
			Current: res.Current,
			Resets:  res.Resets,
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

	// Validate that class/race/background are valid constants
	// This ensures we fail fast on bad data from the database
	if data.ClassChoice.ClassID != "" {
		if _, err := classes.GetByID(string(data.ClassChoice.ClassID)); err != nil {
			return nil, fmt.Errorf("invalid class in draft data '%s': %w", data.ClassChoice.ClassID, err)
		}
	}

	if data.RaceChoice.RaceID != "" {
		if _, err := races.GetByID(string(data.RaceChoice.RaceID)); err != nil {
			return nil, fmt.Errorf("invalid race in draft data '%s': %w", data.RaceChoice.RaceID, err)
		}
	}

	if data.BackgroundChoice != "" {
		if _, err := backgrounds.GetByID(string(data.BackgroundChoice)); err != nil {
			return nil, fmt.Errorf("invalid background in draft data '%s': %w", data.BackgroundChoice, err)
		}
	}

	return &Draft{
		ID:       data.ID,
		PlayerID: data.PlayerID,
		Name:     data.Name,
		Progress: DraftProgress{flags: data.ProgressFlags},

		// Load core identity choices - now validated
		RaceChoice:         data.RaceChoice,
		ClassChoice:        data.ClassChoice,
		BackgroundChoice:   data.BackgroundChoice,
		AbilityScoreChoice: data.AbilityScoreChoice,

		// Load choices with source tracking
		Choices: data.Choices,

		// Load validation state
		ValidationWarnings: data.ValidationWarnings,
		ValidationErrors:   data.ValidationErrors,
		CanFinalize:        data.CanFinalize,

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

		// Save core identity choices
		RaceChoice:         d.RaceChoice,
		ClassChoice:        d.ClassChoice,
		BackgroundChoice:   d.BackgroundChoice,
		AbilityScoreChoice: d.AbilityScoreChoice,

		// Save choices with source tracking
		Choices: d.Choices,

		// Save validation state
		ValidationWarnings: d.ValidationWarnings,
		ValidationErrors:   d.ValidationErrors,
		CanFinalize:        d.CanFinalize,

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

// GetValidationStatus returns the current validation state of the draft
func (d *Draft) GetValidationStatus() (warnings []string, errors []string, canFinalize bool) {
	return d.ValidationWarnings, d.ValidationErrors, d.CanFinalize
}

// HasValidationIssues returns true if the draft has any validation errors or warnings
func (d *Draft) HasValidationIssues() bool {
	return len(d.ValidationErrors) > 0 || len(d.ValidationWarnings) > 0
}

// ValidateChoices validates the draft's choices using the new choices validation system
// and updates the draft's validation state
func (d *Draft) ValidateChoices() (*choices.ValidationResult, error) {
	// Clear previous validation state
	d.ValidationWarnings = nil
	d.ValidationErrors = nil
	d.CanFinalize = false

	// Can only validate if we have class/race/background selected
	if d.ClassChoice.ClassID == "" || d.RaceChoice.RaceID == "" {
		return nil, errors.New("class and race must be selected before validating choices")
	}

	// Convert ChoiceData to TypedSubmissions
	submissions := choices.NewTypedSubmissions()
	for _, choice := range d.Choices {
		// Convert shared types to choices types
		source := convertChoiceSource(choice.Source)
		field := convertChoiceCategory(choice.Category)
		values := extractChoiceValues(choice)

		if len(values) > 0 {
			submissions.AddChoice(choices.ChoiceSubmission{
				Source:   source,
				Field:    field,
				ChoiceID: choice.ChoiceID,
				Values:   values,
			})
		}
	}

	// Build validation context using the rulebook's knowledge of automatic grants
	context := d.buildValidationContextWithRulebookKnowledge()

	// Create validator and validate with typed constants from the Draft
	validator := choices.NewValidator(context)
	result := validator.ValidateAll(
		d.ClassChoice.ClassID,
		d.RaceChoice.RaceID,
		d.BackgroundChoice,
		1, // Level 1 for now
		submissions,
	)

	// Update draft's validation state from result
	d.updateValidationState(result)

	return result, nil
}

// ValidateChoicesWithData is deprecated. Use ValidateChoices() which now uses the rulebook's knowledge.
// This method will be removed before v1.0
//
// Deprecated: Use ValidateChoices instead
func (d *Draft) ValidateChoicesWithData(_ *race.Data, _ *shared.Background) (*choices.ValidationResult, error) {
	return d.ValidateChoices()
}

// updateValidationState updates the draft's validation fields from the validation result
func (d *Draft) updateValidationState(result *choices.ValidationResult) {
	// Collect error messages
	for _, err := range result.Errors {
		d.ValidationErrors = append(d.ValidationErrors, err.Message)
	}

	// Collect incomplete messages as errors (they prevent finalization)
	for _, inc := range result.Incomplete {
		d.ValidationErrors = append(d.ValidationErrors, inc.Message)
	}

	// Collect warning messages
	for _, warn := range result.Warnings {
		d.ValidationWarnings = append(d.ValidationWarnings, warn.Message)
	}

	// Update finalization status
	d.CanFinalize = result.CanFinalize
}

// buildValidationContext creates a validation context from the draft's current state
func (d *Draft) buildValidationContext() *choices.ValidationContext {
	context := choices.NewValidationContext()

	// Add skill proficiencies from all sources
	for _, choice := range d.Choices {
		switch choice.Category {
		case shared.ChoiceSkills:
			for _, skill := range choice.SkillSelection {
				context.AddProficiency(string(skill))
			}
		case shared.ChoiceToolProficiency:
			for _, tool := range choice.ToolProficiencySelection {
				context.AddProficiency(tool)
			}
		}
	}

	context.CharacterLevel = 1 // For now, always level 1
	context.ClassLevel = 1

	return context
}

// buildValidationContextWithGrants creates a validation context with automatic grants populated
// This is used when we have the full race and background data available
// buildValidationContextWithRulebookKnowledge uses the rulebook's knowledge to populate automatic grants
func (d *Draft) buildValidationContextWithRulebookKnowledge() *choices.ValidationContext {
	context := d.buildValidationContext()

	// Get automatic grants from the race using rulebook knowledge
	raceGrants := races.GetAutomaticGrants(d.RaceChoice.RaceID)

	// Add automatic skill grants from race
	for _, skill := range raceGrants.Skills {
		context.AddAutomaticGrant(choices.FieldSkills, string(skill), choices.SourceRace)
	}

	// Add automatic language grants from race
	for _, lang := range raceGrants.Languages {
		context.AddAutomaticGrant(choices.FieldLanguages, string(lang), choices.SourceRace)
	}

	// Get automatic grants from the background using rulebook knowledge
	bgGrants := backgrounds.GetAutomaticGrants(d.BackgroundChoice)

	// Add automatic skill grants from background
	for _, skill := range bgGrants.Skills {
		context.AddAutomaticGrant(choices.FieldSkills, string(skill), choices.SourceBackground)
	}

	// TODO: Add language and tool grants from background when they exist

	return context
}

// Helper methods for DraftProgress

func (p *DraftProgress) setFlag(flag uint32) {
	p.flags |= flag
}

func (p *DraftProgress) hasFlag(flag uint32) bool {
	return p.flags&flag != 0
}

// applyAbilityScoreIncreases applies ability score increases to the given scores
func applyAbilityScoreIncreases(scores shared.AbilityScores, increases map[abilities.Ability]int) {
	// Apply the increases directly - no conversion needed!
	_ = scores.ApplyIncreases(increases) // Ignore errors about exceeding 20 during creation
}

// buildBaseCharacterData creates the base character data structure from draft
func (d *Draft) buildBaseCharacterData() Data {
	data := Data{
		ID:            d.ID,
		PlayerID:      d.PlayerID,
		Name:          d.Name,
		Level:         1, // Starting level
		RaceID:        d.RaceChoice.RaceID,
		ClassID:       d.ClassChoice.ClassID,
		BackgroundID:  d.BackgroundChoice,
		AbilityScores: d.AbilityScoreChoice,
	}

	// Set subrace ID if present
	if d.RaceChoice.SubraceID != "" {
		data.SubraceID = d.RaceChoice.SubraceID
	}

	// Set subclass ID if present
	if d.ClassChoice.SubclassID != "" {
		data.SubclassID = d.ClassChoice.SubclassID
	}

	return data
}

// applyAbilityScoreImprovements applies racial and subracial ability score improvements
func (d *Draft) applyAbilityScoreImprovements(charData *Data, raceData *race.Data) {
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
}

// compileSkills combines chosen skills with automatic grants from race and background
func (d *Draft) compileSkills(
	raceData *race.Data, backgroundData *shared.Background,
) map[skills.Skill]shared.ProficiencyLevel {
	skills := make(map[skills.Skill]shared.ProficiencyLevel)

	// Extract chosen skills from Choices
	for _, choice := range d.Choices {
		if choice.Category == shared.ChoiceSkills && choice.SkillSelection != nil {
			for _, skill := range choice.SkillSelection {
				skills[skill] = shared.Proficient
			}
		}
	}

	// Add background skills (automatic grants)
	// Note: If a skill is already proficient (e.g., Half-Orc gets Intimidation,
	// player chooses Intimidation from Fighter), this is fine - you just don't
	// get double proficiency. The map structure naturally handles this.
	for _, skill := range backgroundData.SkillProficiencies {
		skills[skill] = shared.Proficient
	}

	// Add any racial skill proficiencies
	if raceData.SkillProficiencies != nil {
		for _, skill := range raceData.SkillProficiencies {
			skills[skill] = shared.Proficient
		}
	}

	return skills
}

// compileLanguages combines chosen languages with automatic grants from race and background
func (d *Draft) compileLanguages(raceData *race.Data, backgroundData *shared.Background) []string {
	// Start with ensuring Common is always included
	languageSet := make(map[string]bool)
	languageSet[string(languages.Common)] = true

	// Add race languages (automatic grants)
	for _, lang := range raceData.Languages {
		languageSet[string(lang)] = true
	}

	// Add background languages (automatic grants)
	for _, lang := range backgroundData.Languages {
		languageSet[string(lang)] = true
	}

	// Extract chosen languages from Choices
	for _, choice := range d.Choices {
		if choice.Category == shared.ChoiceLanguages && choice.LanguageSelection != nil {
			for _, lang := range choice.LanguageSelection {
				languageSet[string(lang)] = true
			}
		}
	}

	// Convert set to slice
	languages := make([]string, 0, len(languageSet))
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	return languages
}

// compileChoices creates the complete list of choices including fundamental choices
func (d *Draft) compileChoices() []ChoiceData {
	// Reserve capacity for fundamental choices (name, race, class, background, ability scores)
	const fundamentalChoiceCount = 5
	choices := make([]ChoiceData, 0, len(d.Choices)+fundamentalChoiceCount)

	// Add fundamental choices that should always be tracked
	if d.Name != "" {
		choices = append(choices, ChoiceData{
			Category:      shared.ChoiceName,
			Source:        shared.SourcePlayer,
			ChoiceID:      "character_name",
			NameSelection: &d.Name,
		})
	}

	if d.RaceChoice.RaceID != "" {
		choices = append(choices, ChoiceData{
			Category:      shared.ChoiceRace,
			Source:        shared.SourcePlayer,
			ChoiceID:      "race_selection",
			RaceSelection: &d.RaceChoice,
		})
	}

	if d.ClassChoice.ClassID != "" {
		choices = append(choices, ChoiceData{
			Category:       shared.ChoiceClass,
			Source:         shared.SourcePlayer,
			ChoiceID:       "class_selection",
			ClassSelection: &d.ClassChoice,
		})
	}

	if d.BackgroundChoice != "" {
		choices = append(choices, ChoiceData{
			Category:            shared.ChoiceBackground,
			Source:              shared.SourcePlayer,
			ChoiceID:            "background_selection",
			BackgroundSelection: &d.BackgroundChoice,
		})
	}

	if len(d.AbilityScoreChoice) > 0 {
		choices = append(choices, ChoiceData{
			Category:              shared.ChoiceAbilityScores,
			Source:                shared.SourcePlayer,
			ChoiceID:              "ability_scores",
			AbilityScoreSelection: &d.AbilityScoreChoice,
		})
	}

	// Add remaining choices from draft
	choices = append(choices, d.Choices...)

	return choices
}

// compileEquipment combines starting equipment with player choices
func (d *Draft) compileEquipment(classData *class.Data, backgroundData *shared.Background) []string {
	equipment := make([]string, 0)

	// Add starting equipment from class
	for _, eq := range classData.StartingEquipment {
		equipment = append(equipment, formatEquipmentWithQuantity(eq.ItemID, eq.Quantity))
	}

	// Add equipment from background
	equipment = append(equipment, backgroundData.Equipment...)

	// Extract equipment choices from Choices
	for _, choice := range d.Choices {
		if choice.Category == shared.ChoiceEquipment && choice.EquipmentSelection != nil {
			chosenEquipment := processEquipmentChoices(choice.EquipmentSelection)
			equipment = append(equipment, chosenEquipment...)
		}
	}

	return equipment
}
