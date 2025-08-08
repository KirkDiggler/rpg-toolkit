package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/game"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// CombatTestSuite contains tests for the combat system
type CombatTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	mockRoller *mock_dice.MockRoller
	eventBus   events.EventBus
	combat     *combat.CombatState
}

func (s *CombatTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.eventBus = events.NewBus()

	s.combat = combat.NewCombatState(combat.CombatStateConfig{
		ID:       "test-combat-001",
		Name:     "Test Combat",
		EventBus: s.eventBus,
		Roller:   s.mockRoller,
		Settings: combat.CombatSettings{
			InitiativeRollMode: combat.InitiativeRollModeRoll,
			TieBreakingMode:    combat.TieBreakingModeDexterity,
		},
	})
}

func (s *CombatTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestCombatSuite(t *testing.T) {
	suite.Run(t, new(CombatTestSuite))
}

// Test helper: mock combatant
type mockCombatant struct {
	id         string
	entityType string
	dexMod     int
	dexScore   int
	ac         int
	hp         int
	maxHP      int
	conscious  bool
	defeated   bool
}

func (m *mockCombatant) GetID() string             { return m.id }
func (m *mockCombatant) GetType() string           { return m.entityType }
func (m *mockCombatant) GetDexterityModifier() int { return m.dexMod }
func (m *mockCombatant) GetDexterityScore() int    { return m.dexScore }
func (m *mockCombatant) GetArmorClass() int        { return m.ac }
func (m *mockCombatant) GetHitPoints() int         { return m.hp }
func (m *mockCombatant) GetMaxHitPoints() int      { return m.maxHP }
func (m *mockCombatant) IsConscious() bool         { return m.conscious }
func (m *mockCombatant) IsDefeated() bool          { return m.defeated }

func (s *CombatTestSuite) TestCombatStateCreation() {
	assert.Equal(s.T(), "test-combat-001", s.combat.GetID())
	assert.Equal(s.T(), "Test Combat", s.combat.GetName())
	assert.Equal(s.T(), "combat_encounter", s.combat.GetType())
	assert.Equal(s.T(), combat.CombatStatusPending, s.combat.GetStatus())
	assert.Equal(s.T(), 0, s.combat.GetRound())
}

func (s *CombatTestSuite) TestAddCombatant() {
	combatant := &mockCombatant{
		id:         "fighter-001",
		entityType: "character",
		dexMod:     2,
		dexScore:   14,
		ac:         18,
		hp:         45,
		maxHP:      45,
		conscious:  true,
		defeated:   false,
	}

	err := s.combat.AddCombatant(combatant)
	require.NoError(s.T(), err)

	// Test duplicate addition
	err = s.combat.AddCombatant(combatant)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "already in combat")
}

func (s *CombatTestSuite) TestRollInitiativeBasic() {
	// Create test combatants
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
			dexMod:     2,
			dexScore:   14,
		},
		&mockCombatant{
			id:         "rogue-001",
			entityType: "character",
			dexMod:     4,
			dexScore:   18,
		},
		&mockCombatant{
			id:         "orc-001",
			entityType: "monster",
			dexMod:     1,
			dexScore:   12,
		},
	}

	// Add combatants
	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Set up dice rolls
	s.mockRoller.EXPECT().Roll(20).Return(15, nil) // Fighter
	s.mockRoller.EXPECT().Roll(20).Return(12, nil) // Rogue
	s.mockRoller.EXPECT().Roll(20).Return(18, nil) // Orc

	// Roll initiative
	input := &combat.RollInitiativeInput{
		Combatants: combatants,
		Roller:     s.mockRoller,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	output, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), output)

	// Check results
	assert.Len(s.T(), output.InitiativeEntries, 3)
	assert.Len(s.T(), output.RollResults, 3)
	assert.Len(s.T(), output.UnresolvedTies, 0) // No ties in this case

	// Verify initiative order (highest first)
	entries := output.InitiativeEntries

	// Orc should be first (18 + 1 = 19)
	assert.Equal(s.T(), "orc-001", entries[0].EntityID)
	assert.Equal(s.T(), 18, entries[0].Roll)
	assert.Equal(s.T(), 1, entries[0].Modifier)
	assert.Equal(s.T(), 19, entries[0].Total)

	// Fighter should be second (15 + 2 = 17)
	assert.Equal(s.T(), "fighter-001", entries[1].EntityID)
	assert.Equal(s.T(), 15, entries[1].Roll)
	assert.Equal(s.T(), 2, entries[1].Modifier)
	assert.Equal(s.T(), 17, entries[1].Total)

	// Rogue should be third (12 + 4 = 16)
	assert.Equal(s.T(), "rogue-001", entries[2].EntityID)
	assert.Equal(s.T(), 12, entries[2].Roll)
	assert.Equal(s.T(), 4, entries[2].Modifier)
	assert.Equal(s.T(), 16, entries[2].Total)
}

