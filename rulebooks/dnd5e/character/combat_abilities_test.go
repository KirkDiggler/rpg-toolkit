package character_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CombatAbilitiesTestSuite tests combat abilities integration with Character
type CombatAbilitiesTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

// SetupTest runs before each test function
func (s *CombatAbilitiesTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// SetupSubTest resets test data before each subtest
func (s *CombatAbilitiesTestSuite) SetupSubTest() {
	// Create fresh event bus for each subtest
	s.bus = events.NewEventBus()
}

// Helper: Create a minimal valid draft that can be converted to character
func (s *CombatAbilitiesTestSuite) createFighterDraft() *character.Draft {
	draft := character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-combat-test",
		PlayerID: "player-001",
	})

	// Set name
	err := draft.SetName(&character.SetNameInput{Name: "Combat Test Fighter"})
	s.Require().NoError(err)

	// Set base ability scores
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

	// Set class with all required choices
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
			FightingStyle: fightingstyles.Defense,
		},
	})
	s.Require().NoError(err)

	return draft
}

func (s *CombatAbilitiesTestSuite) TestCharacterGetsStandardCombatAbilities() {
	s.Run("finalized character has Attack ability", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-001", s.bus)
		s.Require().NoError(err)

		abilities := char.GetCombatAbilities()
		s.Assert().NotEmpty(abilities, "character should have combat abilities")

		attackAbility := char.GetCombatAbility("char-001-attack")
		s.Require().NotNil(attackAbility, "character should have Attack ability")
		s.Assert().Equal("Attack", attackAbility.Name())
	})

	s.Run("finalized character has Dash ability", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-002", s.bus)
		s.Require().NoError(err)

		dashAbility := char.GetCombatAbility("char-002-dash")
		s.Require().NotNil(dashAbility, "character should have Dash ability")
		s.Assert().Equal("Dash", dashAbility.Name())
	})

	s.Run("finalized character has Dodge ability", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-003", s.bus)
		s.Require().NoError(err)

		dodgeAbility := char.GetCombatAbility("char-003-dodge")
		s.Require().NotNil(dodgeAbility, "character should have Dodge ability")
		s.Assert().Equal("Dodge", dodgeAbility.Name())
	})

	s.Run("finalized character has Disengage ability", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-004", s.bus)
		s.Require().NoError(err)

		disengageAbility := char.GetCombatAbility("char-004-disengage")
		s.Require().NotNil(disengageAbility, "character should have Disengage ability")
		s.Assert().Equal("Disengage", disengageAbility.Name())
	})

	s.Run("finalized character has all four standard combat abilities", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-005", s.bus)
		s.Require().NoError(err)

		abilities := char.GetCombatAbilities()
		s.Assert().Len(abilities, 4, "character should have exactly 4 standard combat abilities")

		// Verify all four are present
		abilityNames := make(map[string]bool)
		for _, ability := range abilities {
			abilityNames[ability.Name()] = true
		}

		s.Assert().True(abilityNames["Attack"], "should have Attack")
		s.Assert().True(abilityNames["Dash"], "should have Dash")
		s.Assert().True(abilityNames["Dodge"], "should have Dodge")
		s.Assert().True(abilityNames["Disengage"], "should have Disengage")
	})
}

func (s *CombatAbilitiesTestSuite) TestCharacterGetsStandardActions() {
	s.Run("finalized character has Strike action", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-006", s.bus)
		s.Require().NoError(err)

		actions := char.GetActions()
		s.Assert().NotEmpty(actions, "character should have actions")

		strikeAction := char.GetAction("char-006-strike")
		s.Require().NotNil(strikeAction, "character should have Strike action")
	})

	s.Run("finalized character has Move action", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-007", s.bus)
		s.Require().NoError(err)

		moveAction := char.GetAction("char-007-move")
		s.Require().NotNil(moveAction, "character should have Move action")
	})

	s.Run("finalized character has both Strike and Move actions", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-008", s.bus)
		s.Require().NoError(err)

		actions := char.GetActions()
		s.Assert().GreaterOrEqual(len(actions), 2, "character should have at least 2 actions")

		// Verify Strike and Move are present
		var hasStrike, hasMove bool
		for _, action := range actions {
			if action.GetID() == "char-008-strike" {
				hasStrike = true
			}
			if action.GetID() == "char-008-move" {
				hasMove = true
			}
		}

		s.Assert().True(hasStrike, "should have Strike action")
		s.Assert().True(hasMove, "should have Move action")
	})
}

