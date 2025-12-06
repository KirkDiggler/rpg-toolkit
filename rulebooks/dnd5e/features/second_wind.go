// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SecondWind represents the fighter's Second Wind feature.
// It implements core.Action[FeatureInput] for activation and events.BusEffect for resource management.
type SecondWind struct {
	id          string
	name        string
	level       int                         // Fighter level for healing calculation
	characterID string                      // Character this feature belongs to
	resource    *combat.RecoverableResource // Tracks second wind uses (1 per short/long rest)
}

// SecondWindData is the JSON structure for persisting Second Wind state
type SecondWindData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Level       int       `json:"level"`
	CharacterID string    `json:"character_id"`
	Uses        int       `json:"uses"`
	MaxUses     int       `json:"max_uses"`
}

// GetID implements core.Entity
func (s *SecondWind) GetID() string {
	return s.id
}

// GetType implements core.Entity
func (s *SecondWind) GetType() core.EntityType {
	return "feature"
}

// CanActivate implements core.Action[FeatureInput]
func (s *SecondWind) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	// Check if we have uses remaining
	if !s.resource.IsAvailable() {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no second wind uses remaining")
	}

	return nil
}

// Apply subscribes the recoverable resource to the event bus for automatic rest recovery.
// This should be called when the feature is granted to a character.
func (s *SecondWind) Apply(ctx context.Context, bus events.EventBus) error {
	return s.resource.Apply(ctx, bus)
}

// Remove unsubscribes the recoverable resource from the event bus.
// This should be called when the feature is removed from a character.
func (s *SecondWind) Remove(ctx context.Context, bus events.EventBus) error {
	return s.resource.Remove(ctx, bus)
}

// Activate implements core.Action[FeatureInput]
func (s *SecondWind) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := s.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume a use
	if err := s.resource.Use(1); err != nil {
		return rpgerr.Wrapf(err, "failed to use second wind")
	}

	// Roll healing: 1d10 + fighter level
	pool, err := dice.ParseNotation("1d10")
	if err != nil {
		return rpgerr.Wrapf(err, "failed to parse healing dice")
	}

	result := pool.Roll(nil) // nil uses default roller
	if result.Error() != nil {
		return rpgerr.Wrapf(result.Error(), "failed to roll healing dice")
	}

	roll := result.Total() // This includes just the 1d10 roll
	modifier := s.level    // Fighter level is the modifier
	totalHealing := roll + modifier

	// Publish healing received event
	if input.Bus != nil {
		topic := dnd5eEvents.HealingReceivedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.HealingReceivedEvent{
			TargetID: owner.GetID(),
			Amount:   totalHealing,
			Roll:     roll,
			Modifier: modifier,
			Source:   refs.Features.SecondWind().ID,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish healing event")
		}
	}

	return nil
}

// loadJSON loads Second Wind state from JSON
func (s *SecondWind) loadJSON(data json.RawMessage) error {
	var secondWindData SecondWindData
	if err := json.Unmarshal(data, &secondWindData); err != nil {
		return fmt.Errorf("failed to unmarshal second wind data: %w", err)
	}

	s.id = secondWindData.ID
	s.name = secondWindData.Name
	s.level = secondWindData.Level
	s.characterID = secondWindData.CharacterID

	// Set up recoverable resource with current and max uses
	s.resource = combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          refs.Features.SecondWind().ID,
		Maximum:     secondWindData.MaxUses,
		CharacterID: secondWindData.CharacterID,
		ResetType:   coreResources.ResetShortRest,
	})
	// Restore to the saved state
	if secondWindData.Uses < secondWindData.MaxUses {
		if err := s.resource.Use(secondWindData.MaxUses - secondWindData.Uses); err != nil {
			return fmt.Errorf("failed to set resource uses: %w", err)
		}
	}

	return nil
}

// ToJSON converts Second Wind to JSON for persistence
func (s *SecondWind) ToJSON() (json.RawMessage, error) {
	data := SecondWindData{
		Ref:         refs.Features.SecondWind(),
		ID:          s.id,
		Name:        s.name,
		Level:       s.level,
		CharacterID: s.characterID,
		Uses:        s.resource.Current(),
		MaxUses:     s.resource.Maximum(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal second wind data: %w", err)
	}

	return bytes, nil
}
