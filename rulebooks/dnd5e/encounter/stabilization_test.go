// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func stabilizedResult() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind: encounter.ResultStabilized, Target: activationFighter,
		Ref: "dnd5e:spells:spare-the-dying", Name: "Spare the Dying",
		Stabilization: &encounter.StabilizationDetail{
			Before: "dying", After: "stabilized", HitPoints: 0,
			Successes: 0, Failures: 0, SuccessesNeeded: 3, FailuresRemaining: 3,
			Stabilized: true, Dead: false,
		},
	}
}

func (s *RecordActivationSuite) TestStabilizationCastSurvivesReloadWithoutInventedHealingOrRoll() {
	for _, beforeState := range []string{"dying", "stabilized"} {
		s.Run(beforeState, func() {
			standing := &countingStanding{}
			enc := s.scene(standing)
			callsBefore := standing.calls
			worldBefore := enc.ToData()
			result := stabilizedResult()
			result.Stabilization.Before = beforeState
			out, err := enc.RecordCast(&encounter.RecordCastInput{
				Actor:   activationCleric,
				Spell:   encounter.SpellIdentity{Ref: result.Ref, Name: result.Name},
				Targets: []encounter.CastTargetResult{{Target: activationFighter, Results: []encounter.ActivationResult{result}}},
			})
			s.Require().NoError(err)
			s.Require().Len(out.Seqs, 2, "cast and stabilization only")
			s.Less(out.Seqs[0], out.Seqs[1])
			s.Equal(callsBefore+3, standing.calls,
				"one consult for the transaction itself, and two more for the round of the world it pays for (rpg-project#465): the percept rebuild and the notice pass")
			// THE CLOCK MOVED, and that is the point of the slice this
			// assertion was written before (rpg-project#465): a cast is an
			// action, and an action pays one round of the world clock for
			// its actor. What must NOT have changed is everything else.
			s.Equal(worldBefore.Clock.HighWater+1, enc.ToData().Clock.HighWater,
				"the cast paid the world a round")
			s.Equal(worldBefore.Bubbles, enc.ToData().Bubbles)
			s.Equal(worldBefore.Members, enc.ToData().Members)

			entries := s.storyEntries(enc, activationCleric, out.Seqs)
			s.JSONEq(`{"beat":"cast","actor":"cleric","spell":{"ref":"dnd5e:spells:spare-the-dying","name":"Spare the Dying"},"targets":["fighter"]}`, string(entries[0].Payload))
			s.JSONEq(`{"beat":"activation-result","actor":"cleric","result":{"kind":"stabilized","target":"fighter","ref":"dnd5e:spells:spare-the-dying","name":"Spare the Dying","stabilization":{"before":"`+beforeState+`","after":"stabilized","hit_points":0,"successes":0,"failures":0,"successes_needed":3,"failures_remaining":3,"stabilized":true,"dead":false}}}`, string(entries[1].Payload))
			for _, entry := range entries {
				s.Equal("outcome", entry.Tags["tag"])
				s.Equal([]encounter.MemberID{activationCleric, activationFighter, activationGoblin}, entry.Audience)
			}
			for _, who := range []encounter.MemberID{activationFighter, activationGoblin} {
				s.Equal(entries, s.storyEntries(enc, who, out.Seqs), "existing full-roster audience policy")
			}

			// Neither later caller mutation nor persistence/reload can change the fact.
			result.Stabilization.Before = "changed by caller"
			result.Stabilization.Successes = 99
			s.Equal(entries, s.storyEntries(enc, activationCleric, out.Seqs))
			raw, err := json.Marshal(enc.ToData())
			s.Require().NoError(err)
			var data encounter.EncounterData
			s.Require().NoError(json.Unmarshal(raw, &data))
			reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Data: data, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
				Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
				Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			})
			s.Require().NoError(err)
			for _, who := range []encounter.MemberID{activationCleric, activationFighter, activationGoblin} {
				s.Equal(entries, s.storyEntries(reloaded, who, out.Seqs))
			}
		})
	}
}

