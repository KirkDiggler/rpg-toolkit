// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Driver names the encounter's canonical ordinary behavior selection. It is
// stateless: Launch carries compiled configuration, the encounter owns progress,
// and each verb supplies shared dice through its ordinary capabilities. A
// compulsion wrapper is consulted before this ordinary choice.
func Driver() TurnDriver { return behaviorDriver{} }

// behaviorDriver is a marker, never a second brain behind the host's twin view.
// Custom host drivers retain their existing projection path; canonical behavior
// data/progress and selected-row arithmetic need not leak through that view.
type behaviorDriver struct{}

var _ TurnDriver = behaviorDriver{}

// Act refuses accidental invocation through the host-facing projection. The
// manager resolves this marker to BehaviorDriver on the owning encounter seam.
func (behaviorDriver) Act(view MonsterView) (TurnIntent, error) {
	return nil, fmt.Errorf("member %q: canonical behavior is selected on the encounter view, not this twin: %w", view.Self, ErrBadTurnOutcome)
}

// encounterDriverFor resolves only the ordinary marker. The write scope's
// compelledDriver remains outside it and can return an imposed intent first.
func encounterDriverFor(driver TurnDriver) encounter.Driver {
	if _, ok := driver.(behaviorDriver); ok {
		return encounter.BehaviorDriver{}
	}
	return turnDriverSeam{driver: driver}
}
