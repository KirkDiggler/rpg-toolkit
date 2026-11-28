# Run — 2026-11-28 — Fighter's Action Surge S6g the old extra-action grant is deleted

Slice [rpg-project#1273](https://github.com/KirkDiggler/rpg-project/issues/1273) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`1c4b00bc`](https://github.com/KirkDiggler/rpg-toolkit/commit/1c4b00bc4fa6885879a7f3143d649255b3265b68).

## Shipped

- `(*ActionEconomy).GrantExtraAction` and `GrantExtraBonusAction` are deleted from `combat/action_economy.go`; a turn slot is given one way, `GrantSlots` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).
- `features.FeatureInput.ActionEconomy` is deleted, and its now-unused `combat` import with it — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).
- The comments that named the old pair say what is now true (`GrantSlots` is the one way; the `ActionsRemaining` tag no longer says Action Surge gives +1) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).

## Decided

- `shared.ClassResourceActionSurge` stays: it is one value of a general numbered list of class resources, and removing it would renumber the values after it — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).
- The four `GrantExtra` suites and cases are repointed, not retired, because `GrantSlots` had no test that it adds, that a depleted slot is refilled, or that twice gives two; they are renamed `TestGrantSlotsStandard` and `TestGrantSlotsBonus` with the same assertions — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1273#issuecomment-7232784962).

## Found

- F1: non-test hits for `GrantExtraAction|GrantExtraBonusAction` at `1c4b00bc` were the two definitions and one comment in `action_economy_ledger.go:173`; rpg-api `origin/dev` has none — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1273#issuecomment-7232784962).
- F2: no `FeatureInput` sets or reads `ActionEconomy`; every other `ActionEconomy` hit is `combatabilities.CombatAbilityInput`, `character.Data` or a resolution sheet; rpg-api has no `FeatureInput` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1273#issuecomment-7232784962).
- `encounter`, `resolution` and `session` each pass `go build ./...` and `go vet ./...` against this tree through a temporary `-modfile` replace; no `go.mod` changed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).
- In this module `go build`, `go vet`, `go test ./... -count=1` and `golangci-lint run ./...` pass, before and after — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).

## Mistaken

- Deleting the field left `features/types.go` importing `combat` unused; the build said so and the import went — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).
- The first three-module build ran with a replace that never applied (`go mod edit` outside the module); it was rerun from each module and the replace counted — [issue](https://github.com/KirkDiggler/rpg-project/issues/1273).
