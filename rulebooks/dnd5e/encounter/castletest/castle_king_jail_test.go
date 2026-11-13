// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_king_jail_test.go is the cut to the cells (rpg-project#925, ruled
// 2026-11-14): at the arrested gate a taken member says "We'll come quietly."
// and the taken are set down in the cells, an off-screen walk of five
// rounds; custody becomes jailed, which holds nothing but the cell's door;
// and an arresting guard who sees one of them out of the cells again turns
// the garrison.

// comeQuietly is the line the arrested gate offers a taken member.
const comeQuietly = "come-quietly"

// jailedScene is [arrestScene] with the hero having gone quietly, ONCE per
// test binary in [TestMain], and the record's reading just before the line.
var jailedScene struct {
	once        sync.Once
	err         error
	blob        []byte
	beforeSeq   uint64
	beforeRound int
}

// jailedSceneTests are the tests that start from [jailedScene].
var jailedSceneTests = []string{
	"TestMadKingGoingQuietlySetsTheTakenDownInTheCells", "TestMadKingTheJailIsOneVerbsTime",
	"TestMadKingEachEndOfTheJailIsToldWhatItSaw", "TestMadKingTheJailedAreHeldByNothing",
	"TestMadKingAnEscapeTurnsTheGarrison", "TestMadKingTheJailIsKeptOnReload",
}

// goQuietly has member say the arrested gate's line to the captain.
func (s *kingScene) goQuietly(member encounter.MemberID) error {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: member, Target: kingCaptain, OptionID: comeQuietly, Roller: rollsOne{},
		Revision: s.revisionOf(member, kingCaptain, comeQuietly),
	})
	return err
}

// buildTheJailedScene loads the arrest scene and has the hero go quietly.
func buildTheJailedScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			jailedScene.err = fmt.Errorf("the jailed scene: %s", tb.failed)
		}
	}()
	arrestScene.once.Do(buildTheArrestScene)
	require.NoError(tb, arrestScene.err)
	s := loadTheSceneOn(tb, arrestScene.blob, arrestScene.compiled, arrestScene.host)
	next, err := s.enc.NextStorySeq()
	require.NoError(tb, err)
	jailedScene.beforeSeq, jailedScene.beforeRound = next, s.enc.ToData().Clock.HighWater
	require.True(tb, s.offersTo(hero, kingCaptain, comeQuietly), "precondition: the arrested gate offers the line")
	require.NoError(tb, s.goQuietly(hero))
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	jailedScene.blob = blob
}

// theJail loads the jailed scene into a fresh run.
func theJail(t *testing.T) *kingScene {
	t.Helper()
	jailedScene.once.Do(buildTheJailedScene)
	require.NoError(t, jailedScene.err)
	return loadTheSceneOn(t, jailedScene.blob, arrestScene.compiled, arrestScene.host)
}

// TestMadKingGoingQuietlySetsTheTakenDownInTheCells: the three taken stand
// in the cells, custody is jailed, the record names them, the gate's three
// hold `jailed` about each, and no player is told that record.
func TestMadKingGoingQuietlySetsTheTakenDownInTheCells(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	require.Equal(t, "jailed", s.entrance())
	made := s.arrested()
	require.Equal(t, encounter.CustodyJailed, made.Custody)
	require.Equal(t, takenMembers, made.Jailed)
	for _, m := range takenMembers {
		require.Equal(t, encounter.RegionID("region-cells"), s.regionOf(s.at(m)), "%s is in the cells", m)
		for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
			require.True(t, s.recognizesOn(g, m, intel.Topic(encounter.TopicJailed)), "%s holds %s jailed", g, m)
		}
		for _, b := range s.payloadsOf(m, encounter.BeatRecognized) {
			require.NotEqual(t, string(encounter.TopicJailed), b["topic"], "%s is not told the guards' record", m)
		}
	}
	require.Equal(t, s.arrestSceneAt(leftTheGate), s.at(leftTheGate), "the wanted were not taken")
}

