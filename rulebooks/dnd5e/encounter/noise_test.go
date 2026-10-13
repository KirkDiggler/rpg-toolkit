// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// noise_test.go is HEARING THROUGH THE REAL VERBS (rpg-project#569, R4/R5):
// a swing recorded through RecordTrain clangs, and whoever's passive
// Perception beats the clang's DC plus five a hop holds it — on the sounds
// channel, as a kind and a place, never a name.
//
// THE CASTLE IN MINIATURE: three rooms in a row, a | b | c, a door in each
// seam on row 5. A clang is DC 5 where it is made, 10 one room over, 15 two
// rooms over, and a closed door on the way counts as one more room.

const (
	noiseAlice  encounter.MemberID = "alice"
	noiseThug   encounter.MemberID = "thug"
	noiseKeen   encounter.MemberID = "keen"
	noiseDull   encounter.MemberID = "dull"
	noiseSharp  encounter.MemberID = "sharp"
	noiseFar    encounter.MemberID = "far"
	noiseDoorAB encounter.DoorID   = "door-ab"
	noiseDoorBC encounter.DoorID   = "door-bc"
)

// noiseDoor is a door in the seam east of column col, on row 5.
func noiseDoor(id encounter.DoorID, col int, state encounter.DoorState) encounter.DoorInput {
	return encounter.DoorInput{
		ID:    id,
		Edges: []encounter.DoorEdge{{From: cellAt(col, 5), To: cellAt(col+1, 5)}},
		State: state,
	}
}

// noiseHall is the three rooms with the two doors given, and a member list.
// Alice and the thug she hits stand in room a; who listens, and where, is
// the scene's. Nobody acts on a table: the driver passes.
func noiseHall(t *testing.T, ab, bc encounter.DoorState, sheets sheetFacts, members ...encounter.MemberInput) *encounter.Encounter {
	t.Helper()

	return buildHall(t, ab, bc, sheets, passDriver{}, members...)
}

// tableHall is the same hall, both doors open, with every creature doing
// what its own table says when it has time — the first eligible row, always.
func tableHall(t *testing.T, sheets sheetFacts, members ...encounter.MemberInput) *encounter.Encounter {
	t.Helper()

	return buildHall(t, encounter.DoorIsOpen(), encounter.DoorIsOpen(), sheets, tableDriver(), members...)
}

// buildHall is the one constructor both halls share.
func buildHall(
	t *testing.T, ab, bc encounter.DoorState, sheets sheetFacts, driver encounter.Driver,
	members ...encounter.MemberInput,
) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(noiseHallSetup(ab, bc, sheets, driver, members...))
	require.NoError(t, err)

	return enc
}

// noiseHallSetup is the hall's construction input, for the scene that wants to
// see what the door says about it rather than to stand in it.
func noiseHallSetup(
	ab, bc encounter.DoorState, sheets sheetFacts, driver encounter.Driver, members ...encounter.MemberInput,
) *encounter.SetupInput {
	walls := append(squareSeamWall(9, 10, 5), squareSeamWall(19, 10, 5)...)

	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{
				rectRegion("room-a", 0, 0, 10, 10),
				rectRegion("room-b", 10, 0, 10, 10),
				rectRegion("room-c", 20, 0, 10, 10),
			},
			Walls:    walls,
			Doors:    []encounter.DoorInput{noiseDoor(noiseDoorAB, 9, ab), noiseDoor(noiseDoorBC, 19, bc)},
			Factions: []encounter.FactionInput{{ID: hallCastle}, {ID: hallThugs}},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{hallCastle, encounter.FactionParty}, Stance: encounter.StanceNeutral},
				{Between: [2]encounter.FactionID{hallThugs, encounter.FactionParty}, Stance: encounter.StanceNeutral},
			},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
			Roller:        rollsLowest{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	}
}

// The hall's two factions, both NEUTRAL to the party: a creature in no
// faction is opposed to the party, and a fight that formed on first sight
// would put every listener on the turn clock before anybody made a sound.
// The thug has a faction of its own, so the swing that turns the thugs
// hostile leaves the castle's guards at peace and on the world clock.
const (
	hallCastle encounter.FactionID = "castle"
	hallThugs  encounter.FactionID = "thugs"
)

