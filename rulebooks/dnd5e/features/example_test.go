package features_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// Example demonstrates how rage works with the event system
func Example() {
	// Create event bus for game communication
	bus := events.NewEventBus()
	ctx := context.Background()

	// Mock barbarian character
	barbarian := &mockCharacter{id: "conan"}

	// Server has stored feature JSON
	featureJSON := json.RawMessage(`{
		"ref": {"value": "rage"},
		"id": "rage",
		"name": "Rage",
		"level": 5,
		"uses": 2,
		"max_uses": 3
	}`)

	// Load the feature
	feature, _ := features.LoadJSON(featureJSON)

	// Listen for condition applications
	topic := dnd5e.ConditionAppliedTopic.On(bus)
	_, err := topic.Subscribe(ctx, func(_ context.Context, event dnd5e.ConditionAppliedEvent) error {
		if event.Type == dnd5e.ConditionRaging {
			// Type assert the data to get rage-specific info
			if rageData, ok := event.Data.(features.RageEventData); ok {
				fmt.Printf("%s is now raging! Damage bonus: +%d\n",
					event.Target.GetID(),
					rageData.DamageBonus)
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
	id string
}

func (m *mockCharacter) GetID() string            { return m.id }
func (m *mockCharacter) GetType() core.EntityType { return "character" }
