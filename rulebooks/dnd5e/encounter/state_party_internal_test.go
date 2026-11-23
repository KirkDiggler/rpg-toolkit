// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// state_party_internal_test.go pins A STATE NOTES ABOUT THE PARTY IT SEES, AND
// THOSE IT SAW WHO ARE GONE (rpg-project#1134, slice rpg-project#1135): a
// machine's state may give its note about `party_in_sight` (the party of the
// one the beat names whom the creature sees now) and about `the_gone` (the
// ones of that party it does not see now and holds a sight record about).
// Internal, because the intel store and the sight refresh are the
// composition's own.
//
// HOW THE TESTS MOVE THE GUARD'S SIGHT: [spSight] answers the guard a range the
// test sets, and a sight refresh ([Encounter.refreshSight]) after the range
// changes is what makes someone come into or go out of his view. The sight
// record a guard holds about someone he once saw is kept when they go out of
// range. The positions are along one row, the guard at the post (2,1): alice
// and dave one cell off, bob two, carol far.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	spNear = 1 // sees alice and dave, no one further
	spWide = 4 // sees alice, bob and dave, never carol
)

// spSight is everyone seeing the whole map but the guard, whose range the
// test sets.
type spSight struct{ guard int }

// Sight answers the guard's range and the whole map for the rest.
func (s *spSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out, _ := everyoneSeesTheWholeMap{}.Sight(members)
	if _, ok := out[soGuard]; ok {
		out[soGuard] = s.guard
	}

	return out, nil
}

func spPerson(id MemberID, x float64) MemberInput {
	return MemberInput{ID: id, Kind: KindPlayer, Position: spatial.Position{X: x, Y: 1}}
}

// spDave is in no faction.
func spDave() MemberInput {
	return MemberInput{ID: "dave", Kind: KindWorld, Position: spatial.Position{X: 3, Y: 1}}
}

// spGive is a state's note of nvTopic as value, about the word.
func spGive(as, about string) Consequence {
	return Consequence{Recognize: &Recognition{
		Topic: nvTopic, As: as, Holder: RecognitionHolder(WhoItself), About: RecognitionSubjects(about),
	}}
}

// spHall is the posted hall: a guard who notes outcomes on wary, at the sight
// range given, beside alice (one cell off), bob (two), carol (far) and dave.
func spHall(t *testing.T, outcomes []Consequence, guardInParty bool) (*Encounter, *spSight, *SetupInput) {
	t.Helper()
	g := soGuardIn(t, map[string][]Consequence{"wary": outcomes})
	if guardInParty {
		g.Faction = FactionParty
	}
	sight := &spSight{guard: spWide}
	in := postedSetup(sight, g, spPerson("bob", 4), spPerson("carol", 9), spDave())
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc, sight, in
}

// spRange sets the guard's range and has the world look again.
func spRange(t *testing.T, enc *Encounter, sight *spSight, cells int) {
	t.Helper()
	sight.guard = cells
	_, _, err := enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
}

// spNoted is the value the guard holds nvTopic as about the member, and
// whether he holds it at all.
func spNoted(t *testing.T, enc *Encounter, about MemberID) (string, bool) {
	t.Helper()
	held, err := enc.intelLog.AboutOn(sightMember(soGuard), sightMember(about), perceptionTopic(nvTopic))
	if err != nil {
		return "", false
	}
	var why recognitionTestimony
	require.NoError(t, json.Unmarshal(held.Payload, &why))

	return why.As, true
}

// spNotes is who the guard holds nvTopic about, as the value held, in the
// members given.
func spNotes(t *testing.T, enc *Encounter, among ...MemberID) map[MemberID]string {
	t.Helper()
	out := map[MemberID]string{}
	for _, id := range among {
		if as, ok := spNoted(t, enc, id); ok {
			out[id] = as
		}
	}

	return out
}

// spSays has the member say the guard's checked line and miss: the guard
// hears a failed claim, steps to wary, and notes about the one he heard.
func spSays(t *testing.T, enc *Encounter, actor MemberID) {
	t.Helper()
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Range: 10, Actor: actor, Target: soGuard, OptionID: soLine, Roller: wakeRollsOne{},
		Skill: "persuasion", Beaten: false, DC: soCheckDC, Total: 4,
	})
	require.NoError(t, err)
}

var spEveryone = []MemberID{"alice", "bob", "carol", "dave", soGuard}

func TestStatePartyTheOneNamedPartyHeSees(t *testing.T) {
	enc, _, _ := spHall(t, []Consequence{spGive("barred", string(RecognitionPartyInSight))}, false)

	spSays(t, enc, "alice")

	require.Equal(t, "wary", stateOf(enc, soGuard))
	require.Equal(t, map[MemberID]string{"alice": "barred", "bob": "barred"}, spNotes(t, enc, spEveryone...),
		"alice and bob are in his sight and in alice's party; carol is not in his sight; dave is no one of the party")
}

