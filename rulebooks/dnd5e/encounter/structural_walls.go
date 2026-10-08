// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// structural_walls.go is A STRUCTURAL WALL AS THE RUNTIME CARRIES IT
// (rpg-project#169; the parent-owned promotion contract): a line in the
// canonical feet frame, the assembled appearance dimensions, and the openings
// cut into it — one of which may bind an existing footprint door.
//
// # Why this is a second noun beside the placed spans
//
// The placed spans are the MECHANICAL truth: a wall lowers to a handful of
// blocking rectangles and a client joins by one presence id. That is what the
// connected-sight contract reads and it does not change. What it cannot say is
// the thing the author actually drew: one line with a stable identity, an
// absolute length and an OPENING — a conditional cut whose position is exact
// and whose attached door is an existing gameplay door. [SegmentInput] cannot
// say it either: it carries anonymous fractional endpoints and a height
// multiplier, no stable placed identity and no opening bound to a DoorID.
//
// So this is a FIXED LAYOUT DEFINITION linked to existing world identities, not
// another editable transform for a door and not a second pose the source keeps
// in sync. The source compiler resolves the door's opening endpoints from its
// owning line and hands them across once; nothing here reinterprets a source
// XZ coordinate, reads an asset or a catalog, or derives a rule from [Ref].
// Door STATE stays solely in [DoorInput] and the runtime's own state.
//
// # What the encounter validates, and what it only carries
//
// The geometry is validated here — it is compiled construction truth, exactly
// like a placed rectangle — while [Ref] is carried verbatim and never looked
// up. The two references are checked against members this field already has:
// a wall's [StructuralWallInput.ID] must name a STATIC placed contributor (the
// identity-only presence entry the lowering emits), and a bound door's
// [StructuralDoorBindingInput.PlacedID] must name a static placed contributor
// whose [StructuralDoorBindingInput.DoorID] names an existing footprint door.
// Both are the same fixed facts the field already stores; this file only ties
// them together so the projection can answer without consulting mutable state.

// StructuralWallInput is one authored structural wall: its stable placed
// identity, its opaque content reference, its line and assembled dimensions in
// CANONICAL feet, and the openings cut into it.
//
// CANONICAL means the same frame [PlacedPropInput.Placement] is measured in:
// spatial XY feet, one cell [FeetPerCell] across the flats. From and To are the
// wall's two ends; the length between them is the wall's absolute length. The
// dimensions are the assembled appearance the concept drew — height above the
// floor, thickness across the line, and the base elevation — carried as facts,
// never inspected for a rule.
type StructuralWallInput struct {
	// ID is the existing raw, static wall presence id — the same value as the
	// identity-only [PlacedPropInput.ID] the lowering emits for the wall.
	// REQUIRED non-empty and unique among structural walls; it must name a
	// static placed contributor (refused at construction otherwise).
	ID PropID

	// Ref is content's identifier for this wall's appearance, carried through
	// unread. Optional in the sense that this module never inspects it — the
	// authoring dialect is where a catalog membership is refused.
	Ref string

	// From and To are the wall's two ends in canonical feet. REQUIRED finite,
	// representable and distinct: the line must have a positive length.
	From, To spatial.Point

	// Height is the assembled wall height above its base, in canonical feet.
	// REQUIRED finite, representable and positive.
	Height float64

	// Thickness is the assembled wall thickness across the line, in canonical
	// feet. REQUIRED finite, representable and positive. It is independent of
	// the blocking rectangle's depth: changing the appearance never changes
	// what the wall blocks.
	Thickness float64

	// Elevation is the assembled base height above the floor, in canonical
	// feet. REQUIRED finite and representable; NEGATIVE IS ALLOWED, because a
	// structural line may be sunk below the reference plane.
	Elevation float64

	// Openings are the gaps cut into this wall, in authored order. Optional;
	// omitted means none. Each opening fits the line and no two overlap.
	Openings []StructuralOpeningInput
}

// StructuralOpeningInput is one gap in a structural wall: its centre measured
// along the line from the wall's start, its width, and the optional door bound
// to it. All lengths are canonical feet.
type StructuralOpeningInput struct {
	// ID names this opening. REQUIRED non-empty and unique across every
	// opening in the field; an opening id is what the layout editor joins a
	// gap by.
	ID string

	// Position is the gap's centre, measured along From→To from the wall's
	// start, in canonical feet. REQUIRED finite and representable.
	Position float64

	// Width is the gap's width along the line, in canonical feet. REQUIRED
	// finite, representable and positive.
	Width float64

	// Door binds an existing footprint door to this opening. Nil is an
	// ordinary bare opening; a non-nil binding must resolve (see
	// [StructuralDoorBindingInput]).
	Door *StructuralDoorBindingInput
}

