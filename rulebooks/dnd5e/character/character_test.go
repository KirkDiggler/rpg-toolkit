package character

import (
	"context"
	"encoding/json"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/require"
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

func (s *CharacterResourceTestSuite) TestLoadResourceDataIsInertUntilCharacterRest() {
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

	s.character.LoadResourceData(s.ctx, s.bus, data)
	s.Require().Len(s.character.resources, 2)
	rage := s.character.GetResource("rage")
	ki := s.character.GetResource("ki")
	s.Require().Equal(2, rage.Maximum())
	s.Require().Equal(coreResources.ResetLongRest, rage.ResetType)
	s.Require().Equal(5, ki.Maximum())
	s.Require().Equal(coreResources.ResetShortRest, ki.ResetType)

	// Neither the supplied bus nor any other bus owns character-resource
	// recovery. Raw events cannot move the persisted values.
	otherBus := events.NewEventBus()
	for _, bus := range []events.EventBus{otherBus, s.bus} {
		err := dnd5eEvents.RestTopic.On(bus).Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType: coreResources.ResetLongRest, CharacterID: s.character.GetID(),
		})
		s.Require().NoError(err)
		s.Require().Equal(1, rage.Current())
		s.Require().Equal(3, ki.Current())
	}
	s.False(rage.IsApplied())
	s.False(ki.IsApplied())

	// The Character verbs remain the sole rule owners and respect each reset
	// type: short rest restores Ki only; long rest restores both.
	s.character.bus = s.bus
	_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})
	s.Require().NoError(err)
	s.Equal(1, rage.Current())
	s.Equal(5, ki.Current())
	s.Require().NoError(ki.Use(2))
	s.Require().NoError(restErr(s.character.LongRest(s.ctx)))
	s.Equal(2, rage.Current())
	s.Equal(5, ki.Current())
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
	s.Assert().False(rage.IsApplied())
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
	s.Assert().False(rage.IsApplied())

	ki := newChar.GetResource("ki")
	s.Require().NotNil(ki)
	s.Assert().Equal(5, ki.Current())
	s.Assert().Equal(5, ki.Maximum())
	s.Assert().Equal(coreResources.ResetShortRest, ki.ResetType)
	s.Assert().False(ki.IsApplied())
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
		id:            "test-char",
		classID:       classes.Fighter,
		levels:        syntheticLevels(classes.Fighter, 1),
		abilityScores: scores,
		savingThrows:  savingThrows,
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
	// A full save consults the chain on the character's bus (rpg-toolkit#1357)
	char.bus = events.NewEventBus()

	// Make a saving throw against DC 15
	result, err := char.MakeSavingThrow(s.ctx, &MakeSavingThrowInput{
		Ability:   abilities.WIS,
		DC:        15,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.SacredFlame(), Name: "Sacred Flame"},
	})

	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Result should have DC set correctly
	s.Equal(15, result.DC)

	// Total should be roll + 5 (+3 ability + 2 proficiency)
	expectedTotal := result.Roll + 5
	s.Equal(expectedTotal, result.Total, "total should be roll + modifier")
}

// TestMakeSavingThrowRefusesUnattachedCharacter pins the fail-closed side of
// rpg-toolkit#1357: a full save consults the chain, and a character on no bus
// has every save-modifying condition absent — refused loudly, never rolled
// wrong.
func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowRefusesUnattachedCharacter() {
	char := s.createTestCharacter(
		map[string]int{"wis": 16},
		[]string{"wis"},
	)

	result, err := char.MakeSavingThrow(s.ctx, &MakeSavingThrowInput{
		Ability:   abilities.WIS,
		DC:        15,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.SacredFlame(), Name: "Sacred Flame"},
	})

	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "on no bus")
	s.Equal(rpgerr.CodePrerequisiteNotMet, rpgerr.GetCode(err))
}

