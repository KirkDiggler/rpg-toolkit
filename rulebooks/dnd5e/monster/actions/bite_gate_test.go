// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type BiteSaveGateSuite struct {
	suite.Suite
}

func TestBiteSaveGateSuite(t *testing.T) {
	suite.Run(t, new(BiteSaveGateSuite))
}

func (s *BiteSaveGateSuite) TestSaveGateRoundTripsWithCanonicalDamage() {
	gate := &saves.SaveGate{
		Abilities:  []abilities.Ability{abilities.CON},
		DC:         saves.DCFivePlusDamageTaken(),
		OnSuccess:  saves.Half,
		Recurrence: saves.RecurrenceEndOfTurn,
	}
	bite, err := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		Damage:      []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}},
		SaveGate:    gate,
	})
	s.Require().NoError(err)
	s.Equal(gate, bite.SaveGate())

	loaded, err := LoadAction(bite.ToData())
	s.Require().NoError(err)
	s.Equal(gate, loaded.(*BiteAction).SaveGate())
	s.Equal(12, loaded.(*BiteAction).SaveGate().DC.DC(saves.DCInput{DamageTaken: 7}))
}
