// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

// SneakAttackData is the JSON structure for persisting sneak attack condition state.
//
// UsedThisTurn is persisted (not just runtime) because each Encounter.TakeAction
// RPC call goes through LoadFromData → fresh condition instance from JSON. Without
// persisting the once-per-turn flag, a rogue would sneak-attack on every TakeAction
// call, breaking the once-per-turn semantic across separate RPCs in the same turn.
// See rpg-toolkit#654 for the broader sustainable-per-turn-state pattern.
type SneakAttackData struct {
	Ref          *core.Ref `json:"ref"`
	CharacterID  string    `json:"character_id"`
	Level        int       `json:"level"`
	DamageDice   int       `json:"damage_dice"`
	UsedThisTurn bool      `json:"used_this_turn"`
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
	owner           selfPersisting
}

// Ensure SneakAttackCondition implements dnd5eEvents.OwnerAware
var _ dnd5eEvents.OwnerAware = (*SneakAttackCondition)(nil)

// SetOwner hands this condition the sheet its once-per-turn flag is persisted
// on. See [selfPersisting] in raging.go.
func (s *SneakAttackCondition) SetOwner(owner any) {
	if o, ok := owner.(selfPersisting); ok {
		s.owner = o
	}
}

// markDirty records that the once-per-turn flag changed.
func (s *SneakAttackCondition) markDirty() {
	if s.owner != nil {
		s.owner.MarkDirty()
	}
}

// Ensure SneakAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*SneakAttackCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on. Sneak Attack's canonical ref
// lives under refs.Features (it is a rogue feature turned active condition),
// so that is what Ref reports.
func (s *SneakAttackCondition) Ref() *core.Ref { return refs.Features.SneakAttack() }

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
	if s.bus == nil {
		return nil
	}

	total := len(s.subscriptionIDs)
	var errs []error
	for _, id := range s.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", id, err))
		}
	}

	s.subscriptionIDs = nil
	s.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// onTurnEnd resets the once-per-turn flag
func (s *SneakAttackCondition) onTurnEnd(_ context.Context, event dnd5eEvents.TurnEndEvent) error {
	// Only when the flag actually changes. Marking unconditionally would
	// flag every rogue dirty at the end of every turn they did not sneak
	// attack, and a turn boundary is about to become an interaction that
	// runs for every participant.
	if event.CharacterID == s.CharacterID && s.UsedThisTurn {
		s.UsedThisTurn = false
		s.markDirty()
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

	// Advantage, OR another enemy of the target adjacent to it.
	if !s.sneakAttackApplies(ctx, event) {
		return c, nil
	}

	// Roll sneak attack dice (use default roller if none configured, e.g., after JSON load)
	roller := s.roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	rolls := 1
	if event.IsCritical {
		rolls++
	}
	var sneakDice []int
	for range rolls {
		rolled, err := roller.RollN(ctx, s.DamageDice, 6)
		if err != nil {
			return c, rpgerr.Wrap(err, "failed to roll sneak attack dice")
		}
		sneakDice = append(sneakDice, rolled...)
	}

	// Add sneak attack damage component using DamageSourceFeature
	modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		primary := primaryWeaponComponent(e)
		if primary == nil {
			return e, nil
		}
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source:            dnd5eEvents.DamageSourceFeature,
			SourceRef:         refs.Features.SneakAttack(),
			OriginalDiceRolls: sneakDice,
			FinalDiceRolls:    sneakDice,
			Rerolls:           nil,
			FlatBonus:         0,
			DamageType:        e.WeaponDamageType, // Sneak attack uses the marked primary weapon type
			IsCritical:        event.IsCritical,
		})
		return e, nil
	}

	// Mark as used this turn
	s.UsedThisTurn = true
	s.markDirty()

	err := c.Add(combat.StageFeatures, "sneak_attack", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrap(err, "failed to add sneak attack modifier")
	}

	return c, nil
}

// sneakAttackApplies reports whether sneak attack's positional requirement is
// met: the attacker has advantage, or another enemy OF THE TARGET is within
// five feet of it.
//
// Note whose enemy. RAW is "another enemy of the target is within 5 feet of
// it" — the relation is measured from the TARGET's point of view, not the
// attacker's. With two factions those coincide and nobody notices. With three
// they come apart: a hobgoblin standing beside the duergar you are stabbing is
// an enemy of your target and enables this, and it is nobody's ally.
//
// This used to ask whether the adjacent entity's type was "character", which
// baked a two-sided world into the rule and got it wrong in both directions —
// a fellow player counted even when fighting you, and a rival monster never
// counted at all.
//
// Returns a plain bool. A question this cannot answer is not an error: the
// caller folds this into the damage chain, and an errored fold discards every
// other damage component along with this one, exactly as an errored AC fold
// discarded every other AC contributor (rpg-toolkit#1254).
func (s *SneakAttackCondition) sneakAttackApplies(
	ctx context.Context,
	event *dnd5eEvents.DamageChainEvent,
) bool {
	if event.HasAdvantage {
		return true
	}

	room, ok := gamectx.Room(ctx)
	if !ok {
		return false
	}
	cast, ok := gamectx.CastOf(ctx)
	if !ok {
		return false
	}

	targetPos, found := room.GetEntityPosition(event.TargetID)
	if !found {
		return false
	}

	for _, entity := range room.GetEntitiesInRange(targetPos, combat.AdjacentCells) {
		id := entity.GetID()
		if id == event.TargetID || id == event.AttackerID {
			continue
		}
		if hostile, known := cast.IsHostile(event.TargetID, id); known && hostile {
			return true
		}
	}

	return false
}

// ToJSON converts the condition to JSON for persistence
func (s *SneakAttackCondition) ToJSON() (json.RawMessage, error) {
	data := SneakAttackData{
		Ref:          refs.Features.SneakAttack(),
		CharacterID:  s.CharacterID,
		Level:        s.Level,
		DamageDice:   s.DamageDice,
		UsedThisTurn: s.UsedThisTurn,
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
	s.UsedThisTurn = sneakData.UsedThisTurn

	return nil
}
