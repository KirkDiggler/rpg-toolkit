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
// A legacy [PropInput] stands on one cell, and the canvas's entity walk
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
// NOT A LEGACY PROP. A placed footprint is not holdable, does not arrive
// from reserve, carries no content ref and no [PropInput.Facing] word:
// none of those protocols exist for it in this slice, and a changed or
// removed placement is a RECOMPILATION, not a runtime verb. Legacy props
// keep their own path untouched; the two lists never share an ID.
type PlacedPropInput struct {
	// ID names the placement. REQUIRED non-empty and unique — among placed
	// contributors and against every legacy [PropInput.ID], because a cell
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
}

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
		// ONE NAME, ONE THING. A legacy prop's id names a thing the verbs and
		// the atlas address; a placed contributor reusing it would give a Hold
		// and a cell fold two answers.
		for _, legacy := range f.props {
			if legacy.ID == p.ID {
				return fmt.Errorf("placed prop %q shares its id with a legacy prop: %w", p.ID, ErrNoField)
			}
		}
		if err := validatePlacement(p.Placement); err != nil {
			return fmt.Errorf("placed prop %q: %w", p.ID, err)
		}

		// Deep copy: the box is the one pointer in the placement, and a
		// caller flipping its width after construction must not change what
		// ToData writes or what a running fold measures.
		box := *p.Placement.Footprint.Box
		f.placed[i] = placedContributor{
			id: p.ID,
			placement: spatial.FootprintPlacement{
				Footprint:   spatial.Footprint{Box: &box},
				Origin:      p.Placement.Origin,
				Facing:      p.Placement.Facing,
				LocalOffset: p.Placement.LocalOffset,
			},
			blocksMovement:    p.BlocksMovement,
			blocksLineOfSight: p.BlocksLineOfSight,
		}
		seenID[p.ID] = true
	}

	return nil
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
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksMovement {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: p.placement, From: centre, To: centre,
		})
		if err != nil || contact.Contact {
			return p.id, true
		}
	}
	for _, d := range f.blockingDoorFootprints() {
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
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement,
			From:      f.plane.CellCentre(from),
			To:        f.plane.CellCentre(to),
		})
		if err != nil || trace.Interior {
			return d
		}
	}

	return nil
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
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksMovement {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: p.placement,
			From:      f.plane.CellCentre(from),
			To:        f.plane.CellCentre(to),
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
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement,
			From:      f.plane.CellCentre(from),
			To:        f.plane.CellCentre(to),
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
// lane fact [spatial.SightLanes] leans around, never a wall. Endpoints are
// not excluded here: a lane FROM a covered origin starts inside the shape,
// and SightLanes routes around it through the origin-exclusion rule its own
// contract names ("At excludes origins with Contact").
func (f *field) sightBlocksAlong(from, to spatial.Position) (bool, error) {
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksLineOfSight {
			continue
		}
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: p.placement,
			From:      f.plane.CellCentre(from),
			To:        f.plane.CellCentre(to),
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
		trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: *d.placement,
			From:      f.plane.CellCentre(from),
			To:        f.plane.CellCentre(to),
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

// sightBlocksOriginAt reports whether a cell's centre is COVERED by any
// sight-blocking placed contributor — stationary contact, boundary included.
// SightLanes asks this of each candidate alternate origin and skips the ones
// it marks, which is how a covered cell stops being a lane somebody leans
// through.
func (f *field) sightBlocksOriginAt(cell spatial.Position) (bool, error) {
	centre := f.plane.CellCentre(cell)
	for i := range f.placed {
		p := &f.placed[i]
		if !p.blocksLineOfSight {
			continue
		}
		contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
			Placement: p.placement, From: centre, To: centre,
		})
		if err != nil {
			return false, fmt.Errorf("placed prop %q: %w", p.id, err)
		}
		if contact.Contact {
			return true, nil
		}
	}
	for _, d := range f.blockingDoorFootprints() {
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
