package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// ActionInput identifies a shared action definition and the participants it targets.
//
// AttackerID is whoever acts — the swinger of a strike, the CASTER of a cast.
// The name is the strike's and stays the strike's: renaming it is a mechanical
// change across every call site and it should ride a third profile arm rather
// than this one.
type ActionInput struct {
	Definition combatActions.Definition
	AttackerID string

	// TargetID is the legacy single-target spelling.
	// Deprecated: use TargetIDs; put a single target in a one-element slice.
	TargetID string

	// TargetIDs is the canonical ordered target list for every profile arm.
	TargetIDs []string
	Roller    dice.Roller
}

// NewAction validates an inert definition and dispatches by populated profile arm.
//
// # The word "cast" does not reach a machine
//
// This is the single place content chooses a sequence, and the cast arm adds no
// machine to choose from. A definition whose cast profile carries a gate is a
// [NewContest] — a save, and what a failed one delivers. One without a gate has
// nothing to resolve, so it is a DELIVERY: the conditions are built here and
// published on the interaction's bus by the activation machine's arm, which is
// the collector every activation result already travels through.
//
// Nothing below this function names a spell, a cantrip or a school. It sees a
// profile with a gate or without one (ADR-0045).
func NewAction(in *ActionInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if err := in.Definition.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
	}

	targetIDs, err := normalizeActionTargets(in.Definition.Ref, in.TargetID, in.TargetIDs)
	if err != nil {
		return nil, err
	}
	if in.Definition.Attack != nil {
		if len(targetIDs) != 1 {
			return nil, fmt.Errorf("%w: %s attack requires exactly one target; got %d",
				ErrBadAction, in.Definition.Ref.String(), len(targetIDs))
		}
		if targetIDs[0] == "" {
			return nil, fmt.Errorf("%w: %s attack target 0 is empty",
				ErrBadAction, in.Definition.Ref.String())
		}
		return NewStrike(&StrikeInput{
			AttackerID: in.AttackerID,
			TargetID:   targetIDs[0],
			Definition: in.Definition.Clone(),
			Roller:     in.Roller,
		}), nil
	}
	if in.Definition.Cast != nil {
		return newCast(in, targetIDs)
	}
	return nil, fmt.Errorf("%w: definition %q has no supported profile", ErrBadAction, in.Definition.Ref.String())
}

// normalizeActionTargets resolves the deprecated scalar at the public door.
// Its result never shares caller-owned slice storage, so a running machine is
// insulated from later edits to the input and normalization never edits it.
func normalizeActionTargets(ref core.Ref, targetID string, targetIDs []string) ([]string, error) {
	if targetID != "" && len(targetIDs) != 0 {
		return nil, fmt.Errorf("%w: %s received both TargetID and TargetIDs",
			ErrBadAction, ref.String())
	}
	if targetID != "" {
		return []string{targetID}, nil
	}

	return append([]string(nil), targetIDs...), nil
}

