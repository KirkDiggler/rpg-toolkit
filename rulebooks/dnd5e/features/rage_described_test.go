// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// RageDescribedSuite pins the description a barbarian's Rage compiles to and
// that Rage is used from it: a bonus action and a rage charge (none at
// barbarian 20) for the raging status on its user, the feature's own Activate
// refusing, and a refusal of the description for who she is and what she holds,
// never for want of a charge (the gate refuses that payment).
type RageDescribedSuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	rage *Rage
}

func TestRageDescribedSuite(t *testing.T) { suite.Run(t, new(RageDescribedSuite)) }

func (s *RageDescribedSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.rage = newRageForTest("rage-feature")
}

func (s *RageDescribedSuite) TestABarbariansDescription() {
	definition, err := s.rage.Definition(newStubEntityWithRage("barbarian-1", 1))

	s.Require().NoError(err)
	s.Require().NotNil(definition)
	s.Equal(*refs.Features.Rage(), definition.Ref)
	s.Equal("Rage", definition.Name)
	s.Equal(s.rage.Description(), definition.Description)

	s.Require().NotNil(definition.Cost)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Equal(map[coreResources.ResourceKey]int{resources.RageCharges: 1}, definition.Cost.Pools)

	s.Nil(definition.Attack)
	s.Nil(definition.Cast)
	s.Nil(definition.Sequence)
	s.Require().NotNil(definition.Effect)
	s.Equal(actions.EffectSelf, definition.Effect.To)
	s.Nil(definition.Effect.Healing, "rage heals nobody")
	s.Require().NotNil(definition.Effect.Condition)
	s.Equal(*refs.Conditions.Raging(), definition.Effect.Condition.Ref)
	s.JSONEq(`{"damage_bonus":{"barbarian_level":true}}`, string(definition.Effect.Condition.Parameters),
		"the bonus is read from her level when she strikes: no number is written")
	s.Require().NoError(definition.Validate())
}

func (s *RageDescribedSuite) TestAtLevel20TheCostHasNoPool() {
	definition, err := s.rage.Definition(newStubEntityWithRage("barbarian-1", 20))

	s.Require().NoError(err)
	s.Require().NotNil(definition.Cost)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Empty(definition.Cost.Pools, "a level 20 barbarian's rages are unlimited: no charge is named")
	s.Require().NoError(definition.Validate())
}

