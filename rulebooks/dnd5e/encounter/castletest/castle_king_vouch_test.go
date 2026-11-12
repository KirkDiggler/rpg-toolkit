// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_vouch_test.go is the captain goes in; a straggler is vouched
// for (rpg-project#929, ruled on #925 2026-11-12). Once the captain admits
// the party he comes to hold `admitted`, his routine wakes on it and he
// walks once into the great hall and stays; the two gate guards keep their
// posts. The guest claim is his alone, so a straggler who arrives later is
// down that way in: a recognised member must come back to the gate and
// vouch with the straggler in the guard's sight, or the straggler makes
// their own case.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The captain's place in the great hall, as the file writes it.
var captainsPlaceInTheHall = spatial.Position{X: 3, Y: -9}

// The gate guards' lines for whoever comes after the admission.
const (
	vouchLine     = "vouch"
	stragglerLine = "straggler"
	// friendsLine is a stranger asking after their friends (rpg-project#932).
	friendsLine = "friends-inside"
)

// straggler is the party member who falls behind on the road.
const straggler = encounter.MemberID("straggler")

// latecomer is a second party member left on the road (rpg-project#932):
// another stranger to the gate guards, for whom one straggler's failed case
// changes nothing.
const latecomer = encounter.MemberID("latecomer")

// turnedAwayTopic is what a gate guard holds about a straggler whose own
// case he refused (rpg-project#932).
const turnedAwayTopic = "turned-away"

// admittedScene is the admitted gate's stored scene, built ONCE per test
// binary in [TestMain] — the castle's launch, the claim and the captain's
// walk would otherwise be on each test's clock — and loaded back for every
// test that starts from it.
var admittedScene struct {
	once sync.Once
	err  error
	blob []byte
	// inTheHall is the same run once the hero has followed the captain into
	// the hall and stands beside him.
	inTheHall []byte
	compiled  dungeonspec.Compiled
	host      castleHost
}

// admittedSceneTests are the tests that start from the admitted gate.
var admittedSceneTests = []string{
	"TestMadKingTheCaptainGoesIn", "TestMadKingAStragglerIsVouchedFor", "TestMadKingAStragglerMakesTheirOwnCase",
	"TestMadKingTheCaptainsLineGrantsNothingAgain", "TestMadKingAStragglerHasOneTry",
}

// TestMain builds the admitted gate's scene before any test runs, when the
// run's -run pattern can select one of [admittedSceneTests].
func TestMain(m *testing.M) {
	flag.Parse()
	pattern := flag.Lookup("test.run").Value.String()
	re, err := regexp.Compile(strings.SplitN(pattern, "/", 2)[0])
	if pattern == "" || err != nil || slices.ContainsFunc(admittedSceneTests, re.MatchString) {
		admittedScene.once.Do(buildTheAdmittedScene)
	}
	os.Exit(m.Run())
}

// fixtureTB is the scene builder's stand-in for a test before any test
// exists: a failed requirement is recorded and unwinds the build.
type fixtureTB struct {
	testing.TB
	failed string
}

type fixtureFailed struct{}

func (f *fixtureTB) Helper()                        {}
func (f *fixtureTB) Name() string                   { return "admitted-scene" }
func (f *fixtureTB) Logf(string, ...any)            {}
func (f *fixtureTB) Errorf(format string, a ...any) { f.failed += fmt.Sprintf(format, a...) }
func (f *fixtureTB) FailNow()                       { panic(fixtureFailed{}) }
func (f *fixtureTB) Fatalf(format string, a ...any) {
	f.Errorf(format, a...)
	f.FailNow()
}

// buildTheAdmittedScene plays the scene once: the hero admitted at the
// captain, the straggler left on the road out of every gate guard's sight,
// and rounds passed until the captain stands in the hall and gate-1 is back
// on his post.
func buildTheAdmittedScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			admittedScene.err = fmt.Errorf("the admitted scene: %s", tb.failed)
		}
	}()
	start := partyStart(tb)
	s := openTheCastleAs(tb, castleHost{
		at: gatehouseSeat,
		players: []encounter.MemberInput{
			{ID: straggler, Kind: encounter.KindPlayer, Position: spatial.Position{X: start.X + 2, Y: start.Y}},
			{ID: latecomer, Kind: encounter.KindPlayer, Position: spatial.Position{X: start.X + 1, Y: start.Y}},
		},
	})
	s.walkTo(s.at(kingCaptain))
	s.said(kingCaptain, "guest", "persuasion", true)
	require.Equal(tb, "admitted", s.entrance(), "precondition: the guest claim admitted")
	require.False(tb, s.recognizes(kingGate1, straggler), "precondition: the straggler was out of sight")
	require.False(tb, s.recognizes(kingGate1, latecomer), "precondition: so was the latecomer")
	for range 6 {
		if s.at(kingCaptain) == captainsPlaceInTheHall {
			break
		}
		s.aRoundPasses()
	}
	require.Equal(tb, captainsPlaceInTheHall, s.at(kingCaptain), "precondition: the captain stands in the hall")
	s.backOnHisPost()
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	s.walkTo(s.at(kingCaptain))
	inTheHall, err := json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	admittedScene.blob, admittedScene.inTheHall, admittedScene.compiled, admittedScene.host = blob, inTheHall, s.compiled, s.host
}

