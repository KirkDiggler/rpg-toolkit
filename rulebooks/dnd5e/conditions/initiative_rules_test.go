// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// initiativeFixture adds to heldBonusFixture's statuses one an author wrote
// with an ability modifier: Keen mind adds the holder's Wisdom modifier.
const initiativeFixture = `{
 "parts":{
  "quick":{"kind":"initiative_bonus","amount":{"fixed":1}},
  "keen":{"kind":"initiative_bonus","amount":{"ability_modifier":{"ability":"wis"}}},
  "run-end":{"kind":"end_on","params":{"beat":"run_end","reason":"run_ended"}}
 },
 "benefit_sets":{"quick":["quick"],"keen":["keen"]},
 "definitions":{
  "well-fed":{"name":"Well fed","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"quick","set":"quick"}]},
  "keen-mind":{"name":"Keen mind","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"keen","set":"keen"}]}
 }
}`

type initiativeRulesSuite struct{ suite.Suite }

func TestInitiativeRulesSuite(t *testing.T) { suite.Run(t, new(initiativeRulesSuite)) }

func (s *initiativeRulesSuite) setup() (*definitionWorld, *conditions.CompiledDefinitions) {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(initiativeFixture)})
	s.Require().NoError(err)
	return w, defs.Definitions
}

func (s *initiativeRulesSuite) give(w *definitionWorld, defs *conditions.CompiledDefinitions, id, target string) {
	prepared, _, err := w.prepare(defs, id, target, target, id+"-"+target)
	s.Require().NoError(err)
	s.Require().NoError(w.apply(prepared))
}

func (s *initiativeRulesSuite) initiative(w *definitionWorld, who string) *conditions.InitiativeOutput {
	out, err := w.holder(who).Initiative()
	s.Require().NoError(err)
	return out
}

// Catches the rule reading the wrong ability, a modifier formula off by one
// (or rounding a negative toward zero), or a source other than "Dexterity".
func (s *initiativeRulesSuite) TestDexterityAddsItsModifierFromTheRule() {
	for dex, want := range map[int]int{18: 4, 10: 0, 8: -1, 1: -5} {
		out, err := conditions.Initiative(&conditions.InitiativeInput{MemberID: "m",
			Scores: shared.AbilityScores{abilities.DEX: dex, abilities.STR: 20}})
		s.Require().NoError(err)
		s.Equal([]conditions.InitiativeContribution{{Source: "Dexterity", Amount: want}}, out.Contributions, "Dexterity %d", dex)
		s.Equal(want, out.Total)
	}
}

// Catches a missing score read as 0 (which would be -5): a sheet that states
// no ability scores, or none for Dexterity, gets no Dexterity contribution.
// A stated score of 0 is a real score and gives -5.
func (s *initiativeRulesSuite) TestNoStatedScoreMeansNoContribution() {
	for name, scores := range map[string]shared.AbilityScores{"nil": nil, "empty": {}, "no dex": {abilities.STR: 16}} {
		out, err := conditions.Initiative(&conditions.InitiativeInput{MemberID: "m", Scores: scores})
		s.Require().NoError(err, name)
		s.Empty(out.Contributions, name)
		s.Zero(out.Total, name)
	}
	out, err := conditions.Initiative(&conditions.InitiativeInput{MemberID: "m", Scores: shared.AbilityScores{abilities.DEX: 0}})
	s.Require().NoError(err)
	s.Equal(-5, out.Total, "a stated 0 is a score")
}

// Catches a held effect that replaces the rule, comes first, or is counted
// once per half of a status whose two halves sit on one sheet.
func (s *initiativeRulesSuite) TestAHeldEffectAddsAndIsListedSecond() {
	w, defs := s.setup()
	s.give(w, defs, "well-fed", "B")
	out := s.initiative(w, "B")
	s.Equal([]conditions.InitiativeContribution{{Source: "Dexterity", Amount: 2}, {Source: "Well fed", Amount: 1}}, out.Contributions,
		"the sheet's DEX 14 by rule, then the held status, once")
	s.Equal(3, out.Total)
	s.Equal(2, s.initiative(w, "A").Total, "a member without the status has the rule only")
}

// Catches an author's ability-modifier effect reading Dexterity (or nothing)
// instead of the ability it names.
func (s *initiativeRulesSuite) TestAnAuthoredEffectCanPlugInAnotherAbility() {
	w, defs := s.setup()
	s.give(w, defs, "keen-mind", "B")
	out := s.initiative(w, "B")
	s.Equal([]conditions.InitiativeContribution{{Source: "Dexterity", Amount: 2}, {Source: "Keen mind", Amount: 1}}, out.Contributions,
		"the sheet's WIS 12 gives +1")
}

