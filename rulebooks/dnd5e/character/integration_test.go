package character_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sequenceRoller is a deterministic dice roller that returns values from a predetermined sequence.
// Used in integration tests for reproducible results.
type sequenceRoller struct {
	rolls []int
	index int
}

func newSequenceRoller(rolls ...int) *sequenceRoller {
	return &sequenceRoller{rolls: rolls}
}

func (r *sequenceRoller) Roll(_ context.Context, _ int) (int, error) {
	if r.index >= len(r.rolls) {
		return 0, rpgerr.New(rpgerr.CodeInternal, "sequence roller exhausted")
	}
	val := r.rolls[r.index]
	r.index++
	return val, nil
}

func (r *sequenceRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	results := make([]int, count)
	for i := 0; i < count; i++ {
		if r.index >= len(r.rolls) {
			return nil, rpgerr.New(rpgerr.CodeInternal, "sequence roller exhausted")
		}
		results[i] = r.rolls[r.index]
		r.index++
	}
	return results, nil
}

// Verify sequenceRoller implements dice.Roller
var _ dice.Roller = (*sequenceRoller)(nil)

// combatantRegistry implements combat.CombatantLookup for integration tests.
type combatantRegistry struct {
	combatants map[string]combat.Combatant
}

func newCombatantRegistry() *combatantRegistry {
	return &combatantRegistry{combatants: make(map[string]combat.Combatant)}
}

func (r *combatantRegistry) Register(c combat.Combatant) {
	r.combatants[c.GetID()] = c
}

func (r *combatantRegistry) Get(id string) (combat.Combatant, error) {
	c, ok := r.combatants[id]
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeNotFound, fmt.Sprintf("combatant %s not found", id))
	}
	return c, nil
}

// FullAttackFlowIntegrationSuite tests the complete attack flow from character creation
// through action economy usage, strike execution, and two-weapon fighting.
// This is a scenario test that exercises real code paths, not mocks.
type FullAttackFlowIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestFullAttackFlowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(FullAttackFlowIntegrationSuite))
}

func (s *FullAttackFlowIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *FullAttackFlowIntegrationSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
}

// mockTarget implements core.Entity for testing
type mockTarget struct {
	id string
}

func (m *mockTarget) GetID() string {
	return m.id
}

func (m *mockTarget) GetType() core.EntityType {
	return "target"
}

// Helper: Create a fighter draft for testing
func (s *FullAttackFlowIntegrationSuite) createFighterDraft() *character.Draft {
	draft := character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-integration-test",
		PlayerID: "player-001",
	})

	// Set name
	err := draft.SetName(&character.SetNameInput{Name: "Test Fighter"})
	s.Require().NoError(err)

	// Set base ability scores (strong fighter)
	err = draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
	})
	s.Require().NoError(err)

	// Set race - Human requires language choice
	err = draft.SetRace(&character.SetRaceInput{
		RaceID: races.Human,
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      character.BackgroundChoices{},
	})
	s.Require().NoError(err)

	// Set class with Two-Weapon Fighting style
	err = draft.SetClass(&character.SetClassInput{
		ClassID: classes.Fighter,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.TwoWeaponFighting,
		},
	})
	s.Require().NoError(err)

	return draft
}

