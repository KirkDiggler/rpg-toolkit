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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

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
// The second rule reads the attacker→target distance from the attack's frame.
// Distance is not on the attack event and cannot be inferred from it:
// AttackChainEvent.IsMelee is not a proxy for "within 5 feet" in either
// direction — a glaive is melee at ten feet, and a shortbow fired point-blank
// is ranged at zero. Resolution measures it on the room's grid and puts it on
// the frame; the rule, keyed by Prone's reference, is the one information asks
// for a candidate (R17).
//
// **When the distance is unknown, the attack fails.** The rule answers that it
// depends, and at execution that is an error (R13): rolling straight would be
// a rule silently not applied, and a missing fact must not switch a rule off.
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

var _ contributions.ActionAssessor = (*ProneCondition)(nil)

// AssessAction answers whether Prone's disadvantage bears on the framed
// attack: every attack roll its holder makes. Attacks against the prone
// creature are not its holder's action and are not part of this answer.
func (p *ProneCondition) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	return p.attackRule().AssessAction(in)
}

func (p *ProneCondition) attackRule() attackRollRule {
	return attackRollRule{name: "prone", owner: p.CharacterID, text: attackRollText{
		NotOwner:    "Prone affects only its holder's attacks",
		OnlyAttacks: "Prone affects only attack rolls",
		Applies:     "You are prone",
		Benefit:     "Disadvantage on the attack roll",
	}}
}

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

	longRestSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: ConditionAddressOf(p.CharacterID, p), Remove: p.Remove,
	})
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
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	switch {
	case event.AttackerID == p.CharacterID:
		return p.attackingWhileProne(event, c)
	case event.TargetID == p.CharacterID:
		return p.attackedWhileProne(event, c)
	default:
		return c, nil
	}
}

// attackingWhileProne imposes the prone creature's own disadvantage when its
// attack rule applies — the same rule information asks. No geometry is
// involved: it applies to every attack it makes, at any range.
func (p *ProneCondition) attackingWhileProne(
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	executed, err := executeRule(&executeRuleInput{Name: "prone", Rule: p.attackRule(), Frame: event.Frame})
	if err != nil {
		return c, err
	}
	if executed.Answer.Decision.Applicability != contributions.Applies {
		return c, nil
	}

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

// attackedWhileProne resolves the range split through the held rule:
// advantage from within 5 feet, disadvantage from beyond it, as the answer's
// attack mode says.
//
// Both directions are decided by the rule rather than one being the default,
// because "no modifier" is a third, wrong answer that a half-implemented range
// check silently produces.
func (p *ProneCondition) attackedWhileProne(
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	return applyHeldAttack(&heldAttackInput{
		Name: "prone", Rule: p.heldRule(), Event: event, Chain: c,
		SourceRef: refs.Conditions.Prone(), SourceID: p.CharacterID,
		Label: func(mode contributions.AttackMode) (string, string) {
			if mode == contributions.AttackAdvantage {
				return "prone_target_advantage", "Prone target within 5 feet"
			}
			return "prone_target_disadvantage", "Prone target beyond 5 feet"
		},
	})
}

// heldRule is the by-reference rule for this Prone on its holder — the one
// information asks for a candidate.
func (p *ProneCondition) heldRule() contributions.ActionAssessor {
	return newProneHeldRule(p.CharacterID, heldAddress(p.CharacterID, p))
}
