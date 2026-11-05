# Run — 2026-11-03 — nilnil #2220 — resolution's re-pin consumes ErrZeroDistance

processed: 2026-11-05T201556Z

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at ef4d22b0. The wave: #2250 made `ForOption`'s zero-distance answer `ErrZeroDistance`; resolution is its one consumer.

## Shipped

- dnd5e re-pinned `v0.211.1` → `v0.213.2`; `action.go` consumes `actions.ErrZeroDistance` — zero distance is the answer, not a failure, and `profile.Move = move` (nil for the sentinel, per dnd5e's pinned contract) says the actor chose that nothing moves — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Two test pins updated to the sheet dnd5e v0.212+ actually builds: a goblin's driven Disengage carries its Nimble Escape trait condition beside the Disengaging — [driven_activation_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/driven_activation_test.go).

## Decided

- The guard falls through to `profile.Move = move` rather than an explicit two-arm branch: `ForOption`'s `(nil, ErrZeroDistance)` is pinned by dnd5e's own named test, and a nil move is the profile's existing spelling for "this cast moves nobody" — [action.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/action.go).
- The Nimble Escape assertion updates ride in this PR rather than a separate one: they are the same re-pin's consequences, and `TestThornWhipPullChoicesOnlyApplyOnHit` — this PR's mutant-killer for the zero-distance consumption — lives in the same module's suite — [driven_activation_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/driven_activation_test.go).

## Found

- `TestThornWhipPullChoicesOnlyApplyOnHit`'s `no-pull` arm (`Empty(target.Applied)`) is the pin that fails if the sentinel is not consumed: without the guard the action refuses where the actor chose zero — the review round's carried note, answered — [thorn_whip_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/resolution/thorn_whip_test.go).
- The re-pin crossed dnd5e v0.212.0, which landed "goblins carry Nimble Escape" (#2032): the driven-disengage tests pinned the pre-trait sheet and failed until updated — intended upstream change, not a regression — [#2032](https://github.com/KirkDiggler/rpg-toolkit/pull/2033).

## Mistaken

- The first trait-ref lookup chased a truncated grep line that looked like an `ln_escape` id; the ref is `nimble_escape` (`refs.MonsterTraits.NimbleEscape()`), and the assertions use the helper, not a hand-copied string — [refs/monster_traits.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/refs/monster_traits.go).