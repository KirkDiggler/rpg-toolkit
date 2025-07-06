# RPG Toolkit: Architecture Vision & Capabilities

## The Vision

RPG Toolkit is a universal infrastructure for tabletop RPG mechanics. It provides the engine - you provide the rules.

```mermaid
graph TB
    subgraph "Any Rulebook"
        DND5e[D&D 5e]
        PF2e[Pathfinder 2e]
        CoC[Call of Cthulhu]
        FATE[FATE System]
        Homebrew[Your Custom RPG]
    end

    subgraph "RPG Toolkit Infrastructure"
        EventBus[Universal Event Bus]
        Entities[Entity System]
        Modifiers[Modifier Framework]
        Resources[Resource Management]
        Conditions[Condition System]
        Relationships[Entity Relationships]
    end

    DND5e --> EventBus
    PF2e --> EventBus
    CoC --> EventBus
    FATE --> EventBus
    Homebrew --> EventBus
```

## Core Power: The Event-Driven Architecture

### Rich Event Context & Modification

```mermaid
sequenceDiagram
    participant Game
    participant EventBus
    participant Proficiency
    participant MagicItem
    participant Condition
    participant Feature

    Game->>EventBus: skill_check.before {<br/>skill: "stealth",<br/>base_roll: 1d20,<br/>modifiers: []<br/>}
    
    Note over EventBus: All subscribers can modify context
    
    EventBus->>Proficiency: Notify (Priority: 100)
    Proficiency->>EventBus: Add proficiency bonus +3
    
    EventBus->>MagicItem: Notify (Priority: 90)
    Note over MagicItem: Cloak of Elvenkind
    MagicItem->>EventBus: Add advantage
    
    EventBus->>Condition: Notify (Priority: 80)
    Note over Condition: Invisible condition
    Condition->>EventBus: Add +10 bonus
    
    EventBus->>Feature: Notify (Priority: 70)
    Note over Feature: Rogue's Expertise
    Feature->>EventBus: Double proficiency bonus
    
    EventBus->>Game: Final context {<br/>skill: "stealth",<br/>base_roll: 1d20,<br/>modifiers: [+3, +3, +10],<br/>advantage: true<br/>}
```

### Multi-System Combat Example

```mermaid
flowchart TD
    subgraph "Same Event, Different Rules"
        AttackEvent[attack.before Event]
        
        subgraph "D&D 5e Handler"
            DND_AC[Check AC vs 1d20]
            DND_ADV[Apply Advantage]
            DND_CRIT[Crit on nat 20]
        end
        
        subgraph "Pathfinder 2e Handler"
            PF2_AC[Check AC vs 1d20]
            PF2_DEGREES[Success Degrees]
            PF2_CRIT[Crit on +10 over AC]
        end
        
        subgraph "FATE Handler"
            FATE_DICE[Roll 4dF]
            FATE_ASPECTS[Invoke Aspects]
            FATE_BOOST[Spend Fate Points]
        end
    end
    
    AttackEvent --> DND_AC
    AttackEvent --> PF2_AC
    AttackEvent --> FATE_DICE
```

## The Modifier System: Type-Safe Flexibility

```mermaid
classDiagram
    class ModifierValue {
        <<interface>>
        +Calculate() int
        +Description() string
    }
    
    class RawValue {
        +value: int
        +Calculate() int
    }
    
    class DiceValue {
        +expression: string
        +result: int
        +Calculate() int
    }
    
    class Multiplier {
        +multiplier: float64
        +Calculate() int
    }
    
    class Advantage {
        +Calculate() int
    }
    
    ModifierValue <|-- RawValue
    ModifierValue <|-- DiceValue
    ModifierValue <|-- Multiplier
    ModifierValue <|-- Advantage
    
    class AttackContext {
        +modifiers: []ModifierValue
        +AddModifier(m ModifierValue)
        +TotalBonus() int
    }
    
    AttackContext --> ModifierValue
```

### Example: Complex Modifier Stacking

