// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

const cook = `{"name":"Well fed",
 "changes":[{"advantage":"checks","when":{"ability":"charisma"}}],
 "maintenance":[{"ends_on":"run"}],
 "again":"ignored"}`

type statusSuite struct{ suite.Suite }

func TestStatusSuite(t *testing.T) { suite.Run(t, new(statusSuite)) }

// world is the custody a status needs to run: loaded sheets, one bus and the
// application binding over all of them, as resolution supplies it.
type world struct {
	sheets  []*character.Character
	bus     events.EventBus
	binding *conditions.ApplicationBinding
}

func (s *statusSuite) world() *world {
	w := &world{bus: events.NewEventBus()}
	for _, id := range []string{"A", "B"} {
		c, err := character.Load(context.Background(), &character.Data{ID: id, PlayerID: "player-" + id, Name: id, Level: 1, ProficiencyBonus: 2, ClassID: classes.Fighter, RaceID: races.Human, HitPoints: 20, MaxHitPoints: 20, AbilityScores: shared.AbilityScores{abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14, abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8}})
		s.Require().NoError(err)
		w.sheets = append(w.sheets, c)
	}
	binding, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: w.held(), Bus: w.bus})
	s.Require().NoError(err)
	w.binding = binding
	for _, c := range w.sheets {
		s.Require().NoError(character.Attach(context.Background(), c, w.bus))
	}
	s.T().Cleanup(func() { s.NoError(w.binding.Close(context.Background())) })
	return w
}

func (w *world) held() []conditions.ParticipantConditions {
	var out []conditions.ParticipantConditions
	for _, c := range w.sheets {
		out = append(out, conditions.ParticipantConditions{MemberID: c.GetID(), Conditions: c.GetConditions()})
	}
	return out
}

// give applies a compiled status to B the way resolution's GiveStatus does:
// user and recipient are the holder, coordinator first, then each benefit.
func (s *statusSuite) give(w *world, compiled *status.CompileOutput) {
	ctx := context.Background()
	defs, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: compiled.Library})
	s.Require().NoError(err)
	origin := dndevents.ConditionOrigin{Ref: core.Ref{Module: "site", Type: "statuses", ID: compiled.DefinitionID}, EntityID: "cook"}
	bound, err := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: defs.Definitions, DefinitionID: compiled.DefinitionID, UserID: "B", RecipientID: "B", ApplicationID: "give", Origin: origin})
	s.Require().NoError(err)
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "give", CoordinatorID: bound.CoordinatorID, Spec: bound.Spec, Origin: origin, Existing: w.held()})
	s.Require().NoError(err)
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	holder := w.sheets[1]
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: holder, Condition: owner.Condition, ApplicationID: "give"}))
	for i := range bound.Spec.Recipients {
		made, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: i})
		s.Require().NoError(err)
		s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: holder, Condition: made.Condition, ApplicationID: "give"}))
		_, err = w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "give", Key: made.Candidate.Key, Delivered: true})
		s.Require().NoError(err)
	}
	s.Require().NoError(w.binding.CheckComplete())
}

// checkAdvantage reports whether a skill check by B carries the status.
func (s *statusSuite) checkAdvantage(w *world, skill skills.Skill, name string) bool {
	ctx := context.Background()
	e := &dndevents.AbilityCheckChainEvent{CheckerID: "B", Skill: skill, DC: 10}
	c, err := dndevents.AbilityCheckChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[*dndevents.AbilityCheckChainEvent](combat.ModifierStages))
	s.Require().NoError(err)
	out, err := c.Execute(ctx, e)
	s.Require().NoError(err)
	for _, source := range out.AdvantageSources {
		if strings.Contains(source.Name, name) {
			return true
		}
	}
	return false
}

