// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_lied_to_test.go is a line heard moves an NPC's own machine
// (rpg-project#1000), on the castle as wiring: the captain, in the hall,
// hears a failed claim from someone he holds `admitted` about and walks
// back out to his gate; the same claim failed by a member he never admitted
// leaves him where he is.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// captainsGateSpot is where the captain starts, and where `back-to-gate`
// walks him, as the file writes it.
var captainsGateSpot = spatial.Position{X: -4, Y: 8}

// theTurnLine is the captain's Ashford claim.
const theTurnLine = "turn"

// liedToScene is the admitted scene played on, ONCE per test binary in
// [TestMain]: the hero and the straggler know what the king did to Ashford,
// gate-1 has let the straggler in on the hero's vouch (out of the captain's
// earshot), and both stand beside the captain in the hall.
var liedToScene struct {
	once sync.Once
	err  error
	blob []byte
	// afterTheLie is the same run the moment the hero's claim to the
	// captain has failed: the Speak, and the round it paid, played.
	afterTheLie []byte
	// backAtTheGate is the same run played on: the hero's claim failed to
	// the captain, rounds passed until he stood on his gate spot, and one
	// round more, which leaves him there.
	backAtTheGate []byte
}

// liedToSceneTests are the tests that start from [liedToScene].
var liedToSceneTests = []string{
	"TestMadKingTheCaptainLeavesTheHallWhenAnAdmittedGuestLies", "TestMadKingAStrangersLieLeavesTheCaptainInTheHall",
	"TestMadKingTheCaptainHoldsTheGateAgain",
}

// buildTheLiedToScene plays [liedToScene] from the admitted scene. The fact
// is written as learning it writes it (the cook's gossip teaches it), so
// the claim is theirs to make without the walk through the kitchen.
func buildTheLiedToScene() {
	tb := &fixtureTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fixtureFailed); !ok {
				panic(r)
			}
			liedToScene.err = fmt.Errorf("the lied-to scene: %s", tb.failed)
		}
	}()
	admittedScene.once.Do(buildTheAdmittedScene)
	require.NoError(tb, admittedScene.err)
	var data encounter.EncounterData
	require.NoError(tb, json.Unmarshal(admittedScene.blob, &data))
	for _, who := range []string{string(hero), string(straggler)} {
		data.Journal.Facts = append(data.Journal.Facts, encounter.FactData{
			Kind: "known:fact:king-burned-village", Actor: who, Subject: who, Audience: []string{who},
		})
	}
	blob, err := json.Marshal(data)
	require.NoError(tb, err)
	s := loadTheSceneOn(tb, blob, admittedScene.compiled, admittedScene.host)
	s.walkMember(straggler, s.at(hero), false)
	require.NoError(tb, s.speakAs(hero, kingGate1, vouchLine, true))
	require.True(tb, s.recognizes(kingGate1, straggler), "precondition: gate-1 admitted the straggler")
	s.walkTo(s.at(kingCaptain))
	s.walkMember(straggler, s.at(hero), false)
	require.Equal(tb, "in-hall", machineState(tb, s.enc, kingCaptain), "precondition: the captain is in the hall")
	require.True(tb, s.recognizes(kingCaptain, hero), "precondition: he admitted the hero")
	require.False(tb, s.recognizes(kingCaptain, straggler), "precondition: and never the straggler")
	liedToScene.blob, err = json.Marshal(s.enc.ToData())
	require.NoError(tb, err)

	// The hero lies; the captain walks back to his gate spot.
	s.failTheClaim(hero)
	liedToScene.afterTheLie, err = json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
	for range 20 {
		if s.at(kingCaptain) == captainsGateSpot {
			break
		}
		s.aRoundPasses()
	}
	require.Equal(tb, captainsGateSpot, s.at(kingCaptain), "precondition: he reaches his gate spot")
	s.aRoundPasses()
	// The straggler, whom he never admitted, walks back out to the gate.
	s.walkMember(straggler, s.at(kingGate1), false)
	liedToScene.backAtTheGate, err = json.Marshal(s.enc.ToData())
	require.NoError(tb, err)
}

