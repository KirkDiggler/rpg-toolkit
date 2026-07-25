// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// This file holds the fixtures shared across more than one test file
// (decode_test.go, validate_test.go, compile_test.go). A fixture used by
// only one test function belongs function-local in that file instead --
// see e.g. compile_test.go's scatteredYAML/cryptKeyYAML.

// referenceYAML is the dungeon-authoring design's §Schema v1 reference
// example, copied verbatim (rpg-toolkit#842).
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

// placedTombYAML is the dungeon-authoring design's "Schema: the place
// block" tomb fragment (rpg-toolkit#842), wrapped in a complete, valid
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

// validM1YAML is the M1-valid variant of referenceYAML's shape (rpg-toolkit#842):
// same 4-room/3-connector chain (entrance -> gallery -> trap-crossing -> tomb,
// same ids/widths/archetypes/connectors/lock DC 12), but M1-valid: entrance/
// gallery drop their count-based monsters: blocks (their count-based
// obstacles: stay, unaffected by the M1 restriction); the tomb room's boss:
// gains at: [7, 5] and its obstacles: are replaced by the design delta's full
// place: list (coffin/altar/statue-reaper/brazier x2/skeleton), occupying
// rows 1, 2, 3, 5, and 6 — clear of doorRow (row 4, height/2).
const validM1YAML = `
version: 1
key: sunken-crypt
name: The Sunken Crypt
theme: crypt
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 10
    obstacles:
      - { ref: "dnd5e:props:obelisk", count: 1 }
      - { ref: "dnd5e:props:pillar", count: 2 }
  - id: gallery
    archetype: chamber
    width: 8
  - id: trap-crossing
    archetype: corridor
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
  - { from: entrance, to: gallery }
  - { from: gallery, to: trap-crossing }
  - { from: trap-crossing, to: tomb, locked: { dc: 12, ability: dex } }
`
