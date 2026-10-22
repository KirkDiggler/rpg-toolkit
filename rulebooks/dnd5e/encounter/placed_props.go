// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// placed_props.go is THE PLACED FOOTPRINT FACTS (issue #1753).
//
// A cell prop — the v2 map format's [PropInput] — stands on one cell, and the canvas's entity walk
// answers every question about it. An authored table with a real footprint
// is a rectangle in the plane: its centre may sit on one cell while its
// corners reach three, a thin one sits BETWEEN two cells and blocks the
// crossing while leaving both centres clear, and no cell anywhere holds an
// anchor for it. So it is not an entity and not a cell fact — it is a
// contributor, and this file is the one home its facts have.
//
// One contributor set, [field.placed], compiled once from
// [FieldInput.Placed] by [field.compilePlaced] — validated, deep-copied, and
// owned by the field exactly as regions and props are. Every question below
// reads that one set; standing, crossing, sight, persistence and the atlas
// cannot disagree because there is nothing else to read.
//
// The continuous facts are spatial's, not copies: [spatial.TraceFootprint]
// answers both questions this module asks of a placed shape, against the
// canonical plane the field's own orientation names — cell centres five feet
// across the flats ([FeetPerCell]), the frame
// dungeonspec's construction boundary calibrates source scenes to
// (single-room-play.md §3). Stationary contact decides standing; a segment's
// interior decides a crossing. No rasterization, no second hex walk, no
// new geometry.
//
// Sight reads the SAME set through [spatial.SightLanes], which owns the lane
// algorithm; this module only supplies facts (see [canvasRoom]
// .IsLineOfSightBlocked). Footprint props are SOFT obstructions there — a
// thin blocker leans around like an occluding entity does, never like a wall
// — per the approved sight-lanes contract.
//
// # The second kind of contributor: a door (rpg-project#485, R1)
//
// A door that stands as a FOOTPRINT rather than in a crossing is measured by
// the same four queries, with one difference and only one: what it blocks is
// its [DoorState]'s answer, asked per read, rather than two flags fixed at
// compile. Closed and locked block movement and sight; open blocks neither,
// and is skipped before any geometry is traced.
//
// So there is no second geometry and no second algorithm — the rectangle,
// the plane, the contact rule and the interior rule are the ones above. The
// door is held as the RECORD ([field.doorFootprints]), so
// [Encounter.OpenDoor] writing one state is the whole of "the door opened":
// nothing here is rebuilt, invalidated or re-registered, and no snapshot of
// the state exists to fall behind. That is why a shut door and its own open
// self cannot disagree.

