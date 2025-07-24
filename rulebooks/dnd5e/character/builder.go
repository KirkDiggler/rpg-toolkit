// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
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

// Draft represents a character in progress
type Draft struct {
	ID        string
	PlayerID  string
	Name      string
	Choices   map[shared.ChoiceCategory]any
	Progress  DraftProgress
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DraftProgress tracks completion of character creation steps
type DraftProgress struct {
	flags uint32
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
			Choices:   make(map[shared.ChoiceCategory]any),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		validator: NewValidator(),
	}, nil
}

// LoadDraft creates a builder from existing draft data
func LoadDraft(data DraftData) (*Builder, error) {
	draft := &Draft{
		ID:        data.ID,
		PlayerID:  data.PlayerID,
		Name:      data.Name,
		Choices:   data.Choices,
		Progress:  DraftProgress{flags: data.ProgressFlags},
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
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
	b.draft.Choices[shared.ChoiceName] = name
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
		RaceID:    raceData.ID,
		SubraceID: subraceID,
	}

	if err := b.validator.ValidateRaceChoice(choice, &raceData); err != nil {
		return err
	}

	b.draft.Choices[shared.ChoiceRace] = choice
	if subraceID != "" {
		b.draft.Choices[shared.ChoiceSubrace] = subraceID
	}
	b.draft.Progress.setFlag(ProgressRace)
	b.draft.UpdatedAt = time.Now()

	// Add race choices to the draft
	raceObj := race.LoadFromData(raceData)
	for _, choice := range raceObj.GetChoices() {
		// These need to be resolved by the user
		b.draft.Choices[shared.ChoiceCategory("race_"+choice.ID)] = choice
	}

	return nil
}

// SetClassData sets the character's class using class data
func (b *Builder) SetClassData(classData class.Data) error {
	if classData.ID == "" {
		return errors.New("class data must have an ID")
	}

	// Store the class data
	b.classData = &classData

	b.draft.Choices[shared.ChoiceClass] = classData.ID
	b.draft.Progress.setFlag(ProgressClass)
	b.draft.UpdatedAt = time.Now()

	// Add class choices to the draft (skills, equipment)
	classObj := class.LoadFromData(classData)
	for _, choice := range classObj.GetChoicesAtLevel(1) {
		b.draft.Choices[shared.ChoiceCategory("class_"+choice.ID)] = choice
	}

	return nil
}

// SetBackgroundData sets the character's background using background data
func (b *Builder) SetBackgroundData(backgroundData shared.Background) error {
	if backgroundData.ID == "" {
		return errors.New("background data must have an ID")
	}

	// Store the background data
	b.backgroundData = &backgroundData

	b.draft.Choices[shared.ChoiceBackground] = backgroundData.ID
	b.draft.Progress.setFlag(ProgressBackground)
	b.draft.UpdatedAt = time.Now()

	// Add background choices if any
	if backgroundData.LanguageChoice != nil {
		b.draft.Choices[shared.ChoiceCategory("background_language")] = *backgroundData.LanguageChoice
	}
	if backgroundData.ToolChoice != nil {
		b.draft.Choices[shared.ChoiceCategory("background_tool")] = *backgroundData.ToolChoice
	}

	return nil
}

