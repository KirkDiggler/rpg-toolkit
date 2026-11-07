// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

type sequencePatrolSuite struct{ suite.Suite }

func TestSequencePatrolSuite(t *testing.T) { suite.Run(t, new(sequencePatrolSuite)) }

func (s *sequencePatrolSuite) TestTwoAndThreeLegLoops() {
	definition := sequenceDefinition(s.T(), "patrol")
	for _, tc := range []struct {
		name  string
		items []jsontext.Value
		want  []float64
	}{
		{"West", []jsontext.Value{[]byte(`{"x":0,"y":0}`), []byte(`{"x":4,"y":0}`)}, []float64{0, 4, 0}},
		{"Court", []jsontext.Value{[]byte(`{"x":10,"y":0}`), []byte(`{"x":12,"y":0}`), []byte(`{"x":14,"y":0}`)}, []float64{10, 12, 14, 10}},
	} {
		s.Run(tc.name, func() {
			bound, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{ID: "duty", Definition: definition,
				Items: tc.items, Parameters: map[string]any{"weight": 1}, Vocabulary: sequenceVocabulary()})
			s.Require().NoError(err)
			c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{bound.Instance}, Vocabulary: bound.Vocabulary})
			s.Require().NoError(err)
			die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 100, face: 1}}}
			for round, want := range tc.want {
				out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(round), Now: uint64(round), Facts: behavior.Facts{CanMove: true}, Die: die})
				s.Require().NoError(err)
				s.Equal(want, out.Pick.Answer.Toward.At.X)
				s.Equal("duty", out.Instance)
				s.T().Logf("%s: select x=%g, item=%d, d%d=%d", tc.name, out.Pick.Answer.Toward.At.X, bound.TableSources[out.Pick.Key].Item, out.Pick.Of, out.Pick.Roll)
				_, err = c.Observe(&behavior.ObserveInput{Instance: out.Instance, On: bound.Completions[out.Pick.Key]})
				s.Require().NoError(err)
			}
			s.Equal(len(tc.want), die.used)
		})
	}
}

func (s *sequencePatrolSuite) TestSelectionAndCompletionAreDifferent() {
	west, err := behavior.ExpandSequence(sequenceInput(s.T()))
	s.Require().NoError(err)
	courtInput := sequenceInput(s.T())
	courtInput.Items = []jsontext.Value{[]byte(`{"x":10,"y":0}`), []byte(`{"x":12,"y":0}`), []byte(`{"x":14,"y":0}`)}
	court, err := behavior.ExpandSequence(courtInput)
	s.Require().NoError(err)
	wc, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{west.Instance}, Vocabulary: west.Vocabulary})
	s.Require().NoError(err)
	cc, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{court.Instance}, Vocabulary: court.Vocabulary})
	s.Require().NoError(err)
	westBefore, courtBefore := wc.ToData(), cc.ToData()
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}}}
	first, err := wc.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(0), Facts: behavior.Facts{CanMove: true}, Die: die})
	s.Require().NoError(err)
	s.Equal(westBefore, wc.ToData(), "selection is not arrival")
	completion := west.Completions[first.Pick.Key]
	_, err = wc.Observe(&behavior.ObserveInput{Instance: "somebody-else", On: completion})
	s.ErrorIs(err, behavior.ErrUnknownInstance)
	s.Equal(westBefore, wc.ToData())
	_, err = wc.Observe(&behavior.ObserveInput{Instance: first.Instance, On: completion})
	s.Require().NoError(err)
	s.Equal(courtBefore, cc.ToData(), "Court's same local attachment ID owns different progress")
	after := wc.ToData()
	observed, err := wc.Observe(&behavior.ObserveInput{Instance: first.Instance, On: completion})
	s.Require().NoError(err)
	s.Empty(observed.Steps, "the old step's completion does not match the new step")
	s.Equal(after, wc.ToData())
	second, err := wc.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(1), Facts: behavior.Facts{CanMove: true}, Die: die})
	s.Require().NoError(err)
	s.Equal(float64(4), second.Pick.Answer.Toward.At.X)
	s.T().Log("West completion advances West to x=4; Court unchanged; repeated old-step completion is inert")
}

func (s *sequencePatrolSuite) TestInvestigationResumesTheUnfinishedLeg() {
	for _, tc := range []struct {
		face int
		y    float64
	}{{1, 1}, {101, -1}} {
		in := sequenceInput(s.T())
		in.Vocabulary.Beats["heard"] = machine.BeatSpec{}
		in.Vocabulary.Facts = map[machine.FactName]machine.FactSpec{"heard.kind": {Kind: machine.KindString, Read: func(v any) (any, bool) {
			facts, ok := v.(map[string]any)
			if !ok {
				return nil, false
			}
			kind, present := facts["kind"]
			return kind, present
		}}}
		bound, err := behavior.ExpandSequence(in)
		s.Require().NoError(err)
		raw, err := os.ReadFile("testdata/sequence-investigation.json")
		s.Require().NoError(err)
		var investigate behavior.InstanceSpec
		s.Require().NoError(json.Unmarshal(raw, &investigate))
		c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{investigate, bound.Instance}, Vocabulary: bound.Vocabulary})
		s.Require().NoError(err)
		die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 200, face: tc.face}, {sides: 100, face: 1}}}
		first, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(0), Facts: behavior.Facts{CanMove: true}, Die: die})
		s.Require().NoError(err)
		_, err = c.Observe(&behavior.ObserveInput{Instance: first.Instance, On: bound.Completions[first.Pick.Key]})
		s.Require().NoError(err)
		second, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(1), Now: 1, Facts: behavior.Facts{CanMove: true}, Die: die})
		s.Require().NoError(err)
		s.Equal(float64(4), second.Pick.Answer.Toward.At.X)
		pending := c.ToData().Instances[1]
		_, err = c.Observe(&behavior.ObserveInput{On: "heard", Env: map[string]any{"kind": "clang"}, Now: 1})
		s.Require().NoError(err)
		chosen, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(2), Now: 2, Facts: behavior.Facts{CanMove: true}, Die: die})
		s.Require().NoError(err)
		s.Equal("investigate", chosen.Instance)
		s.Equal(tc.y, chosen.Pick.Answer.Toward.At.Y)
		s.Equal(tc.face, chosen.Pick.Roll)
		s.Equal([]string{"duty"}, chosen.Deferred)
		s.Equal(pending, c.ToData().Instances[1])
		resumed, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(3), Now: 3, Facts: behavior.Facts{CanMove: true}, Die: die})
		s.Require().NoError(err)
		s.Equal("duty", resumed.Instance)
		s.Equal(float64(4), resumed.Pick.Answer.Toward.At.X)
		s.Equal(4, die.used)
		s.T().Logf("d200=%d: investigation target (8,%g), then expanded Patrol resumes unfinished (4,0)", tc.face, tc.y)
	}
}

