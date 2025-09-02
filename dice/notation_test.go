// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"testing"
)

func TestParseNotation(t *testing.T) {
	tests := []struct {
		name         string
		notation     string
		wantNotation string // Expected normalized notation
		wantErr      bool
	}{
		{
			name:         "simple d20",
			notation:     "d20",
			wantNotation: "d20",
		},
		{
			name:         "2d6",
			notation:     "2d6",
			wantNotation: "2d6",
		},
		{
			name:         "2d6+3",
			notation:     "2d6+3",
			wantNotation: "2d6+3",
		},
		{
			name:         "3d8-2",
			notation:     "3d8-2",
			wantNotation: "3d8-2",
		},
		{
			name:         "capital D",
			notation:     "2D6+3",
			wantNotation: "2d6+3",
		},
		{
			name:         "with spaces",
			notation:     "  2d6 + 3  ",
			wantNotation: "2d6+3",
		},
		{
			name:         "complex notation",
			notation:     "2d6+1d4+3",
			wantNotation: "2d6+d4+3",
		},
		{
			name:     "empty string",
			notation: "",
			wantErr:  true,
		},
		{
			name:     "invalid notation",
			notation: "invalid",
			wantErr:  true,
		},
		{
			name:     "zero die size",
			notation: "2d0",
			wantErr:  true,
		},
		{
			name:     "negative die size",
			notation: "2d-6",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := ParseNotation(tt.notation)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNotation(%q) error = %v, wantErr %v", tt.notation, err, tt.wantErr)
				return
			}
			if !tt.wantErr && pool.Notation() != tt.wantNotation {
				t.Errorf("ParseNotation(%q).Notation() = %q, want %q", tt.notation, pool.Notation(), tt.wantNotation)
			}
		})
	}
}

func TestParseComplexNotation(t *testing.T) {
	tests := []struct {
		name     string
		notation string
		wantAvg  float64
		wantMin  int
		wantMax  int
	}{
		{
			name:     "2d6+1d4+3",
			notation: "2d6+1d4+3",
			wantAvg:  12.5, // 7 + 2.5 + 3
			wantMin:  6,    // 2 + 1 + 3
			wantMax:  19,   // 12 + 4 + 3
		},
		{
			name:     "d20+d12+5",
			notation: "d20+d12+5",
			wantAvg:  22, // 10.5 + 6.5 + 5
			wantMin:  7,  // 1 + 1 + 5
			wantMax:  37, // 20 + 12 + 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := ParseNotation(tt.notation)
			if err != nil {
				t.Fatalf("ParseNotation(%q) error = %v", tt.notation, err)
			}

			if avg := pool.Average(); avg != tt.wantAvg {
				t.Errorf("Average() = %v, want %v", avg, tt.wantAvg)
			}
			if minValue := pool.Min(); minValue != tt.wantMin {
				t.Errorf("Min() = %v, want %v", minValue, tt.wantMin)
			}
			if maxValue := pool.Max(); maxValue != tt.wantMax {
				t.Errorf("Max() = %v, want %v", maxValue, tt.wantMax)
			}
		})
	}
}

func TestMustParseNotation(t *testing.T) {
	// Test valid notation
	pool := MustParseNotation("2d6+3")
	if pool.Notation() != "2d6+3" {
		t.Errorf("MustParseNotation(\"2d6+3\").Notation() = %q, want \"2d6+3\"", pool.Notation())
	}

	// Test panic on invalid notation
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustParseNotation with invalid notation did not panic")
		}
	}()
	MustParseNotation("invalid")
}
