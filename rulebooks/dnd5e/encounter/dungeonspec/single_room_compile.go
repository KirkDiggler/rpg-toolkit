package dungeonspec

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CompileSingleRoom lowers the complete v3 source into the existing encounter
// field contract. Source cells are axial; FieldInput remains the legacy offset
// frame and is reached only through the canonical spatial inverse.
func CompileSingleRoom(in CompileSingleRoomInput) (Compiled, error) {
	if in.Spec == nil {
		return Compiled{}, singleRoomCompileError("source", "single room spec is nil")
	}
	if errs := validateSingleRoom(in.Spec); len(errs) > 0 {
		return Compiled{}, &ValidationError{Errors: errs}
	}
	spec := in.Spec
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
	presentation := presentationCopy(spec.Room)
	field := encounter.FieldInput{
		RoomScene: &presentation,
		Canvas:    encounter.CanvasInput{Void: encounter.VoidIsTransparent(), Orientation: encounter.HexesArePointyTop()},
		Regions:   []encounter.RegionInput{{ID: spec.Room.Gameplay.ImplicitRegionID, Name: spec.Room.Name, Cells: cells, Archetype: "crypt", Lighting: &bright}},
		Placed:    props,
		Start:     nil,
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
	for _, m := range spec.Room.Gameplay.Monsters {
		if m.Cell == *spec.Room.Gameplay.PartyStart || occupied[m.Cell] {
			return Compiled{}, singleRoomCompileError(fmt.Sprintf("room.room.monsters[%d].cell", len(monsters)), "is occupied")
		}
		occupied[m.Cell] = true
		at := axialOffset(m.Cell, o)
		starts = append(starts, at)
		monsters = append(monsters, MonsterPlacement{ID: m.ID, Ref: m.Ref, Region: spec.Room.Gameplay.ImplicitRegionID, At: at})
	}
	// Validate final placement against the same encounter-owned static fold.
	if err := encounter.ValidateStaticPlacements(field, starts); err != nil {
		path := "room.room.partyStart"
		if len(starts) > 1 {
			path = fmt.Sprintf("room.room.monsters[%d].cell", len(starts)-2)
		}
		return Compiled{}, singleRoomCompileError(path, err.Error())
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
	return Compiled{Key: spec.Key, Name: spec.Room.Scene.Name, Field: field, PartyStart: party, Monsters: monsters}, nil
}

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

func presentationCopy(r RoomSource) encounter.RoomScenePresentation {
	p := encounter.RoomScenePresentation{Version: 1, Frame: r.CoordinateFrame, Workspace: r.Workspace, Scene: r.Scene}
	p.Scene.Items = make([]encounter.RoomSceneItem, len(r.Scene.Items))
	copy(p.Scene.Items, r.Scene.Items)
	p.Scene.Groups = make([]encounter.RoomSceneGroup, len(r.Scene.Groups))
	copy(p.Scene.Groups, r.Scene.Groups)
	for i := range p.Scene.Items {
		if r.Scene.Items[i].HeightScale != nil {
			v := *r.Scene.Items[i].HeightScale
			p.Scene.Items[i].HeightScale = &v
		}
		if r.Scene.Items[i].PointLight != nil {
			v := *r.Scene.Items[i].PointLight
			p.Scene.Items[i].PointLight = &v
		}
	}
	return p
}
