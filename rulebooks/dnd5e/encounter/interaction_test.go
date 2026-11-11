// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// InteractionSuite is Gate S2 of rpg-project#769 (sub-issue #922): THE
// ENTRANCE REMEMBERS. One interaction instance per authored interaction,
// owned by the run (#916 R3): its state gates the lines offered at the gate
// for every speaker (R2), the settled verdict of a line advances it, and it
// survives a reload.
type InteractionSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *InteractionSuite) SetupTest() { s.ctx = context.Background() }

func TestInteractionSuite(t *testing.T) { suite.Run(t, new(InteractionSuite)) }

const theFrontGate encounter.InteractionID = "front-gate"

// frontGateRecipe is the keep's two-claim gate: the first failed claim warns
// and removes itself; the other claim's failure after it arrests; a success
// from any open state admits.
func frontGateRecipe() machine.Recipe {
	to := func(on machine.Beat, state string) machine.Transition {
		return machine.Transition{On: on, To: state, Reason: string(on)}
	}
	return machine.Recipe{
		Ref:     core.Ref{Module: "test", Type: "interactions", ID: "front-gate"},
		Initial: "fresh",
		States: map[string]machine.State{
			"fresh": {Transitions: []machine.Transition{
				to("guest-failed", "warned-guest"), to("troupe-failed", "warned-troupe"), to("admitted", "admitted"),
			}},
			"warned-guest":  {Transitions: []machine.Transition{to("troupe-failed", "arrested"), to("admitted", "admitted")}},
			"warned-troupe": {Transitions: []machine.Transition{to("guest-failed", "arrested"), to("admitted", "admitted")}},
			"admitted":      {Terminal: true},
			"arrested":      {Terminal: true},
		},
	}
}

// gateClaims is the two claims the captain hears, each gated on the gate
// being open to it and advancing the gate on its verdict.
func gateClaims() []encounter.Option {
	claim := func(id string, open []string, failed machine.Beat) encounter.Option {
		return encounter.Option{
			ID: id, Say: "We're expected.",
			Check:    []encounter.CheckApproach{{Ability: "persuasion", DC: 11}},
			Requires: []encounter.Trigger{encounter.TriggerInteraction{Interaction: theFrontGate, In: open}},
			Advance:  &encounter.Advance{Interaction: theFrontGate, Success: "admitted", Failure: failed},
			Success:  []encounter.Answer{{Weight: 1, Say: "Through you go."}},
			Failure:  []encounter.Answer{{Weight: 1, Say: "What's going on here?"}},
		}
	}
	return []encounter.Option{
		claim("guest", []string{"fresh", "warned-troupe"}, "guest-failed"),
		claim("troupe", []string{"fresh", "warned-guest"}, "troupe-failed"),
	}
}

// gate is a yard with alice and billy both in the captain's (the goblin's)
// sight, the captain carrying lines, and the front gate declared.
func (s *InteractionSuite) gate(lines []encounter.Option, interactions ...encounter.InteractionInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:       openAir(),
			Regions:      []encounter.RegionInput{rectRegion("yard", 0, 0, 8, 8)},
			Interactions: interactions,
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 3}, Options: lines},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: (&OptionTestSuite{}).capabilities(),
	})
}

// openGate is the gate with both claims, built or the test fails.
func (s *InteractionSuite) openGate() *encounter.Encounter {
	enc, err := s.gate(gateClaims(), encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
	s.Require().NoError(err)
	return enc
}

// claim speaks a claim to the captain with the verdict given.
func (s *InteractionSuite) claim(enc *encounter.Encounter, who core.EntityID, option string, beaten bool) error {
	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach, Actor: who, Target: goblin, OptionID: option, Skill: "persuasion",
		Beaten: beaten, DC: 11, Total: 8, Roller: rollsLowest{},
		Revision: offeredRevision(enc, who, goblin, option),
	})
	return err
}

// offeredRevision is the revision a line is offered to a speaker at now —
// what a host's reply carries back — or zero when it is not offered.
func offeredRevision(enc *encounter.Encounter, who, target core.EntityID, option string) uint64 {
	offered, err := enc.OptionsFor(who, target)
	if err != nil {
		return 0
	}
	for _, o := range offered {
		if o.ID == option {
			return o.Revision
		}
	}
	return 0
}

