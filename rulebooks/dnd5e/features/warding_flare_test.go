package features

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/suite"
	"testing"
)

type flareCast struct {
	wrathTestCast
	remaining     int
	incapacitated bool
	distance      int
	known         bool
}

func (c *flareCast) ResourceStatus(_ string, key coreResources.ResourceKey) (int, int, bool) {
	return c.remaining, 3, key == resources.WardingFlare
}
func (c *flareCast) HasCondition(string, *core.Ref) (bool, bool) { return c.incapacitated, true }
func (c *flareCast) SeesWithin(_, _ string, feet int) (bool, bool) {
	return c.visible && c.distance <= feet, c.known
}

type WardingFlareSuite struct{ suite.Suite }

func TestWardingFlareSuite(t *testing.T) { suite.Run(t, new(WardingFlareSuite)) }
func (s *WardingFlareSuite) TestEligibilityAndLifecycle() {
	for _, tc := range []struct {
		name                          string
		visible, known, incapacitated bool
		uses, distance                int
		target                        string
		want                          bool
	}{
		{"at thirty feet", true, true, false, 1, 30, "owner", true},
		{"out of range", true, true, false, 1, 31, "owner", false},
		{"fog", false, true, false, 1, 5, "owner", false},
		{"unknown sight", true, false, false, 1, 5, "owner", false},
		{"exhausted", true, true, false, 0, 5, "owner", false},
		{"incapacitated", true, true, true, 1, 5, "owner", false},
		{"another target", true, true, false, 1, 5, "ally", false},
	} {
		s.Run(tc.name, func() {
			out, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Features.WardingFlare().String(), CharacterID: "owner"})
			s.Require().NoError(err)
			encoded, err := out.Feature.ToJSON()
			s.Require().NoError(err)
			loaded, err := LoadJSON(encoded)
			s.Require().NoError(err)
			feature := loaded.(*WardingFlare)
			bus := events.NewEventBus()
			s.Require().NoError(feature.Apply(context.Background(), bus))
			s.Require().Error(feature.Apply(context.Background(), bus))
			ctx := gamectx.WithCast(context.Background(), &flareCast{wrathTestCast: wrathTestCast{visible: tc.visible}, remaining: tc.uses, distance: tc.distance, known: tc.known, incapacitated: tc.incapacitated})
			e := dndEvents.AttackChainEvent{AttackerID: "attacker", TargetID: tc.target}
			c, err := dndEvents.AttackChain.On(bus).PublishWithChain(ctx, e, events.NewStagedChain[dndEvents.AttackChainEvent](combat.ModifierStages))
			s.Require().NoError(err)
			folded, err := c.Execute(ctx, e)
			s.Require().NoError(err)
			s.Empty(folded.DisadvantageSources, "offering does not apply or spend the reaction")
			if tc.want {
				s.Require().Len(folded.BeforeRollOffers, 1)
				s.Equal(string(resources.WardingFlare), folded.BeforeRollOffers[0].ResourceKey)
				s.Equal(refs.Features.WardingFlare(), folded.BeforeRollOffers[0].Disadvantage.SourceRef)
			} else {
				s.Empty(folded.BeforeRollOffers)
			}
			s.Require().NoError(feature.Remove(ctx, bus))
			c, err = dndEvents.AttackChain.On(bus).PublishWithChain(ctx, e, events.NewStagedChain[dndEvents.AttackChainEvent](combat.ModifierStages))
			s.Require().NoError(err)
			folded, err = c.Execute(ctx, e)
			s.Require().NoError(err)
			s.Empty(folded.BeforeRollOffers)
			s.Error(feature.Activate(ctx, nil, FeatureInput{}))
		})
	}
}
