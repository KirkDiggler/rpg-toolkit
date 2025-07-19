package selectables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SimpleEventTestSuite tests basic event publishing functionality
type SimpleEventTestSuite struct {
	suite.Suite
	eventBus       events.EventBus
	ctx            SelectionContext
	table          SelectionTable[string]
	capturedEvents []events.Event
}

// SetupTest initializes test environment before each test
func (s *SimpleEventTestSuite) SetupTest() {
	s.eventBus = events.NewBus()
	testRoller := NewTestRoller([]int{50})
	s.ctx = NewSelectionContextWithRoller(testRoller)

	config := BasicTableConfig{
		ID:       "test_table",
		EventBus: s.eventBus,
		Configuration: TableConfiguration{
			EnableEvents: true,
		},
	}
	s.table = NewBasicTable[string](config)
	s.capturedEvents = make([]events.Event, 0)

	// Simple event capture
	s.eventBus.SubscribeFunc(EventSelectionCompleted, 0, s.captureEvent)
	s.eventBus.SubscribeFunc(EventSelectionFailed, 0, s.captureEvent)
	s.eventBus.SubscribeFunc(EventItemAdded, 0, s.captureEvent)
}

func (s *SimpleEventTestSuite) captureEvent(_ context.Context, event events.Event) error {
	s.capturedEvents = append(s.capturedEvents, event)
	return nil
}

// TestSimpleEventSuite runs the simple event test suite
func TestSimpleEventSuite(t *testing.T) {
	suite.Run(t, new(SimpleEventTestSuite))
}

// TestBasicEventPublishing tests that events are published correctly
func (s *SimpleEventTestSuite) TestBasicEventPublishing() {
	s.Run("publishes events when items are added", func() {
		s.table.Add("sword", 10)

		// Should have captured an item added event
		s.Assert().Len(s.capturedEvents, 1)
		event := s.capturedEvents[0]
		s.Assert().Equal(EventItemAdded, event.Type())
	})

	s.Run("publishes events when selection succeeds", func() {
		// Reset events
		s.capturedEvents = make([]events.Event, 0)

		s.table.Add("item", 100)
		_, err := s.table.Select(s.ctx)
		s.Require().NoError(err)

		// Should have at least one selection completed event
		found := false
		for _, event := range s.capturedEvents {
			if event.Type() == EventSelectionCompleted {
				found = true
				break
			}
		}
		s.Assert().True(found, "Should have selection completed event")
	})

	s.Run("publishes events when selection fails", func() {
		// Reset events and use empty table
		s.capturedEvents = make([]events.Event, 0)
		emptyTable := NewBasicTable[string](BasicTableConfig{
			ID:       "empty_table",
			EventBus: s.eventBus,
			Configuration: TableConfiguration{
				EnableEvents: true,
			},
		})

		_, err := emptyTable.Select(s.ctx)
		s.Assert().Error(err)

		// Should have selection failed event
		found := false
		for _, event := range s.capturedEvents {
			if event.Type() == EventSelectionFailed {
				found = true
				break
			}
		}
		s.Assert().True(found, "Should have selection failed event")
	})
}

// TestEventConfiguration tests that event configuration is respected
func (s *SimpleEventTestSuite) TestEventConfiguration() {
	s.Run("respects event configuration", func() {
		// Create table with events disabled
		config := BasicTableConfig{
			ID:       "no_events_table",
			EventBus: s.eventBus,
			Configuration: TableConfiguration{
				EnableEvents: false,
			},
		}

		noEventsTable := NewBasicTable[string](config)

		// Reset events
		s.capturedEvents = make([]events.Event, 0)

		noEventsTable.Add("item", 10)
		_, _ = noEventsTable.Select(s.ctx)

		// Should have no events
		s.Assert().Empty(s.capturedEvents, "Should have no events when disabled")
	})
}
