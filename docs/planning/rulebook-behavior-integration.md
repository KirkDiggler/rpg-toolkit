# Rulebook-Behavior System Integration

This document explains how the behavior infrastructure integrates with rulebook implementations, demonstrating why the toolkit's "no implementation" philosophy is a strength.

## The Toolkit Philosophy

The toolkit provides **infrastructure without opinions**. This means:
- No default monster behaviors
- No built-in AI decisions  
- No game-specific logic
- Just clean, well-defined integration points

## Why This Works

### 1. Clean Separation of Concerns

```go
// ❌ BAD: Toolkit contains game logic
type Goblin struct {
    HP int
    AC int
    // This couples D&D rules to the toolkit!
    NimbleEscape bool 
}

// ✅ GOOD: Toolkit provides infrastructure
type Entity interface {
    ID() string
    Type() string
    // Game-agnostic, just identity
}
```

### 2. Rulebooks Own Behavior

Each game system implements its own behaviors:

```go
// dnd5e/rulebook/monsters/goblin.go
package monsters

import (
    "github.com/yourgame/rpg-toolkit/behavior"
)

// GoblinBehavior implements D&D 5e goblin tactics
type GoblinBehavior struct {
    nimbleEscape bool
}

// Execute follows D&D 5e goblin behavior:
// - Use ranged attacks from hiding
// - Nimble Escape to disengage
// - Gang up on isolated targets
func (g *GoblinBehavior) Execute(ctx behavior.BehaviorContext) (behavior.Action, error) {
    // D&D 5e specific logic here
    if g.canHide(ctx) && g.hasRangedWeapon() {
        return g.hideAndShoot(ctx)
    }
    
    if g.isInMelee(ctx) && g.nimbleEscape {
        // Nimble Escape is a D&D 5e goblin feature
        return behavior.CompositeAction{
            Actions: []behavior.Action{
                behavior.DisengageAction{}, // No opportunity attacks
                behavior.MoveAction{Target: g.findHidingSpot(ctx)},
            },
        }, nil
    }
    
    return g.standardAttack(ctx)
}
```

### 3. Different Games, Different Behaviors

The same creature type can behave completely differently:

```go
// pathfinder/rulebook/monsters/goblin.go
package monsters

// PathfinderGoblinBehavior - Goblins in Pathfinder are pyromaniacal
type PathfinderGoblinBehavior struct {
    hasAlchemistFire bool
}

func (g *PathfinderGoblinBehavior) Execute(ctx behavior.BehaviorContext) (behavior.Action, error) {
    // Pathfinder goblins love fire!
    if g.hasAlchemistFire && g.seesFlammableObject(ctx) {
        return g.throwAlchemistFire(ctx)
    }
    
    // They also hate dogs and horses
    if target := g.findDogOrHorse(ctx); target != nil {
        return g.rageFueledAttack(ctx, target)
    }
    
    return g.chaoticAttack(ctx)
}
```

## Integration Patterns

### 1. Behavior Factory Pattern

```go
// MonsterBehaviorFactory creates behaviors based on game rules
type MonsterBehaviorFactory interface {
    // CreateBehavior returns appropriate behavior for monster type
    CreateBehavior(monsterType string) behavior.Behavior
}

// DnD5eBehaviorFactory implements D&D 5e monster behaviors
type DnD5eBehaviorFactory struct {
    templates map[string]BehaviorTemplate
}

func (f *DnD5eBehaviorFactory) CreateBehavior(monsterType string) behavior.Behavior {
    switch monsterType {
    case "goblin":
        return &GoblinBehavior{nimbleEscape: true}
    case "orc":
        return &AggressiveBehavior{
            targetPriority: []behavior.TargetPriority{
                behavior.TargetPriorityWeakest, // Orcs target the weak
            },
        }
    case "dragon":
        return &PhasedDragonBehavior{
            phases: f.loadDragonPhases(),
        }
    default:
        return &DefaultMonsterBehavior{}
    }
}
```

### 2. Template Registration

```go
// BehaviorRegistry allows dynamic behavior registration
type BehaviorRegistry struct {
    behaviors map[string]BehaviorConstructor
}

// Register a behavior constructor for a monster type
func (r *BehaviorRegistry) Register(monsterType string, constructor BehaviorConstructor) {
    r.behaviors[monsterType] = constructor
}

// In your game initialization
func InitializeDnD5eBehaviors(registry *BehaviorRegistry) {
    // Register all D&D 5e specific behaviors
    registry.Register("goblin", NewGoblinBehavior)
    registry.Register("hobgoblin", NewHobgoblinBehavior)
    registry.Register("bugbear", NewBugbearBehavior)
    
    // Dragons have complex phased behavior
    registry.Register("adult_red_dragon", func() behavior.Behavior {
        return &PhasedBehavior{
            phases: map[string]Phase{
                "territorial": &TerritorialDragonPhase{},
                "aggressive":  &AggressiveDragonPhase{},
                "desperate":   &DesperateDragonPhase{},
            },
        }
    })
}
```

