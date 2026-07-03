package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	mock_combat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/mock"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// testCombatant implements core.Entity for movement testing
type testCombatant struct {
	id         string
	entityType core.EntityType
}

func (t *testCombatant) GetID() string            { return t.id }
func (t *testCombatant) GetType() core.EntityType { return t.entityType }

type MovementTestSuite struct {
	suite.Suite
	ctrl     *gomock.Controller
	ctx      context.Context
	eventBus events.EventBus
	lookup   *mock_combat.MockCombatantLookup
	room     *spatial.BasicRoom
}

func TestMovementSuite(t *testing.T) {
	suite.Run(t, new(MovementTestSuite))
}

func (s *MovementTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.eventBus = events.NewEventBus()
	s.lookup = mock_combat.NewMockCombatantLookup(s.ctrl)

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
	s.room.ConnectToEventBus(s.eventBus)

	// Set up context with room and combatant lookup
	s.ctx = combat.WithRoom(context.Background(), s.room)
	s.ctx = combat.WithCombatantLookup(s.ctx, s.lookup)
}

func (s *MovementTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *MovementTestSuite) TestMoveEntity_BasicMovementNoThreats() {
	// Place fighter at position (2, 2)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Move fighter from (2,2) to (4,2) - 2 steps right, no enemies nearby
	path := []spatial.Position{
		{X: 3, Y: 2},
		{X: 4, Y: 2},
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(spatial.Position{X: 4, Y: 2}, result.FinalPosition)
	s.Equal(2, result.StepsCompleted)
	s.Empty(result.OAsTriggered, "no opportunity attacks should be triggered")
	s.False(result.MovementStopped)
}

func (s *MovementTestSuite) TestMoveEntity_TriggersOpportunityAttack() {
	// Place fighter at (2, 2)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Place goblin at (2, 3) - adjacent to fighter (within 5ft reach)
	goblin := &testCombatant{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 2, Y: 3})
	s.Require().NoError(err)

	// Create mock combatants for attack resolution
	mockFighter := mock_combat.NewMockCombatant(s.ctrl)
	mockFighter.EXPECT().GetID().Return("fighter-1").AnyTimes()
	mockFighter.EXPECT().AC().Return(16).AnyTimes()

	mockGoblin := mock_combat.NewMockCombatant(s.ctrl)
	mockGoblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	mockGoblin.EXPECT().AbilityScores().Return(shared.AbilityScores{
		abilities.STR: 8,  // -1 modifier
		abilities.DEX: 14, // +2 modifier
	}).AnyTimes()
	mockGoblin.EXPECT().ProficiencyBonus().Return(2).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(mockFighter, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(mockGoblin, nil).AnyTimes()

	// Mock dice roller for OA: roll 18 on d20 (hit), 3 damage
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(18, nil)
	mockRoller.EXPECT().RollN(gomock.Any(), 1, 1).Return([]int{1}, nil) // Unarmed strike = 1 damage

	// Move fighter away from goblin - from (2,2) to (2,0) - moving away triggers OA
	path := []spatial.Position{
		{X: 2, Y: 1}, // Still adjacent
		{X: 2, Y: 0}, // Leaving goblin's reach
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Fighter should complete movement
	s.Equal(spatial.Position{X: 2, Y: 0}, result.FinalPosition)
	s.Equal(2, result.StepsCompleted)
	s.False(result.MovementStopped)

	// Should have triggered one opportunity attack from goblin
	s.Require().Len(result.OAsTriggered, 1, "goblin should have made opportunity attack")
	s.Equal("goblin-1", result.OAsTriggered[0].AttackerID)
	s.True(result.OAsTriggered[0].Hit, "18 should hit AC 16")
}

func (s *MovementTestSuite) TestMoveEntity_DisengagingPreventsOA() {
	// Place fighter at (2, 2)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Place goblin at (2, 3) - adjacent to fighter
	goblin := &testCombatant{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 2, Y: 3})
	s.Require().NoError(err)

	// Subscribe to movement chain to simulate Disengaging condition
	movementTopic := dnd5eEvents.MovementChain.On(s.eventBus)
	_, err = movementTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		event *dnd5eEvents.MovementChainEvent,
		c chain.Chain[*dnd5eEvents.MovementChainEvent],
	) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
		// Only apply to fighter-1
		if event.EntityID != "fighter-1" {
			return c, nil
		}

		// Add Disengaging condition's OA prevention
		addDisengaging := func(_ context.Context, e *dnd5eEvents.MovementChainEvent) (*dnd5eEvents.MovementChainEvent, error) {
			e.OAPreventionSources = append(e.OAPreventionSources, dnd5eEvents.MovementModifierSource{
				Name:       "Disengaging",
				SourceType: "condition",
				SourceRef:  refs.Conditions.Disengaging(),
				EntityID:   "fighter-1",
			})
			return e, nil
		}
		return c, c.Add(combat.StageConditions, "disengaging", addDisengaging)
	})
	s.Require().NoError(err)

	// Move fighter away from goblin
	path := []spatial.Position{
		{X: 2, Y: 1},
		{X: 2, Y: 0},
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Fighter should complete movement
	s.Equal(spatial.Position{X: 2, Y: 0}, result.FinalPosition)
	s.Equal(2, result.StepsCompleted)

	// NO opportunity attacks should be triggered due to Disengaging
	s.Empty(result.OAsTriggered, "Disengaging should prevent opportunity attacks")
}

