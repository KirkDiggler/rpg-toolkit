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
	// hireling is a creature authored into the party's faction, standing in
	// the gatehouse: the party is its players, so it is neither taken nor
	// wanted.
	hireling = encounter.MemberID("hireling")
)

var (
	// onTheRoadInSight is road outside the arrest area that the captain,
	// gate-1 and gate-2 all see through the gatehouse's south doorway.
	onTheRoadInSight = spatial.Position{X: -8, Y: 15}
	// onTheRoadOutOfSight is road outside the area that no gate speaker sees.
	onTheRoadOutOfSight = spatial.Position{X: -6, Y: 16}
	// furtherDownTheRoad is road outside the area that no gate speaker sees,
	// where the speaker walks to from a kept roll.
	furtherDownTheRoad = spatial.Position{X: -6, Y: 17}
)

// arrestScene is the castle played to the arrest, ONCE per test binary in
// [TestMain] — the stranger, the walks and both claims would otherwise be on
// each test's clock — and loaded back for each test.
var arrestScene struct {
	once sync.Once
	err  error
	blob []byte
	// beforeClaim is the run just before the second claim is said.
	beforeClaim []byte
	before      map[encounter.MemberID]spatial.Position
	compiled    dungeonspec.Compiled
	host        castleHost
}

// arrestSceneTests are the tests that start from [arrestScene].
var arrestSceneTests = []string{
	"TestMadKingTheSecondFailedClaimArrests", "TestMadKingTheArrestEndsTheAddressAndMovesNobody",
	"TestMadKingTheArrestIsToldOnlyWhatItsReaderSees", "TestMadKingTheArrestIsKeptOnReload",
	"TestMadKingAKeptRollArrestsAsTheWordsWereSaid",
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
		{ID: hireling, Kind: encounter.KindMonster, Faction: encounter.FactionParty, Position: gatehouseSeat},
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
	beforeClaim, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	arrestScene.beforeClaim = beforeClaim
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
	_, isTaken = taken[hireling]
	_, isWanted = wanted[hireling]
	require.False(t, isTaken || isWanted, "a creature of the party's faction, in the gatehouse, is neither: the party is its players")

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

// payloadsOf is every payload of one beat in member's story.
func (s *kingScene) payloadsOf(member encounter.MemberID, beat string) []map[string]any {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(s.t, err)
	var out []map[string]any
	for _, e := range story {
		var b map[string]any
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		if b["beat"] == beat {
			out = append(out, b)
		}
	}
	return out
}

// TestMadKingTheArrestIsToldOnlyWhatItsReaderSees: the arrested beat names
// to each player only the taken they perceive, and themselves when taken;
// who is wanted is the guards' memory, so no player is told it — not the
// taken speaker, not a taken ally who sees the one who left, not the one who
// left — on the arrest's beat or through the guards' records. The arresting
// guards are told whom they want.
func TestMadKingTheArrestIsToldOnlyWhatItsReaderSees(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)

	told := s.payloadsOf(hero, encounter.BeatArrested)
	require.Len(t, told, 1, "the speaker is told the arrest once")
	require.Contains(t, told[0]["taken"], string(hero), "he is told he is taken")
	require.Contains(t, told[0]["taken"], string(inTheArea), "and the ally beside him")
	require.Equal(t, "region-cells", told[0]["cell"])

	for _, player := range []encounter.MemberID{hero, inTheArea, seenOutside, leftTheGate, neverNear} {
		for _, b := range s.payloadsOf(player, encounter.BeatArrested) {
			require.Nil(t, b["wanted"], "%s is told nobody is wanted", player)
			for _, seen := range b["taken"].([]any) {
				id := encounter.MemberID(seen.(string))
				require.True(t, id == player || s.seesLive(player, id), "%s is told only the taken they see: %s", player, id)
			}
		}
		for _, b := range s.payloadsOf(player, encounter.BeatRecognized) {
			require.NotEqual(t, string(encounter.TopicWanted), b["topic"], "%s reads no guard's record of the wanted", player)
		}
	}
	require.True(t, s.seesLive(hero, leftTheGate) || s.seesLive(inTheArea, leftTheGate) || s.seesLive(seenOutside, leftTheGate),
		"precondition: a taken friend watches the one who left, and is still not told")

	captain := s.payloadsOf(kingCaptain, encounter.BeatArrested)
	require.Len(t, captain, 1)
	require.Equal(t, []any{string(leftTheGate)}, captain[0]["wanted"], "the captain is told whom he wants")
	for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		var wanted bool
		for _, b := range s.payloadsOf(g, encounter.BeatRecognized) {
			if b["topic"] == string(encounter.TopicWanted) {
				require.Equal(t, []any{string(leftTheGate)}, b["subjects"], "%s is told his own holding", g)
				require.Equal(t, string(g), b["holder"], "and only his own")
				wanted = true
			}
		}
		require.True(t, wanted, "%s holds the wanted and is told so", g)
	}
}

// TestMadKingAKeptRollArrestsAsTheWordsWereSaid: the second claim waits on
// the speaker's roll, and the speaker walks down the road out of every
// guard's sight before the host settles it, while the one who left walks
// back into the captain's sight. The arrest is decided as the words were
// said (rpg-project#938's rule): the speaker is taken, standing in the
// gatehouse then; the one who left is not taken for walking into view, and
// stays wanted.
func TestMadKingAKeptRollArrestsAsTheWordsWereSaid(t *testing.T) {
	t.Parallel()
	arrestScene.once.Do(buildTheArrestScene)
	require.NoError(t, arrestScene.err)
	s := loadTheSceneOn(t, arrestScene.beforeClaim, arrestScene.compiled, arrestScene.host)
	require.Equal(t, "warned-guest", s.entrance(), "precondition: one claim failed")

	at := s.revisionOf(hero, kingCaptain, "troupe")
	_, err := s.enc.Reserve(&encounter.ReserveInput{
		Actor: hero, Target: kingCaptain, OptionID: "troupe", Revision: at, Range: speakReach,
	})
	require.NoError(t, err, "the claim waits on the speaker's roll")

	s.walkMember(leftTheGate, onTheRoadInSight, false)
	require.True(t, s.seesLive(kingCaptain, leftTheGate), "precondition: the one who left is back in the captain's sight")
	s.walkMember(hero, furtherDownTheRoad, false)
	for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		require.False(t, s.seesLive(g, hero), "precondition: %s no longer sees the speaker", g)
	}
	var abandoned bool
	for _, d := range s.enc.ToData().Interactions {
		if d.Reservation != nil {
			abandoned = d.Reservation.Abandoned
		}
	}
	require.True(t, abandoned, "precondition: walking away abandons the hold")

	s.roundTrip()
	_, err = s.enc.Speak(t.Context(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: kingCaptain, OptionID: "troupe", Roller: rollsOne{},
		Revision: at, Skill: "performance", Beaten: false, DC: 15, Total: 3,
	})
	require.NoError(t, err, "the host settles the roll as rolled")
	require.Equal(t, "arrested", s.entrance())

	made := s.arrested()
	taken, wanted := takenOf(made), wantedOf(made)
	require.Contains(t, taken, hero, "the speaker is taken where the words were said")
	require.True(t, taken[hero].InArea, "in the gatehouse, then")
	require.NotContains(t, taken, leftTheGate, "walking into view before the settle takes nobody")
	require.Contains(t, wanted, leftTheGate, "the one who left stays wanted")
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
