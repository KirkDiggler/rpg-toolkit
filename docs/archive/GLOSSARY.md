# RPG Toolkit Glossary

This glossary defines common terms used throughout the RPG Toolkit project to ensure consistent communication and understanding across the codebase, documentation, and discussions.

## Core Concepts

### **Entity**
A fundamental game object that implements the `core.Entity` interface. Has an ID and type. Examples: characters, items, spells, rooms, connections.

### **Event Bus**
The communication system that allows modules to publish and subscribe to events without direct coupling. Enables event-driven architecture.

### **Infrastructure vs Implementation**
- **Infrastructure**: Generic tools and systems provided by the toolkit (e.g., spatial positioning, connection systems)
- **Implementation**: Game-specific content and rules that use the infrastructure (e.g., "stone wall" vs "metal bulkhead")

### **Module**
A self-contained package within the toolkit that provides specific functionality. Examples: `core`, `events`, `dice`, `mechanics/conditions`, `tools/spatial`.

## Spatial System Terms

### **Position**
A 2D coordinate in space represented as `Position{X: float64, Y: float64}`. The basic unit of spatial location.

### **Grid**
The underlying coordinate system that defines how positions relate to each other. Types:
- **Square Grid**: Uses Chebyshev distance (D&D 5e style)
- **Hex Grid**: Uses cube coordinates for hexagonal positioning
- **Gridless**: Uses Euclidean distance for theater-of-mind play

### **Room**
A single spatial container that manages entities within a defined area. Implements spatial calculations, entity placement, and movement within one space.

### **Spatial**
Relating to 2D positioning, movement, and relationships between entities in game space. The `tools/spatial` module provides spatial infrastructure.

## Multi-Room System Terms

### **Connection**
A link between two rooms that defines how entities can move between them. Has properties like:
- **Type**: Door, stairs, passage, portal, bridge, tunnel
- **Passability**: Whether entities can currently traverse it
- **Requirements**: Conditions needed to use the connection
- **Cost**: Movement cost to traverse
- **Reversibility**: Whether it works in both directions

### **Room Orchestrator**
A system that manages multiple rooms and their connections. Handles:
- Room management (add/remove rooms)
- Connection management (create/modify connections)
- Entity tracking across rooms
- Pathfinding between connected rooms

### **Layout Pattern**
A spatial arrangement strategy for organizing multiple rooms:
- **Tower**: Vertical stacking (floors connected by stairs)
- **Branching**: Hub and spoke pattern (central room with branches)
- **Grid**: 2D grid arrangement (rooms in rows/columns)
- **Organic**: Irregular, natural-feeling connections

### **Multi-Room Orchestration**
The complete system for managing multiple connected rooms, including room management, connections, entity tracking, and layout patterns.

## Entity Spawning Terms

### **Spawn Engine**
A system that handles entity placement in game spaces. Supports single room and multi-room scenarios with team coordination, player choice mechanics, and environment integration for capacity analysis and room scaling.

### **Split-Aware Architecture**
Design pattern where systems can work with both single rooms and multi-room configurations without requiring different APIs. The spawn engine is split-aware but not split-responsible - it handles placement regardless of room structure.

### **Entity Spawning**
The process of placing game entities (characters, enemies, items) into spatial positions within rooms. Includes validation, collision detection, and constraint satisfaction.

### **Team Cohesion Rules**
Spawn engine configuration that keeps entities of certain types together spatially. Examples: keep all allies near each other, keep enemies in groups, maintain minimum distance between opposing teams.

### **Player Spawn Zones**
Designated areas within rooms where players can choose their starting positions. Each zone has boundaries, entity type restrictions, and capacity limits. Supports player agency in tactical positioning.

### **Capacity-First Approach**
Spawn engine strategy that prioritizes fitting entities into available space. If entities don't fit, the engine recommends room scaling or provides split recommendations rather than failing.

### **Spawn Pattern**
The spatial arrangement strategy for entity placement:
- **Scattered**: Random distribution across available space
- **Clustered**: Groups entities with controlled spacing
- **Formation**: Structured arrangements (lines, circles, etc.)
- **Team-Based**: Teams placed in separate areas with cohesion rules
- **Player Choice**: Players choose positions within designated zones

### **Room Scaling**
Automatic enlargement of room dimensions when entities don't fit in available space. The spawn engine can trigger scaling through environment package integration, preserving aspect ratios and spatial relationships.

