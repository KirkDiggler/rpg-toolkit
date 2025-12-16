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

### Conditions (Passive - subscribed to event bus)

Conditions listen to events and modify them:

```go
// Pack Tactics - advantage if ally adjacent to target
func PackTactics() ConditionBehavior {
    return &packTacticsCondition{}
}

func (p *packTacticsCondition) Subscribe(ctx context.Context, bus events.EventBus, ownerID string) error {
    topic := dnd5eEvents.AttackChainTopic.On(bus)
    _, err := topic.Subscribe(ctx, func(ctx context.Context, event dnd5eEvents.AttackChainEvent) error {
        if event.AttackerID != ownerID {
            return nil
        }
        room, _ := gamectx.Room(ctx)
        if room.HasAllyAdjacentTo(ownerID, event.TargetID) {
            event.AddAdvantage("pack_tactics")
        }
        return nil
    })
    return err
}

// Undead Fortitude - CON save to stay at 1 HP when dropped to 0
func UndeadFortitude() ConditionBehavior

// Vulnerability/Immunity - modify incoming damage
func Vulnerability(damageType damage.Type) ConditionBehavior
func Immunity(damageType damage.Type) ConditionBehavior
```

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
rulebooks/dnd5e/monster/
├── monster.go              # Core Monster struct (exists)
├── data.go                 # Serialization (exists)
├── action.go               # MonsterAction interface (exists)
├── action_loader.go        # LoadAction factory (exists)
│
├── actions/
│   ├── melee.go            # Generic melee attack action
│   ├── ranged.go           # Generic ranged attack action
│   ├── multiattack.go      # Multiattack action (for bosses)
│   └── bite.go             # Bite with knockdown (wolves, bears)
│
├── conditions/
│   ├── pack_tactics.go     # Advantage if ally adjacent to target
│   ├── undead_fortitude.go # CON save to stay at 1 HP
│   └── vulnerability.go    # Damage type modifiers
│
├── targeting.go            # TargetingStrategy enum + selection logic
│
├── monsters/
│   ├── skeleton.go
│   ├── zombie.go
│   ├── ghoul.go
│   ├── giant_rat.go
│   ├── wolf.go
│   ├── brown_bear.go
│   ├── bandit.go           # Melee and ranged variants
│   └── thug.go
│
└── goblin.go               # Existing (consider moving to monsters/)
```

## Implementation Order

1. **Actions** - Melee, Ranged, Multiattack, Bite generics
2. **Targeting** - Add strategy enum + selection logic to TakeTurn
3. **Conditions** - Pack Tactics, Undead Fortitude, Vulnerability/Immunity
4. **Monsters** - Create factories using building blocks
5. **Tests** - Verify each monster works in combat simulation

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