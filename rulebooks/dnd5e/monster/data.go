// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Data represents the serializable form of a monster.
// This is what gets stored in the database - pure JSON, no logic.
type Data struct {
	// CreatureType is the creature family. Older catalogue sheets derive it from Ref.
	CreatureType string `json:"creature_type,omitempty"`
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

	// Experience is what this monster is worth in experience points, authored
	// on the stat block: the 2014 SRD value for the challenge rating the block
	// claims. It is divided among the party on the monster's fall.
	//
	// Zero is honest rather than absent-and-guessed: a monster nobody gave a
	// worth is worth nothing, and the party gets nothing for killing it. No
	// challenge rating is stored anywhere and no CR table exists here — the
	// number the table would produce is the number that is authored.
	Experience int `json:"experience,omitempty"`

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

	// Economy is what is left of this monster's turn: its action, its bonus
	// action, its reaction, and any capacity an action banked. NIL IS A FULL
	// TURN (one action, one bonus action, one reaction): a monster that has
	// spent nothing saves nothing here.
	//
	// It replaced ReactionSpent (2026-11-26, rpg-project#1131): the reaction is
	// one of the three slots, and two records of it would be two truths. A blob
	// saved before that, carrying "reaction_spent": true, loads as a full turn;
	// saved sessions do not outlive a server build today.
	Economy *combat.ActionEconomy `json:"economy,omitempty"`
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

// SlotsLeft reports how many of a turn's slots this saved sheet has left,
// answering from Economy, or from a full turn when Economy is nil. It reads a
// stored sheet without loading it. A slot that is not one of the three answers
// zero.
func (d *Data) SlotsLeft(slot coreCombat.ActionType) int {
	if d.Economy == nil {
		return combat.NewActionEconomy().SlotsLeft(slot)
	}

	return d.Economy.SlotsLeft(slot)
}
