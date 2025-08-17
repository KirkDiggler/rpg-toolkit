// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"sync"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/damage"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/stretchr/testify/require"
)

func TestRage_ConcurrentEventHandling(t *testing.T) {
	// Create a bus
	bus := events.NewBus()

	// Load rage feature
	featureJSON := `{
		"ref": "dnd5e:features:rage",
		"id": "barbarian-rage",
		"data": {
			"uses": 10,
			"level": 5
		}
	}`

	rage, err := features.LoadJSON([]byte(featureJSON), bus)
	require.NoError(t, err)

	// Create barbarian
	barbarian := &mockEntity{id: "conan", entityType: dnd5e.EntityTypeCharacter}

	// Activate rage
	err = rage.Activate(context.Background(), barbarian, features.FeatureInput{})
	require.NoError(t, err)

	// Create wait group for concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 100
	numEventsPerGoroutine := 10

	// Concurrently publish attack events
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < numEventsPerGoroutine; j++ {
				// Mix of different attack types
				if j%2 == 0 {
					// STR melee attack - should get modifier
					attackEvent := dnd5e.NewAttackEvent(
						barbarian,
						&mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster},
						true,
						dnd5e.AbilityStrength,
						8,
					)
					_ = bus.Publish(attackEvent)
				} else {
					// DEX attack - should not get modifier
					attackEvent := dnd5e.NewAttackEvent(
						barbarian,
						&mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster},
						true,
						dnd5e.AbilityDexterity,
						6,
					)
					_ = bus.Publish(attackEvent)
				}
			}
		}()
	}

	// Concurrently publish damage events
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < numEventsPerGoroutine; j++ {
				// Mix of physical and magical damage
				var damageType damage.Type
				if j%2 == 0 {
					damageType = dnd5e.DamageTypeSlashing
				} else {
					damageType = dnd5e.DamageTypeFire
				}

				damageEvent := dnd5e.NewDamageReceivedEvent(
					barbarian,
					&mockEntity{id: "orc", entityType: dnd5e.EntityTypeMonster},
					10,
					damageType,
				)
				_ = bus.Publish(damageEvent)
			}
		}()
	}

	// Concurrently try to activate rage (consumes uses)
	wg.Add(numGoroutines / 10)
	for i := 0; i < numGoroutines/10; i++ {
		go func() {
			defer wg.Done()
			// May succeed or fail depending on uses remaining
			_ = rage.Activate(context.Background(), barbarian, features.FeatureInput{})
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If we get here without deadlock or race conditions, we're good!
	t.Log("Concurrent test completed successfully")
}
