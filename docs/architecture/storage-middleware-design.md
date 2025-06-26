# Storage Middleware Design for RPG Toolkit

## Overview

This document outlines the design for a middleware-based persistence system that leverages the storage module to provide automatic state persistence for the RPG toolkit. The system allows libraries to remain pure and persistence-agnostic while providing flexible, configurable persistence options to end users.

## Core Concepts

### 1. Storage Module
The storage module (`core/storage/`) provides abstract interfaces and concrete implementations for various data stores. It acts as the foundation for all persistence operations.

### 2. Middleware System
Middleware intercepts state changes and automatically persists them based on configuration. This approach keeps individual libraries simple while providing powerful persistence capabilities.

### 3. Separation of Concerns
- **Libraries**: Focus on game logic, emit events, maintain state
- **Storage Module**: Handles actual persistence operations
- **Middleware**: Bridges libraries and storage, handles when/what to persist
- **Users**: Configure persistence strategy, don't worry about implementation

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        User Application                      │
├─────────────────────────────────────────────────────────────┤
│                      Middleware Layer                        │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Persistence │  │   Caching    │  │   Logging    │      │
│  │ Middleware  │  │  Middleware  │  │  Middleware  │      │
│  └──────┬──────┘  └──────────────┘  └──────────────┘      │
├─────────┼───────────────────────────────────────────────────┤
│         │              Core Libraries                        │
│  ┌──────▼──────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Events    │  │ Character    │  │  Inventory   │      │
│  │   System    │  │   System     │  │   System     │      │
│  └──────┬──────┘  └──────────────┘  └──────────────┘      │
├─────────┼───────────────────────────────────────────────────┤
│         │              Storage Module                        │
│  ┌──────▼──────────────────────────────────────────┐       │
│  │            Storage Adapter Interface             │       │
│  └──────┬───────────┬───────────┬─────────────────┘       │
│         │           │           │                           │
│  ┌──────▼────┐ ┌───▼────┐ ┌───▼────┐ ┌──────────┐        │
│  │  MongoDB  │ │  File  │ │Memory │ │ PostgreSQL│        │
│  │  Adapter  │ │Adapter │ │Adapter│ │  Adapter  │        │
│  └───────────┘ └────────┘ └───────┘ └───────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Strategy

### Phase 1: Storage Module Foundation
```typescript
// core/storage/interfaces/StorageAdapter.ts
interface StorageAdapter {
  save<T>(collection: string, id: string, data: T): Promise<void>;
  load<T>(collection: string, id: string): Promise<T | null>;
  delete(collection: string, id: string): Promise<void>;
  query<T>(collection: string, query: Query): Promise<T[]>;
  transaction<T>(fn: () => Promise<T>): Promise<T>;
}

// core/storage/interfaces/Serializable.ts
interface Serializable {
  serialize(): object;
  deserialize(data: object): void;
}
```

### Phase 2: Middleware System
```typescript
// core/middleware/PersistenceMiddleware.ts
interface PersistenceConfig {
  adapter: StorageAdapter;
  collections: {
    [key: string]: {
      events: string[];      // Events that trigger saves
      debounce?: number;     // Debounce saves (ms)
      serialize?: (data: any) => any;
      deserialize?: (data: any) => any;
    }
  };
  autoSave?: boolean;
  saveInterval?: number;     // Auto-save interval (ms)
}

class PersistenceMiddleware {
  constructor(config: PersistenceConfig) {
    // Implementation
  }
  
  attach(eventBus: EventBus): void {
    // Subscribe to configured events
    // Handle persistence logic
  }
}
```

### Phase 3: Integration Example
```typescript
// Example usage in a game
import { Game } from '@rpg-toolkit/core';
import { PersistenceMiddleware } from '@rpg-toolkit/middleware';
import { MongoAdapter } from '@rpg-toolkit/storage-mongo';

const game = new Game();

// Configure persistence
const persistence = new PersistenceMiddleware({
  adapter: new MongoAdapter({ 
    url: 'mongodb://localhost:27017/mygame' 
  }),
  collections: {
    characters: {
      events: ['character.created', 'character.updated', 'character.levelUp'],
      debounce: 1000  // Save at most once per second
    },
    inventory: {
      events: ['inventory.itemAdded', 'inventory.itemRemoved'],
      debounce: 500
    },
    gameState: {
      events: ['game.stateChanged'],
      debounce: 5000
    }
  },
  autoSave: true,
  saveInterval: 60000  // Auto-save every minute
});

// Attach middleware
game.use(persistence);

// Libraries work normally, persistence happens automatically
const character = game.createCharacter({ name: 'Aragorn' });
character.levelUp();  // Automatically persisted after debounce
```

