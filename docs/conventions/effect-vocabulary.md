# Effect vocabulary

Use these terms in planning, documentation, and player-facing labels.

```text
Effect — any rule that changes gameplay
├── Condition — one of the fifteen standard D&D 5e conditions
├── Status — a temporary active state
├── Passive effect — an always-on benefit from a class, feature, or equipment
├── Monster trait — an innate monster rule
└── Damage affinity — resistance, vulnerability, or immunity to a damage type
```

Every condition is an effect, but not every effect is a condition.

The standard condition list is: Blinded, Charmed, Deafened, Exhaustion,
Frightened, Grappled, Incapacitated, Invisible, Paralyzed, Petrified,
Poisoned, Prone, Restrained, Stunned, and Unconscious.

Condition immunity is restricted to that standard list. It does not block
statuses (such as Raging or Hidden), passive effects, monster traits, or damage
affinities.

`ConditionBehavior` remains the current internal Go interface name while the
toolkit migrates toward this vocabulary. It is a technical lifecycle interface,
not a statement that every implementing effect is a D&D condition.
