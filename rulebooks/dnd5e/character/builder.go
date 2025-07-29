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

// Builder handles the multi-step character creation process
type Builder struct {
	draft     *Draft
	validator *Validator

	// Data storage - populated as choices are made
	raceData       *race.Data
	classData      *class.Data
	backgroundData *shared.Background
}

// Progress flags
const (
	ProgressName uint32 = 1 << iota
	ProgressRace
	ProgressClass
	ProgressBackground
	ProgressAbilityScores
	ProgressSkills
	ProgressLanguages
	ProgressEquipment
	ProgressSpells
)

// NewCharacterBuilder creates a new builder with the provided draft ID
func NewCharacterBuilder(draftID string) (*Builder, error) {
	if draftID == "" {
		return nil, errors.New("draft ID is required")
	}
	return &Builder{
		draft: &Draft{
			ID:        draftID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		validator: NewValidator(),
	}, nil
}

// LoadDraft creates a builder from existing draft data
func LoadDraft(data DraftData) (*Builder, error) {
	draft, err := LoadDraftFromData(data)
	if err != nil {
		return nil, err
	}

	return &Builder{
		draft:     draft,
		validator: NewValidator(),
	}, nil
}

// SetName sets the character's name
func (b *Builder) SetName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}

	b.draft.Name = name
	b.draft.Progress.setFlag(ProgressName)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SetRaceData sets the character's race using race data
func (b *Builder) SetRaceData(raceData race.Data, subraceID string) error {
	if raceData.ID == "" {
		return errors.New("race data must have an ID")
	}

	// Store the race data
	b.raceData = &raceData

	choice := RaceChoice{
		RaceID:    constants.Race(raceData.ID),
		SubraceID: constants.Subrace(subraceID),
	}

	if err := b.validator.ValidateRaceChoice(choice, &raceData); err != nil {
		return err
	}

	b.draft.RaceChoice = choice
	b.draft.Progress.setFlag(ProgressRace)
	b.draft.UpdatedAt = time.Now()

	// Note: Race-specific choices (e.g., variant human feat, half-elf skills)
	// should be handled through dedicated methods

	return nil
}

// SetClassData sets the character's class using class data
func (b *Builder) SetClassData(classData class.Data) error {
	if classData.ID == "" {
		return errors.New("class data must have an ID")
	}

	// Store the class data
	b.classData = &classData

	b.draft.ClassChoice = constants.Class(classData.ID)
	b.draft.Progress.setFlag(ProgressClass)
	b.draft.UpdatedAt = time.Now()

	// Note: Class-specific choices (skills, equipment, etc.) will be set
	// through their dedicated methods (SelectSkills, etc.)

	return nil
}

// SetBackgroundData sets the character's background using background data
func (b *Builder) SetBackgroundData(backgroundData shared.Background) error {
	if backgroundData.ID == "" {
		return errors.New("background data must have an ID")
	}

	// Store the background data
	b.backgroundData = &backgroundData

	b.draft.BackgroundChoice = constants.Background(backgroundData.ID)
	b.draft.Progress.setFlag(ProgressBackground)
	b.draft.UpdatedAt = time.Now()

	// Note: Background-specific choices (extra languages, tools)
	// should be handled through SelectLanguages or similar methods

	return nil
}

