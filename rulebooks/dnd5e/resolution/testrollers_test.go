package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// queuedRoller consumes one shared queue across Roll and RollN. Damage-pool
// tests use it to prove the exact amount and order of randomness a strike
// consumes rather than giving every dice API a reusable fallback answer.
type queuedRoller struct {
	values []int
	rolls  int
}

func scripted(values ...int) *queuedRoller {
	return &queuedRoller{values: append([]int(nil), values...)}
}

func (r *queuedRoller) Roll(_ context.Context, sides int) (int, error) {
	values, err := r.take(1, sides)
	if err != nil {
		return 0, err
	}

	return values[0], nil
}

func (r *queuedRoller) RollN(_ context.Context, count, sides int) ([]int, error) {
	return r.take(count, sides)
}

func (r *queuedRoller) take(count, sides int) ([]int, error) {
	if len(r.values) < count {
		return nil, fmt.Errorf("scripted roller exhausted: need %d d%d, have %d values", count, sides, len(r.values))
	}

	out := append([]int(nil), r.values[:count]...)
	r.values = r.values[count:]
	r.rolls += count

	return out, nil
}

// orderAsGiven is the deterministic InitiativeRoller every fixture wires.
//
// The composition requires one to load at all (rpg-toolkit#964), and these
// tests are about interactions rather than about who acts first — so it hands
// back the order it was given and stays out of the way.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

// everyoneStanding is the deterministic Standing every fixture wires.
//
// The composition requires one to load at all (rpg-toolkit#1077), and this
// package never consults it — it carries the caller's capability across a
// round trip through the world. So the fixtures hand over the one answer that
// changes nothing, and any test that came to depend on a different one would
// be testing a question this package does not ask.
type everyoneStanding struct{}

func (everyoneStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// unlimitedSight is the range these fixtures hand out. Further than the longest
// sightline any field in this package draws, because light is not this
// package's subject — and stated as a number rather than left to a default,
// which is the whole reason the capability is required (rpg-toolkit#1033).
const unlimitedSight = 1_000_000

// everyoneSeesTheWholeMap is the deterministic Sight every fixture wires.
//
// The composition requires one to load at all (rpg-toolkit#1111), and this
// package never consults it — it carries the caller's capability across a
// round trip through the world, exactly as it carries everyoneStanding above.
// So the fixtures hand over the one answer that changes nothing, and a test
// that came to depend on a different one would be testing a question this
// package does not ask.
type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
	}

	return out, nil
}
