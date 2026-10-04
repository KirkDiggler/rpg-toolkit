// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"gopkg.in/yaml.v3"
)

// single_room_walls.go is A STRUCTURAL WALL IN THE SINGLE-ROOM DIALECT
// (rpg-project#169; single-room-play.md §3) — the `walls:` block the World
// Builder authors, the refusals it earns, and the one thing it compiles to.
//
// # What a wall is, and what it is not
//
// A wall is a LINE and an independent BLOCKER, exactly as the design law
// states: the author draws the line, the appearance repeats along it as
// presentation, and the blocker rectangle — width, depth, both offsets and
// two independent flags — is what the engine reads (rpg-project#506). The
// appearance never decides a mechanic and a rendered mesh bound never becomes
// a rule.
//
// THE OPENINGS ARE SUBTRACTED FROM THE BLOCKER, AFTER the blocker's own
// longitudinal extent and offset are defined. That ordering is the whole of
// the review's correction #3: cutting the visible line first and then pushing
// each blocking span along by an offset refills the doorway. Define the
// authored rectangle, subtract the opening corridors, and every remaining
// positive span becomes one rotated box on the wall's own line.
//
// # What crosses into the engine
//
// NOTHING NEW. Each remaining span is a [encounter.PlacedPropInput] — the
// same contributor placed props and footprint doors already are — converted
// by the SAME [placedFootprintFrom] adapter every placed prop goes through
// (single_room_placement.go). There is no new collider, no second geometry
// store, and no wall-to-edge substitution: the connected-sight contract reads
// the placed set it always read, and a gap is simply a span that is not
// there.
//
// AND ONE IDENTITY-ONLY PRESENCE ENTRY PER WALL (rpg-project#169, Task 8),
// carrying the raw wall id and the COMPLETE authored blocker rectangle with
// BOTH FLAGS FALSE. It answers identity and presence only — it is what a
// client joins by to the authored wall content, and it must never acquire
// movement or line-of-sight blocking, or it would refill a doorway the spans
// left clear. The blocking geometry is solely the derived spans below; a wall
// whose blocker subtracts to ZERO spans still has its presence. One source
// wall, one lowering, two kinds of placed contributor; no second editable or
// persisted authoring model and no proto field.
//
// THE VISUAL PIECE COUNT IS NOT CONSULTED. Appearance repetition is the
// renderer's concern; blocking geometry is the authored blocker's, cut by
// the opening intervals, and nothing here lets a piece count or an asset's
// measured width change a footprint. The presence rectangle is not used to
// draw the wall or to choose its visible spans either.
//
// # Identity
//
// The presence entry's id is the raw wall id, and a generated span's id is
// deterministic: `wall/<wall id>/span/<index>`, in sorted geometric span
// order (by distance along the wall from its start). The author keeps the wall
// id — it is what a later appearance join names — and no stable-span promise
// is made across edits: cutting a doorway renumbers the spans after it, which
// is honest, because a span is derived.
//
// A generated id colliding with an authored placed or door identity, or with
// another wall's raw identity, is REFUSED by path, never silently resolved by
// replacement ([wallCollision]). Two contributors under one name would give a
// cell fold and an atlas two answers for one name, which is the rule
// [encounter.PlacedPropInput] already states at its own seam.
//
// # Concealment
//
// A source `concealments.<id>.props` entry may name the raw wall id. The
// lowering expands it to that presence entry AND every span id the SAME
// lowering produced, so neither the wall's existence nor its obstruction
// geometry is disclosed to an observer who has not found the secret. An
// attached door on one of its openings is NOT swept in: its independent id is
// selected explicitly, exactly like any other source item.

