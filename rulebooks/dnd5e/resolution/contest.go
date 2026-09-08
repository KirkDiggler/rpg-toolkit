// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ContestInput describes what somebody may save against: a condition, damage,
// or both. At least one of the two must be declared.
type ContestInput struct {
	Gate        *saves.SaveGate
	SaverID     string
	Application combatActions.ConditionApplication

	// Damage is what a failed save costs the saver, declared the same way an
	// attack declares its pools. It is the contest's SECOND consequence rather
	// than a second machine: the sequence is already Request(save) -> outcome
	// policy -> deliver -> Done, and damage is a delivery this step could not
	// express. Empty is the common case and the whole of today's behaviour.
	//
	// A zero-value ContestInput therefore still says what it always said: a
	// contest with no damage declared deals none.
	Damage []damage.Damage

	Cause       dnd5eEvents.SaveCause
	DamageTaken int
	Roller      dice.Roller
	prepared    *preparedCondition
}

// ImposedEffectKind names WHICH consequence one [ImposedEffect] records.
//
// It is stated rather than inferred from which fields are populated: a
// condition with a nil ref and damage that landed for zero are both legal, and
// neither reading of the other fields could tell them apart.
type ImposedEffectKind string

const (
	// ImposedCondition is a condition the contest put on the saver.
	ImposedCondition ImposedEffectKind = "condition"

	// ImposedDamage is damage the contest landed on the saver.
	ImposedDamage ImposedEffectKind = "damage"
)

// ImposedEffect names one consequence a contest delivered, or would have
// delivered, to the saver.
//
// Kind says which of the two it is. Ref and Description are filled for both:
// a condition names the condition, damage names whatever the save's cause named
// as the thing that dealt it. Amount and Components carry the damage's
// arithmetic and are zero and nil on a condition, which is the honest reading —
// a condition has no amount to report.
type ImposedEffect struct {
	Kind        ImposedEffectKind
	Ref         *core.Ref
	Description string

	// Amount is the damage that actually landed, after the saver's own
	// resistances. Zero on a condition, and zero on the AT-STAKE effect of a
	// contest whose dice have not been rolled yet.
	Amount int

	// Components are the damage's roll facts, so a record can show the faces
	// that produced Amount rather than only the total.
	Components []dnd5eEvents.DamageComponent
}

// ContestOutcome records the requested save and what its failure delivered.
//
// AtStake is what the save was against: the condition when one was declared,
// and otherwise the declared damage. Imposed is what a failed save actually
// delivered, damage first and the condition second, and it is empty on a
// success — a made save negates both.
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
		Ref:       application.Ref.String(),
		Config:    application.Parameters,
		MemberID:  targetID,
		SourceRef: sourceRef,
	})
	if err != nil {
		return preparedCondition{}, fmt.Errorf("build condition %s for %q: %w", application.Ref.ID, targetID, err)
	}
	return preparedCondition{declaration: application.Clone(), behavior: built.Condition}, nil
}

func (p preparedCondition) atStake() ImposedEffect {
	ref := p.declaration.Ref
	return ImposedEffect{
		Kind:        ImposedCondition,
		Ref:         &ref,
		Description: fmt.Sprintf("the %s condition", ref.ID),
	}
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
			if err := publishCondition(
				ctx, bus, prepared, target, dnd5eEvents.ConditionSourceDamage,
			); err != nil {
				return nil, err
			}
			return next()
		},
	}
}

// publishCondition is the one place a built condition reaches the bus.
//
// The publish is the whole application: an effect never attaches itself, it
// publishes and the owning keeper applies. Two callers share this — the
// contest's imposition and the gateless cast's delivery — so there is one
// implementation of what "the condition landed" means rather than a copy per
// door.
//
// The source is the caller's to state because the two mean different things by
// it, and neither may guess for the other.
func publishCondition(
	ctx context.Context, bus events.EventBus, prepared preparedCondition,
	target core.Entity, source dnd5eEvents.ConditionSource,
) error {
	err := dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    target,
		Type:      dnd5eEvents.ConditionType(prepared.declaration.Ref.ID),
		Source:    source,
		Condition: prepared.behavior,
	})
	if err != nil {
		return fmt.Errorf("apply %s to %q: %w", prepared.declaration.Ref.ID, target.GetID(), err)
	}

	return nil
}

