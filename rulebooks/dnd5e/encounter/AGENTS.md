# encounter — the map, the roster, the clock everyone is on

> Contract, laws and guarantees: [`doc.go`](./doc.go). **This file is not a summary of that one.**
> It answers a different question: I am building something — does it belong here, and what may I ask?

Three packages sit on this seam. [`encounter`](.) is the composition; [`resolution`](../resolution)
is where a rule runs and the only place a bus exists ([ADR-0038](../../../docs/adr/0038-resolution-owns-the-bus.md));
[`session`](../session) is the host seam and owns no rules. Read
[`../CLAUDE.md`](../CLAUDE.md) for how those three parts are cast.

## What this package owns

Every noun below is one this module is the truth for. Nothing else may hold a second copy.

| Noun | Where | What that means |
|---|---|---|
| The floor, in one frame | [`compilefield.go`](./compilefield.go) | Every authored `[col,row]` becomes an absolute hex cell exactly once, at construction. There is no room-local frame and no bridge to one (W1–W6, [`doc.go`](./doc.go)). |
| Regions | [`region.go`](./region.go), [`field.go:199`](./field.go#L199) | A region is a named set of cells ([ADR-0044](../../../docs/adr/0044-regions-replace-rooms.md)). `RegionAt` says which one holds a cell; a member's region is DERIVED from their cell, never stored beside it. |
| The roster and where each member stands | [`Member`, field.go:1302](./field.go#L1302); [`Members()`, encounter.go:975](./encounter.go#L975) | `placementOf` ([encounter.go:1003](./encounter.go#L1003)) is the ONE projection every member read goes through, so two reads cannot disagree about a position. |
| The live map | [`canvas.go`](./canvas.go) | `Canvas()` hands out the actual `spatial.Room`, behind a view that refuses every write by name. |
| Walls, doors, props, scenery, sealed cells | [`atlas.go`](./atlas.go), [`door.go`](./door.go), [`field.go`](./field.go) | Standable is what an owner grants minus what a wall takes away. |
| What each member KNOWS about location | [`sight.go`](./sight.go), [`projection.go`](./projection.go), [`conceal.go`](./conceal.go) | `play/intel` stores testimony opaquely; this module gives it its `Known(position)`/`Unknown` meaning ([ADR-0047](../../../docs/adr/0047-encounter-owns-location-knowledge.md)). |
| The clock topology | [`clocks.go`](./clocks.go) | Every member is on exactly one clock (R6). The world tick is the default; a fight is a turn bubble. `ClockOf` ([clocks.go:81](./clocks.go#L81)) answers per member. |
| Factions and stance | [`disposition.go`](./disposition.go), [`world.go`](./world.go) | Nothing stores a stance. It is derived on every question from the declaration plus the known facts. |
| The story, and endings | [`record`-backed `Story`](./encounter.go#L1091), [`trigger.go`](./trigger.go) | Beats are audienced; an ending is an authored predicate, not a threshold inferred here. |
| The grid's real-world scale | [`units.go`](./units.go) | `FeetPerCell = 5` and `CellsFromFeet` are THE one place feet become cells, for every module above. |

## What it must never learn

- **Hit points, classes, spells, conditions, damage types, what a `Ref` means.**
  [`go.mod`](./go.mod) requires `core`, `dice`, `play/clock`, `play/intel`, `play/record`,
  `tools/spatial` and `world` — and no `rulebooks/dnd5e`. C1 forbids adding one
  ([design.md §C1](../../../docs/ideas/encounter/design.md)). Everything the rulebook knows is a
  fact this module is TOLD: `Member.Name`, `ActionView.Kind`, `AttackIdentity.DamageType` are all
  carried verbatim and never branched on.
- **Randomness.** C8. `dice` is a PARSE-ONLY dependency — the single use is
  `dice.ParseNotation` validating a persisted roll trace ([roll_trace.go:194](./roll_trace.go#L194)).
  There is no roller here and no place to add one.
- **Who is alive, and what "down" means.** Supplied as `Standing`/`Participation`
  ([standing.go:24](./standing.go#L24), [participation.go:58](./participation.go#L58)); a value that
  does not also satisfy `StandingWithParticipation` is refused with `ErrNoParticipation`. Never
  defaulted to "everyone active" ([rpg-toolkit#1033](https://github.com/KirkDiggler/rpg-toolkit/issues/1033)).
- **How far anybody sees.** Supplied as `Sight` ([sight.go:111](./sight.go#L111)), refused at
  construction with `ErrNoSight`. A number meaning "everyone sees this far" would be this module
  inventing a rule 5e does not have.
- **The bus.** No direct `events` import exists in this package. Results are returned values; a
  rule that wants to react runs in `resolution` ([ADR-0038](../../../docs/adr/0038-resolution-owns-the-bus.md)).
- **Storage.** It hands out `EncounterData` ([data.go](./data.go)); the host persists it.
- **What an archetype implies.** W5: a region's archetype and lighting are authored, required,
  carried unread, and NEVER decide a mechanic.

## Questions it answers, questions it asks

The direction is the whole boundary. **A fact the world needs but cannot compute arrives as a
supplied capability, never as a new import.** Every one of these is required at
`NewEncounter` ([encounter.go:515](./encounter.go#L515)) and none has a default:

| It asks | Interface | Refused without it |
|---|---|---|
| what order a forming fight goes in | `InitiativeRoller` ([trigger.go:25](./trigger.go#L25)) | `ErrNoInitiative` |
| who is down, who counts as present | `Standing` + `Participation` ([participation.go:66](./participation.go#L66)) | `ErrNoParticipation` |
| how far each member sees | `Sight` ([sight.go:111](./sight.go#L111)) | `ErrNoSight` |
| what an unplayed member does on its turn | `TurnDriver` ([turndriver.go:49](./turndriver.go#L49)) | `ErrNoTurnDriver` ([ADR-0043](../../../docs/adr/0043-a-monsters-turn-has-a-driver.md)) |
| how that member's swing resolves | `Striker` ([turndriver.go:473](./turndriver.go#L473)) | `ErrNoStriker` |
| who should hear about a step before it happens | `Mover` ([turndriver.go:509](./turndriver.go#L509)) | `ErrNoMover` |
| who should hear a turn or fight boundary | `Announcer` ([turndriver.go:427](./turndriver.go#L427)) | `ErrNoAnnouncer` |
| did the search find the hidden thing | `CheckResolver` ([conceal.go:81](./conceal.go#L81)) | `ErrNoCheckResolver`, when the field carries concealment |
| who perceives this open concealed door | `Witness` ([conceal.go:127](./conceal.go#L127)) | `ErrNoWitness`, same condition |

It answers, on the other hand, in geometry, placement, knowledge and clocks: `Members`,
`MembersIn`, `RegionAt`, `Region`, `Distance`, `Canvas`, `Grid`, `Atlas`/`AtlasFor`,
`Doors`/`DoorsFor`, `View`, `Story`, `ClockOf`, `Stance`/`IsHostile`/`IsAllied`, `Status`, and the
verbs that change them — `Join`, `Exit`, `Step`, `Pump`, `Transfer`, `EndTurn`, `Dissolve`,
`Search`, `OpenDoor`/`CloseDoor`/`Unlock`, `Interact`, `Loot`, `Hold`, `Record`, `End`.

`session` answering "who is standing" back down into this composition is not the charter breaking.
It is the shape every capability takes: the composition asks, the rulebook answers, and the answer
never becomes an import.

## Predicates and producers

Two kinds of question, and they are not equally safe.

A **predicate** is pointed: *is X within Y of Z*, *what is the distance between these two cells*,
*which region holds this cell*, *which clock is this member on*, *is this sightline blocked*. Its
answer depends only on its arguments. `Distance` ([encounter.go:1038](./encounter.go#L1038)),
`RegionAt`, `ClockOf`, `IsHostile`, `Canvas().IsLineOfSightBlocked` are all this shape. Ask them
freely.

A **producer** returns a set: *who is in this area*, *everyone within five feet of me*. Its answer
depends on what the set was drawn FROM, and if that is not stated the answer silently depends on
what happened to be loaded. So:

> **Every collection-returning API here names the universe it ranges over.** That is an
> observation about the surface as it stands, not a gate on the next one.

It is worth keeping because of *how* it fails when it is missing, which is invisibly: a producer
with an unstated universe returns a plausible list, and the members it quietly left out look
exactly like members that were not there. Nothing throws and no test anybody thought to write goes
red. So when you add one, the question worth asking is not "does this pass a rule" but **"what was
this drawn from, and does the signature say so?"**

The surface today obeys it. `Members()` names the roster. `MembersIn(region)` names the roster
filtered by an authored region. `View(member)` names one member's own holdings. `AtlasFor(member)`
and `DoorsFor(member)` name the field as one member knows it. `Atlas()` and `Doors()` name the
whole field.

**And that is already enough to derive a target set without a new method here.** `Members()` is a
declared universe and `Distance()` is a predicate; the caller folds them. That is not a hypothetical
— it is what `session`'s `Witness` does today, in about fifteen lines, to answer who perceives an
opening ([../session/conceal.go:209](../session/conceal.go#L209)):

```go
roster, err := enc.Members()          // the declared universe
canvas, err := enc.Canvas()           // predicates: blocked sightlines
reach, err := sight.Sight(rosterIDs(roster))
for _, member := range roster {
        if enc.Distance(member.Position, edge.From) <= float64(reach[member.ID]) &&
                !canvas.IsLineOfSightBlocked(member.Position, edge.From) { … }
}
```

`Member.Position` is dungeon-absolute axial ([field.go:1333](./field.go#L1333)) and `Distance` takes
absolute cells, so the fold needs no arithmetic of its own. **Before proposing a new
collection-returning method here, write the fold at the caller and see whether it is longer than
this.**

## Where does this go?

| You are building | Owner | Because |
|---|---|---|
| a new geometric shape — cone, line, blast | `tools/spatial` | This module holds no geometry of its own. `Distance` is `canvas.GetGrid().Distance`; the grid is where a shape belongs. |
| folding a shape into a set of targets | the caller — `resolution` for a rule, `session` for a verb | The universe is the roster, and it is declared at the call site rather than guessed inside a new method here. |
| a new fact the world needs but cannot compute | a capability interface, supplied at `NewEncounter` and refused when absent | C1 keeps rulebook facts out of this go.mod; #1033 keeps them from being defaulted in. |
| a new rule about what a fact MEANS | the rulebook — `resolution`, `conditions`, `combat` | C1. This module carries `Kind`, `Ref`, `DamageType` and never reads them. |
| a new rule about what a LOCATION fact means | here | [ADR-0047](../../../docs/adr/0047-encounter-owns-location-knowledge.md): `play/intel` is opaque; this composition is the only place sight testimony gets its meaning. |
| a new host verb — IDs in, IDs out, load-act-save | [`../session`](../session) | The seam owns no rules; if your verb needs one, the rule lands in `resolution` and the verb calls it. |
| a persistent area that outlives the action that made it | **nobody, today** | Regions are the only persistent footprint and they are authored-only (see Traps). A runtime-minted area needs a decision before it needs code. |
| a new authored field shape | [`dungeonspec/`](./dungeonspec) then [`compilefield.go`](./compilefield.go) | One conversion, at construction, in one place — W4's surviving half. |

## Traps

Things that look like they work.

1. **`MembersIn` has no production callers.** [region.go:108](./region.go#L108) is reached only from
   tests. It is the projection a persistent footprint would want, waiting for a customer that has
   not arrived. Do not cite it as precedent that "the area query already exists".
2. **Regions are authored-only.** `f.regions` and `f.regionCells` are written in exactly one place,
   `compileField` ([compilefield.go:279](./compilefield.go#L279), [:330](./compilefield.go#L330)).
   Nothing mints a region at runtime. A spell that leaves a lingering zone has no home here yet.
3. **`CellsFromFeet` floors, and nothing catches the result.**
   [units.go:33](./units.go#L33) is integer division; `validateMemberFacts`
   ([field.go:981](./field.go#L981)) refuses only a NEGATIVE `RangeFeet`. So an action authored at
   1–4 feet validates, converts to 0 cells, and can only ever reach a target on the caster's own
   cell. 5e authors reach in multiples of 5, which is why this has not bitten yet.
4. **`Canvas()` is the live map, and its range reads state no universe.** `GetEntitiesInRange` and
   `GetPositionsInRange` pass straight through ([canvas.go:197](./canvas.go#L197),
   [:209](./canvas.go#L209)). Their universe is every entity this composition placed — members and
   props — and no doc on that path says so. Prefer `Members()` + `Distance()` when you want members.
5. **`StoryInput.AfterSeq` is INCLUSIVE.** [field.go:1291](./field.go#L1291) — the name predates the
   behaviour. To resume after entry N, pass N+1; passing N replays it.
6. **`SetupInput.Standing` is typed `Standing` but that type is not sufficient.** The concrete value
   must also satisfy `Participation` or construction fails with `ErrNoParticipation`
   ([participation.go:66](./participation.go#L66)). A compiling constructor is not a working one.
7. **A verb's MUTATE phase is not atomic.** R5 promises validate-before-mutate and first-failure-wins;
   it does not promise rollback. `Join`, `Exit` and the clock verbs (`form`, `Dissolve`) can each
   fail partway, leaving a member between clocks. The
   obligation is the whole of it: **on error, drop the encounter — do not save it and do not keep
   using it** ([`doc.go`](./doc.go), "Atomicity, and what R5 does and does not promise").
