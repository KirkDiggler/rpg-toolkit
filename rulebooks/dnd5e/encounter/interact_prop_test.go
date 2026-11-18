// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// interact_prop_test.go is SOMETHING DONE TO A THING (rpg-project#959) on the
// shut noise hall, a | b | c. In room a: Alice, beside a scullion who
// whispers her the way to a hidden grate; Bob and Carol, who see her; Dan,
// blind, in earshot and out of sight. Around Alice: a lever (done, checked,
// stepping the front gate), a sign (said), the hidden grate (done: peek), and
// a stair (done: down to room c).

const (
	ipScullion encounter.MemberID      = "scullion"
	ipLever    encounter.PropID        = "lever"
	ipSign     encounter.PropID        = "sign"
	ipGrate    encounter.PropID        = "grate"
	ipStair    encounter.PropID        = "stair"
	ipPassage  encounter.ConcealmentID = "passage"
	ipHatch    encounter.PropID        = "hatch"
	ipEve      encounter.MemberID      = "eve"
	ipGuard    encounter.MemberID      = "guard"
	ipWay      encounter.IntelID       = "the-way"
	ipPull                             = "Pull the lever"
	ipPulled                           = "pulls the lever"
)

type InteractPropSuite struct {
	suite.Suite
}

func TestInteractPropSuite(t *testing.T) { suite.Run(t, new(InteractPropSuite)) }

// hall is the shut hall with the scullion, the four props and the gate, the
// given members besides; Dan sees nothing.
func (s *InteractPropSuite) hall(members ...encounter.MemberInput) *encounter.Encounter {
	scullion := at(ipScullion, encounter.KindMonster, 3, 4)
	scullion.Options = []encounter.Option{{
		ID: "psst", Say: "A way out?",
		Success: []encounter.Answer{{Weight: 1, Say: "The grate.", Loudness: "whisper"}},
		Grants:  []encounter.RowGrant{{Consequence: encounter.Consequence{Tells: ipWay}}},
	}}
	members = append(members, scullion)
	sheets := sheetFacts{}
	for _, m := range members {
		sheets[m.ID] = encounter.SheetFacts{SpeedFeet: 30}
	}
	setup := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheets, passDriver{}, members...)
	no := false
	prop := func(id encounter.PropID, col, row int, options ...encounter.Option) encounter.PropInput {
		return encounter.PropInput{ID: id, Ref: "dnd5e:props:test:" + string(id), At: spatial.Position{X: float64(col), Y: float64(row)},
			BlocksMovement: &no, BlocksLineOfSight: &no, Options: options}
	}
	setup.Field.Props = []encounter.PropInput{
		prop(ipLever, 4, 2, encounter.Option{
			ID: "pull", Do: ipPull, Seen: ipPulled,
			Check:    []encounter.CheckApproach{{Ability: "athletics", DC: 10}},
			Requires: []encounter.Trigger{encounter.TriggerInteraction{Interaction: theFrontGate, In: []string{"fresh"}}},
			Advance:  &encounter.Advance{Interaction: theFrontGate, Success: "admitted", Failure: "guest-failed"},
			Success:  []encounter.Answer{{Weight: 1, Say: "Something clanks below.", Fact: "lever-pulled"}},
			Failure:  []encounter.Answer{{Weight: 1, Say: "It will not move."}},
		}),
		prop(ipSign, 2, 4, encounter.Option{ID: "read", Say: "What does this say?", Success: []encounter.Answer{{Weight: 1, Say: "Keep out."}}}),
		prop(ipGrate, 5, 4,
			encounter.Option{ID: "peek", Do: "You peer through the grate.", Success: []encounter.Answer{{Weight: 1, Say: "Steps, going down."}}},
			encounter.Option{ID: "hail", Say: "Anyone down there?", Success: []encounter.Answer{{Weight: 1}}}),
		prop(ipHatch, 21, 8, encounter.Option{ID: "peek", Do: "You peer through the hatch.", Success: []encounter.Answer{{Weight: 1}}}),
		prop(ipStair, 4, 6, encounter.Option{
			ID: "down", Do: "You take the stair down.", Success: []encounter.Answer{{Weight: 1}},
			Grants: []encounter.RowGrant{{Consequence: encounter.Consequence{
				Relocate: &encounter.RelocateGrant{To: psgRoomC, Words: "You come out in room c."},
			}}},
		}),
	}
	setup.Field.Intel = []encounter.IntelRecord{{ID: ipWay, Reveals: encounter.RevealTargets{Concealment: ipPassage}}}
	setup.Field.Concealments = []encounter.ConcealmentInput{{
		ID: ipPassage, Checks: []encounter.CheckApproach{{Ability: "investigation", DC: 30}}, Props: []encounter.PropID{ipGrate, ipHatch},
	}}
	setup.Field.Interactions = []encounter.InteractionInput{{ID: theFrontGate, Recipe: frontGateRecipe()}}
	setup.Sight = &sightList{reach: map[encounter.MemberID]int{relDan: 0, ipEve: 3}, fallback: 40}
	setup.CheckResolver = findsNothing{}
	setup.Witness = nobodyPerceives{}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	return enc
}

