package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type ResistanceConditionTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	condition *ResistanceCondition
	removed   []dnd5eEvents.ConditionRemovedEvent
}

func (s *ResistanceConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	condition, err := NewResistanceCondition(NewResistanceConditionInput{
		MemberID: "rogue-1", SourceID: "cleric-1", SourceRef: refs.Spells.Resistance(),
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

// foldOffers runs the save-offer chain the way the resumed-save resolution
// will and hands back what was on the table.
func (s *ResistanceConditionTestSuite) foldOffers(saver string) []dnd5eEvents.Offer {
	event := &dnd5eEvents.PostSaveRollOfferEvent{
		SaverID: saver, Ability: abilities.DEX, DC: 15, Roll: 9, Total: 14,
	}
	chain := events.NewStagedChain[*dnd5eEvents.PostSaveRollOfferEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.PostSaveRollOfferChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	return folded.Offers
}

// TestItOffersOnTheHoldersOwnSave is the whole mechanism: the die goes on
// the table, and no number moves.
func (s *ResistanceConditionTestSuite) TestItOffersOnTheHoldersOwnSave() {
	offers := s.foldOffers("rogue-1")

	s.Require().Len(offers, 1)
	s.Equal(refs.Conditions.Resistance().String(), offers[0].Ref.String())
	s.Equal(ResistanceName, offers[0].Name)
	s.Equal("rogue-1", offers[0].Audience, "the choice belongs to whoever holds the die")
	s.Equal(ResistanceDie, offers[0].Die)
	s.Empty(s.removed, "offering costs nothing")
	s.True(s.condition.IsApplied())
}

// TestItOffersOnAnySave pins RAW's "one saving throw of its choice" — no
// ability filter, unlike a feature scoped to one save.
func (s *ResistanceConditionTestSuite) TestItOffersOnAnySave() {
	s.Len(s.foldOffers("rogue-1"), 1)
}

// TestItOffersNothingOnSomebodyElsesSave pins the filter. Without it a whole
// party's saves would each carry one member's die.
func (s *ResistanceConditionTestSuite) TestItOffersNothingOnSomebodyElsesSave() {
	s.Empty(s.foldOffers("cleric-1"))
}

// TestTakingItSpendsItOnce is the ruling that makes declining free: the die
// goes when the offer is TAKEN, and RAW's "the spell then ends" follows —
// the condition removes itself rather than waiting on concentration.
func (s *ResistanceConditionTestSuite) TestTakingItSpendsItOnce() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "rogue-1", Ref: refs.Conditions.Resistance(), Face: 3,
	}))

	s.Require().Len(s.removed, 1)
	s.Equal("rogue-1", s.removed[0].MemberID)
	s.Equal(refs.Conditions.Resistance().String(), s.removed[0].ConditionRef)
	s.Equal("cleric-1", s.removed[0].SourceID,
		"must match ConditionAddress's own SourceID, or a keeper's three-field "+
			"removal match never finds this condition to drop it")
	s.Equal("spent", s.removed[0].Reason)
	s.False(s.condition.IsApplied())

	// And a second save finds nothing to offer.
	s.Empty(s.foldOffers("rogue-1"))
}

// TestAnotherMembersSpendLeavesThisDieAlone: two Resistance allies on one bus
// must not consume each other's dice.
func (s *ResistanceConditionTestSuite) TestAnotherMembersSpendLeavesThisDieAlone() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "cleric-1", Ref: refs.Conditions.Resistance(), Face: 2,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
	s.Len(s.foldOffers("rogue-1"), 1)
}

// TestAnotherOffersSpendLeavesThisDieAlone: the day a second kind of offer
// rides this chain, its being taken must not eat the Resistance die.
func (s *ResistanceConditionTestSuite) TestAnotherOffersSpendLeavesThisDieAlone() {
	s.Require().NoError(dnd5eEvents.OfferTakenTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.OfferTakenEvent{
		Audience: "rogue-1", Ref: refs.Conditions.Guided(), Face: 5,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
}

// TestALongRestTakesIt: Resistance is concentration, up to one minute — the
// same duration category as Bless and Guidance, not Bardic Inspiration's
// combat-end. This pins the safety-net cleanup rather than a combat-end
// subscription.
func (s *ResistanceConditionTestSuite) TestALongRestTakesIt() {
	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "rogue-1",
		RestType:    coreResources.ResetLongRest,
	}))

	s.Require().Len(s.removed, 1)
	s.Equal("long rest", s.removed[0].Reason)
	s.False(s.condition.IsApplied())
}

func (s *ResistanceConditionTestSuite) TestSomebodyElsesLongRestLeavesIt() {
	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "cleric-1",
		RestType:    coreResources.ResetLongRest,
	}))

	s.Empty(s.removed)
	s.True(s.condition.IsApplied())
}

func (s *ResistanceConditionTestSuite) TestApplyingTwiceIsRefused() {
	s.Require().Error(s.condition.Apply(s.ctx, s.bus))
}

func (s *ResistanceConditionTestSuite) TestRequiresRecipientCasterAndCanonicalSource() {
	_, err := NewResistanceCondition(NewResistanceConditionInput{
		SourceID: "cleric-1", SourceRef: refs.Spells.Resistance(),
	})
	s.Require().Error(err, "no recipient")

	_, err = NewResistanceCondition(NewResistanceConditionInput{
		MemberID: "rogue-1", SourceRef: refs.Spells.Resistance(),
	})
	s.Require().Error(err, "no caster")

	_, err = NewResistanceCondition(NewResistanceConditionInput{
		MemberID: "rogue-1", SourceID: "cleric-1", SourceRef: refs.Spells.Bless(),
	})
	s.Require().Error(err, "wrong source spell")
}

func (s *ResistanceConditionTestSuite) TestRoundTripsThroughJSON() {
	raw, err := s.condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	reloaded, ok := loaded.(*ResistanceCondition)
	s.Require().True(ok)
	s.Equal("rogue-1", reloaded.MemberID)
	s.Equal("cleric-1", reloaded.SourceID)
	s.Equal(refs.Spells.Resistance().String(), reloaded.SourceRef.String())
}

// TestTheFactoryBuildsItFromItsCounterpartKey pins the cast-delivery path a
// real Resistance cast takes: CreateFromRef, not the constructor directly,
// because that is what resolution's generic condition dispatch calls.
func (s *ResistanceConditionTestSuite) TestTheFactoryBuildsItFromItsCounterpartKey() {
	out, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Resistance().String(),
		MemberID:  "rogue-1",
		Config:    json.RawMessage(`{"source_id":"cleric-1"}`),
		SourceRef: refs.Spells.Resistance().String(),
	})
	s.Require().NoError(err)

	built, ok := out.Condition.(*ResistanceCondition)
	s.Require().True(ok)
	s.Equal("rogue-1", built.MemberID)
	s.Equal("cleric-1", built.SourceID)
	s.Equal(refs.Spells.Resistance().String(), built.SourceRef.String())
}

func (s *ResistanceConditionTestSuite) TestTheFactoryRefusesAWrongSourceSpell() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Resistance().String(),
		MemberID:  "rogue-1",
		Config:    json.RawMessage(`{"source_id":"cleric-1"}`),
		SourceRef: refs.Spells.Bless().String(),
	})
	s.Require().Error(err, "only Resistance delivers this condition")
}

func TestResistanceConditionSuite(t *testing.T) {
	suite.Run(t, new(ResistanceConditionTestSuite))
}
