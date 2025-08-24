// Package character provides D&D 5e character creation and management functionality
package character

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/game"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Event types for character-related events

// ConditionAppliedEvent is published when a condition is applied to an entity.
type ConditionAppliedEvent struct {
	CharacterID string                       // ID of the character receiving the condition
	Condition   conditions.ConditionBehavior // The actual condition to apply
	Source      string                       // What caused this condition
}

// ConditionRemovedEvent is published when a condition is removed from an entity
type ConditionRemovedEvent struct {
	CharacterID  string // ID of the character losing the condition
	ConditionRef string // Ref of the condition being removed
	Reason       string // Why it was removed
}

// AttackEvent is published when a character makes an attack
type AttackEvent struct {
	AttackerID string // ID of the attacking character
	TargetID   string // ID of the target
	WeaponRef  string // Reference to the weapon used
	IsMelee    bool   // True for melee attacks
	Damage     int    // Base damage before modifiers
}

// Topic definitions for typed event system
var (
	ConditionAppliedTopic = events.DefineTypedTopic[ConditionAppliedEvent]("dnd5e.condition.applied")
	ConditionRemovedTopic = events.DefineTypedTopic[ConditionRemovedEvent]("dnd5e.condition.removed")
	AttackTopic           = events.DefineTypedTopic[AttackEvent]("dnd5e.combat.attack")
)