// describeDamage names the declared pools the way a step log should read them:
// "1d4 psychic damage", and "1d4 psychic and 1d6 fire damage" for two.
func describeDamage(pools []damage.Damage) string {
	if len(pools) == 0 {
		return "no damage"
	}
	parts := make([]string, 0, len(pools))
	for _, pool := range pools {
		parts = append(parts, fmt.Sprintf("%s %s", pool.Dice, pool.Type))
	}

	return strings.Join(parts, " and ") + " damage"
}

// applyPreparedDamage is publishPreparedCondition's sibling: the other thing a
// failed save can deliver, in the same shape, chained through the same next().
//
// Bus-free on the write, exactly as a strike's damage phase is. The dice are
// rolled HERE rather than in Start, so a contest refused at the door — an
// invalid gate, an unpayable price — rolls nothing.
//
// # No damage-chain fold, and it is named rather than hidden
//
// A strike folds DamageChainEvent before it applies, which is how a condition
// adds to or resists the number. This does not: the fold's input is an
// attacker, a weapon, an ability and a criticality, and a contest has none of
// them — the machine knows only who saved and what the save was against.
// combat.FinalDamage still runs, so a multiplier that reaches these components
// applies; what is missing is the bus round that could put one there. The cost
// is real and bounded: a target resistant to a spell's damage type takes full
// damage this slice. It is bought back when a cast carries a caster the fold
// can name, not by inventing one here.
func applyPreparedDamage(
	pools []damage.Damage, roller dice.Roller, cause dnd5eEvents.SaveCause,
	cast *Participants, targetID string, next func(ImposedEffect) (Step, error),
) Gather {
	return Gather{
		name: "deal " + describeDamage(pools),
		run: func(ctx context.Context, _ events.EventBus) (Step, error) {
			target, err := combatantFor(cast, targetID)
			if err != nil {
				return nil, err
			}
			components, err := rollContestDamage(ctx, pools, roller, cause)
			if err != nil {
				return nil, err
			}

			final, _ := combat.FinalDamage(components)
			instances := make([]combat.DamageInstance, 0, len(final))
			for _, instance := range final {
				instances = append(instances, combat.DamageInstance{
					Amount: instance.Amount,
					Type:   string(instance.Type),
				})
			}

			// The sheet's own business, and the same call a strike makes, so a
			// cantrip that drops somebody to zero flows through the death-save
			// and downed transitions Character.ApplyDamage already owns.
			applied := target.ApplyDamage(ctx, &combat.ApplyDamageInput{Instances: instances})

			return next(ImposedEffect{
				Kind:        ImposedDamage,
				Ref:         cloneCoreRef(cause.EffectRef),
				Description: describeDamage(pools),
				Amount:      applied.TotalDamage,
				Components:  cloneDamageComponents(components),
			})
		},
	}
}

// rollContestDamage rolls each declared pool into the component a record can
// read the faces off.
//
// Provenance is the save's cause: the effect ref that raised the save is what
// dealt the damage, and a cause that named none leaves the source ref nil
// rather than borrowing the condition's.
func rollContestDamage(
	ctx context.Context, pools []damage.Damage, roller dice.Roller, cause dnd5eEvents.SaveCause,
) ([]dnd5eEvents.DamageComponent, error) {
	components := make([]dnd5eEvents.DamageComponent, 0, len(pools))
	for _, declared := range pools {
		pool, err := dice.ParseNotation(declared.Dice)
		if err != nil {
			return nil, fmt.Errorf("%w: damage dice %q: %w", ErrBadAction, declared.Dice, err)
		}
		result := pool.RollContext(ctx, roller)
		if result.Error() != nil {
			return nil, fmt.Errorf("roll damage: %w", result.Error())
		}
		dieSize, err := parseDieSize(declared.Dice)
		if err != nil {
			return nil, fmt.Errorf("%w: damage dice %q: %w", ErrBadAction, declared.Dice, err)
		}

		// Per-die, not summed, for the same reason a strike records them that
		// way: a downstream reroll addresses dice by index.
		rolls := flattenDice(result.Rolls())
		subtotal := 0
		for _, face := range rolls {
			subtotal += face
		}

		component := dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceSpell,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: cloneCoreRef(cause.EffectRef)},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      dice.SimplePool(len(rolls), dieSize, 0).Notation(),
					DieSize:       dieSize,
					OriginalRolls: append([]int(nil), rolls...),
					FinalRolls:    append([]int(nil), rolls...),
					Subtotal:      subtotal,
				},
			},
			DamageType: declared.Type,
			Properties: append([]damage.Property(nil), declared.Properties...),
		}
		if declared.FlatBonus != 0 {
			bonus := declared.FlatBonus
			component.Roll.Modifier = &bonus
		}
		components = append(components, component)
	}

	return components, nil
}