// offered is the claims the captain offers one speaker.
func (s *InteractionSuite) offered(enc *encounter.Encounter, who core.EntityID) []string {
	options, err := enc.OptionsFor(who, goblin)
	s.Require().NoError(err)
	return ids(options)
}

// reloaded is enc saved and loaded again.
func (s *InteractionSuite) reloaded(data encounter.EncounterData) (*encounter.Encounter, error) {
	return encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: (&OptionTestSuite{}).capabilities(),
	})
}

// saved is the gate's saved progress.
func (s *InteractionSuite) saved(enc *encounter.Encounter) encounter.InteractionData {
	data := enc.ToData()
	s.Require().Len(data.Interactions, 1)
	return data.Interactions[0]
}

// interactionBeats is every interaction beat a member was told.
func (s *InteractionSuite) interactionBeats(enc *encounter.Encounter, who core.EntityID) []map[string]any {
	reader := &OptionTestSuite{}
	reader.SetT(s.T())
	return reader.beatsOfKind(enc, who, encounter.BeatInteraction)
}

// TestInteractionRequirementIsJudgedPerRun (Task 2, #916 R2): with the gate
// warned against the guest claim, that claim is absent for BOTH speakers and
// the troupe claim offered to both — the run's state is read, not the
// speaker's — and the door agrees.
func (s *InteractionSuite) TestInteractionRequirementIsJudgedPerRun() {
	enc := s.openGate()
	s.Equal([]string{"guest", "troupe"}, s.offered(enc, alice), "a fresh gate hears both")

	data := enc.ToData()
	data.Interactions[0].Blob.State = "warned-guest"
	warned, err := s.reloaded(data)
	s.Require().NoError(err)

	s.Equal([]string{"troupe"}, s.offered(warned, alice))
	s.Equal([]string{"troupe"}, s.offered(warned, billy), "a different speaker meets the same gate")
	s.ErrorIs(s.claim(warned, billy, "guest", true), encounter.ErrNoOption, "and the door agrees")
}

// TestTheVerdictAdvancesTheEntrance (Task 4): a failed guest claim warns the
// gate — an interaction beat to the witnesses, the claim gone for every
// speaker, the other claim kept; a failed troupe claim after it arrests and
// both are gone; on a fresh gate a succeeded claim admits.
func (s *InteractionSuite) TestTheVerdictAdvancesTheEntrance() {
	enc := s.openGate()

	s.Require().NoError(s.claim(enc, alice, "guest", false))
	s.Equal("warned-guest", s.saved(enc).Blob.State)
	beats := s.interactionBeats(enc, billy)
	s.Require().Len(beats, 1, "billy saw it")
	s.Equal(map[string]any{
		"beat": encounter.BeatInteraction, "interaction": string(theFrontGate), "from": "fresh", "to": "warned-guest",
		"reason": "guest-failed", "option": "guest", "beaten": false,
	}, beats[0])
	s.Equal([]string{"troupe"}, s.offered(enc, billy), "the failed claim is gone for the next speaker")

	reloaded, err := s.reloaded(enc.ToData())
	s.Require().NoError(err)
	s.Equal([]string{"troupe"}, s.offered(reloaded, alice), "and after a reload")

	s.Require().NoError(s.claim(reloaded, billy, "troupe", false))
	s.Equal("arrested", s.saved(reloaded).Blob.State)
	s.Empty(s.offered(reloaded, alice), "a second failure leaves no claim")

	fresh := s.openGate()
	s.Require().NoError(s.claim(fresh, alice, "troupe", true))
	s.Equal("admitted", s.saved(fresh).Blob.State)
	s.Empty(s.offered(fresh, billy))
}