// TestMakeSavingThrowConsultsParkedBusConditions pins the PR's headline
// behavior (rpg-toolkit#1357): a condition subscribed on the character's
// parked bus reaches Character.MakeSavingThrow — the verb passes THAT bus and
// the character's own id, not a fresh bus and not nothing. Dodging grants
// advantage on DEX saves keyed by SaverID; if a refactor ever swaps in a
// different bus or id, this modifier vanishes and this test fails while every
// arithmetic-only test stays green.
func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowResolvesRecipientSelectedBane() {
	char := s.createTestCharacter(map[string]int{"con": 10}, nil)
	char.bus = events.NewEventBus()
	baned, err := conditions.NewBanedCondition(conditions.NewBanedConditionInput{
		MemberID: char.GetID(), SourceID: "bard-a", SourceRef: refs.Spells.Bane(),
	})
	s.Require().NoError(err)
	char.conditions = append(char.conditions, baned)
	roller := &scriptedRoller{results: []int{10, 3}}

	result, err := char.MakeSavingThrow(s.ctx, &MakeSavingThrowInput{
		Roller: roller, Ability: abilities.CON, DC: 10,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.SacredFlame(), Name: "Sacred Flame"},
	})
	s.Require().NoError(err)
	s.Equal([]int{20, 4}, roller.calls)
	s.Equal(7, result.Total)
	s.False(result.Success)
	s.Require().Len(result.Calculation.Components, 3)
	s.True(result.Calculation.Components[2].SubtractDice)
	s.Equal("bard-a", result.Calculation.Components[2].Source.SourceID)
}

func (s *CharacterSavingThrowTestSuite) TestMakeSavingThrowConsultsParkedBusConditions() {
	char := s.createTestCharacter(
		map[string]int{"dex": 14},
		[]string{},
	)
	char.bus = events.NewEventBus()

	dodging := conditions.NewDodgingCondition(char.GetID())
	s.Require().NoError(dodging.Apply(s.ctx, char.bus))

	result, err := char.MakeSavingThrow(s.ctx, &MakeSavingThrowInput{
		Ability:   abilities.DEX,
		DC:        10,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.SacredFlame(), Name: "Sacred Flame"},
	})

	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Require().NotNil(result.Calculation)
	keep := result.Calculation.Components[0].Dice.Keep
	s.Require().NotNil(keep, "advantage from the parked bus must reach the die's keep record")

	var names []string
	for _, src := range keep.Granted {
		names = append(names, src.Name)
	}
	s.Contains(names, "Dodging",
		"the Dodging condition on the parked bus must reach the save through the chain")
}

func TestCharacterSavingThrowSuite(t *testing.T) {
	suite.Run(t, new(CharacterSavingThrowTestSuite))
}

// Death save operation coverage lives in death_save_test.go, where every roll uses
// a Dying-turn capacity fixture rather than the superseded no-economy contract.

// mockHitDiceRoller allows controlled rolls for hit dice tests
type mockHitDiceRoller struct {
	rolls []int // Sequence of rolls to return
	index int   // Current position in sequence
	calls int
}

func (m *mockHitDiceRoller) Roll(_ context.Context, _ int) (int, error) {
	m.calls++
	if m.index >= len(m.rolls) {
		m.index = 0 // Loop back
	}
	result := m.rolls[m.index]
	m.index++
	return result, nil
}

func (m *mockHitDiceRoller) RollN(_ context.Context, n, _ int) ([]int, error) {
	m.calls++
	result := make([]int, n)
	for i := range result {
		if m.index >= len(m.rolls) {
			m.index = 0
		}
		result[i] = m.rolls[m.index]
		m.index++
	}
	return result, nil
}

// CharacterHitDiceTestSuite tests hit dice spending functionality
type CharacterHitDiceTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	character *Character
}

func (s *CharacterHitDiceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.createFreshCharacter()
}

func (s *CharacterHitDiceTestSuite) SetupSubTest() {
	// Reset to fresh state for each subtest
	if s.character != nil {
		_ = s.character.Cleanup(s.ctx)
	}
	s.bus = events.NewEventBus()
	s.createFreshCharacter()
}