// newCast reads a cast profile and returns the machine that already exists for
// the shape it declares.
//
// The definition is CLONED first and everything below reads the clone: the
// save cause holds a pointer to its ref, and a caller reusing its definition
// must not be able to rewrite what a running interaction says caused the save.
func newCast(in *ActionInput, normalizedTargetIDs []string) (Machine, error) {
	definition := in.Definition.Clone()
	profile := definition.Cast
	casterID := in.AttackerID
	targetIDs := append([]string(nil), normalizedTargetIDs...)

	if casterID == "" {
		return nil, fmt.Errorf("%w: %s was cast by nobody", ErrBadAction, definition.Ref.String())
	}
	if err := checkCastTargets(profile, definition.Ref, targetIDs); err != nil {
		return nil, err
	}

	cause := dnd5eEvents.SaveCause{
		Trigger:      dnd5eEvents.SaveTriggerSpell,
		EffectRef:    &definition.Ref,
		InstigatorID: casterID,
	}

	// A self-targeted cast resolves against ONE recipient and that recipient is
	// the caster, so the loop below runs once over the caster's own id.
	//
	// The caster rather than an empty sentinel, because the list this produces
	// becomes [CastOutcome.Targets] and each entry carries the effects
	// delivered to it. Downstream, encounter.RecordCastInput is Actor, Spell
	// and Targets and nothing else — there is no caster-side results lane — so
	// a self cast's condition can only be recorded against a target entry, and
	// an empty member is refused there with ErrNoMember AFTER the sheet writes
	// are durable. A sentinel would leave the condition applied and its beat
	// missing.
	//
	// MinTargets and MaxTargets stay zero for a self cast and that is not in
	// tension with this: they bound what the CALLER may name, and a self cast
	// lets the caller name nobody. Who received the spell is a different
	// question, answered here.
	entries := make([]castTargetMachine, 0, len(targetIDs))
	if profile.Target == combatActions.CastTargetSelf {
		targetIDs = []string{casterID}
	}
	for _, targetID := range targetIDs {
		var inner Machine
		var err error
		if profile.Save != nil {
			inner, err = newGatedCast(definition, casterID, targetID, cause, in.Roller)
		} else {
			inner, err = newGatelessCast(definition, casterID, targetID)
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, castTargetMachine{targetID: targetID, inner: inner})
	}

	return &castMachine{
		spell: definition.Ref, spellName: definition.Name, casterID: casterID,
		profile: profile.Clone(), concentration: profile.Concentration, targets: entries,
	}, nil
}

// CastOutcome is what one cast produced, and it is ONE type for both halves of
// the door.
//
// The session switches on [Output.Outcome] once per verb, and a cast that
// answered with a contest here and an activation there would make Cast the only
// verb needing two arms — while [ActivationOutcome] would additionally mean two
// different things depending on which verb asked for it, which is exactly the
// ambiguity a sealed outcome set exists to prevent.
//
// The deciding fact is downstream: a cast's DELIVERIES are one concept. The
// encounter writes one result beat per delivered effect, and reading them off
// two differently-shaped lists would fork every consumer of a cast — the record,
// the wire, and the client — for a difference the player never sees.
//
// CastTargetOutcome is one target's save and delivered consequences. The
// surrounding [CastOutcome.Targets] preserves caller order.
type CastTargetOutcome struct {
	TargetID string
	Save     *ContestOutcome
	Applied  []ImposedEffect
}

// CastOutcome is one paid cast with every target outcome in caller order.
type CastOutcome struct {
	Spell    core.Ref
	CasterID string
	Targets  []CastTargetOutcome

	// FollowUps preserves every target's damage follow-ups in target order.
	FollowUps []FollowUpOutcome
}

func (CastOutcome) isOutcome() {}

// castMachine is one cast wrapped around its ordered target machines. It
// preflights the whole list, then runs each already-started machine in order.
//
// # Preflight still happens before the door
//
// Start runs every inner machine's Start itself, rather than letting the driver
// do it when each Request is reached. That ordering is the whole point: a cast
// naming a recipient who is not in the interaction, or content that cannot be
// delivered, must be refused while [Resolve] is still in pure preflight — after
// the door, the bard has paid for a cast that cannot run.
//
// # A cast cannot pose, and is refused rather than dropped
//
// A requested machine that suspends strands its requester, which [drive]
// refuses by name. No cast poses today — there is no attack roll to interrupt —
// and the day one does, this is the line that has to change rather than a
// silently discarded question.
type castTargetMachine struct {
	targetID string
	inner    Machine
	first    Step
}

type castMachine struct {
	spell         core.Ref
	spellName     string
	casterID      string
	profile       combatActions.CastProfile
	targets       []castTargetMachine
	concentration *combatActions.CastConcentration
	cast          *Participants
	outcome       CastOutcome
}

