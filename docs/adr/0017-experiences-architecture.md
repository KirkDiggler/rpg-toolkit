# ADR-0017: World Manager Architecture for Complete Game World Orchestration

Date: 2025-01-20

## Status

Proposed

## Context

The RPG Toolkit has evolved to include numerous foundational tools (spatial, spawn, selectables, environments) and mechanics (conditions, effects, features, etc.). While these tools provide excellent infrastructure for individual responsibilities, there is a need for higher-level orchestration that can create and manage complete game worlds.

The immediate driver is a comprehensive "world building and management experience" that would orchestrate:
- Content management (monsters, items, spells from various sources like D&D 5e APIs)
- Multiple location types (towns, dungeons, forests, inns, caves, meadows, etc.)
- Cross-location entity management (NPCs, monsters, items tracked across locations)
- Spatial coordination across multiple connected environments
- Dynamic world state management (population shifts, seasonal changes, events)
- Persistent world data (save/load complete worlds with entity tracking)

Currently, there is no architectural pattern for systems that need to coordinate multiple tools across an entire game world. Individual tools remain focused on single responsibilities, but world-building requires orchestration across multiple domains, locations, and time periods.

The challenge is maintaining the toolkit's core philosophy of "infrastructure, not implementation" while providing higher-level abstractions that can deliver complete world-building experiences.

## Decision

We will introduce an `orchestrators/` module hierarchy that sits above the existing `tools/` and `mechanics/` modules, with a comprehensive World Manager as the primary orchestrator. The World Manager addresses several critical design challenges:

### Design Challenges Addressed

1. **Module Hierarchy Problem**: Where do higher-level orchestrators belong in the toolkit architecture?
2. **Content Normalization Challenge**: How to handle multiple game systems (D&D 5e, Pathfinder, custom content) seamlessly?
3. **Configuration Complexity Balance**: How to serve both simple users (presets) and advanced users (full control)?
4. **Tool Communication Pattern**: Direct coupling vs. loose event-driven coordination?
5. **Cross-Location Entity Management**: How to track entities that move between world locations?
6. **World State Persistence**: How to save/load complete worlds with all entity states?
7. **Error Handling Strategy**: How to gracefully handle failures across multiple coordinated systems?

### Module Structure

```
rpg-toolkit/
├── core/                    # Foundational interfaces  
├── mechanics/               # Game mechanics (single responsibility)
├── tools/                   # Infrastructure tools (single responsibility)
├── content/                 # Content adapters & normalization (see ADR-0018)
└── orchestrators/           # Higher-level orchestrators (new)
    └── worlds/             # World management and building orchestration
        ├── manager.go      # Core WorldManager orchestrator
        ├── locations.go    # Location registry and management
        ├── entities.go     # Cross-location entity tracking
        ├── persistence.go  # World save/load functionality
        └── events.go       # World-specific event definitions
```

### World Manager Architecture

The World Manager is the primary experience orchestrator that manages complete game worlds:

```go
// Core World Manager - orchestrates entire game worlds
type WorldManager struct {
    // Infrastructure dependencies
    contentRegistry    *content.Registry       // Multi-source content (see ADR-0018)
    eventBus          *events.Bus             // World-wide event coordination
    
    // World management
    locationRegistry   *LocationRegistry      // All world locations
    entityTracker      *CrossLocationTracker  // Entities across locations
    worldState         *PersistentWorldState  // Save/load world data
    
    // Orchestration
    validator         *WorldConfigValidator   // Configuration validation
    orchestrationID   string                  // Current operation tracking
}

// Individual locations within the world
type Location struct {
    ID              string                   // Unique location identifier
    Type            LocationType             // "town", "dungeon", "forest", "inn", etc.
    
    // Tool coordination for this location
    SpatialManager  *spatial.Orchestrator    // Location's spatial management
    SpawnEngine     *spawn.Engine           // Entity spawning within location
    Environment     *environments.Generator  // Location's environment generation
    
    // Location-specific data
    Population      *PopulationManager       // Current entities in location
    Connections     []LocationConnection     // Travel routes to other locations
    State           *LocationState           // Persistent location data
}
```

### Content Architecture

A new `tools/content` module will handle multi-source content integration as described in **ADR-0018: Content Provider Interface Architecture**:

```
tools/content/
├── adapters/               # Source-specific adapters
│   ├── dnd5e/             # D&D 5e API integration
│   ├── pathfinder/        # Pathfinder system integration  
│   └── custom/            # User-defined content
├── schemas/               # Normalized content types
└── registry.go           # Content provider management
```

#### Content Normalization Design

Based on analysis of D&D 5e API patterns, we will use normalized schemas that can handle multiple game systems while preserving system-specific data:

