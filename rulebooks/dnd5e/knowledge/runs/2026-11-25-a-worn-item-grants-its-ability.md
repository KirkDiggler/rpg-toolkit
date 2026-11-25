# Run — 2026-11-25 — the item story: a worn item grants its ability and pays from its own count

Slice [rpg-project#1163](https://github.com/KirkDiggler/rpg-project/issues/1163) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`cc4c5009`](https://github.com/KirkDiggler/rpg-toolkit/commit/cc4c5009df509c6d6b8d2fc6c9d346b6185ff25f).

One line per entry. Every line ends with a link.

## Shipped

- `AvailableAbility.Giver` and `AbilityGiver{ItemID, ItemName, Grant}`: a worn item's grants are listed beside the character's own — [slice Design 1](https://github.com/KirkDiggler/rpg-project/issues/1163).
- `GrantedAbilityInput`, `GrantedAbilityOutput{Definition}`, `(*Character).GrantedAbility`, `ItemPoolKey`: the cost names the item's pool and one `combat.Pay(c, cost)` pays it — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1163#issuecomment-7149180112).
- `Character.PoolLeft` and `SpendPool` answer for a worn item's pools after the character's own — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1163#issuecomment-7149180112).
- A short and a long rest refill every owned item's pools by `restSatisfies` — [slice Design 4](https://github.com/KirkDiggler/rpg-project/issues/1163).

## Decided

- A grant whose description does not cost exactly one slot is listed with `CanUse` false and the reason, never dropped; Second Wind cannot do this today — [slice, "When this issue does not cover it"](https://github.com/KirkDiggler/rpg-project/issues/1163).
- An item pool's key at the gate is the pool's own name (`charges`), as the issue's Test 3 states — [slice Test 3](https://github.com/KirkDiggler/rpg-project/issues/1163).

## Found

- F1 to F5 hold at `cc4c5009`; F4's two calls are `character.go:504` and `:690`; F6 prints only `character/` lines (`action_economy.go:190,192,321,322,329,356`, `action_economy_types.go:79,81,102,155`) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1163#issuecomment-7149127566).
- A level 3 sheet loaded from data without feature blobs carries no class feature, so the fighter in the tests is the draft-made level 1 fighter, whose own Second Wind exists — [slice Test 1](https://github.com/KirkDiggler/rpg-project/issues/1163).
- Proofs: `SpendPool` ignoring item keys fails the one-payment, empty-cloak, saved-data (Test 11) and two-ring tests; listing unworn items fails the pack test; skipping items in the rest fails the rests test — [slice, "Prove the tests bite"](https://github.com/KirkDiggler/rpg-project/issues/1163).
- Results: `go build`, `go vet`, `golangci-lint` (0 issues), `go test ./...` all pass at the final head; `-race` on `Item|Worn|Granted` passes in 22s — [slice, "Commands that print pass"](https://github.com/KirkDiggler/rpg-project/issues/1163).

## Mistaken

- A separate `Budget` (a `combat.PoolSource` bound to the live character) was built first and removed in a later commit under amendment 1: the resolver loads its own character from saved data, so a bound budget would debit the wrong copy and the use would be free; Test 11 pins it — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1163#issuecomment-7149180112).
- The first mutation did not compile (an unused variable), so it proved nothing until redone; the redo fails four tests — [slice, "Prove the tests bite"](https://github.com/KirkDiggler/rpg-project/issues/1163).
- The "before" run of the four-package suite was not taken on `origin/main`; only the "after" run was — [slice, "Commands that print pass"](https://github.com/KirkDiggler/rpg-project/issues/1163).

— cross-team agent, on behalf of KirkDiggler
