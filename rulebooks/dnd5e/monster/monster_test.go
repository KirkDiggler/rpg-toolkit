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

func (s *MonsterTestSuite) TestGetSavingThrowModifier() {
	monster := New(Config{
		ID:   "test-monster-1",
		Name: "Test Monster",
		HP:   10,
		AC:   10,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.CON: 10,
			abilities.INT: 3,
			abilities.WIS: 8,
		},
	})

	tests := map[string]struct {
		ability abilities.Ability
		want    int
	}{
		"positive modifier":  {ability: abilities.STR, want: 3},
		"zero modifier":      {ability: abilities.CON, want: 0},
		"negative modifier":  {ability: abilities.WIS, want: -1},
		"odd score below 10": {ability: abilities.INT, want: -4},
	}

	for name, test := range tests {
		s.Run(name, func() {
			s.Equal(test.want, monster.GetSavingThrowModifier(test.ability))
		})
	}
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
