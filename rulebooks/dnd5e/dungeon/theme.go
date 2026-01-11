// Package dungeon provides D&D 5e dungeon generation and runtime management.
package dungeon

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// WallMaterial is a typed constant for dungeon wall materials.
// It maps to proto enums for client-side texture selection.
type WallMaterial string

const (
	// WallMaterialStone represents stone walls, common in crypts and dungeons.
	WallMaterialStone WallMaterial = "stone"
	// WallMaterialRock represents rough rock walls, common in caves.
	WallMaterialRock WallMaterial = "rock"
	// WallMaterialWood represents wooden walls, common in lairs and hideouts.
	WallMaterialWood WallMaterial = "wood"
	// WallMaterialMetal represents metal walls, common in fortifications.
	WallMaterialMetal WallMaterial = "metal"
)

// MonsterRef identifies a monster for selection in dungeon generation.
// It pairs a reference to the monster with combat-relevant metadata.
type MonsterRef struct {
	// Ref is the reference to the monster in the rulebook.
	Ref *core.Ref
	// CR is the challenge rating for budget calculations.
	CR float64
	// Role is the tactical role in encounters (melee, ranged, support, boss).
	Role MonsterRole
}

// Theme configures dungeon generation with monster pools and visual flavor.
// Themes are configuration, not behavior - the generator reads theme config
// to drive generation decisions.
type Theme struct {
	// ID uniquely identifies this theme.
	ID string
	// Name is the human-readable theme name.
	Name string

	// MonsterPool provides weighted selection of regular monsters.
	MonsterPool selectables.SelectionTable[MonsterRef]
	// BossPool provides weighted selection of boss monsters.
	BossPool selectables.SelectionTable[MonsterRef]

	// RoomTables provides room generation parameters.
	RoomTables *environments.RoomTables

	// WallMaterial defines the visual appearance of walls.
	WallMaterial WallMaterial
	// ObstacleTypes lists the kinds of obstacles that can appear.
	ObstacleTypes []ObstacleType
	// TerrainTypes lists the kinds of special terrain that can appear.
	TerrainTypes []TerrainType
}

// monsterEntry is a helper for building monster selection tables.
type monsterEntry struct {
	ref    *core.Ref
	cr     float64
	role   MonsterRole
	weight int
}

// buildMonsterTable creates a SelectionTable from a list of monster entries.
func buildMonsterTable(entries []monsterEntry) selectables.SelectionTable[MonsterRef] {
	table := selectables.NewBasicTable[MonsterRef](selectables.BasicTableConfig{
		ID: "monster_pool",
	})

	for _, entry := range entries {
		monsterRef := MonsterRef{
			Ref:  entry.ref,
			CR:   entry.cr,
			Role: entry.role,
		}
		table.Add(monsterRef, entry.weight)
	}

	return table
}

// buildBossTable creates a SelectionTable for boss monsters.
func buildBossTable(entries []monsterEntry) selectables.SelectionTable[MonsterRef] {
	table := selectables.NewBasicTable[MonsterRef](selectables.BasicTableConfig{
		ID: "boss_pool",
	})

	for _, entry := range entries {
		monsterRef := MonsterRef{
			Ref:  entry.ref,
			CR:   entry.cr,
			Role: entry.role,
		}
		table.Add(monsterRef, entry.weight)
	}

	return table
}

// defaultRoomTables caches the default room tables to avoid recreation.
var defaultRoomTables = environments.GetDefaultRoomTables()

