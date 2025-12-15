// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string            { return m.id }
func (m *mockEntity) GetType() core.EntityType { return m.entityType }

// SneakAttackTestSuite tests the SneakAttackCondition behavior
type SneakAttackTestSuite struct {
	suite.Suite
	ctrl   *gomock.Controller
	ctx    context.Context
	bus    events.EventBus
	roller *mock_dice.MockRoller
	room   spatial.Room
}

func (s *SneakAttackTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.roller = mock_dice.NewMockRoller(s.ctrl)

	// Set up room for spatial tests
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  20,
		Height: 20,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "dungeon",
		Grid: grid,
	})
}

func (s *SneakAttackTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestSneakAttackTestSuite(t *testing.T) {
	suite.Run(t, new(SneakAttackTestSuite))
}

// damageChainInput holds parameters for executeDamageChain
type damageChainInput struct {
	attackerID   string
	targetID     string
	abilityUsed  abilities.Ability
	hasAdvantage bool
}

// executeDamageChain creates a damage chain event and executes it.
func (s *SneakAttackTestSuite) executeDamageChain(input damageChainInput) (*dnd5eEvents.DamageChainEvent, error) {
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{5},
		FinalDiceRolls:    []int{5},
		DamageType:        damage.Piercing,
		IsCritical:        false,
	}

	targetID := input.targetID
	if targetID == "" {
		targetID = "goblin-1"
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   input.attackerID,
		TargetID:     targetID,
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		HasAdvantage: input.hasAdvantage,
		WeaponDamage: "1d6",
		AbilityUsed:  input.abilityUsed,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

// executeDamageChainSimple is a convenience wrapper for simple test cases
//
//nolint:unparam // abilityUsed is intentionally fixed to DEX for most tests
func (s *SneakAttackTestSuite) executeDamageChainSimple(
	attackerID string,
	abilityUsed abilities.Ability,
) (*dnd5eEvents.DamageChainEvent, error) {
	return s.executeDamageChain(damageChainInput{
		attackerID:   attackerID,
		abilityUsed:  abilityUsed,
		hasAdvantage: true, // Default to true so existing tests pass
	})
}

func (s *SneakAttackTestSuite) TestSneakAttackAddsDiceLevel1() {
	// Level 1 rogue gets 1d6 sneak attack
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect 1d6 to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	// Execute damage chain with DEX (finesse weapon) and advantage
	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	// Should have weapon + sneak attack components
	s.Require().Len(finalEvent.Components, 2, "Should have weapon and sneak attack components")

	// Verify sneak attack component uses DamageSourceFeature
	sneakComp := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, sneakComp.Source)
	s.Equal([]int{4}, sneakComp.FinalDiceRolls, "Should have rolled 1d6")
	s.Equal(4, sneakComp.Total(), "Sneak attack should add 4 damage")
}

func (s *SneakAttackTestSuite) TestSneakAttackAddsDiceLevel5() {
	// Level 5 rogue gets 3d6 sneak attack
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       5,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect 3d6 to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 3, 6).
		Return([]int{3, 5, 6}, nil)

	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 2)

	sneakComp := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, sneakComp.Source)
	s.Equal([]int{3, 5, 6}, sneakComp.FinalDiceRolls, "Should have rolled 3d6")
	s.Equal(14, sneakComp.Total(), "Sneak attack should add 14 damage (3+5+6)")
}

func (s *SneakAttackTestSuite) TestSneakAttackOnlyOncePerTurn() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// First attack - expect sneak attack
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "First attack should have sneak attack")

	// Second attack - NO sneak attack (already used this turn)
	// No roller expectation - RollN should NOT be called

	finalEvent2, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent2.Components, 1, "Second attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestSneakAttackResetsOnTurnEnd() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// First attack
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	_, err = s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	// End turn
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: "rogue-1",
		Round:       1,
	})
	s.Require().NoError(err)

	// Next turn - sneak attack should work again
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{6}, nil)

	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack after turn reset")
}

func (s *SneakAttackTestSuite) TestSneakAttackRequiresFinesseWeapon() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - attack with STR should not trigger sneak attack

	// Attack with STR (non-finesse weapon) - even with advantage
	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:   "rogue-1",
		abilityUsed:  abilities.STR,
		hasAdvantage: true,
	})
	s.Require().NoError(err)

	// Should only have weapon component (no sneak attack)
	s.Require().Len(finalEvent.Components, 1, "STR attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestSneakAttackOnlyAffectsOwnAttacks() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - different attacker should not trigger

	// Different character attacks
	finalEvent, err := s.executeDamageChainSimple("rogue-2", abilities.DEX)
	s.Require().NoError(err)

	// Should only have weapon component
	s.Require().Len(finalEvent.Components, 1, "Other character's attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestCalculateSneakAttackDice() {
	testCases := []struct {
		level      int
		damageDice int
	}{
		{level: 1, damageDice: 1},
		{level: 2, damageDice: 1},
		{level: 3, damageDice: 2},
		{level: 4, damageDice: 2},
		{level: 5, damageDice: 3},
		{level: 6, damageDice: 3},
		{level: 7, damageDice: 4},
		{level: 9, damageDice: 5},
		{level: 11, damageDice: 6},
		{level: 13, damageDice: 7},
		{level: 15, damageDice: 8},
		{level: 17, damageDice: 9},
		{level: 19, damageDice: 10},
		{level: 20, damageDice: 10},
	}

	for _, tc := range testCases {
		sneak := NewSneakAttackCondition(SneakAttackInput{
			CharacterID: "rogue-1",
			Level:       tc.level,
			Roller:      s.roller,
		})
		s.Equal(tc.damageDice, sneak.DamageDice, "Level %d should have %dd6", tc.level, tc.damageDice)
	}
}

// =============================================================================
// Sneak Attack Condition Tests (Advantage OR Ally Adjacent)
// =============================================================================

func (s *SneakAttackTestSuite) TestSneakAttackTriggersWithAdvantage() {
	// Sneak attack should trigger when attacker has advantage
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect sneak attack dice to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	// Attack with advantage (no ally nearby)
	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:   "rogue-1",
		abilityUsed:  abilities.DEX,
		hasAdvantage: true,
	})
	s.Require().NoError(err)

	// Should have sneak attack
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack with advantage")
}

