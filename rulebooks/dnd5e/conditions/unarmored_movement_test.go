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
		MemberID:  "monk-1",
		MonkLevel: 3,
	})
}

func (s *UnarmoredMovementTestSuite) TestNewUnarmoredMovementCondition() {
	s.Assert().Equal("monk-1", s.condition.MemberID)
	s.Assert().Equal(3, s.condition.MonkLevel)
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
	s.Assert().Equal(3, umData.MonkLevel)
}

func (s *UnarmoredMovementTestSuite) TestLoadJSON() {
	// Create JSON data
	data := UnarmoredMovementData{
		Ref:       refs.Conditions.UnarmoredMovement(),
		MemberID:  "monk-2",
		MonkLevel: 10,
	}
	jsonData, err := json.Marshal(data)
	s.Require().NoError(err)

	// Load into condition
	condition := &UnarmoredMovementCondition{}
	err = condition.loadJSON(jsonData)
	s.Require().NoError(err)

	s.Assert().Equal("monk-2", condition.MemberID)
	s.Assert().Equal(10, condition.MonkLevel)
}

func (s *UnarmoredMovementTestSuite) TestCalculateSpeedBonus() {
	testCases := []struct {
		name          string
		monkLevel     int
		expectedBonus int
	}{
		{"Level 2", 2, 10},
		{"Level 3", 3, 10},
		{"Level 5", 5, 10},
		{"Level 6", 6, 15},
		{"Level 9", 9, 15},
		{"Level 10", 10, 20},
		{"Level 13", 13, 20},
		{"Level 14", 14, 25},
		{"Level 17", 17, 25},
		{"Level 18", 18, 30},
		{"Level 20", 20, 30},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			condition := NewUnarmoredMovementCondition(UnarmoredMovementInput{
				MemberID:  "monk-test",
				MonkLevel: tc.monkLevel,
			})
			bonus := condition.calculateSpeedBonus()
			s.Assert().Equal(tc.expectedBonus, bonus, "Monk level %d should grant +%d ft", tc.monkLevel, tc.expectedBonus)
		})
	}
}

// NO CAST AT ALL: the question cannot be answered, which is NOT the same as
// "wearing a shield". Reporting known=false keeps a monk from silently losing
// their speed on missing data rather than on a rule.
func (s *UnarmoredMovementTestSuite) TestSpeedBonusWithNoCast() {
	bonus, known := s.condition.SpeedBonus(context.Background())

	s.Assert().False(known, "no cast means the question cannot be answered")
	s.Assert().Equal(0, bonus)
}

// A CAST THAT DOES NOT HOLD THIS MONK is the same answer, and is pinned apart
// from the case above rather than folded into it: here a cast IS installed and
// answering questions, it simply cannot name this member. Collapsing the two
// would let a lookup that ignored its own ID pass.
func (s *UnarmoredMovementTestSuite) TestSpeedBonusWithACastThatLacksThisMonk() {
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "somebody-else", shield: false})

	bonus, known := s.condition.SpeedBonus(ctx)

	s.Assert().False(known, "another monk's shieldless hands say nothing about this one")
	s.Assert().Equal(0, bonus)
}

func (s *UnarmoredMovementTestSuite) TestSpeedBonusUnarmored() {
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-1", shield: false})

	bonus, known := s.condition.SpeedBonus(ctx)

	s.Require().True(known)
	s.Assert().Equal(10, bonus, "Unarmored monk should get speed bonus")
}

// THE SHIELD CASE, which is what D7 put HasShieldEquipped on the member surface
// for. Known and zero: a rule answered, not a gap. It is the only assertion
// here that fails if the shield question stops being asked at all.
func (s *UnarmoredMovementTestSuite) TestSpeedBonusWithShield() {
	ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-1", shield: true})

	bonus, known := s.condition.SpeedBonus(ctx)

	s.Require().True(known, "a shielded monk is a known answer, not an unknown one")
	s.Assert().Equal(0, bonus, "Monk with shield should not get speed bonus")
}

func (s *UnarmoredMovementTestSuite) TestSpeedBonusWithDifferentLevels() {
	testCases := []struct {
		name          string
		monkLevel     int
		expectedBonus int
	}{
		{"Low level", 3, 10},
		{"Mid level", 10, 20},
		{"High level", 18, 30},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			condition := NewUnarmoredMovementCondition(UnarmoredMovementInput{
				MemberID:  "monk-1",
				MonkLevel: tc.monkLevel,
			})
			ctx := castOf(context.Background(), &fakeConditionOwner{id: "monk-1", shield: false})

			bonus, known := condition.SpeedBonus(ctx)
			s.Require().True(known)
			s.Assert().Equal(tc.expectedBonus, bonus)
		})
	}
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
	s.Assert().Equal(s.condition.MonkLevel, newCondition.MonkLevel)

	// Note: bus state is not serialized, so IsApplied will be false
	s.Assert().False(newCondition.IsApplied())
}
