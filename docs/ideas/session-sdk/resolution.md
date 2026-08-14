# Resolution: introducing the bus, one slice at a time

**Date:** 2026-08-14
**Status:** planned — approved by Kirk 2026-08-14 (own module: yes; first
vocabulary `Gather | Done`: yes)
**Decides:** nothing. [ADR-0038](../../adr/0038-resolution-owns-the-bus.md)
already decided the shape; journeys
[053](../../journey/053-resolution-seam-attach-responsibility.md) and
[054](../../journey/054-the-subscribe-interface-and-the-resolution-seam.md)
are how it was reached. This doc answers only the question left over:
**how do we land it?**

## The question

Wave 4's next step was "bring in the bus." Under ADR-0038 the bus does not
come *in* anywhere — it is created inside a resolution package, attaches
everything passed to it, and dies with the call. So "introduce the bus"
became "build resolution," and the open question is what the first PR is.

## What ADR-0038 bundles, and why it matters here

The ADR contains two separable inventions:

1. **Bus custody** — the single attach site, the instrumented surface,
   pass-everyone-in, dirty-participant returns.
2. **The driver and its sealed vocabulary** — `Gather | Pose | Request |
   Done`, which is the re-enterability story.

They can be proved independently, and they are not equally mature. Custody is
derived from code that exists and can be pointed at. The four-step vocabulary
is derived from journey 054's table of six machines, of which **exactly one is
implemented** — the walk, hand-rolled as `frozenResolution{Kind: kindWalk,
Path, Index}`. Sealing a case enumeration against five hypotheticals is the
shape of the mistake ADR-0007 exists to remember, and ADR-0038 makes extending
the vocabulary cost another ADR.

**So the vocabulary ships one case at a time, as callers arrive.** `Gather`
and `Done` now; `Pose` with the walk machine; `Request` with concentration.
Each case lands with the implementation that forced it.

## Where resolution lives — decided by the module graph, not by taste

`rulebooks/dnd5e/encounter` and `rulebooks/dnd5e/session` are separate Go
modules. `encounter` does **not** import `rulebooks/dnd5e` — its requires are
`core`, `play/clock`, `play/intel`, `play/record`, `tools/spatial`, `events`,
`game`. Today `session` is the only module depending on both:

```
dnd5e v0.78.0 ────┐
                  ├──→ session
encounter v0.4.0 ─┘
```

Resolution needs both — the world from `encounter`, the loaders, effects and
saves from `dnd5e` — so it cannot live inside either. `session`'s charter
forbids it holding rules or a bus, so it cannot live there either.
Resolution is a **new sibling module** slotted between them:

```
dnd5e + encounter ──→ resolution ──→ session
```

**Rejected:** keeping resolution inside the `dnd5e` module and taking a narrow
`World` interface instead of importing `encounter`. The reason is not
speculative — `gamectx` already demonstrates it. Effect predicates read world
state through **five separate context installers** (`WithGameContext`,
`WithRoom`, `WithCombatants`, `WithCombatState`, `WithReactionReadiness`), and
`GameContext` itself carries a `CharacterRegistry` whose three methods exist
because Dueling, Protection and friends each needed one more thing. That is
what "a narrow World interface" becomes after a year: it has no stable surface
because the aura ruling lets any predicate read anything. The interface option
also pushes world load and `ToData` custody back onto `session`, contradicting
data-in/data-out.