func (m *castMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
	m.cast = cast
	m.outcome = CastOutcome{Spell: m.spell, CasterID: m.casterID}

	// Preflight the complete list before returning any executable step. This is
	// intentionally construction-only: no rolls, publishes, spends, or removals.
	for i := range m.targets {
		target := &m.targets[i]
		first, err := target.inner.Start(ctx, cast)
		if err != nil {
			return nil, fmt.Errorf("target %d %q: %w", i, target.targetID, err)
		}
		if target.targetID != "" {
			if err := validateCastTarget(ctx, cast, m.casterID, target.targetID, m.profile.RangeFeet); err != nil {
				return nil, fmt.Errorf("target %d %q: %w", i, target.targetID, err)
			}
		}
		target.first = first
	}

	resolve := m.resolveTarget(0)
	if m.concentration == nil {
		return resolve, nil
	}
	if held, holding := concentrationHeldBy(cast, m.casterID); holding {
		return m.drop(held, resolve), nil
	}
	return resolve, nil
}

func (m *castMachine) resolveTarget(index int) Step {
	if index >= len(m.targets) {
		if m.concentration != nil {
			return m.hold(m.outcome)
		}
		return Done{Outcome: m.outcome}
	}
	target := m.targets[index]
	return Request{
		name:    "cast " + m.spell.String() + " on " + target.targetID,
		machine: startedMachine{first: target.first},
		next: func(_ context.Context, out Outcome) (Step, error) {
			shaped, err := m.shapeTarget(target.targetID, out)
			if err != nil {
				return nil, err
			}
			m.outcome.Targets = append(m.outcome.Targets, shaped)
			if shaped.Save != nil {
				m.outcome.FollowUps = append(m.outcome.FollowUps, shaped.Save.FollowUps...)
			}
			return m.resolveTarget(index + 1), nil
		},
	}
}

// drop ends the concentration the caster is already holding, in favour of the
// one about to be cast.
//
// It publishes exactly what a failed check publishes — the owner's removal, and
// only that — through the same helper, so "recast" and "failed the check" end a
// hold the same way and the hold strips its own board either way.
func (m *castMachine) drop(held *conditions.ConcentratingCondition, next Step) Gather {
	removal := &ConditionRemoval{
		Owner:  held.ConditionAddress(),
		Reason: conditions.ConcentrationEndedRecast,
	}
	dropped := publishRemoval(removal, func(ImposedEffect) (Step, error) { return next, nil })

	return Gather{
		name: "drop concentration on " + held.SpellName,
		run:  dropped.run,
	}
}

// hold puts the concentrating condition on the caster and tells it what this
// cast left on the board.
//
// The children are registered BEFORE the condition is applied, so the address
// list is complete in the blob the sheet persists rather than arriving as a
// state change the sheet has to notice.
//
// It is the last step rather than part of the delivery because a cast has two
// halves and only one of them can deliver to its caster: a gated cast's
// contest refuses a caster recipient by name. One step here covers both.
func (m *castMachine) hold(outcome CastOutcome) Gather {
	return Gather{
		name: fmt.Sprintf("concentrate on %s for %s", m.spell.String(), m.casterID),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			holding := conditions.NewConcentratingConditionWithInput(conditions.NewConcentratingConditionInput{
				MemberID: m.casterID, SourceID: m.casterID, SpellRef: m.spell.String(),
				SpellName: m.spellName, TurnEnds: m.concentration.TurnEnds,
				SkipFirstTurnEnd: m.concentration.SkipFirstTurnEnd,
			})
			for _, target := range outcome.Targets {
				for _, applied := range target.Applied {
					if applied.Kind != ImposedCondition || applied.Ref == nil {
						continue
					}
					address := applied.Address
					if address.MemberID == "" {
						address = dnd5eEvents.ConditionAddress{
							MemberID: applied.RecipientID, ConditionRef: applied.Ref.String(),
						}
					}
					if err := holding.AddChild(ctx, address); err != nil {
						return nil, fmt.Errorf("concentrate on %s: %w", m.spell.String(), err)
					}
				}
			}

			caster, err := m.cast.entity(m.casterID)
			if err != nil {
				return nil, err
			}
			if err := dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(
				ctx, dnd5eEvents.ConditionAppliedEvent{
					Target:    caster,
					Type:      dnd5eEvents.ConditionType(holding.Ref().ID),
					Source:    dnd5eEvents.ConditionSourceSpell,
					Condition: holding,
				}); err != nil {
				return nil, fmt.Errorf("concentrate on %s for %q: %w",
					m.spell.String(), m.casterID, err)
			}

			return Done{Outcome: outcome}, nil
		},
	}
}

