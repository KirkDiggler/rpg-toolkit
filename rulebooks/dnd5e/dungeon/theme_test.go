package dungeon_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/dungeon"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// ThemeTestSuite tests the theme system for dungeon generation.
type ThemeTestSuite struct {
	suite.Suite
}

func TestThemeSuite(t *testing.T) {
	suite.Run(t, new(ThemeTestSuite))
}

func (s *ThemeTestSuite) TestWallMaterialConstants() {
	s.Run("WallMaterial constants have correct values", func() {
		s.Assert().Equal(dungeon.WallMaterial("stone"), dungeon.WallMaterialStone)
		s.Assert().Equal(dungeon.WallMaterial("rock"), dungeon.WallMaterialRock)
		s.Assert().Equal(dungeon.WallMaterial("wood"), dungeon.WallMaterialWood)
		s.Assert().Equal(dungeon.WallMaterial("metal"), dungeon.WallMaterialMetal)
	})

	s.Run("WallMaterial constants are distinct", func() {
		materials := []dungeon.WallMaterial{
			dungeon.WallMaterialStone,
			dungeon.WallMaterialRock,
			dungeon.WallMaterialWood,
			dungeon.WallMaterialMetal,
		}

		seen := make(map[dungeon.WallMaterial]bool)
		for _, m := range materials {
			s.Assert().False(seen[m], "duplicate WallMaterial: %s", m)
			seen[m] = true
		}
	})
}

func (s *ThemeTestSuite) TestMonsterRefStructure() {
	s.Run("MonsterRef can be used with selectables", func() {
		// Create a simple table with MonsterRef values
		table := selectables.NewBasicTable[dungeon.MonsterRef](selectables.BasicTableConfig{
			ID: "test_monster_pool",
		})

		// Add a test MonsterRef
		testRef := dungeon.MonsterRef{
			Ref:  nil, // For this test, we don't need a real ref
			CR:   0.5,
			Role: dungeon.RoleMelee,
		}
		table.Add(testRef, 10)

		s.Assert().False(table.IsEmpty(), "table should not be empty after adding")
		s.Assert().Equal(1, table.Size(), "table should have one item")

		items := table.GetItems()
		s.Assert().Contains(items, testRef, "table should contain the added MonsterRef")
	})
}

func (s *ThemeTestSuite) TestThemeStructFields() {
	s.Run("ThemeCrypt is properly populated", func() {
		theme := dungeon.ThemeCrypt

		s.Assert().Equal("crypt", theme.ID)
		s.Assert().Equal("Ancient Crypt", theme.Name)
		s.Assert().NotNil(theme.MonsterPool)
		s.Assert().NotNil(theme.BossPool)
		s.Assert().NotNil(theme.RoomTables)
		s.Assert().Equal(dungeon.WallMaterialStone, theme.WallMaterial)
		s.Assert().NotEmpty(theme.ObstacleTypes)
		s.Assert().NotEmpty(theme.TerrainTypes)
	})

	s.Run("ThemeCave is properly populated", func() {
		theme := dungeon.ThemeCave

		s.Assert().Equal("cave", theme.ID)
		s.Assert().Equal("Natural Cave", theme.Name)
		s.Assert().NotNil(theme.MonsterPool)
		s.Assert().NotNil(theme.BossPool)
		s.Assert().NotNil(theme.RoomTables)
		s.Assert().Equal(dungeon.WallMaterialRock, theme.WallMaterial)
		s.Assert().NotEmpty(theme.ObstacleTypes)
		s.Assert().NotEmpty(theme.TerrainTypes)
	})

	s.Run("ThemeBanditLair is properly populated", func() {
		theme := dungeon.ThemeBanditLair

		s.Assert().Equal("bandit_lair", theme.ID)
		s.Assert().Equal("Bandit Lair", theme.Name)
		s.Assert().NotNil(theme.MonsterPool)
		s.Assert().NotNil(theme.BossPool)
		s.Assert().NotNil(theme.RoomTables)
		s.Assert().Equal(dungeon.WallMaterialWood, theme.WallMaterial)
		s.Assert().NotEmpty(theme.ObstacleTypes)
		s.Assert().NotEmpty(theme.TerrainTypes)
	})
}

func (s *ThemeTestSuite) TestPredefinedThemesExist() {
	s.Run("ThemeCrypt exists and is valid", func() {
		theme := dungeon.ThemeCrypt
		s.Assert().NotEmpty(theme.ID)
		s.Assert().False(theme.MonsterPool.IsEmpty())
		s.Assert().False(theme.BossPool.IsEmpty())
	})

	s.Run("ThemeCave exists and is valid", func() {
		theme := dungeon.ThemeCave
		s.Assert().NotEmpty(theme.ID)
		s.Assert().False(theme.MonsterPool.IsEmpty())
		s.Assert().False(theme.BossPool.IsEmpty())
	})

	s.Run("ThemeBanditLair exists and is valid", func() {
		theme := dungeon.ThemeBanditLair
		s.Assert().NotEmpty(theme.ID)
		s.Assert().False(theme.MonsterPool.IsEmpty())
		s.Assert().False(theme.BossPool.IsEmpty())
	})
}

