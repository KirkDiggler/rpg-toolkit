// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// encounteranswers_test.go holds the encounter's answers of rpg-project#539
// ("Encounter answers, resolution asks") to their done-when: who is in an
// area, told from one membership function at a step and at an area change;
// where an observer believes a subject stands and whether it reaches; and
// what a verb settled, read from the encounter's own record.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type EncounterAnswersSuite struct {
	deathScene
}

func TestEncounterAnswersSuite(t *testing.T) {
	suite.Run(t, new(EncounterAnswersSuite))
}

// fogAt is a labelled area the spell would have declared: its membership
// label is content's, carried and never read here.
func fogAt(center spatial.Position) *encounter.SightAreaInput {
	return &encounter.SightAreaInput{
		ID: "fog", SourceID: "caster", Center: center, RadiusFeet: 5,
		MembershipRef: "dnd5e:conditions:in_fog", MembershipName: "In Fog", MembershipSourceID: "spell",
	}
}

// areaTelling is one membership beat as a member's own story tells it.
type areaTelling struct {
	kind   string
	reason string
	raw    string
}

// tellingsAbout reads the membership beats about one member from that
// member's own story — the audience a membership beat always includes.
func (s *EncounterAnswersSuite) tellingsAbout(enc *encounter.Encounter, member encounter.MemberID) []areaTelling {
	s.T().Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]areaTelling, 0)
	for _, entry := range story {
		var beat struct {
			Beat   string `json:"beat"`
			Actor  string `json:"actor"`
			Result struct {
				Kind   string `json:"kind"`
				Reason string `json:"reason"`
			} `json:"result"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat != encounter.BeatActivationResult || beat.Actor != string(member) {
			continue
		}
		out = append(out, areaTelling{kind: beat.Result.Kind, reason: beat.Result.Reason, raw: string(entry.Payload)})
	}
	return out
}

func (s *EncounterAnswersSuite) loadData(data encounter.EncounterData) *encounter.Encounter {
	s.T().Helper()
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     zeroSheets{},
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// A member stepping into an area and out again inside one walk is told it
// entered and that it left — and the entering is told in exactly the words an
// area change uses for a member it opens on, because both come from the one
// membership function.
func (s *EncounterAnswersSuite) TestAWalkInAndOutIsToldBothFromTheAreaChangesFunction() {
	enc := s.loadData(validEncounterData())
	s.Require().NoError(enc.AddSightArea(fogAt(spatial.Position{X: 4, Y: 1})))
	s.Require().Empty(s.tellingsAbout(enc, "p1"), "p1 starts outside the fog")

	for _, cell := range []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 2, Y: 1}} {
		_, err := enc.Step(&encounter.StepInput{Member: "p1", To: cell})
		s.Require().NoError(err)
	}

	told := s.tellingsAbout(enc, "p1")
	s.Require().Len(told, 2, "one walk in, one walk out")
	s.Equal(string(encounter.ResultConditionApplied), told[0].kind)
	s.Equal(string(encounter.ResultConditionRemoved), told[1].kind)
	s.Equal("left area", told[1].reason, "it walked out; the area did not end")

	// The same member, opened on by the area instead of walking in: the
	// entering is told byte for byte as the step told it.
	data := validEncounterData()
	data.Members[0].Cell = &encounter.PositionData{X: 3, Y: 1}
	opened := s.loadData(data)
	s.Require().NoError(opened.AddSightArea(fogAt(spatial.Position{X: 4, Y: 1})))
	byChange := s.tellingsAbout(opened, "p1")
	s.Require().Len(byChange, 1)
	s.Equal(told[0].raw, byChange[0].raw, "a step and an area change tell entering in one shape")
}

// Adding an area tells every member inside it that it entered; removing it
// tells every member inside it that the area ended. A member outside hears
// neither about itself.
func (s *EncounterAnswersSuite) TestAddingAndRemovingAnAreaTellsEveryMemberInside() {
	data := validEncounterData()
	data.Members = append(data.Members,
		encounter.MemberData{ID: "centre", Kind: encounter.KindPlayer, Cell: &encounter.PositionData{X: 4, Y: 1}},
		encounter.MemberData{ID: "edge", Kind: encounter.KindPlayer, Cell: &encounter.PositionData{X: 3, Y: 1}},
	)
	data.EverMembers = append(data.EverMembers, "centre", "edge")
	enc := s.loadData(data)

	s.Require().NoError(enc.AddSightArea(fogAt(spatial.Position{X: 4, Y: 1})))
	removed, err := enc.RemoveSightArea("caster")
	s.Require().NoError(err)
	s.Require().True(removed)

	for _, inside := range []encounter.MemberID{"centre", "edge"} {
		told := s.tellingsAbout(enc, inside)
		s.Require().Len(told, 2, string(inside))
		s.Equal(string(encounter.ResultConditionApplied), told[0].kind, string(inside))
		s.Equal(string(encounter.ResultConditionRemoved), told[1].kind, string(inside))
		s.Equal("area ended", told[1].reason, string(inside))
	}
	s.Empty(s.tellingsAbout(enc, "p1"), "a member outside the area is told nothing about itself")

	again, err := enc.RemoveSightArea("caster")
	s.Require().NoError(err)
	s.False(again, "a source with no area open removes nothing")
}

// An aim at a remembered location out of range answers not in range; an aim
// at a location the subject has since left answers displaced. Both are read
// from the observer's own testimony, never the subject's live cell.
func (s *EncounterAnswersSuite) TestTheBelievedAimAnswersRangeAndDisplacement() {
	var withTestimony func(encounter.SightTestimony) *encounter.Encounter
	withBelief := func(believed spatial.Position) *encounter.Encounter {
		return withTestimony(encounter.SightTestimony{
			State: encounter.LocationKnown, Position: believed, Down: new(bool), BlocksMovement: new(bool),
		})
	}
	withTestimony = func(testimony encounter.SightTestimony) *encounter.Encounter {
		data := validEncounterData()
		data.Members = append(data.Members,
			encounter.MemberData{ID: "target", Kind: encounter.KindPlayer, Cell: &encounter.PositionData{X: 3, Y: 3}})
		data.EverMembers = append(data.EverMembers, "target")
		payload, err := encounter.EncodeSightTestimony(testimony)
		s.Require().NoError(err)
		setSightHolding(s.T(), &data, payload, false)
		// setSightHolding writes qualified subjects, which is the current
		// perception subject version a saved world carries.
		data.PerceptionSubjects = s.loadData(validEncounterData()).ToData().PerceptionSubjects
		return s.loadData(data)
	}

	far := withBelief(spatial.Position{X: 4, Y: 4})
	out, err := far.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: 5})
	s.Require().NoError(err)
	s.True(out.Held)
	s.Equal(encounter.LocationKnown, out.State)
	s.False(out.InRange, "a remembered point beyond reach is not in range")

	left := withBelief(spatial.Position{X: 2, Y: 1})
	out, err = left.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: 30})
	s.Require().NoError(err)
	s.True(out.InRange, "the believed point is in reach")
	s.True(out.Displaced, "the subject stands elsewhere now")

	stays := withBelief(spatial.Position{X: 3, Y: 3})
	out, err = stays.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: 30})
	s.Require().NoError(err)
	s.True(out.InRange)
	s.False(out.Displaced, "a subject still on the believed point is not displaced")

	near := withBelief(spatial.Position{X: 1, Y: 1})
	out, err = near.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: 0})
	s.Require().NoError(err)
	s.True(out.InRange, "a zero range reaches the observer's own cell")
	out, err = stays.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: 0})
	s.Require().NoError(err)
	s.False(out.InRange, "and nothing beyond it")

	unknown := withTestimony(encounter.SightTestimony{State: encounter.LocationUnknown})
	out, err = unknown.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: 30})
	s.Require().NoError(err)
	s.True(out.Held, "the subject is known")
	s.Equal(encounter.LocationUnknown, out.State)
	s.False(out.InRange, "a subject known without a point has no point in range")
	s.False(out.Displaced)

	none, err := stays.BelievedAim(&encounter.BelievedAimInput{Observer: "target", Subject: "p1", RangeFeet: 30})
	s.Require().NoError(err)
	s.False(none.Held, "an observer with no testimony holds no belief")

	_, err = stays.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "stranger", RangeFeet: 30})
	s.Require().ErrorIs(err, encounter.ErrNotMember)
	_, err = stays.BelievedAim(&encounter.BelievedAimInput{Observer: "p1", Subject: "target", RangeFeet: -5})
	s.Require().ErrorIs(err, encounter.ErrBadReach)
}

// A fight that ends by defeat appears in the settlement read with its members
// and its cause, and the falls that ended it appear with their kind in the
// order they happened — read from the encounter's own record, with no
// audience named and no payload handed to the reader.
func (s *EncounterAnswersSuite) TestAFightEndedByDefeatIsSettled() {
	down := &downList{}
	enc := s.trio(down)
	baseline, err := enc.NextStorySeq()
	s.Require().NoError(err)

	down.down = []encounter.MemberID{goblin, wolf}
	_, err = aRound(enc)
	s.Require().NoError(err)

	settled, err := enc.Settlement(&encounter.SettlementInput{FromSeq: baseline})
	s.Require().NoError(err)
	s.Require().Len(settled.FightsEnded, 1)
	s.Equal(encounter.DissolveByDefeat, settled.FightsEnded[0].Cause)
	s.ElementsMatch([]encounter.MemberID{alice, goblin, wolf}, settled.FightsEnded[0].Members)

	fell := make([]encounter.MemberID, 0, len(settled.Falls))
	for i, f := range settled.Falls {
		s.Equal(encounter.KindMonster, f.Kind, string(f.Member))
		s.Less(f.Seq, settled.FightsEnded[0].Seq, "the falls are told before the ending they explain")
		if i > 0 {
			s.Less(settled.Falls[i-1].Seq, f.Seq, "falls are listed in the order they happened")
		}
		fell = append(fell, f.Member)
	}
	s.ElementsMatch([]encounter.MemberID{goblin, wolf}, fell)

	after, err := enc.NextStorySeq()
	s.Require().NoError(err)
	later, err := enc.Settlement(&encounter.SettlementInput{FromSeq: after})
	s.Require().NoError(err)
	s.Empty(later.FightsEnded, "a bound past the verb settles nothing it did")
	s.Empty(later.Falls)
}

// A fight ended by decision appears in the same read with its own cause.
func (s *EncounterAnswersSuite) TestAFightEndedByDecisionIsSettled() {
	enc := s.pair(everyoneStanding{})
	baseline, err := enc.NextStorySeq()
	s.Require().NoError(err)

	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)

	settled, err := enc.Settlement(&encounter.SettlementInput{FromSeq: baseline})
	s.Require().NoError(err)
	s.Require().Len(settled.FightsEnded, 1)
	s.Equal(encounter.DissolveByDecision, settled.FightsEnded[0].Cause)
	s.ElementsMatch([]encounter.MemberID{alice, goblin}, settled.FightsEnded[0].Members)
	s.Empty(settled.Falls, "nobody fell")

	_, err = enc.Settlement(nil)
	s.Require().ErrorIs(err, encounter.ErrNilInput)
}

// A fall is a true historical fact: a member who fell and then exited is
// still a fall, with the kind it fell as, in the order it fell.
func (s *EncounterAnswersSuite) TestAFallerWhoExitedIsStillAFall() {
	down := &downList{}
	enc := s.trio(down)
	baseline, err := enc.NextStorySeq()
	s.Require().NoError(err)

	down.down = []encounter.MemberID{goblin, wolf}
	_, err = aRound(enc)
	s.Require().NoError(err)
	_, err = enc.Exit(&encounter.ExitInput{Member: goblin})
	s.Require().NoError(err)

	settled, err := enc.Settlement(&encounter.SettlementInput{FromSeq: baseline})
	s.Require().NoError(err)
	s.Require().Len(settled.Falls, 2, "both falls, the departed one included")
	fell := make([]encounter.MemberID, 0, 2)
	for _, f := range settled.Falls {
		s.Equal(encounter.KindMonster, f.Kind, "the kind the beat carried at the fall")
		fell = append(fell, f.Member)
	}
	s.Less(settled.Falls[0].Seq, settled.Falls[1].Seq)
	s.ElementsMatch([]encounter.MemberID{goblin, wolf}, fell)
}

// A fight whose beats reached only the members present then is settled all
// the same: alice joins after the fight between bob and the monsters ended,
// so no beat of it reached her, and the settlement does not depend on whose
// story it is.
func (s *EncounterAnswersSuite) TestASettlementDoesNotDependOnWhoSawIt() {
	down := &downList{}
	enc := s.scene(down,
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		encounter.MemberInput{ID: wolf, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 10}},
	)
	baseline, err := enc.NextStorySeq()
	s.Require().NoError(err)
	down.down = []encounter.MemberID{goblin, wolf}
	_, err = aRound(enc)
	s.Require().NoError(err)
	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: spatial.Position{X: 4, Y: 2}})
	s.Require().NoError(err)

	for _, beat := range s.beatsOf(enc, alice) {
		s.NotEqual(encounter.BeatFightEnded, beat["beat"], "control: the ending never reached alice")
	}

	settled, err := enc.Settlement(&encounter.SettlementInput{FromSeq: baseline})
	s.Require().NoError(err)
	s.Require().Len(settled.FightsEnded, 1)
	s.Equal(encounter.DissolveByDefeat, settled.FightsEnded[0].Cause)
	s.ElementsMatch([]encounter.MemberID{bob, goblin, wolf}, settled.FightsEnded[0].Members)
	s.Len(settled.Falls, 2)
}

// An area's membership beat is written when the change is applied, so it
// survives a save and reload with nothing to flush.
func (s *EncounterAnswersSuite) TestAnAreaBeatSurvivesASave() {
	data := validEncounterData()
	data.Members[0].Cell = &encounter.PositionData{X: 3, Y: 1}
	enc := s.loadData(data)
	s.Require().NoError(enc.AddSightArea(fogAt(spatial.Position{X: 4, Y: 1})))

	reloaded := s.loadData(enc.ToData())
	told := s.tellingsAbout(reloaded, "p1")
	s.Require().Len(told, 1, "the entering was saved with the area")
	s.Equal(string(encounter.ResultConditionApplied), told[0].kind)
}

// A step right after an area change is told after the change, never before
// it: the area's entering precedes the step's leaving.
func (s *EncounterAnswersSuite) TestAStepAfterAnAreaChangeIsToldInOrder() {
	data := validEncounterData()
	data.Members[0].Cell = &encounter.PositionData{X: 3, Y: 1}
	enc := s.loadData(data)
	s.Require().NoError(enc.AddSightArea(fogAt(spatial.Position{X: 4, Y: 1})))
	_, err := enc.Step(&encounter.StepInput{Member: "p1", To: spatial.Position{X: 1, Y: 1}})
	s.Require().NoError(err)

	told := s.tellingsAbout(enc, "p1")
	s.Require().Len(told, 2)
	s.Equal(string(encounter.ResultConditionApplied), told[0].kind)
	s.Equal(string(encounter.ResultConditionRemoved), told[1].kind)
	s.Equal("left area", told[1].reason)
}

// A member the ended area held, who then exits, still has "area ended" in its
// story: the beat was written before it left.
func (s *EncounterAnswersSuite) TestAnAreaEndedBeatOutlivesAnExit() {
	data := validEncounterData()
	data.Members = append(data.Members,
		encounter.MemberData{ID: "inside", Kind: encounter.KindPlayer, Cell: &encounter.PositionData{X: 4, Y: 1}})
	data.EverMembers = append(data.EverMembers, "inside")
	enc := s.loadData(data)
	s.Require().NoError(enc.AddSightArea(fogAt(spatial.Position{X: 4, Y: 1})))
	_, err := enc.RemoveSightArea("caster")
	s.Require().NoError(err)
	_, err = enc.Exit(&encounter.ExitInput{Member: "inside"})
	s.Require().NoError(err)

	told := s.tellingsAbout(enc, "inside")
	s.Require().Len(told, 2)
	s.Equal("area ended", told[1].reason)
}

// An experience beat may name the monster whose fall paid even after that
// monster has exited: the actor is the cause, and a member who was here keeps
// its name in the story. A never-member is still refused, and every other
// kind still requires a current member.
func (s *EncounterAnswersSuite) TestAnExperienceBeatNamesAFallenMonsterThatExited() {
	down := &downList{}
	enc := s.trio(down)
	down.down = []encounter.MemberID{goblin}
	_, err := aRound(enc)
	s.Require().NoError(err)
	_, err = enc.Exit(&encounter.ExitInput{Member: goblin})
	s.Require().NoError(err)

	grant := &encounter.ExperienceDetail{
		Member: string(goblin),
		Grants: []encounter.ExperienceGrant{{Character: string(alice), Amount: 50, Total: 350}},
	}
	out, err := recordOne(enc, &encounter.RecordInput{Kind: encounter.OutcomeExperienceGained, Actor: goblin, Experience: grant})
	s.Require().NoError(err, "a former member is the cause of its own grant")
	s.NotZero(out.Seq)

	_, err = recordOne(enc, &encounter.RecordInput{Kind: encounter.OutcomeExperienceGained, Actor: "stranger", Experience: grant})
	s.Require().ErrorIs(err, encounter.ErrNotMember, "a never-member is nobody")

	// A bought beat has no attack guard ahead of the actor check, so this is
	// the actor rule itself answering.
	_, err = recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeBought, Actor: goblin, Targets: []encounter.MemberID{alice},
		Trade: &encounter.TradeDetail{ItemType: "weapon", ItemID: "longsword", Quantity: 1},
	})
	s.Require().ErrorIs(err, encounter.ErrNotMember, "every other kind still needs a current member")
}
