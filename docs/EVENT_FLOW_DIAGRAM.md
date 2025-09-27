# Event Flow and Relationships: Visual Architecture

## Overview: How Everything Connects

```mermaid
graph TB
    subgraph "Compile Time - Type Safety"
        TD[Typed Topic Definitions]
        CD[Chained Topic Definitions]
        ET[Event Types]
    end

    subgraph "Runtime - Event Bus"
        EB[Event Bus]
        TS[Typed Subscribers]
        CS[Chain Subscribers]
    end

    subgraph "Game Mechanics"
        ACT[Actions]
        EFF[Effects]
        REL[Relationships]
    end

    TD -->|.On(bus)| TS
    CD -->|.On(bus)| CS
    ACT -->|Publish| EB
    EB -->|Notify| TS
    EB -->|Collect| CS
    EFF -->|Subscribe| TS
    REL -->|Manage| EFF
```

## The Typed Topics Pattern

```
┌─────────────────────────────────────────────────────────────┐
│                     COMPILE TIME                            │
├─────────────────────────────────────────────────────────────┤
│  package combat                                             │
│                                                             │
│  type AttackEvent struct {                                  │
│      Attacker string                                        │
│      Target   string                                        │
│      Damage   int                                           │
│  }                                                          │
│                                                             │
│  var AttackTopic = DefineTypedTopic[AttackEvent](           │
│      "combat.attack"                                        │
│  )                                                          │
│  var AttackChain = DefineChainedTopic[AttackEvent](         │
│      "combat.attack.chain"                                  │
│  )                                                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      RUNTIME                                │
├─────────────────────────────────────────────────────────────┤
│  // Connect topic to bus                                    │
│  attacks := combat.AttackTopic.On(bus)                      │
│                                                             │
│  // Type-safe subscription                                  │
│  attacks.Subscribe(ctx, func(ctx, e AttackEvent) error {    │
│      // e is already typed as AttackEvent                   │
│      // No type assertions needed!                          │
│  })                                                         │
└─────────────────────────────────────────────────────────────┘
```

## Staged Chain Processing Flow

```
Attack Event Flow with Modifiers:

     AttackEvent
          │
          ▼
    ┌──────────┐
    │  Chain   │
    └──────────┘
          │
    ┌─────┴───────────────────────────┐
    │                                 │
    ▼                                 ▼
[StageBase: 100]            [Subscribers Add Modifiers]
    │                                 │
    │  ◄──────────────────────────────┘
    ▼
[StageFeatures: 200] ← Sneak Attack adds damage
    │
    ▼
[StageConditions: 300] ← Rage adds damage, Bless adds hit bonus
    │
    ▼
[StageEquipment: 400] ← Magic weapon adds bonus
    │
    ▼
[StageFinal: 500] ← Critical multiplier, Resistance
    │
    ▼
Modified AttackEvent
```

## Relationship Management Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Concentration Example                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Cleric                                                    │
│     │                                                       │
│     ├─[Concentration Relationship]                          │
│     │                                                       │
│     ├──→ BlessEffect1 → Fighter                             │
│     ├──→ BlessEffect2 → Rogue                               │
│     └──→ BlessEffect3 → Wizard                              │
│                                                             │
│   When Cleric takes damage:                                 │
│     1. DamageEvent published                                │
│     2. Triggers concentration save                          │
│     3. If failed: BreakAllRelationships(Cleric)             │
│     4. All three BlessEffects automatically removed         │
│                                                             │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                      Aura Example                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Paladin (center)                                          │
│     │                                                       │
│     ├─[Aura Relationship, range: 10ft]                      │
│     │                                                       │
│     ├──→ AuraBonus1 → Fighter (8ft away) ✓                  │
│     ├──→ AuraBonus2 → Cleric (5ft away) ✓                   │
│     └──X  [No bonus] → Rogue (15ft away) ✗                  │
│                                                             │
│   On Movement:                                              │
│     1. MovementEvent published                              │
│     2. RelationshipManager.UpdateAuras()                    │
│     3. Checks distances                                     │
│     4. Adds/removes effects based on range                  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Action → Effect → Event Flow

