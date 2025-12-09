package monster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type BehaviorTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestBehaviorSuite(t *testing.T) {
	suite.Run(t, new(BehaviorTestSuite))
}

func (s *BehaviorTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// mockTarget implements core.Entity for testing
type mockTarget struct {
	id   string
	name string
}

func (m *mockTarget) GetID() string            { return m.id }
func (m *mockTarget) GetType() core.EntityType { return "character" }
func (m *mockTarget) GetName() string          { return m.name }

func (s *BehaviorTestSuite) TestLoadFromData() {
	data := &Data{
		ID:           "goblin-1",
		Name:         "Goblin",
		MonsterType:  "goblin",
		HitPoints:    7,
		MaxHitPoints: 7,
		ArmorClass:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 8,
			abilities.CHA: 8,
		},
		Speed:  SpeedData{Walk: 30},
		Senses: SensesData{Darkvision: 60, PassivePerception: 9},
		Actions: []ActionData{
			{
				Ref:    *refs.MonsterActions.Scimitar(),
				Config: []byte(`{"attack_bonus": 4, "damage_dice": "1d6+2"}`),
			},
		},
		Proficiencies: []ProficiencyData{
			{Skill: "stealth", Bonus: 6},
		},
	}

	monster, err := LoadFromData(s.ctx, data, s.bus)

	s.Require().NoError(err)
	s.Require().NotNil(monster)
	s.Equal("goblin-1", monster.GetID())
	s.Equal("Goblin", monster.Name())
	s.Equal(7, monster.HP())
	s.Equal(7, monster.MaxHP())
	s.Equal(15, monster.AC())
	s.Equal(30, monster.Speed().Walk)
	s.Equal(60, monster.Senses().Darkvision)

	// Verify action was loaded
	actions := monster.GetActions()
	s.Require().Len(actions, 1)
	s.Equal("scimitar", actions[0].GetID())
}

func (s *BehaviorTestSuite) TestLoadFromDataNoBus() {
	data := &Data{
		ID:   "goblin-1",
		Name: "Goblin",
	}

	monster, err := LoadFromData(s.ctx, data, nil)

	s.Require().Error(err)
	s.Nil(monster)
	s.Contains(err.Error(), "event bus is required")
}

func (s *BehaviorTestSuite) TestScimitarActionScore() {
	scimitar := NewScimitarAction(ScimitarConfig{
		ID:          "scimitar",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
	})

	monster := NewGoblin("goblin-1")

	s.Run("higher score when adjacent", func() {
		perception := &PerceptionData{
			MyPosition: Position{X: 5, Y: 5},
			Enemies: []PerceivedEntity{
				{
					Entity:   &mockTarget{id: "target-1", name: "Fighter"},
					Position: Position{X: 5, Y: 6},
					Distance: 5,
					Adjacent: true,
				},
			},
		}

		score := scimitar.Score(monster, perception)
		s.Equal(70, score) // base 50 + 20 for adjacent
	})

	s.Run("base score when not adjacent", func() {
		perception := &PerceptionData{
			MyPosition: Position{X: 0, Y: 0},
			Enemies: []PerceivedEntity{
				{
					Entity:   &mockTarget{id: "target-1", name: "Fighter"},
					Position: Position{X: 6, Y: 0},
					Distance: 30,
					Adjacent: false,
				},
			},
		}

		score := scimitar.Score(monster, perception)
		s.Equal(50, score) // base only
	})
}

func (s *BehaviorTestSuite) TestScimitarActionCost() {
	scimitar := NewScimitarAction(ScimitarConfig{
		ID: "scimitar",
	})

	s.Equal(CostAction, scimitar.Cost())
	s.Equal(TypeMeleeAttack, scimitar.ActionType())
}

func (s *BehaviorTestSuite) TestTakeTurnSelectsAndExecutesAction() {
	// Create a goblin with a scimitar
	goblin := NewGoblin("goblin-1")
	scimitar := NewScimitarAction(ScimitarConfig{
		ID:          "scimitar",
		AttackBonus: 4,
	})
	goblin.AddAction(scimitar)
	goblin.bus = s.bus

	// Create perception with adjacent enemy
	perception := &PerceptionData{
		MyPosition: Position{X: 5, Y: 5},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockTarget{id: "target-1", name: "Fighter"},
				Position: Position{X: 5, Y: 6},
				Distance: 5,
				Adjacent: true,
			},
		},
	}

	// Create turn input
	input := &TurnInput{
		Bus:           s.bus,
		ActionEconomy: combat.NewActionEconomy(),
		Perception:    perception,
		Roller:        dice.NewRoller(), // Use real roller
	}

	// Execute turn
	result, err := goblin.TakeTurn(s.ctx, input)

	s.Require().NoError(err)
	s.Equal("goblin-1", result.MonsterID)
	s.Require().Len(result.Actions, 1)
	s.Equal("scimitar", result.Actions[0].ActionID)
	s.Equal(TypeMeleeAttack, result.Actions[0].ActionType)
	s.Equal("target-1", result.Actions[0].TargetID)
	s.True(result.Actions[0].Success)

	// Action should have consumed the action
	s.False(input.ActionEconomy.CanUseAction())
	s.True(input.ActionEconomy.CanUseBonusAction()) // Bonus still available
}

