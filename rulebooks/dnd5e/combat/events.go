// Package combat provides D&D 5e combat mechanics following the toolkit's domain patterns
package combat

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Combat event constants following the toolkit's dot notation pattern
const (
	// Combat lifecycle events
	EventCombatStarted = "dnd5e.combat.started"
	EventCombatEnded   = "dnd5e.combat.ended"
	EventCombatPaused  = "dnd5e.combat.paused"
	EventCombatResumed = "dnd5e.combat.resumed"

	// Initiative events
	EventInitiativeRolled = "dnd5e.combat.initiative.rolled"
	EventInitiativeOrder  = "dnd5e.combat.initiative.order_set"

	// Turn events
	EventTurnStarted  = "dnd5e.combat.turn.started"
	EventTurnEnded    = "dnd5e.combat.turn.ended"
	EventRoundStarted = "dnd5e.combat.round.started"
	EventRoundEnded   = "dnd5e.combat.round.ended"

	// Combatant events
	EventCombatantAdded   = "dnd5e.combat.combatant.added"
	EventCombatantRemoved = "dnd5e.combat.combatant.removed"
	EventCombatantUpdated = "dnd5e.combat.combatant.updated"
)

// CombatStartedData contains data for combat start events
type CombatStartedData struct {
	CombatID        string   `json:"combat_id"`
	Combatants      []string `json:"combatants"`       // Entity IDs
	InitiativeOrder []string `json:"initiative_order"` // Ordered by initiative
}

// CombatEndedData contains data for combat end events
type CombatEndedData struct {
	CombatID string `json:"combat_id"`
	Winner   string `json:"winner,omitempty"` // Entity ID or faction
	Duration int    `json:"duration"`         // Total rounds
	Reason   string `json:"reason,omitempty"` // Why combat ended
}

// InitiativeRolledData contains data for initiative roll events
type InitiativeRolledData struct {
	CombatID string      `json:"combat_id"`
	Entity   core.Entity `json:"entity"`
	Roll     int         `json:"roll"`      // d20 roll
	Modifier int         `json:"modifier"`  // DEX modifier
	Total    int         `json:"total"`     // Roll + modifier
	DexScore int         `json:"dex_score"` // For tie-breaking
}

// InitiativeOrderData contains data for initiative order events
type InitiativeOrderData struct {
	CombatID        string   `json:"combat_id"`
	InitiativeOrder []string `json:"initiative_order"` // Entity IDs in turn order
	TiesResolved    bool     `json:"ties_resolved"`
}

// TurnStartedData contains data for turn start events
type TurnStartedData struct {
	CombatID   string      `json:"combat_id"`
	Entity     core.Entity `json:"entity"`
	Round      int         `json:"round"`
	TurnNumber int         `json:"turn_number"` // Turn within this round
	Initiative int         `json:"initiative"`
}

// TurnEndedData contains data for turn end events
type TurnEndedData struct {
	CombatID    string      `json:"combat_id"`
	Entity      core.Entity `json:"entity"`
	Round       int         `json:"round"`
	TurnNumber  int         `json:"turn_number"`
	ActionsUsed []string    `json:"actions_used,omitempty"` // Action types used
}

// RoundStartedData contains data for round start events
type RoundStartedData struct {
	CombatID     string `json:"combat_id"`
	Round        int    `json:"round"`
	Participants int    `json:"participants"` // Number of active combatants
}

// RoundEndedData contains data for round end events
type RoundEndedData struct {
	CombatID     string `json:"combat_id"`
	Round        int    `json:"round"`
	ActionsTotal int    `json:"actions_total"` // Total actions taken this round
}

// CombatantAddedData contains data for combatant addition events
type CombatantAddedData struct {
	CombatID      string      `json:"combat_id"`
	Entity        core.Entity `json:"entity"`
	Initiative    int         `json:"initiative,omitempty"` // If joining mid-combat
	JoinedAtRound int         `json:"joined_at_round"`
}

// CombatantRemovedData contains data for combatant removal events
type CombatantRemovedData struct {
	CombatID       string      `json:"combat_id"`
	Entity         core.Entity `json:"entity"`
	Reason         string      `json:"reason"` // "defeated", "fled", "dismissed"
	RemovedAtRound int         `json:"removed_at_round"`
}

// CombatantUpdatedData contains data for combatant update events
type CombatantUpdatedData struct {
	CombatID   string      `json:"combat_id"`
	Entity     core.Entity `json:"entity"`
	Changes    []string    `json:"changes"`     // What was updated
	UpdateType string      `json:"update_type"` // "stats", "conditions", "position"
}
