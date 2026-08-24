# ADR-0034: Where Encounter Logic Lives — Split the Encounter Along Its Seam

Date: 2026-07-04

## Status

**Superseded in implementation (2026-08-23).** The proposed split was never
performed. The active `rulebooks/dnd5e/encounter`, resolution, and session stack
was built separately; rpg-api moved to that stack in PRs #801/#804; and
rpg-toolkit#1215 deleted the now-unconsumed top-level module. The original
reasoning and alternatives remain below as history.

The general rule survives: a package that mixes reusable infrastructure with
one game's rules has a false boundary. The superseding implementation chose a
clean new composition and host seam rather than relocating the old module's two
halves.

## Context

### The value this decision serves

> "We are here to build composable, extensible architecture that provides a
> strong foundation for our idea to evolve on."

Not to make things *appear* correct by declaration. A module named `encounter`
sitting at the top level while being specific to D&D 5e is a **structural lie** — the
same species of untruth as the `overview.md` claim that "nothing depends on
`rulebooks/dnd5e`" (false the moment the encounter imported it). This ADR does not
merely *describe* the boundary; it commits the structure to change so the layout
tells the truth.

### The divergence this ADR resolves

The remembered model was *"encounter logic lives in the rulebook."* The code
diverged: a top-level `encounter` module **imports `rulebooks/dnd5e`**. No prior
ADR made this a first-class decision — ADR-0030 ratified the import *direction* in
passing while curing the #684 double-subscribe bug, and Journey 050 records an
`EntityHydrator` abstraction that was drafted and **rejected** in favour of
precedent. The question *"where SHOULD encounter logic live?"* was never asked on
its own. This ADR asks it and records the owner's answer.

### What the `encounter` module actually contains

Inventoried by responsibility, split by whether the file imports
`rulebooks/dnd5e`. **This inventory is the whole argument:** the module is already
two things.

**Rulebook-coupled — the dnd5e loop (imports `rulebooks/dnd5e`):**

| File | Responsibility |
|---|---|
| `encounter.go` | Aggregate root; holds hydrated combatants; imports `dnd5eEvents` |
| `combat.go` | Initiative roll + turn loop (`SetMode`, `EndTurn`, `TakeAction` dispatch); publishes `dnd5eEvents.TurnEndTopic` |
| `combat_phased.go`, `combat_resolver.go` | Two-phase attack resolver seam |
| `move_resolver.go` | Movement resolver seam |
| `hydration.go` | The `LoadFromData` cascade (ADR-0030) — hydrates dnd5e character/monster, holds them |
| `take_character_action.go` | Delegates non-attack verbs to the character's own `ActivateAbility` / `ExecuteAction` (ADR-0032) |
| `turn_economy.go` | Seeds the action economy via `character.StartTurn`; publishes `TurnStartTopic` |
| `turn_state.go` | `ActorTurnState` menu projection (`AvailableAbilities` / `AvailableActions`) |
| `npc.go` | NPC dispatch; loads monster |
| `activate_feature.go` | `ActivateFeature` verb; loads character |
| `prompts.go`, `data.go`, `death.go` | Skill-check prompt verbs + `CharacterResolver` hook; `Data` serialization; death handling |

**Rulebook-agnostic — the infrastructure (no dnd5e import):**

| File / package | Responsibility |
|---|---|
| `broker.go` | Event broker; the game-event timestamp authority (ADR-0031) |
| `transport.go`, `transport_inmem.go` | Pluggable per-player delivery (InMemory / Redis) |
| `perception/` | Vision projection — pure spatial functions, explicitly "no broker or transport dependencies" |
| `core/` | Encounter IDs + `Hex`/`HexSet` spatial primitives |
| `events/` | The sealed `EncounterEvent` spine (~23 concrete events) + `AudienceSet` viewer routing |

The dnd5e coupling is concentrated in the loop files; the infrastructure half
imports no dnd5e at all. The event spine even carries dnd5e ref strings (e.g.
`ActionRef` = `"dnd5e:action:attack"`) only as **opaque data** — the *mechanism*
(sealed interface + audience routing) is generic, the *content* is dnd5e. The
seam is already there in the file layout; the decision makes the structure honour
it.

### The de facto boundary that had emerged (Beat 2)

