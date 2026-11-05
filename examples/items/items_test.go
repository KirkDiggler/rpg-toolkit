// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

type compositionSuite struct{ suite.Suite }

func (s *compositionSuite) ring(id string, level, remaining int) *item {
	s.T().Helper()
	out, err := composeItem(composeItemInput{
		Definition: ringDefinition(level), State: instanceData{ID: id, Remaining: remaining},
	})
	s.Require().NoError(err)
	return out.Item
}

func (s *compositionSuite) attach(i *item, bus events.EventBus, holder string) {
	s.T().Helper()
	s.Require().NoError(i.attach(context.Background(), bus, holder))
	s.T().Cleanup(func() { s.NoError(i.detach(context.Background())) })
}

// Detects a missing grant attachment, ignoring item level, or duplicating healing.
func (s *compositionSuite) TestConfiguredLevelFlowsThroughComposedGrantToRealHealing() {
	for _, tc := range []struct{ level, want int }{{1, 7}, {3, 9}} {
		s.Run(fmt.Sprintf("level_%d", tc.level), func() {
			i := s.ring("ring-a", tc.level, 1)
			s.False(i.available())
			s.attach(i, events.NewEventBus(), "non-fighter")
			s.True(i.available())
			roller := &fixedRoller{face: 6}
			out, err := i.use(useInput{Holder: "non-fighter", Roller: roller})
			s.Require().NoError(err)
			s.Equal(tc.want, out.Calculation.Total)
			s.Len(out.Calculation.Components, 2)
			s.Equal("ring-a", out.Calculation.Components[0].Source.SourceID)
			s.Equal("ring-a", out.Calculation.Components[1].Source.SourceID)
			s.Equal(0, i.pool.Current)
			s.False(i.available())
			s.Equal(1, roller.calls)
		})
	}
}

// Detects budgets accidentally keyed by the shared recipe/ability rather than item.
func (s *compositionSuite) TestSameDefinitionItemsHaveIndependentBudgetsAndSources() {
	bus := events.NewEventBus()
	a, b := s.ring("ring-a", 3, 1), s.ring("ring-b", 3, 1)
	s.attach(a, bus, "wearer")
	s.attach(b, bus, "wearer")
	aResult, err := a.use(useInput{Holder: "wearer", Roller: &fixedRoller{face: 6}})
	s.Require().NoError(err)
	s.Equal(0, a.pool.Current)
	s.Equal(1, b.pool.Current)
	bResult, err := b.use(useInput{Holder: "wearer", Roller: &fixedRoller{face: 6}})
	s.Require().NoError(err)
	s.Equal("ring-a", aResult.Calculation.Components[0].Source.SourceID)
	s.Equal("ring-b", bResult.Calculation.Components[0].Source.SourceID)
	// Characterization, not an asserted global-ID contract: these IDs reveal the
	// context the consumer must supply around the existing effects boundary.
	s.T().Logf("provider output IDs: %q / %q; source item IDs: ring-a / ring-b", a.outputID, b.outputID)
}

// Detects premature payment/rolling, wrong-holder access, and lingering grants.
func (s *compositionSuite) TestRefusedUsesDoNotRollOrSpend() {
	i := s.ring("ring", 1, 1)
	roller := &fixedRoller{face: 6}
	_, err := i.use(useInput{Holder: "wearer", Roller: roller})
	s.Error(err)
	s.attach(i, events.NewEventBus(), "wearer")
	_, err = i.use(useInput{Holder: "stranger", Roller: roller})
	s.Error(err)
	s.Equal(1, i.pool.Current)
	s.Zero(roller.calls)
	_, err = i.use(useInput{Holder: "wearer", Roller: roller})
	s.Require().NoError(err)
	_, err = i.use(useInput{Holder: "wearer", Roller: roller})
	s.Error(err)
	s.Equal(1, roller.calls, "exhaustion must refuse before dice")
	s.Require().NoError(i.detach(context.Background()))
	s.False(i.available())
}