```go
// Universal base for all content
type ContentBase struct {
    ID          string            `json:"id"`           // Normalized identifier  
    Name        string            `json:"name"`         // Display name
    Source      string            `json:"source"`       // "dnd5e_api", "pathfinder", "custom"
    Version     string            `json:"version"`      // Content version/timestamp
    SystemData  map[string]any    `json:"system_data"`  // Original system-specific data
}

// Universal reference pattern (damage types, schools, classes, etc.)
type Reference struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Type string `json:"type"`
}

// Normalized monster schema
type Monster struct {
    ContentBase
    
    // Basic Properties
    Size        string   `json:"size"`         // "small", "medium", "large", etc.
    Type        string   `json:"type"`         // "humanoid", "dragon", "undead"
    Subtype     string   `json:"subtype"`      // "goblinoid", "shapechanger", etc.
    Challenge   float32  `json:"challenge"`    // Universal difficulty rating
    
    // Combat Stats (normalized across systems)
    HitPoints   int              `json:"hit_points"`
    ArmorClass  int              `json:"armor_class"`
    Speed       map[string]int   `json:"speed"`       // {"walk": 30, "fly": 60}
    
    // Abilities and Actions
    Abilities   []Ability        `json:"abilities"`
    Actions     []Action         `json:"actions"`
    
    // Resistances/Immunities
    DamageResistances   []string     `json:"damage_resistances"`
    DamageImmunities    []string     `json:"damage_immunities"`
    ConditionImmunities []Reference  `json:"condition_immunities"`
    
    // Theming/Selection (critical for dungeon generation)
    Themes      []string `json:"themes"`       // ["undead", "horror", "boss"]
    Environment []string `json:"environment"`  // ["dungeon", "forest", "urban"]
    Tags        []string `json:"tags"`         // Custom categorization
}

// Normalized equipment schema
type Equipment struct {
    ContentBase
    
    Category    string   `json:"category"`     // "weapon", "armor", "tool", "treasure"
    Subcategory string   `json:"subcategory"`  // "martial_melee", "light_armor", etc.
    Rarity      string   `json:"rarity"`       // "common", "uncommon", "rare", etc.
    
    // Economic
    Value       *Cost    `json:"value,omitempty"`
    Weight      float32  `json:"weight"`
    
    // Combat Properties
    Damage      *Damage     `json:"damage,omitempty"`
    ArmorClass  *ArmorClass `json:"armor_class,omitempty"`
    Properties  []Reference `json:"properties,omitempty"`
    
    // Magical Properties
    MagicalEffects []Effect `json:"magical_effects,omitempty"`
    RequiresAttunement bool `json:"requires_attunement"`
    
    // Theming
    Themes      []string `json:"themes"`       // ["magical", "mundane", "cursed"]
    Tags        []string `json:"tags"`
}
```

**Key Design Principles:**

1. **System Agnostic**: Core fields work across D&D 5e, Pathfinder, custom systems
2. **Preserve Specificity**: `SystemData` maintains original API data for system-specific needs
3. **Theme-Driven Selection**: `Themes` and `Environment` arrays enable content filtering
4. **Universal References**: Common pattern for cross-references (damage types, schools, etc.)
5. **Extensible**: New game systems can map to existing schemas or extend them

### World Configuration Schema

The World Manager uses layered configuration complexity to serve both simple and advanced users:

```go
// Complete world configuration - addresses Configuration Complexity Balance challenge
type WorldConfig struct {
    // High-level world identity
    Name        string            `yaml:"name"`
    Preset      string            `yaml:"preset,omitempty"`     // "medieval_village", "underground_complex"
    Theme       string            `yaml:"theme"`                // "medieval", "sci_fi", "horror", "custom"
    Scale       string            `yaml:"scale"`                // "hamlet", "village", "town", "city", "region"
    
    // Content sources and filtering - addresses Content Normalization Challenge
    Content     ContentConfig     `yaml:"content"`
    
    // World structure and locations
    Locations   []LocationConfig  `yaml:"locations"`
    Connections []ConnectionConfig `yaml:"connections"`
    
    // World-wide systems
    Population  PopulationConfig  `yaml:"population"`
    Economics   EconomicsConfig   `yaml:"economics,omitempty"`
    Events      WorldEventsConfig `yaml:"events,omitempty"`
    
    // Persistence and state management
    Persistence PersistenceConfig `yaml:"persistence,omitempty"`
    
    // Advanced options
    Advanced    AdvancedConfig    `yaml:"advanced,omitempty"`
}

// Individual location configuration within the world
type LocationConfig struct {
    ID          string            `yaml:"id"`
    Type        LocationType      `yaml:"type"`              // "town", "dungeon", "forest", "inn"
    Name        string            `yaml:"name"`
    Preset      string            `yaml:"preset,omitempty"`  // "tavern", "blacksmith", "cave_system"
    
    // Location-specific content and spawning
    Content     ContentConfig     `yaml:"content"`           // Can override world content
    Spatial     SpatialConfig     `yaml:"spatial"`
    Spawning    SpawningConfig    `yaml:"spawning"`
    Environment EnvironmentConfig `yaml:"environment"`
    
    // Location properties
    Population  LocationPopulationConfig `yaml:"population"`
    Services    []ServiceConfig          `yaml:"services,omitempty"`  // Shops, inns, etc.
}

// Cross-location entity management configuration
type PopulationConfig struct {
    // World-wide population settings
    TotalNPCs        int               `yaml:"total_npcs"`
    MobilityRate     float32           `yaml:"mobility_rate"`      // How often entities move
    
    // Population distribution
    DistributionType string            `yaml:"distribution"`       // "even", "urban_focused", "custom"
    
    // Entity lifecycle
    Respawn          RespawnConfig     `yaml:"respawn"`
    Migration        MigrationConfig   `yaml:"migration"`
}

type ContentConfig struct {
    // Content source prioritization
    Sources     []ContentSource   `yaml:"sources"`
    
    // Global content filters (empty/omitted = "all available content")
    MonsterTypes    []string      `yaml:"monster_types,omitempty"`    // ["undead", "fiend"] or [] for all
    EquipmentTypes  []string      `yaml:"equipment_types,omitempty"`  // ["weapon", "armor"] or [] for all
    Themes          []string      `yaml:"themes,omitempty"`           // ["magical", "cursed"] or [] for all
    
    // Challenge/level constraints
    ChallengeRange  *ChallengeRange `yaml:"challenge_range,omitempty"`
    ItemRarity      []string        `yaml:"item_rarity,omitempty"`    // ["common", "uncommon"] or [] for all
}

type SpatialConfig struct {
    // Room generation
    RoomCount    *RoomCount        `yaml:"room_count"`
    Layout       string            `yaml:"layout"`           // "tower", "branching", "grid", "organic"
    GridType     string            `yaml:"grid_type"`        // "square", "hex", "gridless"
    
    // Room properties
    RoomSizes    *RoomSizeConfig   `yaml:"room_sizes,omitempty"`
    Connections  *ConnectionConfig `yaml:"connections,omitempty"`
    
    // Environmental features
    Environment  EnvironmentConfig `yaml:"environment"`
}

type SpawningConfig struct {
    // Density controls
    MonsterDensity   string           `yaml:"monster_density"`    // "sparse", "medium", "dense"
    EliteFrequency   string           `yaml:"elite_frequency"`    // "rare", "occasional", "common"
    BossRooms        int              `yaml:"boss_rooms"`         // Number of boss encounters
    
    // Spawn patterns and constraints
    Patterns         []SpawnPattern   `yaml:"patterns,omitempty"`
    Constraints      []string         `yaml:"constraints,omitempty"` // ["no_flying_in_small_rooms"]
    
    // Monster themes (empty = all available monsters)
    MonsterThemes    []string         `yaml:"monster_themes,omitempty"` // ["undead", "horror"] or [] for all
}

type LootConfig struct {
    // Loot frequency and distribution
    Frequency        string           `yaml:"frequency"`          // "scarce", "standard", "abundant"
    Quality          string           `yaml:"quality"`            // "mundane", "mixed", "magical"
    
    // Treasure placement
    TreasureRooms    int              `yaml:"treasure_rooms"`     // Dedicated treasure rooms
    HiddenLoot       string           `yaml:"hidden_loot"`        // "none", "some", "lots"
    
    // Item themes and types (empty = all available items)
    ItemThemes       []string         `yaml:"item_themes,omitempty"`     // ["magical", "practical"] or [] for all
    PreferredTypes   []string         `yaml:"preferred_types,omitempty"` // ["weapon", "consumable"] or [] for all
}
```

**Configuration Approaches:**

1. **Simple (Preset-based)**: `preset: "horror_dungeon"` with minimal overrides
2. **Intermediate (Theme-based)**: `theme: "nature"` with density/difficulty controls  
3. **Advanced (Full Control)**: Detailed specification of every aspect
4. **"All Content" Support**: Empty filter arrays = use all available content from sources

**Validation and Defaults:**
- Presets provide complete working configurations
- Theme-based configs apply intelligent defaults
- Validation catches incompatible combinations (e.g., fire monsters + underwater theme)
- Missing fields auto-filled based on difficulty and theme

### Orchestrator Architecture

