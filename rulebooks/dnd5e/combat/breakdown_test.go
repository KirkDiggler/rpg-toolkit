package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type BreakdownTestSuite struct {
	suite.Suite
	ctrl     *gomock.Controller
	ctx      context.Context
	eventBus events.EventBus
}

func TestBreakdownSuite(t *testing.T) {
	suite.Run(t, new(BreakdownTestSuite))
}

func (s *BreakdownTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.eventBus = events.NewEventBus()
}

func (s *BreakdownTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *BreakdownTestSuite) TestResolveAttack_DamageBreakdown_BasicMelee() {
	// Create attacker with STR modifier
	attackerScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10, // +0 modifier
	}

	attacker := monster.New(monster.Config{
		ID:            "barbarian-1",
		Name:          "Barbarian",
		HP:            50,
		AC:            15,
		AbilityScores: attackerScores,
	})

	goblin := monster.NewGoblin("goblin-1")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: 15 on d20, [5] on d8
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil)

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.True(result.Hit)

	// Verify damage breakdown exists
	s.Require().NotNil(result.Breakdown, "Breakdown should be populated for hits")

	// Verify base damage
	s.Equal(5, result.Breakdown.BaseDamage, "BaseDamage should be sum of dice rolls")
	s.Equal([]int{5}, result.Breakdown.DiceRolls, "DiceRolls should match rolled values")

	// Verify ability modifier
	s.Equal(3, result.Breakdown.AbilityModifier, "AbilityModifier should be STR +3")
	s.Equal("STR", result.Breakdown.AbilityUsed, "AbilityUsed should be STR for melee")

	// Verify no other bonuses
	s.Equal(0, result.Breakdown.RageBonus, "RageBonus should be 0 when not raging")
	s.Equal(0, result.Breakdown.OtherBonuses, "OtherBonuses should be 0")

	// Verify totals
	s.Equal(3, result.Breakdown.TotalBonus, "TotalBonus should equal ability modifier")
	s.Equal(8, result.Breakdown.TotalDamage, "TotalDamage should be 5 + 3 = 8")
}

func (s *BreakdownTestSuite) TestResolveAttack_DamageBreakdown_WithRage() {
	// Create raging barbarian
	attackerScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10, // +0 modifier
	}

	attacker := monster.New(monster.Config{
		ID:            "barbarian-1",
		Name:          "Barbarian",
		HP:            50,
		AC:            15,
		AbilityScores: attackerScores,
	})

	// Apply rage condition (level 1 barbarian has +2 rage bonus)
	raging := &conditions.RagingCondition{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       1,
		Source:      "class",
	}
	err := raging.Apply(s.ctx, s.eventBus)
	s.Require().NoError(err)

	goblin := monster.NewGoblin("goblin-1")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: 15 on d20, [6] on d8
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{6}, nil)

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.True(result.Hit)

	// Verify damage breakdown with rage
	s.Require().NotNil(result.Breakdown)

	s.Equal(6, result.Breakdown.BaseDamage)
	s.Equal(3, result.Breakdown.AbilityModifier, "STR modifier +3")
	s.Equal("STR", result.Breakdown.AbilityUsed)
	s.Equal(2, result.Breakdown.RageBonus, "Rage adds +2 at level 1")
	s.Equal(0, result.Breakdown.OtherBonuses)

	// Total bonus should be ability + rage
	s.Equal(5, result.Breakdown.TotalBonus, "TotalBonus = 3 (STR) + 2 (rage)")
	s.Equal(11, result.Breakdown.TotalDamage, "TotalDamage = 6 (base) + 5 (bonus)")
}

func (s *BreakdownTestSuite) TestResolveAttack_DamageBreakdown_FinesseWeapon() {
	// Create attacker with higher DEX than STR
	attackerScores := shared.AbilityScores{
		abilities.STR: 12, // +1 modifier
		abilities.DEX: 18, // +4 modifier
	}

	attacker := monster.New(monster.Config{
		ID:            "rogue-1",
		Name:          "Rogue",
		HP:            30,
		AC:            15,
		AbilityScores: attackerScores,
	})

	goblin := monster.NewGoblin("goblin-1")

	// Rapier is a finesse weapon
	rapier := &weapons.Weapon{
		ID:         weapons.Rapier,
		Name:       "Rapier",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Properties: []weapons.WeaponProperty{weapons.PropertyFinesse},
	}

	// Mock roller: 15 on d20, [5] on d8
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil)

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           rapier,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 3,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.True(result.Hit)

	// Verify finesse weapon uses DEX
	s.Require().NotNil(result.Breakdown)
	s.Equal(4, result.Breakdown.AbilityModifier, "Should use DEX +4, not STR +1")
	s.Equal("DEX", result.Breakdown.AbilityUsed, "Finesse weapon should use DEX when higher")
	s.Equal(5, result.Breakdown.BaseDamage)
	s.Equal(9, result.Breakdown.TotalDamage, "5 (base) + 4 (DEX)")
}

func (s *BreakdownTestSuite) TestResolveAttack_DamageBreakdown_CriticalHit() {
	attackerScores := shared.AbilityScores{
		abilities.STR: 14, // +2 modifier
	}

	attacker := monster.New(monster.Config{
		ID:            "fighter-1",
		Name:          "Fighter",
		HP:            40,
		AC:            16,
		AbilityScores: attackerScores,
	})

	goblin := monster.NewGoblin("goblin-1")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Natural 20, then two d8 rolls for critical (doubled dice)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{6}, nil).Times(2)

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.True(result.Critical, "Natural 20 should be critical")
	s.True(result.Hit)

	// Verify critical doubles dice, not bonuses
	s.Require().NotNil(result.Breakdown)
	s.Equal(12, result.Breakdown.BaseDamage, "Critical: 6 + 6 = 12")
	s.Equal([]int{6, 6}, result.Breakdown.DiceRolls, "Should have two dice rolls")
	s.Equal(2, result.Breakdown.AbilityModifier, "STR modifier (not doubled)")
	s.Equal("STR", result.Breakdown.AbilityUsed)
	s.Equal(2, result.Breakdown.TotalBonus, "Bonuses are NOT doubled on crit")
	s.Equal(14, result.Breakdown.TotalDamage, "12 (doubled dice) + 2 (ability)")
}

func (s *BreakdownTestSuite) TestResolveAttack_DamageBreakdown_Miss() {
	attackerScores := shared.AbilityScores{
		abilities.STR: 10, // +0 modifier
	}

	attacker := monster.New(monster.Config{
		ID:            "fighter-1",
		Name:          "Fighter",
		HP:            40,
		AC:            16,
		AbilityScores: attackerScores,
	})

	goblin := monster.NewGoblin("goblin-1")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Roll 5 on d20, +0 bonus = 5 total, misses AC 15
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 0,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.False(result.Hit, "Should miss")

	// Verify no breakdown on miss
	s.Nil(result.Breakdown, "Breakdown should be nil when attack misses")
	s.Equal(0, result.TotalDamage, "No damage on miss")
}
