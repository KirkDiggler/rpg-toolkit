// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const conditionSourceType = "condition"

func detachSubscription(bus events.EventBus, id string) generic.Detach {
	return func(ctx context.Context) error { return bus.Unsubscribe(ctx, id) }
}
func rollSource(out *generic.Bound, in BindInput) dndevents.RollSource {
	return dndevents.RollSource{Ref: out.Host().Ref, Name: in.DisplayName, SourceID: in.Origin.EntityID}
}
func assessmentInput(out *generic.Bound, in BindInput, q Query) *AssessInput {
	return &AssessInput{Output: out, Query: q, Origin: in.Origin, DisplayName: in.DisplayName, Attachment: in.Attachment}
}

func attachDamage(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput) (generic.Detach, error) {
	m, err := damageMeaning(out.Params())
	if err != nil {
		return nil, err
	}
	id, err := dndevents.DamageChain.On(bus).SubscribeWithChain(ctx, func(_ context.Context, event *dndevents.DamageChainEvent, c chain.Chain[*dndevents.DamageChainEvent]) (chain.Chain[*dndevents.DamageChainEvent], error) {
		if event == nil {
			return c, fmt.Errorf("damage effect received nil event")
		}
		if event.AttackerID != out.Host().Holder {
			return c, nil
		}
		if event.Frame.Actor != event.AttackerID {
			return c, fmt.Errorf("%w: damage actor and frame disagree", contributions.ErrRuleCannotAnswer)
		}
		answer, err := executeAssessment(assessmentInput(out, in, Query{Action: &event.Frame}), m)
		if err != nil || answer.effect.State != contributions.StateApplies {
			return c, err
		}
		amount := answer.amount
		return c, c.Add(combat.StageFeatures, out.ID(), func(_ context.Context, e *dndevents.DamageChainEvent) (*dndevents.DamageChainEvent, error) {
			e.Components = append(e.Components, dndevents.DamageComponent{Source: dndevents.DamageSourceCondition, DamageType: e.WeaponDamageType, Roll: dndevents.RollComponent{Source: rollSource(out, in), Modifier: &amount}})
			return e, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return detachSubscription(bus, id), nil
}

func attachResistance(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput) (generic.Detach, error) {
	m, err := resistanceMeaning(out.Params())
	if err != nil {
		return nil, err
	}
	id, err := dndevents.IncomingDamageChain.On(bus).SubscribeWithChain(ctx, func(_ context.Context, event *dndevents.IncomingDamageEvent, c chain.Chain[*dndevents.IncomingDamageEvent]) (chain.Chain[*dndevents.IncomingDamageEvent], error) {
		if event == nil {
			return c, fmt.Errorf("resistance effect received nil event")
		}
		if event.TargetID() != out.Host().Holder {
			return c, nil
		}
		answer, err := executeAssessment(assessmentInput(out, in, Query{Incoming: event}), m)
		if err != nil || answer.effect.State != contributions.StateApplies {
			return c, err
		}
		return c, c.Add(combat.StageFinal, out.ID(), func(_ context.Context, e *dndevents.IncomingDamageEvent) (*dndevents.IncomingDamageEvent, error) {
			for _, kind := range answer.types {
				e.Multipliers = append(e.Multipliers, dndevents.DamageMultiplier{Category: dndevents.DamageSourceCondition, Source: rollSource(out, in), DamageType: kind, Factor: dndevents.DamageFactorResistance})
			}
			return e, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return detachSubscription(bus, id), nil
}

func attachAdvantage(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput) (generic.Detach, error) {
	m, err := advantageMeaning(out.Params())
	if err != nil {
		return nil, err
	}
	if m.domain == domainAttack {
		return attachAttackAdvantage(ctx, bus, out, in, m)
	}
	if m.domain == domainSave {
		return attachSaveAdvantage(ctx, bus, out, in)
	}
	return attachCheckAdvantage(ctx, bus, out, in)
}

func attachSaveAdvantage(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput) (generic.Detach, error) {
	m, err := advantageMeaning(out.Params())
	if err != nil {
		return nil, err
	}
	id, err := dndevents.SavingThrowChain.On(bus).SubscribeWithChain(ctx, func(_ context.Context, event *dndevents.SavingThrowChainEvent, c chain.Chain[*dndevents.SavingThrowChainEvent]) (chain.Chain[*dndevents.SavingThrowChainEvent], error) {
		if event == nil {
			return c, fmt.Errorf("save advantage received nil event")
		}
		if event.SaverID != out.Host().Holder {
			return c, nil
		}
		answer, err := executeAssessment(assessmentInput(out, in, Query{Save: event}), m)
		if err != nil || answer.effect.State != contributions.StateApplies {
			return c, err
		}
		return c, c.Add(combat.StageConditions, out.ID(), func(_ context.Context, e *dndevents.SavingThrowChainEvent) (*dndevents.SavingThrowChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, dndevents.SaveModifierSource{Name: in.DisplayName, SourceType: conditionSourceType, SourceRef: out.Host().Ref, EntityID: in.Origin.EntityID})
			return e, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return detachSubscription(bus, id), nil
}

func attachCheckAdvantage(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput) (generic.Detach, error) {
	m, err := advantageMeaning(out.Params())
	if err != nil {
		return nil, err
	}
	id, err := dndevents.AbilityCheckChain.On(bus).SubscribeWithChain(ctx, func(_ context.Context, event *dndevents.AbilityCheckChainEvent, c chain.Chain[*dndevents.AbilityCheckChainEvent]) (chain.Chain[*dndevents.AbilityCheckChainEvent], error) {
		if event == nil {
			return c, fmt.Errorf("check advantage received nil event")
		}
		if event.CheckerID != out.Host().Holder {
			return c, nil
		}
		answer, err := executeAssessment(assessmentInput(out, in, Query{Check: event}), m)
		if err != nil || answer.effect.State != contributions.StateApplies {
			return c, err
		}
		return c, c.Add(combat.StageConditions, out.ID(), func(_ context.Context, e *dndevents.AbilityCheckChainEvent) (*dndevents.AbilityCheckChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, dndevents.CheckModifierSource{Name: in.DisplayName, SourceType: conditionSourceType, SourceRef: out.Host().Ref, EntityID: in.Origin.EntityID})
			return e, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return detachSubscription(bus, id), nil
}
