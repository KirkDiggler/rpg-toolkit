// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// These live on RecordCastSuite because a concentration break is not a
// transaction of its own — it rides the cast and the strike, so it is tested
// against the same roster, the same record and the same Standing those verbs
// already are.

// concentrationCheck is the CON check the bard fails to keep True Strike up
// after the skeleton hits them.
func concentrationCheck() *encounter.CastSave {
	return &encounter.CastSave{
		Saver: castBard, Ability: "constitution", Roll: 4, Total: 6, DC: 10, Succeeded: false,
	}
}

// strickenChild is one address True Strike was holding when it broke: the
// condition it had put on the skeleton.
func strickenChild() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind:   encounter.ResultConditionRemoved,
		Target: castSkeleton,
		Ref:    "dnd5e:conditions:true-strike",
		Name:   "True Strike",
		Reason: "damage",
	}
}

// theOwnerItself is the concentrating condition coming off the caster's own
// sheet, the last address every break strips.
func theOwnerItself() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind:   encounter.ResultConditionRemoved,
		Target: castBard,
		Ref:    "dnd5e:conditions:concentrating",
		Name:   "Concentrating",
		Reason: "damage",
	}
}

func brokenByDamage() encounter.ConcentrationBreak {
	return encounter.ConcentrationBreak{
		Caster:  castBard,
		Spell:   trueStrike,
		Reason:  "damage",
		Save:    concentrationCheck(),
		Removed: []encounter.ActivationResult{strickenChild(), theOwnerItself()},
	}
}

// theSkeletonHits is the blow that asks the bard to hold on to their spell.
func theSkeletonHits(breaks ...encounter.ConcentrationBreak) *encounter.RecordInput {
	return &encounter.RecordInput{
		Kind:                encounter.OutcomeStruck,
		Actor:               castSkeleton,
		Targets:             []encounter.MemberID{castBard},
		Values:              map[encounter.OutcomeValue]int{encounter.ValueAmount: 9},
		ConcentrationBreaks: breaks,
	}
}

// TestOneHitReadsStruckSavedEndedRemoved is the whole issue in one scene, and
// it is the walk's own log: the break reads INSIDE the hit that caused it,
// from one call, in the order the rulebook produced it.
func (s *RecordCastSuite) TestOneHitReadsStruckSavedEndedRemoved() {
	standing := &countingStanding{}
	enc := s.scene(standing)
	callsBefore := standing.calls

	out, err := enc.Record(theSkeletonHits(brokenByDamage()))
	s.Require().NoError(err)
	s.Require().Len(out.FollowUpSeqs, 4)
	s.Equal(callsBefore+1, standing.calls, "noticeDown is consulted once for the whole train")

	seqs := append([]uint64{out.Seq}, out.FollowUpSeqs...)
	for i := 1; i < len(seqs); i++ {
		s.Less(seqs[i-1], seqs[i], "beat %d precedes beat %d", i-1, i)
	}
	s.Equal(
		[]string{"struck", "saved", "concentration_ended", "condition-removed", "condition-removed"},
		s.beatNames(s.storyEntries(enc, castBard, seqs)),
	)
}

// TestTheConcentrationEndedPayload pins the new body exactly, key for key:
// who was concentrating, what they lose, and why.
func (s *RecordCastSuite) TestTheConcentrationEndedPayload() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.Record(theSkeletonHits(brokenByDamage()))
	s.Require().NoError(err)

	entries := s.storyEntries(enc, castBard, out.FollowUpSeqs)
	s.Equal(
		`{"beat":"saved","saver":"bard","ability":"constitution","roll":4,"total":6,"dc":10,`+
			`"succeeded":false,`+
			`"source":{"ref":"dnd5e:spells:true-strike","name":"True Strike"}}`,
		string(entries[0].Payload),
	)
	s.Equal(
		`{"beat":"concentration_ended","caster":"bard",`+
			`"spell":{"ref":"dnd5e:spells:true-strike","name":"True Strike"},"reason":"damage"}`,
		string(entries[1].Payload),
	)
	s.Equal(encounter.BeatConcentrationEnded, "concentration_ended")
}