```go
// orchestrators/dungeons
type DungeonOrchestrator struct {
    contentRegistry  *content.Registry       // Multi-source content (see ADR-0018)
    spatialManager   *spatial.Orchestrator   // Room management  
    spawnEngine      *spawn.Engine           // Entity placement
    envGenerator     *environments.Generator // Procedural generation
    eventBus         *events.Bus             // Coordination
    validator        *ConfigValidator        // Configuration validation
}

func (d *DungeonExperience) GenerateDungeon(config DungeonConfig) (*Dungeon, error) {
    // 1. Validate and apply defaults to configuration
    if err := d.validator.ValidateAndApplyDefaults(&config); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    // 2. Orchestrate tools through event-driven coordination
    return d.orchestrateGeneration(config)
}
```

### Tool Integration Through Events

The World Manager orchestrates tools through the event bus rather than direct method calls, addressing the Tool Communication Pattern challenge:

```go
// World Manager publishes orchestration events that tools subscribe to
func (w *WorldManager) orchestrateWorldCreation(config WorldConfig) (*World, error) {
    w.orchestrationID = generateOrchestrationID()
    
    // Phase 1: Request content loading for entire world
    contentEvent := events.NewGameEvent("orchestrator.world.content_load_requested", w, nil).
        WithContext("orchestration_id", w.orchestrationID).
        WithContext("config", config.Content).
        WithContext("location_types", w.extractLocationTypes(config))
    
    w.eventBus.Publish(ctx, contentEvent)
    
    // State management handles phase transitions through event listeners
    return w.waitForWorldCreationCompletion()
}

// Tools respond to world orchestration events independently
func (registry *ContentRegistry) setupWorldHandlers() {
    registry.eventBus.SubscribeFunc("orchestrator.world.content_load_requested", 0,
        func(ctx context.Context, event events.Event) error {
            // Load content for all location types and publish completion event
            return registry.handleWorldContentLoadRequest(ctx, event)
        })
}

// Cross-location entity management through events
func (w *WorldManager) MoveEntity(entityID, fromLocationID, toLocationID string) error {
    moveEvent := events.NewGameEvent("orchestrator.world.entity_move_requested", w, nil).
        WithContext("entity_id", entityID).
        WithContext("from_location", fromLocationID).
        WithContext("to_location", toLocationID).
        WithContext("world_id", w.worldState.ID)
    
    return w.eventBus.Publish(context.Background(), moveEvent)
}
```

### Event-Driven World Orchestration Flow

**World Creation Phases:**
1. **Content Loading**: World Manager → Content tool → Available content for all location types
2. **Location Generation**: World Manager → Spatial/Environment tools → Individual location creation
3. **Connection Establishment**: World Manager → Spatial tools → Inter-location travel routes
4. **Population Distribution**: World Manager → Spawn tools → Entity placement across locations
5. **World State Initialization**: World Manager → Persistence layer → Save initial world state

**Ongoing World Management:**
1. **Entity Movement**: Cross-location entity tracking and migration
2. **Dynamic Events**: Seasonal changes, festivals, trade caravans, monster migrations
3. **Population Changes**: NPC lifecycle, respawning, demographic shifts
4. **Economic Systems**: Trade route updates, resource flow between locations
5. **Persistence Operations**: Auto-save, manual save, world state snapshots

**Event Patterns:**
- **World Request Events**: `orchestrator.world.{operation}_requested` (World Manager → Tools)
- **Location Request Events**: `orchestrator.location.{operation}_requested` (World Manager → Tools)
- **Completion Events**: `orchestrator.world.{operation}_completed` (Tools → World Manager)
- **Cross-Location Events**: `orchestrator.world.entity_moved`, `orchestrator.world.connection_used`
- **World State Events**: `orchestrator.world.saved`, `orchestrator.world.loaded`
- **Error Events**: `orchestrator.world.{operation}_error` (Tools → World Manager)
- **Fallback Events**: `orchestrator.world.fallback_triggered` (World Manager → Tools)

### Error Handling and Fallbacks

Tools publish error events when operations fail, triggering graceful fallbacks that address the Error Handling Strategy challenge:

