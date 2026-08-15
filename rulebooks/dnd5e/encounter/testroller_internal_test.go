package encounter

// orderAsGiven is the initiative roller the internal tests install.
//
// It returns the members untouched, which is deterministic on purpose: these
// tests assert on what trigger detection DECIDED, and a shuffled order would
// make every assertion about the bubble depend on a roll nobody is testing.
// The rulebook's real roller is the production consumer's business.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []MemberID) ([]MemberID, error) {
	return members, nil
}
