# Combat architecture

Attack resolution lives in `rulebooks/dnd5e/resolution`. A caller compiles a
character weapon or monster action into a canonical `AttackProfile`, runs a
`resolution.Strike`, and receives a typed `StrikeOutcome` containing the
aggregate damage plus source-attributed components and typed instances.

The `combat` package owns shared rulebook contracts and arithmetic only:

- `Combatant` and `ApplyDamage` are the HP-application boundary.
- `DealDamage` handles generic, non-attack instance damage for spells,
  conditions, and environmental effects.
- `SettleDamage` is the settlement of received damage, per damage type: what
  was dealt, the target's reductions, the effective factor the stacking rules
  chose, and what is taken. Its `FinalDamage` is the instances that land. The
  target step that owns both folds (the dealt `DamageChain` and the target's
  `IncomingDamageChain`) hands it the dealt components and the target's
  answers.

There is no attack resolver, compatibility attack input/output, phased attack
adapter, or event-wide damage metadata in this package. Damage type and other
primary facts remain on their canonical `DamageComponent`.
