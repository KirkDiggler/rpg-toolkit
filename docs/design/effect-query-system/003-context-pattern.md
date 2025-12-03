# Context-Carried Data Pattern

**Date:** 2024-12-01
**Status:** Draft
**Builds on:** 001-problem-statement.md

## Core Idea

Instead of separate query topics, enrich the `context.Context` with game state that effects can access directly.

## The Game Context

```go
// core/gamecontext/context.go

type GameContext struct {
    context.Context

    // Encounter-level data (combat, initiative, positions)
    Encounter *EncounterContext

    // Room/spatial data
    Room *RoomContext

    // The acting character's full state
    Actor *ActorContext

    // Target of the current action (if any)
    Target *TargetContext
}

// Key for storing in context.Context
type gameContextKey struct{}

func WithGameContext(parent context.Context, gc *GameContext) context.Context {
    return context.WithValue(parent, gameContextKey{}, gc)
}

func GetGameContext(ctx context.Context) *GameContext {
    if gc, ok := ctx.Value(gameContextKey{}).(*GameContext); ok {
        return gc
    }
    return nil
}
```

## Encounter Context

```go
type EncounterContext struct {
    // Who's in the encounter
    Participants []Participant

    // Current initiative order
    InitiativeOrder []string  // Character IDs in order

    // Whose turn is it
    CurrentTurnID string

    // Round number
    Round int

    // Spatial relationships (from Room integration)
    Positions map[string]Position
}

type Participant struct {
    ID          string
    Team        Team        // Ally, Enemy, Neutral
    Position    Position
    Conditions  []string    // Active condition IDs
    IsConscious bool
    // etc.
}

type Team string
const (
    TeamAlly    Team = "ally"
    TeamEnemy   Team = "enemy"
    TeamNeutral Team = "neutral"
)

// Query methods on EncounterContext
func (e *EncounterContext) GetAlliesOf(characterID string) []Participant {
    var allies []Participant
    myTeam := e.GetTeam(characterID)
    for _, p := range e.Participants {
        if p.ID != characterID && e.GetTeam(p.ID) == myTeam {
            allies = append(allies, p)
        }
    }
    return allies
}

func (e *EncounterContext) HasAllyAdjacentTo(characterID, targetID string) bool {
    targetPos := e.Positions[targetID]
    for _, ally := range e.GetAlliesOf(characterID) {
        if ally.ID == characterID {
            continue // Skip self
        }
        if e.IsAdjacent(ally.Position, targetPos) {
            return true
        }
    }
    return false
}

func (e *EncounterContext) IsAdjacent(a, b Position) bool {
    // Within 5 feet (1 square)
    return abs(a.X-b.X) <= 1 && abs(a.Y-b.Y) <= 1
}
```

## How Sneak Attack Would Use It

```go
func (s *SneakAttackCondition) onDamageChain(
    ctx context.Context,
    event *combat.DamageChainEvent,
    c chain.Chain[*combat.DamageChainEvent],
) (chain.Chain[*combat.DamageChainEvent], error) {
    // Only apply to our attacks
    if event.AttackerID != s.CharacterID {
        return c, nil
    }

    // Get game context
    gameCtx := gamecontext.GetGameContext(ctx)
    if gameCtx == nil || gameCtx.Encounter == nil {
        // No encounter context - can't determine sneak attack eligibility
        // Could log warning or just skip
        return c, nil
    }

    // Check sneak attack conditions
    eligible := false
    reason := ""

    // Condition 1: Have advantage on the attack
    if s.hasAdvantage {  // Would need to track this from attack chain
        eligible = true
        reason = "advantage on attack"
    }

    // Condition 2: Ally adjacent to target (and no disadvantage)
    if !eligible && !s.hasDisadvantage {
        if gameCtx.Encounter.HasAllyAdjacentTo(s.CharacterID, event.TargetID) {
            eligible = true
            reason = "ally adjacent to target"
        }
    }

    if !eligible {
        return c, nil
    }

    // Add sneak attack damage
    err := c.Add(combat.StageFeatures, "sneak_attack",
        func(ctx context.Context, e *combat.DamageChainEvent) (*combat.DamageChainEvent, error) {
            sneakDice := s.calculateSneakAttackDice()  // Based on rogue level
            rolls := s.rollSneakAttackDamage(ctx, sneakDice)

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
```

