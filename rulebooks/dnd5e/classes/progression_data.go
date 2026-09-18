// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package classes

// The 2014 spellcasting tables, transcribed column for column.
//
// Design R2.1: "The starting configuration is 2014. Every per-level fact we
// author comes from the 2014 tables unless a divergence is recorded as a ruling
// with its reason." Nothing in this file is a rule — moving a number here moves
// when a class gets something, and no function has to change (R2.3).

// fullCasterSlots is the "Spell Slots per Spell Level" block every full caster
// shares: bard (PHB p.52), cleric (p.58), druid (p.65), sorcerer (p.100) and
// wizard (p.113) print the same twenty rows.
//
// One table rather than five copies, because five copies of one fact can
// disagree and this one never differs between those classes.
var fullCasterSlots = [][]int{
	{2},
	{3},
	{4, 2},
	{4, 3},
	{4, 3, 2},
	{4, 3, 3},
	{4, 3, 3, 1},
	{4, 3, 3, 2},
	{4, 3, 3, 3, 1},
	{4, 3, 3, 3, 2},
	{4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 2, 1, 1},
}

// halfCasterSlots is the paladin (PHB p.84) and ranger (p.91) block. A half
// caster has no slots at level 1 at all, which is why its first entry is nil
// rather than zero-length: the class genuinely casts nothing yet.
var halfCasterSlots = [][]int{
	nil,
	{2},
	{3},
	{3},
	{4, 2},
	{4, 2},
	{4, 3},
	{4, 3},
	{4, 3, 2},
	{4, 3, 2},
	{4, 3, 3},
	{4, 3, 3},
	{4, 3, 3, 1},
	{4, 3, 3, 1},
	{4, 3, 3, 2},
	{4, 3, 3, 2},
	{4, 3, 3, 3, 1},
	{4, 3, 3, 3, 1},
	{4, 3, 3, 3, 2},
	{4, 3, 3, 3, 2},
}

// noCantrips is the cantrip column of a class that has none at any level.
var noCantrips = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// preparedCasterPlaceholder is the "spells known" column of a class that does
// not know spells — it prepares them from its whole class list.
//
// A prepared caster has no known-spell count in the 2014 tables, so there is no
// 2014 number to transcribe. What creation asks a cleric or a druid for today
// is what it keeps asking at every level, so nothing about them moves until
// prepared casting arrives (rpg-project#445) and replaces this column with the
// preparation formula it belongs to.
func preparedCasterPlaceholder(creationCount int) []int {
	column := make([]int, MaxClassLevel)
	for i := range column {
		column[i] = creationCount
	}
	return column
}

// clericPreparedSpellCount is what creation asks a cleric to take: the whole
// supported 1st-level cleric list. It is read here so the placeholder column
// and the option list are one fact, and a new supported cleric spell does not
// leave the two disagreeing.
const clericPreparedSpellCount = 6

// spellProgressions is every class's table. A class absent from this map does
// not cast at any level.
var spellProgressions = map[Class]*SpellProgression{
	// Bard — PHB p.52. Cantrips 2/3/4 at levels 1/4/10; spells known is the
	// column that makes bard the proof case for a derived level-up choice:
	// four at level 1, five at level 2.
	Bard: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: []int{2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
		SpellsKnown:   []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 15, 15, 16, 18, 19, 19, 20, 22, 22, 22},
		SpellSlots:    fullCasterSlots,
	},

	// Cleric — PHB p.58. Prepared caster: the spells-known column is the
	// placeholder described on [preparedCasterPlaceholder].
	Cleric: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: []int{3, 3, 3, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		SpellsKnown:   preparedCasterPlaceholder(clericPreparedSpellCount),
		SpellSlots:    fullCasterSlots,
	},

	// Druid — PHB p.65. Prepared caster, and creation asks a druid for no
	// spells at all today, so its placeholder is zero.
	Druid: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: []int{2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
		SpellsKnown:   preparedCasterPlaceholder(0),
		SpellSlots:    fullCasterSlots,
	},

	// Paladin — PHB p.84. No cantrips ever, prepared caster, and no slots
	// until level 2: a level-1 paladin has a spellcasting ability and nothing
	// to spend it on, which is what the empty first slot row says.
	Paladin: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: noCantrips,
		SpellsKnown:   preparedCasterPlaceholder(0),
		SpellSlots:    halfCasterSlots,
	},

	// Ranger — PHB p.91. A KNOWN caster with no cantrips, whose first spells
	// arrive at level 2. This build has no ranger spell list, so a ranger
	// levelling to 2 is refused rather than quietly given nothing; see
	// choices.GetClassRequirementsGainedAtLevel.
	Ranger: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: noCantrips,
		SpellsKnown:   []int{0, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11},
		SpellSlots:    halfCasterSlots,
	},

	// Sorcerer — PHB p.100.
	Sorcerer: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: []int{4, 4, 4, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6},
		SpellsKnown:   []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 12, 13, 13, 14, 14, 15, 15, 15, 15},
		SpellSlots:    fullCasterSlots,
	},

	// Warlock — PHB p.107, DELIBERATELY ONE ENTRY LONG.
	//
	// Pact Magic is a different rule from the rest of this file: every slot is
	// the same level, that level climbs with the warlock, and the whole pool
	// returns on a SHORT rest. Transcribing the table without building that
	// reset would hand a warlock long-rest slots, which is worse than having
	// none. So this row is exactly what the class already had at level 1 —
	// nothing about a warlock changes — and the table above it arrives with
	// Pact Magic (design §8).
	Warlock: {
		SlotReset:     SpellSlotResetPactMagic,
		CantripsKnown: []int{2},
		SpellsKnown:   []int{2},
		SpellSlots:    [][]int{{1}},
	},

	// Wizard — PHB p.113. The spells-known column is the SPELLBOOK: six
	// first-level spells at level 1 and two more with every wizard level.
	Wizard: {
		SlotReset:     SpellSlotResetLongRest,
		CantripsKnown: []int{3, 3, 3, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		SpellsKnown:   []int{6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 40, 42, 44},
		SpellSlots:    fullCasterSlots,
	},
}