// The sentences this file adds, verbatim. Constants for
// single_room_doors.go's reason: the same defect always reports the same
// words, and a test pins the sentence an author reads rather than a
// paraphrase of it.
const (
	// wallNeedsRootV4 is a non-empty `walls:` under a root that cannot hold
	// it. v3 is COMPLETE for what it declares, and a wall is not one of its
	// keys, so the version is the first thing to fix.
	wallNeedsRootV4 = "structural walls require root version 4"

	// wallIDCollision is a wall or opening id that is already an id in this
	// document — another wall's, another opening's, or a scene item's.
	wallIDCollision = "duplicate or colliding id %q"

	// wallBlockerFlagsRequired is a hand-assembled blocker that never said
	// whether it blocks. A decode refuses this at the shape walk; the
	// sentence survives for a spec built in Go.
	wallBlockerFlagsRequired = "must be true or false"

	// wallOpeningsOverlap is the later of two openings whose corridors share
	// line. Touching is legal — the earlier one ends exactly where this one
	// begins — so only a positive overlap earns it.
	wallOpeningsOverlap = "overlaps another opening"

	// wallOpeningOutsideLine is an opening corridor that leaves the visible
	// wall. A gap wider than the wall is a doorway that is not in a wall.
	wallOpeningOutsideLine = "must fit within the line"

	// wallUnrepresentable is a value that is finite but beyond any cell the
	// engine can measure.
	wallUnrepresentable = "must be representable"

	// attachedDoorNeedsBinding is an opening that attaches a door the room
	// declares no state for. Presence REQUIRES an explicit root binding:
	// `{}` is an open door, and absence is a bare opening, so a door with no
	// binding is a door nobody can open or shut.
	attachedDoorNeedsBinding = "an attached door needs its state under doorBindings: " +
		"declare it there, and `{}` means it starts open"
)

// # Source shape

// wallsShape reads the optional `walls:` list. [doorBindingsShape]'s reasons,
// one structural noun over: the typed decode refuses a value of the wrong
// KIND outright, but reads an authored `null` as the Go zero value without a
// word — a wall with no id, a line the author deleted, an opening with no
// width. Every gameplay scalar of a wall is required, because `0` and `false`
// are real values, not stand-ins for "never said".
func wallsShape(gp *yaml.Node, add errSink) {
	walls := optionalNode(gp, "walls", "room.room", add)
	if walls == nil {
		return
	}
	if walls.Kind != yaml.SequenceNode {
		add("room.room.walls", errNotAList)
		return
	}
	for i, raw := range walls.Content {
		p := fmt.Sprintf("room.room.walls[%d]", i)
		w := resolveNode(raw)
		if w == nil || isNull(w) {
			add(p, errNotNull)
			continue
		}
		if w.Kind != yaml.MappingNode {
			add(p, errNotAMapping)
			continue
		}
		requireString(w, "id", p, add)
		requireString(w, "label", p, add)
		if line := requireMapping(w, "line", p, add); line != nil {
			wallPointShape(line, "start", p+".line", add)
			wallPointShape(line, "end", p+".line", add)
		}
		if appearance := requireMapping(w, "appearance", p, add); appearance != nil {
			requireString(appearance, "assetRef", p+".appearance", add)
			for _, k := range [...]string{"height", "thickness", "elevation"} {
				requireNumber(appearance, k, p+".appearance", add)
			}
		}
		if blocker := requireMapping(w, "blocker", p, add); blocker != nil {
			declarationShape(blocker, p+".blocker", add)
		}
		if openings := requireSequence(w, "openings", p, add); openings != nil {
			for j, raw := range openings.Content {
				op := fmt.Sprintf("%s.openings[%d]", p, j)
				entryShape(raw, op, add)
				o := resolveNode(raw)
				if o == nil || o.Kind != yaml.MappingNode {
					continue
				}
				requireString(o, "id", op, add)
				requireNumber(o, "position", op, add)
				requireNumber(o, "width", op, add)
				// AND THE OPTIONAL ATTACHED DOOR (rpg-project#169). Its shape is
				// the web's `{id, assetRef}` and nothing else; an authored null
				// is the author having deleted the door, not an absent one. An
				// unknown key inside it (a `transform`, say) is refused by
				// [yaml.v3]'s KnownFields and named at this path by unknown_key.go.
				if door := optionalNode(o, "door", op, add); door != nil {
					doorPath := op + ".door"
					if door.Kind != yaml.MappingNode {
						add(doorPath, errNotAMapping)
						continue
					}
					requireString(door, "id", doorPath, add)
					requireString(door, "assetRef", doorPath, add)
				}
			}
		}
	}
}

// wallPointShape reads one `{x, z}` point. Both coordinates are required and
// must be numeric; whether they are finite is the value pass's question, for
// [cellShape]'s reason.
func wallPointShape(line *yaml.Node, key, p string, add errSink) {
	pt := requireMapping(line, key, p, add)
	if pt == nil {
		return
	}
	requireNumber(pt, "x", p+"."+key, add)
	requireNumber(pt, "z", p+"."+key, add)
}

