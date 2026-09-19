package character

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// Draft represents a character in the creation process
type Draft struct {
	id       string
	playerID string

	// Basic info
	name string

	// Appearance
	appearance *customization.Appearance

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

// Appearance returns a deep copy of the draft's appearance, or nil when none
// has been selected.
func (d *Draft) Appearance() *customization.Appearance {
	return customization.CloneAppearance(d.appearance)
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

// GetFightingStyleSelection returns the selected fighting style, or nil if none chosen
func (d *Draft) GetFightingStyleSelection() *fightingstyles.FightingStyle {
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceFightingStyle && choice.FightingStyleSelection != nil {
			return choice.FightingStyleSelection
		}
	}
	return nil
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

	// Clear all existing race choices before recording new ones
	// This prevents accumulation when changing races
	d.clearChoicesBySource(shared.SourceRace)

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
		var choiceID choices.ChoiceID
		if d.race == races.HalfElf {
			choiceID = choices.HalfElfSkills
		}
		d.recordChoice(choices.ChoiceData{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceRace,
			ChoiceID:       choiceID,
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

	// Record tool proficiency choices (for Dwarf, etc.)
	if len(input.Choices.Tools) > 0 {
		var choiceID choices.ChoiceID
		if d.race == races.Dwarf {
			choiceID = choices.DwarfToolProficiency
		}
		// Convert SelectionID to proficiencies.Tool
		toolSelection := make([]proficiencies.Tool, 0, len(input.Choices.Tools))
		for _, t := range input.Choices.Tools {
			toolSelection = append(toolSelection, proficiencies.Tool(t))
		}
		d.recordChoice(choices.ChoiceData{
			Category:      shared.ChoiceToolProficiency,
			Source:        shared.SourceRace,
			ChoiceID:      choiceID,
			ToolSelection: toolSelection,
		})
	}

	d.updatedAt = time.Now()

	// Update progress if race choices are complete
	if d.IsRaceComplete() {
		d.progress.Set(ProgressRace)
	}

	return nil
}

// SetAppearance validates and stores a deep copy of the character's
// appearance. A nil input or appearance is rejected without mutating the draft.
func (d *Draft) SetAppearance(input *SetAppearanceInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.Appearance == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "appearance cannot be nil")
	}
	if err := customization.ValidateAppearance(input.Appearance); err != nil {
		return err
	}

	d.appearance = customization.CloneAppearance(input.Appearance)
	d.updatedAt = time.Now()

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

	// Clear all existing class choices before recording new ones
	// This prevents accumulation when changing classes (e.g., Fighter to Barbarian)
	d.clearChoicesBySource(shared.SourceClass)

	d.class = input.ClassID
	d.subclass = input.SubclassID

	// Get class requirements once for all choice recording
	requirements := choices.GetClassRequirementsWithSubclass(d.class, 1, d.subclass)

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

	// Record tool proficiency choices (for Monk, Bard, etc.)
	if len(input.Choices.Tools) > 0 {
		var choiceID choices.ChoiceID
		if requirements.Tools != nil {
			choiceID = requirements.Tools.ID
		}
		// Convert shared.SelectionID to proficiencies.Tool
		toolSelection := make([]proficiencies.Tool, len(input.Choices.Tools))
		for i, t := range input.Choices.Tools {
			toolSelection[i] = proficiencies.Tool(t)
		}
		d.recordChoice(choices.ChoiceData{
			Category:      shared.ChoiceToolProficiency,
			Source:        shared.SourceClass,
			ChoiceID:      choiceID,
			ToolSelection: toolSelection,
		})
	}

	// Record expertise choices (for Rogue L1/L6, Bard L3/L10). Validity
	// (every chosen skill actually being proficient) is checked once,
	// against the final combined race+class+background proficiency set,
	// at ToCharacter time (validateExpertiseSelections) — not here. A
	// point-in-time check at selection would depend on the order class,
	// race, and background were set in, and would never see a source set
	// afterward or changed later.
	if len(input.Choices.Expertise) > 0 {
		var choiceID choices.ChoiceID
		if requirements.Expertise != nil {
			choiceID = requirements.Expertise.ID
		}
		d.recordChoice(choices.ChoiceData{
			Category:           shared.ChoiceExpertise,
			Source:             shared.SourceClass,
			ChoiceID:           choiceID,
			ExpertiseSelection: input.Choices.Expertise,
		})
	}

	// Record equipment choices
	if err := d.recordEquipmentChoices(input.Choices.Equipment, requirements, shared.SourceClass); err != nil {
		return err
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

	if backgrounds.GetGrants(input.BackgroundID) == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "unknown background: %s", input.BackgroundID)
	}

	// Clear all existing background choices before recording new ones
	// This prevents accumulation when changing backgrounds
	d.clearChoicesBySource(shared.SourceBackground)

	d.background = input.BackgroundID
	requirements := choices.GetBackgroundRequirements(d.background)

	// Record language choices
	if len(input.Choices.Languages) > 0 {
		d.recordChoice(choices.ChoiceData{
			Category:          shared.ChoiceLanguages,
			Source:            shared.SourceBackground,
			LanguageSelection: input.Choices.Languages,
		})
	}

	// Record tool proficiency choices (Outlander, Noble/Knight,
	// Criminal/Spy, Soldier's proficiency choice)
	if len(input.Choices.Tools) > 0 {
		var choiceID choices.ChoiceID
		if requirements.Tools != nil {
			choiceID = requirements.Tools.ID
		}
		toolSelection := make([]proficiencies.Tool, len(input.Choices.Tools))
		for i, t := range input.Choices.Tools {
			toolSelection[i] = proficiencies.Tool(t)
		}
		d.recordChoice(choices.ChoiceData{
			Category:      shared.ChoiceToolProficiency,
			Source:        shared.SourceBackground,
			ChoiceID:      choiceID,
			ToolSelection: toolSelection,
		})
	}

	// Record equipment choices (Entertainer, Folk Hero, Guild
	// Artisan/Merchant, Soldier's item choice, Charlatan)
	if err := d.recordEquipmentChoices(input.Choices.Equipment, requirements, shared.SourceBackground); err != nil {
		return err
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
	if err := customization.ValidateAppearance(d.appearance); err != nil {
		return nil, err
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

	// Validate all required choices have been made
	if err := d.ValidateChoices(); err != nil {
		return nil, err
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

	// Fetched once and threaded through every compile* function that needs
	// it, same pattern raceData/classData already use.
	bgGrant := backgrounds.GetGrants(d.background)

	// Build proficiencies
	skillProfs := d.compileSkills(raceData, bgGrant)
	if err := validateExpertiseSelections(d.choices, skillProfs); err != nil {
		return nil, err
	}
	savingThrows := d.compileSavingThrows(classData)
	armorProfs, weaponProfs, toolProfs := d.compileProficiencies(bgGrant)

	// Compile features (can fail)
	charFeatures, err := d.compileFeatures(characterID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to compile features")
	}

	// The spells chosen at creation, as content refs on the sheet
	// (rpg-project#391 §5.2). Compiled BEFORE the character is built, because
	// a choice naming something this build cannot turn into a ref is a content
	// defect and should stop finalization rather than produce a sheet that is
	// quietly missing a spell.
	knownCantrips, err := compileKnownSpells(d.choices, shared.ChoiceCantrips, "cantrip")
	if err != nil {
		return nil, err
	}
	knownSpells, err := compileKnownSpells(d.choices, shared.ChoiceSpells, "spell")
	if err != nil {
		return nil, err
	}

	// Cleric domain grants are additional to chosen preparations and cantrips.
	if d.class == classes.Cleric {
		knownSpells, err = appendSpellGrants(knownSpells, choices.ClericSpellGrants(d.subclass, 1))
		if err != nil {
			return nil, err
		}
		if mods := choices.GetSubclassModifications(d.subclass); mods != nil {
			knownCantrips, err = appendSpellGrants(knownCantrips, mods.GrantedCantrips)
			if err != nil {
				return nil, err
			}
		}
	}

	// Create the character
	char := &Character{
		id:         characterID,
		playerID:   d.playerID,
		name:       d.name,
		appearance: customization.CloneAppearance(d.appearance),
		// Level 1 is a level. The record is complete from the first one
		// rather than backfilled from level 2 onward (design §2.4), and it
		// carries this draft's own choices verbatim — the inputs to level 1,
		// which is exactly what every later entry holds.
		levels: []LevelEntry{{
			Level:          1,
			ClassID:        d.class,
			HitPointGain:   maxHP,
			HitPointMethod: HitPointMethodMax,
			Choices:        slices.Clone(d.choices),
		}},
		raceID:              d.race,
		subraceID:           d.subrace,
		classID:             d.class,
		subclassID:          d.subclass,
		backgroundID:        d.background,
		createdAt:           d.createdAt,
		abilityScores:       finalScores,
		hitPoints:           maxHP,
		maxHitPoints:        maxHP,
		armorClass:          10 + finalScores.Modifier(abilities.DEX), // Base AC
		hitDice:             classData.HitDice,
		skills:              skillProfs,
		savingThrows:        savingThrows,
		armorProficiencies:  armorProfs,
		weaponProficiencies: weaponProfs,
		toolProficiencies:   toolProfs,
		languages:           d.compileLanguages(raceData),
		inventory:           d.compileInventory(bgGrant),
		wallet:              compileWallet(bgGrant),
		knownCantrips:       knownCantrips,
		knownSpells:         knownSpells,
		classResources:      make(map[shared.ClassResourceType]ResourceData),
		resources:           make(map[coreResources.ResourceKey]*combat.RecoverableResource),
		features:            charFeatures,
		combatAbilities:     make([]combatabilities.CombatAbility, 0),
		bus:                 bus,
		conditions:          make([]dnd5eEvents.ConditionBehavior, 0),
		subscriptionIDs:     make([]string, 0),
	}

	// Add standard combat abilities (Attack, Dash, Dodge, Disengage)
	d.initializeStandardCombatAbilities(char)

	// Initialize class-specific resources
	d.initializeClassResources(char)

	// Subscribe to events - character comes out fully initialized
	if err := char.subscribeToEvents(ctx); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to subscribe to events")
	}

	// Apply conditions from choices (e.g., fighting styles)
	initialConditions, err := d.compileConditions(characterID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to compile conditions")
	}

	// Check for Unarmored Defense condition and apply its AC calculation
	// Barbarian: AC = 10 + DEX + CON
	// Monk: AC = 10 + DEX + WIS
	for _, cond := range initialConditions {
		if ud, ok := cond.(*conditions.UnarmoredDefenseCondition); ok {
			char.armorClass = ud.CalculateAC(finalScores)
			break
		}
	}

	conditionTopic := dnd5eEvents.ConditionAppliedTopic.On(bus)
	for _, cond := range initialConditions {
		if err := conditionTopic.Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
			Target:    char,
			Type:      dnd5eEvents.ConditionFightingStyle,
			Source:    dnd5eEvents.ConditionSourceClass,
			Condition: cond,
		}); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to apply initial condition")
		}
	}

	return char, nil
}

