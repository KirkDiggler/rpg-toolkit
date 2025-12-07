# Proto-Informed Event Bus Design for RPG Toolkit

## Learning from Your Work Event Service

Your work event service has some brilliant patterns:

1. **ProtoTypeName as Topic** - Derived from proto package, giving automatic namespacing
2. **EventEnvelope Pattern** - Header (metadata) + Payload (marshaled proto)
3. **Type-Safe Registration** - `map[ProtoName]TaskFunc` ensures handlers exist
4. **Delivery Metadata** - DeliveryReason, OriginApp, Environment in header
5. **Unmarshal at Handler** - Payload stays as `[]byte` until handler needs it

## What I Think Works Really Well

### 1. **The Envelope Pattern is Perfect for Games**
```go
type EventEnvelope struct {
    Header  *EventHeader  // Metadata about delivery
    Payload []byte        // The actual event data
}
```

For games, the header could carry:
- Source entity (who triggered it)
- Target entities (who's affected)
- Location/room context
- Priority/ordering info
- Event correlation ID (for chains of events)

### 2. **ProtoTypeName as Natural Topic**
```go
// Proto: platform.combat.v1.DamageDealt
// Topic: "platform.combat.v1.DamageDealt"
```

This is brilliant because:
- No manual topic management
- Type and topic are inherently linked
- Versioning built in (v1, v2)
- Natural namespacing

### 3. **The Unmarshal-at-Handler Pattern**
Keeping payload as `[]byte` until needed means:
- Can record/replay without knowing types
- Can forward events without unmarshaling
- Can filter/route without full deserialization
- Perfect for pipelines that might transform events

## How This Could Work for RPG Toolkit

### Option 1: Pure Proto Events (Like Your Work)

```go
// Define events in proto
// rpg/combat/v1/events.proto
message DamageDealt {
    string source_id = 1;
    string target_id = 2;
    int32 amount = 3;
    DamageType damage_type = 4;
}

// Auto-generated topic from proto package
// Topic: "rpg.combat.v1.DamageDealt"

// Type-safe subscription
type DamageHandler func(ctx context.Context, e *DamageDealt) error

// Register with derived topic
bus.Subscribe("rpg.combat.v1.DamageDealt", func(ctx context.Context, env *EventEnvelope) error {
    var damage DamageDealt
    if err := proto.Unmarshal(env.Payload, &damage); err != nil {
        return err
    }
    // Handle damage with full type safety
    return handler(ctx, &damage)
})
```

### Option 2: Hybrid - Proto for Wire, Native for Local

```go
// Native Go event (what handlers use)
type DamageEvent struct {
    Source     EntityID
    Target     EntityID
    Amount     int
    DamageType string
}

// Proto for serialization (auto-generated)
type DamageDealtProto = combatv1.DamageDealt

// Conversion layer (could be generated)
func (e *DamageEvent) ToProto() *DamageDealtProto {
    return &DamageDealtProto{
        SourceId:   string(e.Source),
        TargetId:   string(e.Target),
        Amount:     int32(e.Amount),
        DamageType: e.DamageType,
    }
}

// Bus handles conversion
bus.Subscribe(DamageEvent{}, handler) // Native API
// Internally converts to/from proto for transport
```

### Option 3: EventContext as Game Context

Your EventContext idea holding "the things" is perfect:

```go
type GameEventContext struct {
    // Standard context
    context.Context
    
    // Event metadata (like your Header)
    EventID       string
    EventTime     time.Time
    SourceEntity  EntityID
    TargetEntities []EntityID
    
    // Game-specific context
    Room          *RoomRef
    Combat        *CombatRef
    
    // Modifiers (our existing pattern)
    Modifiers     []Modifier
    
    // Pipeline context (for transformations)
    Pipeline      map[string]any
}

// Events get context
type DamageEvent struct {
    ctx    *GameEventContext
    Amount int
    Type   DamageType
}

func (e *DamageEvent) Context() *GameEventContext {
    return e.ctx
}
```

## What I Love About This Direction

1. **Proto gives us SO much for free**:
   - Serialization/deserialization
   - Cross-language support (if needed)
   - Schema evolution
   - Auto-generated types
   - Natural topic naming

2. **The Envelope pattern solves multiple problems**:
   - Routing without full deserialization
   - Metadata without polluting event types
   - Recording/replay capability
   - Network transport ready

3. **Type safety where it matters**:
   - Handlers get typed events
   - Compile-time checking
   - But flexible routing via topics

## My Recommendation: Proto-First Hybrid

```go
// 1. Define events in proto (source of truth)
message DamageDealt {
    string source_id = 1;
    string target_id = 2;
    int32 amount = 3;
}

// 2. Generate Go types AND topic names
//go:generate protoc --go-events_out=. events.proto
// Generates:
// - Go structs
// - Topic constants
// - Conversion functions

// 3. Type-safe subscription with auto-unmarshal
bus.On(&DamageDealt{}, func(ctx *GameEventContext, e *DamageDealt) error {
    // e is already unmarshaled and typed
    return nil
})

// 4. Publish with auto-marshal
damage := &DamageDealt{
    SourceId: "player1",
    TargetId: "goblin",
    Amount: 10,
}
bus.Publish(ctx, damage) // Auto-marshals to envelope

// 5. The bus internally uses topics
// Topic: "rpg.combat.v1.DamageDealt" (from proto package)
```

## Benefits for Game Toolkit

1. **Single source of truth** - Proto defines the contract
2. **Type safety** - Generated types, no manual marshaling
3. **Network ready** - Can send events between services/clients
4. **Recording/replay** - Bytes can be stored/replayed
5. **Evolution** - Proto handles schema changes
6. **Cross-language** - Could have JS/Python/Rust clients

## The Pipeline Integration

With proto events, pipelines become transformers:

```go
// Pipeline receives proto bytes, transforms, outputs proto bytes
type Pipeline interface {
    Process(ctx *GameEventContext, input []byte) ([]byte, error)
}

// Example: Damage reduction pipeline
damagePipeline := NewPipeline(
    ValidateDamage(),      // Check damage is valid
    ApplyResistance(),     // Reduce by resistance
    ApplyShields(),        // Absorb with shields
    RecordDamage(),        // Track for stats
)

// Bus routes damage events through pipeline
bus.RouteThrough(&DamageDealt{}, damagePipeline)
```

## Questions This Raises

1. **Should every event be proto?** Or just network/storage events?
2. **How much goes in context vs event?** Your header pattern suggests context for metadata
3. **Generated code or runtime reflection?** Code gen is faster but needs tooling

## My Take

Your work pattern is battle-tested and solid. The combination of:
- Proto for schema/serialization
- Topics derived from proto package
- Envelope pattern for metadata
- Type-safe handlers via registration

...gives us everything we need for a game event system that can handle local processing, network transport, and persistence. The EventContext holding "the things" is exactly right - it's the game state context that flows through event processing.

I think we should steal your work pattern wholesale! 😄