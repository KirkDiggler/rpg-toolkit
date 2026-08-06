package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CanvasMaxStructuralCells is the largest canvas structural-floor projection
// the encounter runtime accepts. It bounds every canvas allocation and walk.
const CanvasMaxStructuralCells = 1 << 20

// FloorSourceKind identifies how InitDungeon derives its structural floor.
type FloorSourceKind string

const (
	// FloorSourceRoomChain derives floor geometry from Regions and Connectors.
	FloorSourceRoomChain FloorSourceKind = "room-chain"
	// FloorSourceCanvas derives the complete structural floor from Width and Height.
	FloorSourceCanvas FloorSourceKind = "canvas"
)

// ValidateCanvasDimensions validates canvas dimensions and returns their safe
// structural-cell count. It uses division before multiplication so malformed
// MaxInt-sized input is rejected without overflow or allocation. dungeonspec
// calls this exported boundary helper to keep the canvas scale contract owned
// by encounter, which owns the runtime representation.
func ValidateCanvasDimensions(width, height int) (int, error) {
	if width < 1 || height < 1 {
		return 0, fmt.Errorf("canvas dimensions must be positive (got width %d, height %d)", width, height)
	}
	if width > CanvasMaxStructuralCells/height {
		return 0, fmt.Errorf(
			"canvas dimensions %d x %d exceed the maximum of %d structural cells",
			width, height, CanvasMaxStructuralCells,
		)
	}
	return width * height, nil
}

func floorSourceKind(kind FloorSourceKind) (FloorSourceKind, error) {
	switch kind {
	case "", FloorSourceRoomChain:
		return FloorSourceRoomChain, nil
	case FloorSourceCanvas:
		return FloorSourceCanvas, nil
	default:
		return "", fmt.Errorf("unknown floor source %q", kind)
	}
}

func canvasFloorHexes(width, height int) map[core.Hex]struct{} {
	count, _ := ValidateCanvasDimensions(width, height)
	floor := make(map[core.Hex]struct{}, count)
	for col := 0; col < width; col++ {
		for row := 0; row < height; row++ {
			floor[core.HexFromPosition(spatial.Position{X: float64(col), Y: float64(row)})] = struct{}{}
		}
	}
	return floor
}

func canvasFloorContainsDimensions(width, height int, hex core.Hex) bool {
	if hex.Q < 0 || hex.Q >= width {
		return false
	}
	offset := (hex.Q - (hex.Q & 1)) / 2
	if hex.S < -offset || hex.S >= height-offset {
		return false
	}
	return hex.R == -hex.Q-hex.S
}

func validateCanvasDungeonParams(params DungeonParams) error {
	if _, err := ValidateCanvasDimensions(params.Width, params.Height); err != nil {
		return err
	}
	if len(params.Regions) != 0 || len(params.Connectors) != 0 {
		return fmt.Errorf("canvas dungeon must not contain regions or connectors")
	}
	if params.PartyStart.SeatCount < 0 {
		return fmt.Errorf("party start seat count must not be negative (got %d)", params.PartyStart.SeatCount)
	}
	floor := canvasFloorHexes(params.Width, params.Height)
	occupied := make(map[core.Hex]string, len(params.CanvasPlacedObstacles)+len(params.CanvasReservedCells))
	seenIDs := make(map[core.EntityID]int, len(params.CanvasPlacedObstacles))
	for index, obstacle := range params.CanvasPlacedObstacles {
		if obstacle.ID == "" {
			return fmt.Errorf("canvas placed obstacle %d: id required", index)
		}
		if first, duplicate := seenIDs[obstacle.ID]; duplicate {
			return fmt.Errorf("canvas placed obstacle %d (%q): duplicate id (already used by %d)", index, obstacle.ID, first)
		}
		seenIDs[obstacle.ID] = index
		if _, ok := floor[obstacle.At]; !ok {
			return fmt.Errorf("canvas placed obstacle %q at %v is outside canvas floor", obstacle.ID, obstacle.At)
		}
		if prior, conflict := occupied[obstacle.At]; conflict {
			return fmt.Errorf("canvas placed obstacle %q at %v collides with %s", obstacle.ID, obstacle.At, prior)
		}
		occupied[obstacle.At] = fmt.Sprintf("canvas prop %q", obstacle.ID)
	}
	for index, reserved := range params.CanvasReservedCells {
		if reserved.Name == "" {
			return fmt.Errorf("canvas reserved cell %d: name required", index)
		}
		if _, ok := floor[reserved.At]; !ok {
			return fmt.Errorf("canvas reserved cell %q at %v is outside canvas floor", reserved.Name, reserved.At)
		}
		if prior, conflict := occupied[reserved.At]; conflict {
			return fmt.Errorf("canvas reserved cell %q at %v collides with %s", reserved.Name, reserved.At, prior)
		}
		occupied[reserved.At] = reserved.Name
	}
	return nil
}

