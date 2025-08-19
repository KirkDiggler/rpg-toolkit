# Actions and Effects Pattern Guide

## The Magic: Actions as First-Class Citizens

Actions aren't just function calls - they're entities with lifecycle, validation, and state. This pattern turns "doing things" into observable, testable, extensible infrastructure.

```go
// THE PATTERN: Actions are entities that can be activated
rage := &Rage{id: "barbarian-rage", uses: 3}
if err := rage.CanActivate(ctx, barbarian, RageInput{}); err == nil {
    rage.Activate(ctx, barbarian, RageInput{})
    // Effects propagate through event bus automatically
}
```

## Why This Pattern Exists

Traditional RPG implementations bury abilities in character methods, making them hard to:
- Test in isolation
- Extend without modifying core code  
- Track usage and resources
- Apply consistent validation
- Observe for debugging

The Action pattern solves this by making every "doable thing" a first-class entity.

## Core Concepts

### 1. Actions Are Entities

```go
type Action[T any] interface {
    Entity // Has GetID() and GetType()
    CanActivate(ctx context.Context, owner Entity, input T) error
    Activate(ctx context.Context, owner Entity, input T) error
}
```

**Key Insights:**
- Actions have identity (can be referenced, stored, tracked)
- Actions have type (feature, spell, item-use, etc.)
- Actions are generic over their input type
- Actions know their owner but aren't owned

### 2. Input Types Define Requirements

```go
// No input needed
type RageInput struct{}

// Target required
type AttackInput struct {
    Target core.Entity
}

// Complex spell requirements
type FireballInput struct {
    Target   spatial.Position
    SlotLevel int
}
```

**Pattern Benefits:**
- Compile-time validation of requirements
- Self-documenting APIs
- Type-safe action invocation

### 3. Effects Propagate Through Events

Actions don't directly modify state. They publish events that systems subscribe to:

```go
func (r *Rage) Activate(ctx context.Context, owner Entity, input RageInput) error {
    r.uses--
    
    // Publish activation event
    bus.Publish(events.New("feature.activated", map[string]any{
        "feature": "rage",
        "owner":   owner.GetID(),
    }))
    
    // Apply rage effect
    effect := &RageEffect{
        damageBonus: 2,
        resistance:  "physical",
    }
    bus.Publish(events.New("effect.applied", map[string]any{
        "effect": effect,
        "target": owner.GetID(),
        "duration": "1 minute",
    }))
    
    return nil
}
```

## Implementation Patterns

### Simple Feature: Rage

```go
type RageInput struct{} // No input needed

type Rage struct {
    id   string
    uses int
}

func (r *Rage) GetID() string { return r.id }
func (r *Rage) GetType() string { return "feature" }

func (r *Rage) CanActivate(ctx context.Context, owner Entity, input RageInput) error {
    if r.uses <= 0 {
        return errors.New("no rage uses remaining")
    }
    // Check if already raging via effect system
    if hasEffect(owner, "rage") {
        return errors.New("already raging")
    }
    return nil
}

func (r *Rage) Activate(ctx context.Context, owner Entity, input RageInput) error {
    r.uses--
    applyRageEffect(owner)
    return nil
}
```

### Complex Spell: Bless

```go
type BlessInput struct {
    Targets []core.Entity // Up to 3 creatures
}

type Bless struct {
    id        string
    spellSlot *resource.Resource
}

func (b *Bless) CanActivate(ctx context.Context, caster Entity, input BlessInput) error {
    // Validate target count
    if len(input.Targets) == 0 || len(input.Targets) > 3 {
        return fmt.Errorf("bless requires 1-3 targets, got %d", len(input.Targets))
    }
    
    // Check spell slot
    if !b.spellSlot.CanConsume(1) {
        return errors.New("no spell slots available")
    }
    
    // Check concentration
    if hasConcentration(caster) {
        return errors.New("already concentrating on a spell")
    }
    
    return nil
}

func (b *Bless) Activate(ctx context.Context, caster Entity, input BlessInput) error {
    // Consume resource
    b.spellSlot.Consume(1)
    
    // Apply bless to each target
    for _, target := range input.Targets {
        applyBlessEffect(target)
    }
    
    // Start concentration
    startConcentration(caster, "bless")
    
    return nil
}
```

