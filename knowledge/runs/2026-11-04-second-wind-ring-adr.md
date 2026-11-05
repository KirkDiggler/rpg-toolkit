# Run — 2026-11-04 — Keep signature combat and loot S1 ADR framing

Slice [rpg-project#862](https://github.com/KirkDiggler/rpg-project/issues/862) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: toolkit main at [3182088c](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21).

## Shipped

- No implementation shipped; the architectural choice remains Proposed, with R1–R5 (ring level, item-owned pools, composition, recovery and a one-charge fixture) recorded separately from open production API/eligibility decisions — [ADR-0048](../../docs/adr/0048-equipment-granted-abilities.md).

## Decided

- KirkDiggler, 2026-11-04: start with the equipment-granted Second Wind consumer decision; mind/machine frames its own problem and shared-machine extraction is not this decision — [slice direction](https://github.com/KirkDiggler/rpg-project/issues/862).

- KirkDiggler, 2026-11-04: the ring supplies its Second Wind level; level-1 and level-3 rings are examples, while rarity selecting the level remains a suggestion rather than a settled mapping — [R1](../../docs/adr/0048-equipment-granted-abilities.md#settled-use-case-parameter).

- KirkDiggler, 2026-11-04: the item owns a separate pool that remains spent across transfer/re-equipping (R2); the item composes reusable effects with usage/resource orchestration (R3), while API/recharge remain open — [requirements](../../docs/adr/0048-equipment-granted-abilities.md#settled-use-case-parameter).

- KirkDiggler, 2026-11-04: the first Second Wind ring uses normal short-or-long-rest recovery (R4); studio-configurable item/pool policy is the ultimate goal, not an editor deliverable for this slice — [requirement](../../docs/adr/0048-equipment-granted-abilities.md#settled-use-case-parameter).

- KirkDiggler, 2026-11-04: use one charge for the first fixture (R5), then focus on a separate consumer example and its seams rather than serial final-configuration checks — [probe direction](https://github.com/KirkDiggler/rpg-project/issues/862#issuecomment-6648142433).

## Found

- The initial test-only consumer exercises real effects/resource/healing dependencies; its local offer/action integration and item-state reporting are explicit gaps — [initial checkpoint](https://github.com/KirkDiggler/rpg-toolkit/blob/6169f235/examples/items/README.md).
- After the effects cleanup merged, the consumer became an exported experimental Item/Offers/typed-Use package. Its isolated owner/view/state behavior is tested; production action, class-feature and world-delivery integration remain open — [package evidence](https://github.com/KirkDiggler/rpg-project/blob/docs/second-wind-ring/docs/journal/second-wind-ring/2026-11-05-from-probe-to-item-composer.md).

- Existing Second Wind requires fighter levels, owns uses/rest recovery and publishes immediate healing; the fresh existing-component tests pass, while no ring consumer exists in this checkpoint — [journal evidence](https://github.com/KirkDiggler/rpg-project/blob/docs/second-wind-ring/docs/journal/second-wind-ring/2026-11-04-what-the-ring-grants.md#evidence).
- The rulebook already has healing arithmetic independent of spell/bus delivery; the effects prototype's state-held outputs are a different responsibility — [ADR context](../../docs/adr/0048-equipment-granted-abilities.md#what-exists).

## Mistaken

- An initial file lookup assumed `combat/healing/doc.go`; inspection located the actual package at `rulebooks/dnd5e/healing/healing.go`, before any implementation proposal depended on the guessed path — [actual package](../../rulebooks/dnd5e/healing/healing.go).