## Who Sets Up the Context?

The **orchestrator** (combat manager, encounter handler) sets up context before publishing events:

```go
// In combat/resolver.go or similar

func (r *CombatResolver) ResolveAttack(ctx context.Context, input AttackInput) (*AttackResult, error) {
    // Build game context
    gameCtx := &gamecontext.GameContext{
        Context: ctx,
        Encounter: &gamecontext.EncounterContext{
            Participants:    r.encounter.GetParticipants(),
            InitiativeOrder: r.encounter.GetInitiativeOrder(),
            CurrentTurnID:   r.encounter.CurrentTurn(),
            Round:           r.encounter.Round(),
            Positions:       r.encounter.GetPositions(),
        },
        Actor: &gamecontext.ActorContext{
            ID:     input.Attacker.GetID(),
            Scores: input.AttackerScores,
            // etc.
        },
        Target: &gamecontext.TargetContext{
            ID: input.Defender.GetID(),
            AC: input.DefenderAC,
            // etc.
        },
    }

    // Create context with game data
    ctx = gamecontext.WithGameContext(ctx, gameCtx)

    // Now publish events - all subscribers have access to encounter data
    // ...
}
```

## Context for AC Queries?

Could the context pattern work for AC too?

```go
// When building character state
func (c *Character) GetEffectiveAC(ctx context.Context) int {
    // Build context with character's current state
    gameCtx := &gamecontext.GameContext{
        Context: ctx,
        Actor: &gamecontext.ActorContext{
            ID:             c.ID,
            Scores:         c.AbilityScores,
            IsWearingArmor: c.Equipment.IsWearingArmor(),
            ArmorType:      c.Equipment.GetArmorType(),
        },
    }
    ctx = gamecontext.WithGameContext(ctx, gameCtx)

    // Publish AC calculation event
    // Each condition can check context and contribute
    // ...
}
```

**Problem:** This doesn't give us the breakdown. Effects modify a value but we don't track who contributed what.

## Hybrid: Context + Query Results

```go
type ActorContext struct {
    ID     string
    Scores shared.AbilityScores
    // ...

    // Cached query results (with breakdowns)
    ACResult    *ACQueryResult    // Computed AC with sources
    SaveResults map[string]*SaveQueryResult
}

// Before combat, compute and cache
func (c *Character) PrepareForCombat(ctx context.Context, bus events.EventBus) error {
    // Run AC query
    acResult, err := queries.QueryAC(ctx, bus, c.ID, c.buildACInput())
    if err != nil {
        return err
    }

    c.cachedACResult = acResult
    return nil
}
```

## Advantages of Context Pattern

1. **Natural for situational data** - Positions, allies, encounter state
2. **Already familiar** - Go's context pattern is well understood
3. **Single source of truth** - Context set up once, used everywhere
4. **No topic proliferation** - Don't need query topics for everything

## Disadvantages

1. **No automatic breakdown** - Can't easily see "where did +2 come from?"
2. **Context setup required** - Someone must populate the context
3. **Missing context = silent failure** - Effects must handle nil context
4. **Coupling to context structure** - Effects depend on context shape

## When to Use Context vs Query Topics

| Need | Pattern | Why |
|------|---------|-----|
| Positional data (who's adjacent) | Context | Situational, set once per action |
| Current round/turn | Context | Combat state, not computed |
| AC with breakdown | Query | Want to see all contributors |
| Save modifiers | Query | Want to see all contributors |
| "Do I have advantage?" | Query? | Multiple sources could grant it |
| "Is target flanked?" | Context | Positional calculation |

## Open Questions

1. Should context be mutable during event processing?
2. How to handle stale context (positions changed mid-turn)?
3. Should effects be able to ADD to context? (e.g., "I grant advantage")

## Next: Hybrid Design

See 004-hybrid-design.md for combining both patterns.
