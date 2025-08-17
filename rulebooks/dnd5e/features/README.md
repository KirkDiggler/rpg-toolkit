# D&D 5e Features Package

This package implements D&D 5e character features (rage, second wind, action surge, etc.) as self-contained Actions with event-driven effects.

## Core Architecture

Features are self-contained entities that:
- Implement `core.Action[FeatureInput]` for activation
- Manage their own event subscriptions
- Apply effects through the event system
- Track their own state and resources

## The LoadJSON Pattern

Features are loaded from JSON configuration, allowing dynamic feature creation without hardcoding:

```go
featureJSON := `{
    "ref": "dnd5e:features:rage",
    "id": "barbarian-rage",
    "data": {
        "uses": 3,
        "level": 5
    }
}`

// Event bus is passed for dependency injection
rage, err := features.LoadJSON([]byte(featureJSON), eventBus)
```

### Why Pass the Event Bus?

Features need the event bus to:
- Subscribe to combat events (attacks, damage, etc.)
- Publish feature events (rage started/ended)
- Apply modifiers through the event system
- Clean up subscriptions when done

The bus is passed at creation time so features can manage their own subscriptions.

## Feature Interface

```go
// Feature is the D&D 5e specific interface for character features
type Feature interface {
    core.Action[FeatureInput]
    
    // D&D 5e specific methods
    GetResourceType() ResourceType  // rage uses, ki points, etc.
    ResetsOn() ResetType            // short rest, long rest, dawn
}
```

## Example: Rage Implementation

Rage is our first complete feature implementation showing the pattern:

### 1. Feature Structure

```go
type Rage struct {
    id    string
    uses  int
    level int
    bus   *events.Bus
    
    // Thread-safe state management
    mu sync.RWMutex
    
    // Current state (protected by mutex)
    currentUses   int
    active        bool
    owner         core.Entity
    subscriptions []string  // For cleanup
}
```

### 2. Activation Flow

When rage is activated:

```go
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // 1. Check if can activate
    if err := r.CanActivate(ctx, owner, input); err != nil {
        return err
    }
    
    // 2. Update state
    r.currentUses--
    r.active = true
    r.owner = owner
    
    // 3. Subscribe to relevant events
    attackSub, _ := r.bus.SubscribeWithFilter(
        dnd5e.EventRefAttack,
        r.onAttack,
        func(e events.Event) bool {
            // Only handle attacks from the raging barbarian
            attack := e.(*dnd5e.AttackEvent)
            return attack.Attacker == r.owner
        },
    )
    
    damageSub, _ := r.bus.SubscribeWithFilter(
        dnd5e.EventRefDamageReceived,
        r.onDamageReceived,
        func(e events.Event) bool {
            // Only handle damage to the raging barbarian
            damage := e.(*dnd5e.DamageReceivedEvent)
            return damage.Target == r.owner
        },
    )
    
    // 4. Track subscriptions for cleanup
    r.subscriptions = []string{attackSub, damageSub}
    
    // 5. Publish rage started event
    r.bus.Publish(&dnd5e.RageStartedEvent{
        Owner:       owner,
        DamageBonus: r.getDamageBonus(),
    })
    
    return nil
}
```

### 3. Event Handlers

Features modify combat through event handlers:

```go
func (r *Rage) onAttack(e interface{}) error {
    attack := e.(*dnd5e.AttackEvent)
    
    // Add damage bonus to STR-based melee attacks
    if attack.IsMelee && attack.Ability == dnd5e.AbilityStrength {
        ctx := attack.Context()
        ctx.AddModifier(events.NewSimpleModifier(
            dnd5e.ModifierSourceRage,
            dnd5e.ModifierTypeAdditive,
            dnd5e.ModifierTargetDamage,
            200,  // priority
            r.getDamageBonus(),
        ))
    }
    
    return nil
}

func (r *Rage) onDamageReceived(e interface{}) error {
    damage := e.(*dnd5e.DamageReceivedEvent)
    
    // Apply resistance to physical damage
    if damage.DamageType == dnd5e.DamageTypeBludgeoning ||
       damage.DamageType == dnd5e.DamageTypePiercing ||
       damage.DamageType == dnd5e.DamageTypeSlashing {
        
        ctx := damage.Context()
        ctx.AddModifier(events.NewSimpleModifier(
            dnd5e.ModifierSourceRage,
            dnd5e.ModifierTypeResistance,
            dnd5e.ModifierTargetDamage,
            100,  // priority (apply early)
            0.5,  // multiplier for half damage
        ))
    }
    
    return nil
}
```

## Type Safety Through Constants

All strings are typed constants for compile-time safety:

```go
// Feature keys
const FeatureKeyRage FeatureKey = "rage"

// Resource types
const ResourceTypeRageUses ResourceType = "rage_uses"

// Reset types
const ResetTypeLongRest ResetType = "long_rest"

// Modifier sources (in dnd5e package)
const ModifierSourceRage events.ModifierSource = "rage"

// Modifier types (in dnd5e package)
const (
    ModifierTypeAdditive   events.ModifierType = "additive"
    ModifierTypeResistance events.ModifierType = "resistance"
)

// Modifier targets (in dnd5e package)
const ModifierTargetDamage events.ModifierTarget = "damage"

// Damage types
const (
    DamageTypeBludgeoning damage.Type = "bludgeoning"
    DamageTypePiercing    damage.Type = "piercing"
    DamageTypeSlashing    damage.Type = "slashing"
)
```

