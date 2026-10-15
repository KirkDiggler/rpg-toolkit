// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// doorfact_test.go is A DOOR IS A SOURCE (rpg-project#569 R12): a door that
// carries `Opened` teaches that fact when it becomes open — by Open, or by a
// beaten lock, picked or forced — to the door beat's audience: the actor and
// whoever sees the door. Hearers are not taught, a close teaches nothing, and
// a door nobody saw open teaches nobody.
//
// THE NOISE HALL (noise_test.go): three rooms a | b | c, door-ab and door-bc
// on row 5. Alice stands in door-ab's doorway in room a and Sharp watches
// from the same room; Far stands in room c behind door-bc, SHUT, so the wall
// and that door stand between Far and door-ab. Far's ears are sharp enough to
// hear a crash two rooms and a shut door away.

const doorFact encounter.FactID = "cells-opened"

type DoorFactSuite struct{ suite.Suite }

func TestDoorFactSuite(t *testing.T) { suite.Run(t, new(DoorFactSuite)) }

// doorFactSheets gives every member of the hall what the hall asks of them;
// Far's passive beats the crash through two rooms and a shut door (5 + 5 + 5
// + 5 = 20).
var doorFactSheets = sheetFacts{
	noiseAlice: {SpeedFeet: 30},
	noiseSharp: {SpeedFeet: 30},
	noiseFar:   {SpeedFeet: 30, PassivePerception: 25},
}

// doorFactHall is the hall with door-ab in the state given and teaching
// doorFact, door-bc shut, the members given, and any extra endings.
func (s *DoorFactSuite) doorFactHall(ab encounter.DoorState, members []encounter.MemberInput, endings ...encounter.EndingInput) *encounter.Encounter {
	return s.doorFactHallSeeing(everyoneSeesTheWholeMap{}, ab, members, endings...)
}

// doorFactHallSeeing is doorFactHall under the sight model given.
func (s *DoorFactSuite) doorFactHallSeeing(
	sight encounter.Sight, ab encounter.DoorState, members []encounter.MemberInput, endings ...encounter.EndingInput,
) *encounter.Encounter {
	in := noiseHallSetup(ab, encounter.DoorIsClosed(), doorFactSheets, passDriver{}, members...)
	in.Sight = sight
	in.Field.Doors[0].Opened = doorFact
	in.Endings = append(in.Endings, endings...)
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)

	return enc
}

// theWholeHall is Alice at the door, Sharp watching her, Far behind the wall.
func theWholeHall() []encounter.MemberInput {
	return []encounter.MemberInput{
		at(noiseAlice, encounter.KindPlayer, 9, 5),
		at(noiseSharp, encounter.KindPlayer, 4, 5),
		at(noiseFar, encounter.KindMonster, 25, 8),
	}
}

// learnersOf is every member the journal says came to know a fact, sorted,
// one entry per `known:fact` the journal holds — so a member taught twice
// would appear twice.
func learnersOf(enc *encounter.Encounter, fact encounter.FactID) []string {
	data := enc.ToData()
	if data.Journal == nil {
		return nil
	}
	var out []string
	for _, f := range data.Journal.Facts {
		if f.Kind == "known:fact:"+string(fact) {
			out = append(out, f.Subject)
		}
	}
	sort.Strings(out)

	return out
}

// causesOf is the detail every `known:fact` for this fact was written with.
func causesOf(enc *encounter.Encounter, fact encounter.FactID) []string {
	var out []string
	for _, f := range enc.ToData().Journal.Facts {
		if f.Kind == "known:fact:"+string(fact) {
			out = append(out, f.Detail)
		}
	}

	return out
}

var (
	doorFactPick  = encounter.CheckApproach{Ability: "dex", Tool: "thieves-tools", DC: 13}
	doorFactForce = encounter.CheckApproach{Ability: "str", DC: 15}
	doorFactLock  = encounter.DoorIsLocked(encounter.Lock{Approaches: []encounter.CheckApproach{doorFactPick, doorFactForce}})
)

// TestOpenedByHandTeachesTheActorAndAWatchingAlly: Alice opens it, Sharp sees
// it open, Far is behind a wall and a shut door and learns nothing.
func (s *DoorFactSuite) TestOpenedByHandTeachesTheActorAndAWatchingAlly() {
	enc := s.doorFactHall(encounter.DoorIsClosed(), theWholeHall())

	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
	s.Require().NoError(err)

	s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(enc, doorFact),
		"the opener and the ally who watched; not the member behind the wall")
	s.Equal([]string{"opened door door-ab", "opened door door-ab"}, causesOf(enc, doorFact))
}

