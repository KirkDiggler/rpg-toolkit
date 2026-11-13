// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_king_arrest_test.go is the gate's arrest, slice one (rpg-project#925,
// ruled 2026-11-13): the second failed claim arrests, and what it decides is
// decided once and kept — who is taken, who is wanted — while nobody moves
// yet. Played at the session's sight ([sessionSightFeet]), so who the gate
// speakers see is what a player gets.

// The party at the arrest: the speaker, and one ally in each of the three
// ways a member can stand.
const (
	// inTheArea follows the hero into the gatehouse: taken, in the area.
	inTheArea = encounter.MemberID("in-the-area")
	// seenOutside waits on the road in the gate speakers' sight, outside the
	// area: taken, seen.
	seenOutside = encounter.MemberID("seen-outside")
	// leftTheGate came to the gate with the others and walked back out of
	// sight before the second claim: wanted.
	leftTheGate = encounter.MemberID("left-the-gate")
	// neverNear stays at the party start, where no gate speaker ever sees
	// them: neither.
	neverNear = encounter.MemberID("never-near")
)

var (
	// onTheRoadInSight is road outside the arrest area that the captain,
	// gate-1 and gate-2 all see through the gatehouse's south doorway.
	onTheRoadInSight = spatial.Position{X: -8, Y: 15}
	// onTheRoadOutOfSight is road outside the area that no gate speaker sees.
	onTheRoadOutOfSight = spatial.Position{X: -6, Y: 16}
)

// arrestScene is the castle played to the arrest, ONCE per test binary in
// [TestMain] — the stranger, the walks and both claims would otherwise be on
// each test's clock — and loaded back for each test.
var arrestScene struct {
	once     sync.Once
	err      error
	blob     []byte
	before   map[encounter.MemberID]spatial.Position
	compiled dungeonspec.Compiled
	host     castleHost
}

// arrestSceneTests are the tests that start from [arrestScene].
var arrestSceneTests = []string{
	"TestMadKingTheSecondFailedClaimArrests", "TestMadKingTheArrestEndsTheAddressAndMovesNobody",
	"TestMadKingTheArrestIsToldOnlyWhatItsReaderSees", "TestMadKingTheArrestIsKeptOnReload",
}

// buildTheArrestScene plays the castle to the arrest: the hero learns of the
// feast from the stranger; the allies take their places; the hero fails the
// guest claim at the captain; the ally who came to the gate walks back out
// of sight; the hero fails the troupe claim. It keeps the run and each party
// member's cell just before the second claim.
func buildTheArrestScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			arrestScene.err = fmt.Errorf("the arrest scene: %s", tb.failed)
		}
	}()
	start := partyStart(tb)
	ally := func(id encounter.MemberID, dx float64) encounter.MemberInput {
		return encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: spatial.Position{X: start.X + dx, Y: start.Y}}
	}
	s := openTheCastleAs(tb, castleHost{at: start, sightFt: sessionSightFeet, players: []encounter.MemberInput{
		ally(inTheArea, 1), ally(seenOutside, 2), ally(leftTheGate, -1), ally(neverNear, -2),
	}})
	for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		require.False(tb, s.seesLive(g, neverNear), "precondition: %s does not see the party start", g)
	}

	s.walkTo(s.at(kingPedlar))
	s.said(kingPedlar, "tonight", "persuasion", true)
	s.walkTo(s.at(kingCaptain))
	s.walkMember(inTheArea, s.at(hero), false)
	s.walkMember(leftTheGate, s.at(hero), false)
	require.True(tb, s.seesLive(kingCaptain, leftTheGate), "precondition: the captain sees the ally who came to the gate")
	s.walkMember(seenOutside, onTheRoadInSight, false)

	s.said(kingCaptain, "guest", "persuasion", false)
	require.Equal(tb, "warned-guest", s.entrance(), "precondition: the first claim warned")

	s.walkMember(leftTheGate, onTheRoadOutOfSight, false)
	for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		require.False(tb, s.seesLive(g, leftTheGate), "precondition: %s no longer sees the one who left", g)
	}

	before := map[encounter.MemberID]spatial.Position{}
	for _, m := range []encounter.MemberID{hero, inTheArea, seenOutside, leftTheGate, neverNear} {
		before[m] = s.at(m)
	}
	s.said(kingCaptain, "troupe", "performance", false)
	require.Equal(tb, "arrested", s.entrance(), "the second claim arrests")
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	arrestScene.blob, arrestScene.before, arrestScene.compiled, arrestScene.host = blob, before, s.compiled, s.host
}