func (s *CharacterHitDiceTestSuite) createFreshCharacter() {
	// Create a level 4 Fighter (d10 hit dice, +2 CON modifier from 14 CON)
	s.character = &Character{
		id:           "test-fighter",
		classID:      classes.Fighter,
		levels:       syntheticLevels(classes.Fighter, 4),
		hitDice:      10, // d10
		hitPoints:    15,
		maxHitPoints: 40,
		abilityScores: shared.AbilityScores{
			abilities.CON: 14, // +2 modifier
		},
		bus:       s.bus,
		resources: make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}

	// Subscribe to events
	err := s.character.subscribeToEvents(s.ctx)
	s.Require().NoError(err)
}

func (s *CharacterHitDiceTestSuite) TearDownTest() {
	if s.character != nil {
		_ = s.character.Cleanup(s.ctx)
	}
}

func (s *CharacterHitDiceTestSuite) hitDicePool(spent int) *combat.RecoverableResource {
	pool := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          string(resources.HitDice),
		Maximum:     4,
		CharacterID: "test-fighter",
		ResetType:   coreResources.ResetLongRest,
	})
	if spent > 0 {
		s.Require().NoError(pool.Use(spent))
	}
	s.character.AddResource(resources.HitDice, pool)
	return pool
}

func (s *CharacterHitDiceTestSuite) TestShortRestHitDice() {
	s.Run("dead character is rejected before rolling or spending", func() {
		pool := s.hitDicePool(0)
		s.character.hitPoints = 0
		s.character.deathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
		markSaved(s.character)
		roller := &mockHitDiceRoller{rolls: []int{10, 10}}

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{HitDice: 2, Roller: roller})

		s.Require().Error(err)
		s.Equal(rpgerr.CodeInvalidState, rpgerr.GetCode(err))
		s.Nil(result)
		s.Zero(roller.calls)
		s.Equal(4, pool.Current())
		s.Zero(s.character.GetHitPoints())
		s.Equal(&saves.DeathSaveState{Failures: 3, Dead: true}, s.character.GetDeathSaveState())
		s.False(s.character.IsDirty())
	})

	s.Run("spends hit dice and heals character", func() {
		s.hitDicePool(0)
		roller := &mockHitDiceRoller{rolls: []int{6, 6}}

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{HitDice: 2, Roller: roller})

		s.Require().NoError(err)
		s.Require().NotNil(result)
		s.Equal(2, result.HitDiceSpent)
		s.Equal(16, result.Healed, "2 * (6 + 2) = 16")
		s.Equal(2, result.HitDiceRemaining, "4 - 2 = 2 remaining")
		s.Equal(31, s.character.GetHitPoints(), "15 + 16")
		s.Equal(2, s.character.GetResource(resources.HitDice).Current())
	})

	s.Run("caps healing at max HP", func() {
		s.character.hitPoints = 35
		s.hitDicePool(0)

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
			HitDice: 1, Roller: &mockHitDiceRoller{rolls: []int{10}},
		})

		s.Require().NoError(err)
		s.Equal(12, result.Requested, "10 + 2 requested")
		s.Equal(5, result.Healed, "35 to 40 is what landed")
		s.Equal(40, s.character.GetHitPoints(), "should cap at max HP")
	})

	s.Run("more dice than remain spends nothing and rolls nothing", func() {
		pool := s.hitDicePool(3)
		markSaved(s.character)
		roller := &mockHitDiceRoller{rolls: []int{5}}

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{HitDice: 2, Roller: roller})

		s.Require().Error(err)
		s.Equal(rpgerr.CodeResourceExhausted, rpgerr.GetCode(err))
		s.Nil(result)
		s.Zero(roller.calls)
		s.Equal(1, pool.Current())
		s.Equal(15, s.character.GetHitPoints())
		s.False(s.character.IsDirty())
	})

	s.Run("a negative count is refused", func() {
		s.hitDicePool(0)

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
			HitDice: -1, Roller: &mockHitDiceRoller{rolls: []int{5}},
		})

		s.Require().Error(err)
		s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
		s.Nil(result)
	})

	s.Run("dice with no roller are refused", func() {
		pool := s.hitDicePool(0)

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{HitDice: 1})

		s.Require().Error(err)
		s.Nil(result)
		s.Equal(4, pool.Current())
	})

	s.Run("dice asked of a character with no hit dice pool are refused", func() {
		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
			HitDice: 1, Roller: &mockHitDiceRoller{rolls: []int{5}},
		})

		s.Require().Error(err)
		s.Equal(rpgerr.CodeNotFound, rpgerr.GetCode(err))
		s.Nil(result)
	})

	s.Run("nil input is refused", func() {
		result, err := s.character.ShortRest(s.ctx, nil)

		s.Require().Error(err)
		s.Nil(result)
	})

	// The operation it replaced dereferenced the bus to publish its healing
	// after the die was already spent. A sheet with no bus is refused first.
	s.Run("a sheet with no bus is refused before anything moves", func() {
		pool := s.hitDicePool(0)
		s.character.bus = nil
		roller := &mockHitDiceRoller{rolls: []int{6}}

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{HitDice: 1, Roller: roller})

		s.Require().Error(err)
		s.Nil(result)
		s.Zero(roller.calls)
		s.Equal(4, pool.Current())
	})

	s.Run("handles negative CON modifier correctly", func() {
		s.character.abilityScores[abilities.CON] = 6
		s.hitDicePool(0)

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
			HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{3, 3}},
		})

		s.Require().NoError(err)
		s.Equal(2, result.Healed, "2 * (3 + -2) = 2")
		s.Equal(17, s.character.GetHitPoints(), "15 + 2 = 17")
	})

	s.Run("each die floors at zero on its own, and the floor is in the trace", func() {
		s.character.abilityScores[abilities.CON] = 6 // -2
		pool := s.hitDicePool(0)

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
			HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{1, 6}},
		})

		s.Require().NoError(err)
		// Per die: max(1-2, 0) + max(6-2, 0) = 0 + 4. A per-rest floor
		// would heal 3.
		s.Equal(4, result.Requested)
		s.Equal(4, result.Healed)
		s.Equal(4, result.Healing.Total)
		s.Equal(19, s.character.GetHitPoints())
		s.Equal(2, pool.Current(), "the dice are spent whatever they rolled")
		s.Require().NoError(dnd5eEvents.ValidateRollCalculation(result.Healing))

		var floors []int
		for _, component := range result.Healing.Components {
			if component.Source.Ref != nil && component.Source.Ref.Equals(refs.Rules.HitDieFloor()) {
				s.Require().NotNil(component.Modifier)
				floors = append(floors, *component.Modifier)
			}
		}
		s.Equal([]int{1}, floors, "one floor line, for the die that rolled 1, lifting it by 1")
	})

	s.Run("a rest whose every die floors heals nothing and never goes negative", func() {
		s.character.abilityScores[abilities.CON] = 4 // -3
		pool := s.hitDicePool(0)

		result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
			HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{1, 1}},
		})

		s.Require().NoError(err)
		s.Equal(0, result.Healing.Total)
		s.Equal(0, result.Healed)
		s.Equal(15, s.character.GetHitPoints())
		s.Equal(2, pool.Current())
	})
}

