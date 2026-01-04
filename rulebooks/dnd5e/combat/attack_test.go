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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type AttackTestSuite struct {
	suite.Suite
	ctrl     *gomock.Controller
	ctx      context.Context
	eventBus events.EventBus
	lookup   *mock_combat.MockCombatantLookup
}

func TestAttackSuite(t *testing.T) {
	suite.Run(t, new(AttackTestSuite))
}

func (s *AttackTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.eventBus = events.NewEventBus()
	s.lookup = mock_combat.NewMockCombatantLookup(s.ctrl)
	// Add combatant lookup to context
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)
}

func (s *AttackTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *AttackTestSuite) TestResolveAttack_BasicMeleeHit() {
	// Create mock attacker with moderate STR
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("barbarian-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10, // +0 modifier
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(2).AnyTimes()

	// Create mock goblin target (AC 15 per SRD)
	goblin := mock_combat.NewMockCombatant(s.ctrl)
	goblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	goblin.EXPECT().AC().Return(15).AnyTimes()

	// Configure lookup to return our mocks
	s.lookup.EXPECT().Get("barbarian-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(goblin, nil).AnyTimes()

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

	input := &combat.AttackInput{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		Weapon:     longsword,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
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
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("barbarian-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 10, // +0 modifier
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(0).AnyTimes()

	goblin := mock_combat.NewMockCombatant(s.ctrl)
	goblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	goblin.EXPECT().AC().Return(15).AnyTimes()

	s.lookup.EXPECT().Get("barbarian-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(goblin, nil).AnyTimes()

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

	input := &combat.AttackInput{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		Weapon:     longsword,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
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
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("barbarian-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 16, // +3
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(2).AnyTimes()

	goblin := mock_combat.NewMockCombatant(s.ctrl)
	goblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	goblin.EXPECT().AC().Return(15).AnyTimes()

	s.lookup.EXPECT().Get("barbarian-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(goblin, nil).AnyTimes()

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

	input := &combat.AttackInput{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		Weapon:     longsword,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
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

func (s *AttackTestSuite) TestResolveAttack_WithAdvantage() {
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("fighter-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 14, // +2 modifier
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(2).AnyTimes()

	goblin := mock_combat.NewMockCombatant(s.ctrl)
	goblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	goblin.EXPECT().AC().Return(15).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(goblin, nil).AnyTimes()

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: advantage rolls 8 and 15, should take 15 (higher)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{8, 15}, nil) // Advantage roll
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{6}, nil)      // Damage roll

	// Subscribe to chain to add advantage source
	attackChainTopic := dnd5eEvents.AttackChain.On(s.eventBus)
	_, err := attackChainTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		_ dnd5eEvents.AttackChainEvent,
		c chain.Chain[dnd5eEvents.AttackChainEvent],
	) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
		// Add advantage source (simulating a feature like Pack Tactics)
		addAdvantage := func(_ context.Context, evt dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			evt.AdvantageSources = append(evt.AdvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: &core.Ref{Module: "dnd5e", Type: "features", ID: "pack_tactics"},
				SourceID:  "ally-1",
				Reason:    "Pack Tactics: ally within 5ft of target",
			})
			return evt, nil
		}
		return c, c.Add(combat.StageConditions, "pack_tactics", addAdvantage)
	})
	s.Require().NoError(err)

	input := &combat.AttackInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     longsword,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	// Should take higher roll (15)
	s.Equal(15, result.AttackRoll, "should take higher of 8 and 15")
	s.Equal([]int{8, 15}, result.AllRolls, "should record both rolls")
	s.True(result.HasAdvantage, "should indicate advantage was used")
	s.False(result.HasDisadvantage)

	// Attack: 15 (roll) + 2 (STR) + 2 (prof) = 19 vs AC 15
	s.Equal(19, result.TotalAttack)
	s.True(result.Hit)
}

func (s *AttackTestSuite) TestResolveAttack_WithDisadvantage() {
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("fighter-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 14, // +2 modifier
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(2).AnyTimes()

	goblin := mock_combat.NewMockCombatant(s.ctrl)
	goblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	goblin.EXPECT().AC().Return(15).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(goblin, nil).AnyTimes()

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: disadvantage rolls 18 and 5, should take 5 (lower)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{18, 5}, nil) // Disadvantage roll
	// No damage roll expected - attack will miss (5 + 4 = 9 vs AC 15)

	// Subscribe to chain to add disadvantage source
	attackChainTopic := dnd5eEvents.AttackChain.On(s.eventBus)
	_, err := attackChainTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		_ dnd5eEvents.AttackChainEvent,
		c chain.Chain[dnd5eEvents.AttackChainEvent],
	) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
		// Add disadvantage source (simulating Protection fighting style)
		addDisadvantage := func(_ context.Context, evt dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			evt.DisadvantageSources = append(evt.DisadvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: &core.Ref{Module: "dnd5e", Type: "fighting_styles", ID: "protection"},
				SourceID:  "ally-paladin",
				Reason:    "Protection: ally used reaction to impose disadvantage",
			})
			return evt, nil
		}
		return c, c.Add(combat.StageConditions, "protection", addDisadvantage)
	})
	s.Require().NoError(err)

	input := &combat.AttackInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     longsword,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	// Should take lower roll (5)
	s.Equal(5, result.AttackRoll, "should take lower of 18 and 5")
	s.Equal([]int{18, 5}, result.AllRolls, "should record both rolls")
	s.False(result.HasAdvantage)
	s.True(result.HasDisadvantage, "should indicate disadvantage was used")

	// Attack: 5 (roll) + 2 (STR) + 2 (prof) = 9 vs AC 15
	s.Equal(9, result.TotalAttack)
	s.False(result.Hit, "9 should miss AC 15")
}

func (s *AttackTestSuite) TestResolveAttack_AdvantageAndDisadvantageCancelOut() {
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("fighter-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 14, // +2 modifier
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(2).AnyTimes()

	goblin := mock_combat.NewMockCombatant(s.ctrl)
	goblin.EXPECT().GetID().Return("goblin-1").AnyTimes()
	goblin.EXPECT().AC().Return(15).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(goblin, nil).AnyTimes()

	longsword := &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	// Mock roller: single d20 roll (advantage and disadvantage cancel)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().Roll(s.ctx, 20).Return(12, nil)          // Normal roll
	mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{4}, nil) // Damage roll

	// Subscribe to chain to add BOTH advantage and disadvantage
	attackChainTopic := dnd5eEvents.AttackChain.On(s.eventBus)
	_, err := attackChainTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		_ dnd5eEvents.AttackChainEvent,
		c chain.Chain[dnd5eEvents.AttackChainEvent],
	) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
		addBoth := func(_ context.Context, evt dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			// Advantage from flanking
			evt.AdvantageSources = append(evt.AdvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: &core.Ref{Module: "dnd5e", Type: "rules", ID: "flanking"},
				SourceID:  "ally-1",
				Reason:    "Flanking",
			})
			// Disadvantage from being prone
			evt.DisadvantageSources = append(evt.DisadvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: &core.Ref{Module: "dnd5e", Type: "conditions", ID: "prone"},
				SourceID:  "fighter-1",
				Reason:    "Attacker is prone",
			})
			return evt, nil
		}
		return c, c.Add(combat.StageConditions, "flanking_and_prone", addBoth)
	})
	s.Require().NoError(err)

	input := &combat.AttackInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     longsword,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	// D&D 5e rule: any advantage + any disadvantage = cancel out to normal roll
	s.Equal(12, result.AttackRoll)
	s.Equal([]int{12}, result.AllRolls, "should only have one roll when cancelled")
	s.False(result.HasAdvantage, "advantage should be cancelled")
	s.False(result.HasDisadvantage, "disadvantage should be cancelled")

	// Attack: 12 (roll) + 2 (STR) + 2 (prof) = 16 vs AC 15
	s.Equal(16, result.TotalAttack)
	s.True(result.Hit)
}

