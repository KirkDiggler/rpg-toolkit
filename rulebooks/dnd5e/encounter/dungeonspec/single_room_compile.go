package dungeonspec

import (
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CompileSingleRoom lowers the complete single-room source into the existing
// encounter field contract. Source cells are axial; FieldInput remains the
// legacy offset frame and is reached only through the canonical spatial
// inverse.
//
// THE PRESENTATION DOES NOT COME WITH IT (rpg-project#479). The authored
// scene is content, served to the player by dungeon key; what crosses into
// the field is the geometry the lowering read out of it —
// [encounter.PlacedPropInput] per declared prop, and nothing that names an
// asset, a light or a workspace. single_room_lowering.go lists every value this
// compile reads from the presentation, by path.
//
// THE SITE SCOPE AND THE ORDERS COMPILE THROUGH THE SHARED COMPILERS
// (rpg-project#477, rpg-toolkit#1826; rpg-project#484). `factions:` and
// `dispositions:` go through [factionsOf] and [dispositionsOf] over this
// dialect's own cast, so the faction-of-one mind rule and the predicate
// compilation are the same ones the other dialect gets; a creature's compiled
// orders are [ordersOf] — its faction's table with its own laid over
// wholesale, its word beating its faction's mix, and its actions verbatim in
// the authored order. Neither dialect writes those three lines itself.
//
// A DOCUMENT WITH NONE OF THOSE KEYS COMPILES TO WHAT IT ALWAYS DID.
// [encounter.Layer] of two nil tables is nil, [temperOf] of two absences is
// the zero temper, and an absent faction stays the empty string the reserved
// `monsters` side is written as — so every field they fill is omitted from
// the committed pictures exactly as before.
func CompileSingleRoom(in CompileSingleRoomInput) (Compiled, error) {
	if in.Spec == nil {
		return Compiled{}, singleRoomCompileError("source", "single room spec is nil")
	}
	read, errs := validateSingleRoom(in.Spec)
	if len(errs) > 0 {
		return Compiled{}, &ValidationError{Errors: errs}
	}
	spec := in.Spec
	// The cast this dialect placed, and what each declared side hands it.
	cast := roomMembers(&spec.Room.Gameplay)
	from := inheritedOrders(spec.Factions)
	o := spatial.HexOrientationPointyTop
	cells := make([]spatial.Position, 0, len(spec.Room.Gameplay.WalkableHexes))
	for _, c := range spec.Room.Gameplay.WalkableHexes {
		cells = append(cells, axialOffset(c, o))
	}
	bright := encounter.Lighting{Intensity: 1}
	props, err := spec.Room.CanonicalPlacedProps()
	if err != nil {
		return Compiled{}, singleRoomCompileError("room.room.propDeclarations", err.Error())
	}
	field := encounter.FieldInput{
		Canvas:  encounter.CanvasInput{Void: encounter.VoidIsTransparent(), Orientation: encounter.HexesArePointyTop()},
		Regions: []encounter.RegionInput{{ID: spec.Room.Gameplay.ImplicitRegionID, Name: spec.Room.Name, Cells: cells, Archetype: "crypt", Lighting: &bright}},
		Placed:  props,
		// THE DOORS, standing as the footprints their prop declarations draw
		// (rpg-project#485, single_room_doors.go). A door item is in BOTH
		// lists and that is not a duplicate: `Placed` is the rectangle the
		// map DRAWS and reports in its atlas, carrying the two flags the
		// author left false, and this is the same rectangle with its state
		// deciding what it blocks. One authored footprint, one adapter, two
		// questions.
		Doors: singleRoomDoors(spec.Key, &spec.Room.Gameplay, read),
		Start: nil,
		// The sides ride the FIELD, for [Compile]'s reason: the stance graph
		// is seeded from them at every Setup and Load, so they have to be
		// where the field is. Nil when the site declares none.
		Factions:     factionsOf(spec.Factions, cast),
		Dispositions: dispositionsOf(spec.Dispositions),
	}
	starts := []spatial.Position{}
	if spec.Room.Gameplay.PartyStart == nil {
		return Compiled{}, singleRoomCompileError("room.room.partyStart", "is required")
	}
	starts = append(starts, axialOffset(*spec.Room.Gameplay.PartyStart, o))
	monsters := make([]MonsterPlacement, 0, len(spec.Room.Gameplay.Monsters))
	occupied := map[RoomCell]bool{}
	if occupied[*spec.Room.Gameplay.PartyStart] {
		return Compiled{}, singleRoomCompileError("room.room.partyStart", "is occupied")
	}
	for i, m := range spec.Room.Gameplay.Monsters {
		if m.Cell == *spec.Room.Gameplay.PartyStart || occupied[m.Cell] {
			return Compiled{}, singleRoomCompileError(fmt.Sprintf("room.room.monsters[%d].cell", i), "is occupied")
		}
		occupied[m.Cell] = true
		at := axialOffset(m.Cell, o)
		starts = append(starts, at)
		// Its membership comes off the actor and its orders off the binding —
		// absent is the zero block, which orders nothing.
		b := spec.Room.Gameplay.MonsterBindings[m.ID]
		mp := ordersOf(creatureOrders{
			ID: m.ID, Ref: m.Ref, Faction: m.Faction,
			On: b.On, Temper: b.Temper, Actions: b.Actions,
		}, from)
		mp.Region = spec.Room.Gameplay.ImplicitRegionID
		mp.At = at
		monsters = append(monsters, mp)
	}
	// Validate each source placement independently. Besides identifying the
	// actual failing placement (rather than the last one in the batch), this
	// keeps the diagnostic in the author's axial frame instead of exposing the
	// legacy offset cell used by the encounter field.
	placements := make([]struct {
		path string
		cell RoomCell
		at   spatial.Position
	}, 0, len(starts))
	placements = append(placements, struct {
		path string
		cell RoomCell
		at   spatial.Position
	}{path: "room.room.partyStart", cell: *spec.Room.Gameplay.PartyStart, at: starts[0]})
	for i, m := range spec.Room.Gameplay.Monsters {
		placements = append(placements, struct {
			path string
			cell RoomCell
			at   spatial.Position
		}{path: fmt.Sprintf("room.room.monsters[%d].cell", i), cell: m.Cell, at: starts[i+1]})
	}
	for _, placement := range placements {
		if err := encounter.ValidateStaticPlacements(field, []spatial.Position{placement.at}); err != nil {
			var placementErr *encounter.StaticPlacementError
			if errors.As(err, &placementErr) {
				err = errors.New(placementErr.Reason)
			}
			return Compiled{}, singleRoomCompileError(placement.path,
				fmt.Sprintf("at author's axial q=%d r=%d: %s", placement.cell.Q, placement.cell.R, err))
		}
	}
	// Party seats are deterministic nearest-first in the authored region.
	party := deriveSingleRoomSeats(cells, starts[0], occupied, o, field)
	for i := range party {
		party[i].Region = spec.Room.Gameplay.ImplicitRegionID
	}
	if len(party) == 0 {
		return Compiled{}, singleRoomCompileError("room.room.partyStart", "has no free seat")
	}
	field.Start = &encounter.FieldStart{At: starts[0]}
	return Compiled{
		Key: spec.Key, Name: read.Name, Field: field, PartyStart: party, Monsters: monsters,
		// The same lists the field carries, surfaced for a host that wants to
		// read what the site declares without reaching into it ([Compiled]).
		Factions: field.Factions, Dispositions: field.Dispositions,
	}, nil
}

// CompileSingleRoomInput supplies a decoded single-room specification.
type CompileSingleRoomInput struct{ Spec *SingleRoomSpec }

func singleRoomCompileError(path, message string) error {
	return &ValidationError{Errors: []FieldError{{Path: path, Message: message}}}
}

func axialOffset(c RoomCell, o spatial.HexOrientation) spatial.Position {
	cube := spatial.CubeCoordinate{X: c.Q, Y: -c.Q - c.R, Z: c.R}
	return cube.ToOffsetCoordinateWithOrientation(o)
}

func deriveSingleRoomSeats(cells []spatial.Position, start spatial.Position, monsters map[RoomCell]bool, o spatial.HexOrientation, field encounter.FieldInput) []Seat {
	type seatDist struct {
		seat Seat
		d    int
	}
	out := make([]seatDist, 0, len(cells))
	startCube := spatial.OffsetCoordinateToCubeWithOrientation(start, o)
	for _, c := range cells {
		if c == start {
			out = append(out, seatDist{Seat{At: c}, 0})
			continue
		}
		cube := spatial.OffsetCoordinateToCubeWithOrientation(c, o)
		// monster exclusion is checked by authored axial identity.
		axial := RoomCell{Q: cube.X, R: cube.Z}
		if monsters[axial] {
			continue
		}
		if encounter.ValidateStaticPlacements(field, []spatial.Position{c}) != nil {
			continue
		}
		out = append(out, seatDist{Seat{At: c}, startCube.Distance(cube)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].d != out[j].d {
			return out[i].d < out[j].d
		}
		if out[i].seat.At.X != out[j].seat.At.X {
			return out[i].seat.At.X < out[j].seat.At.X
		}
		return out[i].seat.At.Y < out[j].seat.At.Y
	})
	seats := make([]Seat, len(out))
	for i := range out {
		seats[i] = out[i].seat
		seats[i].Region = ""
	}
	for i := range seats {
		seats[i].Region = ""
	}
	return seats
}
