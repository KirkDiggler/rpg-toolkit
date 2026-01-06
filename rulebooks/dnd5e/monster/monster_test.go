package monster

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type MonsterTestSuite struct {
	suite.Suite
}

func TestMonsterSuite(t *testing.T) {
	suite.Run(t, new(MonsterTestSuite))
}

func (s *MonsterTestSuite) TestNew() {
	config := Config{
		ID:   "test-monster-1",
		Name: "Test Monster",
		HP:   50,
		AC:   16,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 12,
			abilities.CON: 14,
			abilities.INT: 8,
			abilities.WIS: 10,
			abilities.CHA: 6,
		},
	}

	monster := New(config)

	s.Require().NotNil(monster)
	s.Equal("test-monster-1", monster.GetID())
	s.Equal(dnd5e.EntityTypeMonster, monster.GetType())
	s.Equal("Test Monster", monster.Name())
	s.Equal(50, monster.HP())
	s.Equal(50, monster.MaxHP())
	s.Equal(16, monster.AC())
	s.True(monster.IsAlive())
}

func (s *MonsterTestSuite) TestNewGoblin() {
	goblin := NewGoblin("goblin-1")

	s.Require().NotNil(goblin)
	s.Equal("goblin-1", goblin.GetID())
	s.Equal(dnd5e.EntityTypeMonster, goblin.GetType())
	s.Equal("Goblin", goblin.Name())
	s.Equal(7, goblin.HP())
	s.Equal(7, goblin.MaxHP())
	s.Equal(15, goblin.AC())

	// Check ability scores
	scores := goblin.AbilityScores()
	s.Equal(8, scores[abilities.STR])
	s.Equal(14, scores[abilities.DEX])
	s.Equal(10, scores[abilities.CON])
	s.Equal(10, scores[abilities.INT])
	s.Equal(8, scores[abilities.WIS])
	s.Equal(8, scores[abilities.CHA])

	// Verify DEX modifier is +2
	s.Equal(2, scores.Modifier(abilities.DEX))
}

func (s *MonsterTestSuite) TestTakeDamage() {
	monster := New(Config{
		ID:   "test-1",
		Name: "Test",
		HP:   20,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
		},
	})

	s.Run("normal damage", func() {
		damage := monster.TakeDamage(5)
		s.Equal(5, damage, "should return actual damage taken")
		s.Equal(15, monster.HP(), "HP should decrease")
		s.True(monster.IsAlive())
	})

	s.Run("overkill damage", func() {
		damage := monster.TakeDamage(100)
		s.Equal(15, damage, "should only deal remaining HP as damage")
		s.Equal(0, monster.HP(), "HP should be 0")
		s.False(monster.IsAlive(), "monster should be dead")
	})

	s.Run("negative damage", func() {
		monster := New(Config{
			ID:   "test-2",
			Name: "Test",
			HP:   20,
			AC:   15,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 10,
			},
		})

		damage := monster.TakeDamage(-5)
		s.Equal(0, damage, "negative damage should be treated as 0")
		s.Equal(20, monster.HP(), "HP should not change")
	})
}

func (s *MonsterTestSuite) TestIsAlive() {
	monster := New(Config{
		ID:   "test-1",
		Name: "Test",
		HP:   10,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
		},
	})

	s.True(monster.IsAlive(), "monster should be alive at full HP")

	monster.TakeDamage(5)
	s.True(monster.IsAlive(), "monster should be alive with partial HP")

	monster.TakeDamage(5)
	s.False(monster.IsAlive(), "monster should be dead at 0 HP")
}

// TestMoveTowardEnemy_AroundObstacle verifies that monsters use A* pathfinding
// to navigate around obstacles using BlockedHexes from PerceptionData.
func (s *MonsterTestSuite) TestMoveTowardEnemy_AroundObstacle() {
	monster := NewGoblin("goblin-1")

	// Monster at (0,0,0), enemy at (3,-3,0)
	// Direct path would be: (1,-1,0) -> (2,-2,0) -> (3,-3,0)
	// Block (1,-1,0) and (2,-2,0) to force alternate route
	enemyPos := spatial.CubeCoordinate{X: 3, Y: -3, Z: 0}
	perception := &PerceptionData{
		MyPosition: spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockMovementTestEntity{id: "enemy-1"},
				Position: enemyPos,
				Distance: 3,
				Adjacent: false,
			},
		},
		BlockedHexes: []spatial.CubeCoordinate{
			{X: 1, Y: -1, Z: 0},
			{X: 2, Y: -2, Z: 0},
		},
	}

	input := &TurnInput{
		Perception: perception,
		Speed:      6, // 30ft = 6 hexes
	}
	result := &TurnResult{
		Movement: make([]spatial.CubeCoordinate, 0),
	}

	monster.moveTowardEnemy(input, result)

	s.Require().NotEmpty(result.Movement, "monster should move")

	// Verify path doesn't include blocked hexes
	blocked := make(map[spatial.CubeCoordinate]bool)
	for _, hex := range perception.BlockedHexes {
		blocked[hex] = true
	}
	for _, pos := range result.Movement {
		s.Falsef(blocked[pos], "movement should not include blocked hex %v", pos)
	}

	// Final position should be adjacent to enemy (distance 1)
	finalPos := result.Movement[len(result.Movement)-1]
	s.Equal(1, finalPos.Distance(enemyPos), "should end up 1 hex away from enemy")

	// Verify perception was updated
	s.True(perception.Enemies[0].Adjacent, "enemy should be marked adjacent after move")
}

// TestMoveTowardEnemy_TrappedStaysPut verifies that monsters stay put when
// completely surrounded by obstacles and cannot find a path to the target.
func (s *MonsterTestSuite) TestMoveTowardEnemy_TrappedStaysPut() {
	monster := NewGoblin("goblin-1")

	// Monster at (0,0,0), completely surrounded by walls
	startPos := spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}
	perception := &PerceptionData{
		MyPosition: startPos,
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockMovementTestEntity{id: "enemy-1"},
				Position: spatial.CubeCoordinate{X: 5, Y: -5, Z: 0},
				Distance: 5,
				Adjacent: false,
			},
		},
		// Block all 6 neighbors
		BlockedHexes: []spatial.CubeCoordinate{
			{X: 1, Y: -1, Z: 0},
			{X: 1, Y: 0, Z: -1},
			{X: 0, Y: 1, Z: -1},
			{X: -1, Y: 1, Z: 0},
			{X: -1, Y: 0, Z: 1},
			{X: 0, Y: -1, Z: 1},
		},
	}

	input := &TurnInput{
		Perception: perception,
		Speed:      6,
	}
	result := &TurnResult{
		Movement: make([]spatial.CubeCoordinate, 0),
	}

	monster.moveTowardEnemy(input, result)

	// Should not move at all
	s.Empty(result.Movement, "trapped monster should not move")

	// Position should remain unchanged
	s.Equal(startPos, perception.MyPosition, "position should not change")
}

// mockMovementTestEntity implements core.Entity for movement integration tests
type mockMovementTestEntity struct {
	id string
}

func (m *mockMovementTestEntity) GetID() string {
	return m.id
}

func (m *mockMovementTestEntity) GetType() core.EntityType {
	return dnd5e.EntityTypeCharacter
}
