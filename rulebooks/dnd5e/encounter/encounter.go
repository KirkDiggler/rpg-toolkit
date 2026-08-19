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
//
// DUNGEON-ABSOLUTE, and carrying no room (rpg-toolkit#1044). A sighted member is
// somewhere on the map, and the chamber they happen to be standing in is this
// composition's own bookkeeping. It matters most here of all the projections,
// because this is the payload a DECIDER reads: a monster comparing a target's
// position against its own needs both in one frame, and the two used to be in
// different ones for every room whose origin is not (0,0).
type SightPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// declaredEnding pairs an ending key with its trigger, in Setup order, plus
// the canvas cell a positional trigger was compiled to.
//
// The cell is computed once, at construction, from the authored room and the
// room-local position the trigger declares (rpg-toolkit#1106). Before the field
// became one canvas, an arrival was compared room-then-cell, in whatever frame
// the verb happened to hold; there is one frame now, so an ending is one cell,
// and the comparison is an equality. Meaningless — and never read — for a
// TriggerExternal, which carries no geometry.
type declaredEnding struct {
	key     string
	trigger Trigger
	cell    spatial.Position
}

// compileEndings resolves each declared ending's authored room and room-local
// target into the single canvas cell an arrival is compared against. Every
// room named here has already been checked to exist by validateEndingTriggers,
// which both construction seams run before this.
func compileEndings(endings []EndingInput, rooms []RoomInput) []declaredEnding {
	origins := make(map[string]spatial.Position, len(rooms))
	for _, r := range rooms {
		origins[r.ID] = r.Origin
	}

	out := make([]declaredEnding, 0, len(endings))
	for _, ei := range endings {
		de := declaredEnding{key: ei.Key, trigger: ei.Trigger}
		if t, ok := ei.Trigger.(TriggerReachedPosition); ok {
			de.cell = t.Position.Add(origins[t.Room])
		}
		out = append(out, de)
	}
	return out
}

// Encounter is the aggregate encounter composition: members, field, clock,
// intel, and record. Construct via NewEncounter or LoadEncounter; the zero
// value is unusable.
type Encounter struct {
	// canvas is THE MAP: one spatial room spanning the whole dungeon, in
	// dungeon-absolute cells, with every authored wall registered on it as an
	// absolute boundary edge (rpg-toolkit#1106).
	//
	// One, not N-plus-an-orchestrator. The composition still AUTHORS rooms —
	// that is the shape a designer writes and the shape the reference tomb is
	// written in — but a room is construction data now, and this is what it
	// compiles into. The orchestrator that used to hold the rooms went with
	// them: with one room there is nothing to orchestrate, and its connection
	// registry described a crossing mechanism this composition no longer has.
	canvas *spatial.BasicRoom

	// roomGrids is each authored room's own grid, kept from construction so
	// the field can answer which chamber owns an absolute cell without
	// rebuilding one per call — see regionAt.
	roomGrids map[string]spatial.Grid

	clock *clock.Tick
	// bubbles are the localized initiative bubbles currently running. Zero or
	// more: a bubble exists only while a fight does, and an encounter with no
	// fight has none.
	//
	// A slice rather than a single pointer even though policy allows at most one
	// today, because a slice grows to N additively and a pointer does not. And
	// deliberately WITHOUT identity: a bubble is never named, it is always
	// reached through a member, which R6 makes a total function ("an entity
	// belongs to at most one clock"). Inventing an ID would create a second
	// thing to keep true.
	bubbles     []*clock.Turn
	intelLog    *intel.Intel
	story       *record.Log
	members     map[MemberID]*memberRecord
	everMembers map[MemberID]bool // Track all members who have ever joined (for Story access)
	deciders    map[MemberID]Decider

	// initiative rolls the order a bubble forms with. Nil until Setup is given
	// one; trigger detection refuses to start a fight without it rather than
	// dropping the fight silently.
	initiative InitiativeRoller
	// standing reports who is down. Required at both constructors — see
	// [Standing] for why it is asked rather than remembered, and why there is
	// no default answer.
	standing Standing
	// endings holds declared endings in Setup order. Evaluation is
	// deterministic (law C8), but NOT globally "first-declared-wins":
	// for a single action (Step, Join) declaration order is
	// the only axis, so the first matching declared ending does win.
	// Pump can execute several monsters' actions in one tick, and there
	// evaluation walks them in DECISION order first — the action
	// decided earliest wins regardless of which of its matching endings
	// was declared later; declaration order is only the tiebreak within
	// one action's own scan. See Pump's ending-evaluation loop.
	endings []declaredEnding
	outcome *Outcome
	// retention is the story-beat window (see DefaultRetention). It persists
	// with the encounter so a reload keeps the policy it was built with.
	retention int
	// logFloor is the lowest Seq the story log still holds — everything below it
	// has been trimmed. Zero means nothing has been trimmed yet.
	//
	// Runtime state, deliberately NOT persisted: it is derived from the log
	// itself at load (logFloorOf), so it cannot drift out of agreement with the
	// entries it describes. A persisted copy could disagree with them after a
	// hand-edited blob and would then reject Story queries the log could
	// actually answer.
	logFloor uint64
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
// Only GridShapeSquare and GridShapeHex ever reach here now (#929 T2):
// LoadEncounter converts its wire-only grid string to a GridShape via
// gridDataToShape BEFORE this is ever called (gridDataToShape's doc
// comment — a stored "gridless" or otherwise unrecognized string is
// rejected there, at the string layer), and Setup's buildValidRoomGrids
// rejects gridless and any unrecognized value before calling this too.
// The switch's default case covers TWO things, only one of which is
// unreachable: it IS the normal, constantly-exercised path for every
// square room (GridShapeSquare is the zero value, so it falls to
// default rather than needing its own case) — that part is reachable on
// every square-family construction. Only the OTHER thing default
// covers — an unrecognized shape value degrading to square instead of
// panicking — is unreachable from either validated path, and exists
// only so a caller that somehow bypasses both validators degrades
// gracefully rather than panicking. GridShapeGridless itself has no
// case here at all as of T2: gridless left the composition in T1 (shape
// legality) and now has no reachable caller anywhere in this module,
// Setup or Load.
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
// square stays fractional-tolerant by design (RoomInput.Grid's doc
// comment) — call this next to every grid-deferred IsValidPosition check
// (or, where no such check exists at a seam, next to where the position
// first enters) so every externally supplied hex-room position is
// covered: member positions at Setup and Load (NewEncounter,
// LoadEncounter), Move's target (moveMember), Join's position,
// connection endpoints (validateConnectionInputs, shared by both
// construction seams), a TriggerReachedPosition ending's target
// (validateEndingTriggers, #929 T3 Opus round F5), and the two
// and a joiner's arrival cell (Join).
// Occluder positions do NOT go through this — they use the universal
// isIntegralPosition instead, every family, not just hex (#929 T3 Opus
// round F2; isIntegralPosition's own doc comment).
func isIntegralAxialPosition(grid spatial.Grid, pos spatial.Position) bool {
	if grid.GetShape() != spatial.GridShapeHex {
		return true
	}
	return pos.X == math.Trunc(pos.X) && pos.Y == math.Trunc(pos.Y)
}

// isIntegralPosition reports whether pos has X and Y that are each a
// REPRESENTABLE integer, with no grid-shape exception — the universal
// origin-legality check (#929 T1 Opus round finding): unlike
// isIntegralAxialPosition, a fractional SQUARE origin is also a defect,
// not just a fractional hex one. W2's disjointness promise (RoomInput.Origin's
// doc comment) is only sound over an INTEGER cell lattice: two 5x5 square
// rooms anchored at (0,0) and (0.5,0.5) have disjoint integer cell sets
// (W2 as enumerated would accept them) while their continuous footprints
// interpenetrate roughly 81% of each room's area, and a Chebyshev-0
// "doorway" (a connection whose two endpoints land on literally the same
// fractional point) would still measure as adjacent.
//
// "Representable" is doing real work here, not just "integral" (#929 T1
// SECOND Opus round finding): pos.X == math.Trunc(pos.X) alone passes for
// +Inf/-Inf (Trunc of infinity is infinity) and for any float64 whose
// magnitude exceeds int64's range, where roomAbsoluteBounds' int()
// conversion is Go-spec implementation-defined and silently produces
// garbage — e.g. two 5x5 rooms anchored at X=1e19 and X=2e19 both passed
// the OLD check (both "integral" by Trunc), then both truncated to the
// SAME implementation-defined int64 value, producing a false W2 overlap
// verdict through the public API for rooms that were never near each
// other, and +Inf was accepted as an anchor outright. Rejecting ±Inf/NaN
// explicitly and requiring an EXACT round trip through int() and back
// closes both holes — see isRepresentableInteger.
func isIntegralPosition(pos spatial.Position) bool {
	return isRepresentableInteger(pos.X) && isRepresentableInteger(pos.Y)
}

// isRepresentableInteger reports whether v is finite, not NaN, and an
// EXACT integer once round-tripped through int() and back — see
// isIntegralPosition for why "integral by Trunc" alone is not enough.
// float64(int(v)) == v also subsumes the plain fractional check: Go's
// float-to-int conversion truncates toward zero, so a fractional v never
// round-trips either, without a separate math.Trunc comparison.
func isRepresentableInteger(v float64) bool {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return false
	}
	return float64(int(v)) == v
}

