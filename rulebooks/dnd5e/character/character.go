// Package character provides D&D 5e character creation and management functionality
package character

import (
	"context"
	"encoding/json"
	"maps"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

const (
	// shieldCategory is the category value for shield items
	shieldCategory = "shield"
)

// Compile-time check that Character implements ActionHolder
var _ actions.ActionHolder = (*Character)(nil)

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

	// Features (rage, second wind, etc) - grant actions and conditions
	features []features.Feature

	// Actions (attack, dash, flurry strike, etc) - things you can do
	actions []actions.Action

	// Conditions (raging, poisoned, stunned, etc) - passive effects
	conditions []dnd5eEvents.ConditionBehavior

	// Death saves (tracked when at 0 HP)
	deathSaveState *saves.DeathSaveState

	// Event handling
	bus             events.EventBus
	subscriptionIDs []string

	// Dirty tracking for persistence
	dirty bool
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

// GetAbilityScores returns all ability scores (implements Combatant interface)
func (c *Character) GetAbilityScores() shared.AbilityScores {
	return c.abilityScores
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

// MakeSavingThrowInput contains parameters for a character saving throw
type MakeSavingThrowInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller

	// Ability is the ability score being tested (STR, DEX, CON, INT, WIS, CHA)
	Ability abilities.Ability

	// DC is the Difficulty Class that must be met or exceeded
	DC int

	// HasAdvantage indicates the character has advantage on this save
	HasAdvantage bool

	// HasDisadvantage indicates the character has disadvantage on this save
	HasDisadvantage bool
}

// MakeSavingThrow makes a saving throw for this character.
// The character's ability modifier and proficiency bonus (if proficient) are automatically applied.
// Returns the result including whether the save succeeded.
func (c *Character) MakeSavingThrow(
	ctx context.Context, input *MakeSavingThrowInput,
) (*saves.SavingThrowResult, error) {
	modifier := c.GetSavingThrowModifier(input.Ability)

	return saves.MakeSavingThrow(ctx, &saves.SavingThrowInput{
		Roller:          input.Roller,
		Ability:         input.Ability,
		DC:              input.DC,
		Modifier:        modifier,
		HasAdvantage:    input.HasAdvantage,
		HasDisadvantage: input.HasDisadvantage,
	})
}

// MakeDeathSaveInput contains parameters for a character death saving throw
type MakeDeathSaveInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller
}

// MakeDeathSave makes a death saving throw for this character.
// The character's death save state is automatically updated based on the roll.
// Returns the result including the updated state.
func (c *Character) MakeDeathSave(
	ctx context.Context, input *MakeDeathSaveInput,
) (*saves.DeathSaveResult, error) {
	// Initialize death save state if nil
	if c.deathSaveState == nil {
		c.deathSaveState = &saves.DeathSaveState{}
	}

	result, err := saves.MakeDeathSave(ctx, &saves.DeathSaveInput{
		Roller: input.Roller,
		State:  c.deathSaveState,
	})
	if err != nil {
		return nil, err
	}

	// Update the character's state with the result
	c.deathSaveState = result.State

	return result, nil
}

// TakeDamageWhileUnconsciousInput contains parameters for taking damage at 0 HP
type TakeDamageWhileUnconsciousInput struct {
	// IsCritical is true if the damage was from a critical hit (adds 2 failures instead of 1)
	IsCritical bool
}

// TakeDamageWhileUnconscious handles taking damage while at 0 HP.
// Adds 1 failure for normal damage, 2 for critical hits.
// Returns the result including the updated state.
func (c *Character) TakeDamageWhileUnconscious(
	ctx context.Context, input *TakeDamageWhileUnconsciousInput,
) (*saves.DamageWhileUnconsciousResult, error) {
	// Initialize death save state if nil
	if c.deathSaveState == nil {
		c.deathSaveState = &saves.DeathSaveState{}
	}

	result, err := saves.TakeDamageWhileUnconscious(ctx, &saves.DamageWhileUnconsciousInput{
		State:      c.deathSaveState,
		IsCritical: input.IsCritical,
	})
	if err != nil {
		return nil, err
	}

	// Update the character's state with the result
	c.deathSaveState = result.State

	return result, nil
}

// GetDeathSaveState returns the character's current death save state.
// Returns an empty state if the character has never made death saves.
func (c *Character) GetDeathSaveState() *saves.DeathSaveState {
	if c.deathSaveState == nil {
		return &saves.DeathSaveState{}
	}
	return c.deathSaveState
}

