# The subscribe interface — options for ADR-0038

**Status:** **B picked (Kirk, 2026-08-14), refined below** — see "The
refinement: does Apply need a bus?". ADR-0038 records it once the refinement
is confirmed.
**Context:** [journey 053](../../journey/053-resolution-seam-attach-responsibility.md)
— read it first; this doc deliberately does not restate it.
**Process:** ADR-0037's note — genuine options with trade-offs in the open,
including one from a different direction, *before* choosing. An option that
fits is not thereby true.

## The question

The resolution package receives participant data objects and is solely
responsible for attaching everything to its one-call bus. **What does an
attachable thing expose so resolution can attach it?**

## Fixed points (not up for grabs here — decided in 053)

- **Authorship:** write one effect, attach it to anything, it manages itself.
  No central registry of what any effect does.
- **Single attach site:** resolution attaches; nobody else. The session passes
  data in and saves data out; the composition raises interactions and stays
  bus-free.
- **Everything at the seams is data.** Phase boundaries persist as data;
  resume = pass the data in again, re-attach, continue.
- **Scope:** pass every member of the encounter in; the effect's own predicate
  decides applicability (the aura ruling).
- **Loader routing is ADR-0037's refs:** heterogeneous data (character,
  monster, trap) routes to the package that can load it. That decision
  reuses itself here and is not re-decided.

## The edge test suite

From 053, abbreviated. Every option is scored against these:

| Edge | What it demands |
|---|---|
| Aura | an effect's predicate reads *another* entity's position (gamectx) |
| Concentration | effects observe lifecycle (damage taken), not just contribute to chains; writes ripple to other participants |
| Shield / resume | re-attach from data must reconstruct the same contributor set (deterministic, C8) |
| Trap | non-character attachables enter through the same seam |
| Silent absence | forgetting to attach must be structurally hard, not disciplined away |
| Migration | ~26 existing modify sites, all `c.Add(stage, id, closure)` via `Apply(ctx, bus)` |

## Option A — per-chain-type methods

```go
type Attachable interface {
    AttackContributions() []Contribution[AttackChainEvent]
    SaveContributions()   []Contribution[SavingThrowChainEvent]
    DamageContributions() []Contribution[DamageChainEvent]
    // ... one method per chain kind
}
```

Statically typed end to end; resolution knows exactly what it holds.