// # Decoded values

// wallValues judges every wall this room declares: its identity, its line, its
// blocker and its openings. It is INERT on a document that declares none, so
// a room without walls validates exactly as it did before this key existed.
//
// THE ROOT VERSION IS ASKED FIRST and the geometry is not graded behind a
// missing version word: a wall under a v3 root is one mistake with one
// sentence, not a version defect followed by a list of geometry defects the
// author cannot act on until the version is fixed.
func wallValues(gp *RoomGameplaySource, rootVersion int, read roomRead, add errSink) {
	if len(gp.Walls) == 0 {
		return
	}
	if rootVersion < 4 {
		add("room.room.walls", wallNeedsRootV4)
		return
	}

	// EVERY ID IS CLAIMED BEFORE ANY OPENING IS READ. The web reserves all
	// wall identities first (validateStructuralWalls), so an opening whose id
	// collides with a LATER wall cannot hide behind authored order — and the
	// refusal is ordering-independent, which is what makes it deterministic.
	claimed := map[string]bool{}
	for id := range read.ItemIDs {
		claimed[id] = true
	}
	for i := range gp.Walls {
		claimWallID(gp.Walls[i].ID, fmt.Sprintf("room.room.walls[%d].id", i), claimed, add)
	}
	for i := range gp.Walls {
		wall := &gp.Walls[i]
		p := fmt.Sprintf("room.room.walls[%d]", i)
		wallBlockerValues(wall, p, add)
		length, lineOK := wallLineValues(wall.Line, p, add)
		for j := range wall.Openings {
			o := &wall.Openings[j]
			op := fmt.Sprintf("%s.openings[%d]", p, j)
			// AN ATTACHED DOOR IS CLAIMED FIRST, matching the web's own order
			// (validateStructuralWalls claims a door id before its opening's),
			// so an id collision reports at the same path in both dialects.
			if o.Door != nil {
				wallAttachedDoorValues(o.Door, op+".door", gp, claimed, add)
			}
			claimWallID(o.ID, op+".id", claimed, add)
		}
		if lineOK {
			wallOpeningValues(wall.Openings, length, p, add)
		}
	}
}

// wallAttachedDoorValues judges the one optional record an opening carries:
// a nonempty identity already spoken nowhere in this document (a scene item, a
// wall, another opening or another attached door), a nonempty appearance
// reference, and an explicit state under `doorBindings`.
//
// THE BINDING IS THE DOOR'S PRESENCE (rpg-dnd5e-web structuralDoorEditing.ts):
// `{}` under `doorBindings` is an OPEN door, so absence means bare opening and
// presence means a door — there is no third reading, and a door whose binding
// is missing is named here at the attachment the author drew it on.
//
// THE APPEARANCE IS CARRIED UNREAD. `assetRef` is required to be a nonempty
// string because it is the one word the renderer needs, but membership in a
// catalog is the web codec's refusal, not this engine's.
func wallAttachedDoorValues(door *RoomWallDoor, doorPath string, gp *RoomGameplaySource, claimed map[string]bool, add errSink) {
	claimWallID(door.ID, doorPath+".id", claimed, add)
	if door.AssetRef == "" {
		add(doorPath+".assetRef", errRequired)
	}
	if door.ID != "" {
		if _, bound := gp.DoorBindings[door.ID]; !bound {
			add(doorPath, attachedDoorNeedsBinding)
		}
	}
}

// claimWallID refuses an empty id and an id already spoken in this document —
// a scene item, another wall, or another opening. The universe is seeded from
// [roomRead.ItemIDs], because a wall or opening id that is also a scene item
// id would let a wall's generated span id collide with a placed prop under
// the same name.
func claimWallID(id, path string, claimed map[string]bool, add errSink) {
	if id == "" {
		add(path, "must be a nonempty string")
		return
	}
	if claimed[id] {
		add(path, fmt.Sprintf(wallIDCollision, id))
		return
	}
	claimed[id] = true
}