func (s *CombatTestSuite) TestRollInitiativeWithTies() {
	// Create combatants with potential for ties
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
			dexMod:     2,
			dexScore:   14,
		},
		&mockCombatant{
			id:         "wizard-001",
			entityType: "character",
			dexMod:     1,
			dexScore:   12,
		},
	}

	// Add combatants
	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Set up dice rolls to create a tie
	s.mockRoller.EXPECT().Roll(20).Return(15, nil) // Fighter: 15 + 2 = 17
	s.mockRoller.EXPECT().Roll(20).Return(16, nil) // Wizard: 16 + 1 = 17

	input := &combat.RollInitiativeInput{
		Combatants: combatants,
		Roller:     s.mockRoller,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	output, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	// Should detect the tie
	assert.Len(s.T(), output.UnresolvedTies, 1)
	assert.Len(s.T(), output.UnresolvedTies[0], 2)

	// Fighter should win tie due to higher DEX score
	entries := output.InitiativeEntries
	assert.Equal(s.T(), "fighter-001", entries[0].EntityID)
	assert.Equal(s.T(), "wizard-001", entries[1].EntityID)
}

func (s *CombatTestSuite) TestResolveTiesWithDexterity() {
	// Create initiative entries with a tie
	entries := []combat.InitiativeEntry{
		{
			EntityID:       "fighter-001",
			Roll:           15,
			Modifier:       2,
			Total:          17,
			DexterityScore: 14,
			Active:         true,
		},
		{
			EntityID:       "wizard-001",
			Roll:           16,
			Modifier:       1,
			Total:          17,
			DexterityScore: 12,
			Active:         true,
		},
	}

	tiedGroups := [][]string{{"fighter-001", "wizard-001"}}

	input := &combat.ResolveTiesInput{
		TiedGroups:        tiedGroups,
		InitiativeEntries: entries,
		TieBreakingMode:   combat.TieBreakingModeDexterity,
	}

	output, err := s.combat.ResolveTies(input)
	require.NoError(s.T(), err)

	// Fighter should win due to higher DEX
	resolvedEntries := output.ResolvedEntries
	assert.Equal(s.T(), "fighter-001", resolvedEntries[0].EntityID)
	assert.Equal(s.T(), "wizard-001", resolvedEntries[1].EntityID)

	// No remaining ties since DEX scores are different
	assert.Len(s.T(), output.RemainingTies, 0)
}

