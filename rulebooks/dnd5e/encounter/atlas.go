// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Atlas is a deterministic, construction-time snapshot of the field in
// dungeon-absolute space: every floor cell, every region and the cells it
// owns, and every prop, wall and doorway standing on the floor
// (rpg-project#256).
//
// FLAT, because the field is. Walls, props and doorways are field-level facts
// now (they were grouped under the room that declared them while rooms had
// origins), so the shape a host reads is the shape the session seam used to
// have to build from this one: one sorted list per kind, in one coordinate
// order, with the regions beside them rather than around them.
type Atlas struct {
	// Orientation is which way this field's hexes point.
	//
	// Reported because every cell below is axial, and a host laying the
	// floor out needs the same frame the encounter used to know where on
	// the screen an axial cell lands.
	Orientation Orientation

	// Cells is every floor cell, sorted by coordinate: the union of every
	// region's cells AND the field's scenery (rpg-project#360).
	//
	// A CELL HERE AND IN NO REGION BELOW IS SCENERY — floor nobody owns and
	// nobody stands on. That is the whole of what a host needs to be told
	// about it, which is why nothing else on this snapshot names it: the two
	// lists already say it, and a third statement of the same fact is a third
	// place for it to be wrong.
	Cells []spatial.Position

	// Regions is every region, sorted by region ID (C8), each listing its
	// own cells sorted the same way Cells is.
	Regions []AtlasRegion

	// Props is every authored thing standing on the floor, sorted by cell
	// then ref. See [AtlasProp].
	Props []AtlasProp

	// Boundaries is every authored wall, both endpoints absolute and
	// normalized (From before To in coordinate order), sorted by From then
	// To — the SAME edges compileCanvas registers, so what a host draws and
	// what the encounter enforces are identical.
	Boundaries []AtlasBoundary

	// Doorways is every door's every edge, sorted by door ID then cell. A
	// doorway is two adjacent floor cells with a door on the edge between
	// them; what state that door is in is [Encounter.Doors]' business, not
	// a snapshot's.
	Doorways []AtlasDoorway

	// Segments is every authored wall AS THE LINE IT IS, in authored order:
	// what a host draws, instead of chaining [Atlas.Boundaries] back into runs
	// under a straightness tolerance (rpg-project#360).
	//
	// PRESENTATION, AND THE SAME WALLS. Boundaries stay the mechanical truth —
	// every crossing nobody may take — and these are the lines those crossings
	// came from. A door's gap is the reader's own arithmetic: it is the
	// doorway's crossing, projected onto the segment it stands in.
	Segments []AtlasSegment

	// StructuralWalls is every authored structural wall (rpg-project#169),
	// sorted by id: its canonical line, its assembled dimensions and its
	// permitted cut list. See [AtlasStructuralWall].
	//
	// FILTERED BY [Encounter.AtlasFor], never sliced, on the same explicit
	// membership the placed contributors use: a wall is projected only when
	// its raw static presence survives, and a bound opening is retained only
	// when its own door is independently permitted. It is the layout a client
	// draws — the geometry the placed blockers were carved from and could not
	// describe.
	StructuralWalls []AtlasStructuralWall

	// StructuralDoors is every independently permitted structural door
	// (rpg-project#169), sorted by canonical DoorID: its opening endpoints and
	// the assembled dimensions it fits. See [AtlasStructuralDoor].
	//
	// ONE FLAT COLLECTION, NOT A PARENT-DEPENDENT SUBTYPE. A door here stands
	// on its own identity: a hidden parent never conceals an independently
	// permitted door, and this record carries no parent id, no hidden-opening
	// association and no state. FILTERED BY [Encounter.AtlasFor] when its own
	// raw static presence is withheld or its door identity is concealed.
	StructuralDoors []AtlasStructuralDoor

	// Placed is every authored footprint placement (issue #1753), sorted by
	// id — the SAME contributor set standing, crossing and sight read, in
	// the CANONICAL frame, so a host drawing or projecting the placed
	// geometry draws exactly what the engine enforces and nothing has to
	// re-derive it from cells.
	//
	// FILTERED BY [Encounter.AtlasFor], never sliced: a placement a
	// concealment hides, or one standing on floor the recipient cannot see,
	// is withheld whole (rpg-project#490). This list used to be withheld
	// WHOLESALE under any concealment, which stopped being honest the moment
	// a footprint door could be the secret.
	Placed []AtlasPlacedProp

	// Sealed is every cell in [Atlas.Cells] NOBODY CAN STAND ON, sorted by
	// coordinate: scenery, and the cells walls leave no room in.
	//
	// Needed because region membership stopped implying standable the moment a
	// wall could halve a cell (rpg-project#360, design §5.2 as amended). A
	// sealed cell keeps its region, its lighting and its archetype — a host
	// draws it exactly as it draws the floor beside it — and refusing a step
	// onto it is the engine's answer, not the client's guess.
	Sealed []spatial.Position

	// Exits is every authored way out, sorted by id (rpg-project#368) — so a
	// client can draw the way out and offer Leave where it stands.
	//
	// STRUCTURE ON THE TRUTH GRAIN, the same for every member, like the floor
	// itself. A way out is not a secret: it is a fact about the building, and
	// a party that has not found the vault has still walked in through the
	// front gate. Nothing here varies by recipient, so this survives
	// [Encounter.AtlasFor] unfiltered, which is exactly the claim its test
	// pins.
	//
	// What an exit MEANS is not here and never will be: an ending naming one
	// ([TriggerExitedHolding]) is a scenario's business, and a client that
	// could read "this is the winning one" off the map would be reading the
	// scenario off the geometry.
	Exits []AtlasExit

	// Start is where the party came in and which way they were looking, or
	// nil when the field declares none (rpg-project#374).
	//
	// STRUCTURE ON THE TRUTH GRAIN, like [Atlas.Exits] beside it and for the
	// same reason: the way in is a fact about the building, identical for
	// every member, so it survives [Encounter.AtlasFor] unfiltered.
	//
	// PRESENTATION, AND IT GATES NOTHING. It exists so a client can open the
	// camera the way the author meant — Kirk, walking the dungeon: "we
	// always start looking the wrong way" — and no rule anywhere reads it.
	// Where members ACTUALLY are is Members' answer and always was; this is
	// where the dungeon says it begins.
	//
	// NIL RATHER THAN A ZERO VALUE, because the zero would lie: a start at
	// [0,0] facing nowhere is a real dungeon somebody could author, so a
	// field that declares none has to be distinguishable from one that
	// declares that.
	Start *AtlasStart
}

