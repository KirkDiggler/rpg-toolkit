// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import "github.com/KirkDiggler/rpg-toolkit/rpgerr"

// ActionEconomy tracks the available actions, bonus actions, and reactions for a combatant
// Purpose: Manages the action economy system for D&D 5e combat, ensuring combatants can only
// take actions they have available in their turn.
type ActionEconomy struct {
	// Primary resources (consumed by abilities/features)
	ActionsRemaining      int // Usually 1, Action Surge gives +1
	BonusActionsRemaining int // Usually 1
	ReactionsRemaining    int // Usually 1

	// Capacity (set when specific abilities are used)
	AttacksRemaining  int // Set when Attack ability is taken (stays 0 until then)
	MovementRemaining int // Set at turn start from character speed

	// Additional capacity for granted actions
	OffHandAttacksRemaining int // Set by TwoWeaponGranter after main-hand attack
	FlurryStrikesRemaining  int // Set by FlurryOfBlows feature (usually 2)
}

// NewActionEconomy creates a new ActionEconomy with default values (1/1/1)
// Purpose: Standard constructor for initializing a combatant's action economy at the start of their turn
func NewActionEconomy() *ActionEconomy {
	return &ActionEconomy{
		ActionsRemaining:      1,
		BonusActionsRemaining: 1,
		ReactionsRemaining:    1,
	}
}

// CanUseAction returns whether an action is available
// Purpose: Allows checking action availability without consuming it
func (ae *ActionEconomy) CanUseAction() bool {
	return ae.ActionsRemaining > 0
}

// CanUseBonusAction returns whether a bonus action is available
// Purpose: Allows checking bonus action availability without consuming it
func (ae *ActionEconomy) CanUseBonusAction() bool {
	return ae.BonusActionsRemaining > 0
}

// CanUseReaction returns whether a reaction is available
// Purpose: Allows checking reaction availability without consuming it
func (ae *ActionEconomy) CanUseReaction() bool {
	return ae.ReactionsRemaining > 0
}

// UseAction consumes an action if available
// Returns CodeResourceExhausted if no actions remain
func (ae *ActionEconomy) UseAction() error {
	if ae.ActionsRemaining <= 0 {
		return rpgerr.ResourceExhausted("action")
	}
	ae.ActionsRemaining--
	return nil
}

// UseBonusAction consumes a bonus action if available
// Returns CodeResourceExhausted if no bonus actions remain
func (ae *ActionEconomy) UseBonusAction() error {
	if ae.BonusActionsRemaining <= 0 {
		return rpgerr.ResourceExhausted("bonus action")
	}
	ae.BonusActionsRemaining--
	return nil
}

// UseReaction consumes a reaction if available
// Returns CodeResourceExhausted if no reactions remain
func (ae *ActionEconomy) UseReaction() error {
	if ae.ReactionsRemaining <= 0 {
		return rpgerr.ResourceExhausted("reaction")
	}
	ae.ReactionsRemaining--
	return nil
}

// Reset restores primary action economy to default values (1/1/1)
// Purpose: Called at the start of a combatant's turn to restore their action economy.
// Note: Does NOT reset AttacksRemaining (stays 0 until Attack ability is used) or
// MovementRemaining (should be set separately via SetMovement at turn start).
// Resets turn-granted capacity (OffHandAttacks, FlurryStrikes) to 0.
func (ae *ActionEconomy) Reset() {
	ae.ActionsRemaining = 1
	ae.BonusActionsRemaining = 1
	ae.ReactionsRemaining = 1
	// Note: AttacksRemaining and MovementRemaining are NOT reset here
	// They are set separately by abilities (Attack) and at turn start (SetMovement)
	ae.OffHandAttacksRemaining = 0
	ae.FlurryStrikesRemaining = 0
}

// GrantExtraAction grants an additional action
// Purpose: Used by features like Action Surge that provide extra actions beyond the normal limit
func (ae *ActionEconomy) GrantExtraAction() {
	ae.ActionsRemaining++
}

// GrantExtraBonusAction grants an additional bonus action
// Purpose: Future-proofing for potential features that grant extra bonus actions
func (ae *ActionEconomy) GrantExtraBonusAction() {
	ae.BonusActionsRemaining++
}

// CanUseAttack returns whether any attacks remain
// Purpose: Allows checking attack availability without consuming it.
// Returns false until Attack ability is used and SetAttacks is called.
func (ae *ActionEconomy) CanUseAttack() bool {
	return ae.AttacksRemaining > 0
}

