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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ContestInput describes a consequence somebody may save against.
//
// The gate is data the content declared — the wolf's bite carries "DC 11 STR or
// prone" in its stat block — and this input is the whole of what turns that
// declaration into an interaction. Nothing here restates the gate: the
// abilities, the DC formula, and what a success buys are read off it, so a
// caller cannot pass a gate saying one thing and a DC saying another.
type ContestInput struct {
	// Gate is the declaration being contested (ADR-0039). Required: a nil gate
	// means there is nothing to contest, which is a caller defect rather than
	// an interaction with no consequence.
	Gate *saves.SaveGate

	// SaverID names the participant the consequence would land on.
	SaverID string

	// Consequence is what a failed save imposes.
	Consequence Consequence

	// Cause is what provoked the save, and is what effect predicates read to
	// decide whether they apply.
	Cause dnd5eEvents.SaveCause

	// DamageTaken is the damage of the blow that provoked this save, read only
	// by the derived DC formulas — Undead Fortitude's 5 + damage, and
	// concentration's max(10, damage / 2). A static gate ignores it.
	DamageTaken int

	// Roller rolls the save. Nil takes the default roller.
	Roller dice.Roller
}

// ImposedEffect names one thing a contest put on, or would have put on, the
// saver.
//
// Carried in the outcome whichever way the save went, because "the save
// succeeded" and "the save succeeded against *what*" are different facts, and
// only the second one lets a client say "you resisted being knocked prone".
type ImposedEffect struct {
	// Ref identifies the effect — a condition ref today.
	Ref *core.Ref

	// Description is the effect in a sentence, for a log line or a UI.
	Description string
}

// ContestOutcome is what a contested consequence produced.
//
// It records the whole interaction rather than its verdict: the save with the
// full breakdown of which effects folded into it, the DC that was actually
// evaluated, the ability actually used, and what was at stake either way. A
// bare "the save succeeded" would be the answer to a different question than
// the one a player asks after rolling.
type ContestOutcome struct {
	// Save is the requested saving throw, with its folded chain.
	Save SaveOutcome

	// DC is the difficulty class the gate evaluated to for this instance.
	DC int

	// Ability is the one actually rolled. Interesting when the gate offered a
	// choice — see [ContestInput.Gate].
	Ability abilities.Ability

	// Succeeded is whether the saver beat the DC, and therefore whether the
	// consequence was averted.
	Succeeded bool

	// AtStake is what the contest was about, set whichever way it went.
	AtStake ImposedEffect

	// Imposed is what actually landed. Empty on a successful save — and empty
	// is a fact here rather than an absence of one, because AtStake says what
	// did not land.
	Imposed []ImposedEffect
}

func (ContestOutcome) isOutcome() {}

// Consequence is what a failed contest imposes.
//
// Sealed like [Step] and [Outcome], and for the same reason: whatever imposes
// an effect does it on resolution's bus, so the set of things that can be
// imposed is resolution's to know. A caller names one through a constructor
// here.
type Consequence interface {
	// validate reports whether this consequence could actually be imposed, so
	// that a contest refuses one that could not before it rolls anything.
	validate() error

	// atStake describes what the contest is about, before it is known whether
	// it lands. It may assume validate passed.
	atStake() ImposedEffect

	// impose puts the consequence on the saver.
	impose(ctx context.Context, in imposeInput) ([]ImposedEffect, error)

	isConsequence()
}

// imposeInput is what a consequence gets to work with: the interaction's bus,
// the cast that is attached to it, and who failed.
type imposeInput struct {
	bus     events.EventBus
	cast    *Participants
	saverID string
}

// ImposeCondition returns the consequence "the saver gains this condition".
//
// Generic over the condition rather than special-cased per rule: the ref goes
// to conditions.CreateFromRef, so anything the rulebook's factory can build is
// a consequence a gate can name — the wolf's prone today, the ghoul's
// paralysis when its gate arrives, with no case added here for either.
func ImposeCondition(ref *core.Ref, conditionType dnd5eEvents.ConditionType) Consequence {
	return conditionConsequence{ref: ref, conditionType: conditionType}
}

