package encounter_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// orderAsGiven is the initiative roller these tests install.
//
// It returns the members untouched, which is deterministic on purpose: these
// tests assert on what trigger detection DECIDED, and a shuffled order would
// make every assertion about the bubble depend on a roll nobody is testing.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}
