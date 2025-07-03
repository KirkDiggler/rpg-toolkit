// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
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
}

// NewRoll creates a new dice roll modifier using the DefaultRoller.
func NewRoll(count, size int) *Roll {
	return &Roll{
		count:  count,
		size:   size,
		roller: DefaultRoller,
	}
}

// NewRollWithRoller creates a new dice roll modifier with a specific roller.
// Useful for testing with MockRoller.
func NewRollWithRoller(count, size int, roller Roller) *Roll {
	return &Roll{
		count:  count,
		size:   size,
		roller: roller,
	}
}

// GetValue rolls the dice (if not already rolled) and returns the total.
// Subsequent calls return the same value.
func (r *Roll) GetValue() int {
	if !r.rolled {
		r.roll()
	}
	return r.result
}

// GetDescription returns a description of the roll in the format:
// "+2d6[4,2]=6" for positive counts or "-2d6[4,2]=-6" for negative counts.
func (r *Roll) GetDescription() string {
	if !r.rolled {
		r.roll()
	}

	// Build notation
	var notation string
	if r.count == 1 {
		notation = fmt.Sprintf("d%d", r.size)
	} else if r.count == -1 {
		notation = fmt.Sprintf("-d%d", r.size)
	} else {
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
func (r *Roll) roll() {
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

	r.rolls = r.roller.RollN(absCount, r.size)

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
func D4(count int) *Roll {
	return NewRoll(count, 4)
}

// D6 creates a d6 roll modifier.
func D6(count int) *Roll {
	return NewRoll(count, 6)
}

// D8 creates a d8 roll modifier.
func D8(count int) *Roll {
	return NewRoll(count, 8)
}

// D10 creates a d10 roll modifier.
func D10(count int) *Roll {
	return NewRoll(count, 10)
}

// D12 creates a d12 roll modifier.
func D12(count int) *Roll {
	return NewRoll(count, 12)
}

// D20 creates a d20 roll modifier.
func D20(count int) *Roll {
	return NewRoll(count, 20)
}

// D100 creates a d100 roll modifier.
func D100(count int) *Roll {
	return NewRoll(count, 100)
}