package spatial_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type QueryHandlerTestSuite struct {
	suite.Suite
	queryHandler *spatial.SpatialQueryHandler
	eventBus     events.EventBus
	room         *spatial.BasicRoom
	entity1      *MockEntity
	entity2      *MockEntity
	entity3      *MockEntity
}

// SetupTest runs before EACH test function
func (s *QueryHandlerTestSuite) SetupTest() {
	s.queryHandler = spatial.NewSpatialQueryHandler()
	s.eventBus = events.NewEventBus()

	// Create test room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})

	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "square",
		Grid: grid,
	})
	s.room.ConnectToEventBus(s.eventBus)

	// Register room with query handler
	s.queryHandler.RegisterRoom(s.room)

	// Create test entities
	s.entity1 = NewMockEntity("entity1", "character")
	s.entity2 = NewMockEntity("entity2", "monster")
	s.entity3 = NewMockEntity("entity3", "item")

	// Place entities in room
	_ = s.room.PlaceEntity(s.entity1, spatial.Position{X: 3, Y: 3})
	_ = s.room.PlaceEntity(s.entity2, spatial.Position{X: 6, Y: 6})
	_ = s.room.PlaceEntity(s.entity3, spatial.Position{X: 8, Y: 8})
}

// TestDirectQueryHandling tests the query handler directly
func (s *QueryHandlerTestSuite) TestDirectQueryHandling() {
	s.Run("positions in range query", func() {
		query := &spatial.QueryPositionsInRangeData{
			Center: spatial.Position{X: 5, Y: 5},
			Radius: 2,
			RoomID: "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryPositionsInRangeData)
		s.Assert().NotNil(queryResult.Results)
		s.Assert().True(len(queryResult.Results) > 0)
		s.Assert().Nil(queryResult.Error)
	})

	s.Run("entities in range query", func() {
		query := &spatial.QueryEntitiesInRangeData{
			Center: spatial.Position{X: 5, Y: 5},
			Radius: 5,
			RoomID: "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryEntitiesInRangeData)
		s.Assert().NotNil(queryResult.Results)
		s.Assert().True(len(queryResult.Results) > 0)
		s.Assert().Nil(queryResult.Error)
	})

	s.Run("entities in range query with filter", func() {
		filter := spatial.NewSimpleEntityFilter().WithEntityTypes("character")

		query := &spatial.QueryEntitiesInRangeData{
			Center: spatial.Position{X: 5, Y: 5},
			Radius: 10,
			RoomID: "test-room",
			Filter: filter,
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryEntitiesInRangeData)
		s.Assert().NotNil(queryResult.Results)
		s.Assert().Len(queryResult.Results, 1) // Only entity1 is a character
		s.Assert().Equal("entity1", queryResult.Results[0].GetID())
		s.Assert().Nil(queryResult.Error)
	})

	s.Run("line of sight query", func() {
		query := &spatial.QueryLineOfSightData{
			From:   spatial.Position{X: 1, Y: 1},
			To:     spatial.Position{X: 9, Y: 9},
			RoomID: "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryLineOfSightData)
		s.Assert().NotNil(queryResult.Results)
		s.Assert().True(len(queryResult.Results) > 0)
		s.Assert().Nil(queryResult.Error)
	})

	s.Run("movement query", func() {
		query := &spatial.QueryMovementData{
			Entity: s.entity1,
			From:   spatial.Position{X: 3, Y: 3},
			To:     spatial.Position{X: 5, Y: 5},
			RoomID: "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryMovementData)
		s.Assert().True(queryResult.Valid)
		s.Assert().True(queryResult.Distance > 0)
		s.Assert().NotNil(queryResult.Path)
		s.Assert().Nil(queryResult.Error)
	})

	s.Run("placement query", func() {
		entity := NewMockEntity("new-entity", "character")

		query := &spatial.QueryPlacementData{
			Entity:   entity,
			Position: spatial.Position{X: 2, Y: 2},
			RoomID:   "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryPlacementData)
		s.Assert().True(queryResult.Valid)
		s.Assert().Nil(queryResult.Error)
	})
}

// TestQueryHandlerRoomManagement tests room registration/unregistration
func (s *QueryHandlerTestSuite) TestQueryHandlerRoomManagement() {
	s.Run("unregister room", func() {
		s.queryHandler.UnregisterRoom("test-room")

		query := &spatial.QueryPositionsInRangeData{
			Center: spatial.Position{X: 5, Y: 5},
			Radius: 2,
			RoomID: "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryPositionsInRangeData)
		s.Assert().NotNil(queryResult.Error)
		s.Assert().Contains(queryResult.Error.Error(), "room test-room not found")
	})

	s.Run("register room again", func() {
		s.queryHandler.RegisterRoom(s.room)

		query := &spatial.QueryPositionsInRangeData{
			Center: spatial.Position{X: 5, Y: 5},
			Radius: 2,
			RoomID: "test-room",
		}

		result, err := s.queryHandler.HandleQuery(context.Background(), query)
		s.Require().NoError(err)

		queryResult := result.(*spatial.QueryPositionsInRangeData)
		s.Assert().Nil(queryResult.Error)
		s.Assert().NotNil(queryResult.Results)
	})
}

// TestUnsupportedQueryType tests handling of unsupported query types
func (s *QueryHandlerTestSuite) TestUnsupportedQueryType() {
	type UnsupportedQuery struct {
		Data string
	}

	query := &UnsupportedQuery{Data: "test"}

	result, err := s.queryHandler.HandleQuery(context.Background(), query)
	s.Assert().Error(err)
	s.Assert().Nil(result)
	s.Assert().Contains(err.Error(), "unsupported query type")
}

// TestEntityFiltering tests the SimpleEntityFilter
func (s *QueryHandlerTestSuite) TestEntityFiltering() {
	s.Run("filter by entity type", func() {
		filter := spatial.NewSimpleEntityFilter().WithEntityTypes("character")

		s.Assert().True(filter.Matches(s.entity1))  // character
		s.Assert().False(filter.Matches(s.entity2)) // monster
		s.Assert().False(filter.Matches(s.entity3)) // item
	})

	s.Run("filter by entity ID", func() {
		filter := spatial.NewSimpleEntityFilter().WithEntityIDs("entity1", "entity3")

		s.Assert().True(filter.Matches(s.entity1))  // entity1
		s.Assert().False(filter.Matches(s.entity2)) // entity2
		s.Assert().True(filter.Matches(s.entity3))  // entity3
	})

	s.Run("exclude entity IDs", func() {
		filter := spatial.NewSimpleEntityFilter().WithExcludeIDs("entity2")

		s.Assert().True(filter.Matches(s.entity1))  // entity1
		s.Assert().False(filter.Matches(s.entity2)) // entity2 (excluded)
		s.Assert().True(filter.Matches(s.entity3))  // entity3
	})

	s.Run("combined filters", func() {
		filter := spatial.NewSimpleEntityFilter().
			WithEntityTypes("character", "monster").
			WithExcludeIDs("entity2")

		s.Assert().True(filter.Matches(s.entity1))  // character, not excluded
		s.Assert().False(filter.Matches(s.entity2)) // monster, but excluded
		s.Assert().False(filter.Matches(s.entity3)) // item, not in allowed types
	})

	s.Run("no filters match all", func() {
		filter := spatial.NewSimpleEntityFilter()

		s.Assert().True(filter.Matches(s.entity1))
		s.Assert().True(filter.Matches(s.entity2))
		s.Assert().True(filter.Matches(s.entity3))
	})

	s.Run("nil entity", func() {
		filter := spatial.NewSimpleEntityFilter()

		s.Assert().False(filter.Matches(nil))
	})
}

// TestQueryConvenienceMethods tests the QueryUtils wrapper functions
func (s *QueryHandlerTestSuite) TestQueryConvenienceMethods() {
	// Create query utilities
	queryUtils := spatial.NewQueryUtils(s.queryHandler)

	s.Run("query entities in range", func() {
		entities, err := queryUtils.QueryEntitiesInRange(
			context.Background(),
			spatial.Position{X: 5, Y: 5},
			5.0,
			"test-room",
			nil, // no filter
		)
		s.Require().NoError(err)
		s.Assert().True(len(entities) > 0)
	})

	s.Run("query positions in range", func() {
		positions, err := queryUtils.QueryPositionsInRange(
			context.Background(),
			spatial.Position{X: 5, Y: 5},
			3.0,
			"test-room",
		)
		s.Require().NoError(err)
		s.Assert().True(len(positions) > 0)
	})

	s.Run("query line of sight", func() {
		positions, blocked, err := queryUtils.QueryLineOfSight(
			context.Background(),
			spatial.Position{X: 1, Y: 1},
			spatial.Position{X: 9, Y: 9},
			"test-room",
		)
		s.Require().NoError(err)
		s.Assert().True(len(positions) > 0)
		s.Assert().NotNil(blocked) // blocked can be true or false
	})

	s.Run("query movement", func() {
		canMove, path, distance, err := queryUtils.QueryMovement(
			context.Background(),
			s.entity1,
			spatial.Position{X: 3, Y: 3},
			spatial.Position{X: 5, Y: 5},
			"test-room",
		)
		s.Require().NoError(err)
		s.Assert().True(canMove)
		s.Assert().NotNil(path)
		s.Assert().True(distance > 0)
	})

	s.Run("query placement", func() {
		newEntity := NewMockEntity("test-placement", "character")
		canPlace, err := queryUtils.QueryPlacement(
			context.Background(),
			newEntity,
			spatial.Position{X: 2, Y: 2},
			"test-room",
		)
		s.Require().NoError(err)
		s.Assert().True(canPlace)
	})
}

// TestFilterFactories tests the filter factory functions
func (s *QueryHandlerTestSuite) TestFilterFactories() {
	s.Run("create monster filter", func() {
		filter := spatial.CreateMonsterFilter()
		s.Assert().True(filter.Matches(s.entity2))  // monster
		s.Assert().False(filter.Matches(s.entity1)) // character
		s.Assert().False(filter.Matches(s.entity3)) // item
	})

	s.Run("create combatant filter", func() {
		filter := spatial.CreateCombatantFilter()
		s.Assert().True(filter.Matches(s.entity1))  // character
		s.Assert().True(filter.Matches(s.entity2))  // monster
		s.Assert().False(filter.Matches(s.entity3)) // item
	})
}

// Run the test suite
func TestQueryHandlerSuite(t *testing.T) {
	suite.Run(t, new(QueryHandlerTestSuite))
}
