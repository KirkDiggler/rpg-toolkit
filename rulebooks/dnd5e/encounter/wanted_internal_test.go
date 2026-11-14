// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// wanted_internal_test.go is THE WANTED, SEIZED ON SIGHT (wanted.go,
// rpg-project#925) on the escape's small site: the cell behind a locked leaf
// to the west, the yard to the east, one arresting guard and the captain who
// is the garrison's mind in the yard. A wanted member stands in the yard; a
// bystander, a party member never taken, stands there too.

const (
	wantWanted    MemberID = "wanted"
	wantBystander MemberID = "bystander"
)

// wantedSite builds the hall with the wanted member and the bystander in the
// yard, and an arrest record holding nobody yet and wanting the one member,
// whom the guard holds `wanted` about. Nothing has refreshed since.
func wantedSite(t *testing.T) *Encounter {
	t.Helper()
	enc := escapeSite(t, CustodyHeld, spatial.Position{X: 1, Y: 1})
	enc.interactions["gate"].arrest.Taken = nil
	enc.interactions["gate"].arrest.Wanted = []WantedMember{{ID: wantWanted, RememberedBy: []MemberID{escGuard}}}
	_, err := enc.Join(&JoinInput{Member: wantWanted, Kind: KindPlayer, Cell: enc.field.cellAt(spatial.Position{X: 3, Y: 1})})
	require.NoError(t, err)
	_, err = enc.Join(&JoinInput{Member: wantBystander, Kind: KindPlayer, Cell: enc.field.cellAt(spatial.Position{X: 3, Y: 2})})
	require.NoError(t, err)
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, escGuard, []MemberID{wantWanted}, TopicWanted, ChannelJudgement,
		recognitionLine{interaction: "gate", holdersOnly: true}))
	return enc
}

// refresh runs one sight refresh for everyone, as any verb's own does.
func refresh(t *testing.T, enc *Encounter) {
	t.Helper()
	_, _, err := enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
}

// arrestedBeats is every arrested beat in member's story.
func arrestedBeats(t *testing.T, enc *Encounter, member MemberID) []map[string]any {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: member})
	require.NoError(t, err)
	var out []map[string]any
	for _, e := range story {
		var b map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		if b["beat"] == BeatArrested {
			out = append(out, b)
		}
	}
	return out
}

// TestAWantedMemberSeenIsSeizedAndHeld: the guard sees the wanted member in
// the yard; they leave Wanted, join Taken held in their own right, the guard
// holds `arrested` about them, they are told they are arrested and where to,
// a bystander who saw it is told who and no cell, and they cannot walk away.
// A second refresh seizes nothing again (the castle's test saves and loads).
func TestAWantedMemberSeenIsSeizedAndHeld(t *testing.T) {
	enc := wantedSite(t)
	refresh(t, enc)
	made := enc.interactions["gate"].arrest
	require.Empty(t, made.Wanted, "no longer wanted")
	require.Len(t, made.Taken, 1)
	require.Equal(t, ArrestedMember{ID: wantWanted, SeenBy: []MemberID{escGuard}, Custody: CustodyHeld}, made.Taken[0])
	require.True(t, enc.inCustody(wantWanted), "held where they stand")
	_, held := enc.intelLog.AboutOn(sightMember(escGuard), sightMember(wantWanted), perception.Topic(TopicArrested))
	require.NoError(t, held, "the guard holds arrested about them")

	own := arrestedBeats(t, enc, wantWanted)
	require.Len(t, own, 1)
	require.Equal(t, []any{string(wantWanted)}, own[0]["taken"])
	require.Equal(t, string(escCell), own[0]["cell"], "the taken are told the cell")
	seen := arrestedBeats(t, enc, wantBystander)
	require.Len(t, seen, 1)
	require.Equal(t, []any{string(wantWanted)}, seen[0]["taken"], "a witness is told who")
	require.NotContains(t, seen[0], "cell", "and never the cell")

	_, err := enc.Step(&StepInput{Member: wantWanted, To: enc.field.cellAt(spatial.Position{X: 3, Y: 3}), EndWalk: true})
	require.ErrorIs(t, err, ErrInCustody)

	// A second refresh seizes nothing again.
	refresh(t, enc)
	require.Len(t, arrestedBeats(t, enc, wantWanted), 1, "nothing seizes twice")
}

