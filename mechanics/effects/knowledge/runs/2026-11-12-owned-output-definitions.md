# Run — 2026-11-12 — Composable conditions S17

Slice [project#950](https://github.com/KirkDiggler/rpg-project/issues/950), supporting [#946](https://github.com/KirkDiggler/rpg-project/issues/946).

## Shipped

- Source checkpoint: Output.Clone and Bound.Definition expose detached authored data, with no runtime/bag mutation — [contract](https://github.com/KirkDiggler/rpg-project/issues/950).

## Decided

- Keep complete output observation in its neutral owner rather than duplicate descriptor state or weaken configuration equality in the rulebook — [consumer finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6836695182).

## Found

- Public observer tests fail on missing APIs, then pass for live-execution isolation, nested guard/amount copies and End-before-detach capture; full module race/vet/lint pass — [verification scope](https://github.com/KirkDiggler/rpg-project/issues/950).

## Mistaken

- The original root-only plan assumed existing Bound observations supplied sufficient immutable configuration; Params omitted guard and amount definitions needed for exact application checks — [finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6836695182).
