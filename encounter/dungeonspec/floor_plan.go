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

// FloorPlan is the provider-owned authoring projection.
type FloorPlan struct {
	Rooms      []FloorPlanRoom
	Connectors []FloorPlanConnector
	// Width is the canvas width only; room chains leave it zero to preserve
	// the wire contract's canvas-only field semantics.
	Width   int
	Height  int
	DoorRow int
	// FloorCells is the v0.3 canvas structural-floor projection only. It is
	// nil for room chains because their region-only data omits connector cells.
	FloorCells []FloorPlanCell
	Entrance   FloorPlanCell
	Edges      []FloorPlanEdge
}

// BuildFloorPlanInput supplies a compiled specification and preview seed.
type BuildFloorPlanInput struct {
	Compiled CompiledDungeon
	Seed     int64
}

// BuildFloorPlan initializes a throwaway toolkit encounter and projects its
// persisted spatial facts. It intentionally does not derive identity, floor
// membership, generated edges, or authored overlays from CompiledDungeon: the
// initialized encounter is the same runtime truth used by real startup.
func BuildFloorPlan(ctx context.Context, in BuildFloorPlanInput) (FloorPlan, error) {
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
	plan := FloorPlan{Height: data.Space.Height, Entrance: cellFromHex(data.Space.Entrance)}

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

func cellFromHex(h core.Hex) FloorPlanCell {
	position := h.ToPosition()
	return FloorPlanCell{Column: int(position.X), Row: int(position.Y)}
}
