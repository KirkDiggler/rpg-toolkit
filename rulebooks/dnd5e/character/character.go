// Package character provides D&D 5e character creation and management functionality
package character

import (
	"context"
	"errors"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/game"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Character represents a D&D 5e character with full game mechanics
type Character struct {
	id               string
	playerID         string
	name             string
	level            int
	proficiencyBonus int

	// Character creation info (IDs only for reference)
	raceID       constants.Race
	classID      constants.Class
	backgroundID constants.Background

	// Core attributes
	abilityScores shared.AbilityScores

	// Physical characteristics (from race during creation)
	size  string
	speed int

	// Combat stats
	hitPoints    int
	maxHitPoints int
	armorClass   int
	initiative   int
	hitDice      int // From class

	// Capabilities (compiled from race/class/background)
	skills        map[constants.Skill]shared.ProficiencyLevel
	savingThrows  map[constants.Ability]shared.ProficiencyLevel
	languages     []constants.Language
	proficiencies shared.Proficiencies
	features      []string // Feature IDs they have

	// Current state - changes during play
	conditions    []conditions.Condition
	effects       []effects.Effect
	exhaustion    int
	deathSaves    shared.DeathSaves
	tempHitPoints int

	// Resources
	spellSlots     SpellSlots
	classResources map[string]Resource // rage uses, ki points, etc

	// Equipment
	equipment []string

	// Choices made during creation
	choices []ChoiceData
}

// SpellSlots tracks available spell slots by level
type SpellSlots map[int]SlotInfo

// SlotInfo represents spell slot usage
type SlotInfo struct {
	Max  int
	Used int
}

// Resource represents a class resource like rage or ki
type Resource struct {
	Name    string
	Max     int
	Current int
	Resets  shared.ResetType
}

// Attack performs an attack roll against a target
// TODO: This is a placeholder implementation. In a complete system, this would:
// - Calculate attack bonus (ability modifier + proficiency if proficient)
// - Roll attack using combat.RollAttack
// - Apply any active effects that modify attacks
// - Return detailed attack results
func (c *Character) Attack(_ Weapon, _ Target) AttackResult {
	// Placeholder implementation - returns empty result
	return AttackResult{}
}

// SaveThrow performs a saving throw
// TODO: This is a placeholder implementation. In a complete system, this would:
// - Calculate ability modifier for the given ability
// - Add proficiency bonus if proficient in that save
// - Apply any active effects (like Bless) that modify saves
// - Roll 1d20 + modifiers vs DC
func (c *Character) SaveThrow(_ string, _ int) SaveResult {
	// Placeholder implementation - returns empty result
	// The commented code shows the intended logic structure
	// mod := c.abilityModifier(ability)
	// if c.isProficient(ability + "_save") {
	// 	mod += c.proficiencyBonus
	// }
	return SaveResult{}
}

// SkillCheck performs a skill check
// TODO: This is a placeholder implementation. In a complete system, this would:
// - Determine ability modifier for the skill
// - Add proficiency bonus if proficient (or expertise for double)
// - Apply any active effects that modify skill checks
// - Roll 1d20 + modifiers vs DC
func (c *Character) SkillCheck(_ string, _ int) CheckResult {
	// Placeholder implementation - returns empty result
	return CheckResult{}
}

// AC returns the character's armor class
func (c *Character) AC() int {
	ac := c.armorClass
	// Apply any AC bonuses from effects
	for _, effect := range c.effects {
		ac += effect.ACBonus
	}
	return ac
}

// Initiative returns the character's initiative modifier
func (c *Character) Initiative() int {
	return c.initiative
}

// AddCondition adds a condition to the character
func (c *Character) AddCondition(condition conditions.Condition) {
	c.conditions = append(c.conditions, condition)
}

// RemoveCondition removes a condition by type
func (c *Character) RemoveCondition(conditionType conditions.ConditionType) {
	var filtered []conditions.Condition
	for _, c := range c.conditions {
		if c.Type != conditionType {
			filtered = append(filtered, c)
		}
	}
	c.conditions = filtered
}

// HasCondition checks if character has a specific condition
func (c *Character) HasCondition(conditionType conditions.ConditionType) bool {
	for _, c := range c.conditions {
		if c.Type == conditionType {
			return true
		}
	}
	return false
}

// AddEffect adds an effect to the character
func (c *Character) AddEffect(effect effects.Effect) {
	c.effects = append(c.effects, effect)
}

// RemoveEffect removes effects from a specific source
func (c *Character) RemoveEffect(source string) {
	var filtered []effects.Effect
	for _, e := range c.effects {
		if e.Source != source {
			filtered = append(filtered, e)
		}
	}
	c.effects = filtered
}

// GetEffects returns all active effects
func (c *Character) GetEffects() []effects.Effect {
	return c.effects
}

// GetEquipment returns the character's equipment
func (c *Character) GetEquipment() []string {
	return c.equipment
}

// GetClassResources returns the character's class resources
func (c *Character) GetClassResources() map[string]Resource {
	return c.classResources
}

// GetSpellSlots returns the character's spell slots
func (c *Character) GetSpellSlots() SpellSlots {
	return c.spellSlots
}

// Data represents the persistent state of a character
type Data struct {
	ID         string `json:"id"`
	PlayerID   string `json:"player_id"`
	Name       string `json:"name"`
	Level      int    `json:"level"`
	Experience int    `json:"experience"`

	// References to external data
	RaceID       constants.Race       `json:"race_id"`
	SubraceID    constants.Subrace    `json:"subrace_id,omitempty"`
	ClassID      constants.Class      `json:"class_id"`
	SubclassID   constants.Subclass   `json:"subclass_id,omitempty"`
	BackgroundID constants.Background `json:"background_id"`

	// Core stats
	AbilityScores shared.AbilityScores `json:"ability_scores"`
	HitPoints     int                  `json:"hit_points"`
	MaxHitPoints  int                  `json:"max_hit_points"`

	// Physical characteristics (denormalized from race)
	Speed int    `json:"speed"`
	Size  string `json:"size"`

	// Proficiencies and skills
	Skills        map[string]shared.ProficiencyLevel `json:"skills"`        // skill name -> proficiency level
	SavingThrows  map[string]shared.ProficiencyLevel `json:"saving_throws"` // ability -> proficiency level
	Languages     []string                           `json:"languages"`
	Proficiencies shared.Proficiencies               `json:"proficiencies"`

	// Current state
	Conditions []conditions.Condition `json:"conditions"`
	Effects    []effects.Effect       `json:"effects"`
	Exhaustion int                    `json:"exhaustion"`
	DeathSaves shared.DeathSaves      `json:"death_saves"`

	// Resources
	SpellSlots     map[int]SlotInfo        `json:"spell_slots"`
	ClassResources map[string]ResourceData `json:"class_resources"`

	// Equipment
	Equipment []string `json:"equipment"`

	// Character creation choices
	Choices []ChoiceData `json:"choices"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResourceData represents persistent resource data
type ResourceData struct {
	Name    string `json:"name"`
	Max     int    `json:"max"`
	Current int    `json:"current"`
	Resets  string `json:"resets"`
}

// ChoiceData represents a choice made during character creation
type ChoiceData struct {
	Category  string `json:"category"`  // Standard: "skills", "languages", etc.
	Source    string `json:"source"`    // "race", "class", "background"
	ChoiceID  string `json:"choice_id"` // Specific choice identifier like "fighter_proficiencies_1"
	Selection any    `json:"selection"` // The actual choice made
}

// ToData converts the character to its persistent representation
func (c *Character) ToData() Data {
	skillsData := make(map[string]shared.ProficiencyLevel)
	for skill, prof := range c.skills {
		skillsData[string(skill)] = prof
	}

	savesData := make(map[string]shared.ProficiencyLevel)
	for save, prof := range c.savingThrows {
		savesData[string(save)] = prof
	}

	resourcesData := make(map[string]ResourceData)
	for name, res := range c.classResources {
		resourcesData[name] = ResourceData{
			Name:    res.Name,
			Max:     res.Max,
			Current: res.Current,
			Resets:  string(res.Resets),
		}
	}

	languagesData := make([]string, len(c.languages))
	for i, lang := range c.languages {
		languagesData[i] = string(lang)
	}

	data := Data{
		PlayerID:       c.playerID,
		ID:             c.id,
		Name:           c.name,
		Level:          c.level,
		RaceID:         c.raceID,
		ClassID:        c.classID,
		BackgroundID:   c.backgroundID,
		AbilityScores:  c.abilityScores,
		HitPoints:      c.hitPoints,
		MaxHitPoints:   c.maxHitPoints,
		Speed:          c.speed,
		Size:           c.size,
		Skills:         skillsData,
		SavingThrows:   savesData,
		Languages:      languagesData,
		Proficiencies:  c.proficiencies,
		Conditions:     c.conditions,
		Effects:        c.effects,
		Exhaustion:     c.exhaustion,
		DeathSaves:     c.deathSaves,
		SpellSlots:     c.spellSlots,
		ClassResources: resourcesData,
		Equipment:      c.equipment,
		Choices:        c.choices,
		UpdatedAt:      time.Now(),
	}

	return data
}

func calculateProficiencyBonus(level int) int {
	return 2 + ((level - 1) / 4)
}

// LoadCharacterFromData creates a character from persistent data and game data
func LoadCharacterFromData(data Data, raceData *race.Data, classData *class.Data,
	backgroundData *shared.Background) (*Character, error) {
	if raceData == nil || classData == nil || backgroundData == nil {
		return nil, errors.New("race, class, and background data are required")
	}

	// Build skills from persisted data
	skills := make(map[constants.Skill]shared.ProficiencyLevel)
	for skillStr, level := range data.Skills {
		skill := constants.Skill(skillStr)
		skills[skill] = level
	}

	// Build languages from persisted data
	languages := make([]constants.Language, len(data.Languages))
	for i, langStr := range data.Languages {
		languages[i] = constants.Language(langStr)
	}

	// Build saving throws from persisted data
	saves := make(map[constants.Ability]shared.ProficiencyLevel)
	for saveStr, level := range data.SavingThrows {
		save := constants.Ability(saveStr)
		saves[save] = level
	}

	resources := make(map[string]Resource)
	for name, res := range data.ClassResources {
		resources[name] = Resource{
			Name:    res.Name,
			Max:     res.Max,
			Current: res.Current,
			Resets:  shared.ResetType(res.Resets),
		}
	}

	// Extract features from class data
	var features []string
	for lvl := 1; lvl <= data.Level; lvl++ {
		for _, feature := range classData.Features[lvl] {
			features = append(features, feature.ID)
		}
	}

	return &Character{
		id:               data.ID,
		playerID:         data.PlayerID,
		name:             data.Name,
		level:            data.Level,
		proficiencyBonus: calculateProficiencyBonus(data.Level),
		raceID:           data.RaceID,
		classID:          data.ClassID,
		backgroundID:     data.BackgroundID,
		abilityScores:    data.AbilityScores,
		size:             data.Size,
		speed:            data.Speed,
		hitPoints:        data.HitPoints,
		maxHitPoints:     data.MaxHitPoints,
		tempHitPoints:    0,                                               // Reset on load
		armorClass:       10 + data.AbilityScores.Modifier(constants.DEX), // Base AC, equipment will modify
		initiative:       data.AbilityScores.Modifier(constants.DEX),
		hitDice:          classData.HitDice,
		skills:           skills,
		savingThrows:     saves,
		languages:        languages,
		proficiencies:    data.Proficiencies,
		features:         features,
		conditions:       data.Conditions,
		effects:          data.Effects,
		exhaustion:       data.Exhaustion,
		deathSaves:       data.DeathSaves,
		spellSlots:       data.SpellSlots,
		classResources:   resources,
		equipment:        data.Equipment,
		choices:          data.Choices,
	}, nil
}

// LoadCharacterFromContext creates a character using the game.Context pattern.
// This provides a consistent loading interface across all game entities.
// Note: This still requires external dependencies (race, class, background) for now.
// A future version will use fully self-contained data as explored in Journey 019.
func LoadCharacterFromContext(_ context.Context, gameCtx game.Context[Data],
	raceData *race.Data, classData *class.Data, backgroundData *shared.Background) (*Character, error) {
	// Use the existing loader with data from context
	char, err := LoadCharacterFromData(gameCtx.Data(), raceData, classData, backgroundData)
	if err != nil {
		return nil, err
	}

	// TODO(#113): When event types are defined, emit character.loaded event
	// if gameCtx.EventBus() != nil {
	//     event := events.NewGameEvent("character.loaded", char, nil)
	//     gameCtx.EventBus().Publish(ctx, event)
	// }

	return char, nil
}

// Weapon represents a weapon for attacks
type Weapon struct {
	Name   string
	Damage string
}

// Target is an interface for attack targets
type Target interface {
	AC() int
}

// AttackResult represents the result of an attack
type AttackResult struct {
	Hit    bool
	Damage int
}

// SaveResult represents the result of a saving throw
type SaveResult struct {
	Success bool
	Roll    int
}

// CheckResult represents the result of a skill check
type CheckResult struct {
	Success bool
	Roll    int
}
