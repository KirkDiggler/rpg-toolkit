// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// tombsource_test.go holds the authored dungeon every test in this package is
// about, in one place, because two very different suites need the same bytes:
// dialect_test.go mutates one line of it at a time to check what gets refused,
// and tomb_test.go compiles it and walks it.
//
// # It is the toolkit's own, not a copy of rpg-api's
//
// Kirk's ruling: "the current version has never run in a game… it is legacy and
// will have no home after." So this is the reference tomb RE-AUTHORED in the
// dialect this package speaks, rather than rpg-api's shipping file plus
// patches. The rooms, their widths, the props and monsters and where they
// stand, the locked DC-12 connector — the dungeon is the same dungeon. What
// changed is everything the old dialect said that this one does not have a
// word for, and everything this one insists on that the old one guessed.
//
// # What went, and why
//
//   - `theme: crypt` — carried and never read, and the ONE thing it looked
//     like it should decide (that the void is stone) is exactly what it may
//     not: see `void:` below.
//   - `name:` — a display title with no reader here. The toolkit compiles a
//     world; what a dungeon is CALLED belongs beside the catalogue entry that
//     names the file.
//   - `archetype:` — the old compiler read `archetype: entrance` to decide
//     where the party stood, which is a word about what a room is FOR quietly
//     deciding geometry. `start:` says it.
//   - `boss:` as its own key beside `place:` — a boss is a monster with a
//     flag, and two lists meant the rules about cells, refs and collisions
//     were written twice.
//
// # What it now insists on
//
//   - `void: opaque` — [encounter.CanvasInput.Void] has no default by ruling
//     (rpg-toolkit#1116, and #1033's law behind it), and no room list can tell
//     a tomb cut from rock apart from an open-air ruin.
//   - `orientation: pointy` — every `at:` in this file is an OFFSET pair, and
//     an offset pair means nothing until the orientation is known.
//   - `blocks_movement` and `blocks_los` ON EVERY PROP. The old dialect read a
//     missing flag as "blocks", and [encounter.PropInput]'s doc comment names
//     that default as the thing it drops. Writing them out is what turns this
//     from a room full of walls into a room: braziers you see over, bones you
//     walk through, candles that are neither, and a coffin that is both ways
//     round.
//   - `start:` — where the party comes in, said rather than derived.
const tombYAML = `
version: 1
key: reference-tomb
void: opaque
orientation: pointy
height: 8
start: [1, 3]
rooms:
  - id: entrance
    width: 6
    place:
      - { ref: "dnd5e:props:brazier", at: [1, 1], blocks_movement: true,  blocks_los: false }
      - { ref: "dnd5e:props:brazier", at: [1, 6], blocks_movement: true,  blocks_los: false }
  - id: hall
    width: 10
    place:
      - { ref: "dnd5e:props:pillar", at: [2, 2], blocks_movement: true,  blocks_los: true }
      - { ref: "dnd5e:props:pillar", at: [2, 5], blocks_movement: true,  blocks_los: true }
      - { ref: "dnd5e:props:pillar", at: [6, 2], blocks_movement: true,  blocks_los: true }
      - { ref: "dnd5e:props:pillar", at: [6, 5], blocks_movement: true,  blocks_los: true }
      - { ref: "dnd5e:props:torch-ornate", at: [4, 1], blocks_movement: false, blocks_los: false }
      - { ref: "dnd5e:props:bone-pile", at: [8, 6], blocks_movement: false, blocks_los: false }
      - { ref: "dnd5e:monsters:skeleton", at: [5, 3], targeting: lowest-health }
      - { ref: "dnd5e:monsters:skeleton", at: [7, 5], targeting: lowest-health }
  - id: tomb
    width: 12
    place:
      - { ref: "dnd5e:props:coffin", at: [6, 3], blocks_movement: true,  blocks_los: false }
      - { ref: "dnd5e:props:altar", at: [9, 3], blocks_movement: true,  blocks_los: false }
      - { ref: "dnd5e:props:statue-reaper", at: [1, 1], blocks_movement: true,  blocks_los: true }
      - { ref: "dnd5e:props:statue-knight-hooded", at: [1, 6], blocks_movement: true,  blocks_los: true }
      - { ref: "dnd5e:props:brazier", at: [3, 1], blocks_movement: true,  blocks_los: false }
      - { ref: "dnd5e:props:brazier", at: [3, 6], blocks_movement: true,  blocks_los: false }
      - { ref: "dnd5e:props:candles", at: [10, 5], blocks_movement: false, blocks_los: false }
      - { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5], targeting: closest, boss: true }
connectors:
  - { from: entrance, to: hall }
  - { from: hall, to: tomb, locked: { dc: 12, ability: dex } }
`