func (s *CombatAbilitiesTestSuite) TestCombatAbilityHolder() {
	s.Run("character implements CombatAbilityHolder", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-009", s.bus)
		s.Require().NoError(err)

		// Verify the interface is implemented
		var holder combatabilities.CombatAbilityHolder = char
		s.Assert().NotNil(holder)
	})

	s.Run("AddCombatAbility adds ability to character", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-010", s.bus)
		s.Require().NoError(err)

		initialCount := len(char.GetCombatAbilities())

		// Add a new bonus dash ability (like from Rogue Cunning Action)
		bonusDash := combatabilities.NewBonusDash("bonus-dash-test")
		err = char.AddCombatAbility(bonusDash)
		s.Require().NoError(err)

		s.Assert().Len(char.GetCombatAbilities(), initialCount+1)
	})

	s.Run("AddCombatAbility returns error for nil", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-011", s.bus)
		s.Require().NoError(err)

		err = char.AddCombatAbility(nil)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "cannot be nil")
	})

	s.Run("RemoveCombatAbility removes ability from character", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-012", s.bus)
		s.Require().NoError(err)

		initialCount := len(char.GetCombatAbilities())

		// Remove the Attack ability
		err = char.RemoveCombatAbility("char-012-attack")
		s.Require().NoError(err)

		s.Assert().Len(char.GetCombatAbilities(), initialCount-1)
		s.Assert().Nil(char.GetCombatAbility("char-012-attack"))
	})

	s.Run("RemoveCombatAbility returns error for nonexistent", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-013", s.bus)
		s.Require().NoError(err)

		err = char.RemoveCombatAbility("nonexistent-ability")
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "not found")
	})

	s.Run("GetCombatAbility returns nil for nonexistent", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-014", s.bus)
		s.Require().NoError(err)

		ability := char.GetCombatAbility("nonexistent")
		s.Assert().Nil(ability)
	})
}

func (s *CombatAbilitiesTestSuite) TestEventBasedActionGranting() {
	s.Run("character adds action when ActionGrantedEvent is published", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-event-001", s.bus)
		s.Require().NoError(err)

		initialActionCount := len(char.GetActions())

		// Create a FlurryStrike action (temporary action)
		flurryStrike := actions.NewFlurryStrike(actions.FlurryStrikeConfig{
			ID:      "test-flurry-strike-1",
			OwnerID: "char-event-001",
		})

		// Publish ActionGrantedEvent
		topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
		err = topic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
			CharacterID: "char-event-001",
			Action:      flurryStrike,
			Source:      "test",
		})
		s.Require().NoError(err)

		// Verify the action was added
		s.Assert().Len(char.GetActions(), initialActionCount+1)
		addedAction := char.GetAction("test-flurry-strike-1")
		s.Require().NotNil(addedAction, "character should have the granted action")
	})

	s.Run("character ignores ActionGrantedEvent for other characters", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-event-002", s.bus)
		s.Require().NoError(err)

		initialActionCount := len(char.GetActions())

		// Create a FlurryStrike action for a different character
		flurryStrike := actions.NewFlurryStrike(actions.FlurryStrikeConfig{
			ID:      "test-flurry-strike-2",
			OwnerID: "other-character",
		})

		// Publish ActionGrantedEvent for different character
		topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
		err = topic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
			CharacterID: "other-character", // Different character
			Action:      flurryStrike,
			Source:      "test",
		})
		s.Require().NoError(err)

		// Verify the action was NOT added to our character
		s.Assert().Len(char.GetActions(), initialActionCount)
		s.Assert().Nil(char.GetAction("test-flurry-strike-2"))
	})

	s.Run("character removes action when ActionRemovedEvent is published", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-event-003", s.bus)
		s.Require().NoError(err)

		// First add an action via event
		flurryStrike := actions.NewFlurryStrike(actions.FlurryStrikeConfig{
			ID:      "test-flurry-strike-3",
			OwnerID: "char-event-003",
		})

		grantTopic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
		err = grantTopic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
			CharacterID: "char-event-003",
			Action:      flurryStrike,
			Source:      "test",
		})
		s.Require().NoError(err)

		// Verify action was added
		s.Require().NotNil(char.GetAction("test-flurry-strike-3"))
		actionCountAfterAdd := len(char.GetActions())

		// Now publish ActionRemovedEvent
		removeTopic := dnd5eEvents.ActionRemovedTopic.On(s.bus)
		err = removeTopic.Publish(s.ctx, dnd5eEvents.ActionRemovedEvent{
			ActionID: "test-flurry-strike-3",
			OwnerID:  "char-event-003",
		})
		s.Require().NoError(err)

		// Verify the action was removed
		s.Assert().Len(char.GetActions(), actionCountAfterAdd-1)
		s.Assert().Nil(char.GetAction("test-flurry-strike-3"))
	})

	s.Run("character ignores ActionRemovedEvent for other characters", func() {
		draft := s.createFighterDraft()

		char, err := draft.ToCharacter(s.ctx, "char-event-004", s.bus)
		s.Require().NoError(err)

		// Add an action first
		flurryStrike := actions.NewFlurryStrike(actions.FlurryStrikeConfig{
			ID:      "test-flurry-strike-4",
			OwnerID: "char-event-004",
		})

		grantTopic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
		err = grantTopic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
			CharacterID: "char-event-004",
			Action:      flurryStrike,
			Source:      "test",
		})
		s.Require().NoError(err)

		actionCountAfterAdd := len(char.GetActions())

		// Publish ActionRemovedEvent for different character
		removeTopic := dnd5eEvents.ActionRemovedTopic.On(s.bus)
		err = removeTopic.Publish(s.ctx, dnd5eEvents.ActionRemovedEvent{
			ActionID: "test-flurry-strike-4",
			OwnerID:  "other-character", // Different character
		})
		s.Require().NoError(err)

		// Verify the action was NOT removed
		s.Assert().Len(char.GetActions(), actionCountAfterAdd)
		s.Assert().NotNil(char.GetAction("test-flurry-strike-4"))
	})
}

func TestCombatAbilitiesSuite(t *testing.T) {
	suite.Run(t, new(CombatAbilitiesTestSuite))
}