// PlacedPropInput is one authored footprint placement on the field: a named,
// canonical rectangle with the two blocking answers its author gave it.
//
// CANONICAL means the geometry is already in the spatial plane the canvas
// runs on: feet, with the footprint's box measured along/across its facing
// and its origin at a plane point. The conversion from the portable source
// frame (editor units, X/Z plane, Three yaw radians) is the construction
// boundary's — dungeonspec owns it, and no second authored pose exists on
// this side of it. This module never converts, never re-scales, and never
// asks where the rectangle's anchor cell is, because it has none.
//
// The two flags are INDEPENDENT, all four combinations real content —
// the same law [PropInput]'s blocking answers follow (rpg-toolkit#1128).
// BlocksMovement governs the two movement facts (centre-covered standing,
// interior crossing); BlocksLineOfSight governs the two sight facts (lane
// obstruction, opaque alternate origin). Neither implies the other, and
// neither is defaulted.
//
// NOT A CELL PROP, and the line moved once (rpg-toolkit#1854). A placed
// footprint CAN be held and CAN arrive from reserve — the three fields
// below are [PropInput]'s own, with [PropInput]'s own meanings, because
// "what a placed thing does" is one protocol and two of them would be two
// answers. What stays true is the rest of the sentence: a placed footprint
// carries no content ref and no [PropInput.Facing] word, and a changed or
// removed placement is a RECOMPILATION, not a runtime verb. Cell props
// keep their own path untouched; the two lists never share an ID.
//
// # Where a footprint STANDS, since it has no anchor cell
//
// Three protocols need a cell for a thing whose geometry is a rectangle in
// the plane: reach, the probe law's visibility gate, and the cell an
// arrival fact names. [field.placedCells] is the one answer all three ask,
// and it derives rather than authors — see its own comment for the rule and
// why nearest-centre is exact rather than approximate.
type PlacedPropInput struct {
	// ID names the placement. REQUIRED non-empty and unique — among placed
	// contributors and against every cell [PropInput.ID], because a cell
	// fold and an atlas would otherwise have two answers for one name.
	ID PropID

	// Placement is the canonical geometry: the footprint box in feet, where
	// it sits, which way it faces (degrees), and the box's offset inside the
	// placement's own axes. Copied at construction — a caller editing their
	// box pointer afterwards must not change a running field.
	Placement spatial.FootprintPlacement

	// BlocksMovement is whether this placement closes cells it covers and
	// segments it lies across. Explicit, never inferred from the geometry.
	BlocksMovement bool

	// BlocksLineOfSight is whether this placement obstructs sight — as a
	// SOFT lane obstruction (see [canvasRoom.IsLineOfSightBlocked]), subject
	// to the same lean-around rule as an occluding entity.
	BlocksLineOfSight bool

	// Holdable is whether a member can pick this placement up
	// ([PropInput.Holdable], rpg-toolkit#1854). Optional, and FALSE IS THE
	// HONEST ZERO VALUE for [PropInput.Holdable]'s reason: a thing nobody
	// declared holdable stays scenery, which is what every placed footprint
	// was before this field existed.
	//
	// TAKING IT IS THE CELL PROP'S REACH RULE, DERIVED (rpg-project#488 R1).
	// [Encounter.Hold] measures [refuseOutOfReachCell] — grid distance from
	// the member's cell, Range 0 meaning adjacent — against every cell this
	// placement stands on ([field.placedCells]). Standing on one of them is
	// distance zero and in reach; being next to one is distance one and in
	// reach. No second threshold exists for a footprint, and none is
	// authored.
	//
	// A HELD PLACEMENT IS OFF THE FLOOR FOR EVERYONE: its rectangle blocks
	// no step, closes no crossing, obstructs no lane and is absent from
	// [Atlas.Placed], folded from the same `held:` fact a cell prop's
	// disappearance folds from (holdings.go). Dropping it puts the
	// rectangle back with its origin on the cell it was dropped on.
	Holdable bool

	// Holds is the intel records this placement carries, by record id
	// ([PropInput.Holds], rpg-project#372 R6). Optional; omitted means none.
	//
	// [PropInput.Holds]' rule verbatim, because it is the same rule: the
	// records are CONSTRUCTION TRUTH and they STAY WITH THE THING, so a
	// scroll handed on or dropped and picked up again teaches the next
	// holder too. A record this field does not declare is refused at
	// construction (ErrNoIntel), and a placement that carries records need
	// not be Holdable — inert, not an error.
	Holds []IntelID

	// Arrives is the predicate that brings this placement onto the floor
	// ([PropInput.Arrives], rpg-project#375). Nil — the zero value, and
	// every placement authored before arrivals existed — means it is there
	// from the first frame.
	//
	// A PLACEMENT WITH A PREDICATE IS IN RESERVE, and the never-authored
	// yardstick governs it exactly as it governs a reserved cell prop: its
	// rectangle blocks nothing, closes nothing, obstructs nothing, is absent
	// from [Atlas.Placed], and [Encounter.Hold] refuses it as an id that
	// names nothing (the probe law). When the predicate holds it is placed
	// WHERE THE AUTHOR DREW IT — a rectangle needs no free cell to stand on
	// and a footprint needs no floor, so nothing searches for room the way
	// [Encounter.arrivalCell] does for a prop or a member — with the same
	// `arrived:<id>@<cell>` fact and the same `arrived` beat every arrival
	// writes. The cell in that fact is [field.placedCells]' first, which is
	// where the thing stands.
	//
	// Refused at construction when it can never hold (ErrNoField), by the
	// liveness rule [field.validatePropArrivals] applies to every prop.
	Arrives Trigger

	// Options is the lines a character can say to this placement —
	// [PropInput.Options]' field with [PropInput.Options]' rules
	// (rpg-project#569).
	Options []Option
}

// placedContributor is one compiled placed prop: the input's facts, deep
// copied, in the CANONICAL frame — construction truth, what ToData writes
// back out. Box is copied on purpose: the input's own pointer must be able
// to mutate without reaching a running field.
type placedContributor struct {
	id                PropID
	placement         spatial.FootprintPlacement
	blocksMovement    bool
	blocksLineOfSight bool

	// reach is the placement's axis-aligned bounds relative to its origin,
	// measured once here so a ray or crossing far from it skips the trace
	// (rpg-toolkit#2049). Zero value traces everything.
	reach footprintBounds

	// holdable, holds and arrives are the three orders a placement can
	// carry (rpg-toolkit#1854), deep-copied like everything else here.
	holdable bool
	holds    []IntelID
	arrives  Trigger

	// options is the lines a character can say to it, deep-copied
	// ([PlacedPropInput.Options]).
	options []Option
}

// mutable reports whether this contributor can leave the floor or arrive
// onto it — the one question that decides whether a query has to fold the
// journal at all. A placement that is neither holdable nor reserved stands
// where it was compiled for the whole run, so every read of it costs
// exactly what it cost before #1854.
func (p *placedContributor) mutable() bool { return p.holdable || p.arrives != nil }

// placedPlaneCellWidth is the across-flats width, in feet, of the plane the
// placed facts are measured in: every cell is [FeetPerCell] across, the one
// real-world scale this composition already speaks (see [FeetPerCell]).
const placedPlaneCellWidth = FeetPerCell

