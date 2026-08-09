package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CanvasMaxStructuralCells is a defensive runtime implementation capacity for
// canvas structural-floor allocations and walks, not a ratified v0.3 grammar
// maximum.
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

func canvasFloorForParams(params DungeonParams) map[core.Hex]struct{} {
	if params.FloorCells == nil {
		return canvasFloorHexes(params.Width, params.Height)
	}
	floor := make(map[core.Hex]struct{}, len(params.FloorCells))
	for _, hex := range params.FloorCells {
		floor[hex] = struct{}{}
	}
	return floor
}

func canvasFloorForSpace(space *SpaceData) map[core.Hex]struct{} {
	if space.FloorCells == nil {
		return canvasFloorHexes(space.Width, space.Height)
	}
	floor := make(map[core.Hex]struct{}, len(space.FloorCells))
	for _, hex := range space.FloorCells {
		floor[hex] = struct{}{}
	}
	return floor
}

func validateCanonicalFloorCells(width, height int, cells []core.Hex) error {
	var previous floorOrderHex
	seen := make(map[core.Hex]struct{}, len(cells))
	for index, hex := range cells {
		if !hex.ToCube().IsValid() || !canvasFloorContainsDimensions(width, height, hex) {
			return fmt.Errorf("floor cell %d %v is outside canvas bounds", index, hex)
		}
		ordered := floorPlanOrderHex(hex)
		if index > 0 && !previous.less(ordered) {
			return fmt.Errorf("floor cells are not canonical sorted or contain a duplicate at index %d", index)
		}
		if _, duplicate := seen[hex]; duplicate {
			return fmt.Errorf("floor cells contain duplicate %v", hex)
		}
		seen[hex] = struct{}{}
		previous = ordered
	}
	return nil
}

// floorOrderHex gives stable offset-coordinate ordering without making
// dungeonspec's projection types a dependency of encounter.
type floorOrderHex struct{ column, row int }

func floorPlanOrderHex(hex core.Hex) floorOrderHex {
	position := hex.ToPosition()
	return floorOrderHex{column: int(position.X), row: int(position.Y)}
}
func (h floorOrderHex) less(other floorOrderHex) bool {
	return h.column < other.column || h.column == other.column && h.row < other.row
}

func canonicalCanvasFloorCells(params DungeonParams) []core.Hex {
	cells := append([]core.Hex(nil), params.FloorCells...)
	if params.FloorCells == nil {
		for hex := range canvasFloorHexes(params.Width, params.Height) {
			cells = append(cells, hex)
		}
		sort.Slice(cells, func(i, j int) bool { return floorPlanOrderHex(cells[i]).less(floorPlanOrderHex(cells[j])) })
	}
	return cells
}

func canvasEnvelopeEdges(floor map[core.Hex]struct{}) []GeneratedEdge {
	edges := make([]GeneratedEdge, 0)
	seen := make(map[generatedEdgeKey]struct{})
	for owner := range floor {
		for _, neighborCube := range owner.ToCube().GetNeighbors() {
			neighbor := core.HexFromCube(neighborCube)
			if _, isFloor := floor[neighbor]; isFloor {
				continue
			}
			from, to := normalizeAuthoredEndpoints(owner, neighbor)
			key := newGeneratedEdgeKey(from, to)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, GeneratedEdge{From: from, To: to, Kind: GeneratedEdgeKindSolid})
		}
	}
	sort.Slice(edges, func(i, j int) bool { return generatedEdgeLess(edges[i], edges[j]) })
	return edges
}

func generatedEdgesEqual(left, right GeneratedEdge) bool {
	return left.From == right.From && left.To == right.To && left.Kind == right.Kind && left.DoorID == right.DoorID
}

func validateEnvelopeEdges(floor map[core.Hex]struct{}, edges []GeneratedEdge) error {
	expected := canvasEnvelopeEdges(floor)
	if len(edges) != len(expected) {
		return fmt.Errorf("envelope edges: got %d records, want %d", len(edges), len(expected))
	}
	for index := range expected {
		if !generatedEdgesEqual(edges[index], expected[index]) {
			return fmt.Errorf("envelope edge %d is not the canonical floor/void pair", index)
		}
	}
	return nil
}

