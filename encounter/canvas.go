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

// CanvasParams configures a canvas-mode dungeon. It is constructor input only;
// persistence uses the distinct CanvasData shape.
type CanvasParams struct {
	Width  int
	Height int
}

// CanvasData is the persisted canvas-mode marker and dimensions. Structural
// floor cells are derived from these validated dimensions and are never stored.
type CanvasData struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NewCanvasParams validates and constructs canvas input. It never returns an
// invalid Params value for a caller to discover later at InitDungeon.
func NewCanvasParams(width, height int) (*CanvasParams, error) {
	if _, err := ValidateCanvasDimensions(width, height); err != nil {
		return nil, err
	}
	return &CanvasParams{Width: width, Height: height}, nil
}

func (p *CanvasParams) toData() *CanvasData {
	if p == nil {
		return nil
	}
	return &CanvasData{Width: p.Width, Height: p.Height}
}

// canvasParamsFromData is the Data-to-runtime conversion owned by the
// encounter aggregate reload boundary.
func canvasParamsFromData(data *CanvasData) (*CanvasParams, error) {
	if data == nil {
		return nil, fmt.Errorf("canvas data is required")
	}
	return NewCanvasParams(data.Width, data.Height)
}

func validateCanvasParams(params *CanvasParams) error {
	if params == nil {
		return fmt.Errorf("canvas params are required")
	}
	_, err := ValidateCanvasDimensions(params.Width, params.Height)
	return err
}

func canvasFloorHexes(params *CanvasParams) map[core.Hex]struct{} {
	count, _ := ValidateCanvasDimensions(params.Width, params.Height)
	floor := make(map[core.Hex]struct{}, count)
	for col := 0; col < params.Width; col++ {
		for row := 0; row < params.Height; row++ {
			floor[core.HexFromPosition(spatial.Position{X: float64(col), Y: float64(row)})] = struct{}{}
		}
	}
	return floor
}

func canvasDataContainsHex(source *CanvasData, hex core.Hex) bool {
	if source == nil {
		return false
	}
	return canvasFloorContainsDimensions(source.Width, source.Height, hex)
}

func canvasFloorContainsHex(source *CanvasParams, hex core.Hex) bool {
	if source == nil {
		return false
	}
	return canvasFloorContainsDimensions(source.Width, source.Height, hex)
}

func canvasFloorContainsDimensions(width, height int, hex core.Hex) bool {
	if hex.Q < 0 || hex.Q >= width {
		return false
	}

	// Pointy-top odd-q conversion: col=q and row=s+(q-(q&1))/2. Bound
	// s before adding the offset so arbitrary external coordinates cannot
	// overflow the conversion.
	offset := (hex.Q - (hex.Q & 1)) / 2
	if hex.S < -offset || hex.S >= height-offset {
		return false
	}
	return hex.R == -hex.Q-hex.S
}

func validateCanvasDungeonParams(params DungeonParams) error {
	if err := validateCanvasParams(params.Canvas); err != nil {
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
	candidates := make([]candidate, 0, params.Canvas.Width*params.Canvas.Height-1)
	for col := 0; col < params.Canvas.Width; col++ {
		for row := 0; row < params.Canvas.Height; row++ {
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
			int(anchorHex.ToPosition().X), int(anchorHex.ToPosition().Y), seatCount, len(candidates)+1)
	}
	seats := make([]spatial.CubeCoordinate, 0, seatCount)
	seats = append(seats, anchor)
	for index := 0; index < seatCount-1; index++ {
		seats = append(seats, candidates[index].hex.ToCube())
	}
	return partyStartReservation{anchor: anchor, seats: seats}, nil
}