// TestAnUngatedBreakAppendsNoSavedBeat — three of the six reasons roll
// nothing, and a save beat reading zero against zero beside one would say a
// check happened that never did.
func (s *RecordCastSuite) TestAnUngatedBreakAppendsNoSavedBeat() {
	for _, reason := range []string{"duration", "combat_end", "spell_ended", "caster_down"} {
		s.Run(reason, func() {
			enc := s.scene(everyoneStanding{})

			broken := brokenByDamage()
			broken.Reason = reason
			broken.Save = nil
			out, err := enc.Record(theSkeletonHits(broken))
			s.Require().NoError(err)

			names := s.beatNames(s.storyEntries(enc, castBard, out.FollowUpSeqs))
			s.Equal(
				[]string{"concentration_ended", "condition-removed", "condition-removed"}, names,
			)
			s.NotContains(names, "saved")
		})
	}
}

// TestABreakRemovalStillRefusesRollFacts — a removal carries no roll, and the
// closed result set's existing refusal is what says so. Nothing this shape
// carries may smuggle a number in.
func (s *RecordCastSuite) TestABreakRemovalStillRefusesRollFacts() {
	for name, mangle := range map[string]func(*encounter.ActivationResult){
		"amount":      func(r *encounter.ActivationResult) { r.Amount = 3 },
		"requested":   func(r *encounter.ActivationResult) { r.Requested = 3 },
		"before":      func(r *encounter.ActivationResult) { r.Before = 7 },
		"after":       func(r *encounter.ActivationResult) { r.After = 4 },
		"calculation": func(r *encounter.ActivationResult) { r.Calculation = &encounter.RollCalculation{} },
	} {
		s.Run(name, func() {
			enc := s.scene(everyoneStanding{})
			before, err := enc.Story(&encounter.StoryInput{Audience: castBard})
			s.Require().NoError(err)

			broken := brokenByDamage()
			removed := theOwnerItself()
			mangle(&removed)
			broken.Removed = []encounter.ActivationResult{removed}

			_, err = enc.Record(theSkeletonHits(broken))
			s.Require().ErrorIs(err, encounter.ErrInvalidData)

			after, err := enc.Story(&encounter.StoryInput{Audience: castBard})
			s.Require().NoError(err)
			s.Len(after, len(before), "a refused break leaves no struck beat behind it")
		})
	}
}

// TestABreakStripsOnlyRemovals — the other four result kinds mean things a
// break cannot mean, so the shape refuses them by name rather than letting a
// caller apply a condition through a beat about one ending.
func (s *RecordCastSuite) TestABreakStripsOnlyRemovals() {
	enc := s.scene(everyoneStanding{})

	broken := brokenByDamage()
	broken.Removed = []encounter.ActivationResult{mockedCondition()}

	_, err := enc.Record(theSkeletonHits(broken))
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
}

// TestABreakSaysWhoAndWhyOrIsRefused — the beat exists to answer "why did that
// spell end", so a break that cannot answer it is not writable.
func (s *RecordCastSuite) TestABreakSaysWhoAndWhyOrIsRefused() {
	cases := map[string]struct {
		mangle func(*encounter.ConcentrationBreak)
		target error
	}{
		"no caster":      {func(b *encounter.ConcentrationBreak) { b.Caster = "" }, encounter.ErrNoMember},
		"unknown caster": {func(b *encounter.ConcentrationBreak) { b.Caster = "nobody" }, encounter.ErrNoMember},
		"no spell ref": {
			func(b *encounter.ConcentrationBreak) { b.Spell.Ref = "" }, encounter.ErrInvalidData,
		},
		"no spell name": {
			func(b *encounter.ConcentrationBreak) { b.Spell.Name = "" }, encounter.ErrInvalidData,
		},
		"no reason": {func(b *encounter.ConcentrationBreak) { b.Reason = "" }, encounter.ErrInvalidData},
		"a check nobody could have rolled": {
			func(b *encounter.ConcentrationBreak) { b.Save.Roll = 0 }, encounter.ErrInvalidData,
		},
		"a removal with no reason": {
			func(b *encounter.ConcentrationBreak) { b.Removed[0].Reason = "" }, encounter.ErrInvalidData,
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			enc := s.scene(everyoneStanding{})
			broken := brokenByDamage()
			tc.mangle(&broken)

			_, err := enc.Record(theSkeletonHits(broken))
			s.Require().ErrorIs(err, tc.target)

			_, err = enc.RecordCast(&encounter.RecordCastInput{
				Actor: castBard, Target: castSkeleton, Spell: viciousMockery,
				ConcentrationBreaks: []encounter.ConcentrationBreak{broken},
			})
			s.Require().ErrorIs(err, tc.target, "the cast path refuses the same break")
		})
	}
}