func (s *BehaviorTestSuite) TestTakeTurnNoEnemies() {
	goblin := NewGoblin("goblin-1")
	scimitar := NewScimitarAction(ScimitarConfig{ID: "scimitar"})
	goblin.AddAction(scimitar)
	goblin.bus = s.bus

	// No enemies
	perception := &PerceptionData{
		MyPosition: Position{X: 5, Y: 5},
		Enemies:    []PerceivedEntity{},
	}

	input := &TurnInput{
		Bus:           s.bus,
		ActionEconomy: combat.NewActionEconomy(),
		Perception:    perception,
		Roller:        dice.NewRoller(),
	}

	result, err := goblin.TakeTurn(s.ctx, input)

	s.Require().NoError(err)
	// No actions should be taken because there's no valid target
	s.Len(result.Actions, 0)
	// Action economy should be unchanged
	s.True(input.ActionEconomy.CanUseAction())
}

func (s *BehaviorTestSuite) TestTakeTurnExhaustsActions() {
	goblin := NewGoblin("goblin-1")
	scimitar := NewScimitarAction(ScimitarConfig{ID: "scimitar"})
	goblin.AddAction(scimitar)
	goblin.bus = s.bus

	// Adjacent enemy
	perception := &PerceptionData{
		MyPosition: Position{X: 5, Y: 5},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockTarget{id: "target-1", name: "Fighter"},
				Position: Position{X: 5, Y: 6},
				Distance: 5,
				Adjacent: true,
			},
		},
	}

	// Start with no actions available
	economy := combat.NewActionEconomy()
	_ = economy.UseAction()      // Use up the action
	_ = economy.UseBonusAction() // Use up bonus action

	input := &TurnInput{
		Bus:           s.bus,
		ActionEconomy: economy,
		Perception:    perception,
		Roller:        dice.NewRoller(),
	}

	result, err := goblin.TakeTurn(s.ctx, input)

	s.Require().NoError(err)
	// No actions because economy exhausted
	s.Len(result.Actions, 0)
}

func (s *BehaviorTestSuite) TestPerceptionHelpers() {
	s.Run("HasAdjacentEnemy", func() {
		perception := &PerceptionData{
			Enemies: []PerceivedEntity{
				{Adjacent: true},
			},
		}
		s.True(perception.HasAdjacentEnemy())

		perception2 := &PerceptionData{
			Enemies: []PerceivedEntity{
				{Adjacent: false},
			},
		}
		s.False(perception2.HasAdjacentEnemy())
	})

	s.Run("AdjacentEnemyCount", func() {
		perception := &PerceptionData{
			Enemies: []PerceivedEntity{
				{Adjacent: true},
				{Adjacent: false},
				{Adjacent: true},
			},
		}
		s.Equal(2, perception.AdjacentEnemyCount())
	})

	s.Run("ClosestEnemy", func() {
		enemy1 := &mockTarget{id: "enemy-1"}
		enemy2 := &mockTarget{id: "enemy-2"}

		perception := &PerceptionData{
			Enemies: []PerceivedEntity{
				{Entity: enemy1, Distance: 10},
				{Entity: enemy2, Distance: 20},
			},
		}
		closest := perception.ClosestEnemy()
		s.Require().NotNil(closest)
		s.Equal("enemy-1", closest.Entity.GetID())

		emptyPerception := &PerceptionData{Enemies: []PerceivedEntity{}}
		s.Nil(emptyPerception.ClosestEnemy())
	})
}

func (s *BehaviorTestSuite) TestHPPercent() {
	monster := New(Config{
		ID:   "test",
		Name: "Test",
		HP:   50,
		AC:   15,
	})

	s.Equal(100, monster.HPPercent())

	monster.TakeDamage(25)
	s.Equal(50, monster.HPPercent())

	monster.TakeDamage(25)
	s.Equal(0, monster.HPPercent())
}

func (s *BehaviorTestSuite) TestToData() {
	data := &Data{
		ID:           "goblin-1",
		Name:         "Goblin",
		MonsterType:  "goblin",
		HitPoints:    7,
		MaxHitPoints: 7,
		ArmorClass:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 8,
			abilities.CHA: 8,
		},
		Speed:  SpeedData{Walk: 30},
		Senses: SensesData{Darkvision: 60},
		Proficiencies: []ProficiencyData{
			{Skill: "stealth", Bonus: 6},
		},
	}

	monster, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Take some damage
	monster.TakeDamage(3)

	// Convert back to data
	outputData := monster.ToData()

	s.Equal("goblin-1", outputData.ID)
	s.Equal("Goblin", outputData.Name)
	s.Equal("goblin", outputData.MonsterType)
	s.Equal(4, outputData.HitPoints) // 7 - 3 = 4
	s.Equal(7, outputData.MaxHitPoints)
	s.Equal(15, outputData.ArmorClass)
	s.Equal(30, outputData.Speed.Walk)
	s.Equal(60, outputData.Senses.Darkvision)

	// Check proficiencies preserved
	s.Len(outputData.Proficiencies, 1)
	s.Equal("stealth", outputData.Proficiencies[0].Skill)
	s.Equal(6, outputData.Proficiencies[0].Bonus)
}

func (s *BehaviorTestSuite) TestCleanup() {
	data := &Data{
		ID:   "goblin-1",
		Name: "Goblin",
	}

	monster, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Cleanup should work without error
	err = monster.Cleanup(s.ctx)
	s.NoError(err)

	// Double cleanup should also be fine
	err = monster.Cleanup(s.ctx)
	s.NoError(err)
}
