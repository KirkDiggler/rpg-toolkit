# Storage Package and Repository Pattern

## Overview

The storage package provides the core abstractions and implementations that repositories build upon. This keeps storage concerns centralized while allowing type-safe, domain-specific repositories.

## Architecture

```
rpg-toolkit/
├── core/
│   └── storage/
│       ├── adapter.go          # Core adapter interface
│       ├── adapters/
│       │   ├── memory.go       # In-memory implementation
│       │   ├── mongo.go        # MongoDB implementation
│       │   └── file.go         # File-based implementation
│       └── repository.go       # Generic repository helpers
└── systems/
    └── rooms/
        └── repository.go       # Room-specific repository
```

## Storage Package Components

### 1. Core Adapter Interface
```go
// core/storage/adapter.go
package storage

import (
    "context"
)

// Adapter is the core storage interface that all backends implement
type Adapter interface {
    Save(ctx context.Context, collection string, id string, data interface{}) error
    Load(ctx context.Context, collection string, id string, dest interface{}) error
    Delete(ctx context.Context, collection string, id string) error
    Query(ctx context.Context, collection string, filter interface{}, results interface{}) error
    
    // Transaction support (optional)
    BeginTx(ctx context.Context) (Transaction, error)
}

type Transaction interface {
    Adapter
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

### 2. Generic Repository Base
```go
// core/storage/repository.go
package storage

import (
    "context"
    "fmt"
)

// Repository provides common CRUD operations for any entity type
type Repository[T any] struct {
    adapter    Adapter
    collection string
}

// Config for creating repositories
type RepositoryConfig struct {
    Adapter    Adapter
    Collection string
}

func NewRepository[T any](cfg RepositoryConfig) *Repository[T] {
    return &Repository[T]{
        adapter:    cfg.Adapter,
        collection: cfg.Collection,
    }
}

// Assuming entities have GetID() method
type Identifiable interface {
    GetID() string
}

func (r *Repository[T]) Save(ctx context.Context, entity T) error {
    if id, ok := any(entity).(Identifiable); ok {
        return r.adapter.Save(ctx, r.collection, id.GetID(), entity)
    }
    return fmt.Errorf("entity must implement Identifiable")
}

func (r *Repository[T]) FindByID(ctx context.Context, id string) (*T, error) {
    var entity T
    err := r.adapter.Load(ctx, r.collection, id, &entity)
    if err != nil {
        return nil, err
    }
    return &entity, nil
}

func (r *Repository[T]) Delete(ctx context.Context, id string) error {
    return r.adapter.Delete(ctx, r.collection, id)
}
```

### 3. MongoDB Adapter Example
```go
// core/storage/adapters/mongo.go
package adapters

import (
    "context"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type MongoConfig struct {
    URI      string
    Database string
}

type MongoAdapter struct {
    client *mongo.Client
    db     *mongo.Database
}

func NewMongoAdapter(cfg MongoConfig) (*MongoAdapter, error) {
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cfg.URI))
    if err != nil {
        return nil, err
    }
    
    return &MongoAdapter{
        client: client,
        db:     client.Database(cfg.Database),
    }, nil
}

func (m *MongoAdapter) Save(ctx context.Context, collection string, id string, data interface{}) error {
    coll := m.db.Collection(collection)
    filter := bson.M{"_id": id}
    update := bson.M{"$set": data}
    opts := options.Update().SetUpsert(true)
    
    _, err := coll.UpdateOne(ctx, filter, update, opts)
    return err
}

func (m *MongoAdapter) Query(ctx context.Context, collection string, filter interface{}, results interface{}) error {
    coll := m.db.Collection(collection)
    cursor, err := coll.Find(ctx, filter)
    if err != nil {
        return err
    }
    return cursor.All(ctx, results)
}
```

## Creating Domain-Specific Repositories

### Room Repository Using Storage Package
```go
// systems/rooms/repository.go
package rooms

import (
    "context"
    "github.com/yourusername/rpg-toolkit/core/storage"
)

// RoomRepository defines room-specific operations
type RoomRepository interface {
    Save(ctx context.Context, room *Room) error
    FindByID(ctx context.Context, id string) (*Room, error)
    FindByType(ctx context.Context, roomType RoomType) ([]*Room, error)
    FindConnected(ctx context.Context, roomID string) ([]*Room, error)
    Delete(ctx context.Context, id string) error
}

// roomRepository implementation using storage package
type roomRepository struct {
    *storage.Repository[Room]  // Embed generic repository
    adapter storage.Adapter    // Keep adapter for custom queries
}

