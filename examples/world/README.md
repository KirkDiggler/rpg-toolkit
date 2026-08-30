# examples/world — the living-world spike

Two scenarios, one composer, one needle. No code anywhere that knows which route
a player took, which company took which job, or who to thank when a region gets
pacified.

This module is a spike, not a product. It lives under `examples/` on purpose:
example code is unadoptable by construction, so the seams stay fluid and nothing
here mints a tag or promises an API. If the seams hold and the walk agrees,
`journal`, `graph`, `quest` and the composer move out to `world/*`, and the
scenarios get rewired to import them exactly as a rulebook would.

## Layout

```
world/                 the composer — assembly, the one write door, the one read door
  journal/             memory: append-only, attributed, audience-scoped facts
  graph/               structure and the present, derived by fold and never stored
  quest/               jobs: templates, populations, claims, distributions
  goal/                the guild's needle: a condition over a region, with a clock
  dnd5eresolver/       the rulebook adapter — the only place a d20 exists
  scripted/            deterministic dice and a stopped clock, so a run reproduces
  banditcamp/          UC-1 content: one camp, five ways through
  hostagecamp/         UC-2 content: one job, three companies, three hostages
  region/              UC-3: composes both camps and states one goal over them
```

The arrows only point one way: `journal <- graph <- quest <- goal <- world`, and
the content sits above all of it. A test parses the imports and fails if that
ever stops being true.

A quest is somebody's job — claimed, about a person or a place, finished by
whoever took it. A goal is nobody's job: no claimant, no subject, and it moves
when the region moves, by whatever means and in whoever's hands.

## Declare, inject, subscribe

A rulebook builds a world through exactly three touchpoints, and the types are
those three:

1. **Declare** — `world.Scenario` is what a content package hands over: graph
   declarations, verbs, quest templates. All data. See `banditcamp.Scenario`.
2. **Inject** — `world.Resolver` is what content deliberately withholds.
   `world.Config` is a Scenario plus a Resolver, and the split in that struct is
   the whole boundary.
3. **Subscribe** — `world.Result` carries the fact that was written and what the
   jobs made of it, returned as values. No bus, and none wanted.

```go
scenario, err := world.Compose(banditcamp.Scenario(), hostagecamp.Scenario(), ties)

w, err := world.New(world.Config{
    Scenario: scenario,   // declared content
    Resolver: resolver,   // injected rules — anything answering "did that work"
    Clock:    clock,      // injected time — deadlines are arithmetic, not simulation
    Goals:    []goal.Goal{region.WeekendGoal(friday)},
})
result, err := w.Act(ctx, world.Act{Verb: banditcamp.Sneak, Actor: rook, Target: camp})
state := w.View(camp)     // the camp's present, folded from what the camp saw
```

Goals and the clock sit on the injected side beside the resolver, and that is a
statement rather than a convenience: a guild goal spans whatever content the
region was composed from, so no single piece of content is in a position to
declare one.

## What the two scenarios prove

**UC-1, the bandit camp.** Five routes through one declaration, none of them a
branch. Kick the door in and the camp is alerted because it witnessed an
assault. Come over the wall and it is surprised because it witnessed nothing.
Kill the chief quietly and wear his face and the camp follows you. Talk instead
and regard crosses a threshold. Blow the disguise in front of one lieutenant and
that lieutenant folds a different present from the camp he stands in.

**UC-2, the hostage camp.** One job with three names on it. A claim takes one
off the board and mints the claiming company's own instance about that person,
and nothing ever puts one back. Ember's failure turns Ember's hostage; Quill's
and Thorn's are untouched, and no code compares companies to make that so. When
the population settles into "nobody captive, everybody turned", a follow-up
opens about exactly those people — a fold over a census, not a rule anybody
wrote — once, and never again.

**UC-3, the weekend goal.** One region composed from both camps, sharing one
journal. Three companies push one needle by three different methods — one talks
the camp round, one frees hostages, one loses a hostage and finishes it the hard
way — and nothing anywhere adds their contributions together, because there is
nothing to add. The needle is a fold. Swap which company does what and the
result is identical; settle the camp by force instead of by talking and the
needle reads the same. `GoalMet` fires once, before the deadline or not at all:
finishing late changes the world and not the unlock.

## Two things worth knowing before you read the code

**Present state is a pure function of (what was declared, what was witnessed).**
Nothing is stored, so nothing can drift. Rewind the journal and the world
rewinds with it; the tests assert exactly that.

**"Before the deadline" is strict.** Meeting the conditions at the exact deadline
instant is a miss, the way finishing as the weekend starts is not finishing
before it. And a miss can only be noticed by *looking* — the absence of an act is
not an act — so a host calls `ObserveGoals` on whatever tick it already has.

**Flags only go up.** Nothing clears one, because unwitnessing is not an event.
A redeemed hostage still carries the flag that says they turned. Redemption wins
by *precedence*: its projection is declared after turning's, and its bucket is
asked before turning's. Order carries meaning — see `hostagecamp.Declaration`.

## Running it

```bash
cd examples/world
go test ./...
```

Everything is deterministic: `scripted.NewRoller` hands the rulebook a
written-down sequence of d20 results, so the same script always produces the
same facts, folds, and ending.

Read the package doc comments before the code; each states its own contract.
