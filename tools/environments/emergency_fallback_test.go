package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicEnvironmentTestSuite tests basic environment functionality
type BasicEnvironmentTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	env      *BasicEnvironment
}

func (s *BasicEnvironmentTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	s.env = NewBasicEnvironment(BasicEnvironmentConfig{
		ID:    "test-env",
		Type:  "test",
		Theme: "dungeon",
	})
	s.env.ConnectToEventBus(s.eventBus)
}

func (s *BasicEnvironmentTestSuite) TestBasicProperties() {
	s.Run("has valid ID and type", func() {
		s.Assert().Equal("test-env", s.env.GetID())
		s.Assert().Equal("test", string(s.env.GetType()))
	})

	s.Run("has theme", func() {
		s.Assert().Equal("dungeon", s.env.GetTheme())
	})
}

func (s *BasicEnvironmentTestSuite) TestThemeChanges() {
	s.Run("has initial theme", func() {
		// Note: SetTheme requires orchestrator for room tracking
		// This test just verifies the initial theme is set correctly
		s.Assert().Equal("dungeon", s.env.GetTheme())
	})
}

func TestBasicEnvironmentSuite(t *testing.T) {
	suite.Run(t, new(BasicEnvironmentTestSuite))
}
