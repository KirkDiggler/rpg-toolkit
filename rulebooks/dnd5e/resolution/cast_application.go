// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/events"
	actions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// bindApplication fixes the intended list during pure preflight. Resume uses the
// frozen bound declaration, not a second calculation of DCs or target admission.
func (m *castMachine) bindApplication(cast *Participants, resuming bool) error {
	if len(m.profile.Maintenance) == 0 {
		return nil
	}
	described, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: m.profile.Maintenance})
	if err != nil {
		return err
	}
	if described.Concentration {
		if _, err := nativeMaintenanceSpell(m.spell, m.spellName); err != nil {
			return err
		}
	}
	if cast.applications == nil {
		return fmt.Errorf("%w: maintained cast requires interaction binding", ErrBadAction)
	}
	if !resuming {
		spec := conditions.ApplicationSpec{Name: m.spellName, CoordinatorParts: m.profile.Maintenance}
		for _, target := range m.targets {
			for _, effect := range m.profile.Effects {
				recipient, counterpart := m.casterID, target.targetID
				if effect.Recipient == actions.CastRecipientTarget {
					recipient, counterpart = target.targetID, m.casterID
				}
				parameters, err := bindCast(effect, counterpart, m.option)
				if err != nil {
					return err
				}
				if effect.SaveDCKey != "" {
					pending := pendingDelivery{ref: effect.Ref, parameters: parameters, saveDCKey: effect.SaveDCKey, casterID: m.casterID, sourceRef: m.spell.String()}
					bound, err := pending.build(cast, recipient)
					if err != nil {
						return err
					}
					parameters = bound.declaration.Parameters
				}
				lifetime := conditions.LifetimeShared
				if effect.IndependentDuration {
					lifetime = conditions.LifetimeIndependent
				}
				spec.Recipients = append(spec.Recipients, conditions.RecipientSpec{MemberID: recipient, Ref: effect.Ref.String(), AddressSourceID: m.casterID, Config: parameters, Lifetime: lifetime})
			}
		}
		m.applicationSpec = &spec
	}
	if m.applicationSpec == nil {
		return fmt.Errorf("%w: maintained resume has no bound application declaration", ErrBadFrozen)
	}
	if resuming {
		m.application, err = conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: m.applicationID, Spec: *m.applicationSpec, Existing: applicationParticipants(cast)})
	} else {
		m.application, err = conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: m.applicationID, CoordinatorID: m.casterID, Origin: dndevents.ConditionOrigin{Ref: m.spell, EntityID: m.casterID}, Spec: *m.applicationSpec, Existing: applicationParticipants(cast)})
	}
	return err
}

func (m *castMachine) applicationDeliveries(index int) ([]preparedDelivery, error) {
	var deliveries []preparedDelivery
	for effectIndex, effect := range m.profile.Effects {
		specIndex := index*len(m.profile.Effects) + effectIndex
		candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: m.application, SpecIndex: specIndex})
		if err != nil {
			return nil, err
		}
		name, err := conditionDescription(effect.Ref)
		if err != nil {
			return nil, err
		}
		prepared := preparedCondition{declaration: actions.ConditionApplication{Ref: effect.Ref}, behavior: candidate.Condition, name: name}
		if candidate.Candidate.Shared {
			prepared.applicationID = m.applicationID
			prepared.memberKey = candidate.Candidate.Key
		}
		deliveries = append(deliveries, preparedDelivery{condition: prepared, recipientID: candidate.Candidate.Address.MemberID})
	}
	return deliveries, nil
}

func (m *castMachine) maintainedTarget(index int) (Machine, error) {
	deliveries, err := m.applicationDeliveries(index)
	if err != nil {
		return nil, err
	}
	target := m.targets[index].targetID
	definition := actions.Definition{Ref: m.spell, Name: m.spellName, Cast: &m.profile}
	if m.profile.Save != nil {
		var prepared *preparedCondition
		if len(deliveries) == 1 {
			value := deliveries[0].condition
			prepared = &value
		}
		cause := dndevents.SaveCause{Trigger: dndevents.SaveTriggerSpell, EffectRef: &m.spell, InstigatorID: m.casterID}
		return newGatedCast(definition, m.casterID, target, m.option, cause, m.roller, prepared)
	}
	return newGatelessCast(definition, m.casterID, target, m.option, m.roller, deliveries)
}

func (m *castMachine) beginApplication(next Step) (Step, error) {
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: m.application})
	if err != nil {
		return nil, err
	}
	created := Gather{name: "establish application " + m.applicationID, run: func(ctx context.Context, bus events.EventBus) (Step, error) {
		caster, err := m.cast.entity(m.casterID)
		if err != nil {
			return nil, err
		}
		if err := dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: caster, Source: dndevents.ConditionSourceSpell, Condition: owner.Condition, ApplicationID: m.applicationID}); err != nil {
			return nil, err
		}
		return next, nil
	}}
	if old, exists := m.application.Replacement(); exists {
		return publishRemoval(&ConditionRemoval{Owner: old, Reason: conditions.ConcentrationEndedRecast}, func(ImposedEffect) (Step, error) { return created, nil }), nil
	}
	return created, nil
}

// A delivered entry was finalized by publishCondition. Any entry still pending
// when its target resolves is a no-effect result, finalized in a bus-owned step.
func (m *castMachine) finishTarget(index int, next Step) Step {
	if m.application == nil {
		return next
	}
	return Gather{name: "finalize target delivery", run: func(ctx context.Context, _ events.EventBus) (Step, error) {
		var info *conditions.LinkedApplicationInfo
		for _, condition := range heldConditions(m.cast, m.casterID) {
			if provider, ok := condition.(interface {
				LinkedApplicationInfo() conditions.LinkedApplicationInfo
			}); ok {
				candidate := provider.LinkedApplicationInfo()
				if candidate.ID == m.applicationID && candidate.Role == conditions.LinkedCoordinator {
					info = &candidate
					break
				}
			}
		}
		if info == nil {
			return nil, fmt.Errorf("%w: application ended before delivery completed", ErrBadStep)
		}
		for effectIndex, effect := range m.profile.Effects {
			if effect.IndependentDuration {
				continue
			}
			candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: m.application, SpecIndex: index*len(m.profile.Effects) + effectIndex})
			if err != nil {
				return nil, err
			}
			if slices.Contains(info.PendingMembers, candidate.Candidate.Key) {
				if _, err := m.cast.applications.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: m.applicationID, Key: candidate.Candidate.Key}); err != nil {
					return nil, err
				}
			}
		}
		return next, nil
	}}
}
