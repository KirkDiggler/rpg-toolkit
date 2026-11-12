// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// A paused cast is a successful persistence boundary, not an atomic transaction.
// Target zero's delivery is already on its sheet when target one's save asks for
// an optional die. The current linked protocol must preserve this behavior or
// the design must explicitly change it; skipping graph validation is not a fix.
func (s *ResistancePoseTestSuite) TestEarlierBaneRecipientIsPersistedBeforeLaterTargetAnswers() {
	fixtures := s.fixtures()
	saver := fixtures.saver(14, s.resistanceJSON())
	machine, err := NewAction(&ActionInput{ApplicationID: testApplicationID(s.T()), Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID, heroID}, Roller: facedRoller{d20: 1, other: 1}})
	s.Require().NoError(err)
	paused, err := fixtures.resolve(saver, machine, baneCost(), baneCaster(1))
	s.Require().NoError(err)
	s.Require().NotNil(paused.Posed)
	s.Equal(heroID, paused.Posed.Ask.Audience)
	s.Nil(paused.Outcome, "the cast is told whole later, but earlier board changes are durable now")
	s.Require().Len(paused.DirtyMonsters, 1)
	s.Equal(wolfID, paused.DirtyMonsters[0].ID)
	var stored []json.RawMessage
	raw, err := json.Marshal(paused.DirtyMonsters[0].Conditions)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, &stored))
	found := false
	for _, blob := range stored {
		condition, err := conditions.LoadJSON(blob)
		s.Require().NoError(err)
		if condition.Ref().Equals(refs.Conditions.Baned()) {
			found = true
		}
	}
	s.True(found, "the first failed save already imposed Bane at the pause boundary")
}
