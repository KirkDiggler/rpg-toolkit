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
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// RagingData is the JSON structure for persisting raging condition state
type RagingData struct {
	Ref               *core.Ref `json:"ref"`
	CharacterID       string    `json:"character_id"`
	DamageBonus       int       `json:"damage_bonus"`
	Level             int       `json:"level"`
	Source            string    `json:"source"` // Ref string in "module:type:value" format (e.g., "dnd5e:features:rage")
	TurnsActive       int       `json:"turns_active"`
	WasHitThisTurn    bool      `json:"was_hit_this_turn"`
	DidAttackThisTurn bool      `json:"did_attack_this_turn"`
}

// RagingCondition represents the barbarian rage state.
// It implements the Condition interface.
type RagingCondition struct {
	CharacterID       string
	DamageBonus       int
	Level             int
	Source            string // Ref string in "module:type:value" format (e.g., "dnd5e:features:rage")
	TurnsActive       int
	WasHitThisTurn    bool
	DidAttackThisTurn bool
	subscriptionIDs   []string
	bus               events.EventBus
}

// Ensure RagingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*RagingCondition)(nil)

// IsApplied returns true if this condition is currently applied
func (r *RagingCondition) IsApplied() bool {
	return r.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if r.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "raging condition already applied")
	}
	r.bus = bus

	// Subscribe to damage events to track if we were hit
	damages := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID1, err := damages.Subscribe(ctx, r.onDamageReceived)
	if err != nil {
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID1)

	// Subscribe to turn end events to check if rage continues
	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	subID2, err := turnEnds.Subscribe(ctx, r.onTurnEnd)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID2)

	// Subscribe to condition applied events to check for unconscious
	conditions := dnd5eEvents.ConditionAppliedTopic.On(bus)
	subID3, err := conditions.Subscribe(ctx, r.onConditionApplied)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID3)

	// Subscribe to damage chain to add rage damage bonus and track successful hits
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID4, err := damageChain.SubscribeWithChain(ctx, r.onDamageChain)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID4)

	// Subscribe to rest events - rage ends on any rest
	restTopic := dnd5eEvents.RestTopic.On(bus)
	subID5, err := restTopic.Subscribe(ctx, r.onRest)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID5)

	// Subscribe to saving throw chain to grant advantage on STR saves
	saveChain := dnd5eEvents.SavingThrowChain.On(bus)
	subID6, err := saveChain.SubscribeWithChain(ctx, r.onSavingThrowChain)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID6)

	// Subscribe to ability check chain to grant advantage on STR checks
	checkChain := dnd5eEvents.AbilityCheckChain.On(bus)
	subID7, err := checkChain.SubscribeWithChain(ctx, r.onAbilityCheckChain)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID7)

	// Subscribe to combat-end events - RAW: rage ends when combat ends
	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	subID8, err := combatEnds.Subscribe(ctx, r.onCombatEnd)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID8)

	// Subscribe to the post-attack-roll chain to track attack attempts for
	// the sustain flag. This fires once per attack roll, hit or miss --
	// unlike the damage chain, which only fires on a hit (rpg-toolkit#755).
	postRolls := dnd5eEvents.PostAttackRollChain.On(bus)
	subID9, err := postRolls.SubscribeWithChain(ctx, r.onPostAttackRoll)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID9)

	return nil
}

