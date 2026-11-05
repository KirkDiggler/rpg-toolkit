# Run — 2026-11-03 — Composable conditions S8: retire legacy features

processed: 2026-11-05T201556Z

Slice [rpg-project#853](https://github.com/KirkDiggler/rpg-project/issues/853) of [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base: [ef4d22b0](https://github.com/KirkDiggler/rpg-toolkit/commit/ef4d22b0b376f25096d466ef1867551b67ccef56).

## Shipped

- Prepared removal of `mechanics/features` and its active navigation/map/quality entries; no rulebook feature code or pins changed — [retirement scope](https://github.com/KirkDiggler/rpg-project/issues/853).

## Decided

- KirkDiggler requested retirement of unused legacy mechanics on 2026-11-03; remove their consumers before their effects provider, one module per PR, preserving historical records — [retirement plan](https://github.com/KirkDiggler/rpg-project/issues/853).
- Keep `mind/machine` and the effects prototype's machine separate; a shared base waits for demonstrated complexity in both, and this cleanup promotes neither — [operator direction](https://github.com/KirkDiggler/rpg-project/issues/853).

## Found

- The source/module census finds no importer of `mechanics/features` outside its own module; API dev at `6472f8b` resolves only `mechanics/resources` among mechanics packages — [consumer evidence](https://github.com/KirkDiggler/rpg-project/issues/853).

## Mistaken

- The broad phrase “mechanics packages are unused” cannot include `mechanics/resources`: it backs character resources, Second Wind and recovery, so the cleanup excludes it — [scope correction](https://github.com/KirkDiggler/rpg-project/issues/853).