// maxRoomSpan bounds RoomInput.Width/Height, and maxAnchorCoord bounds
// |Origin.X| and |Origin.Y| — pure integer-overflow defense (#929 T2
// second review round), not a gameplay limit: no real dungeon room is
// ever within six orders of magnitude of 1<<30 (~1.07 billion). Without
// this bound, roomAbsoluteBounds' interval-sum arithmetic
// (axisBounds' local max + Origin) can silently wrap int64: two
// same-shaped rooms with huge Width and an Origin offset chosen so their
// TRUE (infinite-precision) footprints genuinely overlap can wrap to a
// witness sum that reads as disjoint, producing a FALSE "no overlap" W2
// verdict through the public API (see the mutation-evidence fixture in
// encounter_test.go — TestSetupAnchoringOversizedRoomRejectedNotFalseDisjoint's
// comment walks through the exact wraparound). Both constants are set to the SAME
// value deliberately: with every Width/Height/Origin bounded by
// maxRoomSpan/maxAnchorCoord, the largest possible interval sum
// (maxAnchorCoord + maxRoomSpan) is ~2<<30, leaving over 30 bits of
// headroom before int64 overflow even in the most adversarial legal
// input.
const (
	maxRoomSpan    = 1 << 30
	maxAnchorCoord = 1 << 30
)

// maxRoomCells bounds a single room's total cell count (Width*Height —
// EQUAL for both grid families: axisBounds' span always equals the
// dimension itself, every parity, hex included — see axisBounds' doc
// comment), and maxFieldCells bounds the SUM of every room's cell count
// across one field. Distinct in PURPOSE from maxRoomSpan/maxAnchorCoord
// above (coordinate-OVERFLOW defense): this is the AMPLIFICATION bound.
// Two individually-legal integers under maxRoomSpan (up to 1<<30 each)
// multiply to up to 1<<60, so a 2^30 x 2^30 room passes maxRoomSpan's
// per-axis check cleanly while describing a quintillion cells —
// reachable from a persisted blob a few hundred bytes long, two
// integers and no bulk data (#929 T3 Opus round F1).
//
// IT NO LONGER GUARDS AN ALLOCATION OF OURS, and that is worth saying
// plainly rather than leaving the old reason standing. It was written
// because Atlas materialized every cell of every room through a make()
// sized Width*Height; rpg-toolkit#1108 stopped it enumerating, and
// nothing in this module allocates per cell any more (both grid
// families hold bounds only). What it guards now is the INSTRUCTION the
// region report gives: an AtlasRegion states an anchor and a span and
// leaves a host that genuinely wants the cells to walk that rectangle
// itself, so the module must not hand out a legal region no caller can
// walk. Enforced HERE, in room legality — the shared path both
// NewEncounter and LoadEncounter route through — so a field is refused
// before an Encounter exists to describe it.
const (
	maxRoomCells  = 1 << 20
	maxFieldCells = 1 << 22
)

// DefaultRetention is the number of story beats an encounter keeps when
// SetupInput.Retention is zero.
//
// Deliberately small. Under the event-stream contract the log is the truth and
// a live stream is only an optimisation over it: a client that misses beats
// notices a gap in Seq and re-queries Story from its last known sequence, and
// if it has been gone longer than the window it must resync from scratch
// instead. A generous window would make that resync path almost-never-taken and
// therefore almost-never-tested until a real player's connection dropped. A
// small one makes resync the ordinary case, so the expensive branch is the
// well-trodden one and the cheap delta is the optimisation rather than the
// assumption (#937).
//
// The window is a multiplayer-reconnect decision, not a storage one: it answers
// "how long can a client be gone and still rejoin cheaply," and the storage cost
// falls out of that rather than driving it.
const DefaultRetention = 32

// RetentionUnbounded disables trimming entirely. Appropriate for
// verified-transcript scenes, which assert on the story itself rather than on
// the retention policy, and for any caller that genuinely needs the whole
// history in the blob.
const RetentionUnbounded = -1

// normalizeRetention maps a caller-supplied retention setting onto the value the
// encounter actually uses: zero (the unset zero value) selects DefaultRetention,
// and any negative value means unbounded. Negatives are folded to
// RetentionUnbounded rather than rejected because every negative expresses the
// same intent and there is no defect to report.
func normalizeRetention(r int) int {
	switch {
	case r == 0:
		return DefaultRetention
	case r < 0:
		return RetentionUnbounded
	default:
		return r
	}
}

// logFloorOf derives the lowest Seq a persisted log still holds.
//
// Derived rather than persisted so it cannot disagree with the entries it
// describes: a stored floor could be edited into conflict with the log body, and
// would then reject Story queries the log is perfectly able to answer.
//
// Three cases. A log with entries floors at its smallest Seq — scanned rather
// than read from index zero, because a hand-edited blob is not obliged to be in
// order and the trust boundary is here. An empty log that has already assigned
// sequences was trimmed to nothing and floors at NextSeq: every Seq below it is
// genuinely gone. A never-appended log floors at zero, which exempts nothing,
// because nothing has been lost.
func logFloorOf(data record.LogData) uint64 {
	if len(data.Entries) == 0 {
		if data.NextSeq > 1 {
			return data.NextSeq
		}
		return 0
	}
	floor := data.Entries[0].Seq
	for _, entry := range data.Entries[1:] {
		if entry.Seq < floor {
			floor = entry.Seq
		}
	}
	return floor
}

// appendBeat appends one story beat and then enforces the retention window.
//
// Every beat in the composition goes through here rather than calling
// story.Append directly — eight call sites today. That is the point: a retention
// policy applied at seven of eight append sites is not a policy, and a single
// seam is the only version of this that cannot rot as verbs are added.
func (e *Encounter) appendBeat(in *record.AppendInput) (*record.AppendOutput, error) {
	out, err := e.story.Append(in)
	if err != nil {
		return nil, err
	}
	if err := e.enforceRetention(); err != nil {
		return nil, err
	}
	return out, nil
}

// enforceRetention trims the story log down to the retention window and advances
// logFloor to match.
//
// No-op when unbounded, and no-op while the log is still shorter than the window
// — TrimBefore treats a bound at or below the oldest retained Seq as a no-op
// anyway, but returning early keeps logFloor from being written on every append.
func (e *Encounter) enforceRetention() error {
	if e.retention == RetentionUnbounded {
		return nil
	}

	nextSeq, err := e.story.NextSeq()
	if err != nil {
		return fmt.Errorf("retention next seq: %w", err)
	}

	// Seq is 1-based and gapless, so after N appends nextSeq == N+1 and the log
	// holds [1, N]. Retaining `retention` entries means dropping everything
	// below nextSeq-retention. The subtraction is guarded because nextSeq <=
	// window means the log has not yet reached the window at all, and computing
	// the floor anyway would wrap on uint64.
	window := uint64(e.retention)
	if nextSeq <= window {
		return nil
	}
	floor := nextSeq - window

	// Skip when the computed floor is at or below what the log already starts
	// at — there is nothing there to drop.
	//
	// Without this the log would be trimmed the moment it reached exactly the
	// window: floor would come out as the oldest retained Seq, TrimBefore would
	// do nothing, and logFloor would still be advanced to a value describing a
	// trim that never happened. Harmless today, because a floor equal to the
	// oldest entry rejects nothing that the log can still serve — which is
	// exactly why no test in this package can distinguish the two versions. It
	// is fixed anyway: the comment above claims the early return keeps logFloor
	// from being written on every append, and code that contradicts its own
	// documentation is a trap for whoever next reasons about the floor
	// (Copilot, PR #939).
	//
	// oldest is the lowest Seq the log currently holds: logFloor once anything
	// has been trimmed, and 1 before that, since the log's first assigned Seq
	// is 1 rather than 0.
	oldest := e.logFloor
	if oldest < 1 {
		oldest = 1
	}
	if floor <= oldest {
		return nil
	}

	if _, err := e.story.TrimBefore(&record.TrimBeforeInput{Seq: floor}); err != nil {
		return fmt.Errorf("retention trim: %w", err)
	}
	e.logFloor = floor
	return nil
}