// StructuralDoorBindingInput binds one existing gameplay door to the opening
// it stands in (rpg-project#169, the attached-door contract). It carries no
// state and no independent transform: the opening owns the pose, and the
// door's state remains [DoorInput.State]'s.
type StructuralDoorBindingInput struct {
	// PlacedID is the existing raw, static door presence id — the same value
	// as the nonblocking [PlacedPropInput.ID] the lowering emits for the
	// attached door. REQUIRED non-empty and it must name a static placed
	// contributor.
	PlacedID PropID

	// DoorID is the namespaced gameplay door id the observation and verb
	// paths already use (`<key>/<id>`), and it must name an existing footprint
	// door. REQUIRED non-empty and unique across the field's bindings.
	DoorID DoorID

	// Ref is content's identifier for the door's appearance, carried through
	// unread. Never looked up.
	Ref string

	// From and To are the door's resolved VISUAL opening endpoints in
	// canonical feet. The producing dialect owns resolving the opening's centre
	// and width onto its line; this module carries that resolved pose rather
	// than deriving or cross-checking it against the interval again. Endpoints
	// must be finite, representable and distinct; their distance is visual width.
	From, To spatial.Point
}

// structuralWall is one compiled structural wall: the input facts, deep-copied
// so a caller mutating the input it handed in cannot reach internal state.
type structuralWall struct {
	id        PropID
	ref       string
	from, to  spatial.Point
	height    float64
	thickness float64
	elevation float64
	openings  []structuralOpening
}

// structuralOpening is one compiled opening: its own facts and its optional
// bound door.
type structuralOpening struct {
	id       string
	position float64
	width    float64
	door     *structuralDoorBinding
}

// structuralDoorBinding is one compiled door binding: the two existing
// identities it ties together and the opening endpoints the compiler resolved.
type structuralDoorBinding struct {
	placedID PropID
	doorID   DoorID
	ref      string
	from, to spatial.Point
}

// sortStructuralLayout gives the full and member-scoped reads the same identity
// order independently for walls and doors, including before reveal deltas.
func sortStructuralLayout(out *Atlas) {
	sort.Slice(out.StructuralWalls, func(i, j int) bool { return out.StructuralWalls[i].ID < out.StructuralWalls[j].ID })
	sort.Slice(out.StructuralDoors, func(i, j int) bool { return out.StructuralDoors[i].ID < out.StructuralDoors[j].ID })
}

// structuralNumberBound is the coordinate/dimension ceiling a structural value
// must respect — the same pure-overflow bound [validatePlacement] applies to a
// placed rectangle, reused rather than invented so one extreme scalar has one
// answer.
func structuralRepresentable(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= maxAnchorCoord
}

// compileStructuralWalls validates the authored structural layout and compiles
// its one deep-copied record set. It is INERT on a field that declares none, so
// every field authored before this noun existed compiles byte-identically.
//
// doors is the same authored door list the field carries: a bound DoorID must
// name a footprint door in it. placed contributors are read from the compiled
// field, already validated, because "is this a static placement" is the
// contributor set's own answer.
//
// Validation order is first-failure-wins (R5): identities, then the wall's own
// line and dimensions, then each opening, then the bound door. Nothing is
// written to the field until the whole list validates, so a refusal produces no
// partial output.
func (f *field) compileStructuralWalls(in []StructuralWallInput, doors []DoorInput) error {
	if len(in) == 0 {
		return nil
	}
	compiled, err := structuralWallsFrom(f, in, doors)
	if err != nil {
		return err
	}
	f.structuralWalls = compiled

	return nil
}

// structuralWallsFrom is the compile body: validate every definition against
// the field and the door list, then build the deep copies. Split from the
// assignment so a refusal cannot leave a half-built list on the field.
func structuralWallsFrom(f *field, in []StructuralWallInput, doors []DoorInput) ([]structuralWall, error) {
	out := make([]structuralWall, len(in))
	seenWall := make(map[PropID]bool, len(in))
	seenOpening := make(map[string]bool, len(in))
	seenDoor := make(map[DoorID]bool, len(in))

	for i := range in {
		w := &in[i]
		path := fmt.Sprintf("structural_walls[%d]", i)
		if err := f.validateStructuralWallIdentity(w, path, seenWall); err != nil {
			return nil, err
		}
		length, err := validateStructuralLine(w, path)
		if err != nil {
			return nil, err
		}
		openings, err := validateStructuralOpenings(f, w, path, length, doors, seenOpening, seenDoor)
		if err != nil {
			return nil, err
		}
		out[i] = structuralWall{
			id:        w.ID,
			ref:       w.Ref,
			from:      w.From,
			to:        w.To,
			height:    w.Height,
			thickness: w.Thickness,
			elevation: w.Elevation,
			openings:  openings,
		}
	}

	return out, nil
}

