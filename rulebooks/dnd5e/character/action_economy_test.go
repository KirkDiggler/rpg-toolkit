package character

import (
	"context"
	"encoding/json"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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

// --- Test helper functions ---

// createTestFighterCharacter creates a Fighter with standard combat abilities and SecondWind.
func createTestFighterCharacter(t *testing.T, bus events.EventBus) *Character {
	t.Helper()

	// Load SecondWind feature from JSON
	swData := json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"second_wind"},"id":"second-wind-1","name":"Second Wind","level":3,"character_id":"fighter-1","uses":1,"max_uses":1}`)
	sw, err := features.LoadJSON(swData)
	if err != nil {
		t.Fatalf("failed to load second wind: %v", err)
	}

	return &Character{
		id:               "fighter-1",
		name:             "Test Fighter",
		level:            3,
		proficiencyBonus: 2,
		classID:          classes.Fighter,
		raceID:           races.Human,
		abilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		hitPoints:    28,
		maxHitPoints: 28,
		armorClass:   18,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		resources:    make(map[coreResources.ResourceKey]*combat.RecoverableResource),
		combatAbilities: []combatabilities.CombatAbility{
			combatabilities.NewAttack("attack-1"),
			combatabilities.NewDash("dash-1"),
			combatabilities.NewDodge("dodge-1"),
			combatabilities.NewDisengage("disengage-1"),
		},
		features: []features.Feature{sw},
		bus:      bus,
	}
}

// createTestBarbarianCharacter creates a Barbarian with Rage feature and rage charges.
func createTestBarbarianCharacter(t *testing.T, bus events.EventBus) *Character {
	t.Helper()

	// Load Rage feature from JSON
	rageData := json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"rage"},"id":"rage-1","name":"Rage","level":3}`)
	rage, err := features.LoadJSON(rageData)
	if err != nil {
		t.Fatalf("failed to load rage: %v", err)
	}

	// Create rage charges resource
	rageCharges := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "rage_charges",
		Maximum:     3, // Level 3 barbarian gets 3
		CharacterID: "barbarian-1",
		ResetType:   coreResources.ResetLongRest,
	})

	char := &Character{
		id:               "barbarian-1",
		name:             "Test Barbarian",
		level:            3,
		proficiencyBonus: 2,
		classID:          classes.Barbarian,
		raceID:           races.Human,
		abilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		hitPoints:    32,
		maxHitPoints: 32,
		armorClass:   14,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		resources:    make(map[coreResources.ResourceKey]*combat.RecoverableResource),
		combatAbilities: []combatabilities.CombatAbility{
			combatabilities.NewAttack("attack-1"),
			combatabilities.NewDash("dash-1"),
			combatabilities.NewDodge("dodge-1"),
			combatabilities.NewDisengage("disengage-1"),
		},
		features: []features.Feature{rage},
		bus:      bus,
	}

	char.resources[resources.RageCharges] = rageCharges

	return char
}

// createTestMonkCharacter creates a Monk with appropriate class ID.
func createTestMonkCharacter(t *testing.T, bus events.EventBus) *Character {
	t.Helper()

	return &Character{
		id:               "monk-1",
		name:             "Test Monk",
		level:            3,
		proficiencyBonus: 2,
		classID:          classes.Monk,
		raceID:           races.Human,
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 16,
			abilities.CHA: 8,
		},
		hitPoints:    24,
		maxHitPoints: 24,
		armorClass:   16,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		resources:    make(map[coreResources.ResourceKey]*combat.RecoverableResource),
		combatAbilities: []combatabilities.CombatAbility{
			combatabilities.NewAttack("attack-1"),
			combatabilities.NewDash("dash-1"),
			combatabilities.NewDodge("dodge-1"),
			combatabilities.NewDisengage("disengage-1"),
		},
		features: []features.Feature{},
		bus:      bus,
	}
}

// createTWFCharacter creates a character with two light weapons equipped.
func createTWFCharacter(t *testing.T, bus events.EventBus) *Character {
	t.Helper()

	shortsword := weapons.All[weapons.Shortsword]
	dagger := weapons.All[weapons.Dagger]

	char := &Character{
		id:               "twf-1",
		name:             "Test TWF Fighter",
		level:            3,
		proficiencyBonus: 2,
		classID:          classes.Fighter,
		raceID:           races.Human,
		abilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		hitPoints:    28,
		maxHitPoints: 28,
		armorClass:   16,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		resources:    make(map[coreResources.ResourceKey]*combat.RecoverableResource),
		inventory: []InventoryItem{
			{Equipment: &shortsword, Quantity: 1},
			{Equipment: &dagger, Quantity: 1},
		},
		equipmentSlots: EquipmentSlots{
			SlotMainHand: weapons.Shortsword,
			SlotOffHand:  weapons.Dagger,
		},
		combatAbilities: []combatabilities.CombatAbility{
			combatabilities.NewAttack("attack-1"),
			combatabilities.NewDash("dash-1"),
			combatabilities.NewDodge("dodge-1"),
			combatabilities.NewDisengage("disengage-1"),
		},
		features: []features.Feature{},
		bus:      bus,
	}

	return char
}

