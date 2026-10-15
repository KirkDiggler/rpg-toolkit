// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestCellsRevealedDecodes: the composition's per-cell beat arrives under its
// own kind with its body decoded field for field — two PARTIAL regions stay
// two entries carrying only their own new cells, and every fixed record keeps
// the composition's values. The session adds, drops and merges nothing.
func TestCellsRevealedDecodes(t *testing.T) {
	payload := `{"beat":"` + encounter.BeatCellsRevealed + `",` +
		`"regions":[` +
		`{"id":"corridor","name":"Corridor","archetype":"crypt","lighting":{"intensity":0.5},"cells":[{"x":4,"y":0},{"x":5,"y":0}]},` +
		`{"id":"vault","archetype":"crypt","lighting":{"intensity":1},"cells":[{"x":6,"y":0}]}],` +
		`"scenery":[{"x":5,"y":1}],` +
		`"props":[{"ref":"pillar","at":{"x":4,"y":0},"blocks_line_of_sight":true}],` +
		`"boundaries":[{"from":{"x":5,"y":1},"to":{"x":6,"y":1},"blocks_movement":true,"blocks_line_of_sight":true,"height":0}],` +
		`"segments":[],"sealed":[{"x":5,"y":1}],` +
		`"doorways":[{"door":"gate","from":{"x":5,"y":0},"to":{"x":6,"y":0}}],` +
		`"placed":[{"id":"crate","placement":{"footprint":{"w":1,"d":1},"origin":{"x":2,"y":3}},` +
		`"cells":[{"x":4,"y":0}],"blocks_movement":true,"blocks_line_of_sight":false,"holdable":false}],` +
		`"exits":[{"id":"stairs","at":{"x":6,"y":0}}]}`

	kind, body := decodeBeat([]byte(payload))
	require.Equal(t, EventCellsRevealed, kind)
	revealed, ok := body.(CellsRevealedBody)
	require.True(t, ok, "the beat decodes to its typed body")

	require.Equal(t, []AtlasRegion{
		{ID: "corridor", Name: "Corridor", Archetype: "crypt", Lighting: Lighting{Intensity: 0.5},
			Cells: []spatial.Position{{X: 4, Y: 0}, {X: 5, Y: 0}}},
		{ID: "vault", Archetype: "crypt", Lighting: Lighting{Intensity: 1},
			Cells: []spatial.Position{{X: 6, Y: 0}}},
	}, revealed.Regions, "two partial regions, each with only its new cells")
	require.Equal(t, []spatial.Position{{X: 5, Y: 1}}, revealed.Scenery)
	require.Len(t, revealed.Props, 1)
	require.Equal(t, "pillar", revealed.Props[0].Ref)
	require.True(t, revealed.Props[0].BlocksLineOfSight)
	require.Equal(t, []AtlasBoundary{{From: spatial.Position{X: 5, Y: 1}, To: spatial.Position{X: 6, Y: 1},
		BlocksMovement: true, BlocksLineOfSight: true}}, revealed.Boundaries)
	require.Equal(t, []spatial.Position{{X: 5, Y: 1}}, revealed.Sealed)
	require.Equal(t, []AtlasDoorway{{Door: "gate", From: spatial.Position{X: 5, Y: 0}, To: spatial.Position{X: 6, Y: 0}}},
		revealed.Doorways)
	require.Equal(t, []AtlasPlacedProp{{ID: "crate", Cells: []spatial.Position{{X: 4, Y: 0}}, BlocksMovement: true,
		Placement: FootprintPlacement{Width: 1, Depth: 1, Origin: FootprintPoint{X: 2, Y: 3}}}}, revealed.Placed)
	require.Equal(t, []AtlasExit{{ID: "stairs", At: spatial.Position{X: 6, Y: 0}}}, revealed.Exits)
}

// TestCellsRevealedRefusesARowWithoutIdentity: a region, placed prop or
// structural row that names nothing refuses the whole body — a client's
// cached atlas is never half-patched by a cells beat. The control decodes.
func TestCellsRevealedRefusesARowWithoutIdentity(t *testing.T) {
	for name, payload := range map[string]string{
		"region with no id": `{"beat":"cells_revealed","regions":[{"cells":[{"x":1,"y":0}]}]}`,
		"placed with no id": `{"beat":"cells_revealed","regions":[],"placed":[{"cells":[{"x":1,"y":0}]}]}`,
		"door with no id":   `{"beat":"cells_revealed","regions":[],"structural_doors":[{"ref":"r"}]}`,
		"not an object":     `{"beat":"cells_revealed","regions":5}`,
	} {
		t.Run(name, func(t *testing.T) {
			kind, body := decodeBeat([]byte(payload))
			require.Equal(t, EventCellsRevealed, kind)
			require.Nil(t, body)
		})
	}
	_, body := decodeBeat([]byte(`{"beat":"cells_revealed","regions":[],"scenery":[{"x":1,"y":0}]}`))
	require.NotNil(t, body, "a scenery-only reveal with no region is still a body")
}
