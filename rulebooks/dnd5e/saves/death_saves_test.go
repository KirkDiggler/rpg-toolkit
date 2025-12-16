package saves

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
)

type DeathSaveTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	mockRoller *mock_dice.MockRoller
}

func TestDeathSaveSuite(t *testing.T) {
	suite.Run(t, new(DeathSaveTestSuite))
}

func (s *DeathSaveTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *DeathSaveTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestDeathSaveStateInitialValues tests that a new DeathSaveState starts with zero successes/failures
func (s *DeathSaveTestSuite) TestDeathSaveStateInitialValues() {
	state := &DeathSaveState{}

	s.Equal(0, state.Successes, "initial successes should be 0")
	s.Equal(0, state.Failures, "initial failures should be 0")
	s.False(state.Stabilized, "should not be stabilized initially")
	s.False(state.Dead, "should not be dead initially")
}

// TestRoll1AddsTwoFailures tests that rolling a 1 adds 2 failures (critical fail)
func (s *DeathSaveTestSuite) TestRoll1AddsTwoFailures() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	state := &DeathSaveState{}
	input := &DeathSaveInput{
		Roller: s.mockRoller,
		State:  state,
	}

	result, err := MakeDeathSave(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(1, result.Roll, "roll should be 1")
	s.Equal(2, result.State.Failures, "rolling 1 should add 2 failures")
	s.Equal(0, result.State.Successes, "successes should remain 0")
	s.True(result.IsCriticalFail, "should be marked as critical fail")
}

// TestRoll2To9AddsOneFailure tests that rolling 2-9 adds 1 failure
func (s *DeathSaveTestSuite) TestRoll2To9AddsOneFailure() {
	testCases := []int{2, 5, 9}

	for _, roll := range testCases {
		s.Run(fmt.Sprintf("roll_%d", roll), func() {
			s.SetupTest() // Reset mock for each subtest
			s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(roll, nil)

			state := &DeathSaveState{}
			input := &DeathSaveInput{
				Roller: s.mockRoller,
				State:  state,
			}

			result, err := MakeDeathSave(s.ctx, input)
			s.Require().NoError(err)
			s.Require().NotNil(result)

			s.Equal(roll, result.Roll, "roll should match")
			s.Equal(1, result.State.Failures, "rolling %d should add 1 failure", roll)
			s.Equal(0, result.State.Successes, "successes should remain 0")
			s.False(result.IsCriticalFail, "should not be critical fail")
			s.False(result.IsCriticalSuccess, "should not be critical success")
		})
	}
}

// TestRoll10To19AddsOneSuccess tests that rolling 10-19 adds 1 success
func (s *DeathSaveTestSuite) TestRoll10To19AddsOneSuccess() {
	testCases := []int{10, 15, 19}

	for _, roll := range testCases {
		s.Run(fmt.Sprintf("roll_%d", roll), func() {
			s.SetupTest() // Reset mock for each subtest
			s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(roll, nil)

			state := &DeathSaveState{}
			input := &DeathSaveInput{
				Roller: s.mockRoller,
				State:  state,
			}

			result, err := MakeDeathSave(s.ctx, input)
			s.Require().NoError(err)
			s.Require().NotNil(result)

			s.Equal(roll, result.Roll, "roll should match")
			s.Equal(0, result.State.Failures, "failures should remain 0")
			s.Equal(1, result.State.Successes, "rolling %d should add 1 success", roll)
			s.False(result.IsCriticalFail, "should not be critical fail")
			s.False(result.IsCriticalSuccess, "should not be critical success")
		})
	}
}

// TestRoll20RegainsConsciousness tests that rolling 20 regains consciousness at 1 HP
func (s *DeathSaveTestSuite) TestRoll20RegainsConsciousness() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil)

	state := &DeathSaveState{Failures: 2} // Even with 2 failures, nat 20 saves you
	input := &DeathSaveInput{
		Roller: s.mockRoller,
		State:  state,
	}

	result, err := MakeDeathSave(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(20, result.Roll, "roll should be 20")
	s.True(result.IsCriticalSuccess, "should be marked as critical success")
	s.True(result.RegainedConsciousness, "should regain consciousness")
	s.Equal(1, result.HPRestored, "should restore 1 HP")
	// Failures are reset when regaining consciousness
	s.Equal(0, result.State.Failures, "failures should reset on nat 20")
	s.Equal(0, result.State.Successes, "successes should reset on nat 20")
}

