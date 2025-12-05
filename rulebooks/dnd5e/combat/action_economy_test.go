package combat

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/stretchr/testify/suite"
)

type ActionEconomyTestSuite struct {
	suite.Suite
	economy *ActionEconomy
}

func TestActionEconomySuite(t *testing.T) {
	suite.Run(t, new(ActionEconomyTestSuite))
}

func (s *ActionEconomyTestSuite) SetupTest() {
	s.economy = NewActionEconomy()
}

func (s *ActionEconomyTestSuite) SetupSubTest() {
	// Reset to clean state for each subtest
	s.economy = NewActionEconomy()
}

func (s *ActionEconomyTestSuite) TestNewActionEconomy() {
	s.Run("creates with default values", func() {
		economy := NewActionEconomy()
		s.Require().NotNil(economy)
		s.Equal(1, economy.ActionsRemaining, "should start with 1 action")
		s.Equal(1, economy.BonusActionsRemaining, "should start with 1 bonus action")
		s.Equal(1, economy.ReactionsRemaining, "should start with 1 reaction")
	})
}

func (s *ActionEconomyTestSuite) TestCanUseAction() {
	s.Run("returns true when actions available", func() {
		s.True(s.economy.CanUseAction())
	})

	s.Run("returns false when no actions available", func() {
		s.economy.ActionsRemaining = 0
		s.False(s.economy.CanUseAction())
	})

	s.Run("returns true with multiple actions", func() {
		s.economy.ActionsRemaining = 2
		s.True(s.economy.CanUseAction())
	})
}

func (s *ActionEconomyTestSuite) TestCanUseBonusAction() {
	s.Run("returns true when bonus actions available", func() {
		s.True(s.economy.CanUseBonusAction())
	})

	s.Run("returns false when no bonus actions available", func() {
		s.economy.BonusActionsRemaining = 0
		s.False(s.economy.CanUseBonusAction())
	})

	s.Run("returns true with multiple bonus actions", func() {
		s.economy.BonusActionsRemaining = 2
		s.True(s.economy.CanUseBonusAction())
	})
}

func (s *ActionEconomyTestSuite) TestCanUseReaction() {
	s.Run("returns true when reactions available", func() {
		s.True(s.economy.CanUseReaction())
	})

	s.Run("returns false when no reactions available", func() {
		s.economy.ReactionsRemaining = 0
		s.False(s.economy.CanUseReaction())
	})

	s.Run("returns true with multiple reactions", func() {
		s.economy.ReactionsRemaining = 2
		s.True(s.economy.CanUseReaction())
	})
}

//nolint:dupl // Intentional duplication - testing same pattern for different action types
func (s *ActionEconomyTestSuite) TestUseAction() {
	s.Run("consumes action successfully", func() {
		err := s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ActionsRemaining)
		s.False(s.economy.CanUseAction())
	})

	s.Run("returns error when no actions available", func() {
		s.economy.ActionsRemaining = 0
		err := s.economy.UseAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(0, s.economy.ActionsRemaining, "should not go negative")
	})

	s.Run("consumes multiple actions sequentially", func() {
		s.economy.ActionsRemaining = 2

		err := s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(1, s.economy.ActionsRemaining)

		err = s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ActionsRemaining)

		err = s.economy.UseAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})
}

//nolint:dupl // Intentional duplication - testing same pattern for different action types
func (s *ActionEconomyTestSuite) TestUseBonusAction() {
	s.Run("consumes bonus action successfully", func() {
		err := s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.BonusActionsRemaining)
		s.False(s.economy.CanUseBonusAction())
	})

	s.Run("returns error when no bonus actions available", func() {
		s.economy.BonusActionsRemaining = 0
		err := s.economy.UseBonusAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(0, s.economy.BonusActionsRemaining, "should not go negative")
	})

	s.Run("consumes multiple bonus actions sequentially", func() {
		s.economy.BonusActionsRemaining = 2

		err := s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.Equal(1, s.economy.BonusActionsRemaining)

		err = s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.BonusActionsRemaining)

		err = s.economy.UseBonusAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})
}

//nolint:dupl // Intentional duplication - testing same pattern for different action types
func (s *ActionEconomyTestSuite) TestUseReaction() {
	s.Run("consumes reaction successfully", func() {
		err := s.economy.UseReaction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ReactionsRemaining)
		s.False(s.economy.CanUseReaction())
	})

	s.Run("returns error when no reactions available", func() {
		s.economy.ReactionsRemaining = 0
		err := s.economy.UseReaction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(0, s.economy.ReactionsRemaining, "should not go negative")
	})

	s.Run("consumes multiple reactions sequentially", func() {
		s.economy.ReactionsRemaining = 2

		err := s.economy.UseReaction()
		s.Require().NoError(err)
		s.Equal(1, s.economy.ReactionsRemaining)

		err = s.economy.UseReaction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ReactionsRemaining)

		err = s.economy.UseReaction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})
}

