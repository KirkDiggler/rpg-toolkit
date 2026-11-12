// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"encoding/json"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

func maintenanceFixture() []effects.PartDeclaration {
	return []effects.PartDeclaration{
		{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication},
		{Key: "clock", Kind: effects.KindDuration, Params: json.RawMessage(`{"turn_ends":10,"skip_first":true}`), OnTerminal: generic.EndApplication},
	}
}

func (s *CastProfileSuite) TestMaintenanceClaimsAreValidatedBeforeExecution() {
	p := gatelessProfile()
	hold := effects.PartDeclaration{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication}
	p.Maintenance = []effects.PartDeclaration{hold}
	s.Require().NoError(p.Validate(), "concentration itself does not require a duration")
	other := hold
	other.Key = "other"
	p.Maintenance = append(p.Maintenance, other)
	s.ErrorContains(p.Validate(), "multiple concentration")
	p.Maintenance = []effects.PartDeclaration{{Key: "clock", Kind: effects.KindDuration, Params: json.RawMessage(`{"turn_ends":2}`), OnTerminal: generic.EndApplication}}
	s.Require().NoError(p.Validate(), "a clock does not imply concentration")
	p.Maintenance[0].Params = json.RawMessage(`{"turn_ends":0}`)
	s.Error(p.Validate())
}

func (s *CastProfileSuite) TestHistoricalCombinedConcentrationDeclarationIsRefused() {
	var p actions.CastProfile
	s.Error(json.Unmarshal([]byte(`{"range_feet":30,"target":"self","concentration":{"turn_ends":10}}`), &p), "silently dropping the hold is not a current-data reader")
}
