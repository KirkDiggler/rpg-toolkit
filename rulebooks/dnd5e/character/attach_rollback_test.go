// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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
	heldBefore := heldRefs(char.GetConditions())

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
	s.Require().ElementsMatch(heldBefore, heldRefs(char.GetConditions()), "no condition arrived on the sheet")
}

// assertRetryWorks attaches to a working bus and checks the sheet came through
// whole: conditions applied, keeper listening, and the Character rest verb
// still owning its resources.
func (s *AttachRollbackTestSuite) assertRetryWorks(char *Character) {
	bus := events.NewEventBus()

	s.Require().NoError(Attach(s.ctx, char, bus))
	s.Require().False(char.GetResource(resources.RageCharges).IsApplied(),
		"a successful Attach still does not own character resources")

	s.Require().NotEmpty(authored(char))
	for _, cond := range authored(char) {
		s.Require().True(cond.IsApplied(), "the condition attached on the retry")
	}

	s.Require().NoError(dnd5eEvents.HealingReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: char.GetID(),
		Amount:   5,
	}))
	s.Require().Equal(27, char.GetHitPoints(), "the keeper attached on the retry")

	s.Require().NoError(restErr(char.LongRest(s.ctx)))
	s.Require().Equal(3, char.GetResource(resources.RageCharges).Current(),
		"the Character rest verb still owns resources on the retry")
}

// The third of the keeper's five handlers is refused. The two that landed come
// back off, and the sheet is not left reacting to a third of the world.
func (s *AttachRollbackTestSuite) TestKeeperRollsBackAPartialSubscribe() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	before := marshalData(&s.Suite, mustToData(s.T(), char))
	resource := char.GetResource(resources.RageCharges)
	s.Require().False(resource.IsApplied())

	bus := newFailingBus(3)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errRefused)
	s.Require().Equal(before, marshalData(&s.Suite, mustToData(s.T(), char)))
	s.Require().Empty(char.subscriptionIDs, "the sheet claims no subscriptions")
	s.Require().Nil(char.bus, "and holds the bus it held before, which was none")
	s.Require().False(resource.IsApplied(), "failed Attach does not change resource lifecycle")
	s.assertNothingListening(char, bus)
	s.assertRetryWorks(char)
}

// The keeper attaches, and the condition after it is refused — the sixth
// subscription is the first one Raging makes. Character-owned resources do
// not occupy a subscription because their Character rest verbs own recovery.
// Everything that went on comes
// back off, and the condition is pending again with its ref, so the retry can
// attribute it exactly as the first attempt would have.
func (s *AttachRollbackTestSuite) TestAttachRollsBackAFailedCondition() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	before := marshalData(&s.Suite, mustToData(s.T(), char))

	bus := newFailingBus(6)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().Equal(before, marshalData(&s.Suite, mustToData(s.T(), char)))
	s.Require().NotEmpty(char.GetConditions(), "the condition is still on the sheet")
	for _, cond := range char.GetConditions() {
		s.Require().False(cond.IsApplied(), "just not applied")
	}
	s.Require().ElementsMatch(heldRefs(char.GetConditions()), heldRefs(char.pendingEffects),
		"and still waiting to be, ref and all")
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
	loadedRefs := heldRefs(char.GetConditions())
	s.Require().NotEmpty(loadedRefs)
	before := marshalData(&s.Suite, mustToData(s.T(), char))

	bus := newFailingBus(firstSubscriptionOfTheSecondCondition)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().Equal(before, marshalData(&s.Suite, mustToData(s.T(), char)))
	s.Require().ElementsMatch(loadedRefs, heldRefs(char.pendingEffects), "both are waiting again")
	for _, cond := range char.GetConditions() {
		s.Require().False(cond.IsApplied(), "including the one that had applied")
	}
	s.assertNothingListening(char, bus)

	good := events.NewEventBus()
	s.Require().NoError(Attach(s.ctx, char, good))
	s.Require().ElementsMatch(loadedRefs, heldRefs(authored(char)))
	for _, cond := range authored(char) {
		s.Require().True(cond.IsApplied(), "the retry attached both")
	}
}

// The lenient path keeps the behaviour its callers have: a condition that will
// not apply is dropped and the load succeeds. Rolling the whole attach back
// here would turn a survivable load into a failure for twenty callers.
func (s *AttachRollbackTestSuite) TestLenientAttachStillDropsAndContinues() {
	bus := newFailingBus(6)

	char, err := LoadFromData(s.ctx, fullSheet(&s.Suite), bus)

	s.Require().NoError(err)
	s.Require().Empty(authored(char), "the condition that would not apply was dropped")
	s.Require().NotEmpty(char.subscriptionIDs, "the sheet is attached")
}

