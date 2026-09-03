package resolution

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const (
	secondWolfID = "wolf-2"
	hitRoll      = 15
)

type sequenceRoller struct {
	singles []int
	pair    []int
}

func (r *sequenceRoller) Roll(_ context.Context, _ int) (int, error) {
	if len(r.singles) > 0 {
		next := r.singles[0]
		r.singles = r.singles[1:]
		return next, nil
	}
	return 0, errors.New("sequence roller: scripted singles exhausted")
}

func (r *sequenceRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	if len(r.pair) >= count {
		next := append([]int(nil), r.pair[:count]...)
		r.pair = r.pair[count:]
		return next, nil
	}
	return nil, errors.New("sequence roller: scripted group exhausted")
}

func TestSequenceRollerRefusesExhaustedScripts(t *testing.T) {
	roller := &sequenceRoller{}

	_, err := roller.Roll(context.Background(), 20)
	require.Error(t, err)
	_, err = roller.RollN(context.Background(), 2, 6)
	require.Error(t, err)
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
//
// The same value carries the participation answer the encounter's load door
// asks for since members became scheduled by participation (rpg-toolkit#1453):
// nobody down, nobody removed, everybody waiting for a driver — the answer
// these fixtures always assumed about hit points, stated out loud.
type everyoneStanding struct{}

func (everyoneStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// Assess answers one complete participation question: every member present,
// conscious, in contact, waiting.
func (everyoneStanding) Assess(
	members []encounter.MemberID,
) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait,
		})
	}
	return assessment, nil
}

var _ encounter.StandingWithParticipation = everyoneStanding{}

// passDriver is the deterministic TurnDriver every fixture wires.
//
// The composition requires one to load at all (rpg-toolkit#1162), and this
// package never consults it — it carries the caller's capability across a
// round trip through the world, exactly like Standing and Sight above. So the
// fixtures hand over the one answer that changes nothing.
type passDriver struct{}

func (passDriver) Act(encounter.MonsterView) (encounter.TurnIntent, error) {
	return encounter.Pass{}, nil
}

// noAttacksExpected is the deterministic Striker every fixture wires. This
// package's own fixtures never drive a monster's turn through the
// composition (passDriver never returns anything but Pass), so this is
// never actually called — required at construction regardless
// (rpg-project#254), and it says so honestly rather than fabricating a hit.
type noAttacksExpected struct{}

func (noAttacksExpected) Strike(
	context.Context, *encounter.Encounter, encounter.MemberID, encounter.MemberID, core.Ref,
) error {
	return errors.New("resolution fixtures: no scene here ever attacks")
}

// quietAnnouncer hears every boundary and does nothing with it.
//
// UNLIKE noAttacksExpected this really is called — any pair of fixtures
// standing in contact forms a bubble at first light, and forming one starts
// round 1 and somebody's turn. It succeeds silently because these fixtures are
// about what Resolve does with a world, not about what a turn boundary means
// to a condition; boundary_test.go uses a recording one.
type quietAnnouncer struct{}

func (quietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
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
