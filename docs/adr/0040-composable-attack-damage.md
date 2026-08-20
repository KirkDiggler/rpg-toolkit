# ADR-0040: Composable Attack Damage

Date: 2026-08-20

## Status

Accepted

## Context

An attack can deal several kinds of damage at once. A weapon may have more
than one intrinsic pool (for example, a pseudopod's bludgeoning and acid), and
features may contribute additional dice or flat damage. The former singular
weapon-damage declarations and attack resolver stack could not represent that
shape without special cases. They also made it possible for the critical-hit
rule to depend on where a contribution entered the resolver rather than on
whether its dice belonged to the attack.

The approved composable attack-damage design follows the SRD 5.1 critical-hit
ruling: a critical hit doubles all eligible damage dice of the attack,
including additional attack dice such as Sneak Attack. A selective rule that
doubled only the weapon's physical pool would make an intrinsic secondary pool
such as an ooze's acid dice behave differently from other attack damage dice.
That proposal is retained as history in [ADR-0036](0036-additional-damage-selective-crit.md),
but is superseded by this decision.

The resolution boundary also needs to stay unambiguous. Attack resolution must
roll each declared pool, pass all typed evidence through one damage-chain fold,
and apply the resulting typed instances once. Legacy singular declarations and
the replaced attack-specific resolver APIs must not remain as compatibility
paths that can reintroduce a second damage flow.

## Decision

### Ordered typed pools

Attack damage is an ordered collection of typed pools. Each pool declares its
own dice notation, damage type, intrinsic flat bonus, and optional properties:

```go
type Damage struct {
	Dice       string
	Type       damage.Type
	FlatBonus  int
	Properties []Property
}
```

The collection order is meaningful for the damage breakdown and remains stable
through compilation and resolution. Dice notation is pure notation (for
example, `"2d4"`); a modifier is represented by `FlatBonus` rather than baked
into the notation.

Exactly one pool carries `AddsAttackAbilityModifier`. That marker identifies
the ordinary weapon pool that receives the ability modifier selected by attack
compilation. The modifier travels separately from the pool's intrinsic
`FlatBonus`, so features can identify, replace, or explain the ordinary ability
contribution without changing the declared weapon content. Monster attacks
have no marked pool when they have no compiled attack ability modifier.

For a compiled character attack, the cross-field invariant is strict: a
non-empty ability selection has exactly one marked pool, while an attack with
no ability selection has no marked pool and a zero ability modifier. Unknown
properties, malformed notation, empty collections, and unrecognised damage
types are rejected before any pool is rolled.

### Critical-hit and flat-modifier rules

- A pool's dice are crit-eligible by default.
- On a critical hit, every eligible attack damage die is rolled twice and
  included in the attack's damage, including intrinsic secondary pools and
  Sneak Attack dice.
- `DoesNotCrit` is the explicit exception. A pool carrying it rolls its dice
  once on a critical hit; elemental or secondary damage is not implicitly
  exempt.
- Flat modifiers never double on a critical hit. This includes intrinsic
  `FlatBonus`, the ordinary attack ability modifier, and a Lifedrinker-shaped
  feature contribution (flat necrotic damage equal to the Charisma modifier,
  minimum one). Such a feature contribution is a runtime feature component,
  not weapon content and not the ordinary attack ability modifier.

The `IsCritical` evidence on a component means that that component's own dice
actually doubled. The chain-level critical result still describes the strike,
so a dice-based feature can use the strike's critical result while a flat-only
component remains non-critical. A die granted specifically because a critical
hit occurred, such as Brutal Critical's extra weapon die, is rolled once and
is not itself doubled.

### Resolution ownership

Resolution owns the complete attack-damage boundary: one roll per pool, one
damage-chain fold, and one damage application.

1. It rolls each declared pool once, rolling that pool's dice a second time
   only when the strike is critical and the pool lacks `DoesNotCrit`.
2. It contributes the typed components to one `DamageChainEvent` and performs
   one ordered damage-chain fold. Resistance, vulnerability, and immunity are
   applied independently by damage type through `combat.FinalDamage`.
3. It converts the folded typed result to the aggregate and typed outcome,
   then applies every resulting instance in one damage application.

The attack machine yields the damage gather step; it does not own the bus.
The resolution driver owns the bus, chain fold, and application boundary. A
strike therefore remains one attack, one damage fold, and one application even
when its ordered pool collection contains multiple types.

### Removal of the legacy path

The integrated provider and resolution paths use the canonical pool shape.
Legacy singular damage declarations and attack resolver APIs are deleted. In
particular, the replaced `combat.ResolveAttack`, `ResolveAttackHit`,
`ApplyAttackOutcome`, and attack-specific `ResolveDamage` stack are not
retained as adapters or mirrored fields. `AttackProfile` and `Strike` are the
authoritative compilation and resolution path.

## Consequences

### Positive

- Intrinsic multi-type attacks, feature dice, and flat feature contributions
  share one typed, inspectable representation.
- The critical-hit rule follows the SRD for every eligible attack die instead
  of depending on whether a pool is primary, secondary, or supplied by Sneak
  Attack.
- `DoesNotCrit` makes an exception explicit and local to the pool that needs
  it; no damage type receives an accidental default exemption.
- Resolution has one roll boundary, one chain fold, and one application, so
  typed resistance and vulnerability remain consistent and damage cannot be
  applied twice by parallel resolver paths.
- Removing singular declarations and resolver adapters leaves one canonical
  API and prevents old callers from silently reintroducing the superseded rule.

### Negative

- Content and callers that used singular damage fields must migrate to ordered
  canonical pools; old persisted integrated blobs must be regenerated by their
  owner rather than translated at runtime.
- Consumers that need a non-critical pool must declare `DoesNotCrit`
  explicitly, and authors must validate that exactly one pool carries the
  ability-modifier marker where an ability is compiled.
- Typed damage evidence is preserved in `StrikeOutcome`; existing encounter
  history that stores only aggregate integers does not gain typed instances as
  part of this decision.

### Neutral

- Versatile weapons replace the dice on the marked primary pool when used
  two-handed; other pools are unchanged.
- Flat modifiers and dice-based feature components remain distinguishable in
  the breakdown even though both participate in the same fold.
- Ranged strike semantics, save-gated damage, multiattack orchestration, and
  production Lifedrinker are outside this decision; the Lifedrinker-shaped
  rule above defines the component contract for a future feature.

## Options considered and rejected

- **Keep one weapon pool and add an `AdditionalDamage` escape hatch.** This
  preserves the singular model and makes the first rider easy, but it gives
  intrinsic pools and feature contributions different semantics and cannot
  express per-pool properties cleanly. Rejected in favor of one canonical
  collection.
- **Double only the primary weapon pool on a critical.** This is the selective
  variant recorded by ADR-0036. It conflicts with the SRD 5.1 rule for attack
  damage dice, including Sneak Attack and intrinsic secondary pools. Rejected.
- **Double every damage component, including flat modifiers.** Flat numbers
  are not damage dice and the SRD rule does not double them. Rejected to keep
  ability, intrinsic, and feature bonuses stable on a critical.
- **Keep singular fields and adapt them at the boundary.** Adapters would
  create two sources of truth and permit the deleted resolver stack to survive
  indefinitely. Rejected; integrated callers and persisted content cut over to
  canonical pools together.
- **Let each attack machine apply its own damage.** Multiple applications
  would bypass the shared fold and make typed resistance/vulnerability
  inconsistent. Rejected; resolution owns the single application boundary.
