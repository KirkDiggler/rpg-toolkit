# Run — 2026-11-26 — item story S1 a site file says custom items

Slice [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`3e03721e`](https://github.com/KirkDiggler/rpg-toolkit/commit/3e03721e).

One line per entry. Every line ends with a link.

## Shipped

- A room-dialect site file says `customItems`, a creature's `equipment` and a prop's `item`; the format judges their shape, the compiled site carries them as canonical JSON, the encounter keeps them across a save and the read-back says them; nothing in a run uses them — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).
- The shape grammar, the canonical JSON and the grants are in `dungeonspec/custom_items.go` and `itemgrant.go`; the statements in `dungeonspec/readback/custom_items.go` — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).

## Decided

- Test site copied from `testdata/world-builder-v4-raider-letter.yaml` into `testdata/custom-items/site.yaml`, a subdirectory, because the content-picture net takes every `testdata/*.yaml` and demands a committed picture and a pinned file list — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).
- Only `PlacedPropInput` gains `Item`: `PropInput` is built only by the v2 dialect, `compilefield.go` and the loader, none reachable from a room-dialect file — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1166#issuecomment-7171787043).
- The compiled carrier for a creature is `MonsterPlacement.Equipment`, which a host copies onto `MemberInput.Equipment`; equipment is saved in `MemberData` and `ReserveData` (not a seed like `Holds`), a prop's item in `PlacedPropData` (part of the site piece and its hash) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1166#issuecomment-7171787043).
- YAML decodes maps nested in a named map type as that named type, so items are copied to plain `map[string]any` before the unwritable check and the marshal — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).
- Amendment 1: the read-back call sites in `place.go` and `creatures.go`, and the new word inserted in the expected sentence of seven unknown-key tests — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1166#issuecomment-7172106995).

## Found

- F5: `MemberInput` is built only by hosts (`cmd/freeroam-workbench/main.go:136`, tests) from `MonsterPlacement`; a placed prop is built at `single_room_placement.go:103` and written by `applyPropBindings` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1166#issuecomment-7171787043).
- Every new compiled field is omit-when-empty; no committed picture or golden changed (`git status` shows only new files under `testdata/`); the castle hash moved `ef6511b879c51cd92128e754d608c5fecfd1ef54734342436a2d423a475ab3fa` to `e3b4252104424f014d2df14d2916a6998c3421d27d0131db4875c8a0a2585312` because compiled types gained fields — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).
- The unknown-key refusal lists the keys reflected from the structs and seven tests pin that sentence as a literal — [gap 2](https://github.com/KirkDiggler/rpg-project/issues/1166#issuecomment-7172097206).
- The suite before the change: all packages ok (`go test ./... -count=1 -timeout 20m`, 59s wall); at the final head: all packages ok, `golangci-lint` 0 issues — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).
- Proofs: with the `equipment` declared-item check disabled `TestDungeonspec_CustomItemsAreRefusedWhereTheyAreWrong/equipment_naming_an_item_that_is_not_declared` fails; with equipment dropped from `ToData` `TestCustomItems_TheEncounterKeepsWhoHasThemAndWhichPropIsOne` and `TestCustomItems_LoadRefusesAMalformedGrant` fail — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).

## Mistaken

- A first test copy treated nested maps as plain `map[string]any` and refused the author's own `charges`; the shape test on the real site caught it before any commit — [rpg-project#1166](https://github.com/KirkDiggler/rpg-project/issues/1166).
- Two gaps (read-back call sites, pinned key lists) were found only after the build, because the issue's file list did not trace which existing code emits statements and pins sentences; both reported and ruled — [gap 1](https://github.com/KirkDiggler/rpg-project/issues/1166#issuecomment-7172096685).
