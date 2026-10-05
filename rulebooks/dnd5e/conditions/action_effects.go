// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// unavailableReason is the reason every not-yet-answering effect carries.
const unavailableReason = "This effect cannot yet say whether it applies to this action"

// AssessActionEffectsInput supplies the acting character's own loaded
// conditions, in persisted order, and the frame to answer from.
type AssessActionEffectsInput struct {
	Conditions []dnd5eEvents.ConditionBehavior
	Frame      contributions.Frame
}

// AssessActionEffectsOutput lists one row per effect bearing on the action, in
// persisted order. It is never folded into a total.
type AssessActionEffectsOutput struct {
	Effects []contributions.Effect
}

// AssessActionEffects asks every loaded effect how it bears on the framed
// action. An effect that does not bear yields no row; one whose rule cannot yet
// answer yields an unavailable row with its description; one that answers is
// asked through contributions.ActionAssessor and mapped one to one. When an
// earlier roll contribution in the same stacking group already applies, a
// later one does not apply, with that reason.
//
// Reading spends, rolls and publishes nothing. Errors: nil input, an invalid
// frame, a condition whose ref the census does not classify, a bearing effect
// with no description, an answering effect that is not an ActionAssessor or
// answers invalidly, and a duplicate row ID.
func AssessActionEffects(in *AssessActionEffectsInput) (*AssessActionEffectsOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "assess action effects requires input")
	}
	if err := in.Frame.Validate(); err != nil {
		return nil, rpgerr.Wrap(err, "assess action effects requires a valid frame")
	}
	roll, _ := in.Frame.Action.Roll.Get()
	groups := newRollGroupSelection(&dnd5eEvents.DescribeRollContributionsInput{Kind: roll})

	effects := make([]contributions.Effect, 0, len(in.Conditions))
	ids := make(map[string]struct{}, len(in.Conditions))
	for _, condition := range in.Conditions {
		if condition == nil || condition.Ref() == nil {
			return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "assess action effects received a condition with no ref")
		}
		ref := *condition.Ref()
		entry, classified := actionCensus[ref.String()]
		if !classified {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "condition %s has no action census entry", ref.String())
		}
		if entry.class == actionNotBearing {
			continue
		}
		display, found := DisplayFor(ref)
		if !found || display.Detail == "" {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "condition %s bears on actions but has no description", ref.String())
		}

		effect := contributions.Effect{
			ID:            ref.String(),
			Source:        contributions.Source{Ref: &ref, Name: display.Name},
			Description:   display.Detail,
			Participation: entry.participation,
		}
		if addressed, ok := condition.(dnd5eEvents.ConditionAddressProvider); ok {
			if sourceID := addressed.ConditionAddress().SourceID; sourceID != "" {
				effect.ID += "@" + sourceID
				effect.Source.SourceID = sourceID
			}
		}
		if _, duplicate := ids[effect.ID]; duplicate {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "duplicate effect row id %q", effect.ID)
		}
		ids[effect.ID] = struct{}{}

		switch entry.class {
		case actionNotYetAnswering:
			effect.State = contributions.StateUnavailable
			effect.Reason = unavailableReason
		case actionAnswers:
			if err := answerEffect(&answerEffectInput{
				Condition: condition, Entry: entry, Frame: in.Frame, Groups: groups, Effect: &effect,
			}); err != nil {
				return nil, err
			}
		default:
			return nil, rpgerr.Newf(rpgerr.CodeInternal, "condition %s has unknown census class %q", ref.String(), entry.class)
		}
		effects = append(effects, effect)
	}
	return &AssessActionEffectsOutput{Effects: effects}, nil
}

// answerEffectInput carries one answering condition and the row being filled.
type answerEffectInput struct {
	Condition dnd5eEvents.ConditionBehavior
	Entry     actionCensusEntry
	Frame     contributions.Frame
	Groups    *rollGroupSelection
	Effect    *contributions.Effect
}

