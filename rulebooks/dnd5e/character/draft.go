package character

import (
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Draft represents a character in the creation process
type Draft struct {
	ID       string
	PlayerID string
	
	// Basic info
	Name string
	
	// Core choices
	Race       races.Race
	Subrace    races.Subrace
	Class      classes.Class
	Subclass   classes.Subclass
	Background backgrounds.Background
	
	// Ability scores (before racial modifiers)
	BaseAbilityScores shared.AbilityScores
	
	// Player choices stored for validation
	Choices []choices.ChoiceData
	
	// Progress tracking
	Progress Progress
	
	// Tracking
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewDraft creates a new character draft
func NewDraft(playerID string) *Draft {
	return &Draft{
		ID:                fmt.Sprintf("draft_%d", time.Now().UnixNano()),
		PlayerID:          playerID,
		BaseAbilityScores: make(shared.AbilityScores),
		Choices:           make([]choices.ChoiceData, 0),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// SetName sets the character's name
func (d *Draft) SetName(input *SetNameInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	
	if input.Name == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "name cannot be empty")
	}
	
	d.Name = input.Name
	d.UpdatedAt = time.Now()
	
	// Record the choice
	d.recordChoice(choices.ChoiceData{
		Category:      shared.ChoiceName,
		Source:        shared.SourcePlayer,
		NameSelection: &input.Name,
	})
	
	// Update progress
	d.Progress.Set(ProgressName)
	
	return nil
}

// SetRace sets the character's race and subrace
func (d *Draft) SetRace(input *SetRaceInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	
	// Validate race exists
	raceData := races.GetData(input.RaceID)
	if raceData == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "unknown race: %s", input.RaceID)
	}
	
	d.Race = input.RaceID
	d.Subrace = input.SubraceID
	
	// Record language choices if any
	if len(input.Choices.Languages) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			LanguageSelection: input.Choices.Languages,
		})
	}
	
	// Record skill choices (for Half-Elf, etc.)
	if len(input.Choices.Skills) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			SkillSelection: input.Choices.Skills,
		})
	}
	
	// Record cantrip choices (for High Elf, etc.)
	if len(input.Choices.Cantrips) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceCantrips,
			Source:         shared.SourceRace,
			SpellSelection: input.Choices.Cantrips,
		})
	}
	
	d.UpdatedAt = time.Now()
	
	// Update progress if race choices are complete
	if d.IsRaceComplete() {
		d.Progress.Set(ProgressRace)
	}
	
	return nil
}

// SetClass sets the character's class and subclass
func (d *Draft) SetClass(input *SetClassInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	
	// Validate class exists
	classData := classes.GetData(input.ClassID)
	if classData == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "unknown class: %s", input.ClassID)
	}
	
	// Validate skill count
	if len(input.Choices.Skills) != classData.SkillCount {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, 
			"must choose exactly %d skills, got %d", 
			classData.SkillCount, len(input.Choices.Skills))
	}
	
	d.Class = input.ClassID
	d.Subclass = input.SubclassID
	
	// Record skill choices
	if len(input.Choices.Skills) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			SkillSelection: input.Choices.Skills,
		})
	}
	
	// Record fighting style (for Fighter, Paladin, etc.)
	if input.Choices.FightingStyle != "" {
		style := input.Choices.FightingStyle
		d.recordChoice(choices.ChoiceData{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			FightingStyleSelection: &style,
		})
	}
	
	// Record cantrips (for spellcasters)
	if len(input.Choices.Cantrips) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceCantrips,
			Source:         shared.SourceClass,
			SpellSelection: input.Choices.Cantrips,
		})
	}
	
	// Record spells (for spellcasters)
	if len(input.Choices.Spells) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			SpellSelection: input.Choices.Spells,
		})
	}
	
	d.UpdatedAt = time.Now()
	
	// Update progress if class choices are complete
	if d.IsClassComplete() {
		d.Progress.Set(ProgressClass)
	}
	
	return nil
}

