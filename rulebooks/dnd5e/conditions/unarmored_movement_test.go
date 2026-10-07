// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type UnarmoredMovementTestSuite struct {
	suite.Suite
	condition *UnarmoredMovementCondition
	bus       events.EventBus
	ctx       context.Context
}

func TestUnarmoredMovementSuite(t *testing.T) {
	suite.Run(t, new(UnarmoredMovementTestSuite))
}

func (s *UnarmoredMovementTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.condition = NewUnarmoredMovementCondition(UnarmoredMovementInput{
		MemberID: "monk-1",
	})
}

func (s *UnarmoredMovementTestSuite) TestNewUnarmoredMovementCondition() {
	s.Assert().Equal("monk-1", s.condition.MemberID)
	s.Assert().False(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestApply() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestApplyTwice() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Applying twice should not error (unlike some other conditions)
	err = s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestRemove() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	err = s.condition.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestRemoveWhenNotApplied() {
	err := s.condition.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestToJSON() {
	data, err := s.condition.ToJSON()
	s.Require().NoError(err)
	s.Require().NotNil(data)

	var umData UnarmoredMovementData
	err = json.Unmarshal(data, &umData)
	s.Require().NoError(err)

	s.Assert().Equal(refs.Conditions.UnarmoredMovement(), umData.Ref)
	s.Assert().Equal("monk-1", umData.MemberID)
}

func (s *UnarmoredMovementTestSuite) TestLoadJSON() {
	// Create JSON data
	data := UnarmoredMovementData{
		Ref:      refs.Conditions.UnarmoredMovement(),
		MemberID: "monk-2",
	}
	jsonData, err := json.Marshal(data)
	s.Require().NoError(err)

	// Load into condition
	condition := &UnarmoredMovementCondition{}
	err = condition.loadJSON(jsonData)
	s.Require().NoError(err)

	s.Assert().Equal("monk-2", condition.MemberID)
}

func (s *UnarmoredMovementTestSuite) TestRoundTripSerialization() {
	// Apply condition
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Serialize
	jsonData, err := s.condition.ToJSON()
	s.Require().NoError(err)

	// Deserialize
	newCondition := &UnarmoredMovementCondition{}
	err = newCondition.loadJSON(jsonData)
	s.Require().NoError(err)

	// Verify fields match
	s.Assert().Equal(s.condition.MemberID, newCondition.MemberID)

	// Note: bus state is not serialized, so IsApplied will be false
	s.Assert().False(newCondition.IsApplied())
}
