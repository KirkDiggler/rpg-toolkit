package character

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// countingBridgeRoller pins the face a roll comes back with and counts the
// calls made to it.
type countingBridgeRoller struct {
	face  int
	calls int
}

func (r *countingBridgeRoller) Roll(_ context.Context, _ int) (int, error) {
	r.calls++
	return r.face, nil
}

func (r *countingBridgeRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.calls++
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

var _ dice.Roller = (*countingBridgeRoller)(nil)
