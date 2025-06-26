# Persistence Patterns: Repository Injection vs Middleware

## Overview

This document compares two persistence patterns for the RPG Toolkit: Repository Injection and Middleware-based persistence. Both patterns have their strengths and can actually complement each other in a hybrid approach.

## Pattern 1: Repository Injection

### How It Works
```typescript
// Define repository interface
interface RoomRepository {
  save(room: Room): Promise<void>;
  findById(id: string): Promise<Room | null>;
  findByTag(tag: string): Promise<Room[]>;
  delete(id: string): Promise<void>;
}

// Inject repository into generator
class RoomGenerator {
  constructor(private roomRepo?: RoomRepository) {}
  
  async generateRoom(config: RoomConfig): Promise<Room> {
    const room = this.createRoom(config);
    
    // Only persist if repository is provided
    if (this.roomRepo) {
      await this.roomRepo.save(room);
    }
    
    return room;
  }
}

// Usage
const generator = new RoomGenerator(new MongoRoomRepository());
const room = await generator.generateRoom({ type: 'dungeon' });
```

### Pros
- **Explicit**: Clear what's being persisted and when
- **Testable**: Easy to mock repositories
- **Type-safe**: Full TypeScript support
- **Flexible**: Different repos for different needs
- **Minimal changes**: Just add optional repo parameter

### Cons
- **Manual wiring**: Users must inject repositories
- **Scattered persistence**: Each module handles its own
- **Potential duplication**: Similar repo interfaces across modules

## Pattern 2: Middleware Persistence

### How It Works
```typescript
// Libraries emit events
class RoomGenerator {
  async generateRoom(config: RoomConfig): Promise<Room> {
    const room = this.createRoom(config);
    this.emit('room.generated', room);
    return room;
  }
}

// Middleware handles persistence
const persistence = new PersistenceMiddleware({
  adapter: new MongoAdapter(),
  collections: {
    rooms: {
      events: ['room.generated', 'room.updated'],
      debounce: 1000
    }
  }
});

game.use(persistence);
```

### Pros
- **Transparent**: Libraries don't know about persistence
- **Centralized**: All persistence configured in one place
- **Consistent**: Same pattern across all modules
- **Feature-rich**: Debouncing, batching, auto-save

### Cons
- **Implicit**: Less obvious when persistence happens
- **Complex queries**: Harder to implement custom queries
- **Learning curve**: Users must understand middleware system

## Pattern 3: Hybrid Approach (Recommended)

### Combine Both Patterns
```typescript
// 1. Libraries can accept optional repositories
class RoomGenerator {
  constructor(private roomRepo?: RoomRepository) {}
  
  async generateRoom(config: RoomConfig): Promise<Room> {
    const room = this.createRoom(config);
    
    // Direct persistence if repo provided
    if (this.roomRepo) {
      await this.roomRepo.save(room);
    }
    
    // Always emit event for middleware
    this.emit('room.generated', room);
    
    return room;
  }
  
  // Complex queries through repository
  async findConnectedRooms(roomId: string): Promise<Room[]> {
    if (!this.roomRepo) {
      throw new Error('Repository required for queries');
    }
    return this.roomRepo.findConnected(roomId);
  }
}

// 2. Middleware can create and inject repositories
class PersistenceMiddleware {
  createRepository<T>(collection: string): Repository<T> {
    return new GenericRepository(this.adapter, collection);
  }
  
  autoInject(game: Game) {
    // Automatically inject repositories into modules
    game.roomGenerator = new RoomGenerator(
      this.createRepository<Room>('rooms')
    );
  }
}
```

## Use Case Comparison

### Simple State Persistence
**Winner: Middleware**
```typescript
// Middleware: Automatic
character.levelUp(); // Saved automatically

// Repository: Manual
character.levelUp();
await characterRepo.save(character);
```

### Complex Queries
**Winner: Repository**
```typescript
// Repository: Natural
const rooms = await roomRepo.findByTypeAndLevel('dungeon', 5);

// Middleware: Awkward
const rooms = await persistence.adapter.query('rooms', {
  type: 'dungeon',
  level: 5
});
```

### Testing
**Winner: Repository**
```typescript
// Repository: Simple mock
const mockRepo = { save: jest.fn(), find: jest.fn() };
const generator = new RoomGenerator(mockRepo);

// Middleware: More setup
const generator = new RoomGenerator();
const middleware = new PersistenceMiddleware({ adapter: mockAdapter });
game.use(middleware);
```

### Minimal Code Changes
**Winner: Repository**
```typescript
// Before
const generator = new RoomGenerator();

// After with repository (one line change)
const generator = new RoomGenerator(roomRepo);

// After with middleware (requires event emissions)
class RoomGenerator {
  generateRoom() {
    // ... must add event emission
    this.emit('room.generated', room);
  }
}
```

## Recommended Implementation Strategy

