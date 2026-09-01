package checks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type AbilityCheckTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	mockRoller *mock_dice.MockRoller
	bus        events.EventBus
}

func TestAbilityCheckSuite(t *testing.T) {
	suite.Run(t, new(AbilityCheckTestSuite))
}

func (s *AbilityCheckTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.bus = events.NewEventBus()
}

func (s *AbilityCheckTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestBasicSuccess tests that a check succeeds when roll + modifier >= DC.
// This is the Hide-success shape: Stealth roll beats the observer's passive
// Perception (used as the DC).
func (s *AbilityCheckTestSuite) TestBasicSuccess() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	input := &AbilityCheckInput{
		Roller:    s.mockRoller,
		EventBus:  s.bus,
		CheckerID: "hero",
		Skill:     skills.Stealth,
		DC:        15, // observer's passive Perception
		Modifier:  3,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(14, result.Roll)
	s.Equal(17, result.Total, "total should be 14 + 3 = 17")
	s.Equal(15, result.DC)
	s.True(result.Success, "17 should succeed against DC 15")
	s.False(result.IsNat1)
	s.False(result.IsNat20)
}

// TestBasicFailure tests the Hide-failure branch (R6): Stealth total below
// the observer's passive Perception. This is exercised as a unit test with
// a mock roller because the live orchestrator has no deterministic-roller
// seam (crypto-random roller, no seed in devseed).
func (s *AbilityCheckTestSuite) TestBasicFailure() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	input := &AbilityCheckInput{
		Roller:    s.mockRoller,
		EventBus:  s.bus,
		CheckerID: "hero",
		Skill:     skills.Stealth,
		DC:        15,
		Modifier:  3,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll)
	s.Equal(8, result.Total, "total should be 5 + 3 = 8")
	s.Equal(15, result.DC)
	s.False(result.Success, "8 should fail against DC 15")
}

func (s *AbilityCheckTestSuite) TestAdvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{8, 15}, nil)

	input := &AbilityCheckInput{
		Roller:       s.mockRoller,
		EventBus:     s.bus,
		CheckerID:    "hero",
		Skill:        skills.Stealth,
		DC:           12,
		Modifier:     2,
		HasAdvantage: true,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll, "should use higher roll of 8 and 15")
	s.Equal(17, result.Total)
	s.True(result.Success)
}

func (s *AbilityCheckTestSuite) TestDisadvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{18, 5}, nil)

	input := &AbilityCheckInput{
		Roller:          s.mockRoller,
		EventBus:        s.bus,
		CheckerID:       "hero",
		Skill:           skills.Stealth,
		DC:              15,
		Modifier:        4,
		HasDisadvantage: true,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll, "should use lower roll of 18 and 5")
	s.Equal(9, result.Total)
	s.False(result.Success)
}

func (s *AbilityCheckTestSuite) TestAdvantageAndDisadvantageCancelOut() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(11, nil)

	input := &AbilityCheckInput{
		Roller:          s.mockRoller,
		EventBus:        s.bus,
		CheckerID:       "hero",
		Skill:           skills.Stealth,
		DC:              15,
		Modifier:        2,
		HasAdvantage:    true,
		HasDisadvantage: true,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(11, result.Roll)
	s.Equal(13, result.Total)
	s.False(result.Success)
}

func (s *AbilityCheckTestSuite) TestNatural1AndNatural20() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	result, err := MakeAbilityCheck(s.ctx, &AbilityCheckInput{
		Roller:    s.mockRoller,
		EventBus:  s.bus,
		CheckerID: "hero",
		Skill:     skills.Stealth,
		DC:        5,
		Modifier:  10,
	})
	s.Require().NoError(err)
	s.True(result.IsNat1)
	s.False(result.IsNat20)
}

func (s *AbilityCheckTestSuite) TestNilInput() {
	result, err := MakeAbilityCheck(s.ctx, nil)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "input cannot be nil")
}

