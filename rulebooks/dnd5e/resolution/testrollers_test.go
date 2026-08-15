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