// THE COOK'S STATUS DOES WHAT ITS WORDS SAY: advantage on a Charisma check, not
// on a Strength check, until the holder's run ends.
func (s *statusSuite) TestTheCookExampleDoesWhatItSays() {
	compiled, err := status.Compile(&status.CompileInput{ID: "well-fed", Status: json.RawMessage(cook)})
	s.Require().NoError(err)
	s.Equal("Well fed", compiled.Name)
	s.Equal("well-fed", compiled.DefinitionID)

	w := s.world()
	s.give(w, compiled)
	s.True(s.checkAdvantage(w, skills.Persuasion, "Well fed"), "a Charisma check carries it")
	s.False(s.checkAdvantage(w, skills.Athletics, "Well fed"), "a Strength check does not")

	s.Require().NoError(dndevents.RunEndTopic.On(w.bus).Publish(context.Background(), dndevents.RunEndEvent{SubjectID: "A"}))
	s.NotEmpty(w.sheets[1].GetConditions(), "another member's run ending leaves it")
	s.Require().NoError(dndevents.RunEndTopic.On(w.bus).Publish(context.Background(), dndevents.RunEndEvent{SubjectID: "B"}))
	s.Empty(w.sheets[1].GetConditions(), "the holder's run ending ends it")
	s.False(s.checkAdvantage(w, skills.Persuasion, "Well fed"))
}

// EVERY ABILITY, BY ITS LONG NAME, REACHES THE ENGINE'S FACT: an author
// writes "charisma" and the guard compares the canonical id the check and the
// save carry, so the advantage lands on that ability's check and save and on
// no other ability's.
func (s *statusSuite) TestEachAbilityByNameGuardsItsOwnRolls() {
	ids := map[string]abilities.Ability{
		"strength": abilities.STR, "dexterity": abilities.DEX, "constitution": abilities.CON,
		"intelligence": abilities.INT, "wisdom": abilities.WIS, "charisma": abilities.CHA,
	}
	for name, id := range ids {
		s.Run(name, func() {
			doc := `{"name":"Sharp","changes":[{"advantage":"checks","when":{"ability":"` + name + `"}},` +
				`{"advantage":"saves","when":{"ability":"` + name + `"}}],"maintenance":[{"ends_on":"rest"}],"again":"ignored"}`
			compiled, err := status.Compile(&status.CompileInput{ID: "sharp", Status: json.RawMessage(doc)})
			s.Require().NoError(err)
			w := s.world()
			s.give(w, compiled)
			for _, other := range ids {
				s.Equal(other == id, s.abilityCheck(w, other, "Sharp"), "%s check under a %s status", other, name)
				s.Equal(other == id, s.save(w, other, "Sharp"), "%s save under a %s status", other, name)
			}
		})
	}
}

// abilityCheck reports whether a bare-ability check by B carries the status.
func (s *statusSuite) abilityCheck(w *world, ability abilities.Ability, name string) bool {
	ctx := context.Background()
	e := &dndevents.AbilityCheckChainEvent{CheckerID: "B", Ability: ability, DC: 10}
	c, err := dndevents.AbilityCheckChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[*dndevents.AbilityCheckChainEvent](combat.ModifierStages))
	s.Require().NoError(err)
	out, err := c.Execute(ctx, e)
	s.Require().NoError(err)
	for _, source := range out.AdvantageSources {
		if strings.Contains(source.Name, name) {
			return true
		}
	}
	return false
}

// save reports whether a saving throw by B carries the status.
func (s *statusSuite) save(w *world, ability abilities.Ability, name string) bool {
	ctx := context.Background()
	e := &dndevents.SavingThrowChainEvent{SaverID: "B", Ability: ability, DC: 10}
	c, err := dndevents.SavingThrowChain.On(w.bus).PublishWithChain(ctx, e, events.NewStagedChain[*dndevents.SavingThrowChainEvent](combat.ModifierStages))
	s.Require().NoError(err)
	out, err := c.Execute(ctx, e)
	s.Require().NoError(err)
	for _, source := range out.AdvantageSources {
		if strings.Contains(source.Name, name) {
			return true
		}
	}
	return false
}

