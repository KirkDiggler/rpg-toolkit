// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"context"
	"fmt"
	"strings"
)

// Pool represents a reusable dice configuration that can be rolled multiple times.
// Unlike Roll, Pool doesn't cache results - each Roll() call produces fresh results.
type Pool struct {
	notation string // Original notation for display (e.g., "2d6+3")
	dice     []Spec // Individual dice groups
	modifier int    // Static modifier to add
}

// Spec represents a group of dice of the same size
type Spec struct {
	Count int // Number of dice
	Size  int // Die size (d6 = 6, d20 = 20)
}

// NewPool creates a new dice pool from components
func NewPool(dice []Spec, modifier int) *Pool {
	// Build notation string
	parts := make([]string, 0, len(dice)+1)
	for _, d := range dice {
		if d.Count == 1 {
			parts = append(parts, fmt.Sprintf("d%d", d.Size))
		} else if d.Count > 1 {
			parts = append(parts, fmt.Sprintf("%dd%d", d.Count, d.Size))
		}
	}

	notation := strings.Join(parts, "+")
	if modifier > 0 {
		notation = fmt.Sprintf("%s+%d", notation, modifier)
	} else if modifier < 0 {
		notation = fmt.Sprintf("%s%d", notation, modifier)
	}

	return &Pool{
		notation: notation,
		dice:     dice,
		modifier: modifier,
	}
}

// SimplePool creates a pool for a single dice type (e.g., 2d6+3)
func SimplePool(count, size, modifier int) *Pool {
	return NewPool([]Spec{{Count: count, Size: size}}, modifier)
}

// Notation returns the dice notation string (e.g., "2d6+3")
func (p *Pool) Notation() string {
	return p.notation
}

// Roll performs a fresh roll of the pool using the provided roller
func (p *Pool) Roll(roller Roller) *Result {
	return p.RollContext(context.Background(), roller)
}

// RollContext performs a fresh roll with context support
func (p *Pool) RollContext(ctx context.Context, roller Roller) *Result {
	if roller == nil {
		roller = NewRoller()
	}

	result := &Result{
		pool:     p,
		rolls:    make([][]int, len(p.dice)),
		modifier: p.modifier,
	}

	// Roll each dice group
	for i, spec := range p.dice {
		groupRolls, err := roller.RollN(ctx, spec.Count, spec.Size)
		if err != nil {
			result.err = err
			return result
		}
		result.rolls[i] = groupRolls
	}

	// Calculate total
	result.total = p.modifier
	for _, group := range result.rolls {
		for _, roll := range group {
			result.total += roll
		}
	}

	return result
}

// Average returns the average expected value of the pool
func (p *Pool) Average() float64 {
	avg := float64(p.modifier)
	for _, spec := range p.dice {
		// Average of a die is (1 + size) / 2 * count
		avg += float64(spec.Count) * (float64(spec.Size) + 1) / 2
	}
	return avg
}

// Min returns the minimum possible roll
func (p *Pool) Min() int {
	minValue := p.modifier
	for _, spec := range p.dice {
		minValue += spec.Count // Each die minimum is 1
	}
	return minValue
}

// Max returns the maximum possible roll
func (p *Pool) Max() int {
	maxValue := p.modifier
	for _, spec := range p.dice {
		maxValue += spec.Count * spec.Size
	}
	return maxValue
}