// AtlasStart is the authored way in: a cell, and the direction the party is
// looking when they arrive. [FieldStart] as a snapshot.
type AtlasStart struct {
	// At is the cell, in dungeon-absolute space.
	At spatial.Position

	// Facing is one of the eight true-compass names — n|ne|e|se|s|sw|w|nw
	// (rpg-project#272, the same eight [AtlasProp.Facing] speaks) — or empty
	// when the author stated none. Carried verbatim: the wire names the
	// fact, and turning a name into an angle is the client's own calibrated
	// table, never this module's arithmetic.
	Facing string
}

// AtlasExit is one authored way out: its id, and the cell somebody stands on
// to leave through it. [FieldExit] as a snapshot.
type AtlasExit struct {
	// ID is the author's name for this exit — what an ending names, and what
	// a client shows if it labels the door.
	ID ExitID

	// At is the cell, in dungeon-absolute space.
	At spatial.Position
}

// AtlasSegment is one authored wall as a line: two ends in fractional axial,
// and the height it is drawn at. See [SegmentInput], which is the authoring
// side of the same fact.
//
// NO DOOR IDS AND NO FOOTPRINT. What a recipient may know about a door is
// [Atlas.Doorways]' business and is withheld from a non-knower there; a
// segment that carried its doors would say through the back door what the
// front one refuses. The footing is in [Atlas.Cells], where floor belongs.
type AtlasSegment struct {
	// From and To are the wall's two ends, in fractional axial.
	From, To AxialPointF

	// Height is the authored wall-height multiplier, carried verbatim.
	// 0 = not authored = standard height, [WallInput.Height]'s contract.
	Height float64
}

