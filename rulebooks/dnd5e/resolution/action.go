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

	// AreaMembers are the recipients a caller DERIVED from the profile's
	// declared footprint, for a [combatActions.CastTargetArea] cast. Ignored by
	// every other arm.
	//
	// A SEPARATE FIELD RATHER THAN TargetIDs, because a derived recipient and a
	// named one are not the same thing and the gates here are right to treat
	// them differently. TargetIDs is what the CALLER asked for: it is bounded
	// by MinTargets/MaxTargets, it was offered to a client, and it is re-checked
	// against the caster's reach because a client may echo a stale selection.
	// None of that is true of a member the engine worked out from a shape —
	// nobody offered it, nobody clicked it, and its membership was already
	// decided by geometry.
	//
	// Sharing one field would force every gate downstream to guess which kind
	// it was holding, and a wrong guess is invisible: an area cast would be
	// refused for naming three targets when its profile permits zero, by a rule
	// that is correct for the list it was written about.
	//
	// EMPTY IS LEGAL. A footprint that catches nobody is an ordinary outcome —
	// the cast pays its price, delivers nothing, and records honestly.
	AreaMembers []string

	// Option is the id of the menu entry the caller chose, for a profile that
	// offers one — Command's word.
	//
	// A CAST-TIME INPUT, the way an aimed cell is: content lists the menu on
	// the profile, the client sends back one id, and the engine writes it into
	// the parameters of every effect that names an OptionKey. It is not part of
	// the definition, so a spell with three words is still one declaration and
	// one selector.
	//
	// CHECKED HERE EVEN THOUGH THE CALLER ALREADY CHECKED IT. The host offers
	// the menu and validates the choice against the offer it drew; this package
	// validates it against the profile it is about to execute, and the two are
	// not the same fact — an offer is a compiled snapshot and a definition is
	// what runs. Empty on a profile with a menu, and an id the menu does not
	// list, are both refused rather than resolved into a condition bound to a
	// word nobody can obey.
	Option string

	Roller dice.Roller
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
	if err := checkCastOption(profile, definition.Ref, in.Option); err != nil {
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
	// An area cast's recipients arrive already derived, in their own field, and
	// they replace the caller's list rather than extending it — the caller of an
	// area cast names nobody, which checkCastTargets has just confirmed.
	derived := profile.Target == combatActions.CastTargetArea
	if derived {
		targetIDs = append([]string(nil), in.AreaMembers...)
	}

	entries := make([]castTargetMachine, 0, len(targetIDs))
	if profile.Target == combatActions.CastTargetSelf {
		targetIDs = []string{casterID}
	}
	for _, targetID := range targetIDs {
		var inner Machine
		var err error
		if profile.Save != nil {
			inner, err = newGatedCast(definition, casterID, targetID, in.Option, cause, in.Roller)
		} else {
			inner, err = newGatelessCast(definition, casterID, targetID, in.Option, in.Roller)
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, castTargetMachine{targetID: targetID, inner: inner})
	}

	return &castMachine{
		spell: definition.Ref, spellName: definition.Name, casterID: casterID,
		profile: profile.Clone(), concentration: profile.Concentration, targets: entries,
		derivedTargets: derived,
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

	// derivedTargets records that this cast's recipients were worked out from a
	// declared footprint rather than named by a caller. It changes which
	// preflight checks apply — see Start.
	derivedTargets bool
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
			var err error
			if m.profile.Target == combatActions.CastTargetTouch {
				err = validateTouchTarget(ctx, m.casterID, target.targetID)
			} else if m.profile.Healing != nil {
				err = validateRangedHealingTarget(ctx, m.casterID, target.targetID, m.profile.RangeFeet)
			} else if m.derivedTargets {
				err = castRecipientIsEligible(cast, target.targetID)
			} else {
				err = validateCastTarget(ctx, cast, m.casterID, target.targetID, m.profile.RangeFeet)
			}
			if err != nil {
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
	for _, condition := range heldConditions(cast, memberID) {
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
		applied, err := deliveredEffects(m.spell, inner.Effects)
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
// The activation collector validates and owns the applied facts. Translate each
// supported kind without reducing healing to a condition or losing its trace.
func deliveredEffects(spell core.Ref, effects []ActivationEffect) ([]ImposedEffect, error) {
	applied := make([]ImposedEffect, 0, len(effects))
	for _, effect := range effects {
		if effect.Kind == EffectHealingApplied {
			ref, err := core.ParseString(effect.Ref)
			if err != nil {
				return nil, err
			}
			applied = append(applied, ImposedEffect{Kind: ImposedHealing, Ref: ref, Description: effect.Name, RecipientID: effect.TargetID, Amount: effect.Amount, Requested: effect.Requested, Before: effect.Before, After: effect.After, Calculation: dnd5eEvents.CloneRollCalculation(effect.Calculation)})
			continue
		}
		if effect.Kind != EffectConditionApplied {
			return nil, fmt.Errorf("%w: %s delivered %q, and a gateless cast delivers conditions or healing",
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

// castRecipientIsEligible checks one preflighted participant's current
// eligibility without mutating a sheet or consuming randomness.
//
// APPLIES TO EVERY RECIPIENT, named or derived. Being unconscious does not stop
// a thunderclap reaching you, but it is still the rulebook's answer to whether
// this cast may resolve against you, and it is answered from the sheet rather
// than from the map.
func castRecipientIsEligible(cast *Participants, targetID string) error {
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
	return nil
}

// validateCastTarget checks one NAMED target's eligibility and the caster's
// reach to it.
//
// NOT APPLIED TO A DERIVED RECIPIENT, and the distinction is not a shortcut.
// This measures from the CASTER, which answers "could you have aimed there" —
// the right question for a target a client picked, and the wrong one for a
// member the engine found inside a shape. The two coincide only while a
// footprint is centred on the caster and reaches exactly as far as the spell's
// range, which is true of Thunderclap and of nothing after it: a twenty-foot
// burst dropped at a hundred and fifty feet catches creatures a hundred and
// seventy feet away, every one of which this check would refuse. Containment
// was already decided by whoever derived the members; asking a different
// question here would silently constrain what shapes can exist.
func validateCastTarget(
	ctx context.Context, cast *Participants, casterID, targetID string, rangeFeet int,
) error {
	if err := castRecipientIsEligible(cast, targetID); err != nil {
		return err
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

// checkCastOption refuses a choice the profile's menu does not answer for,
// before the door charges anybody.
//
// FAIL CLOSED IN BOTH DIRECTIONS, and the two refusals are different bugs. A
// profile that offers a menu and receives nothing has lost the player's choice
// somewhere between the client and here; a profile that offers no menu and
// receives an id has a caller sending a word to a spell that has none. Either
// one resolved rather than refused would land a condition configured by
// whatever the factory defaults to, which for a compulsion is a creature
// obeying an order nobody gave.
//
// It is [combatActions.CastProfile.HasOption] that answers, rather than a scan
// written here: content owns what its menu contains, and an empty menu answers
// false for every id, which is the second refusal for free.
func checkCastOption(profile *combatActions.CastProfile, ref core.Ref, option string) error {
	if len(profile.Options) > 0 && option == "" {
		return fmt.Errorf("%w: %s offers %d options and the cast chose none",
			ErrBadAction, ref.String(), len(profile.Options))
	}
	if option != "" && !profile.HasOption(option) {
		return fmt.Errorf("%w: %s does not offer the option %q",
			ErrBadAction, ref.String(), option)
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
	definition combatActions.Definition, casterID, targetID, option string,
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
		parameters, err := bindCast(effect, casterID, option)
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
		Move:        directiveFor(profile.Move, casterID),
		// The compiled definition is the provenance pair: its ref names the
		// spell and its name is what the player reads on the roll.
		SourceName: definition.Name,
		Cause:      cause,
		Roller:     roller,
	}), nil
}

// directiveFor turns content's declaration into the contest's directive by
// adding the one thing content cannot know: who the move is measured from.
//
// The caster, for everything that moves anybody today. A spell that shoves away
// from somewhere else — a point on the floor, a wall, the creature that was hit
// rather than the one who cast — is a real shape and it arrives with a
// declaration that says so, rather than by reinterpreting this line.
func directiveFor(declared *combatActions.CastMove, casterID string) *MoveDirective {
	if declared == nil {
		return nil
	}

	return &MoveDirective{
		Policy:   declared.Policy,
		AnchorID: casterID,
		Cells:    declared.Cells,
		Speed:    declared.Speed,
		// Always false from a cast — the turn budget belongs to a move that IS
		// somebody's turn, and a cast is never that. Copied anyway, so the day
		// content declares one it is not silently dropped by a field this
		// function forgot to carry.
		Turn:     declared.Turn,
		Pays:     declared.Pays,
		Provokes: declared.Provokes,
	}
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
func newGatelessCast(definition combatActions.Definition, casterID, targetID, option string, roller dice.Roller) (Machine, error) {
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
		parameters, err := bindCast(effect, counterpartID, option)
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

	prepared := &preparedCast{source: definition.Ref, conditions: deliveries}
	if profile.Healing != nil {
		if roller == nil {
			return nil, fmt.Errorf("%w: healing requires a roller", ErrBadAction)
		}
		ref := definition.Ref
		prepared.healing = &preparedHealing{declaration: profile.Healing.Clone(), targetID: targetID, source: dnd5eEvents.RollSource{Ref: &ref, Name: definition.Name, SourceID: casterID}, excludes: append([]string(nil), profile.HealingExcludes...), roller: roller}
	}
	return NewActivation(&ActivationInput{
		MemberID: casterID,
		TargetID: targetID,
		cast:     prepared,
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

	return writeParameter(effect.Ref, effect.Parameters, effect.CounterpartKey, counterpartID)
}

// bindCast writes BOTH of a cast's bindings onto one configuration: the other
// party first, then the word the caller chose.
//
// Chained rather than computed side by side, because the two land on the same
// object. Each binding reading the effect's own parameters would hand back a
// configuration carrying one of them and not the other, and the factories
// refuse a config missing either — so the binding that was made would be
// refused as the binding that was not.
func bindCast(effect combatActions.CastEffect, counterpartID, option string) (json.RawMessage, error) {
	bound, err := bindCounterpart(effect, counterpartID)
	if err != nil {
		return nil, err
	}

	return bindOption(effect, bound, option)
}

// bindOption writes the CHOSEN WORD into the parameter the effect names, the
// way bindCounterpart writes the other party into its own.
//
// It takes the parameters rather than reading the effect's, because it is the
// second of two bindings onto one object — see bindCast, which is the only
// caller and the reason this signature differs from its sibling's.
//
// An effect that names a key and receives nothing is REFUSED rather than bound
// to the empty string. The menu was checked at the cast door against the
// profile, so arriving here with no choice means a door was skipped, and a
// compulsion carrying no word is an order the driver cannot read.
func bindOption(
	effect combatActions.CastEffect, parameters json.RawMessage, option string,
) (json.RawMessage, error) {
	if effect.OptionKey == "" {
		return parameters, nil
	}
	if option == "" {
		return nil, fmt.Errorf("condition %s binds %q to the cast's chosen option, and there is none",
			effect.Ref.String(), effect.OptionKey)
	}

	return writeParameter(effect.Ref, parameters, effect.OptionKey, option)
}

// writeParameter puts one string under one key in a condition's configuration
// and hands the whole object back.
//
// The parameters it is given may already carry a binding, so it re-reads them
// rather than starting from the effect: what comes back is everything content
// authored plus everything the engine has bound so far.
func writeParameter(ref core.Ref, parameters json.RawMessage, key, value string) (json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if len(parameters) > 0 {
		if err := json.Unmarshal(parameters, &fields); err != nil {
			return nil, fmt.Errorf("condition %s parameters are not an object: %w", ref.String(), err)
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("condition %s %s %q: %w", ref.String(), key, value, err)
	}
	fields[key] = encoded

	bound, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("condition %s parameters: %w", ref.String(), err)
	}

	return bound, nil
}
