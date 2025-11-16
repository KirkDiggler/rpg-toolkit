package features

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type RageCombatTestSuite struct {
	suite.Suite
	ctx         context.Context
	bus         events.EventBus
	integration *RageCombatIntegration
}

func TestRageCombatSuite(t *testing.T) {
	suite.Run(t, new(RageCombatTestSuite))
}

func (s *RageCombatTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.integration = NewRageCombatIntegration(RageCombatIntegrationConfig{
		Bus: s.bus,
	})
	err := s.integration.Start(s.ctx)
	s.Require().NoError(err)
}

func (s *RageCombatTestSuite) TearDownTest() {
	if s.integration != nil {
		_ = s.integration.Stop(s.ctx)
	}
}

func (s *RageCombatTestSuite) TestRageTracking() {
	barbarian := &StubEntity{id: "barbarian-1"}

	// Initially not raging
	s.False(s.integration.IsRaging("barbarian-1"))
	s.Equal(0, s.integration.GetRageDamageBonus("barbarian-1"))

	// Publish rage condition
	topic := dnd5e.ConditionAppliedTopic.On(s.bus)
	err := topic.Publish(s.ctx, dnd5e.ConditionAppliedEvent{
		Target: barbarian,
		Type:   dnd5e.ConditionRaging,
		Source: "rage-feature",
		Data: RageEventData{
			DamageBonus: 2,
			Level:       3,
		},
	})
	s.Require().NoError(err)

	// Should now be tracked as raging
	s.True(s.integration.IsRaging("barbarian-1"))
	s.Equal(2, s.integration.GetRageDamageBonus("barbarian-1"))

	// Clear rage
	s.integration.ClearRage("barbarian-1")
	s.False(s.integration.IsRaging("barbarian-1"))
}

func (s *RageCombatTestSuite) TestNonRageConditionIgnored() {
	barbarian := &StubEntity{id: "barbarian-1"}

	// Publish different condition type
	topic := dnd5e.ConditionAppliedTopic.On(s.bus)
	err := topic.Publish(s.ctx, dnd5e.ConditionAppliedEvent{
		Target: barbarian,
		Type:   "blessed", // Not rage
		Source: "cleric",
		Data:   nil,
	})
	s.Require().NoError(err)

	// Should not be tracked as raging
	s.False(s.integration.IsRaging("barbarian-1"))
}

func (s *RageCombatTestSuite) TestRageDamageBonus_Integration() {
	// Create a level 3 barbarian (should get +2 rage damage)
	barbarian := &StubEntity{id: "barbarian-1"}
	barbarianScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10,
	}

	// Create a goblin target
	goblin := monster.NewGoblin("goblin-1")

	// Longsword
	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: 15 on d20 (hits AC 15), 5 on d8
	mockRoll := &mockRoller{
		d20Roll: 15,
		d8Rolls: []int{5},
	}

	// Activate rage first
	rage := newRageForTest("rage-feature", 3)
	err := rage.Activate(s.ctx, barbarian, FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Perform attack
	attackInput := &combat.AttackInput{
		Attacker:         barbarian,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   barbarianScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           mockRoll,
	}

	result, err := combat.ResolveAttack(s.ctx, attackInput)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// Attack should hit
	s.True(result.Hit)

	// Damage: 5 (roll) + 3 (STR) + 2 (rage) = 10
	s.Equal(5, result.DamageBonus, "Should include STR(+3) + rage(+2)")
	s.Equal(10, result.TotalDamage, "5 (roll) + 3 (STR) + 2 (rage)")
}

func (s *RageCombatTestSuite) TestRageDamageBonus_HighLevel() {
	// Create a level 16 barbarian (should get +4 rage damage)
	barbarian := &StubEntity{id: "barbarian-2"}
	barbarianScores := shared.AbilityScores{
		abilities.STR: 20, // +5 modifier
		abilities.DEX: 10,
	}

	goblin := monster.NewGoblin("goblin-2")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	mockRoll := &mockRoller{
		d20Roll: 18,
		d8Rolls: []int{6},
	}

	// Activate level 16 rage
	rage := newRageForTest("rage-feature", 16)
	err := rage.Activate(s.ctx, barbarian, FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify rage damage bonus
	s.Equal(4, s.integration.GetRageDamageBonus("barbarian-2"))

	// Perform attack
	result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
		Attacker:         barbarian,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   barbarianScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 5,
		EventBus:         s.bus,
		Roller:           mockRoll,
	})
	s.Require().NoError(err)

	// Damage: 6 (roll) + 5 (STR) + 4 (rage) = 15
	s.Equal(9, result.DamageBonus, "Should include STR(+5) + rage(+4)")
	s.Equal(15, result.TotalDamage, "6 (roll) + 5 (STR) + 4 (rage)")
}

func (s *RageCombatTestSuite) TestNonRagingAttack() {
	// Barbarian without rage active
	barbarian := &StubEntity{id: "barbarian-3"}
	barbarianScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
	}

	goblin := monster.NewGoblin("goblin-3")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	mockRoll := &mockRoller{
		d20Roll: 15,
		d8Rolls: []int{5},
	}

	// Attack WITHOUT activating rage
	result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
		Attacker:         barbarian,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   barbarianScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           mockRoll,
	})
	s.Require().NoError(err)

	// Damage: 5 (roll) + 3 (STR) = 8 (NO rage bonus)
	s.Equal(3, result.DamageBonus, "Should only include STR(+3), no rage")
	s.Equal(8, result.TotalDamage, "5 (roll) + 3 (STR)")
}

func (s *RageCombatTestSuite) TestCriticalHitWithRage() {
	barbarian := &StubEntity{id: "barbarian-4"}
	barbarianScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
	}

	goblin := monster.NewGoblin("goblin-4")

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Natural 20 on attack, 5 on each damage die (2d8 on crit)
	mockRoll := &mockRoller{
		d20Roll: 20,
		d8Rolls: []int{5, 5}, // Two dice for critical
	}

	// Activate rage
	rage := newRageForTest("rage-feature", 3)
	err := rage.Activate(s.ctx, barbarian, FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Perform critical attack
	result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
		Attacker:         barbarian,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   barbarianScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           mockRoll,
	})
	s.Require().NoError(err)

	s.True(result.Critical)
	s.Equal(2, len(result.DamageRolls), "critical should double damage dice")

	// Damage: 5+5 (crit dice) + 3 (STR) + 2 (rage) = 15
	// Note: Rage bonus is NOT doubled on crit (only dice are doubled)
	s.Equal(5, result.DamageBonus, "STR(+3) + rage(+2)")
	s.Equal(15, result.TotalDamage, "10 (crit dice) + 3 (STR) + 2 (rage)")
}

// mockRoller for predictable dice rolls (imported from combat tests pattern)
type mockRoller struct {
	d20Roll int
	d8Rolls []int
}

func (m *mockRoller) Roll(_ context.Context, size int) (int, error) {
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