**Against, decisively:** the interface grows a method per chain kind, every
attachable implements all of them (mostly returning nil), and adding a chain
kind breaks every implementer. This is the enumerated-method-per-case pattern
**ADR-0007 already rejected** ("one generic trigger beats an enumerated
method per case") — re-proposed at a worse scale. It also covers only chain
contribution: concentration's lifecycle observation needs a second mechanism
anyway.

**Edges:** aura fine; concentration needs a bus regardless; resume fine;
trap fine; silent absence good; migration = rewrite everything, twice the
surface.

## Option B — the subscribe interface (self-attachment, resolution-called)

```go
// Attachable is anything that wires itself to a resolution's bus.
type Attachable interface {
    Apply(ctx context.Context, bus events.EventBus) error
}
```

This is Kirk's sentence made literal — *"an interface that says they should
subscribe"* — and it is **today's `Apply(ctx, bus)`, reused**. The flow:
resolution takes each participant's data, routes it to its loader by ref
(ADR-0037), the loader produces runtime effect objects, and resolution calls
`Apply` on each. The effect subscribes to whatever topics it cares about —
chain events for contribution, observation events for lifecycle — exactly as
`DodgingCondition` and `RagingCondition` do now.

**For:**
- **Migration is near zero.** All ~26 modify sites already implement this
  shape; what changes is *who calls Apply and where the bus lives*, not any
  effect's code.
- **One mechanism covers both jobs.** Chain contribution and lifecycle
  observation (concentration hearing damage, Rage expiring on turn start) are
  the same subscribe, which A and C both split in two.
- **Authorship model preserved exactly** — the effect owns its predicate,
  its topics, its stages.
- **Determinism is one rule:** resolution attaches participants in sorted
  order; subscription order follows attach order (C8 holds).

**Against, honestly:**
- **Contribution is invisible until events fire.** You cannot ask an effect
  "what would you add?" without running a chain. Mitigated in practice: chain
  *outputs* carry source records (`AttackModifierSource{SourceRef, SourceID,
  Reason}`), so attribution exists after the fold — where record and the
  client actually need it.
- **A residual silent-absence risk:** an effect whose `Apply` forgets a topic
  contributes nothing, silently. But the model shrinks this from "a load path
  forgot an entity" (killed structurally — resolution attaches everyone) to
  "one effect's own Apply is wrong" — which is exactly what that effect's
  unit test pins. Bounded, testable, and the same risk every option carries
  inside its modifier bodies.

**Edges:** aura — predicate reads gamectx in its closure, fine. Concentration
— natural (subscribe to the damage observation). Resume — re-attach from
data, re-publish the phase's chain event, same contributors. Trap — its
loader returns an Attachable, same seam. Migration — near zero.

## Option C — stage-declared contributions (enumeration all the way down)

```go
type Attachable interface {
    Contributions() []Contribution
    // Contribution{ChainKind, Stage, ID, Modifier func(ctx, T) (T, error)}
}
```

Resolution reads declarations and installs them into chains directly — no bus
for discovery at all. 052's original stance in its purest form.

**For:** the modifier *set* is inspectable data before execution; ordering is
trivially deterministic; discovery cannot be missed because it is a read, not
a broadcast.

**Against:**
- **It still needs a bus.** Effects do more than contribute to chains:
  concentration listens for damage, Rage watches turn starts to expire,
  conditions hear their own removal. Either lifecycle becomes a second
  declared vocabulary (a growing enum of "things an effect can react to" —
  ADR-0007's pattern again) or observation keeps the bus — at which point C
  is B plus a parallel mechanism for half the job.
- **Migration is a rewrite of every effect's hooks** into declarations, for
  a benefit (pre-execution inspection) nothing currently needs — attribution
  already lands in chain outputs.
- The `Modifier` field is still a closure; the "it's all data" appeal is
  thinner than it looks.

**Edges:** aura fine; concentration is the weak point (above); resume fine;
trap fine; silent absence best-in-class for chains, unchanged for
observation; migration worst-in-class.

## Option D — effects as pure data, no closures (the different direction)

Modifiers described entirely as data — `{stage: conditions, op: add_disadvantage,
when: {target_is: self}}` — interpreted by a rules engine. No Go code per
effect.

**Why it is on the table:** it is the homebrew endgame — content authored as
JSON, shipped without compiling, the `homebrew.classes.artificer` future from
the ref-routing discussion. Partially latent already in `loadJSON` configs.

**Why not now:** the predicate language is a DSL, and the DSL grows until it
is a worse programming language — the aura needs spatial predicates, Shield
needs "after the roll is known", concentration needs cross-entity references.
Each is expressible; the sum is an interpreter we would be maintaining
instead of Go. It also ends the write-a-function authorship model rather than
preserving it. **Deferred, not rejected:** B does not foreclose it — a
generic `DataDrivenEffect` implementing `Apply` from declarative config can
arrive later as one more Attachable, which is likely the right door for
homebrew when it comes.

## The score

| | A: per-chain methods | **B: subscribe** | C: declarations | D: pure data |
|---|---|---|---|---|
| Aura | ✓ | ✓ | ✓ | DSL pain |
| Concentration | needs bus anyway | **✓ one mechanism** | needs bus anyway | DSL pain |
| Shield / resume | ✓ | ✓ | ✓ | ✓ |
| Trap | ✓ | ✓ | ✓ | ✓ |
| Silent absence | good | bounded + testable | best (chains only) | good |
| Migration (~26 sites) | rewrite ×2 surface | **~zero** | rewrite | total |
| Mechanisms needed | 2 | **1** | 2 | 1 + interpreter |
| ADR precedent | violates 0007 | extends 0024/0026 | half-violates 0007 | — |

## The Bless example — prior art that amends a ruling

`events/example_journey_test.go` already contains the pattern (Kirk's
recall, verified): `BlessSpell{targets []string, bonus int}` implements
`Apply(bus)`, subscribes to the attack chain, and **projects onto its targets
by ID** — one effect object, living with its owner, matching beneficiaries by
predicate. The aura and Bless are the same shape with different predicates:
proximity versus an ID list.

This amends 053's concentration ruling in a strictly better direction:

- **Breaking concentration is a single-record write.** Bless lives on the
  caster's list; the break removes one condition from one character. There is
  no ripple to the targets, because the targets carry nothing.
- **The orphan problem disappears instead of self-healing.** "Granted
  conditions are claims validated at load" was solving a problem this shape
  does not have. (The dirty-participants ruling stands — damage to several
  targets still writes several records.)
- **The price is already paid** by the pass-everyone-in scope ruling: the
  target's roll sees the owner's projection only if the owner's effects are
  attached when the target acts.

The principle for the ADR: **an effect lives with the entity whose lifecycle
owns it.** Bless dies when the *caster's* concentration breaks → it lives on
the caster. Prone ends when the *target* stands → it lives on the target.
Storage location is derived from what kills the effect, never a style choice
— and concentration linkage then costs nothing, because the effect already
sits on the thing whose state controls it.

It is also evidence for B rather than argument: the example *is* option B —
`Apply(bus)`, `SubscribeWithChain`, predicate over event fields — already in
the repo.

## Recommendation

**B**, and the flag first, per the process note: B is attractive *because it
fits* — it is the smallest change and it is literally the sentence Kirk said.
The check that it is also *true*: the two options that beat it on paper each
need a second mechanism for observation (A, C), and the edge that decides —
concentration — is precisely an observation edge. B is the only shape where
one mechanism covers the whole job, and its real weakness (contribution
invisible until execution) is compensated where it matters by source records
in chain outputs.

What would change the recommendation: a genuine consumer need to inspect
"what *would* this effect contribute" before execution — a planner UI, an AI
scoring hypothetical actions. If that arrives, C's declarations become worth
their migration, and B's `Apply` objects can grow a `Contributions()` view
without breaking anyone — B → B+C is additive; C → B is not.

## The refinement: does Apply need a bus? (Kirk, 2026-08-14)

> *"Does Apply need a bus? What if the apply was in resolution and we just
> asked: gimme the data that powers the effect."*

The layered answer, which resolves B's last conceded weakness:

**What Apply irreducibly needs is a place to register interest**, because the
topics are the effect's own domain knowledge — Bless knows it cares about
attack chains; concentration knows it cares about damage. Moving that out of
the effect is option C's declaration vocabulary, already declined.

**What Apply does NOT need is the bus as an owned thing.** Unbundle "the bus"
into three parts: the registration surface (the effect needs it), the
instance and its lifetime (resolution's alone), the delivery machinery (an
implementation detail). B as originally written hands effects all three; the
refinement keeps only the first in the effect's hands.

**"Gimme the data that powers the effect" is the other half, already built:**
effects persist as ref + config (`ToJSON`), `factory.CreateFromRef` routes
ref → behavior. Resolution's loop:

```
for each participant's effect record:      // the data that powers it
    behavior := factory(ref, config)       // routing already exists
    behavior.Apply(ctx, surface)           // names its topics; nothing else
```

**The free win: `events.EventBus` is an interface**, so the surface resolution
passes can be an *instrumented* implementation that records every subscription
at attach time. At zero migration cost (~26 Apply sites keep their exact
signature) this buys:

- **Pre-execution inspectability** — the weakness conceded to C. Resolution
  can answer "bless hooked the attack chain at StageConditions" before
  anything executes, because it watched the registration happen.
- **Silent absence becomes observable** — "this effect attached zero hooks"
  is a loggable, assertable fact; the per-effect test writes itself.
- **Deterministic teardown** — resolution drops every subscription it
  granted, trusting no effect to clean up (the `character.Cleanup` trap,
  avoided structurally).

A further tightening — a capability type that cannot publish — stays
available as a later, deliberate signature migration. Not forced now.

**The picked shape, in one sentence:** effects are data records; resolution
routes ref → factory → behavior and attaches each behavior through an
instrumented surface it owns — *the bus never leaves resolution*.

## The shape in pseudocode (Kirk asked to see it, 2026-08-14)

The layers — note there is nothing between session and resolution; the depth
is *below* resolution, as library calls returning values:

```
host (rpg-api)
  │  IDs in, projections out (S2)
  ▼
session.Manager      custody: repositories, the lock, persistence. No bus, no rules.
  │  data in, data out
  ▼
resolution           custody: THE bus — created here, dies here. The single attach site.
  ├─→ combat         rules vocabulary: chains, stages, phases     (no bus)
  ├─→ encounter      the world: geometry, story, clocks           (no bus)
  └─→ effects        self-contained; each Apply(ctx, surface)
```

The session verb — gather data, lock, hand off, save what returns dirty:

```go
func (m *Manager) Strike(ctx, in *StrikeInput) (*StrikeOutput, error) {
    unlock := m.locker.LockSession(ctx, in.Session)   // pessimistic, buffer+timeout
    defer unlock()
    scope := m.openForChange(ctx, in.Session)

    party := getAll(m.characters, scope.data.Members) // EVERYONE — the aura ruling
    out := resolution.Resolve(ctx, &resolution.Input{
        World: scope.encData, Characters: party, NPCs: scope.data.NPCs,
        Action: resolution.Strike{Actor: in.Member, Target: in.Target},
    })

    for _, ch := range out.DirtyCharacters { m.characters.SaveCharacter(ctx, ch) }
    m.encounters.SaveEncounter(ctx, scope.encounter, out.World)
    m.sessions.SaveSession(ctx, scope.data)           // world-then-session, as today
    return project(out), nil                          // projections, never objects
}
```

Resolution — Kirk's sentence in order: creates the bus, takes all the data,
applies the bus to them, takes the action:

```go
func Resolve(ctx, in *Input) *Output {
    bus := instrument(events.NewEventBus())     // records every Subscribe

    world := encounter.LoadEncounter(in.World)
    ctx = gamectx.With(ctx, world)              // what effect predicates may read

    participants := map[ID]*loaded{}
    for _, data := range sortByID(in.Characters, in.NPCs) {  // sorted = C8
        p := load(ctx, data, bus)   // ref routes to loader (ADR-0037); each
                                    // behavior.Apply(ctx, bus) names its topics
        participants[p.ID] = p
    }

    // combat is a PURE STEP MACHINE — it never sees the bus (Kirk's catch:
    // an earlier draft passed it in, violating the charter). The suspension
    // requirement forces this shape anyway: W5 suspends BETWEEN phases in a
    // fresh process, so phases must return control at every boundary — which
    // means combat describes what to gather and resolution does the gathering.
    st := combat.NewStrike(in.Action.Actor, in.Action.Target)
    var outcome combat.Outcome
    for done := false; !done; {
        switch s := combat.Next(ctx, st, world).(type) {
        case combat.Gather:                 // "publish this chain event, hand me the fold"
            folded := gather(bus, s.Event)  // resolution's only job; the bus stays home
            st = s.Resume(st, folded)       // st is DATA here — a legal suspend point
        case combat.Done:
            outcome, done = s.Outcome, true
        }
    }

    return &Output{
        World:           world.ToData(),
        DirtyCharacters: dirtyOf(participants),  // IsDirty already exists
        Outcome:         outcome,                // values, not events
        Hooks:           bus.Registrations(),    // "what attached", as data
    }
}
```

The effect — unchanged, which is the point (this is the repo's own
`BlessSpell`, reshaped only cosmetically):

```go
func (b *BlessSpell) Apply(ctx, bus events.EventBus) error {   // today's signature
    return AttackChain.On(bus).SubscribeWithChain(ctx,
        func(ctx, e AttackEvent, c chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
            if !contains(b.Targets, e.AttackerID) { return c, nil }
            return c, c.Add(StageConditions, "bless", plusD4(b.Bonus))
        })
}
```

And the concentration link the repo's example lacks (Kirk's catch) — two
fields and a lifecycle subscription:

```go
type BlessSpell struct {
    Caster        ID     // THE LINK: whose lifecycle owns this effect
    Targets       []ID
    Bonus         int
    Concentration bool   // the flag that already exists in effects/types.go
}

func (b *BlessSpell) Apply(ctx, bus events.EventBus) error {
    attackHook(bus, b.Targets, b.Bonus)   // contribution, as above

    // lifecycle: listen for THE CASTER taking damage
    return DamageReceived.On(bus).Subscribe(ctx, func(ctx, e DamageReceivedEvent) error {
        if e.TargetID != b.Caster { return nil }
        // Do not roll here — REQUEST a follow-up interaction:
        requestSave(ctx, SaveRequest{
            Saver: b.Caster, Ability: CON, DC: max(10, e.Damage/2),
            Trigger: SaveTriggerConcentration,   // already exists
            OnFail:  Expire{Effect: b},          // single-record write, on the caster
        })
        return nil
    })
}
```

`requestSave` queues a follow-up that resolution runs as its own step sequence
after the damage phase — the effect never rolls inline. Three reasons that is
right rather than convenient: it is ADR-0027's reaction-window shape (things
that happen *between* phases); the save is a chain of its own, so the aura's
+CHA folds in — the caster-in-her-own-aura case composes with no special
code; and a queued save is data, so a suspension landing between the damage
and the save persists cleanly. On failure, `Expire` removes Bless from the
caster's list — lifecycle ownership making the break a single-record write.

