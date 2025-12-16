package character

import (
	"context"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

// CharacterResourceTestSuite tests resource storage functionality
type CharacterResourceTestSuite struct {
	suite.Suite
	character *Character
	bus       events.EventBus
	ctx       context.Context
}

func (s *CharacterResourceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.character = &Character{
		id:        "test-char",
		resources: make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}
}

func (s *CharacterResourceTestSuite) TestAddResourceAndGetResource() {
	// Create a resource
	resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "rage",
		Maximum:     2,
		CharacterID: "test-char",
		ResetType:   coreResources.ResetLongRest,
	})

	// Add it to character
	s.character.AddResource("rage", resource)

	// Retrieve it
	retrieved := s.character.GetResource("rage")
	s.Require().NotNil(retrieved)
	s.Assert().Equal(2, retrieved.Maximum())
	s.Assert().Equal(2, retrieved.Current())
	s.Assert().Equal(coreResources.ResetLongRest, retrieved.ResetType)
}

func (s *CharacterResourceTestSuite) TestGetResourceReturnsEmptyForUnknownKey() {
	retrieved := s.character.GetResource("nonexistent")
	s.Assert().NotNil(retrieved, "should return empty resource, not nil")
	s.Assert().True(retrieved.IsEmpty(), "empty resource should be empty")
	s.Assert().Equal(0, retrieved.Maximum(), "empty resource should have 0 maximum")
}

func (s *CharacterResourceTestSuite) TestGetResourceReturnsEmptyWhenMapIsNil() {
	char := &Character{
		id:        "test-char",
		resources: nil,
	}
	retrieved := char.GetResource("anything")
	s.Assert().NotNil(retrieved, "should return empty resource, not nil")
	s.Assert().True(retrieved.IsEmpty(), "empty resource should be empty")
}

func (s *CharacterResourceTestSuite) TestAddResourceInitializesMapIfNil() {
	char := &Character{
		id:        "test-char",
		resources: nil,
	}

	resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "ki",
		Maximum:     3,
		CharacterID: "test-char",
		ResetType:   coreResources.ResetShortRest,
	})

	char.AddResource("ki", resource)

	s.Assert().NotNil(char.resources)
	s.Assert().Equal(1, len(char.resources))
	retrieved := char.GetResource("ki")
	s.Require().NotNil(retrieved)
	s.Assert().Equal(3, retrieved.Maximum())
}

func (s *CharacterResourceTestSuite) TestGetResourceDataReturnsCorrectValues() {
	// Add a resource at full
	resource1 := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "rage",
		Maximum:     2,
		CharacterID: "test-char",
		ResetType:   coreResources.ResetLongRest,
	})
	s.character.AddResource("rage", resource1)

	// Add a resource with some used
	resource2 := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "ki",
		Maximum:     5,
		CharacterID: "test-char",
		ResetType:   coreResources.ResetShortRest,
	})
	_ = resource2.Use(2) // Use 2, leaving 3
	s.character.AddResource("ki", resource2)

	// Get data
	data := s.character.GetResourceData()
	s.Require().NotNil(data)
	s.Assert().Equal(2, len(data))

	// Check rage data
	rageData, exists := data["rage"]
	s.Require().True(exists)
	s.Assert().Equal(2, rageData.Current)
	s.Assert().Equal(2, rageData.Maximum)
	s.Assert().Equal(coreResources.ResetLongRest, rageData.ResetType)

	// Check ki data
	kiData, exists := data["ki"]
	s.Require().True(exists)
	s.Assert().Equal(3, kiData.Current)
	s.Assert().Equal(5, kiData.Maximum)
	s.Assert().Equal(coreResources.ResetShortRest, kiData.ResetType)
}

func (s *CharacterResourceTestSuite) TestGetResourceDataReturnsNilWhenResourcesNil() {
	char := &Character{
		id:        "test-char",
		resources: nil,
	}
	data := char.GetResourceData()
	s.Assert().Nil(data)
}

