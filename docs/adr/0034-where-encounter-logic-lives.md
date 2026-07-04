# ADR-0034: Where Encounter Logic Lives — It Is the D&D 5e Encounter, and the Structure Will Say So

Date: 2026-07-04

## Status

**Accepted** — decided by Kirk (architect/owner) on 2026-07-04. The alternatives
prepared for the decision are preserved under *Options considered*. The decision
records a **structural move that executes later** (a focused restructure after
Beat 2 closes); this PR is docs-only and moves no code.

## Context

### The value this decision serves

> "We are here to build composable, extensible architecture that provides a
> strong foundation for our idea to evolve on."

Not to make things *appear* correct by declaration. A module named `encounter`
sitting at the top level while being secretly D&D-5e-specific is a **structural
lie** — the same species of untruth as the `overview.md` claim that "nothing
depends on `rulebooks/dnd5e`" (which was false the moment the encounter imported
it). Naming a thing what it is, in the structure and not just in a doc comment,
is the point. This ADR does not merely *describe* the boundary — it moves the
code so the layout tells the truth.

### The divergence this ADR resolves

The remembered model was *"encounter logic lives in the rulebook."* The code
diverged: a top-level `encounter` module **imports `rulebooks/dnd5e`**. No prior
ADR made this a first-class decision — ADR-0030 ratified the import *direction*
in passing while curing the #684 double-subscribe bug, and Journey 050 records an
`EntityHydrator` abstraction that was drafted and **rejected** in favour of
precedent. The question *"where SHOULD encounter logic live?"* was never asked on
its own. This ADR asks it and records the owner's answer.

### What the `encounter` module actually contains

Inventoried by responsibility, split by whether the file imports
`rulebooks/dnd5e`:

**Rulebook-coupled — the verbs (import `rulebooks/dnd5e`):**

| File | Responsibility |
|---|---|
| `encounter.go` | Aggregate root; holds hydrated combatants; imports `dnd5eEvents` |
| `combat.go` | Initiative roll + turn loop (`SetMode`, `EndTurn`, `TakeAction` dispatch); publishes `dnd5eEvents.TurnEndTopic` |
| `combat_phased.go`, `combat_resolver.go` | Two-phase attack resolver seam |
| `move_resolver.go` | Movement resolver seam |
| `hydration.go` | The `LoadFromData` cascade (ADR-0030) — hydrates dnd5e character/monster, holds them on the encounter |
| `take_character_action.go` | Delegates non-attack verbs to the character's own `ActivateAbility` / `ExecuteAction` (ADR-0032) |
| `turn_economy.go` | Seeds the action economy via `character.StartTurn` |
| `turn_state.go` | `ActorTurnState` menu projection (`AvailableAbilities` / `AvailableActions`) |
| `npc.go` | NPC dispatch; loads monster |
| `activate_feature.go` | `ActivateFeature` verb; loads character |

**Rulebook-agnostic — the room (no dnd5e import):**

| File / package | Responsibility |
|---|---|
| `broker.go` | Event broker; the game-event timestamp authority (ADR-0031) |
| `transport.go`, `transport_inmem.go` | Pluggable per-player delivery (InMemory / Redis) |
| `perception/` | Vision projection — pure spatial functions, explicitly "no broker or transport dependencies" |
| `core/` | Encounter IDs + `Hex`/`HexSet` spatial primitives |
| `events/` | The sealed `EncounterEvent` spine (~23 concrete events) + `AudienceSet` viewer routing |
| `prompts.go` | Skill-check prompt machinery + the `CharacterResolver` host hook |
| `data.go`, `death.go` | `Data` serialization struct; death handling |

The shape is stark: **the "room" half (broker, transport, perception, core
spatial, event spine, audience routing) is already rulebook-agnostic; the dnd5e
coupling is concentrated in ~10 verb/hydration files.** The event spine
(`encounter/events`) — the substrate a future combat log reassembles from — is
itself agnostic, yet carries dnd5e ref strings (e.g. `ActionRef` =
`"dnd5e:action:attack"`) as **opaque data**. The room already speaks in generic
events; only the verb layer binds to rulebook types. This is what makes both
halves of the decision below clean: the whole is dnd5e (so it moves under the
rulebook), and the genuinely-general internals are already separable (so they
become primitives when pulled).

### The de facto boundary that had emerged (Beat 2)

