# Run — 2026-11-04 — Keep signature combat and loot S1 consumer probe

Slice [rpg-project#862](https://github.com/KirkDiggler/rpg-project/issues/862), story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [3182088c](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21).

## Shipped

- No release or production item API; a test-only `examples/items` consumer exercises the effects/resource/healing seams through real pinned dependencies — [probe](../../examples/items/README.md).

## Decided

- KirkDiggler selected one charge for the first ring fixture and redirected the investigation from final configuration questions to an example consumer and the seams — [direction](https://github.com/KirkDiggler/rpg-project/issues/862#issuecomment-6648142433).
- The probe keeps candidate wiring in test files, explicitly supplies a holder rather than inventing equipment eligibility, returns healing without HP mutation, and makes no production action/transaction claim — [probe assumptions](../../examples/items/README.md#probe-brief).

- KirkDiggler requested readable construction inputs before growing the consumer; this probe now pins the separate provider cleanup and names its recipe, vocabulary, holder and source at the call site — [provider PR #2276](https://github.com/KirkDiggler/rpg-toolkit/pull/2276).

- KirkDiggler refined the construction convention to package-qualified New(Config), avoiding NewInput and stuttering type names; the consumer follows the revised provider without changing its probe behavior — [provider PR #2276](https://github.com/KirkDiggler/rpg-toolkit/pull/2276).

## Found

- Adopting the pushed effects commit rejects the old positional call at compile time; migrating to ComposeInput preserves the full item suite, runnable example, race, vet and lint results — [named consumer input](../../examples/items/fixture_test.go).

- Real effects attachment can supply a local grant and targeted recovery subscription; real resources track its independent budget, and real healing produces 7/9 for level-1/3 with a controlled d10 of 6 — [consumer tests](../../examples/items/items_test.go).
- Two same-recipe instances receive identical recipe-local output IDs; distinct item sources and pools come from consumer context, not `effects.Host.Source` content identity — [observed boundaries](../../examples/items/README.md#observed-seams-and-limits).
- The local offer does not yet cross the real action/payment/HP-delivery boundary, and pool mutations outside the machine bag are not reported through its Changed hook — [limits](../../examples/items/README.md#observed-seams-and-limits).
- All eight suite cases and the runnable example pass; race tests, vet and lint pass. Removing the recovery holder filter fails the transfer test (old holder refills new holder's item), restoring it passes — [test and mutation target](../../examples/items/items_test.go).

## Mistaken

- The initial test-only scaffold did not compile before its fixture helpers existed; this was a missing consumer, not a failing provider behavior. One line-length lint finding was corrected before publication — [fixture and tests](../../examples/items/items_test.go).
