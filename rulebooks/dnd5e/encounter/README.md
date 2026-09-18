# encounter

The D&D 5e **composition**: the module that turns the `play/*` leaves into a
world you can actually play in.

If you are here to write **monster behavior**, skip to
[Writing a decider](#writing-a-decider). That is the seam, and it is the whole
job.

## What it is

An encounter is a composition with an outcome — Setup → play → Outcome. It is
the **courier** between `play/clock`, `mind/perception`, `play/record` and
`tools/spatial`: it hands one perception pass the presences and the geometry,
lets deciders act on their own holdings, appends the story to record, and pumps
the clock.

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

## Writing a creature's table

**A creature's whole policy is an authored table.** There is no behaviour code
to write: `on:` in the dungeon file, keyed by what happened, with a weighted
list of entries under each key. The engine rolls it.

```yaml
on:
  time:
    - { when: { attacked: { within: 3 } }, attack: attacker, weight: 3 }
    - { when: { enemy: reach },            attack: enemy }
    - { when: { enemy: seen },             toward: enemy }
    - { when: { enemy: remembered },       toward: enemy }
    - { when: { enemy: none },             toward: { at: [3, 4] } }
    - { when: { fled: { within: 3 } },     away: actor, weight: 5 }
    - { hold: {} }
  intimidated:
    - { weight: 70, say: "Fine, fine!", fact: camp-cowed }
    - { weight: 30, say: "BOSS!", flee: {} }
```

Five triggers. The four social ones — `intimidated`, `intimidate_failed`,
`persuaded`, `persuade_failed` — fire inside the verb that settled. `time`
fires when the creature has time: its turn in a fight, or a round of the world.

### Four things load the die

1. **The rulebook's default table for the monster's kind.** Content, shipped
   beside the stat block. This is why a placement with no `on:` still fights.
2. **The author's orders**, on a faction (inherited by every placement in it)
   or on the placement itself. For each key the NEAREST layer that names it
   wins WHOLESALE — no merging, so an author never has to reason about what was
   added to what, and overriding a key visibly costs the entries under it.
3. **Temperament.** `temper: coward` on a placement, or a mix on the faction
   (`temper: { coward: 1, soldier: 2, aggressive: 1 }`) dealt per member at
   spawn through the shared dice, with a beat so the streamer sees which goblin
   came out the coward. A temperament is a weight profile and nothing else: it
   adds no entries, holds no memory and carries no trigger.
4. **What the creature has seen and suffered.** `when` reads the creature's own
   holdings — how close the nearest thing it is opposed to has got, or a deed
   done to it within N rounds. An entry whose condition is false is not on the
   table for that roll; it is ABSENT, not weighted zero.

   The four `enemy:` bands are EXCLUSIVE, and exactly one holds at any moment:
   `reach` (within this creature's own reach, the same reach an attack is
   tested against), `seen` (in sight and NOT in reach), `remembered` (none in
   sight, one held from before), `none`. Writing `attack` under `reach` and
   `toward` under `seen` is what lets a creature close and then swing —
   an attack on somebody out of reach is a pass.

### You are given only what you know

A driver receives a `MonsterView` and nothing else: its own position, actions,
table and temperament, its own sight and held location knowledge, the deeds
done to it, and the budget it has to spend. That is the **anti-wall-hack
contract (C2)**, and it is structural — there is no field on the view that
reaches the world or another member's live truth. Opposition is the one fact
the view carries that the creature could not work out alone, and it is
projected onto each sighting as `Opposed` rather than handed over as a graph.

### The roll is seen

Every pick is one die of `Σ (weight × temperament factor)`, rolled through the
shared dice, and the whole arithmetic goes on the `answered` beat: every
eligible entry with its authored weight, its factor and their product, the
total, the face, and the entry that fired. A table nobody can replay is a table
nobody can trust.

### What a round of the world guarantees you

There is no tick verb. **The world clock advances only because somebody acts**
— a walk pays a round every `SpeedFeet / 5` cells, an action pays one for its
actor, a fight round wrapping pays one per fighter — and every advance names
its own member as the driver. Standing still is free.

When an advance raises the high-water, the world thinks, inside the same call:

- **Deterministic order.** Creatures are consulted in stable clock-member
  order.
- **A table and a budget, or nothing.** A monster with no `on:` table is not
  consulted at all: holding is what an absent table means, and asking would
  cost the round a standing consult and a sight sweep to produce a world
  nobody changed.
- **One turn's worth per unit of budget**, with `AttacksLeft: 0`. A creature
  two rounds behind — one that was in a fight while the party walked — gets
  both when the fight lets it go.
- **An `Attack` off the turn clock is an error**, not a skipped intent. An
  enemy in reach on the world clock is a fight sight that has already formed.
- **A refused step does not abort the round.** A cell no region owns, a wall
  in the way: that creature simply gets nowhere this round, and everything
  else proceeds.
- **Sight refreshes once**, after every creature has moved — then the fight
  that anybody walked into forms by the ordinary path.
- **A monster in a fight is not consulted.** The world thinks on the tick and
  a fight thinks in turns; `Form` takes a fighter off the world clock, so the
  world has nothing to give it. Your driver never needs to detect "am I in
  combat".

### What decides, and how you change it

A creature's whole policy is its **table**: `on:`, in the dungeon file, keyed
by what happened — the four social verdicts, and `time` for having time. The
engine supplies one `Driver` seam and one implementation of it, `TableDriver`,
which decides nothing: it rolls the table, loaded by the creature's
temperament, filtered by what the creature has seen and suffered, and turns the
entry that fired into an intent. Changing what a monster does is editing
content, not writing Go.

### What you cannot express yet

**A creature cannot be told to go and look somewhere.** The words are
`hold`, `attack`, `toward`, `away`, `fact` and `flee`, and a selector names a
creature or an authored cell. A creature that walks to where it last saw
somebody and finds nobody there holds: "walk through that door and look" is a
word nobody has written yet. `alarm`, `lure`, `pretend` and `patrol` are
designed and refused by name.

**A temperament may not add entries, hold memory or carry a trigger.** It is a
weight profile and nothing else. The moment it grows one of those it is the
retired mind ladder again under a new name.

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
no round of the world, while staying on the map, in the roster, and recordable
against. Answer only about the members you are asked about; a name that was not
in the question is refused as a mis-wiring rather than ignored.

`Record` asks too, after writing its own beat. It is the one verb whose beat can
CHANGE who is standing, so the killing blow notices its own kill: the strike, the
body, and the `ByDefeat` ending all land in that one call, in that order. Every
other caller of the consult is a verb looking at a world something else changed.

## Contents

| File | Holds |
|---|---|
| `encounter.go` | the aggregate: setup, verbs, member management |
| `table.go` | `Table`, `Layer`, the temperament, and the one evaluator |
| `tabledriver.go` | the one `Driver`: a table rolled, a word turned into an intent |
| `worldtime.go` | what a verb costs the world clock, and the world thinking on it |
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