func (s *CharacterResourceTestSuite) TestLoadResourceDataRestoresResources() {
	// Create data
	data := map[coreResources.ResourceKey]RecoverableResourceData{
		"rage": {
			Current:   1,
			Maximum:   2,
			ResetType: coreResources.ResetLongRest,
		},
		"ki": {
			Current:   3,
			Maximum:   5,
			ResetType: coreResources.ResetShortRest,
		},
	}

	// Load it
	s.character.LoadResourceData(s.ctx, s.bus, data)

	// Verify resources were loaded correctly
	s.Assert().Equal(2, len(s.character.resources))

	// Check rage
	rage := s.character.GetResource("rage")
	s.Require().NotNil(rage)
	s.Assert().Equal(1, rage.Current())
	s.Assert().Equal(2, rage.Maximum())
	s.Assert().Equal(coreResources.ResetLongRest, rage.ResetType)
	s.Assert().True(rage.IsApplied()) // Should be applied to bus

	// Check ki
	ki := s.character.GetResource("ki")
	s.Require().NotNil(ki)
	s.Assert().Equal(3, ki.Current())
	s.Assert().Equal(5, ki.Maximum())
	s.Assert().Equal(coreResources.ResetShortRest, ki.ResetType)
	s.Assert().True(ki.IsApplied()) // Should be applied to bus
}

func (s *CharacterResourceTestSuite) TestLoadResourceDataWithFullResources() {
	// Create data for resources at maximum
	data := map[coreResources.ResourceKey]RecoverableResourceData{
		"rage": {
			Current:   2,
			Maximum:   2,
			ResetType: coreResources.ResetLongRest,
		},
	}

	// Load it
	s.character.LoadResourceData(s.ctx, s.bus, data)

	// Verify resource is at full
	rage := s.character.GetResource("rage")
	s.Require().NotNil(rage)
	s.Assert().Equal(2, rage.Current())
	s.Assert().Equal(2, rage.Maximum())
	s.Assert().True(rage.IsFull())
	s.Assert().True(rage.IsApplied())
}

func (s *CharacterResourceTestSuite) TestLoadResourceDataHandlesNilData() {
	s.character.LoadResourceData(s.ctx, s.bus, nil)
	// Should not panic, resources should remain as initialized
	s.Assert().NotNil(s.character.resources)
	s.Assert().Equal(0, len(s.character.resources))
}

func (s *CharacterResourceTestSuite) TestLoadResourceDataInitializesMapIfNil() {
	char := &Character{
		id:        "test-char",
		resources: nil,
	}

	data := map[coreResources.ResourceKey]RecoverableResourceData{
		"rage": {
			Current:   2,
			Maximum:   2,
			ResetType: coreResources.ResetLongRest,
		},
	}

	char.LoadResourceData(s.ctx, s.bus, data)

	s.Assert().NotNil(char.resources)
	s.Assert().Equal(1, len(char.resources))
}

func (s *CharacterResourceTestSuite) TestRoundTripSerialization() {
	// Add resources
	resource1 := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "rage",
		Maximum:     2,
		CharacterID: "test-char",
		ResetType:   coreResources.ResetLongRest,
	})
	_ = resource1.Use(1) // Use 1, leaving 1
	s.character.AddResource("rage", resource1)

	resource2 := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          "ki",
		Maximum:     5,
		CharacterID: "test-char",
		ResetType:   coreResources.ResetShortRest,
	})
	s.character.AddResource("ki", resource2)

	// Serialize to data
	data := s.character.GetResourceData()

	// Create new character and load data
	newChar := &Character{
		id:        "test-char",
		resources: make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}
	newChar.LoadResourceData(s.ctx, s.bus, data)

	// Verify resources match
	rage := newChar.GetResource("rage")
	s.Require().NotNil(rage)
	s.Assert().Equal(1, rage.Current())
	s.Assert().Equal(2, rage.Maximum())
	s.Assert().Equal(coreResources.ResetLongRest, rage.ResetType)
	s.Assert().True(rage.IsApplied())

	ki := newChar.GetResource("ki")
	s.Require().NotNil(ki)
	s.Assert().Equal(5, ki.Current())
	s.Assert().Equal(5, ki.Maximum())
	s.Assert().Equal(coreResources.ResetShortRest, ki.ResetType)
	s.Assert().True(ki.IsApplied())
}

func TestCharacterResourceSuite(t *testing.T) {
	suite.Run(t, new(CharacterResourceTestSuite))
}

