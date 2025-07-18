# Journey 015: Adaptive Opportunity Generation Tools - Building Blocks for Cooperative RPG Experiences

**Date**: 2025-01-17  
**Context**: Brainstorming session for cooperative dungeon crawler tools and party-adaptive content generation  
**Status**: Design Exploration  

## The Vision

We're exploring a **cooperative dungeon crawler** experience (think Gloomhaven meets Path of Exile) where the game dynamically creates opportunities for each party member to shine based on their unique skills and backgrounds. No GM needed - the system intelligently generates moments where the cleric's religion knowledge, the rogue's stealth, or the wizard's arcane understanding becomes the key to success.

## The Core Insight

Instead of fixed encounters, we want **adaptive opportunity generation** that:
- Analyzes party composition and skills
- Creates moments where each character type can excel
- Provides multiple solution paths for diverse party approaches
- Ensures balanced spotlight distribution across all players

## Proposed Tool Ecosystem

### 1. Party Profiler (`tools/party`)

**Purpose**: Analyze party composition and create opportunity profiles

```go
type PartyProfiler interface {
    AnalyzeParty(party []Character) *PartyProfile
    GetOpportunityWeights(profile *PartyProfile) map[OpportunityType]int
    SuggestContent(profile *PartyProfile) []ContentSuggestion
}

type PartyProfile struct {
    Skills       map[string]int  // "religion": 15, "history": 12, etc.
    Backgrounds  []string        // "acolyte", "scholar", "criminal"
    Classes      []string        // "cleric", "wizard", "rogue"
    Personalities []string       // "cautious", "aggressive", "diplomatic"
    Gaps         []string        // Missing skills/approaches
}
```

**Key Features**:
- Skill proficiency mapping
- Background and class analysis
- Personality trait extraction
- Gap identification for challenge balance

### 2. Skill-Based Content (`tools/skills`)

**Purpose**: Content that adapts to available skills

```go
type SkillOpportunity struct {
    ID           string
    Name         string
    PrimarySkill string         // "religion", "history", "perception"
    Difficulty   int            // DC for success
    Rewards      []Reward       // What success gives
    Alternatives []Alternative  // Other ways to solve
}

type Alternative struct {
    Skill      string
    Difficulty int
    Outcome    string  // Different result from primary approach
}
```

**Key Features**:
- Multiple solution paths per challenge
- Skill-based difficulty scaling
- Alternative approaches for diverse parties
- Meaningful outcome variations

### 3. Moment Creation Engine (`tools/moments`)

**Purpose**: Create "moments to shine" for each character archetype

```go
type MomentGenerator struct {
    archetypes map[string]ArchetypeProfile
    moments    map[string]SelectionTable[Moment]
}

type Moment struct {
    Trigger     string        // When this moment appears
    Challenge   string        // What needs to be overcome
    Approaches  []Approach    // Different ways to handle it
    Spotlight   string        // Which archetype this highlights
}
```

**Key Features**:
- Archetype-specific moment generation
- Multiple approach options per moment
- Clear spotlight distribution
- Trigger-based activation

### 4. Opportunity Orchestrator (`tools/orchestration`)

**Purpose**: Ensure balanced spotlight distribution

```go
type OpportunityOrchestrator struct {
    recentSpotlights map[string]int  // Track who's had chances
    cooldowns        map[string]int  // Prevent spam
    guarantees       map[string]int  // Min opportunities per archetype
}
```

**Key Features**:
- Spotlight tracking per player/archetype
- Cooldown management
- Guaranteed minimum opportunities
- Priority-based selection

## Example Content Types

### Religion Opportunities
- **Consecrated Ground**: Religion check reveals blessing opportunity
- **Unholy Symbol**: Religion knowledge reveals enemy weakness
- **Divine Intervention**: Faith-based solutions to obstacles

### History Opportunities  
- **Ancient Mechanism**: Historical knowledge reveals shortcuts
- **Lost Civilization**: Cultural understanding unlocks secrets
- **Artifact Recognition**: Historical context provides advantages

### Stealth Opportunities
- **Guard Patrol**: Stealth allows surprise or avoidance
- **Infiltration**: Sneaky approaches to objectives
- **Ambush Setup**: Positioning advantages through stealth