func (s *MovementTestSuite) TestMoveEntity_MultipleThreateningEntities() {
	// Place fighter at (3, 3)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 3, Y: 3})
	s.Require().NoError(err)

	// Place two goblins adjacent to fighter
	goblin1 := &testCombatant{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin1, spatial.Position{X: 3, Y: 4})
	s.Require().NoError(err)

	goblin2 := &testCombatant{id: "goblin-2", entityType: "monster"}
	err = s.room.PlaceEntity(goblin2, spatial.Position{X: 4, Y: 3})
	s.Require().NoError(err)

	// Create mock combatants
	mockFighter := mock_combat.NewMockCombatant(s.ctrl)
	mockFighter.EXPECT().GetID().Return("fighter-1").AnyTimes()
	mockFighter.EXPECT().AC().Return(16).AnyTimes()

	mockGoblin1 := mock_combat.NewMockCombatant(s.ctrl)
	mockGoblin1.EXPECT().GetID().Return("goblin-1").AnyTimes()
	mockGoblin1.EXPECT().AbilityScores().Return(shared.AbilityScores{
		abilities.STR: 8,
		abilities.DEX: 14,
	}).AnyTimes()
	mockGoblin1.EXPECT().ProficiencyBonus().Return(2).AnyTimes()

	mockGoblin2 := mock_combat.NewMockCombatant(s.ctrl)
	mockGoblin2.EXPECT().GetID().Return("goblin-2").AnyTimes()
	mockGoblin2.EXPECT().AbilityScores().Return(shared.AbilityScores{
		abilities.STR: 8,
		abilities.DEX: 14,
	}).AnyTimes()
	mockGoblin2.EXPECT().ProficiencyBonus().Return(2).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(mockFighter, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(mockGoblin1, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-2").Return(mockGoblin2, nil).AnyTimes()

	// Mock dice for two OAs
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	// First OA (goblin-1): miss
	mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(5, nil)
	// Second OA (goblin-2): hit
	mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(18, nil)
	mockRoller.EXPECT().RollN(gomock.Any(), 1, 1).Return([]int{1}, nil)

	// Move fighter diagonally away from both goblins
	path := []spatial.Position{
		{X: 2, Y: 2}, // Leaving both goblins' reach
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(spatial.Position{X: 2, Y: 2}, result.FinalPosition)

	// Both goblins should have made opportunity attacks
	s.Require().Len(result.OAsTriggered, 2, "both goblins should make OAs")
}

func (s *MovementTestSuite) TestMoveEntity_MovementPreventedByModifier() {
	// Place fighter at (2, 2)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Subscribe to movement chain to simulate a feature that prevents movement
	movementTopic := dnd5eEvents.MovementChain.On(s.eventBus)
	_, err = movementTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		event *dnd5eEvents.MovementChainEvent,
		c chain.Chain[*dnd5eEvents.MovementChainEvent],
	) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
		// Simulate a feature that prevents movement (like being grappled)
		preventMovement := func(_ context.Context, e *dnd5eEvents.MovementChainEvent) (*dnd5eEvents.MovementChainEvent, error) {
			e.MovementPrevented = true
			e.PreventionReason = "grappled by ogre"
			return e, nil
		}
		return c, c.Add(combat.StageConditions, "grappled", preventMovement)
	})
	s.Require().NoError(err)

	path := []spatial.Position{
		{X: 3, Y: 2},
		{X: 4, Y: 2},
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Movement should be stopped
	s.True(result.MovementStopped)
	s.Equal("grappled by ogre", result.StopReason)
	s.Equal(0, result.StepsCompleted)
	// Final position should be starting position since no movement occurred
	s.Equal(spatial.Position{X: 2, Y: 2}, result.FinalPosition)
}

func (s *MovementTestSuite) TestMoveEntity_MovementChainEventFired() {
	// Place fighter at (2, 2)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Track chain events
	var chainEvents []*dnd5eEvents.MovementChainEvent
	movementTopic := dnd5eEvents.MovementChain.On(s.eventBus)
	_, err = movementTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		event *dnd5eEvents.MovementChainEvent,
		c chain.Chain[*dnd5eEvents.MovementChainEvent],
	) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
		// Make a copy for tracking
		eventCopy := *event
		chainEvents = append(chainEvents, &eventCopy)
		return c, nil
	})
	s.Require().NoError(err)

	path := []spatial.Position{
		{X: 3, Y: 2},
		{X: 4, Y: 2},
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Should have fired 2 chain events (one per step)
	s.Require().Len(chainEvents, 2)

	// Verify first step event
	s.Equal("fighter-1", chainEvents[0].EntityID)
	s.Equal("character", chainEvents[0].EntityType)
	s.Equal(dnd5eEvents.Position{X: 2, Y: 2}, chainEvents[0].FromPosition)
	s.Equal(dnd5eEvents.Position{X: 3, Y: 2}, chainEvents[0].ToPosition)

	// Verify second step event
	s.Equal(dnd5eEvents.Position{X: 3, Y: 2}, chainEvents[1].FromPosition)
	s.Equal(dnd5eEvents.Position{X: 4, Y: 2}, chainEvents[1].ToPosition)
}

