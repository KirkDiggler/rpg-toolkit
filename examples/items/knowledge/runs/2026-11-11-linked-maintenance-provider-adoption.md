# Current maintenance provider adoption

processed: 2026-11-12T165555Z

## Shipped

- Adopt effects de822f5e and D&D root 678e9d74 on real pushed development pins; retain the existing item examples and current single-holder Raging format — [S15](https://github.com/KirkDiggler/rpg-project/issues/905).

## Decided

- No example source rewrite is needed: the compiled consumers already use the current composition API. This is dependency adoption, not a native inventory or item-maintenance feature — [approved T12](https://github.com/KirkDiggler/rpg-toolkit/pull/2323).

## Found

- Baseline and adopted tests pass without compatibility shims or changed assertions: fixed non-Barbarian Rage bonus, independent resources, provenance, spent loot, transfer/reload and grace remain covered — [S15](https://github.com/KirkDiggler/rpg-project/issues/905).

## Mistaken

- none observed — [S15](https://github.com/KirkDiggler/rpg-project/issues/905).