// concentrationHeldBy finds the concentrating condition a member is already
// carrying.
//
// It reads the CONDITION rather than Character.Concentration's view, and the
// difference is the addresses: the view answers which spell and how many
// effects, which is what a badge and a door check need, while a drop has to
// publish one removal per child and only the condition holds those. The day the
// view carries its children, this reads the view.
func concentrationHeldBy(
	cast *Participants, memberID string,
) (*conditions.ConcentratingCondition, bool) {
	var held []dnd5eEvents.ConditionBehavior
	if character, ok := cast.Character(memberID); ok {
		held = character.GetConditions()
	} else if monster, ok := cast.Monster(memberID); ok {
		held = monster.GetConditions()
	}
	for _, condition := range held {
		if holding, ok := condition.(*conditions.ConcentratingCondition); ok {
			return holding, true
		}
	}

	return nil, false
}

// startedMachine hands back a step somebody else already preflighted.
//
// It exists so a composition can run its inner machine's Start at its OWN Start
// — before payment — and still give the driver a [Machine] to run, which is
// what [Request] takes. Nothing else implements Machine this way, and nothing
// should: a machine that has already started is not a machine anybody may start
// twice, and this one is unexported and constructed in exactly one place.
type startedMachine struct{ first Step }

func (m startedMachine) Start(context.Context, *Participants) (Step, error) { return m.first, nil }

// shapeTarget turns one inner machine's answer into its ordered target result.
//
// Both arms are exhaustive and the default is a refusal rather than a zero
// CastTargetOutcome: an inner machine producing something unexpected is a defect in
// this file, and an empty cast that reported success would hide it behind a
// record saying the cantrip did nothing.
func (m *castMachine) shapeTarget(targetID string, out Outcome) (CastTargetOutcome, error) {
	outcome := CastTargetOutcome{TargetID: targetID}
	switch inner := out.(type) {
	case ContestOutcome:
		if m.profile.Save == nil {
			return CastTargetOutcome{}, fmt.Errorf("%w: %s has no gate and contested a save", ErrBadStep, m.spell.String())
		}
		contest := inner
		outcome.Save = &contest
		outcome.Applied = contest.Imposed
		return outcome, nil
	case ActivationOutcome:
		if m.profile.Save != nil {
			return CastTargetOutcome{}, fmt.Errorf("%w: %s has a gate and delivered without contesting it", ErrBadStep, m.spell.String())
		}
		applied, err := deliveredConditions(m.spell, inner.Effects)
		if err != nil {
			return CastTargetOutcome{}, err
		}
		outcome.Applied = applied
		return outcome, nil
	default:
		return CastTargetOutcome{}, fmt.Errorf("%w: %s produced %T", ErrBadStep, m.spell.String(), out)
	}
}

// deliveredConditions reads the gateless delivery's captured facts back as the
// cast's applied effects.
//
// The collector is what validated each condition's identity against the display
// catalog, so this re-parses a ref it already knows is good rather than trusting
// an unchecked one. A kind other than a condition means the delivery published
// something a gateless cast cannot deliver, and it is refused rather than
// dropped from the record.
func deliveredConditions(spell core.Ref, effects []ActivationEffect) ([]ImposedEffect, error) {
	applied := make([]ImposedEffect, 0, len(effects))
	for _, effect := range effects {
		if effect.Kind != EffectConditionApplied {
			return nil, fmt.Errorf("%w: %s delivered %q, and a gateless cast delivers conditions",
				ErrBadStep, spell.String(), effect.Kind)
		}
		ref, err := core.ParseString(effect.Ref)
		if err != nil {
			return nil, fmt.Errorf("%w: %s delivered an unusable condition ref %q: %w",
				ErrBadStep, spell.String(), effect.Ref, err)
		}
		applied = append(applied, ImposedEffect{
			Kind:        ImposedCondition,
			Ref:         ref,
			Description: conditionDescription(*ref),
			RecipientID: effect.TargetID,
			Address:     effect.Address,
		})
	}

	return applied, nil
}

