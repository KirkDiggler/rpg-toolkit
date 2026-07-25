// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// referenceYAML is design.md's §Schema v1 reference example, copied
// verbatim (ideas/dungeon-authoring/design.md, "Schema v1 — reference
// example").
const referenceYAML = `
version: 1
key: sunken-crypt
name: The Sunken Crypt
theme: crypt
height: 8                      # shared by every room — the generator's real shape
rooms:
  - id: entrance
    archetype: entrance
    width: 10
    monsters:
      - { ref: "dnd5e:monsters:skeleton", count: 2 }
    obstacles:
      - { ref: "dnd5e:props:obelisk", count: 1 }
      - { ref: "dnd5e:props:pillar", count: 2 }
  - id: gallery
    archetype: chamber
    width: 8
    monsters:
      - { ref: "dnd5e:monsters:ghoul", count: 1 }
  - id: trap-crossing            # v1: an empty corridor; trap content lands in the
    archetype: corridor          # reserved ` + "`interactions`" + ` seat (see Deferred)
    width: 6
  - id: tomb
    archetype: boss
    width: 12
    boss: { ref: "dnd5e:monsters:skeleton-captain" }
    obstacles:
      - { ref: "dnd5e:props:coffin", count: 1, blocks_los: false }
      - { ref: "dnd5e:props:altar", count: 1 }
connectors:
  - { from: entrance, to: gallery }
  - { from: gallery, to: trap-crossing }
  - { from: trap-crossing, to: tomb, locked: { dc: 12, ability: dex } }
`

func TestDecode_RoundTripsTheReferenceSpec(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(referenceYAML))
	require.NoError(t, err)
	assert.Equal(t, 1, spec.Version)
	assert.Equal(t, "sunken-crypt", spec.Key)
	assert.Equal(t, 8, spec.Height)
	require.Len(t, spec.Rooms, 4)
	assert.Equal(t, "entrance", spec.Rooms[0].ID)
	require.Len(t, spec.Connectors, 3)
	require.NotNil(t, spec.Connectors[2].Locked)
	assert.Equal(t, 12, spec.Connectors[2].Locked.DC)
}

func TestDecode_UnknownFieldFailsLoudly(t *testing.T) {
	_, err := dungeonspec.Decode([]byte("version: 1\nmosnters: []\n"))
	require.Error(t, err) // the typo class the design promises dies at load
	assert.Contains(t, err.Error(), "mosnter")
}
