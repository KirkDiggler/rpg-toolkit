// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

func (s *partsSuite) TestPartBindingsRefuseMissingAndForeignFacts() {
	_, err := dndfx.BindPart(nil)
	s.Error(err)
	for _, change := range []func(*dndfx.BindPartInput){
		func(in *dndfx.BindPartInput) { in.Declaration.Key = "" },
		func(in *dndfx.BindPartInput) { in.Declaration.Kind = "unknown" },
		func(in *dndfx.BindPartInput) { in.Declaration.OnTerminal = fx.EndPart },
		func(in *dndfx.BindPartInput) { in.Declaration.Params = json.RawMessage(`{"spell":"bless"}`) },
		func(in *dndfx.BindPartInput) { in.HolderID = "" },
		func(in *dndfx.BindPartInput) { in.ApplicationID = "" },
		func(in *dndfx.BindPartInput) { in.DisplayName = "" },
		func(in *dndfx.BindPartInput) { in.Origin.EntityID = "" },
		func(in *dndfx.BindPartInput) { in.Ref = core.Ref{} },
		func(in *dndfx.BindPartInput) { in.Owner.MemberID = "other" },
		func(in *dndfx.BindPartInput) { in.Owner.SourceID = "other-application" },
		func(in *dndfx.BindPartInput) { in.Owner.ConditionRef = "not-a-ref" },
	} {
		input := partInput("hold", dndfx.KindConcentration, `{}`)
		change(input)
		got, err := dndfx.BindPart(input)
		s.Error(err)
		s.Nil(got)
	}
	for _, params := range []string{`{}`, `{"turn_ends":0}`, `{"turn_ends":-1}`, `{"turn_ends":2,"unknown":true}`} {
		got, err := dndfx.BindPart(partInput("clock", dndfx.KindDuration, params))
		s.Error(err)
		s.Nil(got)
	}
}

func (s *partsSuite) TestPartInspectionRefusesInvalidStateRatherThanRefilling() {
	declaration := partInput("clock", dndfx.KindDuration, `{"turn_ends":2}`).Declaration
	for _, snapshot := range []fx.Snapshot{
		{},
		{State: "active", Bag: machine.Bag{}},
		{State: "active", Bag: machine.Bag{"remaining": 3}},
		{State: "active", Bag: machine.Bag{"remaining": 0}},
		{State: "fresh", Bag: machine.Bag{"remaining": 2}},
		{State: "unknown", Bag: machine.Bag{"remaining": 2}},
	} {
		got, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: declaration, Snapshot: snapshot})
		s.Error(err)
		s.Nil(got)
	}
}
