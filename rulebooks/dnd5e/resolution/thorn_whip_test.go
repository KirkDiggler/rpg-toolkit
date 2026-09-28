// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastActionTestSuite) TestThornWhipPullChoicesOnlyApplyOnHit() {
	for option, cells := range map[string]int{"no-pull": 0, "pull-5": 1, "pull-10": 2} {
		for _, roll := range []int{1, 18, 20} {
			s.Run(option+string(rune('A'+roll)), func() {
				f := s.fixtures()
				d := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Thornwhip, SpellAttackBonus: 5})
				machine, err := NewAction(&ActionInput{Definition: *d, AttackerID: bardID, TargetIDs: []string{heroID}, Option: option, Roller: facedRoller{d20: roll, other: 2}})
				s.Require().NoError(err)
				cost := baneCost()
				cost.Profile = d.Cost
				out, err := f.resolve(f.saver(14), machine, cost, baneCaster(1, 2))
				s.Require().NoError(err)
				target := s.castOutcome(out).Targets[0]
				s.Require().NotNil(target.Attack)
				s.Equal(roll != 1, target.Attack.Hit)
				expectedDamage := 2
				if roll == 1 {
					expectedDamage = 0
				}
				if roll == 20 {
					expectedDamage = 4
				}
				s.Equal(expectedDamage, target.Attack.Damage)
				if roll != 1 && cells > 0 {
					s.Require().Len(target.Applied, 1)
					s.Equal(ImposedMove, target.Applied[0].Kind)
					s.Equal(cells, target.Applied[0].Move.Cells)
					s.Equal(bardID, target.Applied[0].Move.AnchorID)
					s.False(target.Applied[0].Move.Provokes)
				} else {
					s.Empty(target.Applied)
				}
				payer := f.sheet(out, bardID)
				s.Zero(payer.ActionEconomy.ActionsRemaining)
				s.Equal(2, payer.Resources[resources.SpellSlotLevel1].Current)
			})
		}
	}
}