// TestAdvanceNobodyListensToIsANoOp: a verdict whose beat the current state
// does not listen for steps nothing — the state stays and no beat is
// written.
func (s *InteractionSuite) TestAdvanceNobodyListensToIsANoOp() {
	lines := gateClaims()
	lines[0].Requires = nil // the guest claim is always offered here
	enc, err := s.gate(lines, encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
	s.Require().NoError(err)

	s.Require().NoError(s.claim(enc, alice, "troupe", true))
	s.Require().Equal("admitted", s.saved(enc).Blob.State)
	before := len(s.interactionBeats(enc, alice))

	s.Require().NoError(s.claim(enc, alice, "guest", false), "admitted is terminal and hears nothing")
	s.Equal("admitted", s.saved(enc).Blob.State)
	s.Len(s.interactionBeats(enc, alice), before, "no transition, no beat")
}

// TestARefusedSpeakNeverStepsTheInteraction: the door's refusals all come
// before the first write, so a line refused (here: no such approach) leaves
// the gate where it was.
func (s *InteractionSuite) TestARefusedSpeakNeverStepsTheInteraction() {
	enc := s.openGate()
	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach, Actor: alice, Target: goblin, OptionID: "guest", Skill: "deception",
		DC: 11, Total: 3, Roller: rollsLowest{}, Revision: offeredRevision(enc, alice, goblin, "guest"),
	})
	s.Require().ErrorIs(err, encounter.ErrNoOption)
	s.Equal("fresh", s.saved(enc).Blob.State)
}

// TestAdvanceNamingAnUnknownBeatIsRefused: an advance on a beat no state of
// the recipe listens for, or on an interaction the run lacks, is refused at
// setup — a verdict that could never land.
func (s *InteractionSuite) TestAdvanceNamingAnUnknownBeatIsRefused() {
	for name, advance := range map[string]*encounter.Advance{
		"an unknown beat":        {Interaction: theFrontGate, Success: "admitted", Failure: "bribed"},
		"an unknown interaction": {Interaction: "back-gate", Success: "admitted"},
		"no beat at all":         {Interaction: theFrontGate},
	} {
		s.Run(name, func() {
			lines := gateClaims()
			lines[0].Advance = advance
			_, err := s.gate(lines, encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
			s.ErrorIs(err, encounter.ErrBadMember)
		})
	}
}

// TestARequirementNamingWhatTheRunLacksIsRefused: a requirement on an
// interaction the run does not have, or on a state its recipe does not
// declare, is refused at setup — a line nobody could ever be offered.
func (s *InteractionSuite) TestARequirementNamingWhatTheRunLacksIsRefused() {
	for name, requires := range map[string]encounter.Trigger{
		"an unknown interaction": encounter.TriggerInteraction{Interaction: "back-gate", In: []string{"fresh"}},
		"an unknown state":       encounter.TriggerInteraction{Interaction: theFrontGate, In: []string{"bribed"}},
		"no state":               encounter.TriggerInteraction{Interaction: theFrontGate},
		"a not of an unknown":    encounter.TriggerNot{Inner: encounter.TriggerInteraction{Interaction: "x", In: []string{"y"}}},
	} {
		s.Run(name, func() {
			lines := gateClaims()
			lines[0].Requires = []encounter.Trigger{requires}
			_, err := s.gate(lines, encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
			s.ErrorIs(err, encounter.ErrBadMember)
		})
	}
}

// TestAnInteractionIsNotSomethingToWaitFor: an ending on an interaction's
// state is refused — it is read by an option, never waited for.
func (s *InteractionSuite) TestAnInteractionIsNotSomethingToWaitFor() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas: openAir(), Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 8, 8)},
			Interactions: []encounter.InteractionInput{{ID: theFrontGate, Recipe: frontGateRecipe()}},
		},
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}}},
		Endings: []encounter.EndingInput{{Key: "in", Trigger: encounter.TriggerInteraction{
			Interaction: theFrontGate, In: []string{"admitted"},
		}}},
		Capabilities: (&OptionTestSuite{}).capabilities(),
	})
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
	s.ErrorContains(err, "only something an option requires")
}

