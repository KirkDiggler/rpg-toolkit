# Journey 050: Closing the #684 Double-Subscribe with the Hydration Cascade

## The smell

The encounter vertical kept hitting `events.ErrDuplicateID` —
"modifier ID already exists" — during combat. The host (rpg-api) patched it with
`defer char.Cleanup(ctx)` sprinkled around every place it re-loaded a character:
the combat resolver re-loaded per attack, and `publishTurnEndAndPersistReset`
re-loaded again to publish the turn-boundary signal. Each `character.LoadFromData`
on the **same** encounter bus re-`Apply`'d the character's conditions, so two
`SneakAttackCondition` instances both tried to add the `"sneak_attack"` modifier
to the damage chain. The second `Execute` blew up.

The patches treated the symptom. The disease was that **the encounter SDK held
no entities** — it handed resolvers bare IDs and told them to re-load "from
whatever store the orchestrator wires in." Every consumer of an entity re-loaded
it, and re-loading means re-subscribing.

## The reframe (Kirk's steer)

The first design pass invented an `EntityHydrator` interface — a parallel
"hydration subsystem" the host would inject. Kirk pushed back: hydration is not a
new concept. It IS the toolkit's existing `ToData`/`LoadFromData` round-trip,
which **composes** — an aggregate's `LoadFromData` already cascades into its
children's `LoadFromData` (and `ToData` back out). `character.LoadFromData`
already cascades into `conditions.LoadJSON` → `condition.Apply(ctx, bus)`. That
`Apply` is the single subscribe point. The fix wasn't a new mechanism; it was
moving ownership of the existing one up a level: let `Encounter.LoadFromData`
cascade into the combatants' `LoadFromData`, hold them, and `ToData` back out.

This collapsed the design. No injected interface. The encounter loads each
combatant once, holds it as `combat.Combatant`, and the resolvers get the held
entity instead of an ID to re-load. One load, one subscribe.

## What we learned building it

- **The cure is provable and the regression test must fail without it.** The
  headline test fires a real `SneakAttack` through the real damage chain on the
  held bus N times and asserts no `ErrDuplicateID`. We temporarily made the
  cascade hydrate twice and watched the test fail on attack 1 — confirming it
  guards the actual #684 shape, not a tautology.

- **`IsDirty()` doesn't mean what the plan assumed.** The plan said gate the
  `ToData` write-back on the entity's dirty flag. Reading `character.go` showed
  `dirty` is set only on HP change — condition mutations like
  `UsedThisTurn` never flip it. Gating on `IsDirty()` would have silently
  dropped exactly the per-turn state #689 exists to round-trip. The test caught
  it (UsedThisTurn didn't persist). We dropped the gate: the write-back is
  unconditional, and `character.ToData()` already re-serializes held conditions.
  Code won over the plan; the divergence is recorded in ADR-0030.

- **The cascade created a NEW double-load risk we had to chase down.** Once
  `LoadFromData` hydrated the monster, `NPCAct` was still calling
  `monster.LoadFromData` again on the same bus — reintroducing the very class we
  were curing, just for monsters. Fix: `NPCAct` reuses the cascade-held monster
  and only loads when there's no held instance (the `New`-without-`LoadFromData`
  path some tests use). A dedicated test drives the production
  round-trip-then-NPCAct path and asserts no double-subscribe.

- **"Agnostic" was aspirational, not actual.** The SDK already imported
  `rulebooks/dnd5e` in `npc.go` and `activate_feature.go`. Pretending the new
  path had to stay dnd5e-free would have been inconsistent. Kirk's call: lean
  into the existing precedent, make the coupling coherent and single-sourced,
  and track "fully agnostic engine" separately. The honest status is in the
  package docs and ADR-0030.

## What's left

`ActivateFeature` still loads its own character with `defer Cleanup` — the same
#684 class, off the combat-resolution critical path. Folding it onto the held
combatant is a tracked follow-up. And the whole cure rests on the turn-based,
stateless-per-RPC model (bus reconstructed each load); a real-time in-memory
server would shift the invariant to "subscribe-once for the in-memory lifetime."
Both are deliberate "not now"s, not oversights.
