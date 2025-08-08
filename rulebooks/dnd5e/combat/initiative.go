package combat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// CombatState implements rich domain logic for D&D 5e combat encounters.
// It follows the toolkit pattern of rich domain objects with methods and behavior.
type CombatState struct {
	// Core identity
	id       string
	name     string
	eventBus events.EventBus

	// Combat state tracking
	status          CombatStatus
	round           int
	turnIndex       int
	initiativeOrder []InitiativeEntry
	combatants      map[string]CombatantData
	settings        CombatSettings

	// Timing
	createdAt time.Time
	startedAt time.Time
	endedAt   time.Time

	// Services
	roller dice.Roller

	// Mutex for thread-safe access
	mutex sync.RWMutex
}

// CombatStateConfig holds configuration for creating combat state
type CombatStateConfig struct {
	ID       string
	Name     string
	EventBus events.EventBus
	Settings CombatSettings
	Roller   dice.Roller
}

// NewCombatState creates a new combat state with event integration
func NewCombatState(config CombatStateConfig) *CombatState {
	// Use default roller if none provided
	roller := config.Roller
	if roller == nil {
		roller = dice.DefaultRoller
	}

	// Default settings if none provided
	settings := config.Settings
	if settings.InitiativeRollMode == "" {
		settings.InitiativeRollMode = InitiativeRollModeRoll
	}
	if settings.TieBreakingMode == "" {
		settings.TieBreakingMode = TieBreakingModeDexterity
	}

	combat := &CombatState{
		id:              config.ID,
		name:            config.Name,
		eventBus:        config.EventBus,
		status:          CombatStatusPending,
		round:           0,
		turnIndex:       0,
		initiativeOrder: make([]InitiativeEntry, 0),
		combatants:      make(map[string]CombatantData),
		settings:        settings,
		createdAt:       time.Now(),
		roller:          roller,
	}

	return combat
}

// GetID returns the combat's unique identifier (implements core.Entity)
func (c *CombatState) GetID() string {
	return c.id
}

// GetType returns the combat's type (implements core.Entity)
func (c *CombatState) GetType() string {
	return "combat_encounter"
}

// GetName returns the combat's name
func (c *CombatState) GetName() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.name
}

// GetStatus returns the current combat status
func (c *CombatState) GetStatus() CombatStatus {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.status
}

// GetRound returns the current round number
func (c *CombatState) GetRound() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.round
}

// GetCurrentTurn returns information about whose turn it is
func (c *CombatState) GetCurrentTurn() (*InitiativeEntry, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.status != CombatStatusActive {
		return nil, fmt.Errorf("combat is not active")
	}

	if len(c.initiativeOrder) == 0 {
		return nil, fmt.Errorf("no combatants in initiative order")
	}

	if c.turnIndex < 0 || c.turnIndex >= len(c.initiativeOrder) {
		return nil, fmt.Errorf("invalid turn index: %d", c.turnIndex)
	}

	entry := c.initiativeOrder[c.turnIndex]
	return &entry, nil
}