// Every hit die names the resting character as the entity that threw it, and
// the sheet hears the heal as that sourced calculation.
func (s *CharacterHitDiceTestSuite) TestHitDiceKnowWhoseTheyAre() {
	s.hitDicePool(0)

	var got *dnd5eEvents.HealingAppliedEvent
	_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			got = &event
			return nil
		})
	s.Require().NoError(err)

	result, err := s.character.ShortRest(s.ctx, &ShortRestInput{
		HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{4, 7}},
	})
	s.Require().NoError(err)

	s.Require().NotNil(result.Healing)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(result.Healing))
	var dice []int
	for _, component := range result.Healing.Components {
		if component.Dice == nil {
			continue
		}
		s.Equal("test-fighter", component.Source.SourceID, "every die is the resting character's")
		s.Equal(10, component.Dice.DieSize)
		dice = append(dice, component.Dice.FinalRolls...)
	}
	s.ElementsMatch([]int{4, 7}, dice)
	s.Equal(15, result.Healing.Total, "4 + 7 + 2*2")

	s.Require().NotNil(got, "the sheet publishes the applied heal")
	s.Require().NotNil(got.Calculation, "the heal lands as the sourced roll, not as scalars")
	s.Equal(15, got.Requested)
	s.Equal(15, got.Applied)
	s.Zero(got.Roll)
	s.Zero(got.Modifier)
}

