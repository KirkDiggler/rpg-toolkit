# Hybrid Design: Context + Query Topics

**Date:** 2024-12-01
**Status:** Draft (Recommended Approach)
**Builds on:** 002-query-topics.md, 003-context-pattern.md

## The Insight

Context and Query Topics solve **different problems**:

| Problem | Solution | Example |
|---------|----------|---------|
| "What's the current situation?" | Context | Who's adjacent, whose turn, positions |
| "What modifies X and by how much?" | Query Topic | AC breakdown, save modifiers |
| "Should this effect apply?" | Context | Sneak attack checks ally positions |
| "Show me all active bonuses" | Query Topic | Character sheet display |

## The Hybrid Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Game Context                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │  Encounter  │  │    Room     │  │    Actor    │              │
│  │  - who's    │  │  - spatial  │  │  - current  │              │
│  │    where    │  │    data     │  │    state    │              │
│  │  - teams    │  │  - terrain  │  │  - scores   │              │
│  │  - turn     │  │             │  │             │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Event Bus                                   │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Action Chains (existing)                                    ││
│  │  - AttackChain: modify attack rolls                         ││
│  │  - DamageChain: modify damage                               ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Query Chains (new)                                          ││
│  │  - ACQueryChain: gather AC contributions                    ││
│  │  - SaveQueryChain: gather save modifiers                    ││
│  │  - AttackModQueryChain: preview attack modifiers            ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Notification Topics (existing)                              ││
│  │  - TurnStart, TurnEnd, DamageReceived, etc.                 ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Effects/Conditions                           │
│  Subscribe to:                                                   │
│   - Action chains (to modify combat flow)                        │
│   - Query chains (to report their contributions)                 │
│   - Notification topics (to update state)                        │
│  Access:                                                         │
│   - Game context (to check situational conditions)               │
└─────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
rulebooks/dnd5e/
├── combat/
│   ├── attack.go          # AttackChain, DamageChain (existing)
│   └── stages.go          # Chain stages (existing)
├── queries/
│   ├── ac.go              # ACQueryTopic, ACQueryEvent
│   ├── saves.go           # SaveQueryTopic, SaveQueryEvent
│   ├── attacks.go         # AttackModQueryTopic (preview)
│   ├── stages.go          # Query-specific stages
│   └── doc.go             # Package documentation
├── context/
│   ├── game.go            # GameContext struct
│   ├── encounter.go       # EncounterContext
│   ├── room.go            # RoomContext (integrates with spatial)
│   └── helpers.go         # GetGameContext, WithGameContext
└── conditions/
    ├── raging.go          # Subscribes to DamageChain, queries
    ├── unarmored_defense.go # Subscribes to ACQuery
    └── sneak_attack.go    # Uses context + DamageChain
```

## Effect Subscription Pattern

Each effect subscribes to what it needs:

```go
func (e *SomeEffect) Apply(ctx context.Context, bus events.EventBus) error {
    // Subscribe to action chains (if we modify actions)
    if e.modifiesDamage {
        damageChain := combat.DamageChain.On(bus)
        subID, _ := damageChain.SubscribeWithChain(ctx, e.onDamageChain)
        e.subscriptionIDs = append(e.subscriptionIDs, subID)
    }

    // Subscribe to query chains (if we contribute to queries)
    if e.contributesToAC {
        acQuery := queries.ACQueryTopic.On(bus)
        subID, _ := acQuery.SubscribeWithChain(ctx, e.onACQuery)
        e.subscriptionIDs = append(e.subscriptionIDs, subID)
    }

    // Subscribe to notifications (if we react to events)
    if e.tracksHits {
        damages := dnd5eEvents.DamageReceivedTopic.On(bus)
        subID, _ := damages.Subscribe(ctx, e.onDamageReceived)
        e.subscriptionIDs = append(e.subscriptionIDs, subID)
    }

    return nil
}
```

## Example: Sneak Attack (Full Implementation)

```go
// rulebooks/dnd5e/conditions/sneak_attack.go

