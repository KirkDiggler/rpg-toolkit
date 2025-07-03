// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"testing"
)

func TestMockRoller_Roll(t *testing.T) {
	tests := []struct {
		name     string
		results  []int
		rolls    int
		size     int
		expected []int
	}{
		{
			name:     "single result",
			results:  []int{4},
			rolls:    3,
			size:     6,
			expected: []int{4, 4, 4},
		},
		{
			name:     "multiple results cycling",
			results:  []int{1, 2, 3},
			rolls:    5,
			size:     6,
			expected: []int{1, 2, 3, 1, 2},
		},
		{
			name:     "exact match",
			results:  []int{6, 5, 4, 3, 2, 1},
			rolls:    6,
			size:     6,
			expected: []int{6, 5, 4, 3, 2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockRoller(tt.results...)

			for i := 0; i < tt.rolls; i++ {
				result := mock.Roll(tt.size)
				if result != tt.expected[i] {
					t.Errorf("Roll %d: got %d, want %d", i, result, tt.expected[i])
				}
			}
		})
	}
}

func TestMockRoller_RollN(t *testing.T) {
	mock := NewMockRoller(6, 5, 4, 3, 2, 1)

	results := mock.RollN(4, 6)
	expected := []int{6, 5, 4, 3}

	if len(results) != len(expected) {
		t.Fatalf("RollN(4, 6) returned %d results, want %d", len(results), len(expected))
	}

	for i, result := range results {
		if result != expected[i] {
			t.Errorf("RollN[%d] = %d, want %d", i, result, expected[i])
		}
	}
}

func TestMockRoller_Reset(t *testing.T) {
	mock := NewMockRoller(1, 2, 3)

	// Roll through the sequence
	if got := mock.Roll(6); got != 1 {
		t.Errorf("First roll = %d, want 1", got)
	}
	if got := mock.Roll(6); got != 2 {
		t.Errorf("Second roll = %d, want 2", got)
	}

	// Reset and verify we start over
	mock.Reset()
	if got := mock.Roll(6); got != 1 {
		t.Errorf("After reset roll = %d, want 1", got)
	}
}

func TestMockRoller_Panics(t *testing.T) {
	tests := []struct {
		name      string
		fn        func()
		wantPanic string
	}{
		{
			name:      "NewMockRoller with no results",
			fn:        func() { NewMockRoller() },
			wantPanic: "dice: MockRoller requires at least one result",
		},
		{
			name: "Roll with invalid result for die size",
			fn: func() {
				mock := NewMockRoller(7)
				mock.Roll(6) // 7 is invalid for d6
			},
			wantPanic: "dice: mock result 7 is invalid for d6",
		},
		{
			name: "Roll with zero result",
			fn: func() {
				mock := NewMockRoller(0)
				mock.Roll(6)
			},
			wantPanic: "dice: mock result 0 is invalid for d6",
		},
		{
			name: "Roll with zero size",
			fn: func() {
				mock := NewMockRoller(1)
				mock.Roll(0)
			},
			wantPanic: "dice: invalid die size 0",
		},
		{
			name: "RollN with negative count",
			fn: func() {
				mock := NewMockRoller(1)
				mock.RollN(-1, 6)
			},
			wantPanic: "dice: invalid die count -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic but didn't get one")
				} else if r != tt.wantPanic {
					t.Errorf("Got panic %v, want %v", r, tt.wantPanic)
				}
			}()

			tt.fn()
		})
	}
}