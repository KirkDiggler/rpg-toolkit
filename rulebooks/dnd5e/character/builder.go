// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:      shared.ChoiceName,
		Source:        shared.SourcePlayer,
		ChoiceID:      "character_name",
		NameSelection: &name,
	})

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
		SubraceID: races.Subrace(subraceID),
	}

	if err := b.validator.ValidateRaceChoice(choice, &raceData); err != nil {
		return err
	}

	b.draft.RaceChoice = choice

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:      shared.ChoiceRace,
		Source:        shared.SourcePlayer,
		ChoiceID:      "race_selection",
		RaceSelection: &choice,
	})

	b.draft.Progress.setFlag(ProgressRace)
	b.draft.UpdatedAt = time.Now()

	// Note: Race-specific choices (e.g., variant human feat, half-elf skills)
	// should be handled through dedicated methods

	return nil
}

// SetClassData sets the character's class using class data
func (b *Builder) SetClassData(classData class.Data, subclassID classes.Subclass) error {
	if classData.ID == "" {
		return errors.New("class data must have an ID")
	}

	// Store the class data
	b.classData = &classData

	b.draft.ClassChoice = ClassChoice{
		ClassID:    classData.ID,
		SubclassID: subclassID,
	}

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:       shared.ChoiceClass,
		Source:         shared.SourcePlayer,
		ChoiceID:       "class_selection",
		ClassSelection: &b.draft.ClassChoice,
	})

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

	b.draft.BackgroundChoice = backgroundData.ID

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:            shared.ChoiceBackground,
		Source:              shared.SourcePlayer,
		ChoiceID:            "background_selection",
		BackgroundSelection: &backgroundData.ID,
	})

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

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:              shared.ChoiceAbilityScores,
		Source:                shared.SourcePlayer,
		ChoiceID:              "ability_scores",
		AbilityScoreSelection: &scores,
	})

	b.draft.Progress.setFlag(ProgressAbilityScores)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectSkills records skill proficiency selections
func (b *Builder) SelectSkills(selectedSkills []string) error {
	// Convert string skills to typed constants
	typedSkills := make([]skills.Skill, len(selectedSkills))
	for i, skill := range selectedSkills {
		typedSkills[i] = skills.Skill(skill)
	}

	// Validate skills are available based on class/background
	if err := b.validator.ValidateSkillSelection(b.draft, typedSkills, b.classData, b.backgroundData); err != nil {
		return err
	}

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	// Add skill choice
	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:       shared.ChoiceSkills,
		Source:         shared.SourceClass, // Skills chosen from class options
		ChoiceID:       fmt.Sprintf("%s_skill_proficiencies", b.classData.ID),
		SkillSelection: typedSkills,
	})

	b.draft.Progress.setFlag(ProgressSkills)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectLanguages records language selections
func (b *Builder) SelectLanguages(selectedLanguages []string) error {
	// Convert string languages to typed constants
	typedLanguages := make([]languages.Language, len(selectedLanguages))
	for i, lang := range selectedLanguages {
		typedLanguages[i] = languages.Language(lang)
	}

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	// Language choices could come from race or background
	// TODO(#159): Builder should track which source is requesting the choice
	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:          shared.ChoiceLanguages,
		Source:            shared.SourceRace, // Default to race, but this should be contextual
		ChoiceID:          "additional_languages",
		LanguageSelection: typedLanguages,
	})

	b.draft.Progress.setFlag(ProgressLanguages)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectFightingStyle records fighting style selection (for appropriate classes)
func (b *Builder) SelectFightingStyle(style string) error {
	// TODO: Validate fighting style is available to this class

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:               shared.ChoiceFightingStyle,
		Source:                 shared.SourceClass,
		ChoiceID:               fmt.Sprintf("%s_fighting_style", b.classData.ID),
		FightingStyleSelection: &style,
	})

	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectSpells records spell selections (for spellcasting classes)
func (b *Builder) SelectSpells(spells []string) error {
	// TODO: Validate spells against class spell list

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:       shared.ChoiceSpells,
		Source:         shared.SourceClass,
		ChoiceID:       fmt.Sprintf("%s_spells_known", b.classData.ID),
		SpellSelection: spells,
	})

	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectCantrips records cantrip selections (for spellcasting classes)
