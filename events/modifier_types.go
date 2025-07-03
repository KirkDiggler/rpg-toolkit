// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"fmt"
	"strings"
)

// RawValue represents a flat numeric modifier.
type RawValue struct {
	value  int
	source string
}

// NewRawValue creates a new flat modifier.
func NewRawValue(value int, source string) *RawValue {
	return &RawValue{value: value, source: source}
}

// GetValue returns the numeric value.
func (r *RawValue) GetValue() int {
	return r.value
}

// GetDescription returns the formatted description.
func (r *RawValue) GetDescription() string {
	return fmt.Sprintf("%+d (%s)", r.value, r.source)
}

// DiceValue represents a dice roll that was made at creation time.
type DiceValue struct {
	notation string // e.g., "d4", "2d6"
	rolls    []int  // actual rolls made
	total    int    // sum of rolls
	source   string // what created this modifier
}

// NewDiceValue creates a new dice modifier by rolling immediately.
// This is a placeholder - in the real implementation, it would use
// the dice package to perform actual rolls.
func NewDiceValue(count, size int, source string) *DiceValue {
	// TODO: Replace with actual dice rolling when dice package is available
	// For now, use placeholder values
	rolls := make([]int, count)
	total := 0
	for i := 0; i < count; i++ {
		// Placeholder: just use average roll
		roll := (size / 2) + 1
		rolls[i] = roll
		total += roll
	}

	notation := fmt.Sprintf("%dd%d", count, size)
	if count == 1 {
		notation = fmt.Sprintf("d%d", size)
	}

	return &DiceValue{
		notation: notation,
		rolls:    rolls,
		total:    total,
		source:   source,
	}
}

// GetValue returns the total of all dice rolled.
func (d *DiceValue) GetValue() int {
	return d.total
}

// GetDescription returns the formatted description with roll details.
func (d *DiceValue) GetDescription() string {
	rollStrs := make([]string, len(d.rolls))
	for i, r := range d.rolls {
		rollStrs[i] = fmt.Sprintf("%d", r)
	}
	rollStr := strings.Join(rollStrs, ",")
	return fmt.Sprintf("+%s[%s]=%d (%s)", d.notation, rollStr, d.total, d.source)
}

// IntValue wraps a simple integer as a ModifierValue.
// This is useful for simple numeric modifiers without a source.
type IntValue int

// GetValue returns the integer value.
func (i IntValue) GetValue() int {
	return int(i)
}

// GetDescription returns the formatted integer.
func (i IntValue) GetDescription() string {
	return fmt.Sprintf("%+d", i)
}

// NewIntModifier creates a modifier with a simple integer value.
// This is a convenience function for backwards compatibility.
func NewIntModifier(source, modType string, value int, priority int) *BasicModifier {
	return NewModifier(source, modType, IntValue(value), priority)
}
