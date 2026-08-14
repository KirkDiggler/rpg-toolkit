# ADR-0038: Resolution Owns the Bus — Rules Packages Trade in Values

**Date:** 2026-08-14
**Status:** Accepted (Kirk, 2026-08-14)

## Context

The event bus did two jobs (journey 052's census, ~26 modify sites, ~45
observe sites): **discovery** — "who wants to contribute to this chain?" —
and **observation** — facts for optional listeners. Discovery-by-broadcast
made attach responsibility distributed: every load path had to remember to
attach its entity's effects, and the inventory was already uneven —
characters attached completely in one required-bus call, monsters needed a
three-call assembly the README warned about, the session's `Spawn` attached
nothing, and traps had no owner at all. A forgotten attach does not error;
the monster just fights without Pack Tactics.

Meanwhile wave 4 (session SDK, #959/#965) had to place combat, and `combat`
required a bus to construct (`NewTurnManager` errors without one) and
published its own turn-lifecycle events — duplicating `play/clock`, which
already returns `TurnStarted`/`TurnEnded` as milestone values. The
composition (`rulebooks/dnd5e/encounter`) is entirely bus-free, and every
layer below the session communicates by returning values.

The full deliberation — four options, the worked edges (aura, concentration,
Shield, traps, the tick), Kirk's refinements, and the corrections along the
way — is journey [053](../journey/053-resolution-seam-attach-responsibility.md)
and [054](../journey/054-the-subscribe-interface-and-the-resolution-seam.md).

## Decision

**A resolution package is the one place a bus exists.** Kirk's summary: *"
moving the bus out of combat and character is huge. Resolution needs it. A
character by itself does not."*

1. **Everything at the seams is data.** The session fetches participant data
   from its repositories, passes it in, and saves what comes back dirty. The
   composition raises interactions and stays bus-free.
2. **Resolution creates the bus per interaction, attaches everything passed
   in, and the bus dies with the call.** The single attach site: for each
   participant's effect record, route ref → loader (ADR-0037), call
   `Apply(ctx, bus)` on each behavior. The surface passed is an
   *instrumented* `events.EventBus` implementation that records every
   subscription — pre-execution inspectability, observable silent absence,
   and deterministic teardown, at zero migration cost (the ~26 existing
   `Apply` sites keep their exact signature). *The bus never leaves
   resolution.*
3. **Rules packages are step machines over data — they never see the bus.**
   `combat.NewStrike` (and every verb: `features.NewActivate`,
   `encounter.NewWalk`) yields steps from a **sealed vocabulary — `Gather |
   Pose | Request | Done`** — and resolution drives: fold a chain on its
   bus, open a window and suspend, queue a follow-up machine, or finish.
   Forced by suspension, not taste: reactions resume in a fresh process, so
   no phase may live on a stack; every `Gather` return is a legal suspension
   point. Extending the vocabulary requires an ADR.
4. **Effects keep the authorship model, unchanged.** Write one
   self-contained object; it names its topics in `Apply`; nobody holds a
   central registry of what any effect does. Two kinds, decided by RAW's own
   grammar: **granted** ("whenever *a target* makes…" — lives on the
   beneficiary, works from their sheet, links back to what kills it) and
   **projected** ("*while*… *within 10 feet*" — never leaves the owner).
   *An effect lives where it works, and links to what kills it.*
5. **Inherited rulings** (journeys 053/054): pass every encounter member
   into an interaction (scope is the caller's; applicability is the
   effect's predicate); resolution returns every dirty participant's data;
   granted conditions are claims validated against their source at load
   (crash net for revocation ripples — no transactions); participants
   attach in sorted order (C8 determinism).

## Options considered and rejected

- **Per-chain-type methods on the attachable** — re-proposes the
  enumerated-method-per-case pattern ADR-0007 rejected; breaks every
  implementer per new chain kind; still needs a bus for lifecycle.
- **Stage-declared contributions (pure enumeration)** — best chain-discovery
  story, but effects *observe* as well as contribute, so it is the chosen
  design plus a parallel mechanism for half the job, at
  rewrite-every-effect cost. Its one real advantage (pre-execution
  inspection) is delivered by the instrumented surface instead. If a
  genuine pre-execution consumer arrives (planner UI, AI action scoring),
  declarations can be *added* to `Apply` objects without breaking anyone —
  the reverse migration is not additive.
- **Effects as pure data, no closures** — the homebrew endgame; deferred,
  not rejected. The predicate language is a DSL that grows into a worse
  programming language, and it ends the write-a-function authorship model.
  The door stays open: a `DataDrivenEffect` implementing `Apply` from
  declarative config can arrive later as one more attachable.
- **A combat wrapper / bus inside combat** — an earlier draft passed the bus
  into `combat.RunStrike`; rejected because suspension forces the step
  machine anyway, and because turn-lifecycle publishing duplicated
  `play/clock`'s milestones. Those publishes do not move — they disappear.

## Consequences

**Positive.** One attach loop replaces N load paths that must each remember
(the monster three-call assembly collapses behind its loader); every
registration is recorded at attach time; suspension is the shape of the code
rather than a discipline; migration for existing effects is near zero;
`combat` and `character` shed infrastructure and become what they are —
rules vocabulary and data.

**Negative.** A new package (resolution) must be built and owned; an
effect's contribution is invisible until a chain executes (mitigated:
chain outputs carry source records, and the instrumented surface lists
hooks); an effect whose `Apply` forgets a topic still fails silently — but
the blast radius shrinks to one effect's own unit test.

**Neutral.** Pass-everyone-in costs ~5µs per character against a measured
~187µs click cycle; the aura ruling makes room-scoping an optimization,
never a correctness rule.

## References

- Journeys [052](../journey/052-event-bus-evolution-broadcast-chains-enumeration.md),
  [053](../journey/053-resolution-seam-attach-responsibility.md),
  [054](../journey/054-the-subscribe-interface-and-the-resolution-seam.md)
- Prior art: `events/example_journey_test.go` (`BlessSpell` — the
  subscription mechanics, not a storage model)
- ADR-0007 (generic trigger over enumerated methods), ADR-0024 (typed
  topics), ADR-0026/0027 (damage/attack phases), ADR-0037 (refs as loader
  routing; the options-before-choosing process note)