// at places a member at an authored [col,row] — a member's position is
// authored the way its region's cells are, and the encounter converts it. A
// monster stands in the castle's faction, the thug in its own.
func at(id encounter.MemberID, kind encounter.MemberKind, col, row int) encounter.MemberInput {
	m := encounter.MemberInput{ID: id, Kind: kind, Position: spatial.Position{X: float64(col), Y: float64(row)}}
	switch {
	case id == noiseThug:
		m.Faction = hallThugs
	case kind == encounter.KindMonster:
		m.Faction = hallCastle
	}

	return m
}

// heldSound is one sound a member holds, as stored: the payload's own keys,
// and the clock reading it was landed at.
type heldSound struct {
	payload map[string]any
	at      uint64
}

// soundsHeldBy reads every sound a member holds out of the saved blob, keyed
// by kind — what a reload would hand the creature's table.
func soundsHeldBy(t *testing.T, enc *encounter.Encounter, member encounter.MemberID) map[string]heldSound {
	t.Helper()
	out := map[string]heldSound{}
	for _, h := range enc.ToData().Perception.Intel.Holdings[core.EntityID("member|"+string(member))] {
		if string(h.Channel) != "sounds" {
			continue
		}
		var payload map[string]any
		require.NoError(t, json.Unmarshal(h.Payload, &payload))
		out[payload["kind"].(string)] = heldSound{payload: payload, at: h.Confirmed}
	}

	return out
}

// aliceStrikesTheThug records alice's swing at the thug through the real
// train, which is the door every attack outcome comes in through.
func aliceStrikesTheThug(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: noiseAlice, Targets: []encounter.MemberID{noiseThug},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 4},
	})
	require.NoError(t, err)
}

type NoiseSuite struct{ suite.Suite }

func TestNoiseSuite(t *testing.T) { suite.Run(t, new(NoiseSuite)) }

// TestAClangCarriesOneOpenDoorToTheKeenEarOnly is the plan's first case, at
// the walk's tuning: an attack in room a, heard in room b through one open
// door by a guard with passive 10 (DC 5 + 5 for the hop) and not by one with
// passive 9.
func (s *NoiseSuite) TestAClangCarriesOneOpenDoorToTheKeenEarOnly() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 10}, noiseDull: {SpeedFeet: 30, PassivePerception: 9},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1), at(noiseDull, encounter.KindMonster, 15, 8),
	)

	aliceStrikesTheThug(s.T(), enc)

	keen := soundsHeldBy(s.T(), enc, noiseKeen)
	s.Require().Contains(keen, encounter.SoundClang, "passive 10 beats DC 5 + 5 for one hop")
	s.Empty(soundsHeldBy(s.T(), enc, noiseDull), "passive 9 does not beat 10")
	s.Empty(soundsHeldBy(s.T(), enc, noiseThug),
		"a passive of zero hears nothing, even beside the swing — the sheet that answered nothing is deaf, fail closed")
}

// TestAClosedDoorIsOneMoreHop: the same clang behind a closed door needs 15.
func (s *NoiseSuite) TestAClosedDoorIsOneMoreHop() {
	enc := noiseHall(s.T(), encounter.DoorIsClosed(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 10}, noiseSharp: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1), at(noiseSharp, encounter.KindMonster, 15, 8),
	)

	aliceStrikesTheThug(s.T(), enc)

	s.Empty(soundsHeldBy(s.T(), enc, noiseKeen), "10 hears it through an open door and does not through a shut one")
	s.Contains(soundsHeldBy(s.T(), enc, noiseSharp), encounter.SoundClang, "15 = 5 + 5 for the room + 5 for the door")
}

// TestTwoRoomsAwayIsTwoHops: the far room hears at 15, not at 10.
func (s *NoiseSuite) TestTwoRoomsAwayIsTwoHops() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 10}, noiseFar: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 25, 1), at(noiseFar, encounter.KindMonster, 25, 8),
	)

	aliceStrikesTheThug(s.T(), enc)

	s.Empty(soundsHeldBy(s.T(), enc, noiseKeen), "10 is one room's worth, and this is two")
	s.Contains(soundsHeldBy(s.T(), enc, noiseFar), encounter.SoundClang)
}

