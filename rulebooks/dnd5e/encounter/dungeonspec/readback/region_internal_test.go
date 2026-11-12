// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// RegionSuite holds the read-back's own region lookup (used for post cells
// and exits) to the compiler's: every creature's start cell lands in the
// region the compiler derived for it.
type RegionSuite struct{ suite.Suite }

func TestRegionSuite(t *testing.T) { suite.Run(t, new(RegionSuite)) }

func (s *RegionSuite) TestStartCellsAgreeWithTheCompiler() {
	raw, err := os.ReadFile("../testdata/world-builder-v4-castle.yaml")
	s.Require().NoError(err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: decoded.Spec})
	s.Require().NoError(err)
	fresh, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	b := newBuilder(fresh.Spec, &compiled, raw)
	s.Require().NotEmpty(fresh.Spec.Room.Gameplay.MonsterDeclarations)
	for _, d := range fresh.Spec.Room.Gameplay.MonsterDeclarations {
		m := b.monsters[d.ID]
		s.Require().NotNil(m, d.ID)
		loc := d.StartingCell.Location
		s.Equal(b.regionWords(m.Region), b.regionAt(loc.Q, loc.R), "%s at q %d r %d", d.ID, loc.Q, loc.R)
	}
}
