package character_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

// MockTarget implements the Target interface for testing
type MockTarget struct {
	ac int
}

func (m *MockTarget) AC() int {
	return m.ac
}

type AttackTestSuite struct {
	suite.Suite
	ctx            context.Context
	ctrl           *gomock.Controller
	mockRoller     *mock_dice.MockRoller
	originalRoller dice.Roller
}

func (s *AttackTestSuite) SetupSuite() {
	s.ctx = context.Background()
	// Save original roller
	s.originalRoller = dice.DefaultRoller
}

func (s *AttackTestSuite) SetupTest() {
	// Create new controller for each test
	s.ctrl = gomock.NewController(s.T())
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	dice.SetDefaultRoller(s.mockRoller)
}

func (s *AttackTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *AttackTestSuite) TearDownSuite() {
	// Restore original roller
	dice.SetDefaultRoller(s.originalRoller)
}

func (s *AttackTestSuite) TestSimpleAttackHit() {
	// Setup mock roller expectations
	// D20() uses RollN(1, 20), D8() uses RollN(1, 8)
	s.mockRoller.EXPECT().RollN(1, 20).Return([]int{15}, nil)
	s.mockRoller.EXPECT().RollN(1, 8).Return([]int{6}, nil)
	
	// Create test character using creation data
	abilityScores := shared.AbilityScores{
		constants.STR: 16, // +3 modifier
		constants.DEX: 10,
		constants.CON: 14,
		constants.INT: 10,
		constants.WIS: 12,
		constants.CHA: 8,
	}
	
	creationData := character.CreationData{
		ID:       "test-fighter",
		PlayerID: "player1",
		Name:     "Test Fighter",
		RaceData: &race.Data{
			ID:   "human",
			Name: "Human",
			Size: "Medium",
			Speed: 30,
			AbilityScoreIncreases: map[constants.Ability]int{},
		},
		ClassData: &class.Data{
			ID:      "fighter",
			Name:    "Fighter",
			HitDice: 10,
		},
		BackgroundData: &shared.Background{
			ID:   "soldier",
			Name: "Soldier",
		},
		AbilityScores: abilityScores,
	}
	
	char, err := character.NewFromCreationData(creationData)
	s.Require().NoError(err)
	
	// Create target with AC 10
	target := &MockTarget{ac: 10}
	
	// Create a simple weapon
	weapon := character.Weapon{
		Name:   "Longsword",
		Damage: "1d8",
	}
	
	// Perform attack (character has no event bus from NewFromCreationData)
	result := char.Attack(s.ctx, weapon, target)
	
	// Attack roll: 15 (roll) + 3 (STR) + 2 (prof) = 20 vs AC 10 = hit
	s.True(result.Hit, "Attack should hit")
	
	// Damage: 6 (roll) + 3 (STR) = 9
	s.Equal(9, result.Damage, "Damage should be roll + STR modifier")
}

func (s *AttackTestSuite) TestAttackMiss() {
	// Setup mock roller with low attack roll
	s.mockRoller.EXPECT().RollN(1, 20).Return([]int{2}, nil) // Low roll, no damage roll expected
	
	abilityScores := shared.AbilityScores{
		constants.STR: 10, // +0 modifier
		constants.DEX: 10,
		constants.CON: 10,
		constants.INT: 10,
		constants.WIS: 10,
		constants.CHA: 10,
	}
	
	creationData := character.CreationData{
		ID:       "test-fighter-2",
		PlayerID: "player1",
		Name:     "Test Fighter",
		RaceData: &race.Data{
			ID:   "human",
			Name: "Human",
			Size: "Medium",
			Speed: 30,
			AbilityScoreIncreases: map[constants.Ability]int{},
		},
		ClassData: &class.Data{
			ID:      "fighter",
			Name:    "Fighter",
			HitDice: 10,
		},
		BackgroundData: &shared.Background{
			ID:   "soldier",
			Name: "Soldier",
		},
		AbilityScores: abilityScores,
	}
	
	char, err := character.NewFromCreationData(creationData)
	s.Require().NoError(err)
	
	// Create target with high AC
	target := &MockTarget{ac: 15}
	
	weapon := character.Weapon{
		Name:   "Dagger",
		Damage: "1d4",
	}
	
	result := char.Attack(s.ctx, weapon, target)
	
	// Attack roll: 2 (roll) + 0 (STR) + 2 (prof) = 4 vs AC 15 = miss
	s.False(result.Hit, "Attack should miss")
	s.Equal(0, result.Damage, "No damage on miss")
}

func (s *AttackTestSuite) TestAttackWithBless() {
	// Setup mock roller expectations
	// Attack roll: 10, Damage: 5
	s.mockRoller.EXPECT().RollN(1, 20).Return([]int{10}, nil)
	s.mockRoller.EXPECT().RollN(1, 8).Return([]int{5}, nil)
	
	abilityScores := shared.AbilityScores{
		constants.STR: 14, // +2 modifier
		constants.DEX: 10,
		constants.CON: 10,
		constants.INT: 10,
		constants.WIS: 10,
		constants.CHA: 10,
	}
	
	creationData := character.CreationData{
		ID:       "blessed-fighter",
		PlayerID: "player1",
		Name:     "Blessed Fighter",
		RaceData: &race.Data{
			ID:   "human",
			Name: "Human",
			Size: "Medium",
			Speed: 30,
			AbilityScoreIncreases: map[constants.Ability]int{},
		},
		ClassData: &class.Data{
			ID:      "fighter",
			Name:    "Fighter",
			HitDice: 10,
		},
		BackgroundData: &shared.Background{
			ID:   "soldier",
			Name: "Soldier",
		},
		AbilityScores: abilityScores,
	}
	
	char, err := character.NewFromCreationData(creationData)
	s.Require().NoError(err)
	
	// Create event bus and subscribe to add bless modifier
	bus := events.NewBus()
	bus.SubscribeFunc("before_attack_roll", 50, func(_ context.Context, e events.Event) error {
		// Only add bless if this character is attacking
		if e.Source().GetID() == char.GetID() {
			// Add a fixed bless bonus for test (simulating 1d4 rolled as 3)
			e.Context().AddModifier(events.NewModifier(
				"bless",
				events.ModifierAttackBonus,
				events.NewRawValue(3, "bless bonus"), // Use RawValue for fixed int
				100,
			))
		}
		return nil
	})
	
	// Set the event bus on the character (simulating LoadFromContext)
	char.SetEventBus(bus)
	
	target := &MockTarget{ac: 14}
	weapon := character.Weapon{
		Name:   "Mace",
		Damage: "1d6",
	}
	
	result := char.Attack(s.ctx, weapon, target)
	
	// Attack: 10 (roll) + 2 (STR) + 2 (prof) + 3 (bless) = 17 vs AC 14 = hit
	s.True(result.Hit, "Attack with bless should hit")
	
	// Damage: 5 (roll) + 2 (STR) = 7
	s.Equal(7, result.Damage, "Damage calculation")
}

func TestAttackTestSuite(t *testing.T) {
	suite.Run(t, new(AttackTestSuite))
}