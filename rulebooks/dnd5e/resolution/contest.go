// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

	// SourceName is the display name of whatever [ContestInput.Cause] names as
	// the effect — "Vicious Mockery". REQUIRED whenever damage is declared, and
	// unread otherwise.
	//
	// Supplied rather than looked up, because the lookup would be a spell table
	// and this package does not have one and must not grow one (ADR-0045). It
	// is the provenance PAIR a roll trace is built from — ref and name — and a
	// trace missing either is refused by the rulebook rather than recorded
	// half-named, so a contest that deals damage without one is refused here
	// instead of producing a record nobody can validate.
	SourceName string

	// Removal is the contest's THIRD consequence: conditions that come off
	// sheets when the save fails, and the owner that ends with them.
	//
	// It is how a check produces a strip rather than an application, and it is
	// the shape a concentration break takes. Nil is the common case and the
	// whole of today's behaviour.
	Removal *ConditionRemoval

	// Move is the contest's FOURTH consequence: the saver is moved against
	// their will, which is the only consequence this package describes rather
	// than delivers.
	//
	// Nil is the common case. The anchor is the caller's to name — the cast
	// door names the caster — because a contest knows who saved and what they
	// saved against, and not who is doing the pushing.
	//
	// It RIDES a contest that also delivers something: a contest whose only
	// consequence is a move is refused with the empty one, because what is at
	// stake in a save would have nothing to name. No content declares one —
	// a cast must declare damage or a condition to exist at all.
	Move *MoveDirective

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

	// ImposedConditionRemoved is a condition the contest took OFF somebody.
	//
	// A kind of its own rather than an application with an empty payload,
	// because "this landed" and "this came off" are opposite facts and a reader
	// that had to infer which from a nil field would be inferring a rule.
	ImposedConditionRemoved ImposedEffectKind = "condition-removed"

	// ImposedMove is a move the contest forced on the saver: a shove, a slide,
	// a creature sent running.
	//
	// It is the consequence that is not finished when this package is done
	// with it. A condition is APPLIED here and damage is DEALT here, but a move
	// is only described — the cells it crosses are read from the map under its
	// own fold, which is encounter's, so what leaves here is a directive and
	// whoever holds the board walks it (rpg-project#431 §2).
	ImposedMove ImposedEffectKind = "move"
)

// MoveDirective is a move DESCRIBED: how the creature is moved, what it is
// measured from, how far, and what being moved costs them.
//
// NOTHING HERE IS GEOMETRY. No cells, no directions, no map. It mirrors
// [combatActions.CastMove], which is how content declares the same thing, plus
// the anchor — the creature the policy is measured from, which content cannot
// name because it does not know who cast it.
//
// The division is the point: a spell holding a path search of its own is what
// this shape exists to prevent, and a rules layer that worked out which cells a
// push crosses would be a second author of passability the day it disagreed
// with the fold.
type MoveDirective struct {
	// Policy is how the mover is moved. One value exists and it is the one
	// something can walk (see [combatActions.MovePolicy]).
	Policy combatActions.MovePolicy

	// AnchorID is the creature the policy is measured from — the caster, for
	// everything that pushes today.
	//
	// AN ID RATHER THAN A POSITION, because a position read here would be read
	// at the moment the save resolved and walked at the moment the push runs,
	// and the two are not the same moment. Whoever executes the move asks the
	// board where the anchor is.
	AnchorID string

	// Cells is a fixed budget, and Speed says the budget is the mover's own
	// speed instead. Exactly one of the two, as the declaration requires.
	Cells int
	Speed bool

	// Pays is what being moved costs the creature that is moved. Zero is
	// nothing, which is the push.
	Pays combatActions.MovePays

	// Provokes says whether the move offers opportunity attacks as it goes.
	// Zero is false: being thrown is not walking out of a reach.
	Provokes bool
}

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

	// RecipientID is who it landed on. Always the saver for a contest, and for
	// a cast either party — True Strike's condition goes on the CASTER — which
	// is why it travels with the effect rather than being inferred by whoever
	// reads it.
	RecipientID string

	// Address is the exact source-qualified identity for condition consequences.
	Address dnd5eEvents.ConditionAddress

	// Move is the directive for an [ImposedMove], and nil for every other
	// kind. It is the whole of what this package says about the move: the
	// cells are worked out by whoever owns the board.
	Move *MoveDirective

	// Amount is the damage the SHEET applied. Zero on a condition, and zero on
	// the AT-STAKE effect of a contest whose dice have not been rolled yet.
	Amount int

	// Requested is what the dice and the multipliers settled on, before the
	// sheet had a say. It equals [ImposedEffect.Calculation]'s total by
	// construction, which is the rule a record validates against.
	//
	// It is Amount's sibling for the same reason healing has one: the two
	// separate the moment somebody's hit points stop moving, and a record that
	// carried only the applied number could not show a hit that was bigger than
	// the target had left.
	Requested int

	// Before and After are the recipient's hit points either side of the
	// application, as the sheet reported them.
	Before int
	After  int

	// Components are the TYPED breakdown — how much of which damage type, with
	// the pool's declared properties. Calculation cannot say this: a roll
	// component carries faces and modifiers, not a damage type.
	Components []dnd5eEvents.DamageComponent

	// Calculation is the roll's authoritative arithmetic — every component's
	// dice trace and modifier, and the total they come to. It is the
	// representation a record replays, and its total is [ImposedEffect.Requested].
	Calculation *dnd5eEvents.RollCalculation
}

