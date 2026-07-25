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

// placedTombYAML is design.md's "Schema: the place block" tomb fragment
// (ideas/dungeon-authoring/design.md), wrapped in a complete, valid
// two-room file — this IS content/dungeons/reference-tomb.yaml's content,
// not a separate lookalike. height: 8 is load-bearing: the tomb room's
// placements use rows 1, 2, 3, 5, and 6 (boss at row 5; coffin/altar at
// row 3; statue/skeleton/braziers at rows 1/1/2/6), so height: 8 puts
// doorRow (height/2) at row 4, clear of all of them.
const placedTombYAML = `
version: 1
key: reference-tomb
name: The Reference Tomb
theme: crypt
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 6
  - id: tomb
    archetype: boss
    width: 12
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5] }
    place:
      - { ref: "dnd5e:props:coffin",        at: [6, 3], blocks_los: false }
      - { ref: "dnd5e:props:altar",         at: [9, 3] }
      - { ref: "dnd5e:props:statue-reaper", at: [1, 1] }
      - { ref: "dnd5e:props:brazier",       at: [3, 1] }
      - { ref: "dnd5e:props:brazier",       at: [3, 6] }
      - { ref: "dnd5e:monsters:skeleton",   at: [4, 2] }
connectors:
  - { from: entrance, to: tomb }
`

func TestDecode_PlaceBlockRoundTrips(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(placedTombYAML))
	require.NoError(t, err)
	tomb := spec.Rooms[len(spec.Rooms)-1]
	require.Len(t, tomb.Place, 6) // coffin, altar, statue-reaper, brazier x2, skeleton
	assert.Equal(t, "dnd5e:props:coffin", tomb.Place[0].Ref)
	assert.Equal(t, [2]int{6, 3}, tomb.Place[0].At)
	require.NotNil(t, tomb.Place[0].BlocksLoS)
	assert.False(t, *tomb.Place[0].BlocksLoS)
	assert.Equal(t, "dnd5e:monsters:skeleton", tomb.Place[5].Ref)
	require.NotNil(t, tomb.Boss.At)
	assert.Equal(t, [2]int{7, 5}, *tomb.Boss.At)
}
