# Encounter world anchoring — brainstorm (the WHY)

**Issue:** #929. **Wave:** encounter v0.3.0.

## The itch

The v0.2.0 composition is deliberately room-local: T3 says sight never
crosses a connection, so no rule ever needs to know where a room sits
in the world. But the game's wire contract is the opposite pole —
v1alpha2 `Position` is pinned "dungeon-absolute on the wire — no
room-local coords ever," because the first free-roam delivery taught us
that per-room delivery pushes dungeon-stitching load onto the web
client. Someone has to own the bridge between those two truths, and
today nobody owns it *well*.

## What the prototype taught

rpg-api grew `internal/components/dungeon` (rpg-api#471) exactly to
learn this problem, with graduation to the toolkit planned from the
start (rpg-api#479 — "keeping them local preserves graduation
flexibility when the dungeon component eventually moves to
rpg-toolkit"). Its lessons, kept verbatim:

1. **Local and absolute are distinct spaces**, and the distinction must
   be compiler-checked, not conventional — `LocalPosition` and
   `AbsolutePosition` as separate types killed a whole substitution bug
   class (the "2D-vestige" bugs).
2. **The layout is just an origins map** — `map[roomID]AbsolutePosition`.
   Nothing fancier was ever needed.
3. **The bridge is one function** — element-wise cube addition. "Module
   is the only legal bridge; ad-hoc element-wise math at call sites is
   not allowed."
4. **What the prototype could NOT do** — validate its origins against
   the connection declarations. The component doesn't know connections
   exist; the composition doesn't know origins exist. The live tomb's
   doorways line up because a human placed them carefully. Luck-shaped.

Lesson 4 is the reason this graduates *into the encounter composition*
rather than surviving as a neighbor: the coherence laws need rooms,
origins, and connections in one aggregate, and the composition already
owns two of the three.

The axial/cube switch in v0.2 (Kirk-directed, landed pre-tag on #924)
is what makes the graduation cheap: room-local axial Q/R **is** cube
with S derived, so local→absolute is pure vector addition — the same
arithmetic the prototype already proved.

## The failure mode we're closing

A connection's endpoints are room-local cells. If the declared origins
place the rooms badly, the two endpoints project to absolute hexes that
aren't neighbors — and the client watches an entity teleport across the
map when it traverses. Nothing rejects that dungeon today. After this
wave, it cannot load.

## Alternatives considered

- **A — status quo: projection stays host-side.** Every host
  reimplements the bridge; coherence stays unvalidated forever (the
  host can't check what it can't see); rpg-api keeps doing coordinate
  math it has no authority over. Rejected — this is the prototype
  persisting past its purpose.
- **B — a standalone toolkit module (spatial atlas / play-family
  sibling).** Spatial owns grid semantics, so it's tempting. But the
  coherence check needs the connection declarations, which are
  encounter aggregate data; a standalone module either duplicates them
  (two sources of truth) or imports the composition (upside-down
  dependency). Rejected.
- **C — into the encounter composition (chosen).** Rooms gain an
  anchor; Setup/Load validate coherence with one-defect fixtures like
  every other declaration defect; queries serve absolute. One
  aggregate, laws enforced at both seams, api becomes a pure
  translator — its intended shape.

Platform's earlier pushback (single-room, square-grid) was resolved in
v0.2; their instinct that the map should be toolkit-owned was right,
and this wave delivers it — composition-shaped rather than as a
separate map module.

## Decisions

1. **Anchor as construction input.** `RoomInput.Origin` — the
   dungeon-absolute position of the room's local (0,0). Translation
   only; no rotation (authoring compensates; YAGNI).
2. **No-overlap subsumes origins-required.** Zero-value origin is legal
   (single-room fields never think about anchoring). Multi-room fields
   with defaulted origins collide under the no-overlap law and reject —
   no separate presence rule at Setup. At Load, presence IS required
   (pointer, nil rejects) per the v0.2 endpoint precedent: stored data
   missing a field is corruption, distinct from a declared zero.
3. **One geometry per field.** Absolute space must mean one thing;
   hex-cube adjacency between a hex room and a square room is
   undefined. Mixed grid families reject at declaration. (v0.2 never
   forbade mixing; nothing exercised it. Honest tightening.)
4. **Doorways kiss.** Endpoint pairs project to adjacent absolute
   cells — cube distance exactly 1 for hex, Chebyshev 1 for square,
   Euclidean in (0, 1] for gridless. This is "simple to traverse" as a
   law: a traversal is one ordinary step in world space, and the
   client never learns rooms exist.
5. **Projection is a read.** Rules, verbs, percepts, deciders stay
   room-local — T3 and C2 untouched. Absolute appears in query outputs
   only: a static `Atlas` (the map: room cells, occluders, doorway
   pairs, all absolute) plus one dynamic bridge query (the prototype's
   "only legal bridge" rule, now living where coherence is guaranteed).
6. **Compatibility is explicitly not a constraint.** Kirk: dev is held
   until we come out the other side; main stays where it is. v0.2
   blobs with rooms fail v0.3 load; only this repo's fixtures exist.

## Open questions (for design review)

- Should `View`/percept projections also carry absolute positions, or
  is Atlas + the bridge query enough? Leaning bridge-only to keep the
  v0.2 read surface stable; the host projects holdings through the one
  bridge.
- ~~Wall segments: host-derived from Atlas or Atlas-served?~~
  **RESOLVED (Kirk, 2026-08-11): neither.** Walls are explicit authored
  content in the dungeon YAML — segments with starts and ends. They
  ride the authoring lane to the wire as content the composition never
  sees in v0.3; nothing derives them from geometry. Wall↔doorway
  coherence (no authored segment contradicting a kissing pair) is the
  authoring compiler's check until walls gain blocking semantics in a
  later wave.
- Gridless adjacency bound (the "(0, 1]" above) — is coincidence at a
  shared boundary point acceptable for a gridless doorway? Leaning
  forbid (strictly positive distance) for the same reason endpoints
  can't share a cell on grids.