// TestRefusesNilEventBus pins the required-bus contract (rpg-toolkit#1357):
// a full ability check consults the chain, so a nil bus is refused by name
// rather than quietly skipping every condition. The error points the caller
// at the unaided variant.
func (s *AbilityCheckTestSuite) TestRefusesNilEventBus() {
	result, err := MakeAbilityCheck(s.ctx, &AbilityCheckInput{
		Roller:    s.mockRoller,
		EventBus:  nil,
		CheckerID: "hero",
		Skill:     skills.Stealth,
		DC:        12,
		Modifier:  3,
	})
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "EventBus is required")
	s.Contains(err.Error(), "MakeUnaidedAbilityCheck")
}

// TestRefusesEmptyCheckerID pins the other required parameter: chain
// subscribers key off the checker's id, so an empty id is refused by name.
func (s *AbilityCheckTestSuite) TestRefusesEmptyCheckerID() {
	result, err := MakeAbilityCheck(s.ctx, &AbilityCheckInput{
		Roller:   s.mockRoller,
		EventBus: s.bus,
		Skill:    skills.Stealth,
		DC:       12,
		Modifier: 3,
	})
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "CheckerID is required")
}

// TestChainGrantsAdvantage tests that a chain subscriber (e.g. a future
// Hidden-on-check condition) can grant advantage via AbilityCheckChain.
func (s *AbilityCheckTestSuite) TestChainGrantsAdvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{8, 15}, nil)

	checkChain := dnd5eEvents.AbilityCheckChain.On(s.bus)
	_, err := checkChain.SubscribeWithChain(s.ctx,
		func(_ context.Context, event *dnd5eEvents.AbilityCheckChainEvent,
			c chain.Chain[*dnd5eEvents.AbilityCheckChainEvent],
		) (chain.Chain[*dnd5eEvents.AbilityCheckChainEvent], error) {
			if event.Skill == skills.Stealth {
				addErr := c.Add(combat.StageConditions, "guidance",
					func(_ context.Context, e *dnd5eEvents.AbilityCheckChainEvent,
					) (*dnd5eEvents.AbilityCheckChainEvent, error) {
						e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.CheckModifierSource{
							Name:       "Guidance",
							SourceType: "spell",
							EntityID:   "hero",
						})
						return e, nil
					})
				if addErr != nil {
					return c, addErr
				}
			}
			return c, nil
		})
	s.Require().NoError(err)

	input := &AbilityCheckInput{
		Roller:    s.mockRoller,
		EventBus:  s.bus,
		CheckerID: "hero",
		Skill:     skills.Stealth,
		DC:        15,
		Modifier:  2,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll, "should use higher roll due to advantage")
	s.Equal(17, result.Total)
	s.True(result.Success)
	s.Len(result.AdvantageSources, 1)
	s.Equal("Guidance", result.AdvantageSources[0].Name)
}

// TestChainAddsBonus tests that a chain subscriber can add bonuses to the roll.
func (s *AbilityCheckTestSuite) TestChainAddsBonus() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(10, nil)

	checkChain := dnd5eEvents.AbilityCheckChain.On(s.bus)
	_, err := checkChain.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.AbilityCheckChainEvent,
			c chain.Chain[*dnd5eEvents.AbilityCheckChainEvent],
		) (chain.Chain[*dnd5eEvents.AbilityCheckChainEvent], error) {
			addErr := c.Add(combat.StageConditions, "bless",
				func(_ context.Context, e *dnd5eEvents.AbilityCheckChainEvent,
				) (*dnd5eEvents.AbilityCheckChainEvent, error) {
					e.BonusSources = append(e.BonusSources, dnd5eEvents.CheckBonusSource{
						CheckModifierSource: dnd5eEvents.CheckModifierSource{
							Name:       "Bless",
							SourceType: "spell",
							EntityID:   "cleric",
						},
						Bonus: 3,
					})
					return e, nil
				})
			if addErr != nil {
				return c, addErr
			}
			return c, nil
		})
	s.Require().NoError(err)

	input := &AbilityCheckInput{
		Roller:    s.mockRoller,
		EventBus:  s.bus,
		CheckerID: "hero",
		Skill:     skills.Stealth,
		DC:        15,
		Modifier:  2,
	}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(10, result.Roll)
	s.Equal(15, result.Total, "total should be 10 + 2 (modifier) + 3 (bless) = 15")
	s.True(result.Success)
	s.Len(result.BonusSources, 1)
	s.Equal("Bless", result.BonusSources[0].Name)
	s.Equal(3, result.BonusSources[0].Bonus)
}