**The payoff for sequencing:** a new module consuming already-published
`dnd5e v0.78.0` and `encounter v0.4.0` means **PR 1 changes nothing else in
the repo** — no cross-module publish dance, no `replace` juggling, no
`combat`. Session wiring (#966, step 4.5) is a later PR taking a
`resolution v0.1.0` dependency, which is the repo's normal inside-out merge.

## The first slice: a saving throw

**Why a save and not a strike.** A save is the smallest interaction that
genuinely folds a chain — one `Gather`, one `Done` — and `SavingThrowChain`
already has two real subscribers that nobody has to write:

- `conditions/raging.go:113-114` — advantage on STR saves while raging
- `conditions/dodging.go:76-77` — advantage on DEX saves

`saves.SavingThrowInput` already carries an **optional** `EventBus` with a
documented nil fallback and already fires the chain
(`saves/saves.go:131-145`: `PublishWithChain` then `Execute`). So the save is
not a mechanic we convert to the new model — it already *is* the model, with
custody in the wrong place. The delta PR 1 makes is custody, and only custody.

Strike, by contrast, would land the seam debate and combat's bus divestment in
a single PR — 3,362 non-test lines (measured, matching #965's figure), with
`NewTurnManager` erroring without a bus and bus dependencies across `ac.go`,
`attack.go`, `attack_phases.go`, `damage.go`, `healing.go`, `movement.go`,
`recoverable_resource.go` and `turn_manager.go`.

**Why not an ability check**, the other one-`Gather` candidate: it is
player-initiated and therefore reachable from a session verb, which is
attractive — but `AbilityCheckChain` has only `raging` subscribing, and
`conditions/helped.go:68` explicitly notes it does not subscribe yet. Thinner
proof, and no bug lane behind it.

## PR 1 — `rulebooks/dnd5e/resolution` v0.1.0

### 1. The instrumented surface

`events.EventBus` is a three-method interface (`events/bus.go:29`:
`Subscribe`, `Unsubscribe`, `Publish`), so the recording implementation is a
delegating wrapper of roughly thirty lines.

**Attribution is by construction, not interrogation.** `ConditionBehavior` is
`IsApplied/Apply/Remove/ToJSON` and cannot name itself (#971). It does not
need to: resolution routed on the effect record's ref to *build* the behavior,
so it stamps a per-effect surface with that ref before calling `Apply`. Every
subscription made during that call is attributed to that ref by the surface
that granted it. **PR 1 does not block on #971**, and #971 keeps its own
motivation (host-seam reporting, #653's registry).

This buys ADR-0038's three claims directly:

- pre-execution inspection — the registration list exists before anything runs
- silent absence is observable — "this ref attached zero hooks" is a fact
- deterministic teardown — resolution drops every subscription it granted,
  trusting no effect to clean up after itself

### 2. `Resolve`

```go
func Resolve(ctx context.Context, in *Input) (*Output, error)
```

`Input` carries the world data, the participants (characters and NPCs), the
deciders, and the action. **Note the real signature:** journey 054's
pseudocode has `encounter.LoadEncounter(in.World)`, but it is
`LoadEncounter(data EncounterData, deciders map[MemberID]Decider)`
(`encounter/data.go:393`) — the deciders pass through `Input`.

The body is ADR-0038's loop, in its order: create the bus, load the world,
install game context, sort participants by ID (C8 determinism), route each
effect record's ref to its loader (`conditions.CreateFromRef:39`,
`features.CreateFromRef:34` — ADR-0037 routing already exists), call
`Apply(ctx, stampedSurface)`, run the machine, tear down, return.

**"Install game context" is a real task, not a line of pseudocode.** Journey
054 writes `gamectx.With(ctx, world)`; no such function exists. Predicates read
world state through five installers (`WithGameContext`, `WithRoom`,
`WithCombatants`, `WithCombatState`, `WithReactionReadiness`), and resolution
becomes the one place that populates them — which is the same consolidation the
attach loop performs for effects. PR 1 populates only what a save needs and
leaves the rest; **collapsing the five into one installer is explicitly not in
scope**, but resolution being their single caller is what would later make that
collapse a local change instead of a repo-wide one.

`Output` carries `World` (via `Encounter.ToData()`, `encounter/data.go:147`),
`DirtyCharacters` (`character.IsDirty()`, `character/character.go:775`, and
`monster.IsDirty()`, `monster/monster.go:183`, both exist),
`Outcome`, and `Hooks`.

**The world round-trips even though the save machine reads nothing from it.**
This is deliberate and is the one place PR 1 builds ahead of its own need.
Without it, `Resolve` is `saves.MakeSavingThrow` with extra steps and a
reviewer would rightly ask what the module bought; with it, charter clause 1
(load-act-save, no state survives a call) is demonstrated rather than
asserted, and PR 2 does not redesign the seam.

### 3. The machine — two step kinds

```go
type Step interface{ isStep() }

type Gather struct {          // "fold this chain and hand me the result"
    Event  any
    Resume func(State, any) State
}
type Done struct{ Outcome Outcome }
```

`resolution.NewSave(...)` yields `Gather`(SavingThrowChain) → `Done`.
Resolution folds; the machine never sees the bus. `Pose` and `Request` are
**not declared** until the walk machine and concentration respectively need
them.

### 4. What proves it

The headline test: **a Raging barbarian gets advantage on a STR save with
nobody attaching anything by hand.** Resolution attached everyone; Raging's
own predicate decided. That single assertion is ADR-0038 end to end against
code that already exists. Dodging on a DEX save is the second.

Supporting pins, each mutation-checkable:

- an effect that attaches zero hooks is an assertable fact, not silence
- teardown drops every granted subscription — nothing survives `Resolve`
- two `Resolve` calls over identical data produce an **identical registration
  list** (C8; the Shield/resume edge in miniature)
- a participant who should not be affected is passed in and folds nothing —
  the pass-everyone-in ruling costs correctness nothing
- the world round-trips unchanged through a save that does not touch it

### 5. Explicitly out of PR 1

- **Any session verb.** A save is a consequence, not a player-facing action;
  forcing a verb means inventing a fake surface or dragging trap/knockdown
  content in. The session seam is #966 / step 4.5.
- **The knockdown lane** (#970 → #962 → #961). It becomes PR 2 and gets to be
  resolution's *second* consumer, which is a better test of the seam than any
  amount of PR 1 test-writing.
- **All of `combat`** — `TurnManager`, `attack_phases.go`, `damage.go`, and
  their bus dependencies. Untouched.
- **`Pose`, `Request`**, and anything that suspends.

## Sequence after PR 1

| PR | Lands | Proves |
|---|---|---|
| 1 | `resolution` v0.1.0 — surface, `Resolve`, save machine | bus custody |
| 2 | the knockdown lane (#970 → #962/#961) on top of it | a second consumer |
| 3 | the strike machine; combat's bus divestment (#965, retargeted) | the headline interaction |
| 4 | the walk machine + `Pose`; trigger detection (#964) | suspension, and the vocabulary's third case |
| 5 | session verbs (#966, step 4.5) | the host seam |

## Bookkeeping this implies

- **`plan.md`'s charter clause 2 is now wrong.** It reads "*The encounter owns
  the wiring; combat owns the rules*", and PRs are told to read the charter
  before opening. ADR-0038 reversed it: the composition stays bus-free and
  resolution owns the wiring. Needs a superseded note pointing at the ADR.
- **#965 needs retargeting, not just implementing.** Its title still says "the
  encounter owns the wiring" and its scope says "driven through the
  composition." It becomes the strike machine and combat's divestment; PR 1
  gets its own issue.
- **#917** (module auto-tagging not compatibility- or CI-aware) is open, and a
  new module means a new version line walking straight into it.
- **CLAUDE.md's three-kinds taxonomy** (leaf / composition / host seam) has no
  slot for resolution, which sits *above* the `encounter` composition and
  calls into it. Worth a paragraph once the module exists and its shape is
  observed rather than predicted.

## What would make this plan wrong

- If folding a chain turns out to need something from the machine that
  `Gather` cannot express as data, the vocabulary is wrong at case one and
  the driver should not be generalized at all until strike exists.
- If attaching every participant for a single save is measurably expensive
  against the ~187µs click cycle, the aura ruling's "room-scoping is an
  optimization" needs to become a correctness rule after all.
- If the per-effect stamped surface cannot attribute a subscription made
  through a helper that captures the bus, attribution-by-construction fails
  and #971 becomes a real dependency.

— asset-pipeline agent, on behalf of KirkDiggler
