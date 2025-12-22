package features_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// Example demonstrates how rage works with the event system
func Example() {
	// Create event bus for game communication
	bus := events.NewEventBus()
	ctx := context.Background()

	// Mock barbarian character with rage charges resource
	barbarian := &mockCharacter{
		id: "conan",
		resources: map[coreResources.ResourceKey]int{
			resources.RageCharges: 3, // Level 5 barbarian has 3 rage uses
		},
	}

	// Server has stored feature JSON (resource state is owned by character, not feature)
	featureJSON := json.RawMessage(`{
		"ref": {"module": "dnd5e", "type": "features", "id": "rage"},
		"id": "rage",
		"name": "Rage",
		"level": 5
	}`)

	// Load the feature
	feature, _ := features.LoadJSON(featureJSON)

	// Listen for condition applications
	topic := dnd5eEvents.ConditionAppliedTopic.On(bus)
	_, err := topic.Subscribe(ctx, func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
		if event.Type == dnd5eEvents.ConditionRaging {
			// Type assert the condition to get rage-specific info
			if ragingCond, ok := event.Condition.(*conditions.RagingCondition); ok {
				fmt.Printf("%s is now raging! Damage bonus: +%d\n",
					event.Target.GetID(),
					ragingCond.DamageBonus)
			}
		}

		return nil
	})
	if err != nil {
		fmt.Printf("Failed to subscribe to condition applied topic: %v\n", err)

		return
	}

	// Player activates rage - barbarian provides its bus
	err = feature.Activate(ctx, barbarian, features.FeatureInput{Bus: bus})
	if err != nil {
		fmt.Printf("Failed to rage: %v\n", err)
		return
	}

	// Output: conan is now raging! Damage bonus: +2
}

type mockCharacter struct {
	id        string
	resources map[coreResources.ResourceKey]int
}

func (m *mockCharacter) GetID() string            { return m.id }
func (m *mockCharacter) GetType() core.EntityType { return "character" }

// IsResourceAvailable implements coreResources.ResourceAccessor
func (m *mockCharacter) IsResourceAvailable(key coreResources.ResourceKey) bool {
	if m.resources == nil {
		return false
	}
	current, ok := m.resources[key]
	return ok && current > 0
}

// UseResource implements coreResources.ResourceAccessor
func (m *mockCharacter) UseResource(key coreResources.ResourceKey, amount int) error {
	if m.resources == nil {
		return fmt.Errorf("resource %s not found", key)
	}
	if current, ok := m.resources[key]; ok {
		m.resources[key] = current - amount
	}
	return nil
}
