// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// target_held.go holds the rules for effects the TARGET of an attack holds
// (R17). Each is keyed by condition reference and reads only the frame's held
// list, pairs and target: information asks it for a candidate through
// AssessTargetHeldEffects, and at execution whoever applies the effect asks the
// same function — the loaded condition's attack-chain handler, or resolution
// through ExecuteHeldEffect where the check lives there.

// heldRuleFor builds the rule for one held condition on its holder.
type heldRuleFor func(holder string, held contributions.HeldCondition) contributions.ActionAssessor

// targetHeldRules is every answering target-census ref's rule, keyed by
// condition ref. Each handler builds its rule through the same constructor.
var targetHeldRules = map[string]heldRuleFor{
	refs.Conditions.FaerieFire().String():     newFaerieFireHeldRule,
	refs.Conditions.GuidingBolt().String():    newGuidingBoltHeldRule,
	refs.Conditions.Dodging().String():        newDodgingHeldRule,
	refs.Conditions.Prone().String():          newProneHeldRule,
	refs.Conditions.Sanctuary().String():      newSanctuaryHeldRule,
	refs.Conditions.Hidden().String():         newHiddenHeldRule,
	refs.Conditions.RecklessAttack().String(): newRecklessHeldRule,
}

// heldRule answers for one condition its holder holds, on an attack against
// that holder. The shared gate runs first; decide answers only once the frame
// shows the holder holds it and is the attack's target.
type heldRule struct {
	name   string
	holder string
	held   contributions.HeldCondition
	decide func(frame contributions.Frame, holder string) *contributions.AssessActionOutput
}

// AssessAction runs the shared gate, in order: what the holder holds unknown
// depends; a holder the frame shows not holding this effect does not apply; a
// roll that is not an attack does not apply; an unknown target depends; an
// attack on anyone but the holder does not apply. An invalid frame is an
// error.
func (r heldRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, strings.ToLower(r.name))
	if err != nil {
		return nil, err
	}
	held, known := frame.HeldBy(r.holder)
	if !known {
		return assessed(contributions.Depends, "Depends on what the target holds"), nil
	}
	if !slices.Contains(held, r.held) {
		return assessed(contributions.DoesNotApply, "The target does not hold this effect"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, r.name+" affects only attack rolls"), nil
	}
	target, known := frame.Target.Get()
	if !known {
		return assessed(contributions.Depends, "Depends on the target"), nil
	}
	if target != r.holder {
		return assessed(contributions.DoesNotApply, r.name+" affects only attacks against its holder"), nil
	}
	return r.decide(frame, r.holder), nil
}

// heldApplies is an applying answer with its benefit and attack mode.
func heldApplies(reason, benefit string, mode contributions.AttackMode) *contributions.AssessActionOutput {
	out := assessed(contributions.Applies, reason)
	out.Answer.Benefit = benefit
	out.Answer.AttackMode = mode
	return out
}

const (
	advantageBenefit    = "Advantage on the attack roll"
	disadvantageBenefit = "Disadvantage on the attack roll"
)

// newFaerieFireHeldRule: advantage when the attacker can see the outlined
// target.
func newFaerieFireHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: FaerieFireName, holder: holder, held: held, decide: func(frame contributions.Frame, holder string) *contributions.AssessActionOutput {
		sees, known := frame.Pair(frame.Actor, holder).Sees.Get()
		switch {
		case !known:
			return assessed(contributions.Depends, "Depends on whether you can see the target")
		case !sees:
			return assessed(contributions.DoesNotApply, "You cannot see the target")
		default:
			return heldApplies("You can see the outlined target", advantageBenefit, contributions.AttackAdvantage)
		}
	}}
}

// newGuidingBoltHeldRule: advantage on the next attack against the target.
func newGuidingBoltHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: GuidingBoltName, holder: holder, held: held, decide: func(contributions.Frame, string) *contributions.AssessActionOutput {
		return heldApplies("The target is lit by Guiding Bolt", advantageBenefit, contributions.AttackAdvantage)
	}}
}

// newRecklessHeldRule: advantage on every attack against the reckless holder,
// whoever makes it.
func newRecklessHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: "Reckless Attack", holder: holder, held: held, decide: func(contributions.Frame, string) *contributions.AssessActionOutput {
		return heldApplies("The target is attacking recklessly", advantageBenefit, contributions.AttackAdvantage)
	}}
}

