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
	CharacterID       string    `json:"member_id"`
	DamageBonus       int       `json:"damage_bonus"`
	Level             int       `json:"level"`
	Source            string    `json:"source"` // Ref string in "module:type:value" format (e.g., "dnd5e:features:rage")
	SawTurnEnd        bool      `json:"saw_turn_end"`
	RoundActivated    int       `json:"round_activated"`
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
	SawTurnEnd        bool
	RoundActivated    int
	TurnsActive       int
	WasHitThisTurn    bool
	DidAttackThisTurn bool
	subscriptionIDs   []string
	bus               events.EventBus
}

// Ensure RagingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*RagingCondition)(nil)

// stateChanged reports that this rage's own upkeep state moved — was I hit,
// did I attack, how many turns have I been up. See [publishStateChanged].
func (r *RagingCondition) stateChanged(ctx context.Context) error {
	return publishStateChanged(ctx, r.bus, r.CharacterID, r.Ref())
}

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (r *RagingCondition) Ref() *core.Ref { return refs.Conditions.Raging() }

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
		SawTurnEnd:        r.SawTurnEnd,
		RoundActivated:    r.RoundActivated,
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
	r.SawTurnEnd = ragingData.SawTurnEnd
	r.RoundActivated = ragingData.RoundActivated
	r.TurnsActive = ragingData.TurnsActive
	r.WasHitThisTurn = ragingData.WasHitThisTurn
	r.DidAttackThisTurn = ragingData.DidAttackThisTurn

	return nil
}

// onDamageReceived handles damage events to track if we were hit this turn
func (r *RagingCondition) onDamageReceived(ctx context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	if event.TargetID != r.CharacterID {
		return nil
	}
	if r.WasHitThisTurn {
		return nil // already recorded this turn; nothing changed
	}
	r.WasHitThisTurn = true

	return r.stateChanged(ctx)
}

// rageDurationRounds is 2014's "1 minute": ten rounds, ending at the end of the
// barbarian's own turn on the tenth.
//
// Named so the arithmetic below has something to be one less than. Rage spans
// rounds RoundActivated through RoundActivated+9 inclusive, so the comparison is
// against rageDurationRounds-1 rather than rageDurationRounds — the one place a
// duration like this grows an off-by-one.
const rageDurationRounds = 10

// onTurnEnd decides, at the end of the barbarian's own turn, whether the rage
// goes on.
//
// # 2014, with the activation turn graced
//
// RAW 2014: rage lasts 1 minute and ends early if you are knocked unconscious,
// or if your turn ends and you have neither attacked a hostile creature since
// your last turn nor taken damage since then.
//
// Kirk ruled 2026-08-27 that the turn a rage STARTED is not checked: "I cannot
// imagine activating rage would end at the end of the turn activated so i think
// it lasts 1 full turn... this game does not need to follow RAW to the letter."
// By the letter, a barbarian who rages and then does nothing else drops it the
// instant their turn ends, which is a bad moment rather than an interesting one.
// The divergence is exactly this one branch.
//
// # The anchor comes from the CLOCK, never from the sheet
//
// event.Round is stamped by play/clock itself and is the only round in the
// system that cannot be stale. The character also carries one --
// ActionEconomy.TurnNumber -- and it is deliberately NOT used: that is a
// sheet-local mirror with a documented staleness bug across fights
// (rpg-project#253, a member starting a fight with the movement their previous
// one left behind). Anchoring a duration to it would reintroduce the class of
// defect combat end was built to close.
//
// So the rage DISCOVERS its anchor instead of being handed one: the first turn
// end it hears both establishes RoundActivated and is the graced turn. Zero
// means "not yet anchored", which is play/clock's own vocabulary for "no round"
// rather than an invented sentinel -- a Turn clock holds round 0 only while
// idle.
func (r *RagingCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.SubjectID != r.CharacterID {
		return nil
	}

	if !r.SawTurnEnd {
		// The first turn end this rage has seen: it passes unchecked, and it
		// anchors the duration if it carries a round to anchor to.
		r.SawTurnEnd = true
		if event.Round > 0 {
			r.RoundActivated = event.Round
		}
	} else {
		// The activity check needs no round at all, so it runs regardless of
		// whether the duration below can be evaluated.
		if !r.DidAttackThisTurn && !r.WasHitThisTurn {
			return r.endRage(ctx, "no_combat_activity")
		}

		switch {
		case r.RoundActivated == 0:
			// Never anchored, because no turn end has carried a round yet.
			// Anchor late if this one finally does; the cap simply does not
			// apply until there is something to measure from.
			if event.Round > 0 {
				r.RoundActivated = event.Round
			}

		case event.Round <= 0:
			// A turn end that does not say which round it is. The cap cannot
			// be evaluated and is skipped rather than evaluated against zero,
			// which would read as a clock reset below and end the rage on a
			// malformed event.

		case event.Round < r.RoundActivated:
			// The round went BACKWARDS, so this rage has outlived the clock
			// its anchor came from -- rounds are per-fight and restart at 1.
			// Combat end exists to make this unreachable (rpg-project#295
			// part 1), so reaching it means that removal did not happen.
			// Ending here is both the right rules answer -- the fight it
			// belonged to is over -- and a net under the thing that should
			// have caught it. Re-anchoring instead would silently hand out a
			// fresh ten rounds and hide the regression.
			return r.endRage(ctx, "clock_reset")

		case event.Round-r.RoundActivated >= rageDurationRounds-1:
			return r.endRage(ctx, "duration_expired")
		}
	}

	// DERIVED, never incremented. TurnsActive is display state that the web
	// client reads (rpg-dnd5e-web's ConditionBadge, and its isRagingData type
	// guard, which duck-types on this key being present -- deleting the field
	// would make every rage silently stop rendering as one). Recomputing it
	// from the anchor keeps the values the client already shows while removing
	// the reason it was wrong: an accumulated counter is only correct if no
	// turn end is ever missed, and turn ends went missing for months.
	//
	// NO RULE READS THIS. Everything above decides on RoundActivated.
	//
	// Only recomputed when there is an anchor to recompute from. An unanchored
	// rage keeps whatever it had rather than being handed a number derived
	// from a zero it never agreed to.
	if r.RoundActivated > 0 && event.Round >= r.RoundActivated {
		r.TurnsActive = event.Round - r.RoundActivated + 1
	}

	// Reset for the next turn. On the graced turn too: RAW's window is "since
	// your last turn", so the activation turn's swing must not pay for the next
	// turn's check.
	r.DidAttackThisTurn = false
	r.WasHitThisTurn = false

	return r.stateChanged(ctx)
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
	if event.SubjectID != r.CharacterID {
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
		MemberID:     r.CharacterID,
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
				DamageType:        e.WeaponDamageType, // Same as marked primary weapon type
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
	ctx context.Context,
	event *dnd5eEvents.PostAttackRollEvent,
	c chain.Chain[*dnd5eEvents.PostAttackRollEvent],
) (chain.Chain[*dnd5eEvents.PostAttackRollEvent], error) {
	if event.AttackerID == r.CharacterID && !r.DidAttackThisTurn {
		r.DidAttackThisTurn = true

		return c, r.stateChanged(ctx)
	}
	return c, nil
}