func (s *RageDescribedSuite) TestARageAlreadyRagingIsRefusedWithTheSentenceCanActivateGives() {
	for _, level := range []int{3, 20} {
		owner := newStubEntityWithRage("barbarian-1", level)
		raging, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
			Ref: refs.Conditions.Raging().String(), MemberID: "barbarian-1",
			Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "barbarian-1"},
			Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`),
		})
		s.Require().NoError(err)
		owner.conditions = []dnd5eEvents.ConditionBehavior{raging.Condition}

		definition, err := s.rage.Definition(owner)
		canActivate := s.rage.CanActivate(s.ctx, owner, FeatureInput{})

		s.Require().Error(err, "level %d", level)
		s.Nil(definition)
		s.Equal(rpgerr.CodeConflictingState, rpgerr.GetCode(err), "level %d", level)
		s.Contains(err.Error(), "already raging")
		s.Equal(canActivate.Error(), err.Error(), "the description and the row refuse in the same words")
	}
}

func (s *RageDescribedSuite) TestAnOwnerWhoseConditionsCannotBeReadIsRefused() {
	owner := &resourceOnlyOwner{StubEntity: newStubEntityWithRage("barbarian-1", 3)}

	definition, err := s.rage.Definition(owner)

	s.Require().Error(err, "unknown is not 'not raging'")
	s.Nil(definition)
}

func (s *RageDescribedSuite) TestAnOwnerWithNoBarbarianLevelIsRefusedWithTheSentenceCanActivateGives() {
	for name, levels := range map[string]map[classes.Class]int{
		"no barbarian levels": {classes.Fighter: 3},
		"no levels at all":    nil,
	} {
		owner := newStubEntityWithRage("barbarian-1", 3)
		owner.levels = levels

		definition, err := s.rage.Definition(owner)

		s.Require().Error(err, name)
		s.Nil(definition, name)
		s.Equal(s.rage.CanActivate(s.ctx, owner, FeatureInput{}).Error(), err.Error(), name)
	}
}

func (s *RageDescribedSuite) TestNoChargeLeftStillGetsADescriptionAndTheGateRefusesThePayment() {
	owner := newStubEntityWithRage("barbarian-1", 3)
	owner.resources[resources.RageCharges] = 0

	definition, err := s.rage.Definition(owner)

	s.Require().NoError(err, "a want of a charge is the gate's to refuse, not the description's")
	s.Require().NotNil(definition)
	ledger := &descriptionLedger{owner: owner, bonus: 1}
	payErr := combat.Pay(ledger, definition.Cost)
	s.Require().Error(payErr)
	s.Equal(1, ledger.bonus, "nothing was spent: not the bonus action either")
	s.Equal(0, owner.resources[resources.RageCharges])
	s.Error(s.rage.CanActivate(s.ctx, owner, FeatureInput{}), "the row still reads unavailable")
	s.Contains(s.rage.CanActivate(s.ctx, owner, FeatureInput{}).Error(), "no rage uses remaining")
}

func (s *RageDescribedSuite) TestUsedFromItsDescriptionCostsACharge() {
	owner := newStubEntityWithRage("barbarian-1", 3)
	var published []dnd5eEvents.ConditionAppliedEvent
	_, err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			published = append(published, event)
			return nil
		})
	s.Require().NoError(err)

	status, err := useRageFromItsDescription(s.ctx, s.T(), s.rage, owner, s.bus)

	s.Require().NoError(err)
	s.Equal(2, owner.resources[resources.RageCharges], "3 charges to 2")
	s.Require().Len(published, 1)
	s.Equal(owner, published[0].Target)
	s.Equal(dnd5eEvents.ConditionSourceFeature, published[0].Source)
	s.Equal(refs.Conditions.Raging(), published[0].Condition.Ref())
	s.Same(status, published[0].Condition)
	composed, ok := status.(*conditions.ComposedCondition)
	s.Require().True(ok, "the status is built through the shared composed creator")
	raw, err := composed.ToJSON()
	s.Require().NoError(err)
	var data conditions.ComposedData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Equal("barbarian-1", data.MemberID)
	s.Require().NotNil(data.Origin)
	s.Equal(*refs.Features.Rage(), data.Origin.Ref)
	s.Equal("barbarian-1", data.Origin.EntityID)
	s.JSONEq(`{"damage_bonus":{"barbarian_level":true}}`, string(data.Bindings),
		"persist the strategy, never a cached class level or computed bonus")
}

func (s *RageDescribedSuite) TestAtLevel20SheRagesAndNoChargeExistsOrIsSpent() {
	owner := &StubEntity{id: "barbarian-1", levels: map[classes.Class]int{classes.Barbarian: 20}}
	published := 0
	_, err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(s.ctx,
		func(context.Context, dnd5eEvents.ConditionAppliedEvent) error { published++; return nil })
	s.Require().NoError(err)

	_, err = useRageFromItsDescription(s.ctx, s.T(), s.rage, owner, s.bus)

	s.Require().NoError(err)
	s.Equal(1, published)
	s.Nil(owner.resources, "no pool was ever needed")
}

func (s *RageDescribedSuite) TestTheFeaturesOwnCodeRefusesAndTouchesNothing() {
	owner := newStubEntityWithRage("barbarian-1", 3)
	published := 0
	_, err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(s.ctx,
		func(context.Context, dnd5eEvents.ConditionAppliedEvent) error { published++; return nil })
	s.Require().NoError(err)

	err = s.rage.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})

	s.Require().Error(err)
	s.True(errors.Is(err, ErrUsedFromItsDescription), "got %v", err)
	s.Equal(3, owner.resources[resources.RageCharges], "the refusal spends nothing")
	s.Zero(published, "the refusal applies no status")
}
