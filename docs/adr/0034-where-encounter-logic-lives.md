# ADR-0034: Where Encounter Logic Lives

Date: 2026-07-04

## Status

**Proposed.** This ADR is a decision instrument for the architect/owner (Kirk)
to ratify one of the options below. It records the honest inventory, the real
costs of each direction, and a recommendation — but **no code moves until it is
Accepted.** The only change shipped with it is a documentation correction that is
true regardless of which option wins (see *Housekeeping*).

## Context

### The divergence this ADR exists to resolve

The architect's mental model is *"encounter logic lives in the rulebook."* The
code says otherwise: a top-level `encounter` module **imports
`rulebooks/dnd5e`**. No prior ADR ever made this a first-class decision.
ADR-0030 ratified the import *direction* in passing while solving the #684
double-subscribe bug (its "Boundary stance" section: "lean into the existing
precedent, track a fully-agnostic engine separately"), and Journey 050 records
that an `EntityHydrator` abstraction was drafted and **rejected** in favour of
that precedent. The question *"where SHOULD encounter logic live"* was never put
on the table by itself. This ADR puts it there.

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
| `events/` | The sealed `EncounterEvent` spine (~23 concrete events) + `AudienceSet` |
| `prompts.go` | Skill-check prompt machinery + the `CharacterResolver` host hook |
| `data.go`, `death.go` | `Data` serialization struct; death handling |

The shape is stark: **the "room" half (broker, transport, perception, core
spatial, event spine, prompts) is already rulebook-agnostic; the dnd5e coupling
is concentrated in ~10 verb/hydration files.** Notably the event spine
(`encounter/events`) — the substrate a future combat log reassembles from — is
itself agnostic, yet carries dnd5e ref strings (e.g. `ActionRef` =
`"dnd5e:action:attack"`) as **opaque data**. The room already speaks in generic
events; only the verb layer binds to rulebook types.

### The de facto boundary that has emerged (Beat 2)

Beats 1–2 (rpg-toolkit#697 / #727 / #728) deepened the coupling *productively*
and, in doing so, settled a working boundary:

- **RULEBOOK = what the rules say** — resolution math, condition behaviour,
  per-entity state, the action economy/menu.
- **ENCOUNTER = the room where they happen** — the loop, positions, audiences,
  orchestration, event delivery.

Worked examples: the encounter gathers `PassivePerception` through the
rulebook-owned `Combatant` interface (asks the rules, doesn't compute them); it
computes Hide **observer-sets** in the encounter layer (gate-endorsed: "who sees
whom" is spatial), while the Stealth-vs-Perception **verdict** stays
rulebook-side. That split — *audiences here, verdicts there* — is the boundary
this ADR proposes to name.

### The fossil record

`rulebooks/dnd5e/combat/turn_manager.go` is a self-contained turn orchestrator
(`StartTurn`, `EndTurn`, `Strike`, `Move`, `OffHandStrike`, `FlurryStrike`,
`UseAbility`, `GetAvailableAbilities`, `GetEconomy`) that publishes
`TurnStartTopic` and `TurnEndTopic`. Its only `NewTurnManager` callers are
`turn_manager.go` itself and its own tests — **nothing in `encounter`, and
nothing in rpg-api (verified zero live callers 2026-07-03), constructs it.** It
is the remnant of when the turn loop lived *rulebook-side*, before ADR-0030 /
ADR-0032 moved the loop into the encounter. It is almost certainly why the
architect remembers turn logic living in the rulebook: **it literally did, and
this fossil is still there.**

The fossil is caught mid-decay, and the detail is load-bearing for sequencing:
on `origin/main` the encounter has taken over publishing `TurnEndTopic`
(`combat.go`) but **not** `TurnStartTopic` — `seedActorTurn` seeds the economy
without publishing it. So `TurnManager` is currently the **only** live publisher
of `TurnStartTopic`, while **four conditions** (`Dodging`, `Helped`,
`RecklessAttack`, `Unconscious`) *subscribe* to it. The subscribers are live;
the publisher is orphaned. (An in-flight branch, `feat/699-dodge-turnstart`,
closes the gap by having the encounter publish `TurnStartTopic` in
`turn_economy.go`.) **Deleting `TurnManager` therefore cannot precede the
encounter taking over `TurnStart` publishing** — see *Housekeeping*.

### The stale doc

`docs/architecture/overview.md` (updated 2026-05-23) asserts at line 57:
*"Nothing depends on `rulebooks/dnd5e`. It is the top of the dependency tree."*
That is **false**: `encounter` depends on it. Verified — `encounter/go.mod` and
`rulebooks/dnd5e/go.mod` are the only two `go.mod`s that require
`rulebooks/dnd5e`, so `encounter` is its sole external consumer. The module map
in the same doc omits the `encounter` module entirely. This is the concrete
artifact of the divergence, and it is corrected in this PR regardless of the
decision.

## Options considered

### Option A — Encounter logic moves INTO `rulebooks/dnd5e`

The architect's remembered model: the rulebook owns the game loop; rpg-api
consumes the rulebook directly, with no separate `encounter` module.

- **What is genuinely dnd5e** already sits in the ~10 verb files (turn
  semantics, resolvers, hydration cascade, economy/menu delegation, NPC
  dispatch). Moving *those* into the rulebook is a short trip — they already
  import it.
- **What is genuinely generic** is the problem: the broker, transport,
  perception, event spine, and prompts are **host-infrastructure**. A rulebook
  has no business owning Redis delivery or per-player event projection. So
  Option A forces a choice: (a) drag that infra *into* `rulebooks/dnd5e` —
  which welds transport/delivery concerns into the rules layer and makes them
  unreusable by any other consumer; or (b) split the infra into its own
  module(s) and move only the verbs — which is Option C's decomposition done
  eagerly and without the interface payoff.
- **rpg-api's import surface** does not shrink. It would import
  `rulebooks/dnd5e` for the loop *and* still import the agnostic infra module
  for transport/broker. Its reach into the rulebook grows.
- **What it breaks:** the spirit of the boundary rule ("toolkit implements
  rules, returns breakdowns"). A rulebook that also owns the turn loop, the
  broker, and per-player projection is no longer *the rules* — it is *the rules
  plus the server*. It also forecloses the toolkit-as-product framing: a second
  rulebook could never reuse the loop/broker/perception welded inside dnd5e.

Honest verdict: the parts of the encounter that are "genuinely the rules"
**already live in `rulebooks/dnd5e`** (economy, resolution, conditions). The
encounter is the *orchestration around them*. Folding orchestration + transport
back into the rulebook **inverts the layering** — the rulebook becomes both
top-of-stack and owner of delivery. The code moved away from this model
deliberately; A moves back.

### Option B — Status quo, named honestly: `encounter` IS the dnd5e game-loop SDK

Keep the arrow (`encounter → rulebooks/dnd5e`). Fix the **story**, not the code:

- Re-document `encounter/doc.go` + a component doc to state plainly: *this is the
  dnd5e encounter SDK — the orchestration layer that runs a dnd5e fight; it
  depends on `rulebooks/dnd5e` by design.*
- Correct `overview.md` (line 57 + module map) and point to this ADR + ADR-0030.
- **Delete the dead `TurnManager`** (the fossil that seeds the wrong mental
  model), sequenced behind the `TurnStart` handoff (*Housekeeping*).
- Ratify the **rules-vs-room** split as the load-bearing boundary: rule logic
  drifting into the encounter's agnostic half, or transport/positions drifting
  into the rulebook, become nameable defects.

Cost is almost entirely conceptual, not code: you are ratifying that the toolkit
has **two top-of-stack modules** — the rulebook (rules) and the encounter (the
dnd5e loop) — and that the encounter is **dnd5e-specific**, not a generic
engine. A second rulebook would then need its own encounter-equivalent, or a
later Option C extraction.

### Option C — Invert to a generic engine: pluggable rulebook behind interfaces

The deferred `EntityHydrator` direction (Journey 050), generalized. The encounter
depends on an abstract rules seam; `rulebooks/dnd5e` is injected. The arrow
reverses: `encounter → interfaces ← dnd5e`.

- **What it takes:** define every seam the encounter currently reaches across —
  hydration (load a combatant from opaque data), turn semantics (economy shape +
  turn-boundary signals), resolution (attack/move outcome), and the action menu.
  Journey 050 sketched the first of these (`EntityHydrator`); C needs the full
  set. Then dnd5e implements them and the encounter stops importing it.
- **The cost is real and it is why C was already rejected once:** it
  re-introduces exactly the abstraction the hydration cascade *deleted*.
  ADR-0030's win was "hydration is not a new subsystem — it is the existing
  `ToData`/`LoadFromData` round-trip, composed." An interface seam re-abstracts
  that, paying indirection at every call site the encounter currently makes
  directly into `character` / `monster`.
- **What pain justifies it:** a **second rulebook**. Until a concrete second
  ruleset must run through the same loop, the interfaces would be designed
  against a sample size of one and would almost certainly abstract *dnd5e's*
  shape rather than a *general* shape. The trigger is external, not internal.

## Recommendation: Option B

The code has already voted. The rules genuinely live in `rulebooks/dnd5e`; the
encounter is the orchestration around them, and its agnostic half
(broker/transport/perception/events/prompts) is host-infrastructure that belongs
neither inside a rulebook (A) nor behind speculative interfaces (C) until a
second rulebook forces the question. The divergence flagged is a
**documentation defect, not an architecture defect**: `overview.md` claims
"nothing depends on `rulebooks/dnd5e`" while the encounter has depended on it —
coherently and single-sourced — since ADR-0030. Option B closes the gap between
the map and the territory at near-zero risk, deletes the fossil that seeds the
wrong mental model, and names the rules-vs-room boundary that Beat 2 already
enforces. Option A inverts the layering (the rulebook would own the server);
Option C buys a generality no current requirement needs. Neither delivers
anything today that B does not, and both cost more.

The recommendation is **not** "never do C." It is "do B now, and let a second
rulebook be the event that promotes B → C." B is the honest name for where the
code is; C is where it goes *if* the product needs two rulebooks.

## What tips the decision later

- **B → C:** a concrete second rulebook (or a genuinely rules-agnostic host)
  that must run through the same loop. That is the only signal that makes the
  interface seam worth its indirection — and it supplies the second data point
  that lets the seam be designed right rather than as a dnd5e mould. Cheap
  down-payment that serves B too: if the agnostic half keeps growing and a
  **non-encounter** consumer wants it, extract broker/transport/perception/core
  into their own module first.
- **B → A:** if the "room" concerns turn out to be things a rulebook genuinely
  wants to own **and** there is never a second rulebook — i.e. the product
  collapses to "one rulebook forever" and the separate module is pure overhead.
  Unlikely under the toolkit-as-product framing, but that is the world where
  folding the loop into dnd5e stops being a layering inversion and becomes
  simplification.

## Housekeeping (lands regardless of the option chosen)

1. **`overview.md` is corrected in this PR** — line 57 no longer claims a false
   invariant, and the encounter exception is documented with a pointer to this
   ADR and ADR-0030. This is true under every option, so it ships now.
2. **The dead `TurnManager` is deleted as a follow-up after ratification** (this
   PR is docs-only). Sequencing is mandatory: deleting it removes the last live
   publisher of `TurnStartTopic`, so the deletion **must land together with, or
   after, the encounter taking over `TurnStart` publishing** (in-flight
   `feat/699-dodge-turnstart`). Delete it first and the four subscriber
   conditions lose their turn-start signal entirely.

## Consequences (of the recommended Option B)

### Positive
- The map matches the territory: `overview.md` stops asserting a false
  invariant, and the encounter's dnd5e dependency is documented as intentional.
- The rules-vs-room boundary becomes a named, enforceable rule — drift in either
  direction is now a nameable defect (Beat 2's Hide observer-set split is the
  worked example of applying it).
- Deleting the fossil `TurnManager` removes the artifact that seeds the "turn
  logic lives in the rulebook" mental model.
- Near-zero code risk: the shipped change is documentation.

### Negative
- Ratifies two top-of-stack modules and that the encounter is dnd5e-specific; a
  second rulebook pays the Option C cost later rather than now.
- The rules-vs-room line is a judgment boundary, not a compiler-enforced one; it
  depends on this ADR + the component doc to stay legible.

### Neutral
- `TurnManager` deletion and the `TurnStart` publisher handoff are sequenced as a
  follow-up, not in this PR.
- Option C stays the named evolution; this ADR records its trigger (a second
  rulebook) so a future reader knows exactly when to reopen it.

## Related

- ADR-0030 — encounter owns combatant hydration (ratified the import direction
  in passing; this ADR makes it a first-class decision).
- ADR-0031 / ADR-0032 — the event spine and the TakeAction unification, both of
  which deepened the encounter → rulebook coupling coherently.
- Journey 050 — the `EntityHydrator` that was drafted and rejected (the Option C
  seed).
- rpg-project#75 — the chapter ledger tracking this decision.
