package spatial_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const testBoundaryRoomType = "boundary"

type BoundaryTestSuite struct {
	suite.Suite
	room *spatial.BasicRoom
}

func TestBoundarySuite(t *testing.T) {
	suite.Run(t, new(BoundaryTestSuite))
}

func (s *BoundaryTestSuite) SetupTest() {
	s.room = s.newRoom()
}

func (s *BoundaryTestSuite) newRoom() *spatial.BasicRoom {
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "boundary-room",
		Type: testBoundaryRoomType,
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{
			Width:  6,
			Height: 6,
		}),
	})
	room.ConnectToEventBus(events.NewEventBus())
	return room
}

func (s *BoundaryTestSuite) TestRegisterBoundaryNormalizesAndQueriesBothDirections() {
	left := spatial.Position{X: 1, Y: 2}
	right := spatial.Position{X: 2, Y: 2}

	err := s.room.RegisterBoundary(spatial.Boundary{
		From:              right,
		To:                left,
		BlocksMovement:    true,
		BlocksLineOfSight: false,
	})
	s.Require().NoError(err)

	forward, found := s.room.GetBoundary(left, right)
	s.Require().True(found)
	s.Equal(left, forward.From)
	s.Equal(right, forward.To)
	s.True(forward.BlocksMovement)
	s.False(forward.BlocksLineOfSight)

	reverse, found := s.room.GetBoundary(right, left)
	s.Require().True(found)
	s.Equal(forward, reverse)
	s.True(s.room.IsBoundaryMovementBlocked(left, right))
	s.True(s.room.IsBoundaryMovementBlocked(right, left))
	s.False(s.room.IsBoundaryLineOfSightBlocked(left, right))
}

func (s *BoundaryTestSuite) TestRegisterBoundaryRejectsInvalidEndpoints() {
	tests := []struct {
		name     string
		boundary spatial.Boundary
	}{
		{
			name: "self",
			boundary: spatial.Boundary{
				From: spatial.Position{X: 1, Y: 1},
				To:   spatial.Position{X: 1, Y: 1},
			},
		},
		{
			name: "nonadjacent",
			boundary: spatial.Boundary{
				From: spatial.Position{X: 1, Y: 1},
				To:   spatial.Position{X: 3, Y: 1},
			},
		},
		{
			name: "from out of grid",
			boundary: spatial.Boundary{
				From: spatial.Position{X: -1, Y: 1},
				To:   spatial.Position{X: 0, Y: 1},
			},
		},
		{
			name: "to out of grid",
			boundary: spatial.Boundary{
				From: spatial.Position{X: 5, Y: 1},
				To:   spatial.Position{X: 6, Y: 1},
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Error(s.room.RegisterBoundary(tc.boundary))
			_, found := s.room.GetBoundary(tc.boundary.From, tc.boundary.To)
			s.False(found)
		})
	}
}

func (s *BoundaryTestSuite) TestBoundaryRejectsNonDiscreteEndpointsOnDiscreteGrids() {
	tests := []struct {
		name string
		grid spatial.Grid
	}{
		{
			name: "square grid",
			grid: spatial.NewSquareGrid(spatial.SquareGridConfig{
				Width:  6,
				Height: 6,
			}),
		},
		{
			name: "offset hex",
			grid: spatial.NewHexGrid(spatial.HexGridConfig{
				Width:       6,
				Height:      6,
				Orientation: spatial.HexOrientationPointyTop,
			}),
		},
		{
			name: "axial hex",
			grid: spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
				SpanWidth:  12,
				SpanHeight: 12,
			}),
		},
	}
	invalidEndpoints := []struct {
		name     string
		boundary spatial.Boundary
	}{
		{
			name: "fractional from",
			boundary: spatial.Boundary{
				From: spatial.Position{X: 1.5, Y: 1},
				To:   spatial.Position{X: 2, Y: 1},
			},
		},
		{
			name: "fractional to",
			boundary: spatial.Boundary{
				From: spatial.Position{X: 1, Y: 1},
				To:   spatial.Position{X: 2.5, Y: 1},
			},
		},
		{
			name: "nan",
			boundary: spatial.Boundary{
				From: spatial.Position{X: math.NaN(), Y: 1},
				To:   spatial.Position{X: 2, Y: 1},
			},
		},
		{
			name: "positive infinity",
			boundary: spatial.Boundary{
				From: spatial.Position{X: math.Inf(1), Y: 1},
				To:   spatial.Position{X: 2, Y: 1},
			},
		},
		{
			name: "negative infinity",
			boundary: spatial.Boundary{
				From: spatial.Position{X: 1, Y: 1},
				To:   spatial.Position{X: math.Inf(-1), Y: 1},
			},
		},
	}

	for _, gridTest := range tests {
		s.Run(gridTest.name, func() {
			room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
				ID:   "discrete-boundary-room",
				Type: testBoundaryRoomType,
				Grid: gridTest.grid,
			})
			for _, endpointTest := range invalidEndpoints {
				s.Run(endpointTest.name, func() {
					s.Error(room.RegisterBoundary(endpointTest.boundary))
				})
			}
		})
	}
}