// theArrest loads the arrest scene into a fresh run, with each party
// member's cell just before the second claim.
func theArrest(t *testing.T) (*kingScene, map[encounter.MemberID]spatial.Position) {
	t.Helper()
	arrestScene.once.Do(buildTheArrestScene)
	require.NoError(t, arrestScene.err)
	return loadTheSceneOn(t, arrestScene.blob, arrestScene.compiled, arrestScene.host), arrestScene.before
}

// arrested is the castle's one arrest record.
func (s *kingScene) arrested() encounter.ArrestRecord {
	s.t.Helper()
	made, ok := s.enc.Arrests()["front-gate"]
	require.True(s.t, ok, "the front gate arrested")
	return made
}

// takenIDs and wantedIDs are a record's members.
func takenOf(r encounter.ArrestRecord) map[encounter.MemberID]encounter.ArrestedMember {
	out := map[encounter.MemberID]encounter.ArrestedMember{}
	for _, m := range r.Taken {
		out[m.ID] = m
	}
	return out
}

func wantedOf(r encounter.ArrestRecord) map[encounter.MemberID]encounter.WantedMember {
	out := map[encounter.MemberID]encounter.WantedMember{}
	for _, m := range r.Wanted {
		out[m.ID] = m
	}
	return out
}

// TestMadKingTheSecondFailedClaimArrests is the split the arrest makes: the
// speaker and the ally beside him in the gatehouse are taken as in the area;
// the ally on the road in the gate speakers' sight is taken as seen; the ally
// who came to the gate and left before the second claim is wanted, by the
// guards who saw them; the ally who never came near is neither. The captain
// holds `arrested` about the taken and `wanted` about the wanted, and the
// gate speakers who caught the line hold the same, heard.
func TestMadKingTheSecondFailedClaimArrests(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	made := s.arrested()
	require.Equal(t, "arrested", made.State)
	require.Equal(t, encounter.RegionID("region-cells"), made.Cell)

	taken := takenOf(made)
	require.Len(t, taken, 3, "the speaker, the ally in the area, the ally seen: %+v", made.Taken)
	require.True(t, taken[hero].InArea, "the speaker stood in the gatehouse")
	require.True(t, taken[inTheArea].InArea, "so did the ally beside him")
	require.False(t, taken[seenOutside].InArea, "the ally on the road stood outside the area")
	require.NotEmpty(t, taken[seenOutside].SeenBy, "and is taken because the gate speakers saw them")
	require.Contains(t, taken[seenOutside].SeenBy, kingCaptain)

	wanted := wantedOf(made)
	require.Len(t, wanted, 1, "only the one who left: %+v", made.Wanted)
	require.Contains(t, wanted[leftTheGate].RememberedBy, kingCaptain, "the captain saw them at the gate")
	_, isTaken := taken[neverNear]
	_, isWanted := wanted[neverNear]
	require.False(t, isTaken || isWanted, "the ally who never came near is neither")

	for _, m := range []encounter.MemberID{hero, inTheArea, seenOutside} {
		require.True(t, s.recognizesOn(kingCaptain, m, intel.Topic(encounter.TopicArrested)), "the captain arrested %s", m)
		require.False(t, s.recognizesOn(kingCaptain, m, intel.Topic(encounter.TopicWanted)), "and does not want %s", m)
	}
	require.True(t, s.recognizesOn(kingCaptain, leftTheGate, intel.Topic(encounter.TopicWanted)), "the captain wants the one who left")
	require.False(t, s.recognizesOn(kingCaptain, neverNear, intel.Topic(encounter.TopicWanted)), "and nobody he never saw")
	require.False(t, s.recognizesOn(kingCaptain, neverNear, intel.Topic(encounter.TopicArrested)))
}