### Phase 1: Repository Interfaces
```typescript
// core/storage/repositories/base.ts
interface Repository<T> {
  save(entity: T): Promise<void>;
  findById(id: string): Promise<T | null>;
  findAll(): Promise<T[]>;
  delete(id: string): Promise<void>;
}

// systems/rooms/repositories/RoomRepository.ts
interface RoomRepository extends Repository<Room> {
  findByType(type: RoomType): Promise<Room[]>;
  findConnected(roomId: string): Promise<Room[]>;
  findByTag(tag: string): Promise<Room[]>;
}
```

### Phase 2: Optional Repository Injection
```typescript
// Make repositories optional in constructors
class RoomGenerator {
  constructor(private config: GeneratorConfig, private roomRepo?: RoomRepository) {}
}

class CharacterManager {
  constructor(private characterRepo?: CharacterRepository) {}
}
```

### Phase 3: Storage Module Adapters
```typescript
// Storage module provides base implementations
class MongoRepository<T> implements Repository<T> {
  constructor(
    private adapter: MongoAdapter,
    private collection: string
  ) {}
  
  async save(entity: T): Promise<void> {
    await this.adapter.save(this.collection, entity.id, entity);
  }
}

// Specific implementations extend base
class MongoRoomRepository extends MongoRepository<Room> implements RoomRepository {
  async findByType(type: RoomType): Promise<Room[]> {
    return this.adapter.query(this.collection, { type });
  }
}
```

### Phase 4: Middleware Integration
```typescript
// Middleware can work alongside repositories
const persistence = new PersistenceMiddleware({
  adapter: mongoAdapter,
  repositories: {
    // Auto-inject repositories
    rooms: MongoRoomRepository,
    characters: MongoCharacterRepository
  },
  events: {
    // Still handle events for auto-save
    'room.generated': { collection: 'rooms', debounce: 1000 }
  }
});
```

## Migration Example: Room Generator

### Current Code
```typescript
class RoomGenerator {
  generateRoom(config: RoomConfig): Room {
    // Generate room logic
    return room;
  }
}
```

### Step 1: Add Optional Repository (Minimal Change)
```typescript
class RoomGenerator {
  constructor(private roomRepo?: RoomRepository) {}
  
  async generateRoom(config: RoomConfig): Promise<Room> {
    const room = this.createRoom(config);
    
    // Save if repository provided
    if (this.roomRepo) {
      await this.roomRepo.save(room);
    }
    
    return room;
  }
}
```

### Step 2: Add Events (When Needed)
```typescript
class RoomGenerator extends EventEmitter {
  async generateRoom(config: RoomConfig): Promise<Room> {
    const room = this.createRoom(config);
    
    if (this.roomRepo) {
      await this.roomRepo.save(room);
    }
    
    // Emit for middleware/monitoring
    this.emit('room.generated', room);
    
    return room;
  }
}
```

## Decision Matrix

| Criteria | Repository | Middleware | Hybrid |
|----------|------------|------------|---------|
| Minimal code changes | ⭐⭐⭐ | ⭐ | ⭐⭐ |
| Explicit control | ⭐⭐⭐ | ⭐ | ⭐⭐⭐ |
| Complex queries | ⭐⭐⭐ | ⭐ | ⭐⭐⭐ |
| Auto-persistence | ⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| Testing | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| Type safety | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| Centralized config | ⭐ | ⭐⭐⭐ | ⭐⭐⭐ |

## Recommendation

**Use the Hybrid Approach:**

1. **Start with repository injection** for immediate needs (like RoomGenerator)
2. **Add middleware later** for cross-cutting concerns (auto-save, audit logs)
3. **Keep both options available** for maximum flexibility

This gives you:
- Minimal changes to existing code
- Explicit control where needed
- Automatic persistence where helpful
- Full type safety
- Easy testing

## Example: Room Generator with Hybrid Approach

```typescript
// 1. Define repository interface
interface RoomRepository extends Repository<Room> {
  findByType(type: RoomType): Promise<Room[]>;
  findAdjacent(roomId: string): Promise<Room[]>;
}

// 2. Update RoomGenerator (minimal change)
class RoomGenerator extends EventEmitter {
  constructor(
    private config: GeneratorConfig,
    private roomRepo?: RoomRepository  // Optional repository
  ) {
    super();
  }
  
  async generateRoom(options: RoomOptions): Promise<Room> {
    const room = this.createRoom(options);
    
    // Direct save if repo available
    if (this.roomRepo) {
      await this.roomRepo.save(room);
    }
    
    // Emit event for middleware/monitoring
    this.emit('room.generated', room);
    
    return room;
  }
  
  // Complex queries require repository
  async findSimilarRooms(room: Room): Promise<Room[]> {
    if (!this.roomRepo) {
      return [];
    }
    return this.roomRepo.findByType(room.type);
  }
}

// 3. Usage without persistence
const generator = new RoomGenerator(config);
const room = generator.generateRoom({ type: 'cavern' });

// 4. Usage with repository
const generator = new RoomGenerator(config, mongoRoomRepo);
const room = await generator.generateRoom({ type: 'cavern' });

// 5. Usage with middleware (auto-injection)
game.use(persistenceMiddleware);
const generator = game.createRoomGenerator(); // Repo auto-injected
```

This approach gives you the best of both worlds!