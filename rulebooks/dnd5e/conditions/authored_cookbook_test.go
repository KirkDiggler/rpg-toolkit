// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type authoredEffectsCookbookSuite struct{ suite.Suite }

func TestAuthoredEffectsCookbookSuite(t *testing.T) { suite.Run(t, new(authoredEffectsCookbookSuite)) }
func loadCookbook() (*conditions.CompiledDefinitions, error) {
	raw, err := os.ReadFile("testdata/effects/library.json")
	if err != nil {
		return nil, err
	}
	out, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: raw})
	if err != nil {
		return nil, err
	}
	return out.Definitions, nil
}
func (s *authoredEffectsCookbookSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := loadCookbook()
	s.Require().NoError(err)
	return w, defs
}
func (s *authoredEffectsCookbookSuite) use(w *definitionWorld, defs *conditions.CompiledDefinitions, id, user, target, application string) (*fixtureSource, *preparedDefinitionGrant) {
	source := newFixtureSource("item-"+application, user, id)
	result, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: target, ApplicationID: application, Definitions: defs, Participants: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	return source, result
}
func (s *authoredEffectsCookbookSuite) TestFiveDefinitionsUsePublicAPI() {
	for _, id := range []string{"rage-ring", "rage-scroll", "physical-ward", "damage-gift", "kitchen-sausage"} {
		s.Run(id, func() {
			w, defs := s.setup()
			user := "A"
			if id == "kitchen-sausage" {
				user = "B"
			}
			source, grant := s.use(w, defs, id, user, "B", "use")
			s.Require().NoError(w.apply(grant.Prepared))
			s.Zero(source.Budget.Current)
			s.Require().NoError(w.binding.CheckComplete())
			s.Require().NoError(w.reload())
			s.NotEmpty(w.holder("B").GetConditions())
		})
	}
}
func (s *authoredEffectsCookbookSuite) TestInvalidSourceOrPreflightCostsNothing() {
	for _, which := range []string{"definition", "recipient", "unavailable", "exhausted", "collision"} {
		s.Run(which, func() {
			w, defs := s.setup()
			source := newFixtureSource("item", "A", "rage-ring")
			target := "B"
			application := "new"
			switch which {
			case "definition":
				source.DefinitionID = "missing"
			case "recipient":
				target = "missing"
			case "unavailable":
				source.Available = false
			case "exhausted":
				s.Require().NoError(source.Budget.Use(1))
			case "collision":
				_, grant := s.use(w, defs, "rage-scroll", "A", "B", "old")
				s.Require().NoError(w.apply(grant.Prepared))
				source.UserID = "B"
				target = "C"
				application = grant.Spec.Recipients[0].AddressSourceID
			}
			before := source.Budget.Current
			out, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: target, ApplicationID: application, Definitions: defs, Participants: bindingParticipants(w.sheets)})
			s.Error(err)
			s.Nil(out)
			s.Equal(before, source.Budget.Current)
			s.Require().NoError(w.binding.CheckComplete())
		})
	}
}
func (s *authoredEffectsCookbookSuite) TestPaymentIsSettledBeforeDeliveryAndOnlyOnce() {
	w, defs := s.setup()
	source, grant := s.use(w, defs, "rage-ring", "A", "B", "use")
	s.Zero(grant.Remaining)
	var paid []int
	_, err := dndevents.ConditionAppliedTopic.On(w.bus).Subscribe(context.Background(), func(context.Context, dndevents.ConditionAppliedEvent) error {
		paid = append(paid, source.Budget.Current)
		return nil
	})
	s.Require().NoError(err)
	s.Require().NoError(w.apply(grant.Prepared))
	s.Equal([]int{0, 0}, paid)
	_, err = useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: "B", ApplicationID: "again", Definitions: defs, Participants: bindingParticipants(w.sheets)})
	s.Error(err)
	s.Zero(source.Budget.Current)
}
func (s *authoredEffectsCookbookSuite) TestPreparationDoesNotDeliver() {
	w, defs := s.setup()
	source := newFixtureSource("item", "A", "rage-ring")
	out, err := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: defs, DefinitionID: source.DefinitionID, UserID: "A", RecipientID: "B", ApplicationID: "use", Origin: source.origin()})
	s.Require().NoError(err)
	_, err = conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "use", CoordinatorID: out.CoordinatorID, Spec: out.Spec, Origin: source.origin(), Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	s.Equal(1, source.Budget.Current)
	paid, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: "B", ApplicationID: "use", Definitions: defs, Participants: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	s.Zero(source.Budget.Current)
	s.NotNil(paid.Prepared)
	s.Empty(w.holder("A").GetConditions())
	s.Empty(w.holder("B").GetConditions())
}
func (s *authoredEffectsCookbookSuite) TestFixedBonusNeedsNoBarbarianLevel() {
	w, defs := s.setup()
	_, grant := s.use(w, defs, "damage-gift", "A", "B", "use")
	s.Require().NoError(w.apply(grant.Prepared))
	out, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	total := 0
	for _, component := range out.Components {
		if component.Roll.Modifier != nil {
			total += *component.Roll.Modifier
		}
	}
	s.Equal(12, total)
}
func (s *authoredEffectsCookbookSuite) TestSourceStateRefusesInvalidRemainingBeforeRestore() {
	source := newFixtureSource("item", "A", "rage-ring")
	s.Require().NoError(source.Budget.Use(1))
	saved := source.data()
	restored, err := restoreFixtureSource(saved)
	s.Require().NoError(err)
	s.Zero(restored.Budget.Current)
	for _, n := range []int{-1, 2} {
		bad := saved
		bad.Remaining = n
		_, err := restoreFixtureSource(bad)
		s.Error(err)
	}
}
func (s *authoredEffectsCookbookSuite) TestNoExamplesDependency() {
	command := exec.Command("go", "list", "-deps", "-test", "./conditions", "./effects", "./contributions")
	command.Dir = ".."
	output, err := command.CombinedOutput()
	s.Require().NoError(err, string(output))
	for _, pkg := range strings.Fields(string(output)) {
		s.False(strings.HasPrefix(pkg, "github.com/KirkDiggler/rpg-toolkit/examples/"), pkg)
	}
}