// TestTheHeldSoundNamesNoActor is R4 at the storage grain: the holding is a
// kind and a place, and nothing in it says who swung.
func (s *NoiseSuite) TestTheHeldSoundNamesNoActor() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1),
	)

	aliceStrikesTheThug(s.T(), enc)

	clang, ok := soundsHeldBy(s.T(), enc, noiseKeen)[encounter.SoundClang]
	s.Require().True(ok)
	keys := make([]string, 0, len(clang.payload))
	for k := range clang.payload {
		keys = append(keys, k)
	}
	s.ElementsMatch([]string{"type", "kind", "where"}, keys, "a kind and a place, and no third thing")
	raw, err := json.Marshal(clang.payload)
	s.Require().NoError(err)
	s.NotContains(string(raw), string(noiseAlice), "the maker is nowhere in what was heard")
	where := clang.payload["where"].(map[string]any)
	s.Equal(cellAt(2, 5).X, where["x"], "the sound is where the swing was made")
	s.Equal(cellAt(2, 5).Y, where["y"])
}

// TestTheMakerDoesNotHearItself: alice's ears are the sharpest in the hall,
// and the clang she made is still not something she heard.
func (s *NoiseSuite) TestTheMakerDoesNotHearItself() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30, PassivePerception: 30}, noiseThug: {SpeedFeet: 30, PassivePerception: 10},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
	)

	aliceStrikesTheThug(s.T(), enc)

	s.Empty(soundsHeldBy(s.T(), enc, noiseAlice), "the maker knows what it did; it did not hear it")
	s.Contains(soundsHeldBy(s.T(), enc, noiseThug), encounter.SoundClang, "the one beside her did, at DC 5")
}

// TestAMissClangsToo: a swing that misses is still steel in the air.
func (s *NoiseSuite) TestAMissClangsToo() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1),
	)

	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: noiseAlice, Targets: []encounter.MemberID{noiseThug},
	})
	s.Require().NoError(err)

	s.Contains(soundsHeldBy(s.T(), enc, noiseKeen), encounter.SoundClang)
}

// forceOrPick is a lock with the two routes a castle door has: shoulder it,
// or pick it.
var (
	noiseShoulder = encounter.CheckApproach{Ability: "str", DC: 15}
	noisePick     = encounter.CheckApproach{Ability: "dex", Tool: "thieves-tools", DC: 15}
)

// lockedHall is the hall with door-ab LOCKED, alice standing in its doorway
// and the keen guard listening in room b.
func (s *NoiseSuite) lockedHall() *encounter.Encounter {
	return noiseHall(s.T(),
		encounter.DoorIsLocked(encounter.Lock{Approaches: []encounter.CheckApproach{noiseShoulder, noisePick}}),
		encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 9, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1),
	)
}

// TestAForcedDoorCrashes: a lock beaten by Strength is a door broken in, and
// the room beyond hears it — DC 5, plus 5 for the hop through the door that
// is now open.
func (s *NoiseSuite) TestAForcedDoorCrashes() {
	enc := s.lockedHall()

	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: true, Total: 17, Applied: noiseShoulder,
	})
	s.Require().NoError(err)

	s.Contains(soundsHeldBy(s.T(), enc, noiseKeen), encounter.SoundCrash)
}

// TestAPickedOrFailedLockIsSilent: the same door picked, or shouldered and
// held, makes no crash — a door forced is the deed that sounds.
func (s *NoiseSuite) TestAPickedOrFailedLockIsSilent() {
	picked := s.lockedHall()
	_, err := picked.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: true, Total: 17, Applied: noisePick,
	})
	s.Require().NoError(err)
	s.Empty(soundsHeldBy(s.T(), picked, noiseKeen), "a lock picked is quiet work")

	held := s.lockedHall()
	_, err = held.Unlock(&encounter.UnlockInput{
		Door: noiseDoorAB, Actor: noiseAlice, Beaten: false, Total: 9, Applied: noiseShoulder,
	})
	s.Require().NoError(err)
	s.Empty(soundsHeldBy(s.T(), held, noiseKeen), "a door that did not give did not crash")
}

// castHall is the hall with the both-ways scene's scout standing in room a for
// alice to cast at, and the keen guard listening in room b.
func (s *NoiseSuite) castHall() *encounter.Encounter {
	return noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, bwScout: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(bwScout, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1),
	)
}