// ValidateChoices validates that all required choices have been made.
// Complexity is inherent due to handling multiple choice categories.
//
//nolint:gocyclo // Complexity is inherent due to handling multiple choice categories
func (d *Draft) ValidateChoices() error {
	// Create validator
	validator := choices.NewValidator()

	requirements := choices.GetClassRequirementsWithSubclass(d.class, 1, d.subclass)
	backgroundRequirements := choices.GetBackgroundRequirements(d.background)

	// Equipment is checked before anything is translated, because these are
	// checks on what was PERSISTED rather than on whether it satisfies a
	// requirement: an equipment id this build cannot resolve is a broken
	// record, not a wrong answer.
	for _, choice := range d.choices {
		if choice.Category != shared.ChoiceEquipment || len(choice.EquipmentSelection) == 0 {
			continue
		}
		choiceRequirements := requirements
		if choice.Source == shared.SourceBackground {
			choiceRequirements = backgroundRequirements
		}
		if err := d.validatePersistedCategoryEquipmentChoice(choice, choiceRequirements); err != nil {
			return err
		}
		for _, equipID := range choice.EquipmentSelection {
			if _, err := equipment.GetByID(equipID); err != nil {
				return rpgerr.Newf(rpgerr.CodeNotFound,
					"invalid equipment ID in stored choices: %s", equipID)
			}
		}
	}

	// The same translation advancement uses (design R4.4b). Cantrips and spells
	// are RECORDED by SetClass and were once not converted here at all, so every
	// spellcaster failed its own requirement no matter what the player chose;
	// one shared builder is what keeps the recorder and the validator reading
	// the same choice.
	submissions := choices.SubmissionsFrom(d.choices)
	d.addSubclassSubmission(submissions)

	// Validate choices
	result := &choices.ValidationResult{Valid: true}
	for _, req := range []*choices.Requirements{requirements, choices.GetRaceRequirements(d.race), backgroundRequirements} {
		if req == nil {
			continue
		}
		checked := validator.Validate(req, submissions)
		result.Valid = result.Valid && checked.Valid
		result.Errors = append(result.Errors, checked.Errors...)
	}

	if !result.Valid {
		// Return first error as rpgerr
		if len(result.Errors) > 0 {
			err := result.Errors[0]
			return rpgerr.New(rpgerr.CodeInvalidArgument, err.Message,
				rpgerr.WithMeta("category", string(err.Category)),
				rpgerr.WithMeta("source", string(err.Source)))
		}
	}

	if err := d.validateLifeEquipment(); err != nil {
		return err
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

// validatePersistedCategoryEquipmentChoice verifies the nested selections in a
// serialized bundle choice before that choice is reduced to its option ID for
// the general requirements validator.
func (d *Draft) validatePersistedCategoryEquipmentChoice(
	choice choices.ChoiceData,
	requirements *choices.Requirements,
) error {
	if choice.OptionID == "" {
		return nil
	}

	requirement := d.findEquipmentRequirement(choice.ChoiceID, requirements)
	if requirement == nil {
		return nil
	}

	option := d.findEquipmentOption(choice.OptionID, requirement)
	if option == nil || len(option.CategoryChoices) == 0 {
		return nil
	}

	categorySelectionCount := 0
	for _, categoryChoice := range option.CategoryChoices {
		categorySelectionCount += categoryChoice.Choose
	}

	expectedSelectionCount := len(option.Items) + categorySelectionCount
	if len(choice.EquipmentSelection) != expectedSelectionCount {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"invalid persisted equipment selection for choice '%s' option '%s': expected %d items, got %d",
			choice.ChoiceID, choice.OptionID, expectedSelectionCount, len(choice.EquipmentSelection))
	}

	selectionOffset := len(option.Items)
	for _, categoryChoice := range option.CategoryChoices {
		eligibleEquipment, err := choices.EligibleEquipment(categoryChoice.Type, categoryChoice.Categories)
		if err != nil {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"failed to resolve persisted equipment categories for choice '%s' option '%s': %v",
				choice.ChoiceID, choice.OptionID, err)
		}

		eligibleIDs := make(map[shared.SelectionID]struct{}, len(eligibleEquipment))
		for _, eligible := range eligibleEquipment {
			eligibleIDs[eligible.EquipmentID()] = struct{}{}
		}

		selected := choice.EquipmentSelection[selectionOffset : selectionOffset+categoryChoice.Choose]
		for _, equipmentID := range selected {
			if _, eligible := eligibleIDs[equipmentID]; !eligible {
				return rpgerr.Newf(rpgerr.CodeInvalidArgument,
					"invalid persisted category equipment selection for choice '%s' option '%s': '%s' is not eligible",
					choice.ChoiceID, choice.OptionID, equipmentID)
			}
		}
		selectionOffset += categoryChoice.Choose
	}

	return nil
}