// answerEffect asks one answering condition and maps its answer onto the row,
// applying roll-contribution stacking in persisted order.
func answerEffect(in *answerEffectInput) error {
	ref := in.Condition.Ref().String()
	assessor, ok := in.Condition.(contributions.ActionAssessor)
	if !ok {
		return rpgerr.Newf(rpgerr.CodeInternal, "condition %s is classified as answering but does not answer", ref)
	}
	out, err := assessor.AssessAction(&contributions.AssessActionInput{Frame: in.Frame.Clone()})
	if err != nil {
		return fmt.Errorf("assess %s: %w", ref, err)
	}
	if out == nil {
		return rpgerr.Newf(rpgerr.CodeInternal, "condition %s returned no answer", ref)
	}
	answer := out.Answer
	if err := validateAnswer(answer, in.Entry.participation); err != nil {
		return fmt.Errorf("assess %s: %w", ref, err)
	}

	in.Effect.State = effectState(answer.Decision.Applicability)
	in.Effect.Reason = answer.Decision.Reason
	in.Effect.Benefit = answer.Benefit

	provider, contributesToRolls := in.Condition.(dnd5eEvents.RollContributionProvider)
	if !contributesToRolls {
		return nil
	}
	applies := answer.Decision.Applicability == contributions.Applies
	name := ""
	if applies {
		if len(answer.Roll) == 0 {
			return rpgerr.Newf(rpgerr.CodeInternal, "roll contribution %s applies but describes no dice", ref)
		}
		name = answer.Roll[0].Source.Name
	}
	claim, err := in.Groups.claim(&rollClaimInput{Provider: provider, Applies: applies, Name: name})
	if err != nil {
		return fmt.Errorf("assess %s: %w", ref, err)
	}
	if claim.Applicable != applies {
		return rpgerr.Newf(rpgerr.CodeInternal, "condition %s answer disagrees with its roll contribution metadata", ref)
	}
	if claim.Shadowed {
		in.Effect.State = contributions.StateDoesNotApply
		in.Effect.Reason = "Another " + claim.HeldBy + " already adds to this roll"
		in.Effect.Benefit = ""
	}
	return nil
}

// validateAnswer refuses a producer defect: an invalid decision, a
// participation other than the census declares, or a contribution carried by
// an answer that does not apply.
func validateAnswer(answer contributions.Answer, participation contributions.Participation) error {
	if err := answer.Decision.Validate(); err != nil {
		return err
	}
	if answer.Participation != participation {
		return fmt.Errorf("answer participation %q differs from its census entry %q", answer.Participation, participation)
	}
	if answer.Decision.Applicability != contributions.Applies &&
		(answer.Benefit != "" || len(answer.Damage) != 0 || len(answer.Roll) != 0) {
		return fmt.Errorf("an answer that does not apply cannot carry a benefit or contribution")
	}
	return nil
}

// effectState maps a validated applicability onto its row state.
func effectState(applicability contributions.Applicability) contributions.EffectState {
	switch applicability {
	case contributions.Applies:
		return contributions.StateApplies
	case contributions.DoesNotApply:
		return contributions.StateDoesNotApply
	default:
		return contributions.StateDepends
	}
}

// frameOf opens every rule's answer: a nil input or an invalid frame is an
// error wrapping ErrRuleCannotAnswer, never a negative answer.
func frameOf(in *contributions.AssessActionInput, rule string) (contributions.Frame, error) {
	if in == nil {
		return contributions.Frame{}, fmt.Errorf("%s: %w: no input", rule, contributions.ErrRuleCannotAnswer)
	}
	if err := in.Frame.Validate(); err != nil {
		return contributions.Frame{}, fmt.Errorf("%s: %w: %w", rule, contributions.ErrRuleCannotAnswer, err)
	}
	return in.Frame, nil
}

// executeRuleInput names the rule an execution handler asks and the event's
// frame.
type executeRuleInput struct {
	Name  string
	Rule  contributions.ActionAssessor
	Frame contributions.Frame
}

// executeRuleOutput carries a settled answer: Applies or DoesNotApply.
type executeRuleOutput struct {
	Answer contributions.Answer
}

// executeRule asks a rule the way execution must: the frame is validated
// first, and an invalid frame or a Depends answer is an error wrapping
// ErrRuleCannotAnswer, so a missing fact fails the action instead of switching
// the rule off.
func executeRule(in *executeRuleInput) (*executeRuleOutput, error) {
	if err := in.Frame.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", in.Name, contributions.ErrRuleCannotAnswer, err)
	}
	out, err := in.Rule.AssessAction(&contributions.AssessActionInput{Frame: in.Frame.Clone()})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", in.Name, err)
	}
	if out == nil {
		return nil, fmt.Errorf("%s: rule returned no answer", in.Name)
	}
	if err := out.Answer.Decision.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", in.Name, err)
	}
	if out.Answer.Decision.Applicability == contributions.Depends {
		return nil, fmt.Errorf("%s: %w: %s", in.Name, contributions.ErrRuleCannotAnswer, out.Answer.Decision.Reason)
	}
	return &executeRuleOutput{Answer: out.Answer}, nil
}
