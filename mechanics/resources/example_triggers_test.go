// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// Example demonstrates using custom restoration triggers
// instead of hardcoded rest mechanics.
func Example() {
	// Create a character
	character := &MockEntity{id: "paladin-1", typ: "character"}
	pool := resources.NewSimplePool(character)
	bus := events.NewBus()

	// Create resources with game-specific triggers

	// Divine resources that restore at dawn
	layOnHands := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "lay-on-hands",
		Type:    resources.ResourceTypeCustom,
		Owner:   character,
		Key:     "lay_on_hands_hp",
		Current: 25,
		Maximum: 50, // Level 10 paladin
		RestoreTriggers: map[string]int{
			"my.game.dawn":      -1, // Full restore at dawn
			"my.game.long_rest": -1, // Also on long rest
		},
	})

	// Ability that restores on any rest
	divineSense := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "divine-sense",
		Type:    resources.ResourceTypeAbilityUse,
		Owner:   character,
		Key:     "divine_sense_uses",
		Current: 3,
		Maximum: 6, // 1 + CHA modifier (5)
		RestoreTriggers: map[string]int{
			"my.game.short_rest": -1,
			"my.game.long_rest":  -1,
			"my.game.dawn":       -1, // Paladins regain at dawn too
		},
	})

	// Custom restoration from game events
	inspiration := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "divine-inspiration",
		Type:    resources.ResourceTypeCustom,
		Owner:   character,
		Key:     "inspiration_points",
		Current: 0,
		Maximum: 3,
		RestoreTriggers: map[string]int{
			"my.game.heroic_deed":     1,  // +1 for heroic acts
			"my.game.prayer_answered": 2,  // +2 when deity responds
			"my.game.milestone":       -1, // Full at story milestones
		},
	})

	// Add to pool
	_ = pool.Add(layOnHands)
	_ = pool.Add(divineSense)
	_ = pool.Add(inspiration)

	// Track restoration events (sorted for deterministic output)
	restorations := []string{}
	handler := func(_ context.Context, e events.Event) error {
		event := e.(*resources.ResourceRestoredEvent)
		restorations = append(restorations, fmt.Sprintf("  %s restored %d points (trigger: %s)",
			event.Resource.Key(), event.Amount, event.Reason))
		return nil
	}
	bus.SubscribeFunc(resources.EventResourceRestored, 0, events.HandlerFunc(handler))

	// Helper to print sorted restoration messages
	printRestorations := func() {
		sort.Strings(restorations)
		for _, msg := range restorations {
			fmt.Println(msg)
		}
		restorations = restorations[:0] // Clear for next batch
	}

	// Use some resources to see restoration
	layOnHands.SetCurrent(10)
	divineSense.SetCurrent(1)

	// Simulate game events

	// Dawn breaks - divine resources restore
	fmt.Println("Dawn breaks...")
	pool.ProcessRestoration("my.game.dawn", bus)
	printRestorations()

	// Character performs a heroic deed
	fmt.Println("Paladin saves innocent - heroic deed!")
	pool.ProcessRestoration("my.game.heroic_deed", bus)
	printRestorations()

	// Party reaches a milestone
	fmt.Println("Major story milestone reached!")
	pool.ProcessRestoration("my.game.milestone", bus)
	printRestorations()

	// Game-specific rest (not D&D's short/long rest)
	fmt.Println("Party takes shelter in temple - blessed rest")
	pool.ProcessRestoration("my.game.blessed_rest", bus)
	printRestorations()

	// Output:
	// Dawn breaks...
	//   divine_sense_uses restored 5 points (trigger: my.game.dawn)
	//   lay_on_hands_hp restored 40 points (trigger: my.game.dawn)
	// Paladin saves innocent - heroic deed!
	//   inspiration_points restored 1 points (trigger: my.game.heroic_deed)
	// Major story milestone reached!
	//   inspiration_points restored 2 points (trigger: my.game.milestone)
	// Party takes shelter in temple - blessed rest
}