// compilePlaced validates the authored placements and compiles the one
// contributor set, refusing every defect by name.
//
// A footprint needs NO floor. A table may overhang the void, a bridge may
// span it, and sight keeps the parts of a shape the painted mask does not
// cover — so placement is never checked against [field.isFloor], and nothing
// here invents floor from a footprint (the approved contract's own wording).
// What is refused is a placement this module cannot measure honestly: a
// nameless or duplicated contributor, a missing box, a non-finite or
// non-positive one, or coordinates at magnitudes no field cell can reach —
// the same pure-overflow defense [maxAnchorCoord] gives the authored cells,
// so a trace downstream can never wrap into a wrong answer the way an
// extreme integer once did (see [cubeDistance]'s saturating lesson in
// dungeonspec).
func (f *field) compilePlaced(in []PlacedPropInput) error {
	if len(in) == 0 {
		return nil
	}
	f.placed = make([]placedContributor, len(in))
	seenID := make(map[PropID]bool, len(in))

	for i, p := range in {
		if p.ID == "" {
			return fmt.Errorf("placed[%d] has no id: %w", i, ErrNoField)
		}
		if seenID[p.ID] {
			return fmt.Errorf("duplicate placed prop %q: %w", p.ID, ErrNoField)
		}
		// ONE NAME, ONE THING. A cell prop's id names a thing the verbs and
		// the atlas address; a placed contributor reusing it would give a Hold
		// and a cell fold two answers.
		for _, cell := range f.props {
			if cell.ID == p.ID {
				return fmt.Errorf("placed prop %q shares its id with a cell prop: %w", p.ID, ErrNoField)
			}
		}
		if err := validatePlacement(p.Placement); err != nil {
			return fmt.Errorf("placed prop %q: %w", p.ID, err)
		}

		// Deep copy: the box is the one pointer in the placement, and a
		// caller flipping its width after construction must not change what
		// ToData writes or what a running fold measures.
		box := *p.Placement.Footprint.Box
		placement := spatial.FootprintPlacement{
			Footprint:   spatial.Footprint{Box: &box},
			Origin:      p.Placement.Origin,
			Facing:      p.Placement.Facing,
			LocalOffset: p.Placement.LocalOffset,
		}
		f.placed[i] = placedContributor{
			id:                p.ID,
			placement:         placement,
			reach:             boundsOf(placement),
			blocksMovement:    p.BlocksMovement,
			blocksLineOfSight: p.BlocksLineOfSight,
			holdable:          p.Holdable,
			holds:             append([]IntelID(nil), p.Holds...),
			arrives:           p.Arrives,
			options:           cloneOptions(p.Options),
		}
		if err := validatePropOptions(p.ID, string(p.ID), p.Options); err != nil {
			return err
		}
		seenID[p.ID] = true
		// ONE HOLDABLE INDEX, BOTH KINDS (rpg-toolkit#1854, R-D). "Is this
		// a thing that can be picked up" is a question about the FIELD, and
		// [Encounter.Hold] and [validateEndingTriggers] must not have to ask
		// it twice and risk two answers. The id namespace is already shared
		// — the collision above is what makes that safe.
		if p.Holdable {
			f.holdable[p.ID] = true
		}
	}

	return nil
}

// placedIndexOf is the index of the placed contributor with this id, or -1 —
// [field.propIndexOf] for the other list, and linear for its reason.
func (f *field) placedIndexOf(id PropID) int {
	if id == "" {
		return -1
	}
	for i := range f.placed {
		if f.placed[i].id == id {
			return i
		}
	}

	return -1
}

// placedCentre is the plane point at the middle of a placement's rectangle:
// its origin with the box's local offset applied ALONG and ACROSS the facing,
// which is what [spatial.FootprintPlacement.LocalOffset] means by "the
// footprint's own along/across axes, before Facing".
//
// MIRRORED RATHER THAN ASKED, because spatial does not export it: the two
// lines below are `footprintBox`'s own (placed_footprint.go), and the module
// boundary is the only reason they are repeated. A second reading of one
// basis is the defect this workspace has paid for twice
// (rpg-toolkit#1141, #1150), so it is pinned rather than trusted —
// TestAPlacementsCentreIsTheOneSpatialMeasures requires the point this
// returns to be inside the rectangle spatial itself traces, for a placement
// whose ORIGIN is not.
func placedCentre(p spatial.FootprintPlacement) spatial.Point {
	a := math.Mod(p.Facing, 360) * math.Pi / 180
	along := spatial.Point{X: math.Cos(a), Y: math.Sin(a)}
	across := spatial.Point{X: -math.Sin(a), Y: math.Cos(a)}

	return spatial.Point{
		X: p.Origin.X + along.X*p.LocalOffset.X + across.X*p.LocalOffset.Y,
		Y: p.Origin.Y + along.Y*p.LocalOffset.X + across.Y*p.LocalOffset.Y,
	}
}