// party is Alice, Bob and Carol in sight of each other in room a, and Dan,
// blind, among them.
func (s *InteractPropSuite) party(more ...encounter.MemberInput) *encounter.Encounter {
	return s.hall(append([]encounter.MemberInput{
		at(relAlice, encounter.KindPlayer, 4, 4), at(relBob, encounter.KindPlayer, 7, 4),
		at(relCarol, encounter.KindPlayer, 2, 6), at(relDan, encounter.KindPlayer, 7, 7),
	}, more...)...)
}

// do does an option to a prop at the revision it is offered at now.
func (s *InteractPropSuite) do(enc *encounter.Encounter, who encounter.MemberID, prop encounter.PropID, option string) (*encounter.InteractOutput, error) {
	in := &encounter.InteractInput{
		Actor: who, Prop: prop, OptionID: option, Range: speakReach, Roller: rollsLowest{},
		Revision: s.revision(enc, who, prop, option),
	}
	if prop == ipLever {
		in.Skill, in.Beaten, in.DC, in.Total = "athletics", true, 10, 14
	}
	return enc.Interact(in)
}

// revision is the revision an option is offered at now, zero when it is not.
func (s *InteractPropSuite) revision(enc *encounter.Encounter, who encounter.MemberID, prop encounter.PropID, option string) uint64 {
	offered, err := enc.PropOptionsFor(who, prop)
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

// told has the scullion whisper Alice the way.
func (s *InteractPropSuite) told(enc *encounter.Encounter) {
	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: relAlice, Target: ipScullion, OptionID: "psst", Range: speakReach, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
}

// interacted is every interacted beat by actor in member's story.
func (s *InteractPropSuite) interacted(enc *encounter.Encounter, member, actor encounter.MemberID) []map[string]any {
	var out []map[string]any
	for _, b := range storyBeats(s.T(), enc, member) {
		if b["beat"] == encounter.BeatInteracted && b["actor"] == string(actor) {
			out = append(out, b)
		}
	}
	return out
}

func (s *InteractPropSuite) where(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.FailNow("no such member", string(id))
	return spatial.Position{}
}

// TestEachVerbRefusesTheOtherKind: Speak refuses a thing to do, and
// Interact a line to say, each writing nothing.
func (s *InteractPropSuite) TestEachVerbRefusesTheOtherKind() {
	enc := s.party()
	story := len(storyBeats(s.T(), enc, relAlice))
	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: relAlice, Prop: ipStair, OptionID: "down", Range: speakReach, Roller: rollsLowest{},
	})
	s.ErrorIs(err, encounter.ErrNoOption, "something done is not said")
	_, err = s.do(enc, relAlice, ipSign, "read")
	s.ErrorIs(err, encounter.ErrNoOption, "something said is not done")
	s.Len(storyBeats(s.T(), enc, relAlice), story, "nothing was written")
	s.Equal(cellAt(4, 4), s.where(enc, relAlice))
}