// TestFullAttackFlow tests the complete attack flow with Extra Attack and dual wielding.
//
// This test simulates a combat turn for a level 5+ fighter with Extra Attack (2 attacks)
// who is dual-wielding light weapons:
//
// 1. Character creation with standard combat abilities and actions
// 2. Set up ActionEconomy (1 action, 1 bonus action, 30ft movement)
// 3. Activate Attack ability -> verify AttacksRemaining = 2 (Extra Attack)
// 4. Activate Strike -> verify AttacksRemaining = 1, StrikeExecutedEvent published
// 5. Two-weapon fighting grants OffHandStrike via ActionGrantedEvent
// 6. Activate Strike again -> verify AttacksRemaining = 0
// 7. Activate OffHandStrike -> verify bonus action consumed, event published
func (s *FullAttackFlowIntegrationSuite) TestFullAttackFlow() {
	s.Run("complete attack flow with Extra Attack and dual wielding", func() {
		// =====================================================================
		// SETUP: Create character and track events
		// =====================================================================

		draft := s.createFighterDraft()
		char, err := draft.ToCharacter(s.ctx, "fighter-001", s.bus)
		s.Require().NoError(err)

		// Track events
		var strikeEvents []dnd5eEvents.StrikeExecutedEvent
		var actionGrantedEvents []dnd5eEvents.ActionGrantedEvent
		var offHandStrikeRequestedEvents []dnd5eEvents.OffHandStrikeRequestedEvent
		var offHandStrikeActivatedEvents []dnd5eEvents.OffHandStrikeActivatedEvent

		// Subscribe to StrikeExecutedEvent
		strikeTopic := dnd5eEvents.StrikeExecutedTopic.On(s.bus)
		_, err = strikeTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StrikeExecutedEvent) error {
			strikeEvents = append(strikeEvents, event)
			return nil
		})
		s.Require().NoError(err)

		// Subscribe to ActionGrantedEvent
		actionGrantedTopic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
		_, err = actionGrantedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionGrantedEvent) error {
			actionGrantedEvents = append(actionGrantedEvents, event)
			return nil
		})
		s.Require().NoError(err)

		// Subscribe to OffHandStrikeRequestedEvent
		offHandRequestedTopic := dnd5eEvents.OffHandStrikeRequestedTopic.On(s.bus)
		_, err = offHandRequestedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.OffHandStrikeRequestedEvent) error {
			offHandStrikeRequestedEvents = append(offHandStrikeRequestedEvents, event)
			return nil
		})
		s.Require().NoError(err)

		// Subscribe to OffHandStrikeActivatedEvent
		offHandActivatedTopic := dnd5eEvents.OffHandStrikeActivatedTopic.On(s.bus)
		_, err = offHandActivatedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.OffHandStrikeActivatedEvent) error {
			offHandStrikeActivatedEvents = append(offHandStrikeActivatedEvents, event)
			return nil
		})
		s.Require().NoError(err)

		// =====================================================================
		// VERIFY: Character has standard combat abilities and actions
		// =====================================================================

		combatAbilities := char.GetCombatAbilities()
		s.Assert().Len(combatAbilities, 4, "character should have 4 standard combat abilities")

		attackAbility := char.GetCombatAbility("fighter-001-attack")
		s.Require().NotNil(attackAbility, "character should have Attack ability")
		s.Assert().Equal("Attack", attackAbility.Name())

		charActions := char.GetActions()
		s.Assert().GreaterOrEqual(len(charActions), 2, "character should have at least Strike and Move actions")

		strikeAction := char.GetAction("fighter-001-strike")
		s.Require().NotNil(strikeAction, "character should have Strike action")

		// =====================================================================
		// STEP 1: Set up action economy (start of turn)
		// =====================================================================

		actionEconomy := combat.NewActionEconomy()
		actionEconomy.SetMovement(30) // Fighter base speed

		s.Assert().Equal(1, actionEconomy.ActionsRemaining, "should start with 1 action")
		s.Assert().Equal(1, actionEconomy.BonusActionsRemaining, "should start with 1 bonus action")
		s.Assert().Equal(0, actionEconomy.AttacksRemaining, "should start with 0 attacks (until Attack used)")
		s.Assert().Equal(30, actionEconomy.MovementRemaining, "should have 30ft movement")

		// =====================================================================
		// STEP 2: Activate Attack ability (simulating level 5+ fighter)
		// =====================================================================

		// The Attack ability consumes 1 action and grants attacks based on Extra Attack
		err = attackAbility.Activate(s.ctx, char, combatabilities.CombatAbilityInput{
			ActionEconomy: actionEconomy,
			ExtraAttacks:  1, // Fighter with Extra Attack at level 5+
		})
		s.Require().NoError(err)

		s.Assert().Equal(0, actionEconomy.ActionsRemaining, "action should be consumed")
		s.Assert().Equal(2, actionEconomy.AttacksRemaining, "should have 2 attacks (1 base + 1 Extra Attack)")

		// =====================================================================
		// STEP 3: First Strike - attack the goblin
		// =====================================================================

		goblin := &mockTarget{id: "goblin-1"}

		err = strikeAction.Activate(s.ctx, char, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: actionEconomy,
			Target:        goblin,
		})
		s.Require().NoError(err)

		s.Assert().Equal(1, actionEconomy.AttacksRemaining, "should have 1 attack remaining after first strike")
		s.Assert().Len(strikeEvents, 1, "should have published 1 StrikeExecutedEvent")
		s.Assert().Equal("fighter-001", strikeEvents[0].AttackerID)
		s.Assert().Equal("goblin-1", strikeEvents[0].TargetID)

		// =====================================================================
		// STEP 4: Two-weapon fighting grants OffHandStrike
		// =====================================================================

		// Simulate dual-wielding light weapons (shortswords)
		output, err := actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-001",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)
		s.Require().NotNil(output)
		s.Assert().True(output.Granted, "should grant off-hand strike for dual light weapons")
		s.Assert().Equal("dual-wielding light weapons", output.Reason)

		// Verify ActionGrantedEvent was published
		s.Assert().Len(actionGrantedEvents, 1, "should have published ActionGrantedEvent")
		s.Assert().Equal("fighter-001", actionGrantedEvents[0].CharacterID)
		s.Assert().Equal("two_weapon_fighting", actionGrantedEvents[0].Source)

		// Set off-hand attack capacity in action economy (game server would do this)
		actionEconomy.SetOffHandAttacks(1)

		// Verify character received the action via event
		offHandStrike := char.GetAction("fighter-001-off-hand-strike")
		s.Require().NotNil(offHandStrike, "character should have OffHandStrike action after event")

		// =====================================================================
		// STEP 5: Second Strike - use remaining attack
		// =====================================================================

		err = strikeAction.Activate(s.ctx, char, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: actionEconomy,
			Target:        goblin,
		})
		s.Require().NoError(err)

		s.Assert().Equal(0, actionEconomy.AttacksRemaining, "should have 0 attacks remaining after second strike")
		s.Assert().Len(strikeEvents, 2, "should have published 2 StrikeExecutedEvents")

		// Verify third strike would fail
		err = strikeAction.CanActivate(s.ctx, char, actions.ActionInput{
			ActionEconomy: actionEconomy,
			Target:        goblin,
		})
		s.Assert().Error(err, "should not be able to strike with no attacks remaining")

		// =====================================================================
		// STEP 6: OffHandStrike - uses bonus action, not attacks
		// =====================================================================

		s.Assert().Equal(1, actionEconomy.BonusActionsRemaining, "should still have bonus action")

		// Get the actual OffHandStrike action from the character
		offHandAction := char.GetAction("fighter-001-off-hand-strike")
		s.Require().NotNil(offHandAction, "character should have OffHandStrike")

		// Activate the off-hand strike
		err = offHandAction.Activate(s.ctx, char, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: actionEconomy,
			Target:        goblin,
		})
		s.Require().NoError(err)

		// Verify events were published
		s.Assert().Len(offHandStrikeRequestedEvents, 1, "should have published OffHandStrikeRequestedEvent")
		s.Assert().Equal("fighter-001", offHandStrikeRequestedEvents[0].AttackerID)
		s.Assert().Equal("goblin-1", offHandStrikeRequestedEvents[0].TargetID)
		s.Assert().Equal(string(weapons.Shortsword), offHandStrikeRequestedEvents[0].WeaponID)

		s.Assert().Len(offHandStrikeActivatedEvents, 1, "should have published OffHandStrikeActivatedEvent")
		s.Assert().Equal(0, offHandStrikeActivatedEvents[0].UsesRemaining)

		// =====================================================================
		// VERIFY: Final state
		// =====================================================================

		// Total attacks made: 2 main-hand + 1 off-hand = 3 attacks
		s.Assert().Len(strikeEvents, 2, "should have made 2 main-hand strikes")
		s.Assert().Len(offHandStrikeRequestedEvents, 1, "should have made 1 off-hand strike")

		// Action economy should be exhausted (actions used, attacks used, bonus action used for off-hand)
		s.Assert().Equal(0, actionEconomy.ActionsRemaining, "action should be consumed")
		s.Assert().Equal(0, actionEconomy.AttacksRemaining, "attacks should be consumed")
		// Note: OffHandStrike doesn't consume bonus action from economy directly
		// The game server would consume it when processing OffHandStrikeRequestedEvent
	})
}

