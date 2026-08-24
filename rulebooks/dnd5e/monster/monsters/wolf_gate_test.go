// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type WolfSaveGateSuite struct {
	suite.Suite
}

func TestWolfSaveGateSuite(t *testing.T) {
	suite.Run(t, new(WolfSaveGateSuite))
}

func (s *WolfSaveGateSuite) TestTheWolfDeclaresItsKnockdown() {
	definitions := NewWolf("wolf-1").Actions()
	s.Require().Len(definitions, 1)
	s.Require().NotNil(definitions[0].Attack)
	s.Require().Len(definitions[0].Attack.OnHit, 1)

	gate := definitions[0].Attack.OnHit[0].Save
	s.Require().NotNil(gate)
	s.Equal([]abilities.Ability{abilities.STR}, gate.Abilities)
	s.Equal(11, gate.DC.DC(saves.DCInput{}))
	s.Equal(saves.Negated, gate.OnSuccess)
	s.Equal(saves.RecurrenceNone, gate.Recurrence)
}

func (s *WolfSaveGateSuite) TestTheDeclarationSurvivesSerialization() {
	raw, err := json.Marshal(NewWolf("wolf-1").ToData())
	s.Require().NoError(err)

	var data monster.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := monster.Load(context.Background(), &data)
	s.Require().NoError(err)

	definitions := loaded.Actions()
	s.Require().Len(definitions, 1)
	s.Require().Len(definitions[0].Attack.OnHit, 1)
	s.Equal(saves.NewSaveGate(abilities.STR, 11), definitions[0].Attack.OnHit[0].Save)
}