// --- Persistence tests (existing) ---

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

// --- Task 4: StartTurn, EndTurn, AvailableAbilities, AvailableActions ---

func (s *ActionEconomyTestSuite) TestStartTurn() {
	char := createTestFighterCharacter(s.T(), s.bus)

	output, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Verify action economy initialized
	s.True(char.InCombat())
	s.Equal(1, char.actionEconomy.ActionsRemaining)
	s.Equal(1, char.actionEconomy.BonusActionsRemaining)
	s.Equal(1, char.actionEconomy.ReactionsRemaining)
	s.Equal(30, char.actionEconomy.MovementRemaining)

	// Verify abilities returned
	s.NotEmpty(output.Abilities)

	// Verify actions returned (at least Move should be there)
	s.NotEmpty(output.Actions)

	// Verify Move action is listed and usable
	var foundMove bool
	for _, a := range output.Actions {
		if a.Ref.ID == refs.Actions.Move().ID {
			foundMove = true
			s.True(a.CanUse)
		}
	}
	s.True(foundMove, "Move action should be listed")
}

func (s *ActionEconomyTestSuite) TestStartTurn_ResetsFromPreviousTurn() {
	char := createTestFighterCharacter(s.T(), s.bus)

	// Start first turn and consume resources
	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)
	char.actionEconomy.ActionsRemaining = 0
	char.actionEconomy.BonusActionsRemaining = 0
	char.actionEconomy.Granted[GrantedAttacks] = 2

	// Start second turn
	output, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Verify fresh resources
	s.Equal(1, char.actionEconomy.ActionsRemaining)
	s.Equal(1, char.actionEconomy.BonusActionsRemaining)
	s.Equal(1, char.actionEconomy.ReactionsRemaining)
	s.Equal(30, char.actionEconomy.MovementRemaining)
	s.Equal(0, char.actionEconomy.Granted[GrantedAttacks], "granted should be cleared")
	s.NotEmpty(output.Abilities)
}

func (s *ActionEconomyTestSuite) TestEndTurn_ResetsButStaysInCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	// Start turn
	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Grant some capacity
	char.actionEconomy.Granted[GrantedAttacks] = 1

	// End turn
	_, err = char.EndTurn(s.ctx, &EndTurnInput{})
	s.Require().NoError(err)

	// Verify resources zeroed
	s.Equal(0, char.actionEconomy.ActionsRemaining)
	s.Equal(0, char.actionEconomy.BonusActionsRemaining)
	s.Equal(0, char.actionEconomy.ReactionsRemaining)
	s.Equal(0, char.actionEconomy.MovementRemaining)
	s.Equal(0, char.actionEconomy.Granted[GrantedAttacks], "granted should be cleared")

	// But still in combat
	s.True(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestAvailableAbilities_OutsideCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	abilities := char.AvailableAbilities()
	s.Empty(abilities)
}

func (s *ActionEconomyTestSuite) TestAvailableActions_OutsideCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	actions := char.AvailableActions()
	s.Empty(actions)
}

func (s *ActionEconomyTestSuite) TestAvailableAbilities_InCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	avail := char.AvailableAbilities()

	// Should have combat abilities (Attack, Dash, Dodge, Disengage) + SecondWind feature
	s.GreaterOrEqual(len(avail), 5)

	// Verify Attack is usable
	var foundAttack bool
	for _, a := range avail {
		if a.Ref.ID == refs.CombatAbilities.Attack().ID {
			foundAttack = true
			s.True(a.CanUse)
			s.Empty(a.Reason)
		}
	}
	s.True(foundAttack, "Attack ability should be listed")
}

// --- Task 5: ActivateAbility ---

func (s *ActionEconomyTestSuite) TestActivateAbility_Attack() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)
	s.True(output.Success)
	s.Empty(output.Error)
	s.Equal("1 attack", output.GrantedCapacity)

	// Action should be consumed
	s.Equal(0, char.actionEconomy.ActionsRemaining)

	// Attacks should be granted
	s.Equal(1, char.actionEconomy.Granted[GrantedAttacks])

	// Strike should now appear in available actions
	var foundStrike bool
	for _, a := range output.Actions {
		if a.Ref.ID == refs.Actions.Strike().ID {
			foundStrike = true
			s.True(a.CanUse)
		}
	}
	s.True(foundStrike, "Strike action should be available after Attack")
}

func (s *ActionEconomyTestSuite) TestActivateAbility_Dash() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Dash(),
	})
	s.Require().NoError(err)
	s.True(output.Success)

	// Movement should be doubled
	s.Equal(60, char.actionEconomy.MovementRemaining)
}

func (s *ActionEconomyTestSuite) TestActivateAbility_NoActionRemaining() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Use the action on Attack
	_, err = char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)

	// Try to Dash (also needs action) - should fail
	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Dash(),
	})
	s.Require().NoError(err)
	s.False(output.Success)
	s.NotEmpty(output.Error)
}

