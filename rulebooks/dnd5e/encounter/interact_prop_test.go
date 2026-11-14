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
		prop(ipGrate, 5, 4, encounter.Option{ID: "peek", Do: "You peer through the grate.", Success: []encounter.Answer{{Weight: 1, Say: "Steps, going down."}}}),
		prop(ipStair, 4, 6, encounter.Option{
			ID: "down", Do: "You take the stair down.", Success: []encounter.Answer{{Weight: 1}},
			Grants: []encounter.RowGrant{{Consequence: encounter.Consequence{
				Relocate: &encounter.RelocateGrant{To: psgRoomC, Words: "You come out in room c."},
			}}},
		}),
	}
	setup.Field.Intel = []encounter.IntelRecord{{ID: ipWay, Reveals: encounter.RevealTargets{Concealment: ipPassage}}}
	setup.Field.Concealments = []encounter.ConcealmentInput{{
		ID: ipPassage, Checks: []encounter.CheckApproach{{Ability: "investigation", DC: 30}}, Props: []encounter.PropID{ipGrate},
	}}
	setup.Field.Interactions = []encounter.InteractionInput{{ID: theFrontGate, Recipe: frontGateRecipe()}}
	setup.Sight = &sightList{reach: map[encounter.MemberID]int{relDan: 0}, fallback: 40}
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
// do, and nobody else. Done, Bob and Carol — who see her and are not shown
// the grate — are told only that she did something where she stands: never
// the grate, never the words.
func (s *InteractPropSuite) TestAHiddenThingIsDoneOnlyByWhoKnowsIt() {
	enc := s.party()
	_, err := enc.PropOptionsFor(relAlice, ipGrate)
	s.ErrorIs(err, encounter.ErrNoProp)
	_, err = s.do(enc, relAlice, ipGrate, "peek")
	s.ErrorIs(err, encounter.ErrNoProp, "a hidden thing is refused as one that names nothing")

	s.told(enc)
	offered, err := enc.PropOptionsFor(relAlice, ipGrate)
	s.Require().NoError(err)
	s.Require().Len(offered, 1)
	s.Equal("peek", offered[0].ID)
	s.True(offered[0].Does(), "the offer says it is something done")
	_, err = enc.PropOptionsFor(relBob, ipGrate)
	s.ErrorIs(err, encounter.ErrNoProp, "Bob was not whispered it")

	_, err = s.do(enc, relAlice, ipGrate, "peek")
	s.Require().NoError(err)
	s.Require().Len(s.interacted(enc, relAlice, relAlice), 1)
	for _, m := range []encounter.MemberID{relBob, relCarol} {
		seen := s.interacted(enc, m, relAlice)
		s.Require().Len(seen, 1, "%s sees her do something", m)
		s.Equal(encounter.InteractedViewPlace, seen[0]["view"])
		s.NotNil(seen[0]["at"])
		for _, key := range []string{"prop", "seen", "do", "option"} {
			s.NotContains(seen[0], key, "a concealed thing is never named to %s", m)
		}
		for _, b := range storyBeats(s.T(), enc, m) {
			s.False(b["beat"] == encounter.BeatAnswered && b["option"] == "peek", "the grate's narration is not told to %s", m)
		}
	}
	answered := 0
	for _, b := range storyBeats(s.T(), enc, relAlice) {
		if b["beat"] == encounter.BeatAnswered && b["option"] == "peek" && b["say"] == "Steps, going down." {
			answered++
		}
	}
	s.Equal(1, answered, "the narration is told to the one who did it")
	s.Empty(s.interacted(enc, relDan, relAlice))
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

	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(blob, &data))
	setup := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheetFacts{
		relAlice: {SpeedFeet: 30}, relBob: {SpeedFeet: 30}, relCarol: {SpeedFeet: 30}, relDan: {SpeedFeet: 30},
		ipScullion: {SpeedFeet: 30},
	}, passDriver{})
	setup.Sight = &sightList{reach: map[encounter.MemberID]int{relDan: 0}, fallback: 40}
	setup.CheckResolver = findsNothing{}
	setup.Witness = nobodyPerceives{}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: setup.Capabilities})
	s.Require().NoError(err)
	s.Equal(landed, s.where(loaded, relAlice))
	s.Len(storyBeats(s.T(), loaded, relAlice), before, "nothing fires again")
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
