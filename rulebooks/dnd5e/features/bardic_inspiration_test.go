package features

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

type BardicInspirationTestSuite struct {
	suite.Suite
	ctx     context.Context
	bus     events.EventBus
	feature *BardicInspiration
	applied []dnd5eEvents.ConditionAppliedEvent
}

func (s *BardicInspirationTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.feature = NewBardicInspiration()
	s.applied = nil

	_, err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			s.applied = append(s.applied, event)
			return nil
		})
	s.Require().NoError(err)
}

func (s *BardicInspirationTestSuite) bard(uses int) *StubEntity {
	return &StubEntity{
		id:        "bard-1",
		resources: map[coreResources.ResourceKey]int{resources.Inspiration: uses},
	}
}

func (s *BardicInspirationTestSuite) TestItIsABonusActionNamedForThePlayer() {
	s.Equal(coreCombat.ActionBonus, s.feature.ActionType())
	s.Equal(refs.Features.BardicInspiration(), s.feature.Ref())
	s.Equal(conditions.InspiredName, s.feature.Name())
}

// TestGrantSpendsAUseAndPublishesTheDie is #397's first done-when: the grant
// costs one use and the die lands on the ALLY, not the bard.
func (s *BardicInspirationTestSuite) TestGrantSpendsAUseAndPublishesTheDie() {
	owner := s.bard(3)
	ally := &StubEntity{id: "fighter-1"}

	s.Require().NoError(s.feature.Activate(s.ctx, owner, FeatureInput{Bus: s.bus, Target: ally}))

	s.Equal(2, owner.resources[resources.Inspiration], "one use spent")
	s.Require().Len(s.applied, 1)
	s.Equal("fighter-1", s.applied[0].Target.GetID())
	s.Equal(dnd5eEvents.ConditionInspired, s.applied[0].Type)

	inspired, ok := s.applied[0].Condition.(*conditions.InspiredCondition)
	s.Require().True(ok, "the published condition is the held die")
	s.Equal("fighter-1", inspired.MemberID)
	s.Equal("bard-1", inspired.SourceID, "the die remembers who gave it")
	s.Equal(conditions.InspiredDie, inspired.Die)
}

// TestRefuseSelf is RAW's "a creature other than yourself", and it must refuse
// BEFORE the use is spent — a self-grant that charged and did nothing is the
// failure this checks for.
func (s *BardicInspirationTestSuite) TestRefuseSelf() {
	owner := s.bard(2)

	err := s.feature.Activate(s.ctx, owner, FeatureInput{Bus: s.bus, Target: owner})

	s.Require().Error(err)
	s.Equal(2, owner.resources[resources.Inspiration], "nothing spent on a refused grant")
	s.Empty(s.applied)
}

// TestRefuseEmptyPool: an empty pool is refused before anything moves, and the
// refusal is the same one Afford reads to grey the row out.
func (s *BardicInspirationTestSuite) TestRefuseEmptyPool() {
	owner := s.bard(0)
	ally := &StubEntity{id: "fighter-1"}

	s.Require().Error(s.feature.CanActivate(s.ctx, owner, FeatureInput{}))

	err := s.feature.Activate(s.ctx, owner, FeatureInput{Bus: s.bus, Target: ally})
	s.Require().Error(err)
	s.Empty(s.applied)
}

// TestCanActivateSaysNothingAboutTheTarget pins the reason CanActivate does not
// check one: Afford calls it with an EMPTY input on every compile, and a target
// check there would report every bard as unable to inspire anybody, always.
func (s *BardicInspirationTestSuite) TestCanActivateSaysNothingAboutTheTarget() {
	s.Require().NoError(s.feature.CanActivate(s.ctx, s.bard(1), FeatureInput{}))
}

func (s *BardicInspirationTestSuite) TestRefuseWithoutBusOrTarget() {
	owner := s.bard(1)
	s.Require().Error(s.feature.Activate(s.ctx, owner, FeatureInput{Target: &StubEntity{id: "x"}}))
	s.Require().Error(s.feature.Activate(s.ctx, owner, FeatureInput{Bus: s.bus}))
	s.Equal(1, owner.resources[resources.Inspiration])
}

func (s *BardicInspirationTestSuite) TestRoundTripsThroughJSON() {
	raw, err := s.feature.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	s.Equal(refs.Features.BardicInspiration().String(), loaded.Ref().String())
	s.Equal(conditions.InspiredName, loaded.Name())
	s.Equal(coreCombat.ActionBonus, loaded.ActionType())
}

func (s *BardicInspirationTestSuite) TestStatusReportsTheOwnersPool() {
	out, err := s.feature.Status(&StatusInput{Owner: stubPoolOwner{current: 1, maximum: 3}})
	s.Require().NoError(err)
	s.Require().NotNil(out.Status.Resource)
	s.Equal(resources.Inspiration, out.Status.Resource.Key)
	s.Equal(1, out.Status.Resource.Current)
	s.Equal(3, out.Status.Resource.Maximum)
}

func (s *BardicInspirationTestSuite) TestStatusRefusesAnOwnerWithoutThePool() {
	_, err := s.feature.Status(&StatusInput{Owner: stubPoolOwner{missing: true}})
	s.Require().Error(err)
	_, err = s.feature.Status(nil)
	s.Require().Error(err)
}

// stubPoolOwner answers the one question Status asks.
type stubPoolOwner struct {
	current, maximum int
	missing          bool
}

func (o stubPoolOwner) ResourceStatus(_ coreResources.ResourceKey) (int, int, bool) {
	if o.missing {
		return 0, 0, false
	}
	return o.current, o.maximum, true
}

var _ core.Entity = (*StubEntity)(nil)

func TestBardicInspirationSuite(t *testing.T) {
	suite.Run(t, new(BardicInspirationTestSuite))
}
