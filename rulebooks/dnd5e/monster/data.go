// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Data represents the serializable form of a monster.
// This is what gets stored in the database - pure JSON, no logic.
type Data struct {
	// Identity
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Ref  *core.Ref `json:"ref,omitempty"` // Type reference (e.g., refs.Monsters.Skeleton())

	// Core stats
	HitPoints        int                  `json:"hit_points"`
	MaxHitPoints     int                  `json:"max_hit_points"`
	ArmorClass       int                  `json:"armor_class"`
	AbilityScores    shared.AbilityScores `json:"ability_scores"`
	ProficiencyBonus int                  `json:"proficiency_bonus,omitempty"` // CR-based proficiency bonus

	// Movement
	Speed SpeedData `json:"speed"`

	// Senses (for perception/targeting)
	Senses SensesData `json:"senses"`

	// Actions this monster can take
	Actions []combatActions.Definition `json:"actions"`

	// Features (special abilities like Nimble Escape)
	Features []json.RawMessage `json:"features,omitempty"`

	// Conditions (runtime state: poisoned, hidden, etc.)
	Conditions []json.RawMessage `json:"conditions,omitempty"`

	// Inventory (potions, items)
	Inventory []InventoryItemData `json:"inventory,omitempty"`

	// Proficiencies (for skill checks like Stealth)
	Proficiencies []ProficiencyData `json:"proficiencies,omitempty"`

	// AI behavior
	Targeting TargetingStrategy `json:"targeting,omitempty"`
}

// SpeedData represents monster movement speeds in feet
type SpeedData struct {
	Walk   int `json:"walk"`
	Fly    int `json:"fly,omitempty"`
	Swim   int `json:"swim,omitempty"`
	Climb  int `json:"climb,omitempty"`
	Burrow int `json:"burrow,omitempty"`
}

// SensesData represents monster sensory capabilities
type SensesData struct {
	Darkvision        int `json:"darkvision,omitempty"` // feet
	Blindsight        int `json:"blindsight,omitempty"`
	Tremorsense       int `json:"tremorsense,omitempty"`
	Truesight         int `json:"truesight,omitempty"`
	PassivePerception int `json:"passive_perception"`
}

// InventoryItemData represents a serializable inventory item
type InventoryItemData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// ProficiencyData represents a serializable proficiency
type ProficiencyData struct {
	Skill string `json:"skill"`
	Bonus int    `json:"bonus"`
}