### **Split Recommendation**
Suggestions provided by the spawn engine when a room is too crowded for effective gameplay. Includes analysis of where the room could be divided and how entities would be distributed across the resulting spaces.

## Environment Generation Terms

### **Environment**
A complete game space composed of multiple connected rooms. Examples: dungeons, towns, wilderness areas, spaceships, buildings.

### **Prefab (Room Prefab)**
A pre-designed room template that defines:
- **Shape**: Geometric form (T, L, I, Cross, Circle, etc.)
- **Dimensions**: Size of the room
- **Connection Points**: Where connections can attach
- **Obstacles**: Sparse placement of walls, pillars, etc.
- **Floor Plan**: Walkable vs blocked areas

### **Generation Algorithm**
The method used to create environments:
- **Graph-Based**: Creates abstract room relationships first, then places spatially
- **Spatial (BSP)**: Divides space first, then assigns room types to regions
- **Hybrid**: Combines both approaches

### **Environment Generator**
A system that creates complete environments using prefabs, generation algorithms, and configuration parameters.

### **Wall Pattern**
An algorithmic approach to generating internal walls within rooms. Types:
- **Empty**: No internal walls
- **Random**: Procedural wall placement based on density parameters

### **Wall Entity**
A wall represented as a spatial entity that implements the `Placeable` interface. Integrates with spatial module's obstacle system for line-of-sight and movement blocking.

### **Destructible Wall**
A wall that can be damaged and destroyed during gameplay, creating tactical opportunities for environmental interaction.

### **Wall Density**
A parameter (0.0-1.0) that controls how many walls are generated in a room. 0.0 = no walls, 1.0 = maximum walls.

### **Destructible Ratio**
A parameter (0.0-1.0) that controls what percentage of generated walls can be destroyed. 0.0 = all indestructible, 1.0 = all destructible.

### **Room Builder**
A fluent API for constructing rooms with specific shapes, wall patterns, and features. Follows the toolkit's config pattern.

### **Room Shape**
A geometric template that defines the boundary and connection points of a room. Examples: rectangle, square, L-shape, T-shape, cross, oval.

### **Query Delegation**
An architectural pattern where environment-level queries aggregate results from multiple rooms by delegating to spatial module queries.

### **Capacity Analysis**
Environment package queries that determine if entities will fit in available room space. Used by spawn engine to trigger room scaling or split recommendations when space is insufficient.

### **Feeling-Based Sizing**
Environment generation approach that creates rooms with emotional/atmospheric qualities (cozy, spacious, cramped) rather than purely geometric constraints. Integrates with spawn engine capacity analysis.

## Architecture Terms

### **Three-Tier Architecture**
The project's layered structure:
1. **Foundation Layer**: Core utilities (core, events, dice)
2. **Tools Layer**: Specialized infrastructure (spatial, environments)
3. **Mechanics Layer**: Game mechanics (conditions, spells, features)

### **Middleware Pattern**
The environment module's role as a client-friendly layer over the spatial module, providing semantic enhancement while delegating core operations.

### **Config Pattern**
The consistent approach of using configuration structs for constructors rather than long parameter lists. Example: `NewBasicRoom(BasicRoomConfig{...})`.

### **Helper Functions**
Convenience functions that wrap constructors with common configurations. Example: `CreateDoorConnection()` wraps `NewBasicConnection()` with door-specific defaults.

### **Event-Driven Architecture**
Design pattern where components communicate through events rather than direct method calls, enabling loose coupling and flexibility.

### **Context (Disambiguation)**
This project uses two different types of "context" - be explicit about which one you mean:
- **Go Context**: The standard `context.Context` used for cancellation, timeouts, and request-scoped values
- **Event Context**: The custom `events.Context` system that carries game-specific data (damage, modifiers, entity references) between event handlers

### **Split-Responsible vs Split-Aware**
Important architectural distinction for multi-room systems:
- **Split-Responsible**: System decides when to split rooms and how to manage multiple spaces
- **Split-Aware**: System can work with existing split configurations but doesn't decide splitting policy. The spawn engine is split-aware but not split-responsible.

## Common Misunderstandings

### **"Dungeon" vs "Environment"**
- **Dungeon**: A specific type of environment (underground, combat-focused)
- **Environment**: The general concept of any complete game space (dungeons, towns, forests, spaceships)

### **"Room" vs "Space"**
- **Room**: A single spatial container managed by the spatial module
- **Space**: Can refer to any game area (could be a room, environment, or abstract concept)

