// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package classes_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// SpellProgressionSuite covers the shape of the class tables, which is the
// thing a transcription error shows up in first.
type SpellProgressionSuite struct {
	suite.Suite
}

func TestSpellProgressionSuite(t *testing.T) {
	suite.Run(t, new(SpellProgressionSuite))
}

// TestEveryTableStatesAValueForEveryLevel — a column shorter than twenty holds
// its last entry for every level above it, which is a real reading and also a
// way to leave a table half written. Warlock is the one that means it.
func (s *SpellProgressionSuite) TestEveryTableStatesAValueForEveryLevel() {
	for classID := range classes.ClassData {
		progression := classes.GetSpellProgression(classID)
		if progression == nil {
			continue
		}
		s.Run(string(classID), func() {
			if classID == classes.Warlock {
				s.Len(progression.CantripsKnown, 1,
					"warlock is deliberately one row: Pact Magic is a different rule")
				return
			}
			s.Len(progression.CantripsKnown, classes.MaxClassLevel)
			if classID == classes.Cleric {
				s.Empty(progression.SpellsKnown)
				s.Len(progression.PreparedSpells, classes.MaxClassLevel)
			} else {
				s.Len(progression.SpellsKnown, classes.MaxClassLevel)
				s.Empty(progression.PreparedSpells)
			}
			s.Len(progression.SpellSlots, classes.MaxClassLevel)
		})
	}
}

// TestNoTableGoesBackwards — a known-spell count that fell would make the
// derived requirement a negative number of spells, which is not a question.
func (s *SpellProgressionSuite) TestNoTableGoesBackwards() {
	for classID := range classes.ClassData {
		if classes.GetSpellProgression(classID) == nil {
			continue
		}
		s.Run(string(classID), func() {
			for level := 2; level <= classes.MaxClassLevel; level++ {
				row := classes.SpellProgressionAtLevel(classID, level)
				previous := classes.SpellProgressionAtLevel(classID, level-1)
				s.GreaterOrEqual(row.CantripsKnown, previous.CantripsKnown, "level %d", level)
				s.GreaterOrEqual(row.SpellsKnown, previous.SpellsKnown, "level %d", level)
				s.GreaterOrEqual(row.HighestSpellLevel(), previous.HighestSpellLevel(),
					"level %d", level)
			}
		})
	}
}

// TestTheHighestSpellLevelIsTheTOPOfTheSlotRow — this is the spell level a
// class learns at when it learns one, and a row of {4, 2} means a bard of that
// level learns a 2nd-level spell, not a 1st. Reading the lowest non-zero entry
// instead gives 1 at every level of every caster and looks right forever.
func (s *SpellProgressionSuite) TestTheHighestSpellLevelIsTheTOPOfTheSlotRow() {
	for _, tc := range []struct {
		level   int
		highest int
	}{
		{level: 1, highest: 1},
		{level: 2, highest: 1},
		{level: 3, highest: 2},
		{level: 5, highest: 3},
		{level: 9, highest: 5},
		{level: 17, highest: 9},
	} {
		s.Run("", func() {
			s.Equal(tc.highest, classes.SpellProgressionAtLevel(classes.Bard, tc.level).HighestSpellLevel(),
				"bard level %d", tc.level)
		})
	}
}

// TestASlotRowNeverReachesPastNinthLevel — there is no pool for a tenth-level
// slot, so a table that reached one would be sizing a resource that does not
// exist.
func (s *SpellProgressionSuite) TestASlotRowNeverReachesPastNinthLevel() {
	for classID := range classes.ClassData {
		if classes.GetSpellProgression(classID) == nil {
			continue
		}
		for level := 1; level <= classes.MaxClassLevel; level++ {
			s.LessOrEqual(classes.SpellProgressionAtLevel(classID, level).HighestSpellLevel(), 9,
				"%s level %d", classID, level)
		}
	}
}

// TestANonCasterReadsAsZeroRatherThanAnError — the zero row is the truth about
// a fighter at any level, and a caller that had to handle an error instead
// would be handling one at every level of four classes.
func (s *SpellProgressionSuite) TestANonCasterReadsAsZeroRatherThanAnError() {
	row := classes.SpellProgressionAtLevel(classes.Fighter, 11)

	s.Equal(11, row.ClassLevel)
	s.Zero(row.CantripsKnown)
	s.Zero(row.SpellsKnown)
	s.Empty(row.SpellSlots)
	s.Equal(classes.SpellSlotResetNone, row.SlotReset)
	s.Zero(row.HighestSpellLevel())
}

// TestARowsSlotsAreNotTheTables — a caller that edited the slice it was handed
// would rewrite the class table for every character in the process.
func (s *SpellProgressionSuite) TestARowsSlotsAreNotTheTables() {
	row := classes.SpellProgressionAtLevel(classes.Bard, 3)
	s.Require().Equal([]int{4, 2}, row.SpellSlots)

	row.SpellSlots[0] = 99

	s.Equal([]int{4, 2}, classes.SpellProgressionAtLevel(classes.Bard, 3).SpellSlots)
}

// TestHalfCastersHaveNoSlotsAtLevelOne — a paladin has a spellcasting ability
// and nothing to spend it on until level 2, and an empty first row is how the
// table says so.
func (s *SpellProgressionSuite) TestHalfCastersHaveNoSlotsAtLevelOne() {
	for _, classID := range []classes.Class{classes.Paladin, classes.Ranger} {
		s.Empty(classes.SpellProgressionAtLevel(classID, 1).SpellSlots, "%s", classID)
		s.Equal([]int{2}, classes.SpellProgressionAtLevel(classID, 2).SpellSlots, "%s", classID)
	}
}
