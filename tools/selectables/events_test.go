package selectables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// EventIntegrationTestSuite provides comprehensive testing of event integration
type EventIntegrationTestSuite struct {
	suite.Suite
	eventBus       events.EventBus
	ctx            SelectionContext
	table          SelectionTable[string]
	capturedEvents []events.Event
}

// SetupTest initializes test environment before each test
func (s *EventIntegrationTestSuite) SetupTest() {
	s.eventBus = events.NewBus()
	testRoller := NewTestRoller([]int{50, 25, 75})
	s.ctx = NewSelectionContextWithRoller(testRoller)

	config := BasicTableConfig{
		ID:       "event_test_table",
		EventBus: s.eventBus,
		Configuration: TableConfiguration{
			EnableEvents: true,
		},
	}
	s.table = NewBasicTable[string](config)
	s.capturedEvents = make([]events.Event, 0)

	// Subscribe to all selectables events
	s.eventBus.SubscribeFunc(EventSelectionTableCreated, 0, s.captureEvent)
	s.eventBus.SubscribeFunc(EventItemAdded, 0, s.captureEvent)
	s.eventBus.SubscribeFunc(EventSelectionCompleted, 0, s.captureEvent)
	s.eventBus.SubscribeFunc(EventSelectionFailed, 0, s.captureEvent)
}

// SetupSubTest resets state before each subtest
func (s *EventIntegrationTestSuite) SetupSubTest() {
	s.capturedEvents = make([]events.Event, 0)
}

func (s *EventIntegrationTestSuite) captureEvent(_ context.Context, event events.Event) error {
	s.capturedEvents = append(s.capturedEvents, event)
	return nil
}

// TestEventIntegrationSuite runs the comprehensive event integration test suite
func TestEventIntegrationSuite(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}

// TestTableCreationEvents tests events published during table creation
func (s *EventIntegrationTestSuite) TestTableCreationEvents() {
	s.Run("publishes table creation event", func() {
		// Create a new table to capture its creation event
		s.capturedEvents = make([]events.Event, 0)

		config := BasicTableConfig{
			ID:       "new_test_table",
			EventBus: s.eventBus,
			Configuration: TableConfiguration{
				EnableEvents: true,
			},
		}
		_ = NewBasicTable[string](config)

		creationEvents := s.getEventsByType(EventSelectionTableCreated)
		s.Assert().Len(creationEvents, 1, "Should have one table creation event")

		event := creationEvents[0]
		s.Assert().Equal(EventSelectionTableCreated, event.Type())

		// Verify source entity
		source := event.Source()
		s.Assert().NotNil(source)
		s.Assert().Equal("new_test_table", source.GetID())
		s.Assert().Equal("basic", source.GetType())
	})
}

// TestItemEvents tests events published during item operations
func (s *EventIntegrationTestSuite) TestItemEvents() {
	s.Run("publishes item added events", func() {
		s.table.Add("sword", 10)
		s.table.Add("shield", 20)

		itemEvents := s.getEventsByType(EventItemAdded)
		s.Assert().Len(itemEvents, 2, "Should have two item added events")

		// Verify first event
		event1 := itemEvents[0]
		s.Assert().Equal(EventItemAdded, event1.Type())

		// Verify source entity
		source := event1.Source()
		s.Assert().NotNil(source)
		s.Assert().Equal("event_test_table", source.GetID())
	})

	s.Run("publishes weight change events", func() {
		s.table.Add("sword", 10)
		// Reset captured events to isolate weight change
		s.capturedEvents = make([]events.Event, 0)

		s.table.Add("sword", 20) // Update weight

		itemEvents := s.getEventsByType(EventItemAdded)
		s.Assert().Len(itemEvents, 1, "Should have one weight change event")

		event := itemEvents[0]
		s.Assert().Equal(EventItemAdded, event.Type())
	})
}

