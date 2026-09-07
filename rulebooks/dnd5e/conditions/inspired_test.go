package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type InspiredConditionTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	condition *InspiredCondition
	removed   []dnd5eEvents.ConditionRemovedEvent
}

func (s *InspiredConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.condition = NewInspiredCondition("fighter-1", "bard-1", InspiredDie)
	s.removed = nil

	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			s.removed = append(s.removed, event)
			return nil
		})
	s.Require().NoError(err)
	s.Require().NoError(s.condition.Apply(s.ctx, s.bus))
}

// foldOffers runs the offer chain the way the strike machine does and hands
// back what was on the table.
func (s *InspiredConditionTestSuite) foldOffers(attacker string) []dnd5eEvents.Offer {
	event := &dnd5eEvents.PostRollOfferEvent{
		AttackerID: attacker, TargetID: "skeleton-1",
		Roll: 11, AttackBonus: 5, Total: 16,
	}
	chain := events.NewStagedChain[*dnd5eEvents.PostRollOfferEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.PostRollOfferChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	return folded.Offers
}

// TestItOffersOnTheHoldersOwnRoll is the whole mechanism: the die goes on the
// table, and no number moves.
func (s *InspiredConditionTestSuite) TestItOffersOnTheHoldersOwnRoll() {
	offers := s.foldOffers("fighter-1")

	s.Require().Len(offers, 1)
	s.Equal(refs.Conditions.Inspired().String(), offers[0].Ref.String())
	s.Equal(InspiredName, offers[0].Name)
	s.Equal("fighter-1", offers[0].Audience, "the choice belongs to whoever holds the die")
	s.Equal(InspiredDie, offers[0].Die)
	s.Empty(s.removed, "offering costs nothing")
	s.True(s.condition.IsApplied())
}

// TestItOffersNothingOnSomebodyElsesRoll pins the filter. Without it a whole
// party's rolls would each carry one member's die.
func (s *InspiredConditionTestSuite) TestItOffersNothingOnSomebodyElsesRoll() {
	s.Empty(s.foldOffers("wizard-1"))
}

// TestItNeverTouchesTheAttackChain is R3 made mechanical: the die is not an
// unsourced AttackBonus, so folding the attack chain must change nothing.
func (s *InspiredConditionTestSuite) TestItNeverTouchesTheAttackChain() {
	event := dnd5eEvents.AttackChainEvent{AttackerID: "fighter-1", TargetID: "skeleton-1", AttackBonus: 5}
	chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Equal(5, folded.AttackBonus)
	s.Empty(folded.AdvantageSources)
	s.True(s.condition.IsApplied(), "and the die is still in hand")
}

// TestTakingItSpendsItOnce is the ruling that makes declining free: the die
// goes when the offer is TAKEN, and it goes exactly once.
func (s *InspiredConditionTestSuite) TestTakingItSpendsItOnce() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "fighter-1", Ref: refs.Conditions.Inspired(), Face: 4,
	}))

	s.Require().Len(s.removed, 1)
	s.Equal("fighter-1", s.removed[0].MemberID)
	s.Equal(refs.Conditions.Inspired().String(), s.removed[0].ConditionRef)
	s.Equal("spent", s.removed[0].Reason)
	s.False(s.condition.IsApplied())

	// And a second roll finds nothing to offer.
	s.Empty(s.foldOffers("fighter-1"))
}

// TestAnotherMembersSpendLeavesThisDieAlone: two inspired allies on one bus
// must not consume each other's dice.
func (s *InspiredConditionTestSuite) TestAnotherMembersSpendLeavesThisDieAlone() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "wizard-1", Ref: refs.Conditions.Inspired(), Face: 6,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
	s.Len(s.foldOffers("fighter-1"), 1)
}

// TestAnotherOffersSpendLeavesThisDieAlone: the day a second kind of offer
// rides this chain, its being taken must not eat the inspiration die.
func (s *InspiredConditionTestSuite) TestAnotherOffersSpendLeavesThisDieAlone() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "fighter-1", Ref: refs.Conditions.Helped(), Face: 3,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
}

// TestItEndsWithTheFight is R8's divergence: ten minutes is not a boundary
// this rulebook has, so the fight ending is the end.
func (s *InspiredConditionTestSuite) TestItEndsWithTheFight() {
	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: "fighter-1"}))

	s.Require().Len(s.removed, 1)
	s.Equal("combat ended", s.removed[0].Reason)
	s.False(s.condition.IsApplied())
}

func (s *InspiredConditionTestSuite) TestSomebodyElsesFightEndingLeavesIt() {
	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: "wizard-1"}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
}

func (s *InspiredConditionTestSuite) TestApplyingTwiceIsRefused() {
	s.Require().Error(s.condition.Apply(s.ctx, s.bus))
}

func (s *InspiredConditionTestSuite) TestRoundTripsThroughJSON() {
	raw, err := s.condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	reloaded, ok := loaded.(*InspiredCondition)
	s.Require().True(ok)
	s.Equal("fighter-1", reloaded.MemberID)
	s.Equal("bard-1", reloaded.SourceID)
	s.Equal(InspiredDie, reloaded.Die)
}

func (s *InspiredConditionTestSuite) TestFactoryRefusesAGrantWithNoGranter() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref: refs.Conditions.Inspired().String(), MemberID: "fighter-1",
	})
	s.Require().Error(err, "a die from nobody is a record that cannot be read back")
}

func TestInspiredConditionSuite(t *testing.T) {
	suite.Run(t, new(InspiredConditionTestSuite))
}
