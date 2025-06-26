# Go Persistence Patterns for RPG Toolkit

## Overview

This document outlines persistence strategies for the RPG Toolkit following your Go patterns: Config structs with exported fields that initialize unexported struct members.

## Pattern 1: Repository Injection via Config

### Implementation
```go
// Repository interface
type RoomRepository interface {
    Save(ctx context.Context, room *Room) error
    FindByID(ctx context.Context, id string) (*Room, error)
    FindByTag(ctx context.Context, tag string) ([]*Room, error)
    Delete(ctx context.Context, id string) error
}

// Config with exported fields
type RoomGeneratorConfig struct {
    // Required fields
    DungeonTypes []string
    MaxRoomSize  int
    MinRoomSize  int
    
    // Optional repository for persistence
    RoomRepo RoomRepository
    
    // Optional event emitter
    EventEmitter EventEmitter
}

// Generator with unexported fields
type RoomGenerator struct {
    dungeonTypes []string
    maxRoomSize  int
    minRoomSize  int
    
    // Optional dependencies
    roomRepo     RoomRepository
    eventEmitter EventEmitter
}

// Constructor follows your pattern
func NewRoomGenerator(cfg RoomGeneratorConfig) *RoomGenerator {
    return &RoomGenerator{
        dungeonTypes: cfg.DungeonTypes,
        maxRoomSize:  cfg.MaxRoomSize,
        minRoomSize:  cfg.MinRoomSize,
        roomRepo:     cfg.RoomRepo,     // Can be nil
        eventEmitter: cfg.EventEmitter,  // Can be nil
    }
}

func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    room := g.createRoom(options)
    
    // Only persist if repository was provided
    if g.roomRepo != nil {
        if err := g.roomRepo.Save(ctx, room); err != nil {
            return nil, fmt.Errorf("failed to save room: %w", err)
        }
    }
    
    // Emit event if emitter was provided
    if g.eventEmitter != nil {
        g.eventEmitter.Emit(ctx, Event{
            Type: EventRoomGenerated,
            Data: room,
        })
    }
    
    return room, nil
}

// Usage without persistence
generator := NewRoomGenerator(RoomGeneratorConfig{
    DungeonTypes: []string{"cave", "dungeon", "temple"},
    MaxRoomSize:  100,
    MinRoomSize:  10,
    // RoomRepo is nil - no persistence
})

// Usage with MongoDB repository
generator := NewRoomGenerator(RoomGeneratorConfig{
    DungeonTypes: []string{"cave", "dungeon", "temple"},
    MaxRoomSize:  100,
    MinRoomSize:  10,
    RoomRepo:     mongo.NewRoomRepository(db),
})
```

## Pattern 2: Storage Module with Config Pattern

### Storage Adapter Interface
```go
package storage

// Core storage adapter interface
type Adapter interface {
    Save(ctx context.Context, collection string, id string, data interface{}) error
    Load(ctx context.Context, collection string, id string, dest interface{}) error
    Delete(ctx context.Context, collection string, id string) error
    Query(ctx context.Context, collection string, filter interface{}, results interface{}) error
}

// Config for storage adapters
type AdapterConfig struct {
    Type string // "memory", "mongo", "file", etc.
    
    // MongoDB specific
    MongoURI      string
    MongoDB       string
    
    // File specific
    FilePath      string
    
    // Common options
    MaxRetries    int
    RetryDelay    time.Duration
}

// Factory function
func NewAdapter(cfg AdapterConfig) (Adapter, error) {
    switch cfg.Type {
    case "memory":
        return NewMemoryAdapter(), nil
    case "mongo":
        return NewMongoAdapter(cfg)
    case "file":
        return NewFileAdapter(cfg)
    default:
        return nil, fmt.Errorf("unknown adapter type: %s", cfg.Type)
    }
}
```

