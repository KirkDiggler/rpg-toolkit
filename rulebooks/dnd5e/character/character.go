// Package character provides D&D 5e character creation and management functionality
package character

import (
	"context"
	"encoding/json"
	"maps"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Character represents a playable D&D 5e character
// This is the domain model used during gameplay
type Character struct {
	// Identity
	id       string
	playerID string
	name     string

	// Core attributes
	level            int
	proficiencyBonus int

	// Race and class
	raceID     races.Race
	subraceID  races.Subrace
	classID    classes.Class
	subclassID classes.Subclass

	// Ability scores (includes racial modifiers)
	abilityScores shared.AbilityScores

	// Combat stats
	hitPoints    int
	maxHitPoints int
	armorClass   int
	hitDice      int // Size of hit die (d6, d8, d10, d12)

	// Proficiencies and skills
	skills              map[skills.Skill]shared.ProficiencyLevel
	savingThrows        map[abilities.Ability]shared.ProficiencyLevel
	languages           []languages.Language
	armorProficiencies  []proficiencies.Armor
	weaponProficiencies []proficiencies.Weapon
	toolProficiencies   []proficiencies.Tool

	// Equipment and resources
	inventory      []InventoryItem
	equipmentSlots EquipmentSlots
	spellSlots     map[int]SpellSlotData
	classResources map[shared.ClassResourceType]ResourceData
	resources      map[coreResources.ResourceKey]*combat.RecoverableResource

	// Features (rage, second wind, etc)
	features []features.Feature

	// Conditions (raging, poisoned, stunned, etc)
	conditions []dnd5eEvents.ConditionBehavior

	// Event handling
	bus             events.EventBus
	subscriptionIDs []string
}

// GetID returns the character's unique identifier
func (c *Character) GetID() string {
	return c.id
}

// GetType returns the entity type (implements core.Entity)
func (c *Character) GetType() core.EntityType {
	return "character"
}

// GetName returns the character's name
func (c *Character) GetName() string {
	return c.name
}

// GetLevel returns the character's level
func (c *Character) GetLevel() int {
	return c.level
}

// GetAbilityScore returns the character's ability score (including racial modifiers)
func (c *Character) GetAbilityScore(ability abilities.Ability) int {
	return c.abilityScores[ability]
}

// GetAbilityModifier returns the modifier for an ability score
func (c *Character) GetAbilityModifier(ability abilities.Ability) int {
	return c.abilityScores.Modifier(ability)
}

// GetProficiencyBonus returns the character's proficiency bonus
func (c *Character) GetProficiencyBonus() int {
	return c.proficiencyBonus
}

// GetSkillModifier returns the total modifier for a skill check
func (c *Character) GetSkillModifier(skill skills.Skill) int {
	ability := skills.Ability(skill)
	modifier := c.GetAbilityModifier(ability)

	if level, hasProficiency := c.skills[skill]; hasProficiency {
		switch level {
		case shared.Proficient:
			modifier += c.proficiencyBonus
		case shared.Expert:
			modifier += c.proficiencyBonus * 2
		}
	}

	return modifier
}

// GetSavingThrowModifier returns the total modifier for a saving throw
func (c *Character) GetSavingThrowModifier(ability abilities.Ability) int {
	modifier := c.GetAbilityModifier(ability)

	if level, hasProficiency := c.savingThrows[ability]; hasProficiency && level == shared.Proficient {
		modifier += c.proficiencyBonus
	}

	return modifier
}

// GetFeatures returns all character features
func (c *Character) GetFeatures() []features.Feature {
	return c.features
}

// GetFeature returns a specific feature by ID
func (c *Character) GetFeature(id string) features.Feature {
	for _, f := range c.features {
		if f.GetID() == id {
			return f
		}
	}
	return nil
}

// GetConditions returns all active conditions
func (c *Character) GetConditions() []dnd5eEvents.ConditionBehavior {
	return c.conditions
}

// GetHitPoints returns the character's current hit points
func (c *Character) GetHitPoints() int {
	return c.hitPoints
}

// GetMaxHitPoints returns the character's maximum hit points
func (c *Character) GetMaxHitPoints() int {
	return c.maxHitPoints
}

// emptyResource is returned when a resource doesn't exist.
// It has 0 maximum and 0 current, so IsEmpty() returns true.
var emptyResource = combat.NewRecoverableResource(combat.RecoverableResourceConfig{
	ID:      "",
	Maximum: 0,
})

// GetResource returns the resource for the given key.
// If the resource doesn't exist, returns an empty resource (not nil).
// Use IsEmpty() to check if the resource exists and has uses available.
func (c *Character) GetResource(key coreResources.ResourceKey) *combat.RecoverableResource {
	if c.resources == nil {
		return emptyResource
	}
	if r, ok := c.resources[key]; ok {
		return r
	}
	return emptyResource
}

// AddResource adds a new recoverable resource to the character
func (c *Character) AddResource(key coreResources.ResourceKey, resource *combat.RecoverableResource) {
	if c.resources == nil {
		c.resources = make(map[coreResources.ResourceKey]*combat.RecoverableResource)
	}
	c.resources[key] = resource
}

// GetResourceData returns serializable resource data for persistence
func (c *Character) GetResourceData() map[coreResources.ResourceKey]RecoverableResourceData {
	if c.resources == nil {
		return nil
	}

	data := make(map[coreResources.ResourceKey]RecoverableResourceData, len(c.resources))
	for key, resource := range c.resources {
		data[key] = RecoverableResourceData{
			Current:   resource.Current(),
			Maximum:   resource.Maximum(),
			ResetType: resource.ResetType,
		}
	}
	return data
}

// LoadResourceData loads resources from serialized data and applies them to the event bus.
// Resources are applied so they subscribe to rest events for automatic recovery.
func (c *Character) LoadResourceData(
	ctx context.Context,
	bus events.EventBus,
	data map[coreResources.ResourceKey]RecoverableResourceData,
) {
	if data == nil {
		return
	}

	if c.resources == nil {
		c.resources = make(map[coreResources.ResourceKey]*combat.RecoverableResource)
	}

	for key, resData := range data {
		resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(key),
			Maximum:     resData.Maximum,
			CharacterID: c.id,
			ResetType:   resData.ResetType,
		})

		// Set current value if different from maximum
		if resData.Current != resData.Maximum {
			deficit := resData.Maximum - resData.Current
			_ = resource.Use(deficit) // Ignore error - we know the value is valid
		}

		// Apply resource to subscribe to rest events
		if err := resource.Apply(ctx, bus); err != nil {
			// Clean up on failure and skip this resource
			_ = resource.Remove(ctx, bus)
			continue
		}

		c.resources[key] = resource
	}
}