// TestARecipeThatPicksOrWakesOrReadsTheWorldIsRefused: an interaction rolls
// nothing, nobody wakes it, and it reads no fact — each refused at setup,
// as is a recipe with no ref and an id declared twice.
func (s *InteractionSuite) TestARecipeThatPicksOrWakesOrReadsTheWorldIsRefused() {
	for name, change := range map[string]func(*machine.Recipe){
		"a pick": func(r *machine.Recipe) {
			r.States["fresh"].Transitions[0].Do = []machine.Act{{Pick: "time"}}
		},
		"a wake": func(r *machine.Recipe) {
			r.States["fresh"].Transitions[0].Do = []machine.Act{{WakeIn: 2}}
		},
		"a fact": func(r *machine.Recipe) {
			r.States["fresh"].Transitions[0].When = &machine.Guard{Fact: &machine.FactLeaf{
				Name: "enemy", Op: machine.OpEq, Value: true,
			}}
		},
		"no ref": func(r *machine.Recipe) { r.Ref = core.Ref{} },
	} {
		s.Run(name, func() {
			recipe := frontGateRecipe()
			change(&recipe)
			_, err := s.gate(nil, encounter.InteractionInput{ID: theFrontGate, Recipe: recipe})
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
	_, err := s.gate(nil,
		encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()},
		encounter.InteractionInput{ID: theFrontGate, Recipe: frontGateRecipe()})
	s.ErrorIs(err, encounter.ErrNoField, "an id declared twice")

	recipe := frontGateRecipe()
	recipe.Bag = machine.Bag{"warned": false}
	recipe.States["fresh"].Transitions[0].When = &machine.Guard{Not: &machine.Guard{Flag: "warned"}}
	_, err = s.gate(nil, encounter.InteractionInput{ID: theFrontGate, Recipe: recipe})
	s.NoError(err, "a flag is the interaction's own and is allowed")
}

// TestDefinitionAndProgressRoundTrip (Task 3): the definition and the
// progress survive a save and a load; a saved digest that does not match
// its recipe, a recipe edited under its digest, and a state the recipe does
// not declare are each refused with ErrInvalidData.
func (s *InteractionSuite) TestDefinitionAndProgressRoundTrip() {
	enc := s.openGate()
	s.Require().NoError(s.claim(enc, alice, "guest", false))
	data := enc.ToData()

	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	var decoded encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &decoded))

	reloaded, err := s.reloaded(decoded)
	s.Require().NoError(err)
	s.Equal(data.Interactions, reloaded.ToData().Interactions, "definition and progress come back as saved")
	s.Equal([]string{"troupe"}, s.offered(reloaded, billy))

	tampered := []struct {
		name   string
		change func(*encounter.InteractionData)
	}{
		{"a changed digest", func(d *encounter.InteractionData) { d.Definition = "0000" }},
		{"a recipe edited under its digest", func(d *encounter.InteractionData) {
			var recipe machine.Recipe
			s.Require().NoError(json.Unmarshal(d.Recipe, &recipe))
			recipe.Initial = "warned-guest"
			edited, err := json.Marshal(recipe)
			s.Require().NoError(err)
			d.Recipe = edited
		}},
		{"a state the recipe lacks", func(d *encounter.InteractionData) { d.Blob.State = "bribed" }},
	}
	for _, tc := range tampered {
		s.Run(tc.name, func() {
			var copied encounter.EncounterData
			s.Require().NoError(json.Unmarshal(raw, &copied))
			tc.change(&copied.Interactions[0])
			_, err := s.reloaded(copied)
			s.ErrorIs(err, encounter.ErrInvalidData)
		})
	}
}

// TestNoInteractionsWritesNoKey: a run whose content declares no interaction
// writes no `interactions` key at all — the bytes every earlier blob has.
func (s *InteractionSuite) TestNoInteractionsWritesNoKey() {
	enc, err := s.gate([]encounter.Option{plainSelectionLine()})
	s.Require().NoError(err)
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var blob map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &blob))
	s.NotContains(blob, "interactions")

	reloaded, err := s.reloaded(enc.ToData())
	s.Require().NoError(err)
	again, err := json.Marshal(reloaded.ToData())
	s.Require().NoError(err)
	s.NotContains(string(again), `"interactions"`)
}
