# Run — 2026-11-28 — fighter's Action Surge 6c: the session pins the rules and the resolver

Slice [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`121a4644`](https://github.com/KirkDiggler/rpg-toolkit/commit/121a4644).

One line per entry. Every line ends with a link.

## Shipped

- `go.mod` pins `rulebooks/dnd5e v0.249.0` and `resolution v0.86.1`, committed alone first — [commit 74db3f1a](https://github.com/KirkDiggler/rpg-toolkit/commit/74db3f1a).
- The doc comment on `Manager.Activate` says a feature that carries a description (Second Wind, Action Surge) is paid at the door; no other production change — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).
- `fighter_action_surge_test.go`: Tests 1 to 6, a level-2 fighter's own Action Surge arrives with the level, is used through the door, gives two attacks and not three, needs no action or bonus action, is once until a short rest, and a short rest gives it back — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).

## Decided

- The level-2 fighter is the level-1 fighter that has earned level 2 (`advancingFighter`, given a longsword), taken to level 2 through `Manager.LevelUp`; no test builds a sheet in the old shape, as the operator ruled 2026-11-28 — [rpg-project#1131 ruling](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7232312437).
- Test 6 lays her sheet down with the pool at 0 of 1 in free roam (a rest is refused in a fight), rests through `Manager.Rest`, then authors the turn clock to see the row usable — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).
- Test 4 spends her bonus action by writing it on her saved sheet, because a level-2 fighter has no bonus-action ability of her own on this fixture — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).

## Found

- F5: before this story her own Action Surge through the session was offered but unavailable (`action economy is required for action surge`), `Manager.Activate` was refused `activate: declaration is stale`, and her sheet had no `action_surge` pool — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1271#issuecomment-7232576578).
- F6: after the pins alone build, vet and the whole suite pass; no existing test failed and none is edited — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1271#issuecomment-7232576578).
- Test 2 beats: an `activated` beat naming `dnd5e:features:action_surge` "Action Surge" with no target, then an `activation-result` of kind `capacity-granted` described "1 action"; `Activate` gives `GrantedCapacity` "1 action", the pool reads 0 of 1 and she has 2 actions — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).
- Test 7 (scratch, not committed): outside a fight her row is offered and available, `Manager.Activate` succeeds and spends the use (pool 0 of 1, then "no action surge uses remaining") while her economy stays nil, so no action is given: the use is spent for nothing, the same as the wearer's — [rpg-toolkit#2626](https://github.com/KirkDiggler/rpg-toolkit/issues/2626).
- Proof (a), rules pin back at `v0.248.0`: Tests 1 to 6 all fail (no `action_surge` pool, so 1 of 1 reads 0 of 0; the row is unavailable so Activate is "declaration is stale"; a rest restores nothing) — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).
- Proof (b), a third attack expected to succeed in Test 3: fails with `attack: declaration is stale` — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).
- Suite at `go test ./... -count=1`: passes before, after the pins alone and at the final head; `golangci-lint` 0 issues — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).

## Mistaken

- none observed — [rpg-project#1271](https://github.com/KirkDiggler/rpg-project/issues/1271).