// buildValidRoomGrids rejects room defects before construction (R5
// atomicity — no observable state until Setup succeeds). The first four
// defect classes — empty or duplicate room ID, an unrecognized or
// no-longer-supported grid shape, non-integral occluder positions in
// EVERY family, not just hex (#929 T3 Opus round F2), and a duplicate
// occluder position within one room (#929 hardening round D — see
// isIntegralPosition; the occluder loop below) — are checked inside ONE
// per-room loop, not as four separate global passes: for a GIVEN room,
// that room's own ID defect wins before its shape defect, which wins
// before its occluder defects, but WHICH ROOM's defect is reported first
// depends on slice order, not defect-class priority — a field with an
// unrecognized shape on room A and an empty ID on room B (A listed
// first) reports A's shape defect, not B's empty ID, even though ID is
// listed first in this prose. Every defect class AFTER those three (W1
// onward) genuinely IS a separate pass over every room, run in the order
// named here (docs/ideas/encounter-anchoring/design.md's Validation
// section, Opus-round amendment): W1 (one grid family per field —
// validateGridFamilies); room legality (non-positive OR oversized
// Width/Height, a per-room cell count exceeding maxRoomCells, AND an
// out-of-bounds Origin — checked per room in one pass; ONLY AFTER every
// room clears that pass does a separate, final check compare the summed
// cell count against maxFieldCells, so its error names the TRUE total
// over every room, not a partial sum from wherever the loop happened to
// stop — #929 T3 trailing round N3). A negative dimension used to panic
// NewEncounter via a negative-capacity make() in the since-deleted
// enumeration path; an unbounded dimension or origin can overflow int64
// arithmetic downstream (maxRoomSpan/maxAnchorCoord); and an oversized
// CELL COUNT — legal per-axis but catastrophic multiplied — panics
// Atlas's own allocation instead (maxRoomCells/maxFieldCells, #929 T3
// Opus round F1). In every one of those three cases, a panic or a silent
// overflow is not a rejection. Then origin legality (non-representable
// Origin, for EVERY family now, not just hex — a fractional origin
// defeats W2's disjointness promise for ANY grid, not only hex: see
// isIntegralPosition); and W2 (rooms never overlap in absolute space —
// validateRoomsDisjoint). On success, returns each
// room's constructed Grid keyed by room ID — reused both for downstream
// bounds checks (connections, and transitively member placement via
// spatial's own PlaceEntity) and later in NewEncounter's room-construction
// loop, so a room's shape is built exactly once and every consumer asks the
// SAME grid. Reuses ErrNoField — a malformed room list is as unusable as an
// empty one.
//
// Error messages here carry NO verb prefix (#929 T2 second review round —
// this and validateGridFamilies/validateRoomsDisjoint/validateConnectionInputs
// are shared between NewEncounter and LoadEncounter, and a message baked
// with "newencounter:" leaked into Load's own errors, e.g. "load encounter:
// invalid encounter data: newencounter: room ... has empty id"). Each
// caller wraps its OWN verb at the call site instead — NewEncounter wraps
// "newencounter: %w", LoadEncounter wraps "load encounter: %w: %w" with
// ErrInvalidData — so the SAME validator produces a message honest about
// which seam actually rejected it.
func buildValidRoomGrids(rooms []RoomInput) (map[string]spatial.Grid, error) {
	seenIDs := make(map[string]bool, len(rooms))
	grids := make(map[string]spatial.Grid, len(rooms))
	for _, r := range rooms {
		if r.ID == "" {
			return nil, fmt.Errorf("room has empty id: %w", ErrNoField)
		}
		if seenIDs[r.ID] {
			return nil, fmt.Errorf("duplicate room %q: %w", r.ID, ErrNoField)
		}
		seenIDs[r.ID] = true

		// Shape legality: gridless leaves the composition as of v0.3 — the
		// wire cannot carry a continuous room's absolute projection — so it
		// is rejected explicitly, distinct from a genuinely unrecognized
		// value. Square and hex are the only surviving families; W1 below
		// compares them. This branch is reachable only from a direct Go-level
		// caller of NewEncounter that still constructs a GridShapeGridless
		// RoomInput (#929 T2: LoadEncounter no longer reaches this case at
		// all — a stored "gridless" grid string is rejected earlier, at the
		// string layer, by gridDataToShape before a RoomInput ever exists —
		// see this switch's Load-seam counterpart in that function's doc
		// comment).
		switch r.Grid {
		case spatial.GridShapeSquare, spatial.GridShapeHex:
		case spatial.GridShapeGridless:
			return nil, fmt.Errorf("room %q declares gridless grid shape, no longer supported: %w", r.ID, ErrNoField)
		default:
			return nil, fmt.Errorf("room %q has unknown grid shape %d: %w", r.ID, r.Grid, ErrNoField)
		}
		grids[r.ID] = buildRoomGrid(r.Grid, r.Width, r.Height)

		// Occluders must be integral in EVERY family, not just hex (#929 T3
		// Opus round F2 — the identical square-vs-hex asymmetry T1 already
		// fixed for Origin: see isIntegralPosition's doc comment):
		// an occluder is a CELL of its region, and a region's cells are
		// the integer lattice points inside its anchor-plus-span
		// (AtlasRegion's doc comment). A fractional occluder would be
		// blockage reported at a point that is not one of the region's
		// cells at all — undrawable under the host contract "floor from
		// the span, blockage from Occluders", and indistinguishable from
		// a member's position, which alone may be fractional on a square
		// grid. isIntegralPosition, not isIntegralAxialPosition —
		// universal, not hex-only.
		//
		// This is the ONLY reason left, as of #929 hardening round C: an
		// earlier version of this comment ALSO cited the occluder entity
		// ID (built below) truncating fractional coordinates to int as a
		// second, "reinforcing" reason — that claim was itself wrong in a
		// way integrality could never have fixed: truncation makes
		// coordinate-based IDs collide only WITHIN one room's own
		// occluders, but the old ID scheme concatenated the ROOM ID too
		// (occluder-<room>-<int(X)>-<int(Y)>), and room IDs are arbitrary,
		// unrestricted strings — "r" with occluder (-5,4) and "r-" with
		// occluder (5,4) both produced "occluder-r--5-4" regardless of
		// integrality, a genuine CROSS-room collision on a legal field.
		// The entity ID is index-based now (room's declaration index,
		// occluder's index within it — see the occluder-placement loop
		// below), which can never collide on ANY input, integral or not.
		// Duplicate occluder positions are a room-list defect too (#929
		// hardening round D), not something left to spatial's own voice:
		// before the occluder entity ID went index-based (hardening round
		// C), a duplicate coordinate happened to collide on the OLD
		// coordinate-derived ID and got rejected as "entity ... already
		// indexed" — spatial's vocabulary, not ours, and an accident of
		// that ID scheme, not a real "no duplicate cells" rule (spatial
		// freely allows two DIFFERENT entities to share a position). The
		// index-based ID fixed the cross-room collision but also
		// silently REMOVED that accidental duplicate-catch — two
		// occluders at the same cell now place without error. Caught
		// explicitly here instead: same room-list defect vocabulary
		// every other room check uses.
		seenOccluders := make(map[spatial.Position]bool, len(r.Occluders))
		for _, occ := range r.Occluders {
			if !isIntegralPosition(occ) {
				return nil, fmt.Errorf("room %q occluder (%g,%g) is not a representable integral cell: %w", r.ID, occ.X, occ.Y, ErrNoField)
			}
			if seenOccluders[occ] {
				return nil, fmt.Errorf("room %q has duplicate occluder (%g,%g): %w", r.ID, occ.X, occ.Y, ErrNoField)
			}
			seenOccluders[occ] = true
		}
	}

	// W1 — one geometry per field: every room in this field must share the
	// same grid family. Runs only after every room has individually passed
	// shape legality above, so only square/hex remain here.
	if err := validateGridFamilies(rooms); err != nil {
		return nil, err
	}

	// Room legality: non-positive OR oversized Width/Height is a defect,
	// not a silent no-op (#929 T1 Opus round: Width:-1 previously reached
	// buildRoomGrid and, downstream, a negative-capacity make() in the
	// enumeration W2 used before that same round replaced it with interval
	// math — a panic, not a rejection). The upper bound (maxRoomSpan, #929
	// T2 second review round) is separate, newer defense: an UNBOUNDED
	// dimension doesn't panic, but can silently overflow int64 in
	// roomAbsoluteBounds' interval-sum arithmetic, producing a false "no
	// overlap" W2 verdict instead of a crash — see maxRoomSpan's doc
	// comment and TestSetupAnchoringOversizedRoomRejectedNotFalseDisjoint's
	// mutation evidence. |Origin.X|/|Origin.Y| <= maxAnchorCoord is the
	// SAME overflow defense applied to the other operand of that same
	// interval-sum arithmetic, co-located here rather than in origin
	// legality below (#929 T2 second review round: this tightens
	// isRepresentableInteger's effective envelope — an origin like 1e19,
	// or ±Inf, is caught HERE, by the bound, before origin legality's
	// representability check ever runs; Inf is never <= a finite bound).
	// Runs after W1 (a mixed-family field with a bad dimension/origin
	// reports the family mismatch first) and before origin legality/W2
	// (both need real, bounded dimensions and origins to reason about).
	var totalCells int64
	for _, r := range rooms {
		if r.Width <= 0 || r.Height <= 0 {
			return nil, fmt.Errorf("room %q has non-positive dimensions (%d x %d): %w", r.ID, r.Width, r.Height, ErrNoField)
		}
		if r.Width > maxRoomSpan || r.Height > maxRoomSpan {
			return nil, fmt.Errorf("room %q dimensions (%d x %d) exceed max room span %d: %w", r.ID, r.Width, r.Height, maxRoomSpan, ErrNoField)
		}
		// Allocation safety (#929 T3 Opus round F1), separate from the span
		// check above: Width and Height can each be legal under
		// maxRoomSpan yet multiply to a cell count Atlas cannot safely
		// allocate — see maxRoomCells/maxFieldCells's doc comment.
		cellCount := int64(r.Width) * int64(r.Height)
		if cellCount > maxRoomCells {
			return nil, fmt.Errorf("room %q has %d cells (%d x %d), exceeding max room cells %d: %w", r.ID, cellCount, r.Width, r.Height, maxRoomCells, ErrNoField)
		}
		// Accumulated, not checked yet — checked once below, against the
		// TRUE final sum over every room (#929 T3 trailing round N3): a
		// mid-loop check here would report a partial running total on
		// whichever room happened to tip it over, undercounting the real
		// field total whenever rooms remained after it in the list.
		totalCells += cellCount
		if math.Abs(r.Origin.X) > maxAnchorCoord || math.Abs(r.Origin.Y) > maxAnchorCoord {
			return nil, fmt.Errorf("room %q origin (%g,%g) exceeds max anchor coordinate %d: %w", r.ID, r.Origin.X, r.Origin.Y, maxAnchorCoord, ErrNoField)
		}
	}
	if totalCells > maxFieldCells {
		return nil, fmt.Errorf("field has %d total cells across all rooms, exceeding max field cells %d: %w", totalCells, maxFieldCells, ErrNoField)
	}

	// Origin legality: every room's Origin must be a representable integer
	// (isRepresentableInteger — rejects fractional, NaN, and any magnitude
	// past int64's usable precision; ±Inf and magnitudes past
	// maxAnchorCoord are already caught above, in room legality). Applies
	// for EVERY grid family (#929 T1 Opus round: originally hex-only,
	// extended here — see isIntegralPosition for why a fractional SQUARE
	// origin is equally a defect). Runs after W1 and room legality so a
	// mixed-family or malformed-dimension/origin field reports THAT defect
	// first, not a representability defect on a room whose shape, size,
	// or anchor bound is already wrong.
	for _, r := range rooms {
		if !isIntegralPosition(r.Origin) {
			return nil, fmt.Errorf("room %q origin (%g,%g) is not a representable integral cell: %w", r.ID, r.Origin.X, r.Origin.Y, ErrNoField)
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

	// W6 — the field is one canvas: its whole absolute footprint must fit in a
	// single grid of its family (rpg-toolkit#1106). Runs after W2, which is
	// what makes the footprint a well-defined union in the first place.
	qMin, qMax, rMin, rMax := fieldAbsoluteBounds(rooms)
	if _, _, err := canvasSpan(rooms[0].Grid, qMin, qMax, rMin, rMax); err != nil {
		return nil, err
	}

	return grids, nil
}

// fieldAbsoluteBounds returns the union of every room's absolute footprint —
// the field's own bounding box, in dungeon-absolute cells. Requires at least
// one room, which both construction seams check before calling.
func fieldAbsoluteBounds(rooms []RoomInput) (qMin, qMax, rMin, rMax int) {
	qMin, qMax, rMin, rMax = roomAbsoluteBounds(rooms[0])
	for _, r := range rooms[1:] {
		q0, q1, r0, r1 := roomAbsoluteBounds(r)
		qMin, qMax = min(qMin, q0), max(qMax, q1)
		rMin, rMax = min(rMin, r0), max(rMax, r1)
	}
	return qMin, qMax, rMin, rMax
}

// canvasSpan returns the Width/Height a grid of this family needs in order to
// hold the field's whole absolute footprint — W6, and the check that makes
// "one room spanning the union" expressible at all (rpg-toolkit#1106).
//
// The two families anchor differently, and that difference is the whole rule.
// A SQUARE grid is the half-open rectangle [0,Width) x [0,Height): it starts at
// the origin and there is no way to move it, so a field whose footprint reaches
// a negative cell cannot be drawn on one. A HEX grid is origin-CENTERED — its
// span is [ceil(-dim/2), ceil(dim/2)-1], negative coordinates are ordinary
// there, and widening the span always reaches further both ways — so a hex
// field always fits and this never rejects one.
//
// Both formulas invert axisBounds, the same primitive W2 measures footprints
// with, so the canvas the field is drawn on and the bounds it was checked
// against cannot disagree.
//
// The remedy for a rejection is a relabeling, not a redesign: shifting every
// Origin by the same vector moves the whole dungeon into the non-negative
// quadrant and changes nothing about it, since a field's absolute frame only
// ever means "relative to the other rooms".
func canvasSpan(shape spatial.GridShape, qMin, qMax, rMin, rMax int) (width, height int, err error) {
	if shape == spatial.GridShapeHex {
		// half = dim/2, min = ceil(-half), max = ceil(half)-1 (axisBounds).
		// dim = 2*m gives min = -m and max = m-1, so m must cover both ends.
		return 2 * max(-qMin, qMax+1), 2 * max(-rMin, rMax+1), nil
	}
	if qMin < 0 || rMin < 0 {
		return 0, 0, fmt.Errorf(
			"field's absolute footprint reaches cell (%d,%d), which no square grid can hold "+
				"(square grids start at (0,0)): shift every room Origin by the same vector: %w",
			qMin, rMin, ErrNoField)
	}
	return qMax + 1, rMax + 1, nil
}

// compileCanvas turns the authored rooms into the one spatial room the
// encounter runs on: a grid spanning the field's whole absolute footprint, with
// every room's occluders and walls projected through its Origin and registered
// there.
//
// THIS IS WHERE A WALL BETWEEN TWO ROOMS BECOMES POSSIBLE. Registered on a
// room's own grid, a boundary's endpoints both had to be cells of that room
// (tools/spatial's validateAndNormalizeBoundaryUnsafe), so the seam between two
// chambers — the place a dungeon most needs a wall — was the one place one
// could not be drawn. On the canvas both endpoints are ordinary absolute cells
// and the wall registers like any other.
//
// Shared by NewEncounter and LoadEncounter, and carries NO verb prefix in its
// errors for the reason buildValidRoomGrids' doc comment gives: each caller
// wraps its own.
//
// Both seams run buildValidRoomGrids first, which is what lets this read the
// family off rooms[0] alone: W1 gives every room in a field the same one, and a
// mixed field never reaches here.
func compileCanvas(rooms []RoomInput) (*spatial.BasicRoom, error) {
	qMin, qMax, rMin, rMax := fieldAbsoluteBounds(rooms)
	width, height, err := canvasSpan(rooms[0].Grid, qMin, qMax, rMin, rMax)
	if err != nil {
		return nil, err
	}

	canvas := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "canvas",
		Type: "encounter",
		Grid: buildRoomGrid(rooms[0].Grid, width, height),
	})

	for roomIdx, ri := range rooms {
		// Occluder entity IDs stay index-based — the room's declaration index
		// and the occluder's index within it. Room IDs are arbitrary strings,
		// and a coordinate- or ID-derived key collides on legal fields (#929
		// hardening round C). A pair of slice indices cannot.
		for occIdx, pos := range ri.Occluders {
			occluder := &occluderEntity{id: fmt.Sprintf("occluder-%d-%d", roomIdx, occIdx)}
			if perr := canvas.PlaceEntity(occluder, pos.Add(ri.Origin)); perr != nil {
				return nil, fmt.Errorf("occluder placement: %w: %w", ErrBadPlacement, perr)
			}
		}

		// Endpoint ORDER is not carried: spatial normalizes an undirected pair
		// on registration (normalizedBoundary), so From and To describe the
		// same edge either way round and a wall has no side.
		for _, b := range ri.Boundaries {
			if berr := canvas.RegisterBoundary(spatial.Boundary{
				From:              b.From.Add(ri.Origin),
				To:                b.To.Add(ri.Origin),
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			}); berr != nil {
				return nil, fmt.Errorf("boundary: %w: %w", ErrBadPlacement, berr)
			}
		}
	}

	return canvas, nil
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
				return fmt.Errorf("room %q (%s) and room %q (%s) declare different grid families: %w",
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
				return fmt.Errorf("room %q and room %q overlap at absolute cell %s: %w",
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
// see buildValidRoomGrids) or non-integral (hex rooms only — see
// isIntegralAxialPosition) or on an occluder position, and (W3) endpoints
// that do not kiss — are not adjacent absolute cells once each is anchored
// to its own room's Origin. Error messages carry NO verb prefix — shared
// with LoadEncounter, same reasoning as buildValidRoomGrids' doc comment.
func validateConnectionInputs(rooms []RoomInput, roomGrids map[string]spatial.Grid, connections []ConnectionInput) error {
	roomsByID := make(map[string]RoomInput, len(rooms))
	for _, r := range rooms {
		roomsByID[r.ID] = r
	}

	seenIDs := make(map[string]bool, len(connections))
	for _, c := range connections {
		if c.ID == "" {
			return fmt.Errorf("connection has empty id: %w", ErrBadConnection)
		}
		if seenIDs[c.ID] {
			return fmt.Errorf("duplicate connection %q: %w", c.ID, ErrBadConnection)
		}
		seenIDs[c.ID] = true

		fromRoom, ok := roomsByID[c.From]
		if !ok {
			return fmt.Errorf("connection %q references unknown room %q: %w", c.ID, c.From, ErrBadConnection)
		}
		toRoom, ok := roomsByID[c.To]
		if !ok {
			return fmt.Errorf("connection %q references unknown room %q: %w", c.ID, c.To, ErrBadConnection)
		}
		if c.From == c.To {
			return fmt.Errorf("connection %q connects room %q to itself: %w", c.ID, c.From, ErrBadConnection)
		}

		if !roomGrids[c.From].IsValidPosition(c.FromPosition) {
			return fmt.Errorf("connection %q from-position out of bounds: %w", c.ID, ErrBadConnection)
		}
		if !isIntegralAxialPosition(roomGrids[c.From], c.FromPosition) {
			return fmt.Errorf("connection %q from-position is not an integral axial cell: %w", c.ID, ErrBadConnection)
		}
		if !roomGrids[c.To].IsValidPosition(c.ToPosition) {
			return fmt.Errorf("connection %q to-position out of bounds: %w", c.ID, ErrBadConnection)
		}
		if !isIntegralAxialPosition(roomGrids[c.To], c.ToPosition) {
			return fmt.Errorf("connection %q to-position is not an integral axial cell: %w", c.ID, ErrBadConnection)
		}

		for _, occ := range fromRoom.Occluders {
			if occ.X == c.FromPosition.X && occ.Y == c.FromPosition.Y {
				return fmt.Errorf("connection %q from-position on occluder: %w", c.ID, ErrBadConnection)
			}
		}
		for _, occ := range toRoom.Occluders {
			if occ.X == c.ToPosition.X && occ.Y == c.ToPosition.Y {
				return fmt.Errorf("connection %q to-position on occluder: %w", c.ID, ErrBadConnection)
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
		// Strict != 1, not > 1: origin legality's integrality requirement
		// (every family) only constrains ORIGINS, not endpoints — square
		// endpoints stay fractional-tolerant by design (RoomInput.Grid's
		// doc comment), so a fractional FromPosition/ToPosition can land
		// LESS than 1 unit from another room's boundary even with fully
		// integral, disjoint room origins: e.g. a 3x3 room at Origin (0,0)
		// and a 3x3 room at Origin (3,0), FromPosition (2.5,1) (absolute
		// (2.5,1)), ToPosition (0,1) (absolute (3,1)) — Chebyshev distance
		// 0.5. A `> 1` comparison would wrongly ACCEPT that as "close
		// enough"; `!= 1` correctly rejects it. This IS pinned — see
		// TestSetupAnchoringFractionalSquareEndpointSubUnitDistance (#929
		// T1 second Opus round: an earlier version of this comment
		// wrongly claimed sub-1 distances were unfalsifiable and left the
		// strict form unpinned).
		if dist := roomGrids[c.From].Distance(fromAbs, toAbs); dist != 1 {
			return fmt.Errorf("connection %q endpoints %s and %s are not adjacent (distance %g): %w",
				c.ID, fromAbs, toAbs, dist, ErrBadConnection)
		}
	}
	return nil
}

// validateEndingTriggers rejects a TriggerReachedPosition ending whose Room
// or Position is malformed: an ending that names no real room, or a
// position that can never be reached, can never fire — "an encounter that
// cannot end is a liveness hole" (ErrNoEnding's doc comment) applies to a
// single dead ending exactly as it does to zero endings (#929 T3 Opus round
// F5). TriggerExternal endings carry no spatial data and are skipped.
//
// Checked identically at Setup and Load — the SAME shared-validator
// pattern established for room-list/connection validation (buildValidRoomGrids'
// doc comment): unknown room, out-of-bounds position, or (hex only)
// non-integral position all reject with ErrNoEnding, no verb prefix — each
// caller wraps its own at the call site.
func validateEndingTriggers(endings []EndingInput, roomGrids map[string]spatial.Grid) error {
	for _, ei := range endings {
		trigger, ok := ei.Trigger.(TriggerReachedPosition)
		if !ok {
			continue
		}
		grid, ok := roomGrids[trigger.Room]
		if !ok {
			return fmt.Errorf("ending %q trigger names unknown room %q: %w", ei.Key, trigger.Room, ErrNoEnding)
		}
		if !grid.IsValidPosition(trigger.Position) {
			return fmt.Errorf("ending %q trigger position is out of bounds: %w", ei.Key, ErrNoEnding)
		}
		if !isIntegralAxialPosition(grid, trigger.Position) {
			return fmt.Errorf("ending %q trigger position is not an integral axial cell: %w", ei.Key, ErrNoEnding)
		}
	}
	return nil
}

// NewEncounter constructs and initializes an encounter from SetupInput.
// Validation order (first failure wins, R5 atomicity): nil input, no rooms,
// no endings, empty-or-reserved ending key, duplicate ending key (#929
// hardening round E), empty member ID, duplicate member IDs, a player
// member carrying a Decider (design law C2 — runs in
// the SAME member-ID loop, before room defects; Join's own doc comment
// lists this check too, at its own seam), room defects (empty/duplicate
// ID, unrecognized or no-longer-supported grid shape, non-integral or
// duplicate occluder position in any family, W1 mixed grid families,
// non-positive/oversized/over-cell-budget dimensions, an out-of-bounds
// Origin (maxAnchorCoord) or a non-representable one, W2 overlapping
// absolute footprints), connection defects (empty/duplicate ID, unknown
// room, self-connection, endpoint out of bounds, non-integral (hex), or
// on an occluder, W3 endpoints not adjacent once anchored), member
// position integrality, ending trigger validity (unknown room or
// unreachable position on a TriggerReachedPosition — #929 T3 Opus round
// F5), spatial placement errors.
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

	// Required, because construction is total (S8): trigger detection runs
	// from first light onward, so an encounter that can hold players and
	// monsters can start a fight before its caller does anything, and a fight
	// it cannot order is a misconfiguration. Refusing here rather than
	// mid-fight is the difference between a bug report and a bug — the
	// alternative, discovering it when two members finally see each other,
	// fails at the least convenient moment and looks like a rules bug.
	if in.Initiative == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoInitiative)
	}

	// Required for the same reason, one layer down: the standing consult runs
	// from first light too — a scene can open with a body already on the floor
	// — and an encounter that cannot ask would start fights with corpses and
	// walk them around the map. Refused at the door; never guarded at the use
	// site, and never defaulted (rpg-toolkit#1033).
	if in.Standing == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoStanding)
	}

	// Check ending keys: empty/reserved, and duplicate (#929 hardening
	// round E — two endings sharing a key both used to load; End scans
	// in declaration order, so a reached_position twin declared FIRST
	// permanently shadowed a same-keyed external ending declared after
	// it, the exact liveness hole ErrNoEnding's doc comment already
	// names for zero endings and unreachable triggers, now closed for
	// this class too).
	seenEndingKeys := make(map[string]bool, len(in.Endings))
	for _, ending := range in.Endings {
		if ending.Key == "" || ending.Key == "abandoned" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
		}
		if seenEndingKeys[ending.Key] {
			return nil, fmt.Errorf("newencounter: duplicate ending %q: %w", ending.Key, ErrNoEnding)
		}
		seenEndingKeys[ending.Key] = true
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
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Check connections: unique non-empty IDs, endpoints resolve to distinct
	// declared rooms, endpoints in bounds (per the room's own grid) and off
	// any occluder.
	if err = validateConnectionInputs(in.Field.Rooms, roomGrids, in.Field.Connections); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Hex rooms require integral axial member positions (interim
	// tools/spatial#926 enforcement — see isIntegralAxialPosition). Runs as
	// its own pass over the grid this member's declared room resolved to — a
	// member whose room doesn't exist is caught at placement, unrelated to
	// this check.
	for _, mi := range in.Members {
		if grid, ok := roomGrids[mi.Room]; ok && !isIntegralAxialPosition(grid, mi.Position) {
			return nil, fmt.Errorf("newencounter: member %q position is not an integral axial cell: %w", mi.ID, ErrBadPlacement)
		}
	}

	// A TriggerReachedPosition ending must name a real room and reachable
	// position (#929 T3 Opus round F5) — see validateEndingTriggers.
	if err := validateEndingTriggers(in.Endings, roomGrids); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Connections are stored sorted by ID (C8 determinism — order is
	// observable in ToData).
	connectionsInput := append([]ConnectionInput(nil), in.Field.Connections...)
	sort.Slice(connectionsInput, func(i, j int) bool { return connectionsInput[i].ID < connectionsInput[j].ID })

	// After validation passes, construct (R5: no observable state until success)
	e := &Encounter{
		members:          make(map[MemberID]*memberRecord),
		everMembers:      make(map[MemberID]bool),
		deciders:         make(map[MemberID]Decider),
		roomGrids:        roomGrids,
		initiative:       in.Initiative,
		standing:         in.Standing,
		endings:          nil,
		retention:        normalizeRetention(in.Retention),
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

	// Compile the authored rooms into the one canvas this encounter runs on.
	e.canvas, err = compileCanvas(in.Field.Rooms)
	if err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Place members and collect them
	memberIDs := make([]MemberID, 0, len(in.Members))
	for _, mi := range in.Members {
		memberIDs = append(memberIDs, mi.ID)

		entity := &memberEntity{
			id:   string(mi.ID),
			kind: mi.Kind,
		}

		// Room-local at authoring, absolute on the canvas: the member's
		// declared cell is checked against their own room's grid — the frame
		// they were written in — and then placed at the one cell that is.
		grid, ok := roomGrids[mi.Room]
		if !ok {
			return nil, fmt.Errorf("newencounter member placement: member %q names unknown room %q: %w", mi.ID, mi.Room, ErrBadPlacement)
		}
		if !grid.IsValidPosition(mi.Position) {
			return nil, fmt.Errorf("newencounter member placement: member %q position out of bounds in room %q: %w", mi.ID, mi.Room, ErrBadPlacement)
		}

		if err = e.canvas.PlaceEntity(entity, mi.Position.Add(roomOrigin(in.Field.Rooms, mi.Room))); err != nil {
			return nil, fmt.Errorf("newencounter member placement: %w: %w", ErrBadPlacement, err)
		}

		member := &memberRecord{
			ID:   mi.ID,
			Kind: mi.Kind,
		}
		e.members[mi.ID] = member
		e.everMembers[mi.ID] = true // Track in everMembers

		// Every member starts on the world clock. Free roam is not a mode, it
		// is simply where you are when no fight has pulled you elsewhere.
		if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(mi.ID)}); cerr != nil {
			return nil, fmt.Errorf("newencounter member %q world clock: %w", mi.ID, cerr)
		}

		// Store decider if present (monsters only, validated above)
		if mi.Decider != nil {
			e.deciders[mi.ID] = mi.Decider
		}
	}

	// Store endings in declaration order (deterministic evaluation, C8), each
	// positional one compiled to the canvas cell it fires on.
	e.endings = compileEndings(in.Endings, in.Field.Rooms)

	// First light: build sight percepts for each member using refreshSight
	firstLight, err := e.rebuildPercepts(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("newencounter first light: %w", err)
	}

	// Opening record beat: all members hear "scene-opened"
	beatPayload, _ := json.Marshal(map[string]string{"beat": "scene-opened"})
	_, err = e.appendBeat(&record.AppendInput{
		At:       0,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatPayload,
	})
	if err != nil {
		return nil, fmt.Errorf("newencounter append beat: %w", err)
	}

	// Trigger detection at first light, AFTER the scene has opened. A scene
	// can open with a wolf already staring at the party, and a fight that
	// waited for somebody to take a step would let them stand there
	// indefinitely — but the story still has to read in the order it happened,
	// and a fight that starts before the scene opens is a story nobody can
	// follow. Setup ruled that first; it generalizes to every verb and the
	// law is stated at [Encounter.refreshSight].
	//
	// This is also what makes reading only the transition lists complete
	// everywhere else: every awareness that exists was created by some
	// refreshSight, and this is the first one, so no awareness predates
	// classification and no stale asymmetry can be missed.
	if _, terr := e.applyTrigger(firstLight); terr != nil {
		return nil, fmt.Errorf("newencounter first light: %w", terr)
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
		cell, ok := e.canvas.GetEntityPosition(string(m.ID))
		if !ok {
			continue
		}
		region, _ := e.RegionAt(cell)
		outcomes = append(outcomes, MemberOutcome{ID: m.ID, Region: region, Position: cell})
	}
	return outcomes
}

// Members returns the current member roster in stable order, each with the
// dungeon-absolute cell they stand on.
//
// The position is why this exists in its current shape. Projecting a roster
// used to mean calling ToData — serializing clock, intel, log, field and
// endings to read two floats per member, once per frame in the worst case
// (rpg-toolkit#933). A roster read should cost a roster read.
//
// Returns ErrNoField if a member's room or cell cannot be resolved, which
// would mean the roster and the spatial field disagree about who is placed —
// a defect worth surfacing rather than papering over with a zero position
// that reads like the map's origin.
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
		member, err := e.placementOf(e.members[id])
		if err != nil {
			return nil, fmt.Errorf("members: %w", err)
		}
		members = append(members, member)
	}
	return members, nil
}

