// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/stretchr/testify/suite"
)

// RageV2TestSuite tests the condition-based rage implementation
type RageV2TestSuite struct {
	suite.Suite
	ctx              context.Context
	bus              *events.Bus
	conditionManager conditions.Manager
	rage             *features.RageV2
	barbarian        core.Entity
}

func (s *RageV2TestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewBus()
	s.conditionManager = conditions.NewMemoryManager(s.bus)
	s.rage = features.NewRageV2("rage_001", 2, 5, s.bus)
	s.barbarian = &mockEntity{id: "barbarian_001", entityType: dnd5e.EntityTypeCharacter}
}

func (s *RageV2TestSuite) TestCanActivate() {
	tests := []struct {
		name        string
		setup       func()
		input       features.FeatureInput
		expectError bool
		errorMsg    string
	}{
		{
			name: "can activate with uses remaining",
			input: features.RageV2Input{
				ConditionManager: s.conditionManager,
			},
			expectError: false,
		},
		{
			name: "cannot activate without condition manager",
			input: features.FeatureInput{},
			expectError: true,
			errorMsg:    "rage requires a condition manager",
		},
		{
			name: "cannot activate when already raging",
			setup: func() {
				// Apply raging condition first
				_, err := s.conditionManager.ApplyCondition(s.ctx, &conditions.ApplyConditionInput{
					Target: s.barbarian,
					Condition: conditions.Condition{
						Type:   conditions.Raging,
						Source: "test",
					},
					EventBus: s.bus,
				})
				s.Require().NoError(err)
			},
			input: features.RageV2Input{
				ConditionManager: s.conditionManager,
			},
			expectError: true,
			errorMsg:    "already raging",
		},
		{
			name: "cannot activate with no uses remaining",
			setup: func() {
				// Use up all rage uses
				for i := 0; i < 2; i++ {
					input := features.RageV2Input{
						ConditionManager: s.conditionManager,
					}
					err := s.rage.Activate(s.ctx, s.barbarian, input)
					s.Require().NoError(err)
					// Remove the condition so we can try again
					_, _ = s.conditionManager.RemoveCondition(s.ctx, &conditions.RemoveConditionInput{
						Target:   s.barbarian,
						Type:     conditions.Raging,
						EventBus: s.bus,
					})
				}
			},
			input: features.RageV2Input{
				ConditionManager: s.conditionManager,
			},
			expectError: true,
			errorMsg:    "no rage uses remaining",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset for each test
			s.SetupTest()

			if tt.setup != nil {
				tt.setup()
			}

			err := s.rage.CanActivate(s.ctx, s.barbarian, tt.input)

			if tt.expectError {
				s.Error(err)
				if tt.errorMsg != "" {
					s.Contains(err.Error(), tt.errorMsg)
				}
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *RageV2TestSuite) TestActivate() {
	input := features.RageV2Input{
		ConditionManager: s.conditionManager,
	}

	// Activate rage
	err := s.rage.Activate(s.ctx, s.barbarian, input)
	s.NoError(err)

	// Verify condition was applied
	hasRage, err := s.conditionManager.HasCondition(s.ctx, s.barbarian, conditions.Raging)
	s.NoError(err)
	s.True(hasRage)

	// Verify uses were consumed
	s.Equal(1, s.rage.GetCurrentUses())

	// Verify condition has correct metadata
	conds, err := s.conditionManager.GetConditions(s.ctx, s.barbarian)
	s.NoError(err)
	s.Len(conds, 1)
	s.Equal(conditions.Raging, conds[0].Type)
	s.Equal("rage_feature", conds[0].Source)
	s.Equal(conditions.DurationRounds, conds[0].DurationType)
	s.Equal(10, conds[0].Remaining)
	s.Equal(5, conds[0].Metadata["barbarian_level"])
}

func (s *RageV2TestSuite) TestDeactivate() {
	// First activate rage
	input := features.RageV2Input{
		ConditionManager: s.conditionManager,
	}
	err := s.rage.Activate(s.ctx, s.barbarian, input)
	s.NoError(err)

	// Verify rage is active
	hasRage, err := s.conditionManager.HasCondition(s.ctx, s.barbarian, conditions.Raging)
	s.NoError(err)
	s.True(hasRage)

	// Deactivate rage
	err = s.rage.Deactivate(s.ctx, s.barbarian, s.conditionManager)
	s.NoError(err)

	// Verify rage was removed
	hasRage, err = s.conditionManager.HasCondition(s.ctx, s.barbarian, conditions.Raging)
	s.NoError(err)
	s.False(hasRage)
}

func (s *RageV2TestSuite) TestDurationTicking() {
	// Activate rage
	input := features.RageV2Input{
		ConditionManager: s.conditionManager,
	}
	err := s.rage.Activate(s.ctx, s.barbarian, input)
	s.NoError(err)

	// Tick duration for 5 rounds
	for i := 0; i < 5; i++ {
		output, err := s.conditionManager.TickDuration(s.ctx, &conditions.TickDurationInput{
			Target:       s.barbarian,
			DurationType: conditions.DurationRounds,
			Amount:       1,
		})
		s.NoError(err)
		s.Empty(output.ExpiredConditions) // Should not expire yet
	}

	// Verify rage is still active with 5 rounds remaining
	conds, err := s.conditionManager.GetConditions(s.ctx, s.barbarian)
	s.NoError(err)
	s.Len(conds, 1)
	s.Equal(5, conds[0].Remaining)

	// Tick 5 more rounds to expire
	output, err := s.conditionManager.TickDuration(s.ctx, &conditions.TickDurationInput{
		Target:       s.barbarian,
		DurationType: conditions.DurationRounds,
		Amount:       5,
	})
	s.NoError(err)
	s.Len(output.ExpiredConditions, 1)
	s.Equal(conditions.Raging, output.ExpiredConditions[0].Type)

	// Verify rage was removed
	hasRage, err := s.conditionManager.HasCondition(s.ctx, s.barbarian, conditions.Raging)
	s.NoError(err)
	s.False(hasRage)
}

func (s *RageV2TestSuite) TestReset() {
	// Use up all rage uses
	for i := 0; i < 2; i++ {
		input := features.RageV2Input{
			ConditionManager: s.conditionManager,
		}
		err := s.rage.Activate(s.ctx, s.barbarian, input)
		s.NoError(err)
		// Remove condition for next activation
		_, _ = s.conditionManager.RemoveCondition(s.ctx, &conditions.RemoveConditionInput{
			Target:   s.barbarian,
			Type:     conditions.Raging,
			EventBus: s.bus,
		})
	}

	// Verify no uses remaining
	s.Equal(0, s.rage.GetCurrentUses())

	// Reset (like on long rest)
	s.rage.Reset()

	// Verify uses restored
	s.Equal(2, s.rage.GetCurrentUses())
}

func (s *RageV2TestSuite) TestLevelProgression() {
	testCases := []struct {
		level         int
		expectedBonus int
	}{
		{1, 2},
		{8, 2},
		{9, 3},
		{15, 3},
		{16, 4},
		{20, 4},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("level_%d", tc.level), func() {
			s.SetupTest() // Reset for each test
			s.rage.SetLevel(tc.level)

			input := features.RageV2Input{
				ConditionManager: s.conditionManager,
			}
			err := s.rage.Activate(s.ctx, s.barbarian, input)
			s.NoError(err)

			// Check the condition metadata has correct level
			conds, err := s.conditionManager.GetConditions(s.ctx, s.barbarian)
			s.NoError(err)
			s.Len(conds, 1)
			s.Equal(tc.level, conds[0].Metadata["barbarian_level"])
		})
	}
}

// mockEntity is a test implementation of core.Entity
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string           { return m.id }
func (m *mockEntity) GetType() core.EntityType { return m.entityType }

func TestRageV2Suite(t *testing.T) {
	suite.Run(t, new(RageV2TestSuite))
}