# ADR-0010: Multi-Room Orchestrator Architecture Review

Date: 2025-01-17

## Status

Accepted

## Context

Following the implementation of the multi-room orchestrator system as an extension of the spatial module (ADR-0009), we conducted an architectural review to validate that this placement aligns with the project's architectural principles and three-tier system.

The review was prompted by the need to ensure that the orchestrator belongs in the correct architectural layer and doesn't violate the project's "infrastructure vs implementation" philosophy, especially given that we have yet another layer planned (dungeons module) that will build on this infrastructure.

## Decision

After comprehensive analysis, we confirm that the multi-room orchestrator is correctly placed as an extension of the `tools/spatial/` module rather than as a separate module or higher-level abstraction layer.

### Key Architectural Validation

1. **Three-Tier Architecture Compliance**: The orchestrator fits properly in the tools layer as specialized infrastructure that uses foundation modules (core, events) and will be used by mechanics modules.

2. **Infrastructure vs Implementation**: The orchestrator provides spatial relationship infrastructure (connection types, layout patterns, entity transitions) without crossing into game-specific content or mechanics.

3. **Cohesion Principle**: Multi-room coordination is "spatial relationships at larger scale" - a natural extension of the spatial module's existing concerns.

4. **Abstraction Level**: The orchestrator operates at the same abstraction level as other spatial tools (Room, Grid systems) while providing coordination between them.

## Consequences

### Positive

- **Architectural Consistency**: Maintains clean separation between infrastructure (tools) and game mechanics
- **Cohesive API**: Single import provides all spatial functionality from positioning to multi-room coordination
- **Scalable Design**: Can grow with additional spatial features without breaking architectural boundaries
- **Clear Boundaries**: Establishes clear distinction between spatial infrastructure and game-specific content
- **Future-Proof**: Proper foundation for planned dungeons module to build game-specific content

### Negative

- **Module Growth**: Spatial module becomes larger, though mitigated by internal/ organization
- **Complexity**: More interfaces and abstractions within a single module

### Neutral

- **Documentation**: Requires clear documentation of orchestration patterns and usage
- **Future Refactoring**: May need functional splitting if spatial module becomes too large

## Example

The orchestrator correctly provides infrastructure patterns while games provide meaning:

```go
// Infrastructure: Generic spatial patterns
tower := orchestrator.
    WithLayout(spatial.VerticalTower(5)).
    WithConnections(spatial.StairConnections()).
    WithRoomSize(20, 20)

// Games decide what these mean:
// - Castle tower? Wizard spire? Modern building?
// - Stone stairs? Magical lifts? Elevators?
// - Throne rooms? Laboratories? Offices?
```

This maintains the toolkit's philosophy: we provide spatial relationship infrastructure, games provide the meaning and content.

## Architectural Principles Validated

1. **Tools Layer Definition**: Specialized infrastructure used by game mechanics
2. **Infrastructure Philosophy**: Behavior over identity, generic patterns over content
3. **Event Integration**: Consistent event-driven architecture across modules
4. **Entity Management**: Generic entity interfaces, not game-specific types
5. **Dependency Direction**: Tools use foundation, mechanics use tools

## Related ADRs

- ADR-0009: Multi-Room Orchestration Architecture (original implementation decision)
- ADR-0008: Tools Directory Structure (establishes tools layer principles)
- ADR-0002: Hybrid Architecture (establishes three-tier system)