// TestSelectionCompletedEvents tests events for successful selections
func (s *EventIntegrationTestSuite) TestSelectionCompletedEvents() {
	s.Run("publishes single selection completed events", func() {
		s.table.Add("sword", 100)
		s.capturedEvents = make([]events.Event, 0) // Reset after setup

		_, err := s.table.Select(s.ctx)
		s.Require().NoError(err)

		completedEvents := s.getEventsByType(EventSelectionCompleted)
		s.Assert().Len(completedEvents, 1, "Should have one selection completed event")

		event := completedEvents[0]
		s.Assert().Equal(EventSelectionCompleted, event.Type())

		// Verify source entity
		source := event.Source()
		s.Assert().NotNil(source)
		s.Assert().Equal("event_test_table", source.GetID())
	})

	s.Run("publishes multiple selection completed events", func() {
		s.table.Add("sword", 50).Add("shield", 50)
		s.capturedEvents = make([]events.Event, 0)

		_, err := s.table.SelectMany(s.ctx, 3)
		s.Require().NoError(err)

		completedEvents := s.getEventsByType(EventSelectionCompleted)
		// SelectMany calls Select 3 times (3 events) + 1 overall event = 4 total
		s.Assert().Len(completedEvents, 4, "Should have events for each Select call plus overall SelectMany")

		// Verify all events are selection completed events
		for _, event := range completedEvents {
			s.Assert().Equal(EventSelectionCompleted, event.Type())
		}
	})

	s.Run("publishes unique selection completed events", func() {
		s.table.Add("sword", 25).Add("shield", 25).Add("potion", 25).Add("scroll", 25)
		s.capturedEvents = make([]events.Event, 0)

		_, err := s.table.SelectUnique(s.ctx, 2)
		s.Require().NoError(err)

		completedEvents := s.getEventsByType(EventSelectionCompleted)
		s.Assert().Len(completedEvents, 1, "Should have one selection completed event")

		event := completedEvents[0]
		s.Assert().Equal(EventSelectionCompleted, event.Type())
	})

	s.Run("publishes variable selection completed events", func() {
		s.table.Add("sword", 100)
		s.capturedEvents = make([]events.Event, 0)

		// Our test roller returns 50 (clamped to 4 for 1d4), so 1d4 will return 4
		_, err := s.table.SelectVariable(s.ctx, "1d4")
		s.Require().NoError(err)

		completedEvents := s.getEventsByType(EventSelectionCompleted)
		// SelectVariable delegates to SelectMany(4), which calls Select 4 times + 1 overall = 5 events
		s.Assert().Len(completedEvents, 5, "Should have events for each Select call plus overall SelectMany")

		// Verify all events are selection completed events
		for _, event := range completedEvents {
			s.Assert().Equal(EventSelectionCompleted, event.Type())
		}
	})
}

// TestSelectionFailedEvents tests events for failed selections
func (s *EventIntegrationTestSuite) TestSelectionFailedEvents() {
	s.Run("publishes empty table selection failed events", func() {
		// Empty table
		s.capturedEvents = make([]events.Event, 0)

		_, err := s.table.Select(s.ctx)
		s.Assert().Error(err)

		failedEvents := s.getEventsByType(EventSelectionFailed)
		s.Assert().Len(failedEvents, 1, "Should have one selection failed event")

		event := failedEvents[0]
		s.Assert().Equal(EventSelectionFailed, event.Type())

		// Verify source entity
		source := event.Source()
		s.Assert().NotNil(source)
		s.Assert().Equal("event_test_table", source.GetID())
	})

	s.Run("publishes invalid count selection failed events", func() {
		s.table.Add("item", 10)
		s.capturedEvents = make([]events.Event, 0)

		_, err := s.table.SelectMany(s.ctx, 0) // Invalid count
		s.Assert().Error(err)

		failedEvents := s.getEventsByType(EventSelectionFailed)
		s.Assert().Len(failedEvents, 1, "Should have one selection failed event")

		event := failedEvents[0]
		s.Assert().Equal(EventSelectionFailed, event.Type())
	})

	s.Run("publishes insufficient items selection failed events", func() {
		s.table.Add("item1", 10).Add("item2", 20)
		s.capturedEvents = make([]events.Event, 0)

		_, err := s.table.SelectUnique(s.ctx, 5) // More than available
		s.Assert().Error(err)

		failedEvents := s.getEventsByType(EventSelectionFailed)
		s.Assert().Len(failedEvents, 1, "Should have one selection failed event")

		event := failedEvents[0]
		s.Assert().Equal(EventSelectionFailed, event.Type())
	})

	s.Run("publishes invalid dice expression failed events", func() {
		s.table.Add("item", 10)
		s.capturedEvents = make([]events.Event, 0)

		_, err := s.table.SelectVariable(s.ctx, "invalid_expression")
		s.Assert().Error(err)

		failedEvents := s.getEventsByType(EventSelectionFailed)
		s.Assert().Len(failedEvents, 1, "Should have one selection failed event")

		event := failedEvents[0]
		s.Assert().Equal(EventSelectionFailed, event.Type())
	})
}

// TestEventConfiguration tests that event configuration is respected
func (s *EventIntegrationTestSuite) TestEventConfiguration() {
	s.Run("respects disabled events configuration", func() {
		// Create table with events disabled
		disabledConfig := BasicTableConfig{
			ID:       "disabled_events_table",
			EventBus: s.eventBus,
			Configuration: TableConfiguration{
				EnableEvents: false,
			},
		}

		disabledTable := NewBasicTable[string](disabledConfig)
		s.capturedEvents = make([]events.Event, 0)

		disabledTable.Add("item", 10)
		_, _ = disabledTable.Select(s.ctx)

		// Should have no events
		s.Assert().Empty(s.capturedEvents, "Should have no events when disabled")
	})

	s.Run("respects enabled events configuration", func() {
		// Use existing table with events enabled
		s.capturedEvents = make([]events.Event, 0)

		s.table.Add("item", 10)
		_, _ = s.table.Select(s.ctx)

		// Should have events
		s.Assert().NotEmpty(s.capturedEvents, "Should have events when enabled")
	})
}

// Helper methods

// getEventsByType filters captured events by type
func (s *EventIntegrationTestSuite) getEventsByType(eventType string) []events.Event {
	var filtered []events.Event
	for _, event := range s.capturedEvents {
		if event.Type() == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
