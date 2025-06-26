# Go Persistence Patterns for RPG Toolkit

## Overview

This document outlines persistence strategies for the RPG Toolkit in Go, comparing Repository Injection and Middleware patterns. Both approaches leverage Go's interfaces and composition for clean, testable code.

## Pattern 1: Repository Injection

### Implementation
```go
// Repository interface
type RoomRepository interface {
    Save(ctx context.Context, room *Room) error
    FindByID(ctx context.Context, id string) (*Room, error)
    FindByTag(ctx context.Context, tag string) ([]*Room, error)
    Delete(ctx context.Context, id string) error
}

// Room generator with optional repository
type RoomGenerator struct {
    config   GeneratorConfig
    roomRepo RoomRepository // Can be nil
}

// Constructor with optional repository
func NewRoomGenerator(config GeneratorConfig, repo RoomRepository) *RoomGenerator {
    return &RoomGenerator{
        config:   config,
        roomRepo: repo,
    }
}

func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    room := g.createRoom(options)
    
    // Only persist if repository is provided
    if g.roomRepo != nil {
        if err := g.roomRepo.Save(ctx, room); err != nil {
            return nil, fmt.Errorf("failed to save room: %w", err)
        }
    }
    
    return room, nil
}

// Usage without persistence
generator := NewRoomGenerator(config, nil)
room, err := generator.GenerateRoom(ctx, options)

// Usage with MongoDB
mongoRepo := mongo.NewRoomRepository(db)
generator := NewRoomGenerator(config, mongoRepo)
room, err := generator.GenerateRoom(ctx, options)
```

### Pros
- **Simple**: Just pass nil if no persistence needed
- **Testable**: Easy to mock interfaces
- **Explicit**: Clear when persistence happens
- **Go idiomatic**: Uses standard interface patterns

### Cons
- **Manual wiring**: Must inject each repository
- **Nil checks**: Need to check if repo exists

## Pattern 2: Middleware/Hook System

### Implementation
```go
// Event system
type EventType string

const (
    EventRoomGenerated EventType = "room.generated"
    EventRoomUpdated   EventType = "room.updated"
)

type Event struct {
    Type EventType
    Data interface{}
    Time time.Time
}

type EventHandler func(ctx context.Context, event Event) error

// Middleware interface
type Middleware interface {
    Handle(next EventHandler) EventHandler
}

// Persistence middleware
type PersistenceMiddleware struct {
    adapter StorageAdapter
    config  PersistenceConfig
}

type PersistenceConfig struct {
    Collections map[string]CollectionConfig
}

type CollectionConfig struct {
    Events   []EventType
    Debounce time.Duration
}

func (p *PersistenceMiddleware) Handle(next EventHandler) EventHandler {
    return func(ctx context.Context, event Event) error {
        // Check if we should persist this event
        for collection, config := range p.config.Collections {
            for _, eventType := range config.Events {
                if event.Type == eventType {
                    // Debounced save
                    p.scheduleSave(ctx, collection, event.Data, config.Debounce)
                }
            }
        }
        return next(ctx, event)
    }
}

// Room generator with events
type RoomGenerator struct {
    config  GeneratorConfig
    emitter EventEmitter
}

func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    room := g.createRoom(options)
    
    // Emit event for middleware
    g.emitter.Emit(ctx, Event{
        Type: EventRoomGenerated,
        Data: room,
        Time: time.Now(),
    })
    
    return room, nil
}
```

## Pattern 3: Hybrid Approach (Recommended)

### Combine both patterns for flexibility
```go
// Base repository interface
type Repository[T any] interface {
    Save(ctx context.Context, entity T) error
    FindByID(ctx context.Context, id string) (*T, error)
    Delete(ctx context.Context, id string) error
}

// Room-specific repository
type RoomRepository interface {
    Repository[Room]
    FindByType(ctx context.Context, roomType RoomType) ([]*Room, error)
    FindConnected(ctx context.Context, roomID string) ([]*Room, error)
}

// Room generator with both patterns
type RoomGenerator struct {
    config   GeneratorConfig
    roomRepo RoomRepository // Optional
    emitter  EventEmitter   // Optional
}

// Functional options pattern for flexible construction
type GeneratorOption func(*RoomGenerator)

func WithRepository(repo RoomRepository) GeneratorOption {
    return func(g *RoomGenerator) {
        g.roomRepo = repo
    }
}

func WithEventEmitter(emitter EventEmitter) GeneratorOption {
    return func(g *RoomGenerator) {
        g.emitter = emitter
    }
}

func NewRoomGenerator(config GeneratorConfig, opts ...GeneratorOption) *RoomGenerator {
    g := &RoomGenerator{config: config}
    for _, opt := range opts {
        opt(g)
    }
    return g
}

func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    room := g.createRoom(options)
    
    // Direct persistence if repository available
    if g.roomRepo != nil {
        if err := g.roomRepo.Save(ctx, room); err != nil {
            return nil, fmt.Errorf("save room: %w", err)
        }
    }
    
    // Emit event if emitter available
    if g.emitter != nil {
        g.emitter.Emit(ctx, Event{
            Type: EventRoomGenerated,
            Data: room,
        })
    }
    
    return room, nil
}

// Complex queries through repository
func (g *RoomGenerator) FindSimilarRooms(ctx context.Context, room *Room) ([]*Room, error) {
    if g.roomRepo == nil {
        return nil, errors.New("repository required for queries")
    }
    return g.roomRepo.FindByType(ctx, room.Type)
}
```

## Storage Adapter Pattern