```
Player Action Flow:

[Player Input: "Cast Fireball at position (30, 20)"]
                    │
                    ▼
            ┌───────────────┐
            │ FireballAction│
            │ Action[T]     │
            └───────────────┘
                    │
         ┌──────────┴───────────┐
         ▼                      ▼
    CanActivate?           Activate!
    - Has spell slot?           │
    - In range?                 ├─→ Consume spell slot
    - Has components?           ├─→ Create damage event
                               └─→ Publish to bus
                                        │
                ┌───────────────────────┼───────────────────────┐
                ▼                       ▼                       ▼
        [Find Targets]          [Roll Damage]          [Trigger Saves]
         in 20ft radius          8d6 fire            DC 15 Dexterity
                │                       │                       │
                └───────────────────────┴───────────────────────┘
                                        │
                                        ▼
                                [Apply Damage]
                                 Modified by:
                                 - Resistance
                                 - Vulnerability
                                 - Shield spells
```

## Complete Combat Round Orchestration

```
┌─────────────────────────────────────────────────────────────┐
│                    Combat Round Flow                        │
└─────────────────────────────────────────────────────────────┘

[Initiative Phase]
    │
    ├─→ Roll initiatives (Lazy Dice)
    ├─→ Apply modifiers (Alert feat, etc.)
    └─→ Sort turn order

[For Each Combatant]
    │
    ├─→ [Turn Start Event]
    │     ├─→ Regeneration effects
    │     ├─→ DoT damage
    │     └─→ Start-of-turn saves
    │
    ├─→ [Concentration Check] (if took damage)
    │     ├─→ Build save chain
    │     ├─→ Apply modifiers (War Caster, etc.)
    │     └─→ Break relationships if failed
    │
    ├─→ [Action Phase]
    │     ├─→ Get available actions
    │     ├─→ Choose action (AI/Player)
    │     └─→ Execute Action[T]
    │           ├─→ Publish events
    │           ├─→ Trigger chains
    │           └─→ Apply effects
    │
    └─→ [Turn End Event]
          ├─→ Duration checks
          ├─→ Effect expiration
          └─→ End-of-turn saves

[Round End Event]
    ├─→ Duration countdowns
    ├─→ Lair actions
    └─→ Resource regeneration
```

## Event Types and Their Relationships

```
Event Topology:

TypedTopic[T] - One-way notifications
    ├─→ DamageEvent
    ├─→ MovementEvent
    ├─→ DeathEvent
    └─→ RestEvent

ChainedTopic[T] - Ordered modifier collection
    ├─→ AttackChain (modifies attack rolls)
    ├─→ DamageChain (modifies damage amounts)
    ├─→ SaveChain (modifies saving throws)
    └─→ InitiativeChain (modifies turn order)

Relationships - Lifecycle management
    ├─→ Concentration (one active per caster)
    ├─→ Aura (range-based)
    ├─→ Channeled (requires action)
    ├─→ Linked (removed together)
    └─→ Dependent (cascading removal)
```

## The Beauty: Composition of Simple Patterns

```
Complex Behavior from Simple Rules:

Fighter with Rage + Bless + Magic Weapon attacks:

1. Action[AttackInput] triggered
        │
2. AttackChain.On(bus) publishes event
        │
3. Chain collects modifiers:
        ├─→ Bless: +1d4 to hit (Stage 300)
        ├─→ Rage: +2 damage (Stage 300)
        └─→ Magic: +1 hit/damage (Stage 400)
        │
4. Chain.Execute() applies in order
        │
5. Final attack delivered with all modifiers

No special cases. No spaghetti code. Just patterns.
```

## Key Architectural Insights

1. **Compile-Time Safety**: Topics are defined with types at compile time
2. **Runtime Flexibility**: Effects subscribe dynamically at runtime
3. **Clean Separation**: Each layer has one responsibility
4. **Automatic Cleanup**: Relationships handle lifecycle management
5. **Order Matters**: Chains ensure correct modifier application
6. **No Magic Strings**: Everything is a typed constant

This architecture makes complex RPG mechanics feel simple and natural.