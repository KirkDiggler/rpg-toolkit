// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// frozenSequenceStep is one declared component, stored whole so a step beyond
// the paused one can be rebuilt exactly as [newSequence] built it.
type frozenSequenceStep struct {
	Definition combatActions.Definition         `json:"definition"`
	Imposed    []dndEvents.AttackModifierSource `json:"imposed,omitempty"`
}

// frozenSequence is a multiattack stopped on one of its swings. It holds the
// script and where it stopped — never what already settled, which was told
// with the pause.
type frozenSequence struct {
	Action     core.Ref             `json:"action"`
	Name       string               `json:"name"`
	AttackerID string               `json:"attacker_id"`
	TargetID   string               `json:"target_id"`
	Steps      []frozenSequenceStep `json:"steps"`
	Index      int                  `json:"index"`
	Inner      json.RawMessage      `json:"inner"`
}

// freezeSequence turns the paused swing's pause into the sequence's. Kind and
// price are the swing's. What settled in this call — every earlier swing, and
// the paused swing's own hit when it paused after one — rides the pause to the
// driver, so it is told now, once.
func (m *sequenceMachine) freezeSequence(index int, inner Pause) (Step, error) {
	steps := make([]frozenSequenceStep, len(m.steps))
	for i, step := range m.steps {
		steps[i] = frozenSequenceStep{Definition: step.definition, Imposed: step.imposed}
	}
	frozen, err := writeFrozen(machineSequence, inner.Kind, frozenSequence{
		Action: m.action, Name: m.name, AttackerID: m.attackerID, TargetID: m.targetID,
		Steps: steps, Index: index, Inner: inner.Frozen,
	})
	if err != nil {
		return nil, err
	}
	return Pause{Kind: inner.Kind, Ask: inner.Ask, Cost: inner.Cost, Frozen: frozen,
		settled: m.settledWith(index, inner.settled)}, nil
}

// settledWith is the sequence as it settled in this call, with the paused
// swing's own settled half appended when it has one. Nil when nothing settled.
func (m *sequenceMachine) settledWith(index int, inner Outcome) Outcome {
	out := m.outcome
	out.Steps = append([]SequenceStepOutcome(nil), m.outcome.Steps...)
	out.FollowUps = append([]FollowUpOutcome(nil), m.outcome.FollowUps...)
	if struck, ok := inner.(StrikeOutcome); ok {
		out.Steps = append(out.Steps, SequenceStepOutcome{Action: m.steps[index].action, Strike: struck})
		out.FollowUps = append(out.FollowUps, struck.FollowUps...)
	}
	if len(out.Steps) == 0 {
		return nil
	}
	return out
}

// resumeSequence resumes a multiattack paused on one of its swings. The
// paused swing resumes through the one table; the swings after it are rebuilt
// from the script; the swings before it already happened and are never run
// again.
func resumeSequence(h frozenHeader, in *ResumeInput) (Machine, error) {
	var f frozenSequence
	if err := decodeState(h, &f); err != nil {
		return nil, err
	}
	if f.AttackerID == "" || f.TargetID == "" || f.Index < 0 || f.Index >= len(f.Steps) {
		return nil, fmt.Errorf("%w: invalid sequence continuation", ErrBadFrozen)
	}
	current, err := resumeInner(f.Inner, in, strikeMachines...)
	if err != nil {
		return nil, err
	}
	steps := make([]sequenceStep, len(f.Steps))
	for i, step := range f.Steps {
		inner := NewStrike(&StrikeInput{AttackerID: f.AttackerID, TargetID: f.TargetID, Definition: step.Definition, Imposed: step.Imposed, Roller: in.Roller})
		if i == f.Index {
			inner = current
		}
		steps[i] = sequenceStep{action: step.Definition.Ref, definition: step.Definition, imposed: step.Imposed, inner: inner}
	}
	return &sequenceMachine{action: f.Action, name: f.Name, attackerID: f.AttackerID, targetID: f.TargetID, steps: steps, roller: in.Roller, resumeIndex: f.Index}, nil
}
