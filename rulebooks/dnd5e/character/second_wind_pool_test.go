package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// SecondWindPoolSuite pins that Second Wind's uses are counted in the
// character's shared pool (rpg-project#1131), the way Rage's are.
type SecondWindPoolSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func (s *SecondWindPoolSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	char, err := newFighterDraft(s.T()).ToCharacter(s.ctx, "pool-fighter", s.bus)
	s.Require().NoError(err)
	s.char = char
}

// startFight puts the fighter in a fight on turn n, hurt so a heal lands.
func (s *SecondWindPoolSuite) startFight(turn int) {
	_, err := s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: turn, Speed: 30})
	s.Require().NoError(err)
	s.Require().True(s.char.InCombat())
}

func (s *SecondWindPoolSuite) secondWindFeature() features.Feature {
	for _, f := range s.char.features {
		if f.Ref() != nil && f.Ref().ID == refs.Features.SecondWind().ID {
			return f
		}
	}
	s.FailNow("the fighter carries no Second Wind")
	return nil
}

func (s *SecondWindPoolSuite) TestANewFighterHasThePool() {
	pool, ok := s.char.resources[resources.SecondWind]
	s.Require().True(ok, "a level-1 fighter carries the shared second_wind pool")
	s.Equal(1, pool.Current())
	s.Equal(1, pool.Maximum())
	s.Equal(coreResources.ResetShortRest, pool.ResetType)
}

func (s *SecondWindPoolSuite) TestUsingSecondWindSpendsThePoolAndHealsAsBefore() {
	s.char.hitPoints = 8
	s.char.maxHitPoints = 20
	markSaved(s.char)
	s.startFight(1)

	var received *dnd5eEvents.HealingReceivedEvent
	_, err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			received = &event
			return nil
		})
	s.Require().NoError(err)

	out, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.SecondWind(),
		Roller:     &countingBridgeRoller{face: 6},
	})
	s.Require().NoError(err)
	s.Require().True(out.Success, "second wind activates: %s", out.Error)

	current, maximum, ok := s.char.ResourceStatus(resources.SecondWind)
	s.Require().True(ok)
	s.Equal(0, current, "the use is spent from the shared pool")
	s.Equal(1, maximum)
	s.Equal(0, s.char.actionEconomy.BonusActionsRemaining, "it takes the bonus action")
	s.Require().NotNil(received, "the same healing event is announced")
	s.Equal(7, received.Amount, "a face of 6 plus fighter level 1")
}

func (s *SecondWindPoolSuite) TestASecondUseIsRefusedLikeASpentRage() {
	s.startFight(1)
	_, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.SecondWind(), Roller: &countingBridgeRoller{face: 3},
	})
	s.Require().NoError(err)

	// The same refusal a spent Rage gives for its pool.
	refusal := s.secondWindFeature().CanActivate(s.ctx, s.char, features.FeatureInput{})
	s.Require().Error(refusal)
	s.Equal(rpgerr.CodeResourceExhausted, rpgerr.GetCode(refusal), "got %v", refusal)
	s.Contains(refusal.Error(), "no second wind uses remaining")

	// A fresh turn gives the bonus action back; the pool still refuses.
	s.startFight(2)
	out, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.SecondWind()})
	s.Require().NoError(err)
	s.False(out.Success)
	s.Contains(out.Error, "no second wind uses remaining")
	current, _, _ := s.char.ResourceStatus(resources.SecondWind)
	s.Equal(0, current)
}

func (s *SecondWindPoolSuite) TestRestsRestoreThePool() {
	for name, rest := range map[string]func() error{
		"short rest": func() error { _, err := s.char.ShortRest(s.ctx, &ShortRestInput{}); return err },
		"long rest":  func() error { _, err := s.char.LongRest(s.ctx); return err },
	} {
		s.Run(name, func() {
			s.Require().NoError(s.char.UseResource(resources.SecondWind, 1))
			s.Require().False(s.char.IsResourceAvailable(resources.SecondWind))

			s.Require().NoError(rest())

			current, maximum, ok := s.char.ResourceStatus(resources.SecondWind)
			s.Require().True(ok)
			s.Equal(1, current)
			s.Equal(1, maximum)
		})
	}
}

