package spells

import (
	"testing"
)

func TestSpellData_GetData(t *testing.T) {
	// Test getting data for a known spell
	fireData := GetData(FireBolt)
	if fireData == nil {
		t.Fatal("Expected FireBolt data, got nil")
	}

	if fireData.ID != FireBolt {
		t.Errorf("Expected ID %s, got %s", FireBolt, fireData.ID)
	}

	if fireData.Name != "Fire Bolt" {
		t.Errorf("Expected name 'Fire Bolt', got %s", fireData.Name)
	}

	if fireData.Level != 0 {
		t.Errorf("Expected level 0, got %d", fireData.Level)
	}

	if fireData.Description == "" {
		t.Error("Expected non-empty description")
	}
}

func TestSpellData_GetSpellsByLevel(t *testing.T) {
	// Test cantrips (level 0)
	cantrips := GetSpellsByLevel(0)
	if len(cantrips) == 0 {
		t.Error("Expected some cantrips, got none")
	}

	// Verify all returned spells are level 0
	for _, spell := range cantrips {
		if spell.Level != 0 {
			t.Errorf("Expected level 0 cantrip, got level %d spell %s", spell.Level, spell.Name)
		}
	}

	// Test level 1 spells
	level1 := GetSpellsByLevel(1)
	if len(level1) == 0 {
		t.Error("Expected some level 1 spells, got none")
	}

	// Verify all returned spells are level 1
	for _, spell := range level1 {
		if spell.Level != 1 {
			t.Errorf("Expected level 1 spell, got level %d spell %s", spell.Level, spell.Name)
		}
	}

	// Test level that doesn't exist
	level9 := GetSpellsByLevel(9)
	if len(level9) != 0 {
		t.Errorf("Expected no level 9 spells, got %d", len(level9))
	}
}

func TestSpellData_KnownSpells(t *testing.T) {
	testCases := []struct {
		spell        Spell
		expectedLvl  int
		expectedName string
	}{
		{FireBolt, 0, "Fire Bolt"},
		{MagicMissile, 1, "Magic Missile"},
		{Fireball, 3, "Fireball"},
		{Shield, 1, "Shield"},
		{Guidance, 0, "Guidance"},
	}

	for _, tc := range testCases {
		data := GetData(tc.spell)
		if data == nil {
			t.Errorf("Spell %s not found in SpellData", tc.spell)
			continue
		}

		if data.Level != tc.expectedLvl {
			t.Errorf("Spell %s: expected level %d, got %d", tc.spell, tc.expectedLvl, data.Level)
		}

		if data.Name != tc.expectedName {
			t.Errorf("Spell %s: expected name %s, got %s", tc.spell, tc.expectedName, data.Name)
		}

		if data.ID != tc.spell {
			t.Errorf("Spell %s: expected ID %s, got %s", tc.spell, tc.spell, data.ID)
		}
	}
}
