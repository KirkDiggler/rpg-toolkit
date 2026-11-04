# Run — 2026-11-04 — Keep signature combat and loot S1 ADR framing

Slice [rpg-project#862](https://github.com/KirkDiggler/rpg-project/issues/862) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [3182088c](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21).

## Shipped

- No implementation shipped; the architectural choice remains Proposed, with the ring-level requirement R1 recorded separately from open API/use-lifetime decisions — [ADR-0048](../../docs/adr/0048-equipment-granted-abilities.md).

## Decided

- KirkDiggler, 2026-11-04: start with the equipment-granted Second Wind consumer decision; mind/machine frames its own problem and shared-machine extraction is not this decision — [slice direction](https://github.com/KirkDiggler/rpg-project/issues/862).

- KirkDiggler, 2026-11-04: the ring supplies its Second Wind level; level-1 and level-3 rings are examples, while rarity selecting the level remains a suggestion rather than a settled mapping — [R1](../../docs/adr/0048-equipment-granted-abilities.md#settled-use-case-parameter).

## Found

- Existing Second Wind requires fighter levels, owns uses/rest recovery and publishes immediate healing; the fresh existing-component tests pass, while no ring consumer exists in this checkpoint — [journal evidence](https://github.com/KirkDiggler/rpg-project/blob/docs/second-wind-ring/docs/journal/second-wind-ring/2026-11-04-what-the-ring-grants.md#evidence).
- The rulebook already has healing arithmetic independent of spell/bus delivery; the effects prototype's state-held outputs are a different responsibility — [ADR context](../../docs/adr/0048-equipment-granted-abilities.md#what-exists).

## Mistaken

- An initial file lookup assumed `combat/healing/doc.go`; inspection located the actual package at `rulebooks/dnd5e/healing/healing.go`, before any implementation proposal depended on the guessed path — [actual package](../../rulebooks/dnd5e/healing/healing.go).
