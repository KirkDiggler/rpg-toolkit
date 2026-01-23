// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// HealingSourceType categorizes where healing comes from
type HealingSourceType string

// Healing source type constants
const (
	HealingSourceSpell      HealingSourceType = "spell"
	HealingSourcePotion     HealingSourceType = "potion"
	HealingSourceFeature    HealingSourceType = "feature"
	HealingSourceSecondWind HealingSourceType = "second_wind"
	HealingSourceLayOnHands HealingSourceType = "lay_on_hands"
	HealingSourceSongOfRest HealingSourceType = "song_of_rest"
	HealingSourceHitDice    HealingSourceType = "hit_dice"
	HealingSourceLongRest   HealingSourceType = "long_rest"
	// Add more as needed
)

// HealingComponent represents healing from one source
type HealingComponent struct {
	Source     HealingSourceType
	DiceRolls  []int // Individual dice rolls (e.g., [3, 7, 2] for 3d8)
	FlatBonus  int   // Flat modifier (0 if none)
	HealingMod int   // Additional modifier (e.g., from abilities or features)
}

// Total returns the total healing for this component
func (hc *HealingComponent) Total() int {
	total := hc.FlatBonus + hc.HealingMod
	for _, roll := range hc.DiceRolls {
		total += roll
	}
	return total
}

// HealingChainEvent represents healing flowing through the modifier chain
type HealingChainEvent struct {
	HealerID   string             // Who is healing (could be same as target for self-heal)
	TargetID   string             // Who receives healing
	Components []HealingComponent // All healing sources
}

// TotalHealing calculates the total healing from all components
func (hce *HealingChainEvent) TotalHealing() int {
	total := 0
	for _, component := range hce.Components {
		total += component.Total()
	}
	return total
}

// HealChain provides typed chained topic for healing modifiers
var HealChain = events.DefineChainedTopic[*HealingChainEvent]("dnd5e.combat.healing.chain")
