package dungeon

import (
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/stretchr/testify/suite"
)

type DungeonTestSuite struct {
	suite.Suite
}

func TestDungeonSuite(t *testing.T) {
	suite.Run(t, new(DungeonTestSuite))
}

func (s *DungeonTestSuite) createTestDungeonData() *DungeonData {
	return &DungeonData{
		Environment: environments.EnvironmentData{
			ID:   "dungeon-1",
			Type: "dungeon",
			Zones: []environments.ZoneData{
				{ID: "room-1", Type: "room", Width: 10, Height: 10},
				{ID: "room-2", Type: "room", Width: 10, Height: 10},
				{ID: "room-3", Type: "room", Width: 15, Height: 15},
			},
			Passages: []environments.PassageData{
				{ID: "door-1", FromZoneID: "room-1", ToZoneID: "room-2", Bidirectional: true},
				{ID: "door-2", FromZoneID: "room-2", ToZoneID: "room-3", Bidirectional: true},
			},
		},
		StartRoomID: "room-1",
		BossRoomID:  "room-3",
		Seed:        12345,
		Rooms: map[string]*RoomData{
			"room-1": {
				Type: RoomTypeEntrance,
				Encounter: &EncounterData{
					Monsters: []MonsterPlacementData{
						{ID: "goblin-1", MonsterID: "goblin", Role: RoleMelee, CR: 0.25},
					},
					TotalCR: 0.25,
				},
			},
			"room-2": {
				Type: RoomTypeChamber,
				Encounter: &EncounterData{
					Monsters: []MonsterPlacementData{
						{ID: "skeleton-1", MonsterID: "skeleton", Role: RoleMelee, CR: 0.25},
						{ID: "skeleton-2", MonsterID: "skeleton", Role: RoleMelee, CR: 0.25},
					},
					TotalCR: 0.5,
				},
			},
			"room-3": {
				Type: RoomTypeBoss,
				Encounter: &EncounterData{
					Monsters: []MonsterPlacementData{
						{ID: "boss-1", MonsterID: "ogre", Role: RoleBoss, CR: 2.0},
					},
					TotalCR: 2.0,
				},
			},
		},
		State:         StateActive,
		CurrentRoomID: "room-1",
		RevealedRooms: map[string]bool{"room-1": true},
		OpenDoors:     map[string]bool{},
		CreatedAt:     time.Now(),
	}
}

func (s *DungeonTestSuite) TestNew() {
	s.Run("creates dungeon from valid data", func() {
		data := s.createTestDungeonData()
		d := New(data)

		s.Require().NotNil(d)
		s.Assert().Equal("dungeon-1", d.ID())
		s.Assert().Equal(StateActive, d.State())
	})

	s.Run("returns nil for nil data", func() {
		d := New(nil)
		s.Assert().Nil(d)
	})
}

func (s *DungeonTestSuite) TestRoomAccess() {
	data := s.createTestDungeonData()
	d := New(data)

	s.Run("Room returns room by ID", func() {
		room := d.Room("room-1")
		s.Require().NotNil(room)
		s.Assert().Equal(RoomTypeEntrance, room.Type)
	})

	s.Run("Room returns nil for unknown ID", func() {
		room := d.Room("unknown")
		s.Assert().Nil(room)
	})

	s.Run("CurrentRoom returns current room", func() {
		room := d.CurrentRoom()
		s.Require().NotNil(room)
		s.Assert().Equal(RoomTypeEntrance, room.Type)
	})

	s.Run("StartRoom and BossRoom return correct IDs", func() {
		s.Assert().Equal("room-1", d.StartRoom())
		s.Assert().Equal("room-3", d.BossRoom())
	})

	s.Run("IsBossRoom identifies boss room", func() {
		s.Assert().False(d.IsBossRoom("room-1"))
		s.Assert().False(d.IsBossRoom("room-2"))
		s.Assert().True(d.IsBossRoom("room-3"))
	})

	s.Run("RoomIDs returns all room IDs", func() {
		ids := d.RoomIDs()
		s.Assert().Len(ids, 3)
		s.Assert().Contains(ids, "room-1")
		s.Assert().Contains(ids, "room-2")
		s.Assert().Contains(ids, "room-3")
	})
}

func (s *DungeonTestSuite) TestExplorationState() {
	data := s.createTestDungeonData()
	d := New(data)

	s.Run("RoomRevealed checks revealed state", func() {
		s.Assert().True(d.RoomRevealed("room-1"))
		s.Assert().False(d.RoomRevealed("room-2"))
	})

	s.Run("RevealRoom marks room as revealed", func() {
		d.RevealRoom("room-2")
		s.Assert().True(d.RoomRevealed("room-2"))
	})

	s.Run("DoorOpen checks door state", func() {
		s.Assert().False(d.DoorOpen("door-1"))
	})

	s.Run("OpenDoor marks door as open", func() {
		d.OpenDoor("door-1")
		s.Assert().True(d.DoorOpen("door-1"))
	})

	s.Run("SetCurrentRoom changes current room", func() {
		d.SetCurrentRoom("room-2")
		s.Assert().Equal("room-2", d.CurrentRoomID())
	})
}

