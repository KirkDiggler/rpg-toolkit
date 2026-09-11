// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// SightedBeatTestSuite guards the per-recipient sighting beat: one observer
// being told that WHO THEY CAN SEE changed.
//
// Every case runs on one set — the same 12x12 hall split by a wall across
// y=6 that beatorder_test.go uses — because the beat is about crossing a
// sightline, and that set is the one place in this package where a sightline
// can genuinely be broken and restored by walking.
//
// Alice starts at (6,2) and the goblin at (6,10), directly across the wall's
// span from each other: they cannot see each other at first light, so every
// transition below is caused by the verb under test rather than inherited
// from setup.
type SightedBeatTestSuite struct {
	suite.Suite
}

func TestSightedBeatSuite(t *testing.T) {
	suite.Run(t, new(SightedBeatTestSuite))
}

// sighting is one decoded sighted beat, with the audience it was written for.
type sighting struct {
	audience []encounter.MemberID
	gained   []string
	lost     []string
	changed  []string
	seq      uint64
}

// sightingsOf reads every sighted beat in one member's story.
func (s *SightedBeatTestSuite) sightingsOf(
	enc *encounter.Encounter, audience encounter.MemberID,
) []sighting {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)

	out := make([]sighting, 0)
	for _, entry := range story {
		var beat struct {
			Beat    string   `json:"beat"`
			Gained  []string `json:"gained"`
			Lost    []string `json:"lost"`
			Changed []string `json:"changed"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat != encounter.BeatSighted {
			continue
		}
		out = append(out, sighting{
			audience: entry.Audience, gained: beat.Gained, lost: beat.Lost,
			changed: beat.Changed, seq: entry.Seq,
		})
	}
	return out
}

// keys reads the raw JSON object keys of one member's sighted beats, which is
// how the "omitted, not empty" claim is checked — a decode into a struct
// cannot tell an absent key from a null one.
func (s *SightedBeatTestSuite) sightedKeys(
	enc *encounter.Encounter, audience encounter.MemberID,
) []map[string]any {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)

	out := make([]map[string]any, 0)
	for _, entry := range story {
		var raw map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &raw))
		if raw["beat"] == encounter.BeatSighted {
			out = append(out, raw)
		}
	}
	return out
}

// blocked is the shared set: the two of them across the wall from each other,
// seeing nothing.
func (s *SightedBeatTestSuite) blocked() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// NOTHING CHANGED, SO NOTHING WAS SAID. The wall keeps them apart at first
// light, so neither of them is owed a beat — and this is the case that makes
// the beat worth receiving at all. Without it the composition would append
// one per observer per refresh forever and silence would mean nothing.
func (s *SightedBeatTestSuite) TestNoTransitionAppendsNoBeat() {
	enc := s.blocked()

	s.Empty(s.sightingsOf(enc, alice), "alice saw nobody arrive and nobody leave")
	s.Empty(s.sightingsOf(enc, goblin))
}

// FIRST LIGHT DOES EMIT, when there is something to say. Standing in the open
// they see each other from the opening frame, and each is told about their own
// side of it.
func (s *SightedBeatTestSuite) TestFirstLightTellsEachObserverWhatTheySee() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			// Both clear of the wall's span.
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 1, "one beat, for the one thing that changed")
	s.Equal([]string{string(goblin)}, hers[0].gained)
	s.Empty(hers[0].lost)

	its := s.sightingsOf(enc, goblin)
	s.Require().Len(its, 1)
	s.Equal([]string{string(alice)}, its[0].gained)
}

// THE AUDIENCE IS THE OBSERVER, AND NOBODY ELSE. This is the property the
// whole beat exists for: what alice can see is news for alice. A roster-wide
// audience here would publish the sight graph to everyone, which is the one
// thing a perception beat must never do.
func (s *SightedBeatTestSuite) TestTheBeatIsAddressedToItsObserverAlone() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().NotEmpty(hers)
	for _, beat := range hers {
		s.Equal([]encounter.MemberID{alice}, beat.audience,
			"alice's sighting beat goes to alice and stops there")
	}

	its := s.sightingsOf(enc, goblin)
	s.Require().NotEmpty(its, "the goblin now sees her too, and is told separately")
	for _, beat := range its {
		s.Equal([]encounter.MemberID{goblin}, beat.audience)
	}
}

// STEPPING INTO VIEW IS A GAIN — the first-contact half.
func (s *SightedBeatTestSuite) TestSteppingIntoViewIsGained() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 1)
	s.Equal([]string{string(goblin)}, hers[0].gained, "she walked past the wall and there it was")
	s.Empty(hers[0].lost)
}

// STEPPING OUT OF VIEW IS A LOSS, and the subject becomes a ghost rather than
// vanishing — which is what makes the return below a return.
func (s *SightedBeatTestSuite) TestSteppingOutOfViewIsLost() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 2, "one beat for arriving in view, one for leaving it")
	s.Equal([]string{string(goblin)}, hers[1].lost, "back behind the wall")
	s.Empty(hers[1].gained)
}

// THE RETURN. A ghost becoming real again is its own beat, and this is the
// case the whole slice was built for — Kirk's "when the ghost becomes a
// reality again, how do I know what they're holding?". Without
// intel.SurveilOutput.Reacquired this pass is an ordinary refresh and says
// nothing at all.
func (s *SightedBeatTestSuite) TestSteppingBackIntoViewIsGainedAgain() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 3, "into view, out of view, back into view")
	s.Equal([]string{string(goblin)}, hers[2].gained,
		"the ghost is current again, and she is told so")
	s.Empty(hers[2].lost)
}

// A WALK THAT CHANGES NOTHING SAYS NOTHING. She moves one cell while the wall
// still stands between them — a refresh happened, and no beat came of it.
func (s *SightedBeatTestSuite) TestAStepThatChangesNoPerceptionIsSilent() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(5, 2)})
	s.Require().NoError(err)

	s.Empty(s.sightingsOf(enc, alice), "still behind the wall, still nothing to report")
	s.Empty(s.sightingsOf(enc, goblin))
}

// OMITTED, NOT EMPTY. An absent key is "this did not happen"; an empty list
// invites a reader to think the composition looked and found none, which is a
// different claim and one this beat never makes.
func (s *SightedBeatTestSuite) TestTheHalfThatDidNotHappenIsOmitted() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	raw := s.sightedKeys(enc, alice)
	s.Require().Len(raw, 1)
	s.Contains(raw[0], "gained", "somebody came into view")
	s.NotContains(raw[0], "lost", "nobody left it — so the key is absent, not empty")
}

// CAUSE BEFORE EFFECT, the law refreshSight states, applied to this beat: the
// fight forms BECAUSE she saw it, so the seeing has to be readable first.
func (s *SightedBeatTestSuite) TestTheSightingPrecedesTheFightItCauses() {
	enc := s.blocked()

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "walking into the open puts them in contact")

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 1)
	s.Greater(out.Formed.Seq, hers[0].seq, "she sees it, THEN the fight starts")
	s.Greater(hers[0].seq, out.Seq, "and the step that caused it is ahead of both")
}

// THE STORY ENTRY IS TAGGED AS ONE, so a host can route perception beats
// without decoding every payload — the same courtesy the reveal beats extend.
func (s *SightedBeatTestSuite) TestTheBeatCarriesItsOwnTag() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)

	tagged := make([]record.Entry, 0)
	for _, entry := range story {
		if entry.Tags["tag"] == "sight" {
			tagged = append(tagged, entry)
		}
	}
	s.Require().Len(tagged, 1)
	var beat map[string]any
	s.Require().NoError(json.Unmarshal(tagged[0].Payload, &beat))
	s.Equal(encounter.BeatSighted, beat["beat"])
}

// TestTheNamesAreDeterministic is the guard for the defect CI caught and a
// local run did not.
//
// A percept is built by ranging the member map, so the order of everything
// downstream of it — play/intel's FirstContact, a host's Discovered, and this
// beat's own lists — was whatever that range happened to produce. Two runs of
// one scene were emitting gained:["bob","goblin"] and gained:["goblin","bob"].
// A story is a transcript; two runs of one scene must read the same.
//
// Three members in the open, so each observer gains TWO at once and an
// unordered pair has somewhere to show. Repeated, because Go randomises map
// iteration per run rather than per range — one pass could agree by luck.
func (s *SightedBeatTestSuite) TestTheNamesAreDeterministic() {
	scene := func() []string {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight:     everyoneSeesTheWholeMap{},
			Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field: wallRoom(),
			Members: []encounter.MemberInput{
				// Declared out of alphabetical order on purpose: a beat
				// that echoed declaration order would pass a sorted
				// assertion only by accident.
				{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
				{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		hers := s.sightingsOf(enc, alice)
		s.Require().Len(hers, 1)
		return hers[0].gained
	}

	first := scene()
	s.Require().Len(first, 2, "she sees both of them at first light")
	s.Equal([]string{string(bob), string(goblin)}, first, "named in one settled order")

	for i := 0; i < 40; i++ {
		s.Equal(first, scene(), "run %d told a different story", i)
	}
}

// TestGainedIsSortedAcrossBothItsHalves pins the beat's own sort, which the
// percept's cannot stand in for.
//
// gained concatenates TWO lists — first contacts, then re-acquisitions — so
// even a perfectly ordered percept leaves it grouped by a distinction this
// beat deliberately does not draw. Deterministic, and still not sorted.
//
// The scene is built so the two halves are in the wrong order without the
// sort: the member who RETURNS sorts first, and the member seen for the
// FIRST time sorts last, so a bare concatenation reads ["zzz…", "aaa…"].
func (s *SightedBeatTestSuite) TestGainedIsSortedAcrossBothItsHalves() {
	const (
		returning = encounter.MemberID("aaa-lurker")    // fades and comes back
		newcomer  = encounter.MemberID("zzz-latecomer") // arrives while she is blind
	)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: returning, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Out past the wall and back, so the lurker is a ghost she has already met.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)

	// The latecomer arrives on the far side of the wall, where she cannot
	// see them — so they are still unmet when she walks back out.
	_, err = enc.Join(&encounter.JoinInput{
		Member: newcomer, Kind: encounter.KindPlayer,
		Cell: cellAt(6, 11), SpeedFeet: 30, SightFeet: 60,
	})
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().NotEmpty(hers)
	last := hers[len(hers)-1]
	s.Equal([]string{string(returning), string(newcomer)}, last.gained,
		"one returning and one met for the first time, named in one sorted list")
}

// TestThePerceptItselfIsOrdered guards the fix at the SOURCE rather than at
// this beat — the sighting beat's own sort would hide it.
//
// rebuildPercepts used to build each percept by ranging the member map, so
// play/intel reported FirstContact and Refreshed in no order at all, and a
// host reading those as Discovered inherited the randomness. This asserts the
// delta directly, which is the shape every other consumer of a verb's output
// sees.
func (s *SightedBeatTestSuite) TestThePerceptItselfIsOrdered() {
	const (
		nearer  = encounter.MemberID("aaa-second")
		further = encounter.MemberID("zzz-first")
	)

	seen := func() []string {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight:     everyoneSeesTheWholeMap{},
			Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field: wallRoom(),
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
				// Declared with the later name first, so declaration order
				// cannot be mistaken for sorted order.
				{ID: further, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
				{ID: nearer, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 10}},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
		s.Require().NoError(err)

		delta := out.IntelDeltas[alice]
		s.Require().NotNil(delta, "she stepped past the wall and met two people")

		names := make([]string, 0, len(delta.FirstContact))
		for _, report := range delta.FirstContact {
			names = append(names, string(report.Subject))
		}
		return names
	}

	first := seen()
	s.Require().Len(first, 2, "both of them were behind the wall and are not now")
	s.Equal([]string{string(nearer), string(further)}, first,
		"the percept names them in one settled order, not the map's")

	for i := 0; i < 40; i++ {
		s.Equal(first, seen(), "run %d built a differently ordered percept", i)
	}
}

// open is the shared set for the declared-change cases: the two of them clear
// of the wall's span, watching each other from the opening frame.
func (s *SightedBeatTestSuite) open() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// sightingsAfter reads the sighted beats appended after the given seq, so a
// case can look at what ITS verb produced rather than at first light's.
func (s *SightedBeatTestSuite) sightingsAfter(
	enc *encounter.Encounter, audience encounter.MemberID, after uint64,
) []sighting {
	s.T().Helper()
	out := make([]sighting, 0)
	for _, beat := range s.sightingsOf(enc, audience) {
		if beat.seq > after {
			out = append(out, beat)
		}
	}
	return out
}

// lastSeq is the highest seq in one member's story, which is where a case
// draws the line between setup and the verb under test.
func (s *SightedBeatTestSuite) lastSeq(enc *encounter.Encounter, audience encounter.MemberID) uint64 {
	s.T().Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	if len(story) == 0 {
		return 0
	}
	return story[len(story)-1].Seq
}

// A WATCHER IS TOLD. The plain case the verb exists for: somebody's gear
// changed on a sheet this module cannot read, and the person looking at them
// learns their own picture is stale.
func (s *SightedBeatTestSuite) TestARecheckTellsWhoeverCanSeeTheSubject() {
	enc := s.open()
	mark := s.lastSeq(enc, alice)

	_, err := enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)

	told := s.sightingsAfter(enc, alice, mark)
	s.Require().Len(told, 1, "one beat, for the one thing she is owed")
	s.Equal([]string{string(goblin)}, told[0].changed)
	s.Empty(told[0].gained, "nobody arrived — she could already see it")
	s.Empty(told[0].lost)
	s.Equal([]encounter.MemberID{alice}, told[0].audience)
}

// NOBODY ELSE IS. The subject is not in their own percept, so a member never
// hears that they themselves changed — they are the one who did it.
func (s *SightedBeatTestSuite) TestTheSubjectIsNotToldAboutItself() {
	enc := s.open()
	mark := s.lastSeq(enc, goblin)

	_, err := enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)

	s.Empty(s.sightingsAfter(enc, goblin, mark),
		"it changed its own hands; it does not need telling what is in them")
}

// A DECLARED CHANGE NOBODY CAN SEE IS SILENCE. The wall stands between them,
// so the re-look happens and appends nothing at all — the scoping is per
// observer, not a broadcast of the fact.
func (s *SightedBeatTestSuite) TestARecheckNobodyCanSeeSaysNothing() {
	enc := s.blocked()
	mark := s.lastSeq(enc, alice)

	_, err := enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)

	s.Empty(s.sightingsAfter(enc, alice, mark), "she cannot see it, so there is no news for her")
}

// THE GHOST IS NOT TOLD, and this is the case the whole snapshot model exists
// to protect. Alice holds the goblin as a memory of the moment she last saw
// it. If a re-look reached her, her ghost would acquire news she never
// witnessed — which is exactly the leak rpg-toolkit#1615 asks us not to make
// while closing it.
func (s *SightedBeatTestSuite) TestAGhostHolderIsNotTold() {
	enc := s.blocked()

	// Out past the wall to meet it, then back behind the wall so what she
	// holds is a ghost.
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)
	mark := s.lastSeq(enc, alice)

	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)

	s.Empty(s.sightingsAfter(enc, alice, mark),
		"her picture of it is a memory, and a memory does not get updates")
}

// ONE PIECE OF NEWS, SAID ONCE. A subject who changed while out of sight and
// is walked back into view is a re-acquisition: gained already says "look
// again", so changed must not say it a second time in the same beat.
func (s *SightedBeatTestSuite) TestAReturningSubjectIsGainedAndNotAlsoChanged() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)
	mark := s.lastSeq(enc, alice)

	// It changes while she cannot see it -- silence, as above -- and then she
	// walks back out and re-acquires it.
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	told := s.sightingsAfter(enc, alice, mark)
	s.Require().Len(told, 1, "the change said nothing; the return is the only beat")
	s.Equal([]string{string(goblin)}, told[0].gained)
	s.Empty(told[0].changed, "gained already means look again")
}

// AN ORDINARY REFRESH DECLARES NOTHING, so no other verb can produce a
// changed list by accident. She steps in front of it and the beat is a plain
// first contact.
func (s *SightedBeatTestSuite) TestAnOrdinaryVerbNeverReportsAChange() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	told := s.sightingsOf(enc, alice)
	s.Require().Len(told, 1)
	s.Empty(told[0].changed, "a step declares nothing about anybody's sheet")

	raw := s.sightedKeys(enc, alice)
	s.Require().Len(raw, 1)
	s.NotContains(raw[0], "changed", "omitted, not empty")
}

// The door: nil input, an empty list, a stranger, and a scene that is over.
func (s *SightedBeatTestSuite) TestRecheckRefusesWhatItCannotDo() {
	enc := s.open()

	_, err := enc.Recheck(nil)
	s.Require().ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.Recheck(&encounter.RecheckInput{})
	s.Require().ErrorIs(err, encounter.ErrNoMember, "a re-look of nobody is a caller mistake")

	_, err = enc.Recheck(&encounter.RecheckInput{
		Members: []encounter.MemberID{goblin, "a-stranger"},
	})
	s.Require().ErrorIs(err, encounter.ErrNotMember,
		"one stranger beside a member refuses the whole call, not half of it")

	// ...and the refusal was total: the member named beside the stranger got
	// no re-look of their own (R5 atomicity).
	s.Empty(s.sightingsAfter(enc, alice, s.lastSeq(enc, alice)))

	_, err = enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().ErrorIs(err, encounter.ErrClosed, "the run is over; nobody is watching anybody")
}

// TestAReacquisitionInTheSAMEPassIsGainedOnly is the narrow case the
// exclusion is actually for, and the one a walk cannot stage: a subject who
// is BOTH declared changed AND re-acquired on the same refresh.
//
// Staged through the sight capability rather than by walking, because that is
// the only thing that can move a subject in and out of view without a verb of
// its own. Reach is asked fresh every refresh (see sightList), so shrinking it
// fades the goblin and restoring it brings the goblin back — on a pass that is
// also carrying the declaration.
//
// Both facts are true of the goblin on that pass. gained already means "look
// again", so the beat must say it once.
func (s *SightedBeatTestSuite) TestAReacquisitionInTheSamePassIsGainedOnly() {
	reach := &sightList{reach: map[encounter.MemberID]int{}, fallback: 60}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     reach,
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.Require().Len(s.sightingsOf(enc, alice), 1, "she sees it at first light")

	// Her light fails. The re-look fades the goblin for her.
	reach.reach[alice] = 1
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)
	dark := s.sightingsOf(enc, alice)
	s.Require().Len(dark, 2)
	s.Equal([]string{string(goblin)}, dark[1].lost, "it went dark, so she lost it")
	s.Empty(dark[1].changed, "and a subject she cannot see is not one she is told about")

	// Her light returns on the very pass that declares the goblin changed.
	reach.reach[alice] = 60
	mark := s.lastSeq(enc, alice)
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{goblin}})
	s.Require().NoError(err)

	told := s.sightingsAfter(enc, alice, mark)
	s.Require().Len(told, 1)
	s.Equal([]string{string(goblin)}, told[0].gained, "it came back")
	s.Empty(told[0].changed,
		"and it is not ALSO reported as changed — gained already means look again")
}