// clearChoicesBySource removes all choices from the specified source.
// This is used when changing class, race, or background to ensure old choices
// don't accumulate. For example, when changing from Fighter to Barbarian,
// this clears all Fighter equipment choices before recording Barbarian choices.
func (d *Draft) clearChoicesBySource(source shared.ChoiceSource) {
	filtered := make([]choices.ChoiceData, 0, len(d.choices))
	for _, c := range d.choices {
		// Keep choices that are NOT from the specified source
		if c.Source != source {
			filtered = append(filtered, c)
		}
	}
	d.choices = filtered
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
func (d *Draft) compileSkills(raceData *races.Data, bgGrant *backgrounds.Grant) map[skills.Skill]shared.ProficiencyLevel {
	skillMap := make(map[skills.Skill]shared.ProficiencyLevel)

	// Add racial skill proficiencies
	for _, skill := range raceData.Skills {
		skillMap[skill] = shared.Proficient
	}

	// Add chosen skills from choices
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceSkills {
			for _, skill := range choice.SkillSelection {
				skillMap[skill] = shared.Proficient
			}
		}
	}

	// Add background skill proficiencies. Must happen before the expertise
	// upgrade pass below — a background-granted skill needs to already be
	// in skillMap for an expertise pick naming it to actually upgrade to
	// Expert, not just pass validation (rpg-toolkit#1554).
	if bgGrant != nil {
		for _, skill := range bgGrant.SkillProficiencies {
			skillMap[skill] = shared.Proficient
		}
	}

	// Apply expertise - upgrade proficient skills to expert
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceExpertise {
			for _, skill := range choice.ExpertiseSelection {
				// Only upgrade if already proficient (validation should catch this earlier)
				if _, hasProficiency := skillMap[skill]; hasProficiency {
					skillMap[skill] = shared.Expert
				}
			}
		}
	}

	return skillMap
}

// validateExpertiseSelections confirms every recorded expertise choice
// names a skill the final compiled proficiency set actually contains.
// Checked here, against the fully-compiled skill map (race + class +
// background), rather than at selection time against whatever state
// existed when the expertise choice was submitted — order of selection no
// longer matters, and a background/race set or changed afterward is
// still checked correctly.
func validateExpertiseSelections(
	recorded []choices.ChoiceData,
	skillProfs map[skills.Skill]shared.ProficiencyLevel,
) error {
	for _, choice := range recorded {
		if choice.Category != shared.ChoiceExpertise {
			continue
		}
		for _, skill := range choice.ExpertiseSelection {
			if _, proficient := skillProfs[skill]; !proficient {
				return rpgerr.Newf(rpgerr.CodeInvalidArgument,
					"expertise skill %s must be from a proficient skill (class, race, or background)", skill)
			}
		}
	}
	return nil
}

// compileSavingThrows builds the saving throw proficiency map
func (d *Draft) compileSavingThrows(classData *classes.Data) map[abilities.Ability]shared.ProficiencyLevel {
	saves := make(map[abilities.Ability]shared.ProficiencyLevel)

	for _, ability := range classData.SavingThrows {
		saves[ability] = shared.Proficient
	}

	return saves
}

