// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

func (s *authoredEffectsCookbookSuite) TestWardAndDamageGiftSelectOnlyTheirPieces() {
	for _, tc := range []struct {
		id                               string
		bonus, physical, fire, advantage int
	}{{"physical-ward", 10, 4, 9, 0}, {"damage-gift", 12, 9, 9, 0}, {"fire-ward", 10, 9, 4, 0}, {"rage-ring", 12, 4, 9, 1}} {
		s.Run(tc.id, func() {
			w, defs := s.setup()
			_, grant := s.use(w, defs, tc.id, "A", "B", "use")
			s.Require().NoError(w.apply(grant.Prepared))
			out, err := w.damageBonus("B", "C", abilities.STR, true, true)
			s.Require().NoError(err)
			s.Equal(tc.bonus, totalFixtureDamage(out))
			physical, err := w.incoming("B", damage.Slashing, 9)
			s.Require().NoError(err)
			s.Equal(tc.physical, physical.Types[0].Taken)
			fire, err := w.incoming("B", damage.Fire, 9)
			s.Require().NoError(err)
			s.Equal(tc.fire, fire.Types[0].Taken)
			saves, err := w.saveAdvantage("B", abilities.STR)
			s.Require().NoError(err)
			s.Equal(tc.advantage, saves)
			checks, err := w.checkAdvantage("B", skills.Athletics)
			s.Require().NoError(err)
			s.Equal(tc.advantage, checks)
			saves, err = w.saveAdvantage("A", abilities.STR)
			s.Require().NoError(err)
			s.Zero(saves)
			saves, err = w.saveAdvantage("B", abilities.DEX)
			s.Require().NoError(err)
			s.Zero(saves)
			checks, err = w.checkAdvantage("B", skills.Acrobatics)
			s.Require().NoError(err)
			s.Zero(checks)
		})
	}
}
func (s *authoredEffectsCookbookSuite) TestSausageInformationMatchesBothAttackBearings() {
	w, defs := s.setup()
	_, grant := s.use(w, defs, "kitchen-sausage", "B", "B", "bite")
	s.Require().NoError(w.apply(grant.Prepared))
	for _, variant := range []string{"eligible", "dex", "ranged", "nonweapon", "opportunity"} {
		s.Run(variant, func() {
			f, err := w.frame("B", "C")
			s.Require().NoError(err)
			want := 0
			switch variant {
			case "eligible":
				want = 1
			case "dex":
				f.Action.Ability = contributions.Known(abilities.DEX)
			case "ranged":
				f.Action.Melee = contributions.Known(false)
			case "nonweapon":
				f.Action.WeaponPool = contributions.Known(false)
			case "opportunity":
				f.Action.Opportunity = contributions.Known(true)
			}
			out, err := w.attack(f)
			s.Require().NoError(err)
			s.Len(out.AdvantageSources, want)
			info, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: w.holder("B").GetConditions(), Frame: f})
			s.Require().NoError(err)
			applied := 0
			for _, row := range info.Effects {
				if row.State == contributions.StateApplies {
					applied++
				}
			}
			s.Equal(want, applied)
		})
	}
	f, err := w.frame("C", "B")
	s.Require().NoError(err)
	f.Action.Melee = contributions.Known(false)
	out, err := w.attack(f)
	s.Require().NoError(err)
	s.Len(out.AdvantageSources, 1)
	info, err := conditions.AssessTargetHeldEffects(&conditions.AssessTargetHeldEffectsInput{Frame: f})
	s.Require().NoError(err)
	s.Require().Len(info.Effects, 1)
	s.Equal(contributions.StateApplies, info.Effects[0].State)
	unknown := f.Clone()
	unknown.HeldEffects = nil
	info, err = conditions.AssessTargetHeldEffects(&conditions.AssessTargetHeldEffectsInput{Frame: unknown})
	s.Require().NoError(err)
	s.NotEmpty(info.Effects)
	for _, row := range info.Effects {
		s.Equal(contributions.StateDepends, row.State)
		s.Empty(row.Benefit)
	}
	_, err = w.attack(unknown)
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	empty := f.Clone()
	for i := range empty.HeldEffects {
		empty.HeldEffects[i].Outputs = []contributions.ConfiguredOutput{}
	}
	info, err = conditions.AssessTargetHeldEffects(&conditions.AssessTargetHeldEffectsInput{Frame: empty})
	s.Require().NoError(err)
	s.Empty(info.Effects)
	s.Require().NoError(w.turnStart("C"))
	s.NotEmpty(w.holder("B").GetConditions())
	s.Require().NoError(w.turnStart("B"))
	s.Empty(w.holder("B").GetConditions())
	f, err = w.frame("C", "B")
	s.Require().NoError(err)
	out, err = w.attack(f)
	s.Require().NoError(err)
	s.Empty(out.AdvantageSources)
}
