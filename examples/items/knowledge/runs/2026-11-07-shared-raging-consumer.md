# 2026-11-07 — item consumer of shared Raging creation

Work: [Composable conditions S11 #883](https://github.com/KirkDiggler/rpg-project/issues/883), approved [design/plan #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

## Shipped

- The example delegates binding validation and canonical Raging creation to the rulebook. The private recipe/vocabulary is removed; generic item behavior changes only its full-Output validation signature — [grant adapter](../../rage_grant_test.go).
- Current-format active-effect restore survives item transfer with origin, bound amount, spent pool and consumed first-turn grace intact; full module race tests, both examples, vet and lint pass on pushed providers — [consumer tests](../../rage_item_test.go).

## Decided

- Keep the grant/item refs experimental while using the canonical registered Raging condition and binding schema; do not claim production inventory or class-feature eligibility from this example — [boundary](../../RAGE.md).

## Found

- Adopt root c00e0d76's native post-apply sustain fix. The item lifecycle test first failed because its fixture still emitted legacy DamageReceived; it and the wrong-holder fixture now emit DamageTaken. Full race/examples/vet/lint pass with no private item mechanic added — [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312).

- The canonical-creator test failed against the private example condition ref/flat blob before migration, then passed through the shared registry. The authored +3 loot fixture still produces base 10 plus 3 without class levels — [loot](../../loot_example_test.go).
- Dependencies resolve actual pushed rulebook 15a223ef and effects 605694f3, without replace/go.work; this checkpoint is not final released-tag adoption — [go.mod](../../go.mod).

## Mistaken

- The original experiment plan described the removed local recipe/vocabulary. It is retained explicitly as historical, with current contracts in RAGE.md and the shared implementation plan — [historical plan](../../RAGE-PLAN.md).

— cross-team agent, on behalf of KirkDiggler
