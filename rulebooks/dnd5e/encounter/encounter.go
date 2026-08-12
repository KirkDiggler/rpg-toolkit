// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SightPayload is the composition-owned encoding of a sighted member's position —
// the payload of every sight-channel intel report. Hosts decode it to render what
// a player sees; intel itself never interprets it.
type SightPayload struct {
	Room string  `json:"room"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// Encounter is the aggregate encounter composition: members, field, clock,
// intel, and record. Construct via NewEncounter; zero value unusable.
// declaredEnding pairs an ending key with its trigger, in Setup order.
type declaredEnding struct {
	key     string
	trigger Trigger
}

type Encounter struct {
	orchestrator *spatial.BasicRoomOrchestrator
	clock        *clock.Tick
	intelLog     *intel.Intel
	story        *record.Log
	members      map[MemberID]*Member
	everMembers  map[MemberID]bool // Track all members who have ever joined (for Story access)
	deciders     map[MemberID]Decider
	// endings holds declared endings in Setup order. Evaluation is
	// deterministic (law C8), but NOT globally "first-declared-wins":
	// for a single action (Move, Traverse, Join) declaration order is
	// the only axis, so the first matching declared ending does win.
	// Pump can execute several monsters' actions in one tick, and there
	// evaluation walks them in DECISION order first — the action
	// decided earliest wins regardless of which of its matching endings
	// was declared later; declaration order is only the tiebreak within
	// one action's own scan. See Pump's ending-evaluation loop.
	endings []declaredEnding
	outcome *Outcome
	// fieldInput and connectionsInput are stored at Setup time for persistence.
	// LoadEncounter uses them to rebuild the field deterministically without
	// re-running surveil or requiring an event bus.
	fieldInput       []RoomInput
	connectionsInput []ConnectionInput
}

// deepCopyRoomInputs snapshots the caller's room descriptions — the
// persistence source must never alias caller-owned slices (T6 review
// M4: a caller editing its SetupInput after construction silently
// corrupted ToData and made the encounter unsavable).
func deepCopyRoomInputs(rooms []RoomInput) []RoomInput {
	out := make([]RoomInput, len(rooms))
	for i, r := range rooms {
		rc := r
		rc.Occluders = append([]spatial.Position(nil), r.Occluders...)
		rc.Boundaries = append([]spatial.Boundary(nil), r.Boundaries...)
		out[i] = rc
	}
	return out
}

// buildRoomGrid constructs the spatial Grid for a room's declared shape.
// From Setup (buildValidRoomGrids), only GridShapeSquare and GridShapeHex
// ever reach here — shape legality rejects gridless and any unrecognized
// value before this is called, so the GridShapeGridless case and the
// switch's default (square, for an unrecognized value) are both
// unreachable from that path. Neither is dead code overall, though:
// LoadEncounter (data.go) still calls this directly and still routes a
// stored "gridless" grid string through the GridShapeGridless case — T2
// removes that branch once Load-side rejection of gridless lands alongside
// Origin persistence. Until then this stays a three-shape switch, one
// path reachable only from Load.
//
// Hex rooms build spatial.AxialHexGrid, NOT spatial.HexGrid: the wire (and
// Platform's pathing) speaks cube coordinates natively, and axial (Q, R,
// with S = -(Q+R) derived) is cube's 2D projection — an IDENTITY mapping to
// the wire. HexGrid's bounded offset column/row coordinates would force a
// lossy, orientation-dependent offset<->cube conversion at that seam for no
// benefit inside the composition, which never renders a grid itself.
// width/height become AxialHexGridConfig's SpanWidth/SpanHeight — see
// RoomInput.Grid's doc comment for what that means for a hex room's
// Position values (origin-centered, negative coordinates legal).
func buildRoomGrid(shape spatial.GridShape, width, height int) spatial.Grid {
	switch shape {
	case spatial.GridShapeHex:
		return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: float64(width), SpanHeight: float64(height)})
	case spatial.GridShapeGridless:
		return spatial.NewGridlessRoom(spatial.GridlessConfig{Width: float64(width), Height: float64(height)})
	default:
		return spatial.NewSquareGrid(spatial.SquareGridConfig{Width: float64(width), Height: float64(height)})
	}
}

// isIntegralAxialPosition reports whether pos satisfies spatial's
// implicit integer-cube contract for hex rooms — upholds it at this
// composition's boundary until tools/spatial#926 enforces it at ingress;
// becomes redundant defense once the fixed spatial tag is consumed.
// AxialHexGrid bounds-checks Position.X/Y but does not integrality-check
// them, and all of its cube math truncates, so a fractional axial
// position like (0.5, 0.5) would otherwise persist as a distinct
// position that behaves exactly like (0,0) — an invisible collision with
// an unrelated, legitimately-placed cell. Applies ONLY to hex rooms:
// square stays fractional-tolerant as today, and gridless is continuous
// by design — call this next to every grid-deferred IsValidPosition
// check (or, where no such check exists at a seam, next to where the
// position first enters) so every externally supplied hex-room position
// is covered: member/Move/Join positions, connection endpoints, and
// occluders.
func isIntegralAxialPosition(grid spatial.Grid, pos spatial.Position) bool {
	if grid.GetShape() != spatial.GridShapeHex {
		return true
	}
	return pos.X == math.Trunc(pos.X) && pos.Y == math.Trunc(pos.Y)
}

// isIntegralPosition reports whether pos has integral X and Y, with no
// grid-shape exception — the universal origin-legality check (#929 T1
// Opus round finding): unlike isIntegralAxialPosition, a fractional
// SQUARE origin is also a defect, not just a fractional hex one. W2's
// disjointness promise (RoomInput.Origin's doc comment) is only sound
// over an INTEGER cell lattice: two 5x5 square rooms anchored at (0,0)
// and (0.5,0.5) have disjoint integer cell sets (W2 as enumerated would
// accept them) while their continuous footprints interpenetrate roughly
// 81% of each room's area, and a Chebyshev-0 "doorway" (a connection
// whose two endpoints land on literally the same fractional point) would
// still measure as adjacent. Every room's Origin must land on a whole
// coordinate, for every family, before W2 ever runs.
func isIntegralPosition(pos spatial.Position) bool {
	return pos.X == math.Trunc(pos.X) && pos.Y == math.Trunc(pos.Y)
}

// buildValidRoomGrids rejects room defects before construction (R5
// atomicity — no observable state until Setup succeeds), in this order per
// defect class (docs/ideas/encounter-anchoring/design.md's Validation
// section, Opus-round amendment): empty or duplicate room ID; an
// unrecognized or no-longer-supported grid shape; non-integral hex
// occluder positions; W1 (one grid family per field — validateGridFamilies);
// non-positive Width/Height (room legality — a negative dimension used to
// panic NewEncounter via a negative-capacity make() in the since-deleted
// enumeration path; a panic is not a rejection); non-integral Origin, for
// EVERY family now, not just hex (origin legality — a fractional origin
// defeats W2's disjointness promise for ANY grid, not only hex: see
// isIntegralPosition); and W2 (rooms never overlap in absolute space —
// validateRoomsDisjoint). On success, returns each room's constructed Grid
// keyed by room ID — reused both for downstream bounds checks (connections,
// and transitively member placement via spatial's own PlaceEntity) and
// later in NewEncounter's room-construction loop, so a room's shape is
// built exactly once and every consumer asks the SAME grid. Reuses
// ErrNoField — a malformed room list is as unusable as an empty one.
func buildValidRoomGrids(rooms []RoomInput) (map[string]spatial.Grid, error) {
	seenIDs := make(map[string]bool, len(rooms))
	grids := make(map[string]spatial.Grid, len(rooms))
	for _, r := range rooms {
		if r.ID == "" {
			return nil, fmt.Errorf("newencounter: room has empty id: %w", ErrNoField)
		}
		if seenIDs[r.ID] {
			return nil, fmt.Errorf("newencounter: duplicate room %q: %w", r.ID, ErrNoField)
		}
		seenIDs[r.ID] = true

		// Shape legality: gridless leaves the composition as of v0.3 — the
		// wire cannot carry a continuous room's absolute projection — so it
		// is rejected explicitly, distinct from a genuinely unrecognized
		// value. Square and hex are the only surviving families; W1 below
		// compares them. This branch is unreachable from Setup as of this
		// wave (every room here has already survived it), but LoadEncounter
		// (data.go) still routes a stored "gridless" grid string through
		// buildRoomGrid's own gridless case — Load-side rejection of it is
		// T2's job, alongside persisting Origin.
		switch r.Grid {
		case spatial.GridShapeSquare, spatial.GridShapeHex:
		case spatial.GridShapeGridless:
			return nil, fmt.Errorf("newencounter: room %q declares gridless grid shape, no longer supported: %w", r.ID, ErrNoField)
		default:
			return nil, fmt.Errorf("newencounter: room %q has unknown grid shape %d: %w", r.ID, r.Grid, ErrNoField)
		}
		grids[r.ID] = buildRoomGrid(r.Grid, r.Width, r.Height)

		// Hex rooms require integral axial occluder positions (interim
		// tools/spatial#926 enforcement — see isIntegralAxialPosition).
		for _, occ := range r.Occluders {
			if !isIntegralAxialPosition(grids[r.ID], occ) {
				return nil, fmt.Errorf("newencounter: room %q occluder (%g,%g) is not an integral axial cell: %w", r.ID, occ.X, occ.Y, ErrNoField)
			}
		}
	}

	// W1 — one geometry per field: every room in this field must share the
	// same grid family. Runs only after every room has individually passed
	// shape legality above, so only square/hex remain here.
	if err := validateGridFamilies(rooms); err != nil {
		return nil, err
	}

	// Room legality: non-positive Width or Height is a defect, not a
	// silent no-op (#929 T1 Opus round: Width:-1 previously reached
	// buildRoomGrid and, downstream, a negative-capacity make() in the
	// enumeration W2 used before this same round replaced it with interval
	// math — a panic, not a rejection, either way an unvalidated dimension
	// has no business reaching construction). Runs after W1 (a mixed-family
	// field with a bad dimension reports the family mismatch first) and
	// before origin legality/W2 (both need real dimensions to reason about).
	for _, r := range rooms {
		if r.Width <= 0 || r.Height <= 0 {
			return nil, fmt.Errorf("newencounter: room %q has non-positive dimensions (%d x %d): %w", r.ID, r.Width, r.Height, ErrNoField)
		}
	}

	// Origin legality: every room's Origin must be integral, for EVERY
	// grid family (#929 T1 Opus round: originally hex-only, extended
	// here — see isIntegralPosition for why a fractional SQUARE origin is
	// equally a defect). Runs after W1 and room legality so a mixed-family
	// or malformed-dimension field reports THAT defect first, not an
	// origin defect on a room whose shape or size is already wrong.
	for _, r := range rooms {
		if !isIntegralPosition(r.Origin) {
			return nil, fmt.Errorf("newencounter: room %q origin (%g,%g) is not an integral cell: %w", r.ID, r.Origin.X, r.Origin.Y, ErrNoField)
		}
	}

	// W2 — rooms never overlap: distinct rooms' absolute cell sets (local
	// cell + Origin) must be disjoint. Zero-value origins are legal on
	// their own; a multi-room field that leaves every Origin defaulted
	// collides every room at (0,0) and is rejected HERE — there is no
	// separate "origin required" check (RoomInput.Origin's doc comment).
	if err := validateRoomsDisjoint(rooms); err != nil {
		return nil, err
	}

	return grids, nil
}

// gridShapeName renders a GridShape for W1's mixed-family defect message.
// Called only on values that have already passed shape legality above, so
// square and hex are the only cases exercised in practice; the default
// exists so an unvalidated caller still gets a readable message instead of
// a bare integer.
func gridShapeName(shape spatial.GridShape) string {
	switch shape {
	case spatial.GridShapeSquare:
		return "square"
	case spatial.GridShapeHex:
		return "hex"
	default:
		return fmt.Sprintf("shape(%d)", shape)
	}
}

// validateGridFamilies rejects a field whose rooms declare more than one
// grid family (W1 — one geometry per field). Local→absolute is element-wise
// addition (RoomInput.Origin's doc comment): a hex room's axial Q/R and a
// square room's Cartesian X/Y cannot be added together and mean anything, so
// a mixed field has no coherent absolute space at all — this must reject
// before W2/W3 ever try to build one. Compares ALL pairs semantically, not
// just adjacent-in-slice: a three-room field ordered square, hex, square
// must still be caught even though both adjacent pairs (0,1) and (1,2)
// already mismatch on their own — see #929 T1 mutation notes for why an
// adjacent-only comparison cannot be distinguished from this by any fixture
// once only two families survive shape legality (equality over a two-value
// domain is transitive), and why full pairwise comparison is still the
// correct, future-proof implementation.
func validateGridFamilies(rooms []RoomInput) error {
	for i := 0; i < len(rooms); i++ {
		for j := i + 1; j < len(rooms); j++ {
			if rooms[i].Grid != rooms[j].Grid {
				return fmt.Errorf("newencounter: room %q (%s) and room %q (%s) declare different grid families: %w",
					rooms[i].ID, gridShapeName(rooms[i].Grid), rooms[j].ID, gridShapeName(rooms[j].Grid), ErrNoField)
			}
		}
	}
	return nil
}

// axisBounds returns a room's LOCAL integer [min,max] cell bounds along one
// axis, for a dimension `dim` (Width for Q/X, Height for R/Y) — W2's
// building block (see roomAbsoluteBounds), replacing an earlier
// enumeration-based approach (#929 T1 Opus round: enumerating a 1000x1000
// room's ~1M cells to check overlap measured 1.35s/205MB per NewEncounter
// call, and a negative dimension's negative-capacity make() there is what
// used to panic — see buildValidRoomGrids' room-legality check). Square's
// bound is the one-sided [0,dim) rectangle SquareGrid.IsValidPosition
// itself checks, i.e. integer max = dim-1. Hex's bound is AxialHexGrid's
// origin-centered half-open span (tools/spatial/hex_grid.go,
// AxialHexGridConfig's doc comment) reduced to its integer min/max: half =
// dim/2; min = ceil(-half); max = ceil(half)-1. Both formulas are the SAME
// half-width/half-height arithmetic AxialHexGrid.IsValidPosition itself
// uses, not guessed, and both hold for dim odd or even (verified against
// roomLocalCells' enumeration, since deleted, before this replaced it —
// e.g. dim=3: half=1.5, min=-1, max=1, three integers; dim=4: half=2.0,
// min=-2, max=1, four integers).
func axisBounds(shape spatial.GridShape, dim int) (min, max int) {
	if shape == spatial.GridShapeHex {
		half := float64(dim) / 2
		return int(math.Ceil(-half)), int(math.Ceil(half)) - 1
	}
	return 0, dim - 1
}

// roomAbsoluteBounds returns a room's absolute-space bounding box: its
// local integer cell bounds (axisBounds), offset by Origin, per axis.
// Requires Origin to already be integral (buildValidRoomGrids' origin
// legality runs before W2) — the int() truncation here is exact, not
// lossy, for every Origin this is ever called with.
func roomAbsoluteBounds(r RoomInput) (qMin, qMax, rMin, rMax int) {
	localQMin, localQMax := axisBounds(r.Grid, r.Width)
	localRMin, localRMax := axisBounds(r.Grid, r.Height)
	oq, or := int(r.Origin.X), int(r.Origin.Y)
	return localQMin + oq, localQMax + oq, localRMin + or, localRMax + or
}

// validateRoomsDisjoint rejects a field whose rooms' absolute footprints
// share a cell (W2 — rooms never overlap): absolute = local cell + Origin,
// element-wise (RoomInput.Origin's doc comment). Touching — adjacent cells,
// no shared cell — is legal; this rejects ONLY a shared cell. Both grid
// families are solid rectangles in their OWN coordinate system (a hex
// room's Q,R span is a rhombus embedded in 2D axial space, but each axis
// is independently an interval, exactly like square's X,Y), so two rooms'
// footprints overlap iff BOTH axes' intervals intersect — no enumeration:
// O(1) per room pair, not O(cells). The witness cell, when rejecting, is
// the component-wise max of the two rooms' interval mins — the
// lexicographically-first cell both footprints necessarily contain,
// deterministic regardless of iteration order.
func validateRoomsDisjoint(rooms []RoomInput) error {
	type bounds struct{ qMin, qMax, rMin, rMax int }
	bs := make([]bounds, len(rooms))
	for i, r := range rooms {
		bs[i].qMin, bs[i].qMax, bs[i].rMin, bs[i].rMax = roomAbsoluteBounds(r)
	}
	for i := 0; i < len(rooms); i++ {
		for j := i + 1; j < len(rooms); j++ {
			qOverlap := bs[i].qMin <= bs[j].qMax && bs[j].qMin <= bs[i].qMax
			rOverlap := bs[i].rMin <= bs[j].rMax && bs[j].rMin <= bs[i].rMax
			if qOverlap && rOverlap {
				witness := spatial.Position{
					X: float64(max(bs[i].qMin, bs[j].qMin)),
					Y: float64(max(bs[i].rMin, bs[j].rMin)),
				}
				return fmt.Errorf("newencounter: room %q and room %q overlap at absolute cell %s: %w",
					rooms[i].ID, rooms[j].ID, witness, ErrNoField)
			}
		}
	}
	return nil
}

// validateConnectionInputs rejects connection defects before construction
// (R5 atomicity — no observable state until Setup succeeds): empty or
// duplicate ID, an unknown or self-referencing room, an endpoint outside
// its room's bounds (per that room's own constructed Grid, from roomGrids —
// see buildValidRoomGrids) or on an occluder position, and (W3) endpoints
// that do not kiss — are not adjacent absolute cells once each is anchored
// to its own room's Origin.
func validateConnectionInputs(rooms []RoomInput, roomGrids map[string]spatial.Grid, connections []ConnectionInput) error {
	roomsByID := make(map[string]RoomInput, len(rooms))
	for _, r := range rooms {
		roomsByID[r.ID] = r
	}

	seenIDs := make(map[string]bool, len(connections))
	for _, c := range connections {
		if c.ID == "" {
			return fmt.Errorf("newencounter: connection has empty id: %w", ErrBadConnection)
		}
		if seenIDs[c.ID] {
			return fmt.Errorf("newencounter: duplicate connection %q: %w", c.ID, ErrBadConnection)
		}
		seenIDs[c.ID] = true

		fromRoom, ok := roomsByID[c.From]
		if !ok {
			return fmt.Errorf("newencounter: connection %q references unknown room %q: %w", c.ID, c.From, ErrBadConnection)
		}
		toRoom, ok := roomsByID[c.To]
		if !ok {
			return fmt.Errorf("newencounter: connection %q references unknown room %q: %w", c.ID, c.To, ErrBadConnection)
		}
		if c.From == c.To {
			return fmt.Errorf("newencounter: connection %q connects room %q to itself: %w", c.ID, c.From, ErrBadConnection)
		}

		if !roomGrids[c.From].IsValidPosition(c.FromPosition) {
			return fmt.Errorf("newencounter: connection %q from-position out of bounds: %w", c.ID, ErrBadConnection)
		}
		if !isIntegralAxialPosition(roomGrids[c.From], c.FromPosition) {
			return fmt.Errorf("newencounter: connection %q from-position is not an integral axial cell: %w", c.ID, ErrBadConnection)
		}
		if !roomGrids[c.To].IsValidPosition(c.ToPosition) {
			return fmt.Errorf("newencounter: connection %q to-position out of bounds: %w", c.ID, ErrBadConnection)
		}
		if !isIntegralAxialPosition(roomGrids[c.To], c.ToPosition) {
			return fmt.Errorf("newencounter: connection %q to-position is not an integral axial cell: %w", c.ID, ErrBadConnection)
		}

		for _, occ := range fromRoom.Occluders {
			if occ.X == c.FromPosition.X && occ.Y == c.FromPosition.Y {
				return fmt.Errorf("newencounter: connection %q from-position on occluder: %w", c.ID, ErrBadConnection)
			}
		}
		for _, occ := range toRoom.Occluders {
			if occ.X == c.ToPosition.X && occ.Y == c.ToPosition.Y {
				return fmt.Errorf("newencounter: connection %q to-position on occluder: %w", c.ID, ErrBadConnection)
			}
		}

		// W3 — doorways kiss: once each endpoint is anchored to its own
		// room's Origin, the two absolute cells must be adjacent — cube
		// distance 1 for hex, Chebyshev distance 1 for square. W1
		// guarantees both rooms in a valid field share one grid family, so
		// either room's own Grid.Distance already implements the correct
		// formula (AxialHexGrid: cube distance; SquareGrid: Chebyshev) —
		// using the from-room's is an arbitrary but consistent choice, not
		// a hand-rolled formula that could silently diverge from spatial's.
		fromAbs := c.FromPosition.Add(fromRoom.Origin)
		toAbs := c.ToPosition.Add(toRoom.Origin)
		// Strict != 1, not > 1: distance 0 (coincident endpoints) is
		// unreachable once origin legality requires integral origins for
		// every family and W2 requires disjoint room footprints (a shared
		// or coincident absolute cell is exactly what W2 already rejects
		// before this ever runs) — kept strict anyway as defense-in-depth,
		// deliberately UNPINNED by a dedicated test, since no fixture can
		// falsify it (#929 T1 Opus round).
		if dist := roomGrids[c.From].Distance(fromAbs, toAbs); dist != 1 {
			return fmt.Errorf("newencounter: connection %q endpoints %s and %s are not adjacent (distance %g): %w",
				c.ID, fromAbs, toAbs, dist, ErrBadConnection)
		}
	}
	return nil
}

// NewEncounter constructs and initializes an encounter from SetupInput.
// Validation order (first failure wins, R5 atomicity): nil input, no rooms,
// no endings, reserved ending key, empty member ID, duplicate member IDs,
// room defects (empty/duplicate ID, unrecognized or no-longer-supported
// grid shape, W1 mixed grid families, non-integral hex origin, W2
// overlapping absolute footprints), connection defects (empty/duplicate ID,
// unknown room, self-connection, endpoint out of bounds or on an occluder,
// W3 endpoints not adjacent once anchored), spatial placement errors.
func NewEncounter(in *SetupInput) (*Encounter, error) {
	// Validation order: nil, no rooms, no endings, reserved ending, empty ID, duplicates
	if in == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNilInput)
	}

	if len(in.Field.Rooms) == 0 {
		return nil, fmt.Errorf("newencounter: %w", ErrNoField)
	}

	if len(in.Endings) == 0 {
		return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
	}

	// Check ending keys
	for _, ending := range in.Endings {
		if ending.Key == "" || ending.Key == "abandoned" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
		}
	}

	// Check member IDs: empty or duplicate; validate deciders
	seenIDs := make(map[MemberID]bool)
	for _, m := range in.Members {
		if m.ID == "" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("newencounter: duplicate member %s: %w", m.ID, ErrNoMember)
		}
		seenIDs[m.ID] = true

		// Players cannot carry deciders (design law C2)
		if m.Kind == KindPlayer && m.Decider != nil {
			return nil, fmt.Errorf("newencounter: player %s cannot carry a decider: %w", m.ID, ErrNoMember)
		}
	}

	// Check rooms: unique non-empty IDs, recognized grid shape. roomGrids
	// holds each room's constructed Grid, reused below for connection
	// bounds validation and again in the room-construction loop so a
	// room's shape is built exactly once.
	roomGrids, err := buildValidRoomGrids(in.Field.Rooms)
	if err != nil {
		return nil, err
	}

	// Check connections: unique non-empty IDs, endpoints resolve to distinct
	// declared rooms, endpoints in bounds (per the room's own grid) and off
	// any occluder.
	if err = validateConnectionInputs(in.Field.Rooms, roomGrids, in.Field.Connections); err != nil {
		return nil, err
	}

	// Hex rooms require integral axial member positions (interim
	// tools/spatial#926 enforcement — see isIntegralAxialPosition). No
	// existing bounds pre-check covers members at this seam (placement
	// bounds are enforced by spatial's own PlaceEntity below), so this
	// runs as its own pass over the grid this member's declared room
	// resolved to — a member whose room doesn't exist is caught later,
	// at placement, unrelated to this check.
	for _, mi := range in.Members {
		if grid, ok := roomGrids[mi.Room]; ok && !isIntegralAxialPosition(grid, mi.Position) {
			return nil, fmt.Errorf("newencounter: member %q position is not an integral axial cell: %w", mi.ID, ErrBadPlacement)
		}
	}

	// Connections are stored sorted by ID (C8 determinism — order is
	// observable in ToData).
	connectionsInput := append([]ConnectionInput(nil), in.Field.Connections...)
	sort.Slice(connectionsInput, func(i, j int) bool { return connectionsInput[i].ID < connectionsInput[j].ID })

	// After validation passes, construct (R5: no observable state until success)
	e := &Encounter{
		members:          make(map[MemberID]*Member),
		everMembers:      make(map[MemberID]bool),
		deciders:         make(map[MemberID]Decider),
		endings:          nil,
		fieldInput:       deepCopyRoomInputs(in.Field.Rooms),
		connectionsInput: connectionsInput,
	}

	// Build clock and intel
	e.clock, err = clock.NewTick()
	if err != nil {
		return nil, fmt.Errorf("newencounter clock: %w", err)
	}

	e.intelLog, err = intel.NewIntel()
	if err != nil {
		return nil, fmt.Errorf("newencounter intel: %w", err)
	}

	e.story, err = record.NewLog()
	if err != nil {
		return nil, fmt.Errorf("newencounter story: %w", err)
	}

	// Build field: orchestrator and rooms
	e.orchestrator = spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "encounter-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})

	// Create all rooms, reusing each room's already-constructed Grid
	// (roomGrids, built above) so validation and placement agree exactly.
	roomMap := make(map[string]*spatial.BasicRoom)
	for _, ri := range in.Field.Rooms {
		room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   ri.ID,
			Type: "room",
			Grid: roomGrids[ri.ID],
		})
		err = e.orchestrator.AddRoom(room)
		if err != nil {
			return nil, fmt.Errorf("newencounter add room: %w", err)
		}
		roomMap[ri.ID] = room

		// Add occluders as blocking entities
		for _, pos := range ri.Occluders {
			occluder := &occluderEntity{id: fmt.Sprintf("occluder-%s-%d-%d", ri.ID, int(pos.X), int(pos.Y))}
			_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
				RoomID:   spatial.RoomID(ri.ID),
				Entity:   occluder,
				Position: pos,
			})
			if err != nil {
				return nil, fmt.Errorf("newencounter occluder placement: %w: %w", ErrBadPlacement, err)
			}
		}

		// Add boundaries
		for _, b := range ri.Boundaries {
			boundaryRoom := roomMap[ri.ID]
			if boundaryRoom != nil {
				if br, ok := interface{}(boundaryRoom).(spatial.BoundaryAwareRoom); ok {
					err = br.RegisterBoundary(b)
					if err != nil {
						return nil, fmt.Errorf("newencounter boundary: %w: %w", ErrBadPlacement, err)
					}
				}
			}
		}
	}

	// Add connections
	for _, ci := range in.Field.Connections {
		door := spatial.CreateDoorConnection(ci.ID, ci.From, ci.To, 1.0)
		err = e.orchestrator.AddConnection(door)
		if err != nil {
			return nil, fmt.Errorf("newencounter add connection: %w", err)
		}
	}

	// Place members and collect them
	memberIDs := make([]MemberID, 0, len(in.Members))
	for _, mi := range in.Members {
		memberIDs = append(memberIDs, mi.ID)

		entity := &memberEntity{
			id:   string(mi.ID),
			kind: mi.Kind,
		}

		_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
			RoomID:   spatial.RoomID(mi.Room),
			Entity:   entity,
			Position: mi.Position,
		})
		if err != nil {
			return nil, fmt.Errorf("newencounter member placement: %w: %w", ErrBadPlacement, err)
		}

		member := &Member{
			ID:   mi.ID,
			Kind: mi.Kind,
			Room: mi.Room,
		}
		e.members[mi.ID] = member
		e.everMembers[mi.ID] = true // Track in everMembers

		// Store decider if present (monsters only, validated above)
		if mi.Decider != nil {
			e.deciders[mi.ID] = mi.Decider
		}
	}

	// Store endings in declaration order (deterministic evaluation, C8)
	for _, ei := range in.Endings {
		e.endings = append(e.endings, declaredEnding{key: ei.Key, trigger: ei.Trigger})
	}

	// First light: build sight percepts for each member using refreshSight
	_, err = e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("newencounter first light: %w", err)
	}

	// Opening record beat: all members hear "scene-opened"
	beatPayload, _ := json.Marshal(map[string]string{"beat": "scene-opened"})
	_, err = e.story.Append(&record.AppendInput{
		At:       0,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatPayload,
	})
	if err != nil {
		return nil, fmt.Errorf("newencounter append beat: %w", err)
	}

	return e, nil
}

// View returns the member's current intel holdings.
// Returns ErrNotMember if the member is not part of this encounter.
func (e *Encounter) View(in *ViewInput) ([]intel.Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("view: %w", ErrNilInput)
	}

	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("view: %w", ErrNotMember)
	}

	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}

	return holdings, nil
}

// Members returns the current member roster in stable order.

// buildMemberOutcomes snapshots every current member's placement in
// sorted-ID order — deterministic output for outcomes and persistence
// (map iteration here was a latent nondeterminism, T6 review M1).
func (e *Encounter) buildMemberOutcomes() []MemberOutcome {
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	outcomes := make([]MemberOutcome, 0, len(ids))
	for _, id := range ids {
		m := e.members[id]
		mRoom, ok := e.orchestrator.GetRoom(m.Room)
		if !ok {
			continue
		}
		mPos, ok := mRoom.GetEntityPosition(string(m.ID))
		if !ok {
			continue
		}
		outcomes = append(outcomes, MemberOutcome{ID: m.ID, Room: m.Room, Position: mPos})
	}
	return outcomes
}

func (e *Encounter) Members() ([]Member, error) {
	// Sort by ID for stability
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	members := make([]Member, 0, len(ids))
	for _, id := range ids {
		members = append(members, *e.members[id])
	}
	return members, nil
}

// Status returns the encounter's current state (Open or Closed with Outcome).
// Returns a deep copy of the outcome to prevent aliasing (MUTATION-PROOF).
func (e *Encounter) Status() (*Status, error) {
	if e.outcome != nil {
		// Deep-copy outcome and its Members slice to prevent aliasing
		// (mutation-proof: modifying returned outcome does not affect internal state)
		members := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			members[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		return &Status{
			Open: false,
			Outcome: &Outcome{
				Ending:  e.outcome.Ending,
				At:      e.outcome.At,
				Members: members,
			},
		}, nil
	}
	return &Status{Open: true}, nil
}

// Story returns the story entries for a member after the given sequence number.
// Allows both current members and members who have exited (everMembers).
// Returns ErrNilInput if the input is nil, ErrNoMember if the member never joined.
// Copy-out follows record's own conventions (returned entries are already copies
// per record's implementation).
func (e *Encounter) Story(in *StoryInput) ([]record.Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}

	if _, ok := e.everMembers[in.Audience]; !ok {
		return nil, fmt.Errorf("story: %w", ErrNoMember)
	}

	entries, err := e.story.SliceFor(&record.SliceForInput{
		Viewer:  in.Audience,
		FromSeq: in.AfterSeq,
	})
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}

	return entries, nil
}

// moveMember executes a spatial move for a member and returns the old position if successful,
// or an error if the spatial move was rejected. This is the shared managed seam for both
// player moves (Move verb) and monster moves (Pump). The member must exist and be in an
// open encounter; spatial rejection does not abort the operation (handled by caller).
func (e *Encounter) moveMember(member *Member, to spatial.Position) (spatial.Position, error) {
	// Get the room and current position
	room, ok := e.orchestrator.GetRoom(member.Room)
	if !ok {
		return spatial.Position{}, fmt.Errorf("movemember: %w", ErrBadPlacement)
	}

	currentPos, ok := room.GetEntityPosition(string(member.ID))
	if !ok {
		return spatial.Position{}, fmt.Errorf("movemember: %w", ErrBadPlacement)
	}

	// Hex rooms require integral axial targets (interim tools/spatial#926
	// enforcement — see isIntegralAxialPosition). This is the SHARED path
	// for both the Move verb and Pump's IntentMoveTo execution: a
	// fractional target from either source is rejected here, and Pump's
	// existing silent-skip contract for a rejected move applies exactly
	// as it does for an out-of-bounds one — no special case needed there.
	if !isIntegralAxialPosition(room.GetGrid(), to) {
		return spatial.Position{}, fmt.Errorf("movemember: target is not an integral axial cell: %w", ErrBadPlacement)
	}

	// Attempt the spatial move via managed seam using MoveEntity
	_, err := e.orchestrator.MoveEntity(&spatial.MoveEntityInput{
		RoomID:   spatial.RoomID(member.Room),
		EntityID: core.EntityID(member.ID),
		To:       to,
	})
	if err != nil {
		return spatial.Position{}, fmt.Errorf("movemember: %w: %w", ErrBadPlacement, err)
	}

	return currentPos, nil
}

// Move executes a continuous movement within the same room for ANY
// member — players move themselves; Pump routes monster intents through
// the same path. Unfiltered ReachedPosition endings fire only for
// player members; member-filtered endings fire only for the named
// member.
// Validation order (R5 atomicity): nil input → empty member → closed →
// not a member → spatial move rejection. On success, refreshes sight for all members,
// records beat, and evaluates ReachedPosition endings.
func (e *Encounter) Move(in *MoveInput) (*MoveOutput, error) {
	// Validation order
	if in == nil {
		return nil, fmt.Errorf("move: %w", ErrNilInput)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("move: %w", ErrNoMember)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("move: %w", ErrClosed)
	}

	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("move: %w", ErrNotMember)
	}

	// Execute the move through the managed seam
	currentPos, err := e.moveMember(member, in.To)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	// Get all member IDs for the refresh scope (v1: refresh everyone)
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// Refresh sight for all members
	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("move refresh sight: %w", err)
	}

	// Record the movement beat
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":     "moved",
		"member":   string(in.Member),
		"position": in.To,
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "movement"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("move append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// Evaluate ReachedPosition endings
	var firedOutcome *Outcome
	for _, de := range e.endings {
		endingKey, trigger := de.key, de.trigger
		reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}

		// Check if the ending fires
		if reachedPosTrigger.Room != member.Room {
			continue // Different room
		}

		if reachedPosTrigger.Position.X != in.To.X || reachedPosTrigger.Position.Y != in.To.Y {
			continue // Different position
		}

		// Check member filter: empty = any player member; non-empty = specific member
		if reachedPosTrigger.Member != "" && reachedPosTrigger.Member != in.Member {
			continue // Member filter doesn't match
		}

		// Member filter passes (empty or matches) but we need to check kind
		if reachedPosTrigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only, but moved member is not player
		}

		// Ending fires! Build the outcome with all members' current positions
		memberOutcomes := e.buildMemberOutcomes()

		e.outcome = &Outcome{
			Ending:  endingKey,
			At:      clockReadingForBeat,
			Members: memberOutcomes,
		}
		// Return a deep copy of the outcome (mutation-proof)
		outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			outcomeMembers[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		firedOutcome = &Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		}
		break
	}

	return &MoveOutput{
		Moved: struct {
			Member MemberID
			From   spatial.Position
			To     spatial.Position
		}{
			Member: in.Member,
			From:   currentPos,
			To:     in.To,
		},
		IntelDeltas: intelDeltas,
		Seq:         seqNum,
		Outcome:     firedOutcome,
	}, nil
}

// traverseResult holds the outcome of a successful cross-room move via
// traverseMember — shared by the Traverse verb and Pump's IntentTraverse
// executor.
type traverseResult struct {
	fromRoom string
	fromPos  spatial.Position
	toRoom   string
	toPos    spatial.Position
}

// traverseMember resolves connectionID against this encounter's declared
// connections, determines direction from member's CURRENT room/position
// (they must be standing exactly on one of the connection's two
// endpoints — connections are bidirectional, T1 law), and moves the
// entity between rooms via spatial's TransitionEntity + PlaceEntity
// managed seams, mutating member.Room on success. ANY failure (unknown
// connection, endpoint mismatch, or a spatial rejection) returns before
// member.Room is touched.
//
// Shared by the Traverse verb (which owns the nil/closed/member-lookup
// guards and propagates this function's error as its own public
// contract — ErrNoConnection or ErrBadPlacement, already correctly
// wrapped here) and Pump's phase-2 IntentTraverse executor (which owns
// Pump's silent-skip-on-failure semantics: ANY error returned here is
// treated exactly like a spatially-rejected IntentMoveTo — the monster
// simply fails to act this tick, no abort).
func (e *Encounter) traverseMember(member *Member, connectionID string) (traverseResult, error) {
	var conn *ConnectionInput
	for i := range e.connectionsInput {
		if e.connectionsInput[i].ID == connectionID {
			conn = &e.connectionsInput[i]
			break
		}
	}
	if conn == nil {
		return traverseResult{}, fmt.Errorf("connection %s not found: %w", connectionID, ErrNoConnection)
	}

	room, ok := e.orchestrator.GetRoom(member.Room)
	if !ok {
		return traverseResult{}, fmt.Errorf("%w", ErrBadPlacement)
	}
	fromPos, ok := room.GetEntityPosition(string(member.ID))
	if !ok {
		return traverseResult{}, fmt.Errorf("%w", ErrBadPlacement)
	}
	fromRoom := member.Room

	// The member must be standing exactly on one of the connection's two
	// endpoints. Determine direction from WHICH endpoint they're on — the
	// arrival side is always the connection's OTHER endpoint (never the
	// one the member is currently standing on).
	var toRoom string
	var toPos spatial.Position
	switch {
	case fromRoom == conn.From && fromPos.X == conn.FromPosition.X && fromPos.Y == conn.FromPosition.Y:
		toRoom, toPos = conn.To, conn.ToPosition
	case fromRoom == conn.To && fromPos.X == conn.ToPosition.X && fromPos.Y == conn.ToPosition.Y:
		toRoom, toPos = conn.From, conn.FromPosition
	default:
		return traverseResult{}, fmt.Errorf("member %s is not at connection %s's endpoint: %w", member.ID, connectionID, ErrBadPlacement)
	}

	// Move the entity between rooms via spatial's purpose-built transition
	// seam: TransitionEntity removes from the source room (the SAME
	// registry cleanup RemoveEntity performs — the wave-1 Exit lesson
	// applies here too) and, as a backstop behind our own checks above,
	// independently re-validates against spatial's own bookkeeping that
	// the connection exists, actually links these two rooms (in either
	// direction, since door connections are Reversible), and the entity
	// was truly indexed in the departure room. TransitionEntity
	// deliberately does NOT place the entity — PlaceEntity must always
	// follow, using the SAME entity value TransitionEntity returned.
	transitioned, err := e.orchestrator.TransitionEntity(&spatial.TransitionEntityInput{
		EntityID:     core.EntityID(member.ID),
		FromRoom:     spatial.RoomID(fromRoom),
		ToRoom:       spatial.RoomID(toRoom),
		ConnectionID: spatial.ConnectionID(connectionID),
	})
	if err != nil {
		return traverseResult{}, fmt.Errorf("transition entity: %w: %w", ErrBadPlacement, err)
	}

	// LIMBO WINDOW (defensive, currently unreachable): if PlaceEntity below
	// were to fail after TransitionEntity above already succeeded, the
	// entity would be removed from every room and placed in none — this
	// function does not roll back. Four invariants keep that window
	// unreachable today: (1) rooms are immutable after Setup/Load, so
	// fromRoom/toRoom always exist; (2) member entities never block
	// placement (memberEntity.BlocksMovement() is always false), so
	// occupancy can never reject PlaceEntity; (3) connection endpoints are
	// grid-validated at both entry seams (Setup and Load), so toPos is
	// always valid on the arrival room's grid; (4) door connections are
	// permanently Passable and Reversible (CreateDoorConnection, never
	// mutated after Setup/Load), so TransitionEntity's own passability and
	// direction checks always agree with ours. A future change that
	// violates any one of these must meet this comment as a deliberate
	// decision point, not a silent atomicity regression.
	_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID:   spatial.RoomID(toRoom),
		Entity:   transitioned.Entity,
		Position: toPos,
	})
	if err != nil {
		return traverseResult{}, fmt.Errorf("place entity: %w: %w", ErrBadPlacement, err)
	}

	member.Room = toRoom

	return traverseResult{fromRoom: fromRoom, fromPos: fromPos, toRoom: toRoom, toPos: toPos}, nil
}

// Traverse moves a member across a connection from their current endpoint to
// the connection's OTHER endpoint, in the other room. Validation order (R5
// atomicity): nil input → closed → not a member → traverseMember's own
// checks (connection not found: ErrNoConnection; endpoint mismatch — wrong
// room or wrong position, either rejects the same way: ErrBadPlacement).
//
// The clock is NOT advanced — traversal is an activity, not time (law T4).
// Sight refreshes for ALL members in one refreshSight call: since
// refreshSight scopes each observer's percept to members currently in
// THEIR OWN room, updating member.Room before the refresh (done inside
// traverseMember) is sufficient — departure-room observers naturally stop
// seeing the traverser (intel's existing complete-percept contract ghosts
// them at their last-known position, the departure endpoint) and
// arrival-room observers naturally gain them Current. No per-room
// special-casing needed, and sight cannot cross the opening in either
// direction: rooms are separate spatial containers with no shared
// geometry (spatial ADR-0015), so there is no code path by which an
// observer's line-of-sight computation — scoped entirely to entities in
// their own room — could see into the other room.
//
// Ending evaluation mirrors Move exactly, evaluated against the ARRIVAL
// room/position: unfiltered ReachedPosition triggers fire for player
// members only; member-filtered triggers fire for the named member
// regardless of kind.
func (e *Encounter) Traverse(in *TraverseInput) (*TraverseOutput, error) {
	// Validation order
	if in == nil {
		return nil, fmt.Errorf("traverse: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("traverse: %w", ErrClosed)
	}

	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("traverse: %w", ErrNotMember)
	}

	result, err := e.traverseMember(member, in.Connection)
	if err != nil {
		return nil, fmt.Errorf("traverse: %w", err)
	}

	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("traverse refresh sight: %w", err)
	}

	// Record the traversal beat. The clock is NOT advanced (law T4).
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":       "traversed",
		"member":     string(in.Member),
		"connection": in.Connection,
		"room":       result.toRoom,
		"position":   result.toPos,
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "movement"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("traverse append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// Evaluate ReachedPosition endings against the ARRIVAL room/position.
	var firedOutcome *Outcome
	for _, de := range e.endings {
		endingKey, trigger := de.key, de.trigger
		reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}

		if reachedPosTrigger.Room != result.toRoom {
			continue // Different room
		}

		if reachedPosTrigger.Position.X != result.toPos.X || reachedPosTrigger.Position.Y != result.toPos.Y {
			continue // Different position
		}

		// Check member filter: empty = any player member; non-empty = specific member
		if reachedPosTrigger.Member != "" && reachedPosTrigger.Member != in.Member {
			continue // Member filter doesn't match
		}

		// Member filter passes (empty or matches) but we need to check kind
		if reachedPosTrigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only, but traversing member is not player
		}

		// Ending fires! Build the outcome with all members' current positions
		memberOutcomes := e.buildMemberOutcomes()

		e.outcome = &Outcome{
			Ending:  endingKey,
			At:      clockReadingForBeat,
			Members: memberOutcomes,
		}
		// Return a deep copy of the outcome (mutation-proof)
		outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			outcomeMembers[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		firedOutcome = &Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		}
		break
	}

	return &TraverseOutput{
		Traversed: struct {
			Member   MemberID
			FromRoom string
			From     spatial.Position
			ToRoom   string
			To       spatial.Position
		}{
			Member:   in.Member,
			FromRoom: result.fromRoom,
			From:     result.fromPos,
			ToRoom:   result.toRoom,
			To:       result.toPos,
		},
		IntelDeltas: intelDeltas,
		Seq:         seqNum,
		Outcome:     firedOutcome,
	}, nil
}

// Pump advances the world by one tick: the exploration clock advances,
// each monster member (in deterministic order) acts on its own intel via Decider,
// the complete sight refresh happens once, and the story accrues tick and
// move/traverse beats. Errors from a decider abort the pump atomically (R5):
// no clock advance, no moves, no record entries.
//
// Semantics:
//   - Tick advances by exactly 1 (via clock.Advance with displacement 1).
//   - Monsters act in deterministic order (stable Members() order, filtered to KindMonster).
//   - Each decider receives exactly its own Snapshot: own room, own position,
//     own holdings (anti-wall-hack contract C2 — placement included, not just sight).
//   - IntentHold means do nothing; IntentMoveTo executes via the same-room managed
//     seam; IntentTraverse executes via traverseMember (the Traverse verb's own
//     mechanics, shared — see its doc comment).
//   - A spatial rejection of a monster's move, or an illegal traverse intent
//     (unknown connection, or not at the threshold), does NOT abort the pump; the
//     monster simply fails to act. Only a decider error aborts.
//   - After all monster actions: ONE refreshSight for all members, ONE tick beat
//     (stamped with the new clock reading), then move/traverse beats in
//     decision order (the same order monsters were consulted in).
//   - Ending evaluation fires ReachedPosition triggers (only if the filter matches;
//     empty filter = players only, not monsters) against each action's resulting
//     room/position, in the same decision order.
//   - Returns PumpOutput with the new Tick reading, successful moves, successful
//     traverses, deltas, and beats.
func (e *Encounter) Pump(in *PumpInput) (*PumpOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("pump: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("pump: %w", ErrClosed)
	}

	// PHASE 1 — decide. Every decider is consulted BEFORE anything
	// mutates: a decider error aborts here with zero state touched
	// (R5 — no clock advance, no moves, no beats). This also means a
	// later monster's decider error cannot leave an earlier monster's
	// move half-applied.
	allMembers, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("pump members: %w", err)
	}

	type plannedAction struct {
		memberID MemberID
		intent   Intent
	}
	var planned []plannedAction

	for _, m := range allMembers {
		if m.Kind != KindMonster {
			continue
		}
		decider, hasDecider := e.deciders[m.ID]
		if !hasDecider {
			continue // no decider = hold
		}

		// The monster's own placement, read fresh from spatial — never
		// another member's. A decider that received anyone else's room or
		// position would be a wall hack extended to placement, not just
		// sight (C2).
		room, ok := e.orchestrator.GetRoom(m.Room)
		if !ok {
			return nil, fmt.Errorf("pump snapshot room: %w", ErrBadPlacement)
		}
		ownPos, ok := room.GetEntityPosition(string(m.ID))
		if !ok {
			return nil, fmt.Errorf("pump snapshot position: %w", ErrBadPlacement)
		}

		// The monster's own holdings and nothing else (C2). HeldBy's
		// copy-out is intel's documented contract (pinned in play/intel)
		// — no redundant defensive copy here; the mutating-decider
		// integration test pins the composed guarantee.
		ownHoldings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: m.ID})
		if err != nil {
			return nil, fmt.Errorf("pump held_by: %w", err)
		}

		intent, err := decider.Decide(Snapshot{Room: m.Room, Position: ownPos, Holdings: ownHoldings})
		if err != nil {
			return nil, fmt.Errorf("pump decide: %w", err)
		}

		switch intent.(type) {
		case IntentMoveTo, IntentTraverse:
			planned = append(planned, plannedAction{memberID: m.ID, intent: intent})
		}
	}

	// PHASE 2 — execute. Nothing below returns a decider-shaped error;
	// the world now advances.
	_, err = e.clock.Advance(&clock.AdvanceInput{
		Driver:       core.EntityID("world"),
		Displacement: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("pump advance: %w", err)
	}

	newTickReading := uint64(e.clock.ToData().HighWater)

	// executedAction is built in PLANNED (decision) order regardless of
	// kind, so beats and ending evaluation below stay in the same
	// deterministic per-monster order the deciders were consulted in
	// (C8) — not "all moves then all traverses".
	type executedAction struct {
		member     *Member
		kind       string // "move" or "traverse"
		connection string // only meaningful for "traverse"
		fromRoom   string // only meaningful for "traverse"; a move never changes room
		from       spatial.Position
		toRoom     string // only meaningful for "traverse"
		to         spatial.Position
	}
	var executed []executedAction

	for _, p := range planned {
		// The REAL member pointer, not a Members()-derived copy: a
		// traverse mutates member.Room, and that mutation must reach the
		// live e.members entry, not a value copy that planned would
		// otherwise have carried since Setup/Wave-1 (moves never needed
		// to write back Room, so this distinction was previously latent).
		member := e.members[p.memberID]
		if member == nil {
			// A contract-violating decider removed itself from the
			// encounter (e.g. called Exit on its own member) during
			// phase 1's Decide. Its planned action has no live member
			// to execute against — same silent-skip contract as a
			// spatially-rejected move: absent from output and beats,
			// the pump otherwise proceeds normally.
			continue
		}
		switch intent := p.intent.(type) {
		case IntentMoveTo:
			// Spatial rejection does not abort the pump; the monster
			// simply fails to move and no beat is recorded for it.
			fromPos, moveErr := e.moveMember(member, intent.To)
			if moveErr == nil {
				executed = append(executed, executedAction{
					member: member, kind: "move", from: fromPos, to: intent.To,
				})
			}
		case IntentTraverse:
			// An illegal traverse (unknown connection, or not at the
			// threshold) does not abort the pump either — same silent-skip
			// contract as a spatially-rejected move (see traverseMember's
			// doc comment).
			result, travErr := e.traverseMember(member, intent.Connection)
			if travErr == nil {
				executed = append(executed, executedAction{
					member: member, kind: "traverse", connection: intent.Connection,
					fromRoom: result.fromRoom, from: result.fromPos,
					toRoom: result.toRoom, to: result.toPos,
				})
			}
		}
	}

	// Single refreshSight for all members after all monster actions
	memberIDs := make([]MemberID, 0, len(allMembers))
	for _, m := range allMembers {
		memberIDs = append(memberIDs, m.ID)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("pump refresh sight: %w", err)
	}

	// Record the tick beat first (the frame)
	tickBeatPayload := map[string]interface{}{
		"beat": "tick",
		"tick": newTickReading,
	}
	tickBeatBytes, _ := json.Marshal(tickBeatPayload)

	tickAppendOut, err := e.story.Append(&record.AppendInput{
		At:       newTickReading,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "clock"},
		Payload:  tickBeatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("pump append tick beat: %w", err)
	}

	seqs := []uint64{tickAppendOut.Seq}

	// Then record a beat for each successful action, in decision order.
	for _, action := range executed {
		var beatPayload map[string]interface{}
		switch action.kind {
		case "move":
			beatPayload = map[string]interface{}{
				"beat":     "moved",
				"member":   string(action.member.ID),
				"position": action.to,
			}
		case "traverse":
			beatPayload = map[string]interface{}{
				"beat":       "traversed",
				"member":     string(action.member.ID),
				"connection": action.connection,
				"room":       action.toRoom,
				"position":   action.to,
			}
		}
		beatBytes, _ := json.Marshal(beatPayload)

		actionAppendOut, err := e.story.Append(&record.AppendInput{
			At:       newTickReading,
			Audience: memberIDs,
			Tags:     map[string]string{"tag": "movement"},
			Payload:  beatBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("pump append %s beat: %w", action.kind, err)
		}

		seqs = append(seqs, actionAppendOut.Seq)
	}

	// Evaluate ReachedPosition endings, in decision order. For a
	// traverse, action.member.Room is already the ARRIVAL room
	// (traverseMember mutated it); for a move it's unchanged — either
	// way action.member.Room is correct to check against.
	var firedOutcome *Outcome
	for _, action := range executed {
		for _, de := range e.endings {
			endingKey, trigger := de.key, de.trigger
			reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
			if !ok {
				continue // Not a ReachedPosition trigger
			}

			if reachedPosTrigger.Room != action.member.Room {
				continue // Different room
			}

			if reachedPosTrigger.Position.X != action.to.X || reachedPosTrigger.Position.Y != action.to.Y {
				continue // Different position
			}

			// Check member filter: non-empty = specific member; empty = players only
			// Monsters should never trigger an unfiltered (empty-filter) ending
			if reachedPosTrigger.Member == "" {
				continue // Empty filter means players only, but this member is a monster
			}

			if reachedPosTrigger.Member != action.member.ID {
				continue // Member filter doesn't match
			}

			// Ending fires! Build the outcome with all members' current positions
			memberOutcomes := e.buildMemberOutcomes()

			e.outcome = &Outcome{
				Ending:  endingKey,
				At:      newTickReading,
				Members: memberOutcomes,
			}
			// Return a deep copy of the outcome (mutation-proof)
			outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
			for i, m := range e.outcome.Members {
				outcomeMembers[i] = MemberOutcome{
					ID:       m.ID,
					Room:     m.Room,
					Position: m.Position,
				}
			}
			firedOutcome = &Outcome{
				Ending:  e.outcome.Ending,
				At:      e.outcome.At,
				Members: outcomeMembers,
			}
			break
		}
		if firedOutcome != nil {
			break
		}
	}

	// Build the output, splitting executed actions by kind (each output
	// slice preserves the SAME relative order it had in executed).
	var outputMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}
	var outputTraverses []struct {
		Member   MemberID
		FromRoom string
		From     spatial.Position
		ToRoom   string
		To       spatial.Position
	}
	for _, action := range executed {
		switch action.kind {
		case "move":
			outputMoves = append(outputMoves, struct {
				Member MemberID
				From   spatial.Position
				To     spatial.Position
			}{Member: action.member.ID, From: action.from, To: action.to})
		case "traverse":
			outputTraverses = append(outputTraverses, struct {
				Member   MemberID
				FromRoom string
				From     spatial.Position
				ToRoom   string
				To       spatial.Position
			}{Member: action.member.ID, FromRoom: action.fromRoom, From: action.from, ToRoom: action.toRoom, To: action.to})
		}
	}

	return &PumpOutput{
		Tick:             newTickReading,
		MonsterMoves:     outputMoves,
		MonsterTraverses: outputTraverses,
		IntelDeltas:      intelDeltas,
		Seqs:             seqs,
		Outcome:          firedOutcome,
	}, nil
}

// refreshSight rebuilds the complete percept for all given observers,
// surveils each, and returns a map of member IDs to their SurveilOutput deltas.
// This is shared between Setup's first-light and Move's percept refresh.
// The current clock reading is stamped on each Surveil call.
func (e *Encounter) refreshSight(observers []MemberID) (map[MemberID]*intel.SurveilOutput, error) {
	// Get current clock reading
	clockReadingInt := e.clock.ToData().HighWater
	clockReading := uint64(clockReadingInt)
	deltas := make(map[MemberID]*intel.SurveilOutput)

	for _, observerID := range observers {
		member, ok := e.members[observerID]
		if !ok {
			continue // Skip if not found
		}

		// Get the room
		room, ok := e.orchestrator.GetRoom(member.Room)
		if !ok {
			continue // Room not found
		}

		// Get observer's position
		observerPos, ok := room.GetEntityPosition(string(observerID))
		if !ok {
			continue // Observer position not found
		}

		// List all OTHER members in the same room and check line of sight
		var percept []intel.Report
		for _, otherMember := range e.members {
			if otherMember.ID == observerID {
				continue // Skip self
			}
			if otherMember.Room != member.Room {
				continue // Different room
			}

			// Get the other member's position
			otherPos, ok := room.GetEntityPosition(string(otherMember.ID))
			if !ok {
				continue // Position not found
			}

			// Check line of sight
			if room.IsLineOfSightBlocked(observerPos, otherPos) {
				continue // Blocked by obstacle
			}

			// Add to percept
			pos := SightPayload{
				Room: otherMember.Room,
				X:    otherPos.X,
				Y:    otherPos.Y,
			}
			payload, _ := json.Marshal(pos)
			percept = append(percept, intel.Report{
				Subject: intel.Subject(otherMember.ID),
				Payload: payload,
			})
		}

		// Surveil with the complete percept and current clock reading
		out, err := e.intelLog.Surveil(&intel.SurveilInput{
			Observer: observerID,
			Channel:  intel.Sight,
			Percept:  percept,
			At:       clockReading,
		})
		if err != nil {
			return nil, fmt.Errorf("refreshsight surveil: %w", err)
		}
		deltas[observerID] = out
	}

	return deltas, nil
}

// occluderEntity is an internal entity for blocking line of sight
type occluderEntity struct {
	id string
}

// GetID returns the occluder's ID
func (o *occluderEntity) GetID() string {
	return o.id
}

// GetType returns "occluder"
func (o *occluderEntity) GetType() core.EntityType {
	return core.EntityType("occluder")
}

// GetSize returns 1 (single-cell entity)
func (o *occluderEntity) GetSize() int {
	return 1
}

// BlocksLineOfSight returns true for occluders
func (o *occluderEntity) BlocksLineOfSight() bool {
	return true
}

// BlocksMovement returns false for occluders
func (o *occluderEntity) BlocksMovement() bool {
	return false
}

// Join adds a new member to the encounter. The ambient field is always there
// to join. Validation order (R5 atomicity): nil input → closed → empty member ID →
// already a member → player-with-decider rejected → spatial placement rejection.
// On success, the joiner is placed, all members' sight is refreshed (the joiner's
// first percepts AND incumbents now seeing them), and a beat is recorded.
// ReachedPosition endings are evaluated (a player could join ON the stairs — fires YES).
func (e *Encounter) Join(in *JoinInput) (*JoinOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("join: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("join: %w", ErrClosed)
	}

	if in.Member.ID == "" {
		return nil, fmt.Errorf("join: %w", ErrNoMember)
	}

	// Check if already a member
	if _, exists := e.members[in.Member.ID]; exists {
		return nil, fmt.Errorf("join: member %s is already in the encounter: %w", in.Member.ID, ErrNoMember)
	}

	// Players cannot carry deciders (design law C2)
	if in.Member.Kind == KindPlayer && in.Member.Decider != nil {
		return nil, fmt.Errorf("join: player %s cannot carry a decider: %w", in.Member.ID, ErrNoMember)
	}

	// Hex rooms require integral axial join positions (interim
	// tools/spatial#926 enforcement — see isIntegralAxialPosition). A
	// nonexistent room is left to PlaceEntity's own failure below,
	// unrelated to this check.
	if room, ok := e.orchestrator.GetRoom(in.Member.Room); ok && !isIntegralAxialPosition(room.GetGrid(), in.Member.Position) {
		return nil, fmt.Errorf("join: position is not an integral axial cell: %w", ErrBadPlacement)
	}

	// Place the new member via managed seam
	entity := &memberEntity{
		id:   string(in.Member.ID),
		kind: in.Member.Kind,
	}

	_, err := e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID:   spatial.RoomID(in.Member.Room),
		Entity:   entity,
		Position: in.Member.Position,
	})
	if err != nil {
		return nil, fmt.Errorf("join placement: %w: %w", ErrBadPlacement, err)
	}

	// Register the member
	member := &Member{
		ID:   in.Member.ID,
		Kind: in.Member.Kind,
		Room: in.Member.Room,
	}
	e.members[in.Member.ID] = member
	e.everMembers[in.Member.ID] = true // Track in everMembers

	// Store decider if present (monsters only, validated above)
	if in.Member.Decider != nil {
		e.deciders[in.Member.ID] = in.Member.Decider
	}

	// refreshSight for all members: the joiner sees incumbents, incumbents see the joiner
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("join refresh sight: %w", err)
	}

	// Record the join beat (audience = all members including the joiner)
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":   "joined",
		"member": string(in.Member.ID),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("join append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// Evaluate ReachedPosition endings (a player could join ON the stairs)
	var firedOutcome *Outcome
	for _, de := range e.endings {
		endingKey, trigger := de.key, de.trigger
		reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}

		// Check if the ending fires
		if reachedPosTrigger.Room != member.Room {
			continue // Different room
		}

		if reachedPosTrigger.Position.X != in.Member.Position.X || reachedPosTrigger.Position.Y != in.Member.Position.Y {
			continue // Different position
		}

		// Check member filter: empty = any player member; non-empty = specific member
		if reachedPosTrigger.Member != "" && reachedPosTrigger.Member != in.Member.ID {
			continue // Member filter doesn't match
		}

		// Member filter passes but check kind
		if reachedPosTrigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only
		}

		// Ending fires! Build the outcome with all members' current positions
		memberOutcomes := e.buildMemberOutcomes()

		e.outcome = &Outcome{
			Ending:  endingKey,
			At:      clockReadingForBeat,
			Members: memberOutcomes,
		}
		// Return a deep copy of the outcome (mutation-proof)
		outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			outcomeMembers[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		firedOutcome = &Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		}
		break
	}

	return &JoinOutput{
		Member:      *member,
		IntelDeltas: intelDeltas,
		Seq:         seqNum,
		Outcome:     firedOutcome,
	}, nil
}

// Exit removes a member from the encounter. The member leaves with carry-forward:
// they are removed from the field, their final MemberOutcome is returned, and their
// intel holdings are copied out for the campaign. The encounter auto-closes with the
// reserved ending "abandoned" if the last member exits and no declared ending has fired.
// After exit, remaining members' views fade the departed naturally (their entity left,
// so next refreshSight removes them from new percepts; old holdings ghost).
func (e *Encounter) Exit(in *ExitInput) (*ExitOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("exit: %w", ErrNilInput)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("exit: %w", ErrNoMember)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("exit: %w", ErrClosed)
	}

	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrNotMember)
	}

	// Get the exiting member's final position
	room, ok := e.orchestrator.GetRoom(member.Room)
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}

	finalPos, ok := room.GetEntityPosition(string(in.Member))
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}

	// Capture the exiting member's holdings (carry-forward)
	carry, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("exit held_by: %w", err)
	}

	// Remove from the field via managed seam
	_, err = e.orchestrator.RemoveEntity(&spatial.RemoveEntityInput{
		RoomID:   spatial.RoomID(member.Room),
		EntityID: core.EntityID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("exit remove entity: %w: %w", ErrBadPlacement, err)
	}

	// Remove from member set (and deciders if present)
	delete(e.members, in.Member)
	delete(e.deciders, in.Member)

	// Get remaining member IDs for story and refresh
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// The exit beat's audience is every member INCLUDING the exiter —
	// they witness their own departure (and can re-read it via Story).
	allMemberIDs := make([]MemberID, 0, len(e.members)+1)
	allMemberIDs = append(allMemberIDs, in.Member) // Add the exiter
	for id := range e.members {
		allMemberIDs = append(allMemberIDs, id)
	}
	sort.Slice(allMemberIDs, func(i, j int) bool { return allMemberIDs[i] < allMemberIDs[j] })

	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":   "exited",
		"member": string(in.Member),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: allMemberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("exit append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// refreshSight for REMAINING members only (the exiter's holdings remain in intel archive)
	if len(memberIDs) > 0 {
		_, err := e.refreshSight(memberIDs)
		if err != nil {
			return nil, fmt.Errorf("exit refresh sight: %w", err)
		}
	}

	// Check if we need to auto-close (last member exited and no ending has fired)
	var closedOutcome *Outcome
	if len(e.members) == 0 && e.outcome == nil {
		e.outcome = &Outcome{
			Ending:  "abandoned",
			At:      clockReadingForBeat,
			Members: []MemberOutcome{}, // No members remain
		}
		closedOutcome = &Outcome{
			Ending:  "abandoned",
			At:      clockReadingForBeat,
			Members: []MemberOutcome{},
		}
	}

	return &ExitOutput{
		Outcome: MemberOutcome{
			ID:       in.Member,
			Room:     member.Room,
			Position: finalPos,
		},
		Carry:  carry,
		Seq:    seqNum,
		Closed: closedOutcome,
	}, nil
}

// End fires an externally-triggered ending. Validates the key was declared and
// has an External trigger, then closes the encounter with that Outcome.
func (e *Encounter) End(in *EndInput) (*EndOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("end: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("end: %w", ErrClosed)
	}

	if in.Ending == "" {
		return nil, fmt.Errorf("end: %w", ErrNoEnding)
	}

	// Find and validate the ending
	var foundEnding *declaredEnding
	for i := range e.endings {
		if e.endings[i].key == in.Ending {
			foundEnding = &e.endings[i]
			break
		}
	}

	if foundEnding == nil {
		return nil, fmt.Errorf("end: ending %s not declared: %w", in.Ending, ErrNoEnding)
	}

	// Verify it's an External trigger
	_, isExternal := foundEnding.trigger.(TriggerExternal)
	if !isExternal {
		return nil, fmt.Errorf("end: ending %s is not External: %w", in.Ending, ErrNoEnding)
	}

	// Build outcome with all current members' positions
	memberOutcomes := e.buildMemberOutcomes()

	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)

	e.outcome = &Outcome{
		Ending:  in.Ending,
		At:      clockReadingForBeat,
		Members: memberOutcomes,
	}

	// Record the end beat
	allMemberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		allMemberIDs = append(allMemberIDs, id)
	}
	sort.Slice(allMemberIDs, func(i, j int) bool { return allMemberIDs[i] < allMemberIDs[j] })

	beatPayload := map[string]interface{}{
		"beat":   "ended",
		"ending": in.Ending,
	}
	beatBytes, _ := json.Marshal(beatPayload)

	_, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: allMemberIDs,
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("end append beat: %w", err)
	}

	// Return a deep copy of the outcome (mutation-proof)
	outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
	for i, m := range e.outcome.Members {
		outcomeMembers[i] = MemberOutcome{
			ID:       m.ID,
			Room:     m.Room,
			Position: m.Position,
		}
	}

	return &EndOutput{
		Outcome: Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		},
	}, nil
}

// memberEntity is an internal entity for members
type memberEntity struct {
	id   string
	kind MemberKind
}

// GetID returns the member's ID
func (m *memberEntity) GetID() string {
	return m.id
}

// GetType returns the kind of member as an EntityType
func (m *memberEntity) GetType() core.EntityType {
	return core.EntityType(m.kind)
}

// GetSize returns 1 (single-cell entity)
func (m *memberEntity) GetSize() int {
	return 1
}

// BlocksLineOfSight returns false for members
func (m *memberEntity) BlocksLineOfSight() bool {
	return false
}

// BlocksMovement returns false for members
func (m *memberEntity) BlocksMovement() bool {
	return false
}
