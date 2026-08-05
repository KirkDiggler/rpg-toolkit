package dungeonspec

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// FloorPlanCell is an absolute pointy-top odd-q floor coordinate.
type FloorPlanCell struct {
	Column int
	Row    int
}

// FloorPlanEdgeKind identifies a physical edge in a FloorPlan.
type FloorPlanEdgeKind string

const (
	// FloorPlanEdgeKindSolid is an impassable authored edge.
	FloorPlanEdgeKindSolid FloorPlanEdgeKind = "solid"
	// FloorPlanEdgeKindDoor is a door edge with a stable ID.
	FloorPlanEdgeKindDoor FloorPlanEdgeKind = "door"
)

// FloorPlanEdge is one canonical effective authored edge.
type FloorPlanEdge struct {
	From   FloorPlanCell
	To     FloorPlanCell
	Kind   FloorPlanEdgeKind
	DoorID string
}

// FloorPlanRoom is room-chain projection metadata.
type FloorPlanRoom struct {
	ID          string
	StartColumn int
	Width       int
}

// FloorPlanConnector is a projected room-chain connector.
type FloorPlanConnector struct{ DoorID string }

// FloorPlan is the provider-owned authoring projection.
type FloorPlan struct {
	Rooms      []FloorPlanRoom
	Connectors []FloorPlanConnector
	Width      int
	Height     int
	FloorCells []FloorPlanCell
	Entrance   FloorPlanCell
	Edges      []FloorPlanEdge
}

// BuildFloorPlanInput supplies a compiled specification. Seed is reserved for generated geometry.
type BuildFloorPlanInput struct {
	Compiled CompiledDungeon
	Seed     int64
}

// BuildFloorPlan projects canonical floor-source and authored-edge facts without host geometry work.
func BuildFloorPlan(_ context.Context, in BuildFloorPlanInput) (FloorPlan, error) {
	c := in.Compiled
	p := FloorPlan{Height: c.Params.Height}
	if c.canvas != nil {
		p.Width = c.canvas.width
		p.Height = c.canvas.height
		p.Entrance = c.canvas.entrance
		p.FloorCells = canvasCells(p.Width, p.Height)
	} else {
		offset := 0
		for _, r := range c.Params.Regions {
			p.Rooms = append(p.Rooms, FloorPlanRoom{ID: r.ID, StartColumn: offset, Width: r.Width})
			for col := 0; col < r.Width; col++ {
				for row := 0; row < p.Height; row++ {
					p.FloorCells = append(p.FloorCells, FloorPlanCell{offset + col, row})
				}
			}
			offset += r.Width + 1
		}
		p.Width = offset - 1
		for _, connector := range c.Params.Connectors {
			p.Connectors = append(p.Connectors, FloorPlanConnector{DoorID: string(connector.DoorID)})
		}
		p.Entrance = FloorPlanCell{0, p.Height / 2}
		if c.Params.PartyStart.Anchor != nil {
			p.Entrance = cellFromHex(*c.Params.PartyStart.Anchor)
		}
	}
	for _, edge := range c.Params.AuthoredEdges {
		p.Edges = append(p.Edges, FloorPlanEdge{From: cellFromHex(edge.From), To: cellFromHex(edge.To), Kind: FloorPlanEdgeKind(edge.Kind), DoorID: string(edge.DoorID)})
	}
	sort.Slice(p.Edges, func(i, j int) bool {
		a, b := p.Edges[i], p.Edges[j]
		if a.From != b.From {
			return a.From.Column < b.From.Column || a.From.Column == b.From.Column && a.From.Row < b.From.Row
		}
		return a.To.Column < b.To.Column || a.To.Column == b.To.Column && a.To.Row < b.To.Row
	})
	return p, nil
}

func canvasCells(width, height int) []FloorPlanCell {
	if width < 1 || height < 1 {
		return nil
	}
	cells := make([]FloorPlanCell, 0, width*height)
	for c := 0; c < width; c++ {
		for r := 0; r < height; r++ {
			cells = append(cells, FloorPlanCell{c, r})
		}
	}
	return cells
}
func cellFromHex(h core.Hex) FloorPlanCell {
	p := h.ToPosition()
	return FloorPlanCell{Column: int(p.X), Row: int(p.Y)}
}
func canvasHex(c FloorPlanCell) core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(c.Column), Y: float64(c.Row)})
}
