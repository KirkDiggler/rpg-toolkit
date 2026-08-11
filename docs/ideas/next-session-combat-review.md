# Next-session combat review

## Question: must an attack use an equipment weapon?

No. Wolf bite, bear claw, and Gray Ooze pseudopod are better represented as
**natural attacks**: attack actions that make an attack roll and deal damage
without an equipped weapon.

Do not simply allow a nil weapon in the current resolver. It directly reads
weapon damage, type, properties, range, and ref. Instead, create a broader
attack definition with an optional equipment weapon.

```text
Attack action
├── Weapon attack
├── Natural attack
├── Spell attack
└── Feature attack
```

## Recommended Pseudopod direction

Combine the new multi-component Damage Profile work with a natural-attack
definition:

```text
Name: Pseudopod
Source kind: Natural attack
Attack bonus: +3
Range: melee, 1 hex / 5 feet
Equipment weapon: none
Damage profile:
- 1d6 - 1 bludgeoning (no ability modifier)
- 2d6 acid (no ability modifier)
```

The ability-modifier choice was decided: each damage-profile component has an
explicit `AppliesAbilityModifier` flag. Pseudopod sets both components to
false. A future flaming sword could apply the modifier to slashing but not to
fire.

## Existing combat concern

The current main resolver is weapon-oriented:

```text
ResolveAttackHit -> one Weapon
ApplyAttackOutcome -> one weapon damage pool and type
```

The new `DamageProfileComponent` and `RollDamageProfile` foundation exists.
It rolls every component independently and doubles every component's dice on a
critical hit, while applying modifiers only where allowed. It has not yet been
wired into the main attack-resolution path.

## Why not fake Pseudopod as a weapon?

Pros: quick shortcut.

Cons: incorrect data model; weapon properties and weapon-only rules become
confusing; future bites, claws, and tentacles remain awkward exceptions.

## Recommended next implementation order

1. Add an Attack Definition that can describe a weapon or natural attack.
2. Keep existing weapon attacks working through a compatibility path.
3. Route the definition's Damage Profile through the existing Damage Chain.
4. Add a natural-attack implementation for monster actions.
5. Add Pseudopod as the first natural mixed-damage attack.
6. Test ordinary weapons, mixed damage, critical hits, resistance, and
   immunity before migrating other monster attacks.

## Important rule distinction

`Uses an equipment weapon` is not the same as `is a weapon attack`. Future
attack definitions should carry explicit source/category tags rather than
infer rules from whether a weapon pointer exists.
