// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func castleGroundAt(t testing.TB, region encounter.AtlasRegion, cell spatial.Position) string {
	t.Helper()
	require.Contains(t, region.Cells, cell)
	require.NotNil(t, region.Ground)
	for _, paint := range region.Ground.Paint {
		if slices.Contains(paint.Cells, cell) {
			return paint.Ref
		}
	}
	return region.Ground.BaseRef
}

func TestMadKingCourtyardGroundKeepsTheFloorAndSurvivesTheWalk(t *testing.T) {
	const grass = "dnd5e:ground:fantasy-kingdom:grass_01"
	const dirt = "dnd5e:ground:fantasy-kingdom:dirt_01"
	s := openTheCastle(t, spatial.Position{X: 0, Y: 6})
	court, err := s.enc.Region("region-courtyard")
	require.NoError(t, err)
	require.NotNil(t, court.Ground, "the actual QA castle carries its courtyard appearance")
	require.Equal(t, grass, court.Ground.BaseRef)
	// The gate/hall thresholds and the open barracks/armoury edges remain
	// connected by the authored dirt route, not a new movement rule.
	for _, cell := range []spatial.Position{castleCell(0, 6), castleCell(0, -6), castleCell(-14, 0), castleCell(14, 0)} {
		require.Equal(t, dirt, castleGroundAt(t, court, cell))
	}
	require.Equal(t, grass, castleGroundAt(t, court, castleCell(8, 2)))
	gate, err := s.enc.Region("region-gatehouse")
	require.NoError(t, err)
	require.Nil(t, gate.Ground, "the existing gatehouse floor is not repainted")

	plain := s.compiled.Field
	plain.Regions = append([]encounter.RegionInput(nil), plain.Regions...)
	for i := range plain.Regions {
		plain.Regions[i].Ground = nil
	}
	before, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: plain})
	require.NoError(t, err)
	after, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: s.compiled.Field})
	require.NoError(t, err)
	require.Equal(t, before, after, "appearance changes no floor partition or traversal boundary")

	for row := 5; row >= -6; row-- {
		_, stepErr := s.enc.Step(&encounter.StepInput{Member: hero, To: castleCell(0, row), EndWalk: true})
		require.NoError(t, stepErr)
		s.reload()
	}
	require.Equal(t, castleCell(0, -6), s.at(hero))
	reloaded, err := s.enc.Region("region-courtyard")
	require.NoError(t, err)
	require.Equal(t, court, reloaded, "walking and host reload preserve the captured ground and owner cells")
}
