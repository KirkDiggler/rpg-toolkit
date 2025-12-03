# Query Topics Design

**Date:** 2024-12-01
**Status:** Draft
**Builds on:** 001-problem-statement.md

## Core Idea

Reuse the chain pattern for **questions**, not just **actions**:

| Pattern | Purpose | Flow |
|---------|---------|------|
| Action Chain | Modify ongoing action | AttackEvent → modifiers → FinalAttackEvent |
| Query Chain | Gather current state | ACQuery → contributions → ACResult |

## Query Topic Structure

### Generic Query Event Pattern

```go
// All queries follow this pattern
type QueryEvent[T any] struct {
    // Who/what is being queried
    SubjectID string

    // Input data needed for computation
    Input T

    // Accumulated contributions (grows as chain executes)
    Contributions []Contribution
}

type Contribution struct {
    Source      string  // "Unarmored Defense", "Shield spell", etc.
    Type        string  // "bonus", "set", "advantage", "immunity"
    Value       any     // +2, 15, true, etc.
    Description string  // Human-readable explanation
    Priority    int     // For ordering/stacking rules
}
```

### AC Query Example

```go
// rulebooks/dnd5e/queries/ac.go

type ACQueryInput struct {
    BaseAC         int
    Scores         shared.AbilityScores
    IsWearingArmor bool
    HasShield      bool
    ArmorType      string  // "light", "medium", "heavy", "none"
}

type ACQueryEvent struct {
    CharacterID   string
    Input         ACQueryInput
    Contributions []ACContribution
}

type ACContribution struct {
    Source      string
    Type        ACModType  // SetAC, BonusAC, etc.
    Value       int
    Description string
}

type ACModType string
const (
    ACModSet   ACModType = "set"   // "Your AC is X" (Unarmored Defense, Barkskin)
    ACModBonus ACModType = "bonus" // "+X to AC" (Shield spell, Shield of Faith)
)

var ACQueryTopic = events.DefineChainedTopic[*ACQueryEvent]("dnd5e.query.ac")
```

### How Conditions Subscribe

```go
// In UnarmoredDefenseCondition.Apply()
func (u *UnarmoredDefenseCondition) Apply(ctx context.Context, bus events.EventBus) error {
    u.bus = bus

    // Subscribe to AC queries
    acQueries := queries.ACQueryTopic.On(bus)
    subID, err := acQueries.SubscribeWithChain(ctx, u.onACQuery)
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
    // Only respond if this is about our character
    if event.CharacterID != u.CharacterID {
        return c, nil
    }

    // Only applies when not wearing armor
    if event.Input.IsWearingArmor {
        return c, nil
    }

    // Calculate our AC
    dexMod := event.Input.Scores.Modifier(abilities.DEX)
    secMod := event.Input.Scores.Modifier(u.SecondaryAbility())
    totalAC := 10 + dexMod + secMod

    // Add contribution at StageFeatures
    err := c.Add(combat.StageFeatures, "unarmored_defense",
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

### How to Query

```go
// Somewhere that needs to know AC
func GetEffectiveAC(ctx context.Context, bus events.EventBus, charID string, input queries.ACQueryInput) (*ACResult, error) {
    query := &queries.ACQueryEvent{
        CharacterID:   charID,
        Input:         input,
        Contributions: []queries.ACContribution{},
    }

    // Create chain and publish
    queryChain := events.NewStagedChain[*queries.ACQueryEvent](queries.QueryStages)
    acQueries := queries.ACQueryTopic.On(bus)

    finalChain, err := acQueries.PublishWithChain(ctx, query, queryChain)
    if err != nil {
        return nil, err
    }

    result, err := finalChain.Execute(ctx, query)
    if err != nil {
        return nil, err
    }

    // Process contributions to get final AC
    return computeFinalAC(result), nil
}

func computeFinalAC(event *queries.ACQueryEvent) *ACResult {
    // Start with base AC from armor (or 10 if none)
    baseAC := event.Input.BaseAC

    // Check for "set" modifiers (take highest)
    highestSet := baseAC
    var setSource string
    for _, c := range event.Contributions {
        if c.Type == queries.ACModSet && c.Value > highestSet {
            highestSet = c.Value
            setSource = c.Source
        }
    }

    // Apply bonuses to the base (set or armor)
    finalAC := highestSet
    for _, c := range event.Contributions {
        if c.Type == queries.ACModBonus {
            finalAC += c.Value
        }
    }

    return &ACResult{
        FinalAC:       finalAC,
        BaseSource:    setSource,
        Contributions: event.Contributions,
    }
}
```

## Other Query Topics Needed

### Attack Roll Query
```go
type AttackQueryEvent struct {
    AttackerID    string
    TargetID      string
    WeaponID      string
    IsMelee       bool
    Input         AttackQueryInput
    Contributions []AttackContribution
}

// Contributions could be:
// - Flat bonuses (+2 from Archery)
// - Dice additions (+1d4 from Bless)
// - Advantage/Disadvantage sources
// - Auto-hit/auto-miss conditions
```

### Saving Throw Query
```go
type SaveQueryEvent struct {
    CharacterID   string
    SaveType      abilities.Ability  // STR, DEX, CON, etc.
    TriggerSource string             // What caused this save
    Input         SaveQueryInput
    Contributions []SaveContribution
}

// Contributions:
// - Proficiency
// - Bonuses (Bless, magic items)
// - Advantage (Danger Sense for DEX saves)
// - Immunity (auto-succeed certain saves)
```

### Damage Resistance Query
```go
type ResistanceQueryEvent struct {
    CharacterID   string
    DamageType    string  // "fire", "slashing", etc.
    DamageSource  string  // "weapon", "spell", etc.
    Contributions []ResistanceContribution
}

// Contributions:
// - Resistance (half damage)
// - Immunity (no damage)
// - Vulnerability (double damage)
```

## Query Stages

Queries might need different stages than action chains:

```go
var QueryStages = []chain.Stage{
    StageBase,       // Base calculation (armor AC, base save)
    StageRace,       // Racial features
    StageClass,      // Class features
    StageConditions, // Active conditions
    StageEquipment,  // Magic items, equipped gear
    StageSpells,     // Active spell effects
    StageFinal,      // Final adjustments
}
```

## Advantages of This Pattern

1. **Consistent with existing code** - Same chain pattern we already use
2. **Self-registering** - Effects subscribe during Apply(), no central registry
3. **Full breakdown** - Every contribution tracked with source
4. **Decoupled** - Querier doesn't know about specific effects
5. **Extensible** - New effects just subscribe to relevant queries

## Concerns

1. **Subscription overhead** - Conditions subscribe to many topics
2. **Query frequency** - How often do we query? Cache?
3. **Topic proliferation** - Might need many query topics
4. **Ordering** - How to handle conflicting "set" values?

## Open Questions

1. Should queries be synchronous or go through the event bus?
2. Do we need a QueryBus separate from EventBus?
3. How to handle queries for things that don't have an active condition? (e.g., base proficiencies)

## Next: Context Pattern

See 003-context-pattern.md for the alternative approach using enriched context.
