package character

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Draft represents a character in the creation process
type Draft struct {
	id       string
	playerID string

	// Basic info
	name string

	// Core choices
	race       races.Race
	subrace    races.Subrace
	class      classes.Class
	subclass   classes.Subclass
	background backgrounds.Background

	// Ability scores (before racial modifiers)
	baseAbilityScores shared.AbilityScores

	// Player choices stored for validation
	choices []choices.ChoiceData

	// Progress tracking
	progress Progress

	// Tracking
	createdAt time.Time
	updatedAt time.Time
}

// DraftConfig holds configuration for creating a new draft
type DraftConfig struct {
	ID       string
	PlayerID string
}

// Validate ensures the config is valid
func (c *DraftConfig) Validate() error {
	if c.ID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "draft ID is required")
	}
	if c.PlayerID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "player ID is required")
	}
	return nil
}

// NewDraft creates a new character draft
func NewDraft(config *DraftConfig) (*Draft, error) {
	if config == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "config is required")
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	return &Draft{
		id:                config.ID,
		playerID:          config.PlayerID,
		baseAbilityScores: make(shared.AbilityScores),
		choices:           make([]choices.ChoiceData, 0),
		progress:          ProgressNone,
		createdAt:         now,
		updatedAt:         now,
	}, nil
}

// Getter methods

// ID returns the draft ID
func (d *Draft) ID() string {
	return d.id
}

// PlayerID returns the player ID
func (d *Draft) PlayerID() string {
	return d.playerID
}

// Name returns the character name
func (d *Draft) Name() string {
	return d.name
}

// Race returns the selected race
func (d *Draft) Race() races.Race {
	return d.race
}

// Subrace returns the selected subrace
func (d *Draft) Subrace() races.Subrace {
	return d.subrace
}

// Class returns the selected class
func (d *Draft) Class() classes.Class {
	return d.class
}

// Subclass returns the selected subclass
func (d *Draft) Subclass() classes.Subclass {
	return d.subclass
}

// Background returns the selected background
func (d *Draft) Background() backgrounds.Background {
	return d.background
}

// BaseAbilityScores returns the base ability scores
func (d *Draft) BaseAbilityScores() shared.AbilityScores {
	return d.baseAbilityScores
}

// Choices returns the player's choices
func (d *Draft) Choices() []choices.ChoiceData {
	return d.choices
}

// Progress returns the draft progress
func (d *Draft) Progress() Progress {
	return d.progress
}

// CreatedAt returns when the draft was created
func (d *Draft) CreatedAt() time.Time {
	return d.createdAt
}

// UpdatedAt returns when the draft was last updated
func (d *Draft) UpdatedAt() time.Time {
	return d.updatedAt
}

