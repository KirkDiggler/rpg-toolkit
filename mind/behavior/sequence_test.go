// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// Helpers only load authored data and supply values. Production expands it.
func sequenceDefinition(t *testing.T, file string) behavior.SequenceDefinition {
	t.Helper()
	raw, err := os.ReadFile("testdata/sequence-" + file + ".json")
	require.NoError(t, err)
	var def behavior.SequenceDefinition
	require.NoError(t, json.Unmarshal(raw, &def))
	return def
}

func sequenceVocabulary() machine.Vocabulary {
	return machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{
		"time": {Timed: true, Round: func(v any) (uint64, bool) { n, ok := v.(uint64); return n, ok }},
	}}
}

func sequenceInput(t *testing.T) *behavior.ExpandSequenceInput {
	t.Helper()
	return &behavior.ExpandSequenceInput{
		ID: "duty", Definition: sequenceDefinition(t, "patrol"),
		Items:      []jsontext.Value{[]byte(`{"x":0,"y":0}`), []byte(`{"x":4,"y":0}`)},
		Parameters: map[string]any{"weight": 1}, Vocabulary: sequenceVocabulary(),
	}
}

type sequenceSuite struct{ suite.Suite }

func TestSequenceSuite(t *testing.T) { suite.Run(t, new(sequenceSuite)) }

func (s *sequenceSuite) TestExpandsDifferentLengthsFromOneDefinition() {
	in := sequenceInput(s.T())
	before := append([]byte(nil), in.Definition.Body...)
	west, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	in.Items = []jsontext.Value{[]byte(`{"x":10,"y":0}`), []byte(`{"x":12,"y":0}`), []byte(`{"x":14,"y":0}`)}
	court, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	s.Equal(before, []byte(in.Definition.Body))
	for _, tc := range []struct {
		name  string
		bound *behavior.ExpandSequenceOutput
		want  float64
	}{
		{"west", west, 0}, {"court", court, 10},
	} {
		s.Run(tc.name, func() {
			c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{tc.bound.Instance}, Vocabulary: tc.bound.Vocabulary})
			s.Require().NoError(err)
			die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
			out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(0), Facts: behavior.Facts{CanMove: true}, Die: die})
			s.Require().NoError(err)
			s.Equal(tc.want, out.Pick.Answer.Toward.At.X)
			s.Equal(behavior.SequenceSource{Item: 0, Name: "move"}, tc.bound.TableSources[out.Pick.Key])
			s.Equal(behavior.SequenceSource{Item: 0, Name: "walk"}, tc.bound.StateSources[c.ToData().Instances[0].Blob.State])
			s.NotEmpty(tc.bound.Completions[out.Pick.Key])
		})
	}
}

func (s *sequenceSuite) TestParametersUseExistingMechanicalChecks() {
	for _, values := range []map[string]any{nil, {"weight": 0}, {"weight": 4}, {"weight": 1.0}, {"weight": 1, "typo": 2}} {
		in := sequenceInput(s.T())
		in.Parameters = values
		out, err := behavior.ExpandSequence(in)
		s.Nil(out)
		s.ErrorIs(err, behavior.ErrBadSequence)
		s.ErrorIs(err, behavior.ErrBadParameter)
	}
	in := sequenceInput(s.T())
	in.Definition.Parameters[0].Required = false
	in.Parameters = nil
	out, err := behavior.ExpandSequence(in)
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrBadSequence)
	s.ErrorContains(err, "weight")
}

