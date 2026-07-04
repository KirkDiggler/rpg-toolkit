# ADR-0034: Where Encounter Logic Lives — It Is the D&D 5e Encounter

Date: 2026-07-04

## Status

**Accepted** — decided by Kirk (architect/owner) on 2026-07-04. The alternatives
prepared for the decision are preserved below under *Options considered*. One
mechanical sub-choice (module placement) is left open for the owner on this PR;
everything else is decided.

## Context

### The divergence this ADR exists to resolve

The remembered model was *"encounter logic lives in the rulebook."* The code
diverged: a top-level `encounter` module **imports `rulebooks/dnd5e`**. No prior
ADR ever made this a first-class decision. ADR-0030 ratified the import
*direction* in passing while solving the #684 double-subscribe bug (its
"Boundary stance": "lean into the existing precedent, track a fully-agnostic
engine separately"), and Journey 050 records that an `EntityHydrator` abstraction
was drafted and **rejected** in favour of that precedent. The question *"where
SHOULD encounter logic live?"* had never been put on the table by itself. This
ADR puts it there and records the owner's answer.

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
events; only the verb layer binds to rulebook types.

### The de facto boundary that had emerged (Beat 2)

Beats 1–2 (rpg-toolkit#697 / #727 / #728) deepened the coupling *productively*
and, in doing so, settled a working boundary that the decision below now ratifies:

- **RULEBOOK = what the rules say** — resolution math, condition behaviour,
  per-entity state, the checks/saves receipts (the `Breakdown`).
- **ENCOUNTER = the room where they happen** — the loop, positions, audiences,
  orchestration, event delivery.

Worked example: the encounter gathers `PassivePerception` through the
rulebook-owned `Combatant` interface (asks the rules, doesn't compute them) and
computes Hide **observer-sets** in the encounter layer (gate-endorsed: "who sees
whom" is spatial), while the Stealth-vs-Perception **verdict** stays
rulebook-side. *Audiences here, verdicts there.*

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

The fossil is caught mid-decay, which makes its removal sequence-sensitive: on
`origin/main` the encounter has taken over publishing `TurnEndTopic`
(`combat.go`) but not yet `TurnStartTopic` — `seedActorTurn` seeds the economy
without publishing it. So `TurnManager` is currently the **only** live publisher
of `TurnStartTopic`, while four conditions (`Dodging`, `Helped`, `RecklessAttack`,
`Unconscious`) *subscribe* to it. The subscribers are live; the publisher is
orphaned. Moving `TurnStart` publishing into the encounter is tracked under
rpg-toolkit#699. **Deleting `TurnManager` therefore cannot precede that handoff**
(see *Consequences → follow-ups*).

### The stale doc

`docs/architecture/overview.md` asserted *"Nothing depends on `rulebooks/dnd5e`.
It is the top of the dependency tree."* That is **false**: `encounter` depends on
it. Verified — `encounter/go.mod` is the **only** external module whose `go.mod`
has a `require` on `rulebooks/dnd5e`. The module map in the same doc omitted the
`encounter` module entirely. This PR corrects overview.md and rebuilds it around
two architecture diagrams (a maintenance rule now travels with them so the map
cannot silently rot again).

## Decision

**The `encounter` module is the Dungeons & Dragons 5th Edition encounter. It is
rulebook-specific by design — these are D&D rules — not a generic engine wearing
a dnd5e coat.** The import arrow `encounter → rulebooks/dnd5e` is correct and
intentional. The owner's decision has four parts:

1. **Rulebook-specific, owned.** The encounter is the dnd5e game-loop SDK. It
   depends on `rulebooks/dnd5e` on purpose and is free to speak dnd5e directly.
   There is no obligation to keep it rulebook-agnostic, and the aspiration to a
   "fully agnostic engine" is explicitly **not** a goal of this design.

2. **General capability lives as PRIMITIVES beneath, composed by the dnd5e
   encounter.** Where a capability is genuinely general, it is built as a
   primitive at the layer where it truly belongs, and the dnd5e encounter
   *composes* it — it is not reinvented rulebook-side, and the encounter is not
   forced to stay generic to host it. The pattern already in the tree is the
   template: **`mechanics/effects` is the general form; conditions are the
   rulebook-specific expression of it.** Likewise hex/spatial math is a primitive
   (`tools/spatial`), and the typed event bus is a primitive (`events`). The
   *general vs D&D-shaped* line for every encounter internal is drawn in the
   table below.

3. **Extraction is deferred until pulled.** "We can always move it out when it
   makes sense." No generic-engine inversion now; no speculative interface seams.
   The revisit triggers are named below — a genuinely general internal is
   extracted to its own primitive module only when a real consumer outside the
   dnd5e encounter pulls for it (or a second rulebook does).

4. **Get the dnd5e encounter right first.** The known near-term needs are
   **conditions** and the **turn-tick vocabulary** — `TurnStart` / `TurnEnd` are
   live now; a `combat-ended` tick is designed in the parked rpg-toolkit#596 doc,
   not yet built. The consequence to state plainly: **rpg-api's product surface
   *is* the dnd5e encounter SDK, period.** If the encounter boundary is right,
   the API and web layers fall into place as thin consumers — the API
   orchestrates the encounter's verbs and projects its events; it does not
   re-derive rules.

### The primitive line: general vs D&D-shaped (decision part 2)

| Internal | Classification | Where it lives / goes |
|---|---|---|
| Hex / spatial math | **General primitive (already extracted)** | `tools/spatial`; encounter's `core/Hex` is a slice-local copy pending consolidation there |
| Typed event bus | **General primitive (already extracted)** | `events` module |
| Effect infrastructure | **General primitive (already extracted)** | `mechanics/effects`; dnd5e conditions are its rulebook-specific expression |
| Event broker (pub/sub + timestamp authority) | **General, extractable when pulled** | `encounter/broker.go` today; candidate primitive |
| Transport (per-player delivery) | **General, extractable when pulled** | `encounter/transport*.go`; candidate primitive |
| Perception / vision projection | **General (spatial), extractable when pulled** | `encounter/perception/`; candidate to join `tools/spatial` |
| Audience / viewer routing | **General, extractable when pulled** | `encounter/events/AudienceSet`; candidate primitive |
| Event-spine mechanism (sealed interface + meta) | **General mechanism, dnd5e content** | stays with the encounter; the *shape* is generic, the *events* are dnd5e |
| Turn loop + tick publishes | **D&D-shaped** | the dnd5e encounter |
| Hydration cascade | **D&D-shaped** | the dnd5e encounter (ADR-0030) |
| Resolver seam (attack / move) | **D&D-shaped** | the dnd5e encounter |
| Verb dispatch (`TakeAction`) + economy seeding | **D&D-shaped** | the dnd5e encounter (ADR-0032) |
| NPC dispatch, turn-state menu | **D&D-shaped** | the dnd5e encounter |

The "candidate primitive" rows are **not** work items for now (part 3: deferred
until pulled). They are the map of where the general/specific seam runs *inside*
today's encounter, so a future extraction is a lift-and-name rather than a
rediscovery.

### The one open sub-choice: module placement (for the owner on this PR)

Both mechanical sub-options honour the decision (encounter = dnd5e-owned). They
differ only in where the module physically sits:

- **Sub-option 1 — keep `encounter/` top-level, *declared* dnd5e (recommended).**
  The module stays at its current import path; `doc.go`, this ADR, and the
  overview diagrams declare it the dnd5e encounter SDK.
  - *Pro:* zero import-path churn — rpg-api keeps importing
    `github.com/KirkDiggler/rpg-toolkit/encounter`. The encounter is an
    orchestration module that sits *above* `rulebooks/dnd5e` and composes several
    lower modules (`tools/spatial`, `events`, and the rulebook); a module that
    *depends on* the rulebook reads oddly nested *inside* it.
  - *Con:* a top-level name `encounter/` still looks generic at a glance;
    legibility rests on the declaration (doc + diagrams), not the path.

- **Sub-option 2 — relocate under `rulebooks/dnd5e/` (e.g.
  `rulebooks/dnd5e/encounter/`).**
  - *Pro:* the location itself declares "this is dnd5e"; no separate top-level
    module implying generality.
  - *Con:* a breaking import-path change for every consumer (rpg-api, tests), and
    a module-boundary question — either it folds into the `rulebooks/dnd5e`
    module (losing independent versioning and inheriting the rulebook's full
    dependency/release surface) or it stays a separate `go.mod` nested inside the
    rulebook (unusual, and the import path still churns). Nesting a module that
    *requires* `rulebooks/dnd5e` underneath `rulebooks/dnd5e` also inverts the
    intuitive "contains" reading of the path.

**Recommendation: Sub-option 1.** Declare, don't relocate. The churn of
Sub-option 2 buys path-level self-documentation that the declaration + diagrams
already deliver, and physically nesting an above-the-rulebook orchestration
module inside the rulebook misrepresents the dependency direction. Relocation
stays available as a later, deliberate move if the top-level name proves
misleading in practice.

## Options considered

### Option A — Encounter logic moves INTO `rulebooks/dnd5e` (the remembered model)
The rulebook owns the game loop; rpg-api consumes the rulebook directly, no
separate encounter module. **Rejected.** The genuinely-generic "room" (broker,
transport, perception, event spine) is host-infrastructure a rulebook has no
business owning; folding it in welds transport/delivery into the rules layer and
inverts the layering (the rulebook becomes *rules + server*). The parts that are
"genuinely the rules" already live in `rulebooks/dnd5e`; the encounter is the
orchestration around them. (Note: Option A is about moving the *loop and its
infrastructure* into the rulebook. The chosen decision's Sub-option 2 — merely
relocating the already-coupled encounter module's *path* under `rulebooks/dnd5e/`
without dragging the generic infra into the rules — is a different, narrower
question left open above.)

### Option B — The encounter IS the dnd5e game-loop SDK, primitives beneath — **CHOSEN**
Keep the arrow; name it honestly; push genuinely-general internals down into
primitive layers as they are pulled. This is the decision recorded above.

### Option C — Invert to a generic engine (pluggable rulebook behind interfaces)
The deferred `EntityHydrator` direction (Journey 050), generalized: the encounter
depends on an abstract rules seam and dnd5e is injected. **Deferred, not chosen.**
It re-introduces exactly the abstraction the ADR-0030 hydration cascade *deleted*
("hydration is not a new subsystem — it is the existing `ToData`/`LoadFromData`
round-trip, composed"), paying indirection at every call site, and the interfaces
would be designed against a sample size of one (dnd5e) — almost certainly the
wrong shape. The trigger that would justify it is external: a real second
rulebook.

## Consequences

### Positive
- The map matches the territory: `overview.md` stops asserting a false invariant,
  documents the encounter's dnd5e dependency as intentional, and is rebuilt
  around two diagrams that make the module graph and the rules-vs-room seam
  legible at a glance.
- The rules-vs-room boundary becomes a **named, enforceable rule**: rule logic
  drifting into the encounter's generic half, or transport/positions drifting
  into the rulebook, are now nameable defects (Beat 2's Hide observer-set split
  is the worked example).
- The primitive line is drawn, so future extraction of broker/perception/audience
  is a lift-and-name, not a rediscovery.
- rpg-api's product surface is settled: it is the dnd5e encounter SDK. Getting
  the encounter boundary right is what makes the API and web thin.

### Negative
- Ratifies that the toolkit has a dnd5e-specific module *above* the rulebook, and
  that a second rulebook would need its own encounter (or a later Option C
  extraction). This is an accepted cost, not a surprise.
- The rules-vs-room and general-vs-D&D lines are judgment boundaries, not
  compiler-enforced ones; they depend on this ADR + the overview diagrams staying
  legible. The diagram-maintenance rule in this PR is the mitigation.

### Neutral / follow-ups
- **Delete the fossil `TurnManager`** — a named follow-up, **not** this PR (which
  is docs-only). Sequencing is mandatory: deleting it removes the last live
  publisher of `TurnStartTopic`, so it must land together with, or after, the
  encounter taking over `TurnStart` publishing (rpg-toolkit#699). Delete it first
  and the four subscriber conditions lose their turn-start signal.
- **Module placement** (Sub-option 1 vs 2) is the owner's call on this PR;
  Sub-option 1 (declare, don't relocate) is recommended and is the default if
  nothing else is said.
- **`combat-ended` tick** is designed in the parked rpg-toolkit#596 doc and built
  when the dnd5e encounter needs it (decision part 4).
- Option C stays the named evolution; its trigger (a second rulebook) is recorded
  so a future reader knows exactly when to reopen it.

## Related

- ADR-0030 — encounter owns combatant hydration (ratified the import direction in
  passing; this ADR makes it a first-class decision).
- ADR-0031 / ADR-0032 — the event spine and the TakeAction unification, both of
  which deepened the encounter → rulebook coupling coherently.
- Journey 050 — the `EntityHydrator` that was drafted and rejected (the Option C
  seed).
- rpg-toolkit#596 — the parked `combat-ended` tick design.
- rpg-toolkit#699 — moving `TurnStart` publishing into the encounter (unblocks the
  `TurnManager` deletion).
- rpg-project#75 — the chapter ledger tracking this decision.
