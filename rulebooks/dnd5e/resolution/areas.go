// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// interactionAreas is what one interaction did to the runtime areas, as the
// host applies it: the area a finished cast opens, and the sources whose
// areas end because their concentration ended.
//
// A runtime area and who stands in it are the encounter's. This reads the
// area set the interaction was handed only to say whether an ended
// concentration's caster opened any — a closure that ends nothing is not
// reported — and never asks who is inside one, nor selects one by its spell.
// Closed sources keep the order their concentration ended in, each once.
func interactionAreas(
	standing []encounter.SightAreaData, ended []dnd5eEvents.ConcentrationEndedEvent, outcome Outcome,
) (opened []encounter.SightAreaInput, closed []string) {
	for _, fact := range ended {
		if slices.Contains(closed, fact.CasterID) {
			continue
		}
		if slices.ContainsFunc(standing, func(area encounter.SightAreaData) bool {
			return area.SourceID == fact.CasterID
		}) {
			closed = append(closed, fact.CasterID)
		}
	}
	if cast, ok := outcome.(CastOutcome); ok && cast.openedArea != nil {
		opened = append(opened, *cast.openedArea)
	}
	return opened, closed
}
