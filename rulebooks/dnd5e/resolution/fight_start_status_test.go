// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// heldLibrary is the Keep's hold as the status compiler writes it: it ends when
// a fight its holder is in starts (rpg-project#990), and grants advantage on
// Strength checks meanwhile so the sheet has a benefit to hold.
var heldLibrary = strings.NewReplacer(
	`"beat":"run_end","reason":"run_ended"`, `"beat":"combat_start","reason":"fight_started"`,
	`"well-fed":{"name":"Well fed"`, `"held":{"name":"Held"`,
).Replace(wellFedLibrary)

// A HOLD ENDS WHEN ITS HOLDER'S FIGHT STARTS, through resolution's own
// boundary entry: the fight starting for the hero ends the hero's status for
// its authored reason, and the bard, whose fight has not started, keeps theirs.
func (s *GiveStatusTestSuite) TestAFightStartingEndsAHoldForItsHolderOnly() {
	s.Require().Contains(heldLibrary, `"beat":"combat_start"`, "precondition: the hold ends on a fight starting")
	fixtures := s.fixtures()
	give := func(interaction *Input, member, application string) *Output {
		out, err := GiveStatus(s.ctx, &GiveStatusInput{
			Interaction: interaction, Member: member,
			Library: json.RawMessage(heldLibrary), StatusID: "held",
			SourceID: "hall-1", Source: cookRef(), ApplicationID: application,
		})
		s.Require().NoError(err)
		return out.Resolved
	}
	hero := fixtures.sheet(give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}), heroID, "held:hero"), heroID)
	bard := fixtures.sheet(give(s.interaction(fixtures,
		Participant{Character: hero}, Participant{Character: fixtures.bard(1)}), bardID, "held:bard"), bardID)
	s.Require().NotEmpty(hero.Conditions)
	s.Require().NotEmpty(bard.Conditions)

	interaction := s.interaction(fixtures, Participant{Character: hero}, Participant{Character: bard})
	machine, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: combatStarted, Subject: heroID, Round: 1}}})
	s.Require().NoError(err)
	interaction.Machine = machine
	out, err := Resolve(s.ctx, interaction)
	s.Require().NoError(err)

	s.Empty(fixtures.sheet(out, heroID).Conditions, "the hero's fight started, so their hold ended")
	for _, dirty := range out.DirtyCharacters {
		if dirty.ID == bardID {
			s.NotEmpty(dirty.Conditions, "the bard's fight has not started")
		}
	}
}