func (s *BoundaryTestSuite) TestBoundaryBlocksAdjacentMovementWithoutBlockingEndpoints() {
	left := spatial.Position{X: 1, Y: 2}
	right := spatial.Position{X: 2, Y: 2}
	mover := NewMockEntity("boundary-mover", "character")

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           left,
		To:             right,
		BlocksMovement: true,
	}))
	s.Require().NoError(s.room.PlaceEntity(mover, left))

	s.True(s.room.CanPlaceEntity(mover, left), "a boundary never blocks its first endpoint")
	s.True(s.room.CanPlaceEntity(mover, right), "a boundary never blocks its second endpoint")
	s.Error(s.room.MoveEntity(mover.GetID(), right))

	// Placement has no crossing semantics, so it remains valid. It also lets
	// this test prove the same boundary blocks the reverse crossing.
	s.Require().NoError(s.room.PlaceEntity(mover, right))
	s.Error(s.room.MoveEntity(mover.GetID(), left))
}

func (s *BoundaryTestSuite) TestBoundaryFlagsIndependentlyControlMovementAndLineOfSight() {
	left := spatial.Position{X: 1, Y: 2}
	right := spatial.Position{X: 2, Y: 2}
	mover := NewMockEntity("independent-mover", "character")
	s.Require().NoError(s.room.PlaceEntity(mover, left))

	s.Run("movement only", func() {
		s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
			From:           left,
			To:             right,
			BlocksMovement: true,
		}))

		s.Error(s.room.MoveEntity(mover.GetID(), right))
		s.False(s.room.IsLineOfSightBlocked(left, right))
	})

	s.Run("line of sight only", func() {
		s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
			From:              left,
			To:                right,
			BlocksLineOfSight: true,
		}))

		s.Require().NoError(s.room.MoveEntity(mover.GetID(), right))
		s.True(s.room.IsLineOfSightBlocked(left, right), "adjacent endpoints still cross their shared boundary")
	})
}

func (s *BoundaryTestSuite) TestMultiCellDirectMovementChecksEveryCrossing() {
	from := spatial.Position{X: 1, Y: 2}
	to := spatial.Position{X: 4, Y: 2}
	mover := NewMockEntity("multi-cell-mover", "character")

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           spatial.Position{X: 2, Y: 2},
		To:             spatial.Position{X: 3, Y: 2},
		BlocksMovement: true,
	}))
	s.Require().NoError(s.room.PlaceEntity(mover, from))

	// MoveEntity retains its direct-move semantics: it uses the grid's direct
	// ray rather than finding a detour, and must reject the intermediate edge.
	s.Error(s.room.MoveEntity(mover.GetID(), to))
	position, found := s.room.GetEntityPosition(mover.GetID())
	s.True(found)
	s.Equal(from, position)
}

