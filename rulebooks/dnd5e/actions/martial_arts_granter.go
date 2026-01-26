// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MartialArtsGranterInput contains the information needed to determine
// if a martial arts bonus strike should be granted after an attack.
type MartialArtsGranterInput struct {
	// CharacterID is the ID of the character who made the attack
	CharacterID string

	// WeaponID is the weapon used for the attack (empty string for unarmed)
	WeaponID weapons.WeaponID

	// IsUnarmed is true if this was an unarmed strike
	IsUnarmed bool

	// SourceAbility indicates what granted the attack (e.g., "attack", "flurry", "off_hand")
	// Martial Arts bonus strike should only trigger from the Attack action, not from
	// Flurry of Blows or off-hand attacks.
	SourceAbility string

	// AlreadyGrantedThisTurn is true if a martial arts bonus strike was already granted this turn.
	// Martial Arts only grants ONE bonus strike per Attack action, not per individual strike.
	AlreadyGrantedThisTurn bool

	// EventBus is used to subscribe the action to turn-end events and grant actions via events
	EventBus events.EventBus
}

// MartialArtsGranterOutput contains the result of checking/granting a martial arts bonus strike.
type MartialArtsGranterOutput struct {
	// Granted is true if a MartialArtsBonusStrike was granted
	Granted bool

	// Action is the granted MartialArtsBonusStrike action (nil if not granted)
	Action *MartialArtsBonusStrike

	// Reason explains why the action was or wasn't granted
	Reason string
}

// CheckAndGrantMartialArtsBonusStrike checks if a martial arts bonus strike should be granted
// after an attack and grants it if conditions are met.
//
// Martial Arts bonus strike conditions (PHB p.78):
// 1. Attack was made using the Attack action (not Flurry, off-hand, etc.)
// 2. Attack used an unarmed strike OR monk weapon
// 3. Bonus strike not already granted this turn
//
// If all conditions are met, a MartialArtsBonusStrike action is created, applied to
// the event bus (for turn-end cleanup), and published via ActionGrantedEvent.
func CheckAndGrantMartialArtsBonusStrike(ctx context.Context, input *MartialArtsGranterInput) (*MartialArtsGranterOutput, error) {
	// Must be from Attack action (not Flurry, off-hand, etc.)
	if input.SourceAbility != "attack" && input.SourceAbility != "" {
		return &MartialArtsGranterOutput{
			Granted: false,
			Reason:  fmt.Sprintf("attack source is %s, not Attack action", input.SourceAbility),
		}, nil
	}

	// Must not have already granted this turn
	if input.AlreadyGrantedThisTurn {
		return &MartialArtsGranterOutput{
			Granted: false,
			Reason:  "martial arts bonus strike already granted this turn",
		}, nil
	}

	// Must be unarmed strike or monk weapon
	if !input.IsUnarmed && input.WeaponID != "" {
		// Check if weapon is a monk weapon
		weapon, err := weapons.GetByID(input.WeaponID)
		if err != nil {
			return &MartialArtsGranterOutput{
				Granted: false,
				Reason:  "weapon not found",
			}, nil
		}
		if !isMonkWeapon(&weapon) {
			return &MartialArtsGranterOutput{
				Granted: false,
				Reason:  "weapon is not a monk weapon",
			}, nil
		}
	}

	// All conditions met - create the MartialArtsBonusStrike action
	strike := NewMartialArtsBonusStrike(MartialArtsBonusStrikeConfig{
		ID:      fmt.Sprintf("%s-martial-arts-bonus-strike", input.CharacterID),
		OwnerID: input.CharacterID,
	})

	// Apply to event bus for turn-end cleanup
	if input.EventBus != nil {
		if err := strike.Apply(ctx, input.EventBus); err != nil {
			return nil, fmt.Errorf("failed to apply martial arts bonus strike to event bus: %w", err)
		}

		// Publish ActionGrantedEvent - character subscribes and adds the action
		actionGrantedTopic := dnd5eEvents.ActionGrantedTopic.On(input.EventBus)
		if err := actionGrantedTopic.Publish(ctx, dnd5eEvents.ActionGrantedEvent{
			CharacterID: input.CharacterID,
			Action:      strike,
			Source:      "martial_arts",
		}); err != nil {
			// Rollback event subscription
			_ = strike.Remove(ctx, input.EventBus)
			return nil, fmt.Errorf("failed to publish action granted event: %w", err)
		}
	}

	return &MartialArtsGranterOutput{
		Granted: true,
		Action:  strike,
		Reason:  "unarmed strike or monk weapon attack",
	}, nil
}

// isMonkWeapon checks if a weapon is a monk weapon.
// Monk weapons are shortswords and simple melee weapons without Heavy or Two-Handed properties.
// Note: This duplicates the logic in conditions/martial_arts.go to avoid import cycles.
func isMonkWeapon(weapon *weapons.Weapon) bool {
	// Shortsword is explicitly a monk weapon
	if weapon.ID == weapons.Shortsword {
		return true
	}

	// Must be a simple melee weapon
	if weapon.Category != weapons.CategorySimpleMelee {
		return false
	}

	// Cannot have Heavy property
	if weapon.HasProperty(weapons.PropertyHeavy) {
		return false
	}

	// Cannot have Two-Handed property
	if weapon.HasProperty(weapons.PropertyTwoHanded) {
		return false
	}

	return true
}