type conditionConsequence struct {
	ref           *core.Ref
	conditionType dnd5eEvents.ConditionType
}

func (conditionConsequence) isConsequence() {}

// validate refuses a consequence naming no condition.
//
// Checked once, at the seam, so that atStake and impose can rely on the ref
// rather than each guarding it. The alternative — a stable "unknown" whenever
// the ref is missing — would turn a caller's mistake into a contest that runs,
// rolls, and imposes something nobody can name, which is the shape of wrongness
// this codebase refuses on principle: failing at construction is a bug report,
// failing soft is a bug that ships.
func (c conditionConsequence) validate() error {
	if c.ref == nil {
		return fmt.Errorf("%w: consequence names no condition ref", ErrNilInput)
	}

	return nil
}

func (c conditionConsequence) atStake() ImposedEffect {
	return ImposedEffect{
		Ref:         c.ref,
		Description: fmt.Sprintf("the %s condition", c.ref.ID),
	}
}

// impose builds the condition for this saver and announces it on the bus.
//
// Announcing rather than applying directly is the point. The sheet's own keeper
// is subscribed to ConditionAppliedEvent; it applies the condition, appends it
// to the sheet, and marks the sheet dirty — so the condition reaches the sheet
// through the same path a condition applied by any other effect takes, and
// comes back in Output.DirtyCharacters without this package knowing how a
// character stores anything.
func (c conditionConsequence) impose(ctx context.Context, in imposeInput) ([]ImposedEffect, error) {
	target, err := in.cast.entity(in.saverID)
	if err != nil {
		return nil, err
	}

	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:         c.ref.String(),
		CharacterID: in.saverID,
	})
	if err != nil {
		return nil, fmt.Errorf("build %s for %q: %w", c.ref.ID, in.saverID, err)
	}

	err = dnd5eEvents.ConditionAppliedTopic.On(in.bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: target,
		Type:   c.conditionType,
		// The nearest existing source: a consequence of being struck rather
		// than of the target's own activation. None of the four sources names
		// a failed save, and adding one belongs in the rulebook rather than
		// here — nothing reads this field today.
		Source:    dnd5eEvents.ConditionSourceDamage,
		Condition: built.Condition,
	})
	if err != nil {
		return nil, fmt.Errorf("apply %s to %q: %w", c.ref.ID, in.saverID, err)
	}

	return []ImposedEffect{c.atStake()}, nil
}

// NewContest returns the machine for a contested consequence: request the save
// the gate declares, then impose the consequence or do not.
//
// Gate-generic, and deliberately not knockdown-generic. Nothing here knows what
// a bite is: it reads which abilities the gate offers, asks the gate for its
// DC, requests the save, and hands the verdict to a consequence that knows how
// to impose itself. The wolf's knockdown is one instantiation, and the monk's
// Flurry — same shape, different ability, different DC — needs no code here.
func NewContest(in *ContestInput) Machine {
	return &contestMachine{in: in}
}

type contestMachine struct {
	in *ContestInput
}

// Start validates the gate, picks the ability, evaluates the DC, and requests
// the save.
func (m *contestMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	if m.in.Gate == nil {
		// A nil gate is not "a consequence nobody can contest" — that would be
		// a consequence imposed without a contest, which is not this machine's
		// job. It is a caller that forgot the declaration.
		return nil, fmt.Errorf("%w: contest has no save gate", ErrNilInput)
	}
	if err := m.in.Gate.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadGate, err)
	}
	if m.in.Consequence == nil {
		return nil, fmt.Errorf("%w: contest has no consequence", ErrNilInput)
	}
	if err := m.in.Consequence.validate(); err != nil {
		return nil, err
	}
	if m.in.Gate.Recurrence != saves.RecurrenceNone {
		// Silently treating a recurring gate as a one-shot would produce a
		// paralysis nobody ever shakes off, and it would look like it worked.
		return nil, fmt.Errorf("%w: %q (lands with the ghoul)",
			ErrRecurrenceUnsupported, m.in.Gate.Recurrence)
	}

	ability, err := m.chooseAbility(cast)
	if err != nil {
		return nil, err
	}

	// Evaluated by the gate, not by this package: the formulas and their
	// rounding are the rulebook's, and a second implementation here is a second
	// thing to get wrong.
	dc := m.in.Gate.DC.DC(saves.DCInput{DamageTaken: m.in.DamageTaken})

	return requestSave(&SaveInput{
		SaverID: m.in.SaverID,
		Ability: ability,
		DC:      dc,
		Cause:   m.in.Cause,
		Roller:  m.in.Roller,
	}, func(ctx context.Context, save SaveOutcome) (Step, error) {
		return m.resolve(ctx, cast, ability, dc, save)
	}), nil
}

