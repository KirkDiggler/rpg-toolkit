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

// Tests for AttacksRemaining sub-resource

func (s *ActionEconomyTestSuite) TestNewActionEconomySubResources() {
	s.Run("creates with attacks and movement at zero", func() {
		economy := NewActionEconomy()
		s.Require().NotNil(economy)
		s.Equal(0, economy.AttacksRemaining, "should start with 0 attacks (until Attack ability used)")
		s.Equal(0, economy.MovementRemaining, "should start with 0 movement (set at turn start)")
	})
}

func (s *ActionEconomyTestSuite) TestCanUseAttack() {
	s.Run("returns false when no attacks available", func() {
		s.False(s.economy.CanUseAttack())
	})

	s.Run("returns true when attacks available", func() {
		s.economy.SetAttacks(1)
		s.True(s.economy.CanUseAttack())
	})

	s.Run("returns true with multiple attacks", func() {
		s.economy.SetAttacks(2) // Extra Attack
		s.True(s.economy.CanUseAttack())
	})
}

func (s *ActionEconomyTestSuite) TestUseAttack() {
	s.Run("returns error when no attacks available", func() {
		err := s.economy.UseAttack()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(0, s.economy.AttacksRemaining, "should not go negative")
	})

	s.Run("consumes attack successfully", func() {
		s.economy.SetAttacks(1)
		err := s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(0, s.economy.AttacksRemaining)
		s.False(s.economy.CanUseAttack())
	})

	s.Run("consumes multiple attacks sequentially", func() {
		s.economy.SetAttacks(2) // Extra Attack

		err := s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(1, s.economy.AttacksRemaining)

		err = s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(0, s.economy.AttacksRemaining)

		err = s.economy.UseAttack()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})
}

func (s *ActionEconomyTestSuite) TestSetAttacks() {
	s.Run("sets attacks to specified count", func() {
		s.economy.SetAttacks(1)
		s.Equal(1, s.economy.AttacksRemaining)
	})

	s.Run("sets multiple attacks for Extra Attack", func() {
		s.economy.SetAttacks(2)
		s.Equal(2, s.economy.AttacksRemaining)
	})

	s.Run("overwrites existing attack count", func() {
		s.economy.SetAttacks(2)
		s.economy.SetAttacks(3) // Fighter with Improved Extra Attack
		s.Equal(3, s.economy.AttacksRemaining)
	})
}

// Tests for MovementRemaining sub-resource

func (s *ActionEconomyTestSuite) TestCanUseMovement() {
	s.Run("returns false when no movement available", func() {
		s.False(s.economy.CanUseMovement(5))
	})

	s.Run("returns true when enough movement available", func() {
		s.economy.SetMovement(30)
		s.True(s.economy.CanUseMovement(5))
	})

	s.Run("returns false when insufficient movement", func() {
		s.economy.SetMovement(30)
		s.False(s.economy.CanUseMovement(35))
	})

	s.Run("returns true when exact movement available", func() {
		s.economy.SetMovement(30)
		s.True(s.economy.CanUseMovement(30))
	})

	s.Run("returns true for zero cost movement", func() {
		s.economy.SetMovement(0)
		s.True(s.economy.CanUseMovement(0))
	})
}

func (s *ActionEconomyTestSuite) TestUseMovement() {
	s.Run("returns error when no movement available", func() {
		err := s.economy.UseMovement(5)
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(0, s.economy.MovementRemaining, "should not go negative")
	})

	s.Run("returns error when insufficient movement", func() {
		s.economy.SetMovement(10)
		err := s.economy.UseMovement(15)
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(10, s.economy.MovementRemaining, "should not consume partial movement")
	})

	s.Run("consumes movement successfully", func() {
		s.economy.SetMovement(30)
		err := s.economy.UseMovement(5)
		s.Require().NoError(err)
		s.Equal(25, s.economy.MovementRemaining)
	})

	s.Run("consumes all movement", func() {
		s.economy.SetMovement(30)
		err := s.economy.UseMovement(30)
		s.Require().NoError(err)
		s.Equal(0, s.economy.MovementRemaining)
		s.False(s.economy.CanUseMovement(5))
	})

	s.Run("consumes movement incrementally", func() {
		s.economy.SetMovement(30)

		err := s.economy.UseMovement(10)
		s.Require().NoError(err)
		s.Equal(20, s.economy.MovementRemaining)

		err = s.economy.UseMovement(15)
		s.Require().NoError(err)
		s.Equal(5, s.economy.MovementRemaining)

		err = s.economy.UseMovement(10)
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Equal(5, s.economy.MovementRemaining)
	})

	s.Run("allows zero cost movement", func() {
		s.economy.SetMovement(30)
		err := s.economy.UseMovement(0)
		s.Require().NoError(err)
		s.Equal(30, s.economy.MovementRemaining)
	})
}

func (s *ActionEconomyTestSuite) TestSetMovement() {
	s.Run("sets movement to specified amount", func() {
		s.economy.SetMovement(30)
		s.Equal(30, s.economy.MovementRemaining)
	})

	s.Run("overwrites existing movement", func() {
		s.economy.SetMovement(30)
		s.economy.SetMovement(25) // Dwarf speed
		s.Equal(25, s.economy.MovementRemaining)
	})
}

