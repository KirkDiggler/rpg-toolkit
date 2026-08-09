# Journey 051: The Encounter Reset — From Application Back to Toolkit

*2026-08-08. Kirk audited the encounter module end-to-end after a season of
giving agents more autonomy, and made a call: we're going back to our roots.*

## The occasion

The encounter module grew from a walking skeleton (PR #622) to ~18k lines of
production Go and ~25k lines of tests across 115 commits in three months,
built wave by wave, issue by issue, largely by autonomous agents passing
review gates. It runs the live game. Its scorecard grade is B+, and for
*behavior* that grade is earned — the fog-of-war projection is genuinely
good, the sealed event taxonomy is a solid pattern, and the test suite is
what kept the whole thing upright.

Then Kirk read it.

## The smell

Every individual change in the history is locally defensible. You can watch
the review gates working: rollbacks added from Copilot catches, QA findings
closed, invariants pinned by regression tests. What no agent ever did was
stand at altitude and ask whether the **shape** was still right. The audit
found:

- **A god-aggregate.** One `Encounter` type with methods across ~20 files
  owning combat, movement, fog of war, doors and lock-picking, reactions,
  death saves, turn economy, NPC driving, dungeon *generation*, authored
  crypt content, and event brokering over a pluggable network transport.
  That is not a module. That is the game loop, living in the toolshed.
- **Dual state representation.** Every seat carries a flat HP/AC/damage
  snapshot *and* a full serialized character blob. Two sources of truth,
  whole subsystems built just to keep the twins in sync, and a bug family
  (#683/#684/#783/#784/#808) that is mostly this one decision's children.
- **The bus as a return channel.** The SDK learns what its own resolvers did
  by installing buffered subscribers around each call and draining captured
  events afterward, with load-bearing install-ordering to avoid
  double-applying damage. A return value smuggled through pub/sub.
- **A boundary inversion.** The toolkit charter says we never orchestrate
  and never persist. The module introduces itself as "the orchestrator-facing
  facade" with a Redis-capable transport. It claims to be rulebook-agnostic
  while hard-importing dnd5e character, monster, and combat packages, and it
  bakes cube-coordinate hexes into events, persistence, prompts, and AI.
- **Review-defense comments.** Functions carry essays addressed to past
  reviewers — issue numbers, gate blockers, Copilot catches — the visible
  residue of agents optimizing for passing the *next* review rather than
  informing the next reader. The archaeology belongs here, in journey docs.
  It ended up inline instead.

## The reframe (Kirk's steer)

> "It is an application, not a toolbox. I want to build a tool more than
> anything. I am an architect and not an application developer, and I would
> like to enjoy what we build even if it will not be useful for a while."

That's the whole insight. A **tool is generative** — it does things its
author never anticipated, because its pieces compose. The encounter module
is **exhaustive** — it does exactly what its 115 commits of issues asked
for, and nothing else. You don't build *with* it; you file an issue
*against* it.

The toolkit's own philosophy already says this: *infrastructure, NOT
implementation — we provide the tools, games implement the rules.* `dice`,
`events`, `spatial`, `selectables`, `behavior` honor it. Encounter inverted
it, and nobody stopped it, because every step was a reasonable response to a
well-scoped issue.

The measure of the tool we actually want is **time-to-new-mechanic**. A
stealth encounter, a chase scene, a trap hex, a haunted room where players
hear what they can't see — today each of those is a multi-PR excavation
through the aggregate. In a composed design, each is one new piece plugged
into unchanged machinery. That is what powerful feels like.

## The seven axes

Reading what encounter actually does, it is seven orthogonal concerns fused
together. Pulled apart, they are the toolbox:

1. **Field** — space, occupancy, blockers, topology. `tools/spatial`
   already is this (square, hex, gridless); encounter bypassed it and
   hard-picked hexes. Geometry gets *confined* to this axis, not smeared.
2. **Knowledge** — what each observer knows. Sight is one *channel*;
   hearing, tremorsense, a scrying pool are the same shape: "what does
   observer O learn when fact F occurs." Per-observer memory and projection
   live here — the one part of the old module worth admiring.
3. **Clock** — the scheduling policy. Free-roam and initiative turns are two
   policies; phases, simultaneous resolution, and real-time ticks are
   others. "Combat entry" stops being a hardwired visibility check and
   becomes a policy transition.
4. **Resolution** — the rulebook seam: intent in, outcome out, **as return
   values**. All of dnd5e lives behind this line — economy, AC, death
   saves, conditions. The generic layer stores opaque rulebook state; it
   never *understands* it.
5. **Interruption** — suspend/resume as a first-class control-flow
   primitive. Opportunity attacks, Shield prompts, lock-pick checks, and
   dialogue choices are all one mechanism: pause this resolution, ask this
   decider, resume with the answer.
6. **Record** — the ordered, correlated, audience-projected event log.
   Sequence numbers, correlation IDs, per-viewer slices.
7. **Deciders** — anything that turns a view into an intent. A player's
   client and a monster's behavior tree are the same kind of thing attached
   to different entities, and both execute through the same verbs. Behavior
   can't cheat, and the `behavior/` module finally gets its consumer.

The power is in the products, not the pieces: a new mechanic is a new
implementation on *one* axis times everything that already exists. Features
multiply instead of add.

## The decision: clean room, no salvage

We are starting a new version, and we are not salvaging code. Not because
the old code is worthless — because salvage drags the diseases along. Every
seam in the aggregate assumes fusion; a refactor toward composition would
spend its entire budget fighting assumptions we already know are wrong.
Knowing what we know now, a clean build is the shorter path *and* the one
worth enjoying.

What carries forward is knowledge, not code:

- The **behavioral record**: the old module's scenario tests document real
  invariants (unconscious-mid-move OAs, pocket-clear teardown, TPK gated by
  death saves). We know these behaviors exist and matter. The new tool will
  earn them on its own terms, not inherit their fixtures.
- The **named diseases**: one representation of state; resolvers return
  values; the bus is for observers. These are now design law.
- The **boundary lessons**: the wire layer (broker/transport/per-viewer
  fan-out) is host-side machinery and does not come back into the toolkit.

The old module is not deleted and not deprecated today — it runs the live
game and keeps doing so. It becomes what it always secretly was: the
reference application. New investment stops flowing into it.

## The discipline for the new build

Two rules keep a composable toolkit honest, learned the hard way:

1. **Ship the assembled default.** A dnd5e hex-crawl assembly, batteries
   included, so the common case is one constructor — Godot's lesson: power
   is composable nodes *plus* good defaults.
2. **The default is built only from the public pieces.** No privileged
   access, no private magic. If our own assembly needs a backdoor the
   pieces don't expose, the tool has already failed. The old encounter *is*
   the backdoor; the new one never gets to have it.

And one rule for how we write it: comments state the surviving invariant;
the story of how we got there goes in journey docs like this one.

## Naming: a false start, then the answer was already there

The name matters because the name is the frame. The first pass shortlisted
`session` (bland, collides with every web concept), `runtime` (honest but
smells like systems plumbing), `arena` (combat-only), `scene` (collides
with the Three.js scene graph our own client lives in), `theater`
(spelling-schismed), `table` (names the furniture) — and recommended
**`stage`**: where actors play scenes under a director, a metaphor the
team's own role vocabulary already used.

Kirk rejected it in one sentence:

> "A stage is a location, and an encounter has an outcome."

That's the whole review. `stage` names the **venue** — it quietly promotes
one axis (the field) to umbrella status, and it was seductive precisely
because the old module's best part was spatial. But the thing we're
building is not a place. It is a bounded situation that **resolves**:
entities in a field, on a clock, with stakes, ending in a result the rest
of the game consumes. The domain has had the right word for fifty years.
It's **`encounter`**. The old module didn't fail because of its name; it
failed because it was an application. We keep the word and rebuild what
wears it.

Mechanically this is clean: the module line is v0 (rpg-api pins
`encounter v0.50.1`), and v0 makes no compatibility promise. When the old
module is replaced it is simply **deleted** — the new one takes the same
import path on later v0 tags, and every old pin stays resolvable from the
module proxy until its consumers migrate. No `/v2` suffix, no corpse
carried forward.

## What kind of thing is an encounter?

The toolkit organizes by category: `mechanics/` is rules infrastructure
(conditions, resources, spells), `tools/` is world infrastructure
(spatial, environments, spawn), `rulebooks/` is game content, with
`core`/`events`/`dice` beneath and `behavior` alongside. The layer rule
reads Core → Events → Mechanics → Tools → Rulebooks. So where does an
encounter live? What *kind* of thing is it?

**An encounter is a composition with an outcome.** It is not a mechanic
(it *uses* mechanics), not a tool (it *assembles* tools), not content (it
*hosts* content). It is the layer that wires infrastructure into live play
and runs it to a result.

Two of the seven axes already have homes we consume rather than rebuild:
the **field** is `tools/spatial` (+ environments), and **deciders** are
`behavior/`. The four genuinely new runtime axes get a new category,
parallel to `mechanics/` and `tools/`:

```
play/
  clock/       scheduling policies — free-run, initiative turns, phases
  knowledge/   per-observer channels, memory, projection
  record/      the ordered, correlated, audience-scoped log
  interrupt/   suspend/resume as first-class control flow
```

And `encounter` — the reclaimed top-level module — is the thin composition
layer: it defines the **resolution seam** rulebooks implement, wires the
axes together, owns end conditions, and returns an **Outcome**. The layer
rule gains one entry: … → Tools / Play → **Encounter** → Rulebooks (which
implement the seam and ship the assembled dnd5e default).

The outcome being first-class is itself new. The old module modeled "it
ended" as a mode enum plus a reason string (`"tpk"`). The new one treats
an encounter as a function run interactively:

```
Setup → (play: intents, interrupts, records) → Outcome
```

— deaths, discoveries, spoils, map knowledge, whatever the rulebook says
carries out. That is what makes encounters composable at the next level
up: a campaign is a thing that chains encounters by consuming their
outcomes. The word always implied this. Now the type does.

## Leaf modules and the version story

Every axis is its own Go module — the pattern the repo already runs
(`tools/spatial/v0.5.x`, `mechanics/spells/v0.2.1`): own `go.mod`, own
directory-prefixed tag line, own release cadence. Kirk's call, and the
reasoning is more than taste: **isolation is what makes versioning safe.**

In Go, different major versions are different packages. If `encounter`
speaks `clock/v2` types while `record` still expects `clock/v1` types,
they cannot interoperate — that's the diamond hazard every multi-module
family carries. The defense is structural, not procedural:

> **The dependency-direction law.** `play/*` leaves depend on `core` (at
> most `core` + `events`/`dice`), never on each other, and never on
> rulebooks. Shared vocabulary lives in `core`'s small types. A flat
> graph has no diamonds.

The old module's `go.mod` is the named violation not to repeat:
`encounter → rulebooks/dnd5e` is the exact leaf-to-content edge that made
it un-generic *and* would have made it unversionable. Kept light, an axis
like `knowledge` is nearly a pure library — observers, channels, memory,
projection — testable with no game attached. Simple and powerful is the
same property seen from two sides.

Go enforces the import path, not the promise: the toolchain guarantees a
`/v2` can't be imported as v1, but never checks that v0.5 actually works
with what v0.4 worked with. So compatibility is checked in two layers we
own:

1. **Per-module API gate** — `gorelease`/`apidiff` in CI diffs each
   module's exported surface against its last tag, fails accidental
   breaks, and names the next allowed version.
2. **The assembled default is the compatibility manifest.** Build-law #1
   does double duty: the dnd5e assembly's `go.mod` pins the exact family
   set its behavioral suite proves works together. If the assembly
   resolves and its tests pass, the family is compatible. The matrix test
   *is* the shipped product.

One bridge deliberately left for later: a **parallel** encounter module
while the old one still runs. Semantic import versioning allows
`encounter/v2` (a `v2/` subdirectory, tagged `encounter/v2.0.0`) to
coexist with the old path — even in the same binary, which would let
rpg-api migrate incrementally. The cost: the import path wears `/v2`
forever. The alternative: build under a scaffold name and take over the
clean path at cutover, a trivial rename while the new module has no
consumers. Both work; neither needs deciding until the composition module
actually exists — the axes come first either way.

## Open questions

- **Category name** — `play/` is the proposal for the four new axes
  (parallels `mechanics/` and `tools/` grammatically: the mechanics of
  rules, the tools of worlds, the play of running them). Kirk blesses or
  renames.
- **First axis** — the decider seam is the smallest and unblocks the
  monster-behavior work; the clock is the spine and the most fun to
  design. An architect gets to pick joy: nothing downstream forces the
  order.
- **Parallel path vs takeover** — `encounter/v2` side-by-side or scaffold
  name until cutover (see above). Decide when the composition module
  exists.
- **Cutover** — old `encounter` is bugfix-only from here (proposed);
  deletion happens whenever the new assembly actually hosts a game.
  Nothing forces a date; usefulness is allowed to arrive later. That's
  the point.