// newDodgingHeldRule: disadvantage on attacks against the dodging target. It
// reads no sight; the 5e text's "an attacker you can see" is flagged for a
// ruling, not built.
func newDodgingHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: "Dodging", holder: holder, held: held, decide: func(contributions.Frame, string) *contributions.AssessActionOutput {
		return heldApplies("The target is dodging", disadvantageBenefit, contributions.AttackDisadvantage)
	}}
}

// newHiddenHeldRule: disadvantage on attacks against the hidden target.
func newHiddenHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: "Hidden", holder: holder, held: held, decide: func(contributions.Frame, string) *contributions.AssessActionOutput {
		return heldApplies("The target is hidden", disadvantageBenefit, contributions.AttackDisadvantage)
	}}
}

// newProneHeldRule: advantage from within 5 feet of the prone target,
// disadvantage from beyond, read from the attacker→target distance. Both
// directions are decided here; an unknown distance depends.
func newProneHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: "Prone", holder: holder, held: held, decide: func(frame contributions.Frame, holder string) *contributions.AssessActionOutput {
		distance, known := frame.Pair(frame.Actor, holder).DistanceCells.Get()
		switch {
		case !known:
			return assessed(contributions.Depends, "Depends on how far you are from the target")
		case distance <= combat.AdjacentCells:
			return heldApplies("The prone target is within 5 feet", advantageBenefit, contributions.AttackAdvantage)
		default:
			return heldApplies("The prone target is beyond 5 feet", disadvantageBenefit, contributions.AttackDisadvantage)
		}
	}}
}

// newSanctuaryHeldRule: an attacker other than the warded holder must save
// first. It contributes no attack mode: the save and its consequence are
// resolution's to run.
func newSanctuaryHeldRule(holder string, held contributions.HeldCondition) contributions.ActionAssessor {
	return heldRule{name: SanctuaryName, holder: holder, held: held, decide: func(frame contributions.Frame, holder string) *contributions.AssessActionOutput {
		if frame.Actor == holder {
			return assessed(contributions.DoesNotApply, "Your own ward does not stop your attack")
		}
		return heldApplies("The target is warded by Sanctuary",
			"Wisdom saving throw first; on a failure the attack is lost", "")
	}}
}

// AssessTargetHeldEffectsInput carries the frame; its Target must be known.
type AssessTargetHeldEffectsInput struct {
	Frame contributions.Frame
}

// AssessTargetHeldEffectsOutput lists one row per effect the target holds that
// bears on the attack, in the frame's held order. It is never folded.
type AssessTargetHeldEffectsOutput struct {
	Effects []contributions.Effect
}

// AssessTargetHeldEffects answers which conditions the frame's target holds
// bear on the framed attack, and how. A target whose holdings are unknown
// yields no rows: no effect is known to exist there. A held effect that does
// not bear yields no row; one whose rule cannot yet answer yields an
// unavailable row with its description; one that answers is asked through its
// rule by reference. Each row's ID is "target:" + ref, plus "@" + source when
// it has one, so it never equals a row for the actor's own effect.
//
// Reading spends, rolls and publishes nothing. Errors: nil input, an invalid
// frame, an unknown target, a held ref the target census does not classify, a
// bearing effect with no description, an invalid answer, and a duplicate row
// ID.
func AssessTargetHeldEffects(in *AssessTargetHeldEffectsInput) (*AssessTargetHeldEffectsOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "assess target-held effects requires input")
	}
	if err := in.Frame.Validate(); err != nil {
		return nil, rpgerr.Wrap(err, "assess target-held effects requires a valid frame")
	}
	target, known := in.Frame.Target.Get()
	if !known {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "assess target-held effects requires a known target")
	}
	held, known := in.Frame.HeldBy(target)
	if !known {
		return &AssessTargetHeldEffectsOutput{}, nil
	}

	effects := make([]contributions.Effect, 0, len(held))
	ids := make(map[string]struct{}, len(held))
	for _, condition := range held {
		entry, classified := targetCensus[condition.Ref]
		if !classified {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "held condition %s has no target census entry", condition.Ref)
		}
		if entry.class == censusNotBearing {
			continue
		}
		ref, err := core.ParseString(condition.Ref)
		if err != nil {
			return nil, rpgerr.Wrapf(err, "held condition %s", condition.Ref)
		}
		display, found := DisplayFor(*ref)
		if !found || display.Detail == "" {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "held condition %s bears on attacks but has no description", condition.Ref)
		}
		effect := contributions.Effect{
			ID:            heldEffectID(condition),
			Source:        contributions.Source{Ref: ref, Name: display.Name, SourceID: condition.SourceID},
			Description:   display.Detail,
			Participation: entry.participation,
		}
		if _, duplicate := ids[effect.ID]; duplicate {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "duplicate effect row id %q", effect.ID)
		}
		ids[effect.ID] = struct{}{}

		switch entry.class {
		case censusNotYetAnswering:
			effect.State = contributions.StateUnavailable
			effect.Reason = unavailableReason
		case censusAnswers:
			answer, err := assessHeld(target, condition, in.Frame, entry)
			if err != nil {
				return nil, err
			}
			effect.State = effectState(answer.Decision.Applicability)
			effect.Reason = answer.Decision.Reason
			effect.Benefit = answer.Benefit
		default:
			return nil, rpgerr.Newf(rpgerr.CodeInternal, "held condition %s has unknown census class %q", condition.Ref, entry.class)
		}
		effects = append(effects, effect)
	}
	return &AssessTargetHeldEffectsOutput{Effects: effects}, nil
}