// TestAGuardWhoSeesTheDoorButNotThePickerLearnsIt is R12's wording, "the
// actor and the members who can see the door": a guard in room b watches the
// door swing open, and the rogue picking it from a hex back in room a is past
// the end of his sight. He learns the door is open; that he cannot name who
// opened it is the door beat's business, not the fact's.
func (s *DoorFactSuite) TestAGuardWhoSeesTheDoorButNotThePickerLearnsIt() {
	guard := at(noiseKeen, encounter.KindMonster, 14, 5)
	rogue := at(noiseAlice, encounter.KindPlayer, 8, 5)
	reach := int(adjacencyDistance(cellAt(14, 5), cellAt(10, 5)))
	s.Require().Greater(int(adjacencyDistance(cellAt(14, 5), cellAt(8, 5))), reach,
		"the rogue stands past the guard's sight, the door's near cell inside it")

	enc := s.doorFactHallSeeing(sightRanges{noiseKeen: reach}, doorFactLock, []encounter.MemberInput{guard, rogue})
	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: true, Total: 17, Applied: doorFactPick,
	})
	s.Require().NoError(err)

	s.Equal([]string{string(noiseAlice), string(noiseKeen)}, learnersOf(enc, doorFact))

	// AND THE BEAT STAYS NARROW: it names its actor, so it goes only to those
	// who saw the picker. The guard knows the door is open and holds no
	// beat saying who opened it.
	s.NotEmpty(s.doorBeatsHeldBy(enc, noiseAlice), "the picker holds the beat of her own deed")
	s.Empty(s.doorBeatsHeldBy(enc, noiseKeen), "the guard who never saw her holds no door beat")
}

// doorBeatsHeldBy is every door beat in one member's story.
func (s *DoorFactSuite) doorBeatsHeldBy(enc *encounter.Encounter, member encounter.MemberID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	var out []map[string]any
	for _, entry := range story {
		beat := map[string]any{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatDoor {
			out = append(out, beat)
		}
	}

	return out
}

// sightRanges is a sight model with a range per member, and no limit for
// anybody it does not name.
type sightRanges map[encounter.MemberID]int

func (r sightRanges) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
		if cells, ok := r[id]; ok {
			out[id] = cells
		}
	}

	return out, nil
}

// TestAPickedLockTeaches: a lock beaten by its tool route is the door opening.
func (s *DoorFactSuite) TestAPickedLockTeaches() {
	enc := s.doorFactHall(doorFactLock, theWholeHall())

	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: true, Total: 17, Applied: doorFactPick,
	})
	s.Require().NoError(err)

	s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(enc, doorFact))
}

// TestAFailedLockTeachesNothing: a lock that held re-states the door as
// itself, and a shut door is not an opening.
func (s *DoorFactSuite) TestAFailedLockTeachesNothing() {
	enc := s.doorFactHall(doorFactLock, theWholeHall())

	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: false, Total: 4, Applied: doorFactPick,
	})
	s.Require().NoError(err)

	s.Empty(learnersOf(enc, doorFact))
}

// TestAForcedLockTeachesTheWitnessesAndOnlySoundsToTheHearer: the crash
// still lands as a sound in Far's ears, and hearing it teaches Far nothing.
func (s *DoorFactSuite) TestAForcedLockTeachesTheWitnessesAndOnlySoundsToTheHearer() {
	enc := s.doorFactHall(doorFactLock, theWholeHall())

	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: true, Total: 17, Applied: doorFactForce,
	})
	s.Require().NoError(err)

	s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(enc, doorFact))
	s.Contains(soundsHeldBy(s.T(), enc, noiseFar), encounter.SoundCrash, "the crash is heard behind the wall")
}

