package saves

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

type SavingThrowTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	mockRoller *mock_dice.MockRoller
}

func TestSavingThrowSuite(t *testing.T) {
	suite.Run(t, new(SavingThrowTestSuite))
}

func (s *SavingThrowTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *SavingThrowTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestBasicSuccess tests that a saving throw succeeds when roll + modifier >= DC
func (s *SavingThrowTestSuite) TestBasicSuccess() {
	// Roll 10 with +3 modifier should succeed against DC 13 (10+3=13 >= 13)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(10, nil)

	input := &SavingThrowInput{
		Ability:  abilities.CON,
		DC:       13,
		Modifier: 3,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(10, result.Roll, "roll should be 10")
	s.Equal(13, result.Total, "total should be 10 + 3 = 13")
	s.Equal(13, result.DC, "DC should match input")
	s.True(result.Success, "13 should succeed against DC 13")
	s.False(result.IsNat1, "10 is not a natural 1")
	s.False(result.IsNat20, "10 is not a natural 20")
}

// TestBasicFailure tests that a saving throw fails when roll + modifier < DC
func (s *SavingThrowTestSuite) TestBasicFailure() {
	// Roll 9 with +3 modifier should fail against DC 13 (9+3=12 < 13)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(9, nil)

	input := &SavingThrowInput{
		Ability:  abilities.CON,
		DC:       13,
		Modifier: 3,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(9, result.Roll, "roll should be 9")
	s.Equal(12, result.Total, "total should be 9 + 3 = 12")
	s.Equal(13, result.DC, "DC should match input")
	s.False(result.Success, "12 should fail against DC 13")
	s.False(result.IsNat1, "9 is not a natural 1")
	s.False(result.IsNat20, "9 is not a natural 20")
}

// TestAdvantage tests that advantage rolls two d20s and uses the higher result
func (s *SavingThrowTestSuite) TestAdvantage() {
	// Advantage should roll two dice and take the higher value
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{8, 15}, nil)

	input := &SavingThrowInput{
		Ability:      abilities.WIS,
		DC:           12,
		Modifier:     2,
		HasAdvantage: true,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll, "should use higher roll of 8 and 15")
	s.Equal(17, result.Total, "total should be 15 + 2 = 17")
	s.Equal(12, result.DC, "DC should match input")
	s.True(result.Success, "17 should succeed against DC 12")
	s.False(result.IsNat1, "15 is not a natural 1")
	s.False(result.IsNat20, "15 is not a natural 20")
}

// TestDisadvantage tests that disadvantage rolls two d20s and uses the lower result
func (s *SavingThrowTestSuite) TestDisadvantage() {
	// Disadvantage should roll two dice and take the lower value
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{18, 5}, nil)

	input := &SavingThrowInput{
		Ability:         abilities.DEX,
		DC:              15,
		Modifier:        4,
		HasDisadvantage: true,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll, "should use lower roll of 18 and 5")
	s.Equal(9, result.Total, "total should be 5 + 4 = 9")
	s.Equal(15, result.DC, "DC should match input")
	s.False(result.Success, "9 should fail against DC 15")
	s.False(result.IsNat1, "5 is not a natural 1")
	s.False(result.IsNat20, "5 is not a natural 20")
}

// TestNatural1 tests that a natural 1 is detected
func (s *SavingThrowTestSuite) TestNatural1() {
	// Natural 1 should be detected regardless of modifiers
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	input := &SavingThrowInput{
		Ability:  abilities.STR,
		DC:       5,
		Modifier: 10, // Even with a huge modifier, nat 1 is still detected
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(1, result.Roll, "roll should be 1")
	s.Equal(11, result.Total, "total should be 1 + 10 = 11")
	s.Equal(5, result.DC, "DC should match input")
	s.True(result.Success, "11 should succeed against DC 5 (nat 1 doesn't auto-fail saves)")
	s.True(result.IsNat1, "should detect natural 1")
	s.False(result.IsNat20, "1 is not a natural 20")
}

// TestNatural20 tests that a natural 20 is detected
func (s *SavingThrowTestSuite) TestNatural20() {
	// Natural 20 should be detected regardless of modifiers
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil)

	input := &SavingThrowInput{
		Ability:  abilities.INT,
		DC:       30,
		Modifier: -2, // Even with a negative modifier, nat 20 is still detected
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(20, result.Roll, "roll should be 20")
	s.Equal(18, result.Total, "total should be 20 - 2 = 18")
	s.Equal(30, result.DC, "DC should match input")
	s.False(result.Success, "18 should fail against DC 30 (nat 20 doesn't auto-succeed saves)")
	s.False(result.IsNat1, "20 is not a natural 1")
	s.True(result.IsNat20, "should detect natural 20")
}

// TestNatural20WithAdvantage tests that natural 20 is detected with advantage
func (s *SavingThrowTestSuite) TestNatural20WithAdvantage() {
	// When rolling with advantage, if either die is a 20, it should be detected
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{12, 20}, nil)

	input := &SavingThrowInput{
		Ability:      abilities.CHA,
		DC:           15,
		Modifier:     1,
		HasAdvantage: true,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(20, result.Roll, "should use higher roll of 12 and 20")
	s.Equal(21, result.Total, "total should be 20 + 1 = 21")
	s.True(result.Success, "21 should succeed against DC 15")
	s.False(result.IsNat1, "20 is not a natural 1")
	s.True(result.IsNat20, "should detect natural 20")
}

// TestNatural1WithDisadvantage tests that natural 1 is detected with disadvantage
func (s *SavingThrowTestSuite) TestNatural1WithDisadvantage() {
	// When rolling with disadvantage, if either die is a 1, and it's chosen, it should be detected
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{15, 1}, nil)

	input := &SavingThrowInput{
		Ability:         abilities.CON,
		DC:              10,
		Modifier:        3,
		HasDisadvantage: true,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(1, result.Roll, "should use lower roll of 15 and 1")
	s.Equal(4, result.Total, "total should be 1 + 3 = 4")
	s.False(result.Success, "4 should fail against DC 10")
	s.True(result.IsNat1, "should detect natural 1")
	s.False(result.IsNat20, "1 is not a natural 20")
}

// TestNegativeModifier tests saving throws with negative modifiers
func (s *SavingThrowTestSuite) TestNegativeModifier() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(12, nil)

	input := &SavingThrowInput{
		Ability:  abilities.INT,
		DC:       10,
		Modifier: -2,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(12, result.Roll, "roll should be 12")
	s.Equal(10, result.Total, "total should be 12 - 2 = 10")
	s.Equal(10, result.DC, "DC should match input")
	s.True(result.Success, "10 should succeed against DC 10")
}

// TestZeroModifier tests saving throws with no modifier
func (s *SavingThrowTestSuite) TestZeroModifier() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	input := &SavingThrowInput{
		Ability:  abilities.WIS,
		DC:       14,
		Modifier: 0,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(14, result.Roll, "roll should be 14")
	s.Equal(14, result.Total, "total should be 14 + 0 = 14")
	s.Equal(14, result.DC, "DC should match input")
	s.True(result.Success, "14 should succeed against DC 14")
}

// TestAdvantageAndDisadvantageCancelOut tests that having both advantage and disadvantage results in a normal roll
func (s *SavingThrowTestSuite) TestAdvantageAndDisadvantageCancelOut() {
	// When both advantage and disadvantage are present, they cancel out (D&D 5e rule)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(11, nil)

	input := &SavingThrowInput{
		Ability:         abilities.DEX,
		DC:              15,
		Modifier:        2,
		HasAdvantage:    true,
		HasDisadvantage: true,
	}

	result, err := MakeSavingThrow(s.ctx, s.mockRoller, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(11, result.Roll, "should roll normally when advantage and disadvantage cancel")
	s.Equal(13, result.Total, "total should be 11 + 2 = 13")
	s.False(result.Success, "13 should fail against DC 15")
}
