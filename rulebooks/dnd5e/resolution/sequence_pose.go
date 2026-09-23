package resolution

import (
	"encoding/json"
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const frozenSequenceKind = "attack.sequence"

type frozenSequenceStep struct {
	Definition combatActions.Definition         `json:"definition"`
	Imposed    []dndEvents.AttackModifierSource `json:"imposed,omitempty"`
}
type frozenSequence struct {
	Kind       string               `json:"kind"`
	Version    int                  `json:"version"`
	Action     core.Ref             `json:"action"`
	Name       string               `json:"name"`
	AttackerID string               `json:"attacker_id"`
	TargetID   string               `json:"target_id"`
	Steps      []frozenSequenceStep `json:"steps"`
	Index      int                  `json:"index"`
	Inner      []byte               `json:"inner"`
	Outcome    SequenceOutcome      `json:"outcome"`
}

func (m *sequenceMachine) freezeSequence(index int, pose Pose) (Step, error) {
	steps := make([]frozenSequenceStep, len(m.steps))
	for i, step := range m.steps {
		steps[i] = frozenSequenceStep{Definition: step.definition, Imposed: step.imposed}
	}
	frozen, err := json.Marshal(frozenSequence{Kind: frozenSequenceKind, Version: 1, Action: m.action, Name: m.name, AttackerID: m.attackerID, TargetID: m.targetID, Steps: steps, Index: index, Inner: pose.Frozen, Outcome: m.outcome})
	if err != nil {
		return nil, err
	}
	return Pose{Ask: pose.Ask, Frozen: frozen, BeforeRoll: pose.BeforeRoll, Sequence: &m.outcome}, nil
}

// NewAttackResumed resumes a lone attack or a suspended component of a
// multiattack. Already completed swings are retained and never executed again.
func NewAttackResumed(in *StrikeResumeInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(in.Frozen, &header); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if header.Kind != frozenSequenceKind {
		return NewStrikeResumed(in)
	}
	var f frozenSequence
	if err := json.Unmarshal(in.Frozen, &f); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if f.Version != 1 || f.AttackerID == "" || f.TargetID == "" || f.Index < 0 || f.Index >= len(f.Steps) || len(f.Outcome.Steps) != f.Index {
		return nil, fmt.Errorf("%w: invalid sequence continuation", ErrBadFrozen)
	}
	nested := *in
	nested.Frozen = f.Inner
	current, err := NewStrikeResumed(&nested)
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
	return &sequenceMachine{action: f.Action, name: f.Name, attackerID: f.AttackerID, targetID: f.TargetID, steps: steps, roller: in.Roller, outcome: f.Outcome, resumeIndex: f.Index, resumed: true}, nil
}