// placedCells is WHERE A FOOTPRINT STANDS, in cells — the one derivation
// three protocols ask (rpg-toolkit#1854): [Encounter.Hold]'s reach, the probe
// law's visibility gate, and the cell an arrival fact names.
//
// THE UNION OF TWO THINGS, with no branch between them:
//
//  1. every cell of this field whose centre the rectangle covers —
//     stationary [spatial.TraceFootprint] contact, the module's ONE standing
//     query ([field.standingBlocks], [Encounter.CellAt],
//     [ValidateStaticPlacements]). A table spanning three cells stands on
//     three;
//  2. the one cell that contains the rectangle's own CENTRE
//     ([placedCentre]) — not its origin, because a local offset can put the
//     origin outside the box entirely.
//
// THE SECOND IS EXACT, NOT A FALLBACK GUESS. A hex grid's cells are the
// Voronoi cells of their centres, so the nearest centre to a point is the hex
// that point lies in. Nothing is conditional: a hex-sized prop gets the set it
// would have got from clause 1 alone, because its own cell is already covered;
// a prop SMALLER than a hex gets exactly the hex it sits in, which is what a
// table would say about it.
//
// Clause 2 is why this rule exists at all. Most takeable things are smaller
// than a hex — dungeonspec's feetPerSourceUnit makes the World Builder's own
// 0.3-unit letter a 0.87ft square on a 5ft cell, and it covers ZERO of its
// room's nineteen cell centres — so clause 1 alone would leave every such
// thing standing nowhere, reachable from nowhere and seen by nobody.
//
// NEVER EMPTY for a compiled field, because compileRegions refuses a field
// with no cells: no caller needs a "stands nowhere" branch. Returned in the
// field's own cell order, ties in clause 2 broken by [cellBefore] (C8).
//
// A trace error counts as covering, the fail-closed rule every other reader
// of this geometry applies; compilePlaced makes it unreachable.
func (f *field) placedCells(p spatial.FootprintPlacement) []spatial.Position {
	centre := placedCentre(p)
	reach := boundsOf(p)
	stands := make(map[spatial.Position]bool, len(f.cells))

	var own spatial.Position
	var ownD2 float64
	found := false
	for _, cell := range f.cells {
		at := f.plane.CellCentre(cell)
		if reach.mayTouch(p.Origin, at, at) {
			contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
				Placement: p, From: at, To: at,
			})
			if err != nil || contact.Contact {
				stands[cell] = true
			}
		}
		dx, dy := at.X-centre.X, at.Y-centre.Y
		if d2 := dx*dx + dy*dy; !found || d2 < ownD2 || (d2 == ownD2 && cellBefore(cell, own)) {
			found, own, ownD2 = true, cell, d2
		}
	}
	if found {
		stands[own] = true
	}

	out := make([]spatial.Position, 0, len(stands))
	for _, cell := range f.cells {
		if stands[cell] {
			out = append(out, cell)
		}
	}

	return out
}

// footprintObservationCells is a footprint's support for boundary observation:
// the declared floor cells it overlaps by positive area. A thin wall/door can
// border both rooms without covering either cell centre. Its standing/reach
// support (placedCells) remains unchanged and is not a floor-discovery grant.
// Off-floor scenery retains its existing nearest-floor support when there is
// no overlap at all. The query uses spatial's polygon coverage, not another LOS
// evaluator; observation still asks the same sightReach for each support cell.
//
// Only the cells whose square of half-side one cell width (wider than the
// hex's circumradius, so it contains the whole hex) may touch the footprint's
// reach are measured ([footprintBounds], rpg-toolkit#2049): a cell beyond it
// overlaps by zero area, which is exactly what the filter below drops anyway.
func (f *field) footprintObservationCells(p spatial.FootprintPlacement) ([]spatial.Position, error) {
	cells := f.cells
	if reach := boundsOf(p); reach.ok {
		cells = make([]spatial.Position, 0, len(f.cells))
		for _, cell := range f.cells {
			at := f.plane.CellCentre(cell)
			r := float64(placedPlaneCellWidth)
			lo, hi := spatial.Point{X: at.X - r, Y: at.Y - r}, spatial.Point{X: at.X + r, Y: at.Y + r}
			if reach.mayOverlap(p.Origin, lo, hi) {
				cells = append(cells, cell)
			}
		}
	}
	coverage, err := spatial.PlacedCoverage(spatial.PlacedCoverageInput{
		Embedding: f.plane, Placement: p, Cells: cells,
	})
	if err != nil {
		return nil, err
	}
	out := make([]spatial.Position, 0, len(coverage.Cells))
	for _, cell := range f.cells {
		if coverage.Cells[cell] > 0 {
			out = append(out, cell)
		}
	}
	if len(out) == 0 {
		return f.placedCells(p), nil
	}
	return out, nil
}

// placedFold is ONE journal fold per query, taken only if a query actually
// reaches a placement that can move.
//
// A field whose placements are all scenery — every field authored before
// #1854, and every v4 room whose props are furniture — never folds at all,
// so standing, crossing and the sight lanes cost exactly what they cost
// before. A field with one holdable letter folds once per query rather than
// once per contributor, which is the same discipline [holdings.fold]'s own
// "one walk, two answers" comment states.
type placedFold struct {
	f      *field
	folded bool
	at     map[PropID]propPlacement
}

// placedNow starts a fold for one query. Held by value at the call site: it
// is a per-query scratch, never state.
func (f *field) placedNow() placedFold { return placedFold{f: f} }