// TestItIsSeenNotHeard: Alice pulls the lever. She is told her words and
// her check; Bob and Carol, who see her and are shown the lever, are told
// she pulls it, never her words nor her roll, and the answer as narration.
// Dan, in earshot and out of sight, is told nothing — where a line said to
// the sign reaches him.
func (s *InteractPropSuite) TestItIsSeenNotHeard() {
	enc := s.party()
	out, err := s.do(enc, relAlice, ipLever, "pull")
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal(ipLever, out.Prop)
	s.NotContains(out.Witnesses, relDan)

	alice := s.interacted(enc, relAlice, relAlice)
	s.Require().Len(alice, 1)
	s.Equal(encounter.InteractedViewActor, alice[0]["view"])
	s.Equal(ipPull, alice[0]["do"])
	s.Equal("pull", alice[0]["option"])
	s.Equal(string(ipLever), alice[0]["prop"])
	s.Equal("athletics", alice[0]["skill"])
	s.Equal(true, alice[0]["beaten"])
	s.NotContains(alice[0], "seen")
	for _, m := range []encounter.MemberID{relBob, relCarol} {
		seen := s.interacted(enc, m, relAlice)
		s.Require().Len(seen, 1, "%s sees it done", m)
		s.Equal(encounter.InteractedViewThing, seen[0]["view"])
		s.Equal(ipPulled, seen[0]["seen"])
		s.Equal(string(ipLever), seen[0]["prop"])
		for _, key := range []string{"do", "skill", "total", "option"} {
			s.NotContains(seen[0], key, "%s is not told the doer's %s", m, key)
		}
	}
	s.Empty(s.interacted(enc, relDan, relAlice), "in earshot and out of sight, Dan is told nothing")
	held := func(observer encounter.MemberID) bool {
		holdings, err := enc.View(&encounter.ViewInput{Member: observer})
		s.Require().NoError(err)
		for _, h := range holdings {
			if h.Channel == deed.Channel && h.Subject == deed.Subject(relAlice, encounter.DeedUse) {
				return true
			}
		}
		return false
	}
	s.True(held(relBob), "Bob holds that he saw her use something")
	s.False(held(relDan), "Dan, who saw nothing, holds no deed")
	for _, b := range storyBeats(s.T(), enc, relDan) {
		s.NotEqual(encounter.BeatSpeech, b["beat"], "nothing was said")
		s.NotEqual(encounter.BeatAnswered, b["beat"], "nor answered to him")
	}

	// The control: a line said to the sign reaches Dan's ears.
	_, err = enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: relAlice, Prop: ipSign, OptionID: "read", Range: speakReach, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	heard := 0
	for _, b := range storyBeats(s.T(), enc, relDan) {
		if b["beat"] == encounter.BeatSpeech {
			heard++
		}
	}
	s.Positive(heard, "a said line is heard where a done thing is not")
}

// TestAHiddenThingIsDoneOnlyByWhoKnowsIt: the grate offers nothing to
// anyone until Alice is whispered the way; then it offers her its thing to
// do, and nobody else.
func (s *InteractPropSuite) TestAHiddenThingIsDoneOnlyByWhoKnowsIt() {
	enc := s.party()
	_, err := enc.PropOptionsFor(relAlice, ipGrate)
	s.ErrorIs(err, encounter.ErrNoProp)
	_, err = s.do(enc, relAlice, ipGrate, "peek")
	s.ErrorIs(err, encounter.ErrNoProp, "a hidden thing is refused as one that names nothing")

	s.told(enc)
	offered, err := enc.PropOptionsFor(relAlice, ipGrate)
	s.Require().NoError(err)
	s.Require().Len(offered, 2)
	s.Equal("peek", offered[0].ID)
	s.True(offered[0].Does(), "the offer says it is something done")
	s.False(offered[1].Does(), "and a line to say beside it")
	_, err = enc.PropOptionsFor(relBob, ipGrate)
	s.ErrorIs(err, encounter.ErrNoProp, "Bob was not whispered it")
}

