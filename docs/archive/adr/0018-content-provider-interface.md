# ADR-0018: Content Integration Orchestrator

Date: 2025-01-21

## Status

Proposed

## Context

The orchestrators architecture (ADR-0017) requires content coordination across specialized content domains (monsters, items, quests, rewards, economics). However, content is too broad and complex to handle as a monolithic system. Each content domain has enough complexity to warrant its own specialized tool, similar to how we separated spatial, spawn, and selectables.

Current content domains requiring specialized tools:
- **Monsters/Creatures**: Combat stats, AI behavior, spawning patterns, encounter balancing
- **Items/Equipment**: Properties, enchantments, economy integration, treasure generation  
- **Quests**: Objectives, dependencies, narrative branching, completion tracking
- **Rewards**: Experience points, achievement systems, progression mechanics
- **Economics**: Currency, trade, market simulation, value calculations

The challenge is coordinating these specialized content tools while maintaining clean integration with game content sources (D&D 5e API, custom data) and orchestration systems (World Manager).

Current challenges:
1. **Content Integration**: How do games connect their content sources to our tools (spatial, spawn, selectables)?
2. **Minimal Normalization**: What's the minimum data structure our tools need to function?
3. **Generation vs Persistence**: How do games choose between procedural generation and precise placement?
4. **Seed-Based Recreation**: How do games recreate worlds deterministically without storing full state?
5. **Flexibility**: How do games handle different scenarios (full reset, partial reset, persistence)?

The challenge is defining lightweight interfaces that let games integrate any content source while keeping the toolkit focused on generation infrastructure rather than content management.

## Decision

We will create a **Content Integration Orchestrator** that coordinates specialized content tools and provides unified game integration patterns. This is NOT a content storage system - it's the coordination layer between game content sources and specialized toolkit domains.

### Content Architecture Overview

```
Game Content Sources → Content Integration → Specialized Content Tools → Experience Orchestrators
    (D&D 5e API,           Orchestrator         ↗ tools/monsters
     Custom Files,                              ↗ tools/items  
     Databases)                                 ↗ tools/quests
                                               ↗ tools/rewards
                                               ↗ tools/economics
                                                     ↓
                                               World Manager, etc.
```

### Specialized Content Tools

Each content domain gets its own specialized tool following toolkit patterns:

**tools/monsters** - Creature Management Infrastructure
- Combat statistics and ability systems
- AI behavior patterns and triggers  
- Spawning coordination with spawn engine
- Challenge rating calculations and encounter balancing
- Creature ecosystem relationships

**tools/items** - Equipment and Treasure Infrastructure  
- Item properties, enchantments, and durability systems
- Equipment slot management and compatibility
- Treasure generation and rarity distribution
- Economic integration with tools/economics
- Crafting patterns and item relationships

**tools/rewards** - Experience and Achievement Infrastructure
- Experience point distribution and scaling
- Achievement tracking and milestone systems  
- Progression mechanics and level calculations
- Reward allocation and balancing
- Integration with quest completion systems

**tools/economics** - Economic Simulation Infrastructure
- Currency systems and exchange mechanisms
- Market price fluctuations and supply/demand
- Trade route management between world locations
- Economic event systems (market crashes, embargos)
- Value calculation algorithms

**tools/quests** - Objective Management Infrastructure (already planned)
- Quest lifecycle and dependency management
- Cross-location quest chains and branching
- Dynamic objective generation based on world state
- NPC goal systems and motivations
- Completion tracking and reward integration

### Content Integration Orchestrator

The orchestrator coordinates between game content sources and specialized tools:

```go
// Content Integration Orchestrator
type ContentOrchestrator struct {
    // Game integration
    gameProviders map[string]ContentProvider  // Game-implemented content sources
    
    // Specialized tool coordination  
    monsterTool   *monsters.Manager
    itemTool      *items.Manager
    questTool     *quests.Manager
    rewardTool    *rewards.Manager
    economicsTool *economics.Manager
    
    eventBus      *events.Bus
}

// Unified content request handling
func (c *ContentOrchestrator) RequestContent(ctx context.Context, request ContentRequest) (*ContentResponse, error) {
    // Route to appropriate specialized tool based on content type
    switch request.Domain {
    case "monsters":
        return c.monsterTool.HandleRequest(ctx, request, c.gameProviders)
    case "items": 
        return c.itemTool.HandleRequest(ctx, request, c.gameProviders)
    case "quests":
        return c.questTool.HandleRequest(ctx, request, c.gameProviders)
    // ... etc
    }
}
```

