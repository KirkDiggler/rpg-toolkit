// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dice provides a simple dice rolling system.
// It implements events.ModifierValue by rolling dice when GetValue() is called.
package dice

import (
	"context"
	"fmt"
	"strings"
)

// Roll represents dice to be rolled as a modifier.
// It implements events.ModifierValue by rolling dice when GetValue() is called.
type Roll struct {
	count  int
	size   int
	roller Roller

	// Cache the result after first roll
	rolled bool
	result int
	rolls  []int
	err    error // Store any error that occurred during rolling
}

// NewRoll creates a new dice roll modifier using a new CryptoRoller.
// Returns an error if size <= 0.
func NewRoll(count, size int) (*Roll, error) {
	if size <= 0 {
		return nil, fmt.Errorf("dice: invalid die size %d", size)
	}
	return &Roll{
		count:  count,
		size:   size,
		roller: NewRoller(),
	}, nil
}

// NewRollWithRoller creates a new dice roll modifier with a specific roller.
// Useful for testing with MockRoller.
// Returns an error if size <= 0 or roller is nil.
func NewRollWithRoller(count, size int, roller Roller) (*Roll, error) {
	if size <= 0 {
		return nil, fmt.Errorf("dice: invalid die size %d", size)
	}
	if roller == nil {
		return nil, fmt.Errorf("dice: roller cannot be nil")
	}
	return &Roll{
		count:  count,
		size:   size,
		roller: roller,
	}, nil
}

// GetValue rolls the dice (if not already rolled) and returns the total.
// Subsequent calls return the same value.
// If an error occurred during rolling, returns 0.
func (r *Roll) GetValue() int {
	return r.GetValueWithContext(context.Background())
}

// GetValueWithContext rolls the dice (if not already rolled) and returns the total.
// Subsequent calls return the same value.
// If an error occurred during rolling, returns 0.
// The context parameter allows for cancellation during the rolling process.
func (r *Roll) GetValueWithContext(ctx context.Context) int {
	if !r.rolled {
		r.roll(ctx)
	}
	if r.err != nil {
		return 0
	}
	return r.result
}

// Err returns any error that occurred during rolling.
// This should be checked after calling GetValue() or GetDescription().
func (r *Roll) Err() error {
	return r.ErrWithContext(context.Background())
}

// ErrWithContext returns any error that occurred during rolling.
// This should be checked after calling GetValue() or GetDescription().
// The context parameter allows for cancellation during the rolling process.
func (r *Roll) ErrWithContext(ctx context.Context) error {
	if !r.rolled {
		r.roll(ctx)
	}
	return r.err
}

// GetDescription returns a description of the roll in the format:
// "+2d6[4,2]=6" for positive counts or "-2d6[4,2]=-6" for negative counts.
// If an error occurred during rolling, returns an error description.
func (r *Roll) GetDescription() string {
	return r.GetDescriptionWithContext(context.Background())
}

// GetDescriptionWithContext returns a description of the roll in the format:
// "+2d6[4,2]=6" for positive counts or "-2d6[4,2]=-6" for negative counts.
// If an error occurred during rolling, returns an error description.
// The context parameter allows for cancellation during the rolling process.
func (r *Roll) GetDescriptionWithContext(ctx context.Context) string {
	if !r.rolled {
		r.roll(ctx)
	}

	if r.err != nil {
		return fmt.Sprintf("ERROR: %v", r.err)
	}

	// Build notation
	var notation string
	switch r.count {
	case 1:
		notation = fmt.Sprintf("d%d", r.size)
	case -1:
		notation = fmt.Sprintf("-d%d", r.size)
	default:
		notation = fmt.Sprintf("%dd%d", r.count, r.size)
	}

	// Build roll list
	rollStrs := make([]string, len(r.rolls))
	for i, roll := range r.rolls {
		rollStrs[i] = fmt.Sprintf("%d", roll)
	}

	// Format based on positive/negative
	if r.count >= 0 {
		return fmt.Sprintf("+%s[%s]=%d", notation, strings.Join(rollStrs, ","), r.result)
	}
	return fmt.Sprintf("%s[%s]=%d", notation, strings.Join(rollStrs, ","), r.result)
}

// roll performs the actual dice rolling.
func (r *Roll) roll(ctx context.Context) {
	if r.count == 0 {
		r.rolled = true
		r.result = 0
		r.rolls = []int{}
		return
	}

	// Roll the dice
	absCount := r.count
	if absCount < 0 {
		absCount = -absCount
	}

	rolls, err := r.roller.RollN(ctx, absCount, r.size)
	if err != nil {
		r.err = err
		r.rolled = true
		return
	}
	r.rolls = rolls

	// Calculate total
	total := 0
	for _, roll := range r.rolls {
		total += roll
	}

	// Apply sign
	if r.count < 0 {
		r.result = -total
	} else {
		r.result = total
	}

	r.rolled = true
}

// Helper functions for common dice

// D4 creates a d4 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D4(count int) *Roll {
	roll, _ := NewRoll(count, 4)
	return roll
}

// D6 creates a d6 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D6(count int) *Roll {
	roll, _ := NewRoll(count, 6)
	return roll
}

// D8 creates a d8 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D8(count int) *Roll {
	roll, _ := NewRoll(count, 8)
	return roll
}

// D10 creates a d10 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D10(count int) *Roll {
	roll, _ := NewRoll(count, 10)
	return roll
}

// D12 creates a d12 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D12(count int) *Roll {
	roll, _ := NewRoll(count, 12)
	return roll
}

// D20 creates a d20 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D20(count int) *Roll {
	roll, _ := NewRoll(count, 20)
	return roll
}

// D100 creates a d100 roll modifier.
// Since this uses a valid die size, it will not return an error.
func D100(count int) *Roll {
	roll, _ := NewRoll(count, 100)
	return roll
}