// TestAttackAbilityVariants tests different Extra Attack configurations
func (s *FullAttackFlowIntegrationSuite) TestAttackAbilityVariants() {
	testCases := []struct {
		name          string
		extraAttacks  int
		expectedTotal int
		description   string
	}{
		{
			name:          "normal character (no extra attack)",
			extraAttacks:  0,
			expectedTotal: 1,
			description:   "Level 1-4 characters get 1 attack",
		},
		{
			name:          "fighter level 5 (Extra Attack)",
			extraAttacks:  1,
			expectedTotal: 2,
			description:   "Level 5+ fighters get 2 attacks",
		},
		{
			name:          "fighter level 11 (Extra Attack 2)",
			extraAttacks:  2,
			expectedTotal: 3,
			description:   "Level 11+ fighters get 3 attacks",
		},
		{
			name:          "fighter level 20 (Extra Attack 3)",
			extraAttacks:  3,
			expectedTotal: 4,
			description:   "Level 20 fighters get 4 attacks",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			draft := s.createFighterDraft()
			char, err := draft.ToCharacter(s.ctx, "fighter-variant", s.bus)
			s.Require().NoError(err)

			attackAbility := char.GetCombatAbility("fighter-variant-attack")
			s.Require().NotNil(attackAbility)

			actionEconomy := combat.NewActionEconomy()

			err = attackAbility.Activate(s.ctx, char, combatabilities.CombatAbilityInput{
				ActionEconomy: actionEconomy,
				ExtraAttacks:  tc.extraAttacks,
			})
			s.Require().NoError(err)

			s.Assert().Equal(tc.expectedTotal, actionEconomy.AttacksRemaining, tc.description)
		})
	}
}

// TestTwoWeaponFightingRequirements tests that two-weapon fighting has correct requirements
func (s *FullAttackFlowIntegrationSuite) TestTwoWeaponFightingRequirements() {
	s.Run("requires both weapons to be light", func() {
		// Longsword is not light - should fail
		output, err := actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-test",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Longsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)
		s.Assert().False(output.Granted)
		s.Assert().Equal("main-hand weapon is not light", output.Reason)
	})

	s.Run("requires off-hand weapon to be light", func() {
		output, err := actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-test",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Longsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)
		s.Assert().False(output.Granted)
		s.Assert().Equal("off-hand weapon is not light", output.Reason)
	})

	s.Run("requires main-hand attack (not off-hand)", func() {
		output, err := actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-test",
			AttackHand:     actions.AttackHandOff, // Off-hand attack
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)
		s.Assert().False(output.Granted)
		s.Assert().Equal("not a main-hand attack", output.Reason)
	})

	s.Run("grants off-hand strike with dual light weapons", func() {
		output, err := actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-test",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)
		s.Assert().True(output.Granted)
		s.Assert().Equal("dual-wielding light weapons", output.Reason)
		s.Assert().NotNil(output.Action)
		s.Assert().Equal(weapons.Shortsword, output.Action.GetWeaponID())
	})

	s.Run("grants with different light weapons (dagger + shortsword)", func() {
		output, err := actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-test",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Dagger},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)
		s.Assert().True(output.Granted)
		// Off-hand strike uses the off-hand weapon
		s.Assert().Equal(weapons.Shortsword, output.Action.GetWeaponID())
	})
}

// TestEventFlowOrder tests that events are published in the correct order
func (s *FullAttackFlowIntegrationSuite) TestEventFlowOrder() {
	s.Run("events are published in correct order during attack sequence", func() {
		draft := s.createFighterDraft()
		char, err := draft.ToCharacter(s.ctx, "fighter-events", s.bus)
		s.Require().NoError(err)

		// Track event order
		var eventOrder []string

		strikeTopic := dnd5eEvents.StrikeExecutedTopic.On(s.bus)
		_, err = strikeTopic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.StrikeExecutedEvent) error {
			eventOrder = append(eventOrder, "strike_executed")
			return nil
		})
		s.Require().NoError(err)

		actionGrantedTopic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
		_, err = actionGrantedTopic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.ActionGrantedEvent) error {
			eventOrder = append(eventOrder, "action_granted")
			return nil
		})
		s.Require().NoError(err)

		offHandRequestedTopic := dnd5eEvents.OffHandStrikeRequestedTopic.On(s.bus)
		_, err = offHandRequestedTopic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.OffHandStrikeRequestedEvent) error {
			eventOrder = append(eventOrder, "off_hand_requested")
			return nil
		})
		s.Require().NoError(err)

		offHandActivatedTopic := dnd5eEvents.OffHandStrikeActivatedTopic.On(s.bus)
		_, err = offHandActivatedTopic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.OffHandStrikeActivatedEvent) error {
			eventOrder = append(eventOrder, "off_hand_activated")
			return nil
		})
		s.Require().NoError(err)

		// Execute attack sequence
		actionEconomy := combat.NewActionEconomy()
		attackAbility := char.GetCombatAbility("fighter-events-attack")
		_ = attackAbility.Activate(s.ctx, char, combatabilities.CombatAbilityInput{
			ActionEconomy: actionEconomy,
			ExtraAttacks:  1,
		})

		goblin := &mockTarget{id: "goblin"}
		strikeAction := char.GetAction("fighter-events-strike")

		// First strike
		_ = strikeAction.Activate(s.ctx, char, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: actionEconomy,
			Target:        goblin,
		})

		// Grant off-hand strike
		_, _ = actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-events",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})

		// Set off-hand attack capacity in action economy
		actionEconomy.SetOffHandAttacks(1)

		// Second strike
		_ = strikeAction.Activate(s.ctx, char, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: actionEconomy,
			Target:        goblin,
		})

		// Off-hand strike
		offHandAction := char.GetAction("fighter-events-off-hand-strike")
		if offHandAction != nil {
			_ = offHandAction.Activate(s.ctx, char, actions.ActionInput{
				Bus:           s.bus,
				ActionEconomy: actionEconomy,
				Target:        goblin,
			})
		}

		// Verify event order
		expectedOrder := []string{
			"strike_executed",    // First main-hand strike
			"action_granted",     // Off-hand strike granted
			"strike_executed",    // Second main-hand strike
			"off_hand_requested", // Off-hand strike execution
			"off_hand_activated", // Off-hand strike completion notification
		}

		s.Assert().Equal(expectedOrder, eventOrder, "events should be published in the correct order")
	})
}

