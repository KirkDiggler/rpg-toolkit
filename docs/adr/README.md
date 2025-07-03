# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the RPG Toolkit project.

## What is an ADR?

An ADR documents a significant architectural decision made in the project, including:
- The context and problem statement
- The decision made
- The consequences of that decision

## ADR Format

Each ADR follows this structure:
1. **Title**: ADR-NNNN: Brief description
2. **Date**: When the decision was made
3. **Status**: Proposed, Accepted, Deprecated, Superseded
4. **Context**: Why we needed to make this decision
5. **Decision**: What we decided to do
6. **Consequences**: The results of this decision (positive, negative, neutral)

## Current ADRs

- [ADR-0001: Modifier Value Interface Design](0001-modifier-value-interface.md) - How modifiers work in the event system
- [ADR-0002: Hybrid Event-Driven Architecture](0002-hybrid-architecture.md) - Why we chose a hybrid approach over pure ECS or Event Sourcing
- [ADR-0003: Conditions as Entities](0003-conditions-as-entities.md) - Why conditions implement the Entity interface
- [ADR-0004: Generic Condition Relationships](0004-condition-relationships.md) - How conditions relate to their sources and each other

## Creating a New ADR

1. Copy the template from `template.md`
2. Number it sequentially (e.g., `0002-feature-name.md`)
3. Fill in all sections
4. Set status to "Proposed"
5. After team review, update status to "Accepted"