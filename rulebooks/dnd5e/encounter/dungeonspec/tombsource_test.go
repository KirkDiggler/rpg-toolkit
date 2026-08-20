// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// tombsource_test.go holds the authored dungeon every test in this package is
// about, in one place, because two very different suites need the same bytes:
// dialect_test.go mutates one line of it at a time to check what gets refused,
// and tomb_test.go compiles it and walks it.
//
// IT IS A COPY OF A FILE ANOTHER REPO OWNS, and that is a real cost rather than
// a convenience. rpg-api ships internal/content/dungeons/reference-tomb.yaml
// and the toolkit cannot read across the repo boundary, so this will drift the
// day somebody edits theirs. Said out loud here because the alternative — a
// fixture that quietly stops being the shipping dungeon while every test still
// passes — is how "the reference tomb compiles" becomes a claim about nothing.

// tombYAML is reference-tomb.yaml as rpg-api ships it, with TWO lines added:
//
//   - `void: opaque` — [encounter.CanvasInput.Void] is required and has no
//     default (#1116, and #1033's law behind it). A crypt's void is stone. The
//     compiler is not allowed to infer this from `theme: crypt`, because that
//     would be the same defaulting one layer out.
//   - `orientation: pointy` — the authored [col,row] pairs are offset
//     coordinates, and offset only means something once the orientation is
//     known. Kirk: "flat and pointy top are both valid and should be settable."
//
// Both are declarations the shipping file does not yet carry, which is a
// finding rather than a fixture detail: rpg-api's copy needs the same two lines
// before it can compile on the new stack.
const tombYAML = `
version: 1
key: reference-tomb
name: The Tomb of the Captain
theme: crypt
void: opaque
orientation: pointy
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 6
    place:
      - { ref: "dnd5e:props:brazier", at: [1, 1] }
      - { ref: "dnd5e:props:brazier", at: [1, 6] }
  - id: hall
    archetype: chamber
    width: 10
    place:
      - { ref: "dnd5e:props:pillar", at: [2, 2] }
      - { ref: "dnd5e:props:pillar", at: [2, 5] }
      - { ref: "dnd5e:props:pillar", at: [6, 2] }
      - { ref: "dnd5e:props:pillar", at: [6, 5] }
      - { ref: "dnd5e:props:torch-ornate", at: [4, 1] }
      - { ref: "dnd5e:props:bone-pile", at: [8, 6] }
      - { ref: "dnd5e:monsters:skeleton", at: [5, 3], targeting: lowest-health }
      - { ref: "dnd5e:monsters:skeleton", at: [7, 5], targeting: lowest-health }
  - id: tomb
    archetype: boss
    width: 12
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5] }
    place:
      - { ref: "dnd5e:props:coffin", at: [6, 3], blocks_los: false }
      - { ref: "dnd5e:props:altar", at: [9, 3] }
      - { ref: "dnd5e:props:statue-reaper", at: [1, 1] }
      - { ref: "dnd5e:props:statue-knight-hooded", at: [1, 6] }
      - { ref: "dnd5e:props:brazier", at: [3, 1] }
      - { ref: "dnd5e:props:brazier", at: [3, 6] }
      - { ref: "dnd5e:props:candles", at: [10, 5] }
connectors:
  - { from: entrance, to: hall }
  - { from: hall, to: tomb, locked: { dc: 12, ability: dex } }
`