// AtlasStructuralWall is one authored structural wall as the map reports it:
// its stable identity, its opaque appearance reference, its line in canonical
// feet, the assembled dimensions it is drawn at, and its permitted cut list.
// [StructuralWallInput] as a snapshot.
//
// NO NESTED DOOR METADATA. An opening here carries only its identity, position
// and width; a door bound to it appears — when independently permitted — as its
// own [AtlasStructuralDoor], so a hidden parent cannot leak through a nested
// child and a hidden door leaves no tell behind.
type AtlasStructuralWall struct {
	// ID is the raw placed presence identity the wall's contributors are keyed
	// by ([StructuralWallInput.ID]).
	ID PropID

	// Ref is content's identifier for the wall's appearance, carried verbatim
	// and never inspected.
	Ref string

	// From and To are the wall's two ends in canonical feet.
	From, To spatial.Point

	// Height, Thickness and Elevation are the assembled dimensions in
	// canonical feet ([StructuralWallInput]).
	Height, Thickness, Elevation float64

	// Openings is the permitted cut list, in authored order: every bare
	// opening, and a bound opening only when its door is independently
	// permitted. A withheld bound opening is omitted WHOLE — id, position and
	// width included — so the visible wall does not disclose the secret.
	Openings []AtlasStructuralOpening
}

// AtlasStructuralOpening is one permitted gap in an [AtlasStructuralWall]: its
// identity, its centre along the line and its width, all canonical feet. It
// deliberately carries no door id, no state and no hidden association — the
// door, when permitted, is its own record.
type AtlasStructuralOpening struct {
	// ID names this opening ([StructuralOpeningInput.ID]).
	ID string

	// Position is the gap's centre, measured along From→To from the wall's
	// start, in canonical feet.
	Position float64

	// Width is the gap's width along the line, in canonical feet.
	Width float64
}

// AtlasStructuralDoor is one independently permitted structural door: its
// canonical gameplay DoorID, its opaque appearance reference, its resolved
// visual opening endpoints in canonical feet, and the assembled dimensions it
// fits. [StructuralDoorBindingInput] as a snapshot.
//
// SELF-CONTAINED, WITH NO PARENT. It carries no parent wall id and no opening
// association, so a client places it without a withheld parent's identity and
// one collection holds every permitted attached door.
type AtlasStructuralDoor struct {
	// ID is the actual canonical gameplay door id the observation and verb
	// paths use ([StructuralDoorBindingInput.DoorID]).
	ID DoorID

	// Ref is content's identifier for the door's appearance, carried verbatim
	// and never inspected.
	Ref string

	// From and To are the resolved visual opening endpoints in canonical
	// feet; their nonzero distance is the door's width.
	From, To spatial.Point

	// Height, Thickness and Elevation are the assembled dimensions the door
	// fits, in canonical feet — the owning wall's own assembly, so a door and
	// its opening agree without a second authored pose.
	Height, Thickness, Elevation float64
}

// AtlasRegion is one region: a NAMED SET OF CELLS, enumerated, with the
// per-area world facts it carries.
type AtlasRegion struct {
	// ID is the region's identifier.
	ID RegionID

	// Name is the region's display name, carried verbatim.
	Name string

	// Cells is every cell the region owns, dungeon-absolute, sorted by
	// coordinate.
	Cells []spatial.Position

	// Archetype is the presentation profile the assets resolve, carried
	// unread. It NEVER decides a mechanic — see [RegionInput].
	Archetype string

	// Lighting is the region's light level, carried unread.
	Lighting Lighting

	// A REGION NO LONGER REPORTS WHETHER IT IS HIDDEN. It carried
	// `Concealed bool` while a region was the thing that could be hidden;
	// the concealment primitive replaced the flag (rpg-project#490), and
	// what a non-knower's atlas withholds is [Encounter.AtlasFor]'s
	// business — which TRIMS this entry's cells rather than dropping it.
}