// Character represents a D&D 5e character with full game mechanics
type Character struct {
	id               string
	playerID         string
	name             string
	level            int
	proficiencyBonus int

	// Event bus for condition management
	eventBus events.EventBus

	// Character creation info (IDs only for reference)
	raceID       races.Race
	classID      classes.Class
	backgroundID backgrounds.Background

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
	skills        map[skills.Skill]shared.ProficiencyLevel
	savingThrows  map[abilities.Ability]shared.ProficiencyLevel
	languages     []languages.Language
	proficiencies shared.Proficiencies
	features      []json.RawMessage // Feature data stored as JSON

	// Current state - changes during play
	conditions    []json.RawMessage // Store condition data as JSON
	effects       []effects.Effect
	exhaustion    int
	deathSaves    shared.DeathSaves
	tempHitPoints int

	// Resources
	spellSlots     SpellSlots
	classResources map[shared.ClassResourceType]Resource // rage uses, ki points, etc

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

// Attack performs an attack with the given weapon
func (c *Character) Attack(ctx context.Context, weapon Weapon) *AttackResult {
	if weapon == nil {
		return nil
	}

	// Determine ability modifier (STR for melee, could be DEX for finesse)
	var abilityMod int
	if weapon.IsMelee() {
		abilityMod = c.getAbilityModifier(abilities.STR)
	} else {
		abilityMod = c.getAbilityModifier(abilities.DEX)
	}

	// Check proficiency with weapon
	// For now, simple check: barbarians are proficient with martial weapons
	// Wizards are not proficient with martial weapons
	profBonus := 0
	if c.classID == "barbarian" || !hasProperty(weapon.GetProperties(), "martial") {
		profBonus = c.proficiencyBonus
	}

	// Calculate attack bonus
	attackBonus := profBonus + abilityMod

	// Calculate damage bonus (ability modifier + any condition bonuses)
	damageBonus := abilityMod

	// Check for rage damage bonus
	if c.HasCondition("dnd5e:conditions:raging") && weapon.IsMelee() {
		// Find the rage condition to get damage bonus
		for _, condData := range c.conditions {
			var peek struct {
				Ref         string `json:"ref"`
				DamageBonus int    `json:"damage_bonus"`
			}
			if err := json.Unmarshal(condData, &peek); err == nil {
				if peek.Ref == "dnd5e:conditions:raging" {
					damageBonus += peek.DamageBonus
					break
				}
			}
		}
	}

	// For now, we'll use placeholder rolls
	// In a real system, would use dice roller
	attackRoll := 10 // Would be 1d20
	damageRoll := 5  // Would be weapon damage dice

	// Publish attack event
	if c.eventBus != nil {
		attacks := AttackTopic.On(c.eventBus)
		_ = attacks.Publish(ctx, AttackEvent{
			AttackerID: c.id,
			TargetID:   "", // No target for now
			WeaponRef:  weapon.GetRef(),
			IsMelee:    weapon.IsMelee(),
			Damage:     damageRoll + damageBonus,
		})
	}

	return &AttackResult{
		AttackRoll:       attackRoll,
		AttackBonus:      attackBonus,
		ProficiencyBonus: profBonus,
		AbilityModifier:  abilityMod,
		DamageRoll:       damageRoll,
		DamageBonus:      damageBonus,
		WeaponRef:        weapon.GetRef(),
	}
}

// getAbilityModifier calculates the modifier for an ability score
func (c *Character) getAbilityModifier(ability abilities.Ability) int {
	score := c.abilityScores[ability]
	return (score - 10) / 2
}

// hasProperty checks if a weapon has a specific property
func hasProperty(properties []string, property string) bool {
	for _, p := range properties {
		if p == property {
			return true
		}
	}
	return false
}

// GetID returns the character's unique identifier
func (c *Character) GetID() string {
	return c.id
}

// GetType returns the entity type (character)
func (c *Character) GetType() core.EntityType {
	return "character"
}

// AddFeature adds a feature to the character
// TODO: This needs proper implementation when we modernize character creation
// See issue: https://github.com/KirkDiggler/rpg-toolkit/issues/XXX
func (c *Character) AddFeature(_ interface{}) {
	// FIXME: This is a stub - should convert feature to JSON and store properly
	// For now, do nothing rather than inject nil which causes panics
}

// GetFeatures returns the character's features as JSON
func (c *Character) GetFeatures() []json.RawMessage {
	return c.features
}

// HasFeatureID checks if the character has a feature with the given ID
func (c *Character) HasFeatureID(featureID string) bool {
	for _, featureData := range c.features {
		var peek struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(featureData, &peek); err == nil {
			if peek.ID == featureID {
				return true
			}
		}
	}
	return false
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

// OnConditionApplied handles condition applied events targeting this character.
// It applies the condition and stores its JSON data for persistence.
func (c *Character) OnConditionApplied(ctx context.Context, event ConditionAppliedEvent) error {
	// Only handle conditions for this character
	if event.CharacterID != c.id {
		return nil
	}

	// Check if we have a condition to apply
	if event.Condition == nil {
		return nil
	}

	// Apply the condition - it will subscribe to relevant events
	if err := event.Condition.Apply(ctx, c.eventBus); err != nil {
		return err
	}

	// Convert to JSON for persistence
	conditionJSON, err := event.Condition.ToJSON()
	if err != nil {
		return err
	}

	// Store the condition data as JSON
	c.conditions = append(c.conditions, conditionJSON)
	return nil
}

// OnConditionRemoved handles condition removed events targeting this character
func (c *Character) OnConditionRemoved(_ context.Context, event ConditionRemovedEvent) error {
	// Only handle conditions for this character
	if event.CharacterID != c.id {
		return nil
	}

	// Remove the condition by ref
	var filtered []json.RawMessage
	for _, condData := range c.conditions {
		// Peek at the ref to see if this is the one to remove
		var peek struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(condData, &peek); err == nil {
			if peek.Ref != event.ConditionRef {
				filtered = append(filtered, condData)
			}
		}
	}
	c.conditions = filtered
	return nil
}

// HasCondition checks if character has a specific condition by ref
func (c *Character) HasCondition(conditionRef string) bool {
	for _, condData := range c.conditions {
		var peek struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(condData, &peek); err == nil {
			if peek.Ref == conditionRef {
				return true
			}
		}
	}
	return false
}

// GetConditions returns all active condition data as JSON
func (c *Character) GetConditions() []json.RawMessage {
	return c.conditions
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
func (c *Character) GetClassResources() map[shared.ClassResourceType]Resource {
	return c.classResources
}

// GetSpellSlots returns the character's spell slots
func (c *Character) GetSpellSlots() SpellSlots {
	return c.spellSlots
}

// ApplyToEventBus subscribes the character to relevant events on the bus.
// This should be called after loading the character to enable event handling.
func (c *Character) ApplyToEventBus(ctx context.Context, bus events.EventBus) error {
	// Store the bus for condition Apply() calls
	c.eventBus = bus

	// Subscribe to condition applied events
	conditions := ConditionAppliedTopic.On(bus)
	_, err := conditions.Subscribe(ctx, c.OnConditionApplied)
	if err != nil {
		return err
	}

	// Subscribe to condition removed events
	removals := ConditionRemovedTopic.On(bus)
	_, err = removals.Subscribe(ctx, c.OnConditionRemoved)
	if err != nil {
		return err
	}

	return nil
}

// Data represents the persistent state of a character
type Data struct {
	ID         string `json:"id"`
	PlayerID   string `json:"player_id"`
	Name       string `json:"name"`
	Level      int    `json:"level"`
	Experience int    `json:"experience"`

	// References to external data
	RaceID       races.Race             `json:"race_id"`
	SubraceID    races.Subrace          `json:"subrace_id,omitempty"`
	ClassID      classes.Class          `json:"class_id"`
	SubclassID   classes.Subclass       `json:"subclass_id,omitempty"`
	BackgroundID backgrounds.Background `json:"background_id"`

	// Core stats
	AbilityScores shared.AbilityScores `json:"ability_scores"`
	HitPoints     int                  `json:"hit_points"`
	MaxHitPoints  int                  `json:"max_hit_points"`

	// Physical characteristics (denormalized from race)
	Speed int    `json:"speed"`
	Size  string `json:"size"`

	// Proficiencies and skills
	Skills        map[skills.Skill]shared.ProficiencyLevel      `json:"skills"`        // skill -> proficiency level
	SavingThrows  map[abilities.Ability]shared.ProficiencyLevel `json:"saving_throws"` // ability -> proficiency level
	Languages     []string                                      `json:"languages"`
	Proficiencies shared.Proficiencies                          `json:"proficiencies"`

	// Current state
	Conditions []json.RawMessage `json:"conditions"` // Store as JSON for direct persistence
	Effects    []effects.Effect  `json:"effects"`
	Exhaustion int               `json:"exhaustion"`
	DeathSaves shared.DeathSaves `json:"death_saves"`

	// Resources
	SpellSlots     map[int]SlotInfo                          `json:"spell_slots"`
	ClassResources map[shared.ClassResourceType]ResourceData `json:"class_resources"`

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
	Type    shared.ClassResourceType `json:"type"`    // Resource type enum
	Name    string                   `json:"name"`    // Display name
	Max     int                      `json:"max"`     // Maximum uses
	Current int                      `json:"current"` // Current uses
	Resets  shared.ResetType         `json:"resets"`  // When it resets
}

// ToData converts the character to its persistent representation
func (c *Character) ToData() Data {
	savesData := make(map[abilities.Ability]shared.ProficiencyLevel)
	for save, prof := range c.savingThrows {
		savesData[save] = prof
	}

	resourcesData := make(map[shared.ClassResourceType]ResourceData)
	for resType, res := range c.classResources {
		resourcesData[resType] = ResourceData{
			Type:    resType,
			Name:    res.Name,
			Max:     res.Max,
			Current: res.Current,
			Resets:  res.Resets,
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
		Skills:         c.skills,
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

	// Skills are already typed correctly

	// Build languages from persisted data
	langs := make([]languages.Language, len(data.Languages))
	for i, langStr := range data.Languages {
		langs[i] = languages.Language(langStr)
	}

	// Saving throws are already typed correctly

	resources := make(map[shared.ClassResourceType]Resource)
	for resType, res := range data.ClassResources {
		resources[resType] = Resource{
			Name:    res.Name,
			Max:     res.Max,
			Current: res.Current,
			Resets:  res.Resets,
		}
	}

	// Extract features from class data (store as JSON for persistence)
	var features []json.RawMessage
	for lvl := 1; lvl <= data.Level; lvl++ {
		for _, feature := range classData.Features[lvl] {
			// For now, store feature ID as simple JSON
			featureJSON, _ := json.Marshal(map[string]string{"id": feature.ID})
			features = append(features, featureJSON)
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
		armorClass:       10 + data.AbilityScores.Modifier(abilities.DEX), // Base AC, equipment will modify
		initiative:       data.AbilityScores.Modifier(abilities.DEX),
		hitDice:          classData.HitDice,
		skills:           data.Skills,
		savingThrows:     data.SavingThrows,
		languages:        langs,
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

// Weapon interface represents any weapon that can be used for attacks
type Weapon interface {
	GetRef() string          // Unique reference ID
	GetDamage() string       // Damage dice notation (e.g., "1d8")
	IsMelee() bool           // True for melee weapons
	GetProperties() []string // Weapon properties (heavy, finesse, etc.)
}

// AttackResult represents the result of an attack
type AttackResult struct {
	AttackRoll       int    // The d20 roll result
	AttackBonus      int    // Total attack bonus (proficiency + ability)
	ProficiencyBonus int    // Proficiency bonus if proficient
	AbilityModifier  int    // STR or DEX modifier
	DamageRoll       int    // Damage dice result
	DamageBonus      int    // Total damage bonus (ability + rage, etc.)
	WeaponRef        string // Reference to weapon used
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
