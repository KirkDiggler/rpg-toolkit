package combatabilities_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

// HelpAbilityTestSuite covers the Help combat ability (#697 Beat-1): it consumes
// the standard action and publishes HelpActivatedEvent. The mechanical effect
// (advantage to the ally) is a later beat — these tests cover the constructor,
// economy spend, activation signal, and persistence round-trip.
type HelpAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	help          *combatabilities.Help
}

func TestHelpAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(HelpAbilityTestSuite))
}

func (s *HelpAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-helper"}
	s.actionEconomy = combat.NewActionEconomy()
	s.help = combatabilities.NewHelp("test-help")
}

func (s *HelpAbilityTestSuite) TestNewHelp_Properties() {
	s.Equal("test-help", s.help.GetID())
	s.Equal(core.EntityType("combat_ability"), s.help.GetType())
	s.Equal("Help", s.help.Name())
	s.Equal(coreCombat.ActionStandard, s.help.ActionType())
	s.Equal(refs.CombatAbilities.Help(), s.help.Ref())
}

func (s *HelpAbilityTestSuite) TestCanActivate_Success() {
	err := s.help.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().NoError(err)
}

func (s *HelpAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	s.Require().NoError(s.actionEconomy.UseAction())
	err := s.help.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().Error(err)
}

func (s *HelpAbilityTestSuite) TestCanActivate_RequiresEventBus() {
	err := s.help.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: nil,
	})
	s.Require().Error(err)
}

func (s *HelpAbilityTestSuite) TestActivate_ConsumesActionAndPublishes() {
	received := false
	var got dnd5eEvents.HelpActivatedEvent
	_, err := dnd5eEvents.HelpActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(_ context.Context, e dnd5eEvents.HelpActivatedEvent) error {
			received = true
			got = e
			return nil
		},
	)
	s.Require().NoError(err)

	err = s.help.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().NoError(err)
	s.Equal(0, s.actionEconomy.ActionsRemaining, "Help consumes the standard action")
	s.True(received, "HelpActivatedEvent should be published")
	s.Equal(s.owner.GetID(), got.CharacterID)
}

func (s *HelpAbilityTestSuite) TestActivate_NoEventBus() {
	err := s.help.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})
	s.Require().Error(err)
}

func (s *HelpAbilityTestSuite) TestToJSON_AndLoadRoundTrip() {
	jsonData, err := s.help.ToJSON()
	s.Require().NoError(err)

	var data combatabilities.HelpData
	s.Require().NoError(json.Unmarshal(jsonData, &data))
	s.Equal("test-help", data.ID)
	s.Equal(refs.CombatAbilities.Help(), data.Ref)

	loaded, err := combatabilities.LoadJSON(jsonData)
	s.Require().NoError(err)
	s.Equal("Help", loaded.Name())
	s.Equal(refs.CombatAbilities.Help().ID, loaded.Ref().ID)
}

// HideAbilityTestSuite covers the Hide combat ability (#697 Beat-1): it consumes
// the standard action and publishes HideActivatedEvent. The Stealth check + the
// Hidden condition are a later beat.
type HideAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	hide          *combatabilities.Hide
}

func TestHideAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(HideAbilityTestSuite))
}

func (s *HideAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-sneak"}
	s.actionEconomy = combat.NewActionEconomy()
	s.hide = combatabilities.NewHide("test-hide")
}

func (s *HideAbilityTestSuite) TestNewHide_Properties() {
	s.Equal("test-hide", s.hide.GetID())
	s.Equal(core.EntityType("combat_ability"), s.hide.GetType())
	s.Equal("Hide", s.hide.Name())
	s.Equal(coreCombat.ActionStandard, s.hide.ActionType())
	s.Equal(refs.CombatAbilities.Hide(), s.hide.Ref())
}

func (s *HideAbilityTestSuite) TestActivate_ConsumesActionAndPublishes() {
	received := false
	var got dnd5eEvents.HideActivatedEvent
	_, err := dnd5eEvents.HideActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(_ context.Context, e dnd5eEvents.HideActivatedEvent) error {
			received = true
			got = e
			return nil
		},
	)
	s.Require().NoError(err)

	err = s.hide.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().NoError(err)
	s.Equal(0, s.actionEconomy.ActionsRemaining, "Hide consumes the standard action")
	s.True(received, "HideActivatedEvent should be published")
	s.Equal(s.owner.GetID(), got.CharacterID)
}

func (s *HideAbilityTestSuite) TestCanActivate_RequiresEventBus() {
	err := s.hide.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: nil,
	})
	s.Require().Error(err)
}

func (s *HideAbilityTestSuite) TestToJSON_AndLoadRoundTrip() {
	jsonData, err := s.hide.ToJSON()
	s.Require().NoError(err)

	var data combatabilities.HideData
	s.Require().NoError(json.Unmarshal(jsonData, &data))
	s.Equal("test-hide", data.ID)
	s.Equal(refs.CombatAbilities.Hide(), data.Ref)

	loaded, err := combatabilities.LoadJSON(jsonData)
	s.Require().NoError(err)
	s.Equal("Hide", loaded.Name())
	s.Equal(refs.CombatAbilities.Hide().ID, loaded.Ref().ID)
}