// TestMadKingTheArrestEndsTheAddressAndMovesNobody: the conversation is over
// — the gate addresses nobody — and slice one moves nobody: every member of
// the party stands where they stood before the second claim.
func TestMadKingTheArrestEndsTheAddressAndMovesNobody(t *testing.T) {
	t.Parallel()
	s, before := theArrest(t)
	for _, d := range s.enc.ToData().Interactions {
		require.Nil(t, d.Address, "the arrest ended the gate's address")
	}
	for id, cell := range before {
		require.Equal(t, cell, s.at(id), "%s did not move", id)
	}
}

// TestMadKingTheArrestIsToldOnlyWhatItsReaderSees: the arrested beat names
// to each reader only the members they perceive — the speaker in the
// gatehouse is told who stands beside him, and the one who left only if he
// sees them himself; the one who left is told the arrests they watch, and
// never that they are wanted.
func TestMadKingTheArrestIsToldOnlyWhatItsReaderSees(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(t, err)
	var told []map[string]any
	for _, e := range story {
		var b map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		if b["beat"] == encounter.BeatArrested {
			told = append(told, b)
		}
	}
	require.Len(t, told, 1, "the speaker is told the arrest once")
	require.Contains(t, told[0]["taken"], string(hero), "he is told he is taken")
	require.Contains(t, told[0]["taken"], string(inTheArea), "and the ally beside him")
	if s.seesLive(hero, leftTheGate) {
		require.Contains(t, told[0]["wanted"], string(leftTheGate), "he sees the one who left, so he is told they are wanted")
	} else {
		require.Nil(t, told[0]["wanted"], "never the one who left, whom he does not see")
	}
	for _, seen := range told[0]["taken"].([]any) {
		id := encounter.MemberID(seen.(string))
		require.True(t, id == hero || s.seesLive(hero, id), "he is told only whom he sees: %s", id)
	}
	require.Equal(t, "region-cells", told[0]["cell"])

	away, err := s.enc.Story(&encounter.StoryInput{Audience: leftTheGate})
	require.NoError(t, err)
	for _, e := range away {
		var b map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		if b["beat"] == encounter.BeatRecognized && b["topic"] == string(encounter.TopicWanted) {
			require.NotContains(t, b["subjects"], string(leftTheGate), "nor told, through the guards' record, that they are wanted")
		}
		if b["beat"] != encounter.BeatArrested {
			continue
		}
		require.Nil(t, b["wanted"], "the one who left is never told they are wanted")
		for _, seen := range b["taken"].([]any) {
			require.True(t, s.seesLive(leftTheGate, encounter.MemberID(seen.(string))),
				"watching from the road, they are told only the arrests they see: %s", seen)
		}
	}
}

// TestMadKingTheArrestIsKeptOnReload: the record survives a save and a load,
// and is never decided again — a later change of positions, after the load,
// changes nothing in it.
func TestMadKingTheArrestIsKeptOnReload(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	made := s.arrested()
	s.roundTrip()
	require.Equal(t, made, s.arrested(), "the arrest comes back as it was made")
	s.walkMember(neverNear, s.at(hero), false)
	require.Equal(t, made, s.arrested(), "and positions changing afterwards change nothing in it")
	require.True(t, s.recognizesOn(kingCaptain, leftTheGate, intel.Topic(encounter.TopicWanted)), "the guard still wants the one who left")
}
