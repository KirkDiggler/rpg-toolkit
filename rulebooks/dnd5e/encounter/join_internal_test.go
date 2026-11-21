// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// scriptedRolls is a stand-in initiative roller for the join tests: it
// answers exactly the members it is asked about, those with a scripted roll
// by total (highest first, equal totals by id) and then, in id order, those
// with none, placed without a roll.
type scriptedRolls map[MemberID]InitiativeRoll

func (s scriptedRolls) RollInitiative(members []MemberID) ([]InitiativeRoll, error) {
	var rolled, placed []InitiativeRoll
	for _, id := range members {
		if r, ok := s[id]; ok {
			rolled = append(rolled, r)
			continue
		}
		placed = append(placed, InitiativeRoll{Member: id})
	}
	sort.Slice(rolled, func(i, j int) bool {
		if rolled[i].Total != rolled[j].Total {
			return rolled[i].Total > rolled[j].Total
		}
		return rolled[i].Member < rolled[j].Member
	})
	sort.Slice(placed, func(i, j int) bool { return placed[i].Member < placed[j].Member })

	return append(rolled, placed...), nil
}

// answersWith is a roller that answers the same fixed list whatever it is
// asked: the stand-in for a roller that names somebody else, leaves one out
// or names one twice.
type answersWith []InitiativeRoll

func (a answersWith) RollInitiative([]MemberID) ([]InitiativeRoll, error) {
	return append([]InitiativeRoll(nil), a...), nil
}

func joinCaps(r InitiativeRoller) Capabilities {
	return Capabilities{
		Sight: everyoneSeesTheWholeMap{}, Equipment: UnobservedEquipment{}, Conditions: UnobservedEquipment{},
		Sheets: zeroSheets{}, Participation: everyoneStanding{}, Initiative: r,
		Driver: passDriver{}, Actors: Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

// newJoinEnc is the scene the joiner tests share: players alice, bob and
// carol in r1; monsters goblin, ogre and wolf behind the sealed seam in r2, so
// nothing joins on its own.
func newJoinEnc(t *testing.T, roller InitiativeRoller) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:  CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{rectRegion("r1", 0, 0, 8, 8), rectRegion("r2", 8, 0, 8, 8)}, Walls: sealedSeam(7, 8),
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 14, Y: 6}},
			{ID: "ogre", Kind: KindMonster, Position: spatial.Position{X: 13, Y: 6}},
			{ID: "wolf", Kind: KindMonster, Position: spatial.Position{X: 12, Y: 6}},
		},
		Endings:      []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: joinCaps(roller),
	})
	require.NoError(t, err)

	return enc
}

// joinRoll is one scripted initiative roll: a face and no modifier.
func joinRoll(member MemberID, face int) InitiativeRoll {
	return initiativeRoll(member, face, nil, nil)
}

// formThree forms the fight the tests join: alice 18, bob 12, carol 8 and,
// last, the goblin on 4. A fight with no opposed side dissolves at the first
// turn, so the goblin stands in it; its turns are driven on the way past.
func formThree(t *testing.T, enc *Encounter) {
	t.Helper()
	_, err := enc.form(&FormInput{
		Order: []MemberID{"alice", "bob", "carol", "goblin"},
		Rolls: []InitiativeRoll{joinRoll("alice", 18), joinRoll("bob", 12), joinRoll("carol", 8), joinRoll("goblin", 4)},
	})
	require.NoError(t, err)
}

// fightNow is the running bubble as the tests read it.
func fightNow(t *testing.T, enc *Encounter) (order []string, active, round int) {
	t.Helper()
	require.NotEmpty(t, enc.bubbles, "a fight is running")
	d := enc.bubbles[0].ToData()
	for _, id := range d.Order {
		order = append(order, string(id))
	}

	return order, d.ActiveIdx, d.Round
}

func endTurns(t *testing.T, enc *Encounter, members ...MemberID) *EndTurnOutput {
	t.Helper()
	var out *EndTurnOutput
	for _, id := range members {
		var err error
		out, err = enc.EndTurn(&EndTurnInput{Member: id})
		require.NoError(t, err, "end turn %q", id)
	}

	return out
}

// toRoundThree walks the formed fight to round 3 with alice acting.
func toRoundThree(t *testing.T, enc *Encounter) {
	t.Helper()
	endTurns(t, enc, "alice", "bob", "carol", "alice", "bob", "carol")
	_, active, round := fightNow(t, enc)
	require.Equal(t, 0, active)
	require.Equal(t, 3, round)
}

