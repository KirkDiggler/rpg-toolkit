# Combat architecture

Attack resolution lives in `rulebooks/dnd5e/resolution`. A caller compiles a
character weapon or monster action into a canonical `AttackProfile`, runs a
`resolution.Strike`, and receives a typed `StrikeOutcome` containing the
aggregate damage plus source-attributed components and typed instances.

The `combat` package owns shared rulebook contracts and arithmetic only:

- `Combatant` and `ApplyDamage` are the HP-application boundary.
- `DealDamage` handles generic, non-attack instance damage for spells,
  conditions, and environmental effects.
- `FinalDamage` folds component multipliers by damage type for callers that
  already own the damage-chain fold.

There is no attack resolver, compatibility attack input/output, phased attack
adapter, or event-wide damage metadata in this package. Damage type and other
primary facts remain on their canonical `DamageComponent`.
