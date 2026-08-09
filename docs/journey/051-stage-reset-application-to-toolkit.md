# Journey 051: The Stage Reset — From Application Back to Toolkit

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

## Naming

The name matters because the name is the frame. Candidates considered:

- `encounter/v2` — carries the corpse. No.
- `session` — accurate, bland, collides with every web concept.
- `runtime` — honest ("rulebooks are data, this is what runs them") but
  smells like systems plumbing, not a game.
- `arena` — combat-only; the whole point is scenes beyond combat.
- `scene` — right idea, collides with the Three.js scene graph our own
  client lives in.
- `theater` — evocative (theater of the mind) but spelling-schismed.
- `table` — charming (what happens at the table) but names the furniture,
  not the machinery.
- **`stage`** — where actors play scenes under a director. Short, unclaimed
  in the module tree, reads beautifully at call sites (`stage.Clock`,
  `stage.Field`, `stage.Decider`), covers combat and free-roam and social
  alike, and — not incidentally — this project's team vocabulary already
  has a **director**. Actors, a stage, a director: the metaphor was already
  ours.

Recommendation: **`stage`** as the umbrella, with the seven axes beneath
it. "Staging an encounter" is exactly what the assembly does.

## Open questions

- **Name signoff** — `stage` is a recommendation, not a decree. Kirk calls
  it.
- **Module layout** — one module with subpackages (`stage/field`,
  `stage/knowledge`, ...) vs sibling modules per axis. Subpackages keep the
  early iteration cheap; split later when an axis proves independently
  versionable.
- **First axis** — the decider seam is the smallest and unblocks the
  monster-behavior work; the clock is the spine and the most fun to design.
  An architect gets to pick joy: nothing downstream forces the order.
- **Long-term fate of `encounter`** — freeze policy (bugfix-only?) and
  whether the live game ever migrates, or the new stage simply hosts the
  *next* thing. Deliberately not decided here; usefulness is allowed to
  arrive later. That's the point.