// CharacterSavingThrowTestSuite tests saving throw functionality
type CharacterSavingThrowTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *CharacterSavingThrowTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CharacterSavingThrowTestSuite) createTestCharacter(
	abilityScores map[string]int, proficientSaves []string,
) *Character {
	// Build ability scores
	scores := make(shared.AbilityScores)
	for ability, score := range abilityScores {
		switch ability {
		case "str":
			scores[abilities.STR] = score
		case "dex":
			scores[abilities.DEX] = score
		case "con":
			scores[abilities.CON] = score
		case "int":
			scores[abilities.INT] = score
		case "wis":
			scores[abilities.WIS] = score
		case "cha":
			scores[abilities.CHA] = score
		}
	}

	// Build saving throw proficiencies
	savingThrows := make(map[abilities.Ability]shared.ProficiencyLevel)
	for _, save := range proficientSaves {
		switch save {
		case "str":
			savingThrows[abilities.STR] = shared.Proficient
		case "dex":
			savingThrows[abilities.DEX] = shared.Proficient
		case "con":
			savingThrows[abilities.CON] = shared.Proficient
		case "int":
			savingThrows[abilities.INT] = shared.Proficient
		case "wis":
			savingThrows[abilities.WIS] = shared.Proficient
		case "cha":
			savingThrows[abilities.CHA] = shared.Proficient
		}
	}

	return &Character{
		id:               "test-char",
		level:            1,
		proficiencyBonus: 2, // Level 1 proficiency bonus
		abilityScores:    scores,
		savingThrows:     savingThrows,
	}
}

func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowWithProficiency() {
	// Create a character with 14 CON (+2 modifier) and proficiency in CON saves
	char := s.createTestCharacter(
		map[string]int{"con": 14},
		[]string{"con"},
	)

	// Make a saving throw - the character calculates modifier automatically
	// We're not mocking the roller, so we just verify the modifier was applied correctly
	// by checking GetSavingThrowModifier
	modifier := char.GetSavingThrowModifier(abilities.CON)
	s.Equal(4, modifier, "should be +2 (ability) + 2 (proficiency) = +4")
}

func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowWithoutProficiency() {
	// Create a character with 14 DEX (+2 modifier) but NOT proficient in DEX saves
	char := s.createTestCharacter(
		map[string]int{"dex": 14},
		[]string{}, // No proficiencies
	)

	modifier := char.GetSavingThrowModifier(abilities.DEX)
	s.Equal(2, modifier, "should be +2 (ability only, no proficiency)")
}

func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowNegativeModifier() {
	// Create a character with 8 INT (-1 modifier)
	char := s.createTestCharacter(
		map[string]int{"int": 8},
		[]string{},
	)

	modifier := char.GetSavingThrowModifier(abilities.INT)
	s.Equal(-1, modifier, "should be -1 (8 INT = -1 modifier)")
}

func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowFunctionExists() {
	// Verify the MakeSavingThrow function works end-to-end
	char := s.createTestCharacter(
		map[string]int{"wis": 16}, // +3 modifier
		[]string{"wis"},           // Proficient
	)

	// Make a saving throw against DC 15
	result, err := char.MakeSavingThrow(s.ctx, &MakeSavingThrowInput{
		Ability: abilities.WIS,
		DC:      15,
	})

	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Result should have DC set correctly
	s.Equal(15, result.DC)

	// Total should be roll + 5 (+3 ability + 2 proficiency)
	expectedTotal := result.Roll + 5
	s.Equal(expectedTotal, result.Total, "total should be roll + modifier")
}

func TestCharacterSavingThrowSuite(t *testing.T) {
	suite.Run(t, new(CharacterSavingThrowTestSuite))
}

// CharacterDeathSaveTestSuite tests death saving throw functionality
type CharacterDeathSaveTestSuite struct {
	suite.Suite
	ctx       context.Context
	character *Character
}

func (s *CharacterDeathSaveTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.character = &Character{
		id:             "test-char",
		hitPoints:      0, // At 0 HP for death saves
		maxHitPoints:   10,
		deathSaveState: nil, // Will be initialized by methods
	}
}

func (s *CharacterDeathSaveTestSuite) TestGetDeathSaveStateReturnsEmptyStateInitially() {
	state := s.character.GetDeathSaveState()
	s.Require().NotNil(state)
	s.Equal(0, state.Successes)
	s.Equal(0, state.Failures)
	s.False(state.Stabilized)
	s.False(state.Dead)
}

func (s *CharacterDeathSaveTestSuite) TestMakeDeathSaveUpdatesState() {
	// Make a death save (result depends on roll, but state should be updated)
	result, err := s.character.MakeDeathSave(s.ctx, &MakeDeathSaveInput{})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// State should have been updated (either success or failure added)
	state := s.character.GetDeathSaveState()
	s.True(state.Successes > 0 || state.Failures > 0, "state should have been updated")
}

