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

// TestLoadFromData moved to actions/integration_test.go to avoid import cycle

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

func (s *BehaviorTestSuite) TestNewGoblinHasDefaultActions() {
	goblin := NewGoblin("goblin-1")

	// Goblin should have default scimitar action
	actions := goblin.Actions()
	s.Require().Len(actions, 1)
	s.Equal("scimitar", actions[0].GetID())
	s.Equal(TypeMeleeAttack, actions[0].ActionType())

	// Should have default speed
	s.Equal(30, goblin.Speed().Walk)
}

// TestActionRoundTrip moved to actions/integration_test.go to avoid import cycle

func (s *BehaviorTestSuite) TestTakeTurnMovesAndAttacks() {
	// Create a goblin at position (0, 0)
	goblin := NewGoblin("goblin-1")
	goblin.bus = s.bus

	// Enemy is 35 feet away (outside movement range but within speed+melee)
	perception := &PerceptionData{
		MyPosition: Position{X: 0, Y: 0},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockTarget{id: "fighter-1", name: "Fighter"},
				Position: Position{X: 35, Y: 0},
				Distance: 35, // 35 feet away
				Adjacent: false,
			},
		},
	}

	input := &TurnInput{
		Bus:           s.bus,
		ActionEconomy: combat.NewActionEconomy(),
		Perception:    perception,
		Roller:        dice.NewRoller(),
		Speed:         30, // Goblin speed
	}

	// Execute turn
	result, err := goblin.TakeTurn(s.ctx, input)

	s.Require().NoError(err)
	s.T().Logf("Turn result: MonsterID=%s", result.MonsterID)

	// Should have moved
	s.Require().Len(result.Movement, 2, "Expected start and end positions")
	s.T().Logf("Movement: from (%d,%d) to (%d,%d)",
		result.Movement[0].X, result.Movement[0].Y,
		result.Movement[1].X, result.Movement[1].Y)

	// Started at origin
	s.Equal(0, result.Movement[0].X)
	s.Equal(0, result.Movement[0].Y)

	// Moved toward enemy (should stop at 5ft = adjacent)
	// With 30ft speed and enemy at 35ft, we move 30ft to end up at 30ft (5ft away = adjacent)
	s.Equal(30, result.Movement[1].X)
	s.Equal(0, result.Movement[1].Y)

	// Perception should be updated - now adjacent
	s.True(perception.HasAdjacentEnemy(), "Should be adjacent after moving")
	s.T().Logf("After move: distance=%d, adjacent=%v",
		perception.Enemies[0].Distance, perception.Enemies[0].Adjacent)

	// Should have attacked (now that we're adjacent)
	s.Require().Len(result.Actions, 1, "Expected one attack action")
	s.Equal("scimitar", result.Actions[0].ActionID)
	s.Equal(TypeMeleeAttack, result.Actions[0].ActionType)
	s.Equal("fighter-1", result.Actions[0].TargetID)
	s.T().Logf("Attack: action=%s, target=%s, success=%v",
		result.Actions[0].ActionID, result.Actions[0].TargetID, result.Actions[0].Success)
}

func (s *BehaviorTestSuite) TestTakeTurnAlreadyAdjacent() {
	// Create a goblin already adjacent to enemy
	goblin := NewGoblin("goblin-1")
	goblin.bus = s.bus

	// Enemy is 5 feet away (already adjacent)
	perception := &PerceptionData{
		MyPosition: Position{X: 0, Y: 0},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockTarget{id: "fighter-1", name: "Fighter"},
				Position: Position{X: 5, Y: 0},
				Distance: 5,
				Adjacent: true,
			},
		},
	}

	input := &TurnInput{
		Bus:           s.bus,
		ActionEconomy: combat.NewActionEconomy(),
		Perception:    perception,
		Roller:        dice.NewRoller(),
		Speed:         30,
	}

	result, err := goblin.TakeTurn(s.ctx, input)

	s.Require().NoError(err)

	// Should NOT have moved (already adjacent)
	s.Len(result.Movement, 0, "Should not move when already adjacent")

	// Should have attacked
	s.Require().Len(result.Actions, 1)
	s.Equal("scimitar", result.Actions[0].ActionID)
}

func (s *BehaviorTestSuite) TestTakeTurnEnemyTooFar() {
	// Create a goblin with enemy very far away
	goblin := NewGoblin("goblin-1")
	goblin.bus = s.bus

	// Enemy is 100 feet away - can move toward but not attack
	perception := &PerceptionData{
		MyPosition: Position{X: 0, Y: 0},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockTarget{id: "fighter-1", name: "Fighter"},
				Position: Position{X: 100, Y: 0},
				Distance: 100,
				Adjacent: false,
			},
		},
	}

	input := &TurnInput{
		Bus:           s.bus,
		ActionEconomy: combat.NewActionEconomy(),
		Perception:    perception,
		Roller:        dice.NewRoller(),
		Speed:         30,
	}

	result, err := goblin.TakeTurn(s.ctx, input)

	s.Require().NoError(err)

	// Should have moved full speed toward enemy
	s.Require().Len(result.Movement, 2)
	s.Equal(30, result.Movement[1].X) // Moved 30 feet

	// Should NOT have attacked (still too far)
	s.Len(result.Actions, 0, "Should not attack when still too far")

	// Verify perception updated - still not adjacent
	s.False(perception.HasAdjacentEnemy())
	s.T().Logf("After move: distance=%d (still too far to attack)",
		perception.Enemies[0].Distance)
}