## Thread Safety

Features are designed to be thread-safe for future async event buses:

```go
type Rage struct {
    mu sync.RWMutex  // Protects all state below
    
    currentUses   int
    active        bool
    owner         core.Entity
    subscriptions []string
}

// All state access is protected
func (r *Rage) CanActivate(...) error {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    if r.currentUses <= 0 {
        return errors.New("no rage uses remaining")
    }
    // ...
}
```

## Testing

### Unit Tests

```go
func TestRage_AddsSTRDamageBonus(t *testing.T) {
    bus := events.NewBus()
    
    // Load rage from JSON
    featureJSON := `{
        "ref": "dnd5e:features:rage",
        "id": "test-rage",
        "data": {"uses": 3, "level": 5}
    }`
    
    rage, err := features.LoadJSON([]byte(featureJSON), bus)
    require.NoError(t, err)
    
    // Create barbarian
    barbarian := &mockEntity{id: "conan", entityType: dnd5e.EntityTypeCharacter}
    
    // Activate rage
    err = rage.Activate(context.Background(), barbarian, features.FeatureInput{})
    require.NoError(t, err)
    
    // Create STR melee attack event
    attackEvent := dnd5e.NewAttackEvent(
        barbarian,
        &mockEntity{id: "goblin", entityType: dnd5e.EntityTypeMonster},
        true,  // isMelee
        dnd5e.AbilityStrength,
        8,  // base damage
    )
    
    // Publish and check modifier was added
    err = bus.Publish(attackEvent)
    require.NoError(t, err)
    
    // Verify damage bonus was applied
    modifiers := attackEvent.Context().GetModifiers()
    require.Len(t, modifiers, 1)
    
    mod := modifiers[0].(*events.SimpleModifier)
    assert.Equal(t, dnd5e.ModifierSourceRage, mod.Source)
    assert.Equal(t, 3, mod.Value)  // Level 5 = +3 damage
}
```

### Concurrent Stress Testing

```go
func TestRage_ConcurrentEventHandling(t *testing.T) {
    bus := events.NewBus()
    rage, _ := features.LoadJSON([]byte(featureJSON), bus)
    rage.Activate(ctx, barbarian, features.FeatureInput{})
    
    var wg sync.WaitGroup
    numGoroutines := 100
    numEventsPerGoroutine := 10
    
    // Concurrently publish attack events
    wg.Add(numGoroutines)
    for i := 0; i < numGoroutines; i++ {
        go func() {
            defer wg.Done()
            for j := 0; j < numEventsPerGoroutine; j++ {
                attackEvent := dnd5e.NewAttackEvent(...)
                bus.Publish(attackEvent)
            }
        }()
    }
    
    // Wait for completion - no deadlocks or races!
    wg.Wait()
}
```

Run tests with race detection:
```bash
go test -race ./features
```

## Adding New Features

To add a new feature (e.g., Second Wind):

1. **Define the feature struct**:
```go
type SecondWind struct {
    id          string
    uses        int
    fighterLevel int
    bus         *events.Bus
    mu          sync.RWMutex
}
```

2. **Implement core.Action[FeatureInput]**:
```go
func (sw *SecondWind) CanActivate(...) error
func (sw *SecondWind) Activate(...) error
```

3. **Implement Feature interface**:
```go
func (sw *SecondWind) GetResourceType() ResourceType
func (sw *SecondWind) ResetsOn() ResetType
```

4. **Add to LoadJSON**:
```go
case FeatureKeySecondWind:
    var data struct {
        Uses  int `json:"uses"`
        Level int `json:"level"`
    }
    json.Unmarshal(input.Data, &data)
    return &SecondWind{...}, nil
```

5. **Write tests** covering:
   - Activation conditions
   - Effect application
   - Resource consumption
   - Thread safety

## Design Principles

1. **Self-Contained Features**: Each feature manages its own subscriptions and state
2. **Event-Driven Effects**: Features modify combat through events, not direct state changes
3. **Type Safety**: Constants everywhere, never raw strings
4. **Thread Safety**: Mutex protection for future async event buses
5. **Clean Separation**: Features don't know about each other or the game server
6. **Testability**: Easy to test in isolation with mock event buses

## Current Implementation Status

✅ **Completed**:
- LoadJSON pattern for dynamic feature loading
- Rage feature with full event integration
- Thread-safe state management
- Comprehensive test coverage
- Type-safe modifiers using core/events types

🚧 **Next Steps**:
- Add turn/round tracking for rage duration
- Implement Second Wind (healing feature)
- Implement Action Surge (extra action)
- Add more barbarian features (Reckless Attack, Danger Sense)
- Support for feature prerequisites and validation