// NewContest returns the machine for one save-gated declaration: a condition,
// damage, or both.
func NewContest(in *ContestInput) Machine { return &contestMachine{in: in} }

type contestMachine struct {
	in       *ContestInput
	cast     *Participants
	prepared preparedCondition

	// hasCondition records whether a condition was declared at all. A contest
	// may be damage-only, and "the zero ConditionApplication" is not something
	// a later step should have to re-derive.
	hasCondition bool
}

// rollerOrDefault is the machine's own roller, or the package default when the
// caller named none — the same seam a strike keeps for the same reason.
func (m *contestMachine) rollerOrDefault() dice.Roller {
	if m.in.Roller != nil {
		return m.in.Roller
	}

	return dice.NewRoller()
}

func (m *contestMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	m.cast = cast
	if err := validateConditionGate(m.in.Gate); err != nil {
		return nil, err
	}

	m.hasCondition = m.in.prepared != nil || m.in.Application.Ref != (core.Ref{})
	if !m.hasCondition && len(m.in.Damage) == 0 {
		// Fail closed: a contest that would deliver nothing is a save the
		// player is asked to roll for no reason, and it would look like it
		// worked.
		return nil, fmt.Errorf("%w: a contest must declare a condition, damage, or both", ErrBadAction)
	}

	if m.in.prepared != nil {
		m.prepared = *m.in.prepared
	} else if m.hasCondition {
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

	if len(m.in.Damage) > 0 {
		// Preflight, so a malformed pool is refused before the door charges
		// anybody rather than mid-delivery with the save already rolled.
		if err := damage.Validate(m.in.Damage); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
		}
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

// atStake names what this save is against, which is the condition whenever one
// was declared and the damage otherwise.
func (m *contestMachine) atStake() ImposedEffect {
	if m.hasCondition {
		return m.prepared.atStake()
	}

	return ImposedEffect{
		Kind:        ImposedDamage,
		Ref:         cloneCoreRef(m.in.Cause.EffectRef),
		Description: describeDamage(m.in.Damage),
	}
}

// resolve turns the save into the outcome, and on a failure chains whatever was
// declared: damage first, then the condition.
//
// THE SUCCESS BRANCH IS UNTOUCHED. Done is returned before any delivery step
// exists, so a made save negates the damage and the rider together — which is
// also what the gate's Negated-only policy already promised.
func (m *contestMachine) resolve(ability abilities.Ability, dc int, save SaveOutcome) (Step, error) {
	outcome := ContestOutcome{
		Save:      save,
		DC:        dc,
		Ability:   ability,
		Succeeded: save.Result != nil && save.Result.Success,
		AtStake:   m.atStake(),
	}
	if outcome.Succeeded {
		return Done{Outcome: outcome}, nil
	}

	deliverCondition := func() (Step, error) {
		if !m.hasCondition {
			return Done{Outcome: outcome}, nil
		}

		return publishPreparedCondition(m.prepared, m.cast, m.in.SaverID, func() (Step, error) {
			outcome.Imposed = append(outcome.Imposed, m.prepared.atStake())
			return Done{Outcome: outcome}, nil
		}), nil
	}

	if len(m.in.Damage) == 0 {
		return deliverCondition()
	}

	return applyPreparedDamage(
		m.in.Damage, m.rollerOrDefault(), m.in.Cause, m.cast, m.in.SaverID,
		func(applied ImposedEffect) (Step, error) {
			outcome.Imposed = append(outcome.Imposed, applied)
			return deliverCondition()
		},
	), nil
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
