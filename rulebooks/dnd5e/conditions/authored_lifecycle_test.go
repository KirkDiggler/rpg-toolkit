// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func totalFixtureDamage(out *dndevents.DamageChainEvent) int {
	n := 0
	for _, c := range out.Components {
		if c.Roll.Modifier != nil {
			n += *c.Roll.Modifier
		}
	}
	return n
}
func (s *authoredEffectsCookbookSuite) TestRingMaintainsAnotherIdleHolder() {
	w, defs := s.setup()
	source, grant := s.use(w, defs, "rage-ring", "A", "B", "ring")
	s.Require().NoError(w.apply(grant.Prepared))
	out, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(12, totalFixtureDamage(out))
	s.Equal(source.ID, out.Components[1].Roll.Source.SourceID)
	for range 12 {
		s.Require().NoError(w.turnEnd("B"))
	}
	s.NotEmpty(w.holder("B").GetConditions())
	taken, err := w.hit("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, taken.Amount)
	s.Empty(taken.FollowUps)
	s.Equal(96, w.holder("B").GetHitPoints())
	taken, err = w.hit("A", damage.Fire, 9)
	s.Require().NoError(err)
	s.Equal(91, w.holder("A").GetHitPoints())
	s.Require().Len(taken.FollowUps, 1)
	s.Equal("A", taken.FollowUps[0].SaverID)
	s.Equal(abilities.CON, taken.FollowUps[0].Ability)
	s.Equal(10, taken.FollowUps[0].DC)
	s.Require().NotNil(taken.FollowUps[0].Cause.EffectRef)
	s.Equal(source.Ref, *taken.FollowUps[0].Cause.EffectRef)
	// Target unconsciousness is not a configured ring maintenance rule.
	_, err = w.hit("B", damage.Fire, 100)
	s.Require().NoError(err)
	s.NotEmpty(w.holder("B").GetConditions())
	s.Require().NoError(w.rest("A"))
	s.Empty(w.holder("B").GetConditions())
	s.Zero(source.Budget.Current)
	for _, ending := range []string{"replacement", "user zero"} {
		s.Run(ending, func() {
			world, catalog := s.setup()
			_, first := s.use(world, catalog, "rage-ring", "A", "B", "first")
			s.Require().NoError(world.apply(first.Prepared))
			if ending == "replacement" {
				_, second := s.use(world, catalog, "damage-gift", "A", "C", "second")
				s.Require().NoError(world.apply(second.Prepared))
				s.NotEmpty(world.holder("C").GetConditions())
			} else {
				_, err := world.hit("A", damage.Fire, 100)
				s.Require().NoError(err)
			}
			s.Empty(world.holder("B").GetConditions())
		})
	}
}
func (s *authoredEffectsCookbookSuite) TestScrollUsesRecipientActivityAndCountsTenEnds() {
	w, defs := s.setup()
	_, grant := s.use(w, defs, "rage-scroll", "A", "B", "scroll")
	s.Require().NoError(w.apply(grant.Prepared))
	s.Empty(w.holder("A").GetConditions())
	s.Require().NoError(w.turnEnd("B"))
	info, err := w.maintenance("B", "scroll")
	s.Require().NoError(err)
	s.Nil(info.Concentration)
	s.Equal(9, info.Duration.Remaining)
	s.Require().NoError(w.reload())
	// Removing the user from the loaded universe does not remove recipient custody.
	s.Require().NoError(w.reloadWithout("A"))
	for range 8 {
		s.Require().NoError(w.miss("B"))
		s.Require().NoError(w.turnEnd("B"))
		s.NotEmpty(w.holder("B").GetConditions())
	}
	s.Require().NoError(w.miss("B"))
	s.Require().NoError(w.turnEnd("B"))
	s.Empty(w.holder("B").GetConditions())
	for _, ending := range []string{"idle", "rest", "combat", "zero"} {
		s.Run(ending, func() {
			world, catalog := s.setup()
			_, g := s.use(world, catalog, "rage-scroll", "A", "B", "scroll")
			s.Require().NoError(world.apply(g.Prepared))
			s.Require().NoError(world.turnEnd("B"))
			switch ending {
			case "idle":
				s.Require().NoError(world.miss("A"))
				s.Require().NoError(world.turnEnd("A"))
				s.Require().NoError(world.turnEnd("B"))
			case "rest":
				s.Require().NoError(world.rest("B"))
			case "combat":
				s.Require().NoError(dndevents.CombatEndTopic.On(world.bus).Publish(context.Background(), dndevents.CombatEndEvent{SubjectID: "B"}))
			case "zero":
				_, err := world.hit("B", damage.Fire, 100)
				s.Require().NoError(err)
			}
			s.Empty(world.holder("B").GetConditions())
		})
	}
}
func (s *authoredEffectsCookbookSuite) TestFollowUpConsequenceEndsOnlyItsApplication() {
	w, defs := s.setup()
	_, ring := s.use(w, defs, "rage-ring", "A", "B", "ring")
	s.Require().NoError(w.apply(ring.Prepared))
	_, ward := s.use(w, defs, "physical-ward", "C", "B", "ward")
	s.Require().NoError(w.apply(ward.Prepared))
	fact, err := w.hit("A", damage.Fire, 6)
	s.Require().NoError(err)
	s.Require().Len(fact.FollowUps, 1)
	s.Equal(94, w.holder("A").GetHitPoints())
	consequence := fact.FollowUps[0].OnFailure
	// The test host selects failure; no native save/window resolver is claimed.
	s.Require().NoError(w.remove(consequence.Owner, consequence.Owner.SourceID, consequence.Reason))
	s.Require().Len(w.holder("B").GetConditions(), 1)
	result, err := w.incoming("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, result.Types[0].Taken)
	out, err := w.damageBonus("B", "A", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(10, totalFixtureDamage(out))
}
func (s *authoredEffectsCookbookSuite) TestExhaustedUnavailableSourceAndReloadDoNotRefillOrEndGrant() {
	w, defs := s.setup()
	source, grant := s.use(w, defs, "rage-scroll", "A", "B", "scroll")
	s.Require().NoError(w.apply(grant.Prepared))
	s.Require().NoError(w.turnEnd("B"))
	s.Require().NoError(w.miss("B"))
	source.Available = false
	saved := source.data()
	s.Require().NoError(w.reload())
	restored, err := restoreFixtureSource(saved)
	s.Require().NoError(err)
	s.False(restored.Available)
	s.Zero(restored.Budget.Current)
	raw, err := os.ReadFile("testdata/effects/library.json")
	s.Require().NoError(err)
	changed := strings.ReplaceAll(string(raw), `"fixed": 2`, `"fixed": 99`)
	_, err = conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(changed)})
	s.Require().NoError(err)
	s.Require().NoError(w.turnEnd("B"))
	s.NotEmpty(w.holder("B").GetConditions(), "saved attack flag sustains this end")
	info, err := w.maintenance("B", "scroll")
	s.Require().NoError(err)
	s.Equal(8, info.Duration.Remaining)
	out, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(12, totalFixtureDamage(out))
	s.Equal(source.ID, out.Components[1].Roll.Source.SourceID)
	s.Require().NoError(w.turnEnd("B"))
	s.Empty(w.holder("B").GetConditions())
	s.Zero(restored.Budget.Current)
}
func (s *authoredEffectsCookbookSuite) TestDeliveryFailureIsNotARefundablePureFailure() {
	w, defs := s.setup()
	source, grant := s.use(w, defs, "rage-ring", "A", "B", "paid")
	s.Require().NoError(w.owner(grant.Prepared))
	s.Zero(source.Budget.Current)
	s.Require().NoError(w.binding.CheckComplete())
	// A real topic subscriber refuses after a recipient has been applied. The
	// binding must expose the orphan rather than declaring a successful boundary.
	refused := errors.New("fixture delivery refused")
	_, err := dndevents.ConditionAppliedTopic.On(w.bus).Subscribe(context.Background(), func(_ context.Context, e dndevents.ConditionAppliedEvent) error {
		if e.Target.GetID() == "B" {
			return refused
		}
		return nil
	})
	s.Require().NoError(err)
	_, err = w.deliver(grant.Prepared, true)
	s.ErrorIs(err, refused)
	s.Zero(source.Budget.Current)
	s.Error(w.binding.CheckComplete())
}
func (s *authoredEffectsCookbookSuite) TestEndingObservationAndIndependentSources() {
	w, defs := s.setup()
	observed := 0
	w.beforeAttach = func(bus events.EventBus) error {
		_, err := dndevents.ConditionRemovedTopic.On(bus).Subscribe(context.Background(), func(_ context.Context, e dndevents.ConditionRemovedEvent) error {
			if e.MemberID != "B" || e.ApplicationID != "ring" {
				return nil
			}
			out, err := w.damageBonus("B", "C", abilities.STR, true, true)
			if err != nil {
				return err
			}
			s.Equal(12, totalFixtureDamage(out))
			observed++
			return nil
		})
		return err
	}
	s.Require().NoError(w.reload())
	_, ring := s.use(w, defs, "rage-ring", "A", "B", "ring")
	s.Require().NoError(w.apply(ring.Prepared))
	_, ward := s.use(w, defs, "physical-ward", "C", "B", "ward")
	s.Require().NoError(w.apply(ward.Prepared))
	s.Require().NoError(w.rest("A"))
	s.Equal(1, observed)
	out, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(10, totalFixtureDamage(out))
	result, err := w.incoming("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, result.Types[0].Taken)
	s.Require().Len(w.holder("B").GetConditions(), 1)
}
