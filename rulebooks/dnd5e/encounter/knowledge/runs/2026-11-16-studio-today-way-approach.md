# Run — 2026-11-16 — studio today: a way's alternatives and the parts of one approach

processed: 2026-11-25T190427Z

Issue [rpg-toolkit#2444](https://github.com/KirkDiggler/rpg-toolkit/issues/2444), story [rpg-project#966](https://github.com/KirkDiggler/rpg-project/issues/966). Base: `rpg-toolkit` `main` at [`cdefe89a`](https://github.com/KirkDiggler/rpg-toolkit/commit/cdefe89a).

## Shipped

- A way's method connections say each check alternative's rolled ability or skill with role `by`, and each further part of that approach (its tool) with role `with`, right after its primary and carrying the same DC — [rpg-toolkit#2444](https://github.com/KirkDiggler/rpg-toolkit/issues/2444).

## Decided

- Role `with` and order, not a field: roles are open strings on the wire, so nothing in protos or the api changes; the part follows its primary, and a way's method connections are exempt from deduplication so two approaches sharing an identical part (the same tool at the same DC) each keep it — order-dependence alone was too weak with the old dedupe, which dropped the second approach's tool — [rpg-toolkit#2444](https://github.com/KirkDiggler/rpg-toolkit/issues/2444).
- The part keeps the approach's DC as its Value, so a chip can show it; a total by method counts `by` connections only — [rpg-toolkit#2444](https://github.com/KirkDiggler/rpg-toolkit/issues/2444).

## Found

- On the castle, counting `by` only: `ability:dex` totals 26 over two approaches (kitchen and cells doors, each with thieves' tools), `ability:str` 15, and `tool:dnd5e:item:thieves-tools` is no approach (two parts); the skill totals are unchanged — [rpg-toolkit#2444](https://github.com/KirkDiggler/rpg-toolkit/issues/2444).

## Mistaken

- none observed
