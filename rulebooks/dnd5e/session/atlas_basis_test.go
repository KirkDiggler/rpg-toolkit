// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestAtlasCellsDrawTheAuthoredShape is ADR-0040's promise, checked at the
// wire it was made for (rpg-toolkit#1150): a client that takes Atlas.Cells as
// axial (q, r), reads Atlas.Layout, and applies the STANDARD pixel formula for
// that layout gets the authored picture — with no private knowledge of the
// toolkit.
//
// hexWorld is two 6x6 chambers side by side, authored pointy-top: 12 columns
// by 6 rows. An external reference (the formula), not a round-trip — the only
// kind of test that could see the basis this seam used to hand out.
func (s *ReadTestSuite) TestAtlasCellsDrawTheAuthoredShape() {
	s.startWith(hexWorld(s.T()))
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.HexLayoutPointyTop, atlas.Layout)

	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, c := range atlas.Cells {
		// Pointy-top, circumradius 1: x = √3·(q + r/2), y = 1.5·r.
		x := math.Sqrt(3) * (c.X + c.Y/2)
		y := 1.5 * c.Y
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}

	const cols, rows = 12, 6
	s.InDelta(math.Sqrt(3)*(cols-1+0.5), maxX-minX, 1e-9, "12 authored columns across")
	s.InDelta(1.5*(rows-1), maxY-minY, 1e-9, "6 authored rows down")
}
