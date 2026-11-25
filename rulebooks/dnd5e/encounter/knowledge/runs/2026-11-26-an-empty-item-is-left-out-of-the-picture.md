# Run — 2026-11-26 — item story: an empty item is left out of the picture

Slice [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`b4bfd2ff`](https://github.com/KirkDiggler/rpg-toolkit/commit/b4bfd2ff).

## Shipped

- `PlacedPropInput.Item`, `MemberInput.Equipment` and `JoinInput.Equipment` are tagged `json:"<Name>,omitempty"`, keeping the names they marshal under — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).
- Two guard tests in `dungeonspec/an_empty_item_test.go`: a site with no items has no `"Item"` or `"Equipment"` key in its picture; the custom-items site has both, with the cloak's id — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).

## Decided

- The guard compiles `testdata/world-builder-v4-raider-letter.yaml` (one placed prop, creatures), because it is an existing site with both and no custom items — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).
- `TakenItem.Grant` (`loot.go:89`, `json:"grant"`) is left as it is: a plain struct inside a loot event payload, not a field of the compiled site, and it cannot be omitted when empty — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).

## Found

- F1 to F3 hold at `b4bfd2ff`: `placed_props.go:184`, `field.go:1124`, `:1768`, `:2102`, `dungeonspec/compile.go:351` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1173#issuecomment-7174197393).
- F4: before the tags the first guard case fails with `"Item": null` appearing 1 time, the raider letter having one placed prop — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1173#issuecomment-7174197393).
- Design 2: exported fields typed `ItemGrant`, `*ItemGrant` or `[]ItemGrant` and their tags: `PlacedPropInput.Item` now `Item,omitempty`; `MemberInput.Equipment` now `Equipment,omitempty`; `JoinInput.Equipment` now `Equipment,omitempty`; `PlacedPropData.Item` (`data.go:504`) `item,omitempty` already; `MemberData.Equipment` (`data.go:590`) `equipment,omitempty` already; the reserve record's `Equipment` (`data.go:2388`) `equipment,omitempty` already; `dungeonspec.CompiledMonster.Equipment` (`compile.go:351`) `Equipment,omitempty` already; `TakenItem.Grant` not site data, untagged for omission. No fourth field needed a tag — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).
- The member's own record (`field.go:1768`, `memberRecord`) is unexported and never marshalled — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).
- The custom-items site's picture is byte-identical before and after the tags (7160 bytes, `cmp`) — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).
- Proof the test bites: with the tag removed from `PlacedPropInput.Item`, `TestDungeonspec_ASiteWithNoItemsHasNoItemKeysInItsPicture` fails; restored after — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).
- Suite: `go build`, `go vet`, `golangci-lint` (0 issues) and `go test ./... -count=1` pass in `rulebooks/dnd5e/encounter` before the change (dungeonspec, the package the new file joins) and at the final head (every package) — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).

## Mistaken

- The guard's cloak assertion first looked for `"ID"`; the grant marshals `"id"`. Caught on the first run — [rpg-project#1173](https://github.com/KirkDiggler/rpg-project/issues/1173).

— cross-team agent, on behalf of KirkDiggler
