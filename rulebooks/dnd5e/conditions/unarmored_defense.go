// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// UnarmoredDefenseType distinguishes between class variants of Unarmored Defense
type UnarmoredDefenseType string

const (
	// UnarmoredDefenseBarbarian uses CON modifier (10 + DEX + CON)
	UnarmoredDefenseBarbarian UnarmoredDefenseType = "barbarian"
	// UnarmoredDefenseMonk uses WIS modifier (10 + DEX + WIS)
	UnarmoredDefenseMonk UnarmoredDefenseType = "monk"
)

// UnarmoredDefenseData is the JSON structure for persisting unarmored defense condition state
type UnarmoredDefenseData struct {
	Ref         *core.Ref `json:"ref"`
	Type        string    `json:"type"`         // "barbarian" or "monk"
	CharacterID string    `json:"character_id"` // ID of the character
	// Source is a ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
	Source string `json:"source"`
}

// UnarmoredDefenseCondition represents the Unarmored Defense feature.
// Barbarian: AC = 10 + DEX modifier + CON modifier
// Monk: AC = 10 + DEX modifier + WIS modifier
// Only applies when not wearing armor. Shields can still be used.
type UnarmoredDefenseCondition struct {
	CharacterID     string
	Type            UnarmoredDefenseType
	Source          string // Ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure UnarmoredDefenseCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnarmoredDefenseCondition)(nil)

// UnarmoredDefenseInput provides configuration for creating an unarmored defense condition
type UnarmoredDefenseInput struct {
	CharacterID string               // ID of the character
	Type        UnarmoredDefenseType // Barbarian (CON) or Monk (WIS)
	Source      string               // Ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
}

// NewUnarmoredDefenseCondition creates an unarmored defense condition from input
func NewUnarmoredDefenseCondition(input UnarmoredDefenseInput) *UnarmoredDefenseCondition {
	return &UnarmoredDefenseCondition{
		CharacterID: input.CharacterID,
		Type:        input.Type,
		Source:      input.Source,
	}
}

// IsApplied returns true if this condition is currently applied
func (u *UnarmoredDefenseCondition) IsApplied() bool {
	return u.bus != nil
}

// Apply subscribes this condition to AC chain events.
func (u *UnarmoredDefenseCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if u.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "unarmored defense already applied")
	}
	u.bus = bus

	// Subscribe to ACChain to add secondary ability modifier when unarmored
	acChain := combat.ACChain.On(bus)
	subID, err := acChain.SubscribeWithChain(ctx, u.onACChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to AC chain")
	}
	u.subscriptionIDs = append(u.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (u *UnarmoredDefenseCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if u.bus == nil {
		return nil
	}

	for _, subID := range u.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	u.subscriptionIDs = nil
	u.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (u *UnarmoredDefenseCondition) ToJSON() (json.RawMessage, error) {
	data := UnarmoredDefenseData{
		Ref:         refs.Conditions.UnarmoredDefense(),
		Type:        string(u.Type),
		CharacterID: u.CharacterID,
		Source:      u.Source,
	}
	return json.Marshal(data)
}

// loadJSON loads unarmored defense condition state from JSON
func (u *UnarmoredDefenseCondition) loadJSON(data json.RawMessage) error {
	var udData UnarmoredDefenseData
	if err := json.Unmarshal(data, &udData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unarmored defense data")
	}

	u.CharacterID = udData.CharacterID
	u.Type = UnarmoredDefenseType(udData.Type)
	u.Source = udData.Source

	return nil
}

// CalculateAC computes the AC for this unarmored defense type given ability scores.
// Returns the unarmored AC (10 + DEX + secondary ability).
func (u *UnarmoredDefenseCondition) CalculateAC(scores shared.AbilityScores) int {
	baseAC := 10
	dexMod := scores.Modifier(abilities.DEX)

	var secondaryMod int
	switch u.Type {
	case UnarmoredDefenseBarbarian:
		secondaryMod = scores.Modifier(abilities.CON)
	case UnarmoredDefenseMonk:
		secondaryMod = scores.Modifier(abilities.WIS)
	}

	return baseAC + dexMod + secondaryMod
}

// SecondaryAbility returns the secondary ability used for this type of unarmored defense
func (u *UnarmoredDefenseCondition) SecondaryAbility() abilities.Ability {
	switch u.Type {
	case UnarmoredDefenseBarbarian:
		return abilities.CON
	case UnarmoredDefenseMonk:
		return abilities.WIS
	default:
		return abilities.CON
	}
}

// onACChain adds the secondary ability modifier to AC when unarmored.
func (u *UnarmoredDefenseCondition) onACChain(
	ctx context.Context,
	event *combat.ACChainEvent,
	c chain.Chain[*combat.ACChainEvent],
) (chain.Chain[*combat.ACChainEvent], error) {
	// Only modify AC for this character
	if event.CharacterID != u.CharacterID {
		return c, nil
	}

	// Only apply when NOT wearing armor (shields are fine)
	if event.HasArmor {
		return c, nil
	}

	// Get ability scores from game context
	registry, err := gamectx.RequireCharacters(ctx)
	if err != nil {
		return c, err
	}

	abilityScores := registry.GetCharacterAbilityScores(u.CharacterID)
	if abilityScores == nil {
		return c, nil
	}

	// Get the secondary ability modifier (WIS for Monk, CON for Barbarian)
	secondaryMod := abilityScores.Modifier(u.SecondaryAbility())

	// Add secondary ability modifier at StageFeatures
	modifyAC := func(_ context.Context, e *combat.ACChainEvent) (*combat.ACChainEvent, error) {
		e.Breakdown.AddComponent(combat.ACComponent{
			Type:   combat.ACSourceFeature,
			Source: refs.Conditions.UnarmoredDefense(),
			Value:  secondaryMod,
		})
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "unarmored_defense", modifyAC); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply unarmored defense for character %s", u.CharacterID)
	}

	return c, nil
}
