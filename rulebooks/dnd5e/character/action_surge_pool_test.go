// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// ActionSurgePoolSuite pins that the fighter's Action Surge is counted in the
// character's shared pool, used only from its description, and has no code of
// its own (rpg-project#1131, slice 6a).
type ActionSurgePoolSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func TestActionSurgePoolSuite(t *testing.T) { suite.Run(t, new(ActionSurgePoolSuite)) }

func (s *ActionSurgePoolSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.char = newLevelTwoFighter(s.ctx, s.T(), "surge-fighter", s.bus)
}

func (s *ActionSurgePoolSuite) startFight(turn int) {
	_, err := s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: turn, Speed: 30})
	s.Require().NoError(err)
	s.Require().True(s.char.InCombat())
}

func (s *ActionSurgePoolSuite) surgeFeature() features.Feature {
	feature := s.char.GetFeature(refs.Features.ActionSurge().ID)
	s.Require().NotNil(feature, "the level-2 fighter carries Action Surge")
	return feature
}

func (s *ActionSurgePoolSuite) surgeRow() AvailableAbility {
	for _, row := range s.char.buildAvailableAbilities() {
		if row.Ref != nil && row.Ref.ID == refs.Features.ActionSurge().ID {
			return row
		}
	}
	s.FailNow("no Action Surge row among her abilities")
	return AvailableAbility{}
}

// Test 1: the pool is hers.
func (s *ActionSurgePoolSuite) TestALevelTwoFighterHasThePoolAndALevelOneFighterHasNone() {
	pool, ok := s.char.resources[resources.ActionSurge]
	s.Require().True(ok, "a level-2 fighter carries the shared action_surge pool")
	s.Equal(1, pool.Current())
	s.Equal(1, pool.Maximum())
	s.Equal(coreResources.ResetShortRest, pool.ResetType)

	levelOne, err := newFighterDraft(s.T()).ToCharacter(s.ctx, "level-one", events.NewEventBus())
	s.Require().NoError(err)
	_, has := levelOne.resources[resources.ActionSurge]
	s.False(has, "a level-1 fighter has no Action Surge pool")
	_, has = buildClassResources(&Character{id: "x"}, classes.Fighter, 1, 1)[resources.ActionSurge]
	s.False(has)
	built, has := buildClassResources(&Character{id: "x"}, classes.Fighter, 2, 2)[resources.ActionSurge]
	s.Require().True(has)
	s.Equal(1, built.Maximum())
	_, has = buildClassResources(&Character{id: "x"}, classes.Barbarian, 2, 2)[resources.ActionSurge]
	s.False(has, "no other class gets it")
}

// Test 2: it arrives with the level, and the level says so.
func (s *ActionSurgePoolSuite) TestTheLevelThatGrantsItGivesThePoolFullAndReportsIt() {
	char, err := newFighterDraft(s.T()).ToCharacter(s.ctx, "rising", events.NewEventBus())
	s.Require().NoError(err)
	char.experience = ExperienceThresholdForLevel(2)
	_, has := char.resources[resources.ActionSurge]
	s.Require().False(has)

	out, err := char.Advance(s.ctx, &AdvanceInput{ClassID: classes.Fighter, HitPointMethod: HitPointMethodAverage})
	s.Require().NoError(err)

	pool, has := char.resources[resources.ActionSurge]
	s.Require().True(has)
	s.Equal(1, pool.Current(), "arrives full")
	s.Equal(1, pool.Maximum())
	s.Equal([]ResourceChange{
		{Key: resources.ActionSurge, From: 0, To: 1},
		{Key: resources.HitDice, From: 1, To: 2},
	}, out.Gained.Resources)
}

// Test 3: her row.
func (s *ActionSurgePoolSuite) TestHerRowOffersItAsAFreeActionWhileThePoolHasAUse() {
	s.startFight(1)

	row := s.surgeRow()
	s.Equal(coreCombat.ActionFree, row.ActionType)
	s.Equal(EconomySlotFree, row.EconomySlot)
	s.True(row.CanUse, "usable: %s", row.Reason)
	s.Equal(1, row.ResourceCurrent)
	s.Equal(1, row.ResourceMax)

	s.char.actionEconomy.ActionsRemaining = 0
	s.char.actionEconomy.BonusActionsRemaining = 0
	row = s.surgeRow()
	s.True(row.CanUse, "still usable with her action and bonus action both spent: %s", row.Reason)

	s.Require().NoError(s.char.UseResource(resources.ActionSurge, 1))
	row = s.surgeRow()
	s.False(row.CanUse)
	s.Equal("no action surge uses remaining", row.Reason)
	s.Equal(0, row.ResourceCurrent)
	s.Equal(1, row.ResourceMax)
}

// Test 4: her description.
func (s *ActionSurgePoolSuite) TestHerDescriptionIsOnlyItsPrice() {
	definition, ok, err := s.char.AbilityDefinition(*refs.Features.ActionSurge())
	s.Require().NoError(err)
	s.Require().True(ok)

	s.True(definition.OnlyItsPrice())
	s.Equal(map[coreResources.ResourceKey]int{resources.ActionSurge: 1}, definition.Cost.Pools)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}, definition.Cost.GrantSlots)
	s.Empty(definition.Cost.Slots, "no slot is charged")
	s.Equal(*refs.Features.ActionSurge(), definition.Ref)
	s.Equal("Action Surge", definition.Name)
	s.Equal("Spend a use to take one additional action this turn.", definition.Description)
	// The Fighter is named as the giver when the description is built; the
	// description itself carries no giver, so the proof it was named is that
	// the build succeeded (the named-ability table refuses a giver with no Ref).
}

