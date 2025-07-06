    # RPG Toolkit Architecture Diagram

## Overview
RPG Toolkit provides infrastructure for tabletop RPG mechanics through an event-driven architecture. Games implement their specific rules using our generic tools.

## Core Architecture

```mermaid
graph TB
    subgraph "Game Layer (Your Implementation)"
        Game[Game Rules]
        DND[D&D 5e Rules]
        PF[Pathfinder Rules]
        Custom[Custom RPG Rules]
    end

    subgraph "RPG Toolkit Infrastructure"
        subgraph "Core"
            Entity[Entity Interface<br/>- ID<br/>- Type]
            Events[Event Bus<br/>- Publish<br/>- Subscribe<br/>- Chain Events]
        end

        subgraph "Mechanics"
            Dice[Dice<br/>- Roll expressions<br/>- Modifiers]
            Conditions[Conditions<br/>- Apply/Remove<br/>- Duration<br/>- Effects]
            Proficiency[Proficiency<br/>- Skill bonuses<br/>- Weapon/tool use]
            Resources[Resources<br/>- Pools<br/>- Consumption<br/>- Restoration]
            Effects[Effects Core<br/>- Shared infrastructure<br/>- Subscription tracking]
        end
    end

    Game --> Entity
    Game --> Events
    DND --> Entity
    DND --> Events
    
    Conditions --> Effects
    Proficiency --> Effects
    Resources --> Events
    Conditions --> Events
    Proficiency --> Events
    Dice --> Events
```

## Event Flow Example: D&D 5e Attack with Bless

```mermaid
sequenceDiagram
    participant Player
    participant Game
    participant EventBus
    participant Dice
    participant Conditions
    participant Resources

    Player->>Game: Attack with longsword
    Game->>EventBus: Publish "attack.before"
    
    EventBus->>Conditions: Notify subscribers
    Note over Conditions: Bless condition listening
    Conditions->>EventBus: Add 1d4 modifier
    
    EventBus->>Proficiency: Notify subscribers
    Note over Proficiency: Weapon proficiency
    Proficiency->>EventBus: Add proficiency bonus
    
    Game->>Dice: Roll 1d20 + modifiers
    Dice->>Dice: Roll base die (15)
    Dice->>Dice: Roll Bless die (3)
    Dice->>Game: Total: 15 + 3 + 5 = 23
    
    Game->>EventBus: Publish "attack.hit"
    EventBus->>Resources: Notify subscribers
    Note over Resources: Superiority die
    Resources->>Game: Add 1d8 damage
    
    Game->>Player: Attack hits for damage
```

## Component Interaction: Rest Mechanics

```mermaid
graph LR
    subgraph "Game Decides When"
        LongRest[Player takes long rest]
        ShortRest[Player takes short rest]
        Dawn[Dawn occurs]
        Milestone[Story milestone]
    end

    subgraph "Toolkit Processes"
        Pool[Resource Pool]
        Trigger[ProcessRestoration<br/>Generic trigger handler]
    end

    subgraph "Resources Respond"
        SpellSlots[Spell Slots<br/>Triggers:<br/>- long_rest: -1]
        HitDice[Hit Dice<br/>Triggers:<br/>- long_rest: 5]
        ChannelDiv[Channel Divinity<br/>Triggers:<br/>- short_rest: -1<br/>- dawn: -1]
        Custom[Custom Resource<br/>Triggers:<br/>- milestone: 3<br/>- prayer: 1]
    end

    LongRest -->|"pool.ProcessRestoration('long_rest')"| Pool
    ShortRest -->|"pool.ProcessRestoration('short_rest')"| Pool
    Dawn -->|"pool.ProcessRestoration('dawn')"| Pool
    Milestone -->|"pool.ProcessRestoration('milestone')"| Pool

    Pool --> Trigger
    Trigger --> SpellSlots
    Trigger --> HitDice
    Trigger --> ChannelDiv
    Trigger --> Custom
```

## Effect Composition Pattern

