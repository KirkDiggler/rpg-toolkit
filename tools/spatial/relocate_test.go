package spatial_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RelocateTestSuite pins RelocateEntity (rpg-project#925): an entity set down
// elsewhere without travelling between, atomically, or not at all.
type RelocateTestSuite struct {
	suite.Suite
	room  *spatial.BasicRoom
	moved []spatial.EntityMovedEvent
	mu    sync.Mutex
}

func TestRelocateSuite(t *testing.T) {
	suite.Run(t, new(RelocateTestSuite))
}

func (s *RelocateTestSuite) SetupTest() {
	bus := events.NewEventBus()
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "relocate-room", Type: "test",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 6, Height: 6}),
	})
	s.room.ConnectToEventBus(bus)
	s.moved = nil
	_, err := spatial.EntityMovedTopic.On(bus).Subscribe(context.Background(),
		func(_ context.Context, e spatial.EntityMovedEvent) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.moved = append(s.moved, e)
			return nil
		})
	s.Require().NoError(err)
}

// at is where an entity stands, failing when it has no position.
func (s *RelocateTestSuite) at(id string) spatial.Position {
	pos, ok := s.room.GetEntityPosition(id)
	s.Require().True(ok, "%s has a position", id)
	return pos
}

// TestAcrossAWall: a wall that refuses the walk does not refuse the
// relocation, and the event says it was a relocation; the walk's own event
// stays "normal".
func (s *RelocateTestSuite) TestAcrossAWall() {
	left, right, beyond := spatial.Position{X: 1, Y: 2}, spatial.Position{X: 2, Y: 2}, spatial.Position{X: 5, Y: 5}
	s.Require().NoError(s.room.RegisterBoundary(spatial.Boundary{From: left, To: right, BlocksMovement: true}))
	mover := NewMockEntity("mover", "character")
	s.Require().NoError(s.room.PlaceEntity(mover, left))

	s.Require().Error(s.room.MoveEntity("mover", right), "precondition: the wall refuses the walk")
	s.Require().NoError(s.room.RelocateEntity("mover", right), "the wall does not refuse the relocation")
	s.Equal(right, s.at("mover"))
	s.Require().NoError(s.room.RelocateEntity("mover", beyond), "nor does any distance")
	s.Equal(beyond, s.at("mover"))
	s.Empty(s.room.GetEntitiesAt(left), "the old cells are given up")
	s.Empty(s.room.GetEntitiesAt(right))

	s.Require().NoError(s.room.MoveEntity("mover", spatial.Position{X: 5, Y: 4}))
	s.Require().Len(s.moved, 3)
	s.Equal(spatial.MovementRelocated, s.moved[0].MovementType)
	s.Equal(left, s.moved[0].FromPosition)
	s.Equal(right, s.moved[0].ToPosition)
	s.Equal(spatial.MovementRelocated, s.moved[1].MovementType)
	s.Equal("normal", s.moved[2].MovementType, "a walk's event is told apart")
}

// TestOntoABlockingEntityIsRefusedAndMovesNothing: a cell held by an entity
// that blocks movement refuses the relocation, and the mover stays put.
func (s *RelocateTestSuite) TestOntoABlockingEntityIsRefusedAndMovesNothing() {
	from, taken := spatial.Position{X: 0, Y: 0}, spatial.Position{X: 3, Y: 3}
	s.Require().NoError(s.room.PlaceEntity(NewMockEntity("mover", "character"), from))
	s.Require().NoError(s.room.PlaceEntity(NewMockEntity("wall", "obstacle").WithBlocking(true, true), taken))

	s.Error(s.room.RelocateEntity("mover", taken))
	s.Equal(from, s.at("mover"), "a refusal leaves it where it was")
	s.Len(s.room.GetEntitiesAt(from), 1, "still counted on its own cell")
	s.Len(s.room.GetEntitiesAt(taken), 1, "and not on the refused one")
	s.Empty(s.moved, "nothing is published")
}

// TestOntoAnInvalidCellIsRefusedAndMovesNothing: a position off the grid is
// refused, and the mover stays put.
func (s *RelocateTestSuite) TestOntoAnInvalidCellIsRefusedAndMovesNothing() {
	from := spatial.Position{X: 2, Y: 2}
	s.Require().NoError(s.room.PlaceEntity(NewMockEntity("mover", "character"), from))
	for _, off := range []spatial.Position{{X: -1, Y: 0}, {X: 6, Y: 2}, {X: 2, Y: 99}} {
		s.Error(s.room.RelocateEntity("mover", off), "%v is off the grid", off)
		s.Equal(from, s.at("mover"))
	}
	s.Empty(s.moved)
}

// TestOntoItsOwnCellIsANoOp: relocating an entity to where it stands changes
// nothing and publishes nothing.
func (s *RelocateTestSuite) TestOntoItsOwnCellIsANoOp() {
	here := spatial.Position{X: 4, Y: 1}
	s.Require().NoError(s.room.PlaceEntity(NewMockEntity("mover", "character"), here))
	s.NoError(s.room.RelocateEntity("mover", here))
	s.Equal(here, s.at("mover"))
	s.Len(s.room.GetEntitiesAt(here), 1, "counted once")
	s.Empty(s.moved, "a no-op publishes nothing")
}

// TestAnUnknownEntityIsRefused: only an entity in the room can be relocated;
// relocation never places a newcomer.
func (s *RelocateTestSuite) TestAnUnknownEntityIsRefused() {
	s.Error(s.room.RelocateEntity("nobody", spatial.Position{X: 1, Y: 1}))
	s.Empty(s.room.GetEntitiesAt(spatial.Position{X: 1, Y: 1}))
	s.Empty(s.moved)
}

// TestConcurrentRelocationsNeitherStrandNorDuplicate: many callers relocating
// many entities onto blocking-free cells at once leave every entity on
// exactly one cell, counted once (run under the race detector).
func (s *RelocateTestSuite) TestConcurrentRelocationsNeitherStrandNorDuplicate() {
	const n = 6
	for i := 0; i < n; i++ {
		s.Require().NoError(s.room.PlaceEntity(NewMockEntity(fmt.Sprintf("e%d", i), "character").WithBlocking(true, false),
			spatial.Position{X: float64(i), Y: 0}))
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		for row := 1; row < 6; row++ {
			wg.Add(1)
			go func(i, row int) {
				defer wg.Done()
				// Every goroutine aims at a cell another may also aim at: a
				// loser is refused and keeps its cell.
				_ = s.room.RelocateEntity(fmt.Sprintf("e%d", i), spatial.Position{X: float64((i + row) % n), Y: float64(row)})
			}(i, row)
		}
	}
	wg.Wait()

	seen := map[string]int{}
	for x := 0; x < 6; x++ {
		for y := 0; y < 6; y++ {
			for _, e := range s.room.GetEntitiesAt(spatial.Position{X: float64(x), Y: float64(y)}) {
				seen[e.GetID()]++
			}
		}
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("e%d", i)
		s.Equal(1, seen[id], "%s stands on exactly one cell", id)
		at := s.at(id)
		found := false
		for _, e := range s.room.GetEntitiesAt(at) {
			found = found || e.GetID() == id
		}
		s.True(found, "%s is counted on the cell its position names", id)
	}
	s.Len(s.room.GetAllEntities(), n, "nobody stranded, nobody duplicated")
}
