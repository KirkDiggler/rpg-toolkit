package combat

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type AttackTestSuite struct {
	suite.Suite
	ctx      context.Context
	eventBus events.EventBus
}

func TestAttackSuite(t *testing.T) {
	suite.Run(t, new(AttackTestSuite))
}

func (s *AttackTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.eventBus = events.NewEventBus()
}

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id string
}

func (m *mockEntity) GetID() string {
	return m.id
}

func (m *mockEntity) GetType() core.EntityType {
	return dnd5e.EntityTypeCharacter
}

// mockRoller for predictable dice rolls
type mockRoller struct {
	d20Roll int
	d8Rolls []int
}

func (m *mockRoller) Roll(ctx context.Context, size int) (int, error) {
	if size == 20 {
		return m.d20Roll, nil
	}
	if size == 8 && len(m.d8Rolls) > 0 {
		roll := m.d8Rolls[0]
		m.d8Rolls = m.d8Rolls[1:]
		return roll, nil
	}
	return 1, nil // Default
}

func (m *mockRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
	rolls := make([]int, count)
	for i := 0; i < count; i++ {
		roll, _ := m.Roll(ctx, size)
		rolls[i] = roll
	}
	return rolls, nil
}

func (s *AttackTestSuite) TestResolveAttack_BasicMeleeHit() {
	// Create attacker with moderate STR
	attacker := &mockEntity{id: "attacker"}
	attackerScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10, // +0 modifier
	}

	// Create goblin target
	goblin := monster.NewGoblin("goblin-1")

	// Longsword
	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: 15 on d20, 5 on d8
	mockRoll := &mockRoller{
		d20Roll: 15,
		d8Rolls: []int{5},
	}

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(), // 15
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoll,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Attack: 15 (roll) + 3 (STR) + 2 (prof) = 20
	s.Equal(15, result.AttackRoll)
	s.Equal(5, result.AttackBonus, "STR(+3) + proficiency(+2)")
	s.Equal(20, result.TotalAttack)
	s.True(result.Hit, "20 should hit AC 15")
	s.False(result.Critical)

	// Damage: 5 (roll) + 3 (STR) = 8
	s.Equal([]int{5}, result.DamageRolls)
	s.Equal(3, result.DamageBonus, "STR modifier")
	s.Equal(8, result.TotalDamage)
	s.Equal("slashing", result.DamageType)
}

func (s *AttackTestSuite) TestResolveAttack_NaturalTwenty() {
	attacker := &mockEntity{id: "attacker"}
	attackerScores := shared.AbilityScores{
		abilities.STR: 10, // +0 modifier
	}
	goblin := monster.NewGoblin("goblin-1")
	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Natural 20 on attack, 5 on each damage die (2d8 on crit)
	mockRoll := &mockRoller{
		d20Roll: 20,
		d8Rolls: []int{5, 5}, // Two dice for critical
	}

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 0,
		EventBus:         s.eventBus,
		Roller:           mockRoll,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	s.Equal(20, result.AttackRoll)
	s.True(result.IsNaturalTwenty)
	s.True(result.Critical)
	s.True(result.Hit, "natural 20 always hits")

	// Critical doubles damage dice: 2d8 instead of 1d8
	s.Equal(2, len(result.DamageRolls), "critical should double damage dice")
	s.Equal([]int{5, 5}, result.DamageRolls)
	// Total: 5 + 5 (dice) + 0 (STR) = 10
	s.Equal(10, result.TotalDamage)
}

func (s *AttackTestSuite) TestResolveAttack_NaturalOne() {
	attacker := &mockEntity{id: "attacker"}
	attackerScores := shared.AbilityScores{
		abilities.STR: 20, // +5 modifier (still misses on nat 1)
	}
	goblin := monster.NewGoblin("goblin-1")
	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	mockRoll := &mockRoller{
		d20Roll: 1, // Natural 1
	}

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoll,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	s.Equal(1, result.AttackRoll)
	s.True(result.IsNaturalOne)
	s.False(result.Hit, "natural 1 always misses")
	s.False(result.Critical)
	s.Equal(0, result.TotalDamage, "miss deals no damage")
}

func (s *AttackTestSuite) TestResolveAttack_FinesseWeapon() {
	attacker := &mockEntity{id: "attacker"}
	attackerScores := shared.AbilityScores{
		abilities.STR: 10, // +0 modifier
		abilities.DEX: 18, // +4 modifier (should use this)
	}
	goblin := monster.NewGoblin("goblin-1")

	// Rapier is finesse
	rapier := &weapons.Weapon{
		ID:         weapons.Rapier,
		Name:       "Rapier",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Properties: []weapons.WeaponProperty{weapons.PropertyFinesse},
	}

	mockRoll := &mockRoller{
		d20Roll: 10,
		d8Rolls: []int{4},
	}

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           rapier,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoll,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	// Should use DEX (+4) instead of STR (+0)
	s.Equal(6, result.AttackBonus, "DEX(+4) + proficiency(+2)")
	s.Equal(16, result.TotalAttack)
	s.True(result.Hit)

	// Damage should also use DEX
	s.Equal(4, result.DamageBonus, "DEX modifier")
	s.Equal(8, result.TotalDamage, "4 (roll) + 4 (DEX)")
}

func (s *AttackTestSuite) TestResolveAttack_WithDamageModifier() {
	attacker := &mockEntity{id: "attacker"}
	attackerScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
	}
	goblin := monster.NewGoblin("goblin-1")
	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	mockRoll := &mockRoller{
		d20Roll: 15,
		d8Rolls: []int{5},
	}

	// Subscribe to damage chain to add +2 bonus (simulating Rage)
	damages := DamageChain.On(s.eventBus)
	damages.SubscribeWithChain(s.ctx, func(ctx context.Context, e DamageChainEvent, c chain.Chain[DamageChainEvent]) (chain.Chain[DamageChainEvent], error) {
		if e.AttackerID == "attacker" {
			c.Add(StageFeatures, "test-rage", func(ctx context.Context, e DamageChainEvent) (DamageChainEvent, error) {
				e.DamageBonus += 2
				return e, nil
			})
		}
		return c, nil
	})

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoll,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	s.True(result.Hit)
	// Damage: 5 (roll) + 3 (STR) + 2 (rage) = 10
	s.Equal(5, result.DamageBonus, "STR(+3) + rage(+2)")
	s.Equal(10, result.TotalDamage)
}

func (s *AttackTestSuite) TestParseDiceNotation() {
	tests := []struct {
		notation string
		wantNum  int
		wantSize int
		wantErr  bool
	}{
		{"1d8", 1, 8, false},
		{"2d6", 2, 6, false},
		{"3d10", 3, 10, false},
		{"1d20", 1, 20, false},
		{"invalid", 0, 0, true},
		{"d8", 0, 0, true},
		{"1d", 0, 0, true},
	}

	for _, tt := range tests {
		s.Run(tt.notation, func() {
			num, size, err := parseDiceNotation(tt.notation)
			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.wantNum, num)
				s.Equal(tt.wantSize, size)
			}
		})
	}
}