// TestCloseThenReopenTeachesOnce: closing teaches nothing, and the second
// opening teaches nobody who already knows.
func (s *DoorFactSuite) TestCloseThenReopenTeachesOnce() {
	s.Run("closing a door that starts open teaches nobody", func() {
		// NOBODY KNOWS THE FACT YET, so a close that taught would show here
		// — after an opening, learnFact's idempotence would hide it.
		enc := s.doorFactHall(encounter.DoorIsOpen(), theWholeHall())
		_, err := enc.CloseDoor(&encounter.CloseDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
		s.Require().NoError(err)
		s.Empty(learnersOf(enc, doorFact))
	})

	enc := s.doorFactHall(encounter.DoorIsClosed(), theWholeHall())

	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
	s.Require().NoError(err)
	_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
	s.Require().NoError(err)
	s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(enc, doorFact), "a close teaches nothing")

	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
	s.Require().NoError(err)
	s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(enc, doorFact), "and a reopen teaches nobody twice")
}

// TestAHostOpenNobodySawTeachesNobody: no actor and nobody who sees the door
// — Far is the only member, behind the wall — is a fact nobody has. A host
// open that somebody DOES see teaches them: the audience is the beat's.
func (s *DoorFactSuite) TestAHostOpenNobodySawTeachesNobody() {
	s.Run("nobody sees it", func() {
		enc := s.doorFactHall(encounter.DoorIsClosed(), []encounter.MemberInput{at(noiseFar, encounter.KindMonster, 25, 8)})
		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB})
		s.Require().NoError(err)
		s.Empty(learnersOf(enc, doorFact))
	})
	s.Run("somebody sees it", func() {
		enc := s.doorFactHall(encounter.DoorIsClosed(), theWholeHall())
		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB})
		s.Require().NoError(err)
		s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(enc, doorFact),
			"the two who see the door, with no actor to credit")
	})
}

// TestAFactEndingFiresAtTheOpen: an ending waiting on the door's fact fires
// at the opening, picked or forced — and a forced door's crash and the
// actor's world price still land after the run has ended.
func (s *DoorFactSuite) TestAFactEndingFiresAtTheOpen() {
	freed := encounter.EndingInput{Key: "freed", Trigger: encounter.TriggerFact{Fact: doorFact}}
	for name, route := range map[string]encounter.CheckApproach{"picked": doorFactPick, "forced": doorFactForce} {
		s.Run(name, func() {
			enc := s.doorFactHall(doorFactLock, theWholeHall(), freed)
			_, err := enc.Unlock(&encounter.UnlockInput{
				Door: noiseDoorAB, Actor: noiseAlice, Beaten: true, Total: 17, Applied: route,
			})
			s.Require().NoError(err)

			status, err := enc.Status()
			s.Require().NoError(err)
			s.False(status.Open)
			s.Require().NotNil(status.Outcome)
			s.Equal("freed", status.Outcome.Ending)
		})
	}

	s.Run("an open nobody saw ends nothing", func() {
		enc := s.doorFactHall(encounter.DoorIsClosed(), []encounter.MemberInput{at(noiseFar, encounter.KindMonster, 25, 8)}, freed)
		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB})
		s.Require().NoError(err)
		status, err := enc.Status()
		s.Require().NoError(err)
		s.True(status.Open)
	})
}

// TestASavedRunWithADoorFactLoads: the door's fact is one this field can
// mint, so a blob in which somebody learned it loads — and a door saved shut
// still teaches once it is reloaded.
func (s *DoorFactSuite) TestASavedRunWithADoorFactLoads() {
	s.Run("a blob somebody learned the fact in loads", func() {
		enc := s.doorFactHall(encounter.DoorIsClosed(), theWholeHall())
		_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
		s.Require().NoError(err)

		data := enc.ToData()
		var persisted encounter.FactID
		for _, d := range data.Doors {
			if d.ID == noiseDoorAB {
				persisted = d.Opened
			}
		}
		s.Equal(doorFact, persisted, "the door's fact rides the blob")

		loaded := s.reload(data)
		s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(loaded, doorFact))
	})

	s.Run("a door saved shut still teaches after the load", func() {
		enc := s.doorFactHall(encounter.DoorIsClosed(), theWholeHall())
		loaded := s.reload(enc.ToData())
		_, err := loaded.OpenDoor(&encounter.OpenDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
		s.Require().NoError(err)
		s.Equal([]string{string(noiseAlice), string(noiseSharp)}, learnersOf(loaded, doorFact))
	})
}

// reload loads a blob with the hall's own capabilities.
func (s *DoorFactSuite) reload(data encounter.EncounterData) *encounter.Encounter {
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        doorFactSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Roller:        rollsLowest{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return loaded
}