func (s *ActionEconomyTestSuite) TestReset() {
	s.Run("restores all actions to default", func() {
		// Consume all actions
		_ = s.economy.UseAction()
		_ = s.economy.UseBonusAction()
		_ = s.economy.UseReaction()

		s.Equal(0, s.economy.ActionsRemaining)
		s.Equal(0, s.economy.BonusActionsRemaining)
		s.Equal(0, s.economy.ReactionsRemaining)

		// Reset
		s.economy.Reset()

		s.Equal(1, s.economy.ActionsRemaining)
		s.Equal(1, s.economy.BonusActionsRemaining)
		s.Equal(1, s.economy.ReactionsRemaining)
	})

	s.Run("resets from extra actions granted", func() {
		s.economy.GrantExtraAction()
		s.economy.GrantExtraBonusAction()
		s.Equal(2, s.economy.ActionsRemaining)
		s.Equal(2, s.economy.BonusActionsRemaining)

		s.economy.Reset()

		s.Equal(1, s.economy.ActionsRemaining, "should reset to 1, not 2")
		s.Equal(1, s.economy.BonusActionsRemaining, "should reset to 1, not 2")
	})
}

//nolint:dupl // Intentional duplication - testing same pattern for different action types
func (s *ActionEconomyTestSuite) TestGrantExtraAction() {
	s.Run("grants additional action", func() {
		s.Equal(1, s.economy.ActionsRemaining)

		s.economy.GrantExtraAction()

		s.Equal(2, s.economy.ActionsRemaining)
		s.True(s.economy.CanUseAction())
	})

	s.Run("can grant multiple extra actions", func() {
		s.economy.GrantExtraAction()
		s.economy.GrantExtraAction()

		s.Equal(3, s.economy.ActionsRemaining)
	})

	s.Run("can use all granted actions", func() {
		s.economy.GrantExtraAction() // Now have 2 actions

		err := s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(1, s.economy.ActionsRemaining)

		err = s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ActionsRemaining)

		err = s.economy.UseAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})

	s.Run("grants action even when depleted", func() {
		_ = s.economy.UseAction() // Consume the default action
		s.Equal(0, s.economy.ActionsRemaining)

		s.economy.GrantExtraAction() // Grant action surge

		s.Equal(1, s.economy.ActionsRemaining)
		s.True(s.economy.CanUseAction())
	})
}

//nolint:dupl // Intentional duplication - testing same pattern for different action types
func (s *ActionEconomyTestSuite) TestGrantExtraBonusAction() {
	s.Run("grants additional bonus action", func() {
		s.Equal(1, s.economy.BonusActionsRemaining)

		s.economy.GrantExtraBonusAction()

		s.Equal(2, s.economy.BonusActionsRemaining)
		s.True(s.economy.CanUseBonusAction())
	})

	s.Run("can grant multiple extra bonus actions", func() {
		s.economy.GrantExtraBonusAction()
		s.economy.GrantExtraBonusAction()

		s.Equal(3, s.economy.BonusActionsRemaining)
	})

	s.Run("can use all granted bonus actions", func() {
		s.economy.GrantExtraBonusAction() // Now have 2 bonus actions

		err := s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.Equal(1, s.economy.BonusActionsRemaining)

		err = s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.BonusActionsRemaining)

		err = s.economy.UseBonusAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})

	s.Run("grants bonus action even when depleted", func() {
		_ = s.economy.UseBonusAction() // Consume the default bonus action
		s.Equal(0, s.economy.BonusActionsRemaining)

		s.economy.GrantExtraBonusAction()

		s.Equal(1, s.economy.BonusActionsRemaining)
		s.True(s.economy.CanUseBonusAction())
	})
}

func (s *ActionEconomyTestSuite) TestActionSurgeScenario() {
	s.Run("simulates Fighter using Action Surge", func() {
		// Fighter starts turn with normal action economy
		s.Equal(1, s.economy.ActionsRemaining)

		// Fighter takes first Attack action
		err := s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ActionsRemaining)

		// Fighter uses Action Surge feature
		s.economy.GrantExtraAction()
		s.Equal(1, s.economy.ActionsRemaining)

		// Fighter takes second Attack action
		err = s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ActionsRemaining)

		// No more actions available
		err = s.economy.UseAction()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))

		// Next turn, resets to normal
		s.economy.Reset()
		s.Equal(1, s.economy.ActionsRemaining)
	})
}

func (s *ActionEconomyTestSuite) TestIndependentActionTypes() {
	s.Run("consuming one type does not affect others", func() {
		// Use action
		err := s.economy.UseAction()
		s.Require().NoError(err)

		// Bonus action and reaction still available
		s.True(s.economy.CanUseBonusAction())
		s.True(s.economy.CanUseReaction())
		s.Equal(1, s.economy.BonusActionsRemaining)
		s.Equal(1, s.economy.ReactionsRemaining)

		// Use bonus action
		err = s.economy.UseBonusAction()
		s.Require().NoError(err)

		// Reaction still available
		s.True(s.economy.CanUseReaction())
		s.Equal(1, s.economy.ReactionsRemaining)

		// Use reaction
		err = s.economy.UseReaction()
		s.Require().NoError(err)

		// All depleted
		s.False(s.economy.CanUseAction())
		s.False(s.economy.CanUseBonusAction())
		s.False(s.economy.CanUseReaction())
	})
}