func (s *ThemeTestSuite) TestMonsterPoolSelection() {
	s.Run("MonsterPool.Select returns valid MonsterRef", func() {
		theme := dungeon.ThemeCrypt
		ctx := selectables.NewBasicSelectionContext()

		monsterRef, err := theme.MonsterPool.Select(ctx)
		s.Require().NoError(err)
		s.Assert().NotNil(monsterRef.Ref, "selected monster should have a Ref")
		s.Assert().Greater(monsterRef.CR, 0.0, "selected monster should have positive CR")
		s.Assert().NotEmpty(string(monsterRef.Role), "selected monster should have a role")
	})

	s.Run("MonsterPool contains variety of roles", func() {
		theme := dungeon.ThemeCrypt
		ctx := selectables.NewBasicSelectionContext()

		// Select multiple times to get a sample
		roles := make(map[dungeon.MonsterRole]bool)
		for i := 0; i < 50; i++ {
			monsterRef, err := theme.MonsterPool.Select(ctx)
			s.Require().NoError(err)
			roles[monsterRef.Role] = true
		}

		// We should see at least melee role (most common)
		s.Assert().True(roles[dungeon.RoleMelee], "should have melee monsters")
	})
}

func (s *ThemeTestSuite) TestBossPoolSelection() {
	s.Run("BossPool.Select returns valid MonsterRef with RoleBoss", func() {
		theme := dungeon.ThemeCrypt
		ctx := selectables.NewBasicSelectionContext()

		bossRef, err := theme.BossPool.Select(ctx)
		s.Require().NoError(err)
		s.Assert().NotNil(bossRef.Ref, "selected boss should have a Ref")
		s.Assert().Greater(bossRef.CR, 0.0, "selected boss should have positive CR")
		s.Assert().Equal(dungeon.RoleBoss, bossRef.Role, "boss should have RoleBoss")
	})

	s.Run("all themes have bosses with RoleBoss", func() {
		themes := []dungeon.Theme{
			dungeon.ThemeCrypt,
			dungeon.ThemeCave,
			dungeon.ThemeBanditLair,
		}
		ctx := selectables.NewBasicSelectionContext()

		for _, theme := range themes {
			bossRef, err := theme.BossPool.Select(ctx)
			s.Require().NoError(err, "theme %s should have selectable boss", theme.ID)
			s.Assert().Equal(dungeon.RoleBoss, bossRef.Role,
				"theme %s boss should have RoleBoss", theme.ID)
		}
	})
}

func (s *ThemeTestSuite) TestGetTheme() {
	s.Run("returns correct theme by ID", func() {
		testCases := []struct {
			id           string
			expectedName string
		}{
			{"crypt", "Ancient Crypt"},
			{"cave", "Natural Cave"},
			{"bandit_lair", "Bandit Lair"},
		}

		for _, tc := range testCases {
			theme := dungeon.GetTheme(tc.id)
			s.Require().NotNil(theme, "GetTheme(%q) should return a theme", tc.id)
			s.Assert().Equal(tc.id, theme.ID)
			s.Assert().Equal(tc.expectedName, theme.Name)
		}
	})

	s.Run("returns nil for unknown ID", func() {
		theme := dungeon.GetTheme("unknown")
		s.Assert().Nil(theme, "GetTheme should return nil for unknown ID")

		theme = dungeon.GetTheme("")
		s.Assert().Nil(theme, "GetTheme should return nil for empty ID")

		theme = dungeon.GetTheme("CRYPT") // wrong case
		s.Assert().Nil(theme, "GetTheme should be case-sensitive")
	})
}

func (s *ThemeTestSuite) TestGetThemeIDs() {
	s.Run("returns all theme IDs", func() {
		ids := dungeon.GetThemeIDs()

		s.Assert().GreaterOrEqual(len(ids), 3, "should have at least 3 themes")
		s.Assert().Contains(ids, "crypt")
		s.Assert().Contains(ids, "cave")
		s.Assert().Contains(ids, "bandit_lair")
	})

	s.Run("all returned IDs are valid", func() {
		ids := dungeon.GetThemeIDs()

		for _, id := range ids {
			theme := dungeon.GetTheme(id)
			s.Assert().NotNil(theme, "GetTheme(%q) should return a valid theme", id)
		}
	})

	s.Run("no duplicate IDs", func() {
		ids := dungeon.GetThemeIDs()
		seen := make(map[string]bool)

		for _, id := range ids {
			s.Assert().False(seen[id], "duplicate theme ID: %s", id)
			seen[id] = true
		}
	})
}

func (s *ThemeTestSuite) TestThemeObstacleTypes() {
	s.Run("ThemeCrypt has appropriate obstacles", func() {
		theme := dungeon.ThemeCrypt
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypePillar)
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypeSarcophagus)
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypeAltar)
	})

	s.Run("ThemeCave has appropriate obstacles", func() {
		theme := dungeon.ThemeCave
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypeBoulder)
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypeStalagmite)
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypePool)
	})

	s.Run("ThemeBanditLair has appropriate obstacles", func() {
		theme := dungeon.ThemeBanditLair
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypeCrate)
		s.Assert().Contains(theme.ObstacleTypes, dungeon.ObstacleTypeBarrel)
	})
}

func (s *ThemeTestSuite) TestThemeTerrainTypes() {
	s.Run("all themes have difficult terrain", func() {
		themes := []dungeon.Theme{
			dungeon.ThemeCrypt,
			dungeon.ThemeCave,
			dungeon.ThemeBanditLair,
		}

		for _, theme := range themes {
			s.Assert().Contains(theme.TerrainTypes, dungeon.TerrainTypeDifficult,
				"theme %s should have difficult terrain", theme.ID)
		}
	})

	s.Run("ThemeCave has water terrain", func() {
		theme := dungeon.ThemeCave
		s.Assert().Contains(theme.TerrainTypes, dungeon.TerrainTypeWater)
	})
}
