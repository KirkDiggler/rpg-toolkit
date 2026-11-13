// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const domainAttack domain = "attack"

const (
	sideByHolder      = "by_holder"
	sideAgainstHolder = "against_holder"
)

type advantageRoute struct {
	On   domain         `json:"on"`
	Side jsontext.Value `json:"side,omitempty"`
}
type attackAdvantageParams struct {
	On   domain `json:"on"`
	Side string `json:"side"`
}

func advantageFacts(m meaning) []machine.FactName {
	if m.domain == domainAttack {
		return []machine.FactName{FactAbility, FactMelee, FactWeaponPool, FactOpportunity}
	}
	return []machine.FactName{FactAbility}
}
func checkAdvantageBinding(out generic.Output, in BindInput) error {
	m, err := advantageMeaning(out.Params)
	if err != nil {
		return err
	}
	if m.domain != domainAttack || m.side != sideAgainstHolder {
		return nil
	}
	return validateAttachment(in.Attachment)
}
func validateAttachment(in *AttachmentIdentity) error {
	if in == nil || in.Address.MemberID == "" || in.Address.SourceID == "" || in.ApplicationID == "" {
		return fmt.Errorf("against-holder effect requires its exact local attachment and application")
	}
	if _, err := core.ParseString(in.Address.ConditionRef); err != nil {
		return err
	}
	return nil
}

func assessAttack(in *AssessInput, m meaning) (assessment, error) {
	var result assessment
	if err := validateBinding(&BindInput{Origin: in.Origin, DisplayName: in.DisplayName}); err != nil {
		return result, err
	}
	if err := in.Query.Validate(); err != nil {
		return result, err
	}
	host := in.Output.Host()
	result.effect = contributions.Effect{ID: in.Output.ID(), Source: contributions.Source{Ref: host.Ref, Name: in.DisplayName, SourceID: in.Origin.EntityID}, Description: m.description, Participation: contributions.ContributesNow}
	depends := func(reason string) (assessment, error) {
		result.effect.State = contributions.StateDepends
		result.effect.Reason = reason
		return result, nil
	}
	no := func(reason string) (assessment, error) {
		result.effect.State = contributions.StateDoesNotApply
		result.effect.Reason = reason
		return result, nil
	}
	if in.Query.domain() == domainNone {
		return depends("An attack is needed to assess this effect")
	}
	if in.Query.Action == nil {
		return no("This effect applies to attack rolls")
	}
	frame := in.Query.Action
	roll, _ := frame.Action.Roll.Get()
	if roll != contributions.RollKindAttack {
		return no("This effect applies to attack rolls")
	}
	if m.side == sideByHolder {
		if frame.Actor != host.Holder {
			return no("This effect applies to attacks by its holder")
		}
	} else {
		if err := validateAttachment(in.Attachment); err != nil {
			return result, err
		}
		address := in.Attachment.Address
		if address.MemberID != host.Holder || address.ConditionRef != host.Ref.String() {
			return result, fmt.Errorf("attachment does not identify this output's holder/ref")
		}
		target, known := frame.Target.Get()
		if !known {
			return depends("Depends on the target")
		}
		if target != host.Holder {
			return no("This effect applies to attacks against its holder")
		}
		held, known := frame.HeldBy(host.Holder)
		if !known {
			return depends("Depends on what the target holds")
		}
		if !slices.Contains(held, contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID}) {
			return no("The target does not hold this effect")
		}
	}
	verdict, reason := in.Output.Answer(in.Query)
	result.effect.Reason = reason
	switch verdict {
	case machine.Applies:
		result.effect.State = contributions.StateApplies
		result.effect.Benefit = "Advantage"
	case machine.DoesNotApply:
		result.effect.State = contributions.StateDoesNotApply
	case machine.Depends:
		result.effect.State = contributions.StateDepends
	default:
		return result, fmt.Errorf("unknown guard verdict %q", verdict)
	}
	return result, nil
}

func attachAttackAdvantage(ctx context.Context, bus events.EventBus, out *generic.Bound, in BindInput, m meaning) (generic.Detach, error) {
	if err := checkAdvantageBinding(out.Definition(), in); err != nil {
		return nil, err
	}
	id, err := dndevents.AttackChain.On(bus).SubscribeWithChain(ctx, func(_ context.Context, event dndevents.AttackChainEvent, c chain.Chain[dndevents.AttackChainEvent]) (chain.Chain[dndevents.AttackChainEvent], error) {
		holder := out.Host().Holder
		if (m.side == sideByHolder && event.AttackerID != holder) || (m.side == sideAgainstHolder && event.TargetID != holder) {
			return c, nil
		}
		target, known := event.Frame.Target.Get()
		roll, _ := event.Frame.Action.Roll.Get()
		if event.Frame.Actor != event.AttackerID || !known || target != event.TargetID || roll != contributions.RollKindAttack {
			return c, fmt.Errorf("%w: attack and frame disagree", contributions.ErrRuleCannotAnswer)
		}
		if m.side == sideAgainstHolder {
			if err := validateApplyingDetail(out, in, event.Frame); err != nil {
				return c, fmt.Errorf("%w: %w", contributions.ErrRuleCannotAnswer, err)
			}
		}
		answer, err := executeAssessment(assessmentInput(out, in, Query{Action: &event.Frame}), m)
		if err != nil || answer.effect.State != contributions.StateApplies {
			return c, err
		}
		return c, c.Add(combat.StageFeatures, out.ID(), func(_ context.Context, e dndevents.AttackChainEvent) (dndevents.AttackChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, dndevents.AttackModifierSource{SourceRef: out.Host().Ref, SourceID: in.Origin.EntityID, Reason: in.DisplayName})
			return e, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return detachSubscription(bus, id), nil
}

func validateApplyingDetail(out *generic.Bound, in BindInput, frame contributions.Frame) error {
	if err := frame.Validate(); err != nil {
		return err
	}
	address := in.Attachment.Address
	for _, detail := range frame.HeldEffects {
		if detail.MemberID != address.MemberID || detail.Held.Ref != address.ConditionRef || detail.Held.SourceID != address.SourceID {
			continue
		}
		if detail.ApplicationID != in.Attachment.ApplicationID || detail.DisplayName != in.DisplayName || detail.OriginID != in.Origin.EntityID || detail.OriginRef != in.Origin.Ref {
			return fmt.Errorf("configured holding generation or attribution disagrees with applying attachment")
		}
		for _, value := range detail.Outputs {
			if value.ID != out.ID() {
				continue
			}
			same, err := sameOutput(value.Definition, out.Definition())
			if err != nil {
				return err
			}
			if !same {
				return fmt.Errorf("configured output disagrees with applying definition")
			}
			return nil
		}
		return fmt.Errorf("configured holding omits an applying output")
	}
	return fmt.Errorf("authoritative frame lacks configured holding detail")
}
