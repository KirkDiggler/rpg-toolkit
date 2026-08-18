# encounter

The D&D 5e **composition**: the module that turns the `play/*` leaves into a
world you can actually play in.

If you are here to write **monster behavior**, skip to
[Writing a decider](#writing-a-decider). That is the seam, and it is the whole
job.

## What it is

An encounter is a composition with an outcome — Setup → play → Outcome. It is
the **courier** between `play/clock`, `play/intel`, `play/record` and
`tools/spatial`: it surveils percepts into intel, lets deciders act on their own
intel, appends the story to record, and pumps the clock.

It is also the first layer allowed to have an opinion about D&D. Rules and
trigger detection belong here. The leaves beneath it hold no rules, and the
session seam above it holds no rules either.

## What it is not

- **Not a leaf.** It composes published modules and exposes one aggregate
  persistence pair at the host seam. It is allowed dependencies the `play/*`
  modules are not.
- **Not the old `encounter` module.** The top-level `encounter/` package is the
  previous generation — `NPCAct`, `Monster.TakeTurn`, `ModeFreeRoam` /
  `ModeTurnBased`, a wired `CombatResolver`. That stack still runs the shipped
  game. This one is what new work builds on. They do not share a world model,
  and there is deliberately no adapter between them.
- **Not the host's interface.** That is `rulebooks/dnd5e/session`.

## Why it exists

Because the interesting problems in live play are *compositions* of small
guarantees, and those guarantees are much easier to keep honest when each one
lives alone. Intel cannot lie to you about whether it is lying. A clock cannot
accidentally contain a rule. A ledger cannot peek at the payload it is holding.
Somebody still has to put them together — that is this module, and keeping the
assembly in one place is what lets the pieces stay small.

## Writing a decider

**A decider is monster intelligence.** One method:

```go
type Decider interface {
    Decide(snap Snapshot) (Intent, error)
}
```

### You are given only what you know

```go
type Snapshot struct {
    Room     string          // your own current room
    Position spatial.Position // your own position within it
    Holdings []intel.Holding  // your own held intel — nothing more
}
```

That is the **anti-wall-hack contract (C2)**, and it is structural rather than a
convention: there is no field on `Snapshot` that reaches the world or another
member's live truth, so a decider cannot cheat even by accident. What it gets
instead is *belief* — percepts that may be stale, and may be false. An illusion
is ordinary testimony here. A monster chasing a player who moved away two ticks
ago is behaving correctly.

Note that the contract covers **placement**, not just sight: a decider learns
where *it* stands, never where anyone else stands except through its own
percepts.

### You return an intent, not an action

`Intent` is a sealed type — two today:

| Intent | Means |
|---|---|
| `IntentMoveTo{To}` | step to a cell on the map |
| `IntentHold{}` | do nothing this tick |

`To` is DUNGEON-ABSOLUTE, like everything else a decider is given. There is no
separate intent for crossing a doorway: a doorway's two cells are adjacent in
absolute space, so "step through the door" and "step next to me" are the same
sentence, and the composition works out which mechanism carries it.

You return what you *want*. The composition decides whether it happens and makes
it happen. This is what keeps a decider testable: it is a pure function from a
struct to a struct, and a unit test needs no encounter at all.

### A worked example

The pursuit decider from `pump_test.go`, which is about as much as a decider
ever needs to do:

```go
func (p *pursuitDecider) Decide(snap encounter.Snapshot) (encounter.Intent, error) {
    for _, h := range snap.Holdings {
        if h.Subject != intel.Subject(p.target) {
            continue
        }
        var seen encounter.SightPayload
        if err := json.Unmarshal(h.Payload, &seen); err != nil {
            return nil, err
        }
        // A percept in another room is out of this simple pursuer's reach.
        if snap.Room != seen.Room {
            return encounter.IntentHold{}, nil
        }
        // Standing exactly where the percept places them: try a door, else wait.
        if snap.Position.X == seen.X && snap.Position.Y == seen.Y {
            /* ... look for a connection at this cell ... */
            return encounter.IntentHold{}, nil
        }
        return encounter.IntentMoveTo{To: spatial.Position{X: seen.X, Y: seen.Y}}, nil
    }
    return encounter.IntentHold{}, nil // never seen them — nothing to chase
}
```

Read the last line carefully: **no percept means no pursuit.** The monster is not
told the player is absent; it simply holds nothing about them. That falls out of
the contract rather than being coded defensively.

### What `Pump` guarantees you

`Pump` advances the world one tick, and its ordering is the part worth knowing:

- **Two phases.** *Every* decider is consulted before *anything* executes. So no
  decider can observe another monster's move within the same tick — the pack does
  not get a free turn order advantage, and you never have to reason about who was
  polled first.
- **Deterministic order.** Monsters act in stable `Members()` order.
- **A decider error aborts the pump atomically.** No clock advance, no moves, no
  record beats. Return an error only when you genuinely cannot decide.
- **A rejected step does not abort.** A cell no room owns, a cell in another
  room with no doorway joining it to where you stand, or a step the spatial
  rules refuse: each just means that monster fails to act this tick. Everything
  else proceeds.
- **Sight refreshes once**, after all actions — then one tick beat, then the
  move/traverse beats in decision order.
- **A monster in a fight is not consulted.** `Pump` is the world thinking, and
  a monster caught in a turn bubble (`Form`) belongs to the fight, not the
  world — its decider is skipped entirely until the fight dissolves or the
  monster transfers out. Your decider never needs to detect "am I in combat";
  if it is being asked, it is free-roaming.

### What you cannot express yet

**There is no attack intent.** Movement, pursuit, positioning, patrol, and
traversal are all expressible today; attacking, targeting and the action economy
are wave-4 work (see `docs/ideas/session-sdk/plan.md`). Worth knowing before
choosing a first task — behavior that decides *where to be* is buildable now,
behavior that decides *what to do to someone* is not.

## Contents

| File | Holds |
|---|---|
| `encounter.go` | the aggregate: setup, verbs, `Pump`, member management |
| `decider.go` | `Decider`, `Snapshot`, `Intent` and its three implementations |
| `atlas.go` | the coordinate queries — `Atlas`, `Absolute`, `Locate` |
| `field.go` | rooms, connections, and the per-verb output shapes |
| `data.go` | `EncounterData` and the `ToData` / `LoadFromData` round trip |

## Reading it

Start with `doc.go` — it names the laws that bind this module (**C1–C8**, plus
anchoring **W1–W5**) and points at the design contract in
`docs/ideas/encounter/design.md`. Comments cite those laws by number, so `(C2)`
in the source is pointing at a specific sentence.

Then `example_tombwatch_test.go`, which narrates one member's beliefs as prose
and is the fastest way to feel what intel does.

For the layer above and below, see `play/README.md` and
`rulebooks/dnd5e/session/README.md`.

## Deliberately not here

- **Clocks, turns, initiative.** `play/clock` owns those — `Tick` for the
  world, `Turn` for a localized initiative bubble, `Transfer` between them. There
  is no `Mode` enum in this stack.
- **Storage.** The composition hands out data; the host persists it.
- **Randomness in the leaves.** Orderings and rolls arrive from the rulebook.