type SneakAttackCondition struct {
    CharacterID     string
    RogueLevel      int
    subscriptionIDs []string
    bus             events.EventBus

    // Tracked state from attack chain
    currentAttackHasAdvantage    bool
    currentAttackHasDisadvantage bool
}

func (s *SneakAttackCondition) Apply(ctx context.Context, bus events.EventBus) error {
    s.bus = bus

    // 1. Subscribe to AttackTopic to track advantage/disadvantage
    attacks := dnd5eEvents.AttackTopic.On(bus)
    subID1, _ := attacks.Subscribe(ctx, s.onAttack)
    s.subscriptionIDs = append(s.subscriptionIDs, subID1)

    // 2. Subscribe to AttackChain to track advantage sources
    attackChain := combat.AttackChain.On(bus)
    subID2, _ := attackChain.SubscribeWithChain(ctx, s.onAttackChain)
    s.subscriptionIDs = append(s.subscriptionIDs, subID2)

    // 3. Subscribe to DamageChain to add sneak attack damage
    damageChain := combat.DamageChain.On(bus)
    subID3, _ := damageChain.SubscribeWithChain(ctx, s.onDamageChain)
    s.subscriptionIDs = append(s.subscriptionIDs, subID3)

    return nil
}

func (s *SneakAttackCondition) onAttack(ctx context.Context, event dnd5eEvents.AttackEvent) error {
    if event.AttackerID != s.CharacterID {
        return nil
    }
    // Reset tracking for new attack
    s.currentAttackHasAdvantage = false
    s.currentAttackHasDisadvantage = false
    return nil
}

func (s *SneakAttackCondition) onAttackChain(
    ctx context.Context,
    event combat.AttackChainEvent,
    c chain.Chain[combat.AttackChainEvent],
) (chain.Chain[combat.AttackChainEvent], error) {
    if event.AttackerID != s.CharacterID {
        return c, nil
    }

    // Track advantage/disadvantage from chain
    // (Other effects would have added advantage modifiers by now)
    // This is a late-stage observer

    err := c.Add(combat.StageFinal, "sneak_attack_observer",
        func(ctx context.Context, e combat.AttackChainEvent) (combat.AttackChainEvent, error) {
            // Check if advantage was granted (would need to add this tracking)
            // For now, assume we track it elsewhere
            return e, nil
        })

    return c, err
}

func (s *SneakAttackCondition) onDamageChain(
    ctx context.Context,
    event *combat.DamageChainEvent,
    c chain.Chain[*combat.DamageChainEvent],
) (chain.Chain[*combat.DamageChainEvent], error) {
    if event.AttackerID != s.CharacterID {
        return c, nil
    }

    // Get game context for positional checks
    gameCtx := dnd5econtext.GetGameContext(ctx)

    // Determine eligibility
    eligible, reason := s.checkEligibility(gameCtx, event.TargetID)
    if !eligible {
        return c, nil
    }

    // Add sneak attack damage
    err := c.Add(combat.StageFeatures, "sneak_attack",
        func(ctx context.Context, e *combat.DamageChainEvent) (*combat.DamageChainEvent, error) {
            dice := s.sneakAttackDice()
            rolls := s.rollDice(ctx, dice)

            e.Components = append(e.Components, combat.DamageComponent{
                Source:            combat.DamageSourceSneakAttack,
                OriginalDiceRolls: rolls,
                FinalDiceRolls:    rolls,
                DamageType:        e.DamageType,
                IsCritical:        e.IsCritical,
            })
            return e, nil
        })

    return c, err
}

func (s *SneakAttackCondition) checkEligibility(gameCtx *dnd5econtext.GameContext, targetID string) (bool, string) {
    // Condition 1: Have advantage (and not also disadvantage)
    if s.currentAttackHasAdvantage && !s.currentAttackHasDisadvantage {
        return true, "advantage on attack"
    }

    // Condition 2: Ally adjacent to target (and no disadvantage)
    if !s.currentAttackHasDisadvantage {
        if gameCtx != nil && gameCtx.Encounter != nil {
            if gameCtx.Encounter.HasAllyAdjacentTo(s.CharacterID, targetID) {
                return true, "ally adjacent to target"
            }
        }
    }

    return false, ""
}

