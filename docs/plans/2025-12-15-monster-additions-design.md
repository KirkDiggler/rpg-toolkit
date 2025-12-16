# Monster Additions Design

**Date:** 2025-12-15
**Status:** Approved
**Related Issue:** [#425 - Monster Additions: Skeleton and Boss stat blocks](https://github.com/KirkDiggler/rpg-toolkit/issues/425)

## Overview

Add monsters for 3 dungeon themes (crypt, cave, bandit lair) using a hybrid approach: factory functions with shared building blocks for actions and conditions.

## Monster Roster

### Crypt Theme

| Monster | CR | XP | Role | Key Trait |
|---------|----|----|------|-----------|
| Skeleton | 1/4 | 50 | Trash | Vulnerable to bludgeoning, immune to poison |
| Zombie | 1/4 | 50 | Trash | Undead Fortitude (CON save to stay at 1 HP) |
| Ghoul | 1 | 200 | Boss | Paralyzing touch, multiattack |

### Cave Theme

| Monster | CR | XP | Role | Key Trait |
|---------|----|----|------|-----------|
| Giant Rat | 1/8 | 25 | Trash | Pack Tactics |
| Wolf | 1/4 | 50 | Trash | Pack Tactics, Bite can knock prone |
| Brown Bear | 1 | 200 | Boss | Multiattack (bite + claws), high HP |

### Bandit Theme

| Monster | CR | XP | Role | Key Trait |
|---------|----|----|------|-----------|
| Bandit (melee) | 1/8 | 25 | Trash | Scimitar |
| Bandit (ranged) | 1/8 | 25 | Trash | Light crossbow |
| Thug | 1 | 200 | Boss | Multiattack, Pack Tactics |

## CR Budget

Target: Level 1-3 party of 2-4 players

- **Trash mobs:** CR 1/8 to 1/4 (25-50 XP each)
- **Bosses:** CR 1 (200 XP)

## Implementation Approach

### Actions (Active - used during TakeTurn)

Create reusable action types that monsters can add:

```go
// Generic melee attack
m.AddAction(NewMeleeAction(MeleeConfig{
    Name:        "shortsword",
    AttackBonus: 4,
    DamageDice:  "1d6+2",
    Reach:       5,
}))

// Generic ranged attack
m.AddAction(NewRangedAction(RangedConfig{
    Name:        "shortbow",
    AttackBonus: 4,
    DamageDice:  "1d6+2",
    RangeNormal: 80,
    RangeLong:   320,
}))

// Multiattack (for bosses)
m.AddAction(NewMultiattackAction(MultiattackConfig{
    Attacks: []string{"bite", "claw", "claw"},
}))

// Bite with knockdown
m.AddAction(NewBiteAction(BiteConfig{
    AttackBonus:    4,
    DamageDice:     "2d4+2",
    KnockdownDC:    11,  // STR save or knocked prone
}))
```

### Monster Traits (Passive - subscribed to event bus)

Monster traits live in `rulebooks/dnd5e/monstertraits/` as a sibling package to `conditions/`.
This avoids import cycles: `monster` → `monstertraits` → `events` (one-way).

Traits implement `ConditionBehavior` and listen to events:

```go
// In rulebooks/dnd5e/monstertraits/pack_tactics.go

// PackTactics - advantage if ally adjacent to target
func PackTactics(ownerID string) dnd5eEvents.ConditionBehavior {
    return &packTacticsCondition{ownerID: ownerID}
}

func (p *packTacticsCondition) Apply(ctx context.Context, bus events.EventBus) error {
    // Subscribe to attack events, grant advantage when ally adjacent to target
}

// Undead Fortitude - CON save to stay at 1 HP when dropped to 0
func UndeadFortitude(ownerID string, conModifier int) dnd5eEvents.ConditionBehavior

// Vulnerability/Immunity - modify incoming damage
func Vulnerability(ownerID string, damageType damage.Type) dnd5eEvents.ConditionBehavior
func Immunity(ownerID string, damageType damage.Type) dnd5eEvents.ConditionBehavior
```

**Architecture note:** Character feature behaviors live in `conditions/`, monster trait
behaviors live in `monstertraits/`. Both implement `ConditionBehavior`. Future shared
status effects (Poisoned, Prone) will go in a `statuses/` package.

### Targeting Strategies

Three AI targeting preferences:

```go
type TargetingStrategy int

const (
    TargetClosest   TargetingStrategy = iota  // Default - current behavior
    TargetLowestHP                             // Focus fire wounded
    TargetLowestAC                             // Hit the squishy ones
)

func (m *Monster) SetTargeting(strategy TargetingStrategy)
```

Selection logic integrates with existing `selectTarget()` in TakeTurn.

### Monster Factory Example

```go
func NewWolf(id string) *Monster {
    m := New(Config{
        ID:   id,
        Name: "Wolf",
        HP:   11,
        AC:   13,
        AbilityScores: shared.AbilityScores{
            abilities.STR: 12,
            abilities.DEX: 15,
            abilities.CON: 12,
            abilities.INT: 3,
            abilities.WIS: 12,
            abilities.CHA: 6,
        },
    })

    m.AddAction(NewBiteAction(BiteConfig{
        AttackBonus: 4,
        DamageDice:  "2d4+2",
        KnockdownDC: 11,
    }))

    m.AddCondition(PackTactics())
    m.SetTargeting(TargetLowestHP)  // Wolves focus wounded prey
    m.SetSpeed(SpeedData{Walk: 40})

    return m
}
```

## File Structure

```
rulebooks/dnd5e/
├── monstertraits/              # NEW: Monster-specific behaviors (sibling to conditions/)
│   ├── pack_tactics.go         # Advantage if ally adjacent to target
│   ├── undead_fortitude.go     # CON save to stay at 1 HP
│   ├── vulnerability.go        # Damage vulnerability modifier
│   ├── immunity.go             # Damage immunity modifier
│   └── loader.go               # LoadJSON for trait deserialization
│
├── monster/
│   ├── monster.go              # Core Monster struct (exists)
│   ├── data.go                 # Serialization (exists)
│   ├── action.go               # MonsterAction interface (exists)
│   ├── action_loader.go        # LoadAction factory (exists)
│   │
│   ├── actions/                # NEW: Generic action building blocks
│   │   ├── melee.go            # Generic melee attack action
│   │   ├── ranged.go           # Generic ranged attack action
│   │   ├── multiattack.go      # Multiattack action (for bosses)
│   │   └── bite.go             # Bite with knockdown (wolves, bears)
│   │
│   ├── targeting.go            # NEW: TargetingStrategy enum + selection logic
│   │
│   ├── monsters/               # NEW: Monster factory functions
│   │   ├── skeleton.go
│   │   ├── zombie.go
│   │   ├── ghoul.go
│   │   ├── giant_rat.go
│   │   ├── wolf.go
│   │   ├── brown_bear.go
│   │   ├── bandit.go           # Melee and ranged variants
│   │   └── thug.go
│   │
│   └── goblin.go               # Existing (consider moving to monsters/)
```

**Import graph (no cycles):**
```
events ← monstertraits
events ← monster
monstertraits ← monster/monsters (factories use traits)
monster/actions ← monster/monsters (factories use actions)
```

## Implementation Order

1. **Actions** (#446) - Melee, Ranged, Multiattack, Bite in `monster/actions/`
2. **Targeting** (#447) - Strategy enum + selection logic in `monster/targeting.go`
3. **Monster Traits** (#448) - Pack Tactics, Undead Fortitude, Vulnerability/Immunity in `monstertraits/`
4. **Monsters** (#449) - Factory functions in `monster/monsters/` using building blocks

## Definition of Done

- [ ] 9 monsters creatable via factory functions
- [ ] 3 targeting strategies working
- [ ] Pack Tactics gives advantage when ally adjacent
- [ ] Undead Fortitude allows CON save at 0 HP
- [ ] Vulnerability/Immunity modifies damage correctly
- [ ] Each monster has tests

## References

- D&D 5e API: https://www.dnd5eapi.co/api/2014/monsters
- Existing Goblin implementation: `rulebooks/dnd5e/monster/goblin.go`
- Project: https://github.com/users/KirkDiggler/projects/10