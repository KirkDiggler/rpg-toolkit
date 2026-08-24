# RPG Toolkit documentation

Use progressive disclosure: start with the route for your task, then read the
nearest package README, only the ADRs/journey notes it links, and finally the
code and tests. Code/tests are behavior truth. ADRs record decisions; journeys
record rationale/history. Status, architecture summaries, plans, and how-tos can
lag and must be verified.

## Task routes

### Contribute rulebook content

1. [D&D 5e rulebook mental model](../rulebooks/dnd5e/README.md)
2. [Add a D&D 5e monster](how-to/add-a-dnd5e-monster.md) — recommended first contribution
3. [Nearest monster package guide](../rulebooks/dnd5e/monster/README.md)
4. [Add another rulebook entry](how-to/add-a-rulebook-entry.md)

The monster route includes content provenance, unsupported-clause stop rules,
current factory/registry paths, and full load/round-trip expectations.

### Change behavior

1. [Architecture overview](architecture/overview.md)
2. [Data model and persistence](architecture/data-model.md)
3. [Add a mechanic](how-to/add-a-mechanic.md)
4. The nearest [component guide](architecture/components/)
5. Targeted [ADR](adr/README.md) and [journey](journey/README.md) entries linked
   by that guide

### Develop and verify

- [Run tests](how-to/run-tests.md)
- [Fix local `go.mod` replace directives](how-to/fix-go-mod-replace-directives.md)
- [Current status](status.md)
- [Quality scorecard](quality.md)

## Current-reference documentation

- [Architecture overview](architecture/overview.md) — modules, layer direction,
  host boundary, and clearly labelled pending migrations
- [Data model](architecture/data-model.md) — runtime/data and
  `ToData`/`LoadFromData` patterns
- [Component guides](architecture/components/) — current package/module seams;
  verify their update/confidence header and code before relying on details
- [How-to guides](how-to/) — task instructions
- [Status](status.md) and [quality](quality.md) — current health summaries

## Permanent architecture history

- [Architecture Decision Records](adr/README.md) — binding decisions unless
  superseded by a later ADR; do not delete or rewrite history
- [Journey notes](journey/README.md) — exploration and rationale; not necessarily
  the current API

## Historical and proposed material

- [`plans/`](plans/) contains implementation plans from particular moments.
  Some shipped, some were superseded, and snippets may no longer compile.
- [`ideas/`](ideas/) contains live toolkit-scoped idea records: approved
  `design.md`, executable `plan.md`, and post-merge `implementation.md`. The
  idea PR remains open through implementation; a design title alone does not
  mean the API exists.
- [`archive/`](archive/) is historical reference only.

Monster behavior documents now carry explicit status banners. Start from the
[current monster README](../rulebooks/dnd5e/monster/README.md), not a historical
plan.

## Adding documentation

- Add an ADR for a durable architectural decision; retain superseded ADRs and
  mark the superseding record.
- Add a journey note for exploration/rationale that future contributors need.
- Add a how-to for a repeatable current task, with exact paths and commands
  verified against the code.
- Put current goals, non-goals, seams, load/persistence ownership, and targeted
  history links in the nearest real module/package README or AGENTS file.
- Do not create new archive material, duplicate role policy, or present a plan
  as current behavior.
