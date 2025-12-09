// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// MonsterActionInput provides everything a monster action needs to execute.
// This is the input type for core.Action[MonsterActionInput].
//
//nolint:revive // Name matches design doc, stuttering acceptable for clarity
type MonsterActionInput struct {
	// Event bus for publishing attacks, damage, conditions
	Bus events.EventBus

	// What the monster perceives (targets, distances, cover)
	Perception *PerceptionData

	// Current conditions on the monster
	Conditions []dnd5eEvents.ConditionBehavior

	// Action economy (for tracking what resources have been used)
	ActionEconomy *combat.ActionEconomy

	// Target selection (if action needs a target)
	Target core.Entity

	// Dice roller (for attacks, damage)
	Roller dice.Roller
}

// MonsterAction extends core.Action with behavior scoring.
// Monster actions implement this interface to participate in the TakeTurn loop.
//
//nolint:revive // Name matches design doc, stuttering acceptable for clarity
type MonsterAction interface {
	core.Action[MonsterActionInput]

	// Cost returns the action economy cost of this action
	Cost() ActionCost

	// Score returns how desirable this action is in the current situation.
	// Higher scores mean the action is more desirable.
	Score(monster *Monster, perception *PerceptionData) int

	// ActionType returns the category of action for target selection
	ActionType() ActionType
}

// PerceptionData represents what the monster perceives about the battlefield.
// Built from room/spatial data at the start of each turn.
type PerceptionData struct {
	// Monster's current position
	MyPosition Position

	// Perceived enemies sorted by distance (closest first)
	Enemies []PerceivedEntity

	// Perceived allies
	Allies []PerceivedEntity
}

// Position represents a point in the combat space
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// PerceivedEntity represents an entity the monster can perceive
type PerceivedEntity struct {
	Entity   core.Entity
	Position Position
	Distance int  // Distance in feet
	Adjacent bool // Within 5 feet
}

// HasAdjacentEnemy returns true if any enemy is within melee range
func (p *PerceptionData) HasAdjacentEnemy() bool {
	for _, e := range p.Enemies {
		if e.Adjacent {
			return true
		}
	}
	return false
}

// AdjacentEnemyCount returns the number of enemies within melee range
func (p *PerceptionData) AdjacentEnemyCount() int {
	count := 0
	for _, e := range p.Enemies {
		if e.Adjacent {
			count++
		}
	}
	return count
}

// ClosestEnemy returns the nearest enemy, or nil if none
func (p *PerceptionData) ClosestEnemy() *PerceivedEntity {
	if len(p.Enemies) == 0 {
		return nil
	}
	return &p.Enemies[0]
}

// TurnInput represents everything the game server provides for a monster's turn
type TurnInput struct {
	Bus           events.EventBus
	ActionEconomy *combat.ActionEconomy
	Perception    *PerceptionData
	Roller        dice.Roller
}

// TurnResult represents the outcome of a monster's turn
type TurnResult struct {
	MonsterID string
	Actions   []ExecutedAction
	Movement  []Position
}

// ExecutedAction records a single action taken during a turn
type ExecutedAction struct {
	ActionID   string
	ActionType ActionType
	TargetID   string
	Success    bool
	Details    any // Attack result, healing amount, etc.
}
