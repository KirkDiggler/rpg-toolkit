package combatabilities_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// fakeStealthOwner implements core.Entity plus GetSkillModifier, satisfying
// Hide's stealthChecker interface. Named per the project's hand-written
// test-double convention (fakeX, not mockX — mockX is reserved for gomock).
type fakeStealthOwner struct {
	id             string
	skillModifiers map[skills.Skill]int
}

func (f *fakeStealthOwner) GetID() string            { return f.id }
func (f *fakeStealthOwner) GetType() core.EntityType { return "character" }
func (f *fakeStealthOwner) GetSkillModifier(skill skills.Skill) int {
	return f.skillModifiers[skill]
}

// captureConditionApplied subscribes to ConditionAppliedTopic and returns a
// pointer to the growing slice of captured events — the caller reads
// (*captured) after the action under test runs. A bare returned pointer
// would snapshot the pre-publish nil value; the slice-pointer indirection
// mirrors the encounter package's subscribeConditions helper.
func captureConditionApplied(
	t *testing.T, ctx context.Context, bus events.EventBus,
) *[]dnd5eEvents.ConditionAppliedEvent {
	t.Helper()
	captured := &[]dnd5eEvents.ConditionAppliedEvent{}
	_, err := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, e dnd5eEvents.ConditionAppliedEvent) error {
			*captured = append(*captured, e)
			return nil
		})
	require.NoError(t, err, "subscribe to ConditionAppliedTopic should not fail")
	return captured
}

// HelpAbilityTestSuite covers the Help combat ability: it consumes the
// standard action, publishes HelpActivatedEvent with the target ally's id,
// and applies HelpedCondition to the ally via a cross-entity
// ConditionAppliedEvent (target-threading, rpg-toolkit#716).
type HelpAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	ally          *mockOwner
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
	s.ally = &mockOwner{id: "test-ally"}
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
		ActionEconomy: s.actionEconomy, Bus: s.bus, Target: s.ally,
	})
	s.Require().NoError(err)
}

func (s *HelpAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	s.Require().NoError(s.actionEconomy.UseAction())
	err := s.help.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus, Target: s.ally,
	})
	s.Require().Error(err)
}

func (s *HelpAbilityTestSuite) TestCanActivate_RequiresEventBus() {
	err := s.help.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: nil, Target: s.ally,
	})
	s.Require().Error(err)
}

func (s *HelpAbilityTestSuite) TestCanActivate_RequiresTarget() {
	err := s.help.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "target ally required")
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
		ActionEconomy: s.actionEconomy, Bus: s.bus, Target: s.ally,
	})
	s.Require().NoError(err)
	s.Equal(0, s.actionEconomy.ActionsRemaining, "Help consumes the standard action")
	s.True(received, "HelpActivatedEvent should be published")
	s.Equal(s.owner.GetID(), got.CharacterID)
	s.Equal(s.ally.GetID(), got.AllyID, "AllyID should be the target's id, not empty")
}

// TestActivate_AppliesHelpedConditionToAlly is the regression test for R1's
// label fix: the condition is attributed to the ALLY (the target), not the
// helper.
func (s *HelpAbilityTestSuite) TestActivate_AppliesHelpedConditionToAlly() {
	captured := captureConditionApplied(s.T(), s.ctx, s.bus)

	err := s.help.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus, Target: s.ally,
	})
	s.Require().NoError(err)

	s.Require().Len(*captured, 1, "ConditionAppliedEvent should be published")
	got := (*captured)[0]
	s.Equal(s.ally.GetID(), got.Target.GetID(), "condition targets the ally, not the helper")
	s.Equal(dnd5eEvents.ConditionHelped, got.Type)
	s.Equal(dnd5eEvents.ConditionSourceCombatAbility, got.Source)

	helped, ok := got.Condition.(*conditions.HelpedCondition)
	s.Require().True(ok, "condition should be a *HelpedCondition")
	s.Equal(s.ally.GetID(), helped.CharacterID)
	s.Equal(s.owner.GetID(), helped.HelperID, "HelperID tracks the safety-net turn trigger")
}

func (s *HelpAbilityTestSuite) TestActivate_NoEventBus() {
	err := s.help.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Target: s.ally,
	})
	s.Require().Error(err)
}

func (s *HelpAbilityTestSuite) TestActivate_RequiresTarget() {
	err := s.help.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "target ally required")
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

// HideAbilityTestSuite covers the Hide combat ability: it consumes the
// standard action, makes a Stealth check via checks.MakeAbilityCheck against
// the caller-gathered observer passive Perceptions, and applies
// HiddenCondition on success only.
type HideAbilityTestSuite struct {
	suite.Suite
	ctrl          *gomock.Controller
	ctx           context.Context
	bus           events.EventBus
	owner         *fakeStealthOwner
	actionEconomy *combat.ActionEconomy
	hide          *combatabilities.Hide
	mockRoller    *mock_dice.MockRoller
}

func TestHideAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(HideAbilityTestSuite))
}

func (s *HideAbilityTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &fakeStealthOwner{id: "test-sneak", skillModifiers: map[skills.Skill]int{skills.Stealth: 3}}
	s.actionEconomy = combat.NewActionEconomy()
	s.hide = combatabilities.NewHide("test-hide")
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *HideAbilityTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *HideAbilityTestSuite) TestNewHide_Properties() {
	s.Equal("test-hide", s.hide.GetID())
	s.Equal(core.EntityType("combat_ability"), s.hide.GetType())
	s.Equal("Hide", s.hide.Name())
	s.Equal(coreCombat.ActionStandard, s.hide.ActionType())
	s.Equal(refs.CombatAbilities.Hide(), s.hide.Ref())
}

func (s *HideAbilityTestSuite) TestCanActivate_RequiresEventBus() {
	err := s.hide.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: nil,
	})
	s.Require().Error(err)
}

func (s *HideAbilityTestSuite) TestCanActivate_RequiresStealthChecker() {
	plainOwner := &mockOwner{id: "no-skills"}
	err := s.hide.CanActivate(s.ctx, plainOwner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy, Bus: s.bus,
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "skill checks")
}

// TestActivate_Success covers the Hide-success path (the live MCP playtest
// shape, R6): a Stealth total that beats the highest observer passive
// Perception applies HiddenCondition.
func (s *HideAbilityTestSuite) TestActivate_Success() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil) // 14 + 3 modifier = 17

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
	captured := captureConditionApplied(s.T(), s.ctx, s.bus)

	err = s.hide.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy:              s.actionEconomy,
		Bus:                        s.bus,
		Roller:                     s.mockRoller,
		ObserverPassivePerceptions: []int{12, 15}, // highest is 15; 17 beats it
	})
	s.Require().NoError(err)
	s.Equal(0, s.actionEconomy.ActionsRemaining, "Hide consumes the standard action")
	s.True(received, "HideActivatedEvent should be published")
	s.Equal(s.owner.GetID(), got.CharacterID)

	s.Require().Len(*captured, 1, "a successful Hide applies HiddenCondition")
	gotCond := (*captured)[0]
	s.Equal(s.owner.GetID(), gotCond.Target.GetID())
	s.Equal(dnd5eEvents.ConditionHidden, gotCond.Type)
	s.Equal(dnd5eEvents.ConditionSourceCombatAbility, gotCond.Source)

	hidden, ok := gotCond.Condition.(*conditions.HiddenCondition)
	s.Require().True(ok, "condition should be a *HiddenCondition")
	s.Equal(s.owner.GetID(), hidden.CharacterID)
}

// TestActivate_Failure covers the R6-deferred failure branch as a unit test
// (the design doc explicitly punts this to checks.MakeAbilityCheck coverage
// since the live orchestrator has no deterministic-roller seam — this test
// exercises the same branch one layer up, through Hide.Activate).
func (s *HideAbilityTestSuite) TestActivate_Failure_NoConditionApplied() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(2, nil) // 2 + 3 modifier = 5

	received := false
	_, err := dnd5eEvents.HideActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(_ context.Context, _ dnd5eEvents.HideActivatedEvent) error {
			received = true
			return nil
		},
	)
	s.Require().NoError(err)
	captured := captureConditionApplied(s.T(), s.ctx, s.bus)

	err = s.hide.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy:              s.actionEconomy,
		Bus:                        s.bus,
		Roller:                     s.mockRoller,
		ObserverPassivePerceptions: []int{15},
	})
	s.Require().NoError(err, "a failed Hide check is not an error — the action is still spent")
	s.Equal(0, s.actionEconomy.ActionsRemaining, "the action is consumed even on a failed check")
	s.True(received, "HideActivatedEvent still fires on failure")
	s.Empty(*captured, "no HiddenCondition on a failed check")
}

// TestActivate_NoObservers covers the empty-observer-set case: DC defaults
// to 0, so the check trivially succeeds.
func (s *HideAbilityTestSuite) TestActivate_NoObservers() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil) // even a nat 1 (1+3=4) beats DC 0

	captured := captureConditionApplied(s.T(), s.ctx, s.bus)

	err := s.hide.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
		Roller:        s.mockRoller,
	})
	s.Require().NoError(err)
	s.Require().Len(*captured, 1, "no observers means the check trivially succeeds")
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