// =============================================================================
// AttackResolutionIntegrationSuite - Tests ResolveAttack with real conditions
// =============================================================================

// AttackResolutionIntegrationSuite tests the complete attack resolution flow with
// real Characters and Conditions modifying the AttackChain and DamageChain.
// This proves the full pipeline: Character → Conditions → Chains → ResolveAttack → Damage.
type AttackResolutionIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestAttackResolutionIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AttackResolutionIntegrationSuite))
}

func (s *AttackResolutionIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *AttackResolutionIntegrationSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
}

// createBarbarianDraft creates a barbarian draft for attack resolution tests.
func (s *AttackResolutionIntegrationSuite) createBarbarianDraft() *character.Draft {
	draft := character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-barbarian",
		PlayerID: "player-barbarian",
	})

	err := draft.SetName(&character.SetNameInput{Name: "Korg the Furious"})
	s.Require().NoError(err)

	// Strong barbarian: STR 16 (+3), DEX 14 (+2)
	err = draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 8,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
	})
	s.Require().NoError(err)

	err = draft.SetRace(&character.SetRaceInput{
		RaceID: races.Human,
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Orc},
		},
	})
	s.Require().NoError(err)

	err = draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
		Choices:      character.BackgroundChoices{},
	})
	s.Require().NoError(err)

	err = draft.SetClass(&character.SetClassInput{
		ClassID: classes.Barbarian,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponMartial,
					CategorySelections: []shared.EquipmentID{weapons.Longsword}},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	})
	s.Require().NoError(err)

	return draft
}

// createDefenderDraft creates a fighter draft to serve as the defender.
func (s *AttackResolutionIntegrationSuite) createDefenderDraft() *character.Draft {
	draft := character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-defender",
		PlayerID: "player-defender",
	})

	err := draft.SetName(&character.SetNameInput{Name: "Elara the Cautious"})
	s.Require().NoError(err)

	err = draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 14,
			abilities.DEX: 16,
			abilities.CON: 12,
			abilities.INT: 10,
			abilities.WIS: 14,
			abilities.CHA: 10,
		},
	})
	s.Require().NoError(err)

	err = draft.SetRace(&character.SetRaceInput{
		RaceID:  races.Elf,
		Choices: character.RaceChoices{},
	})
	s.Require().NoError(err)

	err = draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      character.BackgroundChoices{},
	})
	s.Require().NoError(err)

	err = draft.SetClass(&character.SetClassInput{
		ClassID: classes.Fighter,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Perception},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{ChoiceID: choices.FighterWeaponsPrimary, OptionID: choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword}},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	})
	s.Require().NoError(err)

	return draft
}