// TestIfYouSawItYouKnowIt (rpg-project#959, ruled on #848): Alice, told,
// peers through the hidden grate. Bob and Carol, untold, see it done: each
// comes to know the passage by sight — the reveal before the deed in their
// story — is told the `thing` version, and is then offered the grate. Eve,
// who sees Alice and not as far as the grate, is told only the `place`
// version and learns nothing. Dan, blind in earshot, learns nothing. A
// guard, a creature, sees it and holds the passage too (nothing reacts).
func (s *InteractPropSuite) TestIfYouSawItYouKnowIt() {
	enc := s.party(at(ipEve, encounter.KindPlayer, 1, 4), at(ipGuard, encounter.KindMonster, 8, 2))
	s.told(enc)
	_, err := s.do(enc, relAlice, ipGrate, "peek")
	s.Require().NoError(err)

	for _, m := range []encounter.MemberID{relBob, relCarol} {
		s.Equal([]string{"saw it used"}, s.learnedPassage(enc, m), "%s saw it used", m)
		var order []string
		for _, b := range storyBeats(s.T(), enc, m) {
			switch {
			case b["beat"] == encounter.BeatConcealmentRevealed:
				order = append(order, "revealed")
			case b["beat"] == encounter.BeatInteracted && b["actor"] == string(relAlice):
				order = append(order, b["view"].(string))
				s.Equal(string(ipGrate), b["prop"])
				s.NotContains(b, "do", "the doer's own words are the doer's")
			}
		}
		s.Equal([]string{"revealed", encounter.InteractedViewThing}, order, "%s learns it, then sees the thing done", m)
		offered, err := enc.PropOptionsFor(m, ipGrate)
		s.Require().NoError(err, "%s is now offered the grate", m)
		s.Len(offered, 2)
	}

	narrated := func(m encounter.MemberID) bool {
		for _, b := range storyBeats(s.T(), enc, m) {
			if b["beat"] == encounter.BeatAnswered && b["option"] == "peek" {
				return true
			}
		}
		return false
	}
	s.True(narrated(relBob), "shown now, Bob is told the narration as a shown witness is")
	s.False(narrated(ipEve), "Eve, not shown, is not")
	eve := s.interacted(enc, ipEve, relAlice)
	s.Require().Len(eve, 1, "precondition: Eve sees Alice")
	s.Equal(encounter.InteractedViewPlace, eve[0]["view"], "Eve sees her, not the grate")
	s.NotContains(eve[0], "prop")
	s.Empty(s.learnedPassage(enc, ipEve))
	s.Empty(s.learnedPassage(enc, relDan), "in earshot and out of sight, Dan learns nothing")
	s.Empty(s.interacted(enc, relDan, relAlice))
	s.Equal([]string{"saw it used"}, s.learnedPassage(enc, ipGuard), "a creature that sees it knows it too")
}

// TestAnOpenThingOutOfSightIsThePlace: no secret here — the lever hides
// nothing — and Eve, who sees Alice and has never seen as far as the lever,
// is told only the `place` version, as before (rpg-project#959).
func (s *InteractPropSuite) TestAnOpenThingOutOfSightIsThePlace() {
	enc := s.party(at(ipEve, encounter.KindPlayer, 1, 4))
	_, err := enc.PropOptionsFor(ipEve, ipLever)
	s.Require().ErrorIs(err, encounter.ErrNoProp, "precondition: Eve has never been shown the lever")
	_, err = s.do(enc, relAlice, ipLever, "pull")
	s.Require().NoError(err)
	eve := s.interacted(enc, ipEve, relAlice)
	s.Require().Len(eve, 1, "precondition: Eve sees Alice")
	s.Equal(encounter.InteractedViewPlace, eve[0]["view"])
	s.NotContains(eve[0], "prop")
}

// TestALineSaidToAHiddenThingShowsIt: Alice, told, calls down the grate;
// Bob, who sees her say it to the grate, knows the passage by sight.
func (s *InteractPropSuite) TestALineSaidToAHiddenThingShowsIt() {
	enc := s.party()
	s.told(enc)
	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: relAlice, Prop: ipGrate, OptionID: "hail", Range: speakReach, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.Equal([]string{"saw it used"}, s.learnedPassage(enc, relBob))
	s.Empty(s.learnedPassage(enc, relDan), "hearing it said is not seeing the grate")

	// AND IT IS NAMED ONLY TO WHO IS SHOWN IT: Dan catches the words out of
	// sight and is never told which thing they were said to; Bob, revealed
	// by seeing it, is.
	speech := func(m encounter.MemberID) map[string]any {
		for _, b := range storyBeats(s.T(), enc, m) {
			if b["beat"] == encounter.BeatSpeech && b["words"] == "Anyone down there?" {
				return b
			}
		}
		s.FailNow("no speech", string(m))
		return nil
	}
	s.NotContains(speech(relDan), "prop", "a hidden thing is not named to a hearer out of sight")
	s.Equal(string(ipGrate), speech(relBob)["prop"])
	s.Equal(string(ipGrate), speech(relAlice)["prop"], "the speaker always knows what they spoke to")
}

