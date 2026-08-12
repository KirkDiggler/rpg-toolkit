# Encounter world anchoring — design (v0.3.0)

Amends the v0.2 design (`docs/ideas/encounter-transitions/design.md`).
Everything not named here carries over unchanged: laws C1–C8 and T1–T5,
the verb surfaces, persistence-at-the-seam, the family signature law,
R8/R9 discipline, the wire-shape and coordinate-model amendments.

## New laws

- **W1 — one geometry per field.** Every room in a field declares the
  same grid family; dungeon-absolute space inherits that family. A
  field mixing families is a declaration defect. (Consequence: "the
  absolute space" is well-defined — cube for hex fields, integer
  lattice for square.) **Gridless leaves the composition in this wave**
  (Kirk, 2026-08-11): the wire cannot carry it (positions are integral
  hex cube — a contract impossibility, not a rendering gap), its one
  real contribution — shape-routing discrimination while bounded-hex
  validity ≡ square's — was superseded by the v0.2 axial switch, and it
  was the sole source of special cases in W2, W3, and Atlas.
  `spatial.GridlessRoom` lives on for other consumers; re-adding here
  is additive if a real consumer ever appears. Squares stay
  deliberately (Kirk: tempted to drop them too; kept — squares at
  least have a conversion path).
- **W2 — rooms never overlap.** The absolute projections of distinct
  rooms' cell sets are disjoint. Subsumes origins-required: a
  multi-room field whose rooms default their origins collides at (0,0)
  and rejects with no dedicated rule. (W2 is also what makes the
  reverse bridge below well-defined: every absolute cell has at most
  one owner.)
- **W3 — doorways kiss.** For every connection, the two endpoints
  project to **adjacent** absolute cells: cube distance exactly 1
  (hex), Chebyshev distance exactly 1 (square). The doorway is two
  kissing cells, one owned by each room (W2 guarantees distinct
  ownership); a traversal is one ordinary step in world space.
- **W4 — projection is a read.** Absolute coordinates appear only in
  query outputs. Rules, verbs, percept computation, endings, and
  deciders operate room-locally, exactly as in v0.2 — T3 and C2 are
  untouched. No verb accepts an absolute coordinate.
- **W5 — anchors are construction data.** Origins persist in the blob
  and are validated identically at Setup and Load (same laws, same
  one-defect fixtures, both seams). Load additionally requires
  presence (nil origin pointer → `ErrInvalidData`, the endpoint
  precedent): stored data missing an origin is corruption, distinct
  from a declared zero.

## Type changes

```go
// RoomInput gains an anchor. Zero value is legal — a single-room field
// never thinks about anchoring; multi-room fields that leave origins
// defaulted reject under W2 (collision at absolute (0,0)).
type RoomInput struct {
	// ... existing fields (ID, Width, Height, Grid, occluders, ...) ...
	Origin spatial.Position // dungeon-absolute position of local (0,0)
}
// Hex fields: Origin is axial Q/R like every hex position, integral
// (the v0.2 boundary rule extends to origins; demotes to redundant
// defense when tools/spatial#926 ships). Local→absolute is element-wise
// addition — valid cube + valid cube = valid cube, the prototype's
// arithmetic verbatim.

// Atlas is the static world map, computed from construction data only
// (rooms, occluders, connections, origins) — never from live state.
// Everything in it is dungeon-absolute. Hosts read it once per
// encounter and render; it cannot drift because nothing in it moves.
type Atlas struct {
	Rooms    []AtlasRoom    // sorted by room ID (C8)
	Doorways []AtlasDoorway // sorted by connection ID (C8)
}

type AtlasRoom struct {
	ID        string
	Grid      spatial.GridShape
	Origin    spatial.Position
	Width     int
	Height    int
	Cells     []spatial.Position // every cell, absolute — always populated
	Occluders []spatial.Position // absolute
}

type AtlasDoorway struct {
	Connection string
	From       string           // room ID
	FromCell   spatial.Position // absolute
	To         string           // room ID
	ToCell     spatial.Position // absolute
}

// AbsoluteInput/Output — the one dynamic bridge (the prototype's
// "only legal bridge" rule, relocated to where coherence is law).
// Hosts project member positions, percept holdings, and beat positions
// through this; ad-hoc element-wise math at call sites remains
// forbidden, now by convention backed by a served alternative.
type AbsoluteInput struct {
	Room     string
	Position spatial.Position // room-local
}

type AbsoluteOutput struct {
	Position spatial.Position // dungeon-absolute
}

// LocateInput/Output — the reverse bridge: which room owns an absolute
// cell, and where it sits locally. The host's inbound path needs this:
// wire requests carry dungeon-absolute targets ("no room-local coords
// ever"), while verbs speak room-local. Well-defined ONLY because of
// W2 — every absolute cell has at most one owner. Absolute positions
// no room owns (the void between rooms) reject: the bridge refuses to
// invent floor. Heir to the prototype's AbsoluteToLocal.
type LocateInput struct {
	Position spatial.Position // dungeon-absolute
}

type LocateOutput struct {
	Room     string
	Position spatial.Position // room-local
}
```

