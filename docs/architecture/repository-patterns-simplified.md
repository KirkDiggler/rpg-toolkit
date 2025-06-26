# Simplified Repository Patterns

## The Problem
Most repositories need the same basic CRUD operations. We want to avoid reimplementing Save, FindByID, Update, Delete for every entity across every storage backend.

## Option 1: Generic CRUD Functions

```go
// core/storage/crud.go
package storage

import (
    "context"
    "fmt"
)

// Entity must have an ID
type Entity interface {
    GetID() string
}

// Basic CRUD operations as functions
func Save[T Entity](ctx context.Context, adapter Adapter, collection string, entity T) error {
    key := fmt.Sprintf("%s:%s", collection, entity.GetID())
    return adapter.Save(ctx, key, entity)
}

func FindByID[T Entity](ctx context.Context, adapter Adapter, collection string, id string, dest *T) error {
    key := fmt.Sprintf("%s:%s", collection, id)
    return adapter.Load(ctx, key, dest)
}

func Delete(ctx context.Context, adapter Adapter, collection string, id string) error {
    key := fmt.Sprintf("%s:%s", collection, id)
    return adapter.Delete(ctx, key)
}

// Usage in custom repository
type roomRepository struct {
    adapter    Adapter
    collection string
}

func (r *roomRepository) Save(ctx context.Context, room *Room) error {
    return storage.Save(ctx, r.adapter, r.collection, room)
}

func (r *roomRepository) FindByID(ctx context.Context, id string) (*Room, error) {
    var room Room
    err := storage.FindByID(ctx, r.adapter, r.collection, id, &room)
    return &room, err
}

// Custom method
func (r *roomRepository) FindByTag(ctx context.Context, tag string) ([]*Room, error) {
    // Custom implementation
}
```

## Option 2: BaseRepository Struct (Composition, not inheritance)

```go
// core/storage/base_repository.go
package storage

// BaseRepository provides common CRUD operations
type BaseRepository[T Entity] struct {
    adapter    Adapter
    collection string
}

func NewBaseRepository[T Entity](adapter Adapter, collection string) *BaseRepository[T] {
    return &BaseRepository[T]{
        adapter:    adapter,
        collection: collection,
    }
}

func (b *BaseRepository[T]) Save(ctx context.Context, entity T) error {
    key := fmt.Sprintf("%s:%s", b.collection, entity.GetID())
    return b.adapter.Save(ctx, key, entity)
}

func (b *BaseRepository[T]) FindByID(ctx context.Context, id string) (*T, error) {
    var entity T
    key := fmt.Sprintf("%s:%s", b.collection, id)
    err := b.adapter.Load(ctx, key, &entity)
    return &entity, err
}

// Room repository uses composition
type roomRepository struct {
    *BaseRepository[*Room]  // Provides Save, FindByID, Delete
    adapter Adapter         // Keep adapter for custom queries
}

func NewRoomRepository(adapter Adapter) RoomRepository {
    return &roomRepository{
        BaseRepository: NewBaseRepository[*Room](adapter, "rooms"),
        adapter:        adapter,
    }
}

// Custom method
func (r *roomRepository) FindByTag(ctx context.Context, tag string) ([]*Room, error) {
    // Implementation depends on adapter type
    switch a := r.adapter.(type) {
    case RedisAdapter:
        // Use Redis sets
        ids, err := a.SMembers(ctx, fmt.Sprintf("rooms:tag:%s", tag))
        if err != nil {
            return nil, err
        }
        // Load each room...
    case MongoAdapter:
        // Use Mongo query
        var rooms []*Room
        err := a.Find(ctx, "rooms", bson.M{"tags": tag}, &rooms)
        return rooms, err
    default:
        return nil, fmt.Errorf("FindByTag not supported for this adapter")
    }
}
```

## Option 3: Repository Builder Pattern