// validateStructuralWallIdentity asks whether a wall's own identity can be
// compiled: a nonempty id, unique among the structural walls, and naming a
// static placed contributor.
func (f *field) validateStructuralWallIdentity(w *StructuralWallInput, path string, seen map[PropID]bool) error {
	if w.ID == "" {
		return fmt.Errorf("%s has no id: %w", path, ErrNoField)
	}
	if seen[w.ID] {
		return fmt.Errorf("duplicate structural wall %q: %w", w.ID, ErrNoField)
	}
	seen[w.ID] = true
	exists, static := f.placedStaticState(w.ID)
	if !exists {
		return fmt.Errorf("%s.id %q names no placed identity: %w", path, w.ID, ErrNoField)
	}
	if !static {
		return fmt.Errorf("%s.id %q names a holdable or reserved placement, and presence must be static: %w",
			path, w.ID, ErrNoField)
	}

	return nil
}

// validateStructuralLine asks whether a wall's line and assembled dimensions
// can be compiled, returning its canonical length. Endpoints are finite and
// representable and distinct; height and thickness are positive; elevation is
// finite and may be negative.
func validateStructuralLine(w *StructuralWallInput, path string) (float64, error) {
	for _, c := range [...]struct {
		name string
		v    float64
	}{
		{path + ".from.x", w.From.X}, {path + ".from.y", w.From.Y},
		{path + ".to.x", w.To.X}, {path + ".to.y", w.To.Y},
	} {
		if !structuralRepresentable(c.v) {
			return 0, fmt.Errorf("%s must be a finite representable canonical coordinate: %w", c.name, ErrNoField)
		}
	}
	for _, d := range [...]struct {
		name     string
		v        float64
		positive bool
	}{
		{path + ".height", w.Height, true},
		{path + ".thickness", w.Thickness, true},
		{path + ".elevation", w.Elevation, false},
	} {
		if !structuralRepresentable(d.v) {
			return 0, fmt.Errorf("%s must be a finite representable canonical dimension: %w", d.name, ErrNoField)
		}
		if d.positive && d.v <= 0 {
			return 0, fmt.Errorf("%s must be a positive canonical dimension: %w", d.name, ErrNoField)
		}
	}

	length := math.Hypot(w.To.X-w.From.X, w.To.Y-w.From.Y)
	if !structuralRepresentable(length) || length <= 0 {
		return 0, fmt.Errorf("%s line must have a finite positive canonical length: %w", path, ErrNoField)
	}

	return length, nil
}

// validateStructuralOpenings asks whether every opening fits the line, whether
// any two overlap, and whether every bound door resolves. It returns the
// compiled openings, or the first refusal with the opening's own path.
//
// The roundoff is [dungeonspec]'s own convention: eight ULP of the measured
// length, so a corridor that ends within a ULP of an endpoint is legal rather
// than a phantom defect. Touching openings are legal — the earlier one ends
// exactly where the later begins.
func validateStructuralOpenings(
	f *field, w *StructuralWallInput, path string, length float64,
	doors []DoorInput, seenOpening map[string]bool, seenDoor map[DoorID]bool,
) ([]structuralOpening, error) {
	roundoff := 8 * (math.Nextafter(1, 2) - 1) * length
	type cut struct {
		start, end float64
		index      int
	}
	cuts := make([]cut, 0, len(w.Openings))
	out := make([]structuralOpening, 0, len(w.Openings))

	for j := range w.Openings {
		o := &w.Openings[j]
		op := fmt.Sprintf("%s.openings[%d]", path, j)
		if o.ID == "" {
			return nil, fmt.Errorf("%s has no id: %w", op, ErrNoField)
		}
		if seenOpening[o.ID] {
			return nil, fmt.Errorf("duplicate structural opening %q: %w", o.ID, ErrNoField)
		}
		seenOpening[o.ID] = true
		for _, d := range [...]struct {
			name     string
			v        float64
			positive bool
		}{
			{op + ".position", o.Position, false},
			{op + ".width", o.Width, true},
		} {
			if !structuralRepresentable(d.v) {
				return nil, fmt.Errorf("%s must be a finite representable canonical length: %w", d.name, ErrNoField)
			}
			if d.positive && d.v <= 0 {
				return nil, fmt.Errorf("%s must be a positive canonical length: %w", d.name, ErrNoField)
			}
		}
		a, b := o.Position-o.Width/2, o.Position+o.Width/2
		if a < -roundoff || b > length+roundoff {
			return nil, fmt.Errorf("%s must fit within the line: %w", op, ErrNoField)
		}
		if a < 0 {
			a = 0
		}
		if b > length {
			b = length
		}

		opening := structuralOpening{id: o.ID, position: o.Position, width: o.Width}
		if o.Door != nil {
			binding, err := f.validateStructuralDoorBinding(o.Door, op, doors, seenDoor)
			if err != nil {
				return nil, err
			}
			opening.door = binding
		}
		cuts = append(cuts, cut{start: a, end: b, index: j})
		out = append(out, opening)
	}

	sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
	for k := 1; k < len(cuts); k++ {
		if cuts[k].start < cuts[k-1].end {
			return nil, fmt.Errorf("%s.openings[%d] overlaps another opening: %w",
				path, cuts[k].index, ErrNoField)
		}
	}

	return out, nil
}