func TestCharacterHitDiceSuite(t *testing.T) {
	suite.Run(t, new(CharacterHitDiceTestSuite))
}

// TestLegacySpellSlotsAreNotReadOrWritten catches the retired JSON shape
// surviving as hidden mutable state at the character persistence boundary.
func TestLegacySpellSlotsAreNotReadOrWritten(t *testing.T) {
	var data Data
	require.NoError(t, json.Unmarshal([]byte(`{"id":"legacy","spell_slots":{"1":{"max":2,"used":1}}}`), &data))

	raw, err := json.Marshal(&data)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "spell_slots")
}

// CharacterLoadFromDataRoundTripSuite verifies that LoadFromData reads the
// legacy class-resource field and current character data that ToData writes.
type CharacterLoadFromDataRoundTripSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

// SetupTest is run before each test function to establish fresh bus + context.
func (s *CharacterLoadFromDataRoundTripSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// TestAppearanceSurvivesRoundTrip verifies that the complete character data
// round-trip carries the provider-neutral appearance state.
func (s *CharacterLoadFromDataRoundTripSuite) TestAppearanceSurvivesRoundTrip() {
	in := s.minimalSpellcasterData()
	expected := customization.CloneAppearance(in.Appearance)

	char, err := LoadFromData(s.ctx, in, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	out := mustToData(s.T(), char)
	s.Require().Equal(expected, out.Appearance)
}

// A record written before class_resources was deleted still loads, and the
// module never writes the field again: it was round-tripped and never read.
func (s *CharacterLoadFromDataRoundTripSuite) TestARecordCarryingClassResourcesLoadsAndDropsIt() {
	raw, err := json.Marshal(s.minimalSpellcasterData())
	s.Require().NoError(err)
	var record map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &record))
	record["class_resources"] = json.RawMessage(`{"1":{"name":"Rage","max":2,"current":2,"resets":"long_rest"}}`)
	legacy, err := json.Marshal(record)
	s.Require().NoError(err)

	var in Data
	s.Require().NoError(json.Unmarshal(legacy, &in))
	char, err := LoadFromData(s.ctx, &in, s.bus)
	s.Require().NoError(err)

	written, err := json.Marshal(mustToData(s.T(), char))
	s.Require().NoError(err)
	s.NotContains(string(written), "class_resources")
}

// minimalSpellcasterData builds the smallest valid Data shape the test needs.
// LoadFromData has minimal required fields beyond ID + bus; this fixture
// covers the constructor's expected fields without bringing in equipment or
// feature complexity.
func (s *CharacterLoadFromDataRoundTripSuite) minimalSpellcasterData() *Data {
	zero := uint32(0)
	return &Data{
		ID: "wendy-test",
		Appearance: &customization.Appearance{
			Hair: &customization.HairCustomization{ColorSRGB: &zero},
			Outfit: &customization.OutfitCustomization{
				PrimaryColorSRGB: &zero,
			},
		},
		Name:             "Wendy",
		Level:            1,
		ProficiencyBonus: 2,
		HitPoints:        8,
		MaxHitPoints:     8,
		AbilityScores: shared.AbilityScores{
			abilities.INT: 16,
		},
	}
}

// TestCharacterLoadFromDataRoundTripSuite runs the round-trip regression suite.
func TestCharacterLoadFromDataRoundTripSuite(t *testing.T) {
	suite.Run(t, new(CharacterLoadFromDataRoundTripSuite))
}