```go
func (w *WorldManager) handleWorldOperationError(ctx context.Context, event events.Event) error {
    operation, _ := event.Context().GetString("failed_operation")
    locationID, _ := event.Context().GetString("location_id")
    
    // Create simplified fallback configuration
    fallbackConfig := w.createWorldFallbackConfig(operation, locationID)
    
    // Publish fallback event
    fallbackEvent := events.NewGameEvent("orchestrator.world.fallback_triggered", w, nil).
        WithContext("orchestration_id", w.orchestrationID).
        WithContext("failed_operation", operation).
        WithContext("location_id", locationID).
        WithContext("fallback_config", fallbackConfig)
    
    return w.eventBus.Publish(ctx, fallbackEvent)
}

func (w *WorldManager) createWorldFallbackConfig(operation, locationID string) interface{} {
    switch operation {
    case "location_generation":
        // Fallback to simpler location template
        return FallbackLocationConfig{
            UseSimpleTemplate: true,
            ReduceComplexity: true,
            LocationID: locationID,
        }
    case "cross_location_entity_movement":
        // Fallback to teleportation without path validation
        return FallbackMovementConfig{
            AllowTeleport: true,
            SkipPathValidation: true,
        }
    case "world_persistence":
        // Fallback to memory-only state with warning
        return FallbackPersistenceConfig{
            UseMemoryOnly: true,
            WarnUser: true,
        }
    default:
        return nil
    }
}
```

**World-Scale Fallback Strategies:**
- **Content Loading Failure**: Use preset content libraries, reduce location diversity
- **Location Generation Failure**: Fall back to simpler templates, reduce location count
- **Cross-Location Movement Failure**: Allow teleportation, disable complex pathfinding
- **Population Management Failure**: Use static populations, disable migration
- **Persistence Failure**: Continue with memory-only state, warn about data loss
- **Partial Location Failures**: Continue with successfully generated locations, mark failed ones for retry

**Cross-Location Error Recovery:**
- **Entity Tracking Loss**: Rebuild entity registry from location data
- **Connection Failure**: Allow direct location access, bypass travel systems
- **World State Corruption**: Roll back to last known good state, replay operations

This event-driven architecture ensures tools remain independent while enabling sophisticated orchestration of complex world management workflows, with comprehensive error recovery across multiple locations and systems.
```

## Consequences

### Positive

- **Clear Separation of Concerns**: Tools remain focused, experiences handle orchestration
- **Maintains Toolkit Philosophy**: Infrastructure vs implementation boundary preserved
- **Extensible Pattern**: Future experiences (encounters, campaigns) follow same pattern
- **User-Friendly**: Complete use cases rather than requiring users to orchestrate tools
- **Content Flexibility**: Support for multiple game systems and custom content
- **Configuration-Driven**: Declarative approach for non-technical users

### Negative

- **Additional Complexity**: New module hierarchy to understand and maintain
- **Potential Coupling**: Experiences depend on multiple tools, increasing coordination complexity  
- **Module Boundaries**: Need to carefully define what belongs in experiences vs tools
- **Content Normalization Overhead**: Additional layer for content transformation

### Neutral

- **New Module Dependencies**: Experiences will have broader dependency graphs than tools
- **Testing Complexity**: Integration testing across multiple tool boundaries
- **Documentation Scope**: Need to document both tool-level and experience-level usage

## Example

### World Configuration Examples

**Simple (Preset-Based World):**
```yaml
name: "Greenvale Village"
preset: "medieval_village"
scale: "village"

content:
  sources:
    - name: "dnd5e_api"
# No filters = use all available content across all locations
```

**Intermediate (Multi-Location World):**
```yaml
name: "Riverside Trading Post"
theme: "medieval"
scale: "town"

content:
  sources:
    - name: "dnd5e_api"
    - name: "my_custom_npcs"
  monster_types: ["humanoid", "beast"]     # Peaceful area
  equipment_types: []                      # All equipment types

locations:
  - id: "town_center"
    type: "town"
    name: "Riverside Center"
    preset: "trading_hub"
    
  - id: "the_prancing_pony"
    type: "inn"
    name: "The Prancing Pony"
    services: ["lodging", "food", "rumors"]
    
  - id: "nearby_forest"
    type: "forest"
    name: "Whispering Woods"
    spawning:
      monster_themes: ["beast", "nature"]
      monster_density: "sparse"

connections:
  - from: "town_center"
    to: "the_prancing_pony"
    type: "street"
    travel_time: "5_minutes"
  - from: "town_center" 
    to: "nearby_forest"
    type: "path"
    travel_time: "30_minutes"

population:
  total_npcs: 50
  mobility_rate: 0.1
  distribution: "urban_focused"
```

**Advanced (Complete World with Cross-Location Systems):**
```yaml
name: "The Northern Reaches"
theme: "medieval"
scale: "region"

content:
  sources:
    - name: "dnd5e_api"
      weight: 1.0
    - name: "pathfinder_api"
      weight: 0.3
    - name: "custom_northern_content"
      weight: 0.5

