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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
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

// Compile-time check that Character implements CombatAbilityHolder.
var _ combatabilities.CombatAbilityHolder = (*Character)(nil)

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

	// Features (rage, second wind, etc) grant capacity and conditions.
	features []features.Feature

	// Combat abilities (Attack, Dash, Dodge, Disengage) consume action economy to grant capacity.
	combatAbilities []combatabilities.CombatAbility

	// Conditions (raging, poisoned, stunned, etc) - passive effects
	conditions []dnd5eEvents.ConditionBehavior

	// Conditions the load parsed but no bus has seen yet, each still paired
	// with the ref its loader routed on. Attach drains this; a character that
	// was never loaded from data has none.
	pendingEffects []loadedEffect

	// What this sheet does with a persisted blob it cannot read - decided by
	// how it was loaded, and honored again when it is attached.
	policy effectPolicy

	// Death saves (tracked when at 0 HP)
	deathSaveState *saves.DeathSaveState

	// Event handling
	bus             events.EventBus
	subscriptionIDs []string
	keeper          *SheetKeeper

	// Action economy state (nil outside combat)
	actionEconomy *ActionEconomyData

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

// GetSpeed returns the character's base walking speed in feet from their race.
// This is the base speed before condition modifiers (e.g., Unarmored Movement).
// Condition-based speed modifiers are applied through the MovementChain.
func (c *Character) GetSpeed() int {
	raceData := races.GetData(c.raceID)
	if raceData == nil {
		return 30 // Default speed if race data not found
	}
	return raceData.Speed
}

// GetExtraAttacksCount returns the number of extra attacks granted by class features.
// This is used by the Attack combat ability to determine total attacks per action.
// 0 = 1 attack (normal), 1 = 2 attacks (Extra Attack), 2 = 3 attacks, etc.
func (c *Character) GetExtraAttacksCount() int {
	switch c.classID {
	case classes.Fighter:
		switch {
		case c.level >= 20:
			return 3
		case c.level >= 11:
			return 2
		case c.level >= 5:
			return 1
		}
	case classes.Barbarian, classes.Monk, classes.Paladin, classes.Ranger:
		if c.level >= 5 {
			return 1
		}
	}
	return 0
}

// GetAbilityScore returns the character's ability score (including racial modifiers)
func (c *Character) GetAbilityScore(ability abilities.Ability) int {
	return c.abilityScores[ability]
}

// GetAbilityModifier returns the modifier for an ability score
func (c *Character) GetAbilityModifier(ability abilities.Ability) int {
	return c.abilityScores.Modifier(ability)
}

// AbilityScores returns all ability scores (implements Combatant interface)
func (c *Character) AbilityScores() shared.AbilityScores {
	return c.abilityScores
}