## Integration with Existing Tools

### Spatial System Integration
```go
type SpatialContentPlacer interface {
    PlaceMonsters(room *spatial.Room, table SelectionTable[Monster])
    PlaceTreasure(room *spatial.Room, table SelectionTable[Treasure])  
    PlaceOpportunities(room *spatial.Room, profile *PartyProfile)
}
```

### Selection Tables Enhancement
```go
type AdaptiveWeighting struct {
    baseWeights map[string]int
    boosters    map[string]SkillBooster
}

func (w *AdaptiveWeighting) CalculateWeights(profile *PartyProfile) map[string]int {
    // Boost content this party can handle well
    // Reduce content they can't engage with
}
```

## Technical Considerations

### Event-Driven Content
- Content that reacts to game events
- Dynamic opportunities based on player actions
- State-aware selection systems

### Performance Optimization
- Efficient party analysis algorithms
- Cached opportunity generation
- Lazy loading of content possibilities

### Validation & Testing
- Balance testing for opportunity distribution
- Statistical analysis of generation patterns
- A/B testing for content engagement

## Architecture Benefits

### Composability
Every tool works with existing systems:
- **Spatial**: Rooms get party-appropriate content
- **Selection**: Tables weighted by party composition  
- **Events**: Opportunities triggered by game state
- **Dice**: Skill checks with party-appropriate DCs

### Extensibility
- New opportunity types easily added
- Custom archetype definitions
- Pluggable content generators
- Modular spotlight systems

### Testability
- Deterministic generation for testing
- Mockable interfaces for unit tests
- Statistical validation tools
- Balance verification systems

## Open Questions

### Design Dragons 🐉

1. **Spotlight Balance**: How do we ensure fair distribution without feeling artificial?
2. **Skill Scaling**: Should opportunities scale with party skill levels or provide consistent challenge?
3. **Multi-Solution Complexity**: How many alternative approaches per opportunity is optimal?
4. **Content Freshness**: How do we prevent repetitive feeling in procedural generation?

### Implementation Unknowns

1. **Performance**: Can we analyze party composition in real-time during generation?
2. **Storage**: How do we efficiently store and query large opportunity databases?
3. **Customization**: Should DMs/players be able to adjust opportunity weights?
4. **Integration**: How does this fit with existing combat and exploration systems?

## Next Steps

### Phase 1: Foundation
1. Implement basic `PartyProfiler` with skill analysis
2. Create simple `SkillOpportunity` structure
3. Build prototype opportunity selection system
4. Test with basic D&D 5e content

### Phase 2: Intelligence
1. Add `OpportunityOrchestrator` for balance
2. Implement adaptive weighting system
3. Create moment generation engine
4. Add spatial system integration

### Phase 3: Polish
1. Build content validation tools
2. Add statistical analysis
3. Create content authoring tools
4. Implement performance optimizations

## Impact on Project Vision

This tool ecosystem directly supports the **cooperative dungeon crawler** vision by:

- **Eliminating GM Dependency**: System intelligently creates appropriate content
- **Highlighting Player Uniqueness**: Each character gets meaningful moments
- **Enabling Diverse Solutions**: Multiple approaches to every challenge
- **Maintaining Engagement**: Balanced spotlight prevents anyone from being sidelined

The tools follow the project's **composable architecture** principles, building on existing spatial, selection, and event systems while adding the intelligence needed for adaptive content generation.

## Lessons for Tool Design

### Keep It Simple
Each tool should solve one problem well:
- **PartyProfiler**: Analyze composition only
- **SkillOpportunity**: Define content structure only  
- **OpportunityOrchestrator**: Manage balance only

### Composability First
Tools should work together naturally:
- ProfileR + SelectionTable = Weighted opportunities
- Moments + Spatial = Positioned challenges
- Orchestrator + Events = Balanced progression

### Data-Driven Design
Content should be easily authored and modified:
- JSON/YAML opportunity definitions
- Configurable archetype profiles
- Adjustable balance parameters

---

*This journey explores the foundational tools needed for adaptive, party-aware content generation in cooperative RPG experiences. The focus is on building composable tools that intelligently create moments for each player to shine.*