// heldEffectID is a held row's ID: "target:" + ref, plus "@" + source.
func heldEffectID(held contributions.HeldCondition) string {
	id := "target:" + held.Ref
	if held.SourceID != "" {
		id += "@" + held.SourceID
	}
	return id
}

// assessHeld asks one answering held condition's rule and validates its answer.
func assessHeld(
	holder string, held contributions.HeldCondition, frame contributions.Frame, entry censusEntry,
) (contributions.Answer, error) {
	rule, ok := targetHeldRules[held.Ref]
	if !ok {
		return contributions.Answer{}, rpgerr.Newf(rpgerr.CodeInternal, "held condition %s is classified as answering but has no rule", held.Ref)
	}
	out, err := rule(holder, held).AssessAction(&contributions.AssessActionInput{Frame: frame.Clone()})
	if err != nil {
		return contributions.Answer{}, fmt.Errorf("assess held %s: %w", held.Ref, err)
	}
	if out == nil {
		return contributions.Answer{}, rpgerr.Newf(rpgerr.CodeInternal, "held condition %s returned no answer", held.Ref)
	}
	if err := validateAnswer(out.Answer, entry.participation); err != nil {
		return contributions.Answer{}, fmt.Errorf("assess held %s: %w", held.Ref, err)
	}
	return out.Answer, nil
}

// ExecuteHeldEffectInput names one held condition, its holder and the
// execution frame.
type ExecuteHeldEffectInput struct {
	Holder string
	Held   contributions.HeldCondition
	Frame  contributions.Frame
}

// ExecuteHeldEffectOutput carries the settled answer: Applies or DoesNotApply.
type ExecuteHeldEffectOutput struct {
	Answer contributions.Answer
}

// ExecuteHeldEffect asks a held condition's by-reference rule with execution
// semantics, for a caller that applies the effect itself — resolution, where a
// check like Sanctuary's ward lives. Errors: nil input; a ref with no held
// rule; an invalid frame or a Depends answer, wrapping
// contributions.ErrRuleCannotAnswer.
func ExecuteHeldEffect(in *ExecuteHeldEffectInput) (*ExecuteHeldEffectOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "execute held effect requires input")
	}
	rule, ok := targetHeldRules[in.Held.Ref]
	if !ok {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "held condition %s has no held rule", in.Held.Ref)
	}
	executed, err := executeHeld(&executeHeldInput{
		Name: "held " + in.Held.Ref, Holder: in.Holder, Held: in.Held, Rule: rule(in.Holder, in.Held), Frame: in.Frame,
	})
	if err != nil {
		return nil, err
	}
	return &ExecuteHeldEffectOutput{Answer: executed.Answer}, nil
}

// executeHeldInput names a held condition its holder applies, its rule and
// the execution frame.
type executeHeldInput struct {
	Name   string
	Holder string
	Held   contributions.HeldCondition
	Rule   contributions.ActionAssessor
	Frame  contributions.Frame
}

