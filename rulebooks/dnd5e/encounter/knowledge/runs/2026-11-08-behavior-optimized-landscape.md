# Run — 2026-11-08 — canonical behavior in the optimized landscape

Part of [live adoption #866](https://github.com/KirkDiggler/rpg-project/issues/866), integrating [runtime-efficiency #879](https://github.com/KirkDiggler/rpg-project/issues/879).

## Shipped

- Prepared current-main integration of lazy routes, input-keyed folds/participation, PaceDue and whole-act delta reads with the canonical runtime; downstream continuation validation and release gates remain — [authorization](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6717591263).

## Decided

- Read route requirements from the conservative union of canonical attachment tables, because the composer chooses only after the view facts exist; do not pre-step guards, choose twice, or read retired per-actor Table fields — [canonical contract](https://github.com/KirkDiggler/rpg-toolkit/pull/2293).
- Keep the upstream route predicate and opaque-driver contract: a member without authored policy still receives complete routes, and a lower-priority attachment's route requirement must not disappear behind an earlier hold — [route optimization](https://github.com/KirkDiggler/rpg-toolkit/pull/2308).

## Found

- A real canonical hold-only configuration received a nine-cell route under the automatically merged legacy-table gate; the regression fails before adaptation and passes for hold/exact-cell suppression, a route-reading attachment, a lower-priority route reader and no-policy host control afterward — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
- Same-host 10-op garrison comparison, overlay restoring only the old gate versus canonical table requirements: default Move 301.2→200.2ms/op, raise step333.7→199.6ms, allocations585102→497307; posts-asleep170.4→169.0ms. Both runs have identical opening blob bytes and reported growth; these are a bounded integration measurement, not a production latency promise — [#879](https://github.com/KirkDiggler/rpg-project/issues/879).
- Full combined encounter-module race tests pass across all nine packages; root207.023s, source118.625s, keepgen308.972s; lint reports zero issues — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).

## Mistaken

- A clean textual merge did not preserve optimization semantics: canonical actors leave the retired Table nil, so the old gate treated every one as an opaque driver and computed routes nobody read. Canonical-path assertions, not compilation alone, exposed it — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
