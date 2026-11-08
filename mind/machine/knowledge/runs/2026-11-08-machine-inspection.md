# Run — 2026-11-08 — one-pass machine explanations

## Shipped

- Implemented optional TraceStep values beside the existing Step API; no host logging, scheduler or configuration store enters the machine module — [Machine inspection S1](https://github.com/KirkDiggler/rpg-project/issues/899).

## Decided

- Share the real evaluator and collect trace values at actual reads, because a second evaluation could call stateful vocabulary readers or observe a different world — [R2](https://github.com/KirkDiggler/rpg-project/pull/902).
- Keep transition indices and authored child positions, with unevaluated nodes distinct from false/unknown, because a debugger must show alternatives without pretending they were tried — [R2](https://github.com/KirkDiggler/rpg-project/pull/902).
- Record only known scalar observations; retain the reader's Known flag separately from Scalar so malformed opaque values never leak mutable host objects or masquerade as valid data — [R2](https://github.com/KirkDiggler/rpg-project/pull/902).
- Leave commit authority with the caller: a traced candidate blob may still be a losing composer proposal — [R3](https://github.com/KirkDiggler/rpg-project/pull/902).

## Found

- Focused trace tests, full owning-module race and lint pass; tests cover short circuits, three-valued guards, exact reads, zero/false/empty values, mutation isolation, scope/flag/round observations, errors and state/bag/anchor/wake parity — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).
- The double-evaluation overlay fails an actual reader-count assertion (three expected versus six actual); the production source hash remains unchanged — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).
- Same-recipe local benchmark, three runs: baseline ordinary317.7–322.2ns/op,128B/4allocations; changed ordinary323.6–335.9ns/op,128B/4; opt-in trace571.0–598.6ns/op,673B/6. These are tiny-step costs, not keep-scale or production latency evidence — [#898](https://github.com/KirkDiggler/rpg-project/issues/898).

## Mistaken

- None observed in implementation; the initial RED was the deliberately missing TraceStep interface, not a semantic assertion failure. The later double-read mutant supplies discriminating assertion evidence — [#899](https://github.com/KirkDiggler/rpg-project/issues/899).