// executeHeld asks a held rule the way execution must. At execution the
// caller applying the effect is authoritative that its holder holds it — the
// loaded condition is that proof, and so is a ward resolution found on the
// holder's sheet. So a valid frame that lists the holder's holdings WITHOUT
// this address is a frame that cannot answer (R13), never "the target does
// not hold this effect": accepting it would switch the effect off silently,
// and Guiding Bolt would even be spent for nothing. An unknown holding is the
// rule's Depends, and executeRule refuses that too. Information keeps the
// rule's gate as it is, because there a stale sighting is legitimate
// testimony.
func executeHeld(in *executeHeldInput) (*executeRuleOutput, error) {
	if err := in.Frame.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", in.Name, contributions.ErrRuleCannotAnswer, err)
	}
	if listed, known := in.Frame.HeldBy(in.Holder); known && !slices.Contains(listed, in.Held) {
		return nil, fmt.Errorf("%s: %w: the frame omits a held condition its holder applies (%s@%q on %q)",
			in.Name, contributions.ErrRuleCannotAnswer, in.Held.Ref, in.Held.SourceID, in.Holder)
	}
	return executeRule(&executeRuleInput{Name: in.Name, Rule: in.Rule, Frame: in.Frame})
}

// heldAttackInput names a target-held handler's rule and how an applying
// answer reads on the attack chain.
type heldAttackInput struct {
	Name string
	// Holder and Held are the handler's own holder and address: the loaded
	// condition is proof its holder holds it.
	Holder    string
	Held      contributions.HeldCondition
	Rule      contributions.ActionAssessor
	Event     dnd5eEvents.AttackChainEvent
	Chain     chain.Chain[dnd5eEvents.AttackChainEvent]
	SourceRef *core.Ref
	SourceID  string
	// Label names the chain key and the source's reason for the answer's
	// attack mode.
	Label func(contributions.AttackMode) (key, reason string)
}

// applyHeldAttack is the attack-chain half of a target-held handler: it asks
// the held rule from the event's frame and, when it applies, adds the answer's
// attack mode through applyAttackMode. An invalid frame, a frame that omits
// the handler's own address, or a Depends answer fails the attack (see
// executeHeld).
func applyHeldAttack(in *heldAttackInput) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	executed, err := executeHeld(&executeHeldInput{
		Name: in.Name, Holder: in.Holder, Held: in.Held, Rule: in.Rule, Frame: in.Event.Frame,
	})
	if err != nil {
		return in.Chain, err
	}
	return applyAttackMode(&attackModeInput{
		Name: in.Name, Answer: executed.Answer, Chain: in.Chain,
		SourceRef: in.SourceRef, SourceID: in.SourceID, Label: in.Label,
	})
}

// attackModeInput is a settled answer and how its attack mode reads on the
// attack chain.
type attackModeInput struct {
	Name      string
	Answer    contributions.Answer
	Chain     chain.Chain[dnd5eEvents.AttackChainEvent]
	SourceRef *core.Ref
	SourceID  string
	// Label names the chain key and the source's reason for the answer's
	// attack mode.
	Label func(contributions.AttackMode) (key, reason string)
}

// applyAttackMode adds an advantage or disadvantage source as a settled
// answer's attack mode says, so the rule that answered — for a held effect or
// for the holder's own attack — is the one source that decides which. An
// answer that does not apply adds nothing; an applying answer with no mode is
// a producer defect.
func applyAttackMode(in *attackModeInput) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if in.Answer.Decision.Applicability != contributions.Applies {
		return in.Chain, nil
	}
	mode := in.Answer.AttackMode
	key, reason := in.Label(mode)
	source := dnd5eEvents.AttackModifierSource{SourceRef: in.SourceRef, SourceID: in.SourceID, Reason: reason}
	var modify func(context.Context, dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error)
	switch mode {
	case contributions.AttackAdvantage:
		modify = func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, source)
			return e, nil
		}
	case contributions.AttackDisadvantage:
		modify = func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			e.DisadvantageSources = append(e.DisadvantageSources, source)
			return e, nil
		}
	default:
		return in.Chain, rpgerr.Newf(rpgerr.CodeInternal, "%s applies with no attack mode", in.Name)
	}
	if err := in.Chain.Add(combat.StageConditions, key, modify); err != nil {
		return in.Chain, rpgerr.Wrapf(err, "failed to add %s", key)
	}
	return in.Chain, nil
}

// heldAddress is the held condition a loaded condition stands for on its
// holder: its own address, the same one execution frames list.
func heldAddress(member string, condition dnd5eEvents.ConditionBehavior) contributions.HeldCondition {
	address := ConditionAddressOf(member, condition)
	return contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID}
}

// fixedLabel names one chain key and reason whatever the attack mode.
func fixedLabel(key, reason string) func(contributions.AttackMode) (string, string) {
	return func(contributions.AttackMode) (string, string) { return key, reason }
}