// compileProficiencies collects armor, weapon, and tool proficiencies from class, race, and background grants
func (d *Draft) compileProficiencies(
	bgGrant *backgrounds.Grant,
) ([]proficiencies.Armor, []proficiencies.Weapon, []proficiencies.Tool) {
	armorProfs := make([]proficiencies.Armor, 0)
	weaponProfs := make([]proficiencies.Weapon, 0)
	toolProfs := make([]proficiencies.Tool, 0)

	// Collect from class grants
	if d.class != "" {
		grants := classes.GetGrantsForLevel(d.class, 1)
		for _, grant := range grants {
			armorProfs = append(armorProfs, grant.ArmorProficiencies...)
			weaponProfs = append(weaponProfs, grant.WeaponProficiencies...)
			toolProfs = append(toolProfs, grant.ToolProficiencies...)
		}
	}

	// Apply level-1 heavy-armor grants from cleric domains. Domain proficiencies are additive to the cleric base proficiencies.
	if d.class == classes.Cleric && (d.subclass == classes.LifeDomain || d.subclass == classes.TempestDomain || d.subclass == classes.WarDomain) {
		for _, category := range choices.GetSubclassModifications(d.subclass).GrantedProficiencies.Armor {
			armorProfs = append(armorProfs, proficiencies.Armor(category))
		}
	}

	// War grants all martial weapons. The runtime proficiency is "martial",
	// not the picker categories "martial-melee" and "martial-ranged".
	if d.class == classes.Cleric && (d.subclass == classes.WarDomain || d.subclass == classes.TempestDomain) {
		weaponProfs = append(weaponProfs, proficiencies.WeaponMartial)
	}

	// Collect from race grants
	if d.race != "" {
		if grant := races.GetGrants(d.race); grant != nil {
			armorProfs = append(armorProfs, grant.ArmorProficiencies...)
			weaponProfs = append(weaponProfs, grant.WeaponProficiencies...)
			toolProfs = append(toolProfs, grant.ToolProficiencies...)
		}
	}

	// Collect from background grants. Backgrounds never grant armor/weapon
	// proficiencies in RAW, only tools.
	if bgGrant != nil {
		toolProfs = append(toolProfs, bgGrant.ToolProficiencies...)
	}

	// Chosen tool proficiencies (Monk's tools-or-instrument, Dwarf's
	// artisan's tools, and any future source) are recorded as choices
	// during SetClass/SetRace/SetBackground and validated by
	// choices.Validator.validateTools, but only reach the compiled
	// character here — this loop was previously missing entirely, so a
	// validly-chosen tool proficiency never appeared on the finished
	// character (rpg-toolkit#1555).
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceToolProficiency {
			toolProfs = append(toolProfs, choice.ToolSelection...)
		}
	}

	// Entertainer/Folk Hero/Guild Artisan's tool proficiency is derived
	// from their equipment choice rather than asked as a second choice
	// (see choices.GetBackgroundRequirements' doc comment for why) — the
	// selected instrument/tool ID and its matching proficiency constant
	// are the identical string.
	for _, choice := range d.choices {
		if choice.Category != shared.ChoiceEquipment || choice.Source != shared.SourceBackground {
			continue
		}
		switch choice.ChoiceID {
		case choices.EntertainerInstrument, choices.FolkHeroArtisanTools, choices.GuildArtisanTools:
			for _, id := range choice.EquipmentSelection {
				toolProfs = append(toolProfs, proficiencies.Tool(id))
			}
		}
	}

	return armorProfs, weaponProfs, dedupeToolProficiencies(toolProfs)
}

// dedupeToolProficiencies removes duplicate entries from a compiled tool
// proficiency list. Two sources can legitimately grant the same tool
// proficiency (e.g. a Rogue class grant and a Criminal background grant
// both naming thieves' tools) — that's allowed, not an error, but the
// compiled list shouldn't show the same proficiency twice.
func dedupeToolProficiencies(toolProfs []proficiencies.Tool) []proficiencies.Tool {
	seen := make(map[proficiencies.Tool]bool, len(toolProfs))
	deduped := make([]proficiencies.Tool, 0, len(toolProfs))
	for _, tool := range toolProfs {
		if !seen[tool] {
			seen[tool] = true
			deduped = append(deduped, tool)
		}
	}
	return deduped
}

// compileWallet returns a character's starting gold, granted by
// background. Every finalized character previously had a zero Wallet
// regardless of background, since nothing ever set this field.
func compileWallet(bgGrant *backgrounds.Grant) currency.Money {
	if bgGrant == nil {
		return currency.Money{}
	}
	return bgGrant.StartingGold
}