func (s *SneakAttackTestSuite) TestSneakAttackTriggersWithAllyAdjacent() {
	// Place entities in room
	// Rogue at (2, 2) - not adjacent to goblin
	rogue := &mockEntity{id: "rogue-1", entityType: "character"}
	err := s.room.PlaceEntity(rogue, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Goblin (target) at (5, 5) - monster type
	goblin := &mockEntity{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)

	// Fighter (ally) at (5, 6) - adjacent to goblin, character type = ally
	fighter := &mockEntity{id: "fighter-1", entityType: "character"}
	err = s.room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 6})
	s.Require().NoError(err)

	// Create context with room
	ctx := gamectx.WithRoom(s.ctx, s.room)

	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err = sneak.Apply(ctx, s.bus)
	s.Require().NoError(err)

	// Expect sneak attack dice to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{5}, nil)

	// Execute damage chain WITHOUT advantage but WITH ally adjacent
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{5},
		FinalDiceRolls:    []int{5},
		DamageType:        damage.Piercing,
		IsCritical:        false,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   "rogue-1",
		TargetID:     "goblin-1",
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		HasAdvantage: false, // No advantage
		WeaponDamage: "1d6",
		AbilityUsed:  abilities.DEX,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(ctx, damageEvent, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Should have sneak attack due to ally adjacent
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack with ally adjacent")
}

func (s *SneakAttackTestSuite) TestSneakAttackDoesNotTriggerWithoutConditions() {
	// Sneak attack should NOT trigger without advantage OR ally adjacent
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - sneak attack should NOT be rolled

	// Attack without advantage and without ally adjacent (no room/teams context)
	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:   "rogue-1",
		abilityUsed:  abilities.DEX,
		hasAdvantage: false, // No advantage
	})
	s.Require().NoError(err)

	// Should NOT have sneak attack
	s.Require().Len(finalEvent.Components, 1, "Should NOT have sneak attack without conditions")
}

//nolint:dupl // Test functions intentionally similar - different entity positions
func (s *SneakAttackTestSuite) TestSneakAttackDoesNotTriggerWhenAllyTooFar() {
	// Place entities in room
	// Rogue at (2, 2)
	rogue := &mockEntity{id: "rogue-1", entityType: "character"}
	err := s.room.PlaceEntity(rogue, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Goblin (target) at (5, 5)
	goblin := &mockEntity{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)

	// Fighter (ally) at (10, 10) - NOT adjacent to goblin (too far)
	fighter := &mockEntity{id: "fighter-1", entityType: "character"}
	err = s.room.PlaceEntity(fighter, spatial.Position{X: 10, Y: 10})
	s.Require().NoError(err)

	// Create context with room
	ctx := gamectx.WithRoom(s.ctx, s.room)

	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err = sneak.Apply(ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - sneak attack should NOT trigger

	// Execute damage chain WITHOUT advantage and ally too far
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{5},
		FinalDiceRolls:    []int{5},
		DamageType:        damage.Piercing,
		IsCritical:        false,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   "rogue-1",
		TargetID:     "goblin-1",
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		HasAdvantage: false,
		WeaponDamage: "1d6",
		AbilityUsed:  abilities.DEX,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(ctx, damageEvent, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Should NOT have sneak attack - ally is too far
	s.Require().Len(finalEvent.Components, 1, "Should NOT have sneak attack when ally too far")
}

//nolint:dupl // Test functions intentionally similar - different entity positions
func (s *SneakAttackTestSuite) TestSneakAttackDoesNotTriggerWhenOnlyEnemyAdjacent() {
	// Sneak attack should NOT trigger if the only nearby entity is an enemy (monster type)

	// Rogue at (2, 2)
	rogue := &mockEntity{id: "rogue-1", entityType: "character"}
	err := s.room.PlaceEntity(rogue, spatial.Position{X: 2, Y: 2})
	s.Require().NoError(err)

	// Target goblin at (5, 5)
	goblin := &mockEntity{id: "goblin-1", entityType: "monster"}
	err = s.room.PlaceEntity(goblin, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)

	// Another enemy goblin at (5, 6) - adjacent to target but monster type = NOT an ally
	goblin2 := &mockEntity{id: "goblin-2", entityType: "monster"}
	err = s.room.PlaceEntity(goblin2, spatial.Position{X: 5, Y: 6})
	s.Require().NoError(err)

	ctx := gamectx.WithRoom(s.ctx, s.room)

	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err = sneak.Apply(ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation

	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{5},
		FinalDiceRolls:    []int{5},
		DamageType:        damage.Piercing,
		IsCritical:        false,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   "rogue-1",
		TargetID:     "goblin-1",
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		HasAdvantage: false,
		WeaponDamage: "1d6",
		AbilityUsed:  abilities.DEX,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(ctx, damageEvent, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Should NOT have sneak attack - adjacent entity is enemy not ally
	s.Require().Len(finalEvent.Components, 1, "Should NOT have sneak attack when only enemy adjacent")
}

// Suppress unused import warning
var _ dice.Roller = (*mock_dice.MockRoller)(nil)