func (s *BoundaryTestSuite) TestMovementQueryChecksEveryDirectCrossing() {
	from := spatial.Position{X: 1, Y: 2}
	to := spatial.Position{X: 4, Y: 2}
	mover := NewMockEntity("multi-cell-query-mover", "character")
	queryHandler := spatial.NewSpatialQueryHandler()
	queryHandler.RegisterRoom(s.room)

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           spatial.Position{X: 2, Y: 2},
		To:             spatial.Position{X: 3, Y: 2},
		BlocksMovement: true,
	}))

	result, err := queryHandler.HandleQuery(context.Background(), &spatial.QueryMovementData{
		Entity: mover,
		From:   from,
		To:     to,
		RoomID: s.room.GetID(),
	})
	s.Require().NoError(err)
	query := result.(*spatial.QueryMovementData)
	s.False(query.Valid)
	s.Equal([]spatial.Position{
		{X: 1, Y: 2},
		{X: 2, Y: 2},
		{X: 3, Y: 2},
		{X: 4, Y: 2},
	}, query.Path)
}

func (s *BoundaryTestSuite) TestBoundaryLineOfSightChecksEveryRayCrossing() {
	from := spatial.Position{X: 1, Y: 2}
	to := spatial.Position{X: 4, Y: 2}

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:              spatial.Position{X: 2, Y: 2},
		To:                spatial.Position{X: 3, Y: 2},
		BlocksLineOfSight: true,
	}))

	s.True(s.room.IsLineOfSightBlocked(from, to))
	s.True(s.room.IsLineOfSightBlocked(to, from), "a normalized boundary blocks the reverse ray too")
}

func (s *BoundaryTestSuite) TestBoundaryLineOfSightUsesCanonicalSquareRay() {
	from := spatial.Position{X: 0, Y: 0}
	to := spatial.Position{X: 2, Y: 1}

	// Square Bresenham selects different intermediate cells when this ray is
	// reversed. The boundary check must still use one canonical ordering.
	s.Equal([]spatial.Position{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 2, Y: 1},
	}, s.room.GetLineOfSight(from, to))
	s.Equal([]spatial.Position{
		{X: 2, Y: 1},
		{X: 1, Y: 1},
		{X: 0, Y: 0},
	}, s.room.GetLineOfSight(to, from))

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:              spatial.Position{X: 0, Y: 0},
		To:                spatial.Position{X: 1, Y: 0},
		BlocksLineOfSight: true,
	}))

	forwardBlocked := s.room.IsLineOfSightBlocked(from, to)
	reverseBlocked := s.room.IsLineOfSightBlocked(to, from)
	s.True(forwardBlocked)
	s.Equal(forwardBlocked, reverseBlocked, "boundary LoS must be reciprocal")
}

func (s *BoundaryTestSuite) TestCanonicalBoundaryRayDoesNotWeakenEntityBlockers() {
	from := spatial.Position{X: 0, Y: 0}
	to := spatial.Position{X: 2, Y: 1}
	blocker := NewMockEntity("directional-ray-blocker", "wall").WithBlocking(false, true)

	// The reverse Bresenham ray visits (1,1), while the canonical boundary
	// ray does not. Entity blocker checks retain the requested-direction ray.
	s.Require().NoError(s.room.PlaceEntity(blocker, spatial.Position{X: 1, Y: 1}))
	s.True(s.room.IsLineOfSightBlocked(to, from))
}

func (s *BoundaryTestSuite) TestBoundaryClearAndRemovalRestoreOnlyThatCrossing() {
	left := spatial.Position{X: 1, Y: 2}
	right := spatial.Position{X: 2, Y: 2}
	otherLeft := spatial.Position{X: 3, Y: 3}
	otherRight := spatial.Position{X: 4, Y: 3}
	mover := NewMockEntity("clear-mover", "character")
	s.Require().NoError(s.room.PlaceEntity(mover, left))

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           left,
		To:             right,
		BlocksMovement: true,
	}))
	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           otherLeft,
		To:             otherRight,
		BlocksMovement: true,
	}))
	s.Error(s.room.MoveEntity(mover.GetID(), right))

	// Re-registering the same normalized pair updates its flags. An open
	// boundary stays queryable but no longer blocks a crossing.
	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{From: right, To: left}))
	s.Require().NoError(s.room.MoveEntity(mover.GetID(), right))
	s.True(s.room.IsBoundaryMovementBlocked(otherLeft, otherRight), "unrelated boundaries remain unchanged")

	// A closed boundary blocks again until removing the record restores both
	// directions of its crossing.
	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           left,
		To:             right,
		BlocksMovement: true,
	}))
	s.Error(s.room.MoveEntity(mover.GetID(), left))
	s.Require().NoError(s.room.RemoveBoundary(left, right))
	_, found := s.room.GetBoundary(right, left)
	s.False(found)
	s.False(s.room.IsBoundaryMovementBlocked(left, right))
	s.Require().NoError(s.room.MoveEntity(mover.GetID(), left), "removing restores the reverse crossing too")
}