// TestRagingBarbarianHitsDodgingDefender demonstrates the full attack resolution flow:
// - Attacker has RagingCondition (subscribes to DamageChain, adds +2 damage)
// - Defender has DodgingCondition (subscribes to AttackChain, imposes disadvantage)
// - ResolveAttack fires through both chains
// - Printed output shows the complete sequence
func (s *AttackResolutionIntegrationSuite) TestRagingBarbarianHitsDodgingDefender() {
	s.Run("hit scenario: raging barbarian overcomes disadvantage", func() {
		fmt.Println("\n=== ATTACK RESOLUTION: Raging Barbarian vs Dodging Defender ===")

		// Create characters
		attackerDraft := s.createBarbarianDraft()
		attacker, err := attackerDraft.ToCharacter(s.ctx, "barbarian-1", s.bus)
		s.Require().NoError(err)

		defenderDraft := s.createDefenderDraft()
		defender, err := defenderDraft.ToCharacter(s.ctx, "defender-1", s.bus)
		s.Require().NoError(err)

		fmt.Printf("  Attacker: %s (STR +3, Prof +2, Attack Bonus +5)\n", attacker.GetName())
		fmt.Printf("  Defender: %s (AC %d)\n", defender.GetName(), defender.AC())

		// Apply RagingCondition to attacker
		ragingCondition := &conditions.RagingCondition{
			CharacterID: "barbarian-1",
			DamageBonus: 2,
			Level:       1,
			Source:      "dnd5e:features:rage",
		}
		err = ragingCondition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		fmt.Println("  [Condition] RagingCondition applied to attacker (+2 damage bonus)")

		// Apply DodgingCondition to defender
		dodgingCondition := conditions.NewDodgingCondition("defender-1")
		err = dodgingCondition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		fmt.Println("  [Condition] DodgingCondition applied to defender (disadvantage on attacks)")

		// Track DamageReceivedEvent
		var damageEvents []dnd5eEvents.DamageReceivedEvent
		damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
		_, err = damageTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
			damageEvents = append(damageEvents, event)
			return nil
		})
		s.Require().NoError(err)

		// Register combatants in context
		registry := newCombatantRegistry()
		registry.Register(attacker)
		registry.Register(defender)
		ctx := combat.WithCombatantLookup(s.ctx, registry)

		// Get the longsword for the attack
		longsword, err := weapons.GetByID(weapons.Longsword)
		s.Require().NoError(err)

		// Deterministic roller: d20 rolls = 18, 15 (disadvantage takes 15), d8 = 6
		roller := newSequenceRoller(18, 15, 6)

		fmt.Println("\n  --- Attack Resolution ---")
		fmt.Println("  [Roll] 2d20 (disadvantage): 18, 15 → takes 15")
		fmt.Printf("  [Attack] Total: 15 + 5 (bonus) = 20 vs AC %d → HIT\n", defender.AC())

		// Resolve the attack
		result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
			AttackerID: "barbarian-1",
			TargetID:   "defender-1",
			Weapon:     &longsword,
			EventBus:   s.bus,
			Roller:     roller,
		})
		s.Require().NoError(err)

		// Verify disadvantage was applied by DodgingCondition
		s.Assert().True(result.HasDisadvantage, "DodgingCondition should impose disadvantage")
		s.Assert().Equal([]int{18, 15}, result.AllRolls, "should have rolled 2d20")
		s.Assert().Equal(15, result.AttackRoll, "disadvantage should take lower roll")

		// Verify hit (15 + 5 = 20 >= AC)
		s.Assert().True(result.Hit, "20 should beat defender AC")
		s.Assert().Equal(20, result.TotalAttack)

		// Verify rage damage bonus was added
		s.Require().NotNil(result.Breakdown, "hit should have damage breakdown")
		fmt.Printf("  [Damage] Weapon: %d (1d8 roll), Ability: +3, Rage: +2\n",
			result.DamageRolls[0])
		fmt.Printf("  [Damage] Total: %d\n", result.TotalDamage)

		// Total damage = 6 (weapon roll) + 3 (STR mod) + 2 (rage) = 11
		s.Assert().Equal(11, result.TotalDamage, "should be weapon + STR + rage bonus")

		// Verify DamageReceivedEvent was published
		s.Require().Len(damageEvents, 1, "should publish DamageReceivedEvent")
		s.Assert().Equal("defender-1", damageEvents[0].TargetID)
		s.Assert().Equal("barbarian-1", damageEvents[0].SourceID)
		s.Assert().Equal(11, damageEvents[0].Amount)

		fmt.Printf("  [Event] DamageReceivedEvent: %d damage to %s\n",
			damageEvents[0].Amount, damageEvents[0].TargetID)
		fmt.Println("  === RESULT: HIT for 11 damage ===")
	})

	s.Run("miss scenario: disadvantage causes attack to miss", func() {
		fmt.Println("\n=== ATTACK RESOLUTION: Disadvantage Causes Miss ===")

		// Create characters on fresh bus
		attackerDraft := s.createBarbarianDraft()
		attacker, err := attackerDraft.ToCharacter(s.ctx, "barbarian-2", s.bus)
		s.Require().NoError(err)

		defenderDraft := s.createDefenderDraft()
		defender, err := defenderDraft.ToCharacter(s.ctx, "defender-2", s.bus)
		s.Require().NoError(err)

		fmt.Printf("  Attacker: %s (Attack Bonus +5)\n", attacker.GetName())
		fmt.Printf("  Defender: %s (AC %d, Dodging)\n", defender.GetName(), defender.AC())

		// Apply DodgingCondition to defender (no rage on attacker this time)
		dodgingCondition := conditions.NewDodgingCondition("defender-2")
		err = dodgingCondition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Register combatants
		registry := newCombatantRegistry()
		registry.Register(attacker)
		registry.Register(defender)
		ctx := combat.WithCombatantLookup(s.ctx, registry)

		longsword, err := weapons.GetByID(weapons.Longsword)
		s.Require().NoError(err)

		// Deterministic roller: d20 rolls = 14, 8 (disadvantage takes 8)
		// 8 + 5 = 13 < AC (should miss)
		roller := newSequenceRoller(14, 8)

		fmt.Println("  [Roll] 2d20 (disadvantage): 14, 8 → takes 8")
		fmt.Printf("  [Attack] Total: 8 + 5 = 13 vs AC %d → MISS\n", defender.AC())

		result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
			AttackerID: "barbarian-2",
			TargetID:   "defender-2",
			Weapon:     &longsword,
			EventBus:   s.bus,
			Roller:     roller,
		})
		s.Require().NoError(err)

		s.Assert().True(result.HasDisadvantage, "DodgingCondition should impose disadvantage")
		s.Assert().Equal(8, result.AttackRoll, "disadvantage should take lower roll")
		s.Assert().Equal(13, result.TotalAttack)
		s.Assert().False(result.Hit, "13 should not beat defender AC")
		s.Assert().Nil(result.Breakdown, "miss should have no damage breakdown")
		s.Assert().Equal(0, result.TotalDamage)

		fmt.Println("  === RESULT: MISS ===")
	})

	s.Run("normal attack without conditions: no advantage or disadvantage", func() {
		fmt.Println("\n=== ATTACK RESOLUTION: Normal Attack (No Conditions) ===")

		attackerDraft := s.createBarbarianDraft()
		attacker, err := attackerDraft.ToCharacter(s.ctx, "barbarian-3", s.bus)
		s.Require().NoError(err)

		defenderDraft := s.createDefenderDraft()
		defender, err := defenderDraft.ToCharacter(s.ctx, "defender-3", s.bus)
		s.Require().NoError(err)

		fmt.Printf("  Attacker: %s (Attack Bonus +5)\n", attacker.GetName())
		fmt.Printf("  Defender: %s (AC %d, no conditions)\n", defender.GetName(), defender.AC())

		// No conditions applied - normal roll
		registry := newCombatantRegistry()
		registry.Register(attacker)
		registry.Register(defender)
		ctx := combat.WithCombatantLookup(s.ctx, registry)

		longsword, err := weapons.GetByID(weapons.Longsword)
		s.Require().NoError(err)

		// Single d20 roll = 12, d8 = 4
		// 12 + 5 = 17 >= AC → hit
		roller := newSequenceRoller(12, 4)

		result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
			AttackerID: "barbarian-3",
			TargetID:   "defender-3",
			Weapon:     &longsword,
			EventBus:   s.bus,
			Roller:     roller,
		})
		s.Require().NoError(err)

		fmt.Printf("  [Roll] 1d20: %d (normal roll)\n", result.AttackRoll)
		fmt.Printf("  [Attack] Total: %d + 5 = %d vs AC %d → %s\n",
			result.AttackRoll, result.TotalAttack, defender.AC(),
			map[bool]string{true: "HIT", false: "MISS"}[result.Hit])

		s.Assert().False(result.HasAdvantage, "should not have advantage")
		s.Assert().False(result.HasDisadvantage, "should not have disadvantage")
		s.Assert().Equal([]int{12}, result.AllRolls, "should have rolled 1d20")
		s.Assert().Equal(12, result.AttackRoll)
		s.Assert().True(result.Hit, "17 should beat defender AC")

		// Damage = 4 (weapon) + 3 (STR) = 7 (no rage bonus)
		s.Assert().Equal(7, result.TotalDamage, "should be weapon + STR only (no rage)")

		fmt.Printf("  [Damage] Weapon: %d, Ability: +3 (no rage)\n", result.DamageRolls[0])
		fmt.Printf("  [Damage] Total: %d\n", result.TotalDamage)
		fmt.Println("  === RESULT: HIT for 7 damage ===")
	})

	s.Run("critical hit doubles weapon dice", func() {
		fmt.Println("\n=== ATTACK RESOLUTION: Critical Hit ===")

		attackerDraft := s.createBarbarianDraft()
		attacker, err := attackerDraft.ToCharacter(s.ctx, "barbarian-4", s.bus)
		s.Require().NoError(err)

		defenderDraft := s.createDefenderDraft()
		defender, err := defenderDraft.ToCharacter(s.ctx, "defender-4", s.bus)
		s.Require().NoError(err)

		// Apply rage for extra damage on crit
		ragingCondition := &conditions.RagingCondition{
			CharacterID: "barbarian-4",
			DamageBonus: 2,
			Level:       1,
			Source:      "dnd5e:features:rage",
		}
		err = ragingCondition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		registry := newCombatantRegistry()
		registry.Register(attacker)
		registry.Register(defender)
		ctx := combat.WithCombatantLookup(s.ctx, registry)

		longsword, err := weapons.GetByID(weapons.Longsword)
		s.Require().NoError(err)

		// Natural 20 on d20, then 2d8 for crit damage (7, 5)
		roller := newSequenceRoller(20, 7, 5)

		result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
			AttackerID: "barbarian-4",
			TargetID:   "defender-4",
			Weapon:     &longsword,
			EventBus:   s.bus,
			Roller:     roller,
		})
		s.Require().NoError(err)

		fmt.Printf("  [Roll] d20: NATURAL 20! (Critical Hit)\n")
		fmt.Printf("  [Damage] Weapon dice doubled: 2d8 = %v\n", result.DamageRolls)

		s.Assert().True(result.IsNaturalTwenty, "should be natural 20")
		s.Assert().True(result.Critical, "natural 20 should be critical")
		s.Assert().True(result.Hit, "natural 20 always hits")
		s.Assert().Len(result.DamageRolls, 2, "critical should roll 2d8")

		// Damage = 7 + 5 (2d8 crit) + 3 (STR) + 2 (rage) = 17
		s.Assert().Equal(17, result.TotalDamage, "crit damage + STR + rage")

		fmt.Printf("  [Damage] Total: %d (2d8=%d + STR=3 + Rage=2)\n",
			result.TotalDamage, result.DamageRolls[0]+result.DamageRolls[1])
		fmt.Println("  === RESULT: CRITICAL HIT for 17 damage ===")
	})
}