// compileKnownSpells turns one category of recorded spell choices into content
// refs for the sheet.
//
// # Through the catalog, never composed
//
// The id a choice carries is the ref's id, so building "dnd5e:spells:<id>"
// out of it would always succeed — which is the problem. A typo, a renamed
// constant, or a spell this build has no content for would become a ref
// pointing at nothing, persisted, and read back later by whatever mints Cast
// declarations. So this ASKS the ref catalog and refuses what it does not
// know. The validator has already gated the id against the class's option
// list; this is the second half of the same question, and the one that can
// answer "this build has no such spell".
// It takes the choices rather than reading a draft's, because a level-up makes
// the same kind of choice and must reach the sheet the same way (design R4.4c:
// "Advance MUST apply a level's choices through the same compilers creation
// uses, by category"). A second compiler would be a second chance to disagree
// about what a chosen spell becomes.
func compileKnownSpells(
	recorded []choices.ChoiceData, category shared.ChoiceCategory, role string,
) ([]*core.Ref, error) {
	var known []*core.Ref
	for _, choice := range recorded {
		if choice.Category != category {
			continue
		}
		for _, selection := range choice.SpellSelection {
			ref := refs.Spells.ByID(string(selection))
			if ref == nil {
				return nil, rpgerr.Newf(rpgerr.CodeNotFound,
					"chosen %s %q is not a spell this build knows", role, selection)
			}
			// Cloned: the catalog hands back shared singletons, and a sheet
			// that aliased one would let a caller reading its known spells
			// rewrite the catalog for everybody.
			clone := *ref
			known = append(known, &clone)
		}
	}
	return known, nil
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
func (d *Draft) compileInventory(bgGrant *backgrounds.Grant) []InventoryItem {
	inventory := make([]InventoryItem, 0)

	// Add starting equipment from class grants (new Grant system)
	if d.class != "" {
		grants := classes.GetGrantsForLevel(d.class, 1)
		for _, grant := range grants {
			inventory = append(inventory, d.inventoryItemsFromGrant(grant)...)
		}
	}

	// Add starting equipment from background grants. Same resolution as
	// class equipment (materializeItem), so a background-granted pack
	// would decompose into its Contents the same way (rpg-toolkit#1544) --
	// none currently do.
	if bgGrant != nil {
		for _, item := range bgGrant.Equipment {
			equip, err := equipment.GetByID(item.ID)
			if err != nil {
				panic(fmt.Sprintf("BUG: Invalid equipment ID in background grants for %s: %s - %v",
					d.background, item.ID, err))
			}
			inventory = append(inventory, materializeItem(equip, item.Quantity)...)
		}
	}

	// Add equipment from choices (user selections)
	reqs := choices.GetClassRequirementsWithSubclass(d.class, 1, d.subclass)
	bgReqs := choices.GetBackgroundRequirements(d.background)
	for _, choice := range d.choices {
		if choice.Category != shared.ChoiceEquipment {
			continue
		}

		req := d.equipmentRequirementFor(choice.ChoiceID, reqs)
		if req == nil {
			// Choice IDs are unique across sources, so trying background
			// requirements when the class lookup misses is safe.
			req = d.equipmentRequirementFor(choice.ChoiceID, bgReqs)
		}
		if req == nil {
			continue
		}

		option := d.findEquipmentOption(choice.OptionID, req)
		if option == nil {
			panic(fmt.Sprintf("BUG: Invalid equipment option %s for choice %s", choice.OptionID, choice.ChoiceID))
		}

		inventory = append(inventory, d.materializeEquipmentOption(option, choice.EquipmentSelection)...)
	}

	return stackInventory(inventory)
}

func stackInventory(items []InventoryItem) []InventoryItem {
	out := make([]InventoryItem, 0, len(items))
	index := make(map[shared.EquipmentID]int, len(items))
	for _, item := range items {
		id := item.Equipment.EquipmentID()
		if item.Quantity <= 0 {
			panic(fmt.Sprintf("BUG: nonpositive quantity %d for %s", item.Quantity, id))
		}
		if at, ok := index[id]; ok {
			out[at].Quantity += item.Quantity
			continue
		}
		index[id] = len(out)
		out = append(out, item)
	}
	return out
}

func (d *Draft) inventoryItemsFromGrant(grant classes.Grant) []InventoryItem {
	items := make([]InventoryItem, 0, len(grant.Equipment))
	for _, item := range grant.Equipment {
		equip, err := equipment.GetByID(item.ID)
		if err != nil {
			panic(fmt.Sprintf("BUG: Invalid equipment ID in class grants for %s: %s - %v", d.class, item.ID, err))
		}
		items = append(items, materializeItem(equip, item.Quantity)...)
	}
	return items
}

// materializeItem returns the inventory lines one resolved equipment
// grant or choice actually produces. A pack decomposes into its own
// Contents (equipment.ResolvePackContents), scaled by quantity — the same
// resolution session.Unpack uses, so a starting pack and a bought-then-
// unpacked one produce identical inventory shapes (rpg-toolkit#1544).
// Every other equipment type materializes as itself, unchanged.
//
// Panics on an unresolvable pack content, matching every other resolve
// failure in this file's own convention: a class/background grant is
// supposed to only ever name real IDs, and equipment/pack_contents_test.go
// already proves every pack in the current catalog resolves cleanly — this
// is the same "can't happen, and tested that it can't" shape as the
// panics beside it, not a new risk introduced by decomposing packs here.
func materializeItem(equip equipment.Equipment, quantity int) []InventoryItem {
	if equip.EquipmentType() != shared.EquipmentTypePack {
		return []InventoryItem{{Equipment: equip, Quantity: quantity}}
	}

	contents, _, err := equipment.ResolvePackContents(shared.EquipmentID(equip.EquipmentID()))
	if err != nil {
		panic(fmt.Sprintf("BUG: pack %q contents do not resolve against the catalog: %v", equip.EquipmentID(), err))
	}

	items := make([]InventoryItem, 0, len(contents))
	for _, content := range contents {
		contentEquip, err := equipment.GetByID(shared.SelectionID(content.ID))
		if err != nil {
			panic(fmt.Sprintf("BUG: pack %q content %q does not resolve via GetByID: %v", equip.EquipmentID(), content.ID, err))
		}
		items = append(items, InventoryItem{Equipment: contentEquip, Quantity: content.Quantity * quantity})
	}
	return items
}

func (d *Draft) equipmentRequirementFor(
	choiceID choices.ChoiceID,
	reqs *choices.Requirements,
) *choices.EquipmentRequirement {
	if reqs == nil {
		return nil
	}

	return d.findEquipmentRequirement(choiceID, reqs)
}

func (d *Draft) materializeEquipmentOption(
	option *choices.EquipmentOption,
	selected []shared.SelectionID,
) []InventoryItem {
	items := make([]InventoryItem, 0, len(option.Items)+len(selected))

	fixedCount := len(option.Items)
	if len(selected) < fixedCount {
		panic(fmt.Sprintf("BUG: equipment selection for %s lost fixed items", option.ID))
	}

	for _, item := range option.Items {
		equip, err := equipment.GetByID(item.ID)
		if err != nil {
			panic(fmt.Sprintf("BUG: Invalid equipment ID '%s' in class requirements", item.ID))
		}
		items = append(items, materializeItem(equip, item.Quantity)...)
	}

	for _, equipID := range selected[fixedCount:] {
		equip, err := equipment.GetByID(equipID)
		if err != nil {
			panic(fmt.Sprintf("BUG: Invalid equipment ID in draft choices: %s - %v", equipID, err))
		}
		items = append(items, materializeItem(equip, 1)...)
	}

	return items
}

// compileFeatures returns the character's class features using the unified grant system.
// Features are created from FeatureRef grants defined in classes/grant.go.
func (d *Draft) compileFeatures(characterID string) ([]features.Feature, error) {
	featureList := make([]features.Feature, 0)

	// Get grants for the class at level 1 (character creation)
	grants := classes.GetGrantsForLevel(d.class, 1)
	if grants == nil {
		return featureList, nil
	}

	// Create features from each grant's FeatureRefs
	for _, grant := range grants {
		for _, featureRef := range grant.Features {
			output, err := features.CreateFromRef(&features.CreateFromRefInput{
				Ref:         featureRef.Ref,
				Config:      featureRef.Config,
				CharacterID: characterID,
			})
			if err != nil {
				return nil, rpgerr.Wrapf(err, "failed to create feature from ref %s", featureRef.Ref)
			}
			featureList = append(featureList, output.Feature)
		}
	}

	if d.class == classes.Cleric && d.subclass == classes.TempestDomain {
		uses := d.baseAbilityScores.Modifier(abilities.WIS)
		if uses < 1 {
			uses = 1
		}
		cfg, _ := json.Marshal(map[string]int{"uses": uses})
		output, err := features.CreateFromRef(&features.CreateFromRefInput{Ref: refs.Features.WrathOfTheStorm().String(), Config: cfg, CharacterID: characterID})
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to create Wrath of the Storm")
		}
		featureList = append(featureList, output.Feature)
	}

	return featureList, nil
}

// compileConditions creates conditions from grants and draft choices (e.g., fighting styles).
// Conditions can come from two sources:
// 1. Class grants (e.g., Barbarian's Unarmored Defense)
// 2. Player choices (e.g., Fighter's chosen Fighting Style)
func (d *Draft) compileConditions(characterID string) ([]dnd5eEvents.ConditionBehavior, error) {
	conditionList := make([]dnd5eEvents.ConditionBehavior, 0)

	// Get conditions from class grants
	grants := classes.GetGrantsForLevel(d.class, 1)
	classSourceRef := "dnd5e:classes:" + d.class
	for _, grant := range grants {
		for _, condRef := range grant.Conditions {
			output, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
				Ref:       condRef.Ref,
				Config:    condRef.Config,
				MemberID:  characterID,
				SourceRef: classSourceRef,
			})
			if err != nil {
				return nil, rpgerr.Wrapf(err, "failed to create condition from ref %s", condRef.Ref)
			}
			conditionList = append(conditionList, output.Condition)
		}
	}

	// Add conditions from player choices (e.g., fighting styles)
	// Fighting styles are CHOICES, not grants, so they're handled separately
	// Each fighting style maps to its corresponding condition
	if style := d.GetFightingStyleSelection(); style != nil {
		fsCondition, err := createFightingStyleCondition(*style, characterID)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to create fighting style condition")
		}
		conditionList = append(conditionList, fsCondition)
	}

	return conditionList, nil
}

