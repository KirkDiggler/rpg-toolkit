// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type authoredApplicationSuite struct{ suite.Suite }

func TestAuthoredApplicationSuite(t *testing.T) { suite.Run(t, new(authoredApplicationSuite)) }
func (s *authoredApplicationSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(definitionFixture)})
	s.Require().NoError(err)
	return w, defs.Definitions
}
func (s *authoredApplicationSuite) TestSelfUseHasTwoDistinctLocalAddresses() {
	w, defs := s.setup()
	p, spec, err := w.prepare(defs, "ring", "A", "A", "self")
	s.Require().NoError(err)
	s.NotEqual("self", spec.Recipients[0].AddressSourceID)
	s.Require().NoError(w.apply(p))
	s.Len(w.holder("A").GetConditions(), 2)
	s.Require().NoError(w.reload())
	s.Len(w.holder("A").GetConditions(), 2)
}
func (s *authoredApplicationSuite) TestCrossApplicationCollisionRefusedBeforePayment() {
	for _, mode := range []string{"owner", "recipient", "pending"} {
		s.Run(mode, func() {
			w, defs := s.setup()
			old, oldSpec, err := w.prepare(defs, "scroll", "A", "B", "old")
			s.Require().NoError(err)
			if mode == "pending" {
				s.Require().NoError(w.owner(old))
			} else {
				s.Require().NoError(w.apply(old))
			}
			if mode == "owner" {
				_, _, err = w.prepare(defs, "ring", "B", "C", oldSpec.Recipients[0].AddressSourceID)
			} else {
				b, bindErr := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: defs, DefinitionID: "ring", UserID: "C", RecipientID: "B", ApplicationID: "new", Origin: definitionOrigin()})
				s.Require().NoError(bindErr)
				b.Spec.Recipients[0].AddressSourceID = oldSpec.Recipients[0].AddressSourceID
				_, err = conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "new", CoordinatorID: b.CoordinatorID, Origin: definitionOrigin(), Spec: b.Spec, Existing: bindingParticipants(w.sheets)})
			}
			s.Error(err, "foreign addresses cannot be discovered only after payment/delivery")
		})
	}
}
func (s *authoredApplicationSuite) TestReplacementAllowsOnlyTheDescribedOwner() {
	w, defs := s.setup()
	old, oldSpec, err := w.prepare(defs, "ring", "A", "B", "old")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(old))
	next, nextSpec, err := w.prepare(defs, "ring", "A", "B", "new")
	s.Require().NoError(err)
	replacement, ok := next.Replacement()
	s.True(ok)
	s.Equal("old", replacement.SourceID)
	nextSpec.Recipients[0].AddressSourceID = oldSpec.Recipients[0].AddressSourceID
	allowed, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "new", CoordinatorID: "A", Origin: definitionOrigin(), Spec: nextSpec, Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	s.Require().NotNil(allowed)
	foreign, foreignSpec, err := w.prepare(defs, "scroll", "C", "B", "foreign")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(foreign))
	nextSpec.Recipients[0].AddressSourceID = foreignSpec.Recipients[0].AddressSourceID
	_, err = conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "new", CoordinatorID: "A", Origin: definitionOrigin(), Spec: nextSpec, Existing: bindingParticipants(w.sheets)})
	s.Error(err, "same origin is not replacement authority")
}
func (s *authoredApplicationSuite) TestRestoredDeclarationDoesNotReadChangedLibrary() {
	w, defs := s.setup()
	p, _, err := w.prepare(defs, "ring", "A", "B", "frozen")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(p))
	_, err = conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(strings.ReplaceAll(definitionFixture, `"fixed":2`, `"fixed":99`))})
	s.Require().NoError(err)
	s.Require().NoError(w.reload())
	raw, err := w.holder("B").GetConditions()[0].ToJSON()
	s.Require().NoError(err)
	var data conditions.LinkedConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Equal(2, *data.Bindings.Parts[0].Amount.Fixed)
	s.Equal("item-1", data.Origin.EntityID)
}
func (s *authoredApplicationSuite) TestPendingAuthoredDeliveryResumesWithoutNewOwner() {
	w, defs := s.setup()
	p, spec, err := w.prepare(defs, "ring", "A", "B", "pending")
	s.Require().NoError(err)
	s.Require().NoError(w.owner(p))
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(w.reload())
	resumed, err := conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: "pending", Spec: spec, Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	_, err = conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: resumed})
	s.Error(err)
	_, err = w.deliver(resumed, true)
	s.Require().NoError(err)
	s.Require().NoError(w.binding.CheckComplete())
	s.Len(w.holder("A").GetConditions(), 1)
	s.Len(w.holder("B").GetConditions(), 1)
}
func (s *authoredApplicationSuite) TestEarlyAppliedUnfinalizedRecipientRefused() {
	w, defs := s.setup()
	p, _, err := w.prepare(defs, "ring", "A", "B", "orphan")
	s.Require().NoError(err)
	s.Require().NoError(w.owner(p))
	_, err = w.deliver(p, false)
	s.Require().NoError(err)
	s.Error(w.binding.CheckComplete())
}
func (s *authoredApplicationSuite) TestOldGenerationCannotRemoveNewBenefit() {
	w, defs := s.setup()
	p, _, err := w.prepare(defs, "ring", "A", "B", "old")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(p))
	p, spec, err := w.prepare(defs, "ring", "A", "B", "new")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(p))
	addr := dndevents.ConditionAddress{MemberID: "B", ConditionRef: spec.Recipients[0].Ref, SourceID: spec.Recipients[0].AddressSourceID}
	s.Error(w.remove(addr, "old", "stale"))
	s.Len(w.holder("B").GetConditions(), 1)
	s.Require().NoError(w.binding.CheckComplete())
}
func (s *authoredApplicationSuite) TestFrozenGuardsAmountsAndConfigAreOwned() {
	w, defs := s.setup()
	p, spec, err := w.prepare(defs, "ring", "A", "B", "owned")
	s.Require().NoError(err)
	*spec.Recipients[0].Parts[0].Amount.Fixed = 99
	spec.Recipients[0].Parts[0].When.Fact.Value = "dex"
	spec.Recipients[0].Parts[1].Params[0] = '!'
	s.Require().NoError(w.apply(p))
	s.Require().NoError(w.reload())
	raw, err := w.holder("B").GetConditions()[0].ToJSON()
	s.Require().NoError(err)
	var data conditions.LinkedConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Equal(2, *data.Bindings.Parts[0].Amount.Fixed)
	s.Equal("str", data.Bindings.Parts[0].When.Fact.Value)
	s.JSONEq(`{"types":["slashing"]}`, string(data.Bindings.Parts[1].Params))
}
func (s *authoredApplicationSuite) TestEndingCannotResurrectAfterReload() {
	w, defs := s.setup()
	p, spec, err := w.prepare(defs, "ring", "A", "B", "ended")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(p))
	s.Require().NoError(w.remove(dndevents.ConditionAddress{MemberID: "A", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "ended"}, "ended", "end"))
	s.Require().NoError(w.reload())
	_, err = conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: "ended", Spec: spec, Existing: bindingParticipants(w.sheets)})
	s.Error(err)
	s.Empty(w.holder("B").GetConditions())
}
