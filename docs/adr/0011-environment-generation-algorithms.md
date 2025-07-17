# ADR-0011: Environment Generation Algorithms

**Date**: 2025-01-17  
**Status**: Proposed  
**Context**: Environment generator design for tools/environments module  

## Context

We are designing the next layer above the spatial module - an environment generator that creates complete game environments (dungeons, towns, wilderness areas) using room prefabs and connection systems. We need to choose the core generation algorithm(s) that will drive how environments are created.

## Problem Statement

Environment generation requires balancing multiple concerns:
- **Performance**: Generation speed and runtime queries
- **Flexibility**: Ability to modify and customize generated environments
- **Quality**: Producing well-designed, playable environments
- **Complexity**: Implementation and maintenance burden
- **Integration**: Working well with existing spatial orchestration system

## Options Considered

### Option A: Graph-Based Generation Only

**Approach**: Generate environments as abstract graphs of rooms and connections, then place spatially.

**Process**:
1. Create abstract graph of rooms and connections
2. Assign room prefabs to nodes based on requirements
3. Place rooms in spatial coordinates using graph relationships
4. Generate connections using existing spatial orchestrator

**Advantages**:
- **High flexibility**: Easy to modify room relationships
- **Better performance**: O(n) generation, optimal for pathfinding
- **Natural integration**: Works directly with spatial orchestrator
- **Simpler implementation**: Leverages existing connection system
- **Design-friendly**: Supports both procedural and custom/narrative-driven generation

**Disadvantages**:
- **Spatial efficiency**: May create awkward layouts or wasted space
- **Spatial queries**: O(n) performance for "what's near position X" queries
- **Overlap handling**: Requires additional logic to prevent room overlap

### Option B: Spatial Algorithms (BSP Trees) Only

**Approach**: Recursively divide space into regions, then assign room types to regions.

**Process**:
1. Start with large rectangle representing entire environment
2. Recursively split into smaller rectangles using BSP tree
3. Assign room prefabs to leaf rectangles
4. Create connections between adjacent rooms

**Advantages**:
- **Space efficiency**: Guaranteed efficient use of available space
- **Fast spatial queries**: O(log n) for position-based lookups
- **No overlap issues**: Spatial division prevents room overlap
- **Scalable**: Performs well for very large environments

**Disadvantages**:
- **Lower flexibility**: Hard to modify after generation
- **Complex implementation**: Requires spatial tree structures
- **Poor pathfinding**: Need to rebuild connectivity information
- **Less design control**: Harder to create specific narrative flows

### Option C: Hybrid Approach

**Approach**: Implement both algorithms and provide guidelines for when to use each.

**Implementation Strategy**:
```go
type GenerationAlgorithm int

const (
    GraphBased GenerationAlgorithm = iota
    SpatialBSP
    Hybrid  // Graph structure with spatial constraints
)

type EnvironmentGenerator interface {
    SetAlgorithm(algorithm GenerationAlgorithm)
    Generate(config GenerationConfig) Environment
}
```

**Usage Guidelines**:
- **Graph-based**: Small-medium environments (< 100 rooms), narrative-driven, frequent modifications
- **BSP**: Large environments (> 100 rooms), procedural-only, static after generation
- **Hybrid**: Medium environments needing both design control and space efficiency

## Decision

We will implement **Option A: Graph-Based Generation as Primary Solution** with the following strategy:

### Phase 1: Graph-Based Implementation (Immediate)
- Implement graph-based generation as the primary and initial algorithm
- Provide simple spatial placement with overlap detection
- Focus on integration with spatial orchestrator
- Support both procedural and custom generation
- Design API to be extensible for future algorithms

### Future Consideration: BSP Implementation (If Needed)
- **Add BSP tree-based generation only if performance requirements demand it**
- **Trigger conditions**: Large environments (> 100 rooms), frequent spatial queries, memory constraints
- **Implementation approach**: Maintain same public API for seamless switching
- **Tech debt consideration**: Avoid premature optimization - implement when there's clear need