locations:
  - id: "ironhold_city"
    type: "city"
    name: "Ironhold"
    population:
      count: 5000
      demographics: ["human", "dwarf", "halfling"]
    services: ["market", "temple", "guild_hall"]
    content:
      monster_types: ["humanoid"]  # Override: city = safe
      
  - id: "deeprock_mines"
    type: "dungeon" 
    name: "Deeprock Mining Complex"
    spatial:
      layout: "branching"
      room_count: 15
    spawning:
      monster_themes: ["underground", "construct"]
      monster_density: "dense"
      
  - id: "whispering_caverns"
    type: "cave_system"
    name: "Whispering Caverns"
    environment:
      features: ["underground_river", "crystal_formations"]
    spawning:
      monster_themes: ["aberration", "underdark"]

connections:
  - from: "ironhold_city"
    to: "deeprock_mines"
    type: "mountain_path"
    travel_time: "2_hours"
    requirements: ["climbing_gear"]
    
  - from: "deeprock_mines"
    to: "whispering_caverns"
    type: "underground_tunnel"
    travel_time: "45_minutes"
    hidden: true

population:
  total_npcs: 500
  mobility_rate: 0.05
  distribution: "custom"
  migration:
    enabled: true
    trade_routes: ["ironhold_city"]
    seasonal_workers: ["deeprock_mines"]

events:
  enabled: true
  types: ["seasonal_festivals", "trade_caravans", "monster_migrations"]
  frequency: "weekly"

persistence:
  auto_save: true
  save_interval: "10_minutes"
  track_entity_history: true
```

### World Manager Usage
```go
// Create content registry (see ADR-0018 for details)
contentRegistry := content.NewRegistry(content.RegistryConfig{
    EventBus: eventBus,
})
contentRegistry.RegisterProvider("dnd5e", dnd5e.NewAPIProvider(apiConfig))
contentRegistry.RegisterProvider("custom", custom.NewFileProvider("./content/"))

// Create world manager
worldManager := orchestrators.NewWorldManager(WorldManagerConfig{
    ContentRegistry: contentRegistry,
    EventBus:        eventBus,
    PersistenceLayer: persistence.NewFileStore("./worlds/"),
})

// Generate complete world
world, err := worldManager.CreateWorld(config)
if err != nil {
    return fmt.Errorf("failed to create world: %w", err)
}

// World includes:
// - Multiple connected locations (towns, dungeons, forests, etc.)
// - Cross-location entity tracking and population management
// - Persistent world state with save/load capabilities
// - Dynamic event system for world changes
// - All coordinated through event-driven orchestration

// Manage ongoing world operations
worldManager.MoveEntity("npc-trader-1", "town_center", "the_prancing_pony")
worldManager.TriggerWorldEvent("seasonal_festival", "town_center")
worldManager.SaveWorld("my_campaign_world")
```

## Implementation Phases

Given the comprehensive scope of the World Manager architecture, implementation will be broken into manageable phases that build incrementally while delivering value at each stage.

### Phase 1: Single Location Experience (Foundation)

**Goal:** Establish basic experience orchestration with one location to prove the architecture and event patterns.

**Scope:**
- Create `orchestrators/worlds/` module structure
- Implement basic `WorldManager` with single location support
- Establish event-driven tool coordination patterns
- Build integration with existing tools (spatial, spawn, selectables, content per ADR-0018)
- Create simplified configuration schema for single locations
- Implement basic error handling and fallback strategies

**Key Components:**
```go
// Phase 1 simplified architecture
type WorldManager struct {
    contentRegistry *content.Registry
    eventBus       *events.Bus
    validator      *ConfigValidator
    currentWorld   *SingleLocationWorld
}

type SingleLocationWorld struct {
    Location        *Location
    SpatialManager  *spatial.Orchestrator
    SpawnEngine     *spawn.Engine
    Environment     *environments.Generator
}
```

**Configuration Example (Phase 1):**
```yaml
# Simplified single-location config
name: "Test Dungeon"
type: "dungeon"
preset: "horror_dungeon"

content:
  sources:
    - name: "dnd5e_api"
  monster_types: ["undead", "fiend"]

spatial:
  room_count: 5
  layout: "branching"

spawning:
  monster_density: "medium"
