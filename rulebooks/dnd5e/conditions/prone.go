// Copyright (C) 2026 Kirk Diggler
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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// proneReachCells is "within 5 feet" in the units a grid answers in: one cell.
//
// This comment used to explain the conversion here, and note that
// FightingStyleProtectionCondition and SneakAttackCondition spelled out the
// same one. They did — and sneak attack's copy nonetheless used 1.5. The
// rationale now lives once, on [combat.AdjacentCells], and this is an alias so
// the two cannot drift apart again (rpg-toolkit#1255 review).
const proneReachCells = combat.AdjacentCells

// ProneConditionData is the serializable form of the prone condition.
// This is stored by the game server as an opaque JSON blob.
type ProneConditionData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// ProneCondition is D&D 5e's prone condition, as it affects attack rolls.
//
// Two rules, and they pull in opposite directions:
//
//   - The prone creature attacks at disadvantage. Always — being on the floor
//     is bad for your aim regardless of who you are swinging at.
//   - Attacks against the prone creature have advantage if the attacker is
//     within 5 feet, and disadvantage otherwise. Standing over someone is an
//     opportunity; shooting at someone lying down is not.
//
// The second rule is why this condition reads the room. Distance is not on the
// attack event and cannot be inferred from it: AttackChainEvent.IsMelee is not
// a proxy for "within 5 feet" in either direction — a glaive is melee at ten
// feet, and a shortbow fired point-blank is ranged at zero. So the position of
// both parties is read from the room in [gamectx], the way every other
// range-dependent predicate in this package does.
//
// **When there is no room, the second rule does not fire.** No advantage, no
// disadvantage: the attack rolls straight, and the prone creature's own
// disadvantage is unaffected. That is a rule silently not applied, which is
// worth stating plainly rather than discovering — but the alternative is worse.
// A handler that returned an error would abort the whole attack chain (a bus
// publish stops at the first handler error), so a caller that never installed a
// room would find that attacking a prone creature failed rather than merely
// rolled straight. Pinned by TestNoRoomLeavesTheTargetSideRuleUnapplied so it
// cannot be mistaken for a guarantee.
//
// What this condition does NOT do: the movement half of prone. Crawling at half
// speed and standing up costing half your movement are real rules, and movement
// cost is not modelled at this seam — see rpg-toolkit#961. Nothing here applies
// the condition to anyone either; what knocks a creature down is its own
// concern (rpg-toolkit#962).
type ProneCondition struct {
	CharacterID     string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure ProneCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*ProneCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (p *ProneCondition) Ref() *core.Ref { return refs.Conditions.Prone() }

// NewProneCondition creates a prone condition for the specified creature.
//
// It does not remove itself: prone lasts until something stands the creature up,
// unlike Dodging, which expires at the start of its owner's next turn.
func NewProneCondition(characterID string) *ProneCondition {
	return &ProneCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (p *ProneCondition) IsApplied() bool {
	return p.bus != nil
}

// Apply subscribes this condition to the attack chain, which is where both of
// its rules are expressed — one for attacks the prone creature makes, one for
// attacks made against it.
//
// A failed Apply leaves nothing behind: the condition does not end up holding a
// bus it never subscribed to, so it can be applied again.
func (p *ProneCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if p.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "prone condition already applied")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	p.bus = bus

	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, p.onAttackChain)
	if err != nil {
		p.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	p.subscriptionIDs = append(p.subscriptionIDs, subID)

	longRestSubID, err := subscribeRemoveOnLongRest(ctx, bus, p.CharacterID, p.Ref(), p.Remove)
	if err != nil {
		_ = p.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	p.subscriptionIDs = append(p.subscriptionIDs, longRestSubID)

	return nil
}

// Remove unsubscribes this condition from all events.
//
// A nil bus falls back to the one Apply was given, because that is the bus the
// subscription IDs were issued by and the only one that can revoke them —
// passing some other bus revokes nothing, since IDs mean nothing to a bus that
// did not grant them.
//
// The condition stays applied if any unsubscription fails. Clearing state
// regardless would have it report itself removed while its handler is still on
// the bus, and a modifier that keeps appearing from a condition nobody can see
// is a bad afternoon; leaving it applied keeps IsApplied honest and lets the
// caller try again.
func (p *ProneCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if p.bus == nil {
		return nil // Not applied, nothing to remove
	}

	if bus == nil {
		bus = p.bus
	}

	total := len(p.subscriptionIDs)
	var errs []error
	var stillSubscribed []string
	for _, subID := range p.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
			stillSubscribed = append(stillSubscribed, subID)
		}
	}

	if len(errs) > 0 {
		// Keep exactly the ones that are still live, so a retry does not try to
		// revoke what is already gone.
		p.subscriptionIDs = stillSubscribed
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}

	p.subscriptionIDs = nil
	p.bus = nil

	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (p *ProneCondition) ToJSON() (json.RawMessage, error) {
	data := ProneConditionData{
		Ref:      refs.Conditions.Prone(),
		MemberID: p.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads prone condition state from JSON.
func (p *ProneCondition) loadJSON(data json.RawMessage) error {
	var proneData ProneConditionData
	if err := json.Unmarshal(data, &proneData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal prone data")
	}

	p.CharacterID = proneData.MemberID
	return nil
}

// onAttackChain applies whichever of prone's two attack rules this attack is
// subject to.
//
// The attacker branch is checked first, so a prone creature attacking itself —
// which no rule contemplates, but which the event shape permits — gets the
// disadvantage it would get attacking anyone else, and not both modifiers at
// once.
func (p *ProneCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	switch {
	case event.AttackerID == p.CharacterID:
		return p.attackingWhileProne(c)
	case event.TargetID == p.CharacterID:
		return p.attackedWhileProne(ctx, event, c)
	default:
		return c, nil
	}
}

// attackingWhileProne imposes the prone creature's own disadvantage. No geometry
// is involved: it applies to every attack it makes, at any range.
func (p *ProneCondition) attackingWhileProne(
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.Conditions.Prone(),
			SourceID:  p.CharacterID,
			Reason:    "Prone attacker",
		})
		return e, nil
	}

	if err := c.Add(combat.StageConditions, "prone_attacker_disadvantage", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add prone attacker disadvantage for character %s", p.CharacterID)
	}

	return c, nil
}