// SetName sets the character's name
func (d *Draft) SetName(input *SetNameInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}

	if input.Name == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "name cannot be empty")
	}

	d.name = input.Name
	d.updatedAt = time.Now()

	// Record the choice
	d.recordChoice(choices.ChoiceData{
		Category:      shared.ChoiceName,
		Source:        shared.SourcePlayer,
		NameSelection: &input.Name,
	})

	// Update progress
	d.progress.Set(ProgressName)

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

	d.race = input.RaceID
	d.subrace = input.SubraceID

	// Record language choices if any
	if len(input.Choices.Languages) > 0 {
		// Map to correct ChoiceID based on race
		var choiceID choices.ChoiceID
		switch d.race {
		case races.Human:
			choiceID = choices.HumanLanguage
		case races.HalfElf:
			choiceID = choices.HalfElfLanguage
		case races.Elf:
			if d.subrace == races.HighElf {
				choiceID = choices.HighElfLanguage
			}
		}
		d.recordChoice(choices.ChoiceData{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceRace,
			ChoiceID:          choiceID,
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

	d.updatedAt = time.Now()

	// Update progress if race choices are complete
	if d.IsRaceComplete() {
		d.progress.Set(ProgressRace)
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

	d.class = input.ClassID
	d.subclass = input.SubclassID

	// Get class requirements once for all choice recording
	requirements := choices.GetClassRequirements(d.class)

	// Record skill choices
	if len(input.Choices.Skills) > 0 {
		var choiceID choices.ChoiceID
		if requirements.Skills != nil {
			choiceID = requirements.Skills.ID
		}
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       choiceID,
			SkillSelection: input.Choices.Skills,
		})
	}

	// Record fighting style (for Fighter, Paladin, etc.)
	if input.Choices.FightingStyle != "" {
		style := input.Choices.FightingStyle
		var choiceID choices.ChoiceID
		if requirements.FightingStyle != nil {
			choiceID = requirements.FightingStyle.ID
		}
		d.recordChoice(choices.ChoiceData{
			Category:               shared.ChoiceFightingStyle,
			Source:                 shared.SourceClass,
			ChoiceID:               choiceID,
			FightingStyleSelection: &style,
		})
	}

	// Record cantrips (for spellcasters)
	if len(input.Choices.Cantrips) > 0 {
		var choiceID choices.ChoiceID
		if requirements.Cantrips != nil {
			choiceID = requirements.Cantrips.ID
		}
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceCantrips,
			Source:         shared.SourceClass,
			ChoiceID:       choiceID,
			SpellSelection: input.Choices.Cantrips,
		})
	}

	// Record spells (for spellcasters)
	if len(input.Choices.Spells) > 0 {
		var choiceID choices.ChoiceID
		if requirements.Spellbook != nil {
			choiceID = requirements.Spellbook.ID
		}
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceSpells,
			Source:         shared.SourceClass,
			ChoiceID:       choiceID,
			SpellSelection: input.Choices.Spells,
		})
	}

	// Record equipment choices
	if len(input.Choices.Equipment) > 0 {
		for choiceID, selectionID := range input.Choices.Equipment {
			// Check if it's a regular equipment choice (with options)
			for _, req := range requirements.Equipment {
				if req.ID == choiceID {
					// Find the selected option
					for _, opt := range req.Options {
						if opt.ID == selectionID {
							// Extract equipment IDs from this option and validate them
							equipmentIDs := make([]shared.SelectionID, 0, len(opt.Items))
							for _, item := range opt.Items {
								// Validate that the equipment exists
								_, err := equipment.GetByID(item.ID)
								if err != nil {
									// Return error immediately - invalid equipment in class requirements
									return rpgerr.Newf(rpgerr.CodeNotFound,
										"invalid equipment ID '%s' in class requirements", item.ID)
								}
								equipmentIDs = append(equipmentIDs, item.ID)
							}

							d.recordChoice(choices.ChoiceData{
								Category:           shared.ChoiceEquipment,
								Source:             shared.SourceClass,
								ChoiceID:           choiceID,
								OptionID:           opt.ID,
								EquipmentSelection: equipmentIDs,
							})
							break
						}
					}
					break
				}
			}

			// Check if it's a category-based choice
			for _, catReq := range requirements.EquipmentCategories {
				if catReq.ID == choiceID {
					// For category choices, selectionID is the actual equipment ID
					// Validate the equipment ID exists
					_, err := equipment.GetByID(selectionID)
					if err != nil {
						return rpgerr.Newf(rpgerr.CodeNotFound,
							"invalid equipment ID '%s'", selectionID)
					}
					d.recordChoice(choices.ChoiceData{
						Category:           shared.ChoiceEquipment,
						Source:             shared.SourceClass,
						ChoiceID:           choiceID,
						EquipmentSelection: []shared.SelectionID{selectionID},
					})
					break
				}
			}
		}
	}

	d.updatedAt = time.Now()

	// Update progress if class choices are complete
	if d.IsClassComplete() {
		d.progress.Set(ProgressClass)
	}

	return nil
}

// SetBackground sets the character's background
func (d *Draft) SetBackground(input *SetBackgroundInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}

	// TODO: Validate background when we have internal background data
	d.background = input.BackgroundID

	// Record language choices
	if len(input.Choices.Languages) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceBackground,
			LanguageSelection: input.Choices.Languages,
		})
	}

	d.updatedAt = time.Now()

	// Update progress if background choices are complete
	if d.IsBackgroundComplete() {
		d.progress.Set(ProgressBackground)
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

	d.baseAbilityScores = input.Scores

	// Record the choice with method
	d.recordChoice(choices.ChoiceData{
		Category:              shared.ChoiceAbilityScores,
		Source:                shared.SourcePlayer,
		AbilityScoreSelection: input.Scores,
		Method:                input.Method,
	})

	d.updatedAt = time.Now()

	// Update progress
	d.progress.Set(ProgressAbilityScores)

	return nil
}

