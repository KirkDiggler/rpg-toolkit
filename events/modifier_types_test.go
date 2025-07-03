// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"strings"
	"testing"
)

func TestRawValue(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		source   string
		wantVal  int
		wantDesc string
	}{
		{
			name:     "positive value",
			value:    3,
			source:   "strength",
			wantVal:  3,
			wantDesc: "+3 (strength)",
		},
		{
			name:     "negative value",
			value:    -2,
			source:   "weakness",
			wantVal:  -2,
			wantDesc: "-2 (weakness)",
		},
		{
			name:     "zero value",
			value:    0,
			source:   "neutral",
			wantVal:  0,
			wantDesc: "+0 (neutral)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := NewRawValue(tt.value, tt.source)

			if got := rv.GetValue(); got != tt.wantVal {
				t.Errorf("GetValue() = %v, want %v", got, tt.wantVal)
			}

			if got := rv.GetDescription(); got != tt.wantDesc {
				t.Errorf("GetDescription() = %v, want %v", got, tt.wantDesc)
			}
		})
	}
}

func TestDiceValue(t *testing.T) {
	// Test basic dice value creation
	dv := NewDiceValue(1, 4, "bless")

	// Check notation
	if !strings.Contains(dv.notation, "d4") {
		t.Errorf("Expected notation to contain 'd4', got %s", dv.notation)
	}

	// Check that we have the right number of rolls
	if len(dv.rolls) != 1 {
		t.Errorf("Expected 1 roll, got %d", len(dv.rolls))
	}

	// Check that total equals sum of rolls
	expectedTotal := 0
	for _, r := range dv.rolls {
		expectedTotal += r
	}
	if dv.GetValue() != expectedTotal {
		t.Errorf("GetValue() = %v, want %v", dv.GetValue(), expectedTotal)
	}

	// Check description format
	desc := dv.GetDescription()
	if !strings.Contains(desc, "d4[") {
		t.Errorf("Expected description to contain 'd4[', got %s", desc)
	}
	if !strings.Contains(desc, "(bless)") {
		t.Errorf("Expected description to contain '(bless)', got %s", desc)
	}

	// Test multiple dice
	dv2 := NewDiceValue(2, 6, "sneak attack")
	if len(dv2.rolls) != 2 {
		t.Errorf("Expected 2 rolls, got %d", len(dv2.rolls))
	}
	if !strings.Contains(dv2.notation, "2d6") {
		t.Errorf("Expected notation to contain '2d6', got %s", dv2.notation)
	}
}

func TestIntValue(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		wantDesc string
	}{
		{"positive", 5, "+5"},
		{"negative", -3, "-3"},
		{"zero", 0, "+0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iv := IntValue(tt.value)

			if got := iv.GetValue(); got != tt.value {
				t.Errorf("GetValue() = %v, want %v", got, tt.value)
			}

			if got := iv.GetDescription(); got != tt.wantDesc {
				t.Errorf("GetDescription() = %v, want %v", got, tt.wantDesc)
			}
		})
	}
}

func TestNewIntModifier(t *testing.T) {
	mod := NewIntModifier("test", ModifierDamageBonus, 3, 100)

	if mod.Source() != "test" {
		t.Errorf("Source() = %v, want %v", mod.Source(), "test")
	}

	if mod.Type() != ModifierDamageBonus {
		t.Errorf("Type() = %v, want %v", mod.Type(), ModifierDamageBonus)
	}

	if mod.Priority() != 100 {
		t.Errorf("Priority() = %v, want %v", mod.Priority(), 100)
	}

	mv := mod.ModifierValue()
	if mv.GetValue() != 3 {
		t.Errorf("ModifierValue().GetValue() = %v, want %v", mv.GetValue(), 3)
	}
}

func TestModifierInterface(t *testing.T) {
	// Test that our modifiers implement the interface correctly
	modifiers := []Modifier{
		NewModifier("raw", ModifierAttackBonus, NewRawValue(2, "test"), 50),
		NewModifier("dice", ModifierDamageBonus, NewDiceValue(1, 6, "test"), 100),
		NewIntModifier("int", ModifierACBonus, 1, 150),
	}

	for i, mod := range modifiers {
		// Verify we can get the ModifierValue without type assertions
		mv := mod.ModifierValue()
		value := mv.GetValue()
		desc := mv.GetDescription()

		if value < 0 {
			t.Errorf("Modifier %d: unexpected negative value %d", i, value)
		}

		if desc == "" {
			t.Errorf("Modifier %d: empty description", i)
		}

		// Test deprecated Value() method for coverage
		if val := mod.Value(); val == nil {
			t.Errorf("Modifier %d: Value() returned nil", i)
		}
	}
}