func validateCanvasDungeonParams(params DungeonParams) error {
	if _, err := ValidateCanvasDimensions(params.Width, params.Height); err != nil {
		return err
	}
	if params.FloorCells != nil {
		if err := validateCanonicalFloorCells(params.Width, params.Height, params.FloorCells); err != nil {
			return fmt.Errorf("canvas floor: %w", err)
		}
	}
	if len(params.Regions) != 0 || len(params.Connectors) != 0 {
		return fmt.Errorf("canvas dungeon must not contain room-chain regions or connectors")
	}
	if params.PartyStart.SeatCount < 0 {
		return fmt.Errorf("party start seat count must not be negative (got %d)", params.PartyStart.SeatCount)
	}
	floor := canvasFloorForParams(params)
	if params.RequireConnectedFloor {
		if len(floor) == 0 {
			return fmt.Errorf("canvas structural floor must not be empty")
		}
		for anchor := range floor {
			if len(connectedFloorComponent(anchor, floor)) != len(floor) {
				return fmt.Errorf("canvas structural floor must be connected")
			}
			break
		}
	}
	if params.EnvelopeEdges != nil {
		if err := validateEnvelopeEdges(floor, params.EnvelopeEdges); err != nil {
			return err
		}
	}
	occupied := make(map[core.Hex]string, len(params.AbsolutePlacedObstacles)+len(params.AbsoluteReservedCells))
	seenIDs := make(map[core.EntityID]int, len(params.AbsolutePlacedObstacles))
	for index, obstacle := range params.AbsolutePlacedObstacles {
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
	if _, err := validateSemanticRegionParams(params.SemanticRegions, floor); err != nil {
		return err
	}
	for index, reserved := range params.AbsoluteReservedCells {
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
	obstacles := make([]ObstacleData, len(params.AbsolutePlacedObstacles))
	for index, obstacle := range params.AbsolutePlacedObstacles {
		obstacles[index] = ObstacleData{
			ID: obstacle.ID, Ref: obstacle.Ref, Position: obstacle.At,
			BlocksMovement: obstacle.BlocksMovement, BlocksLoS: obstacle.BlocksLoS,
			Facing: cloneFacing(obstacle.Facing), Offset: clonePlacementOffset(obstacle.Offset),
		}
	}
	floor := canvasFloorForParams(params)
	semanticRegions, err := validateSemanticRegionParams(params.SemanticRegions, floor)
	if err != nil {
		return nil, err
	}
	return &dungeonLayout{
		width: params.Width, entrance: reservation.anchor, obstacles: obstacles,
		semanticRegions: semanticRegions, partyStartPositions: reservation.positions(),
	}, nil
}

func resolveCanvasPartyStartReservation(params DungeonParams) (partyStartReservation, error) {
	seatCount := params.PartyStart.SeatCount
	if seatCount == 0 {
		seatCount = 1
	}
	floor := canvasFloorForParams(params)
	blockers := make(map[core.Hex]string, len(params.AbsolutePlacedObstacles)+len(params.AbsoluteReservedCells))
	for _, obstacle := range params.AbsolutePlacedObstacles {
		blockers[obstacle.At] = fmt.Sprintf("canvas prop %q", obstacle.ID)
	}
	for _, reserved := range params.AbsoluteReservedCells {
		blockers[reserved.At] = reserved.Name
	}

	anchors := append([]core.Hex(nil), params.FloorCells...)
	if params.FloorCells == nil {
		for hex := range floor {
			anchors = append(anchors, hex)
		}
	}
	sort.Slice(anchors, func(i, j int) bool { return floorPlanOrderHex(anchors[i]).less(floorPlanOrderHex(anchors[j])) })
	if params.PartyStart.Anchor != nil {
		anchors = []core.Hex{*params.PartyStart.Anchor}
	}

	for _, anchorHex := range anchors {
		if _, ok := floor[anchorHex]; !ok {
			continue
		}
		if _, blocked := blockers[anchorHex]; blocked {
			continue
		}
		component := connectedFloorComponent(anchorHex, floor)
		type candidate struct {
			hex                core.Hex
			col, row, distance int
		}
		candidates := make([]candidate, 0, len(component)-1)
		for cell := range component {
			if cell == anchorHex {
				continue
			}
			if _, blocked := blockers[cell]; blocked {
				continue
			}
			pos := cell.ToPosition()
			candidates = append(candidates, candidate{
				hex: cell, col: int(pos.X), row: int(pos.Y),
				distance: anchorHex.ToCube().Distance(cell.ToCube()),
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
			continue
		}
		seats := make([]spatial.CubeCoordinate, 0, seatCount)
		seats = append(seats, anchorHex.ToCube())
		for index := 0; index < seatCount-1; index++ {
			seats = append(seats, candidates[index].hex.ToCube())
		}
		return partyStartReservation{anchor: anchorHex.ToCube(), seats: seats}, nil
	}
	if params.PartyStart.Anchor != nil {
		anchor := *params.PartyStart.Anchor
		if _, ok := floor[anchor]; !ok {
			return partyStartReservation{}, fmt.Errorf("party start anchor %v is outside canvas floor", anchor)
		}
		if blocker, blocked := blockers[anchor]; blocked {
			return partyStartReservation{}, fmt.Errorf("party start anchor %v collides with %s", anchor, blocker)
		}
	}
	return partyStartReservation{}, fmt.Errorf(
		"canvas floor has no same-component party start envelope with %d seats", seatCount,
	)
}

func connectedFloorComponent(anchor core.Hex, floor map[core.Hex]struct{}) map[core.Hex]struct{} {
	component := map[core.Hex]struct{}{anchor: {}}
	queue := []core.Hex{anchor}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range current.ToCube().GetNeighbors() {
			hex := core.HexFromCube(neighbor)
			if _, ok := floor[hex]; !ok {
				continue
			}
			if _, seen := component[hex]; seen {
				continue
			}
			component[hex] = struct{}{}
			queue = append(queue, hex)
		}
	}
	return component
}
