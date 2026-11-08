# Rage and loot example — S2 experiment plan

Authority: [Keep equipment S2 #873](https://github.com/KirkDiggler/rpg-project/issues/873), following the operator's 2026-11-05 direction. Effects extraction is items-first; conditions follow later. This checkpoint builds evidence, not the extraction or a storage service.

## Shape

**Historical experiment plan:** the current shared-creator adoption is governed by [Composable conditions S11 #883](https://github.com/KirkDiggler/rpg-project/issues/883) and [the approved plan #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295). The original consumer-owned recipe/vocabulary below is removed in that adoption; [RAGE.md](RAGE.md) describes the current boundary.

The original separate consumer supplies a Rage recipe and a vocabulary over real D&D event/chain types. `items.Use[*rageActivation]` prepares an unattached `effects.Composed` and returns source identity with it. The caller applies it on its own bus. The generic item package remains unaware of Rage and of encounters.

The recipe covers the same four benefit kinds and lifecycle reasons as the existing Rage-shaped effects prototype: Strength melee weapon damage bonus, physical resistance, Strength save/check advantage; first-turn grace, attack/hit sustain, inactivity, ten-round expiry, clock reset, unconsciousness, rest and combat end. The example does not claim production condition registration, casting restrictions, actor bonus-action payment or full rulebook migration.

Representative item values are one use, +2 damage and long-rest recovery. These are fixture configuration, not a new rarity/equipment ruling. Source detachment alone does not remove a separately applied effect; explicit removal and its own ending beats do. This observes independent mechanisms without selecting real unequip policy.

## Files / contracts

- `rage_recipe_test.go`: example-owned ability/condition refs, typed parameters, definition/config, shared recipe and pure preparation. No handwritten Rage state runner.
- `rage_vocabulary_test.go`: adapters to existing typed D&D topics and chains; output kinds attach modifiers through real effects. No legacy RagingCondition wrapper.
- `rage_item_test.go`: real consumer tests for benefits, proper targeting/unknown facts, lifespan and item/effect independence.
- `loot_example_test.go`: example-host JSON persistence in a test temporary directory, retrieve-existing versus create-new distinction, runnable trace and delivery-failure evidence. No I/O enters the generic item package.
- README and journal/ADR: measured outcomes, fixture assumptions and extraction gaps.

## Checks

- [x] Prepare through item use → charge spent, four-output effect still unattached until caller Apply.
- [x] Real damage/incoming/save/check chains show the configured bonus/resistance/advantages and preserve item provenance; other actors/nonmatching attacks/types do not receive them.
- [x] Drive grace, attack/hit sustain and each ending beat → expected state/reason, modifiers removed, item budget unchanged.
- [x] Detach/transfer source and restore its pool → no implicit end/refill/reactivation of the independent active effect.
- [x] Build, serialize and retrieve definition + instance before any bus/holder/encounter → ID and budget preserved, no grant exposed until explicit association.
- [x] Existing spent loot remains spent; missing instance data returns an error, never a fresh full-budget item.
- [x] Fault-inject Apply after preparation → report runtime/persisted state separately; demonstrate the example host's commit-on-success boundary without claiming distributed transaction support.
- [x] Full module race tests, examples, vet, current lint, source-format checks and relevant import/knowledge checks pass. A meaningful lifecycle/identity mutant must fail its target assertion.

## Observed checkpoint

The Rage consumer, real-chain harness and host-owned JSON artifact example are implemented. The full module passes with all original cases retained; focused Rage/loot tests exercise the checks above. Expiry-threshold and ignored-saved-state mutants are caught and removed. Applying a prepared effect can fail after the runtime charge is spent; the host example preserves stored state by committing only after successful delivery. This is not a general transaction guarantee.

## Limits / dependencies

Stacked on item PR #2273; one `examples/items` module, no provider changes intended. The effects dependency stays on merged main. Effects and item library interfaces remain provisional. New runtime production contracts or migration work require their own checked plan after this example; no heavy repeated review round is needed merely to learn from it.
