# world

The composer for a living region: declared content plus an injected rulebook,
assembled into one append-only journal and one folded present per observer.

Design record: `rpg-project` `ideas/living-world/`. Rendered doc:
https://kirkdiggler.github.io/rpg-toolkit/living-world/.

## Overview

`world` is rulebook-free by construction — its only external dependency is
testify. A rulebook plugs in through the one seam this module consults for
anything a die or a class feature would decide: [`Resolver`](world.go).

Four subpackages plus the composer at module root:

- `journal` — memory: append-only, attributed, audience-scoped facts
- `graph` — structure and the present, derived by fold and never stored
- `quest` — jobs: templates, populations, claims, distributions
- `goal` — a condition over a region, with a clock
- module root (`world`) — assembly, the one write door (`World.Act`), the one
  read door (`World.View`)

The arrows only point one way: `journal <- graph <- quest <- goal <- world`.

## Installation

```bash
go get github.com/KirkDiggler/rpg-toolkit/world
```

## Declare, inject, subscribe

A rulebook builds a world through exactly three touchpoints:

1. **Declare** — `Scenario` is what a content package hands over: graph
   declarations, verbs, quest templates. All data.
2. **Inject** — `Resolver` is what content deliberately withholds. `Config` is
   a Scenario plus a Resolver, and the split in that struct is the whole
   boundary.
3. **Subscribe** — `Result` carries the fact that was written and what the
   jobs made of it, returned as values. No bus, and none wanted.

```go
w, err := world.New(world.Config{
    Scenario: scenario,   // declared content
    Resolver: resolver,   // injected rules — anything answering "did that work"
    Clock:    clock,      // injected time — deadlines are arithmetic, not simulation
    Goals:    goals,
})
result, err := w.Act(ctx, world.Act{Verb: someVerb, Actor: actor, Target: target})
state := w.View(actor) // actor's present, folded from what they witnessed
```

## Provenance

This module graduated from the living-world spike proved out in
`rpg-toolkit/examples/world` (issue #1333). Read each package's own doc
comment before reading its code — each states its own contract.