// SpendHitDiceInput contains parameters for spending hit dice during a short rest
type SpendHitDiceInput struct {
	// Count is the number of hit dice to spend (must be >= 1)
	Count int

	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	Roller dice.Roller
}

// SpendHitDiceOutput contains the result of spending hit dice
type SpendHitDiceOutput struct {
	// DiceSpent is the number of hit dice that were spent
	DiceSpent int

	// Rolls is the individual die roll results
	Rolls []int

	// CONModifier is the Constitution modifier applied per die
	CONModifier int

	// TotalHealing is the total HP healed (sum of rolls + CON mod per die)
	TotalHealing int

	// Remaining is the number of hit dice remaining after spending
	Remaining int
}

// SpendHitDice spends hit dice during a short rest to heal the character.
// Rolls the character's hit die for each die spent, adds CON modifier per die,
// and heals the character by the total amount (capped at max HP).
func (c *Character) SpendHitDice(ctx context.Context, input *SpendHitDiceInput) (*SpendHitDiceOutput, error) {
	// Validate input
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.Count < 1 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "must spend at least 1 hit die")
	}

	// Get hit dice resource
	hitDiceResource := c.GetResource(resources.HitDice)
	if hitDiceResource.IsEmpty() && hitDiceResource.Maximum() == 0 {
		return nil, rpgerr.New(rpgerr.CodeNotFound, "character has no hit dice resource configured")
	}

	// Check if we have enough hit dice
	if hitDiceResource.Current() < input.Count {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"not enough hit dice: have %d, need %d", hitDiceResource.Current(), input.Count)
	}

	// Use default roller if none provided
	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	// Roll the dice
	rolls, err := roller.RollN(ctx, input.Count, c.hitDice)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to roll hit dice")
	}

	// Calculate healing: sum of rolls + CON modifier per die
	conMod := c.GetAbilityModifier(abilities.CON)
	totalHealing := 0
	for _, roll := range rolls {
		totalHealing += roll + conMod
	}

	// Ensure minimum healing is 0 (can't heal negative even with negative CON)
	if totalHealing < 0 {
		totalHealing = 0
	}

	// Use the hit dice resource
	if err := hitDiceResource.Use(input.Count); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to use hit dice")
	}

	// Publish healing event (character's onHealingReceived will handle HP update)
	healingTopic := dnd5eEvents.HealingReceivedTopic.On(c.bus)
	err = healingTopic.Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: c.id,
		Amount:   totalHealing,
		Roll:     totalHealing - (conMod * input.Count), // Sum of dice rolls
		Modifier: conMod * input.Count,                  // Total CON modifier
		Source:   "hit_dice",
	})
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to publish healing event")
	}

	return &SpendHitDiceOutput{
		DiceSpent:    input.Count,
		Rolls:        rolls,
		CONModifier:  conMod,
		TotalHealing: totalHealing,
		Remaining:    hitDiceResource.Current(),
	}, nil
}

// ResetDeathSaveState clears the character's death save state.
// Call this when the character is healed above 0 HP or regains consciousness.
func (c *Character) ResetDeathSaveState() {
	c.deathSaveState = &saves.DeathSaveState{}
}

// LongRest performs a long rest, restoring HP to maximum and all long-rest resources.
// Also publishes RestEvent for conditions to handle their own removal if appropriate.
func (c *Character) LongRest(ctx context.Context) error {
	if c.bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character has no event bus")
	}

	// Restore HP to maximum
	c.hitPoints = c.maxHitPoints

	// Clear death save state (use empty struct for consistency with ResetDeathSaveState)
	c.deathSaveState = &saves.DeathSaveState{}

	// Directly restore all resources that reset on long rest
	for key, resource := range c.resources {
		if resource.ResetType == coreResources.ResetLongRest ||
			resource.ResetType == coreResources.ResetShortRest {
			// Hit dice have special recovery rules: regain half (minimum 1)
			if key == resources.HitDice {
				amount := resource.Maximum() / 2
				if amount < 1 {
					amount = 1
				}
				resource.Restore(amount)
			} else {
				// All other resources restore to full
				resource.RestoreToFull()
			}
		}
	}

	// Publish RestEvent for conditions to react (e.g., RagingCondition removes itself)
	restTopic := dnd5eEvents.RestTopic.On(c.bus)
	err := restTopic.Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: c.id,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish rest event")
	}

	return nil
}

