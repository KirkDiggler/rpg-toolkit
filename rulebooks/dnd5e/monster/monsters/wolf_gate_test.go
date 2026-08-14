// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// WolfSaveGateSuite pins the first authored gate in the roster. The wolf is the
// case rpg-toolkit#962 started from: a stat block that carried a knockdown DC
// and could not say whether anything read it.
type WolfSaveGateSuite struct {
	suite.Suite
}

func TestWolfSaveGateSuite(t *testing.T) {
	suite.Run(t, new(WolfSaveGateSuite))
}

// "If the target is a creature, it must succeed on a DC 11 Strength saving
// throw or be knocked prone" — declared, and answerable from the stat block
// without executing anything.
func (s *WolfSaveGateSuite) TestTheWolfDeclaresItsKnockdown() {
	wolf := NewWolf("wolf-1")

	s.Require().Len(wolf.Actions(), 1)
	bite, ok := wolf.Actions()[0].(*actions.BiteAction)
	s.Require().True(ok, "the wolf's one action is its bite")

	gate := bite.SaveGate()
	s.Require().NotNil(gate, "and the bite says what can be contested")
	s.Assert().Equal([]abilities.Ability{abilities.STR}, gate.Abilities)
	s.Assert().Equal(11, gate.DC.DC(saves.DCInput{}))
	s.Assert().Equal(saves.Negated, gate.OnSuccess)
	s.Assert().Equal(saves.RecurrenceNone, gate.Recurrence)
	s.Assert().Equal("DC 11 str save, negated on success, recurrence none", gate.String())
}

// The declaration survives the round trip a spawned wolf actually takes.
func (s *WolfSaveGateSuite) TestTheDeclarationSurvivesSerialization() {
	data := NewWolf("wolf-1").ToData()
	s.Require().Len(data.Actions, 1)

	loaded, err := actions.LoadAction(data.Actions[0])
	s.Require().NoError(err)

	bite, ok := loaded.(*actions.BiteAction)
	s.Require().True(ok)
	s.Require().Equal(saves.NewSaveGate(abilities.STR, 11), bite.SaveGate())
}
