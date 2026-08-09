# play/clock — Brainstorm (the WHY)

*2026-08-08. First axis of the encounter reset (journey 051). Design dialogue
between Kirk and the director session; decisions recorded here with their
reasoning and rejected alternatives. The normative WHAT is `design.md`; the
HOW will be `plan.md`.*

## Why clock first

The clock is the spine of the seven axes — every other axis is invoked *at a
time* the clock defines — and it was the axis Kirk most wanted to design.
Nothing downstream forced the order; an architect gets to pick joy.

## The target: Divinity Original Sin 2's mixed time

Kirk named the model precisely: the world runs free-roam; a triggered combat
is a **localized turn-based bubble**; a party member across the island stays
in free-roam, and *falls into* the bubble when they wander too close. That one
sentence settled the biggest fork:

**Clocks are values, several at once — not a singleton coordinator.** One
world clock plus N bubbles, membership owned above the clocks, an entity on
exactly one clock at any moment. The old module's alternative (one global
mode enum with "combat pockets" bolted on) produced the #808 stale-economy
teardown bug; in this model, leaving a bubble *is* the teardown.

The free-roam clock comes from the monster-AI brainstorm
(rpg-project#201/PR#202, "same brain, two clocks"): a **player-driven world
tick** — the world advances *because players act*, never on a timer — with
max-player-displacement (not sum) as the multiplayer fairness rule. No
real-time backend simulation.

## Decisions and their reasoning

### 1. Concrete types, no shared interface

Rejected: a unifying `Clock` interface (`Join/Leave/CanAct/Advance`). With
exactly two real policies, the interface becomes the union of both their
needs — fat or type-asserted around — which is the god-aggregate lesson in
miniature. Extracting an interface later from two concrete types is cheap;
un-fattening a published one is not. The only interfaces in the module are
the two one-method seams `Transfer` provably needs.

Rejected: event-sourced scheduler. Kirk: too early for that complexity, and
if we ever need it, a live streamable server process makes it a *record*-axis
conversation, not a clock one.

### 2. Milestones only — the clock owns no durations

Chosen over shared world-time (every policy defines an exchange rate on day
one, before any consumer exists) and clock-owned timers (pulls
condition-shaped knowledge into a leaf). Durations belong to their consumers,
keyed to milestone kinds; cross-clock conversion (the pre-fight buff falling
into a bubble) gets designed at the join/leave boundary when a real consumer
forces the contract — the outside-in rule applied to our own module.

The trade-off, stated honestly and accepted: **Milestone trades ambient
delivery for explicit causality.** Return values mean the caller learns
everything from the call that caused it, in causal order, deterministically —
the anti-pattern this kills is the old module's install-subscriber /
call / drain-buffer bus capture. The price: if the composition drops the
slice, the fact never happened. The failure mode changes from "delivered
twice, in mystery order" (runtime archaeology) to "forgot to forward"
(visible in review and tests). We prefer the second class.

### 3. Signatures: the three-clause law

Started as "Input/Output for verbs, plain methods for queries," and Kirk's
probing collapsed the exception in two steps. First, the evolvability
argument applies to queries too (a plain `Budget(id) int` that needs a
second answer breaks; an Output struct absorbs it) — and plain parameterized
queries force zero-value ambiguity, a bug class the old module actually
shipped (`IsReactionReady` false-for-unknown). Second, Kirk's reframe:
**the error channel is communication, not just failure** — `Active()` on an
idle clock shouldn't return a guessable zero `EntityID`, it should say
`ErrIdle`. His readability test settled the input side:
`LeaveMember(&LeaveMemberInput{ID: …})` reads; `LeaveMember(42)` means
nothing.

Final law (design R3), all clauses mechanical: (a) every exported function
returns `error` last — sentinels are API vocabulary, dispatched with
`errors.Is`; (b) parameters always travel in a single `*XxxInput` struct;
(c) mutations return `*XxxOutput` (carrying Milestones), zero-arg reads
return bare value + error. Why it's load-bearing and not style: the version
story leans on `gorelease`, and struct fields are additive while positional
parameters are breaking. The rulebook side (`ExecuteActionInput`) evolved
cleanly under this pattern; the old encounter's positional verbs accreted
parameters and inline guards.

### 4. No context.Context in the leaf — the law decides

A `play/*` leaf depends on `core` only, therefore it cannot block, therefore
ctx would be decoration (a `select` on Done in a microsecond map operation
cancels nothing; it adds a failure mode). The toolkit's own rule already
says: Go ctx only where cancellation is genuinely needed. Cancellation is
real one layer up: the composition takes ctx, checks it **between** clock
calls, and on cancel waits out the in-flight microseconds — state coherent,
nothing half-advanced. A leaf that needs ctx is a leaf that has grown an
illegal dependency; its absence is the tell.

### 5. Tick ships to the current game first

The turn half is NOT retrofitted into the old encounter (surgery on frozen
code; reintroduces dual state about time while both representations live).
But the tick half has no competitor in production — out-of-combat monsters
simply don't act today — so rpg-api can compose a `clock.Tick` alongside the
old module: after a player's exploration move, feed displacement, ask who has
budget, invoke behavior. This gives the monster-behavior work its free-roam
clock without waiting for the rebuild, and gives the first `play/` module a
real consumer proving the contract before the new composition exists.

### 6. Transfer: join-first with compensating leave (2026-08-09)

Planning the R6 atomicity forced the one post-approval design amendment:
both `TransferInput` sides are `Membership` (`Leaver` + `Joiner`). The
derivation, walked with Kirk:

**Leave-first can't roll back.** Restoring a departed member needs more
than their position: removal shifts every index behind them; if they were
active, the turn advanced as a side effect (re-inserting at the old `Pos`
puts them in the order but leaves the *next* entity active — position
restored ≠ state restored); if they were the last member, the clock went
idle and restoration collides with the `ErrIdle`-on-insert rule. A
truthful restore token is `{Pos, WasActive, Round, …}` — a snapshot
creeping into the seam. Kirk spotted the other exit — a two-phase
(prepare/commit) leave — which works but doubles the seam surface for a
problem the ordering dissolves.

**Join-first compensates with zero remembered data.** Every common
failure (idle bubble, duplicate, bad position) happens before anything
changed. After a successful join, the only possible leave failure is
`ErrNotMember` (caller bug), and the undo — remove what was just added —
is self-describing: `Insert` never makes the newcomer active, so the undo
never touches the active-advancement branch and restores byte-identically
(reviewer-verified).

Consequences that fall out: the compensation path calls *leave on the
destination*, so `To` must be a `Leaver` — and both sides are typed
`Membership` so the execution strategy never leaks into the signature.
The transient dual membership mid-call is unobservable because milestones
are **return values** — nothing escapes until the function returns, and a
compensated join's milestones are discarded (the return-values law
closing a race a bus would have had). Reported order stays semantic
(leave-then-join, the domain story) regardless of execution order.
`LeaveMemberOutput` deliberately does NOT carry the vacated position —
the one consumer that seemed to want it (rollback) needed more than it,
and the field stays additive for a future real consumer (e.g. revival at
the old initiative slot).

## Elaborations made while writing the design

These follow from the approved sections but weren't individually discussed;
flagged for review: remove-the-active-actor semantics, merge preserving the
receiving clock's active actor, the high-water mechanics implementing
max-displacement accrual, and the `Leaver`/`Joiner` one-method seams under
`Transfer`.

## Links

- `docs/journey/051-encounter-reset-application-to-toolkit.md` — the reset
  decision this axis is the first step of
- rpg-project `ideas/monster-ai/brainstorm.md` (PR #202) — "same brain, two
  clocks", the world-tick unlock, max-displacement fairness
- Old module evidence: `encounter/combat.go` (mode/pockets), #808 (teardown),
  `spliceFromInitiative` (remove-active correctness)
