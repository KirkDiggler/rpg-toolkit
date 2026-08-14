# The subscribe interface — options for ADR-0038

**Status:** options on the table, awaiting Kirk's pick. The winner and the
rejects go into ADR-0038.
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

## What ADR-0038 records, once picked

The chosen interface; the rejects with reasons (including D's deferral and
its homebrew door); the three 053 rulings it inherits (pass everyone in;
dirty-participant returns; granted conditions are claims); and the
determinism rule (attach in sorted participant order).