// Test 5: paid at the gate.
func (s *ActionSurgePoolSuite) TestPaidAtTheGateTheUseIsSpentAndTheTurnGainsAnAction() {
	s.startFight(1)
	s.Require().Equal(1, s.char.actionEconomy.ActionsRemaining)

	s.Require().NoError(useActionSurgeFromItsDescription(s.T(), s.char))
	current, maximum, _ := s.char.ResourceStatus(resources.ActionSurge)
	s.Equal(0, current)
	s.Equal(1, maximum)
	s.Equal(2, s.char.actionEconomy.ActionsRemaining, "one more action")

	s.Require().Error(useActionSurgeFromItsDescription(s.T(), s.char), "a second payment is refused")
	current, _, _ = s.char.ResourceStatus(resources.ActionSurge)
	s.Equal(0, current)
	s.Equal(2, s.char.actionEconomy.ActionsRemaining, "and nothing changed")
}

// Test 6: her own code refuses.
func (s *ActionSurgePoolSuite) TestHerOwnCodeRefusesAndTouchesNothing() {
	s.startFight(1)
	actionsBefore := s.char.actionEconomy.ActionsRemaining

	err := s.surgeFeature().Activate(s.ctx, s.char, features.FeatureInput{Bus: s.bus})
	s.Require().Error(err)
	s.True(errors.Is(err, features.ErrUsedFromItsDescription), "got %v", err)

	out, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.ActionSurge()})
	s.Require().NoError(err)
	s.False(out.Success)
	s.Equal(features.ErrUsedFromItsDescription.Error(), out.Error)

	current, _, _ := s.char.ResourceStatus(resources.ActionSurge)
	s.Equal(1, current, "the pool is unchanged")
	s.Equal(actionsBefore, s.char.actionEconomy.ActionsRemaining, "her turn is unchanged")
}

// Test 7: rests.
func (s *ActionSurgePoolSuite) TestRestsGiveASpentUseBack() {
	for name, rest := range map[string]func() error{
		"short rest": func() error { _, err := s.char.ShortRest(s.ctx, &ShortRestInput{}); return err },
		"long rest":  func() error { _, err := s.char.LongRest(s.ctx); return err },
	} {
		s.Run(name, func() {
			s.Require().NoError(s.char.UseResource(resources.ActionSurge, 1))
			s.Require().False(s.char.IsResourceAvailable(resources.ActionSurge))

			s.Require().NoError(rest())

			current, maximum, ok := s.char.ResourceStatus(resources.ActionSurge)
			s.Require().True(ok)
			s.Equal(1, current)
			s.Equal(1, maximum)
		})
	}
}

// Test 8: saved and loaded.
func (s *ActionSurgePoolSuite) TestASpentUseSurvivesASaveAndLoad() {
	s.Require().NoError(s.char.UseResource(resources.ActionSurge, 1))

	data, err := s.char.ToData()
	s.Require().NoError(err)
	s.Equal(0, data.Resources[resources.ActionSurge].Current, "the count is in the saved resources")
	s.Equal(1, data.Resources[resources.ActionSurge].Maximum)
	for _, raw := range data.Features {
		var envelope map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(raw, &envelope))
		if string(envelope["id"]) == `"`+refs.Features.ActionSurge().ID+`"` {
			s.NotContains(envelope, "uses", "the feature's saved form has no count")
			s.NotContains(envelope, "max_uses")
		}
	}

	loaded, err := Load(s.ctx, data)
	s.Require().NoError(err)
	current, maximum, ok := loaded.ResourceStatus(resources.ActionSurge)
	s.Require().True(ok)
	s.Equal(0, current, "still spent")
	s.Equal(1, maximum)
}

// Test 9: the status view.
func (s *ActionSurgePoolSuite) TestHerSheetShowsOneActionSurgeRowAndTheFeatureAgrees() {
	for _, spent := range []bool{false, true} {
		if spent {
			s.Require().NoError(s.char.UseResource(resources.ActionSurge, 1))
		}
		want := 1
		if spent {
			want = 0
		}
		out, err := s.char.StatusView(&StatusViewInput{})
		s.Require().NoError(err)
		var rows []ResourceView
		for _, row := range out.View.Resources {
			if row.Key == resources.ActionSurge {
				rows = append(rows, row)
			}
		}
		s.Require().Len(rows, 1, "one action_surge row")
		s.Equal("Action Surge", rows[0].Name)
		s.Equal(want, rows[0].Current)
		s.Equal(1, rows[0].Maximum)

		status, err := s.surgeFeature().Status(&features.StatusInput{Owner: s.char})
		s.Require().NoError(err)
		s.Equal(rows[0].Current, status.Status.Resource.Current)
		s.Equal(rows[0].Maximum, status.Status.Resource.Maximum)
	}

	s.True(ownerResourceAllowed(classes.Fighter, resources.ActionSurge))
	s.False(ownerResourceAllowed(classes.Monk, resources.ActionSurge), "another class's sheet is still refused")
}
