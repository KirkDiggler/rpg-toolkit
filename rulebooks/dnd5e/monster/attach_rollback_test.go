// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// errRefused is what the stub bus answers with.
var errRefused = errors.New("bus refused the subscription")

// failingBus is a real bus that refuses the nth Subscribe and behaves normally
// otherwise — the only way to reach the second half of a two-step attach and
// make it fail.
type failingBus struct {
	inner  events.EventBus
	failOn int // 1-based; 0 never fails
	calls  int
}

func newFailingBus(failOn int) *failingBus {
	return &failingBus{inner: events.NewEventBus(), failOn: failOn}
}

func (b *failingBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	b.calls++
	if b.calls == b.failOn {
		return "", errRefused
	}

	return b.inner.Subscribe(ctx, topic, handler)
}

func (b *failingBus) Unsubscribe(ctx context.Context, id string) error {
	return b.inner.Unsubscribe(ctx, id)
}

func (b *failingBus) Publish(ctx context.Context, topic events.Topic, event any) error {
	return b.inner.Publish(ctx, topic, event)
}

// MonsterAttachRollbackTestSuite pins the same contract the character side
// keeps: a failed attach is a no-op — same data out, nothing listening, retry
// works.
type MonsterAttachRollbackTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *MonsterAttachRollbackTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// The keeper subscribes damage, then healing. Refusing the second one used to
// leave the first live and the keeper convinced it was applied — a leaked
// subscription that also refused every retry.
func (s *MonsterAttachRollbackTestSuite) TestKeeperRollsBackAPartialSubscribe() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)
	before := s.marshal(m.ToData())

	bus := newFailingBus(2)
	err = m.SheetKeeper().Apply(s.ctx, bus)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errRefused)
	s.Require().Equal(before, s.marshal(m.ToData()))
	s.Require().Empty(m.subscriptionIDs, "the monster claims no subscriptions")
	s.Require().Nil(m.bus, "and holds the bus it held before, which was none")

	// Nothing is listening: the damage handler that did land is gone.
	s.Require().NoError(dnd5eEvents.DamageReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: m.GetID(),
		Amount:   4,
	}))
	s.Require().Equal(9, m.HP())
	s.Require().False(m.IsDirty())

	// And the attach can simply be tried again.
	good := events.NewEventBus()
	s.Require().NoError(m.SheetKeeper().Apply(s.ctx, good))
	s.Require().NoError(dnd5eEvents.DamageReceivedTopic.On(good).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: m.GetID(),
		Amount:   4,
	}))
	s.Require().Equal(5, m.HP(), "the retry attached both handlers")
}

func (s *MonsterAttachRollbackTestSuite) sheet() *Data {
	return &Data{
		ID:           "skel-rollback",
		Name:         "Skeleton",
		HitPoints:    9,
		MaxHitPoints: 13,
		ArmorClass:   13,
		Actions:      []combatActions.Definition{},
	}
}

func (s *MonsterAttachRollbackTestSuite) marshal(d *Data) string {
	raw, err := json.Marshal(d)
	s.Require().NoError(err)

	return string(raw)
}

func TestMonsterAttachRollbackSuite(t *testing.T) {
	suite.Run(t, new(MonsterAttachRollbackTestSuite))
}
