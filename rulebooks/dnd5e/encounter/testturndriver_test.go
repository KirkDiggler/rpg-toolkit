// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// passDriver is the TurnDriver capability these tests install by default.
//
// Every unplayed member always passes, which is what every fixture written
// before rpg-toolkit#1162 closed was already assuming a monster's turn would
// somehow not need. Installing it explicitly rather than letting a nil mean
// it is the whole point of the capability being required: a scene states what
// it believes an unplayed member does out loud (capabilities are supplied,
// never defaulted).
type passDriver struct{}

func (passDriver) Act(encounter.MemberID) (encounter.TurnOutcome, error) {
	return encounter.Pass{}, nil
}
