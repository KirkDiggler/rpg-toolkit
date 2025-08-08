package combat

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/game"
)

// CombatStateData contains all information needed to persist and reconstruct combat state.
// This follows the established data pattern for serialization and loading.
type CombatStateData struct {
	// ID is the unique identifier for this combat encounter
	ID string `json:"id"`

	// Name is a human-readable name for this combat
	Name string `json:"name,omitempty"`

	// Status indicates the current state of combat
	Status CombatStatus `json:"status"`

	// Round tracks the current round number (1-based)
	Round int `json:"round"`

	// TurnIndex tracks whose turn it is (0-based index into InitiativeOrder)
	TurnIndex int `json:"turn_index"`

	// InitiativeOrder contains combatants ordered by initiative (highest first)
	InitiativeOrder []InitiativeEntry `json:"initiative_order"`

	// Combatants maps entity IDs to their combat data
	Combatants map[string]CombatantData `json:"combatants"`

	// Settings contains combat configuration
	Settings CombatSettings `json:"settings"`

	// CreatedAt timestamp when combat was created
	CreatedAt int64 `json:"created_at"`

	// StartedAt timestamp when combat actually began
	StartedAt int64 `json:"started_at,omitempty"`

	// EndedAt timestamp when combat ended
	EndedAt int64 `json:"ended_at,omitempty"`
}

// CombatStatus represents the current state of combat
type CombatStatus string

const (
	CombatStatusPending   CombatStatus = "pending"   // Created but not started
	CombatStatusActive    CombatStatus = "active"    // Combat in progress
	CombatStatusPaused    CombatStatus = "paused"    // Temporarily suspended
	CombatStatusCompleted CombatStatus = "completed" // Finished normally
	CombatStatusAbandoned CombatStatus = "abandoned" // Ended abnormally
)

// InitiativeEntry represents a single combatant's initiative data
type InitiativeEntry struct {
	// EntityID is the unique identifier of the combatant
	EntityID string `json:"entity_id"`

	// Roll is the d20 result (1-20)
	Roll int `json:"roll"`

	// Modifier is the initiative modifier (typically DEX modifier)
	Modifier int `json:"modifier"`

	// Total is the final initiative value (Roll + Modifier)
	Total int `json:"total"`

	// DexterityScore is used for tie-breaking
	DexterityScore int `json:"dexterity_score"`

	// TieBreaker is an additional value for resolving ties (DM input)
	TieBreaker int `json:"tie_breaker,omitempty"`

	// Active indicates if this combatant is still participating
	Active bool `json:"active"`
}

// CombatantData contains combat-relevant data for a single combatant
type CombatantData struct {
	// EntityID references the combatant entity
	EntityID string `json:"entity_id"`

	// EntityType for quick identification
	EntityType string `json:"entity_type"`

	// Initiative information
	Initiative InitiativeEntry `json:"initiative"`

	// Combat stats (cached for performance)
	HitPoints    int `json:"hit_points"`
	MaxHitPoints int `json:"max_hit_points"`
	ArmorClass   int `json:"armor_class"`

	// Conditions and effects
	Conditions []string `json:"conditions,omitempty"`
	Effects    []string `json:"effects,omitempty"`

	// Turn tracking
	TurnsTaken     int          `json:"turns_taken"`
	LastActionTurn int          `json:"last_action_turn,omitempty"`
	ActionsUsed    []ActionType `json:"actions_used,omitempty"`

	// Status flags
	IsActive      bool `json:"is_active"`
	IsUnconscious bool `json:"is_unconscious"`
	IsDefeated    bool `json:"is_defeated"`
	HasActed      bool `json:"has_acted"` // This round
}

// ActionType represents types of actions that can be taken
type ActionType string

const (
	ActionTypeAction      ActionType = "action"
	ActionTypeBonusAction ActionType = "bonus_action"
	ActionTypeReaction    ActionType = "reaction"
	ActionTypeMovement    ActionType = "movement"
	ActionTypeFreeAction  ActionType = "free_action"
)

// CombatSettings contains configuration for combat behavior
type CombatSettings struct {
	// AutoAdvanceTurns automatically moves to next turn
	AutoAdvanceTurns bool `json:"auto_advance_turns"`

	// MaxRounds limits combat duration (0 = unlimited)
	MaxRounds int `json:"max_rounds"`

	// InitiativeRollMode controls how initiative is rolled
	InitiativeRollMode InitiativeRollMode `json:"initiative_roll_mode"`

	// TieBreakingMode controls how ties are resolved
	TieBreakingMode TieBreakingMode `json:"tie_breaking_mode"`

	// AllowDelayedEntry permits joining combat after it starts
	AllowDelayedEntry bool `json:"allow_delayed_entry"`
}

// InitiativeRollMode controls how initiative is determined
type InitiativeRollMode string

const (
	InitiativeRollModeRoll   InitiativeRollMode = "roll"   // Roll d20 + modifier
	InitiativeRollModeStatic InitiativeRollMode = "static" // Use 10 + modifier
	InitiativeRollModeManual InitiativeRollMode = "manual" // DM sets values
)

// TieBreakingMode controls how initiative ties are resolved
type TieBreakingMode string

const (
	TieBreakingModeDexterity TieBreakingMode = "dexterity" // Higher DEX wins
	TieBreakingModeDM        TieBreakingMode = "dm"        // DM decides
	TieBreakingModeRoll      TieBreakingMode = "roll"      // Re-roll d20
)