// admittedWithAStraggler loads the admitted scene into a fresh run.
func admittedWithAStraggler(t *testing.T) *kingScene {
	t.Helper()
	admittedScene.once.Do(buildTheAdmittedScene)
	require.NoError(t, admittedScene.err)
	return loadTheScene(t, admittedScene.blob)
}

// loadTheScene loads one stored moment of the admitted scene.
func loadTheScene(t *testing.T, blob []byte) *kingScene {
	t.Helper()
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(blob, &data))
	s := &kingScene{
		t: t, compiled: admittedScene.compiled, host: admittedScene.host,
		strikes: &[]cellsStrike{}, standing: &castleStanding{down: map[encounter.MemberID]bool{}},
	}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
	})
	require.NoError(t, err)
	s.enc = enc
	return s
}

// speakAs has a member say a line to a guard at its offered revision.
func (s *kingScene) speakAs(member, target encounter.MemberID, option string, beaten bool) error {
	s.t.Helper()
	in := &encounter.SpeakInput{
		Range: speakReach, Actor: member, Target: target, OptionID: option, Roller: rollsOne{},
		Revision: s.revisionOf(member, target, option),
	}
	if option == stragglerLine {
		in.Skill, in.Beaten, in.DC = "persuasion", beaten, 13
		in.Total = map[bool]int{true: 18, false: 3}[beaten]
	}
	_, err := s.enc.Speak(context.Background(), in)
	return err
}

func TestMadKingTheCaptainGoesIn(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)

	hall, ok := s.enc.RegionAt(captainsPlaceInTheHall)
	require.True(t, ok)
	require.Equal(t, encounter.RegionID("region-great-hall"), hall, "his place is in the great hall")
	require.Equal(t, "in-hall", machineState(t, s.enc, kingCaptain))
	s.onHisPost(kingGate1, "gate-1 keeps his post")
	require.Equal(t, "ready", machineState(t, s.enc, kingGate2), "gate-2 keeps hers")
	require.False(t, s.offersTo(straggler, kingGate1, "guest"), "the guest claim is the captain's alone")
	require.False(t, s.offersTo(straggler, kingCaptain, "guest"), "and he offers it nobody once the gate has admitted")
	for _, data := range s.enc.ToData().Interactions {
		if data.Address != nil {
			require.NotEqual(t, kingCaptain, data.Address.Guard, "the captain in the hall addresses nobody at the gate")
		}
	}

	s.roundTrip()
	s.aRoundPasses()
	require.Equal(t, captainsPlaceInTheHall, s.at(kingCaptain), "after a load he stays in the hall")
	require.Equal(t, "in-hall", machineState(t, s.enc, kingCaptain), "and does not walk again")
}

func TestMadKingAStragglerIsVouchedFor(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)
	require.False(t, s.offersTo(straggler, kingGate1, vouchLine), "the vouch is not offered to a stranger")
	require.True(t, s.offersTo(hero, kingGate1, vouchLine), "it is offered to whom he recognises")

	// From the road, out of the guard's sight, a vouch admits nobody.
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	require.False(t, s.recognizes(kingGate1, straggler), "a straggler out of his sight is not recognised")

	// The straggler comes up to the gatehouse, into gate-1's sight.
	s.walkMember(straggler, s.at(hero), false)
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	require.True(t, s.recognizes(kingGate1, straggler), "the vouch admits the straggler in his sight")
	require.True(t, s.recognizes(kingGate2, straggler), "and gate-2, who saw it said, heard it")
	require.Empty(t, s.heldAgainst(straggler), "the gate lets the straggler through")
	require.Equal(t, "admitted", s.entrance(), "the gate's conversation did not move")
}

func TestMadKingAStragglerMakesTheirOwnCase(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)
	s.walkMember(straggler, s.at(hero), false)
	require.True(t, s.offersTo(straggler, kingGate1, stragglerLine), "offered to whom he does not recognise")
	require.False(t, s.offersTo(hero, kingGate1, stragglerLine), "and not to whom he does")

	require.NoError(t, s.speakAs(straggler, kingGate1, stragglerLine, true))
	require.True(t, s.recognizes(kingGate1, straggler), "a won case admits the straggler")
	require.True(t, s.recognizes(kingGate2, straggler), "and gate-2 heard it")
	require.False(t, s.recognizesOn(kingGate1, straggler, turnedAwayTopic), "a won case turns nobody away")
	require.Empty(t, s.heldAgainst(straggler), "the gate lets the straggler through")
}