func (s *CombatTestSuite) TestResolveTiesWithReroll() {
	// Create initiative entries with identical totals and DEX
	entries := []combat.InitiativeEntry{
		{
			EntityID:       "fighter-001",
			Roll:           15,
			Modifier:       2,
			Total:          17,
			DexterityScore: 14,
			Active:         true,
		},
		{
			EntityID:       "ranger-001",
			Roll:           17,
			Modifier:       0,
			Total:          17,
			DexterityScore: 14, // Same DEX - true tie
			Active:         true,
		},
	}

	tiedGroups := [][]string{{"fighter-001", "ranger-001"}}

	// Mock the re-rolls
	s.mockRoller.EXPECT().Roll(20).Return(12, nil) // Fighter tie-breaker
	s.mockRoller.EXPECT().Roll(20).Return(18, nil) // Ranger tie-breaker

	input := &combat.ResolveTiesInput{
		TiedGroups:        tiedGroups,
		InitiativeEntries: entries,
		TieBreakingMode:   combat.TieBreakingModeRoll,
		Roller:            s.mockRoller,
	}

	output, err := s.combat.ResolveTies(input)
	require.NoError(s.T(), err)

	// Ranger should win due to higher re-roll
	resolvedEntries := output.ResolvedEntries
	assert.Equal(s.T(), "ranger-001", resolvedEntries[0].EntityID)
	assert.Equal(s.T(), 18, resolvedEntries[0].TieBreaker)

	assert.Equal(s.T(), "fighter-001", resolvedEntries[1].EntityID)
	assert.Equal(s.T(), 12, resolvedEntries[1].TieBreaker)

	assert.Len(s.T(), output.RemainingTies, 0)
}

func (s *CombatTestSuite) TestResolveTiesWithDMDecision() {
	entries := []combat.InitiativeEntry{
		{
			EntityID:       "fighter-001",
			Roll:           15,
			Modifier:       2,
			Total:          17,
			DexterityScore: 14,
			Active:         true,
		},
		{
			EntityID:       "ranger-001",
			Roll:           17,
			Modifier:       0,
			Total:          17,
			DexterityScore: 14,
			Active:         true,
		},
	}

	tiedGroups := [][]string{{"fighter-001", "ranger-001"}}

	// DM decides ranger goes first
	manualOrder := map[string]int{
		"ranger-001":  100,
		"fighter-001": 50,
	}

	input := &combat.ResolveTiesInput{
		TiedGroups:        tiedGroups,
		InitiativeEntries: entries,
		TieBreakingMode:   combat.TieBreakingModeDM,
		ManualOrder:       manualOrder,
	}

	output, err := s.combat.ResolveTies(input)
	require.NoError(s.T(), err)

	// Ranger should be first due to DM decision
	resolvedEntries := output.ResolvedEntries
	assert.Equal(s.T(), "ranger-001", resolvedEntries[0].EntityID)
	assert.Equal(s.T(), 100, resolvedEntries[0].TieBreaker)

	assert.Equal(s.T(), "fighter-001", resolvedEntries[1].EntityID)
	assert.Equal(s.T(), 50, resolvedEntries[1].TieBreaker)

	assert.Len(s.T(), output.RemainingTies, 0)
}

func (s *CombatTestSuite) TestStartCombat() {
	// Add combatants and roll initiative first
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
			dexMod:     2,
			dexScore:   14,
		},
		&mockCombatant{
			id:         "orc-001",
			entityType: "monster",
			dexMod:     1,
			dexScore:   12,
		},
	}

	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Roll initiative
	s.mockRoller.EXPECT().Roll(20).Return(15, nil) // Fighter
	s.mockRoller.EXPECT().Roll(20).Return(12, nil) // Orc

	input := &combat.RollInitiativeInput{
		Combatants: combatants,
		Roller:     s.mockRoller,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	_, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	// Start combat
	err = s.combat.StartCombat()
	require.NoError(s.T(), err)

	assert.Equal(s.T(), combat.CombatStatusActive, s.combat.GetStatus())
	assert.Equal(s.T(), 1, s.combat.GetRound())

	// Should be fighter's turn (17 > 13)
	currentTurn, err := s.combat.GetCurrentTurn()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "fighter-001", currentTurn.EntityID)
}