### Item Activation: Healing Potion

```go
type PotionInput struct {
    Consumer core.Entity // Who drinks it (self or other)
}

type HealingPotion struct {
    id       string
    healing  string // "2d4+2"
    consumed bool
}

func (p *HealingPotion) CanActivate(ctx context.Context, owner Entity, input PotionInput) error {
    if p.consumed {
        return errors.New("potion already consumed")
    }
    
    // Check if consumer is reachable (adjacent for others, always for self)
    if input.Consumer.GetID() != owner.GetID() {
        if !isAdjacent(owner, input.Consumer) {
            return errors.New("target not within reach")
        }
    }
    
    return nil
}

func (p *HealingPotion) Activate(ctx context.Context, owner Entity, input PotionInput) error {
    p.consumed = true
    
    // Roll healing
    healing := dice.New(p.healing).Roll()
    
    // Apply healing event
    bus.Publish(events.New("healing.applied", map[string]any{
        "source": p.GetID(),
        "target": input.Consumer.GetID(),
        "amount": healing,
    }))
    
    return nil
}
```

## Effect Lifecycle

Effects created by actions follow a predictable lifecycle:

### 1. Application
```go
effect := &BlessEffect{id: uuid.New()}
bus.Publish(events.New("effect.applied", map[string]any{
    "effect": effect,
    "target": target.GetID(),
}))
```

### 2. Duration Tracking
```go
// Effects can be:
// - Instantaneous (damage, healing)
// - Duration-based ("1 minute", "10 rounds")
// - Conditional ("until next rest", "until saves")
// - Permanent (curses, some conditions)
```

### 3. Cleanup
```go
bus.Publish(events.New("effect.removed", map[string]any{
    "effect": effect.GetID(),
    "target": target.GetID(),
    "reason": "expired", // or "dispelled", "saved", etc.
}))
```

## Testing Strategies

### Unit Testing Actions

```go
func TestRageActivation(t *testing.T) {
    rage := &Rage{id: "rage-1", uses: 3}
    barbarian := &MockEntity{id: "barb-1"}
    
    // Test successful activation
    err := rage.CanActivate(ctx, barbarian, RageInput{})
    require.NoError(t, err)
    
    err = rage.Activate(ctx, barbarian, RageInput{})
    require.NoError(t, err)
    assert.Equal(t, 2, rage.uses)
    
    // Test resource depletion
    rage.uses = 0
    err = rage.CanActivate(ctx, barbarian, RageInput{})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "no rage uses")
}
```

### Integration Testing with Events

```go
func TestBlessWithEventBus(t *testing.T) {
    bus := events.NewBus()
    effectSystem := NewEffectSystem(bus)
    
    bless := &Bless{
        id: "bless-1",
        spellSlot: resource.New(resource.Config{
            MaxValue: 1,
            Current:  1,
        }),
    }
    
    targets := []core.Entity{
        &MockEntity{id: "ally-1"},
        &MockEntity{id: "ally-2"},
    }
    
    // Subscribe to effect events
    var appliedEffects []string
    bus.Subscribe("effect.applied", func(e events.Event) {
        appliedEffects = append(appliedEffects, e.Data["target"].(string))
    })
    
    // Activate bless
    err := bless.Activate(ctx, caster, BlessInput{Targets: targets})
    require.NoError(t, err)
    
    // Verify effects were applied
    assert.Len(t, appliedEffects, 2)
    assert.Contains(t, appliedEffects, "ally-1")
    assert.Contains(t, appliedEffects, "ally-2")
}
```

## Common Patterns

### Resource Management
```go
type ResourceAction struct {
    resource *resource.Resource
}

func (r *ResourceAction) CanActivate(...) error {
    if !r.resource.CanConsume(1) {
        return errors.New("insufficient resources")
    }
    return nil
}
```

### Cooldown Tracking
```go
type CooldownAction struct {
    lastUsed time.Time
    cooldown time.Duration
}

func (c *CooldownAction) CanActivate(...) error {
    if time.Since(c.lastUsed) < c.cooldown {
        remaining := c.cooldown - time.Since(c.lastUsed)
        return fmt.Errorf("on cooldown for %v", remaining)
    }
    return nil
}
```