// ProficiencyBonus returns the character's proficiency bonus (implements Combatant interface)
func (c *Character) ProficiencyBonus() int {
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

// PassivePerception returns the character's passive Perception score
// (10 + Perception skill modifier). Implements combat.Combatant.
func (c *Character) PassivePerception() int {
	return 10 + c.GetSkillModifier(skills.Perception)
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

	// The pool moved, and says so for itself rather than leaning on the healing
	// below: the hit points only get marked if the sheet's keeper is on this
	// bus, and the spent die is persisted either way.
	c.poolChanged()

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

	// Covers all three writes above — hit points, death saves, pools — in one
	// place, because a rest is one change to the sheet.
	c.poolChanged()

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

	// Unconditional rather than only when a pool actually moved: the rest event
	// published below also reaches resources and conditions that restore or
	// remove themselves, and a rest that changed nothing costs one redundant
	// write while a rest that changed something silently would cost the party
	// its ki.
	c.poolChanged()

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

// EndCombat publishes CombatEndEvent so combat-scoped conditions can remove
// themselves (RAW: rage ends when combat ends) — mirrors LongRest/ShortRest's
// RestEvent publish.
//
// DEAD, AND KNOWN TO BE: this method has no callers anywhere — not in the
// toolkit, not in rpg-api, not in a test. What actually ends a fight for its
// members is the composition noticing the fight end and announcing a
// CombatEnded boundary through its Announcer, once per member
// (rpg-project#295). This is the second publisher of a topic that finally has
// a real one, and it is a footgun besides: calling it ends ONE character's
// rage without any fight having ended.
//
// Left in place deliberately rather than deleted here. combat.TurnManager is
// in exactly the same position for the turn topics after rpg-project#294, and
// the two should go together in one pass rather than one at a time as each
// slice happens to walk past. Tracked so it is a decision rather than a
// leftover. A condition opts into combat-scoped lifetime by
// subscribing to CombatEndTopic in its own Apply (see
// RagingCondition.onCombatEnd); a condition that should outlive combat (e.g.
// a curse) simply does not subscribe, so this is not a lifetime taxonomy on
// Character — each condition decides its own scope. Unlike LongRest/ShortRest,
// EndCombat does not touch HP or resources: the encounter-end lifecycle is
// orthogonal to resting.
func (c *Character) EndCombat(ctx context.Context) error {
	if c.bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character has no event bus")
	}

	combatEndTopic := dnd5eEvents.CombatEndTopic.On(c.bus)
	err := combatEndTopic.Publish(ctx, dnd5eEvents.CombatEndEvent{
		SubjectID: c.id,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish combat end event")
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

// AddCombatAbility adds a combat ability to the character's available combat abilities.
// Implements combatabilities.CombatAbilityHolder interface.
// Implements combatabilities.CombatAbilityHolder interface.
// Idempotent: skips if an ability with the same ID is already registered.
func (c *Character) AddCombatAbility(ability combatabilities.CombatAbility) error {
	if ability == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "combat ability cannot be nil")
	}
	// Skip if already registered (same ID)
	for _, existing := range c.combatAbilities {
		if existing.GetID() == ability.GetID() {
			return nil
		}
	}
	c.combatAbilities = append(c.combatAbilities, ability)
	return nil
}

// RemoveCombatAbility removes a combat ability by ID.
// Implements combatabilities.CombatAbilityHolder interface.
func (c *Character) RemoveCombatAbility(abilityID string) error {
	for i, a := range c.combatAbilities {
		if a.GetID() == abilityID {
			c.combatAbilities = append(c.combatAbilities[:i], c.combatAbilities[i+1:]...)
			return nil
		}
	}
	return rpgerr.Newf(rpgerr.CodeNotFound, "combat ability %s not found", abilityID)
}

// GetCombatAbilities returns all available combat abilities.
// Implements combatabilities.CombatAbilityHolder interface.
func (c *Character) GetCombatAbilities() []combatabilities.CombatAbility {
	return c.combatAbilities
}

// initStandardCombatAbilities adds universal combat abilities to a character.
// Called during LoadFromData to re-register abilities that are not persisted.
func initStandardCombatAbilities(char *Character) {
	_ = char.AddCombatAbility(combatabilities.NewAttack(char.id + "-attack"))
	_ = char.AddCombatAbility(combatabilities.NewDash(char.id + "-dash"))
	_ = char.AddCombatAbility(combatabilities.NewDodge(char.id + "-dodge"))
	_ = char.AddCombatAbility(combatabilities.NewDisengage(char.id + "-disengage"))
	_ = char.AddCombatAbility(combatabilities.NewHelp(char.id + "-help"))
	_ = char.AddCombatAbility(combatabilities.NewHide(char.id + "-hide"))
}

// GetCombatAbility returns a specific combat ability by ID, or nil if not found.
// Implements combatabilities.CombatAbilityHolder interface.
func (c *Character) GetCombatAbility(id string) combatabilities.CombatAbility {
	for _, a := range c.combatAbilities {
		if a.GetID() == id {
			return a
		}
	}
	return nil
}

// ActivateCombatAbility finds and activates a combat ability by its ref.
// Implements combat.CombatCharacter interface.
func (c *Character) ActivateCombatAbility(ctx context.Context, input *combat.ActivateAbilityInput) error {
	if input == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "ActivateAbilityInput is nil")
	}
	if input.AbilityRef == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "AbilityRef is required")
	}

	// Find the ability by matching its ref (module/type/id), not the instance ID
	var ability combatabilities.CombatAbility
	for _, a := range c.combatAbilities {
		aRef := a.Ref()
		if aRef != nil &&
			aRef.Module == input.AbilityRef.Module &&
			aRef.Type == input.AbilityRef.Type &&
			aRef.ID == input.AbilityRef.ID {
			ability = a
			break
		}
	}
	if ability == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "combat ability ref %s/%s/%s not found",
			input.AbilityRef.Module, input.AbilityRef.Type, input.AbilityRef.ID)
	}

	// Build CombatAbilityInput from ActivateAbilityInput
	abilityInput := combatabilities.CombatAbilityInput{
		Bus:           input.Bus,
		ActionEconomy: input.Economy,
		Speed:         input.Speed,
		ExtraAttacks:  input.ExtraAttacks,
	}

	if err := ability.CanActivate(ctx, c, abilityInput); err != nil {
		return err
	}

	return ability.Activate(ctx, c, abilityInput)
}