func (s *CombatTestSuite) TestTurnProgression() {
	// Setup combat with two combatants
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
			dexMod:     2,
			dexScore:   14,
		},
		&mockCombatant{
			id:         "orc-001",
			entityType: "monster",
			dexMod:     1,
			dexScore:   12,
		},
	}

	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Roll initiative
	s.mockRoller.EXPECT().Roll(20).Return(15, nil) // Fighter: 17
	s.mockRoller.EXPECT().Roll(20).Return(12, nil) // Orc: 13

	input := &combat.RollInitiativeInput{
		Combatants: combatants,
		Roller:     s.mockRoller,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	_, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	// Start combat
	err = s.combat.StartCombat()
	require.NoError(s.T(), err)

	// Round 1, Turn 1 - Fighter
	assert.Equal(s.T(), 1, s.combat.GetRound())
	currentTurn, err := s.combat.GetCurrentTurn()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "fighter-001", currentTurn.EntityID)

	// Next turn
	err = s.combat.NextTurn()
	require.NoError(s.T(), err)

	// Round 1, Turn 2 - Orc
	assert.Equal(s.T(), 1, s.combat.GetRound())
	currentTurn, err = s.combat.GetCurrentTurn()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "orc-001", currentTurn.EntityID)

	// Next turn should start Round 2
	err = s.combat.NextTurn()
	require.NoError(s.T(), err)

	// Round 2, Turn 1 - Fighter again
	assert.Equal(s.T(), 2, s.combat.GetRound())
	currentTurn, err = s.combat.GetCurrentTurn()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "fighter-001", currentTurn.EntityID)
}

func (s *CombatTestSuite) TestStaticInitiative() {
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
			dexMod:     2,
			dexScore:   14,
		},
		&mockCombatant{
			id:         "wizard-001",
			entityType: "character",
			dexMod:     3,
			dexScore:   16,
		},
	}

	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Use static mode (no dice rolls expected)
	input := &combat.RollInitiativeInput{
		Combatants: combatants,
		RollMode:   combat.InitiativeRollModeStatic,
	}

	output, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	// Verify static calculation (10 + modifier)
	entries := output.InitiativeEntries

	// Wizard should be first (10 + 3 = 13)
	assert.Equal(s.T(), "wizard-001", entries[0].EntityID)
	assert.Equal(s.T(), 10, entries[0].Roll)
	assert.Equal(s.T(), 3, entries[0].Modifier)
	assert.Equal(s.T(), 13, entries[0].Total)

	// Fighter should be second (10 + 2 = 12)
	assert.Equal(s.T(), "fighter-001", entries[1].EntityID)
	assert.Equal(s.T(), 10, entries[1].Roll)
	assert.Equal(s.T(), 2, entries[1].Modifier)
	assert.Equal(s.T(), 12, entries[1].Total)
}

func (s *CombatTestSuite) TestManualInitiative() {
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
		},
		&mockCombatant{
			id:         "wizard-001",
			entityType: "character",
		},
	}

	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Use manual values
	manualValues := map[string]int{
		"fighter-001": 18,
		"wizard-001":  15,
	}

	input := &combat.RollInitiativeInput{
		Combatants:   combatants,
		RollMode:     combat.InitiativeRollModeManual,
		ManualValues: manualValues,
	}

	output, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	entries := output.InitiativeEntries

	// Fighter should be first (18 > 15)
	assert.Equal(s.T(), "fighter-001", entries[0].EntityID)
	assert.Equal(s.T(), 0, entries[0].Roll) // Roll not applicable for manual
	assert.Equal(s.T(), 18, entries[0].Total)

	// Wizard should be second
	assert.Equal(s.T(), "wizard-001", entries[1].EntityID)
	assert.Equal(s.T(), 15, entries[1].Total)

	// Check roll results indicate manual mode
	assert.True(s.T(), output.RollResults["fighter-001"].WasManual)
	assert.True(s.T(), output.RollResults["wizard-001"].WasManual)
}