// ContestOutcome records the requested save and what its failure delivered.
//
// AtStake is what the save was against: the condition when one was declared,
// and otherwise the declared damage. Imposed is what a failed save actually
// delivered, damage first, then the condition, then the move, and it is empty
// on a success — a made save negates them all.
type ContestOutcome struct {
	Save      SaveOutcome
	DC        int
	Ability   abilities.Ability
	Succeeded bool
	AtStake   ImposedEffect
	Imposed   []ImposedEffect

	// FollowUps are the checks this contest's own damage came back with, in
	// append order. Empty is the common case: only damage that landed on
	// somebody holding an ongoing rule produces one.
	FollowUps []FollowUpOutcome
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
	if application.Ref.Equals(refs.Conditions.Baned()) {
		var config struct {
			SourceID string `json:"source_id"`
		}
		if err := json.Unmarshal(application.Parameters, &config); err != nil {
			return preparedCondition{}, fmt.Errorf("build condition baned for %q: %w", targetID, err)
		}
		source, err := core.ParseString(sourceRef)
		if err != nil {
			return preparedCondition{}, fmt.Errorf("build condition baned for %q: source: %w", targetID, err)
		}
		condition, err := conditions.NewBanedCondition(conditions.NewBanedConditionInput{
			MemberID: targetID, SourceID: config.SourceID, SourceRef: source,
		})
		if err != nil {
			return preparedCondition{}, fmt.Errorf("build condition baned for %q: %w", targetID, err)
		}
		return preparedCondition{declaration: application.Clone(), behavior: condition}, nil
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

func (p preparedCondition) atStake(recipientID string) ImposedEffect {
	ref := p.declaration.Ref
	return ImposedEffect{
		Kind:        ImposedCondition,
		Ref:         &ref,
		Description: conditionDescription(ref),
		RecipientID: recipientID,
		Address:     conditions.ConditionAddressOf(recipientID, p.behavior),
	}
}

// conditionDescription is how a delivered condition reads in a step log and in
// a record. One spelling, so the contest's imposition and a cast's delivery
// describe the same condition the same way.
func conditionDescription(ref core.Ref) string {
	return fmt.Sprintf("the %s condition", ref.ID)
}

func publishPreparedCondition(
	prepared preparedCondition, cast *Participants, targetID string,
	source dnd5eEvents.ConditionSource, next func() (Step, error),
) Gather {
	return Gather{
		name: "impose " + conditionDescription(prepared.declaration.Ref),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			target, err := cast.entity(targetID)
			if err != nil {
				return nil, err
			}
			if err := publishCondition(ctx, bus, prepared, target, source); err != nil {
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
// The source is the caller's to state because the callers mean different things
// by it, and none may guess for another.
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
	pools []damage.Damage, roller dice.Roller, cause dnd5eEvents.SaveCause, sourceName string,
	cast *Participants, targetID string, next func(ImposedEffect) (Step, error),
) Gather {
	return Gather{
		name: "deal " + describeDamage(pools),
		run: func(ctx context.Context, _ events.EventBus) (Step, error) {
			target, err := combatantFor(cast, targetID)
			if err != nil {
				return nil, err
			}
			components, err := rollContestDamage(ctx, pools, roller, cause, sourceName)
			if err != nil {
				return nil, err
			}

			calculation, err := damageCalculation(components)
			if err != nil {
				return nil, err
			}

			final, total := combat.FinalDamage(components)
			if total != calculation.Total {
				// The trace no longer explains the number. It cannot happen
				// while nothing folds this damage — every component here came
				// from a declared pool and none carries a multiplier — and if
				// something ever does, a record built from this would show
				// faces that do not add up to the damage the player took.
				return nil, fmt.Errorf(
					"%w: %s dealt %d, and its roll trace explains %d",
					ErrBadAction, describeDamage(pools), total, calculation.Total)
			}

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
				RecipientID: targetID,
				Amount:      applied.TotalDamage,
				Requested:   calculation.Total,
				Before:      applied.PreviousHP,
				After:       applied.CurrentHP,
				Components:  cloneDamageComponents(components),
				Calculation: calculation,
			})
		},
	}
}

// validateMove refuses a directive this stack cannot execute, at the door,
// before the save is rolled or the price is charged.
//
// The policy, the budget and the price are CONTENT's rules, so they are checked
// by content's own validator rather than restated here — one owner for what a
// move may declare, and no second copy to drift. What this layer adds is the
// anchor, which content never names.
//
// # Two legal declarations are refused here and not there, on purpose
//
// A move paid for with a reaction, and a budget that is the mover's own speed.
// Both validate in content because both are real, and neither has an executor
// anywhere in this stack today: nothing spends the reaction and nothing turns a
// speed into cells. Describing one would hand the board a directive it would
// walk for free, or for a distance nobody worked out — an affordance with
// nothing behind it, and silently wrong rather than loudly.
//
// Dissonant Whispers brings both, with the spend and the lookup (rpg-project#431
// §0). It deletes these two arms; it does not work around them.
func validateMove(directive *MoveDirective) error {
	declared := combatActions.CastMove{
		Policy: directive.Policy, Cells: directive.Cells, Speed: directive.Speed,
		Pays: directive.Pays, Provokes: directive.Provokes,
	}
	if err := declared.Validate(); err != nil {
		return fmt.Errorf("%w: contest move: %w", ErrBadAction, err)
	}
	if directive.AnchorID == "" {
		return fmt.Errorf("%w: a move must name the anchor it is measured from", ErrBadAction)
	}
	if directive.Pays != combatActions.PaysNothing {
		return fmt.Errorf("%w: a move priced at %q cannot be imposed here, because nothing in this "+
			"stack spends a reaction for one yet", ErrBadAction, directive.Pays)
	}
	if directive.Speed {
		return fmt.Errorf(
			"%w: a move budgeted by the mover's own speed cannot be imposed here, because nothing "+
				"in this stack reads a speed into cells yet", ErrBadAction)
	}

	return nil
}

// describeMove names the directive the way a step log should read it: "a line
// move of 2 cells".
func describeMove(directive MoveDirective) string {
	budget := fmt.Sprintf("%d cells", directive.Cells)
	if directive.Speed {
		budget = "their own speed"
	}

	return fmt.Sprintf("a %s move of %s", directive.Policy, budget)
}

// imposeMove is applyPreparedDamage's other sibling: the third thing a failed
// save can cost, in the same shape, chained through the same continuation.
//
// IT MOVES NOBODY. What it produces is the directive, handed on as an
// [ImposedMove] for whoever owns the board to route and walk. There is no
// publish and no write here, which is why it is the one consequence that is not
// finished when this package is: a rule that worked out the cells would be
// reading a map it does not own, and would disagree with the fold the day the
// two were written by different hands.
//
// # The dropped are not pushed
//
// Read here rather than decided in resolve, because resolve builds the whole
// chain before the damage step has run: the hit points that matter are the ones
// the saver has at the moment the push would happen, and this is that moment.
// A creature the damage dropped stays where it fell (rpg-project#432 §5), and
// it is `stayed` that says so — the effect is never produced, rather than
// produced and filtered by somebody downstream.
func imposeMove(
	directive MoveDirective, cause dnd5eEvents.SaveCause, cast *Participants, targetID string,
	pushed func(ImposedEffect) (Step, error), stayed func() (Step, error),
) Gather {
	return Gather{
		name: "impose " + describeMove(directive),
		run: func(_ context.Context, _ events.EventBus) (Step, error) {
			target, err := combatantFor(cast, targetID)
			if err != nil {
				return nil, err
			}
			if combat.IsDown(target) {
				return stayed()
			}

			moved := directive
			return pushed(ImposedEffect{
				Kind:        ImposedMove,
				Ref:         cloneCoreRef(cause.EffectRef),
				Description: describeMove(moved),
				RecipientID: targetID,
				Move:        &moved,
			})
		},
	}
}

// damageCalculation is the roll behind the damage, in the one shape a record
// replays: every component's own trace, and the total they come to.
//
// It is BUILT from the components rather than accumulated alongside them, so
// the total and the faces cannot drift apart — and it is validated here, at the
// only place that can still refuse, because the rulebook's validator is what a
// record will run and failing it there would be a beat nobody can write.
func damageCalculation(
	components []dnd5eEvents.DamageComponent,
) (*dnd5eEvents.RollCalculation, error) {
	calculation := &dnd5eEvents.RollCalculation{
		Components: make([]dnd5eEvents.RollComponent, 0, len(components)),
	}
	for _, component := range components {
		calculation.Components = append(calculation.Components, component.Roll)
		if component.Roll.Dice != nil {
			calculation.Total += component.Roll.Dice.Subtotal
		}
		if component.Roll.Modifier != nil {
			calculation.Total += *component.Roll.Modifier
		}
	}

	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return nil, fmt.Errorf("%w: damage roll trace: %w", ErrBadAction, err)
	}

	// Owned from here on: the components it was built from are the machine's,
	// and an outcome handed outward must not alias them.
	return dnd5eEvents.CloneRollCalculation(calculation), nil
}

// rollContestDamage rolls each declared pool into the component a record can
// read the faces off.
//
// Provenance is the save's cause: the effect ref that raised the save is what
// dealt the damage, and a cause that named none leaves the source ref nil
// rather than borrowing the condition's.
func rollContestDamage(
	ctx context.Context, pools []damage.Damage, roller dice.Roller,
	cause dnd5eEvents.SaveCause, sourceName string,
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
				Source: dnd5eEvents.RollSource{Ref: cloneCoreRef(cause.EffectRef), Name: sourceName},
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

// conditionSourceFor answers WHAT SORT OF THING imposed a contested condition,
// read off the cause the caller already stated rather than from a second field
// that could disagree with it.
//
// A cast says so — its cause carries [dnd5eEvents.SaveTriggerSpell] and the
// spell's own ref — and everything else keeps the answer this package has
// always given. Damage is the honest default for the rest: the contest exists
// because something landed on the saver, and the wolf's knockdown is exactly
// that. The condition still names WHICH spell or weapon separately, through the
// source ref it was built with.
func conditionSourceFor(cause dnd5eEvents.SaveCause) dnd5eEvents.ConditionSource {
	if cause.Trigger == dnd5eEvents.SaveTriggerSpell {
		return dnd5eEvents.ConditionSourceSpell
	}

	return dnd5eEvents.ConditionSourceDamage
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

func (m *contestMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	m.cast = cast
	if err := validateConditionGate(m.in.Gate); err != nil {
		return nil, err
	}

	m.hasCondition = m.in.prepared != nil || m.in.Application.Ref != (core.Ref{})
	if !m.hasCondition && len(m.in.Damage) == 0 && m.in.Removal == nil {
		// Fail closed: a contest that would deliver nothing is a save the
		// player is asked to roll for no reason, and it would look like it
		// worked.
		//
		// A removal counts, which is the same widening slice two made for
		// damage: Application may be nil when the contest declares one of the
		// other two consequences.
		return nil, fmt.Errorf(
			"%w: a contest must declare a condition, damage, or a removal", ErrBadAction)
	}
	if m.in.Removal != nil {
		if err := validateRemoval(m.in.Removal); err != nil {
			return nil, err
		}
	}
	if m.in.Move != nil {
		if err := validateMove(m.in.Move); err != nil {
			return nil, err
		}
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
		// A roll trace is a provenance PAIR, and the rulebook refuses a half-
		// named one. Refusing here means a contest that could not be recorded
		// never rolls, rather than dealing damage and failing to write it down.
		if m.in.Cause.EffectRef == nil || strings.TrimSpace(m.in.SourceName) == "" {
			return nil, fmt.Errorf(
				"%w: contest damage needs the ref and name of what dealt it", ErrBadAction)
		}
	}

	ability, err := m.chooseAbility(cast)
	if err != nil {
		return nil, err
	}
	dc := m.in.Gate.DC.DC(saves.DCInput{DamageTaken: m.in.DamageTaken})
	d20Source, err := m.saveSource()
	if err != nil {
		return nil, err
	}
	if m.in.Roller == nil {
		return nil, fmt.Errorf("%w: a contest rolls with no roller", ErrNoRoller)
	}
	return requestSave(&SaveInput{
		SaverID:   m.in.SaverID,
		Ability:   ability,
		DC:        dc,
		Cause:     m.in.Cause,
		D20Source: d20Source,
		Roller:    m.in.Roller,
	}, func(_ context.Context, save SaveOutcome) (Step, error) {
		return m.resolve(ability, dc, save)
	}), nil
}

func (m *contestMachine) saveSource() (dnd5eEvents.RollSource, error) {
	ref := cloneCoreRef(m.in.Cause.EffectRef)
	name := strings.TrimSpace(m.in.SourceName)
	if ref == nil && m.hasCondition {
		conditionRef := m.prepared.declaration.Ref
		ref = cloneCoreRef(&conditionRef)
	}
	if ref == nil && m.in.Removal != nil {
		parsed, err := core.ParseString(m.in.Removal.Owner.ConditionRef)
		if err != nil {
			return dnd5eEvents.RollSource{}, fmt.Errorf("%w: save source: %v", ErrBadAction, err)
		}
		ref = parsed
	}
	if ref == nil {
		return dnd5eEvents.RollSource{}, fmt.Errorf("%w: contest save needs a source ref", ErrBadAction)
	}
	if name == "" {
		name = ref.ID
	}
	return dnd5eEvents.RollSource{Ref: ref, Name: name, SourceID: m.in.Cause.InstigatorID}, nil
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
		return m.prepared.atStake(m.in.SaverID)
	}
	if len(m.in.Damage) == 0 && m.in.Removal != nil {
		// A removal-only contest is a check to KEEP something, so what is at
		// stake is the owner the failure would end.
		return removalEffect(m.in.Removal.Owner, m.in.Removal.Reason)
	}

	return ImposedEffect{
		Kind:        ImposedDamage,
		Ref:         cloneCoreRef(m.in.Cause.EffectRef),
		Description: describeDamage(m.in.Damage),
		RecipientID: m.in.SaverID,
	}
}

// resolve turns the save into the outcome, and on a failure chains whatever was
// declared: damage first, then the condition, then the move.
//
// THE MOVE IS LAST, and that is a rule rather than an ordering accident. Damage
// before the push is ruled by the design (rpg-project#432 §5) — what the damage
// drops is not pushed, and a body sliding across the floor is not a story we
// tell. The condition before the push follows from the same reading: whatever
// the failure put on the creature is on it before it is moved, so a rule that
// has something to say about being moved has already been applied when the
// board comes to walk it.
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

	done := func() (Step, error) { return Done{Outcome: outcome}, nil }

	deliverMove := func() (Step, error) {
		if m.in.Move == nil {
			return done()
		}

		return imposeMove(*m.in.Move, m.in.Cause, m.cast, m.in.SaverID,
			func(directed ImposedEffect) (Step, error) {
				outcome.Imposed = append(outcome.Imposed, directed)
				return done()
			}, done), nil
	}

	deliverRemoval := func() (Step, error) {
		if m.in.Removal == nil {
			return deliverMove()
		}

		return publishRemoval(m.in.Removal, func(stripped ImposedEffect) (Step, error) {
			outcome.Imposed = append(outcome.Imposed, stripped)
			return deliverMove()
		}), nil
	}

	deliverCondition := func() (Step, error) {
		if !m.hasCondition {
			return deliverRemoval()
		}

		return publishPreparedCondition(
			m.prepared, m.cast, m.in.SaverID, conditionSourceFor(m.in.Cause),
			func() (Step, error) {
				outcome.Imposed = append(outcome.Imposed, m.prepared.atStake(m.in.SaverID))
				return deliverRemoval()
			},
		), nil
	}

	if len(m.in.Damage) == 0 {
		return deliverCondition()
	}

	return applyPreparedDamage(
		m.in.Damage, m.in.Roller, m.in.Cause, m.in.SourceName, m.cast, m.in.SaverID,
		func(applied ImposedEffect) (Step, error) {
			outcome.Imposed = append(outcome.Imposed, applied)

			// Say what landed, then answer what came back — the same two steps
			// the strike calls, in the same order, right after the apply. A
			// third damage source gets concentration by calling them too, which
			// is the whole reason they are named once.
			return reportDamage(reportDamageInput{
				MemberID:      m.in.SaverID,
				Amount:        applied.Amount,
				DamageType:    primaryComponentType(applied.Components),
				DroppedToZero: applied.Before > 0 && applied.After == 0,
				Cause:         m.in.Cause,
			}, func(ctx context.Context, ups []dnd5eEvents.FollowUp) (Step, error) {
				return runFollowUps(ctx, ups, 0, m.in.Roller,
					func(followUp FollowUpOutcome) {
						outcome.FollowUps = append(outcome.FollowUps, followUp)
					},
					func(context.Context) (Step, error) { return deliverCondition() },
				)
			}), nil
		},
	), nil
}

// validateRemoval refuses a removal that names nothing to end.
//
// Fail closed: a contest whose failure strips an empty owner address would ask
// somebody to roll and then quietly do nothing, which is exactly the
// affordance-with-nothing-behind-it this stack keeps finding.
func validateRemoval(removal *ConditionRemoval) error {
	if removal.Owner.MemberID == "" || removal.Owner.ConditionRef == "" {
		return fmt.Errorf("%w: a contest removal must name the owner it ends", ErrBadAction)
	}
	if strings.TrimSpace(removal.Reason) == "" {
		return fmt.Errorf("%w: a contest removal must say why", ErrBadAction)
	}
	return nil
}

// removalEffect is one stripped address, as the record reads it.
func removalEffect(address dnd5eEvents.ChildRef, reason string) ImposedEffect {
	effect := ImposedEffect{
		Kind:        ImposedConditionRemoved,
		Description: fmt.Sprintf("%s ended (%s)", address.ConditionRef, reason),
		RecipientID: address.MemberID,
		Address:     address,
	}
	if ref, err := core.ParseString(address.ConditionRef); err == nil {
		effect.Ref = ref
	}

	return effect
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