Beats 1–2 (rpg-toolkit#697 / #727 / #728) deepened the coupling *productively*
and settled a working boundary the decision below ratifies:

- **RULEBOOK = what the rules say** — resolution math, condition behaviour,
  per-entity state, the checks/saves receipts (the `Breakdown`).
- **ENCOUNTER = the room where they happen** — the loop, positions, audiences,
  orchestration, event delivery.

Worked example: the encounter gathers `PassivePerception` through the
rulebook-owned `Combatant` interface (asks the rules, doesn't compute them) and
computes Hide **observer-sets** in the encounter layer (gate-endorsed: "who sees
whom" is spatial), while the Stealth-vs-Perception **verdict** stays
rulebook-side. *Audiences here, verdicts there.*

### The friction that decides the module question (Beat 2, three times)

Because `encounter` is a **separate Go module** that requires `rulebooks/dnd5e`
by pseudo-version, every rules/room change that spanned the two modules paid a
cross-module tax during Beat 2: stale `go.sum`, the bump-the-pseudo-version
ceremony, and squash-merge-SHA confusion (the pseudo-version pins a commit that
the squash rewrites). This happened three times. It is the concrete cost of a
seam that should be an internal package boundary, not a module boundary.

### The fossil record

`rulebooks/dnd5e/combat/turn_manager.go` is a self-contained turn orchestrator
(`StartTurn`, `EndTurn`, `Strike`, `Move`, `OffHandStrike`, `FlurryStrike`,
`UseAbility`, `GetAvailableAbilities`, `GetEconomy`) that publishes
`TurnStartTopic` and `TurnEndTopic`. Its only `NewTurnManager` callers are
`turn_manager.go` itself and its own tests — **nothing in `encounter`, and
nothing in rpg-api (verified zero live callers 2026-07-03), constructs it.** It
is the remnant of when the turn loop lived *rulebook-side*, before ADR-0030 /
ADR-0032 moved the loop into the encounter. It is almost certainly why the loop
was remembered as living in the rulebook: **it literally did, and this fossil is
still there.**

The fossil finished decaying during Beat 2. `combat.go` already published
`TurnEndTopic`, and `seedActorTurn` now publishes `TurnStartTopic` on `e.bus`
too (`encounter/turn_economy.go`, the turn-start revival landed via
rpg-toolkit#699 / #729). So the **encounter now owns both turn-boundary
publishes**, the four subscriber conditions (`Dodging`, `Helped`,
`RecklessAttack`, `Unconscious`) get their signal from it, and `TurnManager` is
**fully orphaned** — no publisher, no constructor, no live caller. The removal is
no longer sequence-sensitive: the post-Beat-2 restructure deletes it outright.

### The stale doc

`docs/architecture/overview.md` asserted *"Nothing depends on `rulebooks/dnd5e`.
It is the top of the dependency tree."* False: `encounter` depends on it.
Verified — `encounter/go.mod` is the **only** external module whose `go.mod` has
a `require` on `rulebooks/dnd5e`. `overview.md` is corrected and rebuilt around
two diagrams that show the **decided end-state** (encounter inside the rulebook),
with today's separate-module reality annotated "migration pending."

## Decision

**The `encounter` module is the Dungeons & Dragons 5th Edition encounter, and the
repository structure will say so.** The import arrow `encounter → rulebooks/dnd5e`
is correct and intentional; the fix is not to relabel it but to move the code so
the layout stops lying. Four parts:

1. **Move the encounter under the rulebook.** It relocates from the top-level
   `encounter/` to **`rulebooks/dnd5e/encounter`**. Declaring it dnd5e in a doc
   comment while leaving it top-level (the earlier draft's recommendation) was
   considered and **rejected**: a name that has to be footnoted to be understood
   is a structural lie. The structure carries the meaning.

2. **Merge it into the `rulebooks/dnd5e` Go module** (recommended; no hard
   blocker found — see the evaluation below), rather than keeping a separate
   `go.mod` at the new path. This deletes the cross-module pseudo-version dance
   that taxed Beat 2 three times and makes a rules-and-room change a single
   atomic commit in one module.

3. **Genuinely-general capability is built/extracted as PRIMITIVES, pulled by
   need.** Spatial/hex math already is one (`tools/spatial`); the **event broker
   is the named next candidate** (`encounter/broker.go` — general pub/sub +
   timestamp authority). Extraction is deferred until a consumer outside the
   dnd5e encounter pulls for it; the `mechanics/effects` → conditions pattern is
   the template (general primitive, rulebook-specific expression). The
   general-vs-D&D-shaped line for every internal is in the table below.

4. **Timing is part of the decision.** The move executes as its **own focused
   restructure with a written migration plan, immediately after Beat 2 closes**
   (playtest + retro) — **never mid-wave.** This ADR records the decision and the
   migration sketch (see *Consequences*); it does not move code. The
   consequence to state plainly: **rpg-api's product surface *is* the dnd5e
   encounter SDK, period** — get the encounter boundary right and the API and web
   fall into place as thin consumers.

### The module-merge evaluation (decision part 2, costed honestly)

Given the move (part 1), the encounter can either stay a **separate module** at
`rulebooks/dnd5e/encounter/go.mod` or **merge into the `rulebooks/dnd5e`
module**. Recommendation: **merge.**

- **No hard blocker.** `rulebooks/dnd5e` does not import `encounter` (the arrow
  runs one way), so folding encounter's packages in creates **no import cycle**.
  Encounter's external deps (`core`, `dice`, `events`, `tools/spatial`) are a
  **subset** of what `rulebooks/dnd5e` already requires — the merged module gains
  no new external dependency. Encounter's subpackages (`core`, `events`,
  `perception`) become `rulebooks/dnd5e/encounter/{core,events,perception}`.
- **What merging buys:** the pseudo-version dance dies (no bump ceremony, no
  stale `go.sum`, no squash-SHA confusion); a rules+room change is one atomic
  commit; the layout states "encounter is part of the D&D 5e rulebook."
- **Honest costs of merging:**
  - *Coarser CI path-gating* — a change touching only `encounter/**` now triggers
    the whole `rulebooks/dnd5e` module's test job, not an encounter-only job.
  - *Bigger module test surface* — the dnd5e module's test run absorbs the
    encounter suite (real broker/transport/perception tests), lengthening it.
  - *rpg-api import migration* — `github.com/KirkDiggler/rpg-toolkit/encounter*`
    → `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter*`. rpg-api is
    the **sole** external consumer; the change is mechanical (import rewrite +
    dropping the separate `require`).
- **Keeping a separate module at the new path** would preserve fine-grained CI
  gating and independent versioning, but at the price of *retaining* the exact
  cross-module pseudo-version tax this decision exists to remove — so it is not
  recommended. It stays available as the fallback if the merged module's CI/test
  surface proves painful in practice.

### The primitive line: general vs D&D-shaped (decision part 3)

| Internal | Classification | Where it lives / goes |
|---|---|---|
| Hex / spatial math | **General primitive (already extracted)** | `tools/spatial`; encounter's `core/Hex` is a slice-local copy pending consolidation there |
| Typed event bus | **General primitive (already extracted)** | `events` module |
| Effect infrastructure | **General primitive (already extracted)** | `mechanics/effects`; dnd5e conditions are its rulebook-specific expression |
| Event broker (pub/sub + timestamp authority) | **General — NAMED next extraction candidate** | `encounter/broker.go` today; the next primitive to pull when a non-encounter consumer needs it |
| Transport (per-player delivery) | **General, extractable when pulled** | `encounter/transport*.go` |
| Perception / vision projection | **General (spatial), extractable when pulled** | `encounter/perception/`; candidate to join `tools/spatial` |
| Audience / viewer routing | **General, extractable when pulled** | `encounter/events/AudienceSet` |
| Event-spine mechanism (sealed interface + meta) | **General mechanism, dnd5e content** | stays with the encounter; the *shape* is generic, the *events* are dnd5e |
| Turn loop + tick publishes | **D&D-shaped** | the dnd5e encounter |
| Hydration cascade | **D&D-shaped** | the dnd5e encounter (ADR-0030) |
| Resolver seam (attack / move) | **D&D-shaped** | the dnd5e encounter |
| Verb dispatch (`TakeAction`) + economy seeding | **D&D-shaped** | the dnd5e encounter (ADR-0032) |
| NPC dispatch, turn-state menu | **D&D-shaped** | the dnd5e encounter |

The "extractable when pulled" rows are **not** work items now (part 4: pulled by
need, never on speculation). They are the map of where the general/specific seam
runs *inside* today's encounter, so a future extraction is a lift-and-name rather
than a rediscovery. Merging into the dnd5e module (part 2) does **not** foreclose
these extractions — a general primitive is lifted *down* to its own lower-layer
module (e.g. broker → a `events`-adjacent module), which is orthogonal to the
encounter living inside the rulebook.

## Options considered

### Option A — Encounter logic moves INTO `rulebooks/dnd5e`, loop and infrastructure and all (the remembered model)
The rulebook owns the game loop *and* the broker/transport/perception, with those
generic pieces welded into the rules package. **Rejected as stated** — a rulebook
package that owns Redis delivery and per-player projection is *rules + server*.
Note the chosen decision is the disciplined form of A's instinct: the encounter
moves *under* the rulebook (part 1) and can even share its module (part 2), but
the genuinely-general internals are pulled *out* as primitives (part 3), not
buried in the rules.

### Option B — Keep `encounter` top-level, *declare* it dnd5e (an earlier draft's recommendation)
Fix the story, not the structure: a doc comment + this ADR + diagrams announce
"this is the dnd5e encounter," module stays where it is. **Rejected.** It leaves
the structural lie in place (a top-level generic-sounding module that is actually
dnd5e) and keeps the cross-module pseudo-version tax. Declaration is not
architecture.

### Option C — Invert to a generic engine (pluggable rulebook behind interfaces)
The deferred `EntityHydrator` direction (Journey 050): the encounter depends on
an abstract rules seam, dnd5e is injected. **Deferred, not chosen.** It
re-introduces exactly the abstraction the ADR-0030 hydration cascade *deleted*,
pays indirection at every call site, and would be designed against a sample size
of one. The trigger that would justify reopening it is external: a real second
rulebook — at which point the general primitives (part 3) are the seams that
second rulebook also composes.

## Consequences

### Positive
- The structure stops lying: the encounter's location states it is D&D 5e, and
  `overview.md`'s diagrams show that end-state. No footnote required to read the
  layout correctly.
- Merging the module (recommended) ends the cross-module pseudo-version dance —
  rules+room changes become atomic, and the three Beat-2 friction classes (stale
  `go.sum`, bump ceremony, squash-SHA confusion) disappear.
- The rules-vs-room boundary becomes a **named, enforceable rule** inside one
  module; the primitive line is drawn so broker/perception extraction is a
  lift-and-name.
- rpg-api's product surface is settled: it is the dnd5e encounter SDK.

### Negative
- Coarser CI path-gating and a bigger dnd5e module test surface (costed above);
  the separate-module fallback exists if this bites.
- The move is a real restructure with a consumer (rpg-api) migration — accepted,
  mechanical, and deliberately scheduled off the critical path (post-Beat-2).
- The rules-vs-room and general-vs-D&D lines remain judgment boundaries, not
  compiler-enforced ones; they depend on this ADR + the overview diagrams staying
  legible (the diagram-maintenance rule in this PR is the mitigation).

### The migration sketch (executed post-Beat-2, its own PR + written plan)
- **New import paths:** `.../encounter` → `.../rulebooks/dnd5e/encounter`;
  `.../encounter/core|events|perception` → `.../rulebooks/dnd5e/encounter/…`.
- **Module change:** delete `encounter/go.mod`; its packages join the
  `rulebooks/dnd5e` module; the `encounter → rulebooks/dnd5e` cross-module
  `require` + pseudo-version is removed.
- **rpg-api (sole consumer):** rewrite imports; drop the separate `encounter`
  `require`; pin only `rulebooks/dnd5e`. Mechanical.
- **Tag/versioning:** the encounter is no longer separately tagged — it rides
  `rulebooks/dnd5e` versions. This is the point (the dance dies).
- **TurnManager deletion folded in:** the `TurnStart` handoff already landed in
  Beat 2 (rpg-toolkit#699 / #729), so the fossil is fully orphaned today; the
  restructure simply deletes `rulebooks/dnd5e/combat/turn_manager.go` (+ its
  tests).
- **Diagrams:** the module map + rules-vs-room diagrams update in the **same PR**
  as the move (the maintenance rule), flipping "migration pending" to done.
- **Timing gate:** the restructure starts only after Beat 2's playtest + retro
  close. Never mid-wave.

### Neutral
- Option C stays the named evolution; its trigger (a second rulebook) is recorded.
- Extraction of the named broker primitive (part 3) is independent of the move and
  is pulled by a real consumer, not scheduled here.

## Related

- ADR-0030 — encounter owns combatant hydration (ratified the import direction in
  passing; this ADR makes it a first-class decision).
- ADR-0031 / ADR-0032 — the event spine and the TakeAction unification, both of
  which deepened the encounter → rulebook coupling coherently.
- Journey 050 — the `EntityHydrator` that was drafted and rejected (the Option C
  seed).
- rpg-toolkit#596 — the parked `combat-ended` tick design (next tick-vocabulary need).
- rpg-toolkit#699 / #729 — the turn-start revival that moved `TurnStart`
  publishing into the encounter and fully orphaned the fossil.
- rpg-project#75 — the chapter ledger tracking this decision.