## Query surface

| Query | Change |
|---|---|
| `Atlas() (Atlas, error)` | NEW, zero-arg read (bare value + error per R3). Deterministic: rooms by ID, doorways by connection ID, cells in grid-iteration order (C8). Copy-out: returned slices never alias internal state. |
| `Absolute(*AbsoluteInput) (*AbsoluteOutput, error)` | NEW. Rejects `ErrNilInput`; unknown room (`ErrNoField` family, the room-list defect vocabulary); a position out of the room's bounds `ErrBadPlacement` — the bridge refuses to project fiction. Legal for any in-bounds position, occupied or not (hosts project percept ghosts at cells no one currently occupies). |
| `Locate(*LocateInput) (*LocateOutput, error)` | NEW — the reverse bridge, for inbound wire targets. Rejects `ErrNilInput`; an absolute position no room owns → `ErrBadPlacement` (void is not floor). Round-trip law, pinned: `Locate(Absolute(r, p)) == (r, p)` for every in-bounds cell of every room. |
| `View`, `Story`, verbs | Unchanged. The v0.2 read surface stays stable; hosts enrich to absolute through the bridge. (Bridge-only ratified by Kirk 2026-08-11; revisit only if wiring shows the same projection loop repeated across call sites.) |

No verb changes. No decider changes — a decider still sees `Snapshot`
room-locally (W4); its construction-time map may of course include
whatever topology the composition author gives it, as in v0.2.

## Validation (Setup and Load — same rules, one-defect fixtures)

Reject when, in this order per defect class:

- **Shape legality**: any room declaring gridless — room-list defect
  (the shape left the composition this wave).
- **W1**: any two rooms declare different grid families — room-list
  defect naming both rooms and both families.
