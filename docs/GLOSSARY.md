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

## Architecture Terms

### **Three-Tier Architecture**
The project's layered structure:
1. **Foundation Layer**: Core utilities (core, events, dice)
2. **Tools Layer**: Specialized infrastructure (spatial, environments)
3. **Mechanics Layer**: Game mechanics (conditions, spells, features)

### **Config Pattern**
The consistent approach of using configuration structs for constructors rather than long parameter lists. Example: `NewBasicRoom(BasicRoomConfig{...})`.

### **Helper Functions**
Convenience functions that wrap constructors with common configurations. Example: `CreateDoorConnection()` wraps `NewBasicConnection()` with door-specific defaults.

### **Event-Driven Architecture**
Design pattern where components communicate through events rather than direct method calls, enabling loose coupling and flexibility.

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

### **"Connection" vs "Door"**
- **Connection**: The abstract link between rooms (infrastructure)
- **Door**: A specific type of connection (implementation detail)

### **"Layout" vs "Generation"**
- **Layout**: The spatial arrangement pattern of rooms
- **Generation**: The complete process of creating an environment

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

### **Avoid Ambiguous Terms**
- **"Space"** - Too vague, use "room" or "environment"
- **"Area"** - Too vague, use "room", "environment", or "region"
- **"Map"** - Could mean many things, be specific
- **"Level"** - Game-specific term, use "environment" or "floor"

---

*This glossary should be updated as new concepts are introduced to the toolkit. When in doubt, prefer precision over brevity in terminology.*