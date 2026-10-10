// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// frozenBeforeRoll is a strike stopped before any attack dice exist, on a
// defender's attack-roll reaction. The fold is stored, never replayed; only
// the answered offer is removed from it on resume.
type frozenBeforeRoll struct {
	AttackerID  string                     `json:"attacker_id"`
	TargetID    string                     `json:"target_id"`
	Opportunity bool                       `json:"opportunity,omitempty"`
	Definition  combatActions.Definition   `json:"definition"`
	Folded      dndEvents.AttackChainEvent `json:"folded"`
	Outcome     StrikeOutcome              `json:"outcome"`
	Offer       dndEvents.AttackRollOffer  `json:"offer"`
	Cost        *Cost                      `json:"cost"`
}

// poseBeforeRoll pauses on the first before-roll offer the attack chain
// folded, at the price the one table states for it.
func (m *strikeMachine) poseBeforeRoll(folded dndEvents.AttackChainEvent) (Step, error) {
	offer := folded.BeforeRollOffers[0]
	cost := reactionCost(offer.ReactorID, coreResources.ResourceKey(offer.ResourceKey))
	frozen, err := writeFrozen(machineBeforeRoll, PauseBeforeRoll, frozenBeforeRoll{
		AttackerID: m.in.AttackerID, TargetID: m.in.TargetID, Opportunity: m.in.Opportunity,
		Definition: m.in.Definition, Folded: folded, Outcome: m.outcome, Offer: offer, Cost: cost,
	})
	if err != nil {
		return nil, err
	}
	return Pause{
		Kind:   PauseBeforeRoll,
		Ask:    Ask{Audience: offer.ReactorID, Offer: offerFromAttackRoll(offer)},
		Cost:   cost,
		Frozen: frozen,
	}, nil
}

// resumeBeforeRoll finishes a strike paused before its d20, keeping exactly
// the validation a before-roll blob always had: both sides named, the reactor
// is the target, no die was rolled, and the offer names its pool, its ref and
// the disadvantage it imposes. The answer must also be one the frozen offer
// accepts ([ErrNotOffered]).
func resumeBeforeRoll(h frozenHeader, in *ResumeInput) (Machine, error) {
	var frozen frozenBeforeRoll
	if err := decodeState(h, &frozen); err != nil {
		return nil, err
	}
	if err := samePrice(frozen.Cost, in.Pause.Cost); err != nil {
		return nil, err
	}
	if frozen.AttackerID == "" || frozen.TargetID == "" || frozen.Offer.ReactorID != frozen.TargetID ||
		frozen.Outcome.Roll != 0 || frozen.Offer.ResourceKey == "" || frozen.Offer.Ref.ID == "" ||
		frozen.Offer.Disadvantage.SourceRef == nil || len(frozen.Folded.BeforeRollOffers) == 0 || frozen.Cost == nil {
		return nil, fmt.Errorf("%w: invalid pre-roll reaction", ErrBadFrozen)
	}
	// The answer is checked against the offer as it was frozen, not only as
	// the host stored it: a host that emptied the stored choices cannot take
	// a reaction without choosing it.
	if err := in.Answer.accepts(offerFromAttackRoll(frozen.Offer)); err != nil {
		return nil, err
	}
	return resumedStrike(frozen.AttackerID, frozen.TargetID, frozen.Definition, frozen.Opportunity, in.Roller,
		&strikeResume{answer: in.Answer, beforeRoll: &frozen}), nil
}

// answerBeforeRoll applies the answer to a before-roll reaction. Taken, the
// reactor pays the frozen price at the one door and the disadvantage joins
// the fold; declined, nothing is charged. Then the strike rolls.
func (m *strikeMachine) answerBeforeRoll() Step {
	return Gather{name: "answer pre-roll reaction", run: func(ctx context.Context, _ events.EventBus) (Step, error) {
		frozen := m.resume.beforeRoll
		m.outcome = frozen.Outcome
		// Prior follow-ups were persisted with the initial interaction.
		m.outcome.FollowUps = nil
		folded := frozen.Folded
		if m.resume.answer.Taken() {
			if err := payAtTheDoor(ctx, frozen.Cost, m.cast); err != nil {
				return nil, err
			}
			folded.DisadvantageSources = append(folded.DisadvantageSources, frozen.Offer.Disadvantage)
		}
		// The fold is never replayed. Only this answered offer is removed.
		folded.BeforeRollOffers = folded.BeforeRollOffers[1:]
		return m.afterAttackChain(ctx, folded)
	}}
}