// Remove unsubscribes this condition from events
func (r *RagingCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if r.bus == nil {
		return nil // Not applied, nothing to remove
	}

	total := len(r.subscriptionIDs)
	var errs []error
	for _, subID := range r.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	r.subscriptionIDs = nil
	r.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (r *RagingCondition) ToJSON() (json.RawMessage, error) {
	data := RagingData{
		Ref:               refs.Conditions.Raging(),
		CharacterID:       r.CharacterID,
		DamageBonus:       r.DamageBonus,
		Level:             r.Level,
		Source:            r.Source,
		TurnsActive:       r.TurnsActive,
		WasHitThisTurn:    r.WasHitThisTurn,
		DidAttackThisTurn: r.DidAttackThisTurn,
	}
	return json.Marshal(data)
}

// loadJSON loads raging condition state from JSON
func (r *RagingCondition) loadJSON(data json.RawMessage) error {
	var ragingData RagingData
	if err := json.Unmarshal(data, &ragingData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal raging data")
	}

	r.CharacterID = ragingData.CharacterID
	r.DamageBonus = ragingData.DamageBonus
	r.Level = ragingData.Level
	r.Source = ragingData.Source
	r.TurnsActive = ragingData.TurnsActive
	r.WasHitThisTurn = ragingData.WasHitThisTurn
	r.DidAttackThisTurn = ragingData.DidAttackThisTurn

	return nil
}

// onDamageReceived handles damage events to track if we were hit this turn
func (r *RagingCondition) onDamageReceived(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	if event.TargetID != r.CharacterID {
		return nil
	}
	r.WasHitThisTurn = true
	return nil
}

// onTurnEnd handles turn end events to check if rage continues
func (r *RagingCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.CharacterID != r.CharacterID {
		return nil
	}

	// Increment turns active
	r.TurnsActive++

	// Check if rage ends due to no combat activity
	if !r.DidAttackThisTurn && !r.WasHitThisTurn {
		return r.endRage(ctx, "no_combat_activity")
	}

	// Check if rage ends due to duration (10 rounds = 1 minute)
	if r.TurnsActive >= 10 {
		return r.endRage(ctx, "duration_expired")
	}

	// Reset combat activity flags for next turn
	r.DidAttackThisTurn = false
	r.WasHitThisTurn = false

	return nil
}

// onConditionApplied handles condition applied events to check for unconscious
func (r *RagingCondition) onConditionApplied(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
	// Check if unconscious was applied to us
	if event.Type == dnd5eEvents.ConditionUnconscious && event.Target.GetID() == r.CharacterID {
		return r.endRage(ctx, "unconscious")
	}
	return nil
}

// onRest handles rest events - rage ends on any rest
func (r *RagingCondition) onRest(ctx context.Context, event dnd5eEvents.RestEvent) error {
	// Only end rage if this is our character
	if event.CharacterID != r.CharacterID {
		return nil
	}
	return r.endRage(ctx, "rest")
}

// onCombatEnd handles combat-end events - RAW: rage ends when combat ends.
// Without this, a raging character whose encounter ends via a route other
// than "no combat activity this turn" (e.g. the killing blow itself ends the
// encounter) would carry the condition into persisted character data and
// silently re-apply it in the next encounter (rpg-toolkit#752).
func (r *RagingCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	// Only end rage if this is our character
	if event.CharacterID != r.CharacterID {
		return nil
	}
	return r.endRage(ctx, "combat_ended")
}

// endRage publishes the removal event and unsubscribes from all events
func (r *RagingCondition) endRage(ctx context.Context, reason string) error {
	if r.bus == nil {
		return nil
	}

	// Publish condition removed event
	removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
	err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  r.CharacterID,
		ConditionRef: refs.Conditions.Raging().String(),
		Reason:       reason,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "error publishing rage removal for character id %s", r.CharacterID)
	}

	// Actually remove the condition (unsubscribe from events)
	return r.Remove(ctx, r.bus)
}

// onDamageChain handles both:
// 1. Adding rage damage bonus when the raging character attacks
// 2. Applying resistance (halve damage) when the raging character is hit by B/P/S damage
func (r *RagingCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Handle attacker side: add rage damage bonus
	if event.AttackerID == r.CharacterID {
		// Add rage damage modifier in the StageFeatures stage. Gated inside the
		// modifier (on the live e.AbilityUsed/e.IsMelee) rather than on the
		// publish-time event above, since other StageFeatures modifiers (e.g.
		// Martial Arts) can change AbilityUsed while the chain executes --
		// checking the pre-chain snapshot would let Rage's bonus survive a
		// swap away from STR.
		modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
			primary := primaryWeaponComponent(e)
			if primary == nil {
				return e, nil
			}
			// RAW: the rage damage bonus only applies to melee weapon attacks
			// that use Strength (including unarmed strikes) -- not ranged or
			// DEX-based attacks.
			if e.AbilityUsed != abilities.STR || !e.IsMelee {
				return e, nil
			}

			// Append rage damage component
			e.Components = append(e.Components, dnd5eEvents.DamageComponent{
				Source:            dnd5eEvents.DamageSourceCondition,
				SourceRef:         refs.Conditions.Raging(),
				OriginalDiceRolls: nil, // No dice
				FinalDiceRolls:    nil,
				Rerolls:           nil,
				FlatBonus:         r.DamageBonus,
				DamageType:        primary.DamageType, // Same as marked primary weapon type
				IsCritical:        false,
			})
			return e, nil
		}
		err := c.Add(combat.StageFeatures, "rage", modifyDamage)
		if err != nil {
			return c, rpgerr.Wrapf(err, "error applying rage damage bonus for character id %s", r.CharacterID)
		}
	}

	// Handle defender side: apply resistance to B/P/S damage
	if event.TargetID == r.CharacterID {
		// Add resistance multiplier in the StageFinal stage
		applyResistance := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
			physicalTypes := make(map[damage.Type]struct{})
			for _, component := range e.Components {
				if component.Multiplier == nil && component.DamageType.IsPhysical() {
					physicalTypes[component.DamageType] = struct{}{}
				}
			}
			for damageType := range physicalTypes {
				e.Components = append(e.Components, dnd5eEvents.DamageComponent{
					Source:     dnd5eEvents.DamageSourceCondition,
					SourceRef:  refs.Conditions.Raging(),
					DamageType: damageType,
					Multiplier: dnd5eEvents.Multiply(0.5), // Resistance halves damage
				})
			}
			return e, nil
		}
		err := c.Add(combat.StageFinal, "rage_resistance", applyResistance)
		if err != nil {
			return c, rpgerr.Wrapf(err, "error applying rage resistance for character id %s", r.CharacterID)
		}
	}

	return c, nil
}