func (b *Builder) SelectCantrips(cantrips []string) error {
	// TODO: Validate cantrips against class cantrip list

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:         shared.ChoiceCantrips,
		Source:           shared.SourceClass,
		ChoiceID:         fmt.Sprintf("%s_cantrips", b.classData.ID),
		CantripSelection: cantrips,
	})

	b.draft.UpdatedAt = time.Now()
	return nil
}

// SelectEquipment records equipment selections
func (b *Builder) SelectEquipment(equipment []string) error {
	// TODO: Validate equipment choices against class/background options

	// Add to choices with source tracking
	if b.draft.Choices == nil {
		b.draft.Choices = []ChoiceData{}
	}

	b.draft.Choices = append(b.draft.Choices, ChoiceData{
		Category:           shared.ChoiceEquipment,
		Source:             shared.SourceClass,
		ChoiceID:           fmt.Sprintf("%s_starting_equipment", b.classData.ID),
		EquipmentSelection: equipment,
	})

	b.draft.Progress.setFlag(ProgressEquipment)
	b.draft.UpdatedAt = time.Now()
	return nil
}

// Validate checks if the draft is valid in its current state
// Deprecated: Use draft.ValidateBasicRequirements() and draft.ValidateChoices() directly
func (b *Builder) Validate() []ValidationError {
	// First check basic requirements
	if err := b.draft.ValidateBasicRequirements(); err != nil {
		// Convert rpgerr to ValidationError for backwards compatibility
		return []ValidationError{{
			Field:   "general",
			Message: err.Error(),
		}}
	}
	
	// Then check choices
	result, err := b.draft.ValidateChoices()
	if err != nil {
		return []ValidationError{{
			Field:   "choices",
			Message: err.Error(),
		}}
	}
	
	// Convert validation result to ValidationError slice for backwards compatibility
	var errors []ValidationError
	for _, e := range result.Errors {
		errors = append(errors, ValidationError{
			Field:   string(e.Field),
			Message: e.Message,
		})
	}
	for _, i := range result.Incomplete {
		errors = append(errors, ValidationError{
			Field:   string(i.Field),
			Message: i.Message,
		})
	}
	
	return errors
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
	// The draft's ToCharacter method handles all validation now
	// It will check both basic requirements and choice validation
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

	// Core identity choices
	RaceChoice         RaceChoice             `json:"race_choice"`
	ClassChoice        ClassChoice            `json:"class_choice"`
	BackgroundChoice   backgrounds.Background `json:"background_choice"`
	AbilityScoreChoice shared.AbilityScores   `json:"ability_score_choice"`

	// All choices with source tracking
	Choices []ChoiceData `json:"choices"`

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
	RaceID    races.Race    `json:"race_id"`
	SubraceID races.Subrace `json:"subrace_id,omitempty"`
}

// MissingRequiredSubrace returns true if this race requires a subrace but none is selected
func (rc RaceChoice) MissingRequiredSubrace() bool {
	// These races require subrace selection per PHB
	requiresSubrace := rc.RaceID == races.Elf ||
		rc.RaceID == races.Dwarf ||
		rc.RaceID == races.Halfling ||
		rc.RaceID == races.Gnome

	return requiresSubrace && rc.SubraceID == ""
}

// HasSubraceOptions returns true if this race has subrace options available
func (rc RaceChoice) HasSubraceOptions() bool {
	return rc.RaceID == races.Elf ||
		rc.RaceID == races.Dwarf ||
		rc.RaceID == races.Halfling ||
		rc.RaceID == races.Gnome
	// Add more races with subraces as they're implemented
}

// GetAutomaticGrants returns the automatic grants for this race (and subrace if applicable)
func (rc RaceChoice) GetAutomaticGrants() races.AutomaticGrants {
	// GetAutomaticGrants returns a value, not a pointer
	baseGrants := races.GetAutomaticGrants(rc.RaceID)

	// If we have a subrace, merge its grants
	// Note: This would need enhancement in the races package to handle subrace grants
	// For now, just return base race grants
	return baseGrants
}

