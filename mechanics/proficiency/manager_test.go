// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// MockEntity implements core.Entity for testing.
type MockEntity struct {
	id    string
	level int
}

func (e *MockEntity) GetID() string   { return e.id }
func (e *MockEntity) GetType() string { return "character" }

// MockLevelProvider implements LevelProvider for testing.
type MockLevelProvider struct {
	levels map[string]int
}

func (p *MockLevelProvider) GetLevel(entity core.Entity) int {
	if entity == nil {
		return 1
	}
	if level, ok := p.levels[entity.GetID()]; ok {
		return level
	}
	return 1
}

func TestProficiencyBonus(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		expected int
	}{
		{"Level 1", 1, 2},
		{"Level 4", 4, 2},
		{"Level 5", 5, 3},
		{"Level 8", 8, 3},
		{"Level 9", 9, 4},
		{"Level 12", 12, 4},
		{"Level 13", 13, 5},
		{"Level 16", 16, 5},
		{"Level 17", 17, 6},
		{"Level 20", 20, 6},
	}

	storage := proficiency.NewMemoryStorage()
	levelProvider := &MockLevelProvider{levels: make(map[string]int)}
	manager := proficiency.NewManager(storage, levelProvider)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &MockEntity{id: "test", level: tt.level}
			levelProvider.levels[entity.GetID()] = tt.level

			bonus := manager.GetProficiencyBonus(entity)
			if bonus != tt.expected {
				t.Errorf("GetProficiencyBonus() = %d, want %d", bonus, tt.expected)
			}
		})
	}
}

func TestWeaponProficiency(t *testing.T) {
	storage := proficiency.NewMemoryStorage()
	levelProvider := &MockLevelProvider{levels: map[string]int{"rogue": 3}}
	manager := proficiency.NewManager(storage, levelProvider)

	rogue := &MockEntity{id: "rogue", level: 3}

	// Initially, no proficiency
	if manager.HasWeaponProficiency(rogue, "shortsword") {
		t.Error("Expected no shortsword proficiency initially")
	}

	// Add specific weapon proficiency
	shortswordProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		Type:     proficiency.ProficiencyTypeWeapon,
		Key:      "shortsword",
		Name:     "Shortsword",
		Category: "",
	})
	manager.AddProficiency(rogue, shortswordProf)

	// Check specific proficiency
	if !manager.HasWeaponProficiency(rogue, "shortsword") {
		t.Error("Expected shortsword proficiency after adding")
	}

	// Check attack bonus with proficiency
	bonus := manager.GetWeaponAttackBonus(rogue, "shortsword")
	if bonus != 2 {
		t.Errorf("Expected attack bonus of 2, got %d", bonus)
	}

	// Check no bonus without proficiency
	bonus = manager.GetWeaponAttackBonus(rogue, "greatsword")
	if bonus != 0 {
		t.Errorf("Expected no attack bonus for greatsword, got %d", bonus)
	}
}

func TestCategoryProficiency(t *testing.T) {
	storage := proficiency.NewMemoryStorage()
	levelProvider := &MockLevelProvider{levels: map[string]int{"fighter": 5}}
	manager := proficiency.NewManager(storage, levelProvider)

	fighter := &MockEntity{id: "fighter", level: 5}

	// Add simple weapons proficiency
	simpleWeaponsProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		Type:     proficiency.ProficiencyTypeWeapon,
		Key:      "simple-weapons",
		Name:     "Simple Weapons",
		Category: "simple-weapons",
	})
	manager.AddProficiency(fighter, simpleWeaponsProf)

	// Check proficiency with simple weapons
	simpleWeapons := []string{"club", "dagger", "quarterstaff", "spear"}
	for _, weapon := range simpleWeapons {
		if !manager.HasWeaponProficiency(fighter, weapon) {
			t.Errorf("Expected proficiency with %s from simple-weapons category", weapon)
		}

		// Level 5 fighter should get +3 proficiency bonus
		bonus := manager.GetWeaponAttackBonus(fighter, weapon)
		if bonus != 3 {
			t.Errorf("Expected attack bonus of 3 for %s, got %d", weapon, bonus)
		}
	}

	// Check no proficiency with martial weapons
	if manager.HasWeaponProficiency(fighter, "greatsword") {
		t.Error("Should not have proficiency with greatsword (martial weapon)")
	}
}

