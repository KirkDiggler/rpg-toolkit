# examples/world — the living-world spike

Two scenarios. One composer. No code anywhere that knows which route a player
took, or which company took which job.

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
  quest/               goals: templates, populations, claims, distributions
  dnd5eresolver/       the rulebook adapter — the only place a d20 exists
  scripted/            deterministic dice, so a whole run reproduces
  banditcamp/          UC-1 content: one camp, five ways through
  hostagecamp/         UC-2 content: one job, three companies, three hostages
```

The arrows only point one way: `journal <- graph <- quest <- world`, and the
scenarios sit above all of it. A test parses the imports and fails if that ever
stops being true.

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
w, err := world.New(world.Config{
    Scenario: banditcamp.Scenario(),
    Resolver: resolver,   // dnd5eresolver, or anything that answers "did that work"
})
result, err := w.Act(ctx, world.Act{Verb: banditcamp.Sneak, Actor: rook, Target: camp})
state := w.View(camp)     // the camp's present, folded from what the camp saw
```

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

## Two things worth knowing before you read the code

**Present state is a pure function of (what was declared, what was witnessed).**
Nothing is stored, so nothing can drift. Rewind the journal and the world
rewinds with it; the tests assert exactly that.

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