func (s *BoundaryTestSuite) TestMovementQueryHonorsOptionalBoundaryCapability() {
	from := spatial.Position{X: 1, Y: 2}
	to := spatial.Position{X: 2, Y: 2}
	mover := NewMockEntity("query-mover", "character")
	queryHandler := spatial.NewSpatialQueryHandler()
	queryHandler.RegisterRoom(s.room)

	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{
		From:           from,
		To:             to,
		BlocksMovement: true,
	}))

	result, err := queryHandler.HandleQuery(context.Background(), &spatial.QueryMovementData{
		Entity: mover,
		From:   from,
		To:     to,
		RoomID: s.room.GetID(),
	})
	s.Require().NoError(err)
	s.False(result.(*spatial.QueryMovementData).Valid)

	s.Require().NoError(s.room.RemoveBoundary(to, from))
	result, err = queryHandler.HandleQuery(context.Background(), &spatial.QueryMovementData{
		Entity: mover,
		From:   from,
		To:     to,
		RoomID: s.room.GetID(),
	})
	s.Require().NoError(err)
	s.True(result.(*spatial.QueryMovementData).Valid)
}

func (s *BoundaryTestSuite) TestBoundarySupportsPointyTopAndAxialHexGrids() {
	tests := []struct {
		name string
		grid spatial.Grid
		from spatial.Position
		to   spatial.Position
	}{
		{
			name: "pointy top offset hex",
			grid: spatial.NewHexGrid(spatial.HexGridConfig{
				Width:       6,
				Height:      6,
				Orientation: spatial.HexOrientationPointyTop,
			}),
			from: spatial.Position{X: 2, Y: 2},
			to:   spatial.Position{X: 3, Y: 2},
		},
		{
			name: "axial hex",
			grid: spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
				SpanWidth:  12,
				SpanHeight: 12,
			}),
			from: spatial.Position{X: 0, Y: 0},
			to:   spatial.Position{X: 1, Y: 0},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
				ID:   "hex-boundary-room",
				Type: testBoundaryRoomType,
				Grid: tc.grid,
			})
			mover := NewMockEntity("hex-boundary-mover", "character")
			s.Require().NoError(room.PlaceEntity(mover, tc.from))
			s.Require().NoError(room.RegisterBoundary(spatial.Boundary{
				From:              tc.from,
				To:                tc.to,
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			}))

			s.Error(room.MoveEntity(mover.GetID(), tc.to))
			s.True(room.IsLineOfSightBlocked(tc.from, tc.to))
		})
	}
}

func (s *BoundaryTestSuite) TestBoundaryRejectsGridlessRooms() {
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "gridless-boundary-room",
		Type: testBoundaryRoomType,
		Grid: spatial.NewGridlessRoom(spatial.GridlessConfig{
			Width:  6,
			Height: 6,
		}),
	})

	s.Error(room.RegisterBoundary(spatial.Boundary{
		From: spatial.Position{X: 1, Y: 1},
		To:   spatial.Position{X: 2, Y: 1},
	}))
}

func (s *BoundaryTestSuite) TestLegacyMovementAndLineOfSightRemainOpenWithoutBoundaries() {
	from := spatial.Position{X: 1, Y: 2}
	to := spatial.Position{X: 2, Y: 2}
	mover := NewMockEntity("legacy-mover", "character")

	s.Require().NoError(s.room.PlaceEntity(mover, from))
	s.False(s.room.IsLineOfSightBlocked(from, to))
	s.Require().NoError(s.room.MoveEntity(mover.GetID(), to))
}
