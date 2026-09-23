// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// SequenceOutcome is what one scripted run of component actions produced.
//
// THE STEPS ARE THE OUTCOME. There is no summed damage, no "hit count", no
// aggregate of any kind: every question a caller can ask about a multiattack
// is a question about one of its swings, and a number computed here would be
// a second copy of an answer already in the list — one that can disagree with
// it the moment a step gains a shape this arithmetic did not expect.
type SequenceOutcome struct {
	// Action is the sequence definition's own ref — the boss's Multiattack,
	// not the scimitar it swung. The component refs ride on the steps.
	Action core.Ref

	// AttackerID and TargetID name the two sides. A sequence strikes ONE
	// target: see [newSequence] for why.
	AttackerID string
	TargetID   string

	// Steps are the component outcomes, in declared order, one per step that
	// actually ran.
	Steps []SequenceStepOutcome

	// Unswung is how many declared steps never happened because the target
	// went down, and zero when the whole script ran.
	//
	// A FIELD RATHER THAN AN INFERENCE. Without it, a two-step sequence that
	// stopped after one blow is indistinguishable from a one-step sequence
	// there is no such thing as, and a caller wanting to tell them apart
	// would have to go back to the definition and subtract — a reconstruction,
	// and reconstructions lie.
	Unswung int

	// FollowUps are every step's follow-ups in the order they were produced —
	// a defender's concentration check against each blow's damage. Flattened
	// rather than kept per-step because [followUpsOf] reads one list off an
	// outcome and a sequence is the fourth damage source it reads.
	//
	// The RECORD of those checks is not flattened: it rides each step, see
	// [SequenceStepOutcome.ConcentrationChecks]. This list is the raw rolls,
	// still reachable whole for anything that reads an outcome's follow-ups
	// without knowing what produced them.
	FollowUps []FollowUpOutcome
}

func (SequenceOutcome) isOutcome() {}

// SequenceStepOutcome is one step's component identity and the blow it
// produced.
//
// THE REF IS CARRIED RATHER THAN LEFT TO BE WORKED OUT. A consumer writing a
// beat per step names the component — a scimitar blow, not "Multiattack" —
// and the only other way to know which component a step was is to walk the
// definition's step list in parallel and trust the two to stay aligned. That
// is a reconstruction, and it would go wrong silently the first time a
// sequence gained a reason to skip a step in the middle rather than truncate.
type SequenceStepOutcome struct {
	// Action is the component definition's ref — the scimitar, not the
	// script that swung it.
	Action core.Ref

	// Strike is the blow itself, in the same shape a lone swing produces.
	Strike StrikeOutcome

	// ConcentrationChecks are the checks a defender MADE against this swing's
	// damage, and ConcentrationBreaks the holds it ended.
	//
	// PER SWING, NOT PER ACTION (Kirk's ruling, 2026-09-20). A concentration
	// check is a roll made against ONE blow, so it belongs on that blow's
	// beat, exactly as a lone strike's does. They are filled by
	// [concentrationCollector.attributeToSteps] after the machine has run,
	// because a hold ends on the bus and the collector is the only thing
	// listening; see its doc for how a fact finds its swing.
	//
	// THE INTERACTION-LEVEL LISTS ARE EMPTY FOR A SEQUENCE. [Output] carries
	// ConcentrationChecks and ConcentrationBreaks for every other machine and
	// carries none for this one, so there is exactly one place these live and
	// a consumer cannot record the same save twice by reading both.
	ConcentrationChecks []encounter.ConcentrationCheck
	ConcentrationBreaks []encounter.ConcentrationBreak
}

