// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// errRefused is what the stub bus answers with. Any error would do; a named one
// keeps the failure legible when a test reports it.
var errRefused = errors.New("bus refused the subscription")

// failingBus is a real bus that refuses the nth Subscribe and behaves normally
// otherwise. Counting rather than matching on topic is deliberate: it lets one
// stub reach every stage of an attach — a keeper handler, a resource, an
// effect — by number, including the stages a test would otherwise have no way
// to make fail.
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

// AttachRollbackTestSuite holds one contract to three sites: a failed attach is
// a no-op. The sheet's data is untouched, the bus carries nothing the failed
// call put there, and the attach can simply be tried again.
//
// The three assertions are the same every time — same data out, nothing
// listening, retry works — because a rollback that satisfies two of them is
// still a bug, just a quieter one.
type AttachRollbackTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *AttachRollbackTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// assertNothingListening publishes on the bus the failed attach was given and
// checks that none of it lands: no healing, no condition.
func (s *AttachRollbackTestSuite) assertNothingListening(char *Character, bus events.EventBus) {
	hitPoints := char.GetHitPoints()
	conditions := len(char.GetConditions())

	s.Require().NoError(dnd5eEvents.HealingReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: char.GetID(),
		Amount:   5,
	}))
	s.Require().Equal(hitPoints, char.GetHitPoints(), "the sheet is not listening for healing")
	s.Require().False(char.IsDirty(), "and nothing has changed on it")

	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    char,
		Condition: &keptCondition{},
	}))
	s.Require().Len(char.GetConditions(), conditions, "no condition arrived on the sheet")
}

// assertRetryWorks attaches to a working bus and checks the sheet came through
// whole: conditions applied, keeper listening, resources hearing a rest.
func (s *AttachRollbackTestSuite) assertRetryWorks(char *Character) {
	bus := events.NewEventBus()

	s.Require().NoError(Attach(s.ctx, char, bus))

	s.Require().Len(char.GetConditions(), 1)
	s.Require().True(char.GetConditions()[0].IsApplied(), "the condition attached on the retry")

	s.Require().NoError(dnd5eEvents.HealingReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: char.GetID(),
		Amount:   5,
	}))
	s.Require().Equal(27, char.GetHitPoints(), "the keeper attached on the retry")

	s.Require().NoError(dnd5eEvents.RestTopic.On(bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    "long_rest",
		CharacterID: char.GetID(),
	}))
	s.Require().Equal(3, char.GetResource(resources.RageCharges).Current(),
		"the resources attached on the retry")
}

// The third of the keeper's five handlers is refused. The two that landed come
// back off, and the sheet is not left reacting to a third of the world.
func (s *AttachRollbackTestSuite) TestKeeperRollsBackAPartialSubscribe() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	before := marshalData(&s.Suite, char.ToData())

	bus := newFailingBus(3)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errRefused)
	s.Require().Equal(before, marshalData(&s.Suite, char.ToData()))
	s.Require().Empty(char.subscriptionIDs, "the sheet claims no subscriptions")
	s.Require().Nil(char.bus, "and holds the bus it held before, which was none")
	s.assertNothingListening(char, bus)
	s.assertRetryWorks(char)
}

// The keeper's handlers all land and its one resource is refused — the sixth
// subscription. The handlers come back off with it: a sheet listening to the
// world but not recovering on a rest is a state nothing about the sheet
// records.
func (s *AttachRollbackTestSuite) TestKeeperRollsBackAFailedResource() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	before := marshalData(&s.Suite, char.ToData())

	bus := newFailingBus(6)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().Equal(before, marshalData(&s.Suite, char.ToData()))
	s.Require().Empty(char.subscriptionIDs)
	s.Require().Nil(char.bus)
	s.assertNothingListening(char, bus)
	s.assertRetryWorks(char)
}

// The keeper attaches, and the condition after it is refused — the seventh
// subscription is the first one Raging makes. Everything that went on comes
// back off, and the condition is pending again with its ref, so the retry can
// attribute it exactly as the first attempt would have.
func (s *AttachRollbackTestSuite) TestAttachRollsBackAFailedCondition() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	before := marshalData(&s.Suite, char.ToData())

	bus := newFailingBus(7)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().Equal(before, marshalData(&s.Suite, char.ToData()))
	s.Require().Len(char.GetConditions(), 1, "the condition is still on the sheet")
	s.Require().False(char.GetConditions()[0].IsApplied(), "just not applied")
	s.Require().Len(char.pendingEffects, 1, "and still waiting to be, ref and all")
	s.Require().Empty(char.subscriptionIDs)
	s.Require().Nil(char.bus)
	s.assertNothingListening(char, bus)
	s.assertRetryWorks(char)
}

// A condition that fails after another has already applied takes that one off
// with it. Half a sheet's conditions attached is a character playing by rules
// nobody wrote — and unlike the other failures here, this one leaves live
// subscriptions from an effect the caller was told did not attach.
func (s *AttachRollbackTestSuite) TestASecondConditionFailingRemovesTheFirst() {
	// Counted, not guessed: the same sheet carrying only its first condition
	// tells us how many subscriptions land before a second one would start, so
	// this stays pointed at the right moment if Raging's internals change.
	counter := newFailingBus(0)
	warmup, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, warmup, counter))
	firstSubscriptionOfTheSecondCondition := counter.calls + 1

	data := fullSheet(&s.Suite)
	data.Conditions = append(data.Conditions, brutalCriticalBlob(&s.Suite))

	char, err := Load(s.ctx, data)
	s.Require().NoError(err)
	s.Require().Len(char.GetConditions(), 2)
	before := marshalData(&s.Suite, char.ToData())

	bus := newFailingBus(firstSubscriptionOfTheSecondCondition)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().Equal(before, marshalData(&s.Suite, char.ToData()))
	s.Require().Len(char.pendingEffects, 2, "both are waiting again")
	for _, cond := range char.GetConditions() {
		s.Require().False(cond.IsApplied(), "including the one that had applied")
	}
	s.assertNothingListening(char, bus)

	good := events.NewEventBus()
	s.Require().NoError(Attach(s.ctx, char, good))
	s.Require().Len(char.GetConditions(), 2)
	for _, cond := range char.GetConditions() {
		s.Require().True(cond.IsApplied(), "the retry attached both")
	}
}

// The lenient path keeps the behaviour its callers have: a condition that will
// not apply is dropped and the load succeeds. Rolling the whole attach back
// here would turn a survivable load into a failure for twenty callers.
func (s *AttachRollbackTestSuite) TestLenientAttachStillDropsAndContinues() {
	bus := newFailingBus(7)

	char, err := LoadFromData(s.ctx, fullSheet(&s.Suite), bus)

	s.Require().NoError(err)
	s.Require().Empty(char.GetConditions(), "the condition that would not apply was dropped")
	s.Require().NotEmpty(char.subscriptionIDs, "the sheet is attached")
}

func TestAttachRollbackSuite(t *testing.T) {
	suite.Run(t, new(AttachRollbackTestSuite))
}