// wallBlockerValues asks a wall's independent rectangle whether it is legal:
// a positive finite width and depth, finite offsets, finite representable
// numbers, and both blocking answers present. The per-prop 12-unit clamp does
// NOT apply here — this is a wall's own rectangle, and a long wall keeps its
// exact extent.
func wallBlockerValues(wall *RoomWall, p string, add errSink) {
	declPath := p + ".blocker"
	if wall.Blocker.BlocksMovement == nil {
		add(declPath+".blocksMovement", wallBlockerFlagsRequired)
	}
	if wall.Blocker.BlocksLineOfSight == nil {
		add(declPath+".blocksLineOfSight", wallBlockerFlagsRequired)
	}
	fp := wall.Blocker.Footprint
	wallNumber(fp.Width, declPath+".footprint.width", true, add)
	wallNumber(fp.Depth, declPath+".footprint.depth", true, add)
	wallNumber(fp.OffsetX, declPath+".footprint.offsetX", false, add)
	wallNumber(fp.OffsetZ, declPath+".footprint.offsetZ", false, add)
}

// wallLineValues asks whether a wall's two endpoints are finite and
// representable and the line between them is nonzero. It reports whether the
// line was readable, so the opening walk is not run against a length that does
// not exist.
//
// THE EDITOR'S `workspace.horizontalLimit` IS NOT RE-IMPOSED HERE. That word
// is the World Builder's drawing bound, and this lowering is deliberately
// indifferent to it (single_room_lowering_test.go's
// TestTheLoweringIsIndifferentToTheEditorsOldBounds pins the same indifference
// for the v3 fixtures). What survives into the engine is the frame the room's
// own scene items are authored in — scene XZ units, finite, and within any
// magnitude the placed-contributor adapter can measure — not the editor's
// scalar limit.
func wallLineValues(line RoomWallLine, p string, add errSink) (float64, bool) {
	ok := true
	for _, c := range [...]struct {
		path string
		v    float64
	}{
		{p + ".line.start.x", line.Start.X},
		{p + ".line.start.z", line.Start.Z},
		{p + ".line.end.x", line.End.X},
		{p + ".line.end.z", line.End.Z},
	} {
		if !wallFinite(c.v) || !wallRepresentable(c.v) {
			add(c.path, wallFiniteMessage(c.v))
			ok = false
		}
	}
	if !ok {
		return 0, false
	}
	dx, dz := line.End.X-line.Start.X, line.End.Z-line.Start.Z
	length := math.Hypot(dx, dz)
	if !wallFinite(length) || length <= 0 {
		add(p+".line", "must have a finite positive length")
		return 0, false
	}
	return length, true
}

// wallOpeningValues asks every opening whether it could be a gap in THIS
// wall: a finite positive width and finite position, a corridor inside the
// visible line, and no overlap with another. Touching is legal and an opening
// at either endpoint is legal (single-room-play.md §3).
//
// THE OPENING FITS THE VISIBLE LINE, NOT THE BLOCKER. The blocker may extend
// far past the line and an opening is still valid; the subtraction below
// removes only the line-local corridor, and whatever blocker lies beyond the
// opening survives — which is the independent-extent rule the geometry tests
// pin.
func wallOpeningValues(openings []RoomWallOpening, length float64, wallPath string, add errSink) {
	type cut struct {
		start, end float64
		index      int
	}
	roundoff := 8 * (math.Nextafter(1, 2) - 1) * length // 8 ULP of the measured length
	cuts := make([]cut, 0, len(openings))
	for j := range openings {
		o := &openings[j]
		ok := true
		for _, c := range [...]struct {
			path     string
			v        float64
			positive bool
		}{
			{fmt.Sprintf("%s.openings[%d].position", wallPath, j), o.Position, false},
			{fmt.Sprintf("%s.openings[%d].width", wallPath, j), o.Width, true},
		} {
			if !wallFinite(c.v) || !wallRepresentable(c.v) {
				add(c.path, wallFiniteMessage(c.v))
				ok = false
				continue
			}
			if c.positive && c.v <= 0 {
				add(c.path, "must be a finite positive number")
				ok = false
			}
		}
		if !ok {
			continue
		}
		a, b := o.Position-o.Width/2, o.Position+o.Width/2
		if a < -roundoff || b > length+roundoff {
			add(fmt.Sprintf("%s.openings[%d]", wallPath, j), wallOpeningOutsideLine)
			continue
		}
		// Clamp the corridor to the visible line. The roundoff above is
		// allowed to touch an endpoint exactly; a clamp keeps a corridor that
		// ends within a ULP of the end from cutting a phantom sliver.
		if a < 0 {
			a = 0
		}
		if b > length {
			b = length
		}
		cuts = append(cuts, cut{start: a, end: b, index: j})
	}

	sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
	for k := 1; k < len(cuts); k++ {
		if cuts[k].start < cuts[k-1].end {
			add(fmt.Sprintf("%s.openings[%d]", wallPath, cuts[k].index), wallOpeningsOverlap)
		}
	}
}

