package combat_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// turnManagerLookup provides combatant lookup for turn manager tests
type turnManagerLookup struct {
	combatants map[string]combat.Combatant
}

func newTurnManagerLookup() *turnManagerLookup {
	return &turnManagerLookup{combatants: make(map[string]combat.Combatant)}
}

func (l *turnManagerLookup) Add(c combat.Combatant) {
	l.combatants[c.GetID()] = c
}

func (l *turnManagerLookup) Get(id string) (combat.Combatant, error) {
	if c, ok := l.combatants[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("combatant not found: %s", id)
}

// TurnManagerTestSuite tests the TurnManager orchestration
type TurnManagerTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *turnManagerLookup
	room       spatial.Room

	fighter *character.Character
	goblin  *character.Character
	weapon  *weapons.Weapon
}

func (s *TurnManagerTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newTurnManagerLookup()
	s.ctx = context.Background()

	// Create a 10x10 square grid room for testing
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "combat",
		Grid: grid,
	})
}

func (s *TurnManagerTestSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
	s.fighter = s.createFighter()
	s.goblin = s.createGoblinCharacter()
	s.weapon = s.createLongsword()

	s.lookup = newTurnManagerLookup()
	s.lookup.Add(s.fighter)
	s.lookup.Add(s.goblin)

	// Place entities in room
	_ = s.room.PlaceEntity(s.fighter, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
}

func (s *TurnManagerTestSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.fighter.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
}

func (s *TurnManagerTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// createFighter creates a level 5 fighter with Extra Attack
func (s *TurnManagerTestSuite) createFighter() *character.Character {
	data := &character.Data{
		ID:               "fighter-1",
		PlayerID:         "player-1",
		Name:             "Sir Reginald",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 18, // +4
			abilities.DEX: 14, // +2
			abilities.CON: 16, // +3
			abilities.INT: 10, // +0
			abilities.WIS: 12, // +1
			abilities.CHA: 10, // +0
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add standard combat abilities
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDash("dash")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDisengage("disengage")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDodge("dodge")))

	return char
}

// createGoblinCharacter creates a goblin as a character for two-combatant scenarios
func (s *TurnManagerTestSuite) createGoblinCharacter() *character.Character {
	data := &character.Data{
		ID:               "goblin-1",
		PlayerID:         "npc",
		Name:             "Goblin Scout",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human, // Using human as stand-in
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,  // -1
			abilities.DEX: 14, // +2
			abilities.CON: 10, // +0
			abilities.INT: 10, // +0
			abilities.WIS: 8,  // -1
			abilities.CHA: 8,  // -1
		},
		HitPoints:    7,
		MaxHitPoints: 7,
		ArmorClass:   13,
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add attack ability for OA capability
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))

	return char
}

func (s *TurnManagerTestSuite) createLongsword() *weapons.Weapon {
	weapon, _ := weapons.GetByID(weapons.Longsword)
	return &weapon
}

func (s *TurnManagerTestSuite) createTurnManager() *combat.TurnManager {
	tm, err := combat.NewTurnManager(&combat.NewTurnManagerInput{
		Character:  s.fighter,
		Combatants: s.lookup,
		Room:       s.room,
		EventBus:   s.bus,
		Roller:     s.mockRoller,
	})
	s.Require().NoError(err)
	return tm
}

// --- Constructor Tests ---

func (s *TurnManagerTestSuite) TestNewTurnManager_NilInput() {
	_, err := combat.NewTurnManager(nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "nil")
}

