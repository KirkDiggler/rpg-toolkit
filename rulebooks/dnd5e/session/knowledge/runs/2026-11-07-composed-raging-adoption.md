# 2026-11-07 — native session adoption of composed Raging

Work: [Composable conditions S12 #884](https://github.com/KirkDiggler/rpg-project/issues/884), approved [design/plan #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

## Shipped

- Adopt pushed resolution 9425c9da/root 3e771a63; fixtures use the shared creator and current composed data, without putting a bus or rules in session — [go.mod](../../go.mod).
- EffectRow now carries SourceID directly rather than dropping it or encoding origin into opaque identity. Real Afford/activation tests retain four unique domain-honest rows, native first-turn saved progress/origin and class payment/refusal semantics — [native row tests](../../effects_test.go).

## Decided

- Preserve the source fact through the existing projection and an additive wire field; proto#422 supplies the missing contract. Generated SDK/API/web adoption is still pending, not inferred from toolkit tests — [wire PR](https://github.com/KirkDiggler/rpg-api-protos/pull/422).
- Use the current resolution TargetIDs singleton form at the two existing single-target callers; the new provider marks TargetID deprecated. No target selection or pricing rule changes — [caller](../../attack.go).

## Found

- Adopted resolution e6e26a2d/root c00e0d76 for the QA-discovered damage-sustain gap, retaining encounter v0.181.0. Full session race passed in 374.526 seconds; vet/lint passed, without changing the SDK's lifecycle ownership — [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312).
- Later API-head CI was absent because dev's newly landed performance pins conflicted with this wave. Merged origin/main into the session branch, retained the shared Raging root/resolution pins, and adopted released encounter v0.181.0 plus its route-observation documentation rather than undoing the landed work. Full session race suite passed in 404.681 seconds; vet/lint passed — [S12](https://github.com/KirkDiggler/rpg-project/issues/884).

- Removed concrete-type fixtures and old one-row/flat-data assertions initially failed. Their replacements retain current identity, state, budget, information and restart facts; old snapshot prose is not a reason to drop the approved four-row result — [current data assertions](../../conditions_test.go).
- The complete race suite exceeded the initial 180-second command bound, then passed in 424.817 seconds; after the deprecated-field fix it passed again in 416.490 seconds. Vet and lint pass. No timeout was counted as a success or hidden with a skipped suite — [slice evidence](https://github.com/KirkDiggler/rpg-project/issues/884).

## Mistaken

- Initial wire planning missed that the SDK and proto rows omitted the already-produced source entity ID. A failing projection test identifies the missing fact; copy it at each boundary rather than reconstructing it in the host/client — [projection test](../../effect_origin_test.go).

— cross-team agent, on behalf of KirkDiggler
