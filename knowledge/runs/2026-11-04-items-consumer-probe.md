# Run — 2026-11-04 — Keep signature combat and loot S1 consumer probe

Slice [rpg-project#862](https://github.com/KirkDiggler/rpg-project/issues/862), story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [3182088c](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21).

## Shipped

- Initial checkpoint (6169f235): test-only wiring exercises the effects/resource/healing seams, without a public item API — [original probe](https://github.com/KirkDiggler/rpg-toolkit/blob/6169f235/examples/items/README.md).
- 2026-11-05 checkpoint: an exported experimental Item owns grants, pools, association and typed use; no extraction or production adoption is claimed — [candidate API](../../examples/items/README.md#candidate-api).

## Decided

- KirkDiggler selected one charge for the first ring fixture and redirected the investigation from final configuration questions to an example consumer and the seams — [direction](https://github.com/KirkDiggler/rpg-project/issues/862#issuecomment-6648142433).
- The initial probe keeps candidate wiring in test files and supplies a holder rather than inventing eligibility, because the first question is whether existing providers can compose — [initial assumptions](https://github.com/KirkDiggler/rpg-toolkit/blob/6169f235/examples/items/README.md#probe-brief).

- KirkDiggler requested readable construction inputs before growing the consumer; this probe now pins the separate provider cleanup and names its recipe, vocabulary, holder and source at the call site — [provider PR #2276](https://github.com/KirkDiggler/rpg-toolkit/pull/2276).

- KirkDiggler refined the construction convention to package-qualified New(Config), avoiding NewInput and stuttering type names; the consumer follows the revised provider without changing its probe behavior — [provider PR #2276](https://github.com/KirkDiggler/rpg-toolkit/pull/2276).

- KirkDiggler authorized merging the effects cleanup and continuing the item example on 2026-11-05; provider #2276 merged at b5b2c101, and the consumer pins that canonical main commit rather than an unmerged branch — [merged provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2276).
- The candidate keeps Item/Config non-generic but uses one generic Use operation to return the consumer's typed result without importing rulebook types or converting results through JSON; its pure-resolution contract is an experiment, not a general rollback guarantee — [experiment plan](../../examples/items/PLAN.md).

## Found

- The initial named-input adoption rejects the old positional call at compile time, then passes the suite after migration to ComposeInput; later naming follows New(Config) — [intermediate consumer at c3460166](https://github.com/KirkDiggler/rpg-toolkit/blob/c3460166/examples/items/fixture_test.go).

- Real effects attachment can supply a local grant and targeted recovery subscription; real resources track its independent budget, and real healing produces 7/9 for level-1/3 with a controlled d10 of 6 — [consumer tests](../../examples/items/items_test.go).
- Two same-recipe instances receive identical recipe-local output IDs; distinct item sources and pools come from consumer context, not `effects.Host.Source` content identity — [initial observed boundaries](https://github.com/KirkDiggler/rpg-toolkit/blob/6169f235/examples/items/README.md#observed-seams-and-limits).
- The candidate grant/use protocol still does not cross the production action/actor-payment/HP-delivery boundary; pool state is explicitly snapshotted rather than reported through the machine's Changed hook — [remaining limits](../../examples/items/README.md#boundaries-still-to-learn).
- Initial checkpoint: eight suite cases and one example pass; removing holder scoping fails the transfer test — [initial tests](https://github.com/KirkDiggler/rpg-toolkit/blob/6169f235/examples/items/items_test.go).
- The exported candidate preserves those cases and adds copied-view/state integrity, cancellation, validation, shared pool-reference and import-boundary checks. Wrong recovery target, zero payment and aliased parameter-buffer mutants are caught — [package consumer tests](../../examples/items/items_test.go).

- The new consumer uses an example-owned ability ref for its parameter schema rather than claiming compatibility with the production feature loader; only the real healing operation is reused here — [consumer vocabulary](../../examples/items/fixture_test.go).

## Mistaken

- A malformed import edit was caught by go fmt before commit; the known-good test file was restored and only the import grouping reapplied — [checked test source](../../examples/items/items_test.go).

- The exported-package checkpoint's local lint used the older base config; CI's merge tree included newly enabled goimports grouping from #2279. Merging main reproduced the failure locally; separating testify from local imports fixes the code without weakening the policy — [failing CI evidence](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42101345612/job/127504747950).
- An ad hoc Markdown-link check initially mistook an inline generic Go signature for a link; excluding code spans/fences verifies the real links. The source documents did not need that false-positive edit — [experiment documentation](../../examples/items/README.md).

- The initial test-only scaffold did not compile before its fixture helpers existed; this was a missing consumer, not a failing provider behavior. One line-length lint finding was corrected before publication — [fixture and tests](../../examples/items/items_test.go).