func (s *sequencePatrolSuite) TestNoBudgetPreservesProgressAndDice() {
	bound, err := behavior.ExpandSequence(sequenceInput(s.T()))
	s.Require().NoError(err)
	c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{bound.Instance}, Vocabulary: bound.Vocabulary})
	s.Require().NoError(err)
	before := c.ToData()
	die := &exactDie{}
	out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(1), Now: 1, Die: die})
	s.Require().NoError(err)
	s.Equal("duty", out.Instance)
	s.True(out.Pick.Answer.Hold)
	s.Equal(-1, out.Pick.Entry)
	s.Zero(die.used)
	s.Equal(before, c.ToData())
	s.T().Log("No movement budget: hold, zero rolls, unchanged continuation")
}

func (s *sequencePatrolSuite) TestReloadKeepsOriginalBindingsAndProgress() {
	for _, tc := range []struct {
		name     string
		items    []jsontext.Value
		complete int
		want     float64
	}{
		{"West", []jsontext.Value{[]byte(`{"x":0,"y":0}`), []byte(`{"x":4,"y":0}`)}, 1, 4},
		{"Court", []jsontext.Value{[]byte(`{"x":10,"y":0}`), []byte(`{"x":12,"y":0}`), []byte(`{"x":14,"y":0}`)}, 2, 14},
	} {
		s.Run(tc.name, func() {
			in := sequenceInput(s.T())
			in.Items = tc.items
			bound, err := behavior.ExpandSequence(in)
			s.Require().NoError(err)
			config := behavior.ComposerConfig{Instances: []behavior.InstanceSpec{bound.Instance}, Vocabulary: bound.Vocabulary}
			c, err := behavior.NewComposer(&config)
			s.Require().NoError(err)
			die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 100, face: 1}}}
			for i := 0; i < tc.complete; i++ {
				out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(i), Facts: behavior.Facts{CanMove: true}, Die: die})
				s.Require().NoError(err)
				_, err = c.Observe(&behavior.ObserveInput{Instance: out.Instance, On: bound.Completions[out.Pick.Key]})
				s.Require().NoError(err)
			}
			saved := struct {
				Instances []behavior.InstanceSpec
				Progress  behavior.ComposerData
			}{config.Instances, c.ToData()}
			raw, err := json.Marshal(saved)
			s.Require().NoError(err)
			var restored struct {
				Instances []behavior.InstanceSpec
				Progress  behavior.ComposerData
			}
			s.Require().NoError(json.Unmarshal(raw, &restored))
			in.Definition.Body = []byte(`invalid subsequently edited source`)
			loaded, err := behavior.LoadComposer(&behavior.LoadComposerInput{Config: behavior.ComposerConfig{Instances: restored.Instances, Vocabulary: config.Vocabulary}, Data: restored.Progress})
			s.Require().NoError(err)
			s.Equal(tc.complete, die.used, "load neither re-expands nor rolls")
			out, err := loaded.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(10), Facts: behavior.Facts{CanMove: true}, Die: die})
			s.Require().NoError(err)
			s.Equal(tc.want, out.Pick.Answer.Toward.At.X)
			restored.Instances[0].Table[out.Pick.Key][0].Toward.At.X = 99
			_, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: behavior.ComposerConfig{Instances: restored.Instances, Vocabulary: config.Vocabulary}, Data: restored.Progress})
			s.ErrorIs(err, behavior.ErrBadComposerData)
			s.T().Logf("%s reload resumes x=%g; changed configuration refused", tc.name, tc.want)
		})
	}
}

func (s *sequencePatrolSuite) TestAnotherBodyUsesTheSameProducer() {
	bound, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{ID: "announcements", Definition: sequenceDefinition(s.T(), "lines"), Items: []jsontext.Value{[]byte(`"alpha"`), []byte(`"beta"`)}, Vocabulary: sequenceVocabulary()})
	s.Require().NoError(err)
	c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{bound.Instance}, Vocabulary: bound.Vocabulary})
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}}}
	for _, want := range []string{"alpha", "beta"} {
		out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(0), Die: die})
		s.Require().NoError(err)
		s.Equal(want, out.Pick.Answer.Say)
		_, err = c.Observe(&behavior.ObserveInput{Instance: out.Instance, On: bound.Completions[out.Pick.Key]})
		s.Require().NoError(err)
	}
	out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(1), Die: die})
	s.Require().NoError(err)
	s.Nil(out.Pick)
	s.Empty(out.Instance)
	s.Equal(2, die.used)
	s.T().Log("Same expander: alpha → beta → terminal, with no movement word or Patrol branch")
}