// AtlasProp is one authored thing standing on the floor, as the map reports
// it: what it is, where it stands in dungeon-absolute space, and what it does
// to a step and to a sightline. See [PropInput] for the authoring side.
type AtlasProp struct {
	// ID is the author's name for this placement, or empty when they gave it
	// none ([PropInput.ID]). Carried, never interpreted — a client renders it
	// and a verb names a prop by it.
	ID PropID

	// Holdable is whether a member can pick this up ([PropInput.Holdable]).
	// Carried so a client can offer the verb without asking a second
	// question about a thing it is already drawing.
	Holdable bool

	// Ref is content's identifier for this thing, carried through the
	// compile unchanged and never interpreted by this module.
	Ref string

	// At is where it stands, in dungeon-absolute space.
	At spatial.Position

	// BlocksMovement is whether a member can end a step on this cell.
	BlocksMovement bool

	// BlocksLineOfSight is whether it obstructs a sightline — subject to
	// spatial's lane rule, so one cell of it obstructs nothing on its own
	// ([PropInput]).
	BlocksLineOfSight bool

	// Facing and Offset are the same authored, uninterpreted presentational
	// facts as [PropInput.Facing] and [PropInput.Offset], carried through
	// unread. Neither is validated here either — dungeonspec is the layer
	// that owns the vocabulary and the bounds.
	Facing string
	Offset [3]float64
}

// AtlasPlacedProp is one authored footprint placement, as the map reports
// it: its name, its canonical geometry, and its two blocking answers.
// [PlacedPropInput] as a snapshot.
type AtlasPlacedProp struct {
	// ID is the author's name for this placement ([PlacedPropInput.ID]).
	ID PropID

	// Placement is the canonical rectangle — feet, facing in degrees, local
	// offset — copied out per call.
	Placement spatial.FootprintPlacement

	// Cells is every cell this placement STANDS ON, in the atlas's own
	// coordinate order (C8): the cells its rectangle covers, UNION the one
	// cell the rectangle's own centre lies in. [field.placedCells] is the
	// derivation, and this is that answer reported rather than re-derived.
	//
	// THE ADJACENCY A CLIENT READS, and the reason this field exists
	// (rpg-api-protos#351). A footprint has no anchor cell, so "what is this
	// thing next to?" is a geometry question — stationary footprint contact
	// against every cell centre, plus the hex the centre point itself lies
	// in for anything smaller than one cell. A client deriving that from
	// [AtlasPlacedProp.Placement] would need this module's plane, its cell
	// list and its tie-break to get the same answer, and would get a
	// different one the day any of the three moved. IT NEVER RE-DERIVES IT:
	// this list is the answer.
	//
	// THE SAME SET [Encounter.Hold]'S REACH JUDGES — holdPlaced in hold.go
	// applies the legacy reach rule (grid distance, Range 0 meaning
	// adjacent) to every cell [field.placedCells] returns, which is exactly
	// this list. So a client offering Hold where this says the member is
	// adjacent, and the engine refusing it, cannot disagree: one derivation,
	// one answer, on both sides of the wire. It is also the set the probe
	// law's visibility gate and an arrival fact's cell read ask
	// (placed_props.go).
	//
	// NEVER EMPTY for a placement on this list: a compiled field has cells,
	// so the centre clause always names one. Freshly allocated per call like
	// every other slice here.
	Cells []spatial.Position

	// BlocksMovement and BlocksLineOfSight are the two answers the engine
	// enforces, carried so a host need not guess from the shape.
	BlocksMovement    bool
	BlocksLineOfSight bool

	// Holdable is whether a member can pick this placement up
	// ([PlacedPropInput.Holdable], rpg-toolkit#1854) — [AtlasProp.Holdable]'s
	// field on the other kind of thing, carried for its reason: a client
	// offers the verb on a thing it is already drawing rather than asking a
	// second question about it.
	Holdable bool
}

// AtlasBoundary is one wall or barrier crossing, with both endpoints in
// dungeon-absolute space.
type AtlasBoundary struct {
	// From is one endpoint of the crossing, in dungeon-absolute space.
	From spatial.Position

	// To is the other endpoint of the crossing, in dungeon-absolute space.
	To spatial.Position

	// BlocksMovement reports whether an entity may cross this boundary.
	BlocksMovement bool

	// BlocksLineOfSight reports whether line of sight may cross this boundary.
	BlocksLineOfSight bool

	// Height is the authored wall-height multiplier, carried verbatim from
	// [WallInput.Height] and unread by this module. 0 = not authored =
	// standard height; see [WallInput.Height] for the full contract.
	Height float64
}