// validateCastTarget checks one preflighted participant's current eligibility
// and range without mutating a sheet or consuming randomness.
func validateCastTarget(
	ctx context.Context, cast *Participants, casterID, targetID string, rangeFeet int,
) error {
	target, err := combatantFor(cast, targetID)
	if err != nil {
		return err
	}
	state := combat.ClassifyLifeState(combat.LifeStateInput{
		Kind: combat.CombatantKindMonster, Down: combat.IsDown(target),
	})
	if character, ok := cast.Character(targetID); ok {
		state = character.ParticipationView().LifeState
	}
	if !combat.ParticipationFor(state).AttackTarget {
		return fmt.Errorf("%w: target is not currently eligible", ErrBadAction)
	}
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadWorld, err)
	}
	casterPosition, ok := room.GetEntityPosition(casterID)
	if !ok {
		return fmt.Errorf("%w: caster %q has no position", ErrBadWorld, casterID)
	}
	targetPosition, ok := room.GetEntityPosition(targetID)
	if !ok {
		return fmt.Errorf("%w: target %q has no position", ErrBadWorld, targetID)
	}
	maximum := float64(encounter.CellsFromFeet(rangeFeet))
	if distance := room.GetGrid().Distance(casterPosition, targetPosition); distance > maximum {
		return fmt.Errorf("%w: distance %.0f cells exceeds maximum %.0f cells (%d feet)",
			ErrOutOfRange, distance, maximum, rangeFeet)
	}
	return nil
}

func checkCastTargets(profile *combatActions.CastProfile, ref core.Ref, targetIDs []string) error {
	if profile == nil {
		return fmt.Errorf("%w: %s has no cast profile", ErrBadAction, ref.String())
	}
	if len(targetIDs) < profile.MinTargets || len(targetIDs) > profile.MaxTargets {
		return fmt.Errorf("%w: %s requires %d..%d targets, got %d", ErrBadAction,
			ref.String(), profile.MinTargets, profile.MaxTargets, len(targetIDs))
	}
	seen := make(map[string]struct{}, len(targetIDs))
	for i, targetID := range targetIDs {
		if targetID == "" {
			return fmt.Errorf("%w: %s target %d is empty", ErrBadAction, ref.String(), i)
		}
		if _, exists := seen[targetID]; exists {
			return fmt.Errorf("%w: %s target %q is duplicated", ErrBadAction, ref.String(), targetID)
		}
		seen[targetID] = struct{}{}
	}
	return nil
}

// newGatedCast is a save and what failing it costs, which is exactly
// [NewContest]. The saver is the creature the cast named; the damage and the
// condition are the profile's, handed over as the contest's two consequences.
//
// # One condition, on the saver
//
// The contest's scope is one saver, one gate, one condition and one damage set,
// so a profile declaring more than one effect, or one that lands on the caster,
// is REFUSED rather than half-delivered. Neither exists in content today and
// both are real shapes — a spell that damages its target and buffs its caster
// on the same failed save arrives with its own customer, and it will widen the
// contest rather than being smuggled through this branch.
func newGatedCast(
	definition combatActions.Definition, casterID, targetID string,
	cause dnd5eEvents.SaveCause, roller dice.Roller,
) (Machine, error) {
	profile := definition.Cast

	var application combatActions.ConditionApplication
	switch len(profile.Effects) {
	case 0:
	case 1:
		effect := profile.Effects[0]
		if effect.Recipient != combatActions.CastRecipientTarget {
			return nil, fmt.Errorf(
				"%w: %s contests a save and delivers to its caster, which no contest can do",
				ErrBadAction, definition.Ref.String())
		}
		parameters, err := bindCounterpart(effect, casterID)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrBadAction, definition.Ref.String(), err)
		}
		application = combatActions.ConditionApplication{Ref: effect.Ref, Parameters: parameters}
	default:
		return nil, fmt.Errorf("%w: %s declares %d conditions on one save, and a contest delivers one",
			ErrBadAction, definition.Ref.String(), len(profile.Effects))
	}

	return NewContest(&ContestInput{
		Gate:        profile.Save,
		SaverID:     targetID,
		Application: application,
		Damage:      profile.Damage,
		// The compiled definition is the provenance pair: its ref names the
		// spell and its name is what the player reads on the roll.
		SourceName: definition.Name,
		Cause:      cause,
		Roller:     roller,
	}), nil
}