func (s *TurnManagerTestSuite) TestNewTurnManager_MissingCharacter() {
	s.Run("missing character", func() {
		_, err := combat.NewTurnManager(&combat.NewTurnManagerInput{
			Combatants: s.lookup,
			Room:       s.room,
			EventBus:   s.bus,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "Character")
	})
}

func (s *TurnManagerTestSuite) TestNewTurnManager_MissingCombatants() {
	s.Run("missing combatants", func() {
		_, err := combat.NewTurnManager(&combat.NewTurnManagerInput{
			Character: s.fighter,
			Room:      s.room,
			EventBus:  s.bus,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "Combatants")
	})
}

func (s *TurnManagerTestSuite) TestNewTurnManager_MissingRoom() {
	s.Run("missing room", func() {
		_, err := combat.NewTurnManager(&combat.NewTurnManagerInput{
			Character:  s.fighter,
			Combatants: s.lookup,
			EventBus:   s.bus,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "Room")
	})
}

func (s *TurnManagerTestSuite) TestNewTurnManager_MissingEventBus() {
	s.Run("missing event bus", func() {
		_, err := combat.NewTurnManager(&combat.NewTurnManagerInput{
			Character:  s.fighter,
			Combatants: s.lookup,
			Room:       s.room,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "EventBus")
	})
}

// --- Lifecycle Tests ---

func (s *TurnManagerTestSuite) TestStartTurn() {
	s.Run("initializes economy with movement", func() {
		tm := s.createTurnManager()

		result, err := tm.StartTurn(s.ctx)

		s.Require().NoError(err)
		s.Require().NotNil(result)
		s.Equal(30, result.Economy.MovementRemaining)
		s.Equal(1, result.Economy.ActionsRemaining)
		s.Equal(1, result.Economy.BonusActionsRemaining)
		s.Equal(1, result.Economy.ReactionsRemaining)
		s.Equal(0, result.Economy.AttacksRemaining) // Not set until Attack ability used
	})
}

func (s *TurnManagerTestSuite) TestStartTurn_PublishesTurnStartEvent() {
	s.Run("publishes turn start event", func() {
		tm := s.createTurnManager()

		var received dnd5eEvents.TurnStartEvent
		topic := dnd5eEvents.TurnStartTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.TurnStartEvent) error {
			received = event
			return nil
		})
		s.Require().NoError(err)

		_, err = tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		s.Equal("fighter-1", received.CharacterID)
	})
}

func (s *TurnManagerTestSuite) TestStartTurn_CannotStartTwice() {
	s.Run("errors if already started", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		_, err = tm.StartTurn(s.ctx)
		s.Require().Error(err)
		s.Contains(err.Error(), "already started")
	})
}

func (s *TurnManagerTestSuite) TestEndTurn() {
	s.Run("publishes turn end event and cleans up", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		var received dnd5eEvents.TurnEndEvent
		topic := dnd5eEvents.TurnEndTopic.On(s.bus)
		_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.TurnEndEvent) error {
			received = event
			return nil
		})
		s.Require().NoError(err)

		result, err := tm.EndTurn(s.ctx)

		s.Require().NoError(err)
		s.Equal("fighter-1", result.CharacterID)
		s.Equal("fighter-1", received.CharacterID)
	})
}

func (s *TurnManagerTestSuite) TestEndTurn_NotStarted() {
	s.Run("errors if turn not started", func() {
		tm := s.createTurnManager()

		_, err := tm.EndTurn(s.ctx)
		s.Require().Error(err)
		s.Contains(err.Error(), "not started")
	})
}

// --- Full Attack Turn ---

func (s *TurnManagerTestSuite) TestFullAttackTurn() {
	s.Run("attack ability grants attacks, strike consumes them", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Use Attack ability (fighter with Extra Attack gets 2 attacks)
		abilityResult, err := tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().NoError(err)
		s.Equal(2, abilityResult.Economy.AttacksRemaining) // 1 base + 1 Extra Attack
		s.Equal(0, abilityResult.Economy.ActionsRemaining) // Action consumed

		// First strike - hits
		// Attack: d20(15) + STR(4) + Prof(3) = 22 vs AC 13
		s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(15, nil)
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 8).Return([]int{6}, nil) // 1d8 longsword

		result1, err := tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: "goblin-1",
			Weapon:   s.weapon,
		})
		s.Require().NoError(err)
		s.True(result1.Hit)
		s.Equal(10, result1.TotalDamage) // 6 + STR(4)

		// Second strike - misses
		// Attack: d20(3) + STR(4) + Prof(3) = 10 vs AC 13
		s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(3, nil)

		result2, err := tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: "goblin-1",
			Weapon:   s.weapon,
		})
		s.Require().NoError(err)
		s.False(result2.Hit)

		// Third strike - should fail (no attacks remaining)
		_, err = tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: "goblin-1",
			Weapon:   s.weapon,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "attack")

		_, err = tm.EndTurn(s.ctx)
		s.Require().NoError(err)
	})
}

// --- Movement Tests ---

func (s *TurnManagerTestSuite) TestMovement() {
	s.Run("movement consumes economy", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Move 2 steps (10 feet)
		path := []spatial.Position{
			{X: 2, Y: 2}, // Starting position
			{X: 3, Y: 2}, // Step 1
			{X: 4, Y: 2}, // Step 2
		}

		// Remove goblin from adjacent position to avoid OA
		_ = s.room.RemoveEntity("goblin-1")

		result, err := tm.Move(s.ctx, &combat.MoveInput{Path: path})
		s.Require().NoError(err)
		s.Equal(2, result.StepsCompleted)
		s.False(result.MovementStopped)

		// Check economy: 30 - 10 = 20 remaining
		economy := tm.GetEconomy()
		s.Equal(20, economy.MovementRemaining)
	})
}

