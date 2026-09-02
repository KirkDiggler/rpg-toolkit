// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	activationFighter encounter.MemberID = "fighter"
	activationCleric  encounter.MemberID = "cleric"
	activationGoblin  encounter.MemberID = "goblin-activation"
)

// RecordActivationSuite pins the encounter-owned transaction that turns one
// successful rulebook activation and its ordered results into durable story.
type RecordActivationSuite struct {
	suite.Suite
}

type countingFailingStanding struct {
	calls  int
	broken bool
}

func (s *countingFailingStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	s.calls++
	if s.broken {
		return nil, errRulebookUnreachable
	}
	return nil, nil
}

func TestRecordActivationSuite(t *testing.T) {
	suite.Run(t, new(RecordActivationSuite))
}

// scene keeps the monster behind a wall so first light does not form a fight;
// activation tests need only a roster, a record, and an observable Standing
// capability.
func (s *RecordActivationSuite) scene(standing encounter.Standing) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Retention: encounter.RetentionUnbounded,
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("activation-yard", 0, 0, 12, 12)},
			Props:   wallRow(6, 4, 8),
		},
		Members: []encounter.MemberInput{
			{ID: activationFighter, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: activationCleric, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 2}},
			{ID: activationGoblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *RecordActivationSuite) storyEntries(enc *encounter.Encounter, who encounter.MemberID, seqs []uint64) []record.Entry {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	s.Require().NoError(err)

	bySeq := make(map[uint64]record.Entry, len(story))
	for _, entry := range story {
		bySeq[entry.Seq] = entry
	}
	out := make([]record.Entry, 0, len(seqs))
	for _, seq := range seqs {
		entry, ok := bySeq[seq]
		s.Require().True(ok, "%s did not receive seq %d", who, seq)
		out = append(out, entry)
	}
	return out
}

func validHealingResult() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind:      encounter.ResultHealingApplied,
		Target:    activationFighter,
		Ref:       "dnd5e:features:second_wind",
		Name:      "Second Wind",
		Amount:    2,
		Requested: 7,
		Roll:      6,
		Modifier:  1,
		Before:    8,
		After:     10,
	}
}

func validActivationInput() *encounter.RecordActivationInput {
	return &encounter.RecordActivationInput{
		Actor:   activationFighter,
		Ability: encounter.ActivationIdentity{Ref: "dnd5e:features:second_wind", Name: "Second Wind"},
		Results: []encounter.ActivationResult{validHealingResult()},
	}
}

// TestRecordActivationSecondWind pins exact payloads, sequence order, outcome
// tagging, and the current full-roster audience policy in one representative
// transaction.
func (s *RecordActivationSuite) TestRecordActivationSecondWind() {
	standing := &countingStanding{}
	enc := s.scene(standing)
	callsBefore := standing.calls

	out, err := enc.RecordActivation(validActivationInput())
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 2)
	s.Less(out.Seqs[0], out.Seqs[1], "activation precedes its result")
	s.Nil(out.IntelDeltas)
	s.Equal(callsBefore+1, standing.calls, "noticeDown is consulted once for the whole transaction")

	wantPayloads := []string{
		`{"beat":"activated","actor":"fighter","ability":{"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`,
		`{"beat":"activation-result","actor":"fighter","result":{"kind":"healing-applied","target":"fighter","amount":2,"requested":7,"roll":6,"modifier":1,"before":8,"after":10,"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`,
	}
	wantAudience := []string{"cleric", "fighter", "goblin-activation"}
	for _, who := range []encounter.MemberID{activationFighter, activationCleric, activationGoblin} {
		entries := s.storyEntries(enc, who, out.Seqs)
		s.Require().Len(entries, 2)
		for i, entry := range entries {
			s.Equal(wantPayloads[i], string(entry.Payload), "%s payload at transaction offset %d", who, i)
			s.Equal("outcome", entry.Tags["tag"])
			gotAudience := make([]string, 0, len(entry.Audience))
			for _, id := range entry.Audience {
				gotAudience = append(gotAudience, string(id))
			}
			s.Equal(wantAudience, gotAudience, "the persisted audience is today's full sorted roster")
		}
	}
}

