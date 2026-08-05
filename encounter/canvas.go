package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// validateCanvasFloorSource makes persisted canvas identity explicit and
// canonical. In particular, Cells cannot be omitted and inferred from empty
// Regions: they are the source-of-truth structural floor record.
func validateCanvasFloorSource(source *CanvasFloorSource) error {
	if source == nil {
		return fmt.Errorf("canvas floor source is required")
	}
	if source.Width < 1 || source.Height < 1 {
		return fmt.Errorf("canvas dimensions must be positive (got width %d, height %d)", source.Width, source.Height)
	}
	if len(source.Cells) != source.Width*source.Height {
		return fmt.Errorf(
			"canvas cells must contain exactly %d canonical cells, got %d",
			source.Width*source.Height, len(source.Cells),
		)
	}
	for col := 0; col < source.Width; col++ {
		for row := 0; row < source.Height; row++ {
			index := col*source.Height + row
			expected := core.HexFromPosition(spatial.Position{X: float64(col), Y: float64(row)})
			if source.Cells[index] != expected {
				return fmt.Errorf("canvas cells[%d] must be canonical cell at [%d,%d]", index, col, row)
			}
		}
	}
	return nil
}

func cloneCanvasFloorSource(source *CanvasFloorSource) *CanvasFloorSource {
	if source == nil {
		return nil
	}
	return &CanvasFloorSource{
		Width: source.Width, Height: source.Height,
		Cells: append([]core.Hex(nil), source.Cells...),
	}
}

func canvasFloorHexes(source *CanvasFloorSource) map[core.Hex]struct{} {
	floor := make(map[core.Hex]struct{}, len(source.Cells))
	for _, cell := range source.Cells {
		floor[cell] = struct{}{}
	}
	return floor
}

func validateCanvasDungeonParams(params DungeonParams) error {
	if err := validateCanvasFloorSource(params.Canvas); err != nil {
		return err
	}
	if len(params.Regions) != 0 || len(params.Connectors) != 0 {
		return fmt.Errorf("canvas dungeon must not contain regions or connectors")
	}
	if params.Height != params.Canvas.Height {
		return fmt.Errorf("canvas dungeon height must equal canvas height %d (got %d)", params.Canvas.Height, params.Height)
	}
	if params.PartyStart.SeatCount < 0 {
		return fmt.Errorf("party start seat count must not be negative (got %d)", params.PartyStart.SeatCount)
	}
	floor := canvasFloorHexes(params.Canvas)
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
			BlocksMovement: obstacle.BlocksMovement, BlocksLoS: obstacle.BlocksLoS,
			Facing: obstacle.Facing,
		}
	}
	return &dungeonLayout{
		width:               params.Canvas.Width,
		entrance:            reservation.anchor,
		obstacles:           obstacles,
		partyStartPositions: reservation.positions(),
	}, nil
}

// resolveCanvasPartyStartReservation selects an ordered absolute party
// envelope before any runtime blockers are installed. Author-provided props
// and monster spawn reservations are excluded by name, so there is never a
// later seed-dependent relocation or collision.
func resolveCanvasPartyStartReservation(params DungeonParams) (partyStartReservation, error) {
	seatCount := params.PartyStart.SeatCount
	if seatCount == 0 {
		seatCount = 1
	}
	anchor := core.HexFromPosition(spatial.Position{}).ToCube()
	if params.PartyStart.Anchor != nil {
		anchor = params.PartyStart.Anchor.ToCube()
	}
	floor := canvasFloorHexes(params.Canvas)
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
		hex      core.Hex
		col, row int
		distance int
	}
	candidates := make([]candidate, 0, len(params.Canvas.Cells)-1)
	for _, cell := range params.Canvas.Cells {
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
			int(anchorHex.ToPosition().X), int(anchorHex.ToPosition().Y), seatCount, len(candidates)+1)
	}
	seats := make([]spatial.CubeCoordinate, 0, seatCount)
	seats = append(seats, anchor)
	for index := 0; index < seatCount-1; index++ {
		seats = append(seats, candidates[index].hex.ToCube())
	}
	return partyStartReservation{anchor: anchor, seats: seats}, nil
}
