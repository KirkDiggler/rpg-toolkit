// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// then_internal_test.go is a disposition's `then` (rpg-project#717): the
// stance a pair's `until` turns it to. Omitted, the other of hostile and
// neutral, as before; `then: allied` makes a recruited faction the party's
// side, through its mind or publicly, and an allied pair that names where it
// goes may now carry an until.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// thenEnc is a yard with the hero, a veteran who is his faction's mind, and
// a thug who is his, under the caller's dispositions.
func thenEnc(t *testing.T, dispositions ...DispositionInput) *Encounter {
	t.Helper()
	enc, err := NewEncounter(thenSetup(dispositions))
	require.NoError(t, err)

	return enc
}

// thenSetup is [thenEnc]'s setup, for a test that wants the refusal.
func thenSetup(dispositions []DispositionInput) *SetupInput {
	return &SetupInput{
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:       CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:      []RegionInput{rectRegion("yard", 0, 0, 16, 4)},
			Factions:     []FactionInput{{ID: "veterans", Mind: "veteran"}, {ID: "thugs", Mind: "thug"}},
			Dispositions: dispositions,
		},
		Members: []MemberInput{
			{ID: "veteran", Kind: KindMonster, Faction: "veterans", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "thug", Kind: KindMonster, Faction: "thugs", Position: spatial.Position{X: 14, Y: 3}},
			{ID: "hero", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        sheetFacts{"veteran": {SpeedFeet: 30}, "thug": {SpeedFeet: 30}, "hero": {SpeedFeet: 30}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	}
}

// joined is the disposition the ally walk authors: neutral until the
// veteran says he joins, then allied.
var joined = DispositionInput{
	Between: [2]FactionID{FactionParty, "veterans"}, Stance: StanceNeutral,
	Until: TriggerFact{Fact: "veteran-joined"}, Then: StanceAllied,
}

// stanceOf is the pair's fold now.
func stanceOf(t *testing.T, enc *Encounter, a, b FactionID) Stance {
	t.Helper()
	s, err := enc.Stance(a, b)
	require.NoError(t, err)

	return s
}

// TestAFactTurnsAPairAllied: the veteran, his faction's mind, learns the
// fact, and the party and his side are allied — for every reader, the beat
// says so, and it survives the blob.
func TestAFactTurnsAPairAllied(t *testing.T) {
	enc := thenEnc(t, joined)
	require.Equal(t, StanceNeutral, stanceOf(t, enc, FactionParty, "veterans"))

	require.NoError(t, enc.learnFact("veteran", "veteran-joined", "joined", 1))

	require.Equal(t, StanceAllied, stanceOf(t, enc, FactionParty, "veterans"))
	allied, known := enc.IsAllied("veteran", "hero")
	require.True(t, known)
	require.True(t, allied)
	require.True(t, enc.alliedAcross("veteran", "hero"), "and the hero is somebody `ally` names")

	story, err := enc.Story(&StoryInput{Audience: "hero"})
	require.NoError(t, err)
	var turned bool
	for _, beat := range story {
		var body struct {
			Beat    string
			Between []string
			Stance  string
		}
		require.NoError(t, json.Unmarshal(beat.Payload, &body))
		turned = turned || (body.Beat == BeatStance && body.Stance == "allied")
	}
	require.True(t, turned, "the stance beat says the pair is allied")

	blob, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(blob, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: thenSetup(nil).Capabilities})
	require.NoError(t, err, "a run whose pair turned allied loads back")
	require.Equal(t, StanceAllied, stanceOf(t, loaded, FactionParty, "veterans"), "and is still allied")
}

// TestAPublicUntilTurnsAPairAllied: the turn that is nobody's knowledge — a
// stance the pair was waiting on — settles the pair to the allied edge too.
// The veterans fall out with the thugs on the fact; the party and the
// veterans wait on that and turn allied.
func TestAPublicUntilTurnsAPairAllied(t *testing.T) {
	enc := thenEnc(t,
		DispositionInput{
			Between: [2]FactionID{"veterans", "thugs"}, Stance: StanceNeutral,
			Until: TriggerFact{Fact: "veteran-joined"},
		},
		DispositionInput{
			Between: [2]FactionID{FactionParty, "veterans"}, Stance: StanceNeutral,
			Until: TriggerStance{Between: [2]FactionID{"veterans", "thugs"}, Stance: StanceHostile},
			Then:  StanceAllied,
		},
	)

	require.NoError(t, enc.learnFact("veteran", "veteran-joined", "joined", 1))

	require.Equal(t, StanceHostile, stanceOf(t, enc, "veterans", "thugs"))
	require.Equal(t, StanceAllied, stanceOf(t, enc, FactionParty, "veterans"))
}

// TestWithNoThenAPairTurnsAsBefore: an until that names no `then` turns its
// pair to the other of hostile and neutral, the meaning every file written
// before `then` has.
func TestWithNoThenAPairTurnsAsBefore(t *testing.T) {
	enc := thenEnc(t, DispositionInput{
		Between: [2]FactionID{FactionParty, "veterans"}, Stance: StanceNeutral,
		Until: TriggerFact{Fact: "veteran-joined"},
	})
	require.NoError(t, enc.learnFact("veteran", "veteran-joined", "joined", 1))
	require.Equal(t, StanceHostile, stanceOf(t, enc, FactionParty, "veterans"))
}

// TestAnAlliedPairThatNamesWhereItGoesTurns: allied until a fact, then
// hostile — the betrayal an allied pair could not author before.
func TestAnAlliedPairThatNamesWhereItGoesTurns(t *testing.T) {
	enc := thenEnc(t, DispositionInput{
		Between: [2]FactionID{FactionParty, "veterans"}, Stance: StanceAllied,
		Until: TriggerFact{Fact: "paid-off"}, Then: StanceHostile,
	})
	require.Equal(t, StanceAllied, stanceOf(t, enc, FactionParty, "veterans"))
	require.NoError(t, enc.learnFact("veteran", "paid-off", "bribed", 1))
	require.Equal(t, StanceHostile, stanceOf(t, enc, FactionParty, "veterans"))
}

// TestAThenThatCannotBeOneIsRefused: `then` beside no until, a word outside
// the stances, the stance the pair already holds, and an until on an allied
// pair that names nowhere to go, each refused by name.
func TestAThenThatCannotBeOneIsRefused(t *testing.T) {
	pair := [2]FactionID{FactionParty, "veterans"}
	fact := TriggerFact{Fact: "veteran-joined"}
	for name, tc := range map[string]struct {
		d    DispositionInput
		says string
	}{
		"no until": {DispositionInput{Between: pair, Stance: StanceNeutral, Then: StanceAllied},
			"dispositions[0] names then \"allied\" and no until, and a pair nothing turns has nowhere to go"},
		"not a stance": {DispositionInput{Between: pair, Stance: StanceNeutral, Until: fact, Then: "friendly"},
			"dispositions[0] turns to \"friendly\", which is not hostile, neutral or allied"},
		"where it already is": {DispositionInput{Between: pair, Stance: StanceNeutral, Until: fact, Then: StanceNeutral},
			"dispositions[0] is neutral and turns to neutral, which is the stance it already holds"},
		"allied, nowhere to go": {DispositionInput{Between: pair, Stance: StanceAllied, Until: fact},
			"dispositions[0] is allied and has an until, and an allied pair has nothing to become unless `then` names where it goes"},
	} {
		_, err := NewEncounter(thenSetup([]DispositionInput{tc.d}))
		require.ErrorIs(t, err, ErrNoFaction, name)
		require.Contains(t, err.Error(), tc.says, name)
	}
}

// TestOnlyAPairThatCanTurnAlliedCarriesTheAlliedTurn: the public allied
// settle is declared, and accepted at load, only for a pair whose own until
// turns it allied.
func TestOnlyAPairThatCanTurnAlliedCarriesTheAlliedTurn(t *testing.T) {
	enc := thenEnc(t, joined)
	require.Equal(t, []Stance{StanceNeutral, StanceAllied, StanceHostile},
		enc.field.settledStancesOf(pairOf(FactionParty, "veterans")))
	require.Equal(t, []Stance{StanceNeutral, StanceHostile},
		enc.field.settledStancesOf(pairOf("veterans", "thugs")),
		"a neutral pair with no until can only come to blows")
}
