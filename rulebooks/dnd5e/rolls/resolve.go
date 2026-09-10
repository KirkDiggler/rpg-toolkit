// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package rolls resolves already-selected D&D 5e dice contributions into
// sourced physical roll components. Condition selection and domain settlement
// remain with the operation that owns the roll.
package rolls

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

var contributionNotation = regexp.MustCompile(`^([1-9][0-9]*)?[dD]([1-9][0-9]*)$`)

// ResolveContributionsInput supplies the required operation-owned roller and
// already-selected unresolved contribution descriptions. It contains no
// conditions or stacking policy; the recipient owner selects descriptions
// before this helper runs.
type ResolveContributionsInput struct {
	Roller        dice.Roller
	Contributions []dnd5eEvents.DiceContribution
}

// ResolveContributionsOutput contains one resolved component per input
// contribution, in the same order.
type ResolveContributionsOutput struct {
	Components []dnd5eEvents.RollComponent
}

type checkedContribution struct {
	description dnd5eEvents.DiceContribution
	count       int
	size        int
}

// ValidateContributions validates a complete selected description list without
// rolling it. Operation owners use it before starting any related RNG.
func ValidateContributions(contributions []dnd5eEvents.DiceContribution) error {
	_, err := checkContributions(contributions)
	return err
}

// ResolveContributions validates every description before rolling any of them,
// then rolls each contribution exactly once and preserves positive physical
// faces separately from its subtractive operator.
func ResolveContributions(
	ctx context.Context,
	input *ResolveContributionsInput,
) (*ResolveContributionsOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("resolve contributions input is required")
	}
	if input.Roller == nil {
		return nil, fmt.Errorf("resolve contributions roller is required")
	}

	checked, err := checkContributions(input.Contributions)
	if err != nil {
		return nil, err
	}

	roller := input.Roller
	output := &ResolveContributionsOutput{}
	if input.Contributions != nil {
		output.Components = make([]dnd5eEvents.RollComponent, 0, len(checked))
	}
	for i, contribution := range checked {
		faces, err := roller.RollN(ctx, contribution.count, contribution.size)
		if err != nil {
			return nil, fmt.Errorf("roll contribution %d: %w", i, err)
		}
		if len(faces) != contribution.count {
			return nil, fmt.Errorf("roll contribution %d returned %d faces, want %d", i, len(faces), contribution.count)
		}
		subtotal := 0
		for faceIndex, face := range faces {
			if face < 1 || face > contribution.size {
				return nil, fmt.Errorf("roll contribution %d face %d is %d outside 1..%d", i, faceIndex, face, contribution.size)
			}
			subtotal += face
		}

		original := append([]int(nil), faces...)
		final := append([]int(nil), faces...)
		output.Components = append(output.Components, dnd5eEvents.RollComponent{
			Source: cloneSource(contribution.description.Source),
			Dice: &dnd5eEvents.DiceTrace{
				Notation:      dice.SimplePool(contribution.count, contribution.size, 0).Notation(),
				DieSize:       contribution.size,
				OriginalRolls: original,
				FinalRolls:    final,
				Subtotal:      subtotal,
			},
			SubtractDice: contribution.description.Subtract,
		})
	}
	return output, nil
}

func checkContributions(
	contributions []dnd5eEvents.DiceContribution,
) ([]checkedContribution, error) {
	checked := make([]checkedContribution, len(contributions))
	for i, contribution := range contributions {
		parsed, err := validateContribution(contribution)
		if err != nil {
			return nil, fmt.Errorf("contribution %d: %w", i, err)
		}
		checked[i] = parsed
	}
	return checked, nil
}

func validateContribution(contribution dnd5eEvents.DiceContribution) (checkedContribution, error) {
	if contribution.Source.Ref == nil {
		return checkedContribution{}, fmt.Errorf("source ref is required")
	}
	if err := contribution.Source.Ref.IsValid(); err != nil {
		return checkedContribution{}, fmt.Errorf("source ref is invalid: %w", err)
	}
	if strings.TrimSpace(contribution.Source.Name) == "" {
		return checkedContribution{}, fmt.Errorf("source name is required")
	}
	if strings.TrimSpace(contribution.Source.SourceID) == "" {
		return checkedContribution{}, fmt.Errorf("source id is required for a dice contribution")
	}

	matches := contributionNotation.FindStringSubmatch(contribution.Dice)
	if matches == nil {
		return checkedContribution{}, fmt.Errorf("dice notation %q must be unsigned homogeneous dice", contribution.Dice)
	}
	count := 1
	var err error
	if matches[1] != "" {
		count, err = strconv.Atoi(matches[1])
		if err != nil {
			return checkedContribution{}, fmt.Errorf("dice count is invalid: %w", err)
		}
	}
	size, err := strconv.Atoi(matches[2])
	if err != nil {
		return checkedContribution{}, fmt.Errorf("die size is invalid: %w", err)
	}
	return checkedContribution{description: contribution, count: count, size: size}, nil
}

func cloneSource(source dnd5eEvents.RollSource) dnd5eEvents.RollSource {
	clone := source
	if source.Ref != nil {
		ref := *source.Ref
		clone.Ref = &ref
	}
	return clone
}