// createFightingStyleCondition creates the appropriate condition for a fighting style.
// Each fighting style maps to its own dedicated condition type.
func createFightingStyleCondition(
	style fightingstyles.FightingStyle, characterID string,
) (dnd5eEvents.ConditionBehavior, error) {
	switch style {
	case fightingstyles.Archery:
		return conditions.NewFightingStyleArcheryCondition(characterID), nil
	case fightingstyles.Defense:
		return conditions.NewFightingStyleDefenseCondition(characterID), nil
	case fightingstyles.Dueling:
		return conditions.NewFightingStyleDuelingCondition(characterID), nil
	case fightingstyles.GreatWeaponFighting:
		return conditions.NewFightingStyleGreatWeaponFightingCondition(characterID, nil), nil
	case fightingstyles.Protection:
		return conditions.NewFightingStyleProtectionCondition(characterID), nil
	case fightingstyles.TwoWeaponFighting:
		return conditions.NewFightingStyleTwoWeaponFightingCondition(characterID), nil
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown fighting style: %s", style)
	}
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
	reqs := choices.GetClassRequirementsWithSubclass(d.class, 1, d.subclass)
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

	return result.Valid && d.validateLifeEquipment() == nil
}

// IsBackgroundComplete checks if background selection and choices are complete
func (d *Draft) IsBackgroundComplete() bool {
	if d.background == "" {
		return false
	}

	// Get background requirements
	reqs := choices.GetBackgroundRequirements(d.background)
	if reqs == nil {
		return true // No choices required
	}

	// Create submissions from draft choices
	subs := d.getBackgroundSubmissions()

	// Validate
	validator := choices.NewValidator()
	result := validator.Validate(reqs, subs)

	return result.Valid
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

			// Handle tool proficiency choices (for Dwarf, etc.)
			if len(choice.ToolSelection) > 0 {
				// Convert proficiencies.Tool to shared.SelectionID
				toolValues := make([]shared.SelectionID, 0, len(choice.ToolSelection))
				for _, t := range choice.ToolSelection {
					toolValues = append(toolValues, shared.SelectionID(t))
				}
				var choiceID choices.ChoiceID
				switch d.race {
				case races.Dwarf:
					choiceID = choices.DwarfToolProficiency
				default:
					choiceID = choices.ChoiceID(string(d.race) + "-tool-proficiency")
				}
				subs.Add(choices.Submission{
					Category: shared.ChoiceToolProficiency,
					Source:   shared.SourceRace,
					ChoiceID: choiceID,
					Values:   toolValues,
				})
			}
		}
	}

	return subs
}

// getBackgroundSubmissions extracts background-related submissions from
// draft choices. Unlike getRaceSubmissions, this uses choice.ChoiceID
// directly rather than rebuilding it from a switch on d.background —
// SetBackground always stores the correct ChoiceID from
// GetBackgroundRequirements at recording time, same as getClassSubmissions
// already relies on SetClass doing.
func (d *Draft) getBackgroundSubmissions() *choices.Submissions {
	subs := choices.NewSubmissions()

	for _, choice := range d.choices {
		if choice.Source != shared.SourceBackground {
			continue
		}

		if len(choice.EquipmentSelection) > 0 {
			values := choice.EquipmentSelection
			if choice.OptionID != "" {
				values = []shared.SelectionID{choice.OptionID}
			}
			subs.Add(choices.Submission{
				Category: shared.ChoiceEquipment,
				Source:   shared.SourceBackground,
				ChoiceID: choice.ChoiceID,
				OptionID: choice.OptionID,
				Values:   values,
			})
		}

		if len(choice.ToolSelection) > 0 {
			toolValues := make([]shared.SelectionID, len(choice.ToolSelection))
			for i, t := range choice.ToolSelection {
				toolValues[i] = shared.SelectionID(t)
			}
			subs.Add(choices.Submission{
				Category: shared.ChoiceToolProficiency,
				Source:   shared.SourceBackground,
				ChoiceID: choice.ChoiceID,
				Values:   toolValues,
			})
		}
	}

	return subs
}

// getClassSubmissions is what completeness reads: this draft's class choices,
// as the submissions the validator matches against the class's requirements.
//
// It TRANSLATES NOTHING ITSELF. It used to carry its own copy of the
// ChoiceData-to-Submission switch, and a second copy of that switch is a second
// opinion about what a choice means — which has now cost two bugs of the same
// shape, each invisible because the other builder agreed with the client:
//
//   - a bard's spells and cantrips were not converted here at all, so a bard
//     passed [Draft.ValidateChoices] and was still 80% complete, and
//     FinalizeDraft refused a draft that had answered every question;
//   - a fighting style was submitted under the constant
//     choices.FighterFightingStyle for EVERY class, so a ranger — the only
//     other class with a level-1 fighting style — could not be created by any
//     client with any choices. Its requirement is "ranger-fighting-style", the
//     submission claimed "fighter-fighting-style", and the answered
//     requirement was never seen.
//
// Both were "a class requirement this builder cannot see is a requirement
// nothing can ever satisfy". One builder is the fix for the class of bug; the
// id belongs to the requirement, and the choice carries the id it was recorded
// with, so no per-class mapping exists to get wrong.
func (d *Draft) getClassSubmissions() *choices.Submissions {
	classChoices := make([]choices.ChoiceData, 0, len(d.choices))
	for _, choice := range d.choices {
		if choice.Source != shared.SourceClass {
			continue
		}
		classChoices = append(classChoices, d.withRecordedChoiceID(choice))
	}

	subs := choices.SubmissionsFrom(classChoices)
	d.addSubclassSubmission(subs)

	return subs
}

// withRecordedChoiceID fills in the requirement id of a stored choice that has
// none.
//
// [Draft.SetClass] records every class choice with the id of the requirement it
// answers, so a draft written by this build always carries one. A draft
// persisted before the fighting style carried its own id does not, and the
// class's row is where that id lives — one lookup, not a per-class map.
func (d *Draft) withRecordedChoiceID(choice choices.ChoiceData) choices.ChoiceData {
	if choice.ChoiceID != "" || choice.Category != shared.ChoiceFightingStyle {
		return choice
	}

	if reqs := choices.GetClassRequirements(d.class); reqs != nil && reqs.FightingStyle != nil {
		choice.ChoiceID = reqs.FightingStyle.ID
	}
	return choice
}

// addSubclassSubmission projects the draft's single subclass field into the
// validator's choice representation without persisting a second selection.
func (d *Draft) addSubclassSubmission(subs *choices.Submissions) {
	classData := classes.GetData(d.class)
	if classData == nil || classData.SubclassLevel != 1 || d.subclass == "" {
		return
	}
	subs.Add(choices.Submission{
		Category: shared.ChoiceClass, Source: shared.SourceClass,
		ChoiceID: choices.ChoiceID(classData.SubclassChoiceID),
		Values:   []shared.SelectionID{d.subclass},
	})
}

// recordEquipmentChoices processes and records equipment selections
func (d *Draft) recordEquipmentChoices(
	selections []EquipmentChoiceSelection,
	requirements *choices.Requirements,
	source shared.ChoiceSource,
) error {
	for _, selection := range selections {
		if err := d.recordEquipmentChoice(selection, requirements, source); err != nil {
			return err
		}
	}
	return nil
}

