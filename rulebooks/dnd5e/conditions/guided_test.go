package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type GuidedConditionTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	condition *GuidedCondition
	removed   []dnd5eEvents.ConditionRemovedEvent
}

func (s *GuidedConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	condition, err := NewGuidedCondition(NewGuidedConditionInput{
		MemberID: "rogue-1", SourceID: "cleric-1", SourceRef: refs.Spells.Guidance(),
	})
	s.Require().NoError(err)
	s.condition = condition
	s.removed = nil

	_, err = dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			s.removed = append(s.removed, event)
			return nil
		})
	s.Require().NoError(err)
	s.Require().NoError(s.condition.Apply(s.ctx, s.bus))
}

// foldOffers runs the check-offer chain the way the resumed-check resolution
// will and hands back what was on the table.
func (s *GuidedConditionTestSuite) foldOffers(checker string) []dnd5eEvents.Offer {
	event := &dnd5eEvents.PostCheckRollOfferEvent{CheckerID: checker, Roll: 9, Total: 14}
	chain := events.NewStagedChain[*dnd5eEvents.PostCheckRollOfferEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.PostCheckRollOfferChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	return folded.Offers
}

// TestItOffersOnTheHoldersOwnCheck is the whole mechanism: the die goes on
// the table, and no number moves.
func (s *GuidedConditionTestSuite) TestItOffersOnTheHoldersOwnCheck() {
	offers := s.foldOffers("rogue-1")

	s.Require().Len(offers, 1)
	s.Equal(refs.Conditions.Guided().String(), offers[0].Ref.String())
	s.Equal(GuidedName, offers[0].Name)
	s.Equal("rogue-1", offers[0].Audience, "the choice belongs to whoever holds the die")
	s.Equal(GuidedDie, offers[0].Die)
	s.Empty(s.removed, "offering costs nothing")
	s.True(s.condition.IsApplied())
}

// TestItOffersOnAnyCheck pins RAW's "one ability check of its choice" — no
// skill filter, unlike a feature scoped to one skill.
func (s *GuidedConditionTestSuite) TestItOffersOnAnyCheck() {
	s.Len(s.foldOffers("rogue-1"), 1)
}

// TestItOffersNothingOnSomebodyElsesCheck pins the filter. Without it a
// whole party's checks would each carry one member's die.
func (s *GuidedConditionTestSuite) TestItOffersNothingOnSomebodyElsesCheck() {
	s.Empty(s.foldOffers("cleric-1"))
}

// TestItNeverTouchesTheCheckChain: the die is an offer, not an unsourced
// bonus, so folding the ordinary ability-check chain must change nothing.
func (s *GuidedConditionTestSuite) TestItNeverTouchesTheCheckChain() {
	event := &dnd5eEvents.AbilityCheckChainEvent{CheckerID: "rogue-1", DC: 15}
	chain := events.NewStagedChain[*dnd5eEvents.AbilityCheckChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.AbilityCheckChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Empty(folded.BonusSources)
	s.True(s.condition.IsApplied(), "and the die is still in hand")
}

// TestTakingItSpendsItOnce is the ruling that makes declining free: the die
// goes when the offer is TAKEN, and RAW's "the spell then ends" follows —
// the condition removes itself rather than waiting on concentration.
func (s *GuidedConditionTestSuite) TestTakingItSpendsItOnce() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "rogue-1", Ref: refs.Conditions.Guided(), Face: 3,
	}))

	s.Require().Len(s.removed, 1)
	s.Equal("rogue-1", s.removed[0].MemberID)
	s.Equal(refs.Conditions.Guided().String(), s.removed[0].ConditionRef)
	s.Equal("cleric-1", s.removed[0].SourceID,
		"must match ConditionAddress's own SourceID, or a keeper's three-field "+
			"removal match never finds this condition to drop it")
	s.Equal("spent", s.removed[0].Reason)
	s.False(s.condition.IsApplied())

	// And a second check finds nothing to offer.
	s.Empty(s.foldOffers("rogue-1"))
}

