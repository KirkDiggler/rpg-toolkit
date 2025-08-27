package selectables

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// EventIntegrationTestSuite tests selectables event integration
type EventIntegrationTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	ctx      SelectionContext
	table    SelectionTable[string]
}

// SetupTest initializes test environment before each test
func (s *EventIntegrationTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	testRoller := NewTestRoller([]int{50, 25, 75})
	s.ctx = NewSelectionContextWithRoller(testRoller)

	config := BasicTableConfig{
		ID: "event_test_table",
		Configuration: TableConfiguration{
			EnableEvents: true,
		},
	}
	s.table = NewBasicTable[string](config)
	if basicTable, ok := s.table.(*BasicTable[string]); ok {
		basicTable.ConnectToEventBus(s.eventBus)
	}
}

// TestEventIntegrationSuite runs the event integration test suite
func TestEventIntegrationSuite(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}

// TestBasicEventIntegration tests basic event functionality
func (s *EventIntegrationTestSuite) TestBasicEventIntegration() {
	s.Run("handles table operations with event bus", func() {
		// Test that event bus connection works
		s.table.Add("sword", 10)
		s.table.Add("shield", 20)

		// Test selection
		result, err := s.table.Select(s.ctx)
		s.Assert().NoError(err)
		s.Assert().NotEmpty(result)
	})

	s.Run("handles multiple selections", func() {
		s.table.Add("item1", 50)
		s.table.Add("item2", 50)

		results, err := s.table.SelectMany(s.ctx, 2)
		s.Assert().NoError(err)
		s.Assert().Len(results, 2)
	})
}

// TestEventConfiguration tests event configuration
func (s *EventIntegrationTestSuite) TestEventConfiguration() {
	s.Run("handles disabled events configuration", func() {
		disabledConfig := BasicTableConfig{
			ID: "disabled_events_table",
			Configuration: TableConfiguration{
				EnableEvents: false,
			},
		}

		disabledTable := NewBasicTable[string](disabledConfig)
		if basicTable, ok := disabledTable.(*BasicTable[string]); ok {
			basicTable.ConnectToEventBus(s.eventBus)
		}

		disabledTable.Add("item", 10)
		result, err := disabledTable.Select(s.ctx)
		s.Assert().NoError(err)
		s.Assert().Equal("item", result)
	})
}

// TestSelectionErrors tests error handling
func (s *EventIntegrationTestSuite) TestSelectionErrors() {
	s.Run("handles empty table selection", func() {
		emptyTable := NewBasicTable[string](BasicTableConfig{
			ID: "empty_table",
			Configuration: TableConfiguration{
				EnableEvents: true,
			},
		})
		if basicTable, ok := emptyTable.(*BasicTable[string]); ok {
			basicTable.ConnectToEventBus(s.eventBus)
		}

		_, err := emptyTable.Select(s.ctx)
		s.Assert().Error(err)
	})

	s.Run("handles invalid count", func() {
		s.table.Add("item", 10)
		_, err := s.table.SelectMany(s.ctx, 0)
		s.Assert().Error(err)
	})
}