// attackedWhileProne resolves the range split: advantage from within 5 feet,
// disadvantage from beyond it.
//
// Both directions are decided here rather than one being the default, because
// "no modifier" is a third, wrong answer that a half-implemented range check
// silently produces.
func (p *ProneCondition) attackedWhileProne(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	within, known := p.attackerIsWithinReach(ctx, event.AttackerID)
	if !known {
		// No room, or someone is not on the map. See the type's godoc: the rule
		// cannot be decided, and refusing the attack outright would be worse
		// than leaving it to roll straight.
		return c, nil
	}

	key := "prone_target_disadvantage"
	reason := "Prone target beyond 5 feet"
	appendSource := func(e dnd5eEvents.AttackChainEvent, src dnd5eEvents.AttackModifierSource) dnd5eEvents.AttackChainEvent {
		e.DisadvantageSources = append(e.DisadvantageSources, src)
		return e
	}

	if within {
		key = "prone_target_advantage"
		reason = "Prone target within 5 feet"
		appendSource = func(e dnd5eEvents.AttackChainEvent, src dnd5eEvents.AttackModifierSource) dnd5eEvents.AttackChainEvent {
			e.AdvantageSources = append(e.AdvantageSources, src)
			return e
		}
	}

	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		return appendSource(e, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.Conditions.Prone(),
			SourceID:  p.CharacterID,
			Reason:    reason,
		}), nil
	}

	if err := c.Add(combat.StageConditions, key, modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add %s for character %s", key, p.CharacterID)
	}

	return c, nil
}

// attackerIsWithinReach answers whether the attacker is within 5 feet of this
// prone creature, and whether that could be answered at all.
//
// The second return is the whole reason this is not a plain bool: "not within
// reach" and "nobody knows where these two are standing" call for different
// modifiers, and collapsing them would make an unmapped attacker roll at
// disadvantage — a rule invented out of missing data.
func (p *ProneCondition) attackerIsWithinReach(ctx context.Context, attackerID string) (within, known bool) {
	room, ok := gamectx.Room(ctx)
	if !ok {
		return false, false
	}

	attackerPos, attackerPlaced := room.GetEntityPosition(attackerID)
	if !attackerPlaced {
		return false, false
	}

	pronePos, pronePlaced := room.GetEntityPosition(p.CharacterID)
	if !pronePlaced {
		return false, false
	}

	return room.GetGrid().Distance(attackerPos, pronePos) <= proneReachCells, true
}
