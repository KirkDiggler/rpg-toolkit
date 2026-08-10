# Journey 052: The Event Bus Grows Up — Broadcast, Chains, and the Composition Reckoning

*2026-08-10. With two play/ axes shipped and interrupt next on the bench, Kirk
asked for an honest evaluation of the event bus: "decoupling effects from
actions and allowing Rage to be a self-contained function is very attractive
for a system that can be modified in all kinds of ways. Composition was not
thought of, and I wonder if it could use adjustments or replacement." This doc
is that evaluation, and the arc that led to it.*

## The occasion

The encounter reset (journey 051) declared four runtime axes and a family law
for the new `play/` modules: leaves depend on core and stdlib only, deltas are
returned and never published, nothing suspends on a call stack. Three modules
shipped under that law — clock, intel, record — without ever touching the bus.
The fourth axis, **interrupt**, is different: reactions, readied actions, and
suspended resolutions are exactly the territory the bus has historically
claimed. Before designing interrupt, we had to know what the bus actually is
in 2026 — not what its README says, and not what we remember it being.

So we ran a census: every substantive bus usage site in the repo, classified
by the job it really does — OBSERVE (a fact, nothing flows back), MODIFY (a
contribution reaches the publisher), RETURN-CHANNEL (the publisher captures
its own events to learn what happened), WIRING (plumbing).

## Act I: the string era

