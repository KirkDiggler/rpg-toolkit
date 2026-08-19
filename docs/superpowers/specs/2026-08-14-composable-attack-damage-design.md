# Composable Attack Damage Design

## Purpose

Represent one attack as an ordered collection of typed damage pools. One attack
roll may produce several damage types, but resolution still performs one strike,
one damage-chain fold, and one damage application.

The shipped path is authoritative:

```text
persisted content
  -> AttackFromCharacter / AttackFromMonsterAction
  -> AttackProfile
  -> Strike
  -> Gather(damage) once
  -> combat.FinalDamage
  -> ApplyDamage once
  -> StrikeOutcome
  -> session adopts the returned world and records the beat
```

The compiler → `AttackProfile` → `Strike` path replaces the legacy
`combat.ResolveAttack`, `ResolveAttackHit`, `ApplyAttackOutcome`, and
attack-specific `ResolveDamage` stack. Those APIs, their unused production
callers, and their compatibility tests are deleted; they do not receive adapters.

## Rules Decisions

- A damage pool is crit-eligible by default.
- A critical hit rolls every eligible attack damage die twice. This follows SRD
  5.1, including additional dice on the same attack. A gray ooze pseudopod
  therefore doubles both its bludgeoning and acid dice.
- `DoesNotCrit` represents an explicit exception; it is not the default for an
  elemental or secondary pool.
- Flat modifiers never double on a critical hit.
- Lifedrinker-shaped damage is a runtime feature contribution: flat necrotic
  damage equal to the Charisma modifier, minimum 1. It is neither weapon content
  nor the ordinary attack ability modifier and does not double on a critical.
- Resistance, vulnerability, and immunity apply independently to each damage
  type through the existing `combat.FinalDamage` arithmetic.

ADR-0036 is superseded by this design. Its selective-critical variant conflicts
with SRD 5.1 because the ooze's acid dice are damage dice of the attack and
therefore double on a critical hit. The ADR, decision index, and living combat
overview are reconciled in the same documentation change as this specification;
implementation has one authoritative rule to follow.

## Scope

This change covers:

- Shared declared and resolved damage types in `rulebooks/dnd5e/damage`.
- Weapon declarations and character attack compilation.
- Generic melee, ranged, and bite monster action damage using canonical arrays.
- Removal of the standalone scimitar action in favor of the generic melee
  action.
- `AttackProfile`, `Strike` damage rolling, the damage-chain carrier, and
  `StrikeOutcome`.
- Conditions that currently assume one weapon pool: Great Weapon Fighting,
  Brutal Critical, Martial Arts, Rage, and Sneak Attack.
- The top-level encounter package's direct monster-action snapshot reader.
- Removal of the replaced legacy combat resolution path and its obsolete
  callers.
- The effective-advantage propagation defect in `Strike`, because the modified
  damage-event construction currently grants canceled advantage to Sneak Attack.

This change does not add production Lifedrinker, pact-weapon state, ranged
strike semantics, save-gated damage, multiattack orchestration, new damage-chain
stages, or a new event-bus lifecycle.

## Shared Damage Types

The leaf `damage` package gains:

```go
// Property identifies behavior attached to one declared damage pool.
type Property string

const (
	// AddsAttackAbilityModifier marks the ordinary weapon pool that receives
	// the ability modifier selected by the attack compiler.
	AddsAttackAbilityModifier Property = "adds-attack-ability-modifier"

	// DoesNotCrit prevents this pool's dice from being rolled a second time on
	// a critical hit. Pools without it are crit-eligible.
	DoesNotCrit Property = "does-not-crit"
)

// Damage declares one dice pool of one damage type.
type Damage struct {
	Dice       string     `json:"dice"`
	Type       Type       `json:"type"`
	FlatBonus  int        `json:"flat_bonus,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

// HasProperty reports whether this pool contains property.
func (d Damage) HasProperty(property Property) bool

