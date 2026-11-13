// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_king_hint_test.go is rpg-project#936: the gate guard's answer to a
// stranger asking after their friends — the hint that a friend can speak for
// them — is SHOUTED. It reaches the next room through the gate's open
// doorway, so a friend inside catches the words: without the name when the
// gatehouse's wall is between them, with it when they stand in line with the
// open gate; two rooms off, in the great hall, nobody catches the words.
// These read each member's own story, which is what a player is shown at
// `party: { sharing: own }` (the session's merge is the only thing the dial
// changes).
//
// THE SESSION'S SIGHT. Every scene here is played with everybody seeing
// [sessionSightFeet], the session's own default, through its own build
// ([hintScene]) — not the castle tests' torchlit sixty feet — and the hero
// WALKS where a player walks, the sighting the run keeps checked against a
// fresh look at every step ([kingScene.walkedTo]). So what these tests say
// is what a player gets (rpg-project#936: a placed hero at sixty feet had
// called a cell in plain view of the gate unseen).

// theHint is gate-1's shouted answer to `friends-inside`.
const theHint = "The troupe? Aye, they’re inside. If one of them comes back and speaks for you, in you go."

var (
	// theEastYard is courtyard floor at the east end of the yard, where the
	// gatehouse's wall stands between the hero and both gate guards, one
	// room in from the gatehouse through the gate's open doorway.
	theEastYard = spatial.Position{X: 6, Y: 2}
	// inLineWithTheGate is courtyard floor across the yard in line with the
	// open gate: at the session's sight, in plain view of gate-1.
	inLineWithTheGate = spatial.Position{X: -12, Y: 2}
)

// hintScene is the admitted scene played at the session's sight, built ONCE
// per test binary in [TestMain] for the tests in [hintSceneTests]. The
// shared admitted scene, at the castle tests' sight, is left as it is.
var hintScene struct {
	once      sync.Once
	err       error
	blob      []byte
	inTheHall []byte
	compiled  dungeonspec.Compiled
	host      castleHost
}

// hintSceneTests are the tests that start from [hintScene].
var hintSceneTests = []string{
	"TestMadKingTheHintReachesTheEastYardWithoutAName", "TestMadKingTheHintInLineWithTheGateNamesHim",
	"TestMadKingNothingThatCatchesTheHintActsOnIt", "TestMadKingTheHeroComesBackToVouch",
	"TestMadKingTheHintDoesNotReachTheGreatHall",
}

// buildTheHintScene plays [hintScene].
func buildTheHintScene() {
	hintScene.blob, hintScene.inTheHall, hintScene.compiled, hintScene.host, hintScene.err = playTheAdmittedScene(sessionSightFeet)
}

// hintVersion is one member's version of the hint, as their story holds it.
type hintVersion struct {
	Utterance string `json:"utterance"`
	Speaker   string `json:"speaker"`
	To        string `json:"to"`
	Words     string `json:"words"`
	Loudness  string `json:"loudness"`
	Hops      *int   `json:"hops"`
}

// hintIn is member's versions of the hint after seq from.
func (s *kingScene) hintIn(member encounter.MemberID, from uint64) []hintVersion {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(s.t, err)
	var out []hintVersion
	for _, e := range story {
		if e.Seq < from {
			continue
		}
		var b struct {
			Beat string `json:"beat"`
			hintVersion
		}
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		if b.Beat == encounter.BeatSpeech && b.Words == theHint {
			out = append(out, b.hintVersion)
		}
	}
	return out
}