### Generic Repository Implementation
```go
// Generic repository config
type RepositoryConfig struct {
    Adapter    Adapter
    Collection string
}

// Generic repository using your pattern
type Repository[T any] struct {
    adapter    Adapter
    collection string
}

func NewRepository[T any](cfg RepositoryConfig) *Repository[T] {
    return &Repository[T]{
        adapter:    cfg.Adapter,
        collection: cfg.Collection,
    }
}

func (r *Repository[T]) Save(ctx context.Context, id string, entity T) error {
    return r.adapter.Save(ctx, r.collection, id, entity)
}

// Room repository extending generic
type MongoRoomRepository struct {
    *Repository[Room]
    db *mongo.Database
}

type MongoRoomRepositoryConfig struct {
    DB         *mongo.Database
    Collection string
}

func NewMongoRoomRepository(cfg MongoRoomRepositoryConfig) RoomRepository {
    return &MongoRoomRepository{
        Repository: NewRepository[Room](RepositoryConfig{
            Adapter:    &MongoAdapter{db: cfg.DB},
            Collection: cfg.Collection,
        }),
        db: cfg.DB,
    }
}

// Custom queries
func (r *MongoRoomRepository) FindByTag(ctx context.Context, tag string) ([]*Room, error) {
    var rooms []*Room
    cursor, err := r.db.Collection(r.collection).Find(ctx, bson.M{"tags": tag})
    if err != nil {
        return nil, err
    }
    if err := cursor.All(ctx, &rooms); err != nil {
        return nil, err
    }
    return rooms, nil
}
```

## Pattern 3: Middleware System (Go Style)

### Middleware Implementation
```go
// Middleware config
type PersistenceMiddlewareConfig struct {
    Adapter     Adapter
    Collections map[string]CollectionConfig
}

type CollectionConfig struct {
    Events   []EventType
    Debounce time.Duration
}

// Middleware struct
type PersistenceMiddleware struct {
    adapter     Adapter
    collections map[string]CollectionConfig
    buffers     map[string]*debounceBuffer
    mu          sync.Mutex
}

func NewPersistenceMiddleware(cfg PersistenceMiddlewareConfig) *PersistenceMiddleware {
    pm := &PersistenceMiddleware{
        adapter:     cfg.Adapter,
        collections: cfg.Collections,
        buffers:     make(map[string]*debounceBuffer),
    }
    
    // Initialize debounce buffers
    for name, config := range cfg.Collections {
        pm.buffers[name] = newDebounceBuffer(config.Debounce)
    }
    
    return pm
}

// Handle events
func (pm *PersistenceMiddleware) HandleEvent(ctx context.Context, event Event) error {
    for collection, config := range pm.collections {
        for _, eventType := range config.Events {
            if event.Type == eventType {
                return pm.scheduleSave(ctx, collection, event.Data)
            }
        }
    }
    return nil
}

// Debounced save
func (pm *PersistenceMiddleware) scheduleSave(ctx context.Context, collection string, data interface{}) error {
    buffer := pm.buffers[collection]
    if buffer == nil {
        return fmt.Errorf("no buffer for collection: %s", collection)
    }
    
    return buffer.Add(ctx, data, func(items []interface{}) error {
        // Batch save if multiple items
        for _, item := range items {
            if entity, ok := item.(Identifiable); ok {
                if err := pm.adapter.Save(ctx, collection, entity.GetID(), item); err != nil {
                    return err
                }
            }
        }
        return nil
    })
}
```

## Complete Example: Game Setup

```go
// Main game config following your pattern
type GameConfig struct {
    // Core game settings
    Name        string
    MaxPlayers  int
    
    // Storage configuration
    StorageAdapter Adapter
    
    // Optional repositories
    RoomRepo      RoomRepository
    CharacterRepo CharacterRepository
    InventoryRepo InventoryRepository
    
    // Optional middleware
    Middleware []Middleware
}

type Game struct {
    name        string
    maxPlayers  int
    
    // Storage
    storage     Adapter
    
    // Repositories
    roomRepo      RoomRepository
    characterRepo CharacterRepository
    inventoryRepo InventoryRepository
    
    // Systems
    roomGenerator *RoomGenerator
    charManager   *CharacterManager
    
    // Middleware
    middleware []Middleware
}

func NewGame(cfg GameConfig) *Game {
    g := &Game{
        name:          cfg.Name,
        maxPlayers:    cfg.MaxPlayers,
        storage:       cfg.StorageAdapter,
        roomRepo:      cfg.RoomRepo,
        characterRepo: cfg.CharacterRepo,
        inventoryRepo: cfg.InventoryRepo,
        middleware:    cfg.Middleware,
    }
    
    // Initialize subsystems with their configs
    g.roomGenerator = NewRoomGenerator(RoomGeneratorConfig{
        DungeonTypes: []string{"cave", "dungeon", "temple"},
        MaxRoomSize:  100,
        MinRoomSize:  10,
        RoomRepo:     cfg.RoomRepo,
    })
    
    g.charManager = NewCharacterManager(CharacterManagerConfig{
        MaxLevel:      20,
        StartingHP:    100,
        CharacterRepo: cfg.CharacterRepo,
    })
    
    return g
}

// Usage example
func main() {
    // Setup storage
    storageAdapter, err := storage.NewAdapter(storage.AdapterConfig{
        Type:     "mongo",
        MongoURI: "mongodb://localhost:27017",
        MongoDB:  "rpg_game",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup repositories
    roomRepo := NewMongoRoomRepository(MongoRoomRepositoryConfig{
        DB:         mongoClient.Database("rpg_game"),
        Collection: "rooms",
    })
    
    // Setup middleware
    persistMiddleware := NewPersistenceMiddleware(PersistenceMiddlewareConfig{
        Adapter: storageAdapter,
        Collections: map[string]CollectionConfig{
            "rooms": {
                Events:   []EventType{EventRoomGenerated, EventRoomUpdated},
                Debounce: 2 * time.Second,
            },
        },
    })
    
    // Create game with everything
    game := NewGame(GameConfig{
        Name:           "My RPG",
        MaxPlayers:     100,
        StorageAdapter: storageAdapter,
        RoomRepo:       roomRepo,
        Middleware:     []Middleware{persistMiddleware},
    })
    
    // Generate room - automatically persisted
    room, err := game.roomGenerator.GenerateRoom(ctx, RoomOptions{
        Type: "dungeon",
        Size: "large",
    })
}
```

