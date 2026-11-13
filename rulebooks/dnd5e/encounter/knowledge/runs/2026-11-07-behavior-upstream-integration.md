# Run — 2026-11-07 — canonical behavior and upstream integration

processed: 2026-11-12T165555Z

Part of [live adoption #866](https://github.com/KirkDiggler/rpg-project/issues/866), on [toolkit #2293](https://github.com/KirkDiggler/rpg-toolkit/pull/2293).

## Shipped

- Prepared merge-on-top integration of current main, including the separately reviewed sight cache, plus canonical configuration forwarding in the newly arrived session-style benchmark; no merge to main or release of this feature is authorized — [upstream cache #2305](https://github.com/KirkDiggler/rpg-toolkit/pull/2305).

## Decided

- Keep the required race/atomic-coverage gate and its timeout unchanged; investigate with the exact command and integrate the already-owned optimization rather than create a competing performance mechanism or reduce assertions — [runtime efficiency story](https://github.com/KirkDiggler/rpg-project/issues/879).
- Adapt the new benchmark's input forwarding and ordinary driver to the canonical interface, because omitting compiled policy simulates an idle garrison rather than the authored one; no behavior factory or test-only runner is added — [session-style benchmark #2301](https://github.com/KirkDiggler/rpg-toolkit/pull/2301).

## Found

- At f6db048b the remote root package timed out at 600 seconds twice under race plus atomic coverage; the earlier c1cbf623 CI run completed it in 465.804s, and the other packages also ran slower on both failed runs — [CI run](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42413589215).
- Local matching-command measurements do not show a review-fix regression: c1cbf623 root 438.260s, exact f6db/main synthetic merge 409.051s, then current-main integration 408.312s; all nine packages pass locally, which is not a waiver of remote CI — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
- The incoming simulation dropped `apothecary-monster-00`'s authored behavior; a benchmark setup assertion failed before forwarding was corrected, then all four one-game styles passed at two operations each — [#2301](https://github.com/KirkDiggler/rpg-toolkit/pull/2301).

## Mistaken

- A single unchanged-head rerun did not clear the gate; both failures are retained as evidence, not labelled flaky or hidden by a timeout increase — [CI run](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42413589215).