// ThemeCrypt is a predefined theme for ancient crypt dungeons.
// Features undead monsters in stone corridors.
var ThemeCrypt = Theme{
	ID:   "crypt",
	Name: "Ancient Crypt",
	MonsterPool: buildMonsterTable([]monsterEntry{
		{ref: refs.Monsters.Skeleton(), cr: 0.25, role: RoleMelee, weight: 40},
		{ref: refs.Monsters.Zombie(), cr: 0.25, role: RoleMelee, weight: 30},
		{ref: refs.Monsters.SkeletonArcher(), cr: 0.25, role: RoleRanged, weight: 20},
		{ref: refs.Monsters.Ghoul(), cr: 1.0, role: RoleMelee, weight: 10},
	}),
	BossPool: buildBossTable([]monsterEntry{
		{ref: refs.Monsters.SkeletonCaptain(), cr: 2.0, role: RoleBoss, weight: 100},
	}),
	RoomTables:   &defaultRoomTables,
	WallMaterial: WallMaterialStone,
	ObstacleTypes: []ObstacleType{
		ObstacleTypePillar,
		ObstacleTypeSarcophagus,
		ObstacleTypeAltar,
	},
	TerrainTypes: []TerrainType{
		TerrainTypeDifficult,
	},
}

// ThemeCave is a predefined theme for natural cave dungeons.
// Features beasts and natural hazards in rocky environments.
var ThemeCave = Theme{
	ID:   "cave",
	Name: "Natural Cave",
	MonsterPool: buildMonsterTable([]monsterEntry{
		{ref: refs.Monsters.GiantRat(), cr: 0.125, role: RoleMelee, weight: 40},
		{ref: refs.Monsters.GiantSpider(), cr: 0.5, role: RoleMelee, weight: 25},
		{ref: refs.Monsters.Wolf(), cr: 0.25, role: RoleMelee, weight: 25},
		{ref: refs.Monsters.GiantWolfSpider(), cr: 0.25, role: RoleRanged, weight: 10},
	}),
	BossPool: buildBossTable([]monsterEntry{
		{ref: refs.Monsters.BrownBear(), cr: 1.0, role: RoleBoss, weight: 100},
	}),
	RoomTables:   &defaultRoomTables,
	WallMaterial: WallMaterialRock,
	ObstacleTypes: []ObstacleType{
		ObstacleTypeBoulder,
		ObstacleTypeStalagmite,
		ObstacleTypePool,
	},
	TerrainTypes: []TerrainType{
		TerrainTypeDifficult,
		TerrainTypeWater,
	},
}

// ThemeBanditLair is a predefined theme for humanoid hideouts.
// Features bandits and thugs in wooden structures.
var ThemeBanditLair = Theme{
	ID:   "bandit_lair",
	Name: "Bandit Lair",
	MonsterPool: buildMonsterTable([]monsterEntry{
		{ref: refs.Monsters.Bandit(), cr: 0.125, role: RoleMelee, weight: 40},
		{ref: refs.Monsters.BanditArcher(), cr: 0.125, role: RoleRanged, weight: 25},
		{ref: refs.Monsters.Thug(), cr: 0.5, role: RoleMelee, weight: 20},
		{ref: refs.Monsters.Goblin(), cr: 0.25, role: RoleMelee, weight: 15},
	}),
	BossPool: buildBossTable([]monsterEntry{
		{ref: refs.Monsters.BanditCaptain(), cr: 2.0, role: RoleBoss, weight: 100},
	}),
	RoomTables:   &defaultRoomTables,
	WallMaterial: WallMaterialWood,
	ObstacleTypes: []ObstacleType{
		ObstacleTypeCrate,
		ObstacleTypeBarrel,
	},
	TerrainTypes: []TerrainType{
		TerrainTypeDifficult,
	},
}

// themeRegistry holds all predefined themes for lookup.
var themeRegistry = map[string]*Theme{
	ThemeCrypt.ID:      &ThemeCrypt,
	ThemeCave.ID:       &ThemeCave,
	ThemeBanditLair.ID: &ThemeBanditLair,
}

// GetTheme returns a predefined theme by ID.
// Returns nil if the theme is not found.
func GetTheme(id string) *Theme {
	return themeRegistry[id]
}

// GetThemeIDs returns the IDs of all available themes.
func GetThemeIDs() []string {
	ids := make([]string, 0, len(themeRegistry))
	for id := range themeRegistry {
		ids = append(ids, id)
	}
	return ids
}