// TestRecordActivationMultiResultOrder proves one result beat per effect in
// caller-supplied order and covers every member of the closed result family.
func (s *RecordActivationSuite) TestRecordActivationMultiResultOrder() {
	enc := s.scene(everyoneStanding{})
	in := &encounter.RecordActivationInput{
		Actor:   activationFighter,
		Target:  activationCleric,
		Ability: encounter.ActivationIdentity{Ref: "dnd5e:features:many-effects", Name: "Many Effects"},
		Results: []encounter.ActivationResult{
			{Kind: encounter.ResultConditionApplied, Target: activationFighter, Ref: "dnd5e:conditions:raging", Name: "Raging"},
			{Kind: encounter.ResultCapacityGranted, Target: activationFighter, Description: "30ft movement"},
			{Kind: encounter.ResultConditionRemoved, Target: activationCleric, Ref: "dnd5e:conditions:helped", Name: "Helped", Reason: "expired"},
			{Kind: encounter.ResultHealingApplied, Target: activationCleric, Ref: "dnd5e:features:many-effects", Name: "Many Effects", Amount: 3, Requested: 3, Roll: 1, Modifier: 2, Before: 4, After: 7},
		},
	}

	out, err := enc.RecordActivation(in)
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 5)
	for i := 1; i < len(out.Seqs); i++ {
		s.Less(out.Seqs[i-1], out.Seqs[i])
	}

	entries := s.storyEntries(enc, activationCleric, out.Seqs)
	var activated struct {
		Beat   string `json:"beat"`
		Target string `json:"target"`
	}
	s.Require().NoError(json.Unmarshal(entries[0].Payload, &activated))
	s.Equal("activated", activated.Beat)
	s.Equal("cleric", activated.Target, "the selected target stays on the activation beat")

	wantKinds := []encounter.ActivationResultKind{
		encounter.ResultConditionApplied,
		encounter.ResultCapacityGranted,
		encounter.ResultConditionRemoved,
		encounter.ResultHealingApplied,
	}
	for i, want := range wantKinds {
		var beat struct {
			Beat   string `json:"beat"`
			Result struct {
				Kind encounter.ActivationResultKind `json:"kind"`
			} `json:"result"`
		}
		s.Require().NoError(json.Unmarshal(entries[i+1].Payload, &beat))
		s.Equal("activation-result", beat.Beat)
		s.Equal(want, beat.Result.Kind, "result %d kept synchronous effect order", i)
	}

	s.JSONEq(`{"beat":"activation-result","actor":"fighter","result":{"kind":"condition-applied","target":"fighter","ref":"dnd5e:conditions:raging","name":"Raging"}}`, string(entries[1].Payload))
	s.JSONEq(`{"beat":"activation-result","actor":"fighter","result":{"kind":"capacity-granted","target":"fighter","description":"30ft movement"}}`, string(entries[2].Payload))
	s.JSONEq(`{"beat":"activation-result","actor":"fighter","result":{"kind":"condition-removed","target":"cleric","ref":"dnd5e:conditions:helped","name":"Helped","reason":"expired"}}`, string(entries[3].Payload))
}