// TestAnotherMembersSpendLeavesThisDieAlone: two Guided allies on one bus
// must not consume each other's dice.
func (s *GuidedConditionTestSuite) TestAnotherMembersSpendLeavesThisDieAlone() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "cleric-1", Ref: refs.Conditions.Guided(), Face: 2,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
	s.Len(s.foldOffers("rogue-1"), 1)
}

// TestAnotherOffersSpendLeavesThisDieAlone: the day a second kind of offer
// rides this chain, its being taken must not eat the Guidance die.
func (s *GuidedConditionTestSuite) TestAnotherOffersSpendLeavesThisDieAlone() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "rogue-1", Ref: refs.Conditions.Inspired(), Face: 5,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
}

// TestALongRestTakesIt: Guidance is concentration, up to one minute — the
// same duration category as Bless, not Bardic Inspiration's combat-end. This
// pins the safety-net cleanup rather than a combat-end subscription.
func (s *GuidedConditionTestSuite) TestALongRestTakesIt() {
	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "rogue-1",
		RestType:    coreResources.ResetLongRest,
	}))

	s.Require().Len(s.removed, 1)
	s.Equal("long rest", s.removed[0].Reason)
	s.False(s.condition.IsApplied())
}

func (s *GuidedConditionTestSuite) TestSomebodyElsesLongRestLeavesIt() {
	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "cleric-1",
		RestType:    coreResources.ResetLongRest,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
}

func (s *GuidedConditionTestSuite) TestApplyingTwiceIsRefused() {
	s.Require().Error(s.condition.Apply(s.ctx, s.bus))
}

func (s *GuidedConditionTestSuite) TestRequiresRecipientCasterAndCanonicalSource() {
	_, err := NewGuidedCondition(NewGuidedConditionInput{SourceID: "cleric-1", SourceRef: refs.Spells.Guidance()})
	s.Require().Error(err, "no recipient")

	_, err = NewGuidedCondition(NewGuidedConditionInput{MemberID: "rogue-1", SourceRef: refs.Spells.Guidance()})
	s.Require().Error(err, "no caster")

	_, err = NewGuidedCondition(NewGuidedConditionInput{
		MemberID: "rogue-1", SourceID: "cleric-1", SourceRef: refs.Spells.Bless(),
	})
	s.Require().Error(err, "wrong source spell")
}

func (s *GuidedConditionTestSuite) TestRoundTripsThroughJSON() {
	raw, err := s.condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	reloaded, ok := loaded.(*GuidedCondition)
	s.Require().True(ok)
	s.Equal("rogue-1", reloaded.MemberID)
	s.Equal("cleric-1", reloaded.SourceID)
	s.Equal(refs.Spells.Guidance().String(), reloaded.SourceRef.String())
}

// TestTheFactoryBuildsItFromItsCounterpartKey pins the cast-delivery path a
// real Guidance cast takes: CreateFromRef, not the constructor directly,
// because that is what resolution's generic condition dispatch calls
// (BlessedCondition's shape needs a parsed *core.Ref too, so this path
// parses the caller-supplied source ref rather than deferring to the
// constructor — [TrueStrikeConditionSuite.TestItsSourceIsTheSpell]'s own
// test for the plain-string factories).
func (s *GuidedConditionTestSuite) TestTheFactoryBuildsItFromItsCounterpartKey() {
	out, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Guided().String(),
		MemberID:  "rogue-1",
		Config:    json.RawMessage(`{"source_id":"cleric-1"}`),
		SourceRef: refs.Spells.Guidance().String(),
	})
	s.Require().NoError(err)

	built, ok := out.Condition.(*GuidedCondition)
	s.Require().True(ok)
	s.Equal("rogue-1", built.MemberID)
	s.Equal("cleric-1", built.SourceID)
	s.Equal(refs.Spells.Guidance().String(), built.SourceRef.String())
}

func (s *GuidedConditionTestSuite) TestTheFactoryRefusesAWrongSourceSpell() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Guided().String(),
		MemberID:  "rogue-1",
		Config:    json.RawMessage(`{"source_id":"cleric-1"}`),
		SourceRef: refs.Spells.Bless().String(),
	})
	s.Require().Error(err, "only Guidance delivers this condition")
}

func TestGuidedConditionSuite(t *testing.T) {
	suite.Run(t, new(GuidedConditionTestSuite))
}
