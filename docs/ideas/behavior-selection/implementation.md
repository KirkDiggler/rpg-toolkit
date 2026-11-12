# Behavior selection implementation evidence

[Story#917](https://github.com/KirkDiggler/rpg-project/issues/917), [S1#918](https://github.com/KirkDiggler/rpg-project/issues/918), [toolkit PR#2351](https://github.com/KirkDiggler/rpg-toolkit/pull/2351). Baseline mainba6d40f2.

## Implemented

- `mind/behavior/selection`: captured, detached evaluation; named Unknown/Unmet/Met requirements; named integer factors; checked shares; inspection, explicit choice and one-die draw.
- Real `behavior.Pick` adoption per table level; original When/affordability reading, group structure, path, row verdicts, raw die sizes and hold/silence semantics retained for valid inputs.
- `behavior.Deal` adoption over sorted words; no competing weighted landing loop retained.
- Executable guard/profile and player-reply example; this is not a new personality catalog or player-dialogue transport/UI.

## Proof

New-package API compile-RED precedes evaluator and choice implementation. Integration tests fail on the old picker returning fabricated choices for invalid dice and an overflow-as-hold result; empty/bad/overflowing deals also fail before adoption. These cases pass with explicit selection errors and no extra roll. The whole owning-module race suite passes, with existing group/composer/trace controls unchanged. Full-module lint reports0issues.

A virtual mutation treating Unknown as eligible fails `TestRequirementsAndWeights`: eligible count3 instead of2, total1010 instead of10 and mismatched row evidence. Production source is not modified by that probe.

## Measured cost

Same existing `BenchmarkComposerDecide`, three runs on the same machine, one selected flat row/two requesting roles, real RNG and world execution excluded:

| | Before | After |
|---|---|---|
| Time |13.075–13.611µs/op|14.813–15.134µs/op|
| Allocation bytes |4052–4053B/op|5244–5247B/op|
| Allocations |34/op|50/op|

The captured pool and detached inspection/choice evidence add roughly1.2KiB and16allocations in this fixture. This is a measured cost, not a gameplay latency measurement or evidence for #879's targets. No performance cleanup or unmeasured caching is folded into this slice.

## Qualifications and changes to the plan

- `dealTemper` contained another weighted loop. Task3 includes its adoption to fulfill the agreed single-primitive boundary; no new temperament policy was added.
- Nil draw contexts are explicitly refused. The deliberate nil-context test has one documented line-local SA1012 annotation; runtime checks and global lint policy remain intact.
- Full module tests, uncached hook, exact-head remote execution and independent PR review are the readiness evidence on the PR. This document alone does not claim those remote/review gates or authorize merge.
- No go.mod, wire, persistence, API/web, old-selectables or live-stack change is made. Releasing the provider remains separate from consuming a future tag.