// wallFinite reports whether a wall scalar is a real number.
func wallFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// wallRepresentable is a coarse source-scalar safety bound, not a proof that
// the derived placement fits. wallLoweringFor also checks the canonical values
// after conversion to feet and after combining the authored offsets.
func wallRepresentable(v float64) bool { return math.Abs(v) <= maxAuthoredCoord }

// wallFiniteMessage is the one sentence for a value that is not a finite,
// representable number. A NaN, an infinity and an overflowing magnitude read
// differently to an author, but all three are the same mistake: a dimension
// that is not a dimension.
func wallFiniteMessage(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "must be finite"
	}
	return wallUnrepresentable
}

// wallNumber is [boundFootprint]'s wall sibling: finite, representable, and,
// for a side that must be positive, strictly greater than zero.
func wallNumber(v float64, p string, positive bool, add errSink) {
	if !wallFinite(v) {
		add(p, "must be finite")
		return
	}
	if !wallRepresentable(v) {
		add(p, wallUnrepresentable)
		return
	}
	if positive && v <= 0 {
		add(p, "must be a finite positive number")
	}
}

// # The lowering

// wallLowering is one authored wall's compiled output: the IDENTITY-ONLY
// presence entry a client joins by raw wall id, and the blocking spans the
// connected-sight contract reads. Both are derived from ONE source wall in
// one geometry pass, so there is no second authoring model and no way for the
// presence rectangle and the spans to disagree about where the wall is. The
// wall index and id are kept so a collision refusal can name the author's own
// path rather than a generated id they never wrote.
type wallLowering struct {
	wallIndex int
	wallID    string
	presence  encounter.PlacedPropInput
	spans     []encounter.PlacedPropInput
}

// canonicalWallLowerings lowers every authored wall into its presence entry
// and its blocking spans, converted by the ONE adapter every placed prop goes
// through.
//
// It is the compile-time half, and it is only reached for a document
// [validateSingleRoom] has already accepted — every source scalar is finite
// and bounded and every flag is present. Derived canonical values are checked
// here, after conversion, so a refusal names the authored wall. It still returns an error rather
// than panicking on a spec assembled in Go, because [RoomSource] is exported.
func canonicalWallLowerings(walls []RoomWall) ([]wallLowering, error) {
	out := make([]wallLowering, 0, len(walls))
	for i := range walls {
		lowered, err := wallLoweringFor(&walls[i], i)
		if err != nil {
			return nil, fmt.Errorf("room.room.walls[%d]: %w", i, err)
		}
		out = append(out, lowered)
	}
	return out, nil
}