func (s *sequenceSuite) TestRefusesInvalidSourceWithoutMutation() {
	out, err := behavior.ExpandSequence(nil)
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrNilInput)
	for _, tc := range []struct {
		name   string
		change func(*behavior.ExpandSequenceInput)
	}{
		{"empty id", func(in *behavior.ExpandSequenceInput) { in.ID = "" }},
		{"empty ref", func(in *behavior.ExpandSequenceInput) { in.Definition.Ref.ID = "" }},
		{"missing end", func(in *behavior.ExpandSequenceInput) { in.Definition.End = "" }},
		{"unknown end", func(in *behavior.ExpandSequenceInput) { in.Definition.End = "guess" }},
		{"no items", func(in *behavior.ExpandSequenceInput) { in.Items = nil }},
		{"invalid item", func(in *behavior.ExpandSequenceInput) { in.Items[0] = []byte(`{`) }},
		{"invalid body", func(in *behavior.ExpandSequenceInput) { in.Definition.Body = []byte(`{`) }},
		{"missing body", func(in *behavior.ExpandSequenceInput) { in.Definition.Body = []byte(`{}`) }},
		{"wrong target type", func(in *behavior.ExpandSequenceInput) { in.Items[0] = []byte(`"not a point"`) }},
	} {
		s.Run(tc.name, func() {
			in := sequenceInput(s.T())
			tc.change(in)
			body := append([]byte(nil), in.Definition.Body...)
			beforeBeats := len(in.Vocabulary.Beats)
			out, err := behavior.ExpandSequence(in)
			s.Nil(out)
			s.ErrorIs(err, behavior.ErrBadSequence)
			s.Equal(body, []byte(in.Definition.Body))
			s.Len(in.Vocabulary.Beats, beforeBeats)
		})
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"unknown marker", `"$item": true`, `"$execute": true`},
		{"false item marker", `"$item": true`, `"$item": false`},
		{"mixed marker", `"$item": true`, `"$item": true, "other": 1`},
		{"unknown parameter", `"$param": "weight"`, `"$param": "missing"`},
		{"unknown completion table", `"$done": "move"`, `"$done": "absent"`},
		{"unknown destination", `{"$next": true}`, `"absent"`},
		{"unknown pick", `"pick": "move"`, `"pick": "absent"`},
		{"unknown beat", `"on": "time"`, `"on": "absent"`},
		{"unknown body field", `"recipe": {`, `"extra": true, "recipe": {`},
		{"unknown recipe field", `"initial": "walk"`, `"invented": true, "initial": "walk"`},
		{"duplicate identity", `"initial": "walk"`, `"ref": "example:behaviors:other", "initial": "walk"`},
	} {
		s.Run(tc.name, func() {
			in := sequenceInput(s.T())
			s.Require().Contains(string(in.Definition.Body), tc.old)
			in.Definition.Body = []byte(strings.Replace(string(in.Definition.Body), tc.old, tc.replacement, 1))
			out, err := behavior.ExpandSequence(in)
			s.Nil(out)
			s.ErrorIs(err, behavior.ErrBadSequence)
		})
	}
}

func (s *sequenceSuite) TestOutputsAreDetachedAndDeterministic() {
	in := sequenceInput(s.T())
	first, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	second, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	s.Equal(first.Instance, second.Instance)
	s.Equal(first.Completions, second.Completions)
	for key := range first.Instance.Table {
		first.Instance.Table[key][0].Toward.At.X = 99
		s.NotEqual(float64(99), second.Instance.Table[key][0].Toward.At.X)
	}
	for _, beat := range first.Completions {
		_, exists := in.Vocabulary.Beats[beat]
		s.False(exists)
		in.Vocabulary.Beats[beat] = machine.BeatSpec{}
		break
	}
	out, err := behavior.ExpandSequence(in)
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrBadSequence)
	s.ErrorContains(err, "collision")
}