Beats 1–2 (rpg-toolkit#697 / #727 / #728) deepened the coupling *productively* and
settled a working boundary: the **rulebook** owns what the rules *say* (resolution
math, condition behaviour, per-entity state, the `Breakdown` receipts); the
**loop** owns the *room* (turn order, positions, audiences, orchestration, event
delivery). Worked example: the loop gathers `PassivePerception` through the
rulebook-owned `Combatant` interface (asks the rules, doesn't compute them) and
computes Hide **observer-sets** in the loop (spatial), while the
Stealth-vs-Perception **verdict** stays rulebook-side. *Audiences here, verdicts
there.*

### The friction that decides the module question (Beat 2, three times)

Because `encounter` is a **separate Go module** requiring `rulebooks/dnd5e` by
pseudo-version, every change that spanned rules and loop paid a cross-module tax:
stale `go.sum`, the bump-the-pseudo-version ceremony, and squash-merge-SHA
confusion (the pseudo-version pins a commit the squash rewrites). This happened
three times in Beat 2. It is the concrete cost of a seam that should be an
*internal package boundary*, not a module boundary.

### The fossil record (now fully orphaned)

`rulebooks/dnd5e/combat/turn_manager.go` is a self-contained turn orchestrator
(`StartTurn`, `EndTurn`, `Strike`, `Move`, `UseAbility`, …) that publishes
`TurnStartTopic` / `TurnEndTopic`. Its only `NewTurnManager` callers are itself
and its own tests — nothing in the encounter, and nothing in rpg-api (verified
zero live callers 2026-07-03), constructs it. It is the remnant of when the turn
loop lived *rulebook-side*, before ADR-0030 / ADR-0032 moved the loop into the
encounter — almost certainly why the loop was remembered as living in the rulebook
(**it literally did**). The turn-start revival landed in Beat 2 —
**rpg-toolkit#699 merged as PR #729 (2026-07-04)** — so `seedActorTurn` now
publishes `TurnStartTopic` and the encounter owns **both** turn-boundary publishes.
`TurnManager` has no publisher, no constructor, and no live caller: it is **fully
orphaned**, and its deletion is a clean line-item of the split migration (no
sequencing hazard remains).

### The stale doc

`docs/architecture/overview.md` asserted *"Nothing depends on `rulebooks/dnd5e`"*
— false: `encounter` depends on it (verified: `encounter/go.mod` is the **only**
external module whose `go.mod` has a `require` on `rulebooks/dnd5e`; the rulebook's
own `go.mod` merely declares its module path). PR #730 corrected that claim and
added the diagrams; this amendment updates both diagrams to the **split**
end-state, with today's layout annotated "migration pending."

## Decision

**Split the encounter along the seam the inventory already found.** It is not one
thing pretending to be generic — it is a **dnd5e game loop** plus a set of
**already-agnostic infrastructure layers** sharing one module. The honest,
composable structure separates them:

1. **The dnd5e loop moves into `rulebooks/dnd5e`.** The rulebook-coupled files
   (the ~10 verb/hydration files plus `prompts.go`, `data.go`, `death.go`, and the
   concrete dnd5e event types) become an `encounter` package **inside** the
   rulebook module. Rules and loop then share one module and one version, and the
   cross-module `encounter → rulebooks/dnd5e` arrow — with the pseudo-version dance
   it forced three times — becomes an ordinary intra-module import.

2. **The agnostic layers are named as primitives now** — not "candidate,
   someday," but named with destinations (table below), extracted to their own
   lower-layer modules that the dnd5e loop *composes*. General infrastructure
   lives at the general layer where any future consumer (a second rulebook, a
   different host) can reuse it; the dnd5e specifics live with the rules. This is
   the composable, extensible foundation the value statement asks for.

3. **Executed post-Beat-2 as its own focused restructure with a written migration
   plan** — after the playtest and retro close, **never mid-wave.** This ADR
   records the decision and the migration sketch (see *Consequences*); it moves no
   code.

The consequence to state plainly: **rpg-api's product surface *is* the dnd5e
encounter SDK, period** — the loop's verbs, now rulebook-module-bound. Get the
encounter boundary right and the API and web fall into place as thin consumers.

### The primitive migration table (decision part 2)

Each agnostic layer, its destination, and whether the module is new or existing.
Proposed module names are finalized in the migration plan; the *destinations* are
decided.

| Agnostic layer (in `encounter` today) | Becomes | Destination |
|---|---|---|
| Event broker (`broker.go`) + Transport seam (`transport.go`, `transport_inmem.go`) | delivery primitive | **new `broker/` module** — pub/sub fan-out + game-event timestamp authority (ADR-0031) + the `Transport` interface and `InMemoryTransport`; composes `events` |
| Event-spine mechanism (`encounter/events`: sealed `EncounterEvent`, `eventMeta`/`Stamp`, `AudienceSet` viewer routing) | spine primitive (mechanism only) | **new `eventspine/` module** — the sealed-interface + audience routing mechanism; the concrete dnd5e event *types* travel with the loop into `rulebooks/dnd5e` |
| Perception (`perception/`: vision projection) | spatial primitive | **new `tools/perception/`** — composes `tools/spatial`, per the package's own stated extraction plan |
| Spatial glue (`encounter/core`: `Hex`, `HexSet`, IDs) | fold into existing primitive | **`tools/spatial`** — consolidate the slice-local hex copy that already exists there |

The already-extracted primitives this pattern follows: `tools/spatial` (hex),
`events` (typed bus), `mechanics/effects` (effect infrastructure, of which dnd5e
conditions are the rulebook-specific expression). The split extends the same
discipline to the infrastructure still trapped inside the encounter.

### The nested-module objection is answered by the split

An earlier draft worried that relocating the encounter *as a module* under
`rulebooks/dnd5e/` would nest a module that **requires** `rulebooks/dnd5e`
underneath `rulebooks/dnd5e` — inverting the intuitive "contains" reading. The
split dissolves this: the dnd5e-coupled files do **not** stay a separate nested
module — they become **part of** the `rulebooks/dnd5e` module (same `go.mod`), so
nothing is nested inside a module it depends on. The agnostic files that *do*
remain separate modules (`broker`, `eventspine`, `tools/perception`) sit **below**
`rulebooks/dnd5e`, which depends on them — the correct, non-inverted direction.
Merge-mechanics, not path-nesting.

## Options considered

### Declare-only — keep `encounter` top-level, *declare* it dnd5e in a doc
The first-merged draft's recommendation: a doc comment + this ADR + diagrams
announce "this is the dnd5e encounter," code stays put. **Rejected — structural
dishonesty.** A name that needs a footnote to be read correctly is the lie this
ADR exists to end, and it keeps the cross-module pseudo-version tax. Declaration
is not architecture.

### Whole-module-merge — move the *entire* `encounter` module under `rulebooks/dnd5e` and merge it in
Relocate every encounter file (loop **and** broker/transport/perception/spine)
into the `rulebooks/dnd5e` module. **Rejected — the opposite lie.** It kills the
pseudo-version dance, but it drags genuinely-general infrastructure *under the
rulebook*, burying reusable primitives inside D&D-specific code. Declare-only
lies by calling dnd5e code generic; whole-merge lies by calling generic code
dnd5e. The split is the only option that tells the truth on both sides.

### Option A — Loop *and infrastructure* welded into the rulebook as rules code
A stronger form of whole-merge: not just co-located but treated as rules
(broker/transport authored as dnd5e concerns). **Rejected** — a rulebook that owns
Redis delivery and per-player projection is *rules + server*.

### Option C — Invert to a generic engine (pluggable rulebook behind interfaces)
The deferred `EntityHydrator` direction (Journey 050): the loop depends on an
abstract rules seam, dnd5e is injected. **Deferred, not chosen.** It re-introduces
exactly the abstraction the ADR-0030 hydration cascade *deleted*, pays indirection
at every call site, and would be designed against a sample size of one. Its
trigger is external — a real second rulebook — at which point the named primitives
(part 2) are the seams that second rulebook also composes.

### The split — **CHOSEN**
dnd5e loop into the rulebook module; agnostic infrastructure out as named
primitives. Honest on both sides, kills the pseudo-version tax, and lays the
composable foundation.

## Consequences

### Positive
- The structure stops lying in **both** directions: dnd5e code lives with the D&D
  rules, generic infrastructure lives at the general layer where anything can
  reuse it.
- The rules+loop module merge ends the cross-module pseudo-version dance — a
  rules-and-loop change is one atomic commit, and the three Beat-2 friction
  classes (stale `go.sum`, bump ceremony, squash-SHA confusion) disappear.
- The agnostic layers become **real, reusable primitives** (`broker`,
  `eventspine`, `tools/perception`, consolidated `tools/spatial`) — the toolkit's
  composable foundation grows instead of staying trapped in the encounter.
- rpg-api's product surface is settled: it is the dnd5e encounter SDK.

### Negative
- A larger restructure than a rename: it touches module boundaries, creates new
  primitive modules, and migrates rpg-api's imports. Accepted, and deliberately
  scheduled off the critical path (post-Beat-2) with a written plan.
- Coarser CI path-gating and a bigger `rulebooks/dnd5e` test surface once the loop
  merges in (its real-broker/transport tests now run under the dnd5e module).
- The rules-vs-loop and general-vs-dnd5e lines remain judgment boundaries, not
  compiler-enforced ones; they depend on this ADR + the overview diagrams staying
  legible (the diagram-maintenance rule is the mitigation).

### The migration sketch (executed post-Beat-2, its own PR + written plan)
- **Loop → rulebook module:** move the rulebook-coupled files into
  `rulebooks/dnd5e/encounter`; delete `encounter/go.mod`; the cross-module
  `require` + pseudo-version is removed.
- **Primitives out:** create `broker/`, `eventspine/`, `tools/perception/`; fold
  `encounter/core` hex into `tools/spatial`; the dnd5e loop and the rulebook
  depend on these downward.
- **rpg-api (sole consumer):** rewrite imports — `.../encounter*` splits into
  `.../rulebooks/dnd5e/encounter` (loop) + the new primitive modules; drop the
  separate `encounter` require. Mechanical.
- **Tag/versioning:** the loop rides `rulebooks/dnd5e` versions (the dance dies);
  each new primitive is versioned as its own lower-layer module.
- **`TurnManager` deletion folded in:** the fossil is already fully orphaned
  (#699/#729), so the restructure deletes `rulebooks/dnd5e/combat/turn_manager.go`
  (+ its tests) outright — a named line-item, no sequencing hazard.
- **Diagrams:** the module map + rules-vs-room diagrams update in the **same PR**
  as the move (maintenance rule), flipping "migration pending" to done.
- **Timing gate:** starts only after Beat 2's playtest + retro close. Never
  mid-wave.

### Neutral
- Option C stays the named evolution; its trigger (a second rulebook) is recorded,
  and the part-2 primitives are the seams it would reuse.
- `combat-ended` tick is designed in the parked rpg-toolkit#596 doc and built when
  the loop needs it.

## Related

- ADR-0030 — encounter owns combatant hydration (ratified the import direction in
  passing; this ADR makes it a first-class decision).
- ADR-0031 / ADR-0032 — the event spine and the TakeAction unification, both of
  which deepened the coupling coherently.
- Journey 050 — the `EntityHydrator` that was drafted and rejected (the Option C
  seed).
- rpg-toolkit#596 — the parked `combat-ended` tick design.
- rpg-toolkit#699 (merged as PR #729, 2026-07-04) — the turn-start revival that
  fully orphaned the fossil.
- rpg-project#75 — the chapter ledger tracking this decision.

### Amendment (2026-07-13): #757 executes part of this ADR's intent early, without the module split

"The walled room" (rpg-toolkit#757) gives `encounter` a real dependency on
`tools/spatial` + `tools/environments` (`Data.Space`, wall-aware LoS,
wall-blocked movement — see
[encounter.md](architecture/components/encounter.md#walled-rooms-wall-aware-los-and-inline-combat-entry-rpg-toolkit757)).
This is directionally what "Primitives out: ... fold `encounter/core` hex
into `tools/spatial`" describes above, but it is **not** that migration:
`encounter/core.Hex` still exists as its own type (bridged to
`spatial.CubeCoordinate`/`spatial.Position` via new converter methods, not
replaced by them), `encounter/go.mod` is untouched as a module boundary, and
none of the migration sketch's other steps (rulebook-coupled files moving to
`rulebooks/dnd5e/encounter`, `broker`/`eventspine`/`tools/perception`
extraction, rpg-api import rewrite) happened. The timing gate above ("starts
only after Beat 2's playtest + retro close... never mid-wave") still governs
the actual split — this PR only adds a new downward dependency edge the
split will eventually have to account for, it doesn't pull the split
forward.
