# Decisions — cliffnotes

**Read this file. Do not read the ADR corpus.**

The ADR corpus is a large context load and much of it is history: superseded
shapes, proposals never built, and narrative that made sense at the time. Loading
it wholesale costs a lot and imports baggage — the danger is not the tokens, it
is reasoning from a decision that no longer holds.

This is the compressed form: the decision, and the **rule** it generalises to,
for each. Open the full ADR only when you are about to *contradict* one, or need
the reasoning behind a specific trade-off.

**Maintenance:** every ADR file must have an entry here — enforced by
`scripts/check-decisions.sh`, so a new ADR fails CI until it is listed. Written
because this directory's `README.md` index silently drifted to listing 7 of 37.

---

## Layering and boundaries

- **0002** — Hybrid architecture: module organisation + event-bus communication +
  interfaces. Deliberately not pure ECS, not event sourcing.
- **0008** — `tools/` holds infrastructure that supports game mechanics but is not
  one. *Rule: infrastructure and rules live in different places.*
- **0023** — `core` provides types and contracts only; rulebooks implement.
  *Rule: the shared layer defines vocabulary, never behaviour.*
- **0021** — Actions are an internal rulebook pattern, not public API.
- **0022** — `Repository (data) → Loader (domain objects) → Orchestrator (workflow)`.
  *Rule: reconstitution is its own layer; repositories trade in data.*
- **0034** — The encounter was split along its real seam: the dnd5e game loop moved
  into `rulebooks/dnd5e`, agnostic infrastructure stayed. *Rule: when one module is
  "generic" plus "one game's rules", that is two modules.*

## Events and the bus

- **0024** — Events are plain data structs on typed topics. *Rule: no interface
  ceremony on an event; it is a payload.*
- **0001** — Modifiers return a typed `ModifierValue`, never `interface{}`.
- **0025** — `gamectx` carries typed registries of game state on `context.Context`,
  so conditions can reach what they need without a global.
- **0031** — The encounter spine stamps `OccurredAt` + `CorrelationID` on every
  event via one embedded `eventMeta`. *Rule: single-source the metadata so no
  event can forget it.*
- **0033** — `TurnStateChangedEvent` push-refreshes the menu/economy, carrying a
  **rulebook-agnostic** flattened snapshot. *Rule: the spine must not import
  rulebook types.*

## Entities, features, conditions

- **0003** — Conditions are entities and participate in the event system.
- **0004** — One generic `RelationshipManager`; concentration is just one
  relationship type. *Rule: do not promote a single instance to a first-class
  concept.*
- **0005** — Effects compose from shared behavioural components while staying
  distinct domain types.
- **0006** — Features: a registry for definitions, entity-centric storage for
  ownership. No central who-has-what table.
- **0007** — `ProcessRestoration(trigger)` replaced `ProcessShortRest`/
  `ProcessLongRest`. *Rule: one generic trigger beats an enumerated method per
  case.*
- **0020** — The `Feature` interface carries only what features actually do,
  keyed by `*core.Ref`.
- **0028** — Three domain types with distinct jobs: Feature (grants), Action
  (does), Condition (listens).
- **0030** — `Encounter.LoadFromData` owns combatant hydration by cascading the
  standard `ToData`/`LoadFromData` round-trip — no injected "hydrator". *Rule:
  apply the existing serialise↔runtime pattern one level up rather than inventing
  a subsystem.*

## Combat

- **0026** — Damage is two-phase: **Resolve** (chain gathers modifiers) → **Apply**
  (mutate) → **Notify**. Lives in the toolkit.
- **0027** — Attack resolution is three-phase with **reaction windows between
  phases**.
- **0029** — Type enums over booleans for mutually exclusive states. *Rule: if a
  thing can only be one of N, do not model it as N booleans.*
- **0032** — The character package's dispatch is canonical; the encounter
  delegates rather than building a parallel registry. *Rule: default to one
  system.*
- **0036** *(superseded by [0040-composable-attack-damage.md](0040-composable-attack-damage.md))* —
  The proposed selective-critical variant is historical; it does not describe
  the current attack damage rules.
- **0040** — Attack damage is an ordered collection of typed pools. Exactly one
  pool receives the attack ability modifier; every eligible attack die,
  including Sneak Attack, doubles on a critical unless that pool explicitly has
  `DoesNotCrit`. Resolution rolls pools, folds one chain, and applies once.