// Detects lost item state or rest subscriptions that outlive their attachment.
func (s *compositionSuite) TestTransferReloadAndRestRespectTheItemBudget() {
	ctx, bus := context.Background(), events.NewEventBus()
	i := s.ring("ring", 3, 1)
	s.attach(i, bus, "old-holder")
	_, err := i.use(useInput{Holder: "old-holder", Roller: &fixedRoller{face: 6}})
	s.Require().NoError(err)
	s.Require().NoError(i.detach(ctx))
	s.Require().NoError(restTopic.On(bus).Publish(ctx, restEvent{Holder: "old-holder", Kind: shortRest}))
	s.Zero(i.pool.Current, "detached consumer must not hear rest")
	encoded, err := json.Marshal(i.data())
	s.Require().NoError(err)
	var restored instanceData
	s.Require().NoError(json.Unmarshal(encoded, &restored))
	out, err := composeItem(composeItemInput{Definition: ringDefinition(3), State: restored})
	s.Require().NoError(err)
	i = out.Item
	s.attach(i, bus, "new-holder")
	s.False(i.available(), "transfer/reload must not mint a charge")
	s.Require().NoError(restTopic.On(bus).Publish(ctx, restEvent{Holder: "old-holder", Kind: shortRest}))
	s.Zero(i.pool.Current)
	s.Require().NoError(restTopic.On(bus).Publish(ctx, restEvent{Holder: "new-holder", Kind: shortRest}))
	s.Equal(1, i.pool.Current)
	_, err = i.use(useInput{Holder: "new-holder", Roller: &fixedRoller{face: 6}})
	s.Require().NoError(err)
	s.Require().NoError(restTopic.On(bus).Publish(ctx, restEvent{Holder: "new-holder", Kind: longRest}))
	s.Equal(1, i.pool.Current)
}

// The long-only variant is a seam test, not a newly selected ring rule.
func (s *compositionSuite) TestRecoveryPolicyComesFromConfigurationNotAbilityRef() {
	ctx, bus := context.Background(), events.NewEventBus()
	normal := s.ring("normal", 1, 0)
	definition := ringDefinition(1)
	definition.Recovery = []restKind{longRest}
	out, err := composeItem(composeItemInput{Definition: definition, State: instanceData{ID: "long-only", Remaining: 0}})
	s.Require().NoError(err)
	s.attach(normal, bus, "wearer")
	s.attach(out.Item, bus, "wearer")
	s.Require().NoError(restTopic.On(bus).Publish(ctx, restEvent{Holder: "wearer", Kind: shortRest}))
	s.Equal(1, normal.pool.Current)
	s.Zero(out.Item.pool.Current)
	s.Require().NoError(restTopic.On(bus).Publish(ctx, restEvent{Holder: "wearer", Kind: longRest}))
	s.Equal(1, out.Item.pool.Current)
}

// This rollback covers pure calculation before delivery, not distributed effects.
func (s *compositionSuite) TestFailedCalculationReturnsTheReservedCharge() {
	i := s.ring("ring", 1, 1)
	s.attach(i, events.NewEventBus(), "wearer")
	_, err := i.use(useInput{Holder: "wearer", Roller: &fixedRoller{err: errors.New("dice unavailable")}})
	s.Error(err)
	s.Equal(1, i.pool.Current)
}

func (s *compositionSuite) TestInvalidStateIsRefusedRatherThanClampedOrRefilled() {
	for _, remaining := range []int{-1, 2} {
		_, err := composeItem(composeItemInput{
			Definition: ringDefinition(1), State: instanceData{ID: "ring", Remaining: remaining},
		})
		s.Error(err)
	}
	_, err := composeItem(composeItemInput{Definition: ringDefinition(0), State: instanceData{ID: "ring", Remaining: 1}})
	s.Error(err)
}

// Detects hiding invalid authored vocabulary until after a grant becomes live.
func (s *compositionSuite) TestInvalidRecoveryIsRefusedBeforeGrantAttachment() {
	def := ringDefinition(1)
	def.Recovery = []restKind{"unsupported"}
	out, err := composeItem(composeItemInput{Definition: def, State: instanceData{ID: "ring", Remaining: 1}})
	s.Require().NoError(err)
	err = out.Item.attach(context.Background(), events.NewEventBus(), "wearer")
	s.Error(err)
	s.False(out.Item.available())
	s.Equal(1, out.Item.pool.Current)
}

func TestItemCompositionSeams(t *testing.T) { suite.Run(t, new(compositionSuite)) }
