package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func TestCanvasFloorContainsHex(t *testing.T) {
	source, err := NewCanvasParams(4, 3)
	require.NoError(t, err)
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1

	tests := []struct {
		name string
		hex  core.Hex
		want bool
	}{
		{name: "inside even column", hex: core.HexFromPosition(spatial.Position{X: 2, Y: 2}), want: true},
		{name: "inside odd column", hex: core.HexFromPosition(spatial.Position{X: 3, Y: 2}), want: true},
		{name: "outside row", hex: core.HexFromPosition(spatial.Position{X: 3, Y: 3})},
		{name: "outside column", hex: core.HexFromPosition(spatial.Position{X: 4, Y: 0})},
		{name: "noncanonical cube", hex: core.Hex{Q: 1, R: 0, S: 0}},
		{name: "maximum column", hex: core.Hex{Q: maxInt, R: 0, S: 0}},
		{name: "minimum column", hex: core.Hex{Q: minInt, R: 0, S: 0}},
		{name: "maximum row coordinate", hex: core.Hex{Q: 0, R: minInt, S: maxInt}},
		{name: "minimum row coordinate", hex: core.Hex{Q: 0, R: maxInt, S: minInt}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, canvasFloorContainsHex(source, tt.hex))
		})
	}
}