Three sharpenings the code makes visible:

1. **"session →→→ resolution" is one arrow.** No intermediate layers; combat
   and encounter sit *below* resolution as value-returning libraries.
2. **Not every verb enters resolution.** A pure free-roam walk loads nobody —
   `session → encounter` directly (~187µs, measured). Resolution is entered
   when an *interaction* occurs: a strike, a trap cell, first contact. Two
   doors into one building.
3. **Combat never sees the bus** — it is a step machine over data, describing
   what to gather; resolution gathers. Forced by suspension, not taste: W5
   resumes in a fresh process, so no phase may live on a stack. Every
   `Gather` return is a legal suspension point, which makes the charter's
   "re-enterable from line one" the shape of the code rather than a
   discipline imposed on it. This is also Kirk's "combat is the composable
   data" noodle, realized.

On "I was trying to remove the bus from Apply": what was removed is real even
though the parameter remains — the effect receives an interface whose
concrete value is resolution's recording surface. It cannot own the bus,
cannot outlive the call, cannot escape observation. The leftover `bus`
parameter is a registration surface wearing an old name.

## The pattern generalizes: one driver, many machines (Kirk, 2026-08-14)

> *"I like `combat.NewStrike` — would we have those in all the things we can
> do? `character.NewActivateFeature`, `something.Move` — and we would just do
> the gather on what we get?"*

