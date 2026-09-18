package monster_test

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
	"testing"
)

type EffectiveACSuite struct{ suite.Suite }

func TestEffectiveACSuite(t *testing.T) { suite.Run(t, new(EffectiveACSuite)) }
func (s *EffectiveACSuite) TestAuthoredACWithTemporaryProtection() {
	ctx := context.Background()
	data := &monster.Data{ID: "skeleton", Name: "Skeleton", HitPoints: 13, MaxHitPoints: 13, ArmorClass: 13}
	unloaded, err := monster.Load(ctx, data)
	s.Require().NoError(err)
	_, err = unloaded.EffectiveAC(ctx)
	s.Error(err, "never silently ignore persisted but unattached effects")
	bus := events.NewEventBus()
	m, err := monster.LoadFromData(ctx, data, bus)
	s.Require().NoError(err)
	c, err := conditions.NewShieldOfFaithCondition(conditions.NewShieldOfFaithConditionInput{
		MemberID: "skeleton", SourceID: "cleric", SourceRef: refs.Spells.ShieldOfFaith(),
	})
	s.Require().NoError(err)
	s.Require().NoError(c.Apply(ctx, bus))
	ac, err := combat.GetEffectiveAC(ctx, m)
	s.Require().NoError(err)
	s.Equal(15, ac)
	s.Equal(13, m.ToData().ArmorClass, "temporary bonus is not baked into the stat block")
	s.Require().NoError(c.Remove(ctx, bus))
	ac, err = combat.GetEffectiveAC(ctx, m)
	s.Require().NoError(err)
	s.Equal(13, ac)
}