func (s *AttackTestSuite) TestResolveAttack_ReactionsConsumedPublishesEvents() {
	attacker := mock_combat.NewMockCombatant(s.ctrl)
	attacker.EXPECT().GetID().Return("goblin-1").AnyTimes()
	attacker.EXPECT().GetAbilityScores().Return(shared.AbilityScores{
		abilities.STR: 14, // +2 modifier
	}).AnyTimes()
	attacker.EXPECT().GetProficiencyBonus().Return(2).AnyTimes()

	defender := mock_combat.NewMockCombatant(s.ctrl)
	defender.EXPECT().GetID().Return("fighter-1").AnyTimes()
	defender.EXPECT().AC().Return(18).AnyTimes()

	s.lookup.EXPECT().Get("goblin-1").Return(attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("fighter-1").Return(defender, nil).AnyTimes()

	scimitar := &weapons.Weapon{
		ID:         weapons.Scimitar,
		Name:       "Scimitar",
		Damage:     "1d6",
		DamageType: damage.Slashing,
	}

	// Mock roller: disadvantage rolls (Protection imposed disadvantage)
	mockRoller := mock_dice.NewMockRoller(s.ctrl)
	mockRoller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{15, 8}, nil) // Takes 8 (lower)
	// No damage - 8 + 4 = 12 vs AC 18 = miss

	// Track ReactionUsedEvents
	var reactionEvents []dnd5eEvents.ReactionUsedEvent
	reactionTopic := dnd5eEvents.ReactionUsedTopic.On(s.eventBus)
	_, err := reactionTopic.Subscribe(s.ctx, func(_ context.Context, e dnd5eEvents.ReactionUsedEvent) error {
		reactionEvents = append(reactionEvents, e)
		return nil
	})
	s.Require().NoError(err)

	// Subscribe to chain to add disadvantage AND consume a reaction
	protectionRef := &core.Ref{Module: "dnd5e", Type: "fighting_styles", ID: "protection"}
	attackChainTopic := dnd5eEvents.AttackChain.On(s.eventBus)
	_, err = attackChainTopic.SubscribeWithChain(s.ctx, func(
		_ context.Context,
		_ dnd5eEvents.AttackChainEvent,
		c chain.Chain[dnd5eEvents.AttackChainEvent],
	) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
		addProtection := func(_ context.Context, evt dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			// Paladin uses Protection fighting style
			evt.DisadvantageSources = append(evt.DisadvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: protectionRef,
				SourceID:  "paladin-1",
				Reason:    "Protection: imposing disadvantage on attack against ally",
			})
			// Record reaction consumption
			evt.ReactionsConsumed = append(evt.ReactionsConsumed, dnd5eEvents.ReactionConsumption{
				CharacterID: "paladin-1",
				FeatureRef:  protectionRef,
				Reason:      "Used Protection fighting style",
			})
			return evt, nil
		}
		return c, c.Add(combat.StageConditions, "protection", addProtection)
	})
	s.Require().NoError(err)

	input := &combat.AttackInput{
		AttackerID: "goblin-1",
		TargetID:   "fighter-1",
		Weapon:     scimitar,
		EventBus:   s.eventBus,
		Roller:     mockRoller,
	}

	result, err := combat.ResolveAttack(s.ctx, input)
	s.Require().NoError(err)

	// Verify attack missed due to disadvantage
	s.Equal(8, result.AttackRoll, "should take lower roll")
	s.False(result.Hit, "12 should miss AC 18")
	s.True(result.HasDisadvantage)

	// Verify ReactionUsedEvent was published
	s.Require().Len(reactionEvents, 1, "should publish one ReactionUsedEvent")
	s.Equal("paladin-1", reactionEvents[0].CharacterID)
	s.Equal(protectionRef, reactionEvents[0].FeatureRef)
	s.Equal("Used Protection fighting style", reactionEvents[0].Reason)
}
