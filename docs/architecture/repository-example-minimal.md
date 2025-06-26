# Minimal Repository Example

## The 80% Use Case

Most libraries just need Save and FindByID. Here's how simple it could be:

```go
// 1. Room Generator defines what it needs (minimal!)
// systems/rooms/repository.go
package rooms

type RoomRepository interface {
    Save(ctx context.Context, room *Room) error
    FindByID(ctx context.Context, id string) (*Room, error)
}
```

```go
// 2. Storage package provides a standard implementation
// core/storage/standard.go
package storage

type StandardRepo[T Entity] struct {
    adapter    Adapter
    collection string
}

func NewStandardRepo[T Entity](adapter Adapter, collection string) *StandardRepo[T] {
    return &StandardRepo[T]{adapter, collection}
}

func (r *StandardRepo[T]) Save(ctx context.Context, entity T) error {
    key := fmt.Sprintf("%s:%s", r.collection, entity.GetID())
    return r.adapter.Save(ctx, key, entity)
}

func (r *StandardRepo[T]) FindByID(ctx context.Context, id string) (*T, error) {
    var entity T
    key := fmt.Sprintf("%s:%s", r.collection, id)
    err := r.adapter.Load(ctx, key, &entity)
    return &entity, err
}
```

```go
// 3. User sets it up (one line!)
// main.go
redisAdapter := adapters.NewRedisAdapter(redisConfig)

roomRepo := storage.NewStandardRepo[*Room](redisAdapter, "rooms")

generator := rooms.NewRoomGenerator(rooms.GeneratorConfig{
    DungeonTypes: []string{"cave", "dungeon"},
    RoomRepo:     roomRepo,  // Done! Works with any adapter!
})
```

## When You Need More (The 20% Case)

```go
// 1. Extend the interface
type AdvancedRoomRepository interface {
    RoomRepository  // Still has Save, FindByID
    FindByTag(ctx context.Context, tag string) ([]*Room, error)
}

// 2. Wrap the standard repo
type myRoomRepo struct {
    *storage.StandardRepo[*Room]
    redis storage.RedisAdapter
}

func NewMyRoomRepo(redis storage.RedisAdapter) AdvancedRoomRepository {
    return &myRoomRepo{
        StandardRepo: storage.NewStandardRepo[*Room](redis, "rooms"),
        redis:        redis,
    }
}

// 3. Add custom method
func (r *myRoomRepo) FindByTag(ctx context.Context, tag string) ([]*Room, error) {
    ids, err := r.redis.SMembers(ctx, fmt.Sprintf("rooms:tag:%s", tag))
    if err != nil {
        return nil, err
    }
    
    rooms := make([]*Room, 0, len(ids))
    for _, id := range ids {
        room, err := r.FindByID(ctx, id)  // Reuse standard method!
        if err == nil {
            rooms = append(rooms, room)
        }
    }
    return rooms, nil
}

// Override Save to maintain indexes
func (r *myRoomRepo) Save(ctx context.Context, room *Room) error {
    // Call standard save
    if err := r.StandardRepo.Save(ctx, room); err != nil {
        return err
    }
    
    // Add to tag indexes
    for _, tag := range room.Tags {
        key := fmt.Sprintf("rooms:tag:%s", tag)
        if err := r.redis.SAdd(ctx, key, room.ID); err != nil {
            return err
        }
    }
    return nil
}
```

## Benefits

1. **Zero boilerplate for simple cases** - Just use StandardRepo
2. **Works with any adapter** - Redis, Mongo, Memory all work the same
3. **Extensible** - Add custom methods when needed
4. **Type safe** - Generics give you compile-time safety
5. **Library authors happy** - Just define minimal interface
6. **Users happy** - One line setup for basic cases

## Real World Flow

```go
// Library author (you)
type ItemRepository interface {
    Save(ctx context.Context, item *Item) error
    FindByID(ctx context.Context, id string) (*Item, error)
}

// User with basic needs
itemRepo := storage.NewStandardRepo[*Item](mongoAdapter, "items")

// User with custom needs
type gameItemRepo struct {
    *storage.StandardRepo[*Item]
    cache *cache.Cache
}

func (g *gameItemRepo) FindByRarity(ctx context.Context, rarity int) ([]*Item, error) {
    // Check cache first
    // Then custom query logic
}

// Both work with your library!
```

This feels like the sweet spot - simple by default, extensible when needed.