// TestRecordActivationNoResults still records the successful use and consults
// consequences exactly once; an empty result list is an ordinary activation.
func (s *RecordActivationSuite) TestRecordActivationNoResults() {
	standing := &countingStanding{}
	enc := s.scene(standing)
	callsBefore := standing.calls

	out, err := enc.RecordActivation(&encounter.RecordActivationInput{
		Actor:   activationFighter,
		Ability: encounter.ActivationIdentity{Ref: "dnd5e:combat-abilities:dodge", Name: "Dodge"},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 1)
	s.Equal(callsBefore+1, standing.calls)

	entries := s.storyEntries(enc, activationGoblin, out.Seqs)
	s.Equal(`{"beat":"activated","actor":"fighter","ability":{"ref":"dnd5e:combat-abilities:dodge","name":"Dodge"}}`, string(entries[0].Payload))
}

// TestRecordActivationHealingZeroFactsAreStored inspects the stored bytes to
// prove all six numeric healing facts survive even when every value is zero.
// Decoding into ints would not distinguish an explicit zero from an omitted
// field, so this exact-byte assertion protects every numeric payload tag from
// acquiring omitempty.
func (s *RecordActivationSuite) TestRecordActivationHealingZeroFactsAreStored() {
	enc := s.scene(everyoneStanding{})
	in := validActivationInput()
	in.Results[0].Amount = 0
	in.Results[0].Requested = 0
	in.Results[0].Roll = 0
	in.Results[0].Modifier = 0
	in.Results[0].Before = 0
	in.Results[0].After = 0

	out, err := enc.RecordActivation(in)
	s.Require().NoError(err)
	entries := s.storyEntries(enc, activationFighter, out.Seqs)
	s.Require().Len(entries, 2)
	s.Equal(
		`{"beat":"activation-result","actor":"fighter","result":{"kind":"healing-applied","target":"fighter","amount":0,"requested":0,"roll":0,"modifier":0,"before":0,"after":0,"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`,
		string(entries[1].Payload),
	)
}

// TestRecordActivationPreservesRulebookArithmetic pins the boundary: encounter
// validates shape, not D&D arithmetic. Inconsistent and negative numbers remain
// the exact facts supplied by the rulebook.
func (s *RecordActivationSuite) TestRecordActivationPreservesRulebookArithmetic() {
	enc := s.scene(everyoneStanding{})
	in := validActivationInput()
	in.Results[0].Amount = -4
	in.Results[0].Requested = 2
	in.Results[0].Roll = 99
	in.Results[0].Modifier = -101
	in.Results[0].Before = 4
	in.Results[0].After = -20

	out, err := enc.RecordActivation(in)
	s.Require().NoError(err)
	entries := s.storyEntries(enc, activationFighter, out.Seqs)
	s.JSONEq(`{"beat":"activation-result","actor":"fighter","result":{"kind":"healing-applied","target":"fighter","amount":-4,"requested":2,"roll":99,"modifier":-101,"before":4,"after":-20,"ref":"dnd5e:features:second_wind","name":"Second Wind"}}`, string(entries[1].Payload))
}

// TestRecordActivationPayloadIsDeterministic compares the stored bytes, not
// only decoded JSON: identical inputs must produce identical transcript bytes.
func (s *RecordActivationSuite) TestRecordActivationPayloadIsDeterministic() {
	recordPayloads := func() []string {
		enc := s.scene(everyoneStanding{})
		out, err := enc.RecordActivation(validActivationInput())
		s.Require().NoError(err)
		entries := s.storyEntries(enc, activationFighter, out.Seqs)
		payloads := make([]string, 0, len(entries))
		for _, entry := range entries {
			payloads = append(payloads, string(entry.Payload))
		}
		return payloads
	}

	s.Equal(recordPayloads(), recordPayloads())
}

// TestRecordActivationValidationBeforeAppend is the closed validation matrix.
// Every rejection compares the complete log (including next_seq) and Standing
// call count, so it cannot pass after an activation or early result was appended.
func (s *RecordActivationSuite) TestRecordActivationValidationBeforeAppend() {
	type testCase struct {
		name    string
		input   *encounter.RecordActivationInput
		wantErr error
	}
	var cases []testCase
	add := func(name string, mutate func(*encounter.RecordActivationInput), wantErr error) {
		in := validActivationInput()
		mutate(in)
		cases = append(cases, testCase{name: name, input: in, wantErr: wantErr})
	}

	cases = append(cases, testCase{name: "nil input", wantErr: encounter.ErrNilInput})
	add("empty actor", func(in *encounter.RecordActivationInput) { in.Actor = "" }, encounter.ErrNoMember)
	add("unknown actor", func(in *encounter.RecordActivationInput) { in.Actor = "ghost" }, encounter.ErrNoMember)
	add("unknown selected target", func(in *encounter.RecordActivationInput) { in.Target = "ghost" }, encounter.ErrNoMember)
	add("empty ability ref", func(in *encounter.RecordActivationInput) { in.Ability.Ref = "" }, encounter.ErrInvalidData)
	add("empty ability name", func(in *encounter.RecordActivationInput) { in.Ability.Name = "" }, encounter.ErrInvalidData)
	add("unknown result kind", func(in *encounter.RecordActivationInput) { in.Results[0].Kind = "teleported" }, encounter.ErrInvalidData)
	add("empty result target", func(in *encounter.RecordActivationInput) { in.Results[0].Target = "" }, encounter.ErrNoMember)
	add("unknown result target", func(in *encounter.RecordActivationInput) { in.Results[0].Target = "ghost" }, encounter.ErrNoMember)
	add("healing missing ref", func(in *encounter.RecordActivationInput) { in.Results[0].Ref = "" }, encounter.ErrInvalidData)
	add("healing missing name", func(in *encounter.RecordActivationInput) { in.Results[0].Name = "" }, encounter.ErrInvalidData)
	add("healing forbids description", func(in *encounter.RecordActivationInput) { in.Results[0].Description = "not healing data" }, encounter.ErrInvalidData)
	add("healing forbids reason", func(in *encounter.RecordActivationInput) { in.Results[0].Reason = "not healing data" }, encounter.ErrInvalidData)

	conditionApplied := func() encounter.ActivationResult {
		return encounter.ActivationResult{Kind: encounter.ResultConditionApplied, Target: activationFighter, Ref: "dnd5e:conditions:raging", Name: "Raging"}
	}
	add("condition applied missing ref", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{conditionApplied()}
		in.Results[0].Ref = ""
	}, encounter.ErrInvalidData)
	add("condition applied missing name", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{conditionApplied()}
		in.Results[0].Name = ""
	}, encounter.ErrInvalidData)
	for _, forbidden := range []struct {
		name string
		set  func(*encounter.ActivationResult)
	}{
		{"amount", func(r *encounter.ActivationResult) { r.Amount = 1 }},
		{"requested", func(r *encounter.ActivationResult) { r.Requested = 1 }},
		{"roll", func(r *encounter.ActivationResult) { r.Roll = 1 }},
		{"modifier", func(r *encounter.ActivationResult) { r.Modifier = 1 }},
		{"before", func(r *encounter.ActivationResult) { r.Before = 1 }},
		{"after", func(r *encounter.ActivationResult) { r.After = 1 }},
		{"description", func(r *encounter.ActivationResult) { r.Description = "unexpected" }},
		{"reason", func(r *encounter.ActivationResult) { r.Reason = "unexpected" }},
	} {
		forbidden := forbidden
		add("condition applied forbids "+forbidden.name, func(in *encounter.RecordActivationInput) {
			in.Results = []encounter.ActivationResult{conditionApplied()}
			forbidden.set(&in.Results[0])
		}, encounter.ErrInvalidData)
	}

	conditionRemoved := func() encounter.ActivationResult {
		return encounter.ActivationResult{Kind: encounter.ResultConditionRemoved, Target: activationFighter, Ref: "dnd5e:conditions:raging", Name: "Raging", Reason: "expired"}
	}
	add("condition removed missing ref", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{conditionRemoved()}
		in.Results[0].Ref = ""
	}, encounter.ErrInvalidData)
	add("condition removed missing name", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{conditionRemoved()}
		in.Results[0].Name = ""
	}, encounter.ErrInvalidData)
	add("condition removed missing reason", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{conditionRemoved()}
		in.Results[0].Reason = ""
	}, encounter.ErrInvalidData)
	for _, forbidden := range []struct {
		name string
		set  func(*encounter.ActivationResult)
	}{
		{"amount", func(r *encounter.ActivationResult) { r.Amount = 1 }},
		{"requested", func(r *encounter.ActivationResult) { r.Requested = 1 }},
		{"roll", func(r *encounter.ActivationResult) { r.Roll = 1 }},
		{"modifier", func(r *encounter.ActivationResult) { r.Modifier = 1 }},
		{"before", func(r *encounter.ActivationResult) { r.Before = 1 }},
		{"after", func(r *encounter.ActivationResult) { r.After = 1 }},
		{"description", func(r *encounter.ActivationResult) { r.Description = "unexpected" }},
	} {
		forbidden := forbidden
		add("condition removed forbids "+forbidden.name, func(in *encounter.RecordActivationInput) {
			in.Results = []encounter.ActivationResult{conditionRemoved()}
			forbidden.set(&in.Results[0])
		}, encounter.ErrInvalidData)
	}

	capacityGranted := func() encounter.ActivationResult {
		return encounter.ActivationResult{Kind: encounter.ResultCapacityGranted, Target: activationFighter, Description: "30ft movement"}
	}
	add("capacity granted missing description", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{capacityGranted()}
		in.Results[0].Description = ""
	}, encounter.ErrInvalidData)
	for _, forbidden := range []struct {
		name string
		set  func(*encounter.ActivationResult)
	}{
		{"ref", func(r *encounter.ActivationResult) { r.Ref = "unexpected" }},
		{"name", func(r *encounter.ActivationResult) { r.Name = "unexpected" }},
		{"amount", func(r *encounter.ActivationResult) { r.Amount = 1 }},
		{"requested", func(r *encounter.ActivationResult) { r.Requested = 1 }},
		{"roll", func(r *encounter.ActivationResult) { r.Roll = 1 }},
		{"modifier", func(r *encounter.ActivationResult) { r.Modifier = 1 }},
		{"before", func(r *encounter.ActivationResult) { r.Before = 1 }},
		{"after", func(r *encounter.ActivationResult) { r.After = 1 }},
		{"reason", func(r *encounter.ActivationResult) { r.Reason = "unexpected" }},
	} {
		forbidden := forbidden
		add("capacity granted forbids "+forbidden.name, func(in *encounter.RecordActivationInput) {
			in.Results = []encounter.ActivationResult{capacityGranted()}
			forbidden.set(&in.Results[0])
		}, encounter.ErrInvalidData)
	}

	add("late invalid result after valid result", func(in *encounter.RecordActivationInput) {
		in.Results = []encounter.ActivationResult{conditionApplied(), capacityGranted()}
		in.Results[1].Description = ""
	}, encounter.ErrInvalidData)

	for _, tc := range cases {
		tc := tc
		s.Run(tc.name, func() {
			standing := &countingStanding{}
			enc := s.scene(standing)
			beforeLog := enc.WorldView().Log
			callsBefore := standing.calls

			_, err := enc.RecordActivation(tc.input)

			s.Require().ErrorIs(err, tc.wantErr)
			s.Equal(beforeLog, enc.WorldView().Log, "validation must leave entries and next_seq untouched")
			s.Equal(callsBefore, standing.calls, "a rejected transaction never consults consequences")
		})
	}
}