// SetAbilityScores sets the character's ability scores
func (b *Builder) SetAbilityScores(scores shared.AbilityScores) error {
	if err := b.validator.ValidateAbilityScores(scores); err != nil {
		return err
	}

	b.draft.Choices[shared.ChoiceAbilityScores] = scores
	b.draft.Progress.setFlag(ProgressAbilityScores)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectSkills records skill proficiency selections
func (b *Builder) SelectSkills(skills []string) error {
	// Validate skills are available based on class/background
	if err := b.validator.ValidateSkillSelection(b.draft, skills, b.classData, b.backgroundData); err != nil {
		return err
	}

	b.draft.Choices[shared.ChoiceSkills] = skills
	b.draft.Progress.setFlag(ProgressSkills)
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

	// Compile all choices into final character
	return b.compileCharacter()
}

// ToData converts the draft to its persistent representation
func (b *Builder) ToData() DraftData {
	return DraftData{
		ID:            b.draft.ID,
		PlayerID:      b.draft.PlayerID,
		Name:          b.draft.Name,
		Choices:       b.draft.Choices,
		ProgressFlags: b.draft.Progress.flags,
		CreatedAt:     b.draft.CreatedAt,
		UpdatedAt:     b.draft.UpdatedAt,
	}
}

// DraftData is the persistent representation of a draft
type DraftData struct {
	ID            string                        `json:"id"`
	PlayerID      string                        `json:"player_id"`
	Name          string                        `json:"name"`
	Choices       map[shared.ChoiceCategory]any `json:"choices"`
	ProgressFlags uint32                        `json:"progress_flags"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

// BuilderProgress provides information about the current state
type BuilderProgress struct {
	CurrentStep     string
	CompletedSteps  []string
	PercentComplete float32
	CanBuild        bool
}

// Helper methods

func (p *DraftProgress) setFlag(flag uint32) {
	p.flags |= flag
}

func (p *DraftProgress) hasFlag(flag uint32) bool {
	return p.flags&flag != 0
}

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

func (b *Builder) compileCharacter() (*Character, error) {
	if b.raceData == nil || b.classData == nil || b.backgroundData == nil {
		return nil, errors.New("missing required data: race, class, or background")
	}

	// Start with base character data
	charData := Data{
		ID:           b.draft.ID,
		Name:         b.draft.Name,
		Level:        1, // Starting level
		RaceID:       b.raceData.ID,
		ClassID:      b.classData.ID,
		BackgroundID: b.backgroundData.ID,
	}

	// Get ability scores from choices
	if scores, ok := b.draft.Choices[shared.ChoiceAbilityScores].(shared.AbilityScores); ok {
		charData.AbilityScores = scores
		// Apply racial ability score improvements
		for ability, bonus := range b.raceData.AbilityScoreIncreases {
			switch ability {
			case shared.AbilityStrength:
				charData.AbilityScores.Strength += bonus
			case shared.AbilityDexterity:
				charData.AbilityScores.Dexterity += bonus
			case shared.AbilityConstitution:
				charData.AbilityScores.Constitution += bonus
			case shared.AbilityIntelligence:
				charData.AbilityScores.Intelligence += bonus
			case shared.AbilityWisdom:
				charData.AbilityScores.Wisdom += bonus
			case shared.AbilityCharisma:
				charData.AbilityScores.Charisma += bonus
			}
		}
	}

	// Calculate HP
	charData.MaxHitPoints = b.classData.HitPointsAt1st + ((charData.AbilityScores.Constitution - 10) / 2)
	charData.HitPoints = charData.MaxHitPoints

	// Skills
	charData.Skills = make(map[string]int)
	if skills, ok := b.draft.Choices[shared.ChoiceSkills].([]string); ok {
		for _, skill := range skills {
			charData.Skills[skill] = int(shared.Proficient)
		}
	}
	// Add background skills
	for _, skill := range b.backgroundData.SkillProficiencies {
		charData.Skills[skill] = int(shared.Proficient)
	}

	// Languages
	charData.Languages = append([]string{}, b.raceData.Languages...)
	charData.Languages = append(charData.Languages, b.backgroundData.Languages...)
	// TODO: Add language choices

	// Proficiencies
	charData.Proficiencies = shared.Proficiencies{
		Armor:   b.classData.ArmorProficiencies,
		Weapons: append(b.classData.WeaponProficiencies, b.raceData.WeaponProficiencies...),
		Tools:   append(b.classData.ToolProficiencies, b.backgroundData.ToolProficiencies...),
	}

	// Saving throws
	charData.SavingThrows = make(map[string]int)
	for _, save := range b.classData.SavingThrows {
		charData.SavingThrows[save] = int(shared.Proficient)
	}

	// Store choices made
	for category, choice := range b.draft.Choices {
		if choiceData, ok := choice.(shared.ChoiceData); ok {
			charData.Choices = append(charData.Choices, ChoiceData{
				Category:  string(category),
				Source:    "draft",
				Selection: choiceData,
			})
		}
	}

	charData.CreatedAt = time.Now()
	charData.UpdatedAt = time.Now()

	// Create the character domain object
	return LoadCharacterFromData(charData, b.raceData, b.classData, b.backgroundData)
}

// RaceChoice represents a race selection with optional subrace
type RaceChoice struct {
	RaceID    string `json:"race_id"`
	SubraceID string `json:"subrace_id,omitempty"`
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