// turnsEnded counts the turn-ended beats the story holds for one member.
func turnsEnded(t *testing.T, enc *Encounter, member MemberID) int {
	t.Helper()
	n := 0
	for _, beat := range beatsTagged(t, enc, BeatTurnEnded) {
		if beat["member"] == string(member) {
			n++
		}
	}

	return n
}

func TestJoinPlace(t *testing.T) {
	order := []core.EntityID{"alice", "goblin", "bob"}
	totals := map[MemberID]int{"alice": 18, "goblin": 12, "bob": 8}
	placedWithoutARoll := InitiativeRoll{Member: "ogre"}

	cases := []struct {
		name      string
		round     int
		active    int
		roll      InitiativeRoll
		pos       int
		nextRound bool
	}{
		{"1: round one, outranks the goblin", 1, 0, joinRoll("ogre", 15), 1, false},
		{"2: round one, higher than everyone, cannot pass the one acting", 1, 0, joinRoll("ogre", 25), 1, false},
		{"3: round one, the round has got to the last", 1, 2, joinRoll("ogre", 15), 3, false},
		{"4: round one, ties bob and sorts after him", 1, 1, joinRoll("ogre", 8), 3, false},
		{"5: round one, ties bob and sorts before him", 1, 1, joinRoll("archer", 8), 2, false},
		{"6: a later round, outranks the goblin who is acting", 3, 1, joinRoll("ogre", 15), 1, true},
		{"7: a later round, outranks bob after the active slot", 3, 1, joinRoll("ogre", 10), 2, true},
		{"8: a later round, higher than everyone", 3, 1, joinRoll("ogre", 25), 0, true},
		{"11: round one, placed without a roll", 1, 0, placedWithoutARoll, 3, false},
		{"12: a later round, placed without a roll", 3, 1, placedWithoutARoll, 3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos, next := joinPlace(order, totals, tc.active, tc.round, tc.roll)
			require.Equal(t, tc.pos, pos)
			require.Equal(t, tc.nextRound, next)
		})
	}

	t.Run("9: two joiners in round one, the second scanned against the first", func(t *testing.T) {
		pos, _ := joinPlace(order, totals, 0, 1, joinRoll("ogre", 15))
		require.Equal(t, 1, pos)
		order2 := []core.EntityID{"alice", "ogre", "goblin", "bob"}
		totals2 := map[MemberID]int{"alice": 18, "ogre": 15, "goblin": 12, "bob": 8}
		pos, next := joinPlace(order2, totals2, 0, 1, joinRoll("wolf", 10))
		require.Equal(t, 3, pos)
		require.False(t, next)
	})

	t.Run("10: two joiners with equal totals in a later round, by id", func(t *testing.T) {
		pos, next := joinPlace(order, totals, 1, 3, joinRoll("ogre", 10))
		require.Equal(t, 2, pos)
		require.True(t, next)
		order2 := []core.EntityID{"alice", "goblin", "ogre", "bob"}
		totals2 := map[MemberID]int{"alice": 18, "goblin": 12, "ogre": 10, "bob": 8}
		pos, next = joinPlace(order2, totals2, 1, 3, joinRoll("wolf", 10))
		require.Equal(t, 3, pos)
		require.True(t, next)
	})
}

func TestAFirstRoundJoinerActsThisRoundAtItsRolledPlace(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 15)})
	formThree(t, enc)

	_, _, err := enc.joinFight([]MemberID{"ogre"})
	require.NoError(t, err)
	order, active, round := fightNow(t, enc)
	require.Equal(t, []string{"alice", "ogre", "bob", "carol", "goblin"}, order, "ogre outranks bob's 12")
	require.Equal(t, 0, active)
	require.Equal(t, 1, round)

	out := endTurns(t, enc, "alice")
	require.Equal(t, MemberID("bob"), out.Next, "the ogre's turn was driven on the way to bob")
	require.Equal(t, 1, turnsEnded(t, enc, "ogre"), "it acted in the round it joined")
}