func (s *ActionEconomyTestSuite) TestAddMovement() {
	s.Run("adds movement to existing amount", func() {
		s.economy.SetMovement(30)
		s.economy.AddMovement(30) // Dash
		s.Equal(60, s.economy.MovementRemaining)
	})

	s.Run("adds movement when currently zero", func() {
		s.economy.AddMovement(30)
		s.Equal(30, s.economy.MovementRemaining)
	})

	s.Run("adds movement multiple times", func() {
		s.economy.SetMovement(30)
		s.economy.AddMovement(30) // Dash
		s.economy.AddMovement(30) // Cunning Action Dash (Rogue)
		s.Equal(90, s.economy.MovementRemaining)
	})
}

func (s *ActionEconomyTestSuite) TestResetDoesNotAffectSubResources() {
	s.Run("reset does not clear attacks remaining", func() {
		s.economy.SetAttacks(2)
		s.economy.Reset()
		s.Equal(2, s.economy.AttacksRemaining, "attacks should persist through reset")
	})

	s.Run("reset does not affect movement remaining", func() {
		s.economy.SetMovement(30)
		s.economy.Reset()
		s.Equal(30, s.economy.MovementRemaining, "movement should persist through reset")
	})
}

func (s *ActionEconomyTestSuite) TestAttackAbilityScenario() {
	s.Run("simulates Fighter taking Attack action with Extra Attack", func() {
		// Turn starts - movement set from character speed
		s.economy.SetMovement(30)

		// Fighter starts with 0 attacks (hasn't taken Attack action yet)
		s.Equal(0, s.economy.AttacksRemaining)
		s.False(s.economy.CanUseAttack())

		// Fighter takes Attack action (consumes 1 action)
		err := s.economy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.ActionsRemaining)

		// Attack ability grants attacks based on Extra Attack
		s.economy.SetAttacks(2) // Extra Attack

		// Fighter can now make strikes
		s.True(s.economy.CanUseAttack())

		// First strike
		err = s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(1, s.economy.AttacksRemaining)

		// Second strike
		err = s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(0, s.economy.AttacksRemaining)

		// No more attacks
		s.False(s.economy.CanUseAttack())
		err = s.economy.UseAttack()
		s.Require().Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
	})
}

func (s *ActionEconomyTestSuite) TestDashAbilityScenario() {
	s.Run("simulates Rogue using Dash", func() {
		// Turn starts - movement set from character speed
		s.economy.SetMovement(30)
		s.Equal(30, s.economy.MovementRemaining)

		// Rogue moves 15 feet
		err := s.economy.UseMovement(15)
		s.Require().NoError(err)
		s.Equal(15, s.economy.MovementRemaining)

		// Rogue uses Dash as action
		err = s.economy.UseAction()
		s.Require().NoError(err)
		s.economy.AddMovement(30) // Dash adds speed again
		s.Equal(45, s.economy.MovementRemaining)

		// Rogue moves another 30 feet
		err = s.economy.UseMovement(30)
		s.Require().NoError(err)
		s.Equal(15, s.economy.MovementRemaining)

		// Rogue uses Cunning Action to Dash again as bonus action
		err = s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.economy.AddMovement(30) // Another Dash
		s.Equal(45, s.economy.MovementRemaining)
	})
}

func (s *ActionEconomyTestSuite) TestFullCombatTurnScenario() {
	s.Run("simulates Fighter full combat turn with Extra Attack and dual wielding", func() {
		// Turn starts
		s.economy.SetMovement(30)
		s.Equal(1, s.economy.ActionsRemaining)
		s.Equal(1, s.economy.BonusActionsRemaining)
		s.Equal(1, s.economy.ReactionsRemaining)
		s.Equal(0, s.economy.AttacksRemaining)
		s.Equal(30, s.economy.MovementRemaining)

		// 1. Fighter moves toward goblin (15ft)
		err := s.economy.UseMovement(15)
		s.Require().NoError(err)
		s.Equal(15, s.economy.MovementRemaining)

		// 2. Fighter activates Attack ability
		err = s.economy.UseAction()
		s.Require().NoError(err)
		s.economy.SetAttacks(2) // Extra Attack
		s.Equal(0, s.economy.ActionsRemaining)
		s.Equal(2, s.economy.AttacksRemaining)

		// 3. Fighter activates Strike on goblin
		err = s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(1, s.economy.AttacksRemaining)
		// DualWieldingCondition would grant OffHandStrike here

		// 4. Fighter moves to flank (5ft)
		err = s.economy.UseMovement(5)
		s.Require().NoError(err)
		s.Equal(10, s.economy.MovementRemaining)

		// 5. Fighter activates Strike on goblin
		err = s.economy.UseAttack()
		s.Require().NoError(err)
		s.Equal(0, s.economy.AttacksRemaining)

		// 6. Fighter activates OffHandStrike on goblin
		err = s.economy.UseBonusAction()
		s.Require().NoError(err)
		s.Equal(0, s.economy.BonusActionsRemaining)
		// OffHandStrike doesn't consume AttacksRemaining, uses bonus action

		// End of turn - reaction still available
		s.True(s.economy.CanUseReaction())
	})
}