func (s *ActionEconomyTestSuite) TestActivateAbility_NotInCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)
	s.False(output.Success)
	s.Equal("not in combat", output.Error)
}

func (s *ActionEconomyTestSuite) TestActivateAbility_Rage() {
	char := createTestBarbarianCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.Rage(),
	})
	s.Require().NoError(err)
	s.True(output.Success)

	// Bonus action should be consumed
	s.Equal(0, char.actionEconomy.BonusActionsRemaining)

	// Rage charge should be consumed
	s.Equal(2, char.GetResource(resources.RageCharges).Current())
}

func (s *ActionEconomyTestSuite) TestActivateAbility_SecondWind() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.SecondWind(),
	})
	s.Require().NoError(err)
	s.True(output.Success)

	// Bonus action should be consumed
	s.Equal(0, char.actionEconomy.BonusActionsRemaining)
}

func (s *ActionEconomyTestSuite) TestActivateAbility_UnknownAbility() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	output, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.FlurryOfBlows(), // Not on this character
	})
	s.Require().NoError(err)
	s.False(output.Success)
	s.Equal("unknown ability", output.Error)
}

// --- Task 6: ExecuteAction ---

func (s *ActionEconomyTestSuite) TestExecuteAction_Strike() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Activate Attack to get strikes
	_, err = char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)
	s.Equal(1, char.actionEconomy.Granted[GrantedAttacks])

	// Execute strike
	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Strike(),
	})
	s.Require().NoError(err)
	s.True(output.Success)

	// Attack consumed
	s.Equal(0, char.actionEconomy.Granted[GrantedAttacks])
}

func (s *ActionEconomyTestSuite) TestExecuteAction_Strike_NoAttacks() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Try to strike without activating Attack first
	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Strike(),
	})
	s.Require().NoError(err)
	s.False(output.Success)
	s.Equal("no attacks remaining", output.Error)
}

func (s *ActionEconomyTestSuite) TestExecuteAction_NotInCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Strike(),
	})
	s.Require().NoError(err)
	s.False(output.Success)
	s.Equal("not in combat", output.Error)
}

func (s *ActionEconomyTestSuite) TestExecuteAction_Strike_GrantsMartialArtsBonus() {
	char := createTestMonkCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Activate Attack
	_, err = char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)

	// Execute strike - should grant martial arts bonus
	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Strike(),
	})
	s.Require().NoError(err)
	s.True(output.Success)

	// Martial arts bonus should be granted
	s.Equal(1, char.actionEconomy.Granted[GrantedMartialArtsBonus])

	// UnarmedStrike should appear in available actions
	var foundUnarmed bool
	for _, a := range output.Actions {
		if a.Ref.ID == refs.Actions.UnarmedStrike().ID {
			foundUnarmed = true
			s.True(a.CanUse)
		}
	}
	s.True(foundUnarmed, "Unarmed Strike should be available after monk strike")
}

func (s *ActionEconomyTestSuite) TestExecuteAction_Strike_GrantsOffHandAttack() {
	char := createTWFCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Activate Attack
	_, err = char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)

	// Execute strike - should grant off-hand attack
	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Strike(),
	})
	s.Require().NoError(err)
	s.True(output.Success)

	// Off-hand strike should be granted
	s.Equal(1, char.actionEconomy.Granted[GrantedOffHandStrikes])

	// OffHandStrike should appear in available actions
	var foundOffHand bool
	for _, a := range output.Actions {
		if a.Ref.ID == refs.Actions.OffHandStrike().ID {
			foundOffHand = true
			s.True(a.CanUse)
		}
	}
	s.True(foundOffHand, "Off-Hand Strike should be available after TWF strike")
}

func (s *ActionEconomyTestSuite) TestExecuteAction_OffHandStrike() {
	char := createTWFCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Grant off-hand strikes directly
	char.actionEconomy.Granted[GrantedOffHandStrikes] = 1

	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.OffHandStrike(),
	})
	s.Require().NoError(err)
	s.True(output.Success)
	s.Equal(0, char.actionEconomy.Granted[GrantedOffHandStrikes])
}

func (s *ActionEconomyTestSuite) TestExecuteAction_UnarmedStrike() {
	char := createTestMonkCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Grant martial arts bonus directly
	char.actionEconomy.Granted[GrantedMartialArtsBonus] = 1

	output, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.UnarmedStrike(),
	})
	s.Require().NoError(err)
	s.True(output.Success)
	s.Equal(0, char.actionEconomy.Granted[GrantedMartialArtsBonus])
}

func (s *ActionEconomyTestSuite) TestGrantCapacity() {
	char := createTestFighterCharacter(s.T(), s.bus)

	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	char.GrantCapacity(GrantedAttacks, 2)
	s.Equal(2, char.actionEconomy.Granted[GrantedAttacks])
	s.True(char.HasGranted(GrantedAttacks))
}

func (s *ActionEconomyTestSuite) TestHasGranted_NotInCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)
	s.False(char.HasGranted(GrantedAttacks))
}