// wallLoweringFor is the one lowering of a source wall: its complete authored
// blocker as the presence rectangle, and that blocker MINUS its openings as
// the blocking spans.
//
// THE ARITHMETIC, TERM BY TERM (single_room_placement.go pins the adapter's
// half):
//
//	L          = |end - start|                       scene units
//	dir        = (end - start) / L                   start→end, scene XZ
//	blocker    = [L/2 + offsetX - width/2,
//	              L/2 + offsetX + width/2]           distance from start
//	opening[j] = [position - width/2,
//	              position + width/2]                 distance from start
//	span       = blocker minus every opening corridor (blocker overhang survives)
//	presence   = blocker, uncut, both flags FALSE
//
// Each span's longitudinal centre sits ON THE WALL LINE at the middle of the
// span, and the transverse offsetZ stays the box's local offset — so the
// authored width, depth, offsetX and offsetZ all survive, exactly (the review
// refused dropping the lateral offset). The PRESENCE rectangle is the same
// authored blocker placed at the wall's own pose — its midpoint on the line —
// so all four footprint fields survive there verbatim too (offsetX is local
// to that midpoint by the wall schema's own law). The source yaw is
// atan2(-dz, dx), which the adapter turns into the canonical facing
// atan2(dz, dx); that sign is pinned against known endpoints in the tests
// rather than assumed.
func wallLoweringFor(wall *RoomWall, index int) (wallLowering, error) {
	if wall.Blocker.BlocksMovement == nil || wall.Blocker.BlocksLineOfSight == nil {
		return wallLowering{}, fmt.Errorf("blocker does not say both blocking answers")
	}
	start, end := wall.Line.Start, wall.Line.End
	dx, dz := end.X-start.X, end.Z-start.Z
	length := math.Hypot(dx, dz)
	if !wallFinite(length) || length <= 0 {
		return wallLowering{}, fmt.Errorf("line has no finite positive length")
	}
	dirX, dirZ := dx/length, dz/length
	yaw := math.Atan2(-dz, dx)

	fp := wall.Blocker.Footprint

	// THE PRESENCE RECTANGLE IS THE COMPLETE AUTHORED BLOCKER at the wall's
	// own pose: its midpoint on the line, with all four of the blocker's
	// footprint fields untouched, so the rectangle is the full blocker the
	// spans below were cut from. It is NEVER cut by an opening, and BOTH FLAGS
	// ARE FALSE: it answers identity/presence only, so it can never refill a
	// doorway the spans left clear. A wall whose blocker subtracts to zero
	// spans still has this entry.
	presencePlacement := placedFootprintFrom(
		RoomPropDeclaration{Footprint: fp},
		scenePose{X: start.X + dirX*(length/2), Z: start.Z + dirZ*(length/2), RotationY: yaw},
	)
	if err := canonicalPlacementBounds(presencePlacement); err != nil {
		return wallLowering{}, err
	}
	lowered := wallLowering{
		wallIndex: index,
		wallID:    wall.ID,
		presence: encounter.PlacedPropInput{
			ID:                encounter.PropID(wall.ID),
			Placement:         presencePlacement,
			BlocksMovement:    false,
			BlocksLineOfSight: false,
		},
	}

	blocker := wallInterval{
		start: length/2 + fp.OffsetX - fp.Width/2,
		end:   length/2 + fp.OffsetX + fp.Width/2,
	}
	spans := subtractIntervals(blocker.start, blocker.end, wallCutIntervals(wall.Openings, length))
	lowered.spans = make([]encounter.PlacedPropInput, 0, len(spans))
	for spanIndex, span := range spans {
		placement := wallPiecePlacement(start, dirX, dirZ, yaw, span, fp.Depth, fp.OffsetZ)
		// Match encounter's canonical placement ceiling, not the source-unit
		// bound: both conversion and offsets can push a legal source scalar
		// past it. Keep the refusal at the authored wall instead of partyStart.
		if err := canonicalPlacementBounds(placement); err != nil {
			return wallLowering{}, err
		}
		lowered.spans = append(lowered.spans, encounter.PlacedPropInput{
			ID:                encounter.PropID(fmt.Sprintf("wall/%s/span/%d", wall.ID, spanIndex)),
			Placement:         placement,
			BlocksMovement:    *wall.Blocker.BlocksMovement,
			BlocksLineOfSight: *wall.Blocker.BlocksLineOfSight,
		})
	}

	return lowered, nil
}

// wallPiecePlacement converts ONE blocking span interval into the canonical
// box the adapter places: the interval's centre sits ON THE WALL LINE, the
// box's length is the interval's exact length, and the blocker's depth and
// transverse offsetZ survive. Every span shares it, so they cannot drift
// from the presence rectangle they were cut out of.
func wallPiecePlacement(
	start RoomWallPoint, dirX, dirZ, yaw float64, span wallInterval, depth, offsetZ float64,
) spatial.FootprintPlacement {
	centre := (span.start + span.end) / 2
	pose := scenePose{
		X:         start.X + dirX*centre,
		Z:         start.Z + dirZ*centre,
		RotationY: yaw,
	}
	decl := RoomPropDeclaration{
		Footprint: RoomFootprint{
			Width:   span.end - span.start,
			Depth:   depth,
			OffsetX: 0, // the interval's centre is already offsetX along the line
			OffsetZ: offsetZ,
		},
	}
	return placedFootprintFrom(decl, pose)
}