// TestThreeFailuresCausesDeath tests that 3 failures results in death
func (s *DeathSaveTestSuite) TestThreeFailuresCausesDeath() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	state := &DeathSaveState{Failures: 2} // One more failure = death
	input := &DeathSaveInput{
		Roller: s.mockRoller,
		State:  state,
	}

	result, err := MakeDeathSave(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(3, result.State.Failures, "should have 3 failures")
	s.True(result.State.Dead, "should be dead with 3 failures")
	s.False(result.State.Stabilized, "dead is not stabilized")
}

// TestThreeSuccessesStabilizes tests that 3 successes results in stabilization
func (s *DeathSaveTestSuite) TestThreeSuccessesStabilizes() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)

	state := &DeathSaveState{Successes: 2} // One more success = stabilized
	input := &DeathSaveInput{
		Roller: s.mockRoller,
		State:  state,
	}

	result, err := MakeDeathSave(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(3, result.State.Successes, "should have 3 successes")
	s.True(result.State.Stabilized, "should be stabilized with 3 successes")
	s.False(result.State.Dead, "stabilized is not dead")
}

// TestRoll1WithTwoFailuresCausesDeath tests that rolling 1 with 2 existing failures causes death (2 more failures)
func (s *DeathSaveTestSuite) TestRoll1WithTwoFailuresCausesDeath() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	state := &DeathSaveState{Failures: 2} // Rolling 1 adds 2, total = 4, capped at 3 = death
	input := &DeathSaveInput{
		Roller: s.mockRoller,
		State:  state,
	}

	result, err := MakeDeathSave(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.True(result.State.Dead, "should be dead")
	s.GreaterOrEqual(result.State.Failures, 3, "should have at least 3 failures")
}

// TestDamageWhileUnconsciousAddsOneFailure tests normal damage adds 1 failure
func (s *DeathSaveTestSuite) TestDamageWhileUnconsciousAddsOneFailure() {
	state := &DeathSaveState{}
	input := &DamageWhileUnconsciousInput{
		State:      state,
		IsCritical: false,
	}

	result, err := TakeDamageWhileUnconscious(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(1, result.State.Failures, "normal damage should add 1 failure")
	s.False(result.State.Dead, "should not be dead with 1 failure")
}

// TestCriticalDamageWhileUnconsciousAddsTwoFailures tests critical damage adds 2 failures
func (s *DeathSaveTestSuite) TestCriticalDamageWhileUnconsciousAddsTwoFailures() {
	state := &DeathSaveState{}
	input := &DamageWhileUnconsciousInput{
		State:      state,
		IsCritical: true,
	}

	result, err := TakeDamageWhileUnconscious(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(2, result.State.Failures, "critical damage should add 2 failures")
	s.False(result.State.Dead, "should not be dead with 2 failures")
}

// TestDamageWhileUnconsciousCausesDeath tests that damage can cause death
func (s *DeathSaveTestSuite) TestDamageWhileUnconsciousCausesDeath() {
	state := &DeathSaveState{Failures: 2}
	input := &DamageWhileUnconsciousInput{
		State:      state,
		IsCritical: false,
	}

	result, err := TakeDamageWhileUnconscious(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(3, result.State.Failures, "should have 3 failures")
	s.True(result.State.Dead, "should be dead with 3 failures")
}

// TestNilInputReturnsError tests that nil input returns an error
func (s *DeathSaveTestSuite) TestNilInputReturnsError() {
	result, err := MakeDeathSave(s.ctx, nil)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "input cannot be nil")
}

// TestNilStateReturnsError tests that nil state in input returns an error
func (s *DeathSaveTestSuite) TestNilStateReturnsError() {
	input := &DeathSaveInput{
		Roller: s.mockRoller,
		State:  nil,
	}

	result, err := MakeDeathSave(s.ctx, input)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "state cannot be nil")
}
