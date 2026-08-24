// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ContestInput describes a declared condition somebody may save against.
type ContestInput struct {
	Gate        *saves.SaveGate
	SaverID     string
	Application combatActions.ConditionApplication
	Cause       dnd5eEvents.SaveCause
	DamageTaken int
	Roller      dice.Roller
	prepared    *preparedCondition
}

// ImposedEffect names one condition a contest put on, or would have put on, the saver.
type ImposedEffect struct {
	Ref         *core.Ref
	Description string
}

// ContestOutcome records the requested save and whether its declared condition landed.
type ContestOutcome struct {
	Save      SaveOutcome
	DC        int
	Ability   abilities.Ability
	Succeeded bool
	AtStake   ImposedEffect
	Imposed   []ImposedEffect
}

func (ContestOutcome) isOutcome() {}

type preparedCondition struct {
	declaration combatActions.ConditionApplication
	behavior    dnd5eEvents.ConditionBehavior
}

// validateConditionGate checks the subset of save gates resolution can execute.
// Shared action data permits recurrence, but this module currently resolves
// only the immediate save that negates a condition on success.
func validateConditionGate(gate *saves.SaveGate) error {
	if gate == nil {
		return fmt.Errorf("%w: contest has no save gate", ErrNilInput)
	}
	if err := gate.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrBadGate, err)
	}
	for _, ability := range gate.Abilities {
		if !supportedSaveAbility(ability) {
			return fmt.Errorf("%w: unsupported save ability %q", ErrBadGate, ability)
		}
	}
	if gate.OnSuccess != saves.Negated {
		return fmt.Errorf("%w: a condition contest must negate on success", ErrBadGate)
	}
	if gate.Recurrence != saves.RecurrenceNone {
		return fmt.Errorf("%w: %q", ErrRecurrenceUnsupported, gate.Recurrence)
	}

	return nil
}

func supportedSaveAbility(ability abilities.Ability) bool {
	switch ability {
	case abilities.STR, abilities.DEX, abilities.CON, abilities.INT, abilities.WIS, abilities.CHA:
		return true
	default:
		return false
	}
}

func prepareCondition(
	application combatActions.ConditionApplication, targetID, sourceRef string,
) (preparedCondition, error) {
	if err := application.Validate(); err != nil {
		return preparedCondition{}, fmt.Errorf("%w: %w", ErrBadAction, err)
	}
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:         application.Ref.String(),
		Config:      application.Parameters,
		CharacterID: targetID,
		SourceRef:   sourceRef,
	})
	if err != nil {
		return preparedCondition{}, fmt.Errorf("build condition %s for %q: %w", application.Ref.ID, targetID, err)
	}
	return preparedCondition{declaration: application.Clone(), behavior: built.Condition}, nil
}

func (p preparedCondition) atStake() ImposedEffect {
	ref := p.declaration.Ref
	return ImposedEffect{Ref: &ref, Description: fmt.Sprintf("the %s condition", ref.ID)}
}

func publishPreparedCondition(
	prepared preparedCondition, cast *Participants, targetID string, next func() (Step, error),
) Gather {
	return Gather{
		name: "impose " + prepared.atStake().Description,
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			target, err := cast.entity(targetID)
			if err != nil {
				return nil, err
			}
			err = dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
				Target:    target,
				Type:      dnd5eEvents.ConditionType(prepared.declaration.Ref.ID),
				Source:    dnd5eEvents.ConditionSourceDamage,
				Condition: prepared.behavior,
			})
			if err != nil {
				return nil, fmt.Errorf("apply %s to %q: %w", prepared.declaration.Ref.ID, targetID, err)
			}
			return next()
		},
	}
}

// NewContest returns the machine for one save-gated condition declaration.
func NewContest(in *ContestInput) Machine { return &contestMachine{in: in} }

type contestMachine struct {
	in       *ContestInput
	cast     *Participants
	prepared preparedCondition
}

func (m *contestMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	m.cast = cast
	if err := validateConditionGate(m.in.Gate); err != nil {
		return nil, err
	}

	if m.in.prepared != nil {
		m.prepared = *m.in.prepared
	} else {
		source := m.in.Application.Ref.String()
		if m.in.Cause.EffectRef != nil {
			source = m.in.Cause.EffectRef.String()
		}
		prepared, err := prepareCondition(m.in.Application, m.in.SaverID, source)
		if err != nil {
			return nil, err
		}
		m.prepared = prepared
	}

	ability, err := m.chooseAbility(cast)
	if err != nil {
		return nil, err
	}
	dc := m.in.Gate.DC.DC(saves.DCInput{DamageTaken: m.in.DamageTaken})
	return requestSave(&SaveInput{
		SaverID: m.in.SaverID,
		Ability: ability,
		DC:      dc,
		Cause:   m.in.Cause,
		Roller:  m.in.Roller,
	}, func(_ context.Context, save SaveOutcome) (Step, error) {
		return m.resolve(ability, dc, save)
	}), nil
}

func (m *contestMachine) chooseAbility(cast *Participants) (abilities.Ability, error) {
	best := m.in.Gate.Abilities[0]
	bestModifier, err := savingThrowModifier(cast, m.in.SaverID, best)
	if err != nil {
		return "", err
	}
	for _, ability := range m.in.Gate.Abilities[1:] {
		modifier, modErr := savingThrowModifier(cast, m.in.SaverID, ability)
		if modErr != nil {
			return "", modErr
		}
		if modifier > bestModifier {
			best, bestModifier = ability, modifier
		}
	}
	return best, nil
}

func (m *contestMachine) resolve(ability abilities.Ability, dc int, save SaveOutcome) (Step, error) {
	outcome := ContestOutcome{
		Save:      save,
		DC:        dc,
		Ability:   ability,
		Succeeded: save.Result != nil && save.Result.Success,
		AtStake:   m.prepared.atStake(),
	}
	if outcome.Succeeded {
		return Done{Outcome: outcome}, nil
	}
	return publishPreparedCondition(m.prepared, m.cast, m.in.SaverID, func() (Step, error) {
		outcome.Imposed = []ImposedEffect{m.prepared.atStake()}
		return Done{Outcome: outcome}, nil
	}), nil
}

func savingThrowModifier(cast *Participants, saverID string, ability abilities.Ability) (int, error) {
	if saver, ok := cast.Character(saverID); ok {
		return saver.GetSavingThrowModifier(ability), nil
	}
	if saver, ok := cast.Monster(saverID); ok {
		return saver.GetSavingThrowModifier(ability), nil
	}
	return 0, fmt.Errorf("%w: %q", ErrNoSaver, saverID)
}

func (p *Participants) entity(id string) (core.Entity, error) {
	if character, ok := p.Character(id); ok {
		return character, nil
	}
	if monster, ok := p.Monster(id); ok {
		return monster, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrNoSaver, id)
}

func requestSave(in *SaveInput, next func(context.Context, SaveOutcome) (Step, error)) Request {
	return Request{
		name:    "saving throw",
		machine: NewSave(in),
		next: func(ctx context.Context, out Outcome) (Step, error) {
			save, ok := out.(SaveOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: saving throw returned %T", ErrBadStep, out)
			}
			return next(ctx, save)
		},
	}
}