Yes — every verb ships a step machine, and resolution is **one generic
driver** that does not know strike from trap. The step vocabulary needs to be
slightly richer than `Gather` alone:

```go
for {
    switch s := machine.Next(ctx, st, world).(type) {
    case Gather:  st = s.Resume(gather(bus, s.Event))     // fold a chain
    case Pose:    return suspend(st, s.Window)            // ask someone; persist; die
    case Request: queue.push(s.Interaction); st = s.Cont  // a follow-up machine
    case Done:    return finish(s.Outcome, queue)         // drain queue, then report
    }
}
```

The use cases, walked:

| Interaction | Machine | Steps that fire |
|---|---|---|
| Strike | `combat.NewStrike` | Gather (attack) → Gather (damage) → Done |
| Rage | `features.NewActivate` | Done — zero gathers, still uniform; facts ride out as values |
| Walk | `encounter.NewWalk` | geometry steps; **Pose on first contact** — this is W2's checkpoint |
| Trap mid-walk | `trap.NewTrigger` via **Request** from the walk | Gather (DEX save — the walker's Dodging folds in) → Done; walk resumes |
| Concentration save | save machine via **Request** from Bless's observation | Gather (save chain — the aura folds in) → Done |
| Reaction (W5) | **Pose** between a strike's phases | human and machine answerers indistinguishable (play/interrupt's charter) — a monster's auto-reaction is the same Pose answered by composition |
| Aura, Bless | **not machines — effects.** They fold into whatever chains machines gather. The two vocabularies do not overlap, which is how you know the seam is real. |