// validateStructuralDoorBinding asks whether a bound door resolves: a nonempty
// door identity unique across the field's bindings, a nonempty placed identity
// naming a static placed contributor, and a DoorID naming an existing footprint
// door. Its endpoints must be finite, representable and distinct — the width
// the record reports is their nonzero distance.
func (f *field) validateStructuralDoorBinding(
	b *StructuralDoorBindingInput, op string, doors []DoorInput, seenDoor map[DoorID]bool,
) (*structuralDoorBinding, error) {
	dp := op + ".door"
	if b.DoorID == "" {
		return nil, fmt.Errorf("%s has no door id: %w", dp, ErrNoField)
	}
	if seenDoor[b.DoorID] {
		return nil, fmt.Errorf("duplicate structural door binding %q: %w", b.DoorID, ErrNoField)
	}
	seenDoor[b.DoorID] = true
	if b.PlacedID == "" {
		return nil, fmt.Errorf("%s has no placed id: %w", dp, ErrNoField)
	}
	exists, static := f.placedStaticState(b.PlacedID)
	if !exists {
		return nil, fmt.Errorf("%s.placed_id %q names no placed identity: %w", dp, b.PlacedID, ErrNoField)
	}
	if !static {
		return nil, fmt.Errorf(
			"%s.placed_id %q names a holdable or reserved placement, and presence must be static: %w",
			dp, b.PlacedID, ErrNoField)
	}
	if !namesFootprintDoor(doors, b.DoorID) {
		return nil, fmt.Errorf("%s.door_id %q names no footprint door: %w", dp, b.DoorID, ErrNoField)
	}
	for _, c := range [...]struct {
		name string
		v    float64
	}{
		{dp + ".from.x", b.From.X}, {dp + ".from.y", b.From.Y},
		{dp + ".to.x", b.To.X}, {dp + ".to.y", b.To.Y},
	} {
		if !structuralRepresentable(c.v) {
			return nil, fmt.Errorf("%s must be a finite representable canonical coordinate: %w", c.name, ErrNoField)
		}
	}
	width := math.Hypot(b.To.X-b.From.X, b.To.Y-b.From.Y)
	if !structuralRepresentable(width) || width <= 0 {
		return nil, fmt.Errorf("%s must resolve a finite positive opening width: %w", dp, ErrNoField)
	}

	return &structuralDoorBinding{
		placedID: b.PlacedID,
		doorID:   b.DoorID,
		ref:      b.Ref,
		from:     b.From,
		to:       b.To,
	}, nil
}

// placedStaticState answers whether a placed contributor with this id exists,
// and whether it is static — neither holdable nor reserved, so nothing can take
// it off the floor or keep it off it. A wall's identity and a bound door's
// identity must both be static, because presence is fixed layout its reader
// joins by, not a running answer.
func (f *field) placedStaticState(id PropID) (exists, static bool) {
	i := f.placedIndexOf(id)
	if i < 0 {
		return false, false
	}

	return true, !f.placed[i].mutable()
}

// namesFootprintDoor reports whether a door with this id stands as a footprint
// rather than in a crossing — the one geometry a bound structural door can
// have, because its opening resolves the pose.
func namesFootprintDoor(doors []DoorInput, id DoorID) bool {
	for i := range doors {
		if doors[i].ID == id {
			return doors[i].Placement != nil
		}
	}

	return false
}
