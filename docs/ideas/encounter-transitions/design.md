# Encounter transitions — design (v0.2.0)

Amends the wave-1 design (`docs/ideas/encounter/design.md`). Everything
not named here carries over unchanged: laws C1–C8, the verb/query
surfaces, persistence-at-the-seam, family signature law, R8/R9
discipline.

## New laws

- **T1 — connections are open.** A connection is a bidirectional
  opening between two rooms with exactly one endpoint cell in each. No
  state, no locks, no Interact. Door state is a future wave.
- **T2 — traversal is standing at the threshold.** `Traverse` requires
  the member's position to equal the connection's endpoint in the
  member's current room; the member arrives at the far endpoint in the
  other room. Anything else is `ErrBadPlacement`.
- **T3 — sight never crosses a connection.** Percept computation
  remains room-scoped. An opening is not a window.
- **T4 — traversal is an activity, not time.** Like `Move`, `Traverse`
  never advances the clock. Only `Pump` advances the clock.
- **T5 — deciders know self, holdings, and the map; never live truth.**
  The decide phase receives the decider's own room + position and its
  holdings. Static topology may be given to a decider at construction.
  C2 stands: nothing about other members' current state ever reaches a
  decider except through its own holdings.

## Type changes

```go
// RoomInput gains a grid choice. Zero value is GridShapeSquare —
// existing setups, tests, and goldens are untouched. Hex rooms make
// the dungeon-absolute hex wire projection an identity mapping.
type RoomInput struct {
	// ... existing fields ...
	Grid spatial.GridShape // square (default) | hex | gridless
}
// In-bounds validation (member placement, connection endpoints,
// movement targets) defers to the constructed grid's own validity
// check; the module owns no rectangle math of its own.

// ConnectionInput gains endpoints (BREAKING for stored v0.1 data that
// declared connections — none exists; rpg-api is not wired yet).
type ConnectionInput struct {
	ID string
	From string                 // room ID
	FromPosition spatial.Position // endpoint cell in From
	To string                   // room ID
	ToPosition spatial.Position   // endpoint cell in To
}

// TraverseInput follows the family signature law.
type TraverseInput struct {
	Member     MemberID
	Connection string // ConnectionInput.ID
}

// TraverseOutput mirrors MoveOutput: deltas, never published; Outcome
// present iff the arrival closed the encounter.
type TraverseOutput struct { /* same shape family as MoveOutput */ }

// Snapshot is what a decider decides from. Replaces the bare holdings
// slice (interface change; zero external implementors).
type Snapshot struct {
	Room     string
	Position spatial.Position
	Holdings []intel.Holding
}

type Decider interface {
	Decide(snap Snapshot) (Intent, error)
}

// IntentTraverse joins IntentMoveTo and IntentHold.
type IntentTraverse struct {
	Connection string
}
```

## Verb surface

