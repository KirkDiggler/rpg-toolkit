// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// countingBus counts live subscriptions.
type countingBus struct {
	events.EventBus
	live int
}

func (b *countingBus) Subscribe(ctx context.Context, topic events.Topic, h any) (string, error) {
	b.live++
	return b.EventBus.Subscribe(ctx, topic, h)
}

func (b *countingBus) Unsubscribe(ctx context.Context, id string) error {
	b.live--
	return b.EventBus.Unsubscribe(ctx, id)
}

type lifecycleSuite struct {
	suite.Suite
	ctx context.Context
	p   *machine.Program
}

func (s *lifecycleSuite) SetupTest() {
	s.ctx = context.Background()
	p, err := rageRecipe().Compile(rageVocabulary())
	s.Require().NoError(err)
	s.p = p
}

// Rage names six beats: two trackers' and four transition beats'.
func (s *lifecycleSuite) TestApplySubscribesOncePerTrackerAndBeat() {
	bus := &countingBus{EventBus: events.NewEventBus()}
	m := machine.New(s.p, holder, nil, nil, "")
	s.False(m.IsApplied())
	s.Require().NoError(m.Apply(s.ctx, bus))
	s.True(m.IsApplied())
	s.Equal(6, bus.live)

	s.Require().ErrorIs(m.Apply(s.ctx, bus), machine.ErrAlreadyApplied)

	s.Require().NoError(m.Remove(s.ctx, bus))
	s.False(m.IsApplied())
	s.Equal(0, bus.live)
	s.Require().NoError(m.Remove(s.ctx, bus), "removing twice is a no-op")
}

func (s *lifecycleSuite) TestAFailedApplyRollsBack() {
	v := rageVocabulary()
	failing := v.Beats[beatRest]
	failing.Subscribe = func(context.Context, events.EventBus, func(context.Context, any) error) (string, error) {
		return "", errors.New("bus refused")
	}
	v.Beats[beatRest] = failing
	p, err := rageRecipe().Compile(v)
	s.Require().NoError(err)

	bus := &countingBus{EventBus: events.NewEventBus()}
	m := machine.New(p, holder, nil, nil, "")
	err = m.Apply(s.ctx, bus)
	s.Require().Error(err)
	s.Contains(err.Error(), "bus refused")
	s.False(m.IsApplied())
	s.Equal(0, bus.live, "every subscription made before the failure is undone")
}

func (s *lifecycleSuite) TestApplyRefusesAnUnknownOrTerminalState() {
	for _, state := range []string{"sulking", stEnded} {
		err := machine.New(s.p, holder, nil, nil, state).Apply(s.ctx, events.NewEventBus())
		s.Require().ErrorIs(err, machine.ErrUnknownState, state)
	}
}

func (s *lifecycleSuite) TestToBlobIsTheHeaderStateAndBag() {
	source := rageSource()
	bag, err := s.p.NewBag(map[string]any{machine.KeyRoundActivated: 2})
	s.Require().NoError(err)
	m := machine.New(s.p, holder, source, bag, stRaging)

	b := m.ToBlob()
	s.Equal(ragingRef, b.Ref)
	s.Equal(holder, b.MemberID)
	s.Equal(source, b.Source)
	s.Equal(stRaging, b.State)
	s.Equal(2, b.Bag[machine.KeyRoundActivated])

	data, err := b.Marshal()
	s.Require().NoError(err)
	var back machine.Blob
	s.Require().NoError(json.Unmarshal(data, &back))
	s.Equal(b, back, "a blob reloads to itself")

	b.Bag[keyWasHit] = true
	s.Equal(false, m.Bag()[keyWasHit], "the blob is a copy")
}

func TestMachineLifecycle(t *testing.T) {
	suite.Run(t, new(lifecycleSuite))
}
