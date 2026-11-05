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
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
)

type compositionSuite struct{ suite.Suite }

func (s *compositionSuite) ring(id string, level, remaining int) *items.Item {
	s.T().Helper()
	state := items.Data{ID: id, DefinitionRef: ringDefinition(level).Ref, Pools: map[string]int{chargePool: remaining}}
	item, err := items.New(ringConfig(id, level, &state))
	s.Require().NoError(err)
	return item
}

func (s *compositionSuite) attach(item *items.Item, bus events.EventBus, holder string) {
	s.T().Helper()
	s.Require().NoError(item.Attach(context.Background(), items.AttachInput{Bus: bus, HolderID: holder}))
	s.T().Cleanup(func() { s.NoError(item.Detach(context.Background())) })
}

func (s *compositionSuite) available(item *items.Item, holder string) bool {
	s.T().Helper()
	out, err := item.Offers(items.OffersInput{HolderID: holder})
	s.Require().NoError(err)
	for _, offer := range out.Grants {
		if offer.ID == windGrant {
			return offer.Affordable
		}
	}
	return false
}

func (s *compositionSuite) rest(bus events.EventBus, holder string, trigger items.Trigger) {
	s.T().Helper()
	s.Require().NoError(items.RecoveryTopic.On(bus).Publish(context.Background(), items.RecoveryEvent{
		HolderID: holder, Trigger: trigger,
	}))
}

func (s *compositionSuite) TestConfiguredLevelFlowsThroughComposedGrantToRealHealing() {
	for _, tc := range []struct{ level, want int }{{1, 7}, {3, 9}} {
		s.Run(fmt.Sprintf("level_%d", tc.level), func() {
			i := s.ring("ring-a", tc.level, 1)
			s.False(s.available(i, "non-fighter"))
			s.attach(i, events.NewEventBus(), "non-fighter")
			s.True(s.available(i, "non-fighter"))
			roller := &fixedRoller{face: 6}
			out, err := useRing(context.Background(), i, "non-fighter", roller)
			s.Require().NoError(err)
			s.Equal(tc.want, out.Result.Total)
			s.Len(out.Result.Components, 2)
			s.Equal("ring-a", out.Result.Components[0].Source.SourceID)
			s.Equal("ring-a", out.Result.Components[1].Source.SourceID)
			s.Zero(out.Remaining)
			s.Zero(i.Data().Pools[chargePool])
			s.False(s.available(i, "non-fighter"))
			s.Equal(1, roller.calls)
		})
	}
}

func (s *compositionSuite) TestSameDefinitionItemsHaveIndependentBudgetsAndSources() {
	bus := events.NewEventBus()
	a, b := s.ring("ring-a", 3, 1), s.ring("ring-b", 3, 1)
	s.attach(a, bus, "wearer")
	s.attach(b, bus, "wearer")
	aResult, err := useRing(context.Background(), a, "wearer", &fixedRoller{face: 6})
	s.Require().NoError(err)
	s.Zero(a.Data().Pools[chargePool])
	s.Equal(1, b.Data().Pools[chargePool])
	bResult, err := useRing(context.Background(), b, "wearer", &fixedRoller{face: 6})
	s.Require().NoError(err)
	s.Equal("ring-a", aResult.Source.ItemID)
	s.Equal("ring-b", bResult.Source.ItemID)
	s.Equal(aResult.Source.ItemRef, bResult.Source.ItemRef)
}

func (s *compositionSuite) TestRefusedUsesDoNotRollOrSpend() {
	i := s.ring("ring", 1, 1)
	roller := &fixedRoller{face: 6}
	_, err := useRing(context.Background(), i, "wearer", roller)
	s.ErrorIs(err, items.ErrUnavailable)
	s.attach(i, events.NewEventBus(), "wearer")
	_, err = useRing(context.Background(), i, "stranger", roller)
	s.ErrorIs(err, items.ErrUnavailable)
	s.Equal(1, i.Data().Pools[chargePool])
	s.Zero(roller.calls)
	_, err = useRing(context.Background(), i, "wearer", roller)
	s.Require().NoError(err)
	_, err = useRing(context.Background(), i, "wearer", roller)
	s.ErrorIs(err, items.ErrUnavailable)
	s.Equal(1, roller.calls)
	s.Require().NoError(i.Detach(context.Background()))
	s.False(s.available(i, "wearer"))
}

