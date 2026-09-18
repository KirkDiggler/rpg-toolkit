// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package classes

// MaxClassLevel is the highest class level the 2014 tables describe. A class
// table states a value for every level up to it.
const MaxClassLevel = 20

// SpellSlotReset says how a class's spell slots come back, because that is a
// property of the class's casting and not of the slots themselves.
//
// It is here rather than implied by the class because the toolkit builds a slot
// POOL only for a recovery rule it actually implements: a Pact Magic pool
// created with a long-rest reset would be wrong every short rest.
type SpellSlotReset string

const (
	// SpellSlotResetNone belongs to a class with no spell slots at all.
	SpellSlotResetNone SpellSlotReset = ""

	// SpellSlotResetLongRest is Vancian casting: every slot returns on a long
	// rest. Bard, cleric, druid, paladin, ranger, sorcerer and wizard.
	SpellSlotResetLongRest SpellSlotReset = "long_rest"

	// SpellSlotResetPactMagic is the warlock's rule: every slot is the same
	// level, that level rises with the warlock, and they all return on a SHORT
	// rest. This build sizes no pool for it — see [SpellProgression] for the
	// seam.
	SpellSlotResetPactMagic SpellSlotReset = "pact_magic"
)

// SpellProgression is a class's spellcasting table, written the way the 2014
// Player's Handbook prints it: one column per fact, one entry per class level.
//
// This replaced three scalars on [Data] — CantripsKnown, SpellsKnown and
// SpellSlots — each of which held only the level-1 value and was commented
// "At level 1". A level-up has to ask what a class knows at level N, and a
// scalar has one answer forever (design R4.5: the level-1 scalars "do not stay
// alongside it — a narrow copy of a wide fact is a second source of truth").
//
// A column shorter than [MaxClassLevel] holds its last entry for every level
// above it. That is how warlock is written: one entry, its level-1 values, and
// nothing above, because Pact Magic's table belongs with Pact Magic's reset
// rule and this wave builds neither (design §8).
type SpellProgression struct {
	// SlotReset is how the slots in this table come back.
	SlotReset SpellSlotReset

	// CantripsKnown is how many cantrips a class of this level knows.
	CantripsKnown []int

	// SpellsKnown is how many spells a class of this level knows.
	//
	// It means different things for different casters, and the class's own
	// table comment says which:
	//   - a KNOWN caster (bard, sorcerer, ranger, warlock) counts spells known;
	//   - a wizard counts the spells in its spellbook;
	//   - Cleric uses PreparedSpells instead;
	//   - other PREPARED casters (druid, paladin) prepare from their whole
	//     class list and has no "known" count at all, so its column is a
	//     placeholder holding what creation asks for today and nothing moves
	//     until prepared casting lands (rpg-project#445).
	SpellsKnown []int

	// PreparedSpells counts chosen preparations, excluding automatic subclass grants.
	// Only Cleric currently adopts the 2024 preparation progression.
	PreparedSpells []int

	// SpellSlots is the slots a class of this level has, lowest spell level
	// first: index 0 is 1st-level slots. Nil means none at that class level.
	SpellSlots [][]int
}

// SpellProgressionRow is what a class's table says at one class level.
//
// The zero row is the truth about a class that does not cast: no cantrips, no
// spells, no slots.
type SpellProgressionRow struct {
	// ClassLevel is the level this row describes.
	ClassLevel int

	// CantripsKnown is how many cantrips this class knows at ClassLevel.
	CantripsKnown int

	// SpellsKnown is how many spells this class knows at ClassLevel, in the
	// sense [SpellProgression.SpellsKnown] describes for this class.
	SpellsKnown int

	// PreparedSpells is the number of chosen preparations at this class level.
	PreparedSpells int

	// SpellSlots is the slots this class has at ClassLevel, lowest first. It is
	// a copy: the table is shared and a caller must not be able to edit it.
	SpellSlots []int

	// SlotReset is how those slots come back.
	SlotReset SpellSlotReset
}

// HighestSpellLevel returns the highest spell level this row has a slot for,
// or 0 when it has none.
//
// This is the spell level a class learns at when it learns one: the 2014 rule
// is that a new spell must be of a level you can cast, and the top of your
// slot table is what that means.
func (r SpellProgressionRow) HighestSpellLevel() int {
	highest := 0
	for i, count := range r.SpellSlots {
		if count > 0 {
			highest = i + 1
		}
	}
	return highest
}

// GetSpellProgression returns a class's spellcasting table, or nil for a class
// that does not cast.
func GetSpellProgression(classID Class) *SpellProgression {
	return spellProgressions[classID]
}

// SpellProgressionAtLevel returns what a class's table says at a class level.
//
// A class with no table, or a level below 1, returns the zero row — which is
// the truthful answer for a fighter at any level, rather than an error the
// caller has to decide what to do with.
func SpellProgressionAtLevel(classID Class, classLevel int) SpellProgressionRow {
	if classLevel < 1 {
		return SpellProgressionRow{}
	}

	progression := spellProgressions[classID]
	if progression == nil {
		return SpellProgressionRow{ClassLevel: classLevel}
	}

	return SpellProgressionRow{
		ClassLevel:     classLevel,
		CantripsKnown:  columnAt(progression.CantripsKnown, classLevel),
		SpellsKnown:    columnAt(progression.SpellsKnown, classLevel),
		PreparedSpells: columnAt(progression.PreparedSpells, classLevel),
		SpellSlots:     slotsAt(progression.SpellSlots, classLevel),
		SlotReset:      progression.SlotReset,
	}
}

// columnAt reads one column at a class level, holding the last authored entry
// for every level above the column's end.
func columnAt(column []int, classLevel int) int {
	if len(column) == 0 {
		return 0
	}
	if classLevel > len(column) {
		return column[len(column)-1]
	}
	return column[classLevel-1]
}

// slotsAt reads the slot column at a class level, returning a copy so the
// shared table cannot be edited through a row.
func slotsAt(column [][]int, classLevel int) []int {
	if len(column) == 0 {
		return nil
	}
	index := classLevel - 1
	if index >= len(column) {
		index = len(column) - 1
	}
	slots := column[index]
	if len(slots) == 0 {
		return nil
	}
	out := make([]int, len(slots))
	copy(out, slots)
	return out
}
