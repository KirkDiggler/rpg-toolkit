// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package healing resolves sourced healing arithmetic. Callers own payment and
// delivery; recipients own HP mutation. No spell or event bus is required.
package healing

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Context describes the healing source to contributing features. SpellLevel is
// zero for a cantrip and ignored unless Spell is true.
type Context struct {
	Spell      bool
	SpellLevel int
}

// Modifier is a fixed contribution with independently attributed provenance.
type Modifier struct {
	Source events.RollSource
	Amount int
}

// Declaration is one homogeneous pool and its sourced fixed contributions.
type Declaration struct {
	Dice      string
	Modifiers []Modifier
}

var notation = regexp.MustCompile(`^([1-9][0-9]*)d([1-9][0-9]*)$`)

func (d Declaration) pool() (int, int, error) {
	parts := notation.FindStringSubmatch(d.Dice)
	if parts == nil {
		return 0, 0, fmt.Errorf("healing dice must be an unsigned homogeneous pool")
	}
	count, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	size, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, err
	}
	return count, size, nil
}

// Validate checks the declaration without rolling.
func (d Declaration) Validate() error {
	if _, _, err := d.pool(); err != nil {
		return err
	}
	for _, modifier := range d.Modifiers {
		value := modifier.Amount
		if err := events.ValidateRollCalculation(&events.RollCalculation{
			Components: []events.RollComponent{{Source: modifier.Source, Modifier: &value}}, Total: value,
		}); err != nil {
			return fmt.Errorf("healing modifier: %w", err)
		}
	}
	return nil
}

// Clone detaches mutable contributions and their source references.
func (d Declaration) Clone() Declaration {
	clone := d
	clone.Modifiers = append([]Modifier(nil), d.Modifiers...)
	for i := range clone.Modifiers {
		if ref := clone.Modifiers[i].Source.Ref; ref != nil {
			copied := *ref
			clone.Modifiers[i].Source.Ref = &copied
		}
	}
	return clone
}

// Resolve rolls a validated declaration and preserves every contribution.
// A negative sum restores zero HP; its adjustment is explicit in the trace.
func Resolve(ctx context.Context, declaration Declaration, source events.RollSource, roller dice.Roller) (*events.RollCalculation, error) {
	if err := declaration.Validate(); err != nil {
		return nil, err
	}
	if roller == nil {
		return nil, fmt.Errorf("healing requires a roller")
	}
	zero := 0
	if err := events.ValidateRollCalculation(&events.RollCalculation{Components: []events.RollComponent{{Source: source, Modifier: &zero}}}); err != nil {
		return nil, err
	}
	count, size, _ := declaration.pool()
	faces, err := roller.RollN(ctx, count, size)
	if err != nil {
		return nil, err
	}
	if len(faces) != count {
		return nil, fmt.Errorf("healing roller returned %d faces, want %d", len(faces), count)
	}
	total := 0
	for _, face := range faces {
		total += face
	}
	calculation := &events.RollCalculation{Total: total, Components: []events.RollComponent{{
		Source: source, Dice: &events.DiceTrace{Notation: declaration.Dice, DieSize: size,
			OriginalRolls: append([]int(nil), faces...), FinalRolls: append([]int(nil), faces...), Subtotal: total},
	}}}
	for _, modifier := range declaration.Modifiers {
		value := modifier.Amount
		calculation.Components = append(calculation.Components, events.RollComponent{Source: modifier.Source, Modifier: &value})
		calculation.Total += value
	}
	if calculation.Total < 0 {
		adjustment := -calculation.Total
		floorSource := source
		floorSource.Label = "Healing cannot restore negative HP"
		calculation.Components = append(calculation.Components, events.RollComponent{Source: floorSource, Modifier: &adjustment})
		calculation.Total = 0
	}
	if err := events.ValidateRollCalculation(calculation); err != nil {
		return nil, err
	}
	return events.CloneRollCalculation(calculation), nil
}
