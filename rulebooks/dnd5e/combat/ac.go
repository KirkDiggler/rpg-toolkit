// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ACChainEvent represents armor class calculation flowing through the modifier chain.
// This allows features like Defense fighting style to modify AC calculations.
type ACChainEvent struct {
	// CharacterID identifies which character's AC is being calculated
	CharacterID string

	// BaseAC is the starting AC value before any chain modifiers
	BaseAC int

	// IsWearingArmor indicates if the character is wearing armor
	// Used by features like Defense fighting style which only apply when wearing armor
	IsWearingArmor bool

	// ArmorType categorizes the armor being worn (light, medium, heavy, or empty string if none)
	ArmorType string

	// FinalAC is the calculated AC after all modifiers have been applied
	FinalAC int
}

// ACChain provides typed chained topic for armor class modifiers.
// Modifiers can subscribe to different stages (features, conditions, equipment, etc.)
// to modify the AC calculation in a predictable order.
//
// Example usage:
//
//	acChain := ACChain.On(eventBus)
//	acChain.Subscribe(StageFeatures, func(ctx context.Context, event ACChainEvent) (ACChainEvent, error) {
//	    if event.IsWearingArmor {
//	        event.FinalAC += 1 // Defense fighting style
//	    }
//	    return event, nil
//	})
var ACChain = events.DefineChainedTopic[ACChainEvent]("dnd5e.combat.ac.chain")