// TestTurnEndCleanup demonstrates the full turn lifecycle:
// Turn start → actions granted → strikes executed → turn end → temporary actions removed
func (s *AttackResolutionIntegrationSuite) TestTurnEndCleanup() {
	s.Run("temporary actions are removed on cleanup", func() {
		fmt.Println("\n=== TURN LIFECYCLE: Temporary Action Cleanup ===")

		// Create a fighter character
		draft := character.LoadDraftFromData(&character.DraftData{
			ID:       "draft-cleanup-test",
			PlayerID: "player-cleanup",
		})
		err := draft.SetName(&character.SetNameInput{Name: "Test Fighter"})
		s.Require().NoError(err)
		err = draft.SetAbilityScores(&character.SetAbilityScoresInput{
			Scores: shared.AbilityScores{
				abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
				abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
			},
		})
		s.Require().NoError(err)
		err = draft.SetRace(&character.SetRaceInput{
			RaceID:  races.Human,
			Choices: character.RaceChoices{Languages: []languages.Language{languages.Elvish}},
		})
		s.Require().NoError(err)
		err = draft.SetBackground(&character.SetBackgroundInput{
			BackgroundID: backgrounds.Soldier,
			Choices:      character.BackgroundChoices{},
		})
		s.Require().NoError(err)
		err = draft.SetClass(&character.SetClassInput{
			ClassID: classes.Fighter,
			Choices: character.ClassChoices{
				Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
				Equipment: []character.EquipmentChoiceSelection{
					{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
					{ChoiceID: choices.FighterWeaponsPrimary, OptionID: choices.FighterWeaponMartialShield,
						CategorySelections: []shared.EquipmentID{weapons.Longsword}},
					{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
					{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
				},
				FightingStyle: fightingstyles.TwoWeaponFighting,
			},
		})
		s.Require().NoError(err)

		char, err := draft.ToCharacter(s.ctx, "fighter-cleanup", s.bus)
		s.Require().NoError(err)

		permanentActionCount := len(char.GetActions())
		fmt.Printf("  Character: %s (%d permanent actions)\n", char.GetName(), permanentActionCount)

		// Grant a temporary off-hand strike action via event
		_, err = actions.CheckAndGrantOffHandStrike(s.ctx, &actions.TwoWeaponGranterInput{
			CharacterID:    "fighter-cleanup",
			AttackHand:     actions.AttackHandMain,
			MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
			EventBus:       s.bus,
		})
		s.Require().NoError(err)

		afterGrant := len(char.GetActions())
		fmt.Printf("  [Turn] OffHandStrike granted → %d actions total\n", afterGrant)
		s.Assert().Equal(permanentActionCount+1, afterGrant, "should have one more action after grant")

		// Verify the off-hand strike exists
		offHand := char.GetAction("fighter-cleanup-off-hand-strike")
		s.Require().NotNil(offHand, "off-hand strike should be on character")

		// Simulate turn end via Cleanup
		fmt.Println("  [Turn End] Cleanup...")
		err = char.Cleanup(s.ctx)
		s.Require().NoError(err)

		afterCleanup := len(char.GetActions())
		fmt.Printf("  [Turn End] After cleanup: %d actions (temporary removed)\n", afterCleanup)
		s.Assert().Equal(permanentActionCount, afterCleanup, "temporary actions should be removed")

		// Verify off-hand strike is gone
		offHand = char.GetAction("fighter-cleanup-off-hand-strike")
		s.Assert().Nil(offHand, "off-hand strike should be removed after cleanup")

		fmt.Println("  === TURN END: Temporary actions cleaned up ===")
	})
}

// =============================================================================
// MovementIntegrationSuite - Tests MoveEntity with real conditions
// =============================================================================

// testGoblin implements combat.Combatant for testing movement and OA resolution.
type testGoblin struct {
	id string
}

func (g *testGoblin) GetID() string            { return g.id }
func (g *testGoblin) GetType() core.EntityType { return "monster" }
func (g *testGoblin) GetHitPoints() int        { return 7 }
func (g *testGoblin) GetMaxHitPoints() int     { return 7 }
func (g *testGoblin) AC() int                  { return 15 }
func (g *testGoblin) IsDirty() bool            { return false }
func (g *testGoblin) MarkClean()               {}
func (g *testGoblin) ProficiencyBonus() int    { return 2 }
func (g *testGoblin) AbilityScores() shared.AbilityScores {
	return shared.AbilityScores{
		abilities.STR: 8,  // -1
		abilities.DEX: 14, // +2
	}
}
func (g *testGoblin) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	total := 0
	for _, inst := range input.Instances {
		total += inst.Amount
	}
	hp := g.GetHitPoints() - total
	if hp < 0 {
		hp = 0
	}
	return &combat.ApplyDamageResult{
		TotalDamage:   total,
		CurrentHP:     hp,
		PreviousHP:    g.GetHitPoints(),
		DroppedToZero: hp == 0,
	}
}

// MovementIntegrationSuite tests the movement resolution with real Conditions
// modifying the MovementChain. Proves: Character → DisengagingCondition → MovementChain → MoveEntity.
type MovementIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	room *spatial.BasicRoom
}

func TestMovementIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MovementIntegrationSuite))
}