// Catches a rule that is stored, or a held effect that outlives its ending:
// after the run ends only the rule is left.
func (s *initiativeRulesSuite) TestEndingTheHeldEffectLeavesOnlyTheRule() {
	w, defs := s.setup()
	s.give(w, defs, "well-fed", "B")
	s.Require().NoError(w.runEnd("B"))
	s.Equal([]conditions.InitiativeContribution{{Source: "Dexterity", Amount: 2}}, s.initiative(w, "B").Contributions)
}

// Catches the rule leaking onto the sheet: it is not in the saved data, not
// a condition the sheet holds, not a status-view row, and survives a reload
// because it is read, not loaded.
func (s *initiativeRulesSuite) TestTheRuleIsNeverOnTheSheet() {
	w, defs := s.setup()
	s.give(w, defs, "well-fed", "B")
	data, err := w.holder("A").ToData()
	s.Require().NoError(err)
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	for _, needle := range []string{"dexterity-initiative", "every-creature", "\"rules\"", "ability_modifier"} {
		s.NotContains(string(raw), needle)
	}
	s.Empty(w.holder("A").GetConditions())
	view, err := w.holder("B").StatusView(&character.StatusViewInput{})
	s.Require().NoError(err)
	for _, row := range view.View.Conditions {
		s.NotEqual("Dexterity", row.Name)
	}
	s.Require().NoError(w.reload())
	s.Equal(2, s.initiative(w, "A").Total)
}

// Catches the new ability-modifier form describing itself with a fixed
// number, or claiming to apply to an attack.
func (s *initiativeRulesSuite) TestAnAbilityModifierEffectDescribesItself() {
	w, defs := s.setup()
	s.give(w, defs, "keen-mind", "B")
	frame, err := w.frame("B", "C")
	s.Require().NoError(err)
	info, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: w.holder("B").GetConditions(), Frame: frame})
	s.Require().NoError(err)
	found := false
	for _, row := range info.Effects {
		if row.Description == "Adds the holder's Wisdom modifier to the holder's initiative." {
			found = true
			s.Equal(contributions.StateDoesNotApply, row.State)
		}
	}
	s.True(found)
}

// Catches an ability modifier accepted where it does not belong or naming
// something that is not an ability, and an unknown member inside it.
func (s *initiativeRulesSuite) TestAbilityModifierRefusals() {
	for name, tc := range map[string]struct{ part, says string }{
		"unknown ability":    {`{"kind":"initiative_bonus","amount":{"ability_modifier":{"ability":"luck"}}}`, "names no ability this rulebook has"},
		"empty ability":      {`{"kind":"initiative_bonus","amount":{"ability_modifier":{"ability":""}}}`, "names no ability"},
		"unknown member":     {`{"kind":"initiative_bonus","amount":{"ability_modifier":{"ability":"dex","bonus":2}}}`, "bonus"},
		"on a hit point max": {`{"kind":"hit_point_maximum","amount":{"ability_modifier":{"ability":"con"}}}`, "requires a fixed whole number"},
	} {
		s.Run(name, func() {
			raw := strings.Replace(initiativeFixture, `{"kind":"initiative_bonus","amount":{"fixed":1}}`, tc.part, 1)
			s.Require().NotEqual(initiativeFixture, raw)
			_, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
			s.Require().Error(err)
			s.Contains(err.Error(), tc.says)
		})
	}
}

// Catches a reader that refuses nothing.
func (s *initiativeRulesSuite) TestTheReaderRefusesNoMember() {
	_, err := conditions.Initiative(nil)
	s.Error(err)
	_, err = conditions.Initiative(&conditions.InitiativeInput{MemberID: " "})
	s.Error(err)
}

// BenchmarkInitiative measures one Initiative call on a sheet with no held
// effects and with five. A measurement for the record, not a gate.
func BenchmarkInitiative(b *testing.B) {
	for _, held := range []int{0, 5} {
		b.Run(map[int]string{0: "no held effects", 5: "five held effects"}[held], func(b *testing.B) {
			w, err := newDefinitionWorld()
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = w.close() }()
			defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(initiativeFixture)})
			if err != nil {
				b.Fatal(err)
			}
			for i := 0; i < held; i++ {
				id := []string{"well-fed", "keen-mind"}[i%2]
				prepared, _, err := w.prepare(defs.Definitions, id, "B", "B", id+"-"+string(rune('a'+i)))
				if err != nil {
					b.Fatal(err)
				}
				if err := w.apply(prepared); err != nil {
					b.Fatal(err)
				}
			}
			sheet := w.holder("B")
			if _, err := sheet.Initiative(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := sheet.Initiative(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