func TestSkillProficiency(t *testing.T) {
	storage := proficiency.NewMemoryStorage()
	levelProvider := &MockLevelProvider{levels: map[string]int{"bard": 7}}
	manager := proficiency.NewManager(storage, levelProvider)

	bard := &MockEntity{id: "bard", level: 7}

	// Add performance skill proficiency
	performanceProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		Type:     proficiency.ProficiencyTypeSkill,
		Key:      "performance",
		Name:     "Performance",
		Category: "",
	})
	manager.AddProficiency(bard, performanceProf)

	// Check skill proficiency
	if !manager.HasSkillProficiency(bard, "performance") {
		t.Error("Expected performance skill proficiency")
	}

	// Level 7 bard should get +3 proficiency bonus
	bonus := manager.GetSkillBonus(bard, "performance")
	if bonus != 3 {
		t.Errorf("Expected skill bonus of 3, got %d", bonus)
	}

	// Check no bonus for non-proficient skill
	bonus = manager.GetSkillBonus(bard, "athletics")
	if bonus != 0 {
		t.Errorf("Expected no skill bonus for athletics, got %d", bonus)
	}
}

func TestSavingThrowProficiency(t *testing.T) {
	storage := proficiency.NewMemoryStorage()
	levelProvider := &MockLevelProvider{levels: map[string]int{"paladin": 10}}
	manager := proficiency.NewManager(storage, levelProvider)

	paladin := &MockEntity{id: "paladin", level: 10}

	// Add wisdom save proficiency
	wisdomSaveProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		Type:     proficiency.ProficiencyTypeSave,
		Key:      "wisdom",
		Name:     "Wisdom",
		Category: "",
	})
	manager.AddProficiency(paladin, wisdomSaveProf)

	// Check save proficiency
	if !manager.HasProficiency(paladin, proficiency.ProficiencyTypeSave, "wisdom") {
		t.Error("Expected wisdom save proficiency")
	}

	// Level 10 paladin should get +4 proficiency bonus
	bonus := manager.GetSaveBonus(paladin, "wisdom")
	if bonus != 4 {
		t.Errorf("Expected save bonus of 4, got %d", bonus)
	}
}

func TestGetAllProficiencies(t *testing.T) {
	storage := proficiency.NewMemoryStorage()
	manager := proficiency.NewManager(storage, nil)

	entity := &MockEntity{id: "test", level: 1}

	// Add multiple proficiencies
	profs := []proficiency.Proficiency{
		proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			Type: proficiency.ProficiencyTypeWeapon,
			Key:  "longsword",
			Name: "Longsword",
		}),
		proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			Type: proficiency.ProficiencyTypeSkill,
			Key:  "athletics",
			Name: "Athletics",
		}),
		proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			Type: proficiency.ProficiencyTypeSave,
			Key:  "strength",
			Name: "Strength",
		}),
	}

	for _, prof := range profs {
		manager.AddProficiency(entity, prof)
	}

	// Get all proficiencies
	allProfs := manager.GetAllProficiencies(entity)
	if len(allProfs) != 3 {
		t.Errorf("Expected 3 proficiencies, got %d", len(allProfs))
	}

	// Verify all proficiencies are present
	profMap := make(map[string]bool)
	for _, prof := range allProfs {
		profMap[prof.Key()] = true
	}

	for _, prof := range profs {
		if !profMap[prof.Key()] {
			t.Errorf("Missing proficiency: %s", prof.Key())
		}
	}
}

func TestRemoveProficiency(t *testing.T) {
	storage := proficiency.NewMemoryStorage()
	manager := proficiency.NewManager(storage, nil)

	entity := &MockEntity{id: "test", level: 1}

	// Add proficiency
	prof := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		Type: proficiency.ProficiencyTypeWeapon,
		Key:  "dagger",
		Name: "Dagger",
	})
	manager.AddProficiency(entity, prof)

	// Verify it exists
	if !manager.HasWeaponProficiency(entity, "dagger") {
		t.Error("Expected dagger proficiency after adding")
	}

	// Remove proficiency
	manager.RemoveProficiency(entity, "dagger")

	// Verify it's gone
	if manager.HasWeaponProficiency(entity, "dagger") {
		t.Error("Expected no dagger proficiency after removal")
	}
}