// stands is where one contributor's rectangle is RIGHT NOW, and whether it
// is on the floor at all.
//
// Three answers, and they are [PropInput.Arrives]' and [Encounter.Hold]'s
// own, asked of a rectangle instead of a cell:
//
//   - In reserve (a predicate that has not held) — not on the floor.
//   - Held (a `held:` fact with no later `dropped:`) — not on the floor.
//   - Dropped — on the floor with its ORIGIN moved to the cell it was
//     dropped on, shape and facing untouched. That is what picking a thing
//     up and putting it down somewhere means for a rectangle, and it is the
//     one place a placement's geometry is not construction truth.
//
// An ARRIVAL does not move it: the author drew where it stands, a footprint
// needs no free cell, and the arrival's own cell is the fact's spelling
// rather than a new pose ([PlacedPropInput.Arrives]).
//
// THE FIRST LINE DECIDES NOTHING, and is said out loud rather than left for
// the next reader to test. A contributor that is neither holdable nor
// reserved has no `held:`, `dropped:` or `arrived:` fact to its name — only a
// holdable thing can be taken, and only a taken thing can be dropped — so
// every branch below would fall through to the same answer for it. Deleting
// the guard changes no result and no test kills it; what it saves is the
// journal walk, which is why a field of plain furniture costs exactly what it
// cost before #1854.
func (pf *placedFold) stands(p *placedContributor) (spatial.FootprintPlacement, bool) {
	if !p.mutable() {
		return p.placement, true
	}
	if !pf.folded {
		pf.folded = true
		if pf.f.holdings != nil {
			pf.at = pf.f.holdings.propPlacements()
		}
	}
	state := pf.at[p.id]
	if p.arrives != nil && !state.arrived {
		return spatial.FootprintPlacement{}, false
	}
	if state.gone {
		return spatial.FootprintPlacement{}, false
	}
	if state.dropped {
		moved := p.placement
		moved.Origin = pf.f.plane.CellCentre(state.at)

		return moved, true
	}

	return p.placement, true
}

// validatePlacement refuses a placement whose geometry is missing,
// non-finite, non-positive, or beyond every representable field — the
// construction-time guard that keeps the trace seams' fail-closed paths
// unreachable for a compiled field. Checked in one fixed order (C8): the
// same broken placement always earns the same sentence.
func validatePlacement(p spatial.FootprintPlacement) error {
	if p.Footprint.Box == nil {
		return fmt.Errorf("placement has no box footprint: %w", ErrNoField)
	}
	for _, v := range []struct {
		name string
		val  float64
	}{
		{"footprint width", p.Footprint.Box.W}, {"footprint depth", p.Footprint.Box.D},
		{"origin x", p.Origin.X}, {"origin y", p.Origin.Y},
		{"facing", p.Facing}, {"local offset x", p.LocalOffset.X}, {"local offset y", p.LocalOffset.Y},
	} {
		if math.IsNaN(v.val) || math.IsInf(v.val, 0) {
			return fmt.Errorf("placement %s is not finite: %w", v.name, ErrNoField)
		}
		if math.Abs(v.val) > maxAnchorCoord {
			return fmt.Errorf("placement %s is beyond any representable field coordinate: %w", v.name, ErrNoField)
		}
	}
	if p.Footprint.Box.W <= 0 || p.Footprint.Box.D <= 0 {
		return fmt.Errorf("placement footprint sides must be positive: %w", ErrNoField)
	}

	// Validate through spatial's released geometry path as well as the basic
	// scalar checks above. Extremely small finite boxes can underflow during
	// polygon measurement, and admitting one would defer an unmeasurable
	// placement error to every runtime trace.
	if _, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
		Placement: p,
		From:      p.Origin,
		To:        p.Origin,
	}); err != nil {
		return fmt.Errorf("placement geometry cannot be measured: %w; %w", ErrNoField, err)
	}

	return nil
}

// attachDoorFootprints holds the doors that stand as rectangles, so every
// query below can ask one what it blocks right now (rpg-project#485, R1).
//
// THE RECORDS THEMSELVES, not copies of their state. A door's state is the
// one thing about it a verb changes mid-scene, and a snapshot taken here
// would be the second answer [Encounter.OpenDoor] then had to remember to
// update — the exact duality [DoorEdge] has no flags in order to avoid.
func (f *field) attachDoorFootprints(doors []*doorRecord) {
	f.doorFootprints = nil
	for _, d := range doors {
		if d.placement != nil {
			f.doorFootprints = append(f.doorFootprints, d)
		}
	}
}

// blockingDoorFootprints is every footprint door whose state blocks right
// now — closed or locked. An open one is skipped entirely: it is the gap
// that was there before the door existed, exactly as an open edge door's
// crossings block nothing.
func (f *field) blockingDoorFootprints() []*doorRecord {
	if len(f.doorFootprints) == 0 {
		return nil
	}
	out := make([]*doorRecord, 0, len(f.doorFootprints))
	for _, d := range f.doorFootprints {
		if d.blocks() {
			out = append(out, d)
		}
	}

	return out
}

