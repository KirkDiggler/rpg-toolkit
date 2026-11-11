// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/stretchr/testify/suite"
	"testing"
)

type MaintenanceDescriptionSuite struct{ suite.Suite }

func TestMaintenanceDescriptionSuite(t *testing.T) { suite.Run(t, new(MaintenanceDescriptionSuite)) }
func (s *MaintenanceDescriptionSuite) TestOneConcentrationClaimIsReportedAndMultipleClaimsAreRefused() {
	hold := effects.PartDeclaration{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication}
	out, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: []effects.PartDeclaration{hold}})
	s.Require().NoError(err)
	s.True(out.Concentration)
	s.Nil(out.Duration)
	hold.Key = "other"
	_, err = effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: []effects.PartDeclaration{{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication}, hold}})
	s.Require().ErrorContains(err, "multiple concentration")
}
func (s *MaintenanceDescriptionSuite) TestDurationIsIndependentAndKeepsEveryClock() {
	clock := effects.PartDeclaration{Key: "clock", Kind: effects.KindDuration, Params: json.RawMessage(`{"turn_ends":10,"skip_first":true}`), OnTerminal: generic.EndApplication}
	out, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: []effects.PartDeclaration{clock}})
	s.Require().NoError(err)
	s.False(out.Concentration)
	s.Require().NotNil(out.Duration)
	s.Equal(10, out.Duration.TurnEnds)
	s.True(out.Duration.SkipFirstTurnEnd)
	second := clock
	second.Key = "second"
	second.Params = json.RawMessage(`{"turn_ends":2}`)
	out, err = effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: []effects.PartDeclaration{clock, second}})
	s.Require().NoError(err)
	s.Nil(out.Duration, "two independent clocks are not a fabricated countdown")
	s.Require().Len(out.Durations, 2)
	s.Equal("second", out.Durations[1].PartKey)
	s.Equal(2, out.Durations[1].Duration.TurnEnds)
}
func (s *MaintenanceDescriptionSuite) TestInvalidDeclarationsFailBeforeBinding() {
	_, err := effects.DescribeMaintenance(nil)
	s.Require().Error(err)
	for _, parts := range [][]effects.PartDeclaration{
		{{Key: "clock", Kind: effects.KindDuration, Params: json.RawMessage(`{"turn_ends":0}`), OnTerminal: generic.EndApplication}},
		{{Key: "unknown", Kind: "unknown", OnTerminal: generic.EndPart}},
		{{Key: "same", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication}, {Key: "same", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication}},
	} {
		_, err = effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{Parts: parts})
		s.Require().Error(err)
	}
	out, err := effects.DescribeMaintenance(&effects.DescribeMaintenanceInput{})
	s.Require().NoError(err)
	s.False(out.Concentration)
	s.Empty(out.Durations)
}