func (s *CombatTestSuite) TestToDataAndLoadFromContext() {
	// Add combatants and set up combat
	combatants := []combat.Combatant{
		&mockCombatant{
			id:         "fighter-001",
			entityType: "character",
			dexMod:     2,
			dexScore:   14,
		},
	}

	for _, combatant := range combatants {
		err := s.combat.AddCombatant(combatant)
		require.NoError(s.T(), err)
	}

	// Roll initiative and start
	s.mockRoller.EXPECT().Roll(20).Return(15, nil)

	input := &combat.RollInitiativeInput{
		Combatants: combatants,
		Roller:     s.mockRoller,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	_, err := s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	err = s.combat.StartCombat()
	require.NoError(s.T(), err)

	// Convert to data
	data := s.combat.ToData()

	// Verify data
	assert.Equal(s.T(), "test-combat-001", data.ID)
	assert.Equal(s.T(), "Test Combat", data.Name)
	assert.Equal(s.T(), combat.CombatStatusActive, data.Status)
	assert.Equal(s.T(), 1, data.Round)
	assert.Equal(s.T(), 0, data.TurnIndex)
	assert.Len(s.T(), data.InitiativeOrder, 1)
	assert.Len(s.T(), data.Combatants, 1)

	// Load from context
	newEventBus := events.NewBus()
	gameCtx, err := game.NewContext(newEventBus, data)
	require.NoError(s.T(), err)

	loadedCombat, err := combat.LoadCombatStateFromContext(context.Background(), gameCtx)
	require.NoError(s.T(), err)

	// Verify loaded combat
	assert.Equal(s.T(), s.combat.GetID(), loadedCombat.GetID())
	assert.Equal(s.T(), s.combat.GetName(), loadedCombat.GetName())
	assert.Equal(s.T(), s.combat.GetStatus(), loadedCombat.GetStatus())
	assert.Equal(s.T(), s.combat.GetRound(), loadedCombat.GetRound())

	// Current turn should be the same
	originalTurn, err := s.combat.GetCurrentTurn()
	require.NoError(s.T(), err)

	loadedTurn, err := loadedCombat.GetCurrentTurn()
	require.NoError(s.T(), err)

	assert.Equal(s.T(), originalTurn.EntityID, loadedTurn.EntityID)
	assert.Equal(s.T(), originalTurn.Total, loadedTurn.Total)
}

func (s *CombatTestSuite) TestEventEmission() {
	// Track events
	var capturedEvents []events.Event
	s.eventBus.SubscribeFunc(combat.EventCombatantAdded, 100, func(_ context.Context, e events.Event) error {
		capturedEvents = append(capturedEvents, e)
		return nil
	})

	s.eventBus.SubscribeFunc(combat.EventInitiativeRolled, 100, func(_ context.Context, e events.Event) error {
		capturedEvents = append(capturedEvents, e)
		return nil
	})

	s.eventBus.SubscribeFunc(combat.EventCombatStarted, 100, func(_ context.Context, e events.Event) error {
		capturedEvents = append(capturedEvents, e)
		return nil
	})

	// Add combatant
	combatant := &mockCombatant{
		id:         "fighter-001",
		entityType: "character",
		dexMod:     2,
		dexScore:   14,
	}

	err := s.combat.AddCombatant(combatant)
	require.NoError(s.T(), err)

	// Roll initiative
	s.mockRoller.EXPECT().Roll(20).Return(15, nil)

	input := &combat.RollInitiativeInput{
		Combatants: []combat.Combatant{combatant},
		Roller:     s.mockRoller,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	_, err = s.combat.RollInitiative(input)
	require.NoError(s.T(), err)

	// Start combat
	err = s.combat.StartCombat()
	require.NoError(s.T(), err)

	// Verify events were emitted
	assert.GreaterOrEqual(s.T(), len(capturedEvents), 3)

	// Check event types
	eventTypes := make([]string, len(capturedEvents))
	for i, event := range capturedEvents {
		eventTypes[i] = event.Type()
	}

	assert.Contains(s.T(), eventTypes, combat.EventCombatantAdded)
	assert.Contains(s.T(), eventTypes, combat.EventInitiativeRolled)
	assert.Contains(s.T(), eventTypes, combat.EventCombatStarted)
}
