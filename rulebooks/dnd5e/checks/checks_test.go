package checks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

// stealthRoute is the route every check in this suite is rolled through. Its
// ref and name say the RULE; MakeAbilityCheck writes the CHECKER's id onto the
// die itself, which is the fact TestD20NamesTheRoller pins.
func stealthRoute() dnd5eEvents.RollSource {
	return dnd5eEvents.RollSource{Ref: refs.Skills.Stealth(), Name: "Stealth"}
}

func (s *AbilityCheckTestSuite) stealthInput(dc, modifier int) *AbilityCheckInput {
	return &AbilityCheckInput{
		Roller:         s.mockRoller,
		EventBus:       s.bus,
		CheckerID:      "hero",
		Skill:          skills.Stealth,
		DC:             dc,
		Modifier:       modifier,
		D20Source:      stealthRoute(),
		ModifierSource: stealthRoute(),
	}
}

// subscribeModifier puts one advantage or disadvantage source on the check
// chain, the way a condition does. Every source carries a ref, a name and the
// entity that brought it, because a keep record that cannot name who brought
// the rule is refused before any die is rolled.
func (s *AbilityCheckTestSuite) subscribeModifier(
	grant bool, name string, ref *core.Ref, entity string,
) {
	checkChain := dnd5eEvents.AbilityCheckChain.On(s.bus)
	_, err := checkChain.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.AbilityCheckChainEvent,
			c chain.Chain[*dnd5eEvents.AbilityCheckChainEvent],
		) (chain.Chain[*dnd5eEvents.AbilityCheckChainEvent], error) {
			addErr := c.Add(combat.StageConditions, name,
				func(_ context.Context, e *dnd5eEvents.AbilityCheckChainEvent,
				) (*dnd5eEvents.AbilityCheckChainEvent, error) {
					source := dnd5eEvents.CheckModifierSource{
						Name: name, SourceType: "condition", SourceRef: ref, EntityID: entity,
					}
					if grant {
						e.AdvantageSources = append(e.AdvantageSources, source)
					} else {
						e.DisadvantageSources = append(e.DisadvantageSources, source)
					}
					return e, nil
				})
			return c, addErr
		})
	s.Require().NoError(err)
}

// d20Trace reads the check's own die off the calculation. Component 0 is the
// d20 by contract — the same contract validateRecordedD20 checks at the
// encounter seam.
func (s *AbilityCheckTestSuite) d20Trace(result *AbilityCheckResult) *dnd5eEvents.DiceTrace {
	s.Require().NotNil(result.Calculation)
	s.Require().NotEmpty(result.Calculation.Components)
	trace := result.Calculation.Components[0].Dice
	s.Require().NotNil(trace)
	return trace
}

// TestBasicSuccess tests that a check succeeds when roll + modifier >= DC.
// This is the Hide-success shape: Stealth roll beats the observer's passive
// Perception (used as the DC).
func (s *AbilityCheckTestSuite) TestBasicSuccess() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 3))
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

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 3))
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll)
	s.Equal(8, result.Total, "total should be 5 + 3 = 8")
	s.Equal(15, result.DC)
	s.False(result.Success, "8 should fail against DC 15")
}

// TestStraightRollKeepsNothing is the first of the four trace facts: nobody
// touched the pool, so the zero value tells the truth — one die, no kept
// indices, no keep record.
func (s *AbilityCheckTestSuite) TestStraightRollKeepsNothing() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 3))
	s.Require().NoError(err)

	trace := s.d20Trace(result)
	s.Equal("1d20", trace.Notation)
	s.Equal([]int{14}, trace.FinalRolls)
	s.Empty(trace.KeptIndices)
	s.Nil(trace.Keep, "a straight roll has no keep rule to record")
}

