package tables

import "github.com/KirkDiggler/rpg-toolkit/mechanics/spells"

// FullCasterTable implements the standard D&D 5e spell slot progression for full casters.
// Used by: Bard, Cleric, Druid, Sorcerer, Wizard
type FullCasterTable struct {
	progression map[int]map[int]int // [class level][spell level] = slots
}

// NewFullCasterTable creates a spell slot table for full casters.
func NewFullCasterTable() spells.SpellSlotTable {
	return &FullCasterTable{
		progression: map[int]map[int]int{
			1:  {1: 2},
			2:  {1: 3},
			3:  {1: 4, 2: 2},
			4:  {1: 4, 2: 3},
			5:  {1: 4, 2: 3, 3: 2},
			6:  {1: 4, 2: 3, 3: 3},
			7:  {1: 4, 2: 3, 3: 3, 4: 1},
			8:  {1: 4, 2: 3, 3: 3, 4: 2},
			9:  {1: 4, 2: 3, 3: 3, 4: 3, 5: 1},
			10: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2},
			11: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1},
			12: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1},
			13: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1},
			14: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1},
			15: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1, 8: 1},
			16: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1, 8: 1},
			17: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 1, 7: 1, 8: 1, 9: 1},
			18: {1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 1, 7: 1, 8: 1, 9: 1},
			19: {1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 2, 7: 1, 8: 1, 9: 1},
			20: {1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 2, 7: 2, 8: 1, 9: 1},
		},
	}
}

// GetSlots returns the number of spell slots for a given class and spell level.
func (t *FullCasterTable) GetSlots(classLevel int, spellLevel int) int {
	if slots, ok := t.progression[classLevel]; ok {
		return slots[spellLevel]
	}
	return 0
}

// HalfCasterTable implements spell slot progression for half casters.
// Used by: Paladin, Ranger
type HalfCasterTable struct {
	progression map[int]map[int]int
}

// NewHalfCasterTable creates a spell slot table for half casters.
func NewHalfCasterTable() spells.SpellSlotTable {
	return &HalfCasterTable{
		progression: map[int]map[int]int{
			// Half casters don't get spells until level 2
			2:  {1: 2},
			3:  {1: 3},
			4:  {1: 3},
			5:  {1: 4, 2: 2},
			6:  {1: 4, 2: 2},
			7:  {1: 4, 2: 3},
			8:  {1: 4, 2: 3},
			9:  {1: 4, 2: 3, 3: 2},
			10: {1: 4, 2: 3, 3: 2},
			11: {1: 4, 2: 3, 3: 3},
			12: {1: 4, 2: 3, 3: 3},
			13: {1: 4, 2: 3, 3: 3, 4: 1},
			14: {1: 4, 2: 3, 3: 3, 4: 1},
			15: {1: 4, 2: 3, 3: 3, 4: 2},
			16: {1: 4, 2: 3, 3: 3, 4: 2},
			17: {1: 4, 2: 3, 3: 3, 4: 3, 5: 1},
			18: {1: 4, 2: 3, 3: 3, 4: 3, 5: 1},
			19: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2},
			20: {1: 4, 2: 3, 3: 3, 4: 3, 5: 2},
		},
	}
}

// GetSlots returns the number of spell slots for a given class and spell level.
func (t *HalfCasterTable) GetSlots(classLevel int, spellLevel int) int {
	if slots, ok := t.progression[classLevel]; ok {
		return slots[spellLevel]
	}
	return 0
}

// PactMagicTable implements pact magic progression.
// Used by: Warlock
type PactMagicTable struct{}

// NewPactMagicTable creates a spell slot table for pact magic.
func NewPactMagicTable() spells.SpellSlotTable {
	return &PactMagicTable{}
}

// GetSlots returns warlock spell slots (all same level).
func (t *PactMagicTable) GetSlots(classLevel int, spellLevel int) int {
	// Warlocks have a different progression
	maxLevel := 1
	slots := 1

	if classLevel >= 2 {
		slots = 2
	}
	if classLevel >= 3 {
		maxLevel = 2
	}
	if classLevel >= 5 {
		maxLevel = 3
	}
	if classLevel >= 7 {
		maxLevel = 4
	}
	if classLevel >= 9 {
		maxLevel = 5
	}
	if classLevel >= 11 {
		slots = 3
	}
	if classLevel >= 17 {
		slots = 4
	}

	if spellLevel == maxLevel {
		return slots
	}
	return 0
}

// ThirdCasterTable implements spell slot progression for third casters.
// Used by: Eldritch Knight, Arcane Trickster
type ThirdCasterTable struct {
	progression map[int]map[int]int
}

// NewThirdCasterTable creates a spell slot table for third casters.
func NewThirdCasterTable() spells.SpellSlotTable {
	return &ThirdCasterTable{
		progression: map[int]map[int]int{
			// Third casters don't get spells until level 3
			3:  {1: 2},
			4:  {1: 3},
			5:  {1: 3},
			6:  {1: 3},
			7:  {1: 4, 2: 2},
			8:  {1: 4, 2: 2},
			9:  {1: 4, 2: 2},
			10: {1: 4, 2: 3},
			11: {1: 4, 2: 3},
			12: {1: 4, 2: 3},
			13: {1: 4, 2: 3, 3: 2},
			14: {1: 4, 2: 3, 3: 2},
			15: {1: 4, 2: 3, 3: 2},
			16: {1: 4, 2: 3, 3: 3},
			17: {1: 4, 2: 3, 3: 3},
			18: {1: 4, 2: 3, 3: 3},
			19: {1: 4, 2: 3, 3: 3, 4: 1},
			20: {1: 4, 2: 3, 3: 3, 4: 1},
		},
	}
}

// GetSlots returns the number of spell slots for a given class and spell level.
func (t *ThirdCasterTable) GetSlots(classLevel int, spellLevel int) int {
	if slots, ok := t.progression[classLevel]; ok {
		return slots[spellLevel]
	}
	return 0
}