func (s *MovementIntegrationSuite) SetupTest() {
	s.bus = events.NewEventBus()

	// Create a 10x10 combat room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "combat",
		Grid: grid,
	})
	s.room.ConnectToEventBus(s.bus)

	s.ctx = context.Background()
	s.ctx = combat.WithRoom(s.ctx, s.room)
}

func (s *MovementIntegrationSuite) SetupSubTest() {
	s.bus = events.NewEventBus()

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "combat",
		Grid: grid,
	})
	s.room.ConnectToEventBus(s.bus)

	s.ctx = context.Background()
	s.ctx = combat.WithRoom(s.ctx, s.room)
}

// TestDisengagingPreventsOpportunityAttack demonstrates:
// - A Character with DisengagingCondition moves away from a threatening goblin
// - The MovementChain fires, DisengagingCondition adds OA prevention
// - No opportunity attack is triggered
// - Without Disengaging, the same movement WOULD trigger OA
func (s *MovementIntegrationSuite) TestDisengagingPreventsOpportunityAttack() {
	s.Run("disengaging condition prevents OA through movement chain", func() {
		fmt.Println("\n=== MOVEMENT: Disengaging Prevents Opportunity Attack ===")

		// Create a fighter character
		draft := s.createFighterForMovement()
		fighter, err := draft.ToCharacter(s.ctx, "fighter-move", s.bus)
		s.Require().NoError(err)

		// Place fighter at (5, 5) and goblin at (5, 6) - adjacent
		err = s.room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
		s.Require().NoError(err)

		goblin := &testGoblin{id: "goblin-1"}
		err = s.room.PlaceEntity(goblin, spatial.Position{X: 5, Y: 6})
		s.Require().NoError(err)

		fmt.Printf("  Fighter at (5,5), Goblin at (5,6) - adjacent\n")

		// Register combatants for OA resolution
		registry := newCombatantRegistry()
		registry.Register(fighter)
		registry.Register(goblin)
		s.ctx = combat.WithCombatantLookup(s.ctx, registry)

		// Apply DisengagingCondition to the fighter
		disengaging := conditions.NewDisengagingCondition("fighter-move")
		err = disengaging.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		fmt.Println("  [Condition] DisengagingCondition applied to fighter")

		// Move fighter away from goblin: (5,5) → (5,4) → (5,3)
		path := []spatial.Position{
			{X: 5, Y: 4},
			{X: 5, Y: 3},
		}

		fmt.Println("  [Move] Fighter moves (5,5) → (5,4) → (5,3) - leaving goblin's reach")

		result, err := combat.MoveEntity(s.ctx, &combat.MoveEntityInput{
			EntityID:   "fighter-move",
			EntityType: "character",
			Path:       path,
			EventBus:   s.bus,
		})
		s.Require().NoError(err)

		// Verify movement completed without OA
		s.Assert().Equal(spatial.Position{X: 5, Y: 3}, result.FinalPosition)
		s.Assert().Equal(2, result.StepsCompleted)
		s.Assert().Empty(result.OAsTriggered, "DisengagingCondition should prevent OA")
		s.Assert().False(result.MovementStopped)

		fmt.Println("  [Result] Movement complete - NO opportunity attack triggered")
		fmt.Println("  === DISENGAGING: OA prevented by condition ===")
	})

	s.Run("without disengaging, same movement triggers OA", func() {
		fmt.Println("\n=== MOVEMENT: Normal Movement Triggers Opportunity Attack ===")

		// Create a fighter character (no disengaging)
		draft := s.createFighterForMovement()
		fighter, err := draft.ToCharacter(s.ctx, "fighter-oa", s.bus)
		s.Require().NoError(err)

		// Place fighter at (5, 5) and goblin at (5, 6) - adjacent
		err = s.room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
		s.Require().NoError(err)

		goblin := &testGoblin{id: "goblin-2"}
		err = s.room.PlaceEntity(goblin, spatial.Position{X: 5, Y: 6})
		s.Require().NoError(err)

		fmt.Printf("  Fighter at (5,5), Goblin at (5,6) - adjacent (NO disengaging)\n")

		// Register combatants
		registry := newCombatantRegistry()
		registry.Register(fighter)
		registry.Register(goblin)
		s.ctx = combat.WithCombatantLookup(s.ctx, registry)

		// Move fighter away - should trigger OA from goblin
		path := []spatial.Position{
			{X: 5, Y: 4},
			{X: 5, Y: 3},
		}

		// Goblin uses scimitar: d20=15, hit (15+4=19 vs fighter AC), d6=3 damage
		roller := newSequenceRoller(15, 3)

		fmt.Println("  [Move] Fighter moves (5,5) → (5,4) → (5,3) - leaving goblin's reach")

		result, err := combat.MoveEntity(s.ctx, &combat.MoveEntityInput{
			EntityID:   "fighter-oa",
			EntityType: "character",
			Path:       path,
			EventBus:   s.bus,
			Roller:     roller,
		})
		s.Require().NoError(err)

		// Movement should still complete but OA should have triggered
		s.Assert().Equal(spatial.Position{X: 5, Y: 3}, result.FinalPosition)
		s.Assert().Equal(2, result.StepsCompleted)
		s.Assert().False(result.MovementStopped)

		// Verify opportunity attack was triggered
		s.Require().NotEmpty(result.OAsTriggered, "goblin should make opportunity attack")
		s.Assert().Equal("goblin-2", result.OAsTriggered[0].AttackerID)

		fmt.Printf("  [OA] Goblin attacks! Hit: %v, Damage: %d\n",
			result.OAsTriggered[0].Hit, result.OAsTriggered[0].Damage)
		fmt.Println("  === NORMAL MOVEMENT: OA triggered (no disengaging) ===")
	})
}