```go
// D&D 5e Paladin Attack with multiple modifiers
eventBus.Subscribe("attack.before", func(ctx context.Context, e Event) error {
    attack := e.(*AttackEvent)
    
    // Base modifiers
    attack.AddModifier(RawValue{5})           // +5 STR
    attack.AddModifier(RawValue{3})           // +3 proficiency
    attack.AddModifier(RawValue{1})           // +1 magic weapon
    
    // Conditional modifiers
    if attack.Target.HasTag("undead") {
        attack.AddModifier(DiceValue{"1d8"})  // Divine Strike
    }
    
    // Buffs
    if HasCondition("bless") {
        attack.AddModifier(DiceValue{"1d4"})  // Bless (rolled fresh)
    }
    
    // Debuffs
    if HasCondition("frightened") {
        attack.AddModifier(Disadvantage{})    // Disadvantage
    }
    
    return nil
})
```

## Entity Relationships: Modeling Complex Interactions

```mermaid
graph TD
    subgraph "Concentration (D&D)"
        Wizard[Wizard Entity]
        Haste[Haste Spell]
        Fly[Fly Spell]
        Wizard -->|concentrates on| Haste
        Wizard -.->|would break| Fly
    end
    
    subgraph "Auras (Multiple Systems)"
        Paladin[Paladin Entity]
        Aura[Protection Aura]
        Ally1[Fighter]
        Ally2[Rogue]
        Paladin -->|emanates| Aura
        Aura -->|affects| Ally1
        Aura -->|affects| Ally2
    end
    
    subgraph "Dependencies (Any System)"
        Rage[Rage Feature]
        Frenzy[Frenzied Rage]
        Mindless[Mindless Rage]
        Rage -->|enables| Frenzy
        Rage -->|enables| Mindless
    end
```

## Resource Management: Universal Patterns

```mermaid
flowchart LR
    subgraph "Game Defines Triggers"
        DND_LR[D&D: Long Rest]
        DND_SR[D&D: Short Rest]
        PF_10MIN[PF2e: 10 Min Rest]
        PF_DAILY[PF2e: Daily Prep]
        FATE_SCENE[FATE: Scene End]
        COC_THERAPY[CoC: Therapy Session]
    end
    
    subgraph "Toolkit Processes"
        Pool[Resource Pool]
        Trigger[ProcessRestoration<br/>with trigger]
    end
    
    subgraph "Resources Respond"
        SpellSlots[Spell Slots<br/>Triggers:<br/>- dnd.long_rest<br/>- pf.daily_prep]
        FocusPoints[Focus Points<br/>Triggers:<br/>- pf.10_min]
        FatePoints[Fate Points<br/>Triggers:<br/>- fate.scene_end]
        Sanity[Sanity<br/>Triggers:<br/>- coc.therapy]
    end
    
    DND_LR --> Pool
    PF_DAILY --> Pool
    FATE_SCENE --> Pool
    COC_THERAPY --> Pool
    
    Pool --> Trigger
    Trigger --> SpellSlots
    Trigger --> FocusPoints
    Trigger --> FatePoints
    Trigger --> Sanity
```

## Real-World Use Cases

### 1. Running Multiple Systems Simultaneously

```go
// Same table, different characters, different rules!
type MultiSystemGame struct {
    eventBus *events.Bus
    dndHandler *DnD5eRules
    pfHandler *Pathfinder2eRules
    fateHandler *FATERules
}

func (g *MultiSystemGame) Initialize() {
    // Each system subscribes to same events with different logic
    g.dndHandler.Subscribe(g.eventBus)
    g.pfHandler.Subscribe(g.eventBus)
    g.fateHandler.Subscribe(g.eventBus)
}

// When a character acts, their system's rules apply
func (g *MultiSystemGame) CharacterAttacks(char Entity) {
    g.eventBus.Publish("attack.initiate", AttackEvent{
        Attacker: char,
        System: char.GetTag("system"), // "dnd5e", "pf2e", etc
    })
}
```

### 2. Homebrew Magic System

```go
// Custom spell system with unique mechanics
type ElementalMagic struct {
    eventBus *events.Bus
}

func (em *ElementalMagic) Subscribe(bus *events.Bus) {
    // Custom resource: Elemental Attunement
    bus.Subscribe("spell.before_cast", func(ctx context.Context, e Event) error {
        spell := e.(*SpellEvent)
        
        // Check elemental alignment
        if !em.HasAttunement(spell.Element) {
            e.Cancel() // Can't cast without attunement
            return nil
        }
        
        // Consume attunement points instead of spell slots
        pool := GetPool(spell.Caster)
        err := pool.Consume(fmt.Sprintf("attunement_%s", spell.Element), 1, bus)
        if err != nil {
            e.Cancel()
        }
        
        return nil
    })
    
    // Environmental bonuses
    bus.Subscribe("spell.calculate_damage", func(ctx context.Context, e Event) error {
        spell := e.(*SpellDamageEvent)
        
        // Near volcano? Fire spells do more
        if spell.Element == "fire" && IsNearVolcano(spell.Location) {
            spell.AddModifier(Multiplier{1.5})
        }
        
        return nil
    })
}
```