// arrestSceneAt is where a member stood in the arrest scene.
func (s *kingScene) arrestSceneAt(id encounter.MemberID) spatial.Position {
	s.t.Helper()
	arrest := loadTheSceneOn(s.t, arrestScene.blob, arrestScene.compiled, arrestScene.host)
	return arrest.at(id)
}

// TestMadKingTheJailIsOneVerbsTime: the jail fires from inside Speak, and its
// move pays the five authored rounds and refreshes sight inside Speak. Time is
// paid once (the plain line pays none of its own), Speak's answer and the
// conversation's step precede every relocated beat, every relocated beat
// precedes the rounds' first tick, and a refresh afterwards finds nothing to
// change.
func TestMadKingTheJailIsOneVerbsTime(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	require.Equal(t, jailedScene.beforeRound+5, s.enc.ToData().Clock.HighWater, "five rounds, paid once")

	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero, AfterSeq: jailedScene.beforeSeq - 1})
	require.NoError(t, err)
	var order []string
	for _, e := range story {
		var b struct {
			Beat string `json:"beat"`
		}
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		order = append(order, b.Beat)
	}
	answered := slices.Index(order, encounter.BeatAnswered)
	stepped := slices.Index(order, encounter.BeatInteraction)
	relocated := slices.Index(order, encounter.BeatRelocated)
	lastRelocated := lastIndex(order, encounter.BeatRelocated)
	tick := slices.Index(order, encounter.BeatTick)
	require.True(t, answered >= 0 && stepped > answered, "the answer, then the conversation's step: %v", order)
	require.Greater(t, relocated, stepped, "then the move: %v", order)
	require.Greater(t, tick, lastRelocated, "and only then the rounds: %v", order)

	before := len(story)
	require.NoError(t, s.enc.RefreshPerception())
	again, err := s.enc.Story(&encounter.StoryInput{Audience: hero, AfterSeq: jailedScene.beforeSeq - 1})
	require.NoError(t, err)
	require.Len(t, again, before, "a second refresh finds nothing to change")
}

// TestMadKingEachEndOfTheJailIsToldWhatItSaw: the hireling — of the party's
// faction, at the gate, not taken — and the friend on the road who was not
// taken are told the taken left and never where; a prisoner in the cells is
// told they arrived and never from where; a knight in the throne room, seeing
// neither end, is told nothing.
func TestMadKingEachEndOfTheJailIsToldWhatItSaw(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	for _, tc := range []struct {
		reader encounter.MemberID
		end    string
		has    string
		hasnt  string
	}{
		{hireling, encounter.RelocatedLeft, "from", "to"},
		{leftTheGate, encounter.RelocatedLeft, "from", "to"},
		{"prisoner-1", encounter.RelocatedArrived, "to", "from"},
	} {
		told := s.payloadsOf(tc.reader, encounter.BeatRelocated)
		require.Len(t, told, len(takenMembers), "%s is told of each taken", tc.reader)
		for _, b := range told {
			require.Equal(t, tc.end, b["end"], tc.reader)
			require.Contains(t, b, tc.has, tc.reader)
			require.NotContains(t, b, tc.hasnt, "%s is never told the other end", tc.reader)
			require.NotContains(t, b, "words", "nor the words")
		}
	}
	require.Empty(t, s.payloadsOf(kingKeep1, encounter.BeatRelocated), "who saw neither end is told nothing")
	for _, m := range takenMembers {
		for _, b := range s.payloadsOf(m, encounter.BeatRelocated) {
			if b["member"] == string(m) {
				require.Equal(t, "The guards march you to the cells.", b["words"], "%s is told the words", m)
			} else {
				require.NotContains(t, b, "words", "%s is not told a friend's words", m)
			}
		}
	}
}

