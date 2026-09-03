package monster

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type MonsterTestSuite struct {
	suite.Suite
}

// HealingAppliedTestSuite isolates the applied-fact contract so focused test
// commands select these tests directly.
type HealingAppliedTestSuite struct {
	suite.Suite
}

func TestMonsterSuite(t *testing.T) {
	suite.Run(t, new(MonsterTestSuite))
}

func (s *MonsterTestSuite) TestNew() {
	config := Config{
		ID:   "test-monster-1",
		Name: "Test Monster",
		HP:   50,
		AC:   16,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 12,
			abilities.CON: 14,
			abilities.INT: 8,
			abilities.WIS: 10,
			abilities.CHA: 6,
		},
	}

	monster := New(config)

	s.Require().NotNil(monster)
	s.Equal("test-monster-1", monster.GetID())
	s.Equal(dnd5e.EntityTypeMonster, monster.GetType())
	s.Equal("Test Monster", monster.Name())
	s.Equal(50, monster.HP())
	s.Equal(50, monster.MaxHP())
	s.Equal(16, monster.AC())
	s.True(monster.IsAlive())
}

func (s *MonsterTestSuite) TestGetSavingThrowModifier() {
	monster := New(Config{
		ID:   "test-monster-1",
		Name: "Test Monster",
		HP:   10,
		AC:   10,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.CON: 10,
			abilities.INT: 3,
			abilities.WIS: 8,
		},
	})

	tests := map[string]struct {
		ability abilities.Ability
		want    int
	}{
		"positive modifier":  {ability: abilities.STR, want: 3},
		"zero modifier":      {ability: abilities.CON, want: 0},
		"negative modifier":  {ability: abilities.WIS, want: -1},
		"odd score below 10": {ability: abilities.INT, want: -4},
	}

	for name, test := range tests {
		s.Run(name, func() {
			s.Equal(test.want, monster.GetSavingThrowModifier(test.ability))
		})
	}
}

func (s *MonsterTestSuite) TestTakeDamage() {
	monster := New(Config{
		ID:   "test-1",
		Name: "Test",
		HP:   20,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
		},
	})

	s.Run("normal damage", func() {
		damage := monster.TakeDamage(5)
		s.Equal(5, damage, "should return actual damage taken")
		s.Equal(15, monster.HP(), "HP should decrease")
		s.True(monster.IsAlive())
	})

	s.Run("overkill damage", func() {
		damage := monster.TakeDamage(100)
		s.Equal(15, damage, "should only deal remaining HP as damage")
		s.Equal(0, monster.HP(), "HP should be 0")
		s.False(monster.IsAlive(), "monster should be dead")
	})

	s.Run("negative damage", func() {
		monster := New(Config{
			ID:   "test-2",
			Name: "Test",
			HP:   20,
			AC:   15,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 10,
			},
		})

		damage := monster.TakeDamage(-5)
		s.Equal(0, damage, "negative damage should be treated as 0")
		s.Equal(20, monster.HP(), "HP should not change")
	})
}

func (s *MonsterTestSuite) TestIsAlive() {
	monster := New(Config{
		ID:   "test-1",
		Name: "Test",
		HP:   10,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
		},
	})

	s.True(monster.IsAlive(), "monster should be alive at full HP")

	monster.TakeDamage(5)
	s.True(monster.IsAlive(), "monster should be alive with partial HP")

	monster.TakeDamage(5)
	s.False(monster.IsAlive(), "monster should be dead at 0 HP")
}

// validSecondWindCalculation builds a valid sourced healing calculation:
// one 1d10 trace plus the Fighter-level modifier, totalling 7.
func validSecondWindCalculation() *dnd5eEvents.RollCalculation {
	modifier := 1
	return &dnd5eEvents.RollCalculation{
		Components: []dnd5eEvents.RollComponent{
			{
				Source: dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      "1d10",
					DieSize:       10,
					OriginalRolls: []int{6},
					FinalRolls:    []int{6},
					Subtotal:      6,
				},
			},
			{
				Source: dnd5eEvents.RollSource{
					Ref: refs.Classes.Fighter(), Name: "Fighter", Label: "Fighter level",
				},
				Modifier: &modifier,
			},
		},
		Total: 7,
	}
}