### Game Integration Interface

Games implement simple content providers for their sources:

```go
// Games implement this for their content sources
type ContentProvider interface {
    GetName() string
    GetDomains() []string  // ["monsters", "items", "spells"]
    
    // Provide raw content for specific domain
    GetContent(ctx context.Context, domain string, criteria map[string]interface{}) ([]interface{}, error)
}

// Example: Game's D&D 5e provider
type GameDnD5eProvider struct {
    apiClient *dnd5e.Client
    cache     GameCache
}

func (p *GameDnD5eProvider) GetContent(ctx context.Context, domain string, criteria map[string]interface{}) ([]interface{}, error) {
    switch domain {
    case "monsters":
        return p.getMonsters(ctx, criteria)
    case "items":
        return p.getItems(ctx, criteria)
    case "spells":
        return p.getSpells(ctx, criteria) 
    }
}
```

### Core Philosophy

**Toolkit Provides:**
- Specialized content tools for each domain (monsters, items, quests, rewards, economics)
- Content integration orchestrator to coordinate between domains
- Game integration interface patterns
- Cross-domain event coordination

**Games Handle:**  
- Content acquisition from their sources (APIs, files, databases)
- Content transformation to domain-specific formats
- Caching and performance optimization
- Actual storage/persistence of game state

### Minimal Content Interface

```go
// Minimal interface our tools need - games implement this
type Entity interface {
    GetID() string
    GetType() string
    GetPosition() *Position      // For spatial placement
    GetChallenge() *float32      // For difficulty balancing (optional)
    GetProperties() map[string]interface{} // System-specific data
}

// Content provider interface - games implement for their sources
type ContentProvider interface {
    // Provide entities matching criteria for generative spawning
    GetEntities(ctx context.Context, criteria EntityCriteria) ([]Entity, error)
    
    // Provide specific entity by ID for direct placement
    GetEntity(ctx context.Context, id string) (Entity, error)
}

// Minimal criteria our tools use for content selection
type EntityCriteria struct {
    Types           []string    // ["monster", "npc", "item"]
    ChallengeRange  *Range      // Min/max difficulty
    Themes          []string    // ["undead", "treasure", "boss"] 
    Tags            []string    // Game-specific categorization
    Count           int         // How many needed
}
```


### Generation Patterns

**Toolkit provides both generative and direct methods:**

```go
// Generative spawning - uses patterns, seeds, content providers
type SpawnEngine interface {
    // Uses content provider for procedural placement
    PopulateArea(seed string, provider ContentProvider, config PopulationConfig) error
    
    // Direct placement for loading saved states
    PlaceEntity(entity Entity, position Position) error
    PlaceEntities(placements []EntityPlacement) error
}

// Similar pattern for all generation tools
type SpatialOrchestrator interface {
    // Seed-based layout generation
    GenerateRooms(seed string, config RoomConfig) (*RoomLayout, error)
    
    // Direct room construction
    CreateRoom(id string, bounds Bounds) (*Room, error)
}
```

### Seed-Based Recreation Patterns

**World Recreation Recipe:**
```yaml
# What games store - minimal generative recipe
world_recipe:
  seed: "abc123"
  locations:
    - id: "dungeon-entrance" 
      type: "dungeon_room"
      seed: "def456"
      content_criteria:
        types: ["monster", "treasure"]
        themes: ["undead", "horror"]
        challenge_range: {min: 1, max: 3}
  connections:
    - from: "dungeon-entrance"
      to: "main-hall"  
      type: "door"
      seed: "ghi789"

# Current game state (separate from recipe)
current_state:
  entities:
    - id: "goblin-1"
      position: {x: 5, y: 3}
      health: 8
    - id: "treasure-chest"
      position: {x: 9, y: 7}
      opened: true
```

**Three Recreation Scenarios:**

```go
// 1. Full random reset - new seeds, fresh everything
worldManager.CreateWorld(NewWorldConfig{
    GenerateNewSeeds: true,
    ContentProvider: gameContentProvider,
})

// 2. Partial reset - keep layout, regenerate content in specific rooms
worldManager.RecreateWorld(worldRecipe, RecreationOptions{
    ResetRooms: []string{"boss-chamber"}, // Only regenerate this room
    PreserveLayout: true,
})

// 3. Load saved state - use seeds for layout, direct placement for entities
world := worldManager.RecreateWorld(worldRecipe)
for _, entity := range currentState.Entities {
    world.PlaceEntity(entity, entity.Position)
}
```

### Storage Interface Patterns