func TestAFirstRoundJoinerWhoseRollHasPassedGoesStraightAfterTheOneActing(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 15)})
	formThree(t, enc)
	endTurns(t, enc, "alice", "bob")

	_, _, err := enc.joinFight([]MemberID{"ogre"})
	require.NoError(t, err)
	order, active, round := fightNow(t, enc)
	require.Equal(t, []string{"alice", "bob", "carol", "ogre", "goblin"}, order, "15 would outrank bob, but bob has acted: last")
	require.Equal(t, 2, active)
	require.Equal(t, 1, round)

	out := endTurns(t, enc, "carol")
	require.True(t, out.RoundWrapped)
	require.Equal(t, MemberID("alice"), out.Next)
	require.Equal(t, 1, turnsEnded(t, enc, "ogre"), "it took its turn in round one, after carol")
}

func TestALaterRoundJoinerHasNoTurnInTheRoundItJoins(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 10)})
	formThree(t, enc)
	toRoundThree(t, enc)
	endTurns(t, enc, "alice")

	_, _, err := enc.joinFight([]MemberID{"ogre"})
	require.NoError(t, err)
	order, active, round := fightNow(t, enc)
	require.Equal(t, []string{"alice", "bob", "ogre", "carol", "goblin"}, order, "ogre 10 outranks carol's 8")
	require.Equal(t, 1, active)
	require.Equal(t, 3, round)

	out := endTurns(t, enc, "bob")
	require.Equal(t, MemberID("carol"), out.Next, "the ogre has no turn in round three")
	require.Zero(t, turnsEnded(t, enc, "ogre"))

	out = endTurns(t, enc, "carol")
	require.True(t, out.RoundWrapped)
	endTurns(t, enc, "alice", "bob")
	require.Equal(t, 1, turnsEnded(t, enc, "ogre"), "it acts in round four, between bob and carol")
}

func TestALaterRoundJoinerWithTheHighestRollActsFirstNextRound(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{"ogre": initiativeRoll("ogre", 20, map[string]int{"Dexterity": 5}, []string{"Dexterity"})})
	formThree(t, enc)
	toRoundThree(t, enc)
	endTurns(t, enc, "alice")

	_, _, err := enc.joinFight([]MemberID{"ogre"})
	require.NoError(t, err)
	order, _, round := fightNow(t, enc)
	require.Equal(t, []string{"ogre", "alice", "bob", "carol", "goblin"}, order)
	require.Equal(t, 3, round)
	require.Zero(t, turnsEnded(t, enc, "ogre"))

	out := endTurns(t, enc, "bob", "carol")
	require.True(t, out.RoundWrapped)
	require.Equal(t, MemberID("alice"), out.Next, "the ogre was driven first in round four")
	require.Equal(t, 1, turnsEnded(t, enc, "ogre"))
}

func TestTwoJoinersInOneVerbArePlacedByTheirRolls(t *testing.T) {
	t.Run("round one, rows 9", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 15), "wolf": joinRoll("wolf", 10)})
		formThree(t, enc)
		_, _, err := enc.joinFight([]MemberID{"ogre", "wolf"})
		require.NoError(t, err)
		order, _, _ := fightNow(t, enc)
		require.Equal(t, []string{"alice", "ogre", "bob", "wolf", "carol", "goblin"}, order)
		endTurns(t, enc, "alice", "bob", "carol")
		require.Equal(t, 1, turnsEnded(t, enc, "ogre"))
		require.Equal(t, 1, turnsEnded(t, enc, "wolf"))
	})

	t.Run("a later round with equal totals, row 10", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 10), "wolf": joinRoll("wolf", 10)})
		formThree(t, enc)
		toRoundThree(t, enc)
		endTurns(t, enc, "alice")
		_, _, err := enc.joinFight([]MemberID{"ogre", "wolf"})
		require.NoError(t, err)
		order, _, _ := fightNow(t, enc)
		require.Equal(t, []string{"alice", "bob", "ogre", "wolf", "carol", "goblin"}, order)
		endTurns(t, enc, "bob", "carol")
		require.Zero(t, turnsEnded(t, enc, "ogre"), "both are marked for the next round")
		require.Zero(t, turnsEnded(t, enc, "wolf"))
		endTurns(t, enc, "alice", "bob")
		require.Equal(t, 1, turnsEnded(t, enc, "ogre"))
		require.Equal(t, 1, turnsEnded(t, enc, "wolf"))
	})
}

