# Run — 2026-11-06 — Item/effect architecture walkthrough

Documentation work [rpg-project#876](https://github.com/KirkDiggler/rpg-project/issues/876), serving the [effects extraction scope](https://github.com/KirkDiggler/rpg-project/issues/705) and [item consumers](https://github.com/KirkDiggler/rpg-project/issues/850).

## Shipped

- A layer walkthrough, topic-specific paper-trail map and discovery links from the docs index, architecture overview and ADR; no runtime or publication workflow change — [walkthrough](../../docs/ideas/item-effect-composition/README.md).

## Decided

- The operator requested a durable explanation connecting decisions, plans and eventual implementation to the larger architecture; the ADR remains the decision authority rather than copying it into another law document — [record map](../../docs/ideas/item-effect-composition/paper-trail.md).
- Name future plan/implementation homes without inventing tasks or outcomes, because experiment plans and an Accepted ADR do not establish extraction readiness or completed production integration — [artifact homes](../../docs/ideas/item-effect-composition/paper-trail.md#homes-and-responsibilities).

## Found

- KindSpec supplies Check/Attach/Assess but no complete designer-field schema; the existing condition factory carries content SourceRef, not the item-instance provenance contract by itself — [explicit edges](../../docs/ideas/item-effect-composition/README.md#edges-that-must-remain-visible).
- Existing skills already own journals, walkthroughs, decisions, checked plans and delivery; a reference map connects them without introducing a second workflow — [rpg-project#876](https://github.com/KirkDiggler/rpg-project/issues/876).

## Mistaken

- The first diagram's two-line cluster title crowded a node; a shorter title renders clearly. Both Mermaid diagrams are rendered and visually inspected with headless system Chrome; no browser configuration or runtime dependency change is needed — [component shape](../../docs/ideas/item-effect-composition/README.md#component-shape).