// Instance is one resolved amount of one damage type.
type Instance struct {
	Amount int  `json:"amount"`
	Type   Type `json:"type"`
}
```

`Dice` is pure notation such as `"2d4"`, never `"2d4+2"`. `FlatBonus` is
intrinsic to the declared pool. The ordinary character attack ability modifier
travels separately in the compiled profile so breakdowns and features can still
identify or replace it.

`Damage` is the one declaration shape used by weapons, monster actions, and
compiled profiles. `Instance` is the one resolved typed-amount shape used after
the damage fold.

## Validation

A damage slice is invalid when:

- it is empty;
- any pool has empty or malformed dice notation;
- notation contains a baked-in modifier;
- a type is `damage.None` or is not a recognized damage type;
- more than one pool has `AddsAttackAbilityModifier`; or
- a property is unknown.

All pools are validated before `Strike` yields the attack chain or consumes any
randomness. Compiler constructors return invalid content as `ErrBadAttack` (or
the package-equivalent invalid-argument error) with the pool index and notation.
No partial rolling or partial application is allowed.

`AttackProfile` validation also enforces its cross-field ability invariant. A
profile with a non-empty `AbilityUsed` must have exactly one
`AddsAttackAbilityModifier` pool; `AbilityModifier` may legitimately be zero.
A profile with an empty `AbilityUsed` must have `AbilityModifier == 0` and no
marked pool. Character compilation produces the first shape. Monster
compilation produces the second. Hand-built or decoded profiles that mix the
shapes are rejected before rolling, so an ability modifier cannot be silently
dropped.

## Persisted Monster Action Cutover

Monster action configs use only:

```go
Damage []damage.Damage `json:"damage"`
```

There are no `damage_dice`, `damage_type`, `DamageDice`, or `DamageType`
compatibility fields. In-repository monster fixtures are rewritten atomically.
Missing, empty, or invalid canonical arrays fail loading; old persisted blobs
must be regenerated by their owner rather than translated by runtime code.
Monster `FlatBonus` remains intrinsic and never implies
`AddsAttackAbilityModifier` or a guessed stat-block ability.

The top-level encounter package's `primaryAttackSnapshot` decodes only the
canonical array:

1. It validates the non-empty `damage` slice; invalid data makes the action
   ineligible.
2. The singular snapshot projects the pool marked
   `AddsAttackAbilityModifier`, or the first pool when none is marked. It
   reconstructs `DamageDice` deterministically as pure dice followed by the
   signed `FlatBonus` when non-zero (`1d8+2`, `1d8-1`, or `1d8`). `DamageType`
   comes from the same pool.
3. The first eligible action wins. A valid projected notation
   remains non-empty, preserving opportunity-attack readiness for
   canonical-only monster data.

This projection is deliberately lossy and exists only for the encounter's flat
readiness snapshot. Real resolution compiles every canonical pool from hydrated
action data. The standalone scimitar action/config/loader branch is removed;
goblins declare the same attack through generic `MeleeConfig` canonical damage.

Ranged action data uses the canonical shape, but
`AttackFromMonsterAction` continues to refuse ranged attacks until ranged strike
semantics ship.

## Compilation

`weapons.Weapon` replaces its singular `Damage` string and `DamageType` with:

```go
Damage []damage.Damage
```

Ordinary player weapons mark exactly one pool with
`AddsAttackAbilityModifier`. `AttackFromCharacter` selects STR or DEX using the
existing finesse and weapon rules and compiles:

```go
type AttackProfile struct {
	Ref             *core.Ref
	AttackBonus     int
	Damage          []damage.Damage
	AbilityUsed     abilities.Ability
	AbilityModifier int
	Gate            *saves.SaveGate
	Imposes         Consequence
}
```

The ability modifier remains outside `Damage.FlatBonus`. `Strike` creates the
ability-source component on the pool marked `AddsAttackAbilityModifier`, which
preserves Martial Arts replacement, two-weapon behavior, and transparent combat
breakdowns.

Versatile remains a weapon property rather than a second always-active damage
pool. It identifies the marked primary pool and its two-handed replacement dice.
When the existing grip selection chooses two hands, `AttackFromCharacter`
copies the canonical slice and replaces only that pool's `Dice`; its type,
intrinsic flat bonus, and properties are unchanged. Other intrinsic pools are
not stepped up. A versatile declaration without exactly one marked primary pool
is invalid.

Monster compilers copy authoritative canonical pools and leave `AbilityUsed`
empty and `AbilityModifier` zero. Static compilation occurs before the bus
exists. Situational effects never become profile fields merely because they add
damage.

## Strike Resolution

The machine's yield shape does not change:

```text
Gather(attack)
  -> roll versus folded AC
  -> on hit, roll every damage pool
  -> Gather(damage) once
  -> FinalDamage
  -> ApplyDamage once
  -> optional Request(contest)
  -> Done