## Benefits

1. **Simple Libraries**: Libraries don't need to know about persistence
2. **Flexible Configuration**: Users choose what, when, and how to persist
3. **Multiple Backends**: Easy to switch between different storage solutions
4. **Performance**: Built-in debouncing and batching
5. **Testability**: Easy to mock storage in tests
6. **Extensibility**: Can add caching, versioning, migrations

## Configuration Examples

### Discord Bot Configuration
```typescript
const persistence = new PersistenceMiddleware({
  adapter: new MongoAdapter({ url: process.env.MONGO_URL }),
  collections: {
    guilds: {
      events: ['guild.settingsChanged'],
      debounce: 5000
    },
    characters: {
      events: ['character.*'],  // All character events
      debounce: 2000
    }
  }
});
```

### Web App Configuration
```typescript
const persistence = new PersistenceMiddleware({
  adapter: new BrowserAdapter({ 
    type: 'indexeddb',
    dbName: 'rpg-game'
  }),
  collections: {
    gameState: {
      events: ['*'],  // Persist everything
      debounce: 1000
    }
  },
  autoSave: true,
  saveInterval: 30000
});
```

### CLI Tool Configuration
```typescript
const persistence = new PersistenceMiddleware({
  adapter: new FileAdapter({ 
    directory: './game-data'
  }),
  collections: {
    campaign: {
      events: ['campaign.updated'],
      serialize: (data) => JSON.stringify(data, null, 2)
    }
  }
});
```

## Advanced Features

### 1. Transactions
```typescript
await game.transaction(async () => {
  character.addItem(sword);
  character.removeGold(100);
  // Both operations succeed or both fail
});
```

### 2. Migrations
```typescript
const persistence = new PersistenceMiddleware({
  adapter: adapter,
  migrations: [
    {
      version: 2,
      up: async (data) => {
        // Transform data to new schema
      }
    }
  ]
});
```

### 3. Caching Layer
```typescript
const cached = new CachingMiddleware({
  backend: persistence,
  ttl: 300000,  // 5 minutes
  maxSize: 1000
});
```

### 4. Selective Loading
```typescript
// Load only what's needed
const character = await game.load('characters', characterId, {
  fields: ['name', 'level', 'class']
});
```

## Security Considerations

1. **Validation**: Validate all data before persistence
2. **Sanitization**: Sanitize user input
3. **Access Control**: Implement proper access controls
4. **Encryption**: Support encryption for sensitive data
5. **Audit Trail**: Optional audit logging

## Performance Considerations

1. **Debouncing**: Prevent excessive saves
2. **Batching**: Batch multiple operations
3. **Lazy Loading**: Load data on demand
4. **Indexing**: Support indexes for queries
5. **Connection Pooling**: Efficient connection management

## Testing Strategy

```typescript
// Easy to test with in-memory adapter
const testGame = new Game();
testGame.use(new PersistenceMiddleware({
  adapter: new MemoryAdapter()
}));

// Test game logic without real persistence
```

## Migration Path

For existing projects:
1. Implement storage adapters for current data stores
2. Gradually add middleware to existing systems
3. Remove direct persistence calls from libraries
4. Standardize on middleware approach

## Open Questions for Team Discussion

1. Should we support schema validation at the middleware level?
2. How should we handle offline/online synchronization?
3. What's our strategy for data versioning and backwards compatibility?
4. Should middleware support data compression?
5. How do we handle large binary data (maps, images)?
6. Should we build a query language or use adapter-specific queries?

## Next Steps

1. Review and refine interfaces with team
2. Prototype basic storage adapter
3. Implement persistence middleware
4. Create example implementations
5. Write comprehensive tests
6. Document best practices

## Related Documents

- [Core Architecture](../core-architecture.md)
- [Event System Design](../event-system-design.md)
- [Security Guidelines](../security-guidelines.md)