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
  `ModeTurnBased`, a wired `CombatResolver`. This one is what new work builds
  on. They do not share a world model, and there is deliberately no adapter
  between them.
- **Not a room chain.** Since rpg-project#256 a dungeon is authored as
  REGIONS — named sets of absolute `[col,row]` cells. There are no rooms,
  origins or connections; `dungeonspec` version 2 is the file format, and
  `docs/adr/0044-regions-replace-rooms.md` is the decision.
- **Not a bag of crossings.** Since rpg-project#360 a WALL IS A LINE: two
  picked positions — a hex side midpoint or a centre — and the crossings it
  blocks, the cells it passes through and how much room it leaves in them are
  all derived. The pair form (`edges`) is deleted, not deprecated. The
  geometry that derives it lives in `dungeonspec/geometry.go` and nowhere
  else; this module is handed the answers and never embeds a hex.
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
    Room     string          // your own current region
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
- **A rejected step does not abort.** A cell no region owns, a wall between you
  and it, or a step the spatial rules refuse: each just means that monster fails
  to act this tick. Everything else proceeds.
- **Sight refreshes once**, after all actions — then one tick beat, then the
  movement beats in decision order.
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

## Capabilities you must supply

Three, all **required at `NewEncounter` and `LoadEncounter`**, all refused at
construction rather than guarded later:

```go
type InitiativeRoller interface {  // what order a fight goes in
    RollInitiative(members []MemberID) ([]MemberID, error)
}

type Standing interface {          // who is down
    Standing(members []MemberID) (down []MemberID, err error)
}

type Sight interface {             // how far each member can see, in cells
    Sight(members []MemberID) (map[MemberID]int, error)
}
```

They are the same move. This module cannot import the rulebook, so randomness,
hit points and light are facts it **asks for**. None has a default: a nil
meaning "unshuffled", "everybody is fine" or "everyone sees this far" would be
the composition deciding a rule it is not allowed to know — and the last of
those three is a rule 5e does not even have, since sight is per-creature and
per-light-source.

`Sight` answers in **cells**, not feet. Cells are the only distance this module
has; "a square is five feet" is a 5e rule, so 60-foot darkvision is `12` and the
division is yours. Today an implementation may answer the same number for
everybody — what is load-bearing is that the SHAPE is per member, so the real
light model (bright, dim, dark, darkvision, blindness) lands later as a better
**answer** rather than as a new mechanism.

Two members answering differently is the point, and it means **A can see B
without B seeing A**. Geometry stays mutual — `spatial` pins line of sight as a
law — and what differs is reach. That is what makes 5e surprise producible: a
monster with darkvision spots a player whose torch does not reach it, the bubble
forms, and the player is in it unaware. This is not stealth
([#1020](https://github.com/KirkDiggler/rpg-toolkit/issues/1020)), which
contests whether an observer's percept holds a subject in plain view; it rides
the same seam and neither has to know the other exists.

`Standing` is a **pull**. Nothing pushes a death in, and nothing here remembers
one — the composition asks at the choke point where it already asks about sight,
so every route to zero is noticed without that route knowing this interface
exists. A member reported down is on no side of a contact, has no turn, and gets
no `Pump` action, while staying on the map, in the roster, and recordable
against. Answer only about the members you are asked about; a name that was not
in the question is refused as a mis-wiring rather than ignored.

`Record` asks too, after writing its own beat. It is the one verb whose beat can
CHANGE who is standing, so the killing blow notices its own kill: the strike, the
body, and the `ByDefeat` ending all land in that one call, in that order. Every
other caller of the consult is a verb looking at a world something else changed.

## Contents

| File | Holds |
|---|---|
| `encounter.go` | the aggregate: setup, verbs, `Pump`, member management |
| `decider.go` | `Decider`, `Snapshot`, `Intent` and its three implementations |
| `trigger.go` | `InitiativeRoller` and the classification that starts a fight |
| `standing.go` | `Standing`, and the world noticing who is down |
| `sight.go` | `Sight`, and how far each member can see this refresh |
| `step.go` | one step on the map, and the one place that decides what one is |
| `atlas.go` | the map reads — `Atlas` (every floor cell, region, prop, wall, doorway and wall SEGMENT, in absolute space; a cell in `Cells` and in no region is scenery, and `Sealed` is every cell nobody stands on) and `Grid` |
| `region.go` | a region is a named set of cells: `RegionAt`, `Region`, `MembersIn` |
| `compilefield.go` | the one place an authored field becomes the canvas: regions to an owner map, walls and props checked, sealed cells subtracted from standable, every `[col,row]` converted once |
| `field.go` | regions, props, walls, segments, and the per-verb output shapes |
| `projection.go` | the field as ONE member knows it: the masquerade, a presented wall's footing, and the height a mask wears |
| `data.go` | `EncounterData` and the `ToData` / `LoadFromData` round trip |

## Reading it

Start with `doc.go` — it names the laws that bind this module (**C1–C8**, plus
anchoring **W1–W6**) and points at the design contract in
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