The first bus was stringly typed all the way down: `bus.Subscribe("attack.roll",
priority=10, handler)`, payloads in `map[string]interface{}`, handlers doing
`ctx.Get("damage").(int)` and hoping (journeys 010, 022). Rage's first
implementation attempt (PR #221) is what broke the illusion. Two effects at the
same priority — who wins? A flat key-value bag — what's base damage and what's
bonus? Journey 014 contains the confession that named the era: **"we built a
beautiful type-safe bus but it was read-only."** The proposed fix at the time
was mutable event payloads plus priority integers. That fix was never built,
and we should be grateful.

## Act II: the pivot — chains, not priorities

Journey 024 is where the system grew up the first time. The insight was that
"who modifies this roll, in what order" is not a pub/sub question at all — it
is a *pipeline* question, and the rulebook owns the pipeline. The design that
shipped (ADR-0024, `events/v0.6.x`):

- **Events are immutable values.** Typed topics pass copies; mutating an event
  in a handler is a documented anti-pattern (`events/CHAIN_PATTERN.md`,
  "Mistake 2").
- **Modification lives in `ChainedTopic` + `StagedChain`.** Subscribers receive
  `(event, chain)` and may add a named closure to a **stage**; they return the
  chain, never a mutated event.
- **The rulebook declares stage order as data** — `base → features →
  conditions → equipment → final` (`rulebooks/dnd5e/combat/stages.go`).
  Priority numbers were explicitly rejected: effects declare *where they fit*
  ("I'm a feature"), not a magic integer.
- **The publisher executes.** `PublishWithChain` returns the assembled chain;
  the caller runs `Execute` itself and reads the result. Collection is
  separated from application.
- The bus itself stayed deliberately dumb: synchronous, same-goroutine,
  subscription-order fan-out, ~600 lines, one implementation.

Worth recording the road not taken: in the 024 conversation, Claude's first
suggestion was to abandon events for a pull-based
`effectManager.GetModifiersFor(attacker, ModifierTypeAttack)`. Kirk pushed
back hard — the decoupling is the point; traps listen to movement, curses
react to priest interactions. He was right. That proposal threw away the
ordering discipline and the self-containment along with the bus. Hold that
thought; it comes back in Act IV wearing different clothes.

## Act III: the census — what the bus actually does today

**Rage, traced end to end.** `Rage.Activate` publishes a
`ConditionAppliedEvent`; the character installs the `RagingCondition`; the
condition subscribes to the damage chain topic; when an attack resolves, the
*publisher* builds a `StagedChain`, publishes it, Rage adds
`Add(StageFeatures, "rage", +2 closure)`, and the publisher executes. The
event is never mutated by a subscriber. Rage is one object owning four chain
hooks, five lifecycle observers, and its own persistence — exactly the
self-contained effect the bus was supposed to enable.

The numbers, repo-wide:

- **MODIFY: ~26 sites, all disciplined.** Every one is a `c.Add(stage, name,
  closure)` chain registration — conditions, fighting styles, monster traits.
  There are **zero** places where attack or damage code queries a character's
  conditions directly. The chain *is* the modifier registry; the bus's only
  role in modification is **discovery** — "who wants to contribute to this
  chain?"
- **OBSERVE: ~45 sites.** Lifecycle bookkeeping, broker bridges, notify tails.
  Healthy where subscribed; in tools/*, roughly twenty topics are published
  that nobody has ever subscribed to — write-only telemetry, shelves stocked
  with props.
- **RETURN-CHANNEL: ~10 sites — zero in dnd5e, all in old encounter.** The SDK
  installs transient capture subscribers around its own resolver calls and
  drains what fired. The in-code comment is disarmingly honest about why:
  *"the resolver interface itself returns only Prevented/PreventReason — it
  does not surface attack outcomes directly."* The verbs return too little, so
  facts travel by bus and get trapped in flight. The disease isn't the bus
  misbehaving; it's what grows wherever a bus exists next to anemic return
  values.
- **Silent failure modes throughout the wiring.** A handler with an unexpected
  signature is skipped without error; a typed-topic mismatch is swallowed;
  publish errors are discarded at nearly every call site; one topic flavor
  hands handlers the subscribe-time context, the other the publish-time
  context. Mis-wiring doesn't crash — it quietly does nothing.
  `tools/spatial`'s orchestrator, whose entity→room index exists only if rooms
  and orchestrator share a bus connected in the right order, is the live
  demonstration (#909).
- **The rot ledger.** Four `mechanics/*` modules still target the *deleted*
  pre-v0.2 API and do not compile (#617). `game.Context` requires a non-nil
  bus even for consumers that never publish. Dead `EventQuery*` constants,
  bound-but-never-published topics (#910).

The reaction system deserves its own line, because it's the stress fracture
that predicts the future. Opportunity Attack and Shield *couldn't* express
"pause this resolution and ask someone" through the bus. So they observe the
chain, publish a `ReactionTriggerEvent`, and the actual modification arrives
later as an ordinary function argument in a second phase. The bus was asked a
suspension question and answered with a workaround — and the workaround is
what pushed the capture-window pattern up into the encounter SDK.

## Act IV: the composition collision

The play/ family's laws are values, determinism, and returns: deltas returned,
never published; state serializable by construction; nothing suspended on a
stack. Three collisions follow directly:

1. **Suspension.** Interrupt's premise is suspension-as-value: a resolution
   that stops mid-flight becomes data — question, audience, opaque resume
   token — persisted and resumed later, possibly in another process. But a
   chain is populated by whoever holds a live subscription: process-local Go
   closures. "Which modifiers were in flight" cannot be persisted or rebuilt.
   ADR-0024 knew: "pausable chains — save chain state to JSON, resume from
   saved state" is parked there as future research. Interrupt is that future
   arriving.
2. **Attribution.** The record axis wants the story: *17 = d20(14) + prof(2) +
   rage(+2)*. Chain contributions are named (`"rage"`), which is halfway
   there — but only the publisher's `Execute` sees them, and nothing durable
   knows who was subscribed. Attribution should fall out of the modifier set
   being enumerable, not be reconstructed from bus archaeology.
3. **The boundary.** Family law says a verb returns everything its caller
   needs. Old encounter's ten capture windows exist because that law didn't
   hold. Under composition the rule hardens: **nothing the caller needs may
   travel on the bus.** Observation is for optional listeners, never for the
   return path.

## The stance

The census points at a split, not a replacement. The bus does two jobs today;
it should keep one and formally hand off the other.

**Keep: observation.** Rulebook-internal, below the resolution seam, carrying
facts for optional listeners — the trap listening to movement, the broker
bridging to clients. This is the decoupling Kirk defended in 024, and it was
never the problem.

**Hand off: discovery.** Here is the observation that makes the fix small:
**the durable modifier registry already exists — it is the character's
condition list.** Conditions persist to JSON and rehydrate on load; the bus is
merely how that registry announces itself into chains at runtime, via
subscriptions installed in `Apply(ctx, bus)`. So: let the resolution pipeline
*enumerate* the entity's active effects directly — each condition exposes its
chain contributions through an interface — instead of broadcasting and hoping
the right closures are listening.

Everything that made 024 right survives intact: the staged chain, the
rulebook-declared order, the publisher executing, Rage as one self-contained
object. Only *how effects are found* changes — enumeration of persisted state
instead of eavesdropping on a live bus. And this is not 024's rejected
`GetModifiersFor` returning from exile: that proposal discarded the chain
discipline; this one is *built on it*. What it buys is exactly what
composition needs: a modifier set that is deterministic (no
registration-order dependence), inspectable (record's attribution falls out),
and **reconstructible** — suspension becomes "persist resolution state plus
condition set; rehydrate; re-enumerate; resume." No closure is ever
serialized, because no closure is ever load-bearing.

## What we are deliberately not doing

- **Not rewriting `events/`.** The module is ~600 healthy, well-tested lines,
  and the chain machinery carries over nearly whole. The change is a usage
  contract, not an API.
- **Not building an effect-enumeration mechanism speculatively.** Empty
  shelves. The stance gets its first consumer when the resolution seam is
  designed — which the interrupt axis forces anyway. That design conversation
  is where this stance gets ratified or amended.
- **Not migrating old encounter.** It is bugfix-only, headed for
  delete-and-tag-pin. Its capture windows are evidence, not a work queue.

## Open questions for the interrupt design

- Where exactly the resolution seam sits — which verbs return `Outcome |
  Suspended`, and what the suspension envelope owns vs. what stays
  rulebook-opaque.
- The shape of the contribute interface — per-chain-type methods, a single
  typed dispatch, or something stage-declared.
- Reaction windows: OA/Shield's two-phase workaround becomes a first-class
  suspension. What happens to `ReactionTriggerEvent` when the thing it was
  compensating for exists?
- Whether observation topics stay per-rulebook or grow a thin shared
  vocabulary — and whether `game.Context` should stop requiring a bus.

## Pointers

- Journeys 010, 014, 022 (the string era and its pain), 024 (the pivot), 026
  ("pipelines all the way down"), 044 (events carry refs, not objects), 051
  (the encounter reset and the play/ family laws).
- ADR-0024 (typed topics + chains — including the parked "pausable chains"
  note), ADR-0026/0027/0029 (chain conventions for damage, attacks, saves,
  movement).
- `docs/architecture/components/events.md` — the module reference.
- Issues: #617 (mechanics/* on the deleted API), #904 (lint version skew),
  #909/#910 (spatial bus-dependency and hygiene, filed from the same survey).
