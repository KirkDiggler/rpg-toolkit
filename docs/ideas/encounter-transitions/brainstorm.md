# Encounter transitions — brainstorm (wave 2 of the free-roam composition)

Issue: #922. Builds on `docs/ideas/encounter/` (wave 1, shipped as
`rulebooks/dnd5e/encounter/v0.1.0`, PR #921). Ratified conversationally
with Kirk on 2026-08-10; this document records the decisions and his
words that shaped them.

## The forcing question

rpg-api#793 wires the composition into the game as a coexisting
encounter type. The wire speaks one dungeon-absolute space — a
deliberate choice: *"we left that per room because it put load on the
web to connect and build dungeons from separate rooms."* And movement
crosses rooms: *"pathing goes from room to room."* The composition's
`Move` is same-room by documented v1 constraint, and wave 1's
`ConnectionInput{ID, From, To}` is declared but inert. Multi-room free
roam blocks on giving that topology teeth.

## Decisions

### 1. The composition owns the topology (confirmed, not new)

Wave 1 already put connections in `FieldInput`; v0.2 makes them do
something. The alternative — an adapter-only `Transfer` verb where
rpg-api decides what's traversable — was rejected because deciders live
at the composition level: if the composition doesn't know rooms
connect, **a monster can never chase a player through a doorway**.
Pursuit would die at every room boundary, capping monster behavior at
single-room forever and quietly moving rules truth (what is
traversable) out of the rulebook.

### 2. Openings only — no door state

Kirk: *"our freeroam can open doors and it can actually be simplified
if that helps us. we can get there when we get there."* The live game's
door interaction is explicitly not a parity requirement. Connections in
v0.2 are always-open openings. Door state (open/closed/locked,
Interact semantics, probably interrupt windows) is a later wave.

### 3. Sight stays room-scoped

Connections do not transmit line of sight in this version. Crossing a
boundary is how you learn what's there — good dungeon-crawl texture,
and it keeps v0.2 from inheriting surprise cross-room occlusion math.
Cross-room LoS becomes its own considered feature later, not a freebie.

### 4. Traversal is an explicit verb, not Move magic

`Move` stays same-room and honest. A new `Traverse` verb crosses a
connection: the member must be standing at one endpoint; they arrive at
the other. The server already owns pathing in the absolute space, so a
cross-room path decomposes at the adapter into move → traverse → move.
Implicit auto-transition on `Move` was rejected as unfalsifiable magic:
the explicit verb gives a pinnable beat and a crisp precondition.

### 5. Deciders learn where they stand (resolving the wave-1 gap)

`Decide(view []intel.Holding)` has no self-placement — flagged at
wave-1 ratification as the View own-placement gap. It bites for real
now: a decider cannot pick a doorway without knowing which room it is
in. v0.2 resolves the gap in the direction the wave-1 design originally
promised: the decider receives a snapshot of its own room + position
alongside its holdings. Static field topology is not secret — deciders
may be constructed knowing the map; what they must never receive is
other members' live truth (C2 stands).

### 6. Intent vocabulary grows `IntentTraverse`

Pursuit through an opening is a decision like any other: decided in
phase 1, executed in phase 2, atomic under R5. A goblin that saw you
slip into the hall holds a ghost whose payload names the room — it can
choose the connection that leads there.

### 7. No dungeon-builder coupling

Kirk: dungeon builder *"spec 0.4 is not verified and 0.3 is not really
playable."* The composition's authoring surface stays `SetupInput`;
rpg-api's wave-1 dungeon keys use a minimal direct spec. Nothing here
consumes or emits the dungeon-builder pipeline.

### 8. Version and compatibility

`v0.2.0`, additive in spirit. Two honest compat notes, both acceptable
at zero external consumers (rpg-api is not wired yet): connection
validation tightens (endpoints become required), and the `Decider`
interface signature changes. Both are called out in the design.

## Shelved (future us answers better than today us)

- Door state, locks, and their Interact/interrupt semantics.
- Cross-room line of sight through openings.
- One-way or conditional connections (chutes, portcullises).
- Combat chaining at a doorway (rides #916).