Three findings the walk-through surfaced:

1. **The walk was the first machine, hand-rolled.** `frozenResolution{Kind:
   kindWalk, Path, Index}` already IS persisted step-machine state — the
   pattern predates its own vocabulary. And since the walk machine lives in
   the composition, **#964's "move the trigger rule out of the session"
   stops being a migration and becomes a restatement**: the walk machine
   ships with the package that owns trigger detection.
2. **The step vocabulary must be sealed and tiny** — `Gather | Pose |
   Request | Done`, the way `Intent` is sealed. If packages can invent step
   kinds, the driver grows a switch forever. Four kinds; extensions need an
   ADR.
3. **Open, to be settled by the first implementation rather than argument:**
   Pose and Request overlap at the edges — is a reaction window a Pose
   (ask mid-machine) or a Request of an interrupt-machine? Lean:
   Pose-is-primitive, since it maps 1:1 onto the existing `interrupt.Ledger`
   pose/answer.

## What ADR-0038 records, once picked

The chosen interface; the rejects with reasons (including D's deferral and
its homebrew door); the rulings it inherits — pass everyone in,
dirty-participant returns, and **an effect lives with the entity whose
lifecycle owns it** (which replaced 053's claims-validated-at-load ruling
once the Bless example resurfaced); and the determinism rule (attach in
sorted participant order).