### 3. Rulebook Integration

```go
// DnD5eRulebook implements game-specific rules including AI
type DnD5eRulebook struct {
    behaviorFactory  MonsterBehaviorFactory
    conditionRules   ConditionRuleset
    combatRules      CombatRuleset
}

// GetMonsterBehavior returns the behavior for a monster type
func (r *DnD5eRulebook) GetMonsterBehavior(monster Monster) behavior.Behavior {
    baseBehavior := r.behaviorFactory.CreateBehavior(monster.Type)
    
    // Apply conditions that affect behavior
    if monster.HasCondition(condition.Frightened) {
        return &FrightenedWrapper{base: baseBehavior}
    }
    
    if monster.HasCondition(condition.Charmed) {
        return &CharmedWrapper{
            base:    baseBehavior,
            charmer: monster.GetCharmer(),
        }
    }
    
    return baseBehavior
}
```

## Benefits of This Approach

### 1. Game Fidelity
Each game's monsters behave exactly as their rules dictate:
- D&D goblins use Nimble Escape
- Pathfinder goblins are pyromaniacs
- Your custom game's goblins can be peaceful farmers

### 2. No Cross-Contamination
```go
// Each rulebook is isolated
dndRulebook := &DnD5eRulebook{}
pathfinderRulebook := &PathfinderRulebook{}

// Same toolkit, different behaviors
dndGoblin := dndRulebook.GetMonsterBehavior(goblin)
pfGoblin := pathfinderRulebook.GetMonsterBehavior(goblin)

// Completely different AI decisions!
```

### 3. Testability
```go
func TestGoblinBehavior(t *testing.T) {
    // Test D&D 5e goblin behavior in isolation
    ctx := &MockBehaviorContext{
        Entity: &MockGoblin{hp: 5},
        Perception: PerceptionData{
            VisibleEnemies: []Entity{fighter},
        },
    }
    
    behavior := &GoblinBehavior{nimbleEscape: true}
    action := behavior.Execute(ctx)
    
    // Should use Nimble Escape when hurt
    assert.IsType(t, CompositeAction{}, action)
}
```

### 4. Evolution Without Breaking Changes

The toolkit can add new behavior paradigms without affecting existing games:

```go
// Version 1: Just state machines
type StateMachineBehavior struct { /* ... */ }

// Version 2: Add behavior trees (existing code still works!)
type BehaviorTreeNode interface { /* ... */ }

// Version 3: Add utility AI (still backward compatible!)
type UtilityAIBehavior struct { /* ... */ }
```

## Real-World Example: Combat Styles

Different games have different combat philosophies:

```go
// D&D 5e: Tactical, grid-based combat
type DnD5eCombatBehavior struct{}

func (b *DnD5eCombatBehavior) Execute(ctx BehaviorContext) (Action, error) {
    // Consider opportunity attacks
    // Use reactions strategically
    // Position for advantage
    return b.tacticalPositioning(ctx)
}

// Narrative Game: Story-driven combat
type NarrativeCombatBehavior struct{}

func (b *NarrativeCombatBehavior) Execute(ctx BehaviorContext) (Action, error) {
    // Choose dramatically appropriate actions
    // Consider story beats
    // Enable cool moments
    return b.cinematicAction(ctx)
}

// OSR Game: Lethal, unpredictable combat
type OSRCombatBehavior struct{}

func (b *OSRCombatBehavior) Execute(ctx BehaviorContext) (Action, error) {
    // Fight dirty
    // Use environment
    // Retreat is always an option
    return b.survivalInstinct(ctx)
}
```

## Documentation Requirements

Per the linter, all exported behavior types need clear documentation:

```go
// GoblinBehavior implements D&D 5e goblin combat tactics including
// hit-and-run attacks, gang-up strategies, and the signature Nimble
// Escape ability that allows disengaging without opportunity attacks.
type GoblinBehavior struct {
    nimbleEscape bool
}

// Execute returns the next action for this goblin to take based on
// D&D 5e goblin tactics: prefer ranged attacks from hiding, use
// Nimble Escape when threatened in melee, and gang up on isolated
// targets when possible.
func (g *GoblinBehavior) Execute(ctx behavior.BehaviorContext) (behavior.Action, error) {
    // Implementation
}
```

## Summary

The toolkit's "infrastructure only" approach:
1. **Enables** game-specific AI without contamination
2. **Enforces** clean separation between engine and rules
3. **Empowers** developers to implement exactly their game's vision
4. **Ensures** the toolkit remains game-agnostic and reusable

The apparent "negative" of having to implement everything is actually the **key feature** that makes the toolkit valuable across different game systems!