# Run — 2026-11-04 — #2254 — resolution's lint drain

processed: 2026-11-12T165555Z

Part of [rpg-toolkit#2254](https://github.com/KirkDiggler/rpg-toolkit/issues/2254). Base: `rpg-toolkit` `main` at 6d8a753a. The debt: 88 findings under the live root config (#2153's limits), the first of the four dam-drain modules.

## Shipped

- All 88 findings fixed: 12 test-file import groups (formatter), 44 unnecessary conversions dropped, 9 preallocations, 11 test-helper params that were always one constant (plus their ~120 call sites), 5 gocritic findings (3 if-else chains → switch, a `Deprecated:` paragraph, an append-assign), 2 line-length wraps, 2 trailing newlines — [#2254](https://github.com/KirkDiggler/rpg-toolkit/issues/2254).
- Two product seams simplified: `newAttackCast` drops its always-nil error return (callers in action.go and cast_pose.go read plainer), and `done` in contest.go answers `Step` with the error spelled where it is consumed — [#2254](https://github.com/KirkDiggler/rpg-toolkit/issues/2254).

## Decided

- The wave's first `//nolint:unparam`, with its because in the comment: `deliverMove`'s signature is forced (payForMove's continuation shape) and its error is not absent but deferred — imposeMove hands back a Gather that runs, and can fail, inside the step machine. The alternative was inventing an error path that cannot fire — [contest.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/contest.go).
- Constant params became literals at their bodies rather than staying parameters no caller varies: baneCaster's 2 slots, rectRegion's row 0, resistanceJSON's member and cleric, conditionRefs' hero, economy's firstTurn — the helper says what it always meant — [the test files](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution).
- `held` keeps its `id` param — it serves varying members — even though its body sits one screen from conditionRefs'; the suite proved the distinction within one run — [concentration_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/concentration_test.go).

## Found

- unparam recurses: dropping a constant param exposes the next one (resistanceJSON lost memberID first, then casterID) — both rounds fixed in this PR — [#2254](https://github.com/KirkDiggler/rpg-toolkit/issues/2254).
- The module is clean under BOTH configs today: the root's full set (enforced locally, `-c`) and the dam's standard+nilnil (today's CI) — the module is ready for the dam's deletion the moment its siblings drain — [#2254](https://github.com/KirkDiggler/rpg-toolkit/issues/2254).

## Mistaken

- A blanket replace hit two functions: `held`'s body took conditionRefs' `heroID` hardcode and broke holds on non-hero members — the suite caught it one run later, the param restored — [concentration_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/concentration_test.go).
- Caller-regex passes missed shapes as they came: string-literal caster ids (`"nobody"`), table-driven args (`tc.actions`), cross-file callers (resistanceJSON in pause_scenes). Each a vet error, each fixed in a pass — the build was the net every time — [#2254](https://github.com/KirkDiggler/rpg-toolkit/issues/2254).