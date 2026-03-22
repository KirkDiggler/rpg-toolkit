package character

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// GetActionEconomy returns the current action economy data, or nil if not in combat.
func (c *Character) GetActionEconomy() *ActionEconomyData {
	return c.actionEconomy
}

// InCombat returns true if the character is currently in combat.
// Combat is indicated by the action economy being initialized (non-nil).
func (c *Character) InCombat() bool {
	return c.actionEconomy != nil
}

// ExitCombat clears the action economy entirely, removing combat state.
// Call this when the encounter ends, not between turns.
func (c *Character) ExitCombat(_ context.Context, _ *ExitCombatInput) (*ExitCombatOutput, error) {
	c.actionEconomy = nil
	return &ExitCombatOutput{}, nil
}

// StartTurn initializes the action economy for a new turn.
// Sets 1 action, 1 bonus action, 1 reaction, and movement from input speed.
// Returns the available abilities and actions for this turn.
func (c *Character) StartTurn(_ context.Context, input *StartTurnInput) (*StartTurnOutput, error) {
	c.actionEconomy = &ActionEconomyData{
		TurnNumber:            input.TurnNumber,
		ActionsRemaining:      1,
		BonusActionsRemaining: 1,
		ReactionsRemaining:    1,
		MovementRemaining:     input.Speed,
		Granted:               make(map[GrantedActionKey]int),
	}

	return &StartTurnOutput{
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// EndTurn resets action economy resources to 0 and clears granted capacity.
// The character remains in combat (actionEconomy stays non-nil).
func (c *Character) EndTurn(_ context.Context, _ *EndTurnInput) (*EndTurnOutput, error) {
	if c.actionEconomy != nil {
		c.actionEconomy.ActionsRemaining = 0
		c.actionEconomy.BonusActionsRemaining = 0
		c.actionEconomy.ReactionsRemaining = 0
		c.actionEconomy.MovementRemaining = 0
		c.actionEconomy.Granted = make(map[GrantedActionKey]int)
	}
	return &EndTurnOutput{}, nil
}

// AvailableAbilities returns the list of abilities the character can potentially use.
// Returns an empty slice if not in combat.
func (c *Character) AvailableAbilities() []AvailableAbility {
	if !c.InCombat() {
		return []AvailableAbility{}
	}
	return c.buildAvailableAbilities()
}

// AvailableActions returns the list of actions the character can potentially take.
// Returns an empty slice if not in combat.
func (c *Character) AvailableActions() []AvailableAction {
	if !c.InCombat() {
		return []AvailableAction{}
	}
	return c.buildAvailableActions()
}

// ActivateAbility activates a combat ability or feature by ref.
// Routes to the appropriate handler based on whether the ref matches a combat ability or feature.
// Returns success=false with an error message if activation fails.
func (c *Character) ActivateAbility(_ context.Context, input *ActivateAbilityInput) (*ActivateAbilityOutput, error) {
	if !c.InCombat() {
		return &ActivateAbilityOutput{
			Success: false,
			Error:   "not in combat",
		}, nil
	}

	// Try combat abilities first
	for _, ca := range c.combatAbilities {
		if ca.Ref().ID == input.AbilityRef.ID {
			return c.activateCombatAbility(ca, input)
		}
	}

	// Try features
	for _, f := range c.features {
		if f.Ref().ID == input.AbilityRef.ID {
			return c.activateFeature(f, input)
		}
	}

	return &ActivateAbilityOutput{
		Success: false,
		Error:   "unknown ability",
	}, nil
}

// ExecuteAction executes an action that consumes granted capacity.
// Routes to the appropriate handler based on the action ref.
// Returns success=false if the action cannot be executed.
func (c *Character) ExecuteAction(_ context.Context, input *ExecuteActionInput) (*ExecuteActionOutput, error) {
	if !c.InCombat() {
		return &ExecuteActionOutput{
			Success: false,
			Error:   "not in combat",
		}, nil
	}

	switch input.ActionRef.ID {
	case refs.Actions.Strike().ID:
		return c.executeStrike()
	case refs.Actions.OffHandStrike().ID:
		return c.executeOffHandStrike()
	case refs.Actions.FlurryStrike().ID:
		return c.executeFlurryStrike()
	case refs.Actions.UnarmedStrike().ID:
		return c.executeUnarmedStrike()
	case refs.Actions.Move().ID:
		return c.executeMove()
	default:
		return &ExecuteActionOutput{
			Success: false,
			Error:   "unknown action",
		}, nil
	}
}

// GrantCapacity grants a specified amount of capacity for a given key.
// Used by external systems to grant additional capacity (e.g., Action Surge granting extra attacks).
func (c *Character) GrantCapacity(key GrantedActionKey, amount int) {
	if c.actionEconomy == nil {
		return
	}
	if c.actionEconomy.Granted == nil {
		c.actionEconomy.Granted = make(map[GrantedActionKey]int)
	}
	c.actionEconomy.Granted[key] += amount
}

// HasGranted returns whether the character has any remaining capacity for the given key.
func (c *Character) HasGranted(key GrantedActionKey) bool {
	if c.actionEconomy == nil || c.actionEconomy.Granted == nil {
		return false
	}
	return c.actionEconomy.Granted[key] > 0
}

// --- Available builders ---

// buildAvailableAbilities builds the list of available abilities from combat abilities and features.
func (c *Character) buildAvailableAbilities() []AvailableAbility {
	result := make([]AvailableAbility, 0, len(c.combatAbilities)+len(c.features))

	// Combat abilities
	for _, ca := range c.combatAbilities {
		canUse := c.canUseAbilityByActionType(ca.ActionType())
		reason := c.actionTypeExhaustedReason(ca.ActionType())

		result = append(result, AvailableAbility{
			Ref:        ca.Ref(),
			Name:       ca.Name(),
			ActionType: ca.ActionType(),
			CanUse:     canUse,
			Reason:     c.actionReason(canUse, reason),
		})
	}

	// Features
	for _, f := range c.features {
		canUse := c.canUseAbilityByActionType(f.ActionType())
		reason := c.actionTypeExhaustedReason(f.ActionType())

		// Check feature-specific resource availability
		if canUse {
			if err := f.CanActivate(context.Background(), c, features.FeatureInput{}); err != nil {
				canUse = false
				reason = err.Error()
			}
		}

		current, max := c.featureResourceInfo(f)

		result = append(result, AvailableAbility{
			Ref:             f.Ref(),
			Name:            f.Name(),
			ActionType:      f.ActionType(),
			CanUse:          canUse,
			Reason:          c.actionReason(canUse, reason),
			ResourceCurrent: current,
			ResourceMax:     max,
		})
	}

	// Check for equipment-based Off-Hand Attack
	if c.hasTwoLightWeapons() {
		canUse := c.canUseAbilityByActionType(coreCombat.ActionBonus)
		reason := c.actionTypeExhaustedReason(coreCombat.ActionBonus)
		result = append(result, AvailableAbility{
			Ref:        refs.CombatAbilities.OffHandAttack(),
			Name:       "Off-Hand Attack",
			ActionType: coreCombat.ActionBonus,
			CanUse:     canUse,
			Reason:     c.actionReason(canUse, reason),
		})
	}

	return result
}

// buildAvailableActions builds the list of available actions from granted capacity.
func (c *Character) buildAvailableActions() []AvailableAction {
	result := make([]AvailableAction, 0)

	// Move is always listed
	moveCanUse := c.actionEconomy.MovementRemaining > 0
	result = append(result, AvailableAction{
		Ref:    refs.Actions.Move(),
		Name:   "Move",
		CanUse: moveCanUse,
		Reason: c.actionReason(moveCanUse, "no movement remaining"),
	})

	// Strike: listed if attacks granted
	if c.actionEconomy.Granted[GrantedAttacks] > 0 {
		result = append(result, AvailableAction{
			Ref:    refs.Actions.Strike(),
			Name:   "Strike",
			CanUse: true,
		})
	}

	// Off-Hand Strike: listed if granted
	if c.actionEconomy.Granted[GrantedOffHandStrikes] > 0 {
		result = append(result, AvailableAction{
			Ref:    refs.Actions.OffHandStrike(),
			Name:   "Off-Hand Strike",
			CanUse: true,
		})
	}

	// Flurry Strike: listed if granted
	if c.actionEconomy.Granted[GrantedFlurryStrikes] > 0 {
		result = append(result, AvailableAction{
			Ref:    refs.Actions.FlurryStrike(),
			Name:   "Flurry Strike",
			CanUse: true,
		})
	}

	// Unarmed Strike (Martial Arts Bonus): listed if granted
	if c.actionEconomy.Granted[GrantedMartialArtsBonus] > 0 {
		result = append(result, AvailableAction{
			Ref:    refs.Actions.UnarmedStrike(),
			Name:   "Unarmed Strike",
			CanUse: true,
		})
	}

	return result
}

// --- Activate helpers ---

// activateCombatAbility uses the bridge pattern to activate a combat ability.
// Converts ActionEconomyData to toolkit ActionEconomy, calls the ability, then syncs back.
func (c *Character) activateCombatAbility(ca combatabilities.CombatAbility, _ *ActivateAbilityInput) (*ActivateAbilityOutput, error) {
	ae := c.toToolkitActionEconomy()

	ctx := context.Background()
	input := combatabilities.CombatAbilityInput{
		Bus:           c.bus,
		ActionEconomy: ae,
		Speed:         c.GetSpeed(),
		ExtraAttacks:  c.GetExtraAttacksCount(),
	}

	// Check if ability can be activated
	if err := ca.CanActivate(ctx, c, input); err != nil {
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	// Activate
	if err := ca.Activate(ctx, c, input); err != nil {
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	// Sync toolkit action economy back to our data
	c.fromToolkitActionEconomy(ae)

	return &ActivateAbilityOutput{
		Success:         true,
		GrantedCapacity: c.describeGrantedCapacity(ca),
		Abilities:       c.buildAvailableAbilities(),
		Actions:         c.buildAvailableActions(),
	}, nil
}

// activateFeature directly manages action economy for feature activation.
// Features manage their own resources (charges) but the Character manages action economy consumption.
func (c *Character) activateFeature(f features.Feature, _ *ActivateAbilityInput) (*ActivateAbilityOutput, error) {
	// Check action economy
	if !c.canUseAbilityByActionType(f.ActionType()) {
		reason := c.actionTypeExhaustedReason(f.ActionType())
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     reason,
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	// Check feature-specific requirements
	ctx := context.Background()
	if err := f.CanActivate(ctx, c, features.FeatureInput{}); err != nil {
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	// Consume action economy
	c.consumeActionType(f.ActionType())

	// Activate the feature
	if err := f.Activate(ctx, c, features.FeatureInput{Bus: c.bus}); err != nil {
		// Rollback action economy on failure
		c.restoreActionType(f.ActionType())
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	return &ActivateAbilityOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// --- Execute helpers ---

// executeStrike decrements granted attacks and checks for post-strike grants.
func (c *Character) executeStrike() (*ExecuteActionOutput, error) {
	attacks := c.actionEconomy.Granted[GrantedAttacks]
	if attacks <= 0 {
		return &ExecuteActionOutput{
			Success:   false,
			Error:     "no attacks remaining",
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	c.actionEconomy.Granted[GrantedAttacks]--
	c.checkPostStrikeGrants()

	return &ExecuteActionOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// executeOffHandStrike decrements granted off-hand strikes.
func (c *Character) executeOffHandStrike() (*ExecuteActionOutput, error) {
	if c.actionEconomy.Granted[GrantedOffHandStrikes] <= 0 {
		return &ExecuteActionOutput{
			Success:   false,
			Error:     "no off-hand strikes remaining",
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	c.actionEconomy.Granted[GrantedOffHandStrikes]--

	return &ExecuteActionOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// executeFlurryStrike decrements granted flurry strikes.
func (c *Character) executeFlurryStrike() (*ExecuteActionOutput, error) {
	if c.actionEconomy.Granted[GrantedFlurryStrikes] <= 0 {
		return &ExecuteActionOutput{
			Success:   false,
			Error:     "no flurry strikes remaining",
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	c.actionEconomy.Granted[GrantedFlurryStrikes]--

	return &ExecuteActionOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// executeUnarmedStrike decrements granted martial arts bonus strikes.
func (c *Character) executeUnarmedStrike() (*ExecuteActionOutput, error) {
	if c.actionEconomy.Granted[GrantedMartialArtsBonus] <= 0 {
		return &ExecuteActionOutput{
			Success:   false,
			Error:     "no martial arts bonus strikes remaining",
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	c.actionEconomy.Granted[GrantedMartialArtsBonus]--

	return &ExecuteActionOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// executeMove handles movement (placeholder - always succeeds if movement remaining).
func (c *Character) executeMove() (*ExecuteActionOutput, error) {
	if c.actionEconomy.MovementRemaining <= 0 {
		return &ExecuteActionOutput{
			Success:   false,
			Error:     "no movement remaining",
			Abilities: c.buildAvailableAbilities(),
			Actions:   c.buildAvailableActions(),
		}, nil
	}

	// Movement amount would be specified by the caller in a real implementation.
	// For now, this just validates that movement is available.

	return &ExecuteActionOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
		Actions:   c.buildAvailableActions(),
	}, nil
}

// checkPostStrikeGrants checks for post-main-hand-strike grants.
// After a main-hand strike, monks get a martial arts bonus strike
// and two-weapon fighters get an off-hand strike.
func (c *Character) checkPostStrikeGrants() {
	// Monk: grant martial arts bonus if bonus action available and not already granted
	if c.classID == classes.Monk &&
		c.actionEconomy.Granted[GrantedMartialArtsBonus] == 0 &&
		c.actionEconomy.BonusActionsRemaining > 0 {
		c.actionEconomy.Granted[GrantedMartialArtsBonus] = 1
	}

	// Two-weapon fighting: grant off-hand strike if bonus action available and not already granted
	if c.hasTwoLightWeapons() &&
		c.actionEconomy.Granted[GrantedOffHandStrikes] == 0 &&
		c.actionEconomy.BonusActionsRemaining > 0 {
		c.actionEconomy.Granted[GrantedOffHandStrikes] = 1
	}
}

// --- Bridge methods ---

// toToolkitActionEconomy converts ActionEconomyData to the toolkit's combat.ActionEconomy.
// This bridges our serializable data with the toolkit's combat ability system.
func (c *Character) toToolkitActionEconomy() *combat.ActionEconomy {
	ae := &combat.ActionEconomy{
		ActionsRemaining:      c.actionEconomy.ActionsRemaining,
		BonusActionsRemaining: c.actionEconomy.BonusActionsRemaining,
		ReactionsRemaining:    c.actionEconomy.ReactionsRemaining,
		MovementRemaining:     c.actionEconomy.MovementRemaining,
	}

	// Map granted capacity
	if attacks, ok := c.actionEconomy.Granted[GrantedAttacks]; ok {
		ae.AttacksRemaining = attacks
	}
	if offHand, ok := c.actionEconomy.Granted[GrantedOffHandStrikes]; ok {
		ae.OffHandAttacksRemaining = offHand
	}
	if flurry, ok := c.actionEconomy.Granted[GrantedFlurryStrikes]; ok {
		ae.FlurryStrikesRemaining = flurry
	}

	return ae
}

// fromToolkitActionEconomy syncs the toolkit's combat.ActionEconomy back to ActionEconomyData.
// Called after a combat ability modifies the toolkit ActionEconomy.
func (c *Character) fromToolkitActionEconomy(ae *combat.ActionEconomy) {
	c.actionEconomy.ActionsRemaining = ae.ActionsRemaining
	c.actionEconomy.BonusActionsRemaining = ae.BonusActionsRemaining
	c.actionEconomy.ReactionsRemaining = ae.ReactionsRemaining
	c.actionEconomy.MovementRemaining = ae.MovementRemaining

	// Sync granted capacity back
	if ae.AttacksRemaining > 0 {
		c.actionEconomy.Granted[GrantedAttacks] = ae.AttacksRemaining
	}
	if ae.OffHandAttacksRemaining > 0 {
		c.actionEconomy.Granted[GrantedOffHandStrikes] = ae.OffHandAttacksRemaining
	}
	if ae.FlurryStrikesRemaining > 0 {
		c.actionEconomy.Granted[GrantedFlurryStrikes] = ae.FlurryStrikesRemaining
	}
}

// --- Helper methods ---

// canUseAbilityByActionType checks if the character has the action economy resource
// for the given action type.
func (c *Character) canUseAbilityByActionType(actionType coreCombat.ActionType) bool {
	switch actionType {
	case coreCombat.ActionStandard:
		return c.actionEconomy.ActionsRemaining > 0
	case coreCombat.ActionBonus:
		return c.actionEconomy.BonusActionsRemaining > 0
	case coreCombat.ActionReaction:
		return c.actionEconomy.ReactionsRemaining > 0
	case coreCombat.ActionFree:
		return true
	default:
		return false
	}
}

// actionTypeExhaustedReason returns a human-readable reason for why an action type is exhausted.
func (c *Character) actionTypeExhaustedReason(actionType coreCombat.ActionType) string {
	switch actionType {
	case coreCombat.ActionStandard:
		return "no action remaining"
	case coreCombat.ActionBonus:
		return "no bonus action remaining"
	case coreCombat.ActionReaction:
		return "no reaction remaining"
	default:
		return ""
	}
}

// actionReason returns the reason string only if canUse is false.
func (c *Character) actionReason(canUse bool, reason string) string {
	if canUse {
		return ""
	}
	return reason
}

// consumeActionType decrements the appropriate action economy counter.
func (c *Character) consumeActionType(actionType coreCombat.ActionType) {
	switch actionType {
	case coreCombat.ActionStandard:
		c.actionEconomy.ActionsRemaining--
	case coreCombat.ActionBonus:
		c.actionEconomy.BonusActionsRemaining--
	case coreCombat.ActionReaction:
		c.actionEconomy.ReactionsRemaining--
	}
}

// restoreActionType increments the appropriate action economy counter (rollback).
func (c *Character) restoreActionType(actionType coreCombat.ActionType) {
	switch actionType {
	case coreCombat.ActionStandard:
		c.actionEconomy.ActionsRemaining++
	case coreCombat.ActionBonus:
		c.actionEconomy.BonusActionsRemaining++
	case coreCombat.ActionReaction:
		c.actionEconomy.ReactionsRemaining++
	}
}

// describeGrantedCapacity returns a human-readable description of what was granted
// by activating a combat ability.
func (c *Character) describeGrantedCapacity(ca combatabilities.CombatAbility) string {
	switch ca.Ref().ID {
	case refs.CombatAbilities.Attack().ID:
		attacks := c.actionEconomy.Granted[GrantedAttacks]
		if attacks == 1 {
			return "1 attack"
		}
		return fmt.Sprintf("%d attacks", attacks)
	case refs.CombatAbilities.Dash().ID:
		return fmt.Sprintf("%dft movement", c.GetSpeed())
	case refs.CombatAbilities.Dodge().ID:
		return "dodging until next turn"
	case refs.CombatAbilities.Disengage().ID:
		return "disengaging until next turn"
	default:
		return ""
	}
}

// featureResourceInfo returns the current and max resource values for a feature.
// Maps feature refs to their corresponding resource keys on the character.
func (c *Character) featureResourceInfo(f features.Feature) (current, max int) {
	ref := f.Ref()
	if ref == nil {
		return 0, 0
	}

	var key *core.Ref

	switch ref.ID {
	case refs.Features.Rage().ID:
		r := c.GetResource(resources.RageCharges)
		return r.Current(), r.Maximum()
	case refs.Features.SecondWind().ID:
		// SecondWind manages its own resource internally
		return 0, 0
	case refs.Features.FlurryOfBlows().ID, refs.Features.PatientDefense().ID, refs.Features.StepOfTheWind().ID:
		r := c.GetResource(resources.Ki)
		return r.Current(), r.Maximum()
	}

	_ = key
	return 0, 0
}

// hasTwoLightWeapons checks if the character has light weapons in both hands.
func (c *Character) hasTwoLightWeapons() bool {
	mainHand := c.GetEquippedSlot(SlotMainHand)
	offHand := c.GetEquippedSlot(SlotOffHand)

	if mainHand == nil || offHand == nil {
		return false
	}

	mainWeapon := mainHand.AsWeapon()
	offWeapon := offHand.AsWeapon()

	if mainWeapon == nil || offWeapon == nil {
		return false
	}

	return mainWeapon.HasProperty(weapons.PropertyLight) && offWeapon.HasProperty(weapons.PropertyLight)
}