func (s *CharacterDeathSaveTestSuite) TestMakeDeathSaveWithMockRoller() {
	// Use a mock roller to test specific outcomes
	mockRoller := &mockDeathSaveRoller{rollValue: 15}

	result, err := s.character.MakeDeathSave(s.ctx, &MakeDeathSaveInput{
		Roller: mockRoller,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.Roll)
	s.Equal(1, result.State.Successes, "roll 15 should add 1 success")
	s.Equal(0, result.State.Failures)
}

func (s *CharacterDeathSaveTestSuite) TestTakeDamageWhileUnconsciousAddsFailure() {
	result, err := s.character.TakeDamageWhileUnconscious(s.ctx, &TakeDamageWhileUnconsciousInput{
		IsCritical: false,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(1, result.FailuresAdded)
	state := s.character.GetDeathSaveState()
	s.Equal(1, state.Failures)
}

func (s *CharacterDeathSaveTestSuite) TestTakeCriticalDamageWhileUnconsciousAddsTwoFailures() {
	result, err := s.character.TakeDamageWhileUnconscious(s.ctx, &TakeDamageWhileUnconsciousInput{
		IsCritical: true,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(2, result.FailuresAdded)
	state := s.character.GetDeathSaveState()
	s.Equal(2, state.Failures)
}

func (s *CharacterDeathSaveTestSuite) TestResetDeathSaveStateClearsState() {
	// Add some failures first
	_, _ = s.character.TakeDamageWhileUnconscious(s.ctx, &TakeDamageWhileUnconsciousInput{
		IsCritical: false,
	})

	// Verify state has failures
	state := s.character.GetDeathSaveState()
	s.Equal(1, state.Failures)

	// Reset the state
	s.character.ResetDeathSaveState()

	// Verify state is cleared
	state = s.character.GetDeathSaveState()
	s.Equal(0, state.Successes)
	s.Equal(0, state.Failures)
	s.False(state.Stabilized)
	s.False(state.Dead)
}

func (s *CharacterDeathSaveTestSuite) TestDeathSaveStatePersistsAcrossCalls() {
	// Make multiple death saves and verify state accumulates
	mockRoller := &mockDeathSaveRoller{rollValue: 5} // Failure

	_, _ = s.character.MakeDeathSave(s.ctx, &MakeDeathSaveInput{Roller: mockRoller})
	_, _ = s.character.MakeDeathSave(s.ctx, &MakeDeathSaveInput{Roller: mockRoller})

	state := s.character.GetDeathSaveState()
	s.Equal(2, state.Failures, "failures should accumulate")
}

func (s *CharacterDeathSaveTestSuite) TestDeathSaveStateSerializationRoundTrip() {
	// Add some death save state
	mockRoller := &mockDeathSaveRoller{rollValue: 5} // Failure
	_, _ = s.character.MakeDeathSave(s.ctx, &MakeDeathSaveInput{Roller: mockRoller})
	_, _ = s.character.TakeDamageWhileUnconscious(s.ctx, &TakeDamageWhileUnconsciousInput{
		IsCritical: false,
	})

	// Serialize to Data
	data := s.character.ToData()
	s.Require().NotNil(data.DeathSaveState)
	s.Equal(2, data.DeathSaveState.Failures, "serialized state should have 2 failures")

	// Load from Data
	bus := events.NewEventBus()
	loaded, err := LoadFromData(s.ctx, data, bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	// Verify state was restored
	loadedState := loaded.GetDeathSaveState()
	s.Equal(2, loadedState.Failures, "loaded state should have 2 failures")
	s.Equal(0, loadedState.Successes, "loaded state should have 0 successes")
	s.False(loadedState.Dead)
	s.False(loadedState.Stabilized)
}

func TestCharacterDeathSaveSuite(t *testing.T) {
	suite.Run(t, new(CharacterDeathSaveTestSuite))
}

// mockDeathSaveRoller is a simple mock for testing death saves
type mockDeathSaveRoller struct {
	rollValue int
}

func (m *mockDeathSaveRoller) Roll(_ context.Context, _ int) (int, error) {
	return m.rollValue, nil
}

func (m *mockDeathSaveRoller) RollN(_ context.Context, n, _ int) ([]int, error) {
	result := make([]int, n)
	for i := range result {
		result[i] = m.rollValue
	}
	return result, nil
}