func (s *compositionSuite) TestTransferReloadAndRestRespectTheItemBudget() {
	ctx, bus := context.Background(), events.NewEventBus()
	i := s.ring("ring", 3, 1)
	s.attach(i, bus, "old-holder")
	_, err := useRing(ctx, i, "old-holder", &fixedRoller{face: 6})
	s.Require().NoError(err)
	s.Require().NoError(i.Detach(ctx))
	s.rest(bus, "old-holder", shortRest)
	s.Zero(i.Data().Pools[chargePool])
	encoded, err := json.Marshal(i.Data())
	s.Require().NoError(err)
	var restored items.Data
	s.Require().NoError(json.Unmarshal(encoded, &restored))
	i, err = items.New(ringConfig("ring", 3, &restored))
	s.Require().NoError(err)
	s.attach(i, bus, "new-holder")
	s.False(s.available(i, "new-holder"))
	s.rest(bus, "old-holder", shortRest)
	s.Zero(i.Data().Pools[chargePool])
	s.rest(bus, "new-holder", shortRest)
	s.Equal(1, i.Data().Pools[chargePool])
	_, err = useRing(ctx, i, "new-holder", &fixedRoller{face: 6})
	s.Require().NoError(err)
	s.rest(bus, "new-holder", longRest)
	s.Equal(1, i.Data().Pools[chargePool])
}

func (s *compositionSuite) TestRecoveryPolicyComesFromConfigurationNotAbilityRef() {
	bus := events.NewEventBus()
	normal := s.ring("normal", 1, 0)
	cfg := ringConfig("long-only", 1, &items.Data{
		ID: "long-only", DefinitionRef: ringDefinition(1).Ref, Pools: map[string]int{chargePool: 0},
	})
	pool := cfg.Definition.Pools[chargePool]
	pool.RestoreOn = []items.Trigger{longRest}
	cfg.Definition.Pools[chargePool] = pool
	longOnly, err := items.New(cfg)
	s.Require().NoError(err)
	s.attach(normal, bus, "wearer")
	s.attach(longOnly, bus, "wearer")
	s.rest(bus, "wearer", shortRest)
	s.Equal(1, normal.Data().Pools[chargePool])
	s.Zero(longOnly.Data().Pools[chargePool])
	s.rest(bus, "wearer", longRest)
	s.Equal(1, longOnly.Data().Pools[chargePool])
}

func (s *compositionSuite) TestFailedCalculationReturnsTheReservedCharge() {
	i := s.ring("ring", 1, 1)
	s.attach(i, events.NewEventBus(), "wearer")
	_, err := useRing(context.Background(), i, "wearer", &fixedRoller{err: errors.New("dice unavailable")})
	s.Error(err)
	s.Equal(1, i.Data().Pools[chargePool])
}

func (s *compositionSuite) TestInvalidStateIsRefusedRatherThanClampedOrRefilled() {
	for _, remaining := range []int{-1, 2} {
		state := items.Data{ID: "ring", DefinitionRef: ringDefinition(1).Ref, Pools: map[string]int{chargePool: remaining}}
		_, err := items.New(ringConfig("ring", 1, &state))
		s.ErrorIs(err, items.ErrInvalidConfig)
	}
	for _, state := range []items.Data{
		{ID: "other", DefinitionRef: ringDefinition(1).Ref, Pools: map[string]int{chargePool: 1}},
		{ID: "ring", DefinitionRef: ringDefinition(3).Ref, Pools: map[string]int{chargePool: 1}},
		{ID: "ring", DefinitionRef: ringDefinition(1).Ref},
	} {
		_, err := items.New(ringConfig("ring", 1, &state))
		s.ErrorIs(err, items.ErrInvalidConfig)
	}
	_, err := items.New(ringConfig("ring", 0, nil))
	s.ErrorIs(err, items.ErrInvalidConfig)
}

func (s *compositionSuite) TestInvalidRecoveryIsRefusedBeforeGrantAttachment() {
	cfg := ringConfig("ring", 1, nil)
	pool := cfg.Definition.Pools[chargePool]
	pool.RestoreOn = []items.Trigger{"unsupported"}
	cfg.Definition.Pools[chargePool] = pool
	_, err := items.New(cfg)
	s.ErrorIs(err, items.ErrInvalidConfig)
}

func (s *compositionSuite) TestReturnedViewsAndInputDefinitionsCannotMutateTheItem() {
	cfg := ringConfig("ring", 3, nil)
	i, err := items.New(cfg)
	s.Require().NoError(err)
	cfg.Definition.Grants[windGrant].Parameters[0] = '!'
	delete(cfg.Definition.Pools, chargePool)
	s.attach(i, events.NewEventBus(), "wearer")
	view, err := i.Offers(items.OffersInput{HolderID: "wearer"})
	s.Require().NoError(err)
	s.Require().Len(view.Grants, 1)
	view.Grants[0].Grant.Parameters[0] = '!'
	state := i.Data()
	state.Pools[chargePool] = 0
	out, err := useRing(context.Background(), i, "wearer", &fixedRoller{face: 6})
	s.Require().NoError(err)
	s.Equal(9, out.Result.Total)
	s.Zero(i.Data().Pools[chargePool])
}