```

**Event Patterns (Phase 1):**
- `orchestrator.location.content_load_requested`
- `orchestrator.location.generation_requested`
- `orchestrator.location.spawning_requested`
- `orchestrator.location.{operation}_completed`
- `orchestrator.location.{operation}_error`

**Deliverables:**
- Working `WorldManager` that can create and populate a single location
- Event orchestration patterns established and documented
- Configuration validation and defaults system
- Integration tests with spatial, spawn, selectables, content tools
- Error handling and fallback mechanisms
- Phase 1 documentation and examples

**Success Criteria:** 
- Can generate a complete single location (dungeon, town, or forest) with populated entities using declarative YAML configuration
- All existing tools integrate through event bus without direct coupling
- Error scenarios trigger appropriate fallbacks
- Configuration validation catches common mistakes with helpful error messages

**Dependencies:** Existing tools (spatial, spawn, selectables), ADR-0018 content providers

---

### Phase 2: Multi-Location World (Expansion)

**Goal:** Add support for multiple connected locations to create true "worlds" rather than single locations.

**Scope:**
- Implement `LocationRegistry` for managing multiple locations
- Create location connection and travel route systems
- Expand configuration schema to support multiple locations
- Develop cross-location event coordination
- Enhanced error handling for partial location failures
- Location-specific content and spawning overrides

**Key Components:**
```go
// Phase 2 expanded architecture
type WorldManager struct {
    contentRegistry   *content.Registry
    locationRegistry  *LocationRegistry
    connectionManager *ConnectionManager
    eventBus         *events.Bus
    validator        *WorldConfigValidator
    currentWorld     *MultiLocationWorld
}

type LocationRegistry struct {
    locations   map[string]*Location
    connections []LocationConnection
}

type LocationConnection struct {
    FromLocationID string
    ToLocationID   string
    ConnectionType string // "door", "path", "portal", etc.
    TravelTime     string
    Requirements   []string // "climbing_gear", "key", etc.
}
```

**Configuration Example (Phase 2):**
```yaml
# Multi-location world config
name: "Riverside Trading Post"
theme: "medieval"
scale: "town"

content:
  sources:
    - name: "dnd5e_api"

locations:
  - id: "town_center"
    type: "town"
    name: "Riverside Center"
    preset: "trading_hub"
    
  - id: "the_prancing_pony"
    type: "inn"
    name: "The Prancing Pony"
    services: ["lodging", "food", "rumors"]
    
  - id: "nearby_forest"
    type: "forest"
    name: "Whispering Woods"
    content:
      monster_types: ["beast", "fey"]  # Override world content
    spawning:
      monster_density: "sparse"

connections:
  - from: "town_center"
    to: "the_prancing_pony"
    type: "street"
    travel_time: "5_minutes"
  - from: "town_center" 
    to: "nearby_forest"
    type: "path"
    travel_time: "30_minutes"
```

**Event Patterns (Phase 2):**
- `orchestrator.world.location_registry_updated`
- `orchestrator.world.connection_established`
- `orchestrator.world.location_generation_requested`
- `orchestrator.world.multi_location_coordination`
- Cross-location event forwarding and aggregation

**Deliverables:**
- Support for 3+ different location types in single world
- Location connection and travel route system
- Multi-location configuration validation
- Cross-location event coordination patterns
- Location-specific content override capabilities
- Enhanced error handling for partial failures

**Success Criteria:** 
- Can create worlds with multiple connected locations of different types
- Travel routes work between all connected locations
- Location-specific content overrides function correctly
- Partial location failures don't break entire world generation
- Event coordination works across multiple location boundaries

**Dependencies:** Phase 1 completion

---

### Phase 3: Cross-Location Entity Management

**Goal:** Track and manage entities moving between locations to create dynamic, living worlds.

**Scope:**
- Implement `CrossLocationTracker` for entity management across locations
- Create entity movement events and coordination systems
- Build population distribution and demographic management
- Develop entity migration and mobility systems
- World-wide population configuration and balancing
- Entity lifecycle management (spawning, movement, despawning)

**Key Components:**
```go
// Phase 3 entity management
type CrossLocationTracker struct {
    entityLocations map[string]string // entityID -> locationID
    entityStates   map[string]*EntityState
    migrationRules *MigrationRuleSet
    eventBus       *events.Bus
}

type EntityState struct {
    Entity      core.Entity
    LocationID  string
    LastMoved   time.Time
    Destination string
    MovementPlan *MovementPlan
}

type PopulationManager struct {
    worldPopulation   int
    locationTargets   map[string]int // locationID -> target population
    migrationEnabled  bool
    mobilityRate      float32
}
```

**Configuration Example (Phase 3):**
```yaml
# Cross-location entity management config
name: "Living Valley"
scale: "region"

population:
  total_npcs: 200
  mobility_rate: 0.15           # 15% of entities move per time period
  distribution: "urban_focused" # More entities in towns
  
  migration:
    enabled: true
    seasonal_patterns: true
    trade_routes: ["town_center", "mining_camp"]
    
locations:
  - id: "town_center"
    population:
      target_count: 100
      demographics: ["human", "halfling", "dwarf"]
      roles: ["merchant", "guard", "citizen"]
      
  - id: "mining_camp"
    population:
      target_count: 50
      demographics: ["human", "dwarf"]
      roles: ["miner", "foreman", "cook"]
      migration_source: "town_center"  # Workers come from town