func TestStatePartyThoseWhoAreGone(t *testing.T) {
	enc, sight, _ := spHall(t, []Consequence{spGive("wanted", WhoTheGone)}, false)
	spRange(t, enc, sight, spWide)
	spRange(t, enc, sight, spNear) // bob goes out of his sight; his record is kept

	spSays(t, enc, "alice")

	require.Equal(t, "wary", stateOf(enc, soGuard))
	require.Equal(t, map[MemberID]string{"bob": "wanted"}, spNotes(t, enc, spEveryone...),
		"bob was seen and is not now; alice is seen now; carol was never seen")
}

func TestStatePartyBothInOneState(t *testing.T) {
	enc, sight, _ := spHall(t, []Consequence{
		spGive("barred", string(RecognitionPartyInSight)), spGive("wanted", WhoTheGone),
	}, false)
	spRange(t, enc, sight, spNear)

	spSays(t, enc, "alice")

	require.Equal(t, map[MemberID]string{"alice": "barred", "bob": "wanted"}, spNotes(t, enc, spEveryone...),
		"alice barred, bob wanted, carol nothing")
}

func TestStatePartyAPartyOfOne(t *testing.T) {
	seen, _, _ := spHall(t, []Consequence{spGive("barred", string(RecognitionPartyInSight))}, false)
	spSays(t, seen, "dave")
	require.Equal(t, map[MemberID]string{"dave": "barred"}, spNotes(t, seen, spEveryone...),
		"dave is in no faction: his party is dave alone")

	gone, sight, _ := spHall(t, []Consequence{spGive("wanted", WhoTheGone)}, false)
	spRange(t, gone, sight, spNear)
	spSays(t, gone, "dave")
	require.Empty(t, spNotes(t, gone, spEveryone...), "dave is seen, and he is the whole party: nobody is gone")
}

func TestStatePartyTheCreatureIsInNeither(t *testing.T) {
	enc, sight, _ := spHall(t, []Consequence{
		spGive("barred", string(RecognitionPartyInSight)), spGive("wanted", WhoTheGone),
	}, true)
	spRange(t, enc, sight, spNear)

	spSays(t, enc, "alice")

	require.Equal(t, map[MemberID]string{"alice": "barred", "bob": "wanted"}, spNotes(t, enc, spEveryone...),
		"he is of the party himself and notes nothing about himself")
}

func TestStatePartyTheEnginesCheck(t *testing.T) {
	g := soGuardIn(t, nil)
	about := func(word string) map[string][]Consequence {
		return map[string][]Consequence{"wary": {spGive("barred", word)}}
	}

	t.Run("a word a state may not note about", func(t *testing.T) {
		err := validateStateOutcomes(soRecipe(t), about(string(RecognitionSpeaker)))
		require.Error(t, err)
		for _, word := range []string{WhoTheOneNamed, string(RecognitionPartyInSight), WhoTheGone} {
			require.Contains(t, err.Error(), word, "the refusal names the words a state may note about")
		}
		_, err = CheckBehaviors(&CheckBehaviorsInput{Behaviors: *g.Behaviors, Outcomes: about(string(RecognitionSpeaker))})
		require.ErrorIs(t, err, ErrBadMember)
	})

	timed := recipe(t, "calm", map[string]machine.State{
		"calm": {Transitions: []machine.Transition{{On: beatTime, To: "wary"}}},
		"wary": {Transitions: []machine.Transition{{On: beatTime, To: "calm"}}},
	})
	for _, word := range []string{string(RecognitionPartyInSight), WhoTheGone, WhoTheOneNamed} {
		t.Run("on a beat that names nobody: "+word, func(t *testing.T) {
			err := validateStateOutcomes(timed, about(word))
			require.Error(t, err)
			require.Contains(t, err.Error(), "`"+word+"`", "the sentence says the word written")
			require.Contains(t, err.Error(), "names nobody")
		})
	}
}

func TestStatePartySavedAndLoaded(t *testing.T) {
	enc, sight, in := spHall(t, []Consequence{
		spGive("barred", string(RecognitionPartyInSight)), spGive("wanted", WhoTheGone),
	}, false)
	spRange(t, enc, sight, spNear)
	spSays(t, enc, "alice")

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err)

	require.Equal(t, map[MemberID]string{"alice": "barred", "bob": "wanted"}, spNotes(t, loaded, spEveryone...),
		"the notes are there after a load")
}
