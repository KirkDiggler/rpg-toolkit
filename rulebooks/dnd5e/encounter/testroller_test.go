package encounter_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// orderAsGiven is the initiative roller these tests install.
//
// It returns the members untouched, which is deterministic on purpose: these
// tests assert on what trigger detection DECIDED, and a shuffled order would
// make every assertion about the bubble depend on a roll nobody is testing.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

// rollsLowest is the reaction die these tests install when they do not care
// which entry fires: it always answers 1, so the FIRST entry of every table
// is the one that fires. Deterministic on purpose — a shuffled pick would
// make every assertion about a threat depend on a roll nobody is testing.
type rollsLowest struct{}

func (rollsLowest) Roll(_ context.Context, _ int) (int, error) { return 1, nil }

func (rollsLowest) RollN(_ context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = 1
	}
	return out, nil
}

// rollsFace is the reaction die a test installs when the face is the point:
// it answers exactly what it was built with, so a test can name the entry it
// expects and say why that face picks it.
type rollsFace struct {
	face int
	// of records the die size the caller was asked to roll, so a test can
	// assert the table's weights were summed rather than counted.
	of *int
}

func (r rollsFace) Roll(_ context.Context, size int) (int, error) {
	if r.of != nil {
		*r.of = size
	}
	return r.face, nil
}

func (r rollsFace) RollN(_ context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		face, err := r.Roll(context.Background(), size)
		if err != nil {
			return nil, err
		}
		out[i] = face
	}
	return out, nil
}

// refusesToRoll is the die a scene installs when the CLAIM is that nothing
// rolls: it fails the test if it is ever asked for a face.
//
// A NIL ROLLER WOULD NOT PROVE IT — the verb refuses one at the door, so a
// nil would prove only that the refusal works. This reaches the roll site and
// reports that it should not have.
type refusesToRoll struct {
	fail func(string)
}

func (r refusesToRoll) Roll(_ context.Context, _ int) (int, error) {
	r.fail("nothing was authored for this outcome, so nothing should have been rolled")
	return 1, nil
}

func (r refusesToRoll) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.fail("nothing was authored for this outcome, so nothing should have been rolled")
	return make([]int, count), nil
}