// onSavingThrowChain grants advantage on Strength saving throws while raging (PHB rage benefits).
func (r *RagingCondition) onSavingThrowChain(
	_ context.Context,
	event *dnd5eEvents.SavingThrowChainEvent,
	c chain.Chain[*dnd5eEvents.SavingThrowChainEvent],
) (chain.Chain[*dnd5eEvents.SavingThrowChainEvent], error) {
	// Only apply to this character's saves
	if event.SaverID != r.CharacterID {
		return c, nil
	}

	// Only apply to STR saves
	if event.Ability != abilities.STR {
		return c, nil
	}

	// Add advantage at the conditions stage
	modifySave := func(_ context.Context, e *dnd5eEvents.SavingThrowChainEvent) (*dnd5eEvents.SavingThrowChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.SaveModifierSource{
			Name:       "Raging",
			SourceType: "condition",
			SourceRef:  refs.Conditions.Raging(),
			EntityID:   r.CharacterID,
		})
		return e, nil
	}

	if err := c.Add(combat.StageConditions, "raging_str_advantage", modifySave); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add raging STR advantage modifier for character %s", r.CharacterID)
	}

	return c, nil
}

// onAbilityCheckChain grants advantage on Strength checks while raging (PHB rage benefits).
func (r *RagingCondition) onAbilityCheckChain(
	_ context.Context,
	event *dnd5eEvents.AbilityCheckChainEvent,
	c chain.Chain[*dnd5eEvents.AbilityCheckChainEvent],
) (chain.Chain[*dnd5eEvents.AbilityCheckChainEvent], error) {
	// Only apply to this character's checks
	if event.CheckerID != r.CharacterID {
		return c, nil
	}

	// Only apply to Strength-based skill checks (e.g., Athletics)
	if skills.Ability(event.Skill) != abilities.STR {
		return c, nil
	}

	// Add advantage at the conditions stage
	modifyCheck := func(_ context.Context, e *dnd5eEvents.AbilityCheckChainEvent) (*dnd5eEvents.AbilityCheckChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.CheckModifierSource{
			Name:       "Raging",
			SourceType: "condition",
			SourceRef:  refs.Conditions.Raging(),
			EntityID:   r.CharacterID,
		})
		return e, nil
	}

	if err := c.Add(combat.StageConditions, "raging_str_check_advantage", modifyCheck); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add raging STR check advantage modifier for character %s", r.CharacterID)
	}

	return c, nil
}

// onPostAttackRoll tracks that this character attempted an attack this turn,
// regardless of whether it hits. RAW (PHB rage): rage continues if you've
// "attacked a hostile creature since your last turn" -- an attempt counts,
// hit or miss. PostAttackRollChain fires once per attack roll (after the d20
// is rolled, before the hit/miss outcome is applied), unlike onDamageChain
// above, which only fires on a hit and was silently dropping rage on a miss
// (rpg-toolkit#755). The chain itself is not modified.
func (r *RagingCondition) onPostAttackRoll(
	_ context.Context,
	event *dnd5eEvents.PostAttackRollEvent,
	c chain.Chain[*dnd5eEvents.PostAttackRollEvent],
) (chain.Chain[*dnd5eEvents.PostAttackRollEvent], error) {
	if event.AttackerID == r.CharacterID {
		r.DidAttackThisTurn = true
	}
	return c, nil
}