// newSequence reads a sequence profile and builds one component machine per
// step over the actor's own repertoire.
//
// # One target, every step
//
// The attack arm requires exactly one target and this arm requires the same,
// for a reason that is upstream of both: a turn's [encounter.Attack] intent
// names one target, so a sequence reached through the driver has one to give.
// The SRD does let a multiattack split its blows, and nothing here forecloses
// it — a per-step target would be a field on [combatActions.SequenceStep],
// arriving with the content that declares one and the caller that can aim it.
// Inventing a second targeting shape now would mean guessing at both.
//
// # Resolved here, and refused here
//
// [combatActions.ResolveSequence] is what matches each step to the component
// definition it named, and it is run at THIS door rather than inside the
// machine: a sequence naming a component the actor does not carry, or naming
// another sequence, is refused while [Resolve] is still in pure preflight —
// before the door has charged anybody for a script that cannot run.
func newSequence(in *ActionInput, normalizedTargetIDs []string) (Machine, error) {
	definition := in.Definition.Clone()
	if in.AttackerID == "" {
		return nil, fmt.Errorf("%w: %s was performed by nobody", ErrBadAction, definition.Ref.String())
	}
	if len(normalizedTargetIDs) != 1 {
		return nil, fmt.Errorf("%w: %s sequence requires exactly one target; got %d",
			ErrBadAction, definition.Ref.String(), len(normalizedTargetIDs))
	}
	targetID := normalizedTargetIDs[0]
	if targetID == "" {
		return nil, fmt.Errorf("%w: %s sequence target 0 is empty",
			ErrBadAction, definition.Ref.String())
	}

	components, err := combatActions.ResolveSequence(definition, in.Components)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
	}

	steps := make([]sequenceStep, 0, len(components))
	for _, component := range components {
		steps = append(steps, sequenceStep{
			action:     component.Definition.Ref,
			definition: component.Definition,
			imposed:    imposedFor(definition.Ref, in.AttackerID, component.Step),
			inner: NewStrike(&StrikeInput{
				AttackerID: in.AttackerID,
				TargetID:   targetID,
				Definition: component.Definition,
				Imposed:    imposedFor(definition.Ref, in.AttackerID, component.Step),
				Roller:     in.Roller,
			}),
		})
	}

	return &sequenceMachine{
		action:     definition.Ref,
		name:       definition.Name,
		attackerID: in.AttackerID,
		targetID:   targetID,
		steps:      steps,
		roller:     in.Roller,
	}, nil
}

// imposedFor turns a step's declared reason into the disadvantage source the
// component swing folds onto its own attack chain.
//
// THE SEQUENCE IS THE SOURCE AND THE ATTACKER IS THE ROLLER. SourceRef names
// the rule — the boss's Multiattack, not the scimitar, because the scimitar
// has nothing to do with why this die is kept low — and SourceID names whose
// die it is (rpg-project#462 R7). A step with no reason imposes nothing, and
// the nil slice says so.
func imposedFor(sequence core.Ref, attackerID string, step combatActions.SequenceStep) []dnd5eEvents.AttackModifierSource {
	if step.Disadvantage == "" {
		return nil
	}
	source := sequence

	return []dnd5eEvents.AttackModifierSource{{
		SourceRef: &source,
		SourceID:  attackerID,
		Reason:    step.Disadvantage,
	}}
}

// sequenceStep is one component machine and the ref it was built from.
type sequenceStep struct {
	definition combatActions.Definition
	imposed    []dnd5eEvents.AttackModifierSource
	action     core.Ref
	inner      Machine
	first      Step
}

// sequenceMachine runs component machines one at a time, in declared order.
//
// IT KNOWS NOTHING ABOUT ATTACKING. Every rule a blow obeys — reach, the
// attack chain, advantage, critical hits, damage, on-hit riders, Sanctuary —
// belongs to the machine the step resolved to, and this one composes those
// through [Request] exactly as a cast composes a contest per target. What it
// owns is the order, the stop rule, and the collected outcome.
type sequenceMachine struct {
	resumeIndex int
	resumed     bool
	action      core.Ref
	name        string
	attackerID  string
	targetID    string
	steps       []sequenceStep

	// roller is carried for the same reason [castMachine] carries one: this
	// machine's own door validates it, and a sequence with no roller is a
	// script whose first swing would fail halfway rather than at Start.
	roller dice.Roller

	// target is the live sheet the stop rule reads between steps.
	//
	// A [combat.Member] because asking whether somebody is still up is a
	// READ, and the field should say so — the same narrowing the effective-AC
	// step makes one file over. The writer surface stays named in strike.go
	// alone, where the applyDamage seam needs it
	// (TestOnlyStrikeNamesTheKeeperSurface).
	target combat.Member

	// outcome accumulates across steps and is the machine's whole state.
	outcome SequenceOutcome
}

