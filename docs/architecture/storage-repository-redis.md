# Storage and Repository Pattern (Practical Implementation)

## Overview

This document shows how to build repositories using the storage package, with a focus on practical implementation and handling different backends like Redis.

## Core Storage Interfaces

```go
// core/storage/adapter.go
package storage

import "context"

// Adapter provides basic storage operations
// Note: Query is intentionally omitted - adapters provide their own query methods
type Adapter interface {
    Save(ctx context.Context, key string, value any) error
    Load(ctx context.Context, key string, dest any) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}

// Each adapter can extend with its own query capabilities
type RedisAdapter interface {
    Adapter
    // Redis-specific operations
    SAdd(ctx context.Context, key string, members ...any) error
    SMembers(ctx context.Context, key string) ([]string, error)
    HSet(ctx context.Context, key string, field string, value any) error
    HGetAll(ctx context.Context, key string) (map[string]string, error)
    ZAdd(ctx context.Context, key string, score float64, member any) error
    ZRangeByScore(ctx context.Context, key string, min, max float64) ([]string, error)
}

type MongoAdapter interface {
    Adapter
    // MongoDB-specific operations
    Find(ctx context.Context, collection string, filter any, results any) error
    FindOne(ctx context.Context, collection string, filter any, result any) error
    Aggregate(ctx context.Context, collection string, pipeline any, results any) error
}
```

## Redis Implementation

```go
// core/storage/adapters/redis.go
package adapters

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/redis/go-redis/v9"
)

type RedisConfig struct {
    Addr     string
    Password string
    DB       int
}

type redisAdapter struct {
    client *redis.Client
    prefix string // Optional key prefix
}

func NewRedisAdapter(cfg RedisConfig) *redisAdapter {
    return &redisAdapter{
        client: redis.NewClient(&redis.Options{
            Addr:     cfg.Addr,
            Password: cfg.Password,
            DB:       cfg.DB,
        }),
        prefix: "rpg:", // Optional prefix for all keys
    }
}

func (r *redisAdapter) key(k string) string {
    return r.prefix + k
}

func (r *redisAdapter) Save(ctx context.Context, key string, value any) error {
    data, err := json.Marshal(value)
    if err != nil {
        return fmt.Errorf("marshal value: %w", err)
    }
    return r.client.Set(ctx, r.key(key), data, 0).Err()
}

func (r *redisAdapter) Load(ctx context.Context, key string, dest any) error {
    data, err := r.client.Get(ctx, r.key(key)).Bytes()
    if err != nil {
        if err == redis.Nil {
            return fmt.Errorf("key not found: %s", key)
        }
        return err
    }
    return json.Unmarshal(data, dest)
}

// Redis-specific methods
func (r *redisAdapter) SAdd(ctx context.Context, key string, members ...any) error {
    return r.client.SAdd(ctx, r.key(key), members...).Err()
}

func (r *redisAdapter) ZAdd(ctx context.Context, key string, score float64, member any) error {
    return r.client.ZAdd(ctx, r.key(key), redis.Z{Score: score, Member: member}).Err()
}

func (r *redisAdapter) ZRangeByScore(ctx context.Context, key string, min, max float64) ([]string, error) {
    return r.client.ZRangeByScore(ctx, r.key(key), &redis.ZRangeBy{
        Min: fmt.Sprintf("%f", min),
        Max: fmt.Sprintf("%f", max),
    }).Result()
}
```

## Building Room Repository

```go
// systems/rooms/repository.go
package rooms

import (
    "context"
    "fmt"
    "github.com/yourusername/rpg-toolkit/core/storage"
)

// RoomRepository interface that libraries use
type RoomRepository interface {
    Save(ctx context.Context, room *Room) error
    FindByID(ctx context.Context, id string) (*Room, error)
    FindByType(ctx context.Context, roomType RoomType) ([]*Room, error)
    FindConnected(ctx context.Context, roomID string) ([]*Room, error)
    Delete(ctx context.Context, id string) error
}

// Redis implementation
type redisRoomRepository struct {
    redis storage.RedisAdapter
}

func NewRedisRepository(redis storage.RedisAdapter) RoomRepository {
    return &redisRoomRepository{redis: redis}
}

func (r *redisRoomRepository) Save(ctx context.Context, room *Room) error {
    // Save room data
    key := fmt.Sprintf("room:%s", room.ID)
    if err := r.redis.Save(ctx, key, room); err != nil {
        return err
    }
    
    // Index by type using Redis sets
    typeKey := fmt.Sprintf("rooms:type:%s", room.Type)
    if err := r.redis.SAdd(ctx, typeKey, room.ID); err != nil {
        return err
    }
    
    // Index connections
    for _, connectedID := range room.ConnectedRooms {
        connKey := fmt.Sprintf("rooms:connected:%s", connectedID)
        if err := r.redis.SAdd(ctx, connKey, room.ID); err != nil {
            return err
        }
    }
    
    return nil
}

func (r *redisRoomRepository) FindByID(ctx context.Context, id string) (*Room, error) {
    var room Room
    key := fmt.Sprintf("room:%s", id)
    if err := r.redis.Load(ctx, key, &room); err != nil {
        return nil, err
    }
    return &room, nil
}

func (r *redisRoomRepository) FindByType(ctx context.Context, roomType RoomType) ([]*Room, error) {
    // Get room IDs from type index
    typeKey := fmt.Sprintf("rooms:type:%s", roomType)
    roomIDs, err := r.redis.SMembers(ctx, typeKey)
    if err != nil {
        return nil, err
    }
    
    // Load each room
    rooms := make([]*Room, 0, len(roomIDs))
    for _, id := range roomIDs {
        room, err := r.FindByID(ctx, id)
        if err != nil {
            continue // Skip missing rooms
        }
        rooms = append(rooms, room)
    }
    
    return rooms, nil
}

// MongoDB implementation
type mongoRoomRepository struct {
    mongo storage.MongoAdapter
}

func NewMongoRepository(mongo storage.MongoAdapter) RoomRepository {
    return &mongoRoomRepository{mongo: mongo}
}

func (r *mongoRoomRepository) Save(ctx context.Context, room *Room) error {
    return r.mongo.Save(ctx, fmt.Sprintf("rooms/%s", room.ID), room)
}

func (r *mongoRoomRepository) FindByType(ctx context.Context, roomType RoomType) ([]*Room, error) {
    var rooms []*Room
    filter := map[string]any{"type": roomType}
    if err := r.mongo.Find(ctx, "rooms", filter, &rooms); err != nil {
        return nil, err
    }
    return rooms, nil
}

// Memory implementation for testing
type memoryRoomRepository struct {
    rooms map[string]*Room
    mu    sync.RWMutex
}

func NewMemoryRepository() RoomRepository {
    return &memoryRoomRepository{
        rooms: make(map[string]*Room),
    }
}

func (r *memoryRoomRepository) Save(ctx context.Context, room *Room) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.rooms[room.ID] = room
    return nil
}

func (r *memoryRoomRepository) FindByType(ctx context.Context, roomType RoomType) ([]*Room, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var rooms []*Room
    for _, room := range r.rooms {
        if room.Type == roomType {
            rooms = append(rooms, room)
        }
    }
    return rooms, nil
}
```