func (s *SneakAttackCondition) sneakAttackDice() int {
    // Rogues get (level+1)/2 d6s
    return (s.RogueLevel + 1) / 2
}
```

## Example: Unarmored Defense with AC Query

```go
// Updated to subscribe to AC query

func (u *UnarmoredDefenseCondition) Apply(ctx context.Context, bus events.EventBus) error {
    u.bus = bus

    // Subscribe to AC queries
    acQuery := queries.ACQueryTopic.On(bus)
    subID, err := acQuery.SubscribeWithChain(ctx, u.onACQuery)
    if err != nil {
        return err
    }
    u.subscriptionIDs = append(u.subscriptionIDs, subID)

    return nil
}

func (u *UnarmoredDefenseCondition) onACQuery(
    ctx context.Context,
    event *queries.ACQueryEvent,
    c chain.Chain[*queries.ACQueryEvent],
) (chain.Chain[*queries.ACQueryEvent], error) {
    if event.CharacterID != u.CharacterID {
        return c, nil
    }

    // Don't apply if wearing armor
    if event.Input.IsWearingArmor {
        return c, nil
    }

    // Calculate unarmored AC
    dexMod := event.Input.Scores.Modifier(abilities.DEX)
    secMod := event.Input.Scores.Modifier(u.SecondaryAbility())
    totalAC := 10 + dexMod + secMod

    err := c.Add(queries.StageClass, "unarmored_defense",
        func(ctx context.Context, e *queries.ACQueryEvent) (*queries.ACQueryEvent, error) {
            e.Contributions = append(e.Contributions, queries.ACContribution{
                Source:      "Unarmored Defense",
                Type:        queries.ACModSet,
                Value:       totalAC,
                Description: fmt.Sprintf("10 + DEX(%+d) + %s(%+d)", dexMod, u.SecondaryAbility(), secMod),
            })
            return e, nil
        })

    return c, err
}
```

## How Character Gets AC

```go
// In character package or wherever

func (c *Character) GetACWithBreakdown(ctx context.Context) (*queries.ACResult, error) {
    input := queries.ACQueryInput{
        BaseAC:         c.getArmorAC(),
        Scores:         c.AbilityScores,
        IsWearingArmor: c.Equipment.IsWearingArmor(),
        ArmorType:      c.Equipment.GetArmorType(),
        HasShield:      c.Equipment.HasShieldEquipped(),
    }

    return queries.QueryAC(ctx, c.bus, c.ID, input)
}

// Simpler version for just the number
func (c *Character) GetAC(ctx context.Context) int {
    result, err := c.GetACWithBreakdown(ctx)
    if err != nil {
        return c.getArmorAC() // Fallback
    }
    return result.FinalAC
}
```

## Summary: When to Use What

| Need | Use | Example |
|------|-----|---------|
| Modify action in progress | Action Chain | Add damage, modify attack roll |
| Get breakdown of modifiers | Query Chain | AC, saves, attack bonuses |
| Check situational conditions | Game Context | Ally positions, terrain |
| React to game events | Notification Topic | Track hits, end conditions |
| Persist effect state | ToJSON/loadJSON | Save/load conditions |

## Benefits of Hybrid

1. **Clear separation of concerns**
   - Context = situation (read-only facts)
   - Query = computation with breakdown
   - Action = modification in progress

2. **Effects declare capabilities** through subscriptions
   - Subscribe to ACQuery = "I affect AC"
   - Subscribe to DamageChain = "I modify damage"

3. **Full traceability**
   - Every modifier tracked to source
   - Can show "why is my AC 18?"

4. **Consistent patterns**
   - All use the same chain infrastructure
   - Effects have uniform Apply/Remove lifecycle

## Next Steps

1. Implement `rulebooks/dnd5e/queries/` package
2. Implement `rulebooks/dnd5e/context/` package
3. Update existing conditions to subscribe to queries
4. Add ACQuery as proof of concept
5. Test with character sheet display