// TestD20NamesTheRoller pins R7 for checks: the die's ref and name say the
// rule it was rolled under, and its SourceID is the CHECKER — so a client can
// draw it in the roller's own dice style without guessing from the beat's
// actor.
func (s *AbilityCheckTestSuite) TestD20NamesTheRoller() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 3))
	s.Require().NoError(err)

	d20 := result.Calculation.Components[0]
	s.Equal("hero", d20.Source.SourceID, "the d20 belongs to the checker")
	s.Equal("Stealth", d20.Source.Name, "and its rule is still the route")
	s.Equal(refs.Skills.Stealth().String(), d20.Source.Ref.String())
}

// TestAdvantage is the second trace fact: two dice, one kept index, the kept
// face is the higher, and the record names who granted it.
func (s *AbilityCheckTestSuite) TestAdvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{8, 15}, nil)
	s.subscribeModifier(true, "Raging", refs.Conditions.Raging(), "hero")

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(12, 2))
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll, "should use higher roll of 8 and 15")
	s.Equal(17, result.Total)
	s.True(result.Success)

	trace := s.d20Trace(result)
	s.Equal("2d20", trace.Notation)
	s.Equal([]int{8, 15}, trace.FinalRolls, "both faces survive to the log")
	s.Equal([]int{1}, trace.KeptIndices)
	s.Require().NotNil(trace.Keep)
	s.Equal(dnd5eEvents.KeepAdvantage, trace.Keep.Rule)
	s.Require().Len(trace.Keep.Granted, 1)
	s.Equal("Raging", trace.Keep.Granted[0].Name)
	s.Equal("hero", trace.Keep.Granted[0].SourceID)
	s.Empty(trace.Keep.Imposed)
}

// TestDisadvantage is the mirror: two dice, the lower face kept, and the rule
// that imposed it named. This is the case the check machine could not show at
// all — it returned one settled face and the second was thrown away.
func (s *AbilityCheckTestSuite) TestDisadvantage() {
	s.mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{18, 5}, nil)
	s.subscribeModifier(false, "Untrained", refs.Rules.Untrained(), "hero")

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 4))
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.Roll, "should use lower roll of 18 and 5")
	s.Equal(9, result.Total)
	s.False(result.Success)

	trace := s.d20Trace(result)
	s.Equal("2d20", trace.Notation)
	s.Equal([]int{18, 5}, trace.FinalRolls)
	s.Equal([]int{1}, trace.KeptIndices)
	s.Require().NotNil(trace.Keep)
	s.Equal(dnd5eEvents.KeepDisadvantage, trace.Keep.Rule)
	s.Require().Len(trace.Keep.Imposed, 1)
	s.Equal("Untrained", trace.Keep.Imposed[0].Name)
	s.Equal(refs.Rules.Untrained().String(), trace.Keep.Imposed[0].Ref.String())
	s.Empty(trace.Keep.Granted)
}

// TestAdvantageAndDisadvantageCancelOut is the fourth fact, and the one the
// log has never been able to show: RAW rolls one die, so we roll one die — and
// the record says which two rules met over it.
func (s *AbilityCheckTestSuite) TestAdvantageAndDisadvantageCancelOut() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(11, nil)
	s.subscribeModifier(true, "Helped", refs.Conditions.Helped(), "ally")
	s.subscribeModifier(false, "Untrained", refs.Rules.Untrained(), "hero")

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 2))
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(11, result.Roll)
	s.Equal(13, result.Total)
	s.False(result.Success)

	trace := s.d20Trace(result)
	s.Equal("1d20", trace.Notation, "cancellation rolls one die, as the book says")
	s.Empty(trace.KeptIndices)
	s.Require().NotNil(trace.Keep, "and unlike a straight roll, it says why")
	s.Equal(dnd5eEvents.KeepCancelled, trace.Keep.Rule)
	s.Require().Len(trace.Keep.Granted, 1)
	s.Equal("ally", trace.Keep.Granted[0].SourceID, "the help was somebody else's")
	s.Require().Len(trace.Keep.Imposed, 1)
	s.Equal("Untrained", trace.Keep.Imposed[0].Name)
}

