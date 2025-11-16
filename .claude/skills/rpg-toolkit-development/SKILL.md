---
name: rpg-toolkit-development
description: Use this skill when working on rpg-toolkit codebase - provides architecture patterns, testing guidelines, error handling, and D&D 5e combat mechanics knowledge
---

# RPG Toolkit Development Skill

Use this skill when working on the rpg-toolkit project to ensure consistency with established patterns and avoid common mistakes.

## When to Use This Skill

- Working on rpg-toolkit codebase
- Implementing D&D 5e combat mechanics
- Creating event-driven features
- Writing tests for toolkit code
- Handling errors and validation
- Working with modifier chains

## Core Principles

### 1. Mock Behavior, Not Data

**❌ WRONG:**
```go
mockEntity := mock_core.NewMockEntity(ctrl)
mockEntity.EXPECT().GetID().Return("test-1").AnyTimes()
```

**✅ CORRECT:**
```go
// Entities are just data - use real implementations
entity := monster.New(monster.Config{
    ID: "test-1",
    Name: "Test Monster",
    HP: 50,
    AC: 15,
    AbilityScores: scores,
})

// Only mock interfaces with complex behavior
mockRoller := mock_dice.NewMockRoller(ctrl)
mockRoller.EXPECT().Roll(ctx, 20).Return(15, nil)
```

**Why:** Data objects are simple to construct. Mocking adds complexity without benefit. Mock interfaces that encapsulate behavior (Roller, EventBus).

### 2. Typed Constants Pattern

**Low-level packages define types:**
```go
// core/chain/types.go
type Stage string
```

**Rulebook packages implement constants:**
```go
// rulebooks/dnd5e/stages.go
const (
    StageBase       chain.Stage = "base"
    StageFeatures   chain.Stage = "features"
    StageConditions chain.Stage = "conditions"
    StageEquipment  chain.Stage = "equipment"
    StageFinal      chain.Stage = "final"
)

var ModifierStages = []chain.Stage{
    StageBase,
    StageFeatures,
    StageConditions,
    StageEquipment,
    StageFinal,
}
```

**Why:** No magic strings. Type safety. Game servers get well-defined constants. Clear ordering.

### 3. Error Handling with rpgerr

**Always use rpgerr package:**
```go
// Validation errors
if input == nil {
    return rpgerr.New(rpgerr.CodeInvalidArgument, "input is nil")
}

// Wrapping errors
result, err := doSomething()
if err != nil {
    return rpgerr.Wrap(err, "failed to do something")
}
```

**Add Validate() methods:**
```go
func (ai *AttackInput) Validate() error {
    if ai == nil {
        return rpgerr.New(rpgerr.CodeInvalidArgument, "AttackInput is nil")
    }
    if ai.Attacker == nil {
        return rpgerr.New(rpgerr.CodeInvalidArgument, "Attacker is nil")
    }
    return nil
}
```

### 4. Event-Driven Architecture

**Event Flow Pattern:**
```
1. Publish notification event (AttackEvent)
   → Lets systems track "attack is happening"

2. Use ChainedTopic for modifiers (AttackChain, DamageChain)
   → Conditions/features subscribe to add bonuses

3. Publish result event (DamageReceivedEvent)
   → Lets systems react to outcome
```

**Example:**
```go
// Step 1: Notify attack is happening
attackTopic.Publish(ctx, AttackEvent{...})

// Step 2: Collect modifiers through chain
chain := events.NewStagedChain[DamageChainEvent](dnd5e.ModifierStages)
damages := DamageChain.On(eventBus)
modifiedChain, _ := damages.PublishWithChain(ctx, event, chain)
final, _ := modifiedChain.Execute(ctx, event)

// Step 3: Notify damage was dealt
damageTopic.Publish(ctx, DamageReceivedEvent{...})
```

### 5. Modifier Chain Usage

**When to subscribe to chains:**
```go
// In condition Apply() method
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
    // Subscribe to DamageChain
    damages := combat.DamageChain.On(bus)
    _, err := damages.SubscribeWithChain(ctx, func(ctx context.Context, e combat.DamageChainEvent, c chain.Chain[combat.DamageChainEvent]) (chain.Chain[combat.DamageChainEvent], error) {
        if e.AttackerID == r.CharacterID {
            // Add modifier in appropriate stage
            c.Add(dnd5e.StageFeatures, "rage", func(_ context.Context, ev combat.DamageChainEvent) (combat.DamageChainEvent, error) {
                ev.DamageBonus += r.DamageBonus
                return ev, nil
            })
        }
        return c, nil
    })
    return err
}
```