// TestAWantedMemberUnseenIsNotSeized: unseen is not seized, both ways a
// member is unseen. Behind the shut leaf, a refresh seizes nobody. In the
// guard's line, sneaking with a standing won verdict against him — the
// record a beaten Stealth roll leaves, which every sight reach reads — the
// rule seizes nobody either.
func TestAWantedMemberUnseenIsNotSeized(t *testing.T) {
	enc := escapeSite(t, CustodyHeld, spatial.Position{X: 1, Y: 1})
	made := enc.interactions["gate"].arrest
	made.Taken, made.Wanted = nil, []WantedMember{{ID: wantWanted, RememberedBy: []MemberID{escGuard}}}
	_, err := enc.Join(&JoinInput{Member: wantWanted, Kind: KindPlayer, Cell: enc.field.cellAt(spatial.Position{X: 0, Y: 4})})
	require.NoError(t, err)
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.False(t, reach.Reaches(perception.Sight, escGuard, wantWanted), "precondition: the leaf hides them")
	require.NoError(t, enc.holdTopic(reach, escGuard, []MemberID{wantWanted}, TopicWanted, ChannelJudgement,
		recognitionLine{interaction: "gate", holdersOnly: true}))
	refresh(t, enc)
	require.Len(t, made.Wanted, 1, "behind the leaf: still wanted")

	sneaker := wantedSite(t)
	sneaker.members[wantWanted].Sneaking = true
	sneaker.sneakVerdicts = map[MemberID]map[MemberID]SneakVerdictData{wantWanted: {escGuard: {Beaten: true, InZone: true}}}
	reach, err = sneaker.placedSightReach()
	require.NoError(t, err)
	require.False(t, reach.Reaches(perception.Sight, escGuard, wantWanted), "precondition: the sneak's verdict hides them")
	require.NoError(t, sneaker.sweepWanted())
	require.Len(t, sneaker.interactions["gate"].arrest.Wanted, 1, "sneaking unseen: still wanted")
	require.False(t, sneaker.inCustody(wantWanted))
}

// TestAHostilePairIsFoughtNotSeized: the garrison has turned (the captain
// knows of an escape); the guard sees the wanted member and the fight forms
// — nobody is seized.
func TestAHostilePairIsFoughtNotSeized(t *testing.T) {
	enc := wantedSite(t)
	require.NoError(t, enc.learnFact(escCaptain, escFact, "test", 0))
	require.Equal(t, StanceHostile, enc.stanceOf(escGate, FactionParty), "precondition: the garrison turned")
	refresh(t, enc)
	require.Len(t, enc.interactions["gate"].arrest.Wanted, 1, "a hostile pair is fought, not seized")
	require.False(t, enc.inCustody(wantWanted))
	bubble, err := enc.bubbleFor(wantWanted)
	require.NoError(t, err)
	require.NotNil(t, bubble, "the fight forms")
}

// TestNothingIsAskedWhenNobodyIsWanted: a record with no wanted members asks
// for no sight at all (the sweep's gate), and seizes nobody.
func TestNothingIsAskedWhenNobodyIsWanted(t *testing.T) {
	enc := wantedSite(t)
	enc.interactions["gate"].arrest.Wanted = nil
	refresh(t, enc)
	require.Empty(t, enc.interactions["gate"].arrest.Taken)
	require.False(t, enc.inCustody(wantWanted))
}

// TestAHostileGuardsFirstSightSeizesNobody pins the opposed-pair gate: the
// garrison has turned while the wanted member stood unseen behind the shut
// leaf; on the refresh where the guard first sees them — before that
// refresh's trigger forms the fight — they are fought, not seized.
func TestAHostileGuardsFirstSightSeizesNobody(t *testing.T) {
	enc := escapeSite(t, CustodyHeld, spatial.Position{X: 1, Y: 1})
	made := enc.interactions["gate"].arrest
	made.Taken, made.Wanted = nil, []WantedMember{{ID: wantWanted, RememberedBy: []MemberID{escGuard}}}
	_, err := enc.Join(&JoinInput{Member: wantWanted, Kind: KindPlayer, Cell: enc.field.cellAt(spatial.Position{X: 0, Y: 4})})
	require.NoError(t, err)
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.False(t, reach.Reaches(perception.Sight, escGuard, wantWanted), "precondition: the leaf hides them")
	require.NoError(t, enc.holdTopic(reach, escGuard, []MemberID{wantWanted}, TopicWanted, ChannelJudgement,
		recognitionLine{interaction: "gate", holdersOnly: true}))
	// The free friend in the yard leaves, so the turn forms no fight yet.
	_, err = enc.Exit(&ExitInput{Member: escFriend})
	require.NoError(t, err)
	require.NoError(t, enc.learnFact(escCaptain, escFact, "test", 0))
	require.Equal(t, StanceHostile, enc.stanceOf(escGate, FactionParty), "precondition: the garrison turned")
	fighting, err := enc.inFight(escGuard)
	require.NoError(t, err)
	require.False(t, fighting, "precondition: nobody in sight, so no fight yet")

	_, err = enc.Unlock(&UnlockInput{Door: "leaf", Actor: escPris, Beaten: true, Total: 15,
		Applied: CheckApproach{Ability: "dex", DC: 13}})
	require.NoError(t, err)
	require.Len(t, made.Wanted, 1, "first seen by a hostile guard: not seized")
	require.False(t, enc.inCustody(wantWanted))
}

// TestAGuardInAFightSeizesNobody pins the in-a-fight gate: the guard is
// already fighting when the wanted member comes into his sight; he seizes
// nobody.
func TestAGuardInAFightSeizesNobody(t *testing.T) {
	enc := wantedSite(t)
	_, err := enc.form(&FormInput{Order: []MemberID{escFriend, escGuard, escCaptain}})
	require.NoError(t, err)
	bubble, err := enc.bubbleFor(escGuard)
	require.NoError(t, err)
	require.NotNil(t, bubble, "precondition: the guard is in a fight")
	require.NoError(t, enc.sweepWanted())
	require.Len(t, enc.interactions["gate"].arrest.Wanted, 1, "a guard in a fight seizes nobody")
	require.False(t, enc.inCustody(wantWanted))
}