func (s *TurnManagerTestSuite) TestMovement_InsufficientMovement() {
	s.Run("errors when insufficient movement", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Try to move 7 steps (35 feet) - more than 30ft speed
		path := make([]spatial.Position, 8) // 7 steps
		for i := range path {
			path[i] = spatial.Position{X: float64(2 + i), Y: 2}
		}

		_, err = tm.Move(s.ctx, &combat.MoveInput{Path: path})
		s.Require().Error(err)
		s.Contains(err.Error(), "movement")
	})
}

// --- Dash + Extended Movement ---

func (s *TurnManagerTestSuite) TestDashExtendsMovement() {
	s.Run("dash adds speed to movement", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Use Dash ability
		_, err = tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Dash(),
		})
		s.Require().NoError(err)

		// Economy should now have 60ft movement (30 base + 30 dash)
		economy := tm.GetEconomy()
		s.Equal(60, economy.MovementRemaining)
		s.Equal(0, economy.ActionsRemaining) // Action consumed by Dash
	})
}

// --- Economy Exhaustion ---

func (s *TurnManagerTestSuite) TestEconomyExhaustion() {
	s.Run("cannot use ability without action remaining", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Use Attack (consumes action)
		_, err = tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().NoError(err)

		// Try to use Dash (also requires action) - should fail
		_, err = tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Dash(),
		})
		s.Require().Error(err)
	})
}

func (s *TurnManagerTestSuite) TestStrikeWithoutAttackAbility() {
	s.Run("strike fails without using attack ability first", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Try to strike without using Attack ability first
		_, err = tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: "goblin-1",
			Weapon:   s.weapon,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "attack")
	})
}

// --- Disengage Prevents OA ---

func (s *TurnManagerTestSuite) TestDisengagePreventsOA() {
	s.Run("disengage prevents opportunity attacks when moving away", func() {
		// Subscribe to DisengageActivatedEvent to apply the Disengaging condition
		// (simulates what the game server would do)
		disengageTopic := dnd5eEvents.DisengageActivatedTopic.On(s.bus)
		_, err := disengageTopic.Subscribe(s.ctx, func(ctx context.Context, event dnd5eEvents.DisengageActivatedEvent) error {
			cond := conditions.NewDisengagingCondition(event.CharacterID)
			return cond.Apply(ctx, s.bus)
		})
		s.Require().NoError(err)

		tm := s.createTurnManager()
		_, err = tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Use Disengage ability
		_, err = tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Disengage(),
		})
		s.Require().NoError(err)

		// Move away from goblin (at position 3,2 - adjacent to fighter at 2,2)
		path := []spatial.Position{
			{X: 2, Y: 2}, // Starting
			{X: 1, Y: 2}, // Moving away
			{X: 0, Y: 2}, // Further away
		}

		result, err := tm.Move(s.ctx, &combat.MoveInput{Path: path})
		s.Require().NoError(err)

		// Movement should complete without being stopped by OA
		s.False(result.MovementStopped)
		s.Equal(2, result.StepsCompleted)
		s.Empty(result.OAsTriggered) // No OAs because of Disengage
	})
}

// --- Query Tests ---

func (s *TurnManagerTestSuite) TestGetAvailableAbilities() {
	s.Run("returns all abilities with availability", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		available := tm.GetAvailableAbilities(s.ctx)
		s.Require().Len(available, 4) // Attack, Dash, Disengage, Dodge

		// All should be usable initially
		for _, a := range available {
			s.True(a.CanUse, "ability %s should be usable", a.Info.Name)
		}

		// Use Attack (consumes action)
		_, err = tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().NoError(err)

		// Now standard-action abilities should not be usable
		available = tm.GetAvailableAbilities(s.ctx)
		for _, a := range available {
			s.False(a.CanUse, "ability %s should not be usable after action spent", a.Info.Name)
		}
	})
}

func (s *TurnManagerTestSuite) TestGetEconomy() {
	s.Run("returns current economy state", func() {
		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		economy := tm.GetEconomy()
		s.Equal(1, economy.ActionsRemaining)
		s.Equal(1, economy.BonusActionsRemaining)
		s.Equal(1, economy.ReactionsRemaining)
		s.Equal(30, economy.MovementRemaining)
		s.Equal(0, economy.AttacksRemaining)
	})
}

// --- Actions Before Turn Started ---

func (s *TurnManagerTestSuite) TestActionsBeforeTurnStarted() {
	s.Run("all actions fail before start turn", func() {
		tm := s.createTurnManager()

		_, err := tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "not started")

		_, err = tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: "goblin-1",
			Weapon:   s.weapon,
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "not started")

		_, err = tm.Move(s.ctx, &combat.MoveInput{
			Path: []spatial.Position{{X: 0, Y: 0}, {X: 1, Y: 0}},
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "not started")
	})
}

func TestTurnManagerSuite(t *testing.T) {
	suite.Run(t, new(TurnManagerTestSuite))
}