### 3. Complex Condition Interactions

```mermaid
stateDiagram-v2
    [*] --> Normal
    
    Normal --> Poisoned: Failed save
    Normal --> Diseased: Contact
    
    Poisoned --> Paralyzed: Poison progresses
    Diseased --> Fevered: Disease progresses
    
    Paralyzed --> Unconscious: Combined effects
    Fevered --> Unconscious: Combined effects
    
    Unconscious --> Dead: Failed death saves
    
    note right of Poisoned
        Each condition:
        - Is an entity
        - Has relationships
        - Publishes events
        - Modifies other events
    end note
```

### 4. Time-Based Mechanics

```go
// Different time systems for different games
type TimeSystem interface {
    Subscribe(bus *events.Bus)
}

// D&D: Rounds (6 seconds)
type DnDTime struct{}
func (d *DnDTime) Subscribe(bus *events.Bus) {
    bus.Subscribe("time.round_end", func(ctx context.Context, e Event) error {
        // Reduce condition durations
        // Restore reactions
        // Trigger round-based effects
        return nil
    })
}

// Narrative: Scenes
type FATETime struct{}
func (f *FATETime) Subscribe(bus *events.Bus) {
    bus.Subscribe("time.scene_end", func(ctx context.Context, e Event) error {
        // Clear temporary aspects
        // Restore fate points
        // Reset scene-based resources
        return nil
    })
}
```

## The Power of Composition

### Building Complex Features

```mermaid
graph TB
    subgraph "Barbarian Rage (D&D 5e)"
        RageBase[Base Rage Feature]
        
        subgraph "Composed From"
            Resource[Resource: Rage Uses]
            Condition[Condition: Raging]
            Duration[Duration: 10 rounds]
            Resistance[Effect: Damage Resistance]
            Bonus[Effect: +2 damage]
            Restriction[Restriction: No spells]
        end
        
        RageBase --> Resource
        RageBase --> Condition
        Condition --> Duration
        Condition --> Resistance
        Condition --> Bonus
        Condition --> Restriction
    end
```

### Event Chain Example: Critical Hit

```mermaid
sequenceDiagram
    participant Attacker
    participant EventBus
    participant CritSystem
    participant Rogue
    participant Paladin
    participant DM
    
    Attacker->>EventBus: attack.rolled {roll: 20}
    EventBus->>CritSystem: Check for crit
    CritSystem->>EventBus: attack.critical_confirmed
    
    EventBus->>Rogue: Sneak Attack dice doubled
    Rogue->>EventBus: damage.add {dice: "6d6"}
    
    EventBus->>Paladin: Smite on crit?
    Paladin->>EventBus: ability.use {smite: true}
    Paladin->>EventBus: damage.add {dice: "4d8"}
    
    EventBus->>DM: Notify for narration
    DM->>EventBus: effect.add {flourish: "devastating"}
```

## Extensibility Examples

### Adding New Systems
- **Vampire: The Masquerade**: Blood pool resources, hunger dice, humanity
- **Shadowrun**: Edge points, initiative passes, cyberware
- **Kids on Bikes**: Collaborative advantages, powered character aspects

### Custom Mechanics
- **Corruption System**: Gradual transformation with event thresholds
- **Faction Reputation**: Entity relationships affecting all interactions
- **Weather Magic**: Environmental conditions modifying spell effects

### Tool Integration
- **VTT Integration**: Events drive visual effects and automation
- **AI Game Master**: Subscribe to events for narrative generation
- **Analytics**: Track all events for balance analysis

## Conclusion

RPG Toolkit's architecture enables:
- **True Multi-System Support**: Run any RPG on the same infrastructure
- **Deep Customization**: Every rule can be modified through events
- **Clean Separation**: Infrastructure vs implementation
- **Extensible Design**: New features without breaking existing code
- **Performance**: Event priorities and smart subscription management
- **Debugging**: Full event history and state tracking

The toolkit is not just infrastructure - it's a foundation for the future of digital tabletop gaming.