// TestADamagingCastBooms: a spell that delivered harm is a boom — DC 0, so
// the next room hears it at 5 — and it is not also a clang: the cast lands
// the same `attacked` deed a swing does, but it does not sound like one.
func (s *NoiseSuite) TestADamagingCastBooms() {
	enc := s.castHall()

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: noiseAlice, Spell: bwMockery,
		Targets: []encounter.CastTargetResult{{
			Target: bwScout, Save: bwSave(false),
			Results: []encounter.ActivationResult{bwPsychic()},
		}},
	})
	s.Require().NoError(err)

	heard := soundsHeldBy(s.T(), enc, noiseKeen)
	s.Contains(heard, encounter.SoundBoom)
	s.NotContains(heard, encounter.SoundClang, "a spell is a boom, not steel")
}

// TestAHarmlessCastIsSilent: the same cantrip saved against delivers nothing,
// and nothing delivered makes no boom — the deed still lands (R5 both ways),
// the noise does not.
func (s *NoiseSuite) TestAHarmlessCastIsSilent() {
	enc := s.castHall()

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: noiseAlice, Spell: bwMockery,
		Targets: []encounter.CastTargetResult{{Target: bwScout, Save: bwSave(true)}},
	})
	s.Require().NoError(err)

	s.Empty(soundsHeldBy(s.T(), enc, noiseKeen))
}

// TestABodyHearsNothing: a guard who is down has the ears for it and hears
// nothing — the same down set every other sense reads.
func (s *NoiseSuite) TestABodyHearsNothing() {
	setup := noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15}, noiseSharp: {SpeedFeet: 30, PassivePerception: 15},
		},
		passDriver{},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1), at(noiseSharp, encounter.KindMonster, 15, 8),
	)
	setup.Participation = &downList{down: []encounter.MemberID{noiseSharp}}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	aliceStrikesTheThug(s.T(), enc)

	s.Contains(soundsHeldBy(s.T(), enc, noiseKeen), encounter.SoundClang, "the control: the same ears, standing")
	s.Empty(soundsHeldBy(s.T(), enc, noiseSharp), "down, it heard nothing")
}

// TestAMonsterHearsButIsToldNothing: the `heard` beat is a player's (H5). A
// monster that hears holds the sound — its table reads it — and its own story
// carries no beat about it.
func (s *NoiseSuite) TestAMonsterHearsButIsToldNothing() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		at(noiseKeen, encounter.KindMonster, 15, 1),
	)

	aliceStrikesTheThug(s.T(), enc)

	s.Require().Contains(soundsHeldBy(s.T(), enc, noiseKeen), encounter.SoundClang, "precondition: it heard")
	for _, beat := range storyBeats(s.T(), enc, noiseKeen) {
		s.NotEqual(encounter.BeatHeard, beat["beat"], "a monster is never handed a heard beat")
	}
}

// TestABoomCarriesTwoRoomsAndZeroIsStillDeaf: a boom is DC 0 where it goes
// off, so a passive-10 guard two rooms away hears it (0 + 5 + 5) — and a
// sheet that answered no passive at all, standing right beside the caster,
// hears nothing, because zero is deaf and not merely hard of hearing.
func (s *NoiseSuite) TestABoomCarriesTwoRoomsAndZeroIsStillDeaf() {
	enc := noiseHall(s.T(), encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, bwScout: {SpeedFeet: 30},
			noiseFar: {SpeedFeet: 30, PassivePerception: 10}, noiseDull: {SpeedFeet: 30},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(bwScout, encounter.KindMonster, 3, 5),
		at(noiseFar, encounter.KindMonster, 25, 1), at(noiseDull, encounter.KindMonster, 2, 6),
	)

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: noiseAlice, Spell: bwMockery,
		Targets: []encounter.CastTargetResult{{
			Target: bwScout, Save: bwSave(false),
			Results: []encounter.ActivationResult{bwPsychic()},
		}},
	})
	s.Require().NoError(err)

	s.Contains(soundsHeldBy(s.T(), enc, noiseFar), encounter.SoundBoom, "two rooms: 0 + 5 + 5 = 10")
	s.Empty(soundsHeldBy(s.T(), enc, noiseDull), "zero hears nothing, even a boom beside it")
	s.Empty(soundsHeldBy(s.T(), enc, bwScout), "nor does the target, whose sheet answered no passive either")
}