// wallIDSet is the universe of authored wall ids, for the callers that only
// need membership rather than the wall itself — the concealment walk, which
// accepts a wall id in `concealments.<id>.props`.
func wallIDSet(walls []RoomWall) map[string]bool {
	out := map[string]bool{}
	for i := range walls {
		out[walls[i].ID] = true
	}
	return out
}

// wallSpanIDs maps each authored wall id to the generated span ids the SAME
// lowering produced, so a concealment names exactly the contributors that
// were compiled rather than a second calculation of them.
func wallSpanIDs(lowerings []wallLowering) map[string][]encounter.PropID {
	out := make(map[string][]encounter.PropID, len(lowerings))
	for _, lowered := range lowerings {
		ids := make([]encounter.PropID, 0, len(lowered.spans))
		for _, span := range lowered.spans {
			ids = append(ids, span.ID)
		}
		out[lowered.wallID] = ids
	}
	return out
}

// canonicalPlacementBounds is the post-conversion safety net every derived
// placement crosses: the same ceiling encounter enforces at construction,
// asked after the source→canonical conversion because both the conversion and
// the authored offsets can push a legal source scalar past it. A wall's
// presence rectangle and every blocking span, and a bound door, all come
// through here so a refusal names the authored item rather than the party's
// start cell.
func canonicalPlacementBounds(p spatial.FootprintPlacement) error {
	for _, value := range [...]float64{
		p.Footprint.Box.W, p.Footprint.Box.D,
		p.Origin.X, p.Origin.Y, p.Facing,
		p.LocalOffset.X, p.LocalOffset.Y,
	} {
		if !wallFinite(value) || math.Abs(value) > maxAuthoredCoord {
			return fmt.Errorf("%s", wallUnrepresentable)
		}
	}
	if p.Footprint.Box.W <= 0 || p.Footprint.Box.D <= 0 {
		return fmt.Errorf("%s", wallUnrepresentable)
	}

	return nil
}

// # Attached doors

// boundDoor is one door attached to a wall opening, resolved to the wall and
// opening that own it and to the author's own path.
type boundDoor struct {
	id      string
	wall    *RoomWall
	opening *RoomWallOpening
	path    string
}

