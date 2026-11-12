// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/stretchr/testify/suite"
	"testing"
)

func (s *CastContentSuite) maintenance(p *actions.CastProfile) *effects.MaintenanceDescription {
	out, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: p.Maintenance})
	s.Require().NoError(err)
	return out
}
func (s *CastContentSuite) assertMaintenance(p *actions.CastProfile, turns int, grace bool) {
	out := s.maintenance(p)
	s.True(out.Concentration)
	s.Require().NotNil(out.Duration)
	s.Equal(turns, out.Duration.TurnEnds)
	s.Equal(grace, out.Duration.SkipFirstTurnEnd)
}

type MaintenanceDeclarationsSuite struct{ suite.Suite }

func TestMaintenanceDeclarationsSuite(t *testing.T) { suite.Run(t, new(MaintenanceDeclarationsSuite)) }
func (s *MaintenanceDeclarationsSuite) TestEveryCurrentConcentrationProfileUsesParts() {
	for _, tc := range []struct {
		id    spells.Spell
		turns int
		grace bool
	}{
		{spells.Bless, 10, true}, {spells.Bane, 10, true}, {spells.FaerieFire, 10, true}, {spells.FogCloud, 600, true},
		{spells.DivineFavor, 10, true}, {spells.ShieldOfFaith, 100, true}, {spells.Sanctuary, 10, true},
		{spells.Guidance, 10, true}, {spells.Resistance, 10, true}, {spells.TrueStrike, 2, false},
	} {
		s.Run(string(tc.id), func() {
			def := spells.CastDefinition(spells.CastDefinitionInput{Spell: tc.id, SpellSaveDC: 13})
			s.Require().NotNil(def)
			s.Require().NoError(def.Validate())
			info, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: def.Cast.Maintenance})
			s.Require().NoError(err)
			s.True(info.Concentration)
			s.Require().NotNil(info.Duration)
			s.Equal(tc.turns, info.Duration.TurnEnds)
			s.Equal(tc.grace, info.Duration.SkipFirstTurnEnd)
			// Runtime policy remains authored alongside the hold, not guessed by resolution.
			found := map[string]bool{}
			for _, part := range def.Cast.Maintenance {
				if part.Kind == effects.KindEndOn {
					var p struct {
						Beat string `json:"beat"`
					}
					s.Require().NoError(json.Unmarshal(part.Params, &p))
					found[p.Beat] = true
				}
				if part.Kind == effects.KindUntilEmpty {
					found["empty"] = true
				}
			}
			s.True(found["rest"])
			s.True(found["combat_end"])
			s.True(found["empty"])
		})
	}
}
