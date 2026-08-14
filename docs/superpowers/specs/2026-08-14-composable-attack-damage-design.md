# Composable Attack Damage Design

## Purpose

Represent every attack as an ordered collection of typed damage pools so one
attack roll can produce multiple damage types. Each pool is rolled and modified
independently, while the attack remains a single hit and a single damage
application.

This design replaces the single-pool declarations used by weapons and monster
actions. It preserves existing one-pool behavior and gives mixed attacks, such
as a gray ooze pseudopod, correct per-type resistance, vulnerability, immunity,
and critical-hit handling.

## Scope

This change covers:

- A shared `damage.Damage` declaration in the leaf `rulebooks/dnd5e/damage`
  package.
- Weapon damage declarations.
- Legacy `monster.ScimitarAction` damage declarations.
- Generic melee, ranged, and bite monster action declarations and their
  serialized configs.
- Attack damage rolling and conversion into damage-chain components.
- Explicit placement of the attacker's ability modifier.
- Per-pool critical-hit eligibility.
- Mixed-type results and notification events without presenting one type as the
  type of the whole attack.
- Tests for migration compatibility, critical hits, and per-type defenses.

This change does not cover save-gated outcomes, multiattack orchestration,
changes to damage-chain staging, spell migration, or the ownership and lifecycle
of the event bus.

## Damage Declaration

The `damage` package gains these exported declarations:

```go
// Property identifies behavior attached to one declared damage pool.
type Property string

const (
	// AddsAbilityModifier adds the attack's resolved STR or DEX modifier to this
	// pool. An attack declaration must contain at most one pool with this property.
	AddsAbilityModifier Property = "adds-ability-modifier"

	// DoesNotCrit prevents this pool's dice from being rolled a second time on a
	// critical hit. Pools without this property are critical-hit eligible.
	DoesNotCrit Property = "does-not-crit"
)

// Damage declares one dice pool with one damage type.
type Damage struct {
	Dice       string     `json:"dice"`
	Type       Type       `json:"type"`
	FlatBonus  int        `json:"flat_bonus,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

// HasProperty reports whether the pool contains property.
func (d Damage) HasProperty(property Property) bool
```

`Dice` contains pure dice notation such as `"1d8"`; it never contains a flat
modifier. `FlatBonus` contains only a bonus intrinsic to that pool. An attacker
ability modifier is not declaration data and is added during attack resolution.

Crit eligibility defaults to true. This preserves the behavior of zero-value
`Properties` and keeps ordinary weapons concise. `DoesNotCrit` is reserved for
rules that explicitly exempt a pool's dice from critical-hit doubling. The gray
ooze is not such an exception: both its bludgeoning and acid dice are attack
damage dice and both double on a critical hit.

## Consumer Shapes and Validation

`weapons.Weapon` replaces `Damage string` and `DamageType damage.Type` with:

```go
Damage []damage.Damage
```

The scimitar, generic melee, ranged, and bite monster action configs make the
same replacement and persist the slice directly in JSON. Their internal action
state also stores the slice rather than parallel dice/type fields.

Attack resolution requires at least one damage pool. It rejects a declaration
when:

- the slice is empty;
- a pool has empty or invalid dice notation;
- a pool has `damage.None` or an unrecognized damage type; or
- more than one pool has `AddsAbilityModifier`.

For compatibility, every migrated ordinary weapon and monster attack has one
pool with `AddsAbilityModifier`. A declaration may intentionally have no such
pool when the attack does not add an ability modifier.

No compatibility fields or alternate representations remain after migration.
Serialized monster action data adopts the new `damage` array shape directly;
support for loading the old JSON shape is outside this issue.

## Resolution Flow

Attack resolution still performs one attack roll. On a hit, it processes every
declared pool uniformly:

1. Parse the pool's pure dice notation.
2. Roll it once, or twice when the attack is critical and the pool does not have
   `DoesNotCrit`.
3. Create one weapon damage component containing the pool's dice, intrinsic flat
   bonus, type, source reference, and whether its dice were doubled.
4. If the pool has `AddsAbilityModifier`, create the existing ability-source
   component using that pool's damage type.
5. Pass all components together through the existing damage chain once.
6. Let existing per-type multipliers group and resolve the components.
7. Apply all final typed instances to the target in one `ApplyDamage` call.

`DamageComponent.IsCritical` continues to mean that the component's dice were
doubled. Flat-only components, including the ability modifier, are not marked
critical because their value is not doubled.

`ResolveDamageInput.IsCritical` and the chain-level critical flag continue to
describe the attack outcome for features that react to a critical hit. Pool
eligibility controls only dice rolling and component-level `IsCritical`.

## Results and Events

An attack with multiple damage types must not be mislabeled as dealing only its
first type.

- `AttackResult` exposes the resolved typed instances and retains the complete
  component breakdown. Its singular `DamageType` field is removed.
- `DamageReceivedEvent` exposes all final typed instances instead of a singular
  `DamageType`. `Amount` remains the aggregate damage applied.
- `DamageChainEvent.DamageType` is removed. Modifiers inspect component types,
  as the current resistance, vulnerability, and immunity implementations already
  do.
- Any consumer that needs a display summary derives it from instances or
  components; no “primary type” convention is introduced.

This is an intentional compile-time migration within the D&D 5e module. All
producers and consumers of these fields must be updated in the same change.

## Error Handling

Invalid damage declarations return `rpgerr.CodeInvalidArgument` before any
damage dice are rolled or chain events are published. Dice parser failures are
wrapped with the pool index and notation so malformed multi-pool declarations
are diagnosable. A failure in any pool aborts the attack damage phase; partial
damage is never applied.

Existing chain and event publication errors retain their wrapping and behavior.

## Testing

Tests follow the repository's testify suite pattern and cover:

1. `damage.Damage.HasProperty` and default crit eligibility.
2. Existing one-pool weapon attacks with unchanged hit, damage, modifier, and
   critical-hit results.
3. A two-pool attack using one attack roll and producing both typed pools.
4. A critical hit doubling both ordinary pools, including the gray-ooze-shaped
   bludgeoning-plus-acid case.
5. A critical hit doubling an ordinary pool while leaving a pool marked
   `DoesNotCrit` at one roll.
6. An ability modifier applied only to the pool marked `AddsAbilityModifier`.
7. An intrinsic `FlatBonus` applied once and not doubled on a critical hit.
8. A mixed bludgeoning-and-poison hit against a target vulnerable to
   bludgeoning and immune to poison, proving independent per-type resolution.
9. Validation failures for empty damage, malformed dice, invalid types, and
   multiple ability-modifier pools.
10. Updated result and event assertions proving all final typed instances are
    visible downstream.
11. Round-trip serialization tests for every migrated monster action config.

The complete D&D 5e module test suite and linter must pass. The committed module
must not contain a `replace` directive or `go.work` file.

## Migration Order

The implementation proceeds inside-out:

1. Add the shared declaration and its unit tests.
2. Migrate weapons and weapon catalog entries.
3. Migrate monster action configs, loaders, assets, and round-trip tests.
4. Generalize attack damage rolling and validation.
5. Replace singular result and event damage types with typed instances.
6. Add mixed-type acceptance tests and run full verification.

Each intermediate commit should compile and pass the tests for the packages it
changes; the final commit must pass the full module checks.