## Spatial and world

- **0009** — Multi-room orchestration extended `tools/spatial` rather than adding
  a module. *Rule: extend the owner of the concept.*
- **0015** — Room connections are **logical relationships**, not spatial
  constraints. Precise spatial connections are opt-in.
- **0011** — Environment generation is graph-based first.
- **0012** — `tools/selectables` is generic weighted selection over any type.
- **0013** — The spawn engine does **not** create entities; clients supply
  categorised pools. *Rule: infrastructure, not implementation.*
- **0014** — Selectables choose generation **parameters**, they do not replace
  generation.
- **0016** — Behaviour is pluggable, with typed memory keys.
- **0035** — Canvas compilation produces one canonical sorted floor mask plus
  envelope; reload validates the exact started snapshot.

## Session SDK

- **0037** — Entity entry splits on **load-vs-instantiate**, not
  player-vs-monster: `Join` loads an existing entity the host owns, `Spawn` builds
  code-resident content from a ref. *Rule: **a ref names the package that can load
  the data** — so a host-owned character has no ref, because claiming one would be
  false. Also: an option that "fits" is not thereby true.*
- **0038** — **Resolution owns the bus**: one package creates it per
  interaction, attaches every participant's effects through an instrumented
  surface, and the bus dies with the call. Rules packages (`combat`,
  `character`) never see it — every verb is a step machine over data yielding
  a **sealed vocabulary** (`Gather | Pose | Request | Done`) that resolution
  drives. Effects keep `Apply(ctx, bus)` unchanged; **granted** effects live
  on the beneficiary with a link to what kills them, **projected** effects
  never leave their owner. *Rule: the bus is wiring, and wiring lives in
  exactly one place — a package that needs the bus to construct is claiming
  wiring it does not own.*
- **0039** — **The save gate is data**: a contestable consequence declares
  `SaveGate{Abilities, DC, OnSuccess, Recurrence}` and resolution turns it
  into a `Request`; every save folds `SavingThrowChain`. **`DCSource` is a
  closed enum of named formulas** (`DCStatic`, `DCFivePlusDamageTaken`,
  `DCHalfDamageFloorTen`) — no function arm. *Rule: a new `DCSource` case must
  cite a RAW rule; 5e already closed this set, and we inherit their closure
  rather than inventing an open one.*
- **0040** — **The atlas says which way its hexes point**: `Atlas.Layout`
  (`pointy_top` / `flat_top`, present iff the grid is hex) is the render word,
  projected from the composition's authoring `Orientation` and deliberately not
  sharing its name — a client drew the tomb as a staircase because the two
  questions shared one word. *Rule: a wire field is named for the question the
  receiver asks, not the one the author answered.*

---

## Aspirational — proposed, never built

**Do not cite these as current architecture.** Each proposes a module that does
not exist in the repo (verified 2026-08-13). They are kept because ADRs are never
archived, and they are listed here so a reader recognises them as history rather
than absorbing them as fact.

- **0017** — World Manager under an `orchestrators/` hierarchy. `orchestrators/`
  does not exist.
- **0018** — Content Integration Orchestrator coordinating `tools/monsters` and
  `tools/items`. Neither exists.
- **0019** — Environment Orchestrator package. Does not exist.
- **0019_dnd5e_module_registry_system.md** — registry + modular content packages
  as separate Go modules. Partially realised in spirit by `refs/` and the module
  layout, but not as described.

## Numbering hazards

The corpus has collisions. When citing a number, cite the **filename**:

- **0006 twice** — `0006-feature-management-pattern.md` and
  `0006-generic-restoration-triggers.md`. The latter is superseded by
  `0007-generic-restoration-triggers.md`.
- **0019 twice** — `0019-environment-orchestrator.md` and
  `0019_dnd5e_module_registry_system.md`, and the second is *titled* "ADR-0014"
  internally.
- **0010 is missing.** Never written.

## Status fields are unreliable

Most ADRs still read "Proposed" or carry no status, including several that shipped
years of code. **Treat the status line as unmaintained** and judge currency by
whether the code exists — which is what the aspirational section above does.
Fixing them retroactively would mean guessing at intent; flagging them costs
nothing and misleads nobody.