// SetBackground sets the character's background
func (d *Draft) SetBackground(input *SetBackgroundInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	
	// TODO: Validate background when we have internal background data
	d.Background = input.BackgroundID
	
	// Record language choices
	if len(input.Choices.Languages) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceBackground,
			LanguageSelection: input.Choices.Languages,
		})
	}
	
	d.UpdatedAt = time.Now()
	
	// Update progress if background choices are complete
	if d.IsBackgroundComplete() {
		d.Progress.Set(ProgressBackground)
	}
	
	return nil
}

// SetAbilityScores sets the character's base ability scores
func (d *Draft) SetAbilityScores(input *SetAbilityScoresInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	
	// Validate all 6 scores are present
	requiredAbilities := []abilities.Ability{
		abilities.STR, abilities.DEX, abilities.CON,
		abilities.INT, abilities.WIS, abilities.CHA,
	}
	
	for _, ability := range requiredAbilities {
		score, ok := input.Scores[ability]
		if !ok {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument, "missing score for %s", ability)
		}
		
		// Validate range (3-18 for base scores)
		if score < 3 || score > 18 {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument, 
				"%s score %d is outside valid range (3-18)", ability, score)
		}
	}
	
	d.BaseAbilityScores = input.Scores
	
	// Record the choice with method
	d.recordChoice(choices.ChoiceData{
		Category:              shared.ChoiceAbilityScores,
		Source:                shared.SourcePlayer,
		AbilityScoreSelection: input.Scores,
		Method:                input.Method,
	})
	
	d.UpdatedAt = time.Now()
	
	// Update progress
	d.Progress.Set(ProgressAbilityScores)
	
	return nil
}

// ToCharacter converts the draft to a playable character
func (d *Draft) ToCharacter() (*Character, error) {
	// Validate we have all required data
	if d.Name == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character name is required")
	}
	if d.Race == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character race is required")
	}
	if d.Class == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character class is required")
	}
	if d.Background == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character background is required")
	}
	if len(d.BaseAbilityScores) != 6 {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "all ability scores must be set")
	}
	
	// Get race and class data
	raceData := races.GetData(d.Race)
	if raceData == nil {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "unknown race: %s", d.Race)
	}
	
	classData := classes.GetData(d.Class)
	if classData == nil {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "unknown class: %s", d.Class)
	}
	
	// Calculate final ability scores (base + racial modifiers)
	finalScores := make(shared.AbilityScores)
	for ability, baseScore := range d.BaseAbilityScores {
		finalScores[ability] = baseScore
	}
	
	// Apply racial ability score improvements
	for ability, bonus := range raceData.AbilityIncreases {
		finalScores[ability] += bonus
	}
	
	// Calculate starting HP
	maxHP := classData.HitDice + finalScores.Modifier(abilities.CON)
	
	// Build proficiencies
	skillProfs := d.compileSkills(raceData, classData)
	savingThrows := d.compileSavingThrows(classData)
	
	// Create the character
	char := &Character{
		id:               fmt.Sprintf("char_%d", time.Now().UnixNano()),
		playerID:         d.PlayerID,
		name:             d.Name,
		level:            1,
		proficiencyBonus: 2,
		raceID:           d.Race,
		subraceID:        d.Subrace,
		classID:          d.Class,
		subclassID:       d.Subclass,
		abilityScores:    finalScores,
		hitPoints:        maxHP,
		maxHitPoints:     maxHP,
		armorClass:       10 + finalScores.Modifier(abilities.DEX), // Base AC
		hitDice:          classData.HitDice,
		skills:           skillProfs,
		savingThrows:     savingThrows,
		languages:        d.compileLanguages(raceData),
		inventory:        d.compileInventory(),
		spellSlots:       d.compileSpellSlots(classData),
		classResources:   make(map[shared.ClassResourceType]Resource),
	}
	
	return char, nil
}