// GetEquippedSlot returns the equipped item for a slot.
// Resolves the slot's item ID to the actual equipment from inventory.
// Returns nil if nothing is equipped in that slot or item not found in inventory.
func (c *Character) GetEquippedSlot(slot InventorySlot) *EquippedItem {
	itemID := c.equipmentSlots.Get(slot)
	if itemID == "" {
		return nil
	}

	// Find the item in inventory
	for _, invItem := range c.inventory {
		if invItem.Equipment.EquipmentID() == itemID {
			return &EquippedItem{Item: invItem.Equipment}
		}
	}

	return nil
}

// EquipItem equips an inventory item to the specified slot.
// Returns error if the item is not in inventory.
func (c *Character) EquipItem(slot InventorySlot, itemID string) error {
	// Verify item exists in inventory
	found := false
	for _, invItem := range c.inventory {
		if invItem.Equipment.EquipmentID() == itemID {
			found = true
			break
		}
	}

	if !found {
		return rpgerr.New(rpgerr.CodeNotFound, "item not found in inventory")
	}

	// Initialize map if nil
	if c.equipmentSlots == nil {
		c.equipmentSlots = make(EquipmentSlots)
	}
	c.equipmentSlots.Set(slot, itemID)
	return nil
}

// UnequipItem removes the item from the specified slot.
func (c *Character) UnequipItem(slot InventorySlot) {
	c.equipmentSlots.Clear(slot)
}

// ToData converts the character to its persistent data form
func (c *Character) ToData() *Data {
	data := &Data{
		ID:                  c.id,
		PlayerID:            c.playerID,
		Name:                c.name,
		Level:               c.level,
		ProficiencyBonus:    c.proficiencyBonus,
		RaceID:              c.raceID,
		SubraceID:           c.subraceID,
		ClassID:             c.classID,
		SubclassID:          c.subclassID,
		AbilityScores:       c.abilityScores,
		HitPoints:           c.hitPoints,
		MaxHitPoints:        c.maxHitPoints,
		ArmorClass:          c.armorClass,
		Skills:              maps.Clone(c.skills),
		SavingThrows:        maps.Clone(c.savingThrows),
		ArmorProficiencies:  c.armorProficiencies,
		WeaponProficiencies: c.weaponProficiencies,
		ToolProficiencies:   c.toolProficiencies,
		UpdatedAt:           time.Now(),
	}

	// Convert inventory to data
	data.Inventory = make([]InventoryItemData, 0, len(c.inventory))
	for _, item := range c.inventory {
		data.Inventory = append(data.Inventory, item.ToData())
	}

	// Copy equipment slots
	data.EquipmentSlots = c.equipmentSlots

	// Copy languages slice
	data.Languages = c.languages

	// Copy spell slots map directly since SpellSlotData is already the data type
	data.SpellSlots = maps.Clone(c.spellSlots)

	// Copy class resources map directly since ResourceData is already the data type
	data.ClassResources = maps.Clone(c.classResources)

	// Convert resources to data
	if len(c.resources) > 0 {
		data.Resources = make(map[coreResources.ResourceKey]RecoverableResourceData, len(c.resources))
		for key, resource := range c.resources {
			data.Resources[key] = RecoverableResourceData{
				Current:   resource.Current(),
				Maximum:   resource.Maximum(),
				ResetType: resource.ResetType,
			}
		}
	}

	// Convert features to persisted JSON
	data.Features = make([]json.RawMessage, 0, len(c.features))
	for _, feature := range c.features {
		// Use the feature's ToJSON method to get the serialized form
		jsonData, err := feature.ToJSON()
		if err != nil {
			// Skip features that can't be serialized
			// TODO: Consider how to handle serialization errors
			continue
		}
		// The feature's ToJSON already includes the fully qualified ref
		data.Features = append(data.Features, jsonData)
	}

	// Convert conditions to persisted JSON (following same pattern as features)
	data.Conditions = make([]json.RawMessage, 0, len(c.conditions))
	for _, condition := range c.conditions {
		// Use the condition's ToJSON method to get the serialized form
		jsonData, err := condition.ToJSON()
		if err != nil {
			// Skip conditions that can't be serialized
			// TODO: Consider how to handle serialization errors
			continue
		}
		data.Conditions = append(data.Conditions, jsonData)
	}

	return data
}