// documents is one authored status per word and value in the table, so every
// word Compile accepts is exercised.
func documents(words []status.Word) map[string]string {
	base := func(change, rule, again string) string {
		return `{"name":"Test","changes":[` + change + `],"maintenance":[` + rule + `],"again":` + again + `}`
	}
	const change, rule, again = `{"advantage":"checks"}`, `{"ends_on":"run"}`, `"ignored"`
	out := map[string]string{}
	for _, w := range words {
		for _, v := range w.Values {
			quoted, _ := json.Marshal(v.Word)
			name := fmt.Sprintf("%s %s=%s", w.Place, w.Word, v.Word)
			switch w.Place {
			case status.PlaceChange:
				if w.Shape == status.ShapeListOf {
					out[name] = base(`{"`+w.Word+`":[`+string(quoted)+`]}`, rule, again)
				} else {
					out[name] = base(`{"`+w.Word+`":`+string(quoted)+`}`, rule, again)
				}
			case status.PlaceWhen:
				out[name] = base(`{"advantage":"attacks","when":{"`+w.Word+`":`+string(quoted)+`}}`, rule, again)
			case status.PlaceMaintenance:
				out[name] = base(change, `{"`+w.Word+`":`+string(quoted)+`}`, again)
			case status.PlaceStatus:
				out[name] = base(change, rule, string(quoted))
			}
		}
		if w.Shape == status.ShapeWholeNumber {
			out[string(w.Place)+" "+w.Word+"=+2"] = base(`{"`+w.Word+`":2}`, rule, again)
			out[string(w.Place)+" "+w.Word+"=-1"] = base(`{"`+w.Word+`":-1,"when":{"attack":"melee","ability":"strength"}}`, rule, again)
		}
	}
	out["two rules, two whens"] = base(`{"advantage":"attacks against you","when":{"ability":"dexterity","attack":"melee"}},{"resistance":["fire","cold"]}`, `{"ends_on":"rest"},{"ends_on":"next turn"}`, again)
	return out
}

// EVERY WORD AND VALUE COMPILES, and the library it produces is accepted by
// the conditions package and can be given to a holder.
func (s *statusSuite) TestEveryWordAndValueCompilesAndRuns() {
	for name, doc := range documents(status.Words(&status.WordsInput{}).Words) {
		s.Run(name, func() {
			compiled, err := status.Compile(&status.CompileInput{ID: "test", Status: json.RawMessage(doc)})
			s.Require().NoError(err, doc)
			_, err = conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: compiled.Library})
			s.Require().NoError(err)
			s.give(s.world(), compiled)
		})
	}
}

// THE TABLE AND THE COMPILER AGREE: every word in the table appears in a
// compiled document above, and every key Compile read in those documents is
// a word in the table.
func (s *statusSuite) TestWordsAndCompileAgree() {
	words := status.Words(&status.WordsInput{}).Words
	known := map[string]bool{}
	for _, w := range words {
		s.NotEmpty(w.Label, w.Word)
		s.Equal(strings.ToLower(w.Label[:1]), w.Label[:1], "%s label starts lower-case", w.Word)
		known[string(w.Place)+":"+w.Word] = true
		for _, v := range w.Values {
			s.NotEmpty(v.Label)
			s.Equal(strings.ToLower(v.Label[:1]), v.Label[:1], "%s label starts lower-case", v.Word)
		}
	}
	used := map[string]bool{}
	var walk func(place status.Place, raw json.RawMessage)
	walk = func(place status.Place, raw json.RawMessage) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil {
			return
		}
		for key, value := range fields {
			used[string(place)+":"+key] = true
			switch {
			case place == status.PlaceStatus && (key == "changes" || key == "maintenance"):
				var items []json.RawMessage
				s.Require().NoError(json.Unmarshal(value, &items))
				next := status.PlaceChange
				if key == "maintenance" {
					next = status.PlaceMaintenance
				}
				for _, item := range items {
					walk(next, item)
				}
			case place == status.PlaceChange && key == "when":
				walk(status.PlaceWhen, value)
			}
		}
	}
	for _, doc := range documents(words) {
		walk(status.PlaceStatus, json.RawMessage(doc))
	}
	for key := range used {
		s.True(known[key], "Compile accepted %s, which Words does not list", key)
	}
	for key := range known {
		s.True(used[key], "Words lists %s, which no compiled document used", key)
	}
}

