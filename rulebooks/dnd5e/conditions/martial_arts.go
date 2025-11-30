// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// MartialArtsData is the JSON structure for persisting martial arts condition state.
// The Name field provides a human-readable display name for API consumers.
type MartialArtsData struct {
	Ref           core.Ref `json:"ref"`
	Name          string   `json:"name"`
	CharacterID   string   `json:"character_id"`
	UnarmedDamage string   `json:"unarmed_damage"` // e.g., "1d4", "1d6", etc.
	Level         int      `json:"level"`
}

// MartialArtsCondition represents the monk's Martial Arts feature.
// At Level 1:
// - Unarmed strikes deal 1d4 damage (increases with level)
// - Can use DEX instead of STR for unarmed strikes and monk weapons
// - Bonus action unarmed strike after Attack action (not yet implemented)
type MartialArtsCondition struct {
	CharacterID   string
	UnarmedDamage string // Dice notation for unarmed strike damage
	Level         int    // Monk level (affects damage die)
	bus           events.EventBus
}

// Ensure MartialArtsCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*MartialArtsCondition)(nil)

// MartialArtsConditionConfig configures a martial arts condition
type MartialArtsConditionConfig struct {
	CharacterID string
	Level       int
}

// NewMartialArtsCondition creates a martial arts condition from config
func NewMartialArtsCondition(cfg MartialArtsConditionConfig) *MartialArtsCondition {
	return &MartialArtsCondition{
		CharacterID:   cfg.CharacterID,
		Level:         cfg.Level,
		UnarmedDamage: getMartialArtsDie(cfg.Level),
	}
}

// getMartialArtsDie returns the martial arts damage die for a given monk level
func getMartialArtsDie(level int) string {
	switch {
	case level >= 17:
		return "1d10"
	case level >= 11:
		return "1d8"
	case level >= 5:
		return "1d6"
	default:
		return "1d4"
	}
}

// IsApplied returns true if this condition is currently applied
func (m *MartialArtsCondition) IsApplied() bool {
	return m.bus != nil
}

// Apply registers this condition with the event bus.
// For Level 1 MVP, Martial Arts is a passive feature that modifies unarmed strike damage.
// Future implementation will subscribe to combat chains for DEX-for-attacks logic.
func (m *MartialArtsCondition) Apply(_ context.Context, bus events.EventBus) error {
	if m.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "martial arts condition already applied")
	}
	m.bus = bus
	return nil
}

// Remove unregisters this condition from the event bus
func (m *MartialArtsCondition) Remove(_ context.Context, _ events.EventBus) error {
	m.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (m *MartialArtsCondition) ToJSON() (json.RawMessage, error) {
	data := MartialArtsData{
		Ref: core.Ref{
			Module: "dnd5e",
			Type:   "conditions",
			Value:  "martial_arts",
		},
		Name:          "Martial Arts",
		CharacterID:   m.CharacterID,
		UnarmedDamage: m.UnarmedDamage,
		Level:         m.Level,
	}
	return json.Marshal(data)
}

// loadJSON loads martial arts condition state from JSON
func (m *MartialArtsCondition) loadJSON(data json.RawMessage) error {
	var maData MartialArtsData
	if err := json.Unmarshal(data, &maData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal martial arts data")
	}

	m.CharacterID = maData.CharacterID
	m.UnarmedDamage = maData.UnarmedDamage
	m.Level = maData.Level

	return nil
}

// GetUnarmedDamage returns the current unarmed strike damage die
func (m *MartialArtsCondition) GetUnarmedDamage() string {
	return m.UnarmedDamage
}
