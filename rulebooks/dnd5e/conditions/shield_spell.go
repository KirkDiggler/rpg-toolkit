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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// ShieldACBonus is the canonical AC bonus the Shield spell applies for the
// remainder of the attack window when cast as a reaction. The bonus is
// applied in phase 2 (combat.ApplyAttackOutcome) via a ReactionModifier.
const ShieldACBonus = 5

// ShieldSpellConditionData is the JSON shape for persisting the Shield
// condition. The condition represents "I have Shield prepared and may cast
// it as a reaction"; it IS persisted on a wizard's character.Data.Conditions.
//
// The readiness flag is NOT persisted on the condition — readiness lives on
// the encounter snapshot's reaction-readiness map (see Encounter.SetReactionReady)
// because it is encounter-scoped state, not character-scoped.
type ShieldSpellConditionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// ShieldSpellCondition publishes a ReactionTriggerEvent when an attack against
// the wizard would hit AND a +5 AC bonus would deflect it AND Shield is readied
// (gamectx.IsReactionReady).
//
// Subscribes to PostAttackRollTopic (published by combat.ResolveAttackHit after
// the d20 has been rolled and wouldHit has been computed). The condition does
// NOT mutate the in-flight chain — the AC modifier is applied in phase 2 via a
// ReactionModifier when the wizard takes the reaction.
//
// Per Wave 2.11d Director ruling B4: BOTH player and NPC reactors publish the
// trigger event. The orchestrator (encounter SDK wrapper / rpg-api) decides
// between auto-resolve (NPC) and prompt-driven (player) flows. After the
// reaction fires, the orchestrator clears Shield's readiness (one-shot for
// spell-cost reactions).
//
// Predicate per post-roll event:
//   - event.TargetID == self (the attack is against the wizard).
//   - event.WouldHit (already a hit; skip if missing without Shield).
//   - event.TotalAttack < event.OriginalAC + ShieldACBonus (Shield would deflect).
//   - gamectx.IsReactionReady(self, Shield-ref) (player has opted in).
//
// The condition does NOT check spell-slot availability today — that lives on
// the orchestrator side because the resource system is keyed differently per
// host. A Shield-with-no-slots player who toggles ready will still see prompts;
// the orchestrator's SubmitCheck handler must validate slot availability before
// running phase 2 with the modifier.
type ShieldSpellCondition struct {
	CharacterID     string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure ShieldSpellCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*ShieldSpellCondition)(nil)

// NewShieldSpellCondition creates a Shield condition for the given character.
// rpg-api Apply()'s this on a character at encounter setup IF the character
// has Shield prepared. Default readiness is OFF (spell-cost reactions are
// opt-in to prevent accidental slot burns).
func NewShieldSpellCondition(characterID string) *ShieldSpellCondition {
	return &ShieldSpellCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied (subscribed).
func (s *ShieldSpellCondition) IsApplied() bool {
	return s.bus != nil
}

// Apply subscribes the condition to PostAttackRollChain.
//
// PostAttackRollChain is used (rather than a typed topic) because the
// chained-topic primitive propagates the publish-time context to subscribers
// — that context carries gamectx.WithReactionReadiness, which Shield's
// predicate depends on. The handler does not modify the chain; it inspects
// the event and conditionally publishes a side-effect ReactionTriggerEvent.
func (s *ShieldSpellCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if s.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "shield spell condition already applied")
	}
	s.bus = bus

	postRollChain := dnd5eEvents.PostAttackRollChain.On(bus)
	subID, err := postRollChain.SubscribeWithChain(ctx, s.onPostAttackRoll)
	if err != nil {
		s.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to post-attack-roll chain")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, subID)
	return nil
}

// Remove unsubscribes the condition from all events.
func (s *ShieldSpellCondition) Remove(ctx context.Context, bus events.EventBus) error {
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

// ToJSON converts the condition to its JSON representation.
func (s *ShieldSpellCondition) ToJSON() (json.RawMessage, error) {
	data := ShieldSpellConditionData{
		Ref:         refs.Spells.Shield(),
		CharacterID: s.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads Shield condition state from JSON.
func (s *ShieldSpellCondition) loadJSON(data json.RawMessage) error {
	var shieldData ShieldSpellConditionData
	if err := json.Unmarshal(data, &shieldData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal shield spell data")
	}
	s.CharacterID = shieldData.CharacterID
	return nil
}

// onPostAttackRoll evaluates the Shield predicate and publishes a trigger
// event when the attack would hit the wizard but Shield's +5 AC would deflect.
// The chain itself is not modified — the AC bonus is applied in phase 2 via
// a ReactionModifier when the wizard takes the reaction.
func (s *ShieldSpellCondition) onPostAttackRoll(
	ctx context.Context,
	event *dnd5eEvents.PostAttackRollEvent,
	c chain.Chain[*dnd5eEvents.PostAttackRollEvent],
) (chain.Chain[*dnd5eEvents.PostAttackRollEvent], error) {
	// Only react when this character is the target.
	if event.TargetID != s.CharacterID {
		return c, nil
	}
	// Skip if the attack already misses (no point burning a reaction).
	if !event.WouldHit {
		return c, nil
	}
	// Natural 20 always hits — Shield cannot prevent it. Skip.
	if event.IsNaturalTwenty {
		return c, nil
	}
	// Skip if +5 AC would not deflect this attack (still hits).
	// effective AC = OriginalAC + ShieldACBonus; miss requires TotalAttack < effectiveAC.
	if event.TotalAttack >= event.OriginalAC+ShieldACBonus {
		return c, nil
	}
	// Readiness gate — opt-in. If unreadied, no trigger fires and the
	// attack proceeds to phase 2 unmodified.
	if !gamectx.IsReactionReady(ctx, s.CharacterID, refs.Spells.Shield().String()) {
		return c, nil
	}

	// Predicate matched — publish trigger event for the orchestrator.
	triggerTopic := dnd5eEvents.ReactionTriggerTopic.On(s.bus)
	if pubErr := triggerTopic.Publish(ctx, dnd5eEvents.ReactionTriggerEvent{
		ReactorID:    s.CharacterID,
		ConditionRef: refs.Spells.Shield().String(),
		TriggerKind:  dnd5eEvents.TriggerKindPostHit,
		SourceEntity: event.AttackerID,
		Payload:      *event,
	}); pubErr != nil {
		return c, rpgerr.Wrap(pubErr, "failed to publish shield reaction trigger event")
	}
	return c, nil
}