// placedIn is a stored moment of [hintScene] loaded with members stood where
// given in the record — a placement, for those whose walk is not what a test
// is about (the hero is walked, [kingScene.walkedTo]) — and everyone's sight
// looked again. The hall's door is set as given when state is not empty.
func placedIn(t *testing.T, blob []byte, at map[encounter.MemberID]spatial.Position, hallDoor encounter.DoorStateKind) *kingScene {
	t.Helper()
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(blob, &data))
	for i := range data.Members {
		if cell, ok := at[data.Members[i].ID]; ok {
			data.Members[i].Cell = &encounter.PositionData{X: cell.X, Y: cell.Y}
		}
	}
	if hallDoor != "" {
		found := false
		for i := range data.Doors {
			if data.Doors[i].ID == "the-mad-king/hall-wall-door" {
				data.Doors[i].State, found = string(hallDoor), true
			}
		}
		require.True(t, found, "precondition: the hall's door")
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	s := loadTheSceneOn(t, raw, hintScene.compiled, hintScene.host)
	// Everyone looks again from where they now stand.
	moved := make([]encounter.MemberID, 0, len(at))
	for id := range at {
		moved = append(moved, id)
	}
	_, err = s.enc.Recheck(&encounter.RecheckInput{Members: moved})
	require.NoError(t, err)
	return s
}

// whereTheHeroWasAdmitted is where the admitted scene leaves the hero:
// gatehouse floor within gate-1's speaking reach.
var whereTheHeroWasAdmitted = spatial.Position{X: -5, Y: 9}

// besideGate1 is gatehouse floor in gate-1's sight and reach, where the
// straggler asks.
var besideGate1 = spatial.Position{X: -4, Y: 8}

// askAfterFriends has the straggler, beside gate-1, ask after their friends,
// returning the record's sequence before it.
func (s *kingScene) askAfterFriends() uint64 {
	s.t.Helper()
	require.True(s.t, s.offersTo(straggler, kingGate1, friendsLine), "precondition: gate-1 offers the straggler the line")
	from := s.enc.ToData().Log.NextSeq
	require.NoError(s.t, s.speakAs(straggler, kingGate1, friendsLine, true))
	return from
}

var (
	// eastOfTheGate is courtyard floor east of the gate, still in gate-1's
	// sight through it: where the walk to the east yard starts, short of
	// where the gatehouse's wall comes between them.
	eastOfTheGate = spatial.Position{X: 0, Y: 3}
	// westOfTheGate is courtyard floor west of the gate in gate-1's sight:
	// where the walk to [inLineWithTheGate] starts.
	westOfTheGate = spatial.Position{X: -9, Y: 6}
)

// openTheHintScene loads [hintScene] with the straggler stood beside gate-1
// and anyone else where given.
func openTheHintScene(t *testing.T, blob func() []byte, hallDoor encounter.DoorStateKind, more map[encounter.MemberID]spatial.Position) *kingScene {
	t.Helper()
	hintScene.once.Do(buildTheHintScene)
	require.NoError(t, hintScene.err, "the admitted scene at the session's sight")
	at := map[encounter.MemberID]spatial.Position{straggler: besideGate1}
	for id, cell := range more {
		at[id] = cell
	}
	return placedIn(t, blob(), at, hallDoor)
}

// walkedTo walks the hero, as a player walks, one step at a time along the
// engine's own route to cell, and asserts at EVERY step that the sighting
// the run keeps for her of each gate guard is what a fresh look from her
// cell gives (SeesWithin), both ways round — so a sighting kept past where it
// should end fails here. It returns whether she sees gate-1 at the end.
func (s *kingScene) walkedTo(cell spatial.Position) bool {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: cell, Budget: 200})
	require.NoError(s.t, err)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != cell {
		path = append(path, cell)
	}
	for i, step := range path {
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: step, EndWalk: i == len(path)-1})
		require.NoError(s.t, err, "the step to %v", step)
		for _, guard := range []encounter.MemberID{kingGate1, kingGate2} {
			for _, pair := range [][2]encounter.MemberID{{hero, guard}, {guard, hero}} {
				fresh, known := s.enc.SeesWithin(pair[0], pair[1], 1_000_000)
				require.True(s.t, known)
				require.Equal(s.t, fresh, s.seesLive(pair[0], pair[1]),
					"at %v: the sighting %s keeps of %s is what a fresh look gives", step, pair[0], pair[1])
			}
		}
	}
	require.Equal(s.t, cell, s.at(hero), "the hero stands where she walked")
	return s.seesLive(hero, kingGate1)
}