## Testing with Config Pattern

```go
// Mock repository
type mockRoomRepo struct {
    rooms map[string]*Room
    mu    sync.Mutex
}

func (m *mockRoomRepo) Save(ctx context.Context, room *Room) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.rooms[room.ID] = room
    return nil
}

// Test
func TestRoomGeneratorWithPersistence(t *testing.T) {
    mockRepo := &mockRoomRepo{
        rooms: make(map[string]*Room),
    }
    
    generator := NewRoomGenerator(RoomGeneratorConfig{
        DungeonTypes: []string{"test"},
        MaxRoomSize:  50,
        MinRoomSize:  10,
        RoomRepo:     mockRepo,
    })
    
    room, err := generator.GenerateRoom(context.Background(), RoomOptions{
        Type: "dungeon",
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, room)
    assert.Equal(t, room, mockRepo.rooms[room.ID])
}

// Test without persistence
func TestRoomGeneratorNoPersistence(t *testing.T) {
    generator := NewRoomGenerator(RoomGeneratorConfig{
        DungeonTypes: []string{"test"},
        MaxRoomSize:  50,
        MinRoomSize:  10,
        // RoomRepo is nil
    })
    
    room, err := generator.GenerateRoom(context.Background(), RoomOptions{
        Type: "dungeon",
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, room)
}
```

## Migration Path for Room Generator

### Current code
```go
type RoomGenerator struct {
    dungeonTypes []string
    maxRoomSize  int
}

func NewRoomGenerator(dungeonTypes []string, maxSize int) *RoomGenerator {
    return &RoomGenerator{
        dungeonTypes: dungeonTypes,
        maxRoomSize:  maxSize,
    }
}
```

### Step 1: Convert to Config pattern
```go
type RoomGeneratorConfig struct {
    DungeonTypes []string
    MaxRoomSize  int
    MinRoomSize  int
    RoomRepo     RoomRepository // Add this
}

type RoomGenerator struct {
    dungeonTypes []string
    maxRoomSize  int
    minRoomSize  int
    roomRepo     RoomRepository // Add this
}

func NewRoomGenerator(cfg RoomGeneratorConfig) *RoomGenerator {
    return &RoomGenerator{
        dungeonTypes: cfg.DungeonTypes,
        maxRoomSize:  cfg.MaxRoomSize,
        minRoomSize:  cfg.MinRoomSize,
        roomRepo:     cfg.RoomRepo, // Can be nil
    }
}
```

### Step 2: Update GenerateRoom
```go
func (g *RoomGenerator) GenerateRoom(ctx context.Context, options RoomOptions) (*Room, error) {
    room := g.createRoom(options)
    
    // Only save if repository provided
    if g.roomRepo != nil {
        if err := g.roomRepo.Save(ctx, room); err != nil {
            return nil, fmt.Errorf("save room: %w", err)
        }
    }
    
    return room, nil
}
```

## Key Benefits

1. **Follows your patterns**: Config structs with exported fields
2. **Minimal changes**: Just update constructor and add nil check
3. **Flexible**: Can use with or without persistence
4. **Testable**: Easy to mock repositories
5. **Go idiomatic**: No magic, explicit dependencies

The room generator literally just needs the config pattern applied and a nil check in GenerateRoom!