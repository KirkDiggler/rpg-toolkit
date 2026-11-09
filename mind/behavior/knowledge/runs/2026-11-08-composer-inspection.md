# Run — 2026-11-08 — composer proposal and commit explanations

## Shipped

- Added opt-in instance outcomes to Observe/Decide, consuming the pushed machine trace provider; this is provider work, not completed live inspection — [Machine inspection S1](https://github.com/KirkDiggler/rpg-project/issues/899).

## Decided

- Record selection and commit at their actual execution sites rather than re-derive them in the formatter, because a deferred preview, a no-candidate hold and a data-only update have different commit authority — [R3](https://github.com/KirkDiggler/rpg-project/pull/902).
- Keep the preview's proposed After separate from the composer's actual After; sleeping and non-addressed roles have no invented machine step — [R2–R3](https://github.com/KirkDiggler/rpg-project/pull/902).
- Leave table eligibility and rolls untouched: only the selected role reaches the picker, and existing row verdicts remain its explanation — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).

## Found

- Full owning-module race and lint pass; trace off/on produces identical state and pick, with two actual guard reads and one d100 selection in the two-role control — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).
- The losing proposal contains a Set and a new state, but neither is committed; returned traces cannot mutate retained progress or configuration. No-candidate and explicit Hold, future wake, targeted observations and atomic refusal are exercised — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).
- A due wake can be consumed without a fired transition. The fired-as-committed overlay fails the real commit assertion, and source hash is unchanged — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).

## Mistaken

- None observed; initial RED was the missing opt-in/outcome interface, followed by the assertion-killed commit-disposition mutant — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).