// standingBlocks answers CENTRE-COVERED STANDING: which movement-blocking
// placed contributor, if any, has the queried cell's centre on or inside its
// rectangle. Stationary [spatial.TraceFootprint] contact — the published
// standing query — including the rectangle boundary. The FIRST contributor
// in compiled order that covers the centre is named; a caller listing
// contributors rather than refusing asks the CellAt fold, which reports
// every one.
//
// A SHUT FOOTPRINT DOOR IS ONE OF THEM, asked last so an authored prop keeps
// the sentence it already had when both cover a cell. Its id is the door's,
// which is what [Encounter.blockedBy] needs to say a door is in the way
// rather than a table.
//
// A trace error cannot occur for a compiled field (compilePlaced refuses
// non-finite geometry), and the movement fold treats the day one somehow
// arises as blocked — fail closed, the rule spatial's own sight boolean
// applies to the same class of impossible answer.
func (f *field) standingBlocks(cell spatial.Position) (PropID, bool) {
	centre := f.plane.CellCentre(cell)
	now := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksMovement {
			continue
		}
		// A PLACEMENT THAT IS NOT ON THE FLOOR CLOSES NOTHING
		// (rpg-toolkit#1854): one waiting in reserve and one somebody is
		// carrying are both absent, exactly as a reserved cell prop is off
		// the canvas and a held one is out of every atlas.
		placement, standing := now.stands(p)
		if !standing || !p.reach.mayTouch(placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: placement, From: centre, To: centre,
		})
		if err != nil || contact.Contact {
			return p.id, true
		}
	}
	for _, d := range f.blockingDoorFootprints() {
		if !d.reach.mayTouch(d.placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: centre, To: centre,
		})
		if err != nil || contact.Contact {
			return d.id, true
		}
	}

	return "", false
}

// doorStandingOn is the shut footprint door covering a cell's centre, or
// nil. The same stationary contact [field.standingBlocks] measures, asked
// for the door itself rather than its id — what [Encounter.stepMember]
// needs to refuse a step with the door's own sentence and sentinel.
func (f *field) doorStandingOn(cell spatial.Position) *doorRecord {
	centre := f.plane.CellCentre(cell)
	for _, d := range f.blockingDoorFootprints() {
		if !d.reach.mayTouch(d.placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: centre, To: centre,
		})
		if err != nil || contact.Contact {
			return d
		}
	}

	return nil
}

// doorAcrossCrossing is the shut footprint door lying across the straight
// crossing between two cells, or nil — [field.crossingBlocks]' interior
// test, asked for the door rather than its id.
func (f *field) doorAcrossCrossing(from, to spatial.Position) *doorRecord {
	for _, d := range f.blockingDoorFootprints() {
		a, b := f.plane.CellCentre(from), f.plane.CellCentre(to)
		if !d.reach.mayTouch(d.placement.Origin, a, b) {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: a, To: b,
		})
		if err != nil || trace.Interior {
			return d
		}
	}

	return nil
}

// doorCrossings is every crossing between two adjacent floor cells that a
// footprint door's rectangle lies across, whatever the door's state: the
// trace [field.doorAcrossCrossing] asks of a shut door before a step, asked
// here of the door's construction geometry. Each crossing is named once,
// earlier cell first, in floor order (C8). An edge door, which has no
// rectangle, has none.
//
// GEOMETRY, NOT STATE: an open door still stands between the two rooms it
// joins, which is what the hearing graph ([Encounter.links]) needs — whether
// it is shut is read live, per sound. The bounds pre-filter skips every
// crossing the rectangle cannot reach, so this is one trace per crossing
// near the door, once per encounter.
func (f *field) doorCrossings(d *doorRecord) [][2]spatial.Position {
	if d.placement == nil {
		return nil
	}
	var out [][2]spatial.Position
	for _, cell := range f.cells {
		a := f.plane.CellCentre(cell)
		for _, next := range adjacencyGrid.GetNeighbors(cell) {
			if !cellBefore(cell, next) || !f.isFloor(next) {
				continue
			}
			b := f.plane.CellCentre(next)
			if !d.reach.mayTouch(d.placement.Origin, a, b) {
				continue
			}
			trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
				Placement: *d.placement, From: a, To: b,
			})
			if err == nil && trace.Interior {
				out = append(out, [2]spatial.Position{cell, next})
			}
		}
	}

	return out
}

// crossingBlocks answers INTERIOR CROSSING: which movement-blocking placed
// contributor, if any, closes the straight crossing between two cells'
// centres. A thin segment between two clear centres is exactly the case a
// cell fold cannot see and this query exists for — the trace's Interior,
// not its Contact, so standing on the very edge of a shape is not crossing
// it. from == to is a zero-length crossing and traces as none.
//
// Errors propagate: the trace is arithmetic about placed geometry, and a
// caller deciding a step refuses rather than guesses (the bool seams below
// fail closed for the same reason).
func (f *field) crossingBlocks(from, to spatial.Position) (PropID, bool, error) {
	now := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksMovement {
			continue
		}
		placement, standing := now.stands(p)
		if !standing {
			continue
		}
		a, b := f.plane.CellCentre(from), f.plane.CellCentre(to)
		if !p.reach.mayTouch(placement.Origin, a, b) {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: placement, From: a, To: b,
		})
		if err != nil {
			return p.id, true, fmt.Errorf("placed prop %q: %w", p.id, err)
		}
		if trace.Interior {
			return p.id, true, nil
		}
	}
	// AND THE SHUT DOORS THAT STAND AS RECTANGLES. A closed door's leaf lies
	// between two cells exactly as a thin table does, and this is the read
	// that sees it: the canvas registered no boundary for it, so without
	// this the step would walk through the door.
	for _, d := range f.blockingDoorFootprints() {
		a, b := f.plane.CellCentre(from), f.plane.CellCentre(to)
		if !d.reach.mayTouch(d.placement.Origin, a, b) {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: a, To: b,
		})
		if err != nil {
			return d.id, true, fmt.Errorf("door %q: %w", d.id, err)
		}
		if trace.Interior {
			return d.id, true, nil
		}
	}

	return "", false, nil
}