// UseAttack consumes one attack if available
// Purpose: Called by Strike actions to consume one of the attacks granted by the Attack ability.
// Returns CodeResourceExhausted if no attacks remain.
func (ae *ActionEconomy) UseAttack() error {
	if ae.AttacksRemaining <= 0 {
		return rpgerr.ResourceExhausted("attack")
	}
	ae.AttacksRemaining--
	return nil
}

// SetAttacks sets the number of attacks remaining
// Purpose: Called by the Attack ability to grant attacks based on Extra Attack feature.
// Normal characters get 1 attack; fighters with Extra Attack get 2 or more.
func (ae *ActionEconomy) SetAttacks(count int) {
	ae.AttacksRemaining = count
}

// CanUseMovement returns whether enough movement remains for the given cost
// Purpose: Allows checking movement availability without consuming it.
// A cost of 0 always returns true (for free movement effects).
func (ae *ActionEconomy) CanUseMovement(cost int) bool {
	return ae.MovementRemaining >= cost
}

// UseMovement consumes the specified amount of movement if available
// Purpose: Called by Move actions to consume movement when moving on the battlefield.
// Returns CodeResourceExhausted if insufficient movement remains.
// Does not consume partial movement - it's all or nothing.
func (ae *ActionEconomy) UseMovement(cost int) error {
	if ae.MovementRemaining < cost {
		return rpgerr.ResourceExhausted("movement")
	}
	ae.MovementRemaining -= cost
	return nil
}

// SetMovement sets the movement remaining to the specified amount
// Purpose: Called at turn start to set movement from character speed.
// Overwrites any existing movement value.
func (ae *ActionEconomy) SetMovement(amount int) {
	ae.MovementRemaining = amount
}

// AddMovement adds the specified amount to movement remaining
// Purpose: Called by the Dash ability to add the character's speed again.
// Can be called multiple times (e.g., Rogue's Cunning Action Dash).
func (ae *ActionEconomy) AddMovement(amount int) {
	ae.MovementRemaining += amount
}

// CanUseOffHandAttack returns whether any off-hand attacks remain
// Purpose: Allows checking off-hand attack availability without consuming it.
// Returns false until TwoWeaponGranter grants off-hand attacks.
func (ae *ActionEconomy) CanUseOffHandAttack() bool {
	return ae.OffHandAttacksRemaining > 0
}

// UseOffHandAttack consumes one off-hand attack if available
// Purpose: Called by OffHandStrike actions to consume one of the attacks granted by two-weapon fighting.
// Returns CodeResourceExhausted if no off-hand attacks remain.
func (ae *ActionEconomy) UseOffHandAttack() error {
	if ae.OffHandAttacksRemaining <= 0 {
		return rpgerr.ResourceExhausted("off-hand attack")
	}
	ae.OffHandAttacksRemaining--
	return nil
}

// SetOffHandAttacks sets the number of off-hand attacks remaining
// Purpose: Called by TwoWeaponGranter to grant off-hand attacks after a main-hand attack.
// Usually grants 1 off-hand attack per turn.
func (ae *ActionEconomy) SetOffHandAttacks(count int) {
	ae.OffHandAttacksRemaining = count
}

// CanUseFlurryStrike returns whether any flurry strikes remain
// Purpose: Allows checking flurry strike availability without consuming it.
// Returns false until FlurryOfBlows grants flurry strikes.
func (ae *ActionEconomy) CanUseFlurryStrike() bool {
	return ae.FlurryStrikesRemaining > 0
}

// UseFlurryStrike consumes one flurry strike if available
// Purpose: Called by FlurryStrike actions to consume one of the attacks granted by Flurry of Blows.
// Returns CodeResourceExhausted if no flurry strikes remain.
func (ae *ActionEconomy) UseFlurryStrike() error {
	if ae.FlurryStrikesRemaining <= 0 {
		return rpgerr.ResourceExhausted("flurry strike")
	}
	ae.FlurryStrikesRemaining--
	return nil
}

// SetFlurryStrikes sets the number of flurry strikes remaining
// Purpose: Called by FlurryOfBlows feature to grant flurry strikes.
// Usually grants 2 flurry strikes when activated.
func (ae *ActionEconomy) SetFlurryStrikes(count int) {
	ae.FlurryStrikesRemaining = count
}