// AtlasDoorway is one crossable pair of cells a door stands in.
//
// Both cells are floor and adjacent — a doorway is an opening in a wall, not
// a cell of its own. See [Encounter.RegionAt] for what that means for
// somebody standing in one.
type AtlasDoorway struct {
	// Door is the door's identifier.
	Door DoorID

	// From is one of the two cells, in dungeon-absolute space.
	From spatial.Position

	// To is the other, adjacent to From.
	To spatial.Position
}

// Atlas returns a deterministic, construction-time snapshot of the field in
// dungeon-absolute space. Computed ONLY from construction data — the same
// compiled field ToData persists — never from live member placement, door
// state or clock, so placing or moving a member, opening a door or Pumping a
// tick never changes it (#929 T3 ruling 3; [Encounter.Doors] for why door
// state is read elsewhere).
//
// Deterministic (C8): every list sorted in one coordinate order, Regions by
// ID. Copy-out: every returned slice is freshly allocated per call; mutating
// the result never reaches internal state.
//
// O(cells) for the floor itself, which is what an enumerated floor costs and
// is the honest shape of it: a region IS its cells now, so the list the host
// wants is the list the composition already holds, copied.
//
// PLUS O(cells) PER STANDING PLACEMENT, so O(cells x (1+P)) per call for P of
// them ([AtlasPlacedProp.Cells], rpg-api-protos#351): saying where a
// rectangle stands is a walk over the floor, and this is the one place that
// walk is paid rather than three readers and a client each paying it
// separately. The field's cell budget (maxFieldCells) bounds both terms, and
// P is the authored placement count, so the product is bounded too.
func (e *Encounter) Atlas() (Atlas, error) {
	f := e.field
	out := Atlas{
		Orientation:     f.orientation,
		Cells:           append([]spatial.Position(nil), f.cells...),
		Regions:         make([]AtlasRegion, 0, len(f.regions)),
		Props:           make([]AtlasProp, 0, len(f.props)),
		Boundaries:      make([]AtlasBoundary, 0, len(f.walls)),
		Doorways:        make([]AtlasDoorway, 0, len(e.doors)),
		Segments:        make([]AtlasSegment, 0, len(f.segments)),
		StructuralWalls: make([]AtlasStructuralWall, 0, len(f.structuralWalls)),
	}

	for _, s := range f.segments {
		out.Segments = append(out.Segments, AtlasSegment{From: s.From, To: s.To, Height: s.Height})
	}

	// PLACED CONTRIBUTORS (issue #1753), in the canonical frame, sorted by
	// id (C8 — every list on this snapshot is sorted, and this one's
	// coordinate is a name). Copies, never the compiled contributors':
	// mutating the result never reaches internal state.
	//
	// AND WHERE EACH ONE IS RIGHT NOW (rpg-toolkit#1854), on the truth grain
	// the legacy props below are folded on: one waiting in reserve is on no
	// map, one somebody picked up is on no map, and one that was dropped
	// stands with its origin on the cell it was dropped on. The same fold
	// standing, crossing and sight read, so no two answers exist.
	nowPlaced := f.placedNow()
	for i := range f.placed {
		p := &f.placed[i]
		placement, standing := nowPlaced.stands(p)
		if !standing {
			continue
		}
		box := *placement.Footprint.Box
		out.Placed = append(out.Placed, AtlasPlacedProp{
			ID:        p.id,
			Placement: placement,
			// WHERE IT STANDS, IN CELLS, derived once here rather than by
			// every reader of this snapshot. Asked of the placement the
			// fold returned, not of the authored one, so a dropped
			// rectangle reports the cells it stands on NOW.
			//
			// This is the O(cells) walk [field.placedCells] is, once per
			// standing placement — the cost of stating the derivation
			// instead of leaving three protocols and one client to repeat
			// it. AtlasFor pays it ONCE, here, instead of a second time in
			// the filter: [Encounter.placedTouchesHidden] reads these cells
			// rather than measuring the rectangle again. On a field that
			// conceals nothing that filter never ran at all, so such a
			// field does pay this walk where it paid none — the cost is
			// stated in [Encounter.Atlas]' own doc rather than hidden here.
			Cells:             f.placedCells(placement),
			BlocksMovement:    p.blocksMovement,
			BlocksLineOfSight: p.blocksLineOfSight,
			Holdable:          p.holdable,
		})
		out.Placed[len(out.Placed)-1].Placement.Footprint.Box = &box
	}
	sort.Slice(out.Placed, func(i, j int) bool { return out.Placed[i].ID < out.Placed[j].ID })

	// Sorted by id rather than left in authored order: every other list on
	// this snapshot is sorted so nothing about how the field was authored
	// leaks through the order, and an exit list is no different.
	for _, ex := range f.exits {
		out.Exits = append(out.Exits, AtlasExit{ID: ex.ID, At: f.cellAt(ex.At)})
	}
	sort.Slice(out.Exits, func(i, j int) bool { return out.Exits[i].ID < out.Exits[j].ID })

	// The way in, converted once through the same cellAt the exits went
	// through. A COPY: the atlas is a snapshot, and a caller holding a
	// pointer into the compiled field could reach back into the world.
	if f.start != nil {
		out.Start = &AtlasStart{At: f.cellAt(f.start.At), Facing: f.start.Facing}
	}
	// ONLY WHEN THERE IS SOMETHING TO FIND. A cell of this floor fails
	// isStandable for exactly two reasons — it belongs to no region, or a wall
	// sealed it — so a field with neither has no unstandable cell and the walk
	// would be a pass over every cell to produce nothing. Measured at 21% of
	// AtlasFor on a dungeon ten times the reference tomb, all of it wasted,
	// because a dungeon of plain rooms and thin walls is exactly that field.
	if len(f.sceneryCells) > 0 || len(f.sealedCells) > 0 {
		for _, c := range f.cells {
			if !f.isStandable(c) {
				out.Sealed = append(out.Sealed, c)
			}
		}
	}

	for _, r := range f.regions {
		out.Regions = append(out.Regions, AtlasRegion{
			ID:        r.ID,
			Name:      r.Name,
			Cells:     append([]spatial.Position(nil), f.regionCells[r.ID]...),
			Archetype: r.Archetype,
			Lighting:  *r.Lighting,
		})
	}
	sort.Slice(out.Regions, func(i, j int) bool { return out.Regions[i].ID < out.Regions[j].ID })

	// WHERE A THING PHYSICALLY IS FOLDS HERE, not in the per-member
	// projection: it is truth-grain state, one answer for every member
	// (ruled 2026-09-01), unlike knowledge — which is audience-scoped and
	// belongs in AtlasFor. A prop somebody picked up is gone from everybody's
	// atlas; a prop somebody dropped stands where they dropped it. Putting
	// this in Atlas rather than in AtlasFor is also what makes the rule
	// total: AtlasFor short-circuits to this answer for a field with no
	// concealment, so a filter added there would silently not apply to
	// every plain dungeon (rpg-project#368).
	//
	// The atlas itself stays CONSTRUCTION TRUTH — nothing above mutates
	// f.props — and this is a fold over the journal, computed fresh, the
	// same move concealment already makes for doors.
	placements := e.holdings.propPlacements()
	for _, p := range f.props {
		at := f.cellAt(p.At)
		// A PROP IN RESERVE IS NOT ON ANY MAP (rpg-project#375, reserve.go):
		// authored with a predicate and not yet arrived, it is withheld here,
		// on the truth grain, for every member alike — the never-authored
		// yardstick, folded from the one fact that would say it came.
		if p.Arrives != nil && !placements[p.ID].arrived {
			continue
		}
		if placement, moved := placements[p.ID]; p.ID != "" && moved {
			if placement.gone {
				continue
			}
			at = placement.at
		}
		out.Props = append(out.Props, AtlasProp{
			ID:                p.ID,
			Holdable:          p.Holdable,
			Ref:               p.Ref,
			At:                at,
			BlocksMovement:    *p.BlocksMovement,
			BlocksLineOfSight: *p.BlocksLineOfSight,
			Facing:            p.Facing,
			Offset:            p.Offset,
		})
	}
	sort.Slice(out.Props, func(i, j int) bool {
		if out.Props[i].At != out.Props[j].At {
			return cellBefore(out.Props[i].At, out.Props[j].At)
		}
		return out.Props[i].Ref < out.Props[j].Ref
	})

	for _, w := range f.walls {
		edge := normalizeDoorEdge(DoorEdge{From: f.cellAt(w.From), To: f.cellAt(w.To)})
		out.Boundaries = append(out.Boundaries, AtlasBoundary{
			From:              edge.From,
			To:                edge.To,
			BlocksMovement:    w.BlocksMovement,
			BlocksLineOfSight: w.BlocksLineOfSight,
			Height:            w.Height,
		})
	}
	sort.Slice(out.Boundaries, func(i, j int) bool {
		if out.Boundaries[i].From != out.Boundaries[j].From {
			return cellBefore(out.Boundaries[i].From, out.Boundaries[j].From)
		}
		return cellBefore(out.Boundaries[i].To, out.Boundaries[j].To)
	})

	// e.doors is already sorted by ID (doorRecordsFrom) and every edge
	// normalized; the sort below keeps Atlas's own determinism
	// self-contained rather than coupled to that invariant.
	for _, d := range e.doors {
		for _, edge := range d.edges {
			out.Doorways = append(out.Doorways, AtlasDoorway{Door: d.id, From: edge.From, To: edge.To})
		}
	}
	sort.Slice(out.Doorways, func(i, j int) bool {
		a, b := out.Doorways[i], out.Doorways[j]
		if a.Door != b.Door {
			return a.Door < b.Door
		}
		if a.From != b.From {
			return cellBefore(a.From, b.From)
		}
		return cellBefore(a.To, b.To)
	})

	// THE STRUCTURAL LAYOUT (rpg-project#169). The FULL author atlas carries
	// every valid definition and every opening, including a bound one; the
	// per-member filter is [Encounter.AtlasFor]'s, exactly as it is for the
	// placed contributors. Sorted by wall id and door id so a map or an
	// authored order never leaks through the list (C8).
	for i := range f.structuralWalls {
		w := &f.structuralWalls[i]
		wall := AtlasStructuralWall{
			ID: w.id, Ref: w.ref, From: w.from, To: w.to,
			Height: w.height, Thickness: w.thickness, Elevation: w.elevation,
			Openings: make([]AtlasStructuralOpening, 0, len(w.openings)),
		}
		for j := range w.openings {
			o := &w.openings[j]
			wall.Openings = append(wall.Openings, AtlasStructuralOpening{ID: o.id, Position: o.position, Width: o.width})
			if o.door != nil {
				out.StructuralDoors = append(out.StructuralDoors, AtlasStructuralDoor{
					ID: o.door.doorID, Ref: o.door.ref, From: o.door.from, To: o.door.to,
					Height: w.height, Thickness: w.thickness, Elevation: w.elevation,
				})
			}
		}
		out.StructuralWalls = append(out.StructuralWalls, wall)
	}
	sortStructuralLayout(&out)

	return out, nil
}

// Grid reports the field's coordinate family, in O(1).
//
// Always [spatial.GridShapeHex] as of rpg-project#256: the square family left
// with the room chain, and a region is painted on a hex grid. Kept as a read
// because a caller doing grid arithmetic of its own should learn which
// arithmetic to do from the field rather than assume it — the two families
// disagree about what one step means, and Chebyshev distance on axial
// coordinates passes almost every fixture while being wrong on the diagonals.
//
// Returns ErrNoField on the zero value, which construction forbids.
func (e *Encounter) Grid() (spatial.GridShape, error) {
	if e.field == nil {
		return spatial.GridShapeHex, fmt.Errorf("grid: %w", ErrNoField)
	}
	return spatial.GridShapeHex, nil
}
