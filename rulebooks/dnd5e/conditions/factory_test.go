// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FactoryTestSuite struct {
	suite.Suite
}

func TestFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(FactoryTestSuite))
}

func (s *FactoryTestSuite) TestCreateFromRef_SneakAttack() {
	// RED: This test should fail because createSneakAttack doesn't exist
	// in the factory switch statement

	input := &CreateFromRefInput{
		Ref:         refs.Conditions.SneakAttack().String(),
		Config:      json.RawMessage(`{"rogue_level": 1}`),
		CharacterID: "rogue-1",
	}

	output, err := CreateFromRef(input)

	s.Require().NoError(err, "CreateFromRef should not return error")
	s.Require().NotNil(output, "Output should not be nil")
	s.Require().NotNil(output.Condition, "Condition should not be nil")

	// Verify it's a SneakAttackCondition
	sneak, ok := output.Condition.(*SneakAttackCondition)
	s.Require().True(ok, "Should be a SneakAttackCondition")

	s.Equal("rogue-1", sneak.CharacterID)
	s.Equal(1, sneak.Level)
	s.Equal(1, sneak.DamageDice, "Level 1 rogue should have 1d6 sneak attack")
}

func (s *FactoryTestSuite) TestCreateFromRef_SneakAttackLevel5() {
	input := &CreateFromRefInput{
		Ref:         refs.Conditions.SneakAttack().String(),
		Config:      json.RawMessage(`{"rogue_level": 5}`),
		CharacterID: "rogue-1",
	}

	output, err := CreateFromRef(input)

	s.Require().NoError(err)
	s.Require().NotNil(output)

	sneak, ok := output.Condition.(*SneakAttackCondition)
	s.Require().True(ok)

	s.Equal(5, sneak.Level)
	s.Equal(3, sneak.DamageDice, "Level 5 rogue should have 3d6 sneak attack")
}
