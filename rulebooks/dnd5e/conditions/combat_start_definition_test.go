// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

// combatStartFixture is a status an author wrote: it lasts until a fight its
// holder is in starts and grants advantage on Strength saving throws
// meanwhile.
const combatStartFixture = `{
 "parts":{
  "strength-saves":{"kind":"advantage","params":{"on":"saving_throw"},"when":{"fact":{"name":"action.ability","op":"eq","value":"str"}}},
  "fight-starts":{"kind":"end_on","params":{"beat":"combat_start","reason":"fight_started"}}
 },
 "benefit_sets":{"held":["strength-saves"]},
 "definitions":{
  "held":{"name":"Held","coordinator":"recipient","maintenance":[{"key":"fight-starts","part":"fight-starts"}],"benefits":[{"key":"held","set":"held"}]}
 }
}`

type combatStartDefinitionSuite struct{ suite.Suite }

func TestCombatStartDefinitionSuite(t *testing.T) { suite.Run(t, new(combatStartDefinitionSuite)) }

func (s *combatStartDefinitionSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(combatStartFixture)})
	s.Require().NoError(err)
	return w, defs.Definitions
}

func (w *definitionWorld) combatStart(who string) error {
	return dndevents.CombatStartTopic.On(w.bus).Publish(context.Background(), dndevents.CombatStartEvent{SubjectID: who})
}

func (s *combatStartDefinitionSuite) TestEndsWhenAFightTheHolderIsInStartsAndNotAnothers() {
	w, defs := s.setup()
	prepared, _, err := w.prepare(defs, "held", "B", "B", "keep")
	s.Require().NoError(err)
	s.Require().NoError(w.apply(prepared))
	s.Require().NotEmpty(w.holder("B").GetConditions())
	// The status is kept on the sheet, so it outlives a save and load.
	s.Require().NoError(w.reload())
	held, err := w.saveAdvantage("B", abilities.STR)
	s.Require().NoError(err)
	s.Positive(held)

	// Another member's fight starting, a rest and a fight ending do not end it.
	s.Require().NoError(w.combatStart("A"))
	s.Require().NoError(w.rest("B"))
	s.Require().NoError(dndevents.CombatEndTopic.On(w.bus).Publish(context.Background(), dndevents.CombatEndEvent{SubjectID: "B"}))
	s.NotEmpty(w.holder("B").GetConditions())
	held, err = w.saveAdvantage("B", abilities.STR)
	s.Require().NoError(err)
	s.Positive(held)

	// A fight the holder is in starting does.
	s.Require().NoError(w.combatStart("B"))
	s.Empty(w.holder("B").GetConditions())
	held, err = w.saveAdvantage("B", abilities.STR)
	s.Require().NoError(err)
	s.Zero(held)
}

func (s *combatStartDefinitionSuite) TestEndOnBeatsOutsideTheVocabularyAreRefused() {
	for name, raw := range map[string]string{
		"unknown beat":      strings.Replace(combatStartFixture, `"beat":"combat_start"`, `"beat":"fight_start"`, 1),
		"missing reason":    strings.Replace(combatStartFixture, `"reason":"fight_started"`, `"reason":" "`, 1),
		"not a holder beat": strings.Replace(combatStartFixture, `"beat":"combat_start"`, `"beat":"attack_attempt"`, 1),
	} {
		s.Run(name, func() {
			out, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
			s.Error(err)
			s.Nil(out)
		})
	}
}