// TestNobodyLearnsWhatNobodySaw: Alice, told, takes the stair to room c and
// peers through the hidden hatch there, in nobody's sight. Nobody learns the
// passage.
func (s *InteractPropSuite) TestNobodyLearnsWhatNobodySaw() {
	enc := s.party()
	s.told(enc)
	_, err := s.do(enc, relAlice, ipStair, "down")
	s.Require().NoError(err)
	out, err := s.do(enc, relAlice, ipHatch, "peek")
	s.Require().NoError(err)
	s.Equal([]encounter.MemberID{relAlice}, out.Witnesses, "precondition: only Alice sees herself in room c")
	for _, m := range []encounter.MemberID{relBob, relCarol, relDan} {
		s.Empty(s.learnedPassage(enc, m), "%s saw nothing", m)
	}
}

// TestWhatWasSeenSurvivesAReload: Bob, who saw it, still knows the grate
// after the run is saved and loaded, and nothing is told again.
func (s *InteractPropSuite) TestWhatWasSeenSurvivesAReload() {
	enc := s.party()
	s.told(enc)
	_, err := s.do(enc, relAlice, ipGrate, "peek")
	s.Require().NoError(err)
	before := len(storyBeats(s.T(), enc, relBob))
	loaded := s.reload(enc)
	s.Len(storyBeats(s.T(), loaded, relBob), before, "nothing fires again")
	_, err = loaded.PropOptionsFor(relBob, ipGrate)
	s.NoError(err, "Bob still knows the grate")
	s.Equal([]string{"saw it used"}, s.learnedPassage(loaded, relBob), "learned once")
}

// learnedPassage is the cause of every fact that taught member the passage.
func (s *InteractPropSuite) learnedPassage(enc *encounter.Encounter, member encounter.MemberID) []string {
	var out []string
	for _, f := range enc.ToData().Journal.Facts {
		if f.Kind == "known:concealment:"+string(ipPassage) && f.Actor == string(member) {
			out = append(out, f.Detail)
		}
	}
	return out
}

// TestAStaleThingIsRefused: the lever steps the front gate, so it is
// offered at the gate's revision; one sent at an older revision is refused
// as a stale line is, with nothing written.
func (s *InteractPropSuite) TestAStaleThingIsRefused() {
	enc := s.party()
	revision := s.revision(enc, relAlice, ipLever, "pull")
	s.Require().NotZero(revision)
	story := len(storyBeats(s.T(), enc, relAlice))
	_, err := enc.Interact(&encounter.InteractInput{
		Actor: relAlice, Prop: ipLever, OptionID: "pull", Range: speakReach, Roller: rollsLowest{},
		Revision: revision - 1, Skill: "athletics", Beaten: true, DC: 10, Total: 14,
	})
	s.ErrorIs(err, encounter.ErrStaleOption)
	s.Len(storyBeats(s.T(), enc, relAlice), story)
}

// TestACheckedThingReservesAndSettles: the lever's check pauses on Alice's
// roll; the gate is held for everyone, Bob's pull pending; her Interact
// settles it, steps the gate and frees it.
func (s *InteractPropSuite) TestACheckedThingReservesAndSettles() {
	enc := s.party()
	offeredAt := s.revision(enc, relAlice, ipLever, "pull")
	reserved, err := enc.Reserve(&encounter.ReserveInput{
		Actor: relAlice, Prop: ipLever, OptionID: "pull", Range: speakReach, Revision: offeredAt,
	})
	s.Require().NoError(err)
	s.Equal(theFrontGate, reserved.Interaction)
	offered, err := enc.PropOptionsFor(relBob, ipLever)
	s.Require().NoError(err)
	s.Require().Len(offered, 1)
	s.True(offered[0].Pending, "the held gate holds the lever for Bob")

	_, err = enc.Interact(&encounter.InteractInput{
		Actor: relAlice, Prop: ipLever, OptionID: "pull", Range: speakReach, Roller: rollsLowest{},
		Revision: offeredAt, Skill: "athletics", Beaten: true, DC: 10, Total: 14,
	})
	s.Require().NoError(err, "her own roll settles it, at the revision it was reserved at")
	offered, err = enc.PropOptionsFor(relBob, ipLever)
	s.Require().NoError(err)
	s.Empty(offered, "the gate stepped to admitted, where the lever is no longer offered")
	s.Equal("admitted", s.gateState(enc))
}

// gateState is the front gate's state, read off the record.
func (s *InteractPropSuite) gateState(enc *encounter.Encounter) string {
	for _, r := range enc.ToData().Interactions {
		if r.ID == theFrontGate {
			return r.Blob.State
		}
	}
	s.FailNow("no front gate")
	return ""
}

