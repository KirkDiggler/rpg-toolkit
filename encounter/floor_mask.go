package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// floorMaskGrid keeps canvas dimensions as workspace bounds while making the
// persisted structural-floor mask the only mechanically valid set of cells.
// Line rays intentionally retain void cells so movement and the room wrapper
// can fail closed when a direct crossing leaves the floor.
type floorMaskGrid struct {
	base  *spatial.HexGrid
	floor map[spatial.Position]struct{}
}

func newFloorMaskGrid(width, height int, floor map[coreHex]struct{}) *floorMaskGrid {
	positions := make(map[spatial.Position]struct{}, len(floor))
	for hex := range floor {
		positions[hex.ToPosition()] = struct{}{}
	}
	return &floorMaskGrid{
		base: spatial.NewHexGrid(spatial.HexGridConfig{
			Width: float64(width), Height: float64(height), Orientation: spatial.HexOrientationPointyTop,
		}),
		floor: positions,
	}
}

// coreHex is a local alias declared below to keep constructors readable.
type coreHex = core.Hex

func (g *floorMaskGrid) GetShape() spatial.GridShape                { return g.base.GetShape() }
func (g *floorMaskGrid) GetDimensions() spatial.Dimensions          { return g.base.GetDimensions() }
func (g *floorMaskGrid) Distance(from, to spatial.Position) float64 { return g.base.Distance(from, to) }
func (g *floorMaskGrid) IsAdjacent(a, b spatial.Position) bool      { return g.base.IsAdjacent(a, b) }
func (g *floorMaskGrid) IsValidPosition(pos spatial.Position) bool {
	if !g.base.IsValidPosition(pos) {
		return false
	}
	_, ok := g.floor[pos]
	return ok
}
func (g *floorMaskGrid) GetNeighbors(pos spatial.Position) []spatial.Position {
	return filterFloorPositions(g.base.GetNeighbors(pos), g)
}
func (g *floorMaskGrid) GetLineOfSight(from, to spatial.Position) []spatial.Position {
	return g.base.GetLineOfSight(from, to)
}
func (g *floorMaskGrid) GetPositionsInRange(center spatial.Position, radius float64) []spatial.Position {
	return filterFloorPositions(g.base.GetPositionsInRange(center, radius), g)
}
func filterFloorPositions(in []spatial.Position, grid *floorMaskGrid) []spatial.Position {
	out := make([]spatial.Position, 0, len(in))
	for _, pos := range in {
		if grid.IsValidPosition(pos) {
			out = append(out, pos)
		}
	}
	return out
}

// floorMaskRoom closes the two gaps a masked Grid alone cannot express through
// BasicRoom: a direct LoS ray crossing void must block, and envelope boundaries
// may have an intentionally invalid void endpoint so are not registered as
// ordinary two-floor boundaries.
type floorMaskRoom struct {
	*spatial.BasicRoom
	grid *floorMaskGrid
}

func (r *floorMaskRoom) GetGrid() spatial.Grid { return r.grid }
func (r *floorMaskRoom) GetLineOfSight(from, to spatial.Position) []spatial.Position {
	return r.grid.GetLineOfSight(from, to)
}
func (r *floorMaskRoom) IsBoundaryMovementBlocked(from, to spatial.Position) bool {
	if !r.grid.IsValidPosition(from) || !r.grid.IsValidPosition(to) {
		return true
	}
	return r.BasicRoom.IsBoundaryMovementBlocked(from, to)
}

func (r *floorMaskRoom) IsLineOfSightBlocked(from, to spatial.Position) bool {
	if !r.grid.IsValidPosition(from) || !r.grid.IsValidPosition(to) {
		return true
	}
	for _, pos := range r.grid.GetLineOfSight(from, to) {
		if !r.grid.IsValidPosition(pos) {
			return true
		}
	}
	return r.BasicRoom.IsLineOfSightBlocked(from, to)
}

var _ spatial.Grid = (*floorMaskGrid)(nil)
var _ spatial.BoundaryAwareRoom = (*floorMaskRoom)(nil)