```

**Event Patterns (Phase 3):**
- `orchestrator.world.entity_move_requested`
- `orchestrator.world.entity_moved`
- `orchestrator.world.population_rebalanced`
- `orchestrator.world.migration_triggered`
- `orchestrator.world.entity_spawned_at_location`
- `orchestrator.world.entity_despawned_from_location`

**Deliverables:**
- Cross-location entity tracking system
- Entity movement APIs and event patterns
- Population management across multiple locations
- Migration and mobility configuration options
- Entity lifecycle management (spawn/move/despawn)
- Demographic and role-based population distribution

**Success Criteria:** 
- NPCs and monsters can move between locations seamlessly
- Population distributions maintain balance across the world
- Entity states are tracked consistently across location boundaries
- Migration patterns create believable population movement
- Entity movement respects location connections and requirements

**Dependencies:** Phase 2 completion

---

### Phase 4: World Persistence & Advanced Features

**Goal:** Create persistent worlds with advanced dynamic systems for long-term campaign management.

**Scope:**
- Integration with persistence tool (Issue #83)
- World save/load functionality with complete entity state preservation
- Advanced configuration options (economics, events, factions when available)
- Dynamic world events and scheduling systems
- World state snapshots and rollback capabilities
- Integration with future world tools (Issue #84)

**Key Components:**
```go
// Phase 4 persistence and advanced features
type PersistentWorldManager struct {
    WorldManager                    // Embed base functionality
    persistenceLayer *persistence.Store
    worldState      *WorldSnapshot
    eventScheduler  *EventScheduler
    advancedSystems *AdvancedSystemsManager
}

type WorldSnapshot struct {
    WorldID        string
    Timestamp      time.Time
    LocationStates map[string]*LocationSnapshot
    EntityStates   map[string]*EntitySnapshot
    SystemStates   map[string]interface{} // Economics, factions, etc.
}

type EventScheduler struct {
    scheduledEvents []ScheduledEvent
    eventTriggers   map[string]EventTrigger
    worldClock     *WorldClock
}
```

**Configuration Example (Phase 4):**
```yaml
# Advanced persistent world config
name: "The Northern Reaches Campaign"
scale: "region"

persistence:
  auto_save: true
  save_interval: "10_minutes"
  snapshot_retention: 10
  track_entity_history: true

events:
  enabled: true
  types: ["seasonal_festivals", "trade_caravans", "monster_migrations"]
  frequency: "weekly"
  custom_events:
    - name: "harvest_festival"
      trigger: "autumn_equinox"
      effects: ["increased_trade", "population_gathering"]

advanced_systems:
  economics:
    enabled: true
    trade_simulation: true
  
  time:
    calendar: "standard_fantasy"
    time_scale: "1_day_per_hour"
    seasonal_effects: true
```

**Event Patterns (Phase 4):**
- `orchestrator.world.save_requested`
- `orchestrator.world.load_requested`
- `orchestrator.world.snapshot_created`
- `orchestrator.world.scheduled_event_triggered`
- `orchestrator.world.advanced_system_updated`
- `orchestrator.world.state_corruption_detected`

**Deliverables:**
- Complete world persistence system with save/load
- Advanced configuration schemas for all systems
- Dynamic event system integration
- World state management and recovery capabilities
- Integration hooks for future advanced tools
- Campaign-scale world management features

**Success Criteria:** 
- Can save/load complete worlds with all entity states preserved
- Dynamic world events trigger and affect world state appropriately  
- World state corruption can be detected and recovered from
- Advanced systems integrate seamlessly with core world management
- Campaign-scale features support long-term play

**Dependencies:** Phase 3 completion, Issue #83 (Persistence Tool)

---

## Phase Implementation Strategy

**Development Approach:**
- Each phase must be fully functional before moving to the next
- Comprehensive testing at each phase boundary
- User feedback collection after each phase
- Architecture refinement between phases as needed

**Risk Mitigation:**
- Phase boundaries allow for course correction
- Each phase delivers independent value
- Complexity is introduced gradually
- Fallback to previous phase if needed

**Success Metrics:**
- Phase completion defined by success criteria achievement
- Integration test pass rates for cross-tool coordination
- Performance benchmarks for scalability
- User feedback scores for usability

This phased approach transforms the comprehensive World Manager vision into manageable, deliverable increments while maintaining architectural coherence and allowing for early feedback and course correction.

This World Manager architecture enables the toolkit to provide complete world-building and management experiences while maintaining the clean separation and reusability of the underlying tools. It transforms the toolkit from individual tool usage to comprehensive world orchestration, making it possible for users to create rich, persistent, multi-location game worlds with minimal technical complexity.