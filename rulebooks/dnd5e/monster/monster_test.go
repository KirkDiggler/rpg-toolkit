package monster

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
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

func (s *MonsterTestSuite) TestHeal() {
	monster := New(Config{
		ID:   "test-1",
		Name: "Test",
		HP:   20,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
		},
	})

	// Damage first
	monster.TakeDamage(10)
	s.Equal(10, monster.HP())

	s.Run("normal healing", func() {
		healed := monster.Heal(5)
		s.Equal(5, healed, "should return actual HP restored")
		s.Equal(15, monster.HP(), "HP should increase")
	})

	s.Run("overheal", func() {
		healed := monster.Heal(100)
		s.Equal(5, healed, "should only heal up to max HP")
		s.Equal(20, monster.HP(), "HP should be at max")
	})

	s.Run("negative healing", func() {
		healed := monster.Heal(-5)
		s.Equal(0, healed, "negative healing should be treated as 0")
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

func (s *MonsterTestSuite) TestAbilityScores() {
	scores := shared.AbilityScores{
		abilities.STR: 16, // +3
		abilities.DEX: 14, // +2
		abilities.CON: 12, // +1
		abilities.INT: 10, // +0
		abilities.WIS: 8,  // -1
		abilities.CHA: 6,  // -2
	}

	monster := New(Config{
		ID:            "test-1",
		Name:          "Test",
		HP:            10,
		AC:            15,
		AbilityScores: scores,
	})

	result := monster.AbilityScores()
	s.Equal(3, result.Modifier(abilities.STR))
	s.Equal(2, result.Modifier(abilities.DEX))
	s.Equal(1, result.Modifier(abilities.CON))
	s.Equal(0, result.Modifier(abilities.INT))
	s.Equal(-1, result.Modifier(abilities.WIS))
	s.Equal(-2, result.Modifier(abilities.CHA))
}