// ValidateChoices validates that all required choices have been made
func (d *Draft) ValidateChoices() error {
	// Create validator
	validator := choices.NewValidator()
	
	// Convert draft choices to submissions
	submissions := choices.NewSubmissions()
	
	// Process stored choices into submissions
	for _, choice := range d.Choices {
		switch choice.Category {
		case shared.ChoiceSkills:
			if len(choice.SkillSelection) > 0 {
				skillValues := make([]string, 0, len(choice.SkillSelection))
				for _, skill := range choice.SkillSelection {
					skillValues = append(skillValues, string(skill))
				}
				submissions.Add(choices.Submission{
					Category: shared.ChoiceSkills,
					Source:   choice.Source,
					ChoiceID: choice.ChoiceID,
					Values:   skillValues,
				})
			}
		case shared.ChoiceEquipment:
			if len(choice.EquipmentSelection) > 0 {
				submissions.Add(choices.Submission{
					Category: shared.ChoiceEquipment,
					Source:   choice.Source,
					ChoiceID: choice.ChoiceID,
					Values:   choice.EquipmentSelection,
				})
			}
		case shared.ChoiceLanguages:
			if len(choice.LanguageSelection) > 0 {
				langValues := make([]string, 0, len(choice.LanguageSelection))
				for _, lang := range choice.LanguageSelection {
					langValues = append(langValues, string(lang))
				}
				submissions.Add(choices.Submission{
					Category: shared.ChoiceLanguages,
					Source:   choice.Source,
					ChoiceID: choice.ChoiceID,
					Values:   langValues,
				})
			}
		}
	}
	
	// Validate choices
	result := validator.ValidateCharacterCreation(d.Class, d.Race, submissions)
	
	if !result.Valid {
		// Return first error as rpgerr
		if len(result.Errors) > 0 {
			err := result.Errors[0]
			return rpgerr.New(rpgerr.CodeInvalidArgument, err.Message,
				rpgerr.WithMeta("category", string(err.Category)),
				rpgerr.WithMeta("source", string(err.Source)))
		}
	}
	
	return nil
}

// recordChoice adds or updates a choice in the draft
func (d *Draft) recordChoice(choice choices.ChoiceData) {
	// Remove any existing choice of the same category and source
	filtered := make([]choices.ChoiceData, 0, len(d.Choices))
	for _, c := range d.Choices {
		if c.Category != choice.Category || c.Source != choice.Source {
			filtered = append(filtered, c)
		}
	}
	d.Choices = append(filtered, choice)
}

// compileSkills builds the skill proficiency map
func (d *Draft) compileSkills(raceData *races.Data, classData *classes.Data) map[skills.Skill]shared.ProficiencyLevel {
	skills := make(map[skills.Skill]shared.ProficiencyLevel)
	
	// Add racial skill proficiencies
	for _, skill := range raceData.Skills {
		skills[skill] = shared.Proficient
	}
	
	// Add chosen skills from choices
	for _, choice := range d.Choices {
		if choice.Category == shared.ChoiceSkills {
			for _, skill := range choice.SkillSelection {
				skills[skill] = shared.Proficient
			}
		}
	}
	
	// TODO: Add background skills when we have internal background data
	
	return skills
}

// compileSavingThrows builds the saving throw proficiency map
func (d *Draft) compileSavingThrows(classData *classes.Data) map[abilities.Ability]shared.ProficiencyLevel {
	saves := make(map[abilities.Ability]shared.ProficiencyLevel)
	
	for _, ability := range classData.SavingThrows {
		saves[ability] = shared.Proficient
	}
	
	return saves
}

// compileLanguages builds the language list
func (d *Draft) compileLanguages(raceData *races.Data) []languages.Language {
	langs := make([]languages.Language, 0)
	
	// Add racial languages
	langs = append(langs, raceData.Languages...)
	
	// Add chosen languages
	for _, choice := range d.Choices {
		if choice.Category == shared.ChoiceLanguages {
			langs = append(langs, choice.LanguageSelection...)
		}
	}
	
	return langs
}

// compileInventory builds the inventory from equipment choices
func (d *Draft) compileInventory() []InventoryItem {
	inventory := make([]InventoryItem, 0)
	
	// Add chosen equipment
	// TODO: This currently uses string equipment selections from choices
	// We need to convert these to proper Equipment items
	// For now, return empty inventory since we don't have equipment data yet
	
	// TODO: Add starting equipment from class and background
	// TODO: Handle equipment packs (Explorer's Pack, etc.)
	// TODO: Handle quantities (20 arrows, 2 handaxes, etc.)
	
	return inventory
}

