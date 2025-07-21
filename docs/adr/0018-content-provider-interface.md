# ADR-0018: Content Provider Interface Architecture

Date: 2025-01-20

## Status

Proposed

## Context

The experiences architecture (ADR-0017) requires a content management system that can integrate multiple game systems (D&D 5e, Pathfinder, custom content) through a unified interface. This content system needs to:

1. **Multi-Source Integration**: Support API-based providers (D&D 5e API), file-based providers (YAML/JSON), and user-generated content
2. **Content Normalization**: Transform system-specific data into normalized schemas for toolkit consumption
3. **Performance Optimization**: Cache content for offline usage and performance
4. **Provider Discovery**: Allow dynamic registration and discovery of content sources
5. **Integration with Existing Tools**: Feed normalized content into selectables, spawn engine, and other toolkit components

Currently, there is no content management infrastructure in the toolkit. Individual tools like selectables can handle content selection, but there's no system for content acquisition, normalization, or multi-source integration.

The challenge is designing a content provider interface that is flexible enough to handle diverse content sources while maintaining the toolkit's philosophy of providing infrastructure rather than implementation.

## Decision

We will create a `/tools/content` module that provides content management infrastructure through a pluggable provider system.

### Content Provider Interface

```go
// Core provider interface - all content sources implement this
type ContentProvider interface {
    // Provider metadata
    GetName() string
    GetVersion() string
    GetSupportedSystems() []string
    GetStatus() ProviderStatus
    
    // Content retrieval with filtering
    GetMonsters(ctx context.Context, criteria MonsterCriteria) ([]Monster, error)
    GetEquipment(ctx context.Context, criteria EquipmentCriteria) ([]Equipment, error)
    GetSpells(ctx context.Context, criteria SpellCriteria) ([]Spell, error)
    
    // Content discovery
    ListAvailableContent(ctx context.Context, contentType ContentType) ([]ContentSummary, error)
    
    // Lifecycle management
    Initialize(ctx context.Context, config ProviderConfig) error
    Refresh(ctx context.Context) error
    Close(ctx context.Context) error
}

// Provider registry for multi-source management
type Registry struct {
    providers map[string]ContentProvider
    cache     ContentCache
    eventBus  events.Bus
}

func (r *Registry) RegisterProvider(name string, provider ContentProvider) error
func (r *Registry) GetContent(ctx context.Context, criteria ContentCriteria) ([]Content, error)
func (r *Registry) CreateSelectionTable(ctx context.Context, criteria ContentCriteria) (*selectables.Table, error)
```

### Provider Implementations

**API-Based Provider (D&D 5e)**
```go
type DnD5eAPIProvider struct {
    client   *http.Client
    baseURL  string
    cache    ContentCache
    transformer *DnD5eTransformer
}

func (p *DnD5eAPIProvider) GetMonsters(ctx context.Context, criteria MonsterCriteria) ([]Monster, error) {
    // 1. Check cache first
    // 2. Hit D&D 5e API with filters
    // 3. Transform to normalized Monster schema
    // 4. Apply theme/environment filters
    // 5. Cache results
    // 6. Return normalized content
}
```

**File-Based Provider (Custom Content)**
```go
type FileProvider struct {
    contentDir string
    templates  TemplateLibrary
    validator  ContentValidator
}

func (p *FileProvider) GetMonsters(ctx context.Context, criteria MonsterCriteria) ([]Monster, error) {
    // 1. Scan content directory for monster files
    // 2. Load and parse YAML/JSON content
    // 3. Apply template expansion if needed
    // 4. Validate content against schemas
    // 5. Apply filters and return
}
```

### Content Criteria and Filtering

```go
type MonsterCriteria struct {
    // Challenge/Difficulty
    ChallengeRange *ChallengeRange
    
    // Type/Category Filters
    Types       []string  // ["undead", "fiend", "humanoid"]
    Subtypes    []string  // ["goblinoid", "shapechanger"]
    Sizes       []string  // ["small", "medium", "large"]
    
    // Theme/Environment Filters  
    Themes      []string  // ["horror", "boss", "minion"]
    Environment []string  // ["dungeon", "forest", "urban"]
    
    // System-Specific Filters
    SystemFilters map[string]interface{}
    
    // Result Controls
    Limit  int
    Offset int
    SortBy string
}

type EquipmentCriteria struct {
    Categories []string  // ["weapon", "armor", "treasure"]
    Rarity     []string  // ["common", "uncommon", "rare"]
    Themes     []string  // ["magical", "mundane", "cursed"]
    ValueRange *ValueRange
    
    // Equipment-specific filters
    WeaponTypes []string  // ["martial_melee", "simple_ranged"]
    ArmorTypes  []string  // ["light", "medium", "heavy"]
    
    Limit  int
    Offset int
}
```

### Caching Strategy