| Verb | Change |
|---|---|
| `Traverse(*TraverseInput) (*TraverseOutput, error)` | NEW. Precondition per T2. Refreshes percepts in **both** rooms (departure and arrival are observable absences/appearances). Appends a `traversed` beat. Runs ending evaluation at the arrival position. Rejects on `ErrClosed`, `ErrNilInput`, `ErrNoMember`, `ErrNoConnection`, `ErrBadPlacement`. |
| `Move` | Unchanged, still same-room; its doc comment stays honest about that. |
| `Pump` | Phase 2 executes `IntentTraverse` with the same T2 precondition. Failure contract (corrected during Task 3 to match the module's shipped v0.1 semantics): a phase-1 decider ERROR aborts the pump atomically (R5 — clock untouched, no beats, nothing executed); a phase-2 EXECUTION failure (illegal position, unknown connection) is silently skipped — the pump completes, and that action is simply absent from output and beats — exactly like a failed `IntentMoveTo` has always behaved. |
| `View`, `Story`, `Join`, `Exit`, `End`, `ToData`/`LoadEncounter` | Unchanged surfaces; behavior notes below. |

New sentinel: `ErrNoConnection` (unknown connection ID), wrapped
multi-`%w` like the family's other sentinels and pinned by `errors.Is`.

## Validation (Setup and Load — same rules, one-defect fixtures)

Reject when: duplicate connection ID; `From`/`To` names an unknown
room; `From == To` (self-connection); an endpoint out of the room's
bounds; an endpoint on an occluder. Load-side violations wrap
`ErrInvalidData` multi-`%w` with discriminating message fragments, in
the wave-1 `validEncounterData()` one-defect table style. Every new
field appears in the rich golden (`TestGoldenJSONRich` grows
connections with both endpoints).

## Persistence

`FieldData` already persists construction inputs; connections gain
their endpoint fields in the blob. Reload pins: a member who traversed
reloads in the **new** room (construct-path reload preserves member
room, not connection history); ordered iteration stays deterministic
(C8 — connections sorted by ID wherever order is observable).

## Decider flow (the point of the wave)

A goblin standing in the crypt saw alice slip through the north opening
last tick. Its holdings contain a ghost whose `SightPayload.Room` says
`hall`. Constructed with the map, it knows `north-door` leads there. It
decides `IntentTraverse{Connection: "north-door"}`; phase 2 executes;
its percepts refresh in the hall; the chase is on. This must exist as
an integration pin (the wave's vandalDecider analogue): decider crosses
a room boundary in pursuit of a ghost, using only self + holdings + the
map it was built with.

## Compatibility notes (honest, both acceptable at zero consumers)

1. Stored v0.1 blobs that declared connections fail v0.2 load
   (endpoints required). No such blobs exist outside this repo's own
   fixtures.
2. `Decider` implementors must adopt `Snapshot`. The only implementors
   live in this module's tests and workbench.

`gorelease` gate stays in CI; tag `rulebooks/dnd5e/encounter/v0.2.0`.

Wire-shape amendments (Copilot round on PR #924, ratified): the blob
persists grid shape as a **string** using spatial's own serialization
vocabulary (`GridTypeHex`/`GridTypeGridless`; empty = square), never
the iota integer — constant reordering must not reinterpret stored
data. Connection endpoints persist as pointers with **presence
required at load** (nil → ErrInvalidData+ErrBadConnection naming the
side) — a missing endpoint must reject, not silently become the legal
cell (0,0). This is what makes compat note 1 above genuinely true.

Coordinate-model amendment (Kirk, 2026-08-11, ratified): **hex rooms
speak axial/cube, not offset.** `buildRoomGrid` constructs spatial's
`AxialHexGrid` — Position.X/Y are axial Q/R (S = -(Q+R) derived);
bounds are origin-centered spans (negative coordinates legal), unlike
square/gridless's (0,0)-origin rectangles; distance/adjacency/LoS run
the pure cube formulas. Why: the wire and server pathing already speak
cube — axial is cube's lossless 2D projection, so the projection at
the seam is an identity, where the previously-chosen offset HexGrid
would have forced a parity-dependent conversion (the exact confusion
Platform flagged). Side effect: hex validity now diverges from
square's, so the hex tests independently kill the build-square-always
mutant (superseding the earlier "only gridless discriminates"
finding).

## Acceptance criteria

- A two-room field with one opening validates, round-trips, and
  rejects each defect class individually with the pinned fragment.
- A player at the threshold traverses; away from it, `ErrBadPlacement`;
  unknown connection, `ErrNoConnection`; after close, `ErrClosed` (the
  sweep grows `Traverse`).
- Traversal triggers ending evaluation: arriving on a
  `TriggerReachedPosition` cell in the destination room closes the
  encounter with the declared key.
- A decider-driven monster pursues a ghost through an opening
  (integration pin above); the pump stays atomic when a traverse
  intent's precondition fails.
- Percepts: departure room observers lose Current status on the
  traverser (ghost forms at the threshold); arrival room observers gain
  Current; sight does not cross the opening in either direction (T3
  pin: observer adjacent to an endpoint sees nothing in the far room).
- Scene test tells one continuous story ending in a cross-room event;
  the wave-1 tombwatch scene and its archived transcript stay
  untouched.
- The module Example gains (or a sibling Example adds) a pinned
  transcript showing a traversal beat; `./scripts/verify.sh
  rulebooks/dnd5e/encounter` prints it.