// TestAClosedEncounterRecordsNoBreak — the door every record verb keeps, and
// the break comes in through those same two doors rather than around them.
func (s *RecordCastSuite) TestAClosedEncounterRecordsNoBreak() {
	enc := s.scene(everyoneStanding{})
	_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)

	_, err = enc.Record(theSkeletonHits(brokenByDamage()))
	s.Require().ErrorIs(err, encounter.ErrClosed)

	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery,
		ConcentrationBreaks: []encounter.ConcentrationBreak{brokenByDamage()},
	})
	s.Require().ErrorIs(err, encounter.ErrClosed)
}

// TestACastDropsWhatItReplaced — casting a second concentration spell ends the
// first, and that break belongs to the cast that caused it. The break's beats
// come after everything the cast itself delivered.
func (s *RecordCastSuite) TestACastDropsWhatItReplaced() {
	enc := s.scene(everyoneStanding{})

	recast := brokenByDamage()
	recast.Reason = "recast"
	recast.Save = nil
	recast.Removed = []encounter.ActivationResult{theOwnerItself()}
	recast.Removed[0].Reason = "recast"

	out, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: failedSave(),
		Results:             []encounter.ActivationResult{psychicDamage()},
		ConcentrationBreaks: []encounter.ConcentrationBreak{recast},
	})
	s.Require().NoError(err)

	s.Equal(
		[]string{"cast", "saved", "damage-applied", "concentration_ended", "condition-removed"},
		s.beatNames(s.storyEntries(enc, castBard, out.Seqs)),
	)
}

// TestTwoBreaksKeepTheirOrder — one interaction can end two members' spells,
// and the story must read them in the order the rulebook ended them rather
// than in whatever order a map happened to hold.
func (s *RecordCastSuite) TestTwoBreaksKeepTheirOrder() {
	enc := s.scene(everyoneStanding{})

	second := encounter.ConcentrationBreak{
		Caster: castFighter,
		Spell:  viciousMockery,
		Reason: "caster_down",
		Removed: []encounter.ActivationResult{{
			Kind: encounter.ResultConditionRemoved, Target: castFighter,
			Ref: "dnd5e:conditions:concentrating", Name: "Concentrating", Reason: "caster_down",
		}},
	}

	out, err := enc.Record(theSkeletonHits(brokenByDamage(), second))
	s.Require().NoError(err)

	entries := s.storyEntries(enc, castBard, out.FollowUpSeqs)
	s.Equal([]string{
		"saved", "concentration_ended", "condition-removed", "condition-removed",
		"concentration_ended", "condition-removed",
	}, s.beatNames(entries))
	s.Contains(string(entries[1].Payload), `"caster":"bard"`)
	s.Contains(string(entries[4].Payload), `"caster":"cast-fighter"`)
}

// TestAnOutcomeWithNoBreakIsByteIdenticalToBefore — the regression that
// matters. Every strike the stack already records must land exactly as it did.
func (s *RecordCastSuite) TestAnOutcomeWithNoBreakIsByteIdenticalToBefore() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.Record(theSkeletonHits())
	s.Require().NoError(err)
	s.Empty(out.FollowUpSeqs, "a strike that broke nothing appends nothing after itself")

	entries := s.storyEntries(enc, castBard, []uint64{out.Seq})
	s.Equal(
		`{"actor":"cast-skeleton","amount":9,"beat":"struck","critical":false,"targets":["bard"]}`,
		string(entries[0].Payload),
	)
}
