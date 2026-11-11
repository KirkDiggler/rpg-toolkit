// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type GroundProjectionSuite struct{ suite.Suite }

func TestGroundProjectionSuite(t *testing.T) { suite.Run(t, new(GroundProjectionSuite)) }

func (s *GroundProjectionSuite) TestSnapshotCopiesExactlyTheProviderGround() {
	in := encounter.Atlas{Regions: []encounter.AtlasRegion{{ID: "court", Cells: []spatial.Position{{X: 1, Y: 2}}, Ground: &encounter.RegionGround{BaseRef: "grass", Paint: []encounter.GroundPaint{{Ref: "grass", Cells: []spatial.Position{{X: 1, Y: 2}}}}}}, {ID: "old"}}}
	out := projectAtlas(in)
	s.Equal(&RegionGround{BaseRef: "grass", Paint: []GroundPaint{{Ref: "grass", Cells: []spatial.Position{{X: 1, Y: 2}}}}}, out.Regions[0].Ground)
	s.Nil(out.Regions[1].Ground)
	in.Regions[0].Ground.BaseRef = "changed"
	in.Regions[0].Ground.Paint[0].Cells[0].X = 9
	s.Equal("grass", out.Regions[0].Ground.BaseRef)
	s.Equal(float64(1), out.Regions[0].Ground.Paint[0].Cells[0].X)
}

func (s *GroundProjectionSuite) TestBothDiscoveryKindsRetainOnlyTheSuppliedPaint() {
	payload := []byte(`{"regions":[{"id":"court","cells":[{"x":1,"y":2}],"archetype":"crypt","lighting":{"intensity":1},"ground":{"baseRef":"grass","paint":[{"ref":"dirt","cells":[{"x":1,"y":2}]}]}}]}`)
	learned, err := cellsLearned(payload)
	s.Require().NoError(err)
	secret, err := secretRevealed(payload)
	s.Require().NoError(err)
	for _, regions := range [][]AtlasRegion{learned.Regions, secret.Regions} {
		s.Require().Len(regions, 1)
		s.Equal(&RegionGround{BaseRef: "grass", Paint: []GroundPaint{{Ref: "dirt", Cells: []spatial.Position{{X: 1, Y: 2}}}}}, regions[0].Ground)
	}
}