// sightBlocksAlong reports whether any sight-blocking placed contributor's
// INTERIOR covers the straight lane between two cells' centres — the SOFT
// lane fact [spatial.SightLanes] may lean around through a clear connection.
// Endpoints are not excluded: a lane from or to an interior point crosses
// the shape. Alternate origins cannot escape that opacity, because their
// connecting lane to the original endpoint must also be clear. This is a
// sight query, independent of whether the footprint blocks movement.
func (f *field) sightBlocksAlong(from, to spatial.Position) (bool, error) {
	return f.sightBlocksAlongExcept(from, to, nil, nil)
}

// sightBlocksAlongExcept is [field.sightBlocksAlong] with some contributors
// left out of the question: the ones [field.screensOf] says cover a cell's
// centre, when the question is whether the FACE of that cover can be seen
// from a neighbour ([sightReach.seesCell]). One loop for both, so the two
// reads cannot disagree about any contributor that is asked.
func (f *field) sightBlocksAlongExcept(
	from, to spatial.Position, skipPlaced map[PropID]bool, skipDoors map[DoorID]bool,
) (bool, error) {
	now := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksLineOfSight || skipPlaced[p.id] {
			continue
		}
		placement, standing := now.stands(p)
		if !standing {
			continue
		}
		a, b := f.plane.CellCentre(from), f.plane.CellCentre(to)
		if !p.reach.mayTouch(placement.Origin, a, b) {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: placement, From: a, To: b,
		})
		if err != nil {
			return false, fmt.Errorf("placed prop %q: %w", p.id, err)
		}
		if trace.Interior {
			return true, nil
		}
	}
	// A SHUT DOOR BLOCKS SIGHT AS WELL AS MOVEMENT, whichever geometry it
	// stands in: an edge door's crossings are registered blocking both, and
	// a footprint door's rectangle is read here. One state, both facts.
	for _, d := range f.blockingDoorFootprints() {
		if skipDoors[d.id] {
			continue
		}
		a, b := f.plane.CellCentre(from), f.plane.CellCentre(to)
		if !d.reach.mayTouch(d.placement.Origin, a, b) {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: a, To: b,
		})
		if err != nil {
			return false, fmt.Errorf("door %q: %w", d.id, err)
		}
		if trace.Interior {
			return true, nil
		}
	}

	return false, nil
}

// screensOf names every sight-blocking contributor covering a cell's centre
// — the same stationary-contact test [field.sightBlocksOriginAt] answers yes
// or no to, here returning which ones. Nil maps mean nothing screens it.
func (f *field) screensOf(cell spatial.Position) (map[PropID]bool, map[DoorID]bool, error) {
	var placed map[PropID]bool
	var doors map[DoorID]bool
	if len(f.placed) == 0 && len(f.doorFootprints) == 0 {
		return placed, doors, nil
	}
	centre := f.plane.CellCentre(cell)
	now := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksLineOfSight {
			continue
		}
		placement, standing := now.stands(p)
		if !standing || !p.reach.mayTouch(placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: placement, From: centre, To: centre,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("placed prop %q: %w", p.id, err)
		}
		if contact.Contact {
			if placed == nil {
				placed = map[PropID]bool{}
			}
			placed[p.id] = true
		}
	}
	for _, d := range f.blockingDoorFootprints() {
		if !d.reach.mayTouch(d.placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: centre, To: centre,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("door %q: %w", d.id, err)
		}
		if contact.Contact {
			if doors == nil {
				doors = map[DoorID]bool{}
			}
			doors[d.id] = true
		}
	}

	return placed, doors, nil
}

// sightBlocksOriginAt reports whether a cell's centre is COVERED by any
// sight-blocking placed contributor — stationary contact, boundary included.
// SightLanes asks this of each candidate alternate origin and skips the ones
// it marks, which is how a covered cell stops being a lane somebody leans
// through.
func (f *field) sightBlocksOriginAt(cell spatial.Position) (bool, error) {
	if f.sightPass == nil {
		return f.computeSightBlocksOriginAt(cell)
	}
	if covered, ok := f.sightPass.originOpaque[cell]; ok {
		return covered, nil
	}
	covered, err := f.computeSightBlocksOriginAt(cell)
	if err != nil {
		return false, err
	}
	f.sightPass.originOpaque[cell] = covered

	return covered, nil
}