func generateCanvasDungeonLayout(params DungeonParams) (*dungeonLayout, error) {
	reservation, err := resolveCanvasPartyStartReservation(params)
	if err != nil {
		return nil, err
	}
	obstacles := make([]ObstacleData, len(params.CanvasPlacedObstacles))
	for index, obstacle := range params.CanvasPlacedObstacles {
		obstacles[index] = ObstacleData{
			ID: obstacle.ID, Ref: obstacle.Ref, Position: obstacle.At,
			BlocksMovement: obstacle.BlocksMovement, BlocksLoS: obstacle.BlocksLoS, Facing: obstacle.Facing,
		}
	}
	return &dungeonLayout{
		width: params.Width, entrance: reservation.anchor, obstacles: obstacles,
		partyStartPositions: reservation.positions(),
	}, nil
}

func resolveCanvasPartyStartReservation(params DungeonParams) (partyStartReservation, error) {
	seatCount := params.PartyStart.SeatCount
	if seatCount == 0 {
		seatCount = 1
	}
	anchor := core.HexFromPosition(spatial.Position{}).ToCube()
	if params.PartyStart.Anchor != nil {
		anchor = params.PartyStart.Anchor.ToCube()
	}
	floor := canvasFloorHexes(params.Width, params.Height)
	anchorHex := core.HexFromCube(anchor)
	if _, ok := floor[anchorHex]; !ok {
		return partyStartReservation{}, fmt.Errorf("party start anchor %v is outside canvas floor", anchorHex)
	}
	blockers := make(map[core.Hex]string, len(params.CanvasPlacedObstacles)+len(params.CanvasReservedCells))
	for _, obstacle := range params.CanvasPlacedObstacles {
		blockers[obstacle.At] = fmt.Sprintf("canvas prop %q", obstacle.ID)
	}
	for _, reserved := range params.CanvasReservedCells {
		blockers[reserved.At] = reserved.Name
	}
	if blocker, blocked := blockers[anchorHex]; blocked {
		return partyStartReservation{}, fmt.Errorf("party start anchor %v collides with %s", anchorHex, blocker)
	}
	type candidate struct {
		hex                core.Hex
		col, row, distance int
	}
	candidates := make([]candidate, 0, params.Width*params.Height-1)
	for col := 0; col < params.Width; col++ {
		for row := 0; row < params.Height; row++ {
			cell := core.HexFromPosition(spatial.Position{X: float64(col), Y: float64(row)})
			if cell == anchorHex {
				continue
			}
			if _, blocked := blockers[cell]; blocked {
				continue
			}
			pos := cell.ToPosition()
			candidates = append(candidates, candidate{
				hex: cell, col: int(pos.X), row: int(pos.Y), distance: anchor.Distance(cell.ToCube()),
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].row != candidates[j].row {
			return candidates[i].row < candidates[j].row
		}
		return candidates[i].col < candidates[j].col
	})
	if len(candidates)+1 < seatCount {
		return partyStartReservation{}, fmt.Errorf(
			"party start anchor at [%d,%d] requires %d seats but canvas has only %d available authored floor cells",
			int(anchorHex.ToPosition().X), int(anchorHex.ToPosition().Y), seatCount, len(candidates)+1,
		)
	}
	seats := make([]spatial.CubeCoordinate, 0, seatCount)
	seats = append(seats, anchor)
	for index := 0; index < seatCount-1; index++ {
		seats = append(seats, candidates[index].hex.ToCube())
	}
	return partyStartReservation{anchor: anchor, seats: seats}, nil
}
