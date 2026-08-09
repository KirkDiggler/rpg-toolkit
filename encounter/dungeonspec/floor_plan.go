package dungeonspec

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// FloorPlanCell is an absolute pointy-top odd-q floor coordinate.
type FloorPlanCell struct {
	Column int
	Row    int
}

// FloorPlanEdgeKind identifies a physical edge in a FloorPlan.
type FloorPlanEdgeKind string

const (
	// FloorPlanEdgeKindSolid is an impassable barrier.
	FloorPlanEdgeKindSolid FloorPlanEdgeKind = "solid"
	// FloorPlanEdgeKindDoor is an edge with a stable door identity.
	FloorPlanEdgeKindDoor FloorPlanEdgeKind = "door"
)

// FloorPlanEdge is one canonical effective runtime edge.
type FloorPlanEdge struct {
	From   FloorPlanCell
	To     FloorPlanCell
	Kind   FloorPlanEdgeKind
	DoorID string
}

// FloorPlanRoom is room-chain projection metadata.
type FloorPlanRoom struct {
	ID          string
	Archetype   string
	StartColumn int
	Width       int
}

// FloorPlanConnector is a projected room-chain connector.
type FloorPlanConnector struct {
	DoorID     string
	Locked     bool
	FromRoomID string
	ToRoomID   string
	Column     int
}

// FloorPlanRegion is one authoring semantic scope. ParentID is derived and
// nil for root-parented/empty scopes; Cells are canonical absolute extents.
type FloorPlanRegion struct {
	ID        string
	Name      *string
	Archetype *string
	Cells     []FloorPlanCell
	ParentID  *string
}

// FloorSource is the resolved authored canvas floor source.
type FloorSource string

const (
	// FloorSourceBounds resolves omission and explicit bounds to the v0.3 rectangle.
	FloorSourceBounds FloorSource = "bounds"
	// FloorSourceRegions resolves floor from the canonical region-cell union.
	FloorSourceRegions FloorSource = "regions"
)

// FloorPlan is the provider-owned authoring projection.
type FloorPlan struct {
	FloorSource FloorSource
	Rooms       []FloorPlanRoom
	Regions     []FloorPlanRegion
	Connectors  []FloorPlanConnector
	// Width is the canvas width only; room chains leave it zero to preserve
	// the wire contract's canvas-only field semantics.
	Width   int
	Height  int
	DoorRow int
	// FloorCells is the complete canonical canvas structural-floor projection. It is
	// nil for room chains because their region-only data omits connector cells.
	FloorCells []FloorPlanCell
	Entrance   *FloorPlanCell
	Edges      []FloorPlanEdge
	// Placements is the authoring-only projection; it carries no runtime identity.
	Placements []CompiledPlacement
}

// BuildFloorPlanInput supplies a compiled specification and preview seed.
type BuildFloorPlanInput struct {
	Compiled CompiledDungeon
	Seed     int64
}

func validateCompiledRuntime(ctx context.Context, compiled CompiledDungeon, seed int64) error {
	params := compiled.Params
	params.RandomSeed = seed
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	defer func() { _ = broker.Close(); _ = transport.Close() }()
	preview := encounter.New(ctx, "dungeon-runtime-validation", broker)
	if err := preview.InitDungeon(params); err != nil {
		return fmt.Errorf("init dungeon: %w", err)
	}
	return nil
}

// BuildFloorPlan projects canonical compiler facts for canvas candidates and
// initialized runtime facts for legacy room-chain candidates.
func BuildFloorPlan(ctx context.Context, in BuildFloorPlanInput) (FloorPlan, error) {
	if in.Compiled.canvas != nil && in.Compiled.canvas.floorSource == FloorSourceRegions {
		return buildCanvasFloorPlan(in.Compiled), nil
	}
	params := in.Compiled.Params
	params.RandomSeed = in.Seed
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	defer func() {
		_ = broker.Close()
		_ = transport.Close()
	}()
	preview := encounter.New(ctx, "dungeon-floor-plan-preview", broker)
	if err := preview.InitDungeon(params); err != nil {
		return FloorPlan{}, fmt.Errorf("init dungeon: %w", err)
	}
	data := preview.ToData()
	if data.Space == nil {
		return FloorPlan{}, fmt.Errorf("init dungeon: no persisted space")
	}
	entrance := cellFromHex(data.Space.Entrance)
	plan := FloorPlan{
		FloorSource: FloorSourceBounds, Height: data.Space.Height, Entrance: &entrance,
		Placements: cloneCompiledPlacements(in.Compiled.Placements),
	}

	if data.Space.FloorSource == encounter.FloorSourceRoomChain {
		plan.DoorRow = data.Space.Height / 2
		startColumn := 0
		for index, region := range params.Regions {
			plan.Rooms = append(plan.Rooms, FloorPlanRoom{
				ID: region.ID, Archetype: string(region.Archetype), StartColumn: startColumn, Width: region.Width,
			})
			if index < len(params.Connectors) {
				connector := params.Connectors[index]
				plan.Connectors = append(plan.Connectors, FloorPlanConnector{
					DoorID: string(connector.DoorID), Locked: connector.Locked,
					FromRoomID: region.ID, ToRoomID: params.Regions[index+1].ID,
					Column: startColumn + region.Width,
				})
			}
			startColumn += region.Width + 1
		}
	}
	if data.Space.FloorSource == encounter.FloorSourceCanvas {
		plan.Width = data.Space.Width
		for _, region := range data.Space.SemanticRegions {
			projected := FloorPlanRegion{ID: region.ID, Name: cloneString(region.Name)}
			if region.Archetype != nil {
				value := string(*region.Archetype)
				projected.Archetype = &value
			}
			if parent, ok := data.Space.SemanticRegionParent(region.ID); ok {
				projected.ParentID = &parent
			}
			for cell := range region.Cells {
				projected.Cells = append(projected.Cells, cellFromHex(cell))
			}
			sort.Slice(projected.Cells, func(i, j int) bool { return floorPlanCellLess(projected.Cells[i], projected.Cells[j]) })
			plan.Regions = append(plan.Regions, projected)
		}
		cells, err := runtimeCanvasFloorCells(data.Space.Width, data.Space.Height)
		if err != nil {
			return FloorPlan{}, fmt.Errorf("validate canvas floor cells: %w", err)
		}
		plan.FloorCells = cells
	}

	described, err := preview.DescribeEdges(encounter.DescribeEdgesInput{})
	if err != nil {
		return FloorPlan{}, fmt.Errorf("describe canonical edges: %w", err)
	}
	plan.Edges = make([]FloorPlanEdge, len(described.Edges))
	for index, edge := range described.Edges {
		from, to := cellFromHex(edge.From), cellFromHex(edge.To)
		if floorPlanCellLess(to, from) {
			from, to = to, from
		}
		plan.Edges[index] = FloorPlanEdge{
			From: from, To: to, Kind: FloorPlanEdgeKind(edge.Kind), DoorID: string(edge.DoorID),
		}
	}
	sort.Slice(plan.Edges, func(i, j int) bool {
		if plan.Edges[i].From != plan.Edges[j].From {
			return floorPlanCellLess(plan.Edges[i].From, plan.Edges[j].From)
		}
		if plan.Edges[i].To != plan.Edges[j].To {
			return floorPlanCellLess(plan.Edges[i].To, plan.Edges[j].To)
		}
		if plan.Edges[i].Kind != plan.Edges[j].Kind {
			return plan.Edges[i].Kind < plan.Edges[j].Kind
		}
		return plan.Edges[i].DoorID < plan.Edges[j].DoorID
	})
	return plan, nil
}

