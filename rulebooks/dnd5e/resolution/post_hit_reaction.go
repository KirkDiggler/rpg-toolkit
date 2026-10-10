// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// RetaliationOutcome preserves provider identity and the save/damage result.
type RetaliationOutcome struct {
	Offer    dndEvents.PostHitOffer
	TargetID string
	Result   ContestOutcome
}

// frozenPostHit is a strike stopped after a settled hit, on a defender's
// post-hit reaction. The hit was told with the pause; the resume tells only
// the retaliation, as a [StrikeOutcome.Continued] strike.
//
// When the retaliation's own save paused, Retaliation holds that contest's
// whole frozen header and Cost is nil: the reaction was paid when it was taken.
type frozenPostHit struct {
	AttackerID  string                   `json:"attacker_id"`
	TargetID    string                   `json:"target_id"`
	Opportunity bool                     `json:"opportunity,omitempty"`
	Definition  combatActions.Definition `json:"definition"`
	Outcome     StrikeOutcome            `json:"outcome"`
	Offer       dndEvents.PostHitOffer   `json:"offer"`
	Cost        *Cost                    `json:"cost,omitempty"`
	Retaliation json.RawMessage          `json:"retaliation,omitempty"`
}

// posePostHit pauses on a settled hit for the defender's reaction, at the
// price the one table states for it. The settled hit rides the pause to the
// driver, so it is told now, once.
func (m *strikeMachine) posePostHit(offer dndEvents.PostHitOffer) (Step, error) {
	cost := reactionCost(offer.ReactorID, coreResources.ResourceKey(offer.ResourceKey))
	frozen, err := writeFrozen(machinePostHit, PausePostHit, frozenPostHit{
		AttackerID: m.in.AttackerID, TargetID: m.in.TargetID, Opportunity: m.in.Opportunity,
		Definition: m.in.Definition, Outcome: m.outcome, Offer: offer, Cost: cost,
	})
	if err != nil {
		return nil, err
	}
	return Pause{
		Kind:    PausePostHit,
		Ask:     Ask{Audience: offer.ReactorID, Offer: offerFromPostHit(offer)},
		Cost:    cost,
		Frozen:  frozen,
		settled: m.reported(),
	}, nil
}

// resumePostHit finishes a strike paused on a post-hit reaction, or on the
// save of a retaliation that reaction started.
func resumePostHit(h frozenHeader, in *ResumeInput) (Machine, error) {
	var frozen frozenPostHit
	if err := decodeState(h, &frozen); err != nil {
		return nil, err
	}
	if err := samePrice(frozen.Cost, in.Pause.Cost); err != nil {
		return nil, err
	}
	if frozen.AttackerID == "" || frozen.TargetID == "" {
		return nil, fmt.Errorf("%w: invalid post-hit identity", ErrBadFrozen)
	}
	if frozen.Offer.ReactorID != frozen.TargetID || !frozen.Outcome.Hit {
		return nil, fmt.Errorf("%w: incomplete post-hit phase", ErrBadFrozen)
	}
	resume := &strikeResume{answer: in.Answer, postHit: &frozen}
	if len(frozen.Retaliation) > 0 {
		nested, err := resumeInner(frozen.Retaliation, in, machineContest)
		if err != nil {
			return nil, err
		}
		resume.retaliation = nested
	} else if h.Kind != PausePostHit {
		return nil, fmt.Errorf("%w: a post-hit reaction paused as %q", ErrBadFrozen, h.Kind)
	}
	return resumedStrike(frozen.AttackerID, frozen.TargetID, frozen.Definition, frozen.Opportunity, in.Roller, resume), nil
}

// answerPostHit applies the answer to a post-hit reaction. Declined, the
// strike finishes and nothing is charged. Taken, the chosen option's follow-up
// is validated, the reactor pays the frozen price at the one door, and the
// retaliation runs.
func (m *strikeMachine) answerPostHit(ctx context.Context) (Step, error) {
	frozen := m.resume.postHit
	if m.resume.retaliation != nil {
		return m.retaliationRequest(m.resume.retaliation), nil
	}
	if !m.resume.answer.Taken() {
		return Done{Outcome: m.continued()}, nil
	}
	var choice *dndEvents.PostHitOption
	for i := range frozen.Offer.Options {
		if frozen.Offer.Options[i].ID == m.resume.answer.Option() {
			choice = &frozen.Offer.Options[i]
			break
		}
	}
	if choice == nil {
		return nil, fmt.Errorf("%w: reaction option %q", ErrNotOffered, m.resume.answer.Option())
	}
	offer := frozen.Offer
	gate := saves.NewSaveGate(choice.Ability, choice.DC)
	if choice.HalfOnSave {
		gate.OnSuccess = saves.Half
	}
	nested := NewContest(&ContestInput{Gate: gate, SaverID: frozen.AttackerID,
		Damage: []damage.Damage{{Dice: choice.Dice, Type: choice.DamageType}}, SourceName: offer.Name,
		Cause: dndEvents.SaveCause{EffectRef: &offer.Ref, InstigatorID: offer.ReactorID}, Roller: m.in.Roller})
	// Validate the follow-up before charging. Start is pure, and the returned
	// step is reused rather than starting the nested machine twice.
	first, err := nested.Start(ctx, m.cast)
	if err != nil {
		return nil, err
	}
	return Gather{name: "pay post-hit reaction", run: func(ctx context.Context, _ events.EventBus) (Step, error) {
		if err := payAtTheDoor(ctx, frozen.Cost, m.cast); err != nil {
			return nil, err
		}
		return m.retaliationRequest(startedMachine{first: first}), nil
	}}, nil
}

// continued is the half of a strike a post-hit resume reports: who struck
// whom, and the retaliation. Every hit field is zero, because the hit was told
// in the output that paused. A strike resumed inside a cast reports whole
// instead, because a cast is one told unit and told nothing at its pause.
func (m *strikeMachine) continued() StrikeOutcome {
	if m.whole {
		return m.reported()
	}
	return StrikeOutcome{
		Continued:   true,
		AttackerID:  m.outcome.AttackerID,
		TargetID:    m.outcome.TargetID,
		Retaliation: m.outcome.Retaliation,
	}
}

// retaliationRequest runs the retaliation's contest and finishes the strike
// with its result. A retaliation whose save pauses re-freezes the strike with
// the contest's header inside, at no price: the reaction was already paid.
func (m *strikeMachine) retaliationRequest(nested Machine) Request {
	return Request{name: "post-hit retaliation", machine: nested,
		next: func(_ context.Context, out Outcome) (Step, error) {
			result, ok := out.(ContestOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: retaliation produced %T", ErrBadStep, out)
			}
			m.outcome.Retaliation = &RetaliationOutcome{Offer: m.resume.postHit.Offer, TargetID: m.in.AttackerID, Result: result}
			return Done{Outcome: m.continued()}, nil
		},
		onPause: func(_ context.Context, pause Pause) (Step, error) {
			frozen := *m.resume.postHit
			frozen.Retaliation = append(json.RawMessage(nil), pause.Frozen...)
			frozen.Cost = nil
			raw, err := writeFrozen(machinePostHit, pause.Kind, frozen)
			if err != nil {
				return nil, err
			}
			return Pause{Kind: pause.Kind, Ask: pause.Ask, Cost: pause.Cost, Frozen: raw, followUps: pause.followUps}, nil
		},
	}
}