// chooseAbility picks which of the gate's abilities the saver rolls.
//
// RAW, a multi-ability gate is the saver's choice, and there is no way to ask
// them yet — asking is a suspension, which is Pose's job. Until then this picks
// the best modifier, which is what a player would choose anyway in the absence
// of a reason not to. **Provisional**: when Pose lands, this becomes the
// default rather than the rule. Ties keep the gate's own order, so the choice
// is deterministic and a registration list stays reproducible.
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

// resolve turns the save's verdict into a consequence, or into an averted one.
func (m *contestMachine) resolve(
	_ context.Context, cast *Participants, ability abilities.Ability, dc int, save SaveOutcome,
) (Step, error) {
	outcome := ContestOutcome{
		Save:      save,
		DC:        dc,
		Ability:   ability,
		Succeeded: save.Result != nil && save.Result.Success,
		AtStake:   m.in.Consequence.atStake(),
	}

	if outcome.Succeeded {
		return Done{Outcome: outcome}, nil
	}

	// Yielded rather than done inline: imposing touches the bus, and a machine
	// never holds one (R6). The closure that does is built here, on
	// resolution's side, exactly as the chain folds are.
	return imposeOnBus(m.in.Consequence, cast, m.in.SaverID, func(imposed []ImposedEffect) Step {
		outcome.Imposed = imposed

		return Done{Outcome: outcome}
	}), nil
}

// imposeOnBus builds the step that puts a consequence on the saver.
//
// A [Gather] rather than a new case in the vocabulary: mechanically it is the
// same thing — a closure resolution runs with the bus, yielding the next step —
// and ADR-0038's set has no case for "do this on the bus" beyond the one that
// already is that. Its Name says what it is, so a reader of the step log sees
// "impose the prone condition" rather than a fold that folds nothing.
func imposeOnBus(
	consequence Consequence, cast *Participants, saverID string, next func([]ImposedEffect) Step,
) Gather {
	return Gather{
		name: "impose " + consequence.atStake().Description,
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			imposed, err := consequence.impose(ctx, imposeInput{bus: bus, cast: cast, saverID: saverID})
			if err != nil {
				return nil, err
			}

			return next(imposed), nil
		},
	}
}

// savingThrowModifier reads one participant's modifier for an ability off the
// sheet resolution loaded.
func savingThrowModifier(cast *Participants, saverID string, ability abilities.Ability) (int, error) {
	if saver, ok := cast.Character(saverID); ok {
		return saver.GetSavingThrowModifier(ability), nil
	}
	if saver, ok := cast.Monster(saverID); ok {
		return saver.GetSavingThrowModifier(ability), nil
	}

	return 0, fmt.Errorf("%w: %q", ErrNoSaver, saverID)
}

// entity returns the loaded sheet for an ID as a core.Entity, which is what a
// ConditionAppliedEvent targets.
func (p *Participants) entity(id string) (core.Entity, error) {
	if character, ok := p.Character(id); ok {
		return character, nil
	}
	if monster, ok := p.Monster(id); ok {
		return monster, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrNoSaver, id)
}

// requestSave builds the Request step for a saving throw, typed so the
// requester resumes with a SaveOutcome rather than switching on Outcome.
func requestSave(in *SaveInput, next func(context.Context, SaveOutcome) (Step, error)) Request {
	return Request{
		name:    "saving throw",
		machine: NewSave(in),
		next: func(ctx context.Context, out Outcome) (Step, error) {
			save, ok := out.(SaveOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: saving throw produced %T", ErrBadStep, out)
			}

			return next(ctx, save)
		},
	}
}