// subscribeToEvents subscribes the character to gameplay events
func (c *Character) subscribeToEvents(ctx context.Context) error {
	if c.bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character has no event bus")
	}

	// Subscribe to condition applied events
	appliedTopic := dnd5eEvents.ConditionAppliedTopic.On(c.bus)
	subID, err := appliedTopic.Subscribe(ctx, c.onConditionApplied)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to condition applied")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, subID)

	// Subscribe to condition removed events
	removedTopic := dnd5eEvents.ConditionRemovedTopic.On(c.bus)
	subID, err = removedTopic.Subscribe(ctx, c.onConditionRemoved)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to condition removed events")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, subID)

	// Subscribe to healing received events
	healingTopic := dnd5eEvents.HealingReceivedTopic.On(c.bus)
	subID, err = healingTopic.Subscribe(ctx, c.onHealingReceived)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to healing received")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, subID)

	return nil
}

// onConditionApplied handles ConditionAppliedEvent
func (c *Character) onConditionApplied(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
	// Only process events for this character
	if event.Target.GetID() != c.id {
		return nil
	}

	// Apply the condition (subscribes to events)
	if err := event.Condition.Apply(ctx, c.bus); err != nil {
		// Clean up any partial subscriptions to avoid resource leaks
		_ = event.Condition.Remove(ctx, c.bus)
		return rpgerr.Wrapf(err, "failed to apply condition")
	}

	// Store the condition
	c.conditions = append(c.conditions, event.Condition)

	return nil
}

// onConditionRemoved handles ConditionRemovedEvent
func (c *Character) onConditionRemoved(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
	// Only process events for this character
	if event.CharacterID != c.id {
		return nil
	}

	// Remove the condition from our list by matching the ConditionRef
	filtered := make([]dnd5eEvents.ConditionBehavior, 0, len(c.conditions))
	for _, cond := range c.conditions {
		// Get the condition's ref by converting to JSON and parsing
		jsonData, err := cond.ToJSON()
		if err != nil {
			return rpgerr.Wrapf(err, "failed to serialize condition for removal check")
		}

		// Parse the ref from JSON
		var refData struct {
			Ref core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(jsonData, &refData); err != nil {
			return rpgerr.Wrapf(err, "failed to parse condition ref from JSON")
		}

		// Keep condition if it doesn't match the removed ref
		if refData.Ref.String() != event.ConditionRef {
			filtered = append(filtered, cond)
		}
	}
	c.conditions = filtered

	return nil
}

// onHealingReceived handles HealingReceivedEvent
func (c *Character) onHealingReceived(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
	// Only process events for this character
	if event.TargetID != c.id {
		return nil
	}

	// Apply healing: add Amount to hitPoints, cap at maxHitPoints
	c.hitPoints += event.Amount
	if c.hitPoints > c.maxHitPoints {
		c.hitPoints = c.maxHitPoints
	}

	return nil
}

// Cleanup unsubscribes from all events and removes all active conditions
func (c *Character) Cleanup(ctx context.Context) error {
	if c.bus == nil {
		return nil
	}

	var errors []error

	// Remove all active conditions - collect errors but try to remove all
	for _, cond := range c.conditions {
		if err := cond.Remove(ctx, c.bus); err != nil {
			errors = append(errors, rpgerr.Wrapf(err, "failed to remove condition"))
		}
	}
	c.conditions = nil

	// Unsubscribe from events - collect errors but try to unsubscribe all
	for _, subID := range c.subscriptionIDs {
		if err := c.bus.Unsubscribe(ctx, subID); err != nil {
			errors = append(errors, rpgerr.Wrapf(err, "failed to unsubscribe"))
		}
	}
	c.subscriptionIDs = nil

	// Return first error if any occurred
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}