// TestMadKingTheJailedAreHeldByNothing: jailed holds nothing by itself — a
// jailed member steps about the cell as anyone does.
func TestMadKingTheJailedAreHeldByNothing(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	at := s.at(hero)
	require.NoError(t, s.stepOff(hero), "jailed is no hold: the jailed walk")
	require.NotEqual(t, at, s.at(hero))
}

// TestMadKingAnEscapeTurnsTheGarrison is the escape row's WIRING on the
// castle: a jailed hero set down back in the gatehouse, in the gate guards'
// sight and out of the cells, teaches the captain `prisoners-escaped`, the
// garrison turns, and the fight forms. Each row of the rule itself — the
// lock picked from inside and the walk out, seen through the opening, held
// not jailed, already turned, no mind — is proved on a small site
// (escape_internal_test.go); picking the castle's cell lock is
// [TestMadKingPickingTheCellLockEndsTheRunFreed]'s.
func TestMadKingAnEscapeTurnsTheGarrison(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	require.Empty(t, s.learners("prisoners-escaped"), "precondition: nobody has escaped")
	back := spatial.Position{X: -5, Y: 9}
	_, err := s.enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: hero, To: back}},
		Cause: encounter.RelocationCause{Ref: core.Ref{Module: "test", Type: "shortcuts", ID: "back-at-the-gate"}, Words: "You are back at the gate."},
	})
	require.NoError(t, err, "jailed holds nothing, so the exported verb may set the hero down")
	require.Equal(t, encounter.RegionID("region-gatehouse"), s.regionOf(s.at(hero)), "precondition: out of the cells")
	require.True(t, s.learners("prisoners-escaped")[kingCaptain], "the captain learns of the escape")
	require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty), "the garrison turns")
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "and the fight forms")
}

// TestMadKingTheJailIsKeptOnReload: saved and loaded at jailed, nobody moves,
// nothing is told again, and the line is no longer offered.
func TestMadKingTheJailIsKeptOnReload(t *testing.T) {
	t.Parallel()
	s := theJail(t)
	at := map[encounter.MemberID]spatial.Position{}
	for _, m := range takenMembers {
		at[m] = s.at(m)
	}
	n := len(s.payloadsOf(hero, encounter.BeatRelocated))
	s.roundTrip()
	require.Equal(t, "jailed", s.entrance())
	require.Equal(t, encounter.CustodyJailed, s.custody())
	for _, m := range takenMembers {
		require.Equal(t, at[m], s.at(m))
	}
	require.Len(t, s.payloadsOf(hero, encounter.BeatRelocated), n, "nothing fires again")
	require.False(t, s.offersTo(hero, kingCaptain, comeQuietly), "the jailed gate offers the line no more")
}

// TestMadKingATakenMemberWhoLeftStaysWanted: one of the taken leaves the run
// before the line; the other two are jailed, the one who left is not moved,
// is recorded wanted, and the guards who arrested them hold `wanted` about
// them, told to no player.
func TestMadKingATakenMemberWhoLeftStaysWanted(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	_, err := s.enc.Exit(&encounter.ExitInput{Member: seenOutside})
	require.NoError(t, err)
	require.NoError(t, s.goQuietly(hero))

	made := s.arrested()
	require.Equal(t, []encounter.MemberID{hero, inTheArea}, made.Jailed, "only the present are jailed")
	require.Contains(t, wantedOf(made), seenOutside, "the one who left is wanted")
	for _, g := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		require.True(t, s.recognizesOn(g, seenOutside, intel.Topic(encounter.TopicWanted)), "%s wants them", g)
	}
	for _, m := range []encounter.MemberID{hero, inTheArea} {
		for _, b := range s.payloadsOf(m, encounter.BeatRecognized) {
			require.NotEqual(t, string(encounter.TopicWanted), b["topic"], "%s is not told", m)
		}
	}
}

// lastIndex is where a kind last appears in a list of beat kinds, or -1.
func lastIndex(kinds []string, kind string) int {
	for i := len(kinds) - 1; i >= 0; i-- {
		if kinds[i] == kind {
			return i
		}
	}
	return -1
}
