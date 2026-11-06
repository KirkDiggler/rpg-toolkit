# Rage-granting loot: an ongoing-effect consumer

[Keep equipment S2 — #873](https://github.com/KirkDiggler/rpg-project/issues/873).
This extends the Second Wind consumer without changing the generic `items` or
`effects` packages. The outcome being investigated is effects extraction driven
by items first; production condition migration follows later (#703 / #705).

## One artifact, three lifetimes

```text
outside encounters:
  authored Definition + built instance Data → JSON files → retrieve existing item

encounter-like host:
  attach item to holder → grant is visible
  items.Use[*rageActivation] → charge spent, effect prepared but NOT applied
  caller applies effects.Composed → Rage's outputs and machine become active

then independently:
  detach/transfer item → its grant association changes; budget stays spent
  drive Rage's own ending beat / explicitly remove it → outputs detach
  recover item pool → charge returns, but an ended Rage does not reactivate
```

This observes independent mechanisms, not a ruling that every real unequip must
leave an effect running. The caller still owns actual equipment/attunement policy
and can explicitly remove an effect when that policy requires it.

## What is real here

- `rage_recipe_test.go` authors states/transitions over the existing machine,
  following the established Rage-shaped prototype. No handwritten state runner.
- `rage_vocabulary_test.go` is a consumer plug over actual D&D typed topics and
  damage, incoming-damage, save and skill-check chains—not a wrapper around the
  handwritten `RagingCondition`.
- Damage uses authoritative `contributions.Frame` facts. Unknown relevant facts
  refuse the fold rather than silently dropping the bonus. Incoming damage uses
  the actual immutable-input check and `combat.SettleDamage`.
- The effect has four benefits: configured Strength melee weapon damage bonus,
  physical resistance, Strength-save advantage and Strength-check advantage.
- The real effects machine handles first-turn grace, attack/hit sustain,
  inactivity, ten-round expiry, clock reset, unconsciousness, rest and combat end.
- Contribution provenance carries the item instance ID. The effect remains bound
  to its original recipient if the source item is later handed elsewhere.

The example uses its own ability/condition refs and a representative one-charge,
+2, long-rest item definition. A persisted +3 variant proves retrieval honors
stored authored data rather than silently reconstructing the default fixture.
Those values do not select production rarity/equipment rules.

## Built items do not require encounters

`loot_example_test.go` is the **example host**. It saves an authored Definition
and a built instance's Data to separate temporary JSON files and retrieves them
before any bus or holder association exists. The generic library gains no I/O,
encounter dependency, catalog service or inventory system.

Giving existing loot means associating that same ID and budget with a consumer;
it is not `New` with an omitted State. The retrieval path always supplies saved
Data. Missing/null storage refuses instead of minting a fresh charged item.
Definitions and instances are distinct: creating a new drop from a definition is
an explicit creation path, not a fallback for failed retrieval.

This is a runnable storage/handoff example, not a production repository design or
native encounter loot feature. No database, network API, multiplayer ownership
transfer or persistence transaction is claimed.

## The preparation/delivery boundary is visible

A fault-injected bus refuses the prepared Rage's application. The working item's
charge is already spent: the item library did not undo a later delivery failure.
The host therefore does not save the failed operation's working Data; reloading
the stored instance still yields its original charge. The successful retry applies
the effect before committing the spent item Data.

That demonstrates a bounded load–act–commit-on-success flow. Atomic persistence
across actors/items, failures during commit, arbitrary effect rollback and
interrupted activations remain unproved. The pure resolver protocol remains an
experiment to evaluate at extraction, not a universal transaction guarantee.

## Run

```sh
cd examples/items
go test -count=1 -run 'Example_rageLoot|TestRageGrantingItem|TestBuiltItem|TestMissingOrNull|TestHostCommits' -v ./...
go test -count=1 -race ./...
go vet ./...
golangci-lint run ./...
```

The executable example verifies:

```text
retrieved outside encounter: loot-ring-17; charges 1
prepared Rage: applied false; charges 0
raging Strength melee damage: 12
source detached; active effect remains: true
combat ended; effect active: false; charges 0
retrieved after encounter: loot-ring-17; charges 0
```

Changing the expiry threshold from nine rounds since the anchor to ninety-nine
fails the real turn-walk assertion. Reconstructing retrieved loot without its
saved State fails the spent-instance assertion. Both mutants are removed.

## What this does not certify

Production condition registration/stacking, casting/concentration or armor
eligibility, actor bonus-action payment, and complete class-feature reuse are not
implemented by this consumer. No condition is migrated. The encounter is a test
host on a real event bus, not the live encounter/session SDK. Extraction home,
real storage contracts and production integration still need their checked plan.

The next provider plan must follow the operator's current ordering: item consumers
justify extraction first; the old combined condition-first promotion plan is
superseded, with its parity requirements preserved for later adoption.