func (s *MovementTestSuite) TestMoveEntity_ValidationErrors() {
	s.Run("nil input", func() {
		result, err := combat.MoveEntity(s.ctx, nil)
		s.Require().Error(err)
		s.Nil(result)
		s.Contains(err.Error(), "nil")
	})

	s.Run("empty entity ID", func() {
		input := &combat.MoveEntityInput{
			EntityID:   "",
			EntityType: "character",
			Path:       []spatial.Position{{X: 1, Y: 1}},
			EventBus:   s.eventBus,
		}
		result, err := combat.MoveEntity(s.ctx, input)
		s.Require().Error(err)
		s.Nil(result)
		s.Contains(err.Error(), "EntityID")
	})

	s.Run("empty entity type", func() {
		input := &combat.MoveEntityInput{
			EntityID:   "fighter-1",
			EntityType: "",
			Path:       []spatial.Position{{X: 1, Y: 1}},
			EventBus:   s.eventBus,
		}
		result, err := combat.MoveEntity(s.ctx, input)
		s.Require().Error(err)
		s.Nil(result)
		s.Contains(err.Error(), "EntityType")
	})

	s.Run("empty path", func() {
		input := &combat.MoveEntityInput{
			EntityID:   "fighter-1",
			EntityType: "character",
			Path:       []spatial.Position{},
			EventBus:   s.eventBus,
		}
		result, err := combat.MoveEntity(s.ctx, input)
		s.Require().Error(err)
		s.Nil(result)
		s.Contains(err.Error(), "Path")
	})

	s.Run("nil event bus", func() {
		input := &combat.MoveEntityInput{
			EntityID:   "fighter-1",
			EntityType: "character",
			Path:       []spatial.Position{{X: 1, Y: 1}},
			EventBus:   nil,
		}
		result, err := combat.MoveEntity(s.ctx, input)
		s.Require().Error(err)
		s.Nil(result)
		s.Contains(err.Error(), "EventBus")
	})
}

func (s *MovementTestSuite) TestMoveEntity_EntityNotInRoom() {
	// Don't place any entity in the room
	path := []spatial.Position{
		{X: 3, Y: 2},
	}

	input := &combat.MoveEntityInput{
		EntityID:   "nonexistent-entity",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "not found")
}

func (s *MovementTestSuite) TestMoveEntity_NoRoomInContext() {
	// Create context without room
	ctxNoRoom := combat.WithCombatantLookup(context.Background(), s.lookup)

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       []spatial.Position{{X: 1, Y: 1}},
		EventBus:   s.eventBus,
	}

	result, err := combat.MoveEntity(ctxNoRoom, input)
	s.Require().Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "room")
}

// armedTestCombatant wraps a combat.Combatant mock and additionally
// implements combat.MeleeWeaponProvider, so tests can control which weapon
// an OA attacker swings without regenerating the gomock MockCombatant to
// cover the new optional interface (rpg-toolkit#722).
type armedTestCombatant struct {
	combat.Combatant
	weapon *weapons.Weapon
}

// MeleeWeapon implements combat.MeleeWeaponProvider.
func (a *armedTestCombatant) MeleeWeapon() *weapons.Weapon { return a.weapon }