func buildCanvasFloorPlan(compiled CompiledDungeon) FloorPlan {
	canvas := compiled.canvas
	plan := FloorPlan{
		FloorSource: canvas.floorSource, Width: canvas.width, Height: canvas.height,
		FloorCells: append([]FloorPlanCell(nil), canvas.floorCells...), Entrance: cloneFloorPlanCell(canvas.entrance),
		Regions:    append([]FloorPlanRegion(nil), canvas.regions...),
		Placements: cloneCompiledPlacements(compiled.Placements),
	}
	for _, edge := range canvas.envelope {
		from, to := cellFromHex(edge.From), cellFromHex(edge.To)
		if floorPlanCellLess(to, from) {
			from, to = to, from
		}
		plan.Edges = append(plan.Edges, FloorPlanEdge{From: from, To: to, Kind: FloorPlanEdgeKindSolid})
	}
	for _, edge := range compiled.Params.AuthoredEdges {
		from, to := cellFromHex(edge.From), cellFromHex(edge.To)
		if floorPlanCellLess(to, from) {
			from, to = to, from
		}
		plan.Edges = append(plan.Edges, FloorPlanEdge{
			From: from, To: to, Kind: FloorPlanEdgeKind(edge.Kind), DoorID: string(edge.DoorID),
		})
	}
	sort.Slice(plan.Edges, func(i, j int) bool {
		if plan.Edges[i].From != plan.Edges[j].From {
			return floorPlanCellLess(plan.Edges[i].From, plan.Edges[j].From)
		}
		if plan.Edges[i].To != plan.Edges[j].To {
			return floorPlanCellLess(plan.Edges[i].To, plan.Edges[j].To)
		}
		if plan.Edges[i].Kind != plan.Edges[j].Kind {
			return plan.Edges[i].Kind < plan.Edges[j].Kind
		}
		return plan.Edges[i].DoorID < plan.Edges[j].DoorID
	})
	return plan
}

func cloneFloorPlanCell(value *FloorPlanCell) *FloorPlanCell {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func floorPlanCellLess(left, right FloorPlanCell) bool {
	return left.Column < right.Column || left.Column == right.Column && left.Row < right.Row
}

func runtimeCanvasFloorCells(width, height int) ([]FloorPlanCell, error) {
	cellCount, err := encounter.ValidateCanvasDimensions(width, height)
	if err != nil {
		return nil, err
	}
	cells := make([]FloorPlanCell, 0, cellCount)
	for column := 0; column < width; column++ {
		for row := 0; row < height; row++ {
			cells = append(cells, FloorPlanCell{Column: column, Row: row})
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Column != cells[j].Column {
			return cells[i].Column < cells[j].Column
		}
		return cells[i].Row < cells[j].Row
	})
	return cells, nil
}

func cloneCompiledPlacements(placements []CompiledPlacement) []CompiledPlacement {
	if placements == nil {
		return nil
	}
	clones := make([]CompiledPlacement, len(placements))
	for index, placement := range placements {
		clones[index] = placement
		clones[index].Offset = clonePlacementOffset(placement.Offset)
		if placement.Facing != nil {
			facing := *placement.Facing
			clones[index].Facing = &facing
		}
	}
	return clones
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cellFromHex(h core.Hex) FloorPlanCell {
	position := h.ToPosition()
	return FloorPlanCell{Column: int(position.X), Row: int(position.Y)}
}
