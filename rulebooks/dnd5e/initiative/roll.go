// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package initiative

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

// Modifier is one sourced amount added to an initiative roll.
type Modifier struct {
	// Source is what adds it. Ref and Name are required.
	Source dnd5eEvents.RollSource
	// Amount is what it adds; negative subtracts; zero is kept and shown.
	Amount int
}

// RollOneInput asks for one member's initiative.
type RollOneInput struct {
	// MemberID is who rolls. Required.
	MemberID string
	// Modifiers are what the member adds, in the order they are recorded.
	Modifiers []Modifier
	// Roller rolls the d20. Required; no hidden randomness.
	Roller dice.Roller
}

// RollOneOutput is the roll and the complete record of how it was got.
type RollOneOutput struct {
	// Face is the d20 face that counts.
	Face int
	// Total is Face plus every Modifier.
	Total int
	// Calculation is the roll's record: the d20 pool first, sourced to the
	// initiative rule and the member, then one component per Modifier in
	// order. Its Total equals Total. Already validated.
	Calculation *dnd5eEvents.RollCalculation
}

// RollOne rolls one member's initiative and returns the full record of it:
// the d20, each modifier with the content it comes from, and the total.
//
// Errors: a nil input, a blank member, a nil roller, a modifier whose source
// has no ref, an invalid ref or no name, and a roller that fails. Every refusal
// except the roller's own failure happens before the die; a refused roll
// consumes no randomness.
func RollOne(ctx context.Context, in *RollOneInput) (*RollOneOutput, error) {
	if in == nil {
		return nil, errors.New("initiative roll requires an input")
	}
	if strings.TrimSpace(in.MemberID) == "" {
		return nil, errors.New("initiative roll requires a member")
	}
	if in.Roller == nil {
		return nil, errors.New("initiative roll requires a roller")
	}

	for i, m := range in.Modifiers {
		if err := checkModifierSource(m.Source); err != nil {
			return nil, fmt.Errorf("initiative roll for %q: modifier %d: %w", in.MemberID, i, err)
		}
	}

	ruleRef := *refs.Rules.Initiative()
	d20Source := dnd5eEvents.RollSource{Ref: &ruleRef, Name: "Initiative", SourceID: in.MemberID}
	d20, err := rolls.RollD20(ctx, &rolls.RollD20Input{Roller: in.Roller, Source: d20Source})
	if err != nil {
		return nil, fmt.Errorf("initiative roll for %q: %w", in.MemberID, err)
	}

	components := make([]dnd5eEvents.RollComponent, 0, 1+len(in.Modifiers))
	components = append(components, dnd5eEvents.RollComponent{Source: d20Source, Dice: d20.Trace})
	for _, m := range in.Modifiers {
		amount := m.Amount
		components = append(components, dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.CloneRollSource(m.Source),
			Modifier: &amount,
		})
	}
	calculation := dnd5eEvents.NewRollCalculation(components)
	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return nil, fmt.Errorf("initiative roll for %q: %w", in.MemberID, err)
	}
	return &RollOneOutput{Face: d20.Face, Total: calculation.Total, Calculation: calculation}, nil
}

// checkModifierSource refuses a source with no ref, an invalid ref or no name,
// so the refusal can come before the die is rolled.
func checkModifierSource(src dnd5eEvents.RollSource) error {
	if src.Ref == nil {
		return errors.New("source has no ref")
	}
	if err := src.Ref.IsValid(); err != nil {
		return fmt.Errorf("source ref: %w", err)
	}
	if strings.TrimSpace(src.Name) == "" {
		return errors.New("source has no name")
	}
	return nil
}
