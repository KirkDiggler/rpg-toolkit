package selectables

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// TestRoller implements dice.Roller with predictable results for testing
// Purpose: Provides deterministic dice rolls for reliable test outcomes
type TestRoller struct {
	values []int
	index  int
}

// NewTestRoller creates a new test roller with predefined values
// The roller will cycle through the provided values, returning to the beginning when exhausted
func NewTestRoller(values []int) dice.Roller {
	if len(values) == 0 {
		values = []int{1} // Default to 1 if no values provided
	}
	return &TestRoller{
		values: values,
		index:  0,
	}
}

// Roll returns the next predefined value, cycling through the values array
func (t *TestRoller) Roll(size int) (int, error) {
	if size <= 0 {
		return 0, fmt.Errorf("dice: invalid die size %d", size)
	}

	// Get the next value and cycle
	value := t.values[t.index]
	t.index = (t.index + 1) % len(t.values)

	// Ensure the value is within the valid range [1, size]
	if value > size {
		value = size
	}
	if value < 1 {
		value = 1
	}

	return value, nil
}

// RollN rolls multiple dice, each returning the next value in sequence
func (t *TestRoller) RollN(count, size int) ([]int, error) {
	if size <= 0 {
		return nil, fmt.Errorf("dice: invalid die size %d", size)
	}
	if count < 0 {
		return nil, fmt.Errorf("dice: invalid die count %d", count)
	}

	results := make([]int, count)
	for i := 0; i < count; i++ {
		roll, err := t.Roll(size)
		if err != nil {
			return nil, err
		}
		results[i] = roll
	}
	return results, nil
}