// placementOf builds a member's read shape: the stored record plus where they
// actually stand, projected into dungeon-absolute space.
//
// ONE projection path, used by every read that reports a member. Join and
// Members answering the same question differently is the kind of drift that
// stays invisible until two clients disagree about where somebody is.
func (e *Encounter) placementOf(record *memberRecord) (Member, error) {
	cell, err := e.cellOf(record)
	if err != nil {
		return Member{}, err
	}

	region, _ := e.RegionAt(cell)
	return Member{
		ID:       record.ID,
		Kind:     record.Kind,
		Region:   region,
		Position: cell,
	}, nil
}

// cellOf reads a member's cell off the canvas, which is the only place that
// knows it — and it is already the dungeon-absolute one every report speaks.
func (e *Encounter) cellOf(record *memberRecord) (spatial.Position, error) {
	cell, ok := e.canvas.GetEntityPosition(string(record.ID))
	if !ok {
		return spatial.Position{}, fmt.Errorf("member %q: not placed on the map: %w", record.ID, ErrNoField)
	}
	return cell, nil
}

// roomOrigin is the construction-time anchor lookup compileCanvas and member
// placement share. Rooms are few and this runs once per placed thing at
// construction, so a linear scan is the whole implementation.
func roomOrigin(rooms []RoomInput, id string) spatial.Position {
	for _, r := range rooms {
		if r.ID == id {
			return r.Origin
		}
	}
	return spatial.Position{}
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
				Region:   m.Region,
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

// Story returns a member's story entries from the given sequence number
// onward, INCLUSIVE of it — AfterSeq is passed through as record.SliceFor's
// FromSeq, and its name is a misnomer kept for compatibility (see
// StoryInput.AfterSeq). To resume after entry N, pass N+1.
//
// Allows both current members and members who have exited (everMembers).
// Returns ErrNilInput if the input is nil, ErrNoMember if the member never
// joined, and ErrTrimmed if a non-zero AfterSeq names a sequence that has
// already aged out of the retention window — the caller must resync rather
// than resume, since a short answer would be indistinguishable from a complete
// one. AfterSeq == 0 is exempt and always answerable.
// Copy-out follows record's own conventions (returned entries are already copies
// per record's implementation).
func (e *Encounter) Story(in *StoryInput) ([]record.Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}

	if _, ok := e.everMembers[in.Audience]; !ok {
		return nil, fmt.Errorf("story: %w", ErrNoMember)
	}

	// A resume point below the retained floor cannot be honoured, and must be
	// REJECTED rather than partially answered. A caller passing a sequence is
	// asserting "I already hold everything below this"; returning only the
	// surviving tail would be indistinguishable from a complete answer and would
	// leave a silent, permanent hole in that caller's story. Rejecting tells it
	// to resync (#937).
	//
	// AfterSeq == 0 is exempt: zero means "I hold nothing, send what you have,"
	// which is always answerable. That is the difference between a first load
	// and a reconnect, and it is why trimming does not break the most common
	// call in the system.
	if in.AfterSeq > 0 && in.AfterSeq < e.logFloor {
		return nil, fmt.Errorf("story: seq %d below retained floor %d: %w",
			in.AfterSeq, e.logFloor, ErrTrimmed)
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
func (e *Encounter) moveMember(member *memberRecord, to spatial.Position) (spatial.Position, error) {
	currentPos, ok := e.canvas.GetEntityPosition(string(member.ID))
	if !ok {
		return spatial.Position{}, fmt.Errorf("movemember: %w", ErrBadPlacement)
	}

	// The canvas refuses a move that crosses a movement-blocking boundary
	// (tools/spatial's MoveEntity checks every crossing on the canonical ray),
	// which is what a wall between two chambers now does and what a room
	// membership test used to stand in for.
	if err := e.canvas.MoveEntity(string(member.ID), to); err != nil {
		return spatial.Position{}, fmt.Errorf("movemember: %w: %w", ErrBadPlacement, err)
	}

	return currentPos, nil
}

// rosterIDs is every member of this encounter, in stable ID order.
//
// The refresh scope for every verb that changes what can be seen, and the
// audience for every beat those verbs append. Sorted because determinism is
// module law (C8) and because a beat's audience is persisted: an unstable
// order would rewrite the blob on a save that changed nothing.
func (e *Encounter) rosterIDs() []MemberID {
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// appendMovementBeat records one executed step in the story and returns its
// sequence number.
//
// ONE narration path for every movement this composition performs — the Step
// verb, and every monster action inside a Pump. A movement is reported TWICE,
// once as a typed output and once as a beat, and a host reading both must be
// told the same cell; two copies of this arithmetic is how those two answers
// drift apart.
//
// Cells are DUNGEON-ABSOLUTE (#1040). A room-local coordinate with no room
// attached — which is exactly what the moved beat carried before — names
// nowhere in a multi-room field: two members in different rooms could report
// the same "position" and mean cells at opposite ends of the map. A crossing's
// arrival cell is projected through the ARRIVAL room's anchor, which is a
// different one from the room it left.
//
// CALL THIS BEFORE refreshSight. A verb's own beat precedes any beat its
// consequences append — the law is stated at [Encounter.refreshSight].
func (e *Encounter) appendMovementBeat(action executedAction, audience []MemberID, at uint64) (uint64, error) {
	payload := map[string]interface{}{
		"beat":     "moved",
		"member":   string(action.member.ID),
		"position": action.to,
	}
	if action.connection != "" {
		// A step that went through a doorway names it. The BEAT does not
		// change — it is still "moved", because that is what happened
		// (rpg-toolkit#1106): a crossing stopped being a second kind of
		// movement when the field stopped being a set of rooms.
		payload["connection"] = action.connection
	}

	// PROPAGATED, not discarded. Every movement beat in this module used to
	// build its own payload and drop this error on the floor, which writes a
	// nil payload into the story and calls it a success — a beat a host reads
	// as an empty object rather than as a movement. It is unreachable in
	// practice (the payload is two strings and a position of finite float64s,
	// and construction refuses NaN and ±Inf), and it is one line here because
	// the four verbs now share one writer. clocks.go and outcome.go already
	// propagate theirs; this is the movement half catching up.
	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal moved beat: %w", err)
	}

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: audience,
		Tags:     map[string]string{"tag": "movement"},
		Payload:  beatBytes,
	})
	if err != nil {
		return 0, err
	}
	return appendOut.Seq, nil
}