func (s *SecondWindPoolSuite) TestTheSharedViewSaysTheTruth() {
	feature := s.secondWindFeature()

	current, maximum := s.char.featureResourceInfo(feature)
	s.Equal(1, current)
	s.Equal(1, maximum)
	s.assertViewRows(1, 1)

	s.Require().NoError(s.char.UseResource(resources.SecondWind, 1))

	current, maximum = s.char.featureResourceInfo(feature)
	s.Equal(0, current)
	s.Equal(1, maximum)
	s.assertViewRows(0, 1)
}

// assertViewRows checks the status view's resource row and its feature row.
func (s *SecondWindPoolSuite) assertViewRows(current, maximum int) {
	out, err := s.char.StatusView(&StatusViewInput{})
	s.Require().NoError(err, "a fighter holding the second_wind pool still has a status view")

	var found bool
	for _, row := range out.View.Resources {
		if row.Key == resources.SecondWind {
			found = true
			s.Equal(current, row.Current)
			s.Equal(maximum, row.Maximum)
		}
	}
	s.True(found, "the resource row is present")

	status, err := s.secondWindFeature().Status(&features.StatusInput{Owner: s.char})
	s.Require().NoError(err)
	s.Require().NotNil(status.Status.Resource)
	s.Equal(current, status.Status.Resource.Current)
	s.Equal(maximum, status.Status.Resource.Maximum)
}

func (s *SecondWindPoolSuite) TestTheStatusViewOfAFighterDoesNotFail() {
	out, err := s.char.StatusView(&StatusViewInput{})
	s.Require().NoError(err)
	s.True(ownerResourceAllowed(classes.Fighter, resources.SecondWind))
	s.True(ownerResourceAllowed(classes.Fighter, resources.HitDice))
	s.False(ownerResourceAllowed(classes.Fighter, resources.RageCharges), "the fighter still holds no rage")
	s.NotEmpty(out.View.Resources)
}

func (s *SecondWindPoolSuite) TestThePoolSurvivesASaveAndALoad() {
	s.Require().NoError(s.char.UseResource(resources.SecondWind, 1))

	data, err := s.char.ToData()
	s.Require().NoError(err)
	loaded, err := Load(s.ctx, data)
	s.Require().NoError(err)

	current, maximum, ok := loaded.ResourceStatus(resources.SecondWind)
	s.Require().True(ok, "the loaded fighter carries the pool")
	s.Equal(0, current)
	s.Equal(1, maximum)
}

func (s *SecondWindPoolSuite) TestAnOldBlobWithUsesLoadsAndTheKeysAreIgnored() {
	blob := json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"second_wind"},` +
		`"id":"second_wind","name":"Second Wind","character_id":"old-fighter","uses":0,"max_uses":3}`)

	feature, err := features.LoadJSON(blob)
	s.Require().NoError(err)

	out, err := feature.(*features.SecondWind).ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(out), "uses", "the private counter is no longer saved")
	s.NotContains(string(out), "max_uses")
}

func (s *SecondWindPoolSuite) TestNeighboursAreUnchanged() {
	// A barbarian's Rage pool: sized from its level, long rest.
	pools := buildClassResources(&Character{id: "b"}, classes.Barbarian, 1, 1)
	rage, ok := pools[resources.RageCharges]
	s.Require().True(ok)
	s.Equal(2, rage.Maximum())
	s.Equal(coreResources.ResetLongRest, rage.ResetType)
	_, hasSecondWind := pools[resources.SecondWind]
	s.False(hasSecondWind, "a barbarian holds no second wind")

	// A monk's ki: level 2 and up, short rest.
	pools = buildClassResources(&Character{id: "m"}, classes.Monk, 2, 2)
	ki, ok := pools[resources.Ki]
	s.Require().True(ok)
	s.Equal(2, ki.Maximum())
	s.Equal(coreResources.ResetShortRest, ki.ResetType)
	_, hasSecondWind = pools[resources.SecondWind]
	s.False(hasSecondWind)
}

func TestSecondWindPoolSuite(t *testing.T) {
	suite.Run(t, new(SecondWindPoolSuite))
}
