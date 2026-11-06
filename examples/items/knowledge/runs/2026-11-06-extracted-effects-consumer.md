# 2026-11-06 — item consumer of the extracted effects runtime

Work: [items S1 #2273](https://github.com/KirkDiggler/rpg-toolkit/pull/2273), [provider #2291](https://github.com/KirkDiggler/rpg-toolkit/pull/2291), [extraction #705](https://github.com/KirkDiggler/rpg-project/issues/705).

## Shipped

- S1 imports mechanics/effects and calls Compose(ComposeInput), with no item-behavior changes; go.mod pins the pushed provider e15c6e54 as v0.2.2-0.20261106234706-e15c6e54ef81, without replace/go.work — [dependency](../../go.mod).
- Full module race tests, runnable example, vet and lint pass; dependency inventory contains no examples/effects package — [tests](../../items_test.go).

## Decided

- Item APIs remain experimental; this is evidence for the provider extraction, not production inventory adoption or a condition migration — [scope](https://github.com/KirkDiggler/rpg-project/issues/705).

## Found

- Second Wind, source-local pools, charge preservation and targeted rest recovery remain exercised through the renamed composition entry — [fixture and tests](../../fixture_test.go).

## Mistaken

- A pushed provider pseudo-version is not a released tag. The consumer must be re-pinned and re-tested after an authorized provider merge and CI-generated tag — [release sequence](https://github.com/KirkDiggler/rpg-toolkit/pull/2291).

— cross-team agent, on behalf of KirkDiggler