```mermaid
graph TB
    subgraph "Base Infrastructure"
        EffectCore[Effect Core<br/>- Apply/Remove<br/>- Event subscriptions<br/>- State tracking]
    end

    subgraph "Domain Types"
        Condition[Condition<br/>Uses Effect Core]
        Proficiency[Proficiency<br/>Uses Effect Core]
        Feature[Feature<br/>Uses Effect Core]
        Equipment[Equipment Effect<br/>Uses Effect Core]
    end

    subgraph "D&D Examples"
        Bless[Bless<br/>Type: Condition<br/>Duration: 10 rounds<br/>Effect: +1d4 attacks]
        Poisoned[Poisoned<br/>Type: Condition<br/>Effect: Disadvantage]
        
        WeaponProf[Longsword Proficiency<br/>Type: Proficiency<br/>Effect: Add prof bonus]
        SkillProf[Acrobatics Proficiency<br/>Type: Proficiency<br/>Effect: Add prof bonus]
        
        Rage[Barbarian Rage<br/>Type: Feature<br/>Duration: 10 rounds<br/>Resource: Rage uses<br/>Effect: Damage resistance]
        
        MagicSword[+1 Longsword<br/>Type: Equipment<br/>Effect: +1 attack/damage]
    end

    EffectCore --> Condition
    EffectCore --> Proficiency
    EffectCore --> Feature
    EffectCore --> Equipment

    Condition --> Bless
    Condition --> Poisoned
    Proficiency --> WeaponProf
    Proficiency --> SkillProf
    Feature --> Rage
    Equipment --> MagicSword
```

## Key Design Principles

1. **Infrastructure, Not Implementation**: Toolkit provides generic event handling, games define what "poisoned" means
2. **Event-Driven Communication**: Modules don't call each other directly, they communicate through events
3. **Composition Over Inheritance**: Effects are composed from behaviors, not inherited from base classes
4. **Game-Agnostic Triggers**: Resources restore on "dawn", not "D&D long rest"

## Example: Spell Casting Flow

```mermaid
flowchart TD
    Start[Player casts Fireball]
    
    CheckSlot{Has level 3+<br/>spell slot?}
    Start --> CheckSlot
    
    CheckSlot -->|No| Fail[Cast fails]
    CheckSlot -->|Yes| ConsumeSlot[Consume spell slot<br/>pool.ConsumeSpellSlot level 3]
    
    ConsumeSlot --> PublishCast[Publish spell.cast event]
    
    PublishCast --> Counterspell{Enemy<br/>counterspells?}
    
    Counterspell -->|Yes| Cancel[Event cancelled<br/>Spell fails]
    Counterspell -->|No| Targets[Select targets]
    
    Targets --> SaveLoop[For each target]
    
    SaveLoop --> PublishSave[Publish saving.throw event]
    PublishSave --> RollSave[Target rolls DEX save]
    
    RollSave --> SaveResult{Save >= DC?}
    SaveResult -->|Yes| HalfDamage[Take half damage]
    SaveResult -->|No| FullDamage[Take full damage]
    
    HalfDamage --> NextTarget{More targets?}
    FullDamage --> NextTarget
    
    NextTarget -->|Yes| SaveLoop
    NextTarget -->|No| Complete[Spell complete]
```

## Benefits of This Architecture

- **Extensibility**: New mechanics don't require toolkit changes
- **Modularity**: Each component can be used independently  
- **Testability**: Components can be tested in isolation
- **Flexibility**: Games can implement any rule system
- **Reusability**: Common patterns (conditions, resources) work across different games

## D&D 5e Implementation Example

```go
// Game layer defines D&D-specific rules
type DnD5eGame struct {
    eventBus *events.Bus
    pools    map[string]resources.Pool
}

// Initialize a character with D&D mechanics
func (g *DnD5eGame) CreateWizard(level int) {
    wizard := &Character{ID: "wizard-1", Type: "character"}
    pool := resources.NewSimplePool(wizard)
    
    // D&D-specific spell slots
    spellSlots := resources.CreateSpellSlots(wizard, map[int]int{
        1: 4, // 4 first level slots
        2: 3, // 3 second level slots  
        3: 2, // 2 third level slots
    })
    
    // D&D-specific rest mechanics
    for _, slot := range spellSlots {
        pool.Add(slot) // Already configured for long rest
    }
    
    // Listen for D&D rest events
    g.eventBus.Subscribe("game.long_rest_completed", func(e Event) {
        pool.ProcessRestoration("long_rest", g.eventBus)
    })
    
    // Listen for dawn (some D&D abilities refresh at dawn)
    g.eventBus.Subscribe("time.dawn", func(e Event) {
        pool.ProcessRestoration("dawn", g.eventBus)
    })
}
```

This architecture allows rpg-toolkit to support any tabletop RPG system while maintaining clean separation between generic infrastructure and game-specific rules.