// GetAbilityInfos returns metadata about all combat abilities on this character.
// Implements combat.CombatCharacter interface.
func (c *Character) GetAbilityInfos() []combat.AbilityInfo {
	infos := make([]combat.AbilityInfo, 0, len(c.combatAbilities))
	for _, a := range c.combatAbilities {
		infos = append(infos, combat.AbilityInfo{
			Ref:        a.Ref(),
			Name:       a.Name(),
			ActionType: a.ActionType(),
		})
	}
	return infos
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

// poolChanged records that a write landed on the resource pools or on the state
// that moves with them.
//
// GetResourceData feeds Data.Resources, so a spent ki point or a restored hit
// die is state ToData writes — and, like the action economy, state that only
// reaches storage if the sheet reports IsDirty() (#1087). The rests call it for
// everything they touch at once, hit points and death saves included, because a
// rest is one change to the sheet rather than three.
//
// The load paths deliberately do not call this: loadResources and
// LoadResourceData rebuild pools at the values they were read from, so the
// sheet already matches storage, and marking there would make every loaded
// sheet write itself back over what it was read from.
func (c *Character) poolChanged() {
	c.dirty = true
}

// MarkClean marks the character as saved (not dirty).
// Implements combat.Combatant interface.
func (c *Character) MarkClean() {
	c.dirty = false
}

// MarkDirty records that something changed on this sheet that has to be
// persisted.
//
// This is the write half of an effect's owner handle. A condition keeps its
// own turn-scoped memory in its own fields — RagingCondition.WasHitThisTurn,
// SneakAttackCondition.UsedThisTurn — and those fields are serialized as part
// of THIS character. Nothing else notices when one of them changes, and
// resolution.Resolve hands back only participants that report IsDirty, so a
// condition that updates itself and says nothing has its update thrown away.
//
// Until now that survived on luck: the rogue who spent a sneak attack also
// paid an action, and paying an action set the flag. An interaction whose
// whole purpose is condition state — ending a turn, ending a fight, taking a
// rest — has no such accident to rely on.
//
// Deliberately marking rather than diffing: a condition that mutates and does
// not call this is an ordinary local bug, and a test catches it. That is not
// the same as a misconfiguration, where identical code behaves differently
// depending on whether something ran elsewhere; those earn a structural
// guarantee, and this does not. The snapshot-diff that would make forgetting
// impossible belongs in the test harness, and lives there.
func (c *Character) MarkDirty() {
	c.dirty = true
}

// emptyResource is returned when a resource doesn't exist.
// It has 0 maximum and 0 current, so IsEmpty() returns true.
var emptyResource = combat.NewRecoverableResource(combat.RecoverableResourceConfig{
	ID:      "",
	Maximum: 0,
})

// ResourceStatus implements features.ResourceReader by reading the character's
// actual resources map with presence. It never falls back to GetResource's
// absent-key zero value: a key the sheet does not carry reports ok=false so a
// feature's Status (and the StatusView projection) can tell a missing pool
// from an empty one and fail loudly rather than silently reporting zeros.
func (c *Character) ResourceStatus(key coreResources.ResourceKey) (int, int, bool) {
	if c.resources == nil {
		return 0, 0, false
	}
	r, ok := c.resources[key]
	if !ok {
		return 0, 0, false
	}
	return r.Current(), r.Maximum(), true
}

// GetResource returns the resource for the given key.
// If the resource doesn't exist, returns an empty resource (not nil).
// Use IsEmpty() to check if the resource exists and has uses available.
//
// The pool comes back live, not copied. Reading it is free; spending through it
// mutates the sheet without the sheet noticing, and the spend is then dropped
// on the next write-back. Spend through UseResource, which marks the sheet
// dirty (#1087).
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
	c.poolChanged()
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
	if err := r.Use(amount); err != nil {
		return err
	}

	// The choke point every feature spend goes through — rage charges, ki for
	// Flurry of Blows, Patient Defense, Step of the Wind — so this one line is
	// what makes those spends survive the write-back.
	c.poolChanged()

	return nil
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
//
// A load path, so it does not mark the sheet dirty: the pools come back at the
// values they were read from, and a sheet that reported itself dirty for
// having been read would write those values straight back over storage.
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

// HasShieldEquipped reports whether a shield occupies either hand slot.
//
// A static equipment fact a condition like Protection ("you must be
// wielding a shield") reads off its own live sheet at fold time — part of
// rpg-toolkit#1178's move away from a context-installed gamectx registry.
// Checking both hands rather than assuming off-hand-only matches
// GetEquippedSlot's own indifference to which hand a caller names.
func (c *Character) HasShieldEquipped() bool {
	for _, slot := range [...]InventorySlot{SlotMainHand, SlotOffHand} {
		equipped := c.GetEquippedSlot(slot)
		if equipped == nil {
			continue
		}
		if a := equipped.AsArmor(); a != nil && a.Category == armor.CategoryShield {
			return true
		}
	}
	return false
}

// EquipItem equips an inventory item to the specified slot, enforcing
// two-handed weapon occupancy (rpg-toolkit#811): a two-handed weapon
// always claims main hand and clears the off hand; equipping into the off
// hand while a two-handed weapon is held frees the main hand for the new
// item. Equipping into an occupied slot overwrites the slot mapping — the
// previous occupant remains in inventory, simply no longer equipped.
// Returns error if the item is not in inventory or cannot occupy the slot.
func (c *Character) EquipItem(slot InventorySlot, itemID string) error {
	// Verify item exists in inventory and count how many copies are owned.
	var item equipment.Equipment
	copies := 0
	found := false
	for _, invItem := range c.inventory {
		if invItem.Equipment.EquipmentID() == itemID {
			if !found {
				item = invItem.Equipment
				found = true
			}
			copies += invItem.Quantity
		}
	}

	if !found {
		return rpgerr.New(rpgerr.CodeNotFound, "item not found in inventory")
	}

	if !equipmentFitsSlot(item, slot) {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "item cannot be equipped in that slot",
			rpgerr.WithMeta("item_id", itemID),
			rpgerr.WithMeta("slot", slot))
	}

	// Initialize map if nil
	if c.equipmentSlots == nil {
		c.equipmentSlots = make(EquipmentSlots)
	}

	// Only vacate the old slot when the character owns a single copy.
	// Duplicate equippable items (daggers, handaxes, etc.) need both
	// copies to remain equip-able at the same time; a one-copy weapon still
	// moves between slots as before.
	if copies <= 1 {
		for s, id := range c.equipmentSlots {
			if id == itemID {
				c.equipmentSlots.Clear(s)
			}
		}
	}

	// A two-handed weapon claims main hand and forces the off hand empty.
	// equipmentFitsSlot above already limits two-handed weapons to main
	// hand, so slot == SlotMainHand whenever this branch runs.
	if isTwoHanded(item) {
		c.equipmentSlots.Clear(SlotOffHand)
		c.equipmentSlots.Set(SlotMainHand, itemID)
		return nil
	}

	// Main hand holding a two-handed weapon blocks the off hand until
	// something is equipped there, which frees the main hand.
	if slot == SlotOffHand {
		if mainHand := c.GetEquippedSlot(SlotMainHand); mainHand != nil && isTwoHanded(mainHand.Item) {
			c.equipmentSlots.Clear(SlotMainHand)
		}
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

	// Deep-copy action economy state (nil outside combat) to avoid sharing mutable maps
	if c.actionEconomy != nil {
		aeCopy := *c.actionEconomy
		if c.actionEconomy.Granted != nil {
			aeCopy.Granted = maps.Clone(c.actionEconomy.Granted)
		}
		data.ActionEconomy = &aeCopy
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

// subscribeToEvents subscribes the character to gameplay events on the bus it
// is already holding.
//
// The subscriptions are the [SheetKeeper]'s — this is the same five handlers,
// reached the way a character that was built rather than loaded reaches them:
// Draft.Finalize wires the bus into the sheet and then asks it to listen.
// Deliberately not the keeper's full Apply, which would also put the
// character's resources on the bus; see SheetKeeper.subscribeSelf.
func (c *Character) subscribeToEvents(ctx context.Context) error {
	if c.bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character has no event bus")
	}

	return c.SheetKeeper().subscribeSelf(ctx, c.bus)
}

// onConditionApplied handles ConditionAppliedEvent.
//
// bus is the one the sheet was attached to, handed down from the
// [SheetKeeper] rather than read off the character: a purely loaded sheet has
// no bus of its own, and one that does must not answer on a different bus than
// the one that delivered the event.
func (c *Character) onConditionApplied(
	ctx context.Context, bus events.EventBus, event dnd5eEvents.ConditionAppliedEvent,
) error {
	// Only process events for this character
	if event.Target.GetID() != c.id {
		return nil
	}

	// Apply the condition (subscribes to events)
	if err := event.Condition.Apply(ctx, bus); err != nil {
		// Clean up any partial subscriptions to avoid resource leaks
		_ = event.Condition.Remove(ctx, bus)
		return rpgerr.Wrapf(err, "failed to apply condition")
	}

	// Store the condition
	c.conditions = append(c.conditions, event.Condition)

	// ToData serializes conditions, so a sheet that gained one and did not go
	// dirty is a sheet whose new condition never gets written down.
	c.dirty = true

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

	if len(filtered) != len(c.conditions) {
		// Same reason as onConditionApplied: what ToData writes has changed.
		c.dirty = true
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

	// HP is persisted, and ApplyDamage marks dirty for the same reason.
	c.dirty = true

	return nil
}

// Cleanup unsubscribes from all events and removes all active conditions.
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

// dropCondition takes a condition back off the sheet by identity.
//
// By identity rather than by ref because this runs when a condition failed to
// apply, and the sheet may legitimately carry two conditions with the same ref
// — only the one instance that failed should leave.
func (c *Character) dropCondition(behavior dnd5eEvents.ConditionBehavior) {
	for i, cond := range c.conditions {
		if cond == behavior {
			c.conditions = append(c.conditions[:i], c.conditions[i+1:]...)
			return
		}
	}
}

// forgetSubscriptions drops the given subscription IDs from the character's
// list, so that a Cleanup after a [SheetKeeper.Remove] does not try to revoke
// hooks that are already gone.
func (c *Character) forgetSubscriptions(revoked []string) {
	if len(revoked) == 0 || len(c.subscriptionIDs) == 0 {
		return
	}

	gone := make(map[string]struct{}, len(revoked))
	for _, id := range revoked {
		gone[id] = struct{}{}
	}

	kept := make([]string, 0, len(c.subscriptionIDs))
	for _, id := range c.subscriptionIDs {
		if _, ok := gone[id]; !ok {
			kept = append(kept, id)
		}
	}
	c.subscriptionIDs = kept
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

// EffectiveAC calculates the character's armor class with detailed breakdown.
//
// # It refuses rather than guesses
//
// The fold rides the bus parked on the sheet, so a sheet that was never
// attached has no subscribers and every AC contributor is silently absent —
// Unarmored Defense, the fighting styles, Shield. The number that comes back
// from that is not a smaller answer, it is a WRONG one, and it is wrong in the
// most plausible direction there is: exactly base armour, which reads like a
// character who simply has no features.
//
// That is how a monk fought at 10+DEX with Unarmored Defense attached and
// nobody noticed, and it is the same shape as rpg-api#842. So an unattached
// sheet is an error here, not a fallback. Chain failures are returned for the
// same reason: this used to swallow both the publish and the execute error and
// return whatever the breakdown happened to hold, which meant a broken
// contributor degraded the total instead of failing the read.
//
// Callers holding a sheet from the bus-free [Load] must [Attach] it before
// asking. A stat block that has no chain to fold wants [Character.AC].
func (c *Character) EffectiveAC(ctx context.Context) (*combat.ACBreakdown, error) {
	if c.bus == nil {
		return nil, rpgerr.New(rpgerr.CodePrerequisiteNotMet,
			"effective AC needs an attached sheet: this character is on no bus, "+
				"so every condition and feature that contributes AC is absent")
	}

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
				Source: refs.Abilities.Dexterity(),
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
				Source: refs.Abilities.Dexterity(),
				Value:  dexMod,
			})
		}
	}

	// Add shield bonus if equipped
	if shieldItem != nil && shieldItem.Category == armor.CategoryShield {
		breakdown.AddComponent(calculateShieldAC(shieldItem))
	}

	// Fire ACChain event for conditions and features to modify
	acEvent := &combat.ACChainEvent{
		CharacterID: c.id,
		Breakdown:   breakdown,
		HasArmor:    armorItem != nil,
		HasShield:   shieldItem != nil && shieldItem.Category == armor.CategoryShield,
	}

	// Create and publish through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acTopic := combat.ACChain.On(c.bus)

	modifiedChain, err := acTopic.PublishWithChain(ctx, acEvent, acChain)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "publish AC chain for character %s", c.id)
	}

	finalEvent, err := modifiedChain.Execute(ctx, acEvent)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "fold AC chain for character %s", c.id)
	}

	return finalEvent.Breakdown, nil
}