### **"Spatial" vs "Environment"**
- **Spatial**: Low-level positioning and movement infrastructure
- **Environment**: Higher-level complete game spaces built using spatial infrastructure

### **"Wall Pattern" vs "Wall Entity"**
- **Wall Pattern**: The algorithmic approach to generating walls (empty, random)
- **Wall Entity**: Individual wall segments placed as spatial entities

### **"Connection" vs "Door"**
- **Connection**: The abstract link between rooms (infrastructure)
- **Door**: A specific type of connection (implementation detail)

### **"Layout" vs "Generation"**
- **Layout**: The spatial arrangement pattern of rooms
- **Generation**: The complete process of creating an environment

### **"Spawning" vs "Placement"**
- **Spawning**: The complete process of entity creation and spatial positioning, including validation and event publishing
- **Placement**: The specific act of putting an entity at spatial coordinates (subset of spawning)

### **"Team" vs "Group"**
- **Team**: Entities with shared tactical relationships (allies, enemies) that affect spawn cohesion rules
- **Group**: Organizational unit in spawn configuration that defines what entities to spawn together

### **"Zone" vs "Area" vs "Region"**
- **Zone**: Specific spawn engine concept for designated player choice areas with boundaries and restrictions
- **Area**: Generic spatial concept referring to any bounded space
- **Region**: Environment concept for larger spatial divisions (avoid when discussing spawn zones)

## Generation Type Terms

### **Procedural Generation**
Algorithmic creation of environments using parameters and rules, producing different results each time.

### **Custom Generation**
Creating environments from specific, pre-designed configurations or templates.

### **Endless Generation**
Infinite or very large environments that generate content as players explore, typically procedural.

### **Preset Environment**
Pre-designed environments created for specific narrative or gameplay purposes.

## Technical Terms

### **Orchestrator**
A coordination system that manages multiple related components. Examples: RoomOrchestrator, EnvironmentOrchestrator.

### **Factory Pattern**
A design pattern for creating objects. Note: In this project, we use "helper functions" rather than true factories.

### **ADR (Architecture Decision Record)**
Documents that record important architectural decisions, their context, and consequences.

### **Journey**
Documentation that tracks the evolution of features or architectural decisions over time.

## Size Classifications

### **Environment Sizes**
- **Small**: 5-15 rooms (typical for single encounters or small areas)
- **Medium**: 15-50 rooms (typical for dungeon floors or town districts)
- **Large**: 50-100 rooms (typical for complete dungeons or large towns)
- **Massive**: 100+ rooms (typically requires performance optimization)

## Performance Terms

### **Spatial Queries**
Questions about spatial relationships: "What's near position X?", "Can I move from A to B?", "What's in this area?"

### **Pathfinding**
Finding routes between locations, typically using graph algorithms like breadth-first search.

### **Event Overhead**
The performance cost of publishing and handling events, particularly relevant for high-frequency operations.

## Usage Guidelines

### **When to Use Each Term**
- Use **"spatial"** when discussing positioning, movement, and 2D relationships
- Use **"environment"** when discussing complete game spaces
- Use **"room"** when discussing single spatial containers
- Use **"connection"** when discussing links between rooms
- Use **"orchestrator"** when discussing coordination systems
- Use **"generation"** when discussing the creation of environments
- Use **"wall pattern"** when discussing algorithmic wall generation
- Use **"wall entity"** when discussing individual wall segments
- Use **"destructible"** when discussing walls that can be damaged/destroyed
- Use **"spawn engine"** when discussing entity placement systems
- Use **"spawning"** when discussing the complete entity placement process
- Use **"team cohesion"** when discussing keeping related entities together
- Use **"player spawn zone"** when discussing designated areas for player positioning
- Use **"capacity analysis"** when discussing whether entities fit in available space
- Use **"split-aware"** when discussing systems that work with multi-room configurations
- Use **"room scaling"** when discussing automatic room size adjustments

### **Avoid Ambiguous Terms**
- **"Space"** - Too vague, use "room" or "environment"
- **"Area"** - Too vague, use "room", "environment", "region", or "spawn zone"
- **"Map"** - Could mean many things, be specific
- **"Level"** - Game-specific term, use "environment" or "floor"
- **"Placement"** - When you mean full spawning process, use "spawning"
- **"Group"** - Could mean team or entity group, be specific
- **"Position"** - When you mean spawn choice or zone, be specific

---

*This glossary should be updated as new concepts are introduced to the toolkit. When in doubt, prefer precision over brevity in terminology.*