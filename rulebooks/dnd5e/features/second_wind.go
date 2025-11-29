// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// SecondWind represents the fighter's Second Wind feature.
// It implements core.Action[FeatureInput] for activation.
type SecondWind struct {
	id       string
	name     string
	level    int                 // Fighter level for healing calculation
	resource *resources.Resource // Tracks second wind uses (1 per short/long rest)
}

// SecondWindData is the JSON structure for persisting Second Wind state
type SecondWindData struct {
	Ref     core.Ref `json:"ref"`
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Level   int      `json:"level"`
	Uses    int      `json:"uses"`
	MaxUses int      `json:"max_uses"`
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
			Source:   "second_wind",
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

	// Set up resource with current and max uses
	s.resource = resources.NewResource("second_wind", secondWindData.MaxUses)
	s.resource.SetCurrent(secondWindData.Uses)

	return nil
}

// ToJSON converts Second Wind to JSON for persistence
func (s *SecondWind) ToJSON() (json.RawMessage, error) {
	data := SecondWindData{
		Ref: core.Ref{
			Module: "dnd5e",
			Type:   "features",
			Value:  "second_wind",
		},
		ID:      s.id,
		Name:    s.name,
		Level:   s.level,
		Uses:    s.resource.Current,
		MaxUses: s.resource.Maximum,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal second wind data: %w", err)
	}

	return bytes, nil
}

// RestoreOnShortRest restores second wind use on a short or long rest
func (s *SecondWind) RestoreOnShortRest() {
	s.resource.RestoreToFull()
}