func (s *RecordActivationSuite) TestStabilizationIsAlsoAnAbilityResultAndPreservesSuppliedFacts() {
	enc := s.scene(everyoneStanding{})
	result := stabilizedResult()
	result.Ref, result.Name = "custom:features:stabilize", "Custom Stabilization"
	// This layer carries vocabulary and numeric facts, never re-runs the rules.
	result.Stabilization = &encounter.StabilizationDetail{
		Before: "custom-before", After: "custom-after", HitPoints: 7,
		Successes: 1, Failures: 2, SuccessesNeeded: 4, FailuresRemaining: 5,
		Stabilized: false, Dead: true,
	}
	out, err := enc.RecordActivation(&encounter.RecordActivationInput{
		Actor: activationCleric, Target: activationFighter,
		Ability: encounter.ActivationIdentity{Ref: result.Ref, Name: result.Name},
		Results: []encounter.ActivationResult{validHealingResult(), result, {
			Kind: encounter.ResultCapacityGranted, Target: activationCleric, Description: "supplied capacity",
		}},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 4)
	entries := s.storyEntries(enc, activationCleric, out.Seqs)
	for i, want := range []encounter.ActivationResultKind{encounter.ResultHealingApplied, encounter.ResultStabilized, encounter.ResultCapacityGranted} {
		var beat struct {
			Result struct {
				Kind encounter.ActivationResultKind `json:"kind"`
			} `json:"result"`
		}
		s.Require().NoError(json.Unmarshal(entries[i+1].Payload, &beat))
		s.Equal(want, beat.Result.Kind, "supplied result order is preserved")
	}
	s.JSONEq(`{"beat":"activation-result","actor":"cleric","result":{"kind":"stabilized","target":"fighter","ref":"custom:features:stabilize","name":"Custom Stabilization","stabilization":{"before":"custom-before","after":"custom-after","hit_points":7,"successes":1,"failures":2,"successes_needed":4,"failures_remaining":5,"stabilized":false,"dead":true}}}`, string(entries[2].Payload))
}

func (s *RecordActivationSuite) TestStabilizationRejectsMalformedResultsBeforeAnyAppend() {
	cases := []struct {
		name   string
		mutate func(*encounter.ActivationResult)
		want   error
	}{
		{"missing target", func(r *encounter.ActivationResult) { r.Target = "" }, encounter.ErrNoMember},
		{"unknown target", func(r *encounter.ActivationResult) { r.Target = "absent" }, encounter.ErrNoMember},
		{"missing ref", func(r *encounter.ActivationResult) { r.Ref = "" }, encounter.ErrInvalidData},
		{"missing name", func(r *encounter.ActivationResult) { r.Name = "" }, encounter.ErrInvalidData},
		{"missing detail", func(r *encounter.ActivationResult) { r.Stabilization = nil }, encounter.ErrInvalidData},
		{"missing before", func(r *encounter.ActivationResult) { r.Stabilization.Before = "" }, encounter.ErrInvalidData},
		{"missing after", func(r *encounter.ActivationResult) { r.Stabilization.After = "" }, encounter.ErrInvalidData},
		{"calculation", func(r *encounter.ActivationResult) { r.Calculation = validHealingResult().Calculation }, encounter.ErrInvalidData},
		{"amount", func(r *encounter.ActivationResult) { r.Amount = 1 }, encounter.ErrInvalidData},
		{"requested", func(r *encounter.ActivationResult) { r.Requested = 1 }, encounter.ErrInvalidData},
		{"healing before", func(r *encounter.ActivationResult) { r.Before = 1 }, encounter.ErrInvalidData},
		{"healing after", func(r *encounter.ActivationResult) { r.After = 1 }, encounter.ErrInvalidData},
		{"condition address", func(r *encounter.ActivationResult) {
			r.Address = &encounter.ConditionAddress{MemberID: activationFighter}
		}, encounter.ErrInvalidData},
		{"damage type", func(r *encounter.ActivationResult) { r.DamageType = "fire" }, encounter.ErrInvalidData},
		{"description", func(r *encounter.ActivationResult) { r.Description = "unexpected" }, encounter.ErrInvalidData},
		{"reason", func(r *encounter.ActivationResult) { r.Reason = "unexpected" }, encounter.ErrInvalidData},
		{"moved", func(r *encounter.ActivationResult) { r.Moved = 1 }, encounter.ErrInvalidData},
		{"stopped by", func(r *encounter.ActivationResult) { r.StoppedBy = "wall" }, encounter.ErrInvalidData},
	}
	for _, tc := range cases {
		for _, cast := range []bool{false, true} {
			verb := "activation"
			if cast {
				verb = "cast"
			}
			s.Run(verb+"/"+tc.name, func() {
				standing := &countingStanding{}
				enc := s.scene(standing)
				before, calls := enc.ToData(), standing.calls
				result := stabilizedResult()
				tc.mutate(&result)
				results := []encounter.ActivationResult{validHealingResult(), result}
				if cast {
					out, err := enc.RecordCast(&encounter.RecordCastInput{
						Actor: activationCleric, Spell: encounter.SpellIdentity{Ref: "test:spells:result", Name: "Result"},
						Targets: []encounter.CastTargetResult{{Target: activationFighter, Results: results}},
					})
					s.ErrorIs(err, tc.want)
					s.Nil(out)
				} else {
					out, err := enc.RecordActivation(&encounter.RecordActivationInput{
						Actor: activationCleric, Ability: encounter.ActivationIdentity{Ref: "test:features:result", Name: "Result"}, Results: results,
					})
					s.ErrorIs(err, tc.want)
					s.Nil(out)
				}
				s.Equal(before, enc.ToData(), "even the earlier valid result must not append")
				s.Equal(calls, standing.calls, "refusal precedes noticeDown")
			})
		}
	}
}

func (s *RecordActivationSuite) TestOtherResultKindsRejectStabilizationDetail() {
	for _, kind := range []encounter.ActivationResultKind{
		encounter.ResultHealingApplied, encounter.ResultDamageApplied, encounter.ResultConditionApplied,
		encounter.ResultConditionRemoved, encounter.ResultCapacityGranted, encounter.ResultMoved,
	} {
		s.Run(string(kind), func() {
			enc := s.scene(everyoneStanding{})
			before := enc.WorldView().Log
			result := stabilizedResult()
			result.Kind = kind
			out, err := enc.RecordActivation(&encounter.RecordActivationInput{
				Actor: activationCleric, Ability: encounter.ActivationIdentity{Ref: result.Ref, Name: result.Name},
				Results: []encounter.ActivationResult{result},
			})
			s.ErrorIs(err, encounter.ErrInvalidData)
			s.Require().ErrorContains(err, "forbids stabilization")
			s.Nil(out)
			s.Equal(before, enc.WorldView().Log)
		})
	}
}