// theHintWalkedTo is the hint scene with the hero, admitted, stood in the
// yard at from in gate-1's sight, WALKED to cell, and the straggler, beside
// gate-1, asking after their friends: the hint shouted. It returns the scene,
// whether the hero sees gate-1 there, and the record's sequence before the
// question.
func theHintWalkedTo(t *testing.T, from, cell spatial.Position) (*kingScene, bool, uint64) {
	t.Helper()
	s := openTheHintScene(t, func() []byte { return hintScene.blob }, "", map[encounter.MemberID]spatial.Position{hero: from})
	require.True(t, s.seesLive(hero, kingGate1), "precondition: at %v she sees gate-1", from)
	sees := s.walkedTo(cell)
	require.Equal(t, encounter.RegionID("region-courtyard"), s.regionOf(s.at(hero)), "precondition: the hero is in the yard")
	return s, sees, s.askAfterFriends()
}

// theHintInTheEastYard is the hint scene with the hero stood in the east
// yard — out of both gate guards' sight, as the walked test proves — and the
// hint asked, for a test about what follows rather than about her sight.
func theHintInTheEastYard(t *testing.T) (*kingScene, uint64) {
	t.Helper()
	s := openTheHintScene(t, func() []byte { return hintScene.blob }, "", map[encounter.MemberID]spatial.Position{hero: theEastYard})
	require.False(t, s.seesLive(hero, kingGate1), "precondition: out of gate-1's sight")
	return s, s.askAfterFriends()
}

// TestMadKingTheHintReachesTheEastYardWithoutAName: the hero, admitted,
// walks from east of the gate, in gate-1's sight, to the east end of the
// yard, where the gatehouse's
// wall comes between her and both gate guards. The straggler asks after
// their friends and gate-1 shouts the hint: she catches its words with no
// speaker and no one spoken to, from one room off.
func TestMadKingTheHintReachesTheEastYardWithoutAName(t *testing.T) {
	t.Parallel()
	s, sees, from := theHintWalkedTo(t, eastOfTheGate, theEastYard)
	require.False(t, sees, "precondition: out of gate-1's sight")
	require.False(t, s.seesLive(hero, kingGate2), "precondition: and gate-2's")

	got := s.hintIn(hero, from)
	require.Len(t, got, 1, "the hero catches the shouted hint")
	require.Equal(t, "", got[0].Speaker, "from someone she did not see")
	require.Equal(t, "", got[0].To, "said to nobody she saw")
	require.Equal(t, "shout", got[0].Loudness)
	require.NotNil(t, got[0].Hops)
	require.Equal(t, 1, *got[0].Hops, "one room off")
	require.Len(t, s.hintIn(straggler, from), 1, "the straggler, spoken to, catches it too")
}

// TestMadKingTheHintInLineWithTheGateNamesHim: walked across the yard to
// its west, in line with the open gate, the hero still sees gate-1 at the
// session's sight, and the same shout reaches her with his name.
func TestMadKingTheHintInLineWithTheGateNamesHim(t *testing.T) {
	t.Parallel()
	s, sees, from := theHintWalkedTo(t, westOfTheGate, inLineWithTheGate)
	require.True(t, sees, "precondition: gate-1 in her sight through the open gate")
	got := s.hintIn(hero, from)
	require.Len(t, got, 1)
	require.Equal(t, string(kingGate1), got[0].Speaker, "his name")
	require.Nil(t, got[0].Hops, "no distance beside a name")
}

// TestMadKingNothingThatCatchesTheHintActsOnIt: the shout's voice reaches
// every creature within earshot, and none of them acts on it in the round
// after — gate-1 keeps his post, gate-2, the hall's guard, the barracks'
// sergeant and turnkey keep their states, the two reserves wait, and the
// throne room's keep guards hold no voice at all. (Run against the same
// scene with the answer at `speak`, every creature stands and moves exactly
// as here: the only difference the shout makes to them is the voice the
// hall's guard now holds.)
func TestMadKingNothingThatCatchesTheHintActsOnIt(t *testing.T) {
	t.Parallel()
	s, _ := theHintInTheEastYard(t)
	for _, id := range []encounter.MemberID{"hall-1", "sergeant", "turnkey", "reserve-1", "reserve-2"} {
		require.True(t, s.heardKind(id, encounter.SoundVoice), "%s holds the voice", id)
	}
	for _, id := range []encounter.MemberID{kingKeep1, kingKeep2} {
		require.False(t, s.heardKind(id, encounter.SoundVoice), "%s, in the throne room, holds none", id)
	}
	s.aRoundPasses()
	s.onHisPost(kingGate1, "gate-1 keeps his post")
	for id, want := range map[encounter.MemberID]string{
		kingGate2: "ready", "hall-1": "ready", "sergeant": "ready", "turnkey": "ready",
		"reserve-1": "waiting", "reserve-2": "waiting", kingKeep1: "ready", kingKeep2: "ready",
	} {
		require.Equal(t, want, machineState(t, s.enc, id), "%s keeps its state", id)
	}
}