// Start preflights EVERY component before returning an executable step.
//
// Construction-only, [castMachine.Start]'s own contract and for its reason: a
// step that cannot run — a component out of reach, a target with no sheet —
// must be refused while nothing has been rolled, published or paid. Half a
// multiattack is worse than none, because the half that happened cannot be
// taken back.
//
// The preflights run against the board as it stands NOW, before any blow has
// landed, and that is correct for what they ask: reach, both combatants'
// presence, and the riders each component declares. Whether the target is
// still up is a different question with a different answer at every step, and
// it is asked between steps rather than here.
func (m *sequenceMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
	if m.roller == nil {
		return nil, fmt.Errorf("%w: a sequence rolls with no roller", ErrNoRoller)
	}
	if len(m.steps) == 0 {
		// Unreachable through [newSequence], which resolves a validated
		// profile of at least two steps. Refused anyway rather than reported
		// as a sequence that swung nothing and succeeded.
		return nil, fmt.Errorf("%w: %s resolved to no steps", ErrBadAction, m.action.String())
	}

	target, err := combatantFor(cast, m.targetID)
	if err != nil {
		return nil, err
	}
	m.target = target
	if !m.resumed {
		m.outcome = SequenceOutcome{Action: m.action, AttackerID: m.attackerID, TargetID: m.targetID}
	}

	for index := m.resumeIndex; index < len(m.steps); index++ {
		first, startErr := m.steps[index].inner.Start(ctx, cast)
		if startErr != nil {
			return nil, fmt.Errorf("sequence %s step %d %s: %w",
				m.action.String(), index, m.steps[index].action.String(), startErr)
		}
		m.steps[index].first = first
	}

	return m.resolveStep(m.resumeIndex), nil
}

// resolveStep runs the step at index, or ends the sequence when the script is
// spent.
func (m *sequenceMachine) resolveStep(index int) Step {
	if index >= len(m.steps) {
		return Done{Outcome: m.outcome}
	}
	step := m.steps[index]

	return Request{
		name:    fmt.Sprintf("sequence %s step %d: %s", m.action.String(), index, step.action.String()),
		onPose:  func(_ context.Context, pose Pose) (Step, error) { return m.freezeSequence(index, pose) },
		machine: startedMachine{first: step.first},
		next: func(_ context.Context, out Outcome) (Step, error) {
			struck, ok := out.(StrikeOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: sequence step %d produced %T", ErrBadStep, index, out)
			}
			m.outcome.Steps = append(m.outcome.Steps, SequenceStepOutcome{
				Action: step.action, Strike: struck,
			})
			m.outcome.FollowUps = append(m.outcome.FollowUps, struck.FollowUps...)

			// THE STOP RULE, and it is about the TARGET rather than the blow.
			//
			// A miss does not end a multiattack: "the goblin makes two attacks
			// with its scimitar" is two swings whatever the first one rolled,
			// and a sequence that quit on a miss would quietly halve a boss.
			//
			// A dead target does end it, because there is nothing left to
			// swing at — and the alternative is not "swing anyway", it is a
			// refusal: the next component's own machine would run against a
			// downed defender and produce a beat about hitting a corpse.
			//
			// Asked HERE rather than at Start because the answer changes with
			// every blow, and asked of the live sheet because that is where
			// the damage just landed.
			if index+1 < len(m.steps) && combat.IsDown(m.target) {
				m.outcome.Unswung = len(m.steps) - (index + 1)
				return Done{Outcome: m.outcome}, nil
			}

			return m.resolveStep(index + 1), nil
		},
	}
}