func (s *HealingAppliedTestSuite) TestHealingAppliedReportsPostClampFacts() {
	ctx := context.Background()
	bus := events.NewEventBus()
	monster := New(Config{ID: "healed-monster", Name: "Healed", HP: 10, AC: 10})
	monster.TakeDamage(2)
	s.Require().NoError(monster.SheetKeeper().Apply(ctx, bus))

	source := *refs.Features.SecondWind()
	calculation := validSecondWindCalculation()
	var got *dnd5eEvents.HealingAppliedEvent
	_, err := dnd5eEvents.HealingAppliedTopic.On(bus).Subscribe(
		ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			got = &event
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID:    monster.GetID(),
		Amount:      7,
		SourceRef:   &source,
		SourceName:  "Second Wind",
		Calculation: calculation,
	})

	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(monster.GetID(), got.TargetID)
	s.Require().Equal(7, got.Requested)
	s.Require().Equal(2, got.Applied)
	s.Require().Equal(8, got.HPBefore)
	s.Require().Equal(10, got.HPAfter)
	s.Require().True(got.SourceRef.Equals(refs.Features.SecondWind()))
	s.Require().Equal("Second Wind", got.SourceName)
	s.Require().NotSame(&source, got.SourceRef, "the applied fact owns its source identity")

	// The applied fact carries a deep clone of the received calculation.
	s.Require().NotNil(got.Calculation, "the applied fact carries the roll calculation")
	s.Require().Equal(calculation, got.Calculation)
	s.Require().NotSame(calculation, got.Calculation, "the applied fact owns its calculation")

	source.ID = "mutated_by_caller"
	calculation.Components[0].Dice.FinalRolls[0] = 9
	calculation.Components[0].Dice.Subtotal = 9
	*calculation.Components[1].Modifier = 5
	calculation.Total = 14
	s.Require().True(got.SourceRef.Equals(refs.Features.SecondWind()),
		"mutating the request after publication cannot rewrite the applied fact")
	s.Require().Equal(7, got.Calculation.Total,
		"mutating the received calculation cannot rewrite the applied total")
	s.Require().Equal([]int{6}, got.Calculation.Components[0].Dice.FinalRolls,
		"mutating the received calculation cannot rewrite the applied faces")
	s.Require().Equal(1, *got.Calculation.Components[1].Modifier,
		"mutating the received calculation cannot rewrite the applied modifier")
	s.Require().True(monster.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedAtMaximumReportsZeroApplied() {
	ctx := context.Background()
	bus := events.NewEventBus()
	monster := New(Config{ID: "full-monster", Name: "Full", HP: 10, AC: 10})
	s.Require().NoError(monster.SheetKeeper().Apply(ctx, bus))

	var got *dnd5eEvents.HealingAppliedEvent
	_, err := dnd5eEvents.HealingAppliedTopic.On(bus).Subscribe(
		ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			got = &event
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: monster.GetID(),
		Amount:   7,
	})

	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(7, got.Requested)
	s.Require().Zero(got.Applied)
	s.Require().Equal(10, got.HPBefore)
	s.Require().Equal(10, got.HPAfter)
	s.Require().Nil(got.Calculation, "non-roll healing carries no calculation")
	s.Require().True(monster.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedForSomeoneElseIsIgnored() {
	ctx := context.Background()
	bus := events.NewEventBus()
	monster := New(Config{ID: "bystander", Name: "Bystander", HP: 10, AC: 10})
	s.Require().NoError(monster.SheetKeeper().Apply(ctx, bus))

	published := 0
	_, err := dnd5eEvents.HealingAppliedTopic.On(bus).Subscribe(
		ctx, func(_ context.Context, _ dnd5eEvents.HealingAppliedEvent) error {
			published++
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: "someone-else",
		Amount:   7,
	})

	s.Require().NoError(err)
	s.Require().Zero(published)
	s.Require().Equal(10, monster.HP())
	s.Require().False(monster.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedSubscriberErrorPropagatesAfterMutation() {
	ctx := context.Background()
	bus := events.NewEventBus()
	monster := New(Config{ID: "healed-monster", Name: "Healed", HP: 10, AC: 10})
	monster.TakeDamage(2)
	s.Require().NoError(monster.SheetKeeper().Apply(ctx, bus))

	sentinel := errors.New("healing applied subscriber failed")
	observedHP := 0
	observedDirty := false
	_, err := dnd5eEvents.HealingAppliedTopic.On(bus).Subscribe(
		ctx, func(_ context.Context, _ dnd5eEvents.HealingAppliedEvent) error {
			observedHP = monster.HP()
			observedDirty = monster.IsDirty()
			return sentinel
		})
	s.Require().NoError(err)

	// The handler must publish on the bus captured by SheetKeeper.Apply, not the
	// parked bus retained only for Cleanup.
	monster.bus = events.NewEventBus()

	err = dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: monster.GetID(),
		Amount:   7,
	})

	s.Require().ErrorIs(err, sentinel)
	s.Require().Equal(10, observedHP, "the callback observes post-clamp HP")
	s.Require().True(observedDirty, "the callback observes the dirty mark")
	s.Require().Equal(10, monster.HP(), "the mutation precedes applied-fact publication")
	s.Require().True(monster.IsDirty())
}

func TestHealingAppliedSuite(t *testing.T) {
	suite.Run(t, new(HealingAppliedTestSuite))
}