**Stage ordering matters:**
- `StageBase` - Base values (rolls, ability mods)
- `StageFeatures` - Class features (rage, sneak attack)
- `StageConditions` - Status effects (bless, bane, prone)
- `StageEquipment` - Item bonuses (magic weapons)
- `StageFinal` - Final adjustments (resistance, caps)

### 6. D&D 5e Combat Mechanics

**Attack Resolution Order:**
1. Publish `AttackEvent` (before any rolls)
2. Roll d20 for attack
3. Fire `AttackChain` for attack roll modifiers
4. Compare vs AC (nat 1 always misses, nat 20 always hits)
5. If hit: Roll damage dice
6. Fire `DamageChain` for damage modifiers
7. Publish `DamageReceivedEvent`

**Critical Hits:**
- Double damage **dice**, not bonuses
- Roll the dice pool twice and combine
- Rage bonus (+2/+3/+4) is NOT doubled

**Dice Integration:**
```go
// Parse weapon damage
pool, err := dice.ParseNotation("1d8")

// Roll with provided roller
result := pool.RollContext(ctx, roller)
total := result.Total()
rolls := result.Rolls() // [][]int - flattened for display
```

### 7. Testing Patterns

**Use testify suite pattern:**
```go
type MyTestSuite struct {
    suite.Suite
    ctrl *gomock.Controller
    ctx  context.Context
}

func (s *MyTestSuite) SetupTest() {
    s.ctrl = gomock.NewController(s.T())
    s.ctx = context.Background()
}

func (s *MyTestSuite) TearDownTest() {
    s.ctrl.Finish()
}
```

**Test organization:**
- Unit tests in same package
- Use real entities (Monster, Character)
- Mock only behavior (Roller, EventBus if needed)
- Test event publishing with subscribers

### 8. Game Server Architecture

**Game server is data-driven and generic:**
```go
// Game server loads features from DB
for _, featureData := range characterData.Features {
    feature := features.LoadJSON(featureData) // Returns Action interface
    err := feature.Activate(ctx, character, &features.FeatureInput{
        Bus: eventBus,
    })
}
```

**Game server doesn't know specifics:**
- Doesn't know what "rage" is
- Doesn't know what DamageChain is
- Just loads data and calls Action.Activate()
- Events handle the magic

**Rulebook packages contain intelligence:**
- Rage.Activate() publishes ConditionAppliedEvent
- RagingCondition.Apply() subscribes to DamageChain
- Everything is wired through events

## Common Mistakes to Avoid

1. ❌ Creating local mock implementations instead of using gomock
2. ❌ Mocking data objects (Entity) instead of behavior (Roller)
3. ❌ Using magic strings instead of typed constants
4. ❌ Manual dice parsing instead of dice.ParseNotation()
5. ❌ Using fmt.Errorf instead of rpgerr
6. ❌ Forgetting to publish notification events (AttackEvent, DamageReceivedEvent)
7. ❌ Not validating inputs
8. ❌ Doubling damage bonuses on crits (only dice are doubled)

## Module Structure

```
rpg-toolkit/
├── core/           - Base types (Entity, Action, chain.Stage)
├── events/         - Event bus, ChainedTopic, StagedChain
├── dice/           - Dice rolling (Pool, ParseNotation)
├── rulebooks/
│   └── dnd5e/
│       ├── stages.go      - Modifier stage constants
│       ├── events.go      - Event definitions
│       ├── monster/       - Simple entities
│       ├── combat/        - Attack resolution
│       ├── features/      - Class features (Rage)
│       └── conditions/    - Conditions (RagingCondition)
```

## Quick Reference

**Create a new condition that modifies combat:**
1. Implement ConditionBehavior interface
2. In Apply(), subscribe to combat chains (AttackChain, DamageChain)
3. In chain handler, add modifiers using c.Add(stage, id, modifier)
4. In Remove(), unsubscribe from events

**Add a new modifier stage:**
1. Add constant to dnd5e/stages.go
2. Add to ModifierStages slice in correct order
3. Document when it runs

**Test combat mechanics:**
1. Use real Monster entities
2. Mock dice.Roller with gomock
3. Subscribe to events to verify they're published
4. Check AttackResult for correct values
