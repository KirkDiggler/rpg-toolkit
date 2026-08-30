# examples/world — the living-world spike

One camp. Many ways in. Many endings. No code anywhere that knows which one you
took.

This module is a spike, not a product. It holds three kernel packages and the
bandit camp that wires them to real D&D 5e, and it lives under `examples/` on
purpose: example code is unadoptable by construction, so the seams stay fluid
and nothing here mints a tag or promises an API. If UC-1 proves the seams and
the walk agrees, `journal`, `graph` and `quest` move out to `world/*` and this
example gets rewired to import them exactly as a rulebook would.

## The three packages

```
journal  <-  graph  <-  quest
(memory)     (structure    (goals)
              + present)
```

- **`journal`** — an append-only log of attributed, audience-scoped facts.
  Depends on nothing. Defines the vocabulary everything else is written in, plus
  `Resolver`, the one seam a host fills in.
- **`graph`** — entities, typed edges, roles as slots, and the declared folds
  that turn facts into the present. Stores no present state at all.
- **`quest`** — templates, instances, claims, and objectives as predicates over
  derived state. Watches; never acts.

`banditcamp` is the only package permitted to import a rulebook, and a test
enforces it.

## The claim being tested

Present state is a pure function of *(what was declared, what was witnessed)*.
Nothing is stored, so nothing can drift.

Knowledge is the audience of facts. There is no belief database: an entity's
present is the fold over the facts it witnessed, which is why stealth is
controlling the audience of your own events and disguise is planting a fact in
someone else's feed.

Five routes through one camp are asserted, and not one of them is a branch:

| route | what the player did | why the world moved |
|---|---|---|
| Front door | approach, assault, win | the camp witnessed an assault, so it is alerted and formed up; defeat retires its hostility |
| Back way | sneak, enter | the camp witnessed nothing, so it is surprised |
| Changeling | quiet kill, claim command | allegiance follows whoever holds the `leads` slot, and the camp never heard about the body |
| Diplomacy | parley, persuade ×3 | regard crossed the declared threshold, converting `hostile-to` into `allied-with` |
| Blown disguise | changeling, then fail in front of one lieutenant | the lieutenant folded one extra fact and reached a different present from the camp he stands in |

## Running it

```bash
cd examples/world
go test ./...
```

Everything is deterministic: `ScriptedRoller` hands `checks.MakeAbilityCheck` a
written-down sequence of d20 results, so the same script always produces the
same facts, folds, and ending.

## Wiring your own

Three touchpoints, and no fourth:

1. **Declare** — content as data. See `banditcamp.Declaration`, `Verbs`, and
   `Contract`. There is no route into the camp in any of them.
2. **Inject** — implement `journal.Resolver`. `banditcamp.CheckResolver` is
   ~40 lines over real character sheets. A different rulebook is a different
   resolver and nothing else.
3. **Subscribe** — `quest.Instance.Observe` returns events as values. What
   completing a contract is *worth* is your rulebook's opinion.

Read the package doc comments before the code; each states its own contract.