// newGatelessCast is a delivery, not a resolution. Nobody resists it, so there
// is nothing to roll and no policy to apply: the conditions are built here —
// pure, before the door charges anybody — and published on the interaction's
// bus by the activation machine's cast arm.
//
// Damage with no gate is refused. Undefended damage is a real shape (a magic
// missile has one) and it is not this one: every consequence in this slice is
// either contested or a condition, and a branch that silently dropped a
// declared damage pool would be the affordance-with-nothing-behind-it this
// stack keeps finding.
func newGatelessCast(definition combatActions.Definition, casterID, targetID string) (Machine, error) {
	profile := definition.Cast
	if len(profile.Damage) > 0 {
		return nil, fmt.Errorf("%w: %s deals damage with no save, which this module cannot yet deliver",
			ErrBadAction, definition.Ref.String())
	}

	deliveries := make([]preparedDelivery, 0, len(profile.Effects))
	for _, effect := range profile.Effects {
		recipientID, counterpartID := casterID, targetID
		if effect.Recipient == combatActions.CastRecipientTarget {
			recipientID, counterpartID = targetID, casterID
		}
		parameters, err := bindCounterpart(effect, counterpartID)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrBadAction, definition.Ref.String(), err)
		}
		prepared, err := prepareCondition(
			combatActions.ConditionApplication{Ref: effect.Ref, Parameters: parameters},
			recipientID, definition.Ref.String(),
		)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, preparedDelivery{condition: prepared, recipientID: recipientID})
	}

	return NewActivation(&ActivationInput{
		MemberID: casterID,
		TargetID: targetID,
		cast:     &preparedCast{source: definition.Ref, conditions: deliveries},
	})
}

// bindCounterpart writes the cast's OTHER party into the parameter the effect
// names, and hands back the condition's complete configuration.
//
// The key is DECLARED by content rather than conventional here, because the two
// conditions this slice ships spell it differently and mean different things by
// it: True Strike is keyed to the target its advantage is good against, Vicious
// Mockery records the bard who imposed it. Both factories refuse a config
// without it, so a binding this function skipped would fail at build time
// rather than resolving into a condition bound to nobody.
func bindCounterpart(effect combatActions.CastEffect, counterpartID string) (json.RawMessage, error) {
	if effect.CounterpartKey == "" {
		return append(json.RawMessage(nil), effect.Parameters...), nil
	}
	if counterpartID == "" {
		return nil, fmt.Errorf("condition %s binds %q to the cast's other party, and there is none",
			effect.Ref.String(), effect.CounterpartKey)
	}

	fields := map[string]json.RawMessage{}
	if len(effect.Parameters) > 0 {
		if err := json.Unmarshal(effect.Parameters, &fields); err != nil {
			return nil, fmt.Errorf("condition %s parameters are not an object: %w", effect.Ref.String(), err)
		}
	}
	encoded, err := json.Marshal(counterpartID)
	if err != nil {
		return nil, fmt.Errorf("condition %s counterpart %q: %w", effect.Ref.String(), counterpartID, err)
	}
	fields[effect.CounterpartKey] = encoded

	bound, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("condition %s parameters: %w", effect.Ref.String(), err)
	}

	return bound, nil
}
