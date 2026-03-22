package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

// ActionEconomyTestSuite tests the action economy types and persistence
type ActionEconomyTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

// SetupTest runs before each test function
func (s *ActionEconomyTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// TestActionEconomyTestSuite runs the test suite
func TestActionEconomyTestSuite(t *testing.T) {
	suite.Run(t, new(ActionEconomyTestSuite))
}

func (s *ActionEconomyTestSuite) TestInCombat_NilActionEconomy() {
	char := &Character{}
	s.False(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestInCombat_WithActionEconomy() {
	char := &Character{
		actionEconomy: &ActionEconomyData{
			ActionsRemaining:      1,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
		},
	}
	s.True(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestExitCombat() {
	char := &Character{
		actionEconomy: &ActionEconomyData{
			ActionsRemaining: 1,
		},
	}

	_, err := char.ExitCombat(s.ctx, &ExitCombatInput{})
	s.Require().NoError(err)
	s.False(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestToData_NilActionEconomyOmitted() {
	char := &Character{
		id:           "test-char",
		name:         "Test",
		level:        1,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
	}
	char.actionEconomy = nil

	data := char.ToData()
	s.Nil(data.ActionEconomy)

	// Verify it marshals without the field
	bytes, err := json.Marshal(data)
	s.Require().NoError(err)
	s.NotContains(string(bytes), "action_economy")
}

func (s *ActionEconomyTestSuite) TestToData_IncludesActionEconomy() {
	char := &Character{
		id:           "test-char",
		name:         "Test",
		level:        1,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
	}
	char.actionEconomy = &ActionEconomyData{
		ActionsRemaining:      1,
		BonusActionsRemaining: 0,
		ReactionsRemaining:    1,
		MovementRemaining:     15,
		Granted: map[GrantedActionKey]int{
			GrantedAttacks: 1,
		},
	}

	data := char.ToData()
	s.Require().NotNil(data.ActionEconomy)
	s.Equal(1, data.ActionEconomy.ActionsRemaining)
	s.Equal(0, data.ActionEconomy.BonusActionsRemaining)
	s.Equal(15, data.ActionEconomy.MovementRemaining)
	s.Equal(1, data.ActionEconomy.Granted[GrantedAttacks])
}

func (s *ActionEconomyTestSuite) TestLoadFromData_RoundTrip() {
	// Create minimal valid Data with action economy
	data := &Data{
		ID:               "test-char",
		PlayerID:         "player-1",
		Name:             "Test Fighter",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
		Skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		SavingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		ActionEconomy: &ActionEconomyData{
			ActionsRemaining:      0,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
			MovementRemaining:     15,
			Granted: map[GrantedActionKey]int{
				GrantedAttacks: 2,
			},
		},
	}

	// Load from data
	loaded, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	// Verify action economy was restored
	s.True(loaded.InCombat())

	// Round-trip through ToData
	roundTripped := loaded.ToData()
	s.Require().NotNil(roundTripped.ActionEconomy)
	s.Equal(0, roundTripped.ActionEconomy.ActionsRemaining)
	s.Equal(1, roundTripped.ActionEconomy.BonusActionsRemaining)
	s.Equal(1, roundTripped.ActionEconomy.ReactionsRemaining)
	s.Equal(15, roundTripped.ActionEconomy.MovementRemaining)
	s.Equal(2, roundTripped.ActionEconomy.Granted[GrantedAttacks])
}

func (s *ActionEconomyTestSuite) TestLoadFromData_NilActionEconomy() {
	// Create minimal valid Data without action economy
	data := &Data{
		ID:               "test-char",
		PlayerID:         "player-1",
		Name:             "Test Fighter",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
		Skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		SavingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
	}

	// Load from data
	loaded, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	// Verify not in combat
	s.False(loaded.InCombat())
}
