# 2026-11-06 — item consumer of the extracted effects runtime

processed: 2026-11-12T165555Z

Work: [items S1 #2273](https://github.com/KirkDiggler/rpg-toolkit/pull/2273), [provider #2291](https://github.com/KirkDiggler/rpg-toolkit/pull/2291), [extraction #705](https://github.com/KirkDiggler/rpg-project/issues/705).

## Shipped

- Initial S1 integration imported mechanics/effects and called Compose(ComposeInput), with no item-behavior changes; it pinned pushed provider e15c6e54 as v0.2.2-0.20261106234706-e15c6e54ef81, without replace/go.work — [dependency](../../go.mod).
- Full module race tests, runnable example, vet and lint pass; dependency inventory contains no examples/effects package — [tests](../../items_test.go).

- Released adoption on 2026-11-07 pins mechanics/effects v0.3.0 after provider merge c7cf2461 and successful CI tagging; full race/example/vet/lint gates pass again — [dependency](../../go.mod).

## Decided

- Item APIs remain experimental; this is evidence for the provider extraction, not production inventory adoption or a condition migration — [scope](https://github.com/KirkDiggler/rpg-project/issues/705).

## Found

- Second Wind, source-local pools, charge preservation and targeted rest recovery remain exercised through the renamed composition entry — [fixture and tests](../../fixture_test.go).

- Final pin audit found two inherited main-commit pseudo-versions. Resources v0.3.2 is tree-identical to the old pin; the D&D root at v0.213.5 differs only in run-record Markdown (nested modules excluded). Adopt those existing tags rather than claiming they needed a new release; full local gates pass again — [go.mod](../../go.mod).

## Mistaken

- Review caught omitted direct dice/resources arrows in the module map and stale release wording in the example README; both are corrected without runtime changes — [README](../../README.md).
- A pushed provider pseudo-version is not a released tag; the final v0.3.0 pin and repeated gates supply that missing evidence. Main catch-up conflicted only in the root module map: retain examples/items, remove examples/effects and retain mechanics/effects — [release sequence](https://github.com/KirkDiggler/rpg-toolkit/pull/2291).

— cross-team agent, on behalf of KirkDiggler
