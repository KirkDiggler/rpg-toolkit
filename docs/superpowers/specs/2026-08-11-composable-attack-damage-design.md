# Composable Attack and Damage Design

## Goal

Give characters, monsters, and creatures one combat path for attacks with one
or more damage pools. The first natural attacks are Gray Ooze and Ochre Jelly
Pseudopod. Existing weapon attacks must keep their current behavior.

## Vocabulary and boundaries

`damage.Damage` is the reusable, unrolled declaration for one damage pool. It
belongs in `rulebooks/dnd5e/damage` and is usable by attacks, spells, traps,
and features.

```text
damage.Damage
- Dice: pure dice, such as 1d6
- Type: one standard damage type
- FlatBonus: intrinsic fixed damage, such as -1
- Properties: per-pool behavior, initially CritEligible
- Save: optional future save-gates-damage declaration

damage.DamageSpec
- Pools: one or more damage.Damage values
```

`damage.DamageSpec` is an unrolled recipe. `events.DamageComponent` remains
the realized, rolled result used by the existing damage chain. No second rolled
damage structure is introduced.

An `attack.AttackDefinition` owns attack-only facts:

```text
- ActionID: stable within the owning combatant
- DisplayName: player-facing label; not globally unique
- Category: equipment weapon, natural, spell, or feature attack
- Attack-roll rule: derived or fixed bonus
- Targeting: melee/ranged and reach/range in hexes
- Optional equipment weapon
- Required damage.DamageSpec for a damaging attack
```

The combination of the attacker ID and ActionID identifies an action. Multiple
creatures may therefore each use ActionID `pseudopod` and display name
`Pseudopod` without collision.

## Resolution

An action produces an `attack.AttackDefinition`. The attack resolver uses the
definition for the attack roll, runs the existing attack chain, then passes
each declared damage pool through the existing damage chain.

- A fixed-bonus natural attack uses its declared bonus.
- An equipment-weapon attack uses the existing derived attacker-and-weapon
  calculation.
- Every pool marked CritEligible doubles its dice on a critical hit.
- Intrinsic flat bonuses do not double on a critical hit.
- Resistance, vulnerability, and immunity inspect each realized damage
  component separately.
- Natural attacks use a natural-attack damage source instead of weapon damage.

Ability modifiers are runtime effects. They are added at resolution as their
own existing damage-chain component, not stored in `damage.Damage`. This keeps
the declaration reusable and permits ordinary character weapons to retain
their present ability-modifier behavior.

Gray Ooze Pseudopod declares a fixed +3 attack bonus, melee reach 1 hex, and
two crit-eligible pools: `1d6` bludgeoning with intrinsic `-1`, and `2d6` acid.
Ochre Jelly Pseudopod uses the same local action ID and display name but a
fixed +4 bonus with `2d6` bludgeoning and intrinsic `-2`, plus `1d6` acid.

## Compatibility migration

The new `DamageSpec` fields are additive. They are added beside the existing
single-pool fields on weapon, monster-action, monster persistence, and combat
input data. The resolver prefers a supplied DamageSpec and otherwise converts
the legacy fields to the equivalent one-pool behavior.

The current `combat.DamageProfileComponent` and
`events.AttackDamageComponent` are temporary migration shapes. They will be
adapted to the new model, then removed in a later deliberate cleanup. Existing
callers remain valid throughout this change.

## Explicitly deferred

- Gray Ooze corrosion and other on-hit riders.
- The save-to-damage bridge that resolves SaveSpec and applies half/negate.
- Save multiplier stacking with damage affinities.
- Migrating spells, traps, features, and the external rpg-api project.
- Removing legacy flat fields after every consumer has migrated.

## Verification

Tests will prove that existing simple weapon attacks retain their results; new
specs take precedence when present; each ooze Pseudopod resolves independently;
per-pool critical behavior is correct; acid resistance affects only acid;
ability modifiers remain runtime components; and invalid declarations fail
before rolling. Save data is validated and persisted but has no resolution
effect in this increment.