// recordEquipmentChoice processes a single equipment selection
func (d *Draft) recordEquipmentChoice(
	selection EquipmentChoiceSelection,
	requirements *choices.Requirements,
	source shared.ChoiceSource,
) error {
	// Try to find as a bundle requirement
	if req := d.findEquipmentRequirement(selection.ChoiceID, requirements); req != nil {
		return d.recordBundleEquipment(selection, req, source)
	}

	// Try to find as a category requirement
	if catReq := d.findCategoryRequirement(selection.ChoiceID, requirements); catReq != nil {
		return d.recordCategoryEquipment(selection, source)
	}

	return rpgerr.Newf(rpgerr.CodeNotFound, "unknown equipment choice '%s'", selection.ChoiceID)
}

// findEquipmentRequirement finds a bundle equipment requirement by ID
func (d *Draft) findEquipmentRequirement(
	choiceID choices.ChoiceID,
	requirements *choices.Requirements,
) *choices.EquipmentRequirement {
	for _, req := range requirements.Equipment {
		if req.ID == choiceID {
			return req
		}
	}
	return nil
}

// findCategoryRequirement finds a category requirement by ID
func (d *Draft) findCategoryRequirement(
	choiceID choices.ChoiceID,
	requirements *choices.Requirements,
) *choices.EquipmentCategoryRequirement {
	for _, catReq := range requirements.EquipmentCategories {
		if catReq.ID == choiceID {
			return catReq
		}
	}
	return nil
}

// recordBundleEquipment processes equipment from a bundle requirement
func (d *Draft) recordBundleEquipment(
	selection EquipmentChoiceSelection,
	req *choices.EquipmentRequirement,
	source shared.ChoiceSource,
) error {
	// Find the selected option
	selectedOption := d.findEquipmentOption(selection.OptionID, req)
	if selectedOption == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound,
			"unknown equipment option '%s' for choice '%s'",
			selection.OptionID, selection.ChoiceID)
	}

	// Build equipment list from fixed items and category selections
	equipmentIDs, err := d.buildEquipmentList(selection, selectedOption)
	if err != nil {
		return err
	}

	d.recordChoice(choices.ChoiceData{
		Category:           shared.ChoiceEquipment,
		Source:             source,
		ChoiceID:           selection.ChoiceID,
		OptionID:           selectedOption.ID,
		EquipmentSelection: equipmentIDs,
	})

	return nil
}

// findEquipmentOption finds an option within a requirement
func (d *Draft) findEquipmentOption(
	optionID shared.SelectionID,
	req *choices.EquipmentRequirement,
) *choices.EquipmentOption {
	for _, opt := range req.Options {
		if opt.ID == optionID {
			return &opt
		}
	}
	return nil
}

// buildEquipmentList builds the final equipment list from fixed items and category selections
func (d *Draft) buildEquipmentList(
	selection EquipmentChoiceSelection,
	option *choices.EquipmentOption,
) ([]shared.SelectionID, error) {
	equipmentIDs := make([]shared.SelectionID, 0)

	// Add fixed items
	for _, item := range option.Items {
		if _, err := equipment.GetByID(item.ID); err != nil {
			return nil, rpgerr.Newf(rpgerr.CodeNotFound,
				"invalid equipment ID '%s' in class requirements", item.ID)
		}
		equipmentIDs = append(equipmentIDs, item.ID)
	}

	// Add category selections if option has category choices
	if len(option.CategoryChoices) > 0 {
		categoryIDs, err := d.validateCategorySelections(selection, option)
		if err != nil {
			return nil, err
		}
		equipmentIDs = append(equipmentIDs, categoryIDs...)
	}

	return equipmentIDs, nil
}

// validateCategorySelections validates category selections and returns the equipment IDs
func (d *Draft) validateCategorySelections(
	selection EquipmentChoiceSelection,
	option *choices.EquipmentOption,
) ([]shared.SelectionID, error) {
	if len(selection.CategorySelections) == 0 {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"option '%s' requires category selections", selection.OptionID)
	}

	// Validate count matches
	totalRequired := 0
	for _, catChoice := range option.CategoryChoices {
		totalRequired += catChoice.Choose
	}
	if len(selection.CategorySelections) != totalRequired {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"option '%s' requires %d category selections, got %d",
			selection.OptionID, totalRequired, len(selection.CategorySelections))
	}

	selectionOffset := 0
	for _, categoryChoice := range option.CategoryChoices {
		validEquipment, err := choices.EligibleEquipment(categoryChoice.Type, categoryChoice.Categories)
		if err != nil {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"failed to resolve equipment categories for option '%s': %v", selection.OptionID, err)
		}

		validIDs := make(map[shared.EquipmentID]struct{}, len(validEquipment))
		for _, eligible := range validEquipment {
			validIDs[eligible.EquipmentID()] = struct{}{}
		}

		selected := selection.CategorySelections[selectionOffset : selectionOffset+categoryChoice.Choose]
		for _, equipID := range selected {
			if _, err := equipment.GetByID(equipID); err != nil {
				return nil, rpgerr.Newf(rpgerr.CodeNotFound, "invalid equipment ID '%s'", equipID)
			}
			if _, valid := validIDs[equipID]; !valid {
				return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
					"Invalid equipment choice '%s' - must be from specified categories", equipID)
			}
		}
		selectionOffset += categoryChoice.Choose
	}

	return selection.CategorySelections, nil
}

// recordCategoryEquipment processes a top-level category equipment choice
func (d *Draft) recordCategoryEquipment(selection EquipmentChoiceSelection, source shared.ChoiceSource) error {
	if len(selection.CategorySelections) == 0 {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"category choice '%s' requires category selections", selection.ChoiceID)
	}

	// Validate equipment IDs
	for _, equipID := range selection.CategorySelections {
		if _, err := equipment.GetByID(equipID); err != nil {
			return rpgerr.Newf(rpgerr.CodeNotFound, "invalid equipment ID '%s'", equipID)
		}
	}

	d.recordChoice(choices.ChoiceData{
		Category:           shared.ChoiceEquipment,
		Source:             source,
		ChoiceID:           selection.ChoiceID,
		EquipmentSelection: selection.CategorySelections,
	})

	return nil
}

// calculateBarbarianRageUses determines max rage uses based on barbarian level.
// Duplicated here intentionally - character creation owns resource initialization,
// features package owns resource consumption. Each proves the values independently.
func calculateBarbarianRageUses(level int) int {
	switch {
	case level < 3:
		return 2
	case level < 6:
		return 3
	case level < 12:
		return 4
	case level < 17:
		return 5
	case level < 20:
		return 6
	default:
		return -1 // Unlimited at level 20
	}
}