### Conditional Activation
```go
func (a *SneakAttack) CanActivate(ctx context.Context, rogue Entity, input AttackInput) error {
    // Must have advantage OR ally adjacent to target
    if !hasAdvantage(ctx) && !hasAllyAdjacent(input.Target) {
        return errors.New("sneak attack conditions not met")
    }
    return nil
}
```

## Migration Guide

### Converting Method-Based Abilities

**Before:**
```go
func (c *Character) Rage() error {
    if c.rageUses <= 0 {
        return errors.New("no uses")
    }
    c.rageUses--
    c.isRaging = true
    c.damageBonus += 2
    return nil
}
```

**After:**
```go
// Rage is now a separate entity
rage := &Rage{id: "rage-feature", uses: 3}

// Character stores action reference
character.AddAction(rage)

// Usage is consistent with all actions
rage.Activate(ctx, character, RageInput{})
```

### Adding New Action Types

1. Define input type
2. Implement Action interface
3. Add resource/cooldown management
4. Publish appropriate events
5. Write tests

## Best Practices

### DO:
- ✅ Make actions self-contained entities
- ✅ Use typed inputs for requirements
- ✅ Validate thoroughly in CanActivate
- ✅ Publish events for observability
- ✅ Track resources/uses in the action
- ✅ Test actions in isolation

### DON'T:
- ❌ Embed actions in character structs
- ❌ Modify owner state directly
- ❌ Skip validation in CanActivate
- ❌ Forget cleanup for duration effects
- ❌ Mix game rules with infrastructure

## Common Pitfalls

### 1. Forgetting Effect Cleanup
```go
// BAD: Effect applied but never removed
func (s *Spell) Activate(...) {
    applyEffect(target, effect)
    // What happens when spell ends?
}

// GOOD: Track duration and cleanup
func (s *Spell) Activate(...) {
    applyEffect(target, effect)
    scheduleRemoval(effect, "1 minute")
}
```

### 2. Direct State Modification
```go
// BAD: Directly modifying owner
func (r *Rage) Activate(ctx context.Context, owner Entity, ...) {
    owner.(*Character).damageBonus += 2 // Type assertion and mutation!
}

// GOOD: Use events
func (r *Rage) Activate(ctx context.Context, owner Entity, ...) {
    bus.Publish(events.New("rage.activated", map[string]any{
        "owner": owner.GetID(),
        "bonus": 2,
    }))
}
```

### 3. Validation After Activation
```go
// BAD: Check after consuming resources
func (s *Spell) Activate(...) {
    s.slot.Consume(1)
    if len(input.Targets) > 3 {
        return errors.New("too many targets") // Slot already consumed!
    }
}

// GOOD: Validate first
func (s *Spell) CanActivate(...) {
    if len(input.Targets) > 3 {
        return errors.New("too many targets")
    }
    return s.slot.CanConsume(1)
}
```

## Advanced Topics

### Composite Actions
Actions that trigger other actions:

```go
type ActionSurge struct {
    extraAction Action[AttackInput]
}

func (a *ActionSurge) Activate(ctx context.Context, owner Entity, input ActionSurgeInput) error {
    // Grant additional action
    return a.extraAction.Activate(ctx, owner, input.AttackInput)
}
```

### Reaction Actions
Actions triggered by events:

```go
type Shield struct{}

func (s *Shield) OnAttackEvent(e events.Event) {
    if canReact(e.Target) {
        s.Activate(ctx, e.Target, ShieldInput{Attacker: e.Attacker})
    }
}
```

### Persistent Effects
Effects that modify future actions:

```go
type Bless struct{}

func (b *Bless) ModifyRoll(roll *dice.Roll) {
    roll.AddDice("1d4") // Bless adds 1d4 to attacks/saves
}
```

## Summary

The Actions and Effects pattern transforms RPG abilities from embedded methods into first-class entities. This provides:

1. **Testability**: Actions can be tested in isolation
2. **Extensibility**: New actions don't modify core code
3. **Observability**: All actions flow through events
4. **Consistency**: Common patterns for all "doable things"
5. **Type Safety**: Generic inputs provide compile-time validation

Remember: Actions are infrastructure. The toolkit provides the pattern, rulebooks provide the implementations.