```go
// Pattern for what games should store
type WorldRecipe struct {
    Seed        string             `json:"seed"`
    Locations   []LocationRecipe   `json:"locations"`  
    Connections []ConnectionRecipe `json:"connections"`
}

type LocationRecipe struct {
    ID       string           `json:"id"`
    Type     string           `json:"type"`
    Seed     string           `json:"seed"`
    Criteria EntityCriteria   `json:"content_criteria"` // What was supposed to spawn
}

// Games can extend with their own data
type GameWorldRecipe struct {
    WorldRecipe                    // Embed toolkit recipe
    QuestStates map[string]string  `json:"quest_states"`
    NPCRelations map[string]int    `json:"npc_relations"`
    // ... other game-specific data
}
```

### Integration Example

**How games integrate their D&D 5e content:**

```go
// Game implements content provider for D&D 5e API
type GameDnD5eProvider struct {
    apiClient *dnd5e.Client
    cache     GameCache  // Game handles caching
}

func (p *GameDnD5eProvider) GetEntities(ctx context.Context, criteria EntityCriteria) ([]Entity, error) {
    // 1. Game hits D&D 5e API with filters
    monsters, err := p.apiClient.GetMonsters(dnd5e.MonsterFilter{
        Challenge: criteria.ChallengeRange,
        Type:      criteria.Types,
    })
    
    // 2. Game transforms to toolkit interface
    entities := make([]Entity, len(monsters))
    for i, monster := range monsters {
        entities[i] = &GameMonster{
            id:         monster.Index,
            entityType: "monster",
            challenge:  &monster.ChallengeRating,
            properties: map[string]interface{}{
                "hit_points": monster.HitPoints,
                "armor_class": monster.ArmorClass,
                "dnd5e_data": monster, // Preserve original
            },
        }
    }
    return entities, nil
}

// Use with toolkit
spawnEngine := spawn.NewEngine(spawn.Config{
    SpatialManager: spatialManager,
    EventBus:      eventBus,
})

// Generative spawning
spawnEngine.PopulateArea("room-seed-123", gameContentProvider, spawn.PopulationConfig{
    Criteria: EntityCriteria{
        Types: ["monster"],
        ChallengeRange: &Range{Min: 1, Max: 5},
        Themes: ["undead"],
    },
    Density: "medium",
})

// Or direct placement for loading saves
spawnEngine.PlaceEntity(savedGoblin, Position{X: 5, Y: 3})
```

## Consequences

### Positive

- **True Toolkit Philosophy**: We provide patterns, games handle implementation
- **Maximum Flexibility**: Games choose their content sources and storage strategies  
- **Lightweight**: Minimal interface requirements, no toolkit content management overhead
- **Deterministic**: Seed-based generation enables reliable recreation
- **Choice of Control**: Games choose between generative convenience and precise control

### Negative  

- **Integration Burden**: Games must implement content provider interfaces
- **No Built-in Caching**: Games responsible for performance optimizations
- **Documentation Need**: Clear examples needed for integration patterns

### Neutral

- **Game-Specific Storage**: Games design their own persistence strategies
- **Content Source Agnostic**: Works with any content source games choose

## Example

### Complete Integration Flow

```go
// Game creates content provider
contentProvider := &MyGameContentProvider{
    dnd5eAPI: dnd5e.NewClient(apiKey),
    customContent: loadCustomContent("./monsters/"),
    cache: newGameCache(),
}

// Game creates world recipe
recipe := WorldRecipe{
    Seed: "my-campaign-world",
    Locations: []LocationRecipe{{
        ID:   "starter-dungeon",
        Type: "dungeon", 
        Seed: "dungeon-123",
        Criteria: EntityCriteria{
            Types: ["monster", "treasure"],
            ChallengeRange: &Range{Min: 1, Max: 3},
            Themes: ["undead", "horror"],
        },
    }},
}

// Generate fresh world
world := worldManager.CreateWorldFromRecipe(recipe, contentProvider)

// Later: save minimal recipe + current positions
gameState := GameState{
    Recipe: recipe,
    EntityPositions: world.GetAllEntityPositions(),
    QuestStates: getCurrentQuests(),
}
saveGameState(gameState)

// Later: load game
loadedState := loadGameState()
world = worldManager.RecreateWorld(loadedState.Recipe, contentProvider)
world.PlaceEntities(loadedState.EntityPositions)
restoreQuests(loadedState.QuestStates)
```

This approach keeps the toolkit focused on generation infrastructure while giving games complete control over their content sources and storage strategies.