// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

// Test events
type TestNotificationEvent struct {
	ID      string
	Message string
	Value   int
}

type TestActionEvent struct {
	ActorID  string
	TargetID string
	Action   string
}

// Test topic constants
const (
	TopicNotification events.Topic = "test.notification"
	TopicAction       events.Topic = "test.action"
)

// Test typed topics
var (
	NotificationTopic = events.DefineTypedTopic[TestNotificationEvent](TopicNotification)
	ActionTopic       = events.DefineTypedTopic[TestActionEvent](TopicAction)
)

// TypedTopicTestSuite tests pure notification topics
type TypedTopicTestSuite struct {
	suite.Suite
	bus   events.EventBus
	ctx   context.Context
	topic events.TypedTopic[TestNotificationEvent]
}

func (s *TypedTopicTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.topic = NotificationTopic.On(s.bus)
}

func (s *TypedTopicTestSuite) TestSubscribeAndPublish() {
	// Track received events
	var received []TestNotificationEvent

	// Subscribe
	id, err := s.topic.Subscribe(s.ctx, func(_ context.Context, e TestNotificationEvent) error {
		received = append(received, e)
		return nil
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(id)

	// Publish event
	event := TestNotificationEvent{
		ID:      "test-1",
		Message: "Hello World",
		Value:   42,
	}
	err = s.topic.Publish(s.ctx, event)
	s.Require().NoError(err)

	// Verify received
	s.Require().Len(received, 1)
	s.Equal(event, received[0])
}

func (s *TypedTopicTestSuite) TestMultipleSubscribers() {
	// Track calls for each subscriber
	var calls1, calls2, calls3 int

	// Subscribe multiple handlers
	_, err := s.topic.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		calls1++
		return nil
	})
	s.Require().NoError(err)

	_, err = s.topic.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		calls2++
		return nil
	})
	s.Require().NoError(err)

	_, err = s.topic.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		calls3++
		return nil
	})
	s.Require().NoError(err)

	// Publish once
	err = s.topic.Publish(s.ctx, TestNotificationEvent{ID: "test"})
	s.Require().NoError(err)

	// All subscribers should be called
	s.Equal(1, calls1)
	s.Equal(1, calls2)
	s.Equal(1, calls3)
}

func (s *TypedTopicTestSuite) TestUnsubscribe() {
	var callCount int

	// Subscribe
	id, err := s.topic.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		callCount++
		return nil
	})
	s.Require().NoError(err)

	// First publish - should receive
	err = s.topic.Publish(s.ctx, TestNotificationEvent{ID: "1"})
	s.Require().NoError(err)
	s.Equal(1, callCount)

	// Unsubscribe
	err = s.topic.Unsubscribe(s.ctx, id)
	s.Require().NoError(err)

	// Second publish - should NOT receive
	err = s.topic.Publish(s.ctx, TestNotificationEvent{ID: "2"})
	s.Require().NoError(err)
	s.Equal(1, callCount) // Still 1, not incremented
}

func (s *TypedTopicTestSuite) TestDifferentTopicsAreIsolated() {
	// Set up notification topic
	var notificationReceived bool
	_, err := s.topic.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		notificationReceived = true
		return nil
	})
	s.Require().NoError(err)

	// Set up action topic
	actionTopic := ActionTopic.On(s.bus)
	var actionReceived bool
	_, err = actionTopic.Subscribe(s.ctx, func(_ context.Context, _ TestActionEvent) error {
		actionReceived = true
		return nil
	})
	s.Require().NoError(err)

	// Publish to notification topic
	err = s.topic.Publish(s.ctx, TestNotificationEvent{ID: "notify"})
	s.Require().NoError(err)

	// Only notification should receive
	s.True(notificationReceived)
	s.False(actionReceived)

	// Reset
	notificationReceived = false
	actionReceived = false

	// Publish to action topic
	err = actionTopic.Publish(s.ctx, TestActionEvent{ActorID: "actor"})
	s.Require().NoError(err)

	// Only action should receive
	s.False(notificationReceived)
	s.True(actionReceived)
}

func (s *TypedTopicTestSuite) TestSameTopicDifferentInstances() {
	// Create two instances of the same topic
	topic1 := NotificationTopic.On(s.bus)
	topic2 := NotificationTopic.On(s.bus)

	var received1, received2 bool

	// Subscribe on first instance
	_, err := topic1.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		received1 = true
		return nil
	})
	s.Require().NoError(err)

	// Subscribe on second instance
	_, err = topic2.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		received2 = true
		return nil
	})
	s.Require().NoError(err)

	// Publish from first instance
	err = topic1.Publish(s.ctx, TestNotificationEvent{ID: "test"})
	s.Require().NoError(err)

	// Both should receive (same bus, same topic ID)
	s.True(received1)
	s.True(received2)
}

func (s *TypedTopicTestSuite) TestHandlerError() {
	testError := errors.New("handler error")

	// Subscribe handler that returns error
	_, err := s.topic.Subscribe(s.ctx, func(_ context.Context, _ TestNotificationEvent) error {
		return testError
	})
	s.Require().NoError(err)

	// Publish should propagate the error
	err = s.topic.Publish(s.ctx, TestNotificationEvent{ID: "test"})
	s.Error(err)
	s.Equal(testError, err)
}

func TestTypedTopicSuite(t *testing.T) {
	suite.Run(t, new(TypedTopicTestSuite))
}
