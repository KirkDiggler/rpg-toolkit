# Effect Query System - Problem Statement

**Date:** 2024-12-01
**Status:** Exploring
**Authors:** Kirk, Claude

## The Core Problem

We have effects (conditions, features, passives) that modify character state. Currently they:
1. Subscribe to action chains (AttackChain, DamageChain) to modify combat flow
2. Store state and respond to events (TurnEnd, DamageReceived, etc.)

What we **cannot** do:
- Ask "what is currently affecting my AC and by how much?"
- Ask "what modifiers apply to my saving throws right now?"
- Get a breakdown of where a computed value comes from

## Concrete Use Cases

### Use Case 1: AC Breakdown
A player wants to see their AC calculation:
```
Base AC: 10
+ DEX modifier: +3
+ Unarmored Defense (CON): +2
+ Shield spell: +5
- Prone condition: (attackers within 5ft have advantage, not AC)
= Total AC: 20
```

Currently, we'd have to manually track all this. There's no way to query "what affects AC?"

### Use Case 2: Sneak Attack Eligibility
Rogue wants to know if sneak attack applies:
- Is the target within 5 feet of an ally?
- Does the rogue have advantage on the attack?
- Is the weapon finesse or ranged?

This requires **encounter context** (who's where) that isn't in the current event model.

### Use Case 3: Attack Roll Modifiers
Before rolling, show what will affect the attack:
```
Base attack bonus: +5
+ Archery fighting style: +2
+ Bless: +1d4
- Poisoned condition: disadvantage
```

### Use Case 4: Condition Immunity Check
Before applying a condition, check if anything grants immunity:
- Elves are immune to magical sleep
- Paladins (high level) are immune to disease
- Some items grant condition immunities

## What We Have Today

### Action Chains (Working Well)
```go
// Damage flows through, modifiers accumulate
DamageChain.On(bus).PublishWithChain(ctx, damageEvent, chain)
// Execute applies all modifiers
finalEvent, _ := chain.Execute(ctx, damageEvent)
```

This works for **computation** - modifying values as they flow through.

### Conditions Subscribe to Topics
```go
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
    // Subscribe to damage chain to add +2 damage
    damageChain := combat.DamageChain.On(bus)
    damageChain.SubscribeWithChain(ctx, r.onDamageChain)
    // ...
}
```

Conditions hook themselves up, but there's no registry of "what modifies what."

## The Gap

We need a way to **query** effects for their contributions to specific attributes, with full context about the situation.

## Two Approaches Identified

### Approach A: Query Topics (Chain Pattern for Questions)
Use the same chain pattern but for gathering information:
```go
var ACQueryTopic = events.DefineChainedTopic[*ACQueryEvent]("dnd5e.query.ac")

// Conditions subscribe during Apply()
func (u *UnarmoredDefenseCondition) Apply(ctx, bus) {
    ACQueryTopic.On(bus).SubscribeWithChain(ctx, u.onACQuery)
}

// Query by publishing
query := &ACQueryEvent{CharacterID: "bob", BaseAC: 10}
result := ACQueryTopic.On(bus).PublishWithChain(ctx, query, chain)
// result.Contributions contains all modifier sources
```

**Pros:**
- Consistent with existing chain pattern
- Effects self-register for queries they care about
- Decoupled - querier doesn't need to know about specific effects

**Cons:**
- Every queryable attribute needs a topic
- Conditions must subscribe to many topics

### Approach B: Context-Carried Data
Enrich the context passed to effects with encounter/room data:
```go
type GameContext struct {
    context.Context
    Encounter *EncounterContext  // Who's where, initiative order
    Room      *RoomContext       // Spatial data
    Character *CharacterContext  // The acting character's full state
}

// Sneak attack checks context directly
func (s *SneakAttack) onDamageChain(ctx context.Context, event, chain) {
    gameCtx := GetGameContext(ctx)
    if gameCtx.Encounter.HasAllyAdjacentTo(event.TargetID) {
        // Add sneak attack damage
    }
}
```

**Pros:**
- Rich context available everywhere
- No need for separate query topics
- Natural for encounter-level data (positions, allies, etc.)

**Cons:**
- Context must be set up correctly before any event
- Harder to get "breakdown" of contributions
- Context could become bloated

### Approach C: Hybrid
- Use **Query Topics** for attribute breakdowns (AC, saves, skills)
- Use **Context** for situational data (encounter, positions, allies)

This might be the sweet spot.

## Questions to Resolve

1. **What needs query topics vs context?**
   - AC calculation → Query topic (want breakdown)
   - "Is ally adjacent?" → Context (situational)
   - Attack modifiers → Query topic? Or just let chain accumulate?

2. **When are queries executed?**
   - On demand when character sheet displayed?
   - Before each attack to show preview?
   - Cached and invalidated when conditions change?

3. **How does context get populated?**
   - Who sets up the encounter context?
   - What happens if context is missing?

4. **Should Effect declare what it modifies?**
   - Static declaration: `DeclaredModifiers() []ModifierTarget`
   - Or just subscribe to relevant query topics?

## Next Steps

1. Design the query topic pattern in detail (002-query-topics.md)
2. Design the context enrichment pattern (003-context-pattern.md)
3. Compare and decide on hybrid approach (004-hybrid-design.md)
4. Prototype with AC query as first example
