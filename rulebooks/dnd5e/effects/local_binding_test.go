// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type localPartBindingSuite struct{ suite.Suite }

func TestLocalPartBindingSuite(t *testing.T) { suite.Run(t, new(localPartBindingSuite)) }
func (s *localPartBindingSuite) TestLocalPartsNeedNoInventedApplication() {
	parts, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{
		{Key: "ward", Effect: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}, OnTerminal: generic.EndPart},
		{Key: "activity", Effect: dndfx.CombatActivity{SkipFirst: true}, OnTerminal: generic.EndAttachment},
		{Key: "rounds", Effect: dndfx.RoundDuration{Rounds: 10, ExpiredReason: "expired", ResetReason: "reset"}, OnTerminal: generic.EndAttachment},
	}})
	s.Require().NoError(err)
	for _, decl := range parts.Parts {
		in := partInput(decl.Key, decl.Kind, "")
		in.Declaration = decl
		in.ApplicationID = ""
		in.Owner = dndevents.ConditionAddress{}
		out, err := dndfx.BindPart(in)
		s.NoError(err)
		s.NotNil(out)
	}
}
func (s *localPartBindingSuite) TestApplicationMaintenanceStillRequiresIdentity() {
	in := partInput("hold", dndfx.KindConcentration, `{}`)
	in.ApplicationID = ""
	in.Owner = dndevents.ConditionAddress{}
	out, err := dndfx.BindPart(in)
	s.Error(err)
	s.Nil(out)
}