```go
type ContentCache interface {
    Get(key string) ([]byte, bool)
    Set(key string, data []byte, ttl time.Duration) error
    Clear(pattern string) error
    Stats() CacheStats
}

// Multi-tier caching
type MultiTierCache struct {
    memory ContentCache  // Fast in-memory cache
    disk   ContentCache  // Persistent disk cache  
    ttl    time.Duration // Cache expiration
}
```

### Integration with Selectables

```go
// Registry creates selection tables from content
func (r *Registry) CreateMonsterTable(ctx context.Context, criteria MonsterCriteria) (*selectables.Table[Monster], error) {
    monsters, err := r.GetContent(ctx, criteria)
    if err != nil {
        return nil, err
    }
    
    // Create weighted table based on challenge rating, rarity, etc.
    table := selectables.NewBasicTable[Monster](selectables.BasicTableConfig{
        ID: "monster-table",
    })
    
    for _, monster := range monsters {
        weight := calculateWeight(monster, criteria)
        table.AddItem(monster, weight)
    }
    
    return table, nil
}
```

### Provider Configuration

```go
type ProviderConfig struct {
    Type     string                 `yaml:"type"`     // "dnd5e_api", "file", "composite"
    Name     string                 `yaml:"name"`
    Settings map[string]interface{} `yaml:"settings"`
    Cache    CacheConfig           `yaml:"cache"`
    Filters  DefaultFilters        `yaml:"filters"`
}

// Example configurations
dnd5e_config:
  type: "dnd5e_api"
  name: "Official D&D 5e"
  settings:
    api_url: "https://www.dnd5eapi.co/api"
    timeout: "30s"
  cache:
    ttl: "24h"
    max_size: "100MB"
    
custom_config:
  type: "file"
  name: "My Custom Monsters"
  settings:
    content_dir: "./content/monsters"
    watch_changes: true
  filters:
    default_themes: ["custom"]
```

## Consequences

### Positive

- **Pluggable Architecture**: Easy to add new content sources (Pathfinder, Starfinder, etc.)
- **Performance**: Multi-tier caching reduces API calls and improves response times
- **Unified Interface**: All content sources expose the same interface for consistent usage
- **Integration Ready**: Direct integration with selectables and spawn engine
- **Offline Support**: Cached content enables offline dungeon generation
- **User Flexibility**: Support for custom content alongside official sources

### Negative

- **Implementation Complexity**: Multiple provider types require different implementation strategies
- **Cache Management**: Cache invalidation and consistency across multiple sources
- **Error Handling**: Network failures, API changes, and malformed content need robust handling
- **Memory Usage**: Caching large content databases may consume significant memory

### Neutral

- **Provider Dependencies**: API-based providers depend on external service availability
- **Content Versioning**: Need to handle content updates and version compatibility
- **Configuration Complexity**: Multiple providers require careful configuration management

## Example

### Provider Registration and Usage

```go
// Initialize content registry
registry := content.NewRegistry(content.RegistryConfig{
    EventBus: eventBus,
    Cache:    content.NewMultiTierCache(cacheConfig),
})

// Register D&D 5e API provider
dnd5eProvider := adapters.NewDnD5eAPIProvider(dnd5eConfig)
registry.RegisterProvider("dnd5e_official", dnd5eProvider)

// Register custom content provider
customProvider := adapters.NewFileProvider(customConfig)
registry.RegisterProvider("my_custom", customProvider)

// Use in dungeon generation
monsterCriteria := content.MonsterCriteria{
    ChallengeRange: &content.ChallengeRange{Min: 1, Max: 5},
    Themes:        []string{"undead", "horror"},
    Environment:   []string{"dungeon"},
    Limit:         20,
}

// Get content from all registered providers
monsters, err := registry.GetContent(ctx, monsterCriteria)

// Or create selection table for spawn engine
monsterTable, err := registry.CreateMonsterTable(ctx, monsterCriteria)
spawnEngine.RegisterEntityGroup("dungeon_monsters", spawn.EntityGroup{
    SelectionTable: monsterTable,
    SpawnPattern:   spawn.PatternScattered,
})
```

### Custom Content Definition

```yaml
# custom_monsters.yaml
monsters:
  - id: "shadow_stalker"
    name: "Shadow Stalker"
    type: "undead"
    subtype: "shadow"
    challenge: 3.0
    hit_points: 45
    armor_class: 14
    themes: ["horror", "stealth"]
    environment: ["dungeon", "ruins"]
    abilities:
      - name: "Shadow Step"
        description: "Teleport between shadows"
        usage:
          type: "recharge"
          dice_size: 6
          min_value: 4
    actions:
      - name: "Drain Touch"
        type: "attack"
        attack_bonus: 6
        damage:
          - dice: "2d6+3"
            type: "necrotic"
```

This architecture provides the foundation for flexible, performant content management that can scale from simple custom content to complex multi-system integration.