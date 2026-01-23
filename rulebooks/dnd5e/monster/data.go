// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
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
	Actions []ActionData `json:"actions"`

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

// ActionData represents a serializable monster action.
// The Ref identifies which action implementation to load.
type ActionData struct {
	Ref    core.Ref        `json:"ref"`    // e.g., refs.MonsterActions.Scimitar()
	Config json.RawMessage `json:"config"` // Action-specific configuration
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

// ActionCost represents the action economy cost of an action
type ActionCost int

// Action cost constants
const (
	CostNone ActionCost = iota
	CostAction
	CostBonusAction
	CostReaction
)

// ActionType categorizes monster actions for behavior decisions
type ActionType string

// Action type constants
const (
	TypeMeleeAttack  ActionType = "melee_attack"
	TypeRangedAttack ActionType = "ranged_attack"
	TypeSpell        ActionType = "spell"
	TypeHeal         ActionType = "heal"
	TypeMovement     ActionType = "movement"
	TypeStealth      ActionType = "stealth"
	TypeDefend       ActionType = "defend"
)
