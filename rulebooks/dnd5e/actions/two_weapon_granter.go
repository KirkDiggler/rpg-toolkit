package actions

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// AttackHand indicates which hand made the attack.
// Mirrors combat.AttackHand to avoid import cycle.
type AttackHand string

const (
	// AttackHandMain is a main hand attack using a standard action.
	AttackHandMain AttackHand = "main"

	// AttackHandOff is an off-hand attack using a bonus action.
	AttackHandOff AttackHand = "off"
)

// EquippedWeaponInfo provides weapon information for two-weapon fighting checks.
type EquippedWeaponInfo struct {
	WeaponID weapons.WeaponID
}

// TwoWeaponGranterInput contains the information needed to determine
// if an off-hand strike should be granted after a main-hand attack.
type TwoWeaponGranterInput struct {
	// CharacterID is the ID of the character who made the attack
	CharacterID string

	// AttackHand indicates which hand made the attack (main or off)
	AttackHand AttackHand

	// MainHandWeapon is the weapon in the main hand (nil if none)
	MainHandWeapon *EquippedWeaponInfo

	// OffHandWeapon is the weapon in the off hand (nil if none or shield)
	OffHandWeapon *EquippedWeaponInfo

	// ActionHolder is used to grant the off-hand strike action (deprecated - use EventBus instead)
	// Kept for backwards compatibility. If EventBus is provided, actions are granted via events.
	ActionHolder ActionHolder

	// EventBus is used to subscribe the action to turn-end events and grant actions via events
	EventBus events.EventBus
}

// TwoWeaponGranterOutput contains the result of checking/granting an off-hand strike.
type TwoWeaponGranterOutput struct {
	// Granted is true if an OffHandStrike was granted
	Granted bool

	// Action is the granted OffHandStrike action (nil if not granted)
	Action *OffHandStrike

	// Reason explains why the action was or wasn't granted
	Reason string
}

// CheckAndGrantOffHandStrike checks if an off-hand strike should be granted
// after a main-hand attack and grants it if conditions are met.
//
// Two-weapon fighting conditions:
// 1. Attack was made with main hand
// 2. Main hand weapon is light
// 3. Off hand has a light weapon (not shield)
//
// If all conditions are met, an OffHandStrike action is created, applied to
// the event bus (for turn-end cleanup), and added to the character's actions.
func CheckAndGrantOffHandStrike(ctx context.Context, input *TwoWeaponGranterInput) (*TwoWeaponGranterOutput, error) {
	// Must be a main-hand attack
	if input.AttackHand != AttackHandMain && input.AttackHand != "" {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "not a main-hand attack",
		}, nil
	}

	// Must have main-hand weapon
	if input.MainHandWeapon == nil {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "no main-hand weapon",
		}, nil
	}

	// Must have off-hand weapon
	if input.OffHandWeapon == nil {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "no off-hand weapon",
		}, nil
	}

	// Main-hand weapon must be light
	mainWeapon, err := weapons.GetByID(input.MainHandWeapon.WeaponID)
	if err != nil {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "main-hand weapon not found",
		}, nil
	}
	if !mainWeapon.HasProperty(weapons.PropertyLight) {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "main-hand weapon is not light",
		}, nil
	}

	// Off-hand weapon must be light
	offWeapon, err := weapons.GetByID(input.OffHandWeapon.WeaponID)
	if err != nil {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "off-hand weapon not found",
		}, nil
	}
	if !offWeapon.HasProperty(weapons.PropertyLight) {
		return &TwoWeaponGranterOutput{
			Granted: false,
			Reason:  "off-hand weapon is not light",
		}, nil
	}

	// All conditions met - create the OffHandStrike action
	strike := NewOffHandStrike(OffHandStrikeConfig{
		ID:       fmt.Sprintf("%s-off-hand-strike", input.CharacterID),
		OwnerID:  input.CharacterID,
		WeaponID: input.OffHandWeapon.WeaponID,
	})

	// Apply to event bus for turn-end cleanup
	if input.EventBus != nil {
		if err := strike.Apply(ctx, input.EventBus); err != nil {
			return nil, fmt.Errorf("failed to apply off-hand strike to event bus: %w", err)
		}

		// Publish ActionGrantedEvent - character subscribes and adds the action
		actionGrantedTopic := dnd5eEvents.ActionGrantedTopic.On(input.EventBus)
		if err := actionGrantedTopic.Publish(ctx, dnd5eEvents.ActionGrantedEvent{
			CharacterID: input.CharacterID,
			Action:      strike,
			Source:      "two_weapon_fighting",
		}); err != nil {
			// Rollback event subscription
			_ = strike.Remove(ctx, input.EventBus)
			return nil, fmt.Errorf("failed to publish action granted event: %w", err)
		}
	} else if input.ActionHolder != nil {
		// Fallback to direct ActionHolder for backwards compatibility
		if err := input.ActionHolder.AddAction(strike); err != nil {
			return nil, fmt.Errorf("failed to add off-hand strike to character: %w", err)
		}
	}

	return &TwoWeaponGranterOutput{
		Granted: true,
		Action:  strike,
		Reason:  "dual-wielding light weapons",
	}, nil
}
