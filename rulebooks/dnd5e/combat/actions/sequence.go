// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// minimumSequenceSteps is the shortest script that is a script. One step is
// the component action itself wearing a second name.
const minimumSequenceSteps = 2

// SequenceProfile declares a SCRIPT: component actions one actor performs in
// order, under a single declaration. The SRD's Multiattack is the whole of
// what it is for today.
//
// # The definition owns the script; the machine only runs it
//
// "Two scimitar attacks, the second at disadvantage" is a fact about a goblin
// boss, not a fact about attacking. It is written here, as data, so the
// machine that reads it never learns the word "multiattack" — which is the
// extension rule resolution/ARCHITECTURE.md states: dispatch by profile type,
// never by content identity.
//
// # It is not an economy
//
// A sequence is ONE action that lands several blows, and that is why it is a
// profile and not a ledger. A character's Extra Attack is the other shape
// entirely — a capacity that lets its owner take the Attack action's swing
// more than once — and nothing here is a step toward giving monsters one.
//
// # What it replaces
//
// The runtime multiattack removed by rpg-toolkit#1198 chained sub-actions by
// STRING NAME and ignored whatever a sub-action reported back. Steps here
// name a [core.Ref], and [ResolveSequence] refuses a step that names nothing
// the actor carries rather than skipping it.
type SequenceProfile struct {
	// Steps are the component actions in the order they are performed. At
	// least [minimumSequenceSteps] of them.
	Steps []SequenceStep `json:"steps"`
}

// SequenceStep names one component action and whatever applies to that step
// alone.
type SequenceStep struct {
	// Action is the component action's ref. It names an action the SAME
	// actor already carries — this is a script over a repertoire, not a
	// place to author a new attack — and the component's own profile is what
	// the step resolves through.
	Action core.Ref `json:"action"`

	// Disadvantage is WHY this step is rolled at disadvantage, and the
	// reason IS the declaration: present means imposed with these words,
	// absent means not imposed.
	//
	// NO BOOLEAN BESIDE IT. A flag and a reason can disagree, and the pair
	// that survives the disagreement is always the one the log cannot
	// explain — an anonymous die (rpg-project#462). One field cannot say
	// "disadvantage from nowhere".
	//
	// The words travel to the d20's keep record as the imposing source's
	// name, so "the second attack of a multiattack" is what a player reads
	// when they ask why the low die counted.
	Disadvantage string `json:"disadvantage,omitempty"`
}

// Validate reports whether the sequence declares enough steps and each step
// names a valid ref under an honest per-step declaration.
//
// # What it cannot check, and does not pretend to
//
// Whether a step's component EXISTS, and whether that component is itself a
// sequence. Neither question is answerable from one definition: the components
// live on the actor and this type has never seen the actor. [ResolveSequence]
// asks both, because it is the first place the repertoire is in hand.
func (p SequenceProfile) Validate() error {
	if len(p.Steps) < minimumSequenceSteps {
		return fmt.Errorf("sequence must declare at least %d steps, got %d",
			minimumSequenceSteps, len(p.Steps))
	}
	for index, step := range p.Steps {
		if err := step.Action.IsValid(); err != nil {
			return fmt.Errorf("sequence step %d action ref is invalid: %w", index, err)
		}
		if step.Disadvantage != "" && strings.TrimSpace(step.Disadvantage) == "" {
			return fmt.Errorf("sequence step %d imposes disadvantage with a blank reason", index)
		}
	}
	return nil
}

// Clone returns a deep copy whose step list does not alias the original's.
func (p SequenceProfile) Clone() SequenceProfile {
	clone := p
	if p.Steps != nil {
		clone.Steps = make([]SequenceStep, len(p.Steps))
		copy(clone.Steps, p.Steps)
	}
	return clone
}

// SequenceComponent is one step matched to the component definition it named.
type SequenceComponent struct {
	// Step is what the sequence declared: the ref, and the per-step rules.
	Step SequenceStep

	// Definition is the component action itself, taken from the actor's own
	// repertoire. A CLONE: a resolved script must not alias the definitions
	// it was read off, for [Definition.Clone]'s own reason.
	Definition Definition
}

// ResolveSequence matches a sequence definition's steps against the actor's
// own action definitions, in declared order.
//
// # This is where a sequence is checked for real
//
// [Definition.Validate] sees one definition and refuses what is malformed on
// its face. The two checks that matter most need the repertoire, and they are
// here:
//
//   - a step naming an action the actor does not carry is REFUSED, rather
//     than skipped the way the removed runtime multiattack skipped a
//     sub-action it could not find;
//   - a step naming another SEQUENCE is REFUSED, because a script of scripts
//     is a recursion nothing in this model bounds.
//
// Repertoire is the actor's whole action list, the sequence definition
// included; a sequence never finds itself, because a step that names its own
// definition is already refused by [Definition.Validate].
func ResolveSequence(definition Definition, repertoire []Definition) ([]SequenceComponent, error) {
	if definition.Sequence == nil {
		return nil, fmt.Errorf("action %s declares no sequence", definition.Ref.String())
	}
	if err := definition.Validate(); err != nil {
		return nil, fmt.Errorf("sequence %s is invalid: %w", definition.Ref.String(), err)
	}

	components := make([]SequenceComponent, 0, len(definition.Sequence.Steps))
	for index, step := range definition.Sequence.Steps {
		component, found := findDefinition(repertoire, step.Action)
		if !found {
			return nil, fmt.Errorf("sequence %s step %d names %s, which the actor does not carry",
				definition.Ref.String(), index, step.Action.String())
		}
		if component.Sequence != nil {
			return nil, fmt.Errorf("sequence %s step %d names %s, which is itself a sequence",
				definition.Ref.String(), index, step.Action.String())
		}
		components = append(components, SequenceComponent{Step: step, Definition: component.Clone()})
	}

	return components, nil
}

// findDefinition looks one ref up in a repertoire. Linear because an actor's
// action list is a stat block's worth of entries, and a map built per lookup
// would cost more than the scan it replaced.
func findDefinition(repertoire []Definition, ref core.Ref) (Definition, bool) {
	for _, candidate := range repertoire {
		if candidate.Ref == ref {
			return candidate, true
		}
	}
	return Definition{}, false
}
