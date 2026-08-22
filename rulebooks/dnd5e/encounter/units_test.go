// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/suite"
)

// CellsFromFeetTestSuite pins the ONE feet-to-cells conversion this
// composition offers (Kirk, rpg-project#254 review: "convert feet->cells
// once... in one exported helper"), so the reach gate and the monster
// turn's movement budget both use the same arithmetic rather than each
// re-deriving it.
type CellsFromFeetTestSuite struct {
	suite.Suite
}

func TestCellsFromFeetSuite(t *testing.T) {
	suite.Run(t, new(CellsFromFeetTestSuite))
}

func (s *CellsFromFeetTestSuite) TestExactMultiplesConvertCleanly() {
	s.Equal(1, encounter.CellsFromFeet(5), "5 feet is one cell — standard melee reach")
	s.Equal(2, encounter.CellsFromFeet(10), "10 feet is two cells — the Reach property")
	s.Equal(24, encounter.CellsFromFeet(120), "120 feet is 24 cells — a character's default sight range")
	s.Equal(12, encounter.CellsFromFeet(60), "60 feet is 12 cells — a common darkvision range")
}

func (s *CellsFromFeetTestSuite) TestZeroFeetIsZeroCells() {
	s.Equal(0, encounter.CellsFromFeet(0))
}

func (s *CellsFromFeetTestSuite) TestFeetPerCellIsFive() {
	s.Equal(5, encounter.FeetPerCell, "a cell is 5 feet everywhere in this codebase's data")
}
