package spatial_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func ExamplePlacedCoverage() {
	emb := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{CellWidth: 5})
	placement := spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 4, D: 10}},
		Origin:    spatial.Point{X: 3.25, Y: -1.75}, Facing: 37,
	}
	coverage, err := spatial.PlacedCoverage(spatial.PlacedCoverageInput{
		Embedding: emb, Placement: placement,
		Cells: []spatial.Position{{}, {X: 1}, {Y: -1}},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
		Placement: placement,
		From:      spatial.Point{X: -5}, To: spatial.Point{X: 8},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(coverage.Cells, trace.Contact, trace.Interior)
}