// liedTo loads [liedToScene] into a fresh run.
func liedTo(t *testing.T) *kingScene {
	t.Helper()
	liedToScene.once.Do(buildTheLiedToScene)
	require.NoError(t, liedToScene.err)
	// Its preconditions are the build's: the captain in the hall, holding
	// `admitted` about the hero and not about the straggler. Each read of
	// the whole run is a test's own clock, so they are not read again here.
	return loadTheScene(t, liedToScene.blob)
}

// failTheClaim has a member miss the Ashford claim to the captain.
func (s *kingScene) failTheClaim(member encounter.MemberID) {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: member, Target: kingCaptain, OptionID: theTurnLine,
		Skill: "persuasion", Beaten: false, DC: 15, Total: 3, Roller: rollsOne{},
		Revision: s.revisionOf(member, kingCaptain, theTurnLine),
	})
	require.NoError(s.t, err, "%s says the claim to the captain", member)
}

// TestMadKingTheCaptainLeavesTheHallWhenAnAdmittedGuestLies is read off the
// stored moment right after the hero's failed claim (the build played the
// Speak): he answered it, turned for the gate in that Speak, and the round
// the checked line paid already has him walking.
func TestMadKingTheCaptainLeavesTheHallWhenAnAdmittedGuestLies(t *testing.T) {
	t.Parallel()
	liedToScene.once.Do(buildTheLiedToScene)
	require.NoError(t, liedToScene.err)
	s := loadTheScene(t, liedToScene.afterTheLie)
	require.Equal(t, "back-to-gate", machineState(t, s.enc, kingCaptain), "the admitted hero lied: he turns for the gate")
	require.Equal(t, []encounter.MemberID{kingCaptain}, s.answeredWithWords("Lies. Get out before I change my mind."),
		"and he answered the line")
	require.Less(t, s.enc.Distance(s.at(kingCaptain), captainsGateSpot), s.enc.Distance(captainsPlaceInTheHall, captainsGateSpot),
		"his first steps are toward his gate")
}

func TestMadKingAStrangersLieLeavesTheCaptainInTheHall(t *testing.T) {
	t.Parallel()
	s := liedTo(t)
	s.failTheClaim(straggler)
	require.Equal(t, []encounter.MemberID{kingCaptain}, s.answeredWithWords("Lies. Get out before I change my mind."),
		"he answers the straggler")
	require.Equal(t, "in-hall", machineState(t, s.enc, kingCaptain), "and stays: he holds nothing about them")
}

// TestMadKingTheCaptainHoldsTheGateAgain: back on his gate spot he keeps
// the gate as he did in `ready` — his post bars the straggler, whom he holds
// nothing about, though both gate guards let them through — and, still
// holding `admitted` about the hero, nothing sends him back into the hall:
// a round on, he is there.
func TestMadKingTheCaptainHoldsTheGateAgain(t *testing.T) {
	t.Parallel()
	liedToScene.once.Do(buildTheLiedToScene)
	require.NoError(t, liedToScene.err)
	s := loadTheScene(t, liedToScene.backAtTheGate)
	require.Equal(t, captainsGateSpot, s.at(kingCaptain), "he stands on his gate spot, a round after arriving")
	require.Equal(t, "back-to-gate", machineState(t, s.enc, kingCaptain), "in the state that keeps his post")
	require.True(t, s.recognizes(kingCaptain, hero), "he still holds `admitted` about the hero")
	require.Equal(t, []encounter.MemberID{kingCaptain}, s.heldAgainst(straggler),
		"his post bars the straggler he never admitted, whom the gate guards let through")
}

// answeredWithWords is every creature the hero was written answering with
// these words, in story order.
func (s *kingScene) answeredWithWords(words string) []encounter.MemberID {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(s.t, err)
	var out []encounter.MemberID
	for _, entry := range story {
		var b map[string]any
		require.NoError(s.t, json.Unmarshal(entry.Payload, &b))
		if b["beat"] == encounter.BeatSpeech && b["words"] == words {
			if speaker, ok := b["speaker"].(string); ok {
				out = append(out, encounter.MemberID(speaker))
			}
		}
	}
	return out
}