// ShortRest restores resources that reset on a short rest (e.g., Second Wind, Ki).
// Unlike LongRest, ShortRest does not restore HP or clear death saves.
// Resources with ResetShortRest type are restored to full.
func (c *Character) ShortRest(ctx context.Context) error {
	if c.bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character has no event bus")
	}

	// Restore all resources that reset on short rest
	for _, resource := range c.resources {
		if resource.ResetType == coreResources.ResetShortRest {
			resource.RestoreToFull()
		}
	}

	// Publish RestEvent for conditions to react (e.g., RagingCondition removes itself)
	restTopic := dnd5eEvents.RestTopic.On(c.bus)
	err := restTopic.Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetShortRest,
		CharacterID: c.id,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish rest event")
	}

	return nil
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

// AddAction adds an action to the character's available actions.
// Implements actions.ActionHolder interface.
func (c *Character) AddAction(action actions.Action) error {
	if action == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action cannot be nil")
	}
	c.actions = append(c.actions, action)
	return nil
}

// RemoveAction removes an action by ID.
// Implements actions.ActionHolder interface.
func (c *Character) RemoveAction(actionID string) error {
	for i, a := range c.actions {
		if a.GetID() == actionID {
			c.actions = append(c.actions[:i], c.actions[i+1:]...)
			return nil
		}
	}
	return rpgerr.Newf(rpgerr.CodeNotFound, "action %s not found", actionID)
}

// GetActions returns all available actions.
// Implements actions.ActionHolder interface.
func (c *Character) GetActions() []actions.Action {
	return c.actions
}