func (s *sequenceSuite) TestNextReferenceCannotBeCapturedByLocalName() {
	in := &behavior.ExpandSequenceInput{ID: "lines", Definition: sequenceDefinition(s.T(), "lines"),
		Items: []jsontext.Value{[]byte(`"alpha"`), []byte(`"beta"`)}, Vocabulary: sequenceVocabulary()}
	in.Vocabulary.Beats["visit-local"] = machine.BeatSpec{}
	in.Definition.Body = []byte(`{
		"recipe":{"initial":"speak","states":{
			"speak":{"transitions":[
				{"on":"time","do":[{"pick":"line"}]},
				{"on":{"$done":"line"},"to":{"$next":true}},
				{"on":"visit-local","to":"item/1/speak"}
			]},
			"item/1/speak":{"transitions":[{"on":"time","do":[{"pick":"trap"}]}]}
		}},
		"table":{"line":[{"Weight":1,"Say":{"$item":true}}],"trap":[{"Weight":1,"Say":"captured"}]}
	}`)
	bound, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	for _, tc := range []struct{ event, want string }{{"complete", "beta"}, {"visit-local", "captured"}} {
		c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{bound.Instance}, Vocabulary: bound.Vocabulary})
		s.Require().NoError(err)
		die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}}}
		first, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(0), Die: die})
		s.Require().NoError(err)
		on := machine.Beat(tc.event)
		if tc.event == "complete" {
			on = bound.Completions[first.Pick.Key]
		}
		_, err = c.Observe(&behavior.ObserveInput{Instance: "lines", On: on})
		s.Require().NoError(err)
		out, err := c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(1), Die: die})
		s.Require().NoError(err)
		s.Equal(tc.want, out.Pick.Answer.Say, "explicit next and literal local references have different identities")
	}
}

func (s *sequenceSuite) TestLocalBagReferencesAreScoped() {
	in := &behavior.ExpandSequenceInput{ID: "lines", Definition: sequenceDefinition(s.T(), "lines"),
		Items: []jsontext.Value{[]byte(`"alpha"`), []byte(`"beta"`)}, Vocabulary: sequenceVocabulary()}
	in.Definition.End = behavior.SequenceRepeat
	in.Vocabulary.Beats["reset"] = machine.BeatSpec{}
	in.Definition.Body = []byte(`{
		"recipe":{"initial":"work","bag":{"seen":false},"states":{"work":{"transitions":[
			{"on":"time","when":{"not":{"any":[{"flag":"seen"}]}},"do":[{"set":{"key":"seen","value":true}},{"pick":"line"}]},
			{"on":"time","when":{"all":[{"flag":"seen"}]},"do":[{"pick":"repeat"}]},
			{"on":"reset","do":[{"clear":["seen"]}]},
			{"on":{"$done":"line"},"to":{"$next":true}},
			{"on":{"$done":"repeat"},"to":{"$next":true}}
		]}}},
		"table":{"line":[{"Weight":1,"Say":{"$item":true}}],"repeat":[{"Weight":1,"Say":"already"}]}
	}`)
	bound, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	c, err := behavior.NewComposer(&behavior.ComposerConfig{Instances: []behavior.InstanceSpec{bound.Instance}, Vocabulary: bound.Vocabulary})
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 100, face: 1}, {sides: 100, face: 1}}}
	var last *behavior.DecideOutput
	for _, step := range []struct{ before, want string }{
		{"", "alpha"}, {"", "already"}, {"reset", "alpha"}, {"complete", "beta"}, {"complete", "already"},
	} {
		if step.before != "" {
			on := machine.Beat("reset")
			if step.before == "complete" {
				on = bound.Completions[last.Pick.Key]
			}
			_, err = c.Observe(&behavior.ObserveInput{Instance: "lines", On: on})
			s.Require().NoError(err)
		}
		last, err = c.Decide(context.Background(), &behavior.DecideInput{On: "time", Event: uint64(0), Die: die})
		s.Require().NoError(err)
		s.Equal(step.want, last.Pick.Answer.Say)
	}
}

func (s *sequenceSuite) TestInsertedValuesAndStringsStayLiteral() {
	in := &behavior.ExpandSequenceInput{ID: "announcements", Definition: sequenceDefinition(s.T(), "lines"),
		Items: []jsontext.Value{[]byte(`"$item"`), []byte(`"prefix $param suffix"`)}, Vocabulary: sequenceVocabulary()}
	out, err := behavior.ExpandSequence(in)
	s.Require().NoError(err)
	for key, source := range out.TableSources {
		want := []string{"$item", "prefix $param suffix"}[source.Item]
		s.Equal(want, out.Instance.Table[key][0].Say)
	}
}