func (s *DungeonTestSuite) TestDoorQueries() {
	data := s.createTestDungeonData()
	d := New(data)

	s.Run("Doors returns all passages", func() {
		doors := d.Doors()
		s.Assert().Len(doors, 2)
	})

	s.Run("DoorsFromRoom returns connections from specific room", func() {
		doors := d.DoorsFromRoom("room-1")
		s.Assert().Len(doors, 1)
		s.Assert().Equal("door-1", doors[0].ID)

		doors = d.DoorsFromRoom("room-2")
		s.Assert().Len(doors, 2) // Connected to both room-1 and room-3
	})

	s.Run("VisibleDoors returns doors to unrevealed rooms", func() {
		// room-1 is revealed, room-2 and room-3 are not
		visible := d.VisibleDoors()
		s.Assert().Len(visible, 1)
		s.Assert().Equal("door-1", visible[0].ID) // Leads to room-2

		// Reveal room-2 and move there
		d.RevealRoom("room-2")
		d.SetCurrentRoom("room-2")

		visible = d.VisibleDoors()
		s.Assert().Len(visible, 1)
		s.Assert().Equal("door-2", visible[0].ID) // Leads to room-3
	})
}

func (s *DungeonTestSuite) TestMetrics() {
	data := s.createTestDungeonData()
	d := New(data)

	s.Run("tracks rooms cleared", func() {
		s.Assert().Equal(0, d.RoomsCleared())
		d.IncrementRoomsCleared()
		s.Assert().Equal(1, d.RoomsCleared())
	})

	s.Run("tracks monsters killed", func() {
		s.Assert().Equal(0, d.MonstersKilled())
		d.IncrementMonstersKilled(3)
		s.Assert().Equal(3, d.MonstersKilled())
		d.IncrementMonstersKilled(2)
		s.Assert().Equal(5, d.MonstersKilled())
	})
}

func (s *DungeonTestSuite) TestStateTransitions() {
	s.Run("MarkVictory sets state and completion time", func() {
		data := s.createTestDungeonData()
		d := New(data)

		s.Assert().Nil(d.CompletedAt())
		d.MarkVictory()
		s.Assert().Equal(StateVictorious, d.State())
		s.Assert().NotNil(d.CompletedAt())
	})

	s.Run("MarkFailed sets state and completion time", func() {
		data := s.createTestDungeonData()
		d := New(data)

		d.MarkFailed()
		s.Assert().Equal(StateFailed, d.State())
		s.Assert().NotNil(d.CompletedAt())
	})

	s.Run("MarkAbandoned sets state and completion time", func() {
		data := s.createTestDungeonData()
		d := New(data)

		d.MarkAbandoned()
		s.Assert().Equal(StateAbandoned, d.State())
		s.Assert().NotNil(d.CompletedAt())
	})
}

func (s *DungeonTestSuite) TestPersistence() {
	data := s.createTestDungeonData()
	d := New(data)

	s.Run("ToData returns underlying data", func() {
		returned := d.ToData()
		s.Assert().Equal(data, returned)
	})

	s.Run("mutations persist to data", func() {
		d.RevealRoom("room-2")
		d.OpenDoor("door-1")
		d.SetCurrentRoom("room-2")
		d.IncrementRoomsCleared()
		d.IncrementMonstersKilled(5)

		returned := d.ToData()
		s.Assert().True(returned.RevealedRooms["room-2"])
		s.Assert().True(returned.OpenDoors["door-1"])
		s.Assert().Equal("room-2", returned.CurrentRoomID)
		s.Assert().Equal(1, returned.RoomsCleared)
		s.Assert().Equal(5, returned.MonstersKilled)
	})

	s.Run("room modifications persist via pointer", func() {
		// Get room pointer and modify it
		room := d.Room("room-1")
		s.Require().NotNil(room)
		s.Require().NotNil(room.Encounter)

		// Clear the encounter (simulating defeating monsters)
		room.Encounter = nil

		// Verify the change persisted
		roomAgain := d.Room("room-1")
		s.Assert().Nil(roomAgain.Encounter, "room modifications should persist")

		// Also verify via ToData
		returned := d.ToData()
		s.Assert().Nil(returned.Rooms["room-1"].Encounter)
	})
}

func (s *DungeonTestSuite) TestEnvironmentAccess() {
	data := s.createTestDungeonData()
	d := New(data)

	s.Run("Environment returns environment data", func() {
		env := d.Environment()
		s.Require().NotNil(env)
		s.Assert().Equal("dungeon-1", env.ID)
		s.Assert().Len(env.Zones, 3)
	})

	s.Run("Seed returns generation seed", func() {
		s.Assert().Equal(int64(12345), d.Seed())
	})
}
