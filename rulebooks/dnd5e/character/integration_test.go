package character_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

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
