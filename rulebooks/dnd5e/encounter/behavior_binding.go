// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// BehaviorVocabularyInput names the open fact IDs and clock names a producer
// needs while materializing definitions. These are declarations, not readings.
type BehaviorVocabularyInput struct {
	Facts  []FactID
	Clocks []string
}

// BehaviorVocabularyOutput supplies the encounter-owned beat/fact vocabulary.
// Binding may validate against it but never executes its world-dependent readers.
type BehaviorVocabularyOutput struct {
	Vocabulary machine.Vocabulary
}

// BehaviorVocabulary returns fresh maps for bind-time production. Nil input
// returns ErrNilInput; empty fact/clock names are refused. Fixed sound, enemy,
// opportunity and named deed words come from their existing encounter owners.
func BehaviorVocabulary(in *BehaviorVocabularyInput) (*BehaviorVocabularyOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	v := machineVocabulary()
	for _, deed := range WhenDeeds {
		v.Beats[machine.Beat(deedPrefix+deed)] = v.Beats[beatDeed]
	}
	for _, name := range in.Clocks {
		if name == "" {
			return nil, fmt.Errorf("behavior vocabulary: empty clock name")
		}
		v.Beats[clockBeat(name)] = machine.BeatSpec{Round: wakeRound}
	}
	for _, id := range in.Facts {
		if id == "" {
			return nil, fmt.Errorf("behavior vocabulary: empty fact ID")
		}
		addGuardFacts(v.Facts, &machine.Guard{Fact: &machine.FactLeaf{Name: MachineFact(string(id))}})
	}
	return &BehaviorVocabularyOutput{Vocabulary: v}, nil
}

// CheckBehaviorsInput is canonical materialized policy plus its physical-state
// bindings. Geometry and source names belong to the producing dungeon compiler.
type CheckBehaviorsInput struct {
	Behaviors      BehaviorInput
	BlockingStates []string
	Posts          map[string]Post
}

// CheckBehaviorsOutput identifies the resolved body instance, if any. No live
// progress or callback-bearing runtime crosses the producer boundary.
type CheckBehaviorsOutput struct{ BodyInstance string }

// CheckBehaviors validates the same configuration/physical state relationship as
// installation without constructing an encounter, rolling or reading facts.
// Nil input returns ErrNilInput; malformed composition/state names wrap ErrBadMember.
func CheckBehaviors(in *CheckBehaviorsInput) (*CheckBehaviorsOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	bound, err := newBehaviorRuntime(&in.Behaviors, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadMember, err)
	}
	if err := validateBlockingStates(bound.bodyRecipe(), in.BlockingStates); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadMember, err)
	}
	if err := validatePostStates(bound.bodyRecipe(), in.Posts); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadMember, err)
	}
	return &CheckBehaviorsOutput{BodyInstance: bound.input.BodyInstance}, nil
}
