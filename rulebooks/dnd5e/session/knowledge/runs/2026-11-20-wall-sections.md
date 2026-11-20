# Run — 2026-11-20 — Wall-section delivery at the host seam

## Shipped

- Candidate wall-section DTO/projection and reveal decode retain opaque refs, wall-local IDs and canonical-foot spans without a live-world lookup — [Courtyard S6](https://github.com/KirkDiggler/rpg-project/issues/1022).

## Decided

- Use seam-owned values with detached snapshot/reveal lists. Required recorded numeric fields distinguish explicit zero from missing/null; the composition still owns interval validity and disclosure — [continuation plan](https://github.com/KirkDiggler/rpg-project/blob/feat/wall-kit-fit/ideas/wall-kit-fit/sections-plan.md).

## Found

- Snapshot/reveal equality, isolation, malformed-whole-reveal refusal and legacy absence checks pass against the pushed Encounter candidate 9c166b2d — [Encounter provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2503).

## Mistaken

- Bumping from Encounter236 exposed the already-merged initiative interface change before section tests could compile. Stacked on the existing green adapter instead of duplicating another wave's implementation, then included its test-only follow-up; this branch remains dependent on that base — [upstream Session adapter](https://github.com/KirkDiggler/rpg-toolkit/pull/2502).
