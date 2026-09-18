// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package resolution

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastActionTestSuite) TestGuidingBoltPaysAndDeliversOnlyOnHit() {
	for _, tc := range []struct {
		name          string
		roll, damage  int
		hit, critical bool
	}{
		{"miss", 1, 0, false, false}, {"hit", 18, 8, true, false}, {"critical", 20, 16, true, true},
	} {
		s.Run(tc.name, func() {
			f := s.fixtures()
			d := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.GuidingBolt, SpellAttackBonus: 5})
			machine, err := NewAction(&ActionInput{Definition: *d, AttackerID: bardID, TargetIDs: []string{heroID}, Roller: facedRoller{d20: tc.roll, other: 2}})
			s.Require().NoError(err)
			cost := baneCost()
			cost.Profile = d.Cost
			out, err := f.resolve(f.saver(14), machine, cost, baneCaster(1, 2))
			s.Require().NoError(err)
			outcome := s.castOutcome(out)
			s.Require().Len(outcome.Targets, 1)
			attack := outcome.Targets[0].Attack
			s.Require().NotNil(attack)
			s.Equal(tc.hit, attack.Hit)
			s.Equal(tc.critical, attack.Critical)
			s.Equal(tc.damage, attack.Damage)
			s.Equal(tc.roll+5, attack.Total)
			s.Nil(outcome.Targets[0].Save)
			payer := f.sheet(out, bardID)
			s.Zero(payer.ActionEconomy.ActionsRemaining)
			s.Equal(1, payer.Resources[resources.SpellSlotLevel1].Current)
			if tc.hit {
				recipient := f.sheet(out, heroID)
				found := false
				for _, raw := range recipient.Conditions {
					var light struct {
						Ref          core.Ref
						SourceID     string `json:"source_id"`
						TurnEndsLeft int    `json:"turn_ends_left"`
					}
					s.Require().NoError(json.Unmarshal(raw, &light))
					if light.Ref.String() == refs.Conditions.GuidingBolt().String() {
						found = true
						s.Equal(bardID, light.SourceID)
						s.Equal(2, light.TurnEndsLeft)
					}
				}
				s.True(found, "hit persists the target-held light")
			} else {
				s.Empty(attack.Conditions)
			}
		})
	}
}
