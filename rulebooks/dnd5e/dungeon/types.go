// Package dungeon provides D&D 5e dungeon structures with spatial awareness.
// It composes tools/environments for spatial infrastructure and adds game-specific
// concepts like encounters, room types, and exploration state.
package dungeon

// DungeonState tracks the lifecycle of a dungeon run.
type DungeonState int

const (
	// StateActive indicates the dungeon is in progress
	StateActive DungeonState = iota
	// StateVictorious indicates the boss was defeated
	StateVictorious
	// StateFailed indicates a total party kill
	StateFailed
	// StateAbandoned indicates players left the dungeon
	StateAbandoned
)

// RoomType defines the purpose of a room for gameplay and generation.
type RoomType string

const (
	// RoomTypeEntrance is the dungeon entry point
	RoomTypeEntrance RoomType = "entrance"
	// RoomTypeChamber is a standard tactical room
	RoomTypeChamber RoomType = "chamber"
	// RoomTypeCorridor is a narrow connecting passage
	RoomTypeCorridor RoomType = "corridor"
	// RoomTypeBoss is the final boss encounter room
	RoomTypeBoss RoomType = "boss"
	// RoomTypeTreasure contains loot and is defensible
	RoomTypeTreasure RoomType = "treasure"
	// RoomTypeTrap is maze-like with limited sightlines
	RoomTypeTrap RoomType = "trap"
)

// MonsterRole defines tactical purpose in encounters.
type MonsterRole string

const (
	// RoleMelee is for close combat monsters
	RoleMelee MonsterRole = "melee"
	// RoleRanged is for distance attackers
	RoleRanged MonsterRole = "ranged"
	// RoleSupport is for healers and buffers
	RoleSupport MonsterRole = "support"
	// RoleBoss is for boss monsters
	RoleBoss MonsterRole = "boss"
)

// ConnectionType defines how rooms are linked.
type ConnectionType string

const (
	// ConnectionTypeDoor is a standard door
	ConnectionTypeDoor ConnectionType = "door"
	// ConnectionTypeStairs connects different levels
	ConnectionTypeStairs ConnectionType = "stairs"
	// ConnectionTypePassage is an open passage
	ConnectionTypePassage ConnectionType = "passage"
)

// Direction represents a cardinal direction for connection placement.
type Direction string

const (
	// DirectionNorth places connection on the north (top) wall
	DirectionNorth Direction = "north"
	// DirectionSouth places connection on the south (bottom) wall
	DirectionSouth Direction = "south"
	// DirectionEast places connection on the east (right) wall
	DirectionEast Direction = "east"
	// DirectionWest places connection on the west (left) wall
	DirectionWest Direction = "west"
	// DirectionUp for vertical connections (stairs up)
	DirectionUp Direction = "up"
	// DirectionDown for vertical connections (stairs down)
	DirectionDown Direction = "down"
)

// Opposite returns the opposite direction.
func (d Direction) Opposite() Direction {
	switch d {
	case DirectionNorth:
		return DirectionSouth
	case DirectionSouth:
		return DirectionNorth
	case DirectionEast:
		return DirectionWest
	case DirectionWest:
		return DirectionEast
	case DirectionUp:
		return DirectionDown
	case DirectionDown:
		return DirectionUp
	default:
		return ""
	}
}

// ObstacleType defines kinds of obstacles in rooms.
type ObstacleType string

const (
	// ObstacleTypePillar is a column
	ObstacleTypePillar ObstacleType = "pillar"
	// ObstacleTypeSarcophagus is a stone coffin
	ObstacleTypeSarcophagus ObstacleType = "sarcophagus"
	// ObstacleTypeAltar is a ritual table
	ObstacleTypeAltar ObstacleType = "altar"
	// ObstacleTypeBoulder is a large rock
	ObstacleTypeBoulder ObstacleType = "boulder"
	// ObstacleTypeStalagmite is a floor spike
	ObstacleTypeStalagmite ObstacleType = "stalagmite"
	// ObstacleTypePool is a water feature
	ObstacleTypePool ObstacleType = "pool"
	// ObstacleTypeCrate is a wooden box
	ObstacleTypeCrate ObstacleType = "crate"
	// ObstacleTypeBarrel is a container
	ObstacleTypeBarrel ObstacleType = "barrel"
)

// TerrainType defines kinds of special terrain.
type TerrainType string

const (
	// TerrainTypeDifficult slows movement
	TerrainTypeDifficult TerrainType = "difficult"
	// TerrainTypeHazardous damages entities
	TerrainTypeHazardous TerrainType = "hazardous"
	// TerrainTypeWater is shallow water
	TerrainTypeWater TerrainType = "water"
	// TerrainTypeLava is molten rock
	TerrainTypeLava TerrainType = "lava"
	// TerrainTypeIce is slippery surface
	TerrainTypeIce TerrainType = "ice"
)

// ZoneType defines the purpose of a spawn zone.
type ZoneType string

const (
	// ZoneTypePlayerSpawn is where players enter
	ZoneTypePlayerSpawn ZoneType = "player_spawn"
	// ZoneTypeMonsterSpawn is where monsters spawn
	ZoneTypeMonsterSpawn ZoneType = "monster_spawn"
	// ZoneTypeBoss is for boss encounters
	ZoneTypeBoss ZoneType = "boss"
	// ZoneTypeEntrance is a room entrance
	ZoneTypeEntrance ZoneType = "entrance"
	// ZoneTypeExit is a room exit
	ZoneTypeExit ZoneType = "exit"
)
