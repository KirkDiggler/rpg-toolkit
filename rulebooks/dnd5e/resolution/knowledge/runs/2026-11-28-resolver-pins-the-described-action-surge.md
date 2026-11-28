# Run — 2026-11-28 — fighter's Action Surge S6b the resolver pins the rules where it is described

Slice [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`71c8b97e`](https://github.com/KirkDiggler/rpg-toolkit/commit/71c8b97e).

One line per entry. Every line ends with a link: a PR, an issue comment, a commit, or a design section anchor.

## Shipped

- `rulebooks/dnd5e` pinned at v0.249.0 in this module, alone in its own commit, with no production change — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- The level-4 fighter's long-rest fixture holds her Action Surge count in `resources.ActionSurge` (0 of 1); the test reads 1 of 1 from the pool after the rest — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- Two new tests in `fighter_action_surge_test.go`: her own Action Surge resolved at the door (pool 1 to 0, two actions, outcome "1 action", refused the second time) and the feature path refusing it — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).

## Decided

- The new tests are methods of the activation suite in a new file, reusing `fighterAt`, `resolveEffect` and `effectCost` from `effect_test.go` rather than a third way to resolve a description; the issue allowed either file — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- No test builds a feature JSON carrying `uses` or `max_uses` for Action Surge; the operator ruled on 2026-11-28 that the old way is not tested — [rpg-project#1131 ruling](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7232312437).
- The long-rest test also asserts `dnd5e:features:action_surge` is in `Refilled`; the issue asked what the list holds, and an assertion states it — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- The unused helper `featureWithRef` was deleted from `long_rest_test.go`: case (a) removed its only caller and the linter refuses an unused function — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).

## Found

- After the pin alone `go build ./...` passes and `go vet`/`go test` fail to compile at `long_rest_test.go:132,133,303` (`Uses`, `MaxUses`) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1270#issuecomment-7232471426).
- With only those fields removed (scratch, not committed) exactly two tests fail, both with `feature fighter-action-surge reported malformed status: owner does not carry action_surge`: `TestFighterRecoveryIsCompleteAndIndependent` and `TestPersistedMetadataSurvivesAndUpdatedAtRefreshes`; the bus-subscription counters did not move — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1270#issuecomment-7232471426).
- The `Refilled` list of the level-4 fighter holds three entries: `dnd5e:features:action_surge`, `dnd5e:features:second_wind`, `dnd5e:resources:fighter-rest-pool`; the pool is named by its feature ref, not a resource ref — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- Her `got.Resources` holds 4 entries (hit dice, the rest pool, second wind, action surge), where it held 3; the barbarian's count is unchanged — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- Test 3 sees `resolution: ability refused to activate: this ability is used from its description: build the action with Definition and pay its cost at the gate` — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- On the old pin (v0.246.0, scratch) Test 2 fails at "the level-2 fighter holds Action Surge, and it carries a description" and Test 3 fails with `no action surge uses remaining`, so both fail for the stated reason before the pin — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- Proof (a): with her pool starting at 1 of 1 the `Current == 1` read still passes, so it cannot tell a restoring rest from a no-op; the `Refilled` assertion fails (`does not contain "dnd5e:features:action_surge"`), which is why the fixture starts at 0 and `Refilled` is asserted. Setting the expected value to 0 fails with expected 0, actual 1 — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- Proof (b): asserting three actions in Test 2 fails with expected 3, actual 2 — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
- Suite results in this module: pass before the pin; compile failure after the pin alone; pass at the final head, with build, vet and `golangci-lint run ./...` clean — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).

## Mistaken

- none observed — [rpg-project#1270](https://github.com/KirkDiggler/rpg-project/issues/1270).