// TestARelocatingThingIsRefusedWholeInAFight: an orc beside Alice opens a
// fight; the stair's thing to do, whose row would set her down in room c,
// is refused before anything is written.
func (s *InteractPropSuite) TestARelocatingThingIsRefusedWholeInAFight() {
	orc := encounter.MemberInput{ID: relOrc, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 5}}
	enc := s.party(orc)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: relAlice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clock.Kind, "precondition: the orc opened a fight")
	story := len(storyBeats(s.T(), enc, relAlice))
	_, err = s.do(enc, relAlice, ipStair, "down")
	s.ErrorIs(err, encounter.ErrInBubble)
	s.Equal(cellAt(4, 4), s.where(enc, relAlice))
	s.Len(storyBeats(s.T(), enc, relAlice), story)
}

// TestReloadRepeatsNothing: saved after Alice takes the stair, the run
// loads with her in room c and the story exactly as long.
func (s *InteractPropSuite) TestReloadRepeatsNothing() {
	enc := s.party()
	_, err := s.do(enc, relAlice, ipStair, "down")
	s.Require().NoError(err)
	region, ok := enc.RegionAt(s.where(enc, relAlice))
	s.Require().True(ok)
	s.Equal(psgRoomC, region, "the thing done set her down in room c")
	before, landed := len(storyBeats(s.T(), enc, relAlice)), s.where(enc, relAlice)
	loaded := s.reload(enc)
	s.Equal(landed, s.where(loaded, relAlice))
	s.Len(storyBeats(s.T(), loaded, relAlice), before, "nothing fires again")
}

// reload saves the run and loads it back on the hall's host.
func (s *InteractPropSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(blob, &data))
	setup := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheetFacts{
		relAlice: {SpeedFeet: 30}, relBob: {SpeedFeet: 30}, relCarol: {SpeedFeet: 30}, relDan: {SpeedFeet: 30},
		ipScullion: {SpeedFeet: 30}, ipEve: {SpeedFeet: 30}, ipGuard: {SpeedFeet: 30},
	}, passDriver{})
	setup.Sight = &sightList{reach: map[encounter.MemberID]int{relDan: 0, ipEve: 3}, fallback: 40}
	setup.CheckResolver = findsNothing{}
	setup.Witness = nobodyPerceives{}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: setup.Capabilities})
	s.Require().NoError(err)
	return loaded
}

// TestTheKindIsRefusedWhereItIsWrong: the engine's own door on an option's
// kind (rpg-project#959), for every caller the authoring dialect is not.
func (s *InteractPropSuite) TestTheKindIsRefusedWhereItIsWrong() {
	no := false
	prop := func(o encounter.Option) error {
		setup := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheetFacts{relAlice: {SpeedFeet: 30}}, passDriver{},
			at(relAlice, encounter.KindPlayer, 4, 4))
		setup.Field.Props = []encounter.PropInput{{ID: "thing", Ref: "dnd5e:props:test:thing", At: spatial.Position{X: 2, Y: 2},
			BlocksMovement: &no, BlocksLineOfSight: &no, Options: []encounter.Option{o}}}
		_, err := encounter.NewEncounter(setup)
		return err
	}
	ok := encounter.Option{ID: "x", Do: "You touch it.", Seen: "touches it", Success: []encounter.Answer{{Weight: 1}}}
	s.Require().NoError(prop(ok), "precondition: a thing to do with its seen words is accepted")
	for name, mutate := range map[string]func(o *encounter.Option){
		"said and done":      func(o *encounter.Option) { o.Say = "Hello." },
		"neither":            func(o *encounter.Option) { o.Do, o.Seen = "", "" },
		"done loudly":        func(o *encounter.Option) { o.Loudness = string(encounter.LoudnessShout) },
		"seen beside a line": func(o *encounter.Option) { o.Do, o.Say = "", "Hello." },
	} {
		o := ok
		mutate(&o)
		s.ErrorIs(prop(o), encounter.ErrNoField, name)
	}

	creature := at(ipScullion, encounter.KindMonster, 3, 4)
	creature.Options = []encounter.Option{ok}
	_, err := encounter.NewEncounter(noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(),
		sheetFacts{ipScullion: {SpeedFeet: 30}}, passDriver{}, creature))
	s.ErrorIs(err, encounter.ErrBadMember, "something done is done to a thing, never a creature")
}