// UnaidedAbilityCheckTestSuite pins MakeUnaidedAbilityCheck's contract: the
// same roll arithmetic as the full check — advantage/disadvantage
// cancellation, modifier, DC, natural 1/20 — with no chain and no bus, so
// no condition can reach the roll (rpg-toolkit#1357).
type UnaidedAbilityCheckTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	mockRoller *mock_dice.MockRoller
}

func TestUnaidedAbilityCheckSuite(t *testing.T) {
	suite.Run(t, new(UnaidedAbilityCheckTestSuite))
}

func (s *UnaidedAbilityCheckTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *UnaidedAbilityCheckTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestBasicSuccess is the session-resolver shape: roll + sheet modifier
// against a DC, no conditions consulted (the session seam is bus-free by
// structural pin).
func (s *UnaidedAbilityCheckTestSuite) TestBasicSuccess() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)

	result, err := MakeUnaidedAbilityCheck(s.ctx, &UnaidedAbilityCheckInput{
		Roller:   s.mockRoller,
		DC:       12,
		Modifier: 3,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll)
	s.Equal(18, result.Total, "total should be 15 + 3 = 18")
	s.Equal(12, result.DC)
	s.True(result.Success, "18 should succeed against DC 12")
	s.Empty(result.AdvantageSources, "no conditions consulted, nothing granted advantage")
	s.Empty(result.DisadvantageSources)
	s.Empty(result.BonusSources)
}

func (s *UnaidedAbilityCheckTestSuite) TestBasicFailure() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	result, err := MakeUnaidedAbilityCheck(s.ctx, &UnaidedAbilityCheckInput{
		Roller:   s.mockRoller,
		DC:       15,
		Modifier: 3,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll)
	s.Equal(8, result.Total, "total should be 5 + 3 = 8")
	s.False(result.Success, "8 should fail against DC 15")
}

func (s *UnaidedAbilityCheckTestSuite) TestAdvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{8, 15}, nil)

	result, err := MakeUnaidedAbilityCheck(s.ctx, &UnaidedAbilityCheckInput{
		Roller:       s.mockRoller,
		DC:           12,
		Modifier:     2,
		HasAdvantage: true,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll, "should use higher roll of 8 and 15")
	s.Equal(17, result.Total)
	s.True(result.Success)
	s.Len(result.AdvantageSources, 1, "input advantage is tracked for auditability")
	s.Equal("Input", result.AdvantageSources[0].Name)
}

func (s *UnaidedAbilityCheckTestSuite) TestDisadvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{18, 5}, nil)

	result, err := MakeUnaidedAbilityCheck(s.ctx, &UnaidedAbilityCheckInput{
		Roller:          s.mockRoller,
		DC:              15,
		Modifier:        4,
		HasDisadvantage: true,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll, "should use lower roll of 18 and 5")
	s.Equal(9, result.Total)
	s.False(result.Success)
	s.Len(result.DisadvantageSources, 1, "input disadvantage is tracked for auditability")
	s.Equal("Input", result.DisadvantageSources[0].Name)
}

func (s *UnaidedAbilityCheckTestSuite) TestAdvantageAndDisadvantageCancelOut() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(11, nil)

	result, err := MakeUnaidedAbilityCheck(s.ctx, &UnaidedAbilityCheckInput{
		Roller:          s.mockRoller,
		DC:              15,
		Modifier:        2,
		HasAdvantage:    true,
		HasDisadvantage: true,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(11, result.Roll, "should roll normally when advantage and disadvantage cancel")
	s.Equal(13, result.Total)
	s.False(result.Success)
}

func (s *UnaidedAbilityCheckTestSuite) TestNatural1AndNatural20() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	result, err := MakeUnaidedAbilityCheck(s.ctx, &UnaidedAbilityCheckInput{
		Roller:   s.mockRoller,
		DC:       5,
		Modifier: 10,
	})
	s.Require().NoError(err)
	s.True(result.IsNat1)
	s.False(result.IsNat20)
}

func (s *UnaidedAbilityCheckTestSuite) TestNilInput() {
	result, err := MakeUnaidedAbilityCheck(s.ctx, nil)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "input cannot be nil")
}
