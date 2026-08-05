package dungeonspec

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const priorCanvasYAML = `version: 1
key: prior-canvas
name: Prior Canvas
canvas: { width: 5, height: 3 }
rooms: []
place:
  - { ref: "dnd5e:props:altar", at: [4, 0] }
walls:
  - { from: [3, 2], to: [4, 2], kind: solid }
start: [3, 1]
`

func canvasCandidate(width int) []byte {
	return []byte(fmt.Sprintf(`version: 1
key: prior-canvas
name: Candidate Canvas
canvas: { width: %d, height: 3 }
rooms: []
`, width))
}

func TestLoadWithPreviousCanvasChecksOrderedSourceOccupancy(t *testing.T) {
	config := LoadConfig{PartyStartSeatCount: 1}
	previous, err := LoadWithConfig([]byte(priorCanvasYAML), config)
	require.NoError(t, err)
	_, err = LoadWithPrevious(canvasCandidate(4), config, previous)
	require.ErrorContains(t, err, "place[0]")

	// Removing the earlier placement makes walls' source-order endpoints the
	// first offender; from is checked before to.
	withoutPlace := strings.Replace(priorCanvasYAML, "place:\n  - { ref: \"dnd5e:props:altar\", at: [4, 0] }\n", "", 1)
	previous, err = LoadWithConfig([]byte(withoutPlace), config)
	require.NoError(t, err)
	_, err = LoadWithPrevious(canvasCandidate(3), config, previous)
	require.ErrorContains(t, err, "walls[0].from")

	// With from retained but to outside, the second endpoint is named.
	wallToOutside := strings.Replace(withoutPlace, "from: [3, 2], to: [4, 2]", "from: [2, 2], to: [3, 2]", 1)
	previous, err = LoadWithConfig([]byte(wallToOutside), config)
	require.NoError(t, err)
	_, err = LoadWithPrevious(canvasCandidate(3), config, previous)
	require.ErrorContains(t, err, "walls[0].to")

	withoutWalls := strings.Replace(withoutPlace, "walls:\n  - { from: [3, 2], to: [4, 2], kind: solid }\n", "", 1)
	previous, err = LoadWithConfig([]byte(withoutWalls), config)
	require.NoError(t, err)
	_, err = LoadWithPrevious(canvasCandidate(3), config, previous)
	require.ErrorContains(t, err, "start")
}

func TestLoadWithPreviousCanvasInternalRegionCellFixture(t *testing.T) {
	config := LoadConfig{PartyStartSeatCount: 1}
	previous, err := LoadWithConfig([]byte(priorCanvasYAML), config)
	require.NoError(t, err)
	// This is intentionally private test-only state: v0.3 has no regions
	// grammar, and callers never reconstruct this occupancy through exported API.
	previous.canvas.occupancy = nil
	previous.canvas.regionCells = []namedCanvasCell{{name: "synthetic-region-cell", cell: [2]int{4, 2}}}
	_, err = LoadWithPrevious(canvasCandidate(4), config, previous)
	require.ErrorContains(t, err, `region cell "synthetic-region-cell" at [4,2]`)

	_, err = LoadWithPrevious(canvasCandidate(6), config, previous)
	require.NoError(t, err)
}