// ToCharacter converts the draft to a playable character
func (d *Draft) ToCharacter(ctx context.Context, characterID string, bus events.EventBus) (*Character, error) {
	// Validate we have all required data
	if characterID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "character ID is required")
	}
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if d.name == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character name is required")
	}
	if d.race == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character race is required")
	}
	if d.class == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character class is required")
	}
	if d.background == "" {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "character background is required")
	}
	if len(d.baseAbilityScores) != 6 {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet, "all ability scores must be set")
	}

	// Get race and class data
	raceData := races.GetData(d.race)
	if raceData == nil {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "unknown race: %s", d.race)
	}

	classData := classes.GetData(d.class)
	if classData == nil {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "unknown class: %s", d.class)
	}

	// Calculate final ability scores (base + racial modifiers)
	finalScores := make(shared.AbilityScores)
	for ability, baseScore := range d.baseAbilityScores {
		finalScores[ability] = baseScore
	}

	// Apply racial ability score improvements
	for ability, bonus := range raceData.AbilityIncreases {
		finalScores[ability] += bonus
	}

	// Calculate starting HP
	maxHP := classData.HitDice + finalScores.Modifier(abilities.CON)

	// Build proficiencies
	skillProfs := d.compileSkills(raceData)
	savingThrows := d.compileSavingThrows(classData)

	// Compile features (can fail)
	charFeatures, err := d.compileFeatures()
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to compile features")
	}

	// Create the character
	char := &Character{
		id:               characterID,
		playerID:         d.playerID,
		name:             d.name,
		level:            1,
		proficiencyBonus: 2,
		raceID:           d.race,
		subraceID:        d.subrace,
		classID:          d.class,
		subclassID:       d.subclass,
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
		classResources:   make(map[shared.ClassResourceType]ResourceData),
		features:         charFeatures,
		bus:              bus,
		conditions:       make([]dnd5eEvents.ConditionBehavior, 0),
		subscriptionIDs:  make([]string, 0),
	}

	// Subscribe to events - character comes out fully initialized
	if err := char.subscribeToEvents(ctx); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to subscribe to events")
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
	for _, choice := range d.choices {
		switch choice.Category {
		case shared.ChoiceSkills:
			if len(choice.SkillSelection) > 0 {
				skillValues := make([]shared.SelectionID, 0, len(choice.SkillSelection))
				skillValues = append(skillValues, choice.SkillSelection...)
				submissions.Add(choices.Submission{
					Category: shared.ChoiceSkills,
					Source:   choice.Source,
					ChoiceID: choice.ChoiceID,
					Values:   skillValues,
				})
			}
		case shared.ChoiceEquipment:
			if len(choice.EquipmentSelection) > 0 {
				// Validate all equipment IDs exist before adding to submissions
				for _, equipID := range choice.EquipmentSelection {
					_, err := equipment.GetByID(equipID)
					if err != nil {
						return rpgerr.Newf(rpgerr.CodeNotFound,
							"invalid equipment ID in stored choices: %s", equipID)
					}
				}

				// For equipment bundles with options, use the option ID as the value
				// For category-based choices, use the actual equipment IDs
				values := choice.EquipmentSelection
				if choice.OptionID != "" {
					// This is a bundle choice - use the option ID as the single value
					values = []shared.SelectionID{choice.OptionID}
				}
				submissions.Add(choices.Submission{
					Category: shared.ChoiceEquipment,
					Source:   choice.Source,
					ChoiceID: choice.ChoiceID,
					OptionID: choice.OptionID,
					Values:   values,
				})
			}
		case shared.ChoiceLanguages:
			if len(choice.LanguageSelection) > 0 {
				langValues := make([]shared.SelectionID, 0, len(choice.LanguageSelection))
				langValues = append(langValues, choice.LanguageSelection...)
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
	result := validator.ValidateCharacterCreation(d.class, d.race, submissions)

	if !result.Valid {
		// Return first error as rpgerr
		if len(result.Errors) > 0 {
			err := result.Errors[0]
			return rpgerr.New(rpgerr.CodeInvalidArgument, err.Message,
				rpgerr.WithMeta("category", string(err.Category)),
				rpgerr.WithMeta("source", string(err.Source)))
		}
	}

	// If validation passed, update progress flags
	if result.Valid {
		// Check and update each progress flag
		if d.name != "" {
			d.progress.Set(ProgressName)
		}
		if d.IsRaceComplete() {
			d.progress.Set(ProgressRace)
		}
		if d.IsClassComplete() {
			d.progress.Set(ProgressClass)
		}
		if d.IsBackgroundComplete() {
			d.progress.Set(ProgressBackground)
		}
		// Check if all ability scores are set (non-zero)
		if d.baseAbilityScores[abilities.STR] > 0 &&
			d.baseAbilityScores[abilities.DEX] > 0 &&
			d.baseAbilityScores[abilities.CON] > 0 &&
			d.baseAbilityScores[abilities.INT] > 0 &&
			d.baseAbilityScores[abilities.WIS] > 0 &&
			d.baseAbilityScores[abilities.CHA] > 0 {
			d.progress.Set(ProgressAbilityScores)
		}
	}

	return nil
}

// recordChoice adds or updates a choice in the draft
func (d *Draft) recordChoice(choice choices.ChoiceData) {
	// Remove any existing choice with the same choiceID (for equipment) or same category and source (for others)
	filtered := make([]choices.ChoiceData, 0, len(d.choices))
	for _, c := range d.choices {
		// For equipment choices, check choiceID since we can have multiple equipment choices
		if choice.Category == shared.ChoiceEquipment && c.Category == shared.ChoiceEquipment {
			if c.ChoiceID != choice.ChoiceID {
				filtered = append(filtered, c)
			}
		} else {
			// For non-equipment choices, check category and source as before
			if c.Category != choice.Category || c.Source != choice.Source {
				filtered = append(filtered, c)
			}
		}
	}
	filtered = append(filtered, choice)

	d.choices = filtered
}

// TODO: check if class can grant skills or all they all chosen
// compileSkills builds the skill proficiency map
func (d *Draft) compileSkills(raceData *races.Data) map[skills.Skill]shared.ProficiencyLevel {
	skills := make(map[skills.Skill]shared.ProficiencyLevel)

	// Add racial skill proficiencies
	for _, skill := range raceData.Skills {
		skills[skill] = shared.Proficient
	}

	// Add chosen skills from choices
	for _, choice := range d.choices {
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
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceLanguages {
			langs = append(langs, choice.LanguageSelection...)
		}
	}

	return langs
}

// compileInventory builds the inventory from equipment choices and grants
func (d *Draft) compileInventory() []InventoryItem {
	inventory := make([]InventoryItem, 0)

	// Add starting equipment from class grants
	if d.class != "" {
		if classGrants := classes.GetAutomaticGrants(d.class); classGrants != nil {
			for _, item := range classGrants.StartingEquipment {
				equip, err := equipment.GetByID(item.ID)
				if err != nil {
					panic(fmt.Sprintf("BUG: Invalid equipment ID in class grants for %s: %s - %v", d.class, item.ID, err))
				}
				inventory = append(inventory, InventoryItem{
					Equipment: equip,
					Quantity:  item.Quantity,
				})
			}
		}
	}

	// Add equipment from choices (user selections)
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceEquipment {
			for _, equipID := range choice.EquipmentSelection {
				// Each selection is a separate item (no merging)
				equip, err := equipment.GetByID(equipID)
				if err != nil {
					// This should never happen if SetClass validation is working correctly
					panic(fmt.Sprintf("BUG: Invalid equipment ID in draft choices: %s - %v", equipID, err))
				}
				inventory = append(inventory, InventoryItem{
					Equipment: equip,
					Quantity:  1, // Default quantity for choices
				})
			}
		}
	}

	return inventory
}

// compileSpellSlots determines starting spell slots
func (d *Draft) compileSpellSlots(classData *classes.Data) map[int]SpellSlotData {
	slots := make(map[int]SpellSlotData)

	// Only spellcasters get spell slots
	if classData.SpellcastingAbility == "" {
		return slots
	}

	// Level 1 spell slots based on class
	switch d.class {
	case classes.Wizard, classes.Sorcerer, classes.Cleric, classes.Druid, classes.Bard:
		slots[1] = SpellSlotData{Max: 2, Used: 0}
	case classes.Warlock:
		slots[1] = SpellSlotData{Max: 1, Used: 0}
	case classes.Ranger, classes.Paladin:
		// Half-casters don't get spells until level 2
	}

	return slots
}

// compileFeatures returns the character's class features
func (d *Draft) compileFeatures() ([]features.Feature, error) {
	featureList := make([]features.Feature, 0)

	// Level 1 barbarian gets rage
	if d.class == classes.Barbarian {
		// Create rage feature with proper JSON data
		rageData := map[string]interface{}{
			"ref": core.Ref{
				Module: "dnd5e",
				Type:   "features",
				Value:  "rage",
			},
			"id":       "rage",
			"name":     "Rage",
			"level":    1,
			"uses":     2, // Level 1 has 2 uses
			"max_uses": 2,
		}

		// Create rage from JSON
		jsonBytes, err := json.Marshal(rageData)
		if err != nil {
			return nil, rpgerr.WrapWithCode(err, rpgerr.CodeInternal, "failed to marshal rage data")
		}

		rage, err := features.LoadJSON(jsonBytes)
		if err != nil {
			return nil, rpgerr.WrapWithCode(err, rpgerr.CodeInternal, "failed to load rage feature")
		}
		featureList = append(featureList, rage)
	}

	// TODO: Add other class features (second wind for fighter, etc)

	return featureList, nil
}

// Progress validation methods

// IsRaceComplete checks if race selection and all race choices are complete
func (d *Draft) IsRaceComplete() bool {
	if d.race == "" {
		return false
	}

	// Get race requirements
	reqs := choices.GetRaceRequirements(d.race)
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
	if d.class == "" {
		return false
	}

	// Get class requirements (includes subclass if needed at level 1)
	reqs := choices.GetClassRequirements(d.class)
	if reqs == nil {
		return true // No choices required
	}

	// Check if subclass is required at level 1
	if needsSubclassAtLevel1(d.class) && d.subclass == "" {
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
	if d.background == "" {
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

	for _, choice := range d.choices {
		if choice.Source == shared.SourceRace {
			// Convert ChoiceData to Submission
			// This would need proper mapping of choice data to submission format
			// For now, simplified version
			if len(choice.SkillSelection) > 0 {
				skillValues := make([]shared.SelectionID, 0, len(choice.SkillSelection))
				skillValues = append(skillValues, choice.SkillSelection...)
				subs.Add(choices.Submission{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceRace,
					ChoiceID: choices.HalfElfSkills, // Would need to map based on race
					Values:   skillValues,
				})
			}

			// Handle language choices
			if len(choice.LanguageSelection) > 0 {
				langValues := make([]shared.SelectionID, 0, len(choice.LanguageSelection))
				langValues = append(langValues, choice.LanguageSelection...)
				// Map to correct ChoiceID based on race
				var choiceID choices.ChoiceID
				switch d.race {
				case races.Human:
					choiceID = choices.HumanLanguage
				case races.HalfElf:
					choiceID = choices.HalfElfLanguage
				case races.Elf:
					// Check if it's High Elf subrace
					if d.subrace == races.HighElf {
						choiceID = choices.HighElfLanguage
					}
				default:
					// For other races that might have language choices
					choiceID = choices.ChoiceID(string(d.race) + "-language")
				}
				subs.Add(choices.Submission{
					Category: shared.ChoiceLanguages,
					Source:   shared.SourceRace,
					ChoiceID: choiceID,
					Values:   langValues,
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

	for _, choice := range d.choices {
		if choice.Source == shared.SourceClass {
			// Convert ChoiceData to Submission
			// This would need proper mapping of choice data to submission format
			// For now, simplified version
			if len(choice.SkillSelection) > 0 {
				// Use the choice ID that was stored when SetClass was called
				skillValues := make([]shared.SelectionID, 0, len(choice.SkillSelection))
				skillValues = append(skillValues, choice.SkillSelection...)
				subs.Add(choices.Submission{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: choice.ChoiceID, // Already stored correctly by SetClass
					Values:   skillValues,
				})
			}

			// Handle equipment choices
			if len(choice.EquipmentSelection) > 0 {
				// For equipment bundles with options, use the option ID as the value
				// For category-based choices, use the actual equipment IDs
				values := make([]shared.SelectionID, 0)
				if choice.OptionID != "" {
					// This is a bundle choice - use the option ID as the single value
					values = append(values, choice.OptionID)
				} else {
					// Category-based choice - use the equipment IDs
					values = choice.EquipmentSelection
				}
				subs.Add(choices.Submission{
					Category: shared.ChoiceEquipment,
					Source:   shared.SourceClass,
					ChoiceID: choice.ChoiceID,
					OptionID: choice.OptionID,
					Values:   values,
				})
			}

			// Handle fighting style choices
			if choice.FightingStyleSelection != nil {
				subs.Add(choices.Submission{
					Category: shared.ChoiceFightingStyle,
					Source:   shared.SourceClass,
					ChoiceID: choices.FighterFightingStyle, // Would need mapping for other classes
					Values:   []shared.SelectionID{*choice.FightingStyleSelection},
				})
			}

			// Add other choice types...
		}
	}

	return subs
}