// initializeClassResources adds class-specific resources to the character.
// Called during ToCharacter after the character struct is created.
func (d *Draft) initializeClassResources(char *Character) {
	for key, resource := range buildClassResources(char, d.class, char.ClassLevel(d.class), char.GetLevel()) {
		char.resources[key] = resource
	}
}

// buildClassResources returns the class-granted pools a character of this
// class level has, each at full.
//
// classLevel sizes what the CLASS grants — rage charges, Ki, the starting
// spell slot — because a feature arrives at its own class's level (R4.6).
// characterLevel sizes hit dice, which count every level whatever class took
// it. Today R2.4 forces the two equal; they are passed separately so the
// arithmetic stays true when it stops being.
//
// Creation assigns these as they come. [Character.Advance] keeps what is
// already spent and grants only the difference; see resizeClassResources.
func buildClassResources(
	char *Character, class classes.Class, classLevel, characterLevel int,
) map[coreResources.ResourceKey]*combat.RecoverableResource {
	built := make(map[coreResources.ResourceKey]*combat.RecoverableResource)
	level := classLevel

	switch class {
	case classes.Barbarian:
		// Rage charges - recovered on long rest
		maxRages := calculateBarbarianRageUses(level)
		if maxRages > 0 {
			// Level 20 barbarians return -1 (unlimited rages) and don't need a resource.
			// The Rage feature's CanActivate checks level >= 20 and bypasses resource check.
			rageResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
				ID:          string(resources.RageCharges),
				Maximum:     maxRages,
				CharacterID: char.id,
				ResetType:   coreResources.ResetLongRest,
			})
			built[resources.RageCharges] = rageResource
		}

	case classes.Bard:
		// Bardic Inspiration uses - Charisma modifier, minimum one, recovered
		// on long rest. The minimum is RAW and is what keeps a bard with a
		// Charisma of 10 from carrying a pool nothing can ever spend.
		maxUses := char.abilityScores.Modifier(abilities.CHA)
		if maxUses < 1 {
			maxUses = 1
		}
		built[resources.Inspiration] = combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(resources.Inspiration),
			Maximum:     maxUses,
			CharacterID: char.id,
			ResetType:   coreResources.ResetLongRest,
		})

	case classes.Monk:
		// Ki points - equal to monk level, recovered on short or long rest.
		// SRD: Ki starts at Monk level 2 (Flurry of Blows, Patient Defense, and
		// Step of the Wind -- the features that spend Ki -- are all level-2
		// features), so a level 1 Monk gets no Ki resource at all.
		if level >= 2 {
			kiResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
				ID:          string(resources.Ki),
				Maximum:     level,
				CharacterID: char.id,
				ResetType:   coreResources.ResetShortRest,
			})
			built[resources.Ki] = kiResource
		}
	}

	// Spell slots - every class whose progression table has them, at the size
	// that table gives its class level. Not a case in the switch above: the
	// table is what says which classes cast and how much.
	addSpellSlots(built, char, class, classLevel)

	// Hit dice - all classes get hit dice for short rest healing.
	// Uses helper which includes special recovery logic (half per long rest, min 1).
	// Counted by CHARACTER level: every level taken adds a die, whatever class
	// took it.
	built[resources.HitDice] = resources.NewHitDiceResource(resources.HitDiceResourceConfig{
		CharacterID: char.id,
		Level:       characterLevel,
	})

	return built
}

// addSpellSlots sizes a caster's slot pools from its class progression table,
// one pool per spell level the table reaches at this class level.
//
// This replaced a per-class switch case that read a single level-1 constant
// (design R4.6a: "Slot pools are sized from the table by class level for every
// class whose table has slots; the per-class switch case in buildClassResources
// goes"). Under the old shape a level-2 bard computed a gain of 2 - 2 = 0 and
// its pool never moved, and wizard, druid and sorcerer had slot data and never
// received a pool at all, so casting was silently impossible for them.
//
// Pact Magic is excluded and not forgotten: a warlock's slots are all the same
// level, climb with the warlock, and return on a SHORT rest. A pool built here
// would recover on the wrong rest, which is worse than the nothing a warlock
// has today, so the table says pact_magic and this declines to size it (§8).
func addSpellSlots(
	built map[coreResources.ResourceKey]*combat.RecoverableResource,
	char *Character, class classes.Class, classLevel int,
) {
	row := classes.SpellProgressionAtLevel(class, classLevel)
	if row.SlotReset != classes.SpellSlotResetLongRest {
		return
	}

	for index, count := range row.SpellSlots {
		if count <= 0 {
			continue
		}
		key, ok := resources.SpellSlotLevel(index + 1)
		if !ok {
			// A table reaching past 9th level is a content defect, not a
			// resource: there is no pool to put those slots in.
			continue
		}
		built[key] = combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(key),
			Maximum:     count,
			CharacterID: char.id,
			ResetType:   coreResources.ResetLongRest,
		})
	}
}

// initializeStandardCombatAbilities adds universal combat abilities to the character.
// These are available to all characters: Attack, Dash, Dodge, Disengage.
// Called during ToCharacter after the character struct is created.
func (d *Draft) initializeStandardCombatAbilities(char *Character) {
	// Attack - consumes action economy to grant attack capacity
	attackAbility := combatabilities.NewAttack(char.id + "-attack")
	_ = char.AddCombatAbility(attackAbility)

	// Dash - consumes action economy to add movement
	dashAbility := combatabilities.NewDash(char.id + "-dash")
	_ = char.AddCombatAbility(dashAbility)

	// Dodge - consumes action economy to grant Dodging condition
	dodgeAbility := combatabilities.NewDodge(char.id + "-dodge")
	_ = char.AddCombatAbility(dodgeAbility)

	// Disengage - consumes action economy to grant Disengaging condition
	disengageAbility := combatabilities.NewDisengage(char.id + "-disengage")
	_ = char.AddCombatAbility(disengageAbility)

	// Help - consumes action economy to aid an ally (advantage on their next roll)
	helpAbility := combatabilities.NewHelp(char.id + "-help")
	_ = char.AddCombatAbility(helpAbility)

	// Hide - consumes action economy to attempt a Stealth check (become hidden)
	hideAbility := combatabilities.NewHide(char.id + "-hide")
	_ = char.AddCombatAbility(hideAbility)
}

// appendSpellGrants resolves automatic grants through the same catalog as choices.
func appendSpellGrants(selected []*core.Ref, grants []spells.Spell) ([]*core.Ref, error) {
	compiled, err := compileKnownSpells([]choices.ChoiceData{{Category: shared.ChoiceSpells, SpellSelection: grants}}, shared.ChoiceSpells, "domain spell")
	if err != nil {
		return nil, err
	}
	for _, grant := range compiled {
		found := false
		for _, existing := range selected {
			if existing.String() == grant.String() {
				found = true
				break
			}
		}
		if !found {
			selected = append(selected, grant)
		}
	}
	return selected, nil
}