// computeSightBlocksOriginAt is [field.sightBlocksOriginAt] without the
// pass memo: one answer, counted on originOpacityComputed.
func (f *field) computeSightBlocksOriginAt(cell spatial.Position) (bool, error) {
	f.originOpacityComputed++
	centre := f.plane.CellCentre(cell)
	now := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksLineOfSight {
			continue
		}
		placement, standing := now.stands(p)
		if !standing || !p.reach.mayTouch(placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: placement, From: centre, To: centre,
		})
		if err != nil {
			return false, fmt.Errorf("placed prop %q: %w", p.id, err)
		}
		if contact.Contact {
			return true, nil
		}
	}
	for _, d := range f.blockingDoorFootprints() {
		if !d.reach.mayTouch(d.placement.Origin, centre, centre) {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement, From: centre, To: centre,
		})
		if err != nil {
			return false, fmt.Errorf("door %q: %w", d.id, err)
		}
		if contact.Contact {
			return true, nil
		}
	}

	return false, nil
}

// crossingBlocked folds the two facts a DIRECT crossing between two cells is
// judged on: a movement-blocking boundary the canvas registered (walls,
// shut doors — the canvas's own answer, still the thing that decides doors),
// or a placed footprint's interior ([field.crossingBlocks]).
//
// ONE FOLD, three readers — [Encounter.stepMember], the route flood
// ([Encounter.floodFrom]) and the directive walk — so a route, a step and a
// directed walk cannot disagree about what a thin blocker closes
// (rpg-toolkit#1652's lesson, kept for the second kind of crossing). A trace
// error fails every reader closed, the same rule the bool seams it feeds
// apply; for a compiled field it cannot arise.
func (e *Encounter) crossingBlocked(from, to spatial.Position) (PropID, bool, error) {
	if e.canvas.IsBoundaryMovementBlocked(from, to) {
		return "", true, nil
	}

	return e.field.crossingBlocks(from, to)
}

// attachHoldings gives the field the run's holdings reader, so the four
// placed-geometry queries can ask where each rectangle is right now
// (rpg-toolkit#1854).
//
// THE READER, NOT A SNAPSHOT — [field.attachDoorFootprints]' rule for the
// second kind of thing that changes mid-scene. Called by both construction
// seams once the journal exists; a field compiled without a run keeps nil
// here and reads the authored geometry, which is what a field nobody is
// playing is.
func (f *field) attachHoldings(h *holdings) { f.holdings = h }

// sightPassMemo is ONE SIGHT PASS'S SHARED WORK (rpg-toolkit#2097): answers
// that depend only on the field's geometry as it stands, which no observer's
// part of the pass changes, so the pass computes each once and every
// observer reads it.
//
//   - originOpaque: whether a sight-blocking footprint covers a cell's
//     centre ([field.sightBlocksOriginAt]). Every lane of every observer's
//     sight test asks it of the cells round both ends of the lane, so the
//     same cells are asked again and again within one pass.
//   - placedCells and observationCells: where each standing placement
//     stands, and its observation support ([field.placedCells],
//     [field.footprintObservationCells]), keyed by placement id. Every
//     observer who learns cells builds its atlas before and after, and each
//     build walked every placement over the whole floor.
//
// IT LIVES FOR ONE PASS AND NO LONGER. Doors open and shut, props are picked
// up and dropped, and members move between passes, never inside one, so a
// memo that outlived its pass would answer from a field that is gone.
// [field.beginSightPass] starts it and its returned func drops it.
type sightPassMemo struct {
	originOpaque     map[spatial.Position]bool
	placedCells      map[PropID][]spatial.Position
	observationCells map[PropID][]spatial.Position
}

// beginSightPass starts the pass memo and returns the func that drops it.
// A pass begun inside a pass shares the outer pass's memo, and its func
// drops nothing: the outer pass ends it.
func (f *field) beginSightPass() (end func()) {
	if f.sightPass != nil {
		return func() {}
	}
	f.sightPass = &sightPassMemo{
		originOpaque:     map[spatial.Position]bool{},
		placedCells:      map[PropID][]spatial.Position{},
		observationCells: map[PropID][]spatial.Position{},
	}

	return func() { f.sightPass = nil }
}

// placedCellsOf is [field.placedCells] for the standing placement with id,
// read from the pass memo when a pass is under way. The returned slice is
// the caller's own.
func (f *field) placedCellsOf(id PropID, p spatial.FootprintPlacement) []spatial.Position {
	if f.sightPass == nil {
		f.placedCellsComputed++
		return f.placedCells(p)
	}
	cells, ok := f.sightPass.placedCells[id]
	if !ok {
		f.placedCellsComputed++
		cells = f.placedCells(p)
		f.sightPass.placedCells[id] = cells
	}

	return append([]spatial.Position(nil), cells...)
}

// observationCellsOf is [field.footprintObservationCells] for the standing
// placement with id, read from the pass memo when a pass is under way. The
// returned slice is read-only: callers range over it.
func (f *field) observationCellsOf(id PropID, p spatial.FootprintPlacement) ([]spatial.Position, error) {
	if f.sightPass == nil {
		return f.footprintObservationCells(p)
	}
	if cells, ok := f.sightPass.observationCells[id]; ok {
		return cells, nil
	}
	cells, err := f.footprintObservationCells(p)
	if err != nil {
		return nil, err
	}
	f.sightPass.observationCells[id] = cells

	return cells, nil
}
