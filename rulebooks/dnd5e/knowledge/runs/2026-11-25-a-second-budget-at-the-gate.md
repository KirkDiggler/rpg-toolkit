# Run — 2026-11-25 — one way to pay for an ability S1 a second budget at the gate

Slice [rpg-project#1158](https://github.com/KirkDiggler/rpg-project/issues/1158) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`046bd908`](https://github.com/KirkDiggler/rpg-toolkit/commit/046bd908).

## Shipped

- `combat.PoolSource` and `combat.WithPools`: one payment can spend the payer's slots and a pool another budget owns, checked together and spent together — [rpg-project#1158](https://github.com/KirkDiggler/rpg-project/issues/1158).

## Decided

- The source's own pools go to the source when both hold a pool of the same name; pinned by a test — [rpg-project#1158 Design](https://github.com/KirkDiggler/rpg-project/issues/1158).
- A nil source returns the base itself and a nil base returns an untyped nil `Ledger`, so `Pay` refuses it as today — [rpg-project#1158 Design 2](https://github.com/KirkDiggler/rpg-project/issues/1158).

## Found

- F1, F2 and F3 hold at `046bd908`: `Ledger` at gate.go:25, `Pay` at :99, `check` at :131, the spy at gate_test.go:43 — [Rests on comment](https://github.com/KirkDiggler/rpg-project/issues/1158#issuecomment-7145957939).
- Mutation 1 (`PoolLeft` always asks the base) fails TestPaysBonusActionAndSourcePoolTogether and TestPoolTheSourceDoesNotOwnGoesToBase — [rpg-project#1158](https://github.com/KirkDiggler/rpg-project/issues/1158).
- Mutation 2 (`SpendPool` always goes to the base) fails those two and TestSourceOwnPoolWinsOverBaseSameName — [rpg-project#1158](https://github.com/KirkDiggler/rpg-project/issues/1158).
- `go build`, `go vet`, `golangci-lint` clean; `combat/...` passes before and after; `go test -race ./combat/...` 54.9s; whole module passes at the final head — [rpg-project#1158](https://github.com/KirkDiggler/rpg-project/issues/1158).

## Mistaken

- The first mutation restore used `git checkout` on untracked files and did nothing; the file was repaired by hand and re-tested green, so mutations on new files now restore from a copy — [rpg-project#1158](https://github.com/KirkDiggler/rpg-project/issues/1158).

— cross-team agent, on behalf of KirkDiggler