- **Origin legality**: hex fields, non-integral origin (the v0.2
  integral-axial rule extended; same interim status vs spatial#926).
- **W2**: any absolute cell owned by two rooms — defect names both
  rooms and one witness cell. Touching is legal; overlap is not.
- **W3**: a connection whose endpoints project to non-adjacent absolute
  cells — wraps `ErrBadConnection`, names the connection and both
  absolute cells with their distance.

Load-side, all of the above plus **W5 presence**: nil origin →
`ErrInvalidData` naming the room. All load violations wrap
`ErrInvalidData` multi-`%w` with discriminating message fragments, in
the established one-defect table style. The rich golden grows origins
on every room — including a hex fixture with a **negative-axial
origin** (the origin-centered span proof carried to world space).

## Persistence

`RoomData` gains `Origin *PositionData` (`json:"origin"`), presence
required at load per W5. Setup-path `ToData` always writes it (a
declared zero origin persists as an explicit `{"x":0,"y":0}`, never as
absence). Grid strings: only spatial's hex vocabulary and empty
(square) remain legal — a stored `"gridless"` rejects at load
(`ErrInvalidData`; v0.2 accepted it, covered by the compat stance).
Ordered iteration unchanged (C8). Version: tag
`rulebooks/dnd5e/encounter/v0.3.0`; gorelease gate stays. Compat note,
honest as ever: v0.2 blobs containing rooms fail v0.3 load; only this
repo's fixtures exist (dev is held — Kirk-directed; no external blobs).

## The acceptance story — traversal is absolute-continuous

The payoff law, stated as a test: walk the v0.2 vault chase through the
bridge. Project every position along the pursuit — moves, the
traversal, the arrival — and the resulting absolute path is
**continuous**: every consecutive pair is at most one step apart in the
field's geometry, *including the doorway step*. The room boundary is
invisible in world space. That single pin is W2 + W3 + the bridge all
working at once, and it is exactly the property the web client relies
on to render traversal as ordinary movement.

## Acceptance criteria

- A two-room hex field with declared origins validates; each W-law
  defect class rejects individually with its pinned fragment (gridless
  declaration; W1 mixed family; W2 overlap with witness cell,
  touching-is-legal proven by a sibling fixture; W3 non-adjacent
  doorway with distance named; hex non-integral origin). Load-side
  additionally: stored `"gridless"` grid string rejects.
- `Atlas` is deterministic, copy-out immune, and matches the golden
  fixture exactly (exact-struct pin including a negative-axial hex
  room).
- `Absolute` projects in-bounds positions for both grid families;
  rejects unknown room and out-of-bounds; hex projection over a
  negative-axial origin is exact (identity-plus-offset, no parity
  anywhere).
- `Locate` inverts `Absolute` over every cell of the asymmetric
  two-room fixture (the round-trip pin); the two cells of a kissing
  doorway locate to their respective owners; void positions between
  rooms reject.
- Round-trip: origins survive ToData/Load; reload's Atlas deep-equals
  pre-reload's; every load rejection has a one-defect fixture;
  mutation-proof pins per the family standard (a pin isn't real until
  shown to FAIL under the exact breakage it guards).
- The absolute-continuity scene: the vault chase projected through the
  bridge yields a continuous world-space path across the doorway.
- Workbench and Example updated: the workbench prints the atlas (rooms
  placed in world space, doorway kissing pair visible); an Example pins
  a projected transcript; `./scripts/verify.sh rulebooks/dnd5e/encounter`
  prints it.

## Boundary with the authoring lane (triaged 2026-08-11, dungeon-builder feedback)

The dungeon-builder session enumerated nine facts the Dungeon YAML
pipeline carries and asked where each lives. None is missing from this
wave — each has exactly one home:

| Authoring fact | Home | How |
|---|---|---|
| odd-q `[col,row]` → axial conversion | **Authoring compiler** (rpg-api content lane) | The #928 durable clause: ONE documented ingress conversion, validated by the composition's load laws. Never composition scope. |
| `canvas.floor_source: regions` | **Authoring compiler + future composition wave** | Region-union authoring compiles to room cell sets. Blocked on the freeform-mask enumerator below; deliberately NOT this wave (Kirk: no dungeon-builder coupling while 0.4 is unverified). |
| Freeform floor masks and holes | **Holes: composition, TODAY** (occluder carving — a donut map is a W×H room with the middle occluded). **Full masks: future additive wave** — the W-laws, Atlas, and bridges are all defined on cell sets, so a mask room kind changes only enumeration, no law. |
| Semantic regions | **Authoring compiler — compiled away** | Regions resolve to concrete facts the composition already accepts: cell lists for endings, positions for placements. The composition never needs region identity at runtime. |
| Entrance / start | **Authoring compiler → host** | Authored entrance compiles to the positions the host passes at `Join`. The composition's Join surface is unchanged. |
| Authored placements | **Already covered** | Compile to `SetupInput` members/positions — the existing setup surface. |
| Explicit or generated envelope walls | **Authoring lane → wire, composition-invisible** (Kirk-resolved above) | Explicit segments pass through; "generated envelope" is a compiler feature that *generates* those segments. Either way the composition never sees them. |
| Immutable per-encounter snapshot of authoring facts | **rpg-api, dual record** | An encounter persists as TWO immutable records side by side: the composition blob (rules truth — rooms, origins, occluders, connections, members) and the api's compiled dungeon content record (presentation truth — walls, region names, entrance, provenance). The api stores both at StartEncounter; neither duplicates the other. |
| `reference-tomb.yaml` as acceptance fixture | **Authoring compiler's acceptance test**, when that wave runs | Wave-1 wiring uses the minimal direct spec per Kirk's #793 ruling; whether the old YAML dialect gets a compiler is the v0.4-revival decision. |

## Out of scope (unchanged from the issue)

Room rotation; door state/locks/Interact; multi-level dungeons (the
absolute plane is single-level; cube Z is a hex coordinate, not
elevation); walls (explicit authored segments with start/end in the dungeon
document — Kirk-resolved 2026-08-11: they ride the authoring lane to
the wire as content; the composition neither stores nor derives them
in v0.3, and wall↔doorway coherence is the authoring compiler's check
until walls gain blocking semantics in a later wave); the authored dungeon document (rpg-api content lane —
it compiles down to `SetupInput`).