func TestAJoinerPlacedWithoutARollGoesLastAndStillWaitsInALaterRound(t *testing.T) {
	t.Run("round one", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{})
		formThree(t, enc)
		_, _, err := enc.joinFight([]MemberID{"ogre"})
		require.NoError(t, err)
		order, _, _ := fightNow(t, enc)
		require.Equal(t, []string{"alice", "bob", "carol", "goblin", "ogre"}, order)
		endTurns(t, enc, "alice", "bob", "carol")
		require.Equal(t, 1, turnsEnded(t, enc, "ogre"))
	})

	t.Run("a later round", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{})
		formThree(t, enc)
		toRoundThree(t, enc)
		endTurns(t, enc, "alice")
		_, _, err := enc.joinFight([]MemberID{"ogre"})
		require.NoError(t, err)
		order, _, _ := fightNow(t, enc)
		require.Equal(t, []string{"alice", "bob", "carol", "goblin", "ogre"}, order)
		endTurns(t, enc, "bob", "carol")
		require.Zero(t, turnsEnded(t, enc, "ogre"), "no turn at the end of the round it joined")
		endTurns(t, enc, "alice", "bob", "carol")
		require.Equal(t, 1, turnsEnded(t, enc, "ogre"))
	})
}

// transferredEntries reads every transferred beat of the story.
func transferredEntries(t *testing.T, enc *Encounter) []map[string]json.RawMessage {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	var out []map[string]json.RawMessage
	for _, entry := range story {
		if beatKind(t, entry.Payload) != BeatTransferred {
			continue
		}
		var beat map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}

	return out
}

func TestAJoinRecordsItsRollRoundAndFirstRound(t *testing.T) {
	t.Run("rolled, in round three", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 10)})
		formThree(t, enc)
		toRoundThree(t, enc)
		endTurns(t, enc, "alice")
		_, _, err := enc.joinFight([]MemberID{"ogre"})
		require.NoError(t, err)

		beats := transferredEntries(t, enc)
		require.Len(t, beats, 1)
		var round, first int
		require.NoError(t, json.Unmarshal(beats[0]["round"], &round))
		require.NoError(t, json.Unmarshal(beats[0]["first_round"], &first))
		require.Equal(t, 3, round)
		require.Equal(t, 4, first, "it has no turn in round three")
		var rolls []InitiativeRoll
		require.NoError(t, json.Unmarshal(beats[0]["rolls"], &rolls))
		require.Len(t, rolls, 1)
		require.Equal(t, joinRoll("ogre", 10), rolls[0])
	})

	t.Run("rolled, in round one", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 15)})
		formThree(t, enc)
		_, _, err := enc.joinFight([]MemberID{"ogre"})
		require.NoError(t, err)
		beats := transferredEntries(t, enc)
		require.Len(t, beats, 1)
		require.JSONEq(t, "1", string(beats[0]["round"]))
		require.JSONEq(t, "1", string(beats[0]["first_round"]))
	})

	t.Run("placed without a roll, no rolls key", func(t *testing.T) {
		enc := newJoinEnc(t, scriptedRolls{})
		formThree(t, enc)
		_, _, err := enc.joinFight([]MemberID{"ogre"})
		require.NoError(t, err)
		beats := transferredEntries(t, enc)
		require.Len(t, beats, 1)
		require.NotContains(t, beats[0], "rolls")
		require.Contains(t, beats[0], "first_round")
	})
}

func TestAJoinRefusesARollerThatAnswersSomebodyElse(t *testing.T) {
	cases := []struct {
		name    string
		answers answersWith
		asked   []MemberID
		want    error
	}{
		{"somebody who was not asked", answersWith{joinRoll("wolf", 10)}, []MemberID{"ogre"}, ErrNotMember},
		{"somebody left out", answersWith{joinRoll("ogre", 10)}, []MemberID{"ogre", "wolf"}, ErrInvalidData},
		{"somebody twice", answersWith{joinRoll("ogre", 10), joinRoll("ogre", 9)}, []MemberID{"ogre"}, ErrInvalidData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := newJoinEnc(t, tc.answers)
			formThree(t, enc)
			_, _, err := enc.joinFight(tc.asked)
			require.ErrorIs(t, err, tc.want)
			order, _, _ := fightNow(t, enc)
			require.Equal(t, []string{"alice", "bob", "carol", "goblin"}, order, "a refused join moves nobody")
			require.Empty(t, transferredEntries(t, enc))
		})
	}
}