// TestMoveEntity_OAUsesAttackersRealWeapon is the failing-first regression
// test for rpg-toolkit#722: an OA attacker with a real melee weapon
// (goblin + scimitar) must resolve the reaction attack with that weapon's
// damage dice and damage type — not the unarmed-strike fallback.
//
// Before the fix, getAttackerMeleeWeapon always returned the catalog
// unarmed-strike weapon regardless of the attacker, so this OA would deal
// unarmed damage (1 bludgeoning) instead of the scimitar's 1d6+DEX
// slashing.
func (s *MovementTestSuite) TestMoveEntity_OAUsesAttackersRealWeapon() {
	// Place fighter at (2, 2), goblin at (2, 3) - adjacent (within reach).
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	s.Require().NoError(s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2}))
	goblin := &testCombatant{id: "goblin-1", entityType: "monster"}
	s.Require().NoError(s.room.PlaceEntity(goblin, spatial.Position{X: 2, Y: 3}))

	mockFighter := mock_combat.NewMockCombatant(s.ctrl)
	mockFighter.EXPECT().GetID().Return("fighter-1").AnyTimes()
	mockFighter.EXPECT().AC().Return(16).AnyTimes()

	mockGoblin := mock_combat.NewMockCombatant(s.ctrl)
	mockGoblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	mockGoblin.EXPECT().AbilityScores().Return(shared.AbilityScores{
		abilities.STR: 8,  // -1 modifier
		abilities.DEX: 14, // +2 modifier — scimitar is Finesse, so DEX applies
	}).AnyTimes()
	mockGoblin.EXPECT().ProficiencyBonus().Return(2).AnyTimes()

	scimitar, err := weapons.GetByID(weapons.Scimitar)
	s.Require().NoError(err)
	armedGoblin := &armedTestCombatant{Combatant: mockGoblin, weapon: &scimitar}

	s.lookup.EXPECT().Get("fighter-1").Return(mockFighter, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(armedGoblin, nil).AnyTimes()

	// d20=18 hits AC 16 (18 + 2 DEX + 2 prof = 22). Damage die rolls max (6)
	// on the scimitar's 1d6, plus the +2 DEX modifier = 8 total, matching
	// the "scimitar 8 (6+2) slashing" shape from the #722 playtest report.
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(18, nil)
	mockRoller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{6}, nil)

	var damageEvt *dnd5eEvents.DamageReceivedEvent
	damages := dnd5eEvents.DamageReceivedTopic.On(s.eventBus)
	_, err = damages.Subscribe(s.ctx, func(_ context.Context, e dnd5eEvents.DamageReceivedEvent) error {
		damageEvt = &e
		return nil
	})
	s.Require().NoError(err)

	path := []spatial.Position{
		{X: 2, Y: 1},
		{X: 2, Y: 0},
	}
	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Require().Len(result.OAsTriggered, 1)
	s.True(result.OAsTriggered[0].Hit)
	s.Equal(8, result.OAsTriggered[0].Damage, "scimitar 1d6(6)+DEX(2) = 8, not unarmed's 1")

	s.Require().NotNil(damageEvt, "OA hit must publish a DamageReceivedEvent")
	s.Equal(8, damageEvt.Amount)
	s.Equal(damage.Slashing, damageEvt.DamageType, "scimitar deals slashing, not unarmed's bludgeoning")
}

func (s *MovementTestSuite) TestMoveEntity_OAMissDoesNotStopMovement() {
	// Place fighter at (2, 2)
	fighter := &testCombatant{id: "fighter-1", entityType: "character"}
	err := s.room.PlaceEntity(fighter, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Place goblin at (2, 3)
	goblin := &testCombatant{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 2, Y: 3})
	s.Require().NoError(err)

	// Create mock combatants
	mockFighter := mock_combat.NewMockCombatant(s.ctrl)
	mockFighter.EXPECT().GetID().Return("fighter-1").AnyTimes()
	mockFighter.EXPECT().AC().Return(20).AnyTimes() // High AC to ensure miss

	mockGoblin := mock_combat.NewMockCombatant(s.ctrl)
	mockGoblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	mockGoblin.EXPECT().AbilityScores().Return(shared.AbilityScores{
		abilities.STR: 8,
		abilities.DEX: 14,
	}).AnyTimes()
	mockGoblin.EXPECT().ProficiencyBonus().Return(2).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(mockFighter, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(mockGoblin, nil).AnyTimes()

	// Mock dice for OA: roll 5 (miss against AC 20)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(5, nil)

	// Move fighter away
	path := []spatial.Position{
		{X: 2, Y: 1},
		{X: 2, Y: 0},
	}

	input := &combat.MoveEntityInput{
		EntityID:   "fighter-1",
		EntityType: "character",
		Path:       path,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.MoveEntity(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Movement should complete even though OA was triggered
	s.Equal(spatial.Position{X: 2, Y: 0}, result.FinalPosition)
	s.Equal(2, result.StepsCompleted)
	s.False(result.MovementStopped)

	// OA should be recorded as a miss
	s.Require().Len(result.OAsTriggered, 1)
	s.False(result.OAsTriggered[0].Hit, "OA should miss")
	s.Equal(0, result.OAsTriggered[0].Damage)
}