### Rationale for Graph-First Approach
- **Meets current needs**: Graph-based handles typical RPG environment sizes efficiently
- **Lower complexity**: Reduces implementation and maintenance burden
- **Faster delivery**: Can deliver value sooner with focused implementation
- **Flexibility priority**: Graph-based provides the design flexibility we prioritize
- **Integration advantage**: Works naturally with existing spatial orchestrator

### Public API Design
```go
type GenerationConfig struct {
    Size         EnvironmentSize     // Small, Medium, Large
    Type         EnvironmentType     // Procedural, Custom, Endless
    Constraints  SpatialConstraints  // Bounds, density, etc.
    Prefabs      []PrefabType       // Available room prefabs
}

type EnvironmentGenerator interface {
    Generate(config GenerationConfig) Environment
}

// Initial implementation focuses on graph-based generation
// API designed to be extensible for future algorithms if needed
type GraphBasedGenerator struct {
    orchestrator spatial.RoomOrchestrator
    prefabs      map[PrefabType]RoomPrefab
}
```

## Implementation Complexity Assessment

### Graph-Based Implementation: **Medium Complexity**
- **Estimated effort**: 2-3 weeks for initial implementation
- **Key components**: Graph builder, spatial placement, overlap detection
- **Integration**: Direct use of spatial orchestrator
- **Risk**: Medium - spatial placement logic needs optimization

### Future BSP Implementation: **High Complexity (If Needed)**
- **Estimated effort**: 3-4 weeks when/if implemented
- **Key components**: BSP tree, space division, connection generation
- **Integration**: Requires adaptation layer for spatial orchestrator
- **Risk**: High - complex spatial algorithms and edge cases
- **Decision point**: Only implement if performance profiling shows clear need

### Current Implementation Plan: **2-3 weeks**
- Focus on graph-based generation only
- Design API to be extensible for future algorithms
- Avoid premature optimization and complexity

## Consequences

### Positive
- **Focused implementation**: Lower complexity and faster delivery
- **Flexibility**: Graph-based provides the design flexibility we prioritize
- **Performance**: Meets current RPG environment size requirements efficiently
- **Integration**: Works naturally with existing spatial orchestrator
- **Extensible API**: Designed to accommodate future algorithms if needed
- **Reduced tech debt**: Avoids premature optimization

### Negative
- **Potential future work**: May need BSP implementation for very large environments
- **Spatial efficiency**: May not be optimal for space-constrained scenarios
- **Performance ceiling**: Could hit limits with extremely large environments

### Neutral
- **Decision deferral**: BSP implementation can be added based on actual need
- **Learning opportunity**: Can gather real-world performance data before optimization
- **Backwards compatibility**: Future algorithms can be added without breaking changes

## Alternatives Considered

### Single Algorithm Approach
**Rejected** because:
- Limits flexibility for different use cases
- May not be optimal for all environment sizes
- Harder to evolve and improve over time

### Plugin Architecture
**Deferred** because:
- Adds significant complexity without clear benefit
- Can be added later if third-party algorithms are needed
- Current two algorithms cover most use cases

## Success Criteria

### Initial Implementation (Graph-Based)
- [ ] Graph-based algorithm implemented with spatial orchestrator integration
- [ ] Support for procedural, custom, and endless generation types
- [ ] Room prefab system with T, L, I, and other basic shapes
- [ ] Spatial placement with overlap detection and resolution
- [ ] Comprehensive test suite for graph-based generation
- [ ] Clear documentation and usage examples
- [ ] Performance benchmarks for typical RPG environment sizes

### Future Considerations (BSP - If Needed)
- [ ] Performance profiling data showing need for BSP implementation
- [ ] BSP algorithm implemented with efficient spatial queries
- [ ] Unified API allowing seamless algorithm switching
- [ ] Performance benchmarks demonstrating trade-offs between algorithms

---

**Author**: Development Team  
**Reviewers**: TBD  
**Implementation**: tools/environments module (future work)