// AddCombatant adds a combatant to the encounter
func (c *CombatState) AddCombatant(combatant Combatant) error {
	if combatant == nil {
		return fmt.Errorf("combatant cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	entityID := combatant.GetID()

	// Check if already added
	if _, exists := c.combatants[entityID]; exists {
		return fmt.Errorf("combatant %s already in combat", entityID)
	}

	// Create combatant data
	combatantData := CombatantData{
		EntityID:      entityID,
		EntityType:    combatant.GetType(),
		HitPoints:     combatant.GetHitPoints(),
		MaxHitPoints:  combatant.GetMaxHitPoints(),
		ArmorClass:    combatant.GetArmorClass(),
		Conditions:    make([]string, 0),
		Effects:       make([]string, 0),
		ActionsUsed:   make([]ActionType, 0),
		IsActive:      true,
		IsUnconscious: !combatant.IsConscious(),
		IsDefeated:    combatant.IsDefeated(),
		HasActed:      false,
	}

	c.combatants[entityID] = combatantData

	// Emit event
	if c.eventBus != nil {
		event := events.NewGameEvent(EventCombatantAdded, combatant, nil)
		eventData := CombatantAddedData{
			CombatID:      c.id,
			Entity:        combatant,
			JoinedAtRound: c.round,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// RemoveCombatant removes a combatant from the encounter
func (c *CombatState) RemoveCombatant(entityID string, reason string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if combatant exists
	combatantData, exists := c.combatants[entityID]
	if !exists {
		return fmt.Errorf("combatant %s not found", entityID)
	}

	// Mark as inactive
	combatantData.IsActive = false
	c.combatants[entityID] = combatantData

	// Remove from initiative order
	newOrder := make([]InitiativeEntry, 0)
	wasInOrder := false
	removedIndex := -1

	for i, entry := range c.initiativeOrder {
		if entry.EntityID == entityID {
			wasInOrder = true
			removedIndex = i
			entry.Active = false
			// Keep in order but mark inactive for history
			newOrder = append(newOrder, entry)
		} else {
			newOrder = append(newOrder, entry)
		}
	}

	c.initiativeOrder = newOrder

	// Adjust turn index if necessary
	if wasInOrder && removedIndex <= c.turnIndex && c.turnIndex > 0 {
		c.turnIndex--
	}

	// Emit event
	if c.eventBus != nil {
		// Create a minimal entity for the event
		entity := &simpleEntity{id: entityID, entityType: combatantData.EntityType}
		event := events.NewGameEvent(EventCombatantRemoved, entity, nil)
		eventData := CombatantRemovedData{
			CombatID:       c.id,
			Entity:         entity,
			Reason:         reason,
			RemovedAtRound: c.round,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// RollInitiative rolls initiative for all combatants
func (c *CombatState) RollInitiative(input *RollInitiativeInput) (*RollInitiativeOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if len(input.Combatants) == 0 {
		return nil, fmt.Errorf("no combatants to roll initiative for")
	}

	roller := input.Roller
	if roller == nil {
		roller = c.roller
	}

	rollMode := input.RollMode
	if rollMode == "" {
		rollMode = c.settings.InitiativeRollMode
	}

	// Roll for each combatant
	initiativeEntries := make([]InitiativeEntry, 0, len(input.Combatants))
	rollResults := make(map[string]InitiativeRollResult)

	for _, combatant := range input.Combatants {
		entityID := combatant.GetID()
		modifier := combatant.GetDexterityModifier()
		dexScore := combatant.GetDexterityScore()

		var roll, total int
		wasManual := false

		switch rollMode {
		case InitiativeRollModeRoll:
			diceRoll, err := roller.Roll(20)
			if err != nil {
				return nil, fmt.Errorf("failed to roll initiative for %s: %w", entityID, err)
			}
			roll = diceRoll
			total = roll + modifier

		case InitiativeRollModeStatic:
			roll = 10
			total = roll + modifier

		case InitiativeRollModeManual:
			manualValue, exists := input.ManualValues[entityID]
			if !exists {
				return nil, fmt.Errorf("manual value not provided for %s", entityID)
			}
			roll = 0 // Not applicable for manual
			total = manualValue
			wasManual = true

		default:
			return nil, fmt.Errorf("unknown initiative roll mode: %s", rollMode)
		}

		entry := InitiativeEntry{
			EntityID:       entityID,
			Roll:           roll,
			Modifier:       modifier,
			Total:          total,
			DexterityScore: dexScore,
			Active:         true,
		}

		initiativeEntries = append(initiativeEntries, entry)

		rollResult := InitiativeRollResult{
			EntityID:       entityID,
			Roll:           roll,
			Modifier:       modifier,
			Total:          total,
			DexterityScore: dexScore,
			WasManual:      wasManual,
		}
		rollResults[entityID] = rollResult

		// Emit individual roll event
		if c.eventBus != nil {
			event := events.NewGameEvent(EventInitiativeRolled, combatant, nil)
			eventData := InitiativeRolledData{
				CombatID: c.id,
				Entity:   combatant,
				Roll:     roll,
				Modifier: modifier,
				Total:    total,
				DexScore: dexScore,
			}
			event.Context().Set("data", eventData)
			_ = c.eventBus.Publish(context.Background(), event)
		}
	}

	// Sort by initiative
	SortInitiativeEntries(initiativeEntries)

	// Find tied groups
	tiedGroups := FindTiedGroups(initiativeEntries)

	output := &RollInitiativeOutput{
		InitiativeEntries: initiativeEntries,
		UnresolvedTies:    tiedGroups,
		RollResults:       rollResults,
	}

	// Store the order (even if there are ties)
	c.initiativeOrder = initiativeEntries

	// Emit initiative order event
	if c.eventBus != nil {
		entityIDs := make([]string, len(initiativeEntries))
		for i, entry := range initiativeEntries {
			entityIDs[i] = entry.EntityID
		}

		event := events.NewGameEvent(EventInitiativeOrder, nil, nil)
		eventData := InitiativeOrderData{
			CombatID:        c.id,
			InitiativeOrder: entityIDs,
			TiesResolved:    len(tiedGroups) == 0,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	return output, nil
}

// ResolveTies resolves initiative ties using the specified method
func (c *CombatState) ResolveTies(input *ResolveTiesInput) (*ResolveTiesOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	tieBreakingMode := input.TieBreakingMode
	if tieBreakingMode == "" {
		tieBreakingMode = c.settings.TieBreakingMode
	}

	resolvedEntries := make([]InitiativeEntry, len(input.InitiativeEntries))
	copy(resolvedEntries, input.InitiativeEntries)

	remainingTies := make([][]string, 0)

	for _, group := range input.TiedGroups {
		switch tieBreakingMode {
		case TieBreakingModeDexterity:
			// Already handled by SortInitiativeEntries - no additional work needed
			// Ties that remain after DEX comparison are truly tied
			if c.stillTied(resolvedEntries, group) {
				remainingTies = append(remainingTies, group)
			}

		case TieBreakingModeDM:
			if input.ManualOrder == nil {
				remainingTies = append(remainingTies, group)
				continue
			}

			// Apply manual tie breaker values
			for i := range resolvedEntries {
				for _, entityID := range group {
					if resolvedEntries[i].EntityID == entityID {
						if tieBreaker, exists := input.ManualOrder[entityID]; exists {
							resolvedEntries[i].TieBreaker = tieBreaker
						}
					}
				}
			}

		case TieBreakingModeRoll:
			roller := input.Roller
			if roller == nil {
				roller = c.roller
			}

			// Re-roll for tied combatants
			for i := range resolvedEntries {
				for _, entityID := range group {
					if resolvedEntries[i].EntityID == entityID {
						reroll, err := roller.Roll(20)
						if err != nil {
							return nil, fmt.Errorf("failed to re-roll for %s: %w", entityID, err)
						}
						resolvedEntries[i].TieBreaker = reroll
					}
				}
			}

		default:
			return nil, fmt.Errorf("unknown tie breaking mode: %s", tieBreakingMode)
		}
	}

	// Re-sort with tie breakers applied
	SortInitiativeEntries(resolvedEntries)

	// Update our initiative order
	c.initiativeOrder = resolvedEntries

	output := &ResolveTiesOutput{
		ResolvedEntries: resolvedEntries,
		RemainingTies:   remainingTies,
	}

	// Emit updated initiative order event
	if c.eventBus != nil {
		entityIDs := make([]string, len(resolvedEntries))
		for i, entry := range resolvedEntries {
			entityIDs[i] = entry.EntityID
		}

		event := events.NewGameEvent(EventInitiativeOrder, nil, nil)
		eventData := InitiativeOrderData{
			CombatID:        c.id,
			InitiativeOrder: entityIDs,
			TiesResolved:    len(remainingTies) == 0,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	return output, nil
}

// stillTied checks if a group of combatants are still tied after sorting
func (c *CombatState) stillTied(entries []InitiativeEntry, group []string) bool {
	if len(group) <= 1 {
		return false
	}

	// Find the entries for this group
	var groupEntries []InitiativeEntry
	for _, entry := range entries {
		for _, entityID := range group {
			if entry.EntityID == entityID {
				groupEntries = append(groupEntries, entry)
				break
			}
		}
	}

	if len(groupEntries) <= 1 {
		return false
	}

	// Check if they all have the same total and DEX score
	firstEntry := groupEntries[0]
	for i := 1; i < len(groupEntries); i++ {
		if groupEntries[i].Total != firstEntry.Total ||
			groupEntries[i].DexterityScore != firstEntry.DexterityScore {
			return false
		}
	}

	return true
}

// StartCombat begins the combat encounter
func (c *CombatState) StartCombat() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.status != CombatStatusPending {
		return fmt.Errorf("combat cannot be started from status: %s", c.status)
	}

	if len(c.initiativeOrder) == 0 {
		return fmt.Errorf("cannot start combat without initiative order")
	}

	c.status = CombatStatusActive
	c.round = 1
	c.turnIndex = 0
	c.startedAt = time.Now()

	// Emit combat started event
	if c.eventBus != nil {
		combatantIDs := make([]string, 0, len(c.combatants))
		initiativeOrder := make([]string, len(c.initiativeOrder))

		for entityID := range c.combatants {
			combatantIDs = append(combatantIDs, entityID)
		}

		for i, entry := range c.initiativeOrder {
			initiativeOrder[i] = entry.EntityID
		}

		event := events.NewGameEvent(EventCombatStarted, nil, nil)
		eventData := CombatStartedData{
			CombatID:        c.id,
			Combatants:      combatantIDs,
			InitiativeOrder: initiativeOrder,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	// Start the first turn
	return c.startCurrentTurn()
}

// NextTurn advances to the next combatant's turn
func (c *CombatState) NextTurn() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.status != CombatStatusActive {
		return fmt.Errorf("combat is not active")
	}

	// End current turn
	if err := c.endCurrentTurn(); err != nil {
		return fmt.Errorf("failed to end current turn: %w", err)
	}

	// Advance turn index
	c.turnIndex++

	// Check if we need to start a new round
	if c.turnIndex >= len(c.initiativeOrder) {
		c.turnIndex = 0
		c.round++

		// Emit round events
		if c.eventBus != nil {
			// End previous round
			prevRound := c.round - 1
			if prevRound > 0 {
				event := events.NewGameEvent(EventRoundEnded, nil, nil)
				eventData := RoundEndedData{
					CombatID: c.id,
					Round:    prevRound,
				}
				event.Context().Set("data", eventData)
				_ = c.eventBus.Publish(context.Background(), event)
			}

			// Start new round
			activeCount := 0
			for _, combatant := range c.combatants {
				if combatant.IsActive {
					activeCount++
				}
			}

			event := events.NewGameEvent(EventRoundStarted, nil, nil)
			eventData := RoundStartedData{
				CombatID:     c.id,
				Round:        c.round,
				Participants: activeCount,
			}
			event.Context().Set("data", eventData)
			_ = c.eventBus.Publish(context.Background(), event)
		}

		// Reset has acted flags
		for entityID, combatant := range c.combatants {
			combatant.HasActed = false
			c.combatants[entityID] = combatant
		}
	}

	// Start the next turn
	return c.startCurrentTurn()
}

// startCurrentTurn begins the current combatant's turn
func (c *CombatState) startCurrentTurn() error {
	if len(c.initiativeOrder) == 0 {
		return fmt.Errorf("no combatants in initiative order")
	}

	currentEntry := c.initiativeOrder[c.turnIndex]

	// Skip inactive combatants
	if !currentEntry.Active {
		if c.turnIndex < len(c.initiativeOrder)-1 {
			c.turnIndex++
			return c.startCurrentTurn() // Recursively try next
		} else {
			// End of round with no active combatants
			return fmt.Errorf("no active combatants remaining")
		}
	}

	// Mark combatant as having started their turn
	if combatantData, exists := c.combatants[currentEntry.EntityID]; exists {
		combatantData.HasActed = true
		combatantData.TurnsTaken++
		combatantData.ActionsUsed = make([]ActionType, 0) // Reset actions for new turn
		c.combatants[currentEntry.EntityID] = combatantData
	}

	// Emit turn started event
	if c.eventBus != nil {
		// Create a minimal entity for the event
		combatantData := c.combatants[currentEntry.EntityID]
		entity := &simpleEntity{id: currentEntry.EntityID, entityType: combatantData.EntityType}

		event := events.NewGameEvent(EventTurnStarted, entity, nil)
		eventData := TurnStartedData{
			CombatID:   c.id,
			Entity:     entity,
			Round:      c.round,
			TurnNumber: c.turnIndex + 1,
			Initiative: currentEntry.Total,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// endCurrentTurn ends the current combatant's turn
func (c *CombatState) endCurrentTurn() error {
	if len(c.initiativeOrder) == 0 {
		return fmt.Errorf("no combatants in initiative order")
	}

	currentEntry := c.initiativeOrder[c.turnIndex]

	// Update turn tracking
	if combatantData, exists := c.combatants[currentEntry.EntityID]; exists {
		combatantData.LastActionTurn = c.round
		c.combatants[currentEntry.EntityID] = combatantData
	}

	// Emit turn ended event
	if c.eventBus != nil {
		combatantData := c.combatants[currentEntry.EntityID]
		entity := &simpleEntity{id: currentEntry.EntityID, entityType: combatantData.EntityType}

		// Convert ActionType slice to string slice for event
		actionsUsed := make([]string, len(combatantData.ActionsUsed))
		for i, action := range combatantData.ActionsUsed {
			actionsUsed[i] = string(action)
		}

		event := events.NewGameEvent(EventTurnEnded, entity, nil)
		eventData := TurnEndedData{
			CombatID:    c.id,
			Entity:      entity,
			Round:       c.round,
			TurnNumber:  c.turnIndex + 1,
			ActionsUsed: actionsUsed,
		}
		event.Context().Set("data", eventData)
		_ = c.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// ToData converts the CombatState to CombatStateData for persistence
func (c *CombatState) ToData() CombatStateData {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Copy combatants map
	combatants := make(map[string]CombatantData)
	for k, v := range c.combatants {
		combatants[k] = v
	}

	// Copy initiative order
	initiativeOrder := make([]InitiativeEntry, len(c.initiativeOrder))
	copy(initiativeOrder, c.initiativeOrder)

	return CombatStateData{
		ID:              c.id,
		Name:            c.name,
		Status:          c.status,
		Round:           c.round,
		TurnIndex:       c.turnIndex,
		InitiativeOrder: initiativeOrder,
		Combatants:      combatants,
		Settings:        c.settings,
		CreatedAt:       c.createdAt.Unix(),
		StartedAt:       c.startedAt.Unix(),
		EndedAt:         c.endedAt.Unix(),
	}
}

// simpleEntity is a minimal implementation of core.Entity for events
type simpleEntity struct {
	id         string
	entityType string
}

func (e *simpleEntity) GetID() string   { return e.id }
func (e *simpleEntity) GetType() string { return e.entityType }
