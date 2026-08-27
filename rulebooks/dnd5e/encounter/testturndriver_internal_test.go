// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// passDriver is the internal-package twin of encounter_test's own — see its
// doc. A separate type because the internal test package cannot import the
// external one (and does not need to: this file's whole job is being small).
type passDriver struct{}

func (passDriver) Act(MonsterView) (TurnIntent, error) {
	return Pass{}, nil
}

// errPassStrikerNeverAttacks is what passStriker returns if it is ever
// actually called — see its own doc.
var errPassStrikerNeverAttacks = errors.New("passStriker: passDriver never returns an Attack intent")

// passStriker is the internal-package twin of encounter_test's own — see its
// doc.
type passStriker struct{}

func (passStriker) Strike(context.Context, *Encounter, MemberID, MemberID, core.Ref) error {
	return errPassStrikerNeverAttacks
}

// quietAnnouncer is the internal-package twin of encounter_test's own — see
// its doc.
type quietAnnouncer struct{}

func (quietAnnouncer) Announce(context.Context, *Encounter, []Boundary) error {
	return nil
}