// firedReachedPosition evaluates every declared ReachedPosition ending against
// where a member has just come to rest, closing the encounter if one fires.
//
// The cell is DUNGEON-ABSOLUTE, and so is the ending's, compiled from its
// authored room and local target once at construction (see compileEndings).
// One frame, one equality — where this used to compare a room label and then a
// coordinate, in whichever frame the calling verb happened to hold.
//
// The member filter carries a rule that reads backwards until you know it:
// EMPTY means any PLAYER member, not any member at all. A monster wandering
// onto the tomb's exit tile does not end the scene — the party leaving does.
// Naming a member explicitly overrides that, and then kind does not matter.
//
// Returns a DEEP COPY (mutation-proof): a caller holding the returned outcome
// cannot reach into this encounter's own.
func (e *Encounter) firedReachedPosition(member *memberRecord, cell spatial.Position, at uint64) *Outcome {
	for _, de := range e.endings {
		trigger, ok := de.trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}
		if de.cell.X != cell.X || de.cell.Y != cell.Y {
			continue // Different cell
		}
		if trigger.Member != "" && trigger.Member != member.ID {
			continue // Member filter doesn't match
		}
		if trigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only
		}

		e.outcome = &Outcome{
			Ending:  de.key,
			At:      at,
			Members: e.buildMemberOutcomes(),
		}

		members := make([]MemberOutcome, len(e.outcome.Members))
		copy(members, e.outcome.Members)
		return &Outcome{Ending: e.outcome.Ending, At: e.outcome.At, Members: members}
	}
	return nil
}