// clericFeatureSheet is a Light cleric's sheet carrying two features that go on
// the bus: Warding Flare, which subscribes to the attack chain, first, and
// Wrath of the Storm after it. The count is the shared pool on the sheet, as
// every pool is.
func (s *AttachRollbackTestSuite) clericFeatureSheet() *Data {
	wardingFlare, err := json.Marshal(features.WardingFlareData{
		Ref: refs.Features.WardingFlare(), ID: "rollback-warding-flare", Name: "Warding Flare",
		CharacterID: "rollback-cleric",
	})
	s.Require().NoError(err)
	wrath, err := json.Marshal(features.WrathOfTheStormData{
		Ref: refs.Features.WrathOfTheStorm(), ID: "rollback-wrath", Name: "Wrath of the Storm",
		CharacterID: "rollback-cleric",
	})
	s.Require().NoError(err)

	return &Data{RaceID: "human", Level: 1,
		ID:      "rollback-cleric",
		ClassID: classes.Cleric,
		// Warding Flare first: it is the first feature that subscribes, and the
		// pool it reports is on the sheet.
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.WardingFlare:    {Current: 0, Maximum: 1, ResetType: coreResources.ResetLongRest},
			resources.WrathOfTheStorm: {Current: 0, Maximum: 1, ResetType: coreResources.ResetLongRest},
		},
		Features: []json.RawMessage{wardingFlare, wrath},
	}
}

// Features are applied after the keeper in persisted order. If a later feature
// cannot subscribe, the keeper comes back off; the
// strict sheet remains byte-identical and can be attached again.
func (s *AttachRollbackTestSuite) TestAttachRollsBackAFailedFeature() {
	// Counted, not guessed: the keeper's hooks land before the first feature's,
	// so the feature's first subscription is the one after them.
	counter := newFailingBus(0)
	onlyFlare := s.clericFeatureSheet()
	onlyFlare.Features = onlyFlare.Features[:1]
	warmup, err := Load(s.ctx, onlyFlare)
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, warmup, counter))
	firstFeatureSubscription := counter.calls
	s.Require().Equal(6, firstFeatureSubscription, "five keeper hooks, then Warding Flare")

	char, err := Load(s.ctx, s.clericFeatureSheet())
	s.Require().NoError(err)
	before := marshalData(&s.Suite, mustToData(s.T(), char))

	bus := newFailingBus(firstFeatureSubscription)
	err = Attach(s.ctx, char, bus)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errRefused)
	s.Require().Equal(before, marshalData(&s.Suite, mustToData(s.T(), char)))
	s.Require().Empty(char.subscriptionIDs)
	s.Require().Nil(char.bus)

	s.Require().NoError(dnd5eEvents.RestTopic.On(bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType: coreResources.ResetLongRest, CharacterID: char.GetID(),
	}))
	s.Zero(mustToData(s.T(), char).Resources[resources.WardingFlare].Current,
		"nothing refills the shared pool after rollback")

	good := events.NewEventBus()
	s.Require().NoError(Attach(s.ctx, char, good))
	_, err = char.LongRest(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, mustToData(s.T(), char).Resources[resources.WardingFlare].Current,
		"the keeper refills the shared Warding Flare pool")
}

// A lenient load drops only the attachable feature whose Apply fails and keeps
// attaching later features, matching its existing per-blob drop policy.
func (s *AttachRollbackTestSuite) TestLenientAttachDropsFailedFeatureAndContinues() {
	bus := newFailingBus(6) // Warding Flare, the first feature to subscribe after the five keeper hooks

	char, err := LoadFromData(s.ctx, s.clericFeatureSheet(), bus)

	s.Require().NoError(err)
	var featureRefs []string
	for _, feature := range char.GetFeatures() {
		featureRefs = append(featureRefs, feature.Ref().String())
	}
	s.Require().NotContains(featureRefs, refs.Features.WardingFlare().String(), "the failed feature is dropped")
	s.Require().Contains(featureRefs, refs.Features.WrathOfTheStorm().String(), "the later feature still loads")
	_, err = char.LongRest(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, mustToData(s.T(), char).Resources[resources.WardingFlare].Current,
		"the sheet is still attached and refills the shared pool")
}

func TestAttachRollbackSuite(t *testing.T) {
	suite.Run(t, new(AttachRollbackTestSuite))
}