func (s *RecordActivationSuite) TestRecordActivationClosedEncounterAppendsNothing() {
	standing := &countingStanding{}
	enc := s.scene(standing)
	_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)
	beforeLog := enc.WorldView().Log
	callsBefore := standing.calls

	_, err = enc.RecordActivation(validActivationInput())

	s.Require().ErrorIs(err, encounter.ErrClosed)
	s.Equal(beforeLog, enc.WorldView().Log)
	s.Equal(callsBefore, standing.calls)
}

// TestRecordActivationNoticeDownFailure pins doc.go's post-append posture: all
// transaction beats remain in memory, no output is returned, the Standing
// error is preserved, and consequences were consulted once rather than once
// per beat.
func (s *RecordActivationSuite) TestRecordActivationNoticeDownFailure() {
	standing := &countingFailingStanding{}
	enc := s.scene(standing)
	before := enc.WorldView().Log
	callsBefore := standing.calls
	standing.broken = true

	out, err := enc.RecordActivation(validActivationInput())

	s.Nil(out)
	s.Require().ErrorIs(err, errRulebookUnreachable)
	s.Equal(callsBefore+1, standing.calls, "a failing noticeDown is still one transaction-level consult")
	after := enc.WorldView().Log
	s.Equal(len(before.Entries)+2, len(after.Entries), "activation and result append before noticeDown")
	s.Equal(before.NextSeq+2, after.NextSeq)
	last := after.Entries[len(after.Entries)-2:]
	s.Equal([]uint64{before.NextSeq, before.NextSeq + 1}, []uint64{last[0].Seq, last[1].Seq})
}

// TestRecordActivationClosedShapes keeps this provider limited to the brief's
// primitive carrier rather than importing or embedding root D&D event types.
func (s *RecordActivationSuite) TestRecordActivationClosedShapes() {
	s.Equal([]string{"Ref", "Name"}, structFieldNames(encounter.ActivationIdentity{}))
	s.Equal([]string{"Kind", "Target", "Ref", "Name", "Amount", "Requested", "Roll", "Modifier", "Before", "After", "Description", "Reason"}, structFieldNames(encounter.ActivationResult{}))
	s.Equal([]string{"Actor", "Target", "Ability", "Results"}, structFieldNames(encounter.RecordActivationInput{}))
	s.Equal([]string{"Seqs", "IntelDeltas"}, structFieldNames(encounter.RecordActivationOutput{}))
}