```

For each pool, `Strike`:

1. Rolls its dice once.
2. Rolls them a second time when the strike is critical and the pool lacks
   `DoesNotCrit`.
3. Creates a weapon component containing dice notation, per-die rolls,
   intrinsic flat bonus, type, properties, source ref, and whether that
   component's dice doubled.
4. Creates one ability-source component for the pool marked
   `AddsAttackAbilityModifier`, using the compiled `AbilityModifier` and that
   pool's type.

All components enter one `DamageChainEvent`. The machine never receives or
touches the bus; it yields `Gather` and the resolution driver performs the fold.

`DamageComponent.IsCritical` means that component's dice were actually doubled.
Flat-only ability and feature components use false. The chain-level
`IsCritical` continues to describe the strike outcome for rules that react to a
critical hit.

Dice-based feature contributions made during the fold use the chain-level
critical result. Sneak Attack rolls its eligible dice once on an ordinary hit
and twice on a critical hit, emits one typed feature component containing both
sets of rolls, and sets that component's `IsCritical` true only when its dice
doubled. Brutal Critical is different: it contributes the extra weapon die
granted because the hit is critical, but that newly granted die is rolled once;
its component records `IsCritical` false because its own dice were not doubled.

## Damage-Chain Semantics

Every weapon component carries its own dice notation and properties. This
removes the current dependence on one `DamageChainEvent.WeaponDamage` string.

The event gains narrowly named primary metadata derived from the pool marked
`AddsAttackAbilityModifier`:

```go
WeaponDamageDice string
WeaponDamageType damage.Type
```

These fields do not describe the whole attack. They exist for rules that
explicitly inherit the ordinary weapon die or type:

- Rage and Sneak Attack inherit `WeaponDamageType`.
- Brutal Critical uses `WeaponDamageDice`.
- Martial Arts replaces the marked primary component.
- Great Weapon Fighting rerolls the marked primary weapon component using that
  component's own notation; it does not apply one primary die size to every
  intrinsic pool.
- Resistance, vulnerability, and immunity inspect component types only.
- A Lifedrinker-shaped feature appends a flat `damage.Necrotic`
  `DamageSourceFeature` component at `StageFeatures`; it does not inherit either
  primary field.

The existing singular `WeaponDamage` and `DamageType` fields are removed rather
than mirrored. The same change migrates Great Weapon Fighting,
Brutal Critical, Martial Arts, Rage, Sneak Attack, Dueling, and Two-Weapon
Fighting to the new primary metadata or typed components as appropriate. Rage
resistance reads each final component's type.

Effective advantage is computed as advantage present and disadvantage absent.
The damage event must not set `HasAdvantage` merely because an advantage source
survived alongside a canceling disadvantage source.

## Outcomes and Notification Boundary

`StrikeOutcome` preserves both the convenient aggregate and the typed evidence:

```go
type StrikeOutcome struct {
	// existing attack and contest fields...
	Damage           int
	DamageInstances  []damage.Instance
	DamageComponents []dnd5eEvents.DamageComponent
}
```

`combat.FinalDamage` remains the sole multiplier arithmetic. `Strike` stores its
typed result, converts it once for `ApplyDamage`, and applies every instance in
one call.

Typed evidence is guaranteed through `StrikeOutcome`. The current encounter
record and session beat accept only named integers, so this design keeps their
existing aggregate `ValueAmount` representation; it does not claim that typed
instances survive into that schema. Carrying typed damage into encounter
history requires a separately designed, versioned outcome schema.

The replaced attack stack's `DamageReceivedEvent` publication is removed with
that stack. `Strike` applies damage directly once. Session adopts the returned
world, records the beat, and emits its stream event from `StrikeOutcome`.

## Lifedrinker Proof

Production Lifedrinker is out of scope, but a synthetic chain subscriber proves
the extension seam. A critical pact-longsword-shaped strike with STR +3 and CHA
+5 resolves as:

```text
2d8 + 3 slashing
       5 necrotic
