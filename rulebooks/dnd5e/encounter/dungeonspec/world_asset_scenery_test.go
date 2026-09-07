// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func worldAssetSceneryYAML(ref, placementFields string) string {
	return fmt.Sprintf(`version: 2
key: world-assets
orientation: pointy
void: opaque
regions:
  - id: room
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0]]
start: [0,0]
place:
  - { ref: %q, at: [1,0], %s }
`, ref, placementFields)
}

func TestWorldAssetNamespacesCompileAsScenery(t *testing.T) {
	refs := []string{
		"dnd5e:props:dark-fortress:altar_01",
		"dnd5e:items:dark-fortress:health_potion_01",
		"dnd5e:weapons:dark-fortress:sword_01",
		"dnd5e:env:dark-fortress:gate_01",
	}
	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			compiled, err := dungeonspec.Load([]byte(worldAssetSceneryYAML(
				ref,
				"blocks_movement: false, blocks_los: true, facing: ne, offset: [0.2,-0.1,0.3]",
			)))
			require.NoError(t, err)
			require.Len(t, compiled.Field.Props, 1)
			prop := compiled.Field.Props[0]
			require.Equal(t, ref, prop.Ref)
			require.Equal(t, spatial.Position{X: 1, Y: 0}, prop.At)
			require.False(t, *prop.BlocksMovement)
			require.True(t, *prop.BlocksLineOfSight)
			require.Equal(t, "ne", prop.Facing)
			require.Equal(t, [3]float64{0.2, -0.1, 0.3}, prop.Offset)
		})
	}
}

func TestNonPropsSceneryUsesPropValidation(t *testing.T) {
	tests := []struct {
		name, ref, fields, path, message string
	}{
		{
			name:    "item missing blocks_movement",
			ref:     "dnd5e:items:dark-fortress:health_potion_01",
			fields:  "blocks_los: true",
			path:    "place[0].blocks_movement",
			message: "there is no default",
		},
		{
			name:    "weapon missing blocks_los",
			ref:     "dnd5e:weapons:dark-fortress:sword_01",
			fields:  "blocks_movement: false",
			path:    "place[0].blocks_los",
			message: "there is no default",
		},
		{
			name:    "environment with targeting",
			ref:     "dnd5e:env:dark-fortress:gate_01",
			fields:  "blocks_movement: true, blocks_los: true, targeting: lowest-health",
			path:    "place[0].targeting",
			message: "not a monster",
		},
		{
			name:    "item marked boss",
			ref:     "dnd5e:items:dark-fortress:health_potion_01",
			fields:  "blocks_movement: false, blocks_los: false, boss: true",
			path:    "place[0].boss",
			message: "not a monster",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defects := dungeonspec.Validate(mustDecodeWorldAsset(
				t,
				worldAssetSceneryYAML(tc.ref, tc.fields),
			))
			require.Len(t, defects, 1)
			require.Equal(t, tc.path, defects[0].Path)
			require.Contains(t, defects[0].Message, tc.message)
		})
	}
}

func TestNonWorldAssetNamespaceStillCannotBePlaced(t *testing.T) {
	defects := dungeonspec.Validate(mustDecodeWorldAsset(
		t,
		worldAssetSceneryYAML(
			"dnd5e:spells:fireball",
			"blocks_movement: false, blocks_los: false",
		),
	))
	require.NotEmpty(t, defects)
	require.Equal(t, "place[0].ref", defects[0].Path)
	require.Contains(t, defects[0].Message, `type "spells"`)
}

func mustDecodeWorldAsset(t *testing.T, raw string) *dungeonspec.Spec {
	t.Helper()
	spec, err := dungeonspec.Decode([]byte(raw))
	require.NoError(t, err)
	return spec
}
