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
	eventBus                   events.EventBus
	ctx                        SelectionContext
	table                      SelectionTable[string]
	capturedItemAdded          []ItemAddedEvent
	capturedSelectionCompleted []SelectionCompletedEvent
	capturedSelectionFailed    []SelectionFailedEvent
}

// SetupTest initializes test environment before each test
func (s *SimpleEventTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	testRoller := NewTestRoller([]int{50})
	s.ctx = NewSelectionContextWithRoller(testRoller)

	config := BasicTableConfig{
		ID: "test_table",
		Configuration: TableConfiguration{
			EnableEvents: true,
		},
	}
	s.table = NewBasicTable[string](config)

	// Connect the table to the event bus
	if basicTable, ok := s.table.(*BasicTable[string]); ok {
		basicTable.ConnectToEventBus(s.eventBus)
	}

	// Initialize capture slices
	s.capturedItemAdded = make([]ItemAddedEvent, 0)
	s.capturedSelectionCompleted = make([]SelectionCompletedEvent, 0)
	s.capturedSelectionFailed = make([]SelectionFailedEvent, 0)

	// Subscribe to typed events
	_, _ = ItemAddedTopic.On(s.eventBus).Subscribe(context.Background(), s.captureItemAddedEvent)
	_, _ = SelectionCompletedTopic.On(s.eventBus).Subscribe(context.Background(), s.captureSelectionCompletedEvent)
	_, _ = SelectionFailedTopic.On(s.eventBus).Subscribe(context.Background(), s.captureSelectionFailedEvent)
}

func (s *SimpleEventTestSuite) captureItemAddedEvent(_ context.Context, event ItemAddedEvent) error {
	s.capturedItemAdded = append(s.capturedItemAdded, event)
	return nil
}

func (s *SimpleEventTestSuite) captureSelectionCompletedEvent(_ context.Context, event SelectionCompletedEvent) error {
	s.capturedSelectionCompleted = append(s.capturedSelectionCompleted, event)
	return nil
}

func (s *SimpleEventTestSuite) captureSelectionFailedEvent(_ context.Context, event SelectionFailedEvent) error {
	s.capturedSelectionFailed = append(s.capturedSelectionFailed, event)
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
		s.Assert().Len(s.capturedItemAdded, 1)
		event := s.capturedItemAdded[0]
		s.Assert().Equal("test_table", event.TableID)
		s.Assert().Equal("sword", event.ItemID)
		s.Assert().Equal(10, event.Weight)
	})

	s.Run("publishes events when selection succeeds", func() {
		// Reset events
		s.capturedSelectionCompleted = make([]SelectionCompletedEvent, 0)

		s.table.Add("item", 100)
		_, err := s.table.Select(s.ctx)
		s.Require().NoError(err)

		// Should have at least one selection completed event
		s.Assert().Len(s.capturedSelectionCompleted, 1)
		event := s.capturedSelectionCompleted[0]
		s.Assert().Equal("test_table", event.TableID)
		s.Assert().Equal("select", event.Operation)
		s.Assert().Equal(1, event.RequestCount)
		s.Assert().Equal(1, event.ActualCount)
	})

	s.Run("publishes events when selection fails", func() {
		// Reset events and use empty table
		s.capturedSelectionFailed = make([]SelectionFailedEvent, 0)
		emptyTable := NewBasicTable[string](BasicTableConfig{
			ID: "empty_table",
			Configuration: TableConfiguration{
				EnableEvents: true,
			},
		})

		// Connect the empty table to the event bus
		if basicEmptyTable, ok := emptyTable.(*BasicTable[string]); ok {
			basicEmptyTable.ConnectToEventBus(s.eventBus)
		}

		_, err := emptyTable.Select(s.ctx)
		s.Assert().Error(err)

		// Should have selection failed event
		s.Assert().Len(s.capturedSelectionFailed, 1)
		event := s.capturedSelectionFailed[0]
		s.Assert().Equal("empty_table", event.TableID)
		s.Assert().Equal("select", event.Operation)
		s.Assert().Contains(event.Error, "empty table")
	})
}

// TestEventConfiguration tests that event configuration is respected
func (s *SimpleEventTestSuite) TestEventConfiguration() {
	s.Run("respects event configuration", func() {
		// Create table with events disabled
		config := BasicTableConfig{
			ID: "no_events_table",
			Configuration: TableConfiguration{
				EnableEvents: false,
			},
		}

		noEventsTable := NewBasicTable[string](config)

		// Connect table to bus (but events should be disabled by config)
		if basicNoEventsTable, ok := noEventsTable.(*BasicTable[string]); ok {
			basicNoEventsTable.ConnectToEventBus(s.eventBus)
		}

		// Reset events
		s.capturedItemAdded = make([]ItemAddedEvent, 0)
		s.capturedSelectionCompleted = make([]SelectionCompletedEvent, 0)

		noEventsTable.Add("item", 10)
		_, _ = noEventsTable.Select(s.ctx)

		// Should have no events
		s.Assert().Empty(s.capturedItemAdded, "Should have no item added events when disabled")
		s.Assert().Empty(s.capturedSelectionCompleted, "Should have no selection completed events when disabled")
	})
}