// Config for room repository
type RepositoryConfig struct {
    Adapter storage.Adapter
}

// Create room repository using storage adapter
func NewRepository(cfg RepositoryConfig) RoomRepository {
    return &roomRepository{
        Repository: storage.NewRepository[Room](storage.RepositoryConfig{
            Adapter:    cfg.Adapter,
            Collection: "rooms",
        }),
        adapter: cfg.Adapter,
    }
}

// Custom query methods
func (r *roomRepository) FindByType(ctx context.Context, roomType RoomType) ([]*Room, error) {
    var rooms []*Room
    filter := map[string]interface{}{"type": roomType}
    err := r.adapter.Query(ctx, "rooms", filter, &rooms)
    return rooms, err
}

func (r *roomRepository) FindConnected(ctx context.Context, roomID string) ([]*Room, error) {
    var rooms []*Room
    filter := map[string]interface{}{"connected_to": roomID}
    err := r.adapter.Query(ctx, "rooms", filter, &rooms)
    return rooms, err
}
```

## Usage Examples

### 1. Setting Up Storage and Repositories
```go
package main

import (
    "github.com/yourusername/rpg-toolkit/core/storage/adapters"
    "github.com/yourusername/rpg-toolkit/systems/rooms"
)

func setupGame(ctx context.Context) (*Game, error) {
    // Create storage adapter
    mongoAdapter, err := adapters.NewMongoAdapter(adapters.MongoConfig{
        URI:      "mongodb://localhost:27017",
        Database: "rpg_game",
    })
    if err != nil {
        return nil, err
    }
    
    // Create repositories using the adapter
    roomRepo := rooms.NewRepository(rooms.RepositoryConfig{
        Adapter: mongoAdapter,
    })
    
    characterRepo := characters.NewRepository(characters.RepositoryConfig{
        Adapter: mongoAdapter,
    })
    
    // Create game with repositories
    game := NewGame(GameConfig{
        Name:          "My RPG",
        RoomRepo:      roomRepo,
        CharacterRepo: characterRepo,
    })
    
    return game, nil
}
```

### 2. Different Storage Backends
```go
// For testing - use memory adapter
memAdapter := adapters.NewMemoryAdapter()
roomRepo := rooms.NewRepository(rooms.RepositoryConfig{
    Adapter: memAdapter,
})

// For file-based games
fileAdapter := adapters.NewFileAdapter(adapters.FileConfig{
    BasePath: "./game-data",
})
roomRepo := rooms.NewRepository(rooms.RepositoryConfig{
    Adapter: fileAdapter,
})

// For production - MongoDB
mongoAdapter, _ := adapters.NewMongoAdapter(adapters.MongoConfig{
    URI:      os.Getenv("MONGO_URI"),
    Database: "rpg_production",
})
roomRepo := rooms.NewRepository(rooms.RepositoryConfig{
    Adapter: mongoAdapter,
})
```

### 3. Room Generator Integration
```go
// Your existing room generator just needs the repo
generator := NewRoomGenerator(RoomGeneratorConfig{
    DungeonTypes: []string{"cave", "dungeon", "temple"},
    MaxRoomSize:  100,
    MinRoomSize:  10,
    RoomRepo:     roomRepo,  // Repository created from storage package
})

// Generate and auto-persist
room, err := generator.GenerateRoom(ctx, RoomOptions{
    Type: "dungeon",
})
```

## Benefits of This Approach

1. **Separation of Concerns**
   - Storage package handles persistence mechanics
   - Repositories provide domain-specific interfaces
   - Libraries (like RoomGenerator) just use interfaces

2. **Flexibility**
   - Swap storage backends without changing repository interfaces
   - Add new storage adapters without touching domain code
   - Test with memory adapter, deploy with MongoDB

3. **Type Safety**
   - Generic repository provides type-safe base operations
   - Domain repositories add specific queries
   - No interface{} in domain code

4. **Minimal Boilerplate**
   - Generic repository handles common CRUD
   - Only write custom methods for domain-specific queries
   - Storage adapters are reusable across all repositories

## Storage Package Structure
```
core/storage/
├── adapter.go              # Core interfaces
├── repository.go           # Generic repository
├── errors.go              # Common error types
├── adapters/
│   ├── memory.go          # In-memory adapter
│   ├── mongo.go           # MongoDB adapter
│   ├── file.go            # File system adapter
│   └── postgres.go        # PostgreSQL adapter
└── testing/
    └── mock.go            # Mock adapter for tests
```

This way, the storage package provides the foundation, and each system creates its own repositories using that foundation. Simple, clean, and maintains separation of concerns!