// SetAbilityScores sets the character's ability scores
func (b *Builder) SetAbilityScores(scores shared.AbilityScores) error {
	if err := b.validator.ValidateAbilityScores(scores); err != nil {
		return err
	}

	b.draft.AbilityScoreChoice = scores
	b.draft.Progress.setFlag(ProgressAbilityScores)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectSkills records skill proficiency selections
func (b *Builder) SelectSkills(skills []string) error {
	// Convert string skills to typed constants
	typedSkills := make([]constants.Skill, len(skills))
	for i, skill := range skills {
		typedSkills[i] = constants.Skill(skill)
	}

	// Validate skills are available based on class/background
	if err := b.validator.ValidateSkillSelection(b.draft, typedSkills, b.classData, b.backgroundData); err != nil {
		return err
	}
	b.draft.SkillChoices = typedSkills
	b.draft.Progress.setFlag(ProgressSkills)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectLanguages records language selections
func (b *Builder) SelectLanguages(languages []string) error {
	// Convert string languages to typed constants
	typedLanguages := make([]constants.Language, len(languages))
	for i, lang := range languages {
		typedLanguages[i] = constants.Language(lang)
	}
	b.draft.LanguageChoices = typedLanguages
	b.draft.Progress.setFlag(ProgressLanguages)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectFightingStyle records fighting style selection (for appropriate classes)
func (b *Builder) SelectFightingStyle(style string) error {
	// TODO: Validate fighting style is available to this class
	b.draft.FightingStyleChoice = style
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectSpells records spell selections (for spellcasting classes)
func (b *Builder) SelectSpells(spells []string) error {
	// TODO: Validate spells against class spell list
	b.draft.SpellChoices = spells
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectCantrips records cantrip selections (for spellcasting classes)
func (b *Builder) SelectCantrips(cantrips []string) error {
	// TODO: Validate cantrips against class cantrip list
	b.draft.CantripChoices = cantrips
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectEquipment records equipment selections
func (b *Builder) SelectEquipment(equipment []string) error {
	// TODO: Validate equipment choices against class/background options
	b.draft.EquipmentChoices = equipment
	b.draft.Progress.setFlag(ProgressEquipment)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// Validate checks if the draft is valid in its current state
func (b *Builder) Validate() []ValidationError {
	return b.validator.ValidateDraft(b.draft, b.raceData, b.classData, b.backgroundData)
}

// Progress returns the current progress of character creation
func (b *Builder) Progress() BuilderProgress {
	// Calculate total steps based on what's actually needed
	totalSteps := b.calculateTotalSteps()
	completedSteps := b.calculateCompletedSteps()

	percentComplete := float32(0)
	if totalSteps > 0 {
		percentComplete = float32(completedSteps) / float32(totalSteps) * 100
	}

	return BuilderProgress{
		CurrentStep:     b.getCurrentStep(),
		CompletedSteps:  b.getCompletedSteps(),
		PercentComplete: percentComplete,
		CanBuild:        b.canBuild(),
	}
}

// Build creates the final character from the draft
func (b *Builder) Build() (*Character, error) {
	if !b.canBuild() {
		return nil, errors.New("character draft is incomplete")
	}

	errors := b.Validate()
	if len(errors) > 0 {
		return nil, fmt.Errorf("validation failed: %v", errors)
	}

	// Use the draft's ToCharacter method
	return b.draft.ToCharacter(b.raceData, b.classData, b.backgroundData)
}

// ToData converts the draft to its persistent representation
func (b *Builder) ToData() DraftData {
	return b.draft.ToData()
}

// DraftData is the persistent representation of a draft
type DraftData struct {
	ID            string `json:"id"`
	PlayerID      string `json:"player_id"`
	Name          string `json:"name"`
	ProgressFlags uint32 `json:"progress_flags"`

	// Explicit typed choices - matches Draft struct
	RaceChoice          RaceChoice           `json:"race_choice"`
	ClassChoice         constants.Class      `json:"class_choice"`
	BackgroundChoice    constants.Background `json:"background_choice"`
	AbilityScoreChoice  shared.AbilityScores `json:"ability_score_choice"`
	SkillChoices        []constants.Skill    `json:"skill_choices"`
	LanguageChoices     []constants.Language `json:"language_choices"`
	FightingStyleChoice string               `json:"fighting_style_choice,omitempty"`
	SpellChoices        []string             `json:"spell_choices,omitempty"`
	CantripChoices      []string             `json:"cantrip_choices,omitempty"`
	EquipmentChoices    []string             `json:"equipment_choices,omitempty"`
	FeatChoices         []string             `json:"feat_choices,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BuilderProgress provides information about the current state
type BuilderProgress struct {
	CurrentStep     string
	CompletedSteps  []string
	PercentComplete float32
	CanBuild        bool
}

// Helper methods

func (b *Builder) getCurrentStep() string {
	if !b.draft.Progress.hasFlag(ProgressName) {
		return "name"
	}
	if !b.draft.Progress.hasFlag(ProgressRace) {
		return "race"
	}
	if !b.draft.Progress.hasFlag(ProgressClass) {
		return "class"
	}
	if !b.draft.Progress.hasFlag(ProgressBackground) {
		return "background"
	}
	if !b.draft.Progress.hasFlag(ProgressAbilityScores) {
		return "ability_scores"
	}
	if !b.draft.Progress.hasFlag(ProgressSkills) {
		return "skills"
	}
	return "equipment"
}

func (b *Builder) getCompletedSteps() []string {
	steps := []string{}
	if b.draft.Progress.hasFlag(ProgressName) {
		steps = append(steps, "name")
	}
	if b.draft.Progress.hasFlag(ProgressRace) {
		steps = append(steps, "race")
	}
	if b.draft.Progress.hasFlag(ProgressClass) {
		steps = append(steps, "class")
	}
	if b.draft.Progress.hasFlag(ProgressBackground) {
		steps = append(steps, "background")
	}
	if b.draft.Progress.hasFlag(ProgressAbilityScores) {
		steps = append(steps, "ability_scores")
	}
	if b.draft.Progress.hasFlag(ProgressSkills) {
		steps = append(steps, "skills")
	}
	return steps
}

func (b *Builder) canBuild() bool {
	required := ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores
	return b.draft.Progress.flags&required == required
}

func (b *Builder) calculateTotalSteps() int {
	// Base required steps
	steps := 5 // Name, Race, Class, Background, Ability Scores

	// Add optional steps based on context
	if b.classData != nil {
		// Skills are always needed
		steps++

		// Spells only for spellcasters
		if b.classData.Spellcasting != nil {
			steps++
		}
	}

	// Languages might be needed based on race choices
	if b.raceData != nil && b.raceData.LanguageChoice != nil {
		steps++
	}

	// Equipment choices (future enhancement)
	// steps++

	return steps
}

func (b *Builder) calculateCompletedSteps() int {
	count := 0

	// Count required steps
	if b.draft.Progress.hasFlag(ProgressName) {
		count++
	}
	if b.draft.Progress.hasFlag(ProgressRace) {
		count++
	}
	if b.draft.Progress.hasFlag(ProgressClass) {
		count++
	}
	if b.draft.Progress.hasFlag(ProgressBackground) {
		count++
	}
	if b.draft.Progress.hasFlag(ProgressAbilityScores) {
		count++
	}

	// Count contextual steps
	if b.draft.Progress.hasFlag(ProgressSkills) {
		count++
	}

	// Only count spells if class is a spellcaster
	if b.draft.Progress.hasFlag(ProgressSpells) && b.classData != nil && b.classData.Spellcasting != nil {
		count++
	}

	// Only count languages if there was a choice to make
	if b.draft.Progress.hasFlag(ProgressLanguages) && b.raceData != nil && b.raceData.LanguageChoice != nil {
		count++
	}

	return count
}

// RaceChoice represents a race selection with optional subrace
type RaceChoice struct {
	RaceID    constants.Race    `json:"race_id"`
	SubraceID constants.Subrace `json:"subrace_id,omitempty"`
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