```go
// core/storage/builder.go
package storage

type RepositoryBuilder[T Entity] struct {
    adapter    Adapter
    collection string
    indexers   map[string]func(T) []string  // For building indexes
}

func NewRepositoryBuilder[T Entity](adapter Adapter, collection string) *RepositoryBuilder[T] {
    return &RepositoryBuilder[T]{
        adapter:    adapter,
        collection: collection,
        indexers:   make(map[string]func(T) []string),
    }
}

// Add index builder
func (b *RepositoryBuilder[T]) WithIndex(name string, fn func(T) []string) *RepositoryBuilder[T] {
    b.indexers[name] = fn
    return b
}

// Build a repository with just basic CRUD
func (b *RepositoryBuilder[T]) Build() Repository[T] {
    return &builtRepository[T]{
        adapter:    b.adapter,
        collection: b.collection,
        indexers:   b.indexers,
    }
}

// Usage
roomRepo := storage.NewRepositoryBuilder[*Room](adapter, "rooms").
    WithIndex("type", func(r *Room) []string { return []string{string(r.Type)} }).
    WithIndex("tags", func(r *Room) []string { return r.Tags }).
    Build()

// Now roomRepo has Save, FindByID, Delete automatically
// Plus can query by indexes:
rooms, err := roomRepo.FindByIndex(ctx, "type", "dungeon")
```

## Option 4: Minimal Interface + Adapter Extensions

```go
// Libraries define minimal interface
type RoomRepository interface {
    Save(ctx context.Context, room *Room) error
    FindByID(ctx context.Context, id string) (*Room, error)
    // That's it for 80% of use cases!
}

// Storage package provides simple implementation
type SimpleRepository[T Entity] struct {
    adapter    Adapter
    collection string
}

func (s *SimpleRepository[T]) Save(ctx context.Context, entity T) error {
    return Save(ctx, s.adapter, s.collection, entity)
}

func (s *SimpleRepository[T]) FindByID(ctx context.Context, id string) (*T, error) {
    return FindByID[T](ctx, s.adapter, s.collection, id)
}

// For libraries that need more:
type AdvancedRoomRepository interface {
    RoomRepository
    FindByType(ctx context.Context, roomType RoomType) ([]*Room, error)
    FindConnected(ctx context.Context, roomID string) ([]*Room, error)
}

// Users can:
// 1. Use SimpleRepository for basic needs
roomRepo := &SimpleRepository[*Room]{adapter, "rooms"}

// 2. Extend for custom needs
type myRoomRepo struct {
    *SimpleRepository[*Room]
}

func (m *myRoomRepo) FindByType(ctx context.Context, roomType RoomType) ([]*Room, error) {
    // Custom implementation
}
```

## Option 5: Hybrid - Start Simple, Extend When Needed

```go
// core/storage/standard.go
package storage

// StandardRepository provides CRUD for any entity
func NewStandardRepository[T Entity](adapter Adapter, collection string) Repository[T] {
    return &BaseRepository[T]{adapter, collection}
}

// Room library starts simple
type RoomGeneratorConfig struct {
    RoomRepo Repository[*Room]  // Just needs basic CRUD
}

// Later, if custom queries needed:
type RoomQueryRepository interface {
    Repository[*Room]
    FindByType(ctx context.Context, roomType RoomType) ([]*Room, error)
}

// Extend the standard repository
type extendedRoomRepo struct {
    Repository[*Room]
    adapter Adapter
}

func (e *extendedRoomRepo) FindByType(ctx context.Context, roomType RoomType) ([]*Room, error) {
    // Custom logic here
}
```

## Recommendation: Start with Option 4/5

For most libraries:
```go
// 1. Define minimal interface
type ItemRepository interface {
    Save(ctx context.Context, item *Item) error
    FindByID(ctx context.Context, id string) (*Item, error)
}

// 2. Users can use standard implementation
itemRepo := storage.NewStandardRepository[*Item](adapter, "items")
generator := NewItemGenerator(ItemGeneratorConfig{
    ItemRepo: itemRepo,  // Done!
})

// 3. If they need custom queries later, they extend
type myItemRepo struct {
    storage.Repository[*Item]
    redis RedisAdapter
}

func (m *myItemRepo) FindRare(ctx context.Context) ([]*Item, error) {
    // Custom Redis logic
}
```

This way:
- 80% of cases just use StandardRepository
- No boilerplate for basic CRUD
- Can extend when needed
- Libraries stay simple