// TestMadKingAStragglerHasOneTry: one try at the straggler's own case
// (rpg-project#932, ruled 2026-11-12: "1 try and the option should be
// gone"). A missed case is remembered by both gate guards as turned away, so
// neither offers it to that straggler again; another stranger still has it.
// The straggler may still ask after their friends, and a friend's vouch
// still lets them in. The captain, in the hall, wakes on `admitted` only.
func TestMadKingAStragglerHasOneTry(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)
	s.walkMember(straggler, s.at(hero), false)
	// The latecomer stays on the road: what is offered him is the guards'
	// judgement of him, wherever he stands.
	require.True(t, s.offersTo(latecomer, kingGate1, stragglerLine), "precondition: the latecomer is a stranger too")

	require.NoError(t, s.speakAs(straggler, kingGate1, stragglerLine, false))
	require.False(t, s.recognizes(kingGate1, straggler), "a missed case is a refusal")
	require.Equal(t, "admitted", s.entrance(), "and moves nothing toward arrest")
	require.True(t, s.recognizesOn(kingGate1, straggler, turnedAwayTopic), "gate-1 turned the straggler away")
	require.True(t, s.recognizesOn(kingGate2, straggler, turnedAwayTopic), "and gate-2 saw it")
	require.False(t, s.offersTo(straggler, kingGate1, stragglerLine), "one try: the case is gone")
	require.False(t, s.offersTo(straggler, kingGate2, stragglerLine), "on both guards")
	require.True(t, s.offersTo(latecomer, kingGate1, stragglerLine), "another stranger still has it")
	require.True(t, s.offersTo(latecomer, kingGate2, stragglerLine), "with either guard")
	require.False(t, s.recognizesOn(kingGate1, latecomer, turnedAwayTopic), "the refusal was about the speaker alone")

	// Asking after their friends is still offered, and changes nothing.
	require.True(t, s.offersTo(straggler, kingGate1, friendsLine), "the straggler may ask after their friends")
	require.False(t, s.offersTo(hero, kingGate1, friendsLine), "not whom he recognises")
	require.NoError(t, s.speakAs(straggler, kingGate1, friendsLine, true))
	for _, guard := range []encounter.MemberID{kingGate1, kingGate2} {
		require.False(t, s.recognizesOn(guard, straggler, admittedTopic), "the answer admits nobody: %s", guard)
		require.True(t, s.recognizesOn(guard, straggler, turnedAwayTopic), "and leaves the refusal as it was: %s", guard)
	}
	require.Equal(t, "admitted", s.entrance(), "and moves nothing")

	// The captain, in the hall, does not stir for a turned-away straggler.
	require.Equal(t, "in-hall", machineState(t, s.enc, kingCaptain))

	// A reload keeps the refusal.
	s.roundTrip()
	require.True(t, s.recognizesOn(kingGate1, straggler, turnedAwayTopic), "after a load gate-1 still remembers")
	require.False(t, s.offersTo(straggler, kingGate1, stragglerLine), "and the case is still gone")

	// A friend's vouch still lets the straggler in.
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	require.True(t, s.recognizes(kingGate1, straggler), "the vouch admits the turned-away straggler")
	require.Empty(t, s.heldAgainst(straggler), "the gate lets them through")
}

// TestMadKingTheCaptainsLineGrantsNothingAgain: the captain's line for whom
// he recognises, said to him in the hall, writes no recognition and no beat.
// A recognised-only line's answer does not re-grant (rpg-project#928).
func TestMadKingTheCaptainsLineGrantsNothingAgain(t *testing.T) {
	t.Parallel()
	admittedScene.once.Do(buildTheAdmittedScene)
	require.NoError(t, admittedScene.err)
	s := loadTheScene(t, admittedScene.inTheHall)
	region, ok := s.enc.RegionAt(s.at(hero))
	require.True(t, ok)
	require.Equal(t, encounter.RegionID("region-great-hall"), region, "precondition: the hero followed him into the hall")
	granted := s.recognitionBeats()
	require.NoError(t, s.speakAs(hero, kingCaptain, theWayIn, true), "the captain answers the line in the hall")
	require.Equal(t, granted, s.recognitionBeats(), "speaking on grants no recognition and writes no beat")
	require.True(t, s.recognizes(kingCaptain, hero), "and he still recognises the hero")
}