func TestAJoinRefusesABadInitiativeRoll(t *testing.T) {
	bad := joinRoll("ogre", 10)
	bad.Roll, bad.Total = 0, 0
	bad.Calculation = &RollCalculation{Components: bad.Calculation.Components, Total: 0}
	enc := newJoinEnc(t, scriptedRolls{"ogre": bad})
	formThree(t, enc)

	_, _, err := enc.joinFight([]MemberID{"ogre"})
	require.ErrorIs(t, err, ErrInvalidData)
	order, _, _ := fightNow(t, enc)
	require.Equal(t, []string{"alice", "bob", "carol", "goblin"}, order)
	require.Empty(t, transferredEntries(t, enc))
}

func TestTheStragglerDoorRollsInitiative(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 15)})
	formThree(t, enc)

	participation, err := enc.participationNow()
	require.NoError(t, err)
	_, _, err = enc.applyVerdict(&trigger{joined: []MemberID{"ogre"}}, participation)
	require.NoError(t, err)

	order, _, _ := fightNow(t, enc)
	require.Equal(t, []string{"alice", "ogre", "bob", "carol", "goblin"}, order, "placed by its roll, not at the end")
	beats := transferredEntries(t, enc)
	require.Len(t, beats, 1)
	var rolls []InitiativeRoll
	require.NoError(t, json.Unmarshal(beats[0]["rolls"], &rolls))
	require.Len(t, rolls, 1)
	require.Equal(t, joinRoll("ogre", 15), rolls[0])
}

func TestAFightKeepsItsTotalsThroughASave(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{"ogre": joinRoll("ogre", 15)})
	formThree(t, enc)

	data := enc.ToData()
	require.Len(t, data.Initiative, 1)
	require.Equal(t, map[MemberID]int{"alice": 18, "bob": 12, "carol": 8, "goblin": 4}, data.Initiative[0].Totals)

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	var decoded EncounterData
	require.NoError(t, json.Unmarshal(raw, &decoded))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: decoded, Capabilities: joinCaps(scriptedRolls{"ogre": joinRoll("ogre", 15)})})
	require.NoError(t, err)

	_, _, err = loaded.joinFight([]MemberID{"ogre"})
	require.NoError(t, err)
	order, _, _ := fightNow(t, loaded)
	require.Equal(t, []string{"alice", "ogre", "bob", "carol", "goblin"}, order, "placed by the totals the save kept")
}

func TestAFightSavedWithoutItsTotalsIsRefusedPlainly(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{})
	formThree(t, enc)
	data := enc.ToData()
	data.Initiative = nil

	_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: joinCaps(scriptedRolls{})})
	require.ErrorIs(t, err, ErrSavedByOlderVersion)
}

func TestASavedFightWithTotalsThatDoNotFitIsRefused(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{})
	formThree(t, enc)

	data := enc.ToData()
	data.Initiative = append(data.Initiative, BubbleInitiativeData{})
	_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: joinCaps(scriptedRolls{})})
	require.ErrorIs(t, err, ErrInvalidData, "one entry too many")

	data = enc.ToData()
	data.Initiative[0].Totals = map[MemberID]int{"ogre": 15}
	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: joinCaps(scriptedRolls{})})
	require.ErrorIs(t, err, ErrInvalidData, "a total for somebody not in the fight")
}

func TestAMemberWhoLeavesTheFightTakesItsTotalWithIt(t *testing.T) {
	enc := newJoinEnc(t, scriptedRolls{})
	formThree(t, enc)

	_, err := enc.Transfer(&TransferInput{Member: "carol", To: ClockWorld})
	require.NoError(t, err)
	require.Equal(t, map[MemberID]int{"alice": 18, "bob": 12, "goblin": 4}, enc.ToData().Initiative[0].Totals)

	_, err = enc.leaveAnyClock("bob")
	require.NoError(t, err)
	require.Equal(t, map[MemberID]int{"alice": 18, "goblin": 4}, enc.ToData().Initiative[0].Totals)

	_, err = enc.leaveAnyClock("alice")
	require.NoError(t, err)
	_, err = enc.leaveAnyClock("goblin")
	require.NoError(t, err)
	require.Empty(t, enc.bubbles, "the fight is gone")
	require.Empty(t, enc.initiativeTotals, "and so are its totals")
	require.Empty(t, enc.ToData().Initiative)
}