// Combatant interface defines what entities need to participate in combat
type Combatant interface {
	core.Entity

	// GetDexterityModifier returns the entity's DEX modifier for initiative
	GetDexterityModifier() int

	// GetDexterityScore returns the entity's DEX score for tie-breaking
	GetDexterityScore() int

	// GetArmorClass returns the entity's AC
	GetArmorClass() int

	// GetHitPoints returns current hit points
	GetHitPoints() int

	// GetMaxHitPoints returns maximum hit points
	GetMaxHitPoints() int

	// IsConscious returns true if the entity can act
	IsConscious() bool

	// IsDefeated returns true if the entity is out of combat
	IsDefeated() bool
}

// RollInitiativeInput contains parameters for rolling initiative
type RollInitiativeInput struct {
	// Combatants to roll initiative for
	Combatants []Combatant `json:"combatants"`

	// Roller for random number generation
	Roller dice.Roller `json:"-"`

	// Settings for how to handle the roll
	RollMode InitiativeRollMode `json:"roll_mode"`

	// Manual values if using manual mode (EntityID -> InitiativeValue)
	ManualValues map[string]int `json:"manual_values,omitempty"`
}

// RollInitiativeOutput contains the results of rolling initiative
type RollInitiativeOutput struct {
	// InitiativeEntries ordered by total (highest first)
	InitiativeEntries []InitiativeEntry `json:"initiative_entries"`

	// UnresolvedTies contains groups of tied combatants (if any)
	UnresolvedTies [][]string `json:"unresolved_ties,omitempty"`

	// RollResults contains individual roll details for transparency
	RollResults map[string]InitiativeRollResult `json:"roll_results"`
}

// InitiativeRollResult contains detailed information about a single initiative roll
type InitiativeRollResult struct {
	EntityID       string `json:"entity_id"`
	Roll           int    `json:"roll"`
	Modifier       int    `json:"modifier"`
	Total          int    `json:"total"`
	DexterityScore int    `json:"dexterity_score"`
	WasManual      bool   `json:"was_manual"`
}

// ResolveTiesInput contains parameters for resolving initiative ties
type ResolveTiesInput struct {
	// TiedGroups contains groups of tied combatants
	TiedGroups [][]string `json:"tied_groups"`

	// InitiativeEntries to modify
	InitiativeEntries []InitiativeEntry `json:"initiative_entries"`

	// TieBreakingMode determines resolution method
	TieBreakingMode TieBreakingMode `json:"tie_breaking_mode"`

	// ManualOrder if using DM resolution (EntityID -> Priority)
	ManualOrder map[string]int `json:"manual_order,omitempty"`

	// Roller for re-rolling if needed
	Roller dice.Roller `json:"-"`
}

// ResolveTiesOutput contains the results of tie resolution
type ResolveTiesOutput struct {
	// ResolvedEntries with ties broken
	ResolvedEntries []InitiativeEntry `json:"resolved_entries"`

	// RemainingTies if some could not be resolved
	RemainingTies [][]string `json:"remaining_ties,omitempty"`
}

// SortInitiativeEntries sorts initiative entries by total, then by DEX score, then by tie breaker
func SortInitiativeEntries(entries []InitiativeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		// Higher total initiative wins
		if entries[i].Total != entries[j].Total {
			return entries[i].Total > entries[j].Total
		}

		// If tied, higher DEX score wins
		if entries[i].DexterityScore != entries[j].DexterityScore {
			return entries[i].DexterityScore > entries[j].DexterityScore
		}

		// If still tied, higher tie breaker wins
		return entries[i].TieBreaker > entries[j].TieBreaker
	})
}

// FindTiedGroups identifies groups of combatants with identical initiative totals
func FindTiedGroups(entries []InitiativeEntry) [][]string {
	tiedGroups := make([][]string, 0)

	i := 0
	for i < len(entries) {
		currentTotal := entries[i].Total
		group := []string{entries[i].EntityID}

		// Look for others with same total
		j := i + 1
		for j < len(entries) && entries[j].Total == currentTotal {
			group = append(group, entries[j].EntityID)
			j++
		}

		// Only add if there's actually a tie
		if len(group) > 1 {
			tiedGroups = append(tiedGroups, group)
		}

		i = j
	}

	return tiedGroups
}

// LoadCombatStateFromContext creates a CombatState from data using the GameContext pattern.
// This allows combat to integrate with the event system and other game infrastructure.
func LoadCombatStateFromContext(ctx context.Context, gameCtx game.Context[CombatStateData]) (*CombatState, error) {
	data := gameCtx.Data()
	eventBus := gameCtx.EventBus()

	// Validate required data
	if data.ID == "" {
		return nil, fmt.Errorf("combat ID is required")
	}

	// Create the combat state
	combat := NewCombatState(CombatStateConfig{
		ID:       data.ID,
		Name:     data.Name,
		EventBus: eventBus,
		Settings: data.Settings,
	})

	// Restore state
	combat.status = data.Status
	combat.round = data.Round
	combat.turnIndex = data.TurnIndex
	combat.initiativeOrder = make([]InitiativeEntry, len(data.InitiativeOrder))
	copy(combat.initiativeOrder, data.InitiativeOrder)

	// Restore combatants
	for entityID, combatantData := range data.Combatants {
		combat.combatants[entityID] = combatantData
	}

	// Restore timestamps
	combat.createdAt = time.Unix(data.CreatedAt, 0)
	combat.startedAt = time.Unix(data.StartedAt, 0)
	combat.endedAt = time.Unix(data.EndedAt, 0)

	return combat, nil
}
