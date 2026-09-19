// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ReactionDecline is the no-cost answer to a post-hit offer.
const ReactionDecline = "decline"

// RetaliationOutcome preserves provider identity and the save/damage result.
type RetaliationOutcome struct {
	Offer    dndEvents.PostHitOffer
	TargetID string
	Result   ContestOutcome
}

func (m *strikeMachine) resumePostHit(ctx context.Context) (Step, error) {
	frozen := m.resume.frozen
	if len(frozen.Retaliation) > 0 {
		nested, err := newContestResumed(frozen.Retaliation, m.resume.answer, m.in.Roller)
		if err != nil {
			return nil, err
		}
		return m.retaliationRequest(nested), nil
	}
	if m.resume.answer == OfferKeep {
		if m.resume.option != "" {
			return nil, fmt.Errorf("%w: declining a reaction carries no option", ErrNotOffered)
		}
		return Done{Outcome: m.outcome}, nil
	}
	var choice *dndEvents.PostHitOption
	for i := range frozen.PostHit.Options {
		if frozen.PostHit.Options[i].ID == m.resume.option {
			choice = &frozen.PostHit.Options[i]
			break
		}
	}
	if choice == nil {
		return nil, fmt.Errorf("%w: reaction option %q", ErrNotOffered, m.resume.option)
	}
	offer := frozen.PostHit
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
		price := &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionReaction: 1}, Pools: map[coreResources.ResourceKey]int{coreResources.ResourceKey(offer.ResourceKey): 1}}
		if err := payAtTheDoor(ctx, &Cost{PayerID: offer.ReactorID, Profile: price}, m.cast); err != nil {
			return nil, err
		}
		return m.retaliationRequest(startedMachine{first: first}), nil
	}}, nil
}

func (m *strikeMachine) retaliationRequest(nested Machine) Request {
	return Request{name: "post-hit retaliation", machine: nested,
		next: func(_ context.Context, out Outcome) (Step, error) {
			result, ok := out.(ContestOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: retaliation produced %T", ErrBadStep, out)
			}
			m.outcome.Retaliation = &RetaliationOutcome{Offer: *m.resume.frozen.PostHit, TargetID: m.in.AttackerID, Result: result}
			return Done{Outcome: m.outcome}, nil
		},
		onPose: func(_ context.Context, pose Pose) (Step, error) {
			frozen := m.resume.frozen
			frozen.Retaliation = append(json.RawMessage(nil), pose.Frozen...)
			raw, err := json.Marshal(frozen)
			if err != nil {
				return nil, err
			}
			return Pose{Ask: pose.Ask, Frozen: raw}, nil
		},
	}
}