```

The test proves that the necrotic flat component does not double, the minimum-1
rule can be expressed by the feature, and slashing and necrotic defenses affect
only their respective components.

## Testing

Tests follow the repository's testify suite pattern and cover:

1. Damage property lookup and validation.
2. Canonical positive, zero, and negative intrinsic monster modifiers.
3. Missing, empty, malformed, and unknown-type canonical declarations.
4. Canonical-only JSON round trips with no legacy field names.
5. Goblin scimitar content represented by the generic melee action.
6. Bite canonical damage preserving `SaveGate`.
7. Ranged canonical data without enabling ranged Strike compilation.
8. Existing single-pool player and monster results unchanged.
9. Two pools using one attack roll, one damage gather, and one application.
10. Both gray ooze pools doubling on an SRD critical hit.
11. An explicit `DoesNotCrit` pool rolling only once.
12. Intrinsic and ability flat bonuses applied once and never doubled.
13. Ability modifier attached only to the marked pool.
14. Bludgeoning vulnerability plus poison immunity on one strike.
15. Great Weapon Fighting rerolling only the marked primary component with its
    own notation.
16. Primary-pool Martial Arts and Brutal Critical behavior, including Brutal
    Critical's granted die being rolled once with component `IsCritical=false`.
17. Rage and Sneak Attack inheriting the primary weapon type.
18. Sneak Attack dice rolling twice on a critical hit with component
    `IsCritical=true`.
19. Advantage plus disadvantage rolling straight and not granting Sneak Attack
    solely through canceled advantage.
20. Typed instances and components surviving into `StrikeOutcome`, while the
    session beat continues to record aggregate damage.
21. A synthetic Lifedrinker-shaped flat necrotic feature contribution.
22. Encounter opportunity-attack readiness from canonical action damage,
    including invalid arrays, primary-pool projection, and
    positive/zero/negative flat bonuses.
23. One-handed and two-handed versatile attacks changing only the marked
    primary pool.
24. Dueling, Two-Weapon Fighting, and Rage using only canonical metadata.
25. `AttackProfile` rejecting missing, duplicate, and ability-less marker
    combinations without consuming randomness.
26. Repository searches proving the old persisted fields, standalone scimitar,
    legacy attack APIs, compatibility mirrors, and obsolete callers are absent.

## Module and Release Order

The repository's Go modules consume published sibling versions unless local
overrides are installed. Development and release proceed inside-out:

1. Modify and test `rulebooks/dnd5e`.
2. Use an uncommitted `go.work` or temporary `replace` while downstream modules
   consume local changes.
3. Modify and test `rulebooks/dnd5e/resolution`.
4. Modify and test the top-level `encounter` module's snapshot migration against
   local `rulebooks/dnd5e`.
5. Modify and test `rulebooks/dnd5e/session` against local
   `rulebooks/dnd5e`, `rulebooks/dnd5e/resolution`, and its existing
   `rulebooks/dnd5e/encounter` dependency. The latter requires no schema change
   for typed damage because the beat remains aggregate.
6. Run targeted and full module suites plus lint.
7. Remove every `go.work` and `replace` before committing or merging.
8. Publish `rulebooks/dnd5e`; update, test, and publish
   `rulebooks/dnd5e/resolution` and the top-level `encounter` module against that
   version; then update, test, and publish `rulebooks/dnd5e/session` against the
   published D&D and resolution versions. Publish `rulebooks/dnd5e/encounter`
   only if implementation reveals an actual change in that module.

No committed module may contain a local `replace` directive or `go.work` file.
