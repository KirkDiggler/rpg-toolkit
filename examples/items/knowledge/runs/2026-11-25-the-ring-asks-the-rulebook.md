# Run — 2026-11-25 — one way to pay for an ability S1157 the ring asks the rulebook

Slice [rpg-project#1157](https://github.com/KirkDiggler/rpg-project/issues/1157) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`868f209b`](https://github.com/KirkDiggler/rpg-toolkit/commit/868f209b).

## Shipped

- The module pins `rulebooks/dnd5e` at v0.230.0; `mechanics/effects` moved v0.5.0 to v0.8.0; go.sum follows — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- The ring's grant ref is `*refs.Features.SecondWind()`, its parameters are `features.SecondWindParameters`; `example:abilities:second_wind`, `windParameters` and `parseWind` are gone — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- `validateSecondWind` is `features.ValidateParameters`; the resolver asks `features.DescribeFromRef` (giver the item, no pools) and resolves `*definition.Effect.Healing` with `healing.Resolve` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- README sentences about the prototype consumer's ref now say the ring asks the rulebook's named ability — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- Test added: `TestRulebookRefusesABadRingGrant` (level 0 and unknown member, both refused at `items.New` by the rulebook's sentences) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).

## Decided

- The new test lives in `fixture_test.go` because the issue lists no other new test file — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).

## Found

- F1 to F5 held; F5 also found `loot_example_test.go:66` (`case windRef:`), which compiles unchanged against the new `windRef` — [Rests on comment](https://github.com/KirkDiggler/rpg-project/issues/1157#issuecomment-7145624635).
- go.mod diff: `mechanics/effects v0.5.0 => v0.8.0`, `rulebooks/dnd5e v0.215.0 => v0.230.0` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- Baseline at origin/main: suite ok, 48 tests run (`=== RUN` lines); pin alone: ok, 48, nothing failed; final head: ok, 51 (three added: one test, two subtests) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- Assertions changed: none; the example output (healing calculation 9, charges 0/0/0/1) is unchanged — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
- Gates at head: build, vet, test, race test (8.2s) and golangci-lint (0 issues) pass — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).

## Mistaken

- none observed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1157).