// EVERY REFUSAL NAMES THE AUTHOR'S PATH and the words allowed there.
func (s *statusSuite) TestRefusalsNameThePathAndTheWords() {
	for name, tc := range map[string]struct{ doc, says string }{
		"unknown change":           {strings.Replace(cook, `"advantage":"checks"`, `"inspiration":"checks"`, 1), "changes[0].inspiration: not a change; a change is one of: advantage, resistance, damage"},
		"unknown value":            {strings.Replace(cook, `"advantage":"checks"`, `"advantage":"skills"`, 1), `changes[0].advantage: "skills" is not one of: "checks"`},
		"unknown when key":         {strings.Replace(cook, `"ability":"charisma"`, `"skill":"persuasion"`, 1), "changes[0].when.skill: not a when word; one of: ability, attack"},
		"unknown when value":       {strings.Replace(cook, `"charisma"`, `"cha"`, 1), `changes[0].when.ability: "cha" is not one of: "strength"`},
		"when the kind can't use":  {strings.Replace(cook, `"ability":"charisma"`, `"attack":"melee"`, 1), "changes[0].when.attack: advantage cannot be narrowed by attack; it can use ability"},
		"resistance takes no when": {strings.Replace(cook, `"advantage":"checks"`, `"resistance":["fire"]`, 1), "changes[0].when.ability: resistance cannot be narrowed by ability; it can use nothing"},
		"unknown damage type":      {strings.Replace(cook, `"advantage":"checks","when":{"ability":"charisma"}`, `"resistance":["sonic"]`, 1), `changes[0].resistance[0]: "sonic" is not one of`},
		"zero damage":              {strings.Replace(cook, `"advantage":"checks","when":{"ability":"charisma"}`, `"damage":0`, 1), "changes[0].damage: must be a whole number other than zero"},
		"fractional damage":        {strings.Replace(cook, `"advantage":"checks","when":{"ability":"charisma"}`, `"damage":1.5`, 1), "changes[0].damage: must be a whole number"},
		"two changes in one":       {strings.Replace(cook, `"advantage":"checks"`, `"advantage":"checks","damage":2`, 1), "changes[0]: one change per entry"},
		"empty maintenance":        {strings.Replace(cook, `[{"ends_on":"run"}]`, `[]`, 1), "maintenance: needs at least one rule"},
		"unknown rule":             {strings.Replace(cook, `"ends_on":"run"`, `"lasts":"run"`, 1), "maintenance[0].lasts: not a maintenance rule; one of: ends_on"},
		"unknown rule value":       {strings.Replace(cook, `"ends_on":"run"`, `"ends_on":"dawn"`, 1), `maintenance[0].ends_on: "dawn" is not one of: "run"`},
		"unknown again":            {strings.Replace(cook, `"again":"ignored"`, `"again":"refresh"`, 1), `again: "refresh" is not one of: "ignored"`},
		"missing name":             {strings.Replace(cook, `"name":"Well fed",`, ``, 1), "the status has no name"},
		"blank name":               {strings.Replace(cook, `"Well fed"`, `" "`, 1), "name: must be non-empty text"},
		"unknown top-level word":   {strings.Replace(cook, `"again":"ignored"`, `"again":"ignored","lasts":"forever"`, 1), "the status: unknown word lasts; one of: name, changes, maintenance, again"},
		"no changes":               {strings.Replace(cook, `[{"advantage":"checks","when":{"ability":"charisma"}}]`, `[]`, 1), "changes: needs at least one change"},
		"repeated key":             {strings.Replace(cook, `"again":"ignored"`, `"again":"ignored","again":"ignored"`, 1), "the status: must be an object of"},
	} {
		s.Run(name, func() {
			out, err := status.Compile(&status.CompileInput{ID: "well-fed", Status: json.RawMessage(tc.doc)})
			s.Require().ErrorIs(err, status.ErrBadStatus)
			s.Nil(out)
			s.Contains(err.Error(), tc.says)
		})
	}
}
