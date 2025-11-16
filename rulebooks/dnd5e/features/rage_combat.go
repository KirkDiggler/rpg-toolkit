package features

import (
	"context"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// RageCombatIntegration wires the Rage feature to combat damage chains
// Purpose: When a barbarian activates Rage, their attacks deal bonus damage
// This is implemented by subscribing to DamageChain events and adding modifiers
type RageCombatIntegration struct {
	bus                   events.EventBus
	mu                    sync.RWMutex
	ragingEntities        map[string]int // entityID -> damage bonus
	conditionSubscription string         // Subscription ID for cleanup
}

// RageCombatIntegrationConfig provides configuration for the combat integration
type RageCombatIntegrationConfig struct {
	Bus events.EventBus
}

// NewRageCombatIntegration creates a new rage combat integration
// Call Start() to begin listening for rage conditions and wire up damage modifiers
func NewRageCombatIntegration(config RageCombatIntegrationConfig) *RageCombatIntegration {
	return &RageCombatIntegration{
		bus:            config.Bus,
		ragingEntities: make(map[string]int),
	}
}

// Start begins listening for rage condition events and wires up damage chain modifiers
// This should be called during game initialization
func (r *RageCombatIntegration) Start(ctx context.Context) error {
	// Subscribe to condition applied events to track raging entities
	conditionTopic := dnd5e.ConditionAppliedTopic.On(r.bus)
	subID, err := conditionTopic.Subscribe(ctx, r.handleConditionApplied)
	if err != nil {
		return err
	}
	r.conditionSubscription = subID

	// Subscribe to damage chain to add rage bonus
	damageChain := combat.DamageChain.On(r.bus)
	_, err = damageChain.SubscribeWithChain(ctx, r.handleDamageChain)
	if err != nil {
		// Clean up condition subscription if damage chain fails
		_ = conditionTopic.Unsubscribe(ctx, subID)
		return err
	}

	return nil
}

// Stop cleans up event subscriptions
func (r *RageCombatIntegration) Stop(ctx context.Context) error {
	if r.conditionSubscription != "" {
		conditionTopic := dnd5e.ConditionAppliedTopic.On(r.bus)
		return conditionTopic.Unsubscribe(ctx, r.conditionSubscription)
	}
	return nil
}

// handleConditionApplied tracks when entities start raging
func (r *RageCombatIntegration) handleConditionApplied(_ context.Context, event dnd5e.ConditionAppliedEvent) error {
	// Only care about Rage conditions
	if event.Type != dnd5e.ConditionRaging {
		return nil
	}

	// Extract damage bonus from rage data
	rageData, ok := event.Data.(RageEventData)
	if !ok {
		return nil // Not a rage event or data missing
	}

	// Track this entity as raging with their damage bonus
	r.mu.Lock()
	r.ragingEntities[event.Target.GetID()] = rageData.DamageBonus
	r.mu.Unlock()

	return nil
}

// handleDamageChain adds rage damage bonus to attacks made by raging entities
func (r *RageCombatIntegration) handleDamageChain(
	_ context.Context,
	event combat.DamageChainEvent,
	c chain.Chain[combat.DamageChainEvent],
) (chain.Chain[combat.DamageChainEvent], error) {
	// Check if the attacker is raging
	r.mu.RLock()
	damageBonus, isRaging := r.ragingEntities[event.AttackerID]
	r.mu.RUnlock()

	if !isRaging {
		return c, nil // Not raging, no modifier to add
	}

	// Add rage damage modifier in the StageFeatures stage
	err := c.Add(
		combat.StageFeatures,
		"rage",
		func(_ context.Context, e combat.DamageChainEvent) (combat.DamageChainEvent, error) {
			e.DamageBonus += damageBonus
			return e, nil
		},
	)
	if err != nil {
		return c, err
	}

	return c, nil
}

// ClearRage removes the rage condition from an entity
// This would typically be called when rage ends (after combat, or manually ended)
func (r *RageCombatIntegration) ClearRage(entityID string) {
	r.mu.Lock()
	delete(r.ragingEntities, entityID)
	r.mu.Unlock()
}

// IsRaging returns true if the specified entity is currently raging
func (r *RageCombatIntegration) IsRaging(entityID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, raging := r.ragingEntities[entityID]
	return raging
}

// GetRageDamageBonus returns the current rage damage bonus for an entity
// Returns 0 if the entity is not raging
func (r *RageCombatIntegration) GetRageDamageBonus(entityID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ragingEntities[entityID]
}
