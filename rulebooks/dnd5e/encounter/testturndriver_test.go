// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// passDriver is the TurnDriver capability these tests install by default.
//
// Every unplayed member always passes, which is what every fixture written
// before rpg-toolkit#1162 closed was already assuming a monster's turn would
// somehow not need. Installing it explicitly rather than letting a nil mean
// it is the whole point of the capability being required: a scene states what
// it believes an unplayed member does out loud (capabilities are supplied,
// never defaulted).
//
// A thin wrapper over the production [encounter.PassDriver] (rpg-toolkit#1167)
// rather than encounter.PassDriver itself, so a test file that wants ITS OWN
// TurnDriver behaviour can still name this type in a mutate-the-fixture table
// without reaching for the production one by accident.
type passDriver struct{}

func (passDriver) Act(encounter.MonsterView) (encounter.TurnIntent, error) {
	return encounter.Pass{}, nil
}

// errPassStrikerNeverAttacks is what passStriker returns if it is ever
// actually called — see its own doc.
var errPassStrikerNeverAttacks = errors.New("passStriker: passDriver never returns an Attack intent")

// passStriker is the Striker capability these tests install alongside
// passDriver. Every fixture using passDriver never returns an Attack
// intent, so this is never actually called — it exists only because the
// capability is required (rpg-toolkit#1033, rpg-project#254), and says so
// honestly rather than fabricating a hit no test asked for.
type passStriker struct{}

func (passStriker) Strike(context.Context, *encounter.Encounter, encounter.MemberID, encounter.MemberID, core.Ref) error {
	return errPassStrikerNeverAttacks
}