func (s *AbilityCheckTestSuite) TestNatural1AndNatural20() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(5, 10))
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
// an ability check consults the chain, so a nil bus is refused by name
// rather than quietly skipping every condition.
func (s *AbilityCheckTestSuite) TestRefusesNilEventBus() {
	input := s.stealthInput(12, 3)
	input.EventBus = nil

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "EventBus is required")
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err), "rpg-api routes on the code, not the text")
}

// TestRefusesEmptyCheckerID pins the other required parameter: chain
// subscribers key off the checker's id, so an empty id is refused by name.
func (s *AbilityCheckTestSuite) TestRefusesEmptyCheckerID() {
	input := s.stealthInput(12, 3)
	input.CheckerID = ""

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "CheckerID is required")
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err), "rpg-api routes on the code, not the text")
}

// TestRefusesUnsourcedD20 pins the calculation's own strictness: a check that
// cannot say which rule threw its die is refused before any die is thrown.
func (s *AbilityCheckTestSuite) TestRefusesUnsourcedD20() {
	input := s.stealthInput(12, 3)
	input.D20Source = dnd5eEvents.RollSource{}

	result, err := MakeAbilityCheck(s.ctx, input)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "d20 source is invalid")
}

// TestRefusesAdvantageSourceWithoutARef pins the keep record's strictness at
// the place it can still fail closed: a rule that reaches the chain without a
// ref cannot be named in the log, so the roll is refused rather than recorded
// as an anonymous advantage.
func (s *AbilityCheckTestSuite) TestRefusesAdvantageSourceWithoutARef() {
	s.subscribeModifier(true, "Mystery", nil, "hero")

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(12, 3))
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "source ref is required")
}

// TestRefusesAdvantageSourceWithoutAnEntity is R7 on the keep record: a rule
// granted by nobody is refused.
func (s *AbilityCheckTestSuite) TestRefusesAdvantageSourceWithoutAnEntity() {
	s.subscribeModifier(true, "Raging", refs.Conditions.Raging(), "")

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(12, 3))
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "source id is required")
}

// TestChainSourceMapsOntoRollSource pins the one-to-one mapping the log reads:
// SourceRef->Ref, Name->Name, SourceType->Label, EntityID->SourceID. The word
// the log prints comes down from the server; the web never invents it.
func (s *AbilityCheckTestSuite) TestChainSourceMapsOntoRollSource() {
	source := dnd5eEvents.CheckModifierSource{
		Name:       "Untrained",
		SourceType: "rule",
		SourceRef:  refs.Rules.Untrained(),
		EntityID:   "hero",
	}

	mapped := source.RollSource()

	s.Equal(refs.Rules.Untrained().String(), mapped.Ref.String())
	s.Equal("Untrained", mapped.Name)
	s.Equal("rule", mapped.Label)
	s.Equal("hero", mapped.SourceID)
	s.NotSame(source.SourceRef, mapped.Ref, "the mapped source owns its ref")
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
							SourceRef:  refs.Spells.Bless(),
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

	result, err := MakeAbilityCheck(s.ctx, s.stealthInput(15, 2))
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(10, result.Roll)
	s.Equal(15, result.Total, "total should be 10 + 2 (modifier) + 3 (bless) = 15")
	s.True(result.Success)
	s.Len(result.BonusSources, 1)
	s.Equal("Bless", result.BonusSources[0].Name)
	s.Equal(3, result.BonusSources[0].Bonus)

	s.Require().NotNil(result.Calculation)
	s.Require().Len(result.Calculation.Components, 3, "d20, modifier, then one component per bonus")
	s.Equal(15, result.Calculation.Total)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(result.Calculation))
	s.Equal("cleric", result.Calculation.Components[2].Source.SourceID, "the bonus names the cleric")
}