// Pump advances the world by one tick: the exploration clock advances,
// each monster member (in deterministic order) acts on its own intel via Decider,
// the complete sight refresh happens once, and the story accrues tick and
// movement beats. Errors from a decider abort the pump atomically (R5):
// no clock advance, no moves, no record entries.
//
// WHAT PUMP DRIVES IN v1, stated plainly because the answer narrowed: members
// on the WORLD clock. Under v1's sight model (rpg-toolkit#964) any monster a
// player can see is in a bubble, and a bubble member is deliberately not
// pumped — so in practice this verb moves the monsters NOBODY HAS SEEN YET.
// Free-roam monster behaviour is offscreen behaviour.
//
// That is a narrowing rather than the intent. It widens back when percept
// production grows: asymmetric perception (#1020) lets a monster be watched
// without a fight starting, and a faction model lets a visible creature be
// non-hostile. Both change what forms a bubble, not what Pump does — see
// classify's doc for the invariant. Pinned by
// TestPumpStopsMovingAMonsterOnceSeen.
//
// Semantics:
//   - Tick advances by exactly 1 (via clock.Advance with displacement 1).
//   - Monsters act in deterministic order (stable Members() order, filtered to KindMonster).
//   - Each decider receives exactly its own Snapshot: its own cell on the map and
//     its own holdings (anti-wall-hack contract C2 — placement included, not just
//     sight).
//   - IntentHold means do nothing; IntentMoveTo names a cell in dungeon-absolute
//     space and executes through stepTo — one step on the map, whether or not it
//     goes through a doorway.
//   - A refused step does NOT abort the pump — a cell no room owns, a wall in the
//     way, or any other spatial rejection all mean the monster simply fails to
//     act. Only a decider error aborts.
//   - After all monster actions: ONE refreshSight for all members, ONE tick beat
//     (stamped with the new clock reading), then movement beats in decision order
//     (the same order monsters were consulted in).
//   - Ending evaluation fires ReachedPosition triggers (only if the filter matches;
//     empty filter = players only, not monsters) against each action's resulting
//     cell, in the same decision order.
//   - Returns PumpOutput with the new Tick reading, the steps monsters took,
//     deltas, and beats.
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

	// Who is down, asked before anything is planned: a body has no action to
	// take, so its decider is not consulted at all rather than consulted and
	// discarded (a decider is behaviour, and running a corpse's behaviour is
	// the second census defect — Pump had no standing filter and dead monsters
	// kept patrolling).
	//
	// This is the SECOND consult in a Pump — refreshSight runs another at the
	// end, through noticeDown, which is what narrates and splices. Deliberate,
	// both ways round: the answer is not carried forward because carrying it
	// is a cache ([Standing]), and the narration cannot happen here because a
	// down beat appended before Pump's own tick beat would break the ordering
	// law refreshSight states.
	down, err := e.standingNow()
	if err != nil {
		return nil, fmt.Errorf("pump standing: %w", err)
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

		if down[m.ID] {
			continue
		}

		// A monster caught in a bubble is not the world's to think for: the
		// world thinks on the tick, and a fight thinks in turns. Skipped, not
		// rejected — being mid-fight is ordinary state, and Pump's job is
		// everyone else. Its budget entry is gone with it (Form removed it
		// from the tick), so the Advance below grants it nothing either.
		bubble, berr := e.bubbleFor(m.ID)
		if berr != nil {
			return nil, fmt.Errorf("pump bubble: %w", berr)
		}
		if bubble != nil {
			continue
		}

		decider, hasDecider := e.deciders[m.ID]
		if !hasDecider {
			continue // no decider = hold
		}

		// The monster's own placement, read fresh from the canvas — never
		// another member's. A decider that received anyone else's position
		// would be a wall hack extended to placement, not just sight (C2).
		ownCell, ok := e.canvas.GetEntityPosition(string(m.ID))
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

		intent, err := decider.Decide(Snapshot{
			Position: ownCell,
			Holdings: ownHoldings,
		})
		if err != nil {
			return nil, fmt.Errorf("pump decide: %w", err)
		}

		switch intent.(type) {
		case IntentMoveTo:
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

	// executedAction (declared at package scope, beside stepTo which builds
	// one) is collected in PLANNED (decision) order regardless of kind, so
	// beats and ending evaluation below stay in the same deterministic
	// per-monster order the deciders were consulted in (C8) — not "all moves
	// then all crossings".
	var executed []executedAction

	for _, p := range planned {
		// The REAL member pointer, not a Members()-derived copy: an
		// executedAction carries it into beats and ending evaluation, and a
		// value copy of a member who left mid-tick would keep those alive.
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
		if intent, ok := p.intent.(IntentMoveTo); ok {
			if action, stepped := e.stepTo(member, intent.To); stepped {
				executed = append(executed, action)
			}
		}
	}

	// Single refreshSight for all members after all monster actions.
	//
	// Derived from the roster snapshot taken BEFORE phase 1, not from live
	// membership: a contract-violating decider that removed itself mid-Decide
	// still belongs in this tick's beat audience, because an exited member
	// keeps Story access to the beats they were present for. This is why Pump
	// does not share rosterIDs with the other verbs — every one of those reads
	// a roster nothing has had a chance to change.
	memberIDs := make([]MemberID, 0, len(allMembers))
	for _, m := range allMembers {
		memberIDs = append(memberIDs, m.ID)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// Pump's own beats — the tick frame and every action inside it — are
	// recorded BEFORE sight refreshes: the monsters' walk is the cause,
	// anything trigger detection appends is its effect (see refreshSight).
	//
	// Record the tick beat first (the frame)
	tickBeatPayload := map[string]interface{}{
		"beat": "tick",
		"tick": newTickReading,
	}
	tickBeatBytes, _ := json.Marshal(tickBeatPayload)

	tickAppendOut, err := e.appendBeat(&record.AppendInput{
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
		actionSeq, err := e.appendMovementBeat(action, memberIDs, newTickReading)
		if err != nil {
			return nil, fmt.Errorf("pump append movement beat: %w", err)
		}
		seqs = append(seqs, actionSeq)
	}

	intelDeltas, formed, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("pump refresh sight: %w", err)
	}

	// Evaluate ReachedPosition endings, in decision order, against the cell
	// each step landed on.
	//
	// A monster never fires an UNFILTERED ending: the empty filter means "any
	// player member", which firedReachedPosition enforces by kind, so a
	// wandering goblin cannot end the scene by standing on the exit.
	var firedOutcome *Outcome
	for _, action := range executed {
		if firedOutcome = e.firedReachedPosition(action.member, action.to, newTickReading); firedOutcome != nil {
			break
		}
	}

	// Build the output. ONE list, in the order the steps were executed —
	// which is the order the deciders were consulted in (C8).
	//
	// It used to be two, split by which mechanism carried the step. The split
	// was never about the world: a crossing looked different because the
	// composition had two ways to move somebody, and it has one
	// (rpg-toolkit#1106). Every cell here is read straight off the canvas, so
	// what a host reads on this output and what it reads on the same movement's
	// beat cannot disagree — the drift rpg-toolkit#1062 chased was two
	// projections of one fact, and there are no projections left.
	var outputMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}
	for _, action := range executed {
		outputMoves = append(outputMoves, struct {
			Member MemberID
			From   spatial.Position
			To     spatial.Position
		}{
			Member: action.member.ID,
			From:   action.from,
			To:     action.to,
		})
	}

	return &PumpOutput{
		Tick:         newTickReading,
		MonsterMoves: outputMoves,
		IntelDeltas:  intelDeltas,
		Seqs:         seqs,
		Outcome:      firedOutcome,
		Formed:       formed,
	}, nil
}

// refreshSight rebuilds every observer's percept AND runs trigger detection on
// what changed, returning the deltas and any fight that started.
//
// The two are ONE call on purpose. Trigger detection is a rule about sight, so
// it belongs wherever sight changes — and wiring it at the verbs instead left
// crossings and Join silently untriggered until review caught them
// (rpg-toolkit#964). A verb cannot refresh sight and forget the rule if
// refreshing sight IS running the rule; a future verb gets it by writing the
// obvious call.
//
// CALL THIS AFTER YOUR VERB HAS APPENDED ITS OWN BEAT. The law, stated once
// here because here is where every verb meets it: A VERB'S OWN BEAT PRECEDES
// ANY BEAT ITS CONSEQUENCES APPEND — cause before effect, in every story. A
// reader of Story must be able to see the walk that started the fight before
// the fight. Setup ruled this first (a scene records that it opened before it
// records a fight starting inside it); the same law holds for Step, Pump and
// Join, and each one pins its own half of it.
//
// [Encounter.rebuildPercepts] is the half without the rule, and Setup is its
// only caller — Setup needs the two halves separated so its scene-opened beat
// can land between them.
func (e *Encounter) refreshSight(observers []MemberID) (map[MemberID]*intel.SurveilOutput, *FormedBubble, error) {
	deltas, err := e.rebuildPercepts(observers)
	if err != nil {
		return nil, nil, err
	}

	// A closed encounter has nothing to start a fight about, and form refuses
	// one anyway — checking here turns that refusal into a non-event.
	if e.outcome != nil {
		return deltas, nil, nil
	}

	formed, err := e.applyTrigger(deltas)
	if err != nil {
		return nil, nil, err
	}

	return deltas, formed, nil
}

// rebuildPercepts rebuilds the complete percept for all given observers,
// surveils each, and returns a map of member IDs to their SurveilOutput deltas.
// The current clock reading is stamped on each Surveil call.
func (e *Encounter) rebuildPercepts(observers []MemberID) (map[MemberID]*intel.SurveilOutput, error) {
	// Get current clock reading
	clockReadingInt := e.clock.ToData().HighWater
	clockReading := uint64(clockReadingInt)
	deltas := make(map[MemberID]*intel.SurveilOutput)

	for _, observerID := range observers {
		if _, ok := e.members[observerID]; !ok {
			continue // Skip if not found
		}

		observerCell, ok := e.canvas.GetEntityPosition(string(observerID))
		if !ok {
			continue // Observer not placed
		}

		// Every OTHER member on the map, kept or dropped by GEOMETRY ALONE.
		//
		// There was a room-membership test here, immediately before the line of
		// sight check, and it decided almost everything: two members in
		// different chambers never saw each other, however close, and the check
		// below never ran for them. It was not a range rule, it was the ONLY
		// visibility rule — standing in for the walls the composition could not
		// express (rpg-toolkit#1105/#1106). With one canvas and real walls it
		// has nothing left to say, so it is gone and the geometry answers.
		//
		// NOTHING BOUNDS SIGHT BY DISTANCE YET, and that is a known gap rather
		// than an oversight — measured, not guessed. The reference tomb ships
		// with every doorway on one row (dungeonspec puts them at
		// height/2), so its longest unobstructed sightline runs 29 cells,
		// 145 feet, from the entrance to the far wall of the tomb; the
		// skeleton-captain can see six of the entrance's forty-eight cells,
		// from 100 to 125 feet away. Sight forms a bubble, so that is a FIGHT
		// at 125 feet, in which neither side can move — there is no in-fight
		// movement verb yet.
		//
		// What is missing there is a LIGHT model, not a range term. That
		// sightline is a genuinely unobstructed run down three aligned
		// doorways, and seeing along it is correct; what a crypt denies you is
		// illumination, which this composition has never modelled. A distance
		// cutoff would paper over that with a number instead of naming it.
		//
		// So the range term is rpg-toolkit#1105's remaining half, and it lands
		// with the caller that supplies it rather than as a default this module
		// would be inventing (the argument [Standing] makes about answers this
		// module is not allowed to decide for itself). Sight the other way is
		// already better than it was: a member 35 feet down the hall is
		// visible now, and was invisible under the room label.
		var percept []intel.Report
		for _, otherMember := range e.members {
			if otherMember.ID == observerID {
				continue // Skip self
			}

			otherCell, ok := e.canvas.GetEntityPosition(string(otherMember.ID))
			if !ok {
				continue // Not placed
			}

			if e.canvas.IsLineOfSightBlocked(observerCell, otherCell) {
				continue // A wall, or something standing in the way
			}

			pos := SightPayload{X: otherCell.X, Y: otherCell.Y}
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

	if in.Member == "" {
		return nil, fmt.Errorf("join: %w", ErrNoMember)
	}

	// Check if already a member
	if _, exists := e.members[in.Member]; exists {
		return nil, fmt.Errorf("join: member %s is already in the encounter: %w", in.Member, ErrNoMember)
	}

	// Players cannot carry deciders (design law C2)
	if in.Kind == KindPlayer && in.Decider != nil {
		return nil, fmt.Errorf("join: player %s cannot carry a decider: %w", in.Member, ErrNoMember)
	}

	// Hex fields require integral axial cells (interim tools/spatial#926
	// enforcement — see isIntegralAxialPosition). Asked first, for the reason
	// [Encounter.stepMember] asks it first: a fractional cell is an arithmetic
	// mistake and must not be reported as a map one.
	if !isIntegralAxialPosition(e.canvas.GetGrid(), in.Cell) {
		return nil, fmt.Errorf("join: position is not an integral axial cell: %w", ErrBadPlacement)
	}

	// The arrival cell must be FLOOR — some authored chamber's footprint has
	// to hold it. The canvas spans the field's whole bounding box, so "on the
	// map" and "somewhere a member can stand" are different questions, and
	// this is the one that matters (the same check [Encounter.stepMember]
	// makes for a step).
	if _, owned := e.RegionAt(in.Cell); !owned {
		return nil, fmt.Errorf("join: cell %v is not floor: %w", in.Cell, ErrBadPlacement)
	}

	entity := &memberEntity{
		id:   string(in.Member),
		kind: in.Kind,
	}

	if err := e.canvas.PlaceEntity(entity, in.Cell); err != nil {
		return nil, fmt.Errorf("join placement: %w: %w", ErrBadPlacement, err)
	}

	// Register the member
	member := &memberRecord{
		ID:   in.Member,
		Kind: in.Kind,
	}
	e.members[in.Member] = member
	e.everMembers[in.Member] = true // Track in everMembers

	// A joiner lands on the world clock, never mid-fight. Being pulled into a
	// running bubble is Transfer's job and is a separate decision from joining
	// the encounter at all.
	if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(in.Member)}); cerr != nil {
		return nil, fmt.Errorf("join member %q world clock: %w", in.Member, cerr)
	}

	// Store decider if present (monsters only, validated above)
	if in.Decider != nil {
		e.deciders[in.Member] = in.Decider
	}

	// Audience for both the join beat and the sight refresh: the joiner sees
	// incumbents, incumbents see the joiner.
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// Record the join beat BEFORE refreshing sight: arriving is the cause,
	// anything trigger detection appends is its effect (see refreshSight).
	// Audience = all members including the joiner.
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":   "joined",
		"member": string(in.Member),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("join append beat: %w", err)
	}

	seqNum := appendOut.Seq

	intelDeltas, formed, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("join refresh sight: %w", err)
	}

	// Evaluate ReachedPosition endings (a player could join ON the stairs),
	// through the SAME firedReachedPosition every other arrival uses.
	//
	// Join used to carry its own copy of that scan — the census's side-finding
	// 2, and the defect class rpg-toolkit#1059 spent two PRs eliminating for
	// movement. The copy existed because Join held a room and a local cell
	// while the shared path held the member's current room; with one frame
	// there is nothing left for a second implementation to differ about.
	firedOutcome := e.firedReachedPosition(member, in.Cell, clockReadingForBeat)

	placement, err := e.placementOf(member)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	return &JoinOutput{
		Formed:      formed,
		Member:      placement,
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

	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("exit: %w", ErrNotMember)
	}

	// Get the exiting member's final cell, and the region it falls in.
	finalCell, ok := e.canvas.GetEntityPosition(string(in.Member))
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}
	finalRegion, _ := e.RegionAt(finalCell)

	// Capture the exiting member's holdings (carry-forward)
	carry, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("exit held_by: %w", err)
	}

	// Remove from the map
	if err = e.canvas.RemoveEntity(string(in.Member)); err != nil {
		return nil, fmt.Errorf("exit remove entity: %w: %w", ErrBadPlacement, err)
	}

	// Remove from whichever clock holds them — the world clock normally, a
	// bubble if they were in a fight when they left.
	if cerr := e.leaveAnyClock(in.Member); cerr != nil {
		return nil, fmt.Errorf("exit member %q clock: %w", in.Member, cerr)
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

	appendOut, err := e.appendBeat(&record.AppendInput{
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
		_, _, err := e.refreshSight(memberIDs)
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
			Region:   finalRegion,
			Position: finalCell,
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

	_, err := e.appendBeat(&record.AppendInput{
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
			Region:   m.Region,
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
