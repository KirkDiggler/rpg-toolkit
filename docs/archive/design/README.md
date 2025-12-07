# Design Documents

This folder contains detailed design documents for specific implementations and features. These are not architectural decisions (those go in `/adr`), but rather implementation designs and specifications.

## Current Designs

### Core Systems

- [D&D 5e Data Mapping Architecture](./dnd5e-data-mapping-architecture.md) - How the toolkit provides typed data structures and mapping tools for game servers to convert external D&D 5e data sources

### Historical Designs (Completed)

- [Character Loading Final](./character-loading-final.md) - Final design for character loading system
- [Effect Composition Design](./effect-composition-design.md) - How effects compose together
- [Event Bus Detailed Design](./event-bus-detailed-design.md) - Implementation of the event bus system

## Design Document Guidelines

Design docs should include:
- **Problem Statement** - What problem is being solved
- **Design Principles** - Key principles guiding the design
- **Implementation Details** - How it will be built
- **Usage Examples** - How it will be used
- **Trade-offs** - What alternatives were considered

Design docs are NOT:
- **ADRs** - Architectural decisions go in `/adr`
- **Journey Docs** - Exploration and discovery go in `/journey`
- **Guides** - How-to guides go in `/guides`