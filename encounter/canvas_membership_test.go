package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func TestCanvasFloorContainsDimensions(t *testing.T) {
	for _, tt := range []struct {
		name string
		hex  core.Hex
		want bool
	}{
		{"origin", core.HexFromPosition(spatial.Position{}), true},
		{"last", core.HexFromPosition(spatial.Position{X: 3, Y: 2}), true},
		{"outside column", core.HexFromPosition(spatial.Position{X: 4, Y: 0}), false},
		{"outside row", core.HexFromPosition(spatial.Position{X: 1, Y: 3}), false},
	} {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, canvasFloorContainsDimensions(4, 3, tt.hex)) })
	}
}
