// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SneakAttackData is the JSON structure for persisting sneak attack condition state
type SneakAttackData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
	Level       int       `json:"level"`
	DamageDice  int       `json:"damage_dice"`
}

// SneakAttackCondition represents the rogue's sneak attack feature.
// It adds extra damage dice when the rogue has advantage or an ally adjacent to the target.
// It implements the ConditionBehavior interface.
type SneakAttackCondition struct {
	CharacterID     string
	Level           int
	DamageDice      int  // Number of d6s to roll
	UsedThisTurn    bool // Sneak attack can only be used once per turn
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller
}

// Ensure SneakAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*SneakAttackCondition)(nil)

// SneakAttackInput provides configuration for creating a sneak attack condition
type SneakAttackInput struct {
	CharacterID string      // ID of the rogue
	Level       int         // Rogue level (determines number of dice)
	Roller      dice.Roller // Dice roller for rolling extra damage
}

// NewSneakAttackCondition creates a sneak attack condition from input
func NewSneakAttackCondition(input SneakAttackInput) *SneakAttackCondition {
	return &SneakAttackCondition{
		CharacterID: input.CharacterID,
		Level:       input.Level,
		DamageDice:  calculateSneakAttackDice(input.Level),
		roller:      input.Roller,
	}
}

// calculateSneakAttackDice determines number of d6s based on rogue level
// Sneak Attack starts at 1d6 at level 1 and increases by 1d6 every odd level
func calculateSneakAttackDice(level int) int {
	if level < 1 {
		return 0
	}
	return (level + 1) / 2 // 1d6 at 1, 2d6 at 3, 3d6 at 5, etc.
}

// IsApplied returns true if this condition is currently applied
func (s *SneakAttackCondition) IsApplied() bool {
	return s.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (s *SneakAttackCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if s.bus != nil {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "sneak attack condition already applied")
	}

	s.bus = bus

	// Subscribe to damage chain to add sneak attack dice
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, s.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, subID)

	// Subscribe to turn end to reset the once-per-turn flag
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)
	turnSubID, err := turnEndTopic.Subscribe(ctx, s.onTurnEnd)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to turn end")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, turnSubID)

	return nil
}

// Remove unsubscribes this condition from events
func (s *SneakAttackCondition) Remove(ctx context.Context, bus events.EventBus) error {
	for _, id := range s.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, id); err != nil {
			return rpgerr.Wrapf(err, "failed to unsubscribe %s", id)
		}
	}
	s.subscriptionIDs = nil
	s.bus = nil
	return nil
}

// onTurnEnd resets the once-per-turn flag
func (s *SneakAttackCondition) onTurnEnd(_ context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.CharacterID == s.CharacterID {
		s.UsedThisTurn = false
	}
	return nil
}

// onDamageChain adds sneak attack dice when conditions are met
func (s *SneakAttackCondition) onDamageChain(
	ctx context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only apply to this character's attacks
	if event.AttackerID != s.CharacterID {
		return c, nil
	}

	// Only apply once per turn
	if s.UsedThisTurn {
		return c, nil
	}

	// Must be a finesse or ranged weapon attack
	// For now, we check if the attack uses DEX (finesse weapons use DEX when it's higher)
	// TODO: Add proper weapon property checking via WeaponRef
	if event.AbilityUsed != "dex" {
		return c, nil
	}

	// Check sneak attack conditions: advantage OR ally within 5ft of target
	if !s.checkSneakAttackConditions(ctx, event) {
		return c, nil
	}

	// Roll sneak attack dice (use default roller if none configured, e.g., after JSON load)
	roller := s.roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	sneakDice, err := roller.RollN(ctx, s.DamageDice, 6)
	if err != nil {
		return c, rpgerr.Wrap(err, "failed to roll sneak attack dice")
	}

	// Add sneak attack damage component using DamageSourceFeature
	modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source:            dnd5eEvents.DamageSourceFeature,
			SourceRef:         refs.Features.SneakAttack(),
			OriginalDiceRolls: sneakDice,
			FinalDiceRolls:    sneakDice,
			Rerolls:           nil,
			FlatBonus:         0,
			DamageType:        e.DamageType, // Sneak attack uses weapon's damage type
			IsCritical:        event.IsCritical,
		})
		return e, nil
	}

	// Mark as used this turn
	s.UsedThisTurn = true

	err = c.Add(combat.StageFeatures, "sneak_attack", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrap(err, "failed to add sneak attack modifier")
	}

	return c, nil
}

// checkSneakAttackConditions checks if sneak attack conditions are met.
// Returns true if:
// - Attacker has advantage on the attack roll, OR
// - An ally of the attacker is within 5ft of the target
func (s *SneakAttackCondition) checkSneakAttackConditions(
	ctx context.Context,
	event *dnd5eEvents.DamageChainEvent,
) bool {
	// Condition 1: Has advantage
	if event.HasAdvantage {
		return true
	}

	// Condition 2: Ally within 5ft of target
	// Need both Room and Teams context to check ally positions
	room, hasRoom := gamectx.Room(ctx)
	teams, hasTeams := gamectx.Teams(ctx)

	if !hasRoom || !hasTeams {
		// Without spatial/team context, can't verify ally adjacent
		return false
	}

	// Get target position
	targetPos, found := room.GetEntityPosition(event.TargetID)
	if !found {
		return false
	}

	// Query entities within 5ft (1 square = 5ft, use radius 1.5 to include diagonals)
	entitiesNearTarget := room.GetEntitiesInRange(targetPos, 1.5)

	// Check if any entity near target is an ally of the attacker
	for _, entity := range entitiesNearTarget {
		entityID := entity.GetID()

		// Skip the target itself
		if entityID == event.TargetID {
			continue
		}

		// Skip the attacker (they might be adjacent but don't count as "ally")
		if entityID == event.AttackerID {
			continue
		}

		// Check if this entity is an ally of the attacker
		if teams.AreAllies(s.CharacterID, entityID) {
			return true
		}
	}

	return false
}

// ToJSON converts the condition to JSON for persistence
func (s *SneakAttackCondition) ToJSON() (json.RawMessage, error) {
	data := SneakAttackData{
		Ref:         refs.Features.SneakAttack(),
		CharacterID: s.CharacterID,
		Level:       s.Level,
		DamageDice:  s.DamageDice,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to marshal sneak attack data")
	}

	return bytes, nil
}

// loadJSON loads the condition from JSON
func (s *SneakAttackCondition) loadJSON(data json.RawMessage) error {
	var sneakData SneakAttackData
	if err := json.Unmarshal(data, &sneakData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal sneak attack data")
	}

	s.CharacterID = sneakData.CharacterID
	s.Level = sneakData.Level
	s.DamageDice = sneakData.DamageDice

	return nil
}