// TestMadKingTheHeroComesBackToVouch: having caught the hint in the east
// yard, the hero comes back to the gate and vouches with the straggler in
// gate-1's sight, and the straggler is let through.
func TestMadKingTheHeroComesBackToVouch(t *testing.T) {
	t.Parallel()
	s, from := theHintInTheEastYard(t)
	require.Len(t, s.hintIn(hero, from), 1, "precondition: the hero caught the hint")
	s.walkMember(hero, whereTheHeroWasAdmitted, false)
	require.True(t, s.offersTo(hero, kingGate1, vouchLine), "precondition: back at the gate, the vouch is offered")
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	require.True(t, s.recognizes(kingGate1, straggler), "the vouch admits the straggler")
	require.Empty(t, s.heldAgainst(straggler), "and the gate lets them through")
}

// theHallWithItsDoor is [hintScene] with the hero beside the captain in the
// great hall — two rooms in from the gatehouse — the straggler beside gate-1,
// and the hall's door as given.
func theHallWithItsDoor(t *testing.T, state encounter.DoorStateKind) *kingScene {
	t.Helper()
	s := openTheHintScene(t, func() []byte { return hintScene.inTheHall }, state, nil)
	require.Equal(t, encounter.RegionID("region-great-hall"), s.regionOf(s.at(hero)), "precondition: the hero is in the great hall")
	return s
}

// speechSince is every speech version in member's story after seq from.
func (s *kingScene) speechSince(member encounter.MemberID, from uint64) []string {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: member, AfterSeq: from})
	require.NoError(s.t, err)
	var out []string
	for _, e := range story {
		var b struct {
			Beat string `json:"beat"`
		}
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		if b.Beat == encounter.BeatSpeech {
			out = append(out, string(e.Payload))
		}
	}
	return out
}

// TestMadKingTheHintDoesNotReachTheGreatHall: the hero in the great hall,
// two rooms in from the gatehouse. Behind its CLOSED door she is shown
// nothing of the shout at all. With the door standing open (as the captain
// leaves it) the words still do not reach her — a shout carries one room.
// At the session's sight she may SEE the straggler speak, through the open
// doors and across the yard, and so be shown who spoke and to whom; never
// the words, and never the hint at all.
func TestMadKingTheHintDoesNotReachTheGreatHall(t *testing.T) {
	t.Parallel()
	t.Run("behind the closed door, nothing", func(t *testing.T) {
		t.Parallel()
		s := theHallWithItsDoor(t, encounter.DoorClosed)
		from := s.askAfterFriends()
		require.Empty(t, s.speechSince(hero, from), "no version of anything said at the gate")
	})
	t.Run("through the open door, no words", func(t *testing.T) {
		t.Parallel()
		s := theHallWithItsDoor(t, encounter.DoorOpen)
		from := s.askAfterFriends()
		require.Empty(t, s.hintIn(hero, from), "two rooms off, no words of the hint")
		for _, v := range s.speechSince(hero, from) {
			var b hintVersion
			require.NoError(t, json.Unmarshal([]byte(v), &b))
			require.Empty(t, b.Words, "no words of anything said at the gate: %s", v)
		}
	})
}

// regionOf is the authored region a cell is in.
func (s *kingScene) regionOf(cell spatial.Position) encounter.RegionID {
	s.t.Helper()
	id, ok := s.enc.RegionAt(cell)
	require.True(s.t, ok, "%v is floor", cell)
	return id
}
