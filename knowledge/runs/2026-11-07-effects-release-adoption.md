# 2026-11-07 — effects release and item-consumer adoption

Work: [extraction #705](https://github.com/KirkDiggler/rpg-project/issues/705), [Rage/loot #2287](https://github.com/KirkDiggler/rpg-toolkit/pull/2287).

## Shipped

- Provider #2291 merged as c7cf2461 after independent review and operator go; CI published mechanics/effects v0.3.0 at that commit, and both consumers pin it with repeated race/example/vet/lint gates passing — [release evidence](../../docs/ideas/item-effect-composition/implementation.md#published-provider-and-released-consumer-adoption).

## Decided

- The operator approved retraction of the retired legacy API because it should not be selected automatically; explicit historic pins remain available, and no compatibility wrapper is added — [retraction](../../mechanics/effects/go.mod).

## Found

- Normal Go version queries list only v0.3.0; explicit retracted-version queries report the rationale for all three legacy tags. S2 targets main before parent deletion so its current head can receive actual CI — [consumer PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2287).

## Mistaken

- The first post-tag proxy query still listed old versions; direct verification followed by normal proxy queries proved propagation. S1's main catch-up conflicted only in the module map; preserve the item example node and new mechanics node, not the removed example-provider node — [map](../map.md).

— cross-team agent, on behalf of KirkDiggler