// GetAction returns a specific action by ID, or nil if not found.
// Implements actions.ActionHolder interface.
func (c *Character) GetAction(id string) actions.Action {
	for _, a := range c.actions {
		if a.GetID() == id {
			return a
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

// ApplyDamage reduces the character's HP by the damage amount(s).
// HP cannot go below 0. Returns the result of the damage application.
//
// This method directly mutates the character's HP. The caller is responsible
// for persisting the updated character state.
//
// Implements combat.Combatant interface.
//
//nolint:revive // ctx is unused but kept for interface consistency and future use
func (c *Character) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	if input == nil {
		return &combat.ApplyDamageResult{
			CurrentHP:  c.hitPoints,
			PreviousHP: c.hitPoints,
		}
	}

	previousHP := c.hitPoints
	totalDamage := 0

	// Sum all damage instances
	for _, instance := range input.Instances {
		totalDamage += instance.Amount
	}

	// Apply damage (minimum HP is 0)
	c.hitPoints -= totalDamage
	if c.hitPoints < 0 {
		c.hitPoints = 0
	}

	c.dirty = true // Mark dirty when HP changes

	return &combat.ApplyDamageResult{
		TotalDamage:   totalDamage,
		CurrentHP:     c.hitPoints,
		DroppedToZero: c.hitPoints == 0 && previousHP > 0,
		PreviousHP:    previousHP,
	}
}

// AC returns the character's armor class.
// Implements combat.Combatant interface.
func (c *Character) AC() int {
	return c.armorClass
}

// IsDirty returns true if the character has been modified since last save.
// Implements combat.Combatant interface.
func (c *Character) IsDirty() bool {
	return c.dirty
}

// MarkClean marks the character as saved (not dirty).
// Implements combat.Combatant interface.
func (c *Character) MarkClean() {
	c.dirty = false
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

// IsResourceAvailable implements coreResources.ResourceAccessor.
// Returns true if the resource exists and has at least 1 use remaining.
func (c *Character) IsResourceAvailable(key coreResources.ResourceKey) bool {
	if c.resources == nil {
		return false
	}
	r, ok := c.resources[key]
	if !ok {
		return false
	}
	return r.IsAvailable()
}

// UseResource implements coreResources.ResourceAccessor.
// Attempts to consume the specified amount from a resource.
// Returns an error if the resource doesn't exist or has insufficient uses.
func (c *Character) UseResource(key coreResources.ResourceKey, amount int) error {
	if c.resources == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "resource %s not found", key)
	}
	r, ok := c.resources[key]
	if !ok {
		return rpgerr.Newf(rpgerr.CodeNotFound, "resource %s not found", key)
	}
	return r.Use(amount)
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
		DeathSaveState:      c.deathSaveState,
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

	// Subscribe to action removed events
	actionRemovedTopic := dnd5eEvents.ActionRemovedTopic.On(c.bus)
	subID, err = actionRemovedTopic.Subscribe(ctx, c.onActionRemoved)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to action removed")
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

// onActionRemoved handles ActionRemovedEvent
func (c *Character) onActionRemoved(_ context.Context, event dnd5eEvents.ActionRemovedEvent) error {
	// Only process events for this character
	if event.OwnerID != c.id {
		return nil
	}

	// Remove the action from our list
	_ = c.RemoveAction(event.ActionID)
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

// calculateArmorAC creates an AC component for equipped armor
func calculateArmorAC(armorItem *armor.Armor) combat.ACComponent {
	return combat.ACComponent{
		Type: combat.ACSourceArmor,
		Source: &core.Ref{
			Module: refs.Module,
			Type:   "armor",
			ID:     armorItem.ID,
		},
		Value: armorItem.AC,
	}
}

// calculateDexModifier calculates the DEX modifier to add to AC, respecting armor's MaxDexBonus cap
func (c *Character) calculateDexModifier(armorItem *armor.Armor) int {
	dexMod := c.abilityScores.Modifier(abilities.DEX)
	if armorItem != nil && armorItem.MaxDexBonus != nil {
		// Cap DEX modifier
		if dexMod > *armorItem.MaxDexBonus {
			dexMod = *armorItem.MaxDexBonus
		}
	}
	return dexMod
}

// calculateShieldAC creates an AC component for an equipped shield
func calculateShieldAC(shieldItem *armor.Armor) combat.ACComponent {
	return combat.ACComponent{
		Type: combat.ACSourceShield,
		Source: &core.Ref{
			Module: refs.Module,
			Type:   "armor",
			ID:     shieldItem.ID,
		},
		Value: shieldItem.AC,
	}
}

// EffectiveAC calculates the character's armor class with detailed breakdown
func (c *Character) EffectiveAC(ctx context.Context) *combat.ACBreakdown {
	breakdown := &combat.ACBreakdown{
		Total:      0,
		Components: []combat.ACComponent{},
	}

	// Check for equipped armor
	equippedArmor := c.GetEquippedSlot(SlotArmor)
	armorItem := equippedArmor.AsArmor()

	// Check for equipped shield (shields are armor type in off-hand)
	equippedShield := c.GetEquippedSlot(SlotOffHand)
	shieldItem := equippedShield.AsArmor()

	// Calculate base AC
	if armorItem != nil {
		// Wearing armor: use armor's AC
		breakdown.AddComponent(calculateArmorAC(armorItem))

		// Add DEX modifier, respecting armor's MaxDexBonus cap
		dexMod := c.calculateDexModifier(armorItem)
		if dexMod != 0 {
			breakdown.AddComponent(combat.ACComponent{
				Type:   combat.ACSourceAbility,
				Source: nil, // Ability modifiers don't have specific refs
				Value:  dexMod,
			})
		}
	} else {
		// Unarmored: base 10 + full DEX
		breakdown.AddComponent(combat.ACComponent{
			Type:   combat.ACSourceBase,
			Source: nil,
			Value:  10,
		})

		dexMod := c.calculateDexModifier(nil)
		if dexMod != 0 {
			breakdown.AddComponent(combat.ACComponent{
				Type:   combat.ACSourceAbility,
				Source: nil,
				Value:  dexMod,
			})
		}
	}

	// Add shield bonus if equipped
	if shieldItem != nil && shieldItem.Category == shieldCategory {
		breakdown.AddComponent(calculateShieldAC(shieldItem))
	}

	// Fire ACChain event for conditions and features to modify
	acEvent := &combat.ACChainEvent{
		CharacterID: c.id,
		Breakdown:   breakdown,
		HasArmor:    armorItem != nil,
		HasShield:   shieldItem != nil && shieldItem.Category == shieldCategory,
	}

	// Create and publish through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acTopic := combat.ACChain.On(c.bus)

	modifiedChain, err := acTopic.PublishWithChain(ctx, acEvent, acChain)
	if err == nil {
		// Execute chain to get final AC with all modifiers
		finalEvent, err := modifiedChain.Execute(ctx, acEvent)
		if err == nil {
			breakdown = finalEvent.Breakdown
		}
	}

	return breakdown
}
