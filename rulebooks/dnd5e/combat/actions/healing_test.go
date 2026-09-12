// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func (s *CastProfileSuite) TestHealingDeclarationRejectsUnsupportedDeliveryBeforeExecution() {
	base := actions.CastProfile{RangeFeet: 5, Target: actions.CastTargetTouch, MinTargets: 1, MaxTargets: 1, Healing: &healing.Declaration{Dice: "1d8"}}
	s.Require().NoError(base.Validate())
	for _, tc := range []struct {
		name   string
		mutate func(*actions.CastProfile)
	}{
		{"ranged touch", func(p *actions.CastProfile) { p.RangeFeet = 30 }},
		{"two recipients", func(p *actions.CastProfile) { p.MaxTargets = 2 }},
		{"signed dice", func(p *actions.CastProfile) { p.Healing.Dice = "-1d8" }},
		{"unsourced modifier", func(p *actions.CastProfile) { p.Healing.Modifiers = []healing.Modifier{{Amount: 3}} }},
		{"gated healing", func(p *actions.CastProfile) { p.Save = &saves.SaveGate{} }},
		{"exclusions without healing", func(p *actions.CastProfile) { p.Healing = nil; p.HealingExcludes = []string{"undead"} }},
	} {
		s.Run(tc.name, func() { p := base.Clone(); tc.mutate(&p); s.Error(p.Validate()) })
	}
}