// GetProficiencies returns the proficiencies for this race choice (including subrace)
func (rc RaceChoice) GetProficiencies() (Proficiencies, error) {
	if rc.RaceID == "" {
		return Proficiencies{}, rpgerr.New(rpgerr.CodeInvalidArgument, "race ID is required",
			rpgerr.WithMeta("raceID", rc.RaceID))
	}

	grants := rc.GetAutomaticGrants()

	prof := Proficiencies{
		Skills:    grants.Skills,
		Languages: grants.Languages,
	}

	// Weapon proficiencies would need to come from race data
	// which we don't have here. They're handled separately in compileCharacter.
	// TODO: Consider if we should pass race data here or handle differently

	return prof, nil
}

// IsValid validates that the race choice is complete and correct
func (rc RaceChoice) IsValid() error {
	if rc.RaceID == "" {
		return fmt.Errorf("race must be selected")
	}

	if rc.MissingRequiredSubrace() {
		return fmt.Errorf("%s requires a subrace selection", rc.RaceID)
	}

	// Could add validation that subrace belongs to race if we have that data
	return nil
}

// ClassChoice represents a class selection with optional subclass
type ClassChoice struct {
	ClassID    classes.Class    `json:"class_id"`
	SubclassID classes.Subclass `json:"subclass_id,omitempty"`
}

// MissingSubclass returns true if this class requires a subclass at level 1 but none is selected
func (cc ClassChoice) MissingSubclass() bool {
	if (cc.ClassID == classes.Cleric ||
		cc.ClassID == classes.Sorcerer ||
		cc.ClassID == classes.Warlock) && cc.SubclassID == "" {
		return true
	}

	return false
}

// RequiresSubclassAtLevel returns true if this class requires a subclass at the given level
func (cc ClassChoice) RequiresSubclassAtLevel(level int) bool {
	switch cc.ClassID {
	case classes.Cleric, classes.Sorcerer, classes.Warlock:
		return level >= 1
	case classes.Barbarian, classes.Bard, classes.Druid, classes.Monk, classes.Ranger, classes.Rogue, classes.Wizard:
		return level >= 2
	case classes.Fighter, classes.Paladin:
		return level >= 3
	default:
		return false
	}
}

// Proficiencies represents all proficiencies that can be granted by any source
type Proficiencies struct {
	Armor        []proficiencies.Armor
	Weapons      []proficiencies.Weapon
	Tools        []proficiencies.Tool
	Skills       []skills.Skill
	Languages    []languages.Language
	SavingThrows []abilities.Ability
}

// GetProficiencies returns the proficiencies for this class choice (including subclass)
func (cc ClassChoice) GetProficiencies() (Proficiencies, error) {
	if cc.ClassID == "" {
		return Proficiencies{}, rpgerr.New(rpgerr.CodeInvalidArgument, "class ID is required",
			rpgerr.WithMeta("classID", cc.ClassID))
	}

	if cc.SubclassID != "" {
		// Get subclass-specific grants (includes base class grants)
		grants := classes.GetSubclassGrants(cc.SubclassID)
		if grants == nil {
			// Subclass doesn't exist or has no grants - this is an error
			return Proficiencies{}, rpgerr.NewfWithOpts(rpgerr.CodeNotFound,
				[]rpgerr.Option{
					rpgerr.WithMeta("classID", cc.ClassID),
					rpgerr.WithMeta("subclassID", cc.SubclassID),
				},
				"subclass %s not found or has no grants", cc.SubclassID)
		}

		return Proficiencies{
			Armor:        grants.ArmorProficiencies,
			Weapons:      grants.WeaponProficiencies,
			Tools:        grants.ToolProficiencies,
			SavingThrows: grants.SavingThrows,
		}, nil
	}

	// Fall back to base class grants
	grants := classes.GetAutomaticGrants(cc.ClassID)
	if grants == nil {
		// Class doesn't exist or has no grants - this is an error
		return Proficiencies{}, rpgerr.NewfWithOpts(rpgerr.CodeNotFound,
			[]rpgerr.Option{
				rpgerr.WithMeta("classID", cc.ClassID),
			},
			"class %s not found or has no grants", cc.ClassID)
	}

	return Proficiencies{
		Armor:        grants.ArmorProficiencies,
		Weapons:      grants.WeaponProficiencies,
		Tools:        grants.ToolProficiencies,
		SavingThrows: grants.SavingThrows,
	}, nil
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
