package combat

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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// testEntity is a simple entity for testing attacks without importing monster
type testEntity struct {
	id   string
	name string
	hp   int
	ac   int
}

func (t *testEntity) GetID() string            { return t.id }
func (t *testEntity) GetType() core.EntityType { return "test-entity" }
func (t *testEntity) AC() int                  { return t.ac }

type AttackTestSuite struct {
	suite.Suite
	ctrl     *gomock.Controller
	ctx      context.Context
	eventBus events.EventBus
}

func TestAttackSuite(t *testing.T) {
	suite.Run(t, new(AttackTestSuite))
}

func (s *AttackTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.eventBus = events.NewEventBus()
}

func (s *AttackTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *AttackTestSuite) TestResolveAttack_BasicMeleeHit() {
	// Create attacker entity with moderate STR
	attackerScores := shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10, // +0 modifier
	}

	attacker := &testEntity{
		id:   "barbarian-1",
		name: "Barbarian",
		hp:   50,
		ac:   15,
	}

	// Create goblin target (AC 15 per SRD)
	goblin := &testEntity{
		id:   "goblin-1",
		name: "Goblin",
		hp:   7,
		ac:   15,
	}

	// Longsword
	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: 15 on d20, 5 on d8
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil)

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(), // 15
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	//nolint:gocritic // Math explanation for test assertion, not commented code
	// Attack: 15 (roll) + 3 (STR) + 2 (prof) = 20
	s.Equal(15, result.AttackRoll)
	s.Equal(5, result.AttackBonus, "STR(+3) + proficiency(+2)")
	s.Equal(20, result.TotalAttack)
	s.True(result.Hit, "20 should hit AC 15")
	s.False(result.Critical)

	//nolint:gocritic // Math explanation for test assertion, not commented code
	// Damage: 5 (roll) + 3 (STR) = 8
	s.Equal([]int{5}, result.DamageRolls)
	s.Equal(3, result.DamageBonus, "STR modifier")
	s.Equal(8, result.TotalDamage)
	s.Equal(damage.Slashing, result.DamageType)
}

func (s *AttackTestSuite) TestResolveAttack_NaturalTwenty() {
	attackerScores := shared.AbilityScores{
		abilities.STR: 10, // +0 modifier
	}

	attacker := &testEntity{
		id:   "barbarian-1",
		name: "Barbarian",
		hp:   50,
		ac:   15,
	}

	goblin := &testEntity{
		id:   "goblin-1",
		name: "Goblin",
		hp:   7,
		ac:   15,
	}

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Natural 20 on attack, 5 on each damage die (2d8 on crit = two separate rolls)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil).Times(2)

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 0,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
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
	//nolint:gocritic // Math explanation for test assertion, not commented code
	// Total: 5 + 5 (dice) + 0 (STR) = 10
	s.Equal(10, result.TotalDamage)
}

func (s *AttackTestSuite) TestResolveAttack_PublishesEvents() {
	attackerScores := shared.AbilityScores{
		abilities.STR: 16, // +3
	}

	attacker := &testEntity{
		id:   "barbarian-1",
		name: "Barbarian",
		hp:   50,
		ac:   15,
	}

	goblin := &testEntity{
		id:   "goblin-1",
		name: "Goblin",
		hp:   7,
		ac:   15,
	}

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil)

	// Track events
	var attackChainFired bool
	var damageEvent *dnd5eEvents.DamageReceivedEvent

	// Subscribe to AttackChain (fires before roll to collect modifiers)
	attackChainTopic := dnd5eEvents.AttackChain.On(s.eventBus)
	onAttack := func(
		_ context.Context,
		e dnd5eEvents.AttackChainEvent,
		c chain.Chain[dnd5eEvents.AttackChainEvent],
	) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
		attackChainFired = true
		s.Equal("barbarian-1", e.AttackerID)
		s.Equal("goblin-1", e.TargetID)
		s.True(e.IsMelee)
		return c, nil
	}
	_, err := attackChainTopic.SubscribeWithChain(s.ctx, onAttack)
	s.Require().NoError(err)

	// Subscribe to DamageReceivedEvent
	damages := dnd5eEvents.DamageReceivedTopic.On(s.eventBus)
	_, err = damages.Subscribe(s.ctx, func(_ context.Context, e dnd5eEvents.DamageReceivedEvent) error {
		damageEvent = &e
		return nil
	})
	s.Require().NoError(err)

	input := &AttackInput{
		Attacker:         attacker,
		Defender:         goblin,
		Weapon:           longsword,
		AttackerScores:   attackerScores,
		DefenderAC:       goblin.AC(),
		ProficiencyBonus: 2,
		EventBus:         s.eventBus,
		Roller:           mockRoller,
	}

	result, err := ResolveAttack(s.ctx, input)
	s.Require().NoError(err)
	s.True(result.Hit)

	// Verify AttackChain was fired
	s.True(attackChainFired, "AttackChain should be fired before the roll")

	// Verify DamageReceivedEvent was published
	s.Require().NotNil(damageEvent, "DamageReceivedEvent should be published")
	s.Equal("goblin-1", damageEvent.TargetID)
	s.Equal("barbarian-1", damageEvent.SourceID)
	s.Equal(8, damageEvent.Amount)
	s.Equal(damage.Slashing, damageEvent.DamageType)
}
