# Decisions — examples/items

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-11-25 · the ring pays at the gate: `Item.Budget(holderID)` and `Budget.HasPool`, `PoolLeft`, `SpendPool` in `budget.go` with no rulebook import, and `payRing` pays `combat.Pay(combat.WithPools(economy, item.Budget(holder)), definition.Cost)` in one call before resolving the healing; `SpendPool` calls `Resource.Use` and ignores its refusal, so a spend above what is left changes nothing and never goes below zero · options: a use refilling or paying apart from the action's cost · because: the issue's words on rpg-project#1160; `seam_test.go` allows the one `core/resources` import · https://github.com/KirkDiggler/rpg-toolkit/blob/3e03721e88bfc1c97f695d61ed93e7de4c65c063/examples/items/knowledge/runs/2026-11-25-the-ring-pays-at-the-gate.md · upheld: none yet

## Findings

- 2026-11-12 · Item association, instance-local uses and an applied effect have independent lifetimes: transfer does not refill a pool, ending an effect does not refill the item, and detachment does not secretly end the result. The example now delegates Raging creation to the canonical rulebook; its host withholds a failed working copy rather than promising provider rollback or production inventory. · https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/examples/items/knowledge/runs/2026-11-05-rage-item-and-loot.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/examples/items/knowledge/runs/2026-11-06-extracted-effects-consumer.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/examples/items/knowledge/runs/2026-11-06-rage-extracted-effects.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/examples/items/knowledge/runs/2026-11-07-shared-raging-consumer.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/examples/items/knowledge/runs/2026-11-11-linked-maintenance-provider-adoption.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/knowledge/runs/2026-11-04-items-consumer-probe.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/knowledge/runs/2026-11-04-second-wind-ring-adr.md
- 2026-11-25 · The ring's grant ref is `*refs.Features.SecondWind()` with `features.SecondWindParameters`; `example:abilities:second_wind`, `windParameters` and `parseWind` are gone; the resolver asks `features.DescribeFromRef` (giver the item, no pools) and resolves `*definition.Effect.Healing` with `healing.Resolve`, and the rulebook refuses a bad ring grant (level 0, unknown member) at `items.New` by its own sentences. · https://github.com/KirkDiggler/rpg-toolkit/blob/3e03721e88bfc1c97f695d61ed93e7de4c65c063/examples/items/knowledge/runs/2026-11-25-the-ring-asks-the-rulebook.md

## Friction

- none yet

## Open

- none yet
