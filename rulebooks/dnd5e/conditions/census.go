// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// censusClass says how one loaded effect bears on an action: in the action
// census, on its holder's own action; in the target census, on an attack
// against its holder.
type censusClass string

const (
	// censusAnswers marks an effect whose rule answers through
	// contributions.ActionAssessor.
	censusAnswers censusClass = "answers"
	// censusNotBearing marks an effect that does not bear on the action. It
	// yields no row.
	censusNotBearing censusClass = "not_bearing"
	// censusNotYetAnswering marks an effect that bears on the action but whose
	// rule cannot yet answer. It is shown unavailable, never dropped and never
	// shown as not applying.
	censusNotYetAnswering censusClass = "not_yet_answering"
)

// censusEntry classifies one loader ref. Participation is empty for an
// effect that does not bear.
type censusEntry struct {
	class         censusClass
	participation contributions.Participation
}

var (
	answersNow      = censusEntry{class: censusAnswers, participation: contributions.ContributesNow}
	answersLater    = censusEntry{class: censusAnswers, participation: contributions.LaterChoice}
	notYetAnswering = censusEntry{class: censusNotYetAnswering, participation: contributions.ContributesNow}
	notBearing      = censusEntry{class: censusNotBearing}
)

// bearingRefs lists, sorted, a census's refs that are not classed as not
// bearing.
func bearingRefs(census map[string]censusEntry) []string {
	bearing := make([]string, 0, len(census))
	for ref, entry := range census {
		if entry.class != censusNotBearing {
			bearing = append(bearing, ref)
		}
	}
	slices.Sort(bearing)
	return bearing
}