func (s *compositionSuite) TestMissingResolverDoesNotConsumeTheGrant() {
	i := s.ring("ring", 1, 1)
	s.attach(i, events.NewEventBus(), "wearer")
	_, err := items.Use[int](context.Background(), items.UseInput[int]{Item: i, HolderID: "wearer", GrantID: windGrant})
	s.Error(err)
	s.Equal(1, i.Data().Pools[chargePool])
}

func (s *compositionSuite) TestGrantPoolReferencesSelectTheMutationOwner() {
	cfg := ringConfig("ring", 1, nil)
	cfg.Definition.Grants["alternate"] = cfg.Definition.Grants[windGrant]
	i, err := items.New(cfg)
	s.Require().NoError(err)
	s.attach(i, events.NewEventBus(), "wearer")
	_, err = useRing(context.Background(), i, "wearer", &fixedRoller{face: 6})
	s.Require().NoError(err)
	view, err := i.Offers(items.OffersInput{HolderID: "wearer"})
	s.Require().NoError(err)
	s.Require().Len(view.Grants, 2)
	s.Equal("alternate", view.Grants[0].ID)
	s.Equal(windGrant, view.Grants[1].ID)
	s.False(view.Grants[0].Affordable)
	s.False(view.Grants[1].Affordable)
	s.Zero(i.Data().Pools[chargePool])
}

func (s *compositionSuite) TestCancellationRefusesBeforeTheConsumerRuns() {
	i := s.ring("ring", 1, 1)
	s.attach(i, events.NewEventBus(), "wearer")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	roller := &fixedRoller{face: 6}
	_, err := useRing(ctx, i, "wearer", roller)
	s.ErrorIs(err, context.Canceled)
	s.Zero(roller.calls)
	s.Equal(1, i.Data().Pools[chargePool])
}

func (s *compositionSuite) TestInvalidDefinitionCannotCreateALiveGrant() {
	cases := map[string]func(*items.Config){
		"missing validator": func(c *items.Config) { c.ValidateGrant = nil },
		"missing id":        func(c *items.Config) { c.ID = "" },
		"missing name":      func(c *items.Config) { c.Definition.Name = "" },
		"zero capacity":     func(c *items.Config) { c.Definition.Pools[chargePool] = items.PoolDefinition{} },
		"unknown pool": func(c *items.Config) {
			g := c.Definition.Grants[windGrant]
			g.Pool = "missing"
			c.Definition.Grants[windGrant] = g
		},
		"zero price": func(c *items.Config) {
			g := c.Definition.Grants[windGrant]
			g.Uses = 0
			c.Definition.Grants[windGrant] = g
		},
		"unsupported ability": func(c *items.Config) {
			g := c.Definition.Grants[windGrant]
			g.Ref.ID = "unsupported"
			c.Definition.Grants[windGrant] = g
		},
		"malformed parameters": func(c *items.Config) {
			g := c.Definition.Grants[windGrant]
			g.Parameters = []byte("!")
			c.Definition.Grants[windGrant] = g
		},
	}
	for name, change := range cases {
		s.Run(name, func() {
			cfg := ringConfig("ring", 1, nil)
			change(&cfg)
			_, err := items.New(cfg)
			s.ErrorIs(err, items.ErrInvalidConfig)
		})
	}
}

func (s *compositionSuite) TestAuthoredDefinitionRoundTripsWithoutRuntimeHooks() {
	encoded, err := json.Marshal(ringDefinition(3))
	s.Require().NoError(err)
	var definition items.Definition
	s.Require().NoError(json.Unmarshal(encoded, &definition))
	cfg := ringConfig("authored-ring", 3, nil)
	cfg.Definition = definition
	i, err := items.New(cfg)
	s.Require().NoError(err)
	s.attach(i, events.NewEventBus(), "wearer")
	out, err := useRing(context.Background(), i, "wearer", &fixedRoller{face: 6})
	s.Require().NoError(err)
	s.Equal(9, out.Result.Total)
	s.Equal("authored-ring", out.Source.ItemID)
}

func TestItemCompositionSeams(t *testing.T) { suite.Run(t, new(compositionSuite)) }