// boundDoorsOf lists every attached door this room carries, in stable id
// order. One place the door path, the lowering and the concealment universe
// all read, so they cannot come to disagree about which opening owns an id.
//
// A door the author attached without an id is skipped rather than keyed under
// the empty string: the value walk already refused it, and no caller here has
// an honest answer for a door named nothing.
func boundDoorsOf(walls []RoomWall) []boundDoor {
	var out []boundDoor
	for i := range walls {
		wall := &walls[i]
		for j := range wall.Openings {
			o := &wall.Openings[j]
			if o.Door == nil || o.Door.ID == "" {
				continue
			}
			out = append(out, boundDoor{
				id:      o.Door.ID,
				wall:    wall,
				opening: o,
				path:    fmt.Sprintf("room.room.walls[%d].openings[%d].door", i, j),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].id < out[j].id })

	return out
}

// boundDoorIDSet is the universe of attached door ids, for the callers that
// only need membership rather than the owning wall.
func boundDoorIDSet(walls []RoomWall) map[string]bool {
	out := map[string]bool{}
	for _, b := range boundDoorsOf(walls) {
		out[b.id] = true
	}

	return out
}

// attachedDoorDeclaration is the shape a bound door stands as, resolved from
// the opening and the wall it belongs to (Task 5's canonical geometry):
//
//	width   = opening.width
//	depth   = wall.blocker.footprint.depth
//	offsetX = 0            (the opening's centre already sits on the line)
//	offsetZ = wall.blocker.footprint.offsetZ
//
// The two flags are FALSE because a door asserts no blocking of its own — its
// doorBinding state decides what it closes, and the transverse offset keeps the
// door across the same strip as its wall even when the blocker is shifted.
func attachedDoorDeclaration(wall *RoomWall, opening *RoomWallOpening) RoomPropDeclaration {
	assertsNothing := false
	fp := wall.Blocker.Footprint

	return RoomPropDeclaration{
		BlocksMovement:    &assertsNothing,
		BlocksLineOfSight: &assertsNothing,
		Footprint: RoomFootprint{
			Width:   opening.Width,
			Depth:   fp.Depth,
			OffsetX: 0,
			OffsetZ: fp.OffsetZ,
		},
	}
}

// attachedDoorPose is the opening's resolved pose in the editor's scene frame:
// the point at `position` along start→end, and the source yaw atan2(-dz, dx)
// the one adapter turns into the canonical facing. The opening owns the pose,
// so nothing here reads a scene item or a prop declaration.
func attachedDoorPose(wall *RoomWall, opening *RoomWallOpening) (scenePose, error) {
	start, end := wall.Line.Start, wall.Line.End
	dx, dz := end.X-start.X, end.Z-start.Z
	length := math.Hypot(dx, dz)
	if !wallFinite(length) || length <= 0 {
		return scenePose{}, fmt.Errorf("line has no finite positive length")
	}

	return scenePose{
		X:         start.X + dx/length*opening.Position,
		Z:         start.Z + dz/length*opening.Position,
		RotationY: math.Atan2(-dz, dx),
	}, nil
}

// wallInterval is a half-open corridor along the wall, measured from its
// start in scene units.
type wallInterval struct{ start, end float64 }

// wallCutIntervals is every opening corridor clamped to the visible line, in
// sorted order — the intervals [subtractIntervals] removes. An opening is a
// gap, and this reads nothing of any door it may carry — only position and
// width.
func wallCutIntervals(openings []RoomWallOpening, length float64) []wallInterval {
	cuts := make([]wallInterval, 0, len(openings))
	for i := range openings {
		a := openings[i].Position - openings[i].Width/2
		b := openings[i].Position + openings[i].Width/2
		if a < 0 {
			a = 0
		}
		if b > length {
			b = length
		}
		if b > a {
			cuts = append(cuts, wallInterval{start: a, end: b})
		}
	}
	sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
	return cuts
}

// subtractIntervals removes sorted disjoint (or touching) corridors from one
// blocker interval, returning the positive spans that remain in sorted order.
// Touching corridors are merged by the walk: a corridor starting exactly where
// the previous ended leaves no phantom span between them.
func subtractIntervals(start, end float64, cuts []wallInterval) []wallInterval {
	var out []wallInterval
	cursor := start
	for _, cut := range cuts {
		if cut.end <= cursor {
			continue
		}
		if cut.start > cursor {
			out = append(out, wallInterval{start: cursor, end: cut.start})
		}
		if cut.end > cursor {
			cursor = cut.end
		}
		if cursor >= end {
			break
		}
	}
	if cursor < end {
		out = append(out, wallInterval{start: cursor, end: end})
	}
	return out
}

// wallCollision refuses a wall presence id or a generated span id that is
// already an authored placed or door identity — or a generated span id that
// is another wall's raw identity — rather than letting [encounter]'s compile
// report a duplicate with no source address.
//
// It runs at the COMPILE, after the authored props are known and beside the
// doors, because the id namespace is shared by construction: a placed prop, a
// door, a wall's presence entry and a generated span all answer the same
// kinds of questions, and two of them under one name is the defect
// [encounter.PlacedPropInput] already names. The refusal is at the WALL's
// path, because the wall is the thing the author can change.
//
// EVERY PRESENCE IDENTITY IS CLAIMED BEFORE ANY SPAN, so a raw wall id that
// is another wall's generated span id is one collision with one honest order
// rather than an accident of authored order.
func wallCollision(existing []encounter.PlacedPropInput, lowerings []wallLowering, doors []encounter.DoorInput) error {
	taken := map[encounter.PropID]string{}
	for _, p := range existing {
		taken[p.ID] = "an authored placed identity"
	}
	for _, d := range doors {
		taken[encounter.PropID(d.ID)] = "an authored door identity"
	}
	for _, lowered := range lowerings {
		if owner, ok := taken[lowered.presence.ID]; ok {
			return singleRoomCompileError(fmt.Sprintf("room.room.walls[%d]", lowered.wallIndex),
				fmt.Sprintf("wall identity %q collides with %s", lowered.presence.ID, owner))
		}
		taken[lowered.presence.ID] = "an authored wall identity"
	}
	for _, lowered := range lowerings {
		for _, span := range lowered.spans {
			if owner, ok := taken[span.ID]; ok {
				return singleRoomCompileError(fmt.Sprintf("room.room.walls[%d]", lowered.wallIndex),
					fmt.Sprintf("generated span id %q collides with %s", span.ID, owner))
			}
			taken[span.ID] = "a generated span identity"
		}
	}
	return nil
}