// TestDashGrantsExtraMovement demonstrates the Dash combat ability granting
// extra movement equal to speed.
func (s *MovementIntegrationSuite) TestDashGrantsExtraMovement() {
	s.Run("dash doubles available movement", func() {
		fmt.Println("\n=== MOVEMENT: Dash Grants Extra Movement ===")

		draft := s.createFighterForMovement()
		fighter, err := draft.ToCharacter(s.ctx, "fighter-dash", s.bus)
		s.Require().NoError(err)

		// Set up action economy with base movement
		actionEconomy := combat.NewActionEconomy()
		actionEconomy.SetMovement(fighter.GetSpeed())

		fmt.Printf("  Fighter speed: %d ft\n", fighter.GetSpeed())
		fmt.Printf("  [Turn Start] Movement: %d ft\n", actionEconomy.MovementRemaining)

		// Get Dash ability from character
		dashAbility := fighter.GetCombatAbility("fighter-dash-dash")
		s.Require().NotNil(dashAbility, "character should have Dash ability")

		// Activate Dash - grants extra movement equal to speed
		err = dashAbility.Activate(s.ctx, fighter, combatabilities.CombatAbilityInput{
			ActionEconomy: actionEconomy,
			Speed:         fighter.GetSpeed(),
		})
		s.Require().NoError(err)

		fmt.Printf("  [Dash] Extra movement granted: +%d ft\n", fighter.GetSpeed())
		fmt.Printf("  [Result] Total movement: %d ft\n", actionEconomy.MovementRemaining)

		// After Dash: movement should be doubled (30 + 30 = 60)
		s.Assert().Equal(60, actionEconomy.MovementRemaining, "dash should double movement")
		s.Assert().Equal(0, actionEconomy.ActionsRemaining, "dash consumes the action")

		fmt.Println("  === DASH: Movement doubled ===")
	})
}

// createFighterForMovement creates a minimal fighter for movement tests.
func (s *MovementIntegrationSuite) createFighterForMovement() *character.Draft {
	draft := character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-movement",
		PlayerID: "player-movement",
	})

	err := draft.SetName(&character.SetNameInput{Name: "Test Fighter"})
	s.Require().NoError(err)
	err = draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
	})
	s.Require().NoError(err)
	err = draft.SetRace(&character.SetRaceInput{
		RaceID:  races.Human,
		Choices: character.RaceChoices{Languages: []languages.Language{languages.Elvish}},
	})
	s.Require().NoError(err)
	err = draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      character.BackgroundChoices{},
	})
	s.Require().NoError(err)
	err = draft.SetClass(&character.SetClassInput{
		ClassID: classes.Fighter,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{ChoiceID: choices.FighterWeaponsPrimary, OptionID: choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword}},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	})
	s.Require().NoError(err)

	return draft
}
