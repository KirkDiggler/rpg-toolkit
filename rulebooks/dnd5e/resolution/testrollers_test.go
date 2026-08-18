package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

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