// compileSpellSlots determines starting spell slots
func (d *Draft) compileSpellSlots(classData *classes.Data) map[int]SpellSlot {
	slots := make(map[int]SpellSlot)
	
	// Only spellcasters get spell slots
	if classData.SpellcastingAbility == "" {
		return slots
	}
	
	// Level 1 spell slots based on class
	switch d.Class {
	case classes.Wizard, classes.Sorcerer, classes.Cleric, classes.Druid, classes.Bard:
		slots[1] = SpellSlot{Max: 2, Used: 0}
	case classes.Warlock:
		slots[1] = SpellSlot{Max: 1, Used: 0}
	case classes.Ranger, classes.Paladin:
		// Half-casters don't get spells until level 2
	}
	
	return slots
}

// Progress validation methods

// IsRaceComplete checks if race selection and all race choices are complete
func (d *Draft) IsRaceComplete() bool {
	if d.Race == "" {
		return false
	}
	
	// Get race requirements
	reqs := choices.GetRaceRequirements(d.Race)
	if reqs == nil {
		return true // No choices required
	}
	
	// Create submissions from draft choices
	subs := d.getRaceSubmissions()
	
	// Validate
	validator := choices.NewValidator()
	result := validator.Validate(reqs, subs)
	
	return result.Valid
}

// IsClassComplete checks if class selection and all class choices are complete
func (d *Draft) IsClassComplete() bool {
	if d.Class == "" {
		return false
	}
	
	// Get class requirements (includes subclass if needed at level 1)
	reqs := choices.GetClassRequirements(d.Class)
	if reqs == nil {
		return true // No choices required
	}
	
	// Check if subclass is required at level 1
	if needsSubclassAtLevel1(d.Class) && d.Subclass == "" {
		return false
	}
	
	// Create submissions from draft choices
	subs := d.getClassSubmissions()
	
	// Validate
	validator := choices.NewValidator()
	result := validator.Validate(reqs, subs)
	
	return result.Valid
}

// IsBackgroundComplete checks if background selection and choices are complete
func (d *Draft) IsBackgroundComplete() bool {
	if d.Background == "" {
		return false
	}
	
	// TODO: Get background requirements when we have background data
	// For now, just check that background is set
	return true
}

// Helper to check if class needs subclass at level 1
func needsSubclassAtLevel1(classID classes.Class) bool {
	classData := classes.ClassData[classID]
	return classData != nil && classData.SubclassLevel == 1
}

// getRaceSubmissions extracts race-related submissions from draft choices
func (d *Draft) getRaceSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()
	
	for _, choice := range d.Choices {
		if choice.Source == shared.SourceRace {
			// Convert ChoiceData to Submission
			// This would need proper mapping of choice data to submission format
			// For now, simplified version
			if len(choice.SkillSelection) > 0 {
				skillValues := make([]string, 0, len(choice.SkillSelection))
				for _, skill := range choice.SkillSelection {
					skillValues = append(skillValues, string(skill))
				}
				subs.Add(choices.Submission{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceRace,
					ChoiceID: choices.HalfElfSkills, // Would need to map based on race
					Values:   skillValues,
				})
			}
			// Add other choice types...
		}
	}
	
	return subs
}

// getClassSubmissions extracts class-related submissions from draft choices
func (d *Draft) getClassSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()
	
	for _, choice := range d.Choices {
		if choice.Source == shared.SourceClass {
			// Convert ChoiceData to Submission
			// This would need proper mapping of choice data to submission format
			// For now, simplified version
			if len(choice.SkillSelection) > 0 {
				skillValues := make([]string, 0, len(choice.SkillSelection))
				for _, skill := range choice.SkillSelection {
					skillValues = append(skillValues, string(skill))
				}
				// Map to correct ChoiceID based on class
				var choiceID choices.ChoiceID
				switch d.Class {
				case classes.Fighter:
					choiceID = choices.FighterSkills
				case classes.Rogue:
					choiceID = choices.RogueSkills
				case classes.Wizard:
					choiceID = choices.WizardSkills
				case classes.Cleric:
					choiceID = choices.ClericSkills
				}
				subs.Add(choices.Submission{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: choiceID,
					Values:   skillValues,
				})
			}
			// Add other choice types...
		}
	}
	
	return subs
}