// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import "fmt"

// MockRoller implements Roller with predetermined results for testing.
type MockRoller struct {
	results []int
	index   int
}

// NewMockRoller creates a MockRoller with the given predetermined results.
// Results are used in order, cycling back to the beginning when exhausted.
func NewMockRoller(results ...int) *MockRoller {
	if len(results) == 0 {
		panic("dice: MockRoller requires at least one result")
	}
	return &MockRoller{
		results: results,
		index:   0,
	}
}

// Roll returns the next predetermined result.
func (m *MockRoller) Roll(size int) int {
	if size <= 0 {
		panic(fmt.Sprintf("dice: invalid die size %d", size))
	}

	result := m.results[m.index]
	m.index = (m.index + 1) % len(m.results)

	// Validate the result is valid for the die size
	if result < 1 || result > size {
		panic(fmt.Sprintf("dice: mock result %d is invalid for d%d", result, size))
	}

	return result
}

// RollN returns multiple predetermined results.
func (m *MockRoller) RollN(count, size int) []int {
	if size <= 0 {
		panic(fmt.Sprintf("dice: invalid die size %d", size))
	}
	if count < 0 {
		panic(fmt.Sprintf("dice: invalid die count %d", count))
	}

	results := make([]int, count)
	for i := 0; i < count; i++ {
		results[i] = m.Roll(size)
	}
	return results
}

// Reset resets the mock roller to start from the beginning of results.
func (m *MockRoller) Reset() {
	m.index = 0
}