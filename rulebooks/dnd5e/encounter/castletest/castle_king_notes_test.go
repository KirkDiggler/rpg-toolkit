// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_notes_test.go holds what the engine lands to what the
// read-back says (rpg-project#1001): every note a real run of the castle
// lands (who notes it, which topic, by which way) is one a `notes` edge of
// the read-back predicted. A new way of giving a note that ships without
// its edges fails here.

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// noteKey is one note as landed or predicted: who notes it, the topic, the
// way it came by it.
type noteKey struct{ holder, topic, way string }

// noteReadBack is the castle's read-back, built ONCE per test binary in
// [TestMain] (a whole castle's read-back is over a second under race) for
// the tests that hold landed notes to it.
var noteReadBack struct {
	once sync.Once
	rb   *readback.ReadBack
	err  error
}

// noteTests are the tests that read [noteReadBack].
var noteTests = []string{
	"TestMadKingTheOutcomeAndTheVouchLandWhatTheyPredict", "TestMadKingTheArrestLandsWhatItPredicts",
	"TestMadKingTheJailLandsWhatItPredicts", "TestMadKingASeizureLandsWhatItPredicts",
}

func buildTheNoteReadBack() {
	raw, err := os.ReadFile(kingFixture)
	if err != nil {
		noteReadBack.err = err
		return
	}
	noteReadBack.rb, noteReadBack.err = readback.Of(raw)
}

// castleReadBack is the castle file's read-back.
func castleReadBack(t *testing.T) *readback.ReadBack {
	t.Helper()
	noteReadBack.once.Do(buildTheNoteReadBack)
	require.NoError(t, noteReadBack.err)
	return noteReadBack.rb
}

// has reports whether st carries a connection like c (its kind, id, role,
// is and at).
func has(st readback.Statement, c readback.Connection) bool {
	for _, x := range st.Connects {
		if x.Kind == c.Kind && x.ID == c.ID && x.Role == c.Role && x.Is == c.Is && x.At == c.At {
			return true
		}
	}
	return false
}

// predictedBy is every note the statements of one give path predict: those
// carrying the path's own edge.
func predictedBy(rb *readback.ReadBack, path readback.Connection) map[noteKey]bool {
	out := map[noteKey]bool{}
	for _, st := range rb.Statements() {
		if !has(st, path) {
			continue
		}
		for _, c := range st.Connects {
			if c.Kind == "creature" && c.Role == readback.RoleNotes {
				out[noteKey{c.ID, c.At, c.Is}] = true
			}
		}
	}
	return out
}

// landedNotes is every note the run's record says a holder came to have.
func (s *kingScene) landedNotes() map[noteKey]bool {
	s.t.Helper()
	all, err := s.enc.ReadRecord(&encounter.ReadRecordInput{})
	require.NoError(s.t, err)
	out := map[noteKey]bool{}
	for _, e := range all.Entries {
		var b map[string]any
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		holder, _ := b["holder"].(string)
		if b["beat"] != encounter.BeatRecognized || holder == "" {
			continue
		}
		out[noteKey{holder, b["topic"].(string), b["channel"].(string)}] = true
	}
	return out
}

// The driven tests below hold what the engine lands to what the read-back
// says, one give path each: a conversation's outcome (the guest claim that
// admits) and a line's row (the vouch), the arrest, the jail and a seizure
// on sight. Every note each path lands is one that path's own statements
// predicted.

// whoEdge is a who-word edge as the read-back writes it.
func whoEdge(word, role, is, at string) readback.Connection {
	return readback.Connection{Kind: readback.ConnWho, ID: word, Role: role, Is: is, At: at}
}

// entering is a conversation outcome's cause edge.
func entering(state string) readback.Connection {
	return readback.Connection{Kind: "conversation", ID: "front-gate", Role: "enters", Is: state}
}

// checkNotes holds the notes a path landed to the ones it predicted, and
// that it landed one of topic at all.
func checkNotes(t *testing.T, path string, landed, predicted map[noteKey]bool, topic string) {
	t.Helper()
	require.NotEmpty(t, predicted, "%s: the read-back says this path", path)
	seen := false
	for n := range landed {
		require.True(t, predicted[n], "%s: %s noted %s by %s, which no notes edge of this path predicted", path, n.holder, n.topic, n.way)
		seen = seen || n.topic == topic
	}
	require.True(t, seen, "%s: precondition: the run landed a note of %s", path, topic)
}

// notesSince is the notes in after that were not in before.
func notesSince(after, before map[noteKey]bool) map[noteKey]bool {
	out := map[noteKey]bool{}
	for n := range after {
		if !before[n] {
			out[n] = true
		}
	}
	return out
}

func TestMadKingTheOutcomeAndTheVouchLandWhatTheyPredict(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	s := admittedWithAStraggler(t)
	before := s.landedNotes()
	checkNotes(t, "the outcome", before, predictedBy(rb, entering("admitted")), "admitted")
	s.walkMember(straggler, s.at(hero), false)
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	checkNotes(t, "the vouch", notesSince(s.landedNotes(), before),
		predictedBy(rb, readback.Connection{Kind: "option", ID: vouchLine, Role: "on", Is: "said"}), "admitted")
}

func TestMadKingTheArrestLandsWhatItPredicts(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	s, _ := theArrest(t)
	predicted := predictedBy(rb, whoEdge(string(encounter.RecognitionCarriersWhoWitnessed), "takes", "heard", string(encounter.TopicArrested)))
	// The scene was admitted before the arrest: those notes are the outcome's.
	for n := range predictedBy(rb, entering("admitted")) {
		predicted[n] = true
	}
	checkNotes(t, "the arrest", s.landedNotes(), predicted, string(encounter.TopicArrested))
}

func TestMadKingTheJailLandsWhatItPredicts(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	arrested, _ := theArrest(t)
	jailed := theJail(t)
	checkNotes(t, "the jail", notesSince(jailed.landedNotes(), arrested.landedNotes()),
		predictedBy(rb, entering("jailed")), string(encounter.TopicJailed))
}

func TestMadKingASeizureLandsWhatItPredicts(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	s := theJail(t)
	before := s.landedNotes()
	s.seizeLeftTheGate()
	checkNotes(t, "a seizure", notesSince(s.landedNotes(), before),
		predictedBy(rb, whoEdge(encounter.WhoTheArresting, "takes", "judgement", string(encounter.TopicArrested))), string(encounter.TopicArrested))
}