```go
// Generic storage adapter interface
type StorageAdapter interface {
    Save(ctx context.Context, collection string, id string, data interface{}) error
    Load(ctx context.Context, collection string, id string, dest interface{}) error
    Delete(ctx context.Context, collection string, id string) error
    Query(ctx context.Context, collection string, filter interface{}, results interface{}) error
}

// MongoDB implementation
type MongoAdapter struct {
    db *mongo.Database
}

func (m *MongoAdapter) Save(ctx context.Context, collection string, id string, data interface{}) error {
    coll := m.db.Collection(collection)
    filter := bson.M{"_id": id}
    update := bson.M{"$set": data}
    opts := options.Update().SetUpsert(true)
    
    _, err := coll.UpdateOne(ctx, filter, update, opts)
    return err
}

// File system implementation
type FileAdapter struct {
    basePath string
}

func (f *FileAdapter) Save(ctx context.Context, collection string, id string, data interface{}) error {
    path := filepath.Join(f.basePath, collection, id+".json")
    
    bytes, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }
    
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    
    return os.WriteFile(path, bytes, 0644)
}

// Generic repository implementation using adapter
type GenericRepository[T any] struct {
    adapter    StorageAdapter
    collection string
}

func (r *GenericRepository[T]) Save(ctx context.Context, entity T) error {
    // Assume entities have GetID() method
    type identifiable interface {
        GetID() string
    }
    
    if id, ok := any(entity).(identifiable); ok {
        return r.adapter.Save(ctx, r.collection, id.GetID(), entity)
    }
    return errors.New("entity must implement GetID()")
}
```

## Practical Examples

### Example 1: Simple Usage
```go
// Just generation, no persistence
generator := NewRoomGenerator(config)
room, _ := generator.GenerateRoom(ctx, options)

// With repository
repo := mongo.NewRoomRepository(db)
generator := NewRoomGenerator(config, WithRepository(repo))
room, _ := generator.GenerateRoom(ctx, options)
```

### Example 2: Discord Bot
```go
// Setup storage
adapter := mongo.NewAdapter(mongoClient.Database("rpg-bot"))
repo := repositories.NewRoomRepository(adapter)

// Setup middleware
persistence := middleware.NewPersistence(middleware.Config{
    Adapter: adapter,
    Collections: map[string]middleware.CollectionConfig{
        "rooms": {
            Events:   []EventType{"room.generated", "room.updated"},
            Debounce: 2 * time.Second,
        },
    },
})

// Create game with both
game := NewGame(
    WithMiddleware(persistence),
    WithRoomRepository(repo),
)
```

### Example 3: Testing
```go
// Mock repository
type mockRoomRepo struct {
    rooms map[string]*Room
}

func (m *mockRoomRepo) Save(ctx context.Context, room *Room) error {
    m.rooms[room.ID] = room
    return nil
}

// Test
func TestRoomGenerator(t *testing.T) {
    mock := &mockRoomRepo{rooms: make(map[string]*Room)}
    generator := NewRoomGenerator(config, WithRepository(mock))
    
    room, err := generator.GenerateRoom(ctx, options)
    assert.NoError(t, err)
    assert.NotNil(t, mock.rooms[room.ID])
}
```

## Migration Strategy for Existing Code

### Current room generator code
```go
type RoomGenerator struct {
    config GeneratorConfig
}

func (g *RoomGenerator) GenerateRoom(options RoomOptions) *Room {
    // Generation logic
    return room
}
```

### Step 1: Add optional repository (minimal change)
```go
type RoomGenerator struct {
    config   GeneratorConfig
    roomRepo RoomRepository // Add this field
}

// Update constructor
func NewRoomGenerator(config GeneratorConfig, repo RoomRepository) *RoomGenerator {
    return &RoomGenerator{
        config:   config,
        roomRepo: repo, // Can be nil
    }
}

// Update method to handle persistence
func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    room := g.createRoom(options)
    
    if g.roomRepo != nil {
        if err := g.roomRepo.Save(ctx, room); err != nil {
            return nil, err
        }
    }
    
    return room, nil
}
```

### Step 2: Add context support
```go
// Add context to all methods for proper Go patterns
func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    // Can now handle timeouts, cancellation, tracing
}
```

## Decision Matrix

| Feature | Repository | Middleware | Hybrid |
|---------|------------|------------|---------|
| Minimal code changes | ✓✓✓ | ✓ | ✓✓ |
| Go idiomatic | ✓✓✓ | ✓✓ | ✓✓✓ |
| Testability | ✓✓✓ | ✓✓ | ✓✓✓ |
| Explicit control | ✓✓✓ | ✓ | ✓✓✓ |
| Auto-persistence | ✓ | ✓✓✓ | ✓✓✓ |
| Complex queries | ✓✓✓ | ✓ | ✓✓✓ |

## Recommendations

1. **Start with repository injection** - It's the most Go-idiomatic approach
2. **Use functional options** - Provides flexibility without breaking changes
3. **Add context everywhere** - Essential for production Go code
4. **Consider middleware later** - When you need cross-cutting concerns

## Error Handling Best Practices

```go
// Wrap errors with context
func (g *RoomGenerator) GenerateRoom(ctx context.Context, opts RoomOptions) (*Room, error) {
    room := g.createRoom(opts)
    
    if g.roomRepo != nil {
        if err := g.roomRepo.Save(ctx, room); err != nil {
            return nil, fmt.Errorf("generate room: save to repository: %w", err)
        }
    }
    
    return room, nil
}

// Specific error types
type RepositoryError struct {
    Op  string
    Err error
}

func (e RepositoryError) Error() string {
    return fmt.Sprintf("repository %s: %v", e.Op, e.Err)
}
```

## Next Steps

1. Define core interfaces in `core/storage/interfaces.go`
2. Implement base repository types
3. Create adapter implementations
4. Update room generator with optional repository
5. Add comprehensive tests

This approach gives you maximum flexibility with minimal code changes!