## Factory Pattern for Repository Creation

```go
// systems/rooms/factory.go
package rooms

type RepositoryType string

const (
    RepositoryRedis  RepositoryType = "redis"
    RepositoryMongo  RepositoryType = "mongo"
    RepositoryMemory RepositoryType = "memory"
)

type RepositoryFactoryConfig struct {
    Type  RepositoryType
    Redis storage.RedisAdapter
    Mongo storage.MongoAdapter
}

func NewRepositoryFromConfig(cfg RepositoryFactoryConfig) (RoomRepository, error) {
    switch cfg.Type {
    case RepositoryRedis:
        if cfg.Redis == nil {
            return nil, fmt.Errorf("redis adapter required")
        }
        return NewRedisRepository(cfg.Redis), nil
    
    case RepositoryMongo:
        if cfg.Mongo == nil {
            return nil, fmt.Errorf("mongo adapter required")
        }
        return NewMongoRepository(cfg.Mongo), nil
    
    case RepositoryMemory:
        return NewMemoryRepository(), nil
    
    default:
        return nil, fmt.Errorf("unknown repository type: %s", cfg.Type)
    }
}
```

## Usage Examples

```go
// Setup with Redis
redisAdapter := adapters.NewRedisAdapter(adapters.RedisConfig{
    Addr: "localhost:6379",
})

roomRepo := rooms.NewRedisRepository(redisAdapter)

generator := NewRoomGenerator(RoomGeneratorConfig{
    DungeonTypes: []string{"cave", "dungeon"},
    MaxRoomSize:  100,
    RoomRepo:     roomRepo,
})

// Setup with MongoDB
mongoAdapter, _ := adapters.NewMongoAdapter(adapters.MongoConfig{
    URI:      "mongodb://localhost:27017",
    Database: "rpg_game",
})

roomRepo := rooms.NewMongoRepository(mongoAdapter)

// Testing with memory
roomRepo := rooms.NewMemoryRepository()
```

## Key Points

1. **No Query Method on Base Adapter** - Each adapter provides its own query methods
2. **Adapter-Specific Interfaces** - RedisAdapter, MongoAdapter extend base with their operations
3. **Repository Implementations** - Each storage backend gets its own repository implementation
4. **Type Assertions Where Needed** - When you need backend-specific features:
   ```go
   if redisRepo, ok := repo.(*redisRoomRepository); ok {
       // Can access Redis-specific features
   }
   ```

## Redis-Specific Patterns

```go
// Using Redis for leaderboards
type LeaderboardRepository interface {
    AddScore(ctx context.Context, playerID string, score int) error
    GetTopPlayers(ctx context.Context, limit int) ([]PlayerScore, error)
}

type redisLeaderboard struct {
    redis storage.RedisAdapter
}

func (l *redisLeaderboard) AddScore(ctx context.Context, playerID string, score int) error {
    return l.redis.ZAdd(ctx, "leaderboard", float64(score), playerID)
}

func (l *redisLeaderboard) GetTopPlayers(ctx context.Context, limit int) ([]PlayerScore, error) {
    // This shows why we need Redis-specific methods
    members, err := l.redis.ZRevRangeWithScores(ctx, "leaderboard", 0, int64(limit-1))
    if err != nil {
        return nil, err
    }
    
    scores := make([]PlayerScore, len(members))
    for i, m := range members {
        scores[i] = PlayerScore{
            PlayerID: m.Member.(string),
            Score:    int(m.Score),
        }
    }
    return scores, nil
}
```

This approach:
- No fake inheritance with embedding
- Clear adapter-specific interfaces
- Repositories built for each backend
- Libraries just use the repository interface