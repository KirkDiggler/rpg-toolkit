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
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// noteKey is one note as predicted: who may note it, the topic, the way it
// comes by it.
type noteKey struct{ holder, topic, way string }

// landedNote is one note as the record lands it: one beat, with whom it is
// about.
type landedNote struct {
	noteKey
	subjects []string
}

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
	"TestMadKingTheJailWantsTheGoneByJudgement",
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

// predictedBy is every note the statements of one give path predict (those
// carrying the path's own edge), each with the who-words of whom it may be
// about.
func predictedBy(rb *readback.ReadBack, path readback.Connection) map[noteKey][]string {
	out := map[noteKey][]string{}
	for _, st := range rb.Statements() {
		if !has(st, path) {
			continue
		}
		about := map[string]string{}
		for _, c := range st.Connects {
			if c.Kind == readback.ConnWho && c.Role == "about" {
				about[c.At] = c.ID
			}
		}
		for _, c := range st.Connects {
			if c.Kind == "creature" && c.Role == readback.RoleMayNote {
				k := noteKey{c.ID, c.At, c.Is}
				if !slices.Contains(out[k], about[c.At]) {
					out[k] = append(out[k], about[c.At])
				}
			}
		}
	}
	return out
}

// landedNotes is every note the run's record lands, one per beat, in the
// record's order.
func (s *kingScene) landedNotes() []landedNote {
	s.t.Helper()
	all, err := s.enc.ReadRecord(&encounter.ReadRecordInput{})
	require.NoError(s.t, err)
	var out []landedNote
	for _, e := range all.Entries {
		var b struct {
			Beat     string   `json:"beat"`
			Holder   string   `json:"holder"`
			Topic    string   `json:"topic"`
			Channel  string   `json:"channel"`
			Subjects []string `json:"subjects"`
		}
		require.NoError(s.t, json.Unmarshal(e.Payload, &b))
		if b.Beat != encounter.BeatRecognized || b.Holder == "" {
			continue
		}
		subjects := slices.Clone(b.Subjects)
		slices.Sort(subjects)
		out = append(out, landedNote{noteKey{b.Holder, b.Topic, b.Channel}, subjects})
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

// whom resolves a who-word of whom a note is about, for one run: the
// members it names there.
type whom map[string][]encounter.MemberID

// checkNotes holds each note a path landed to the path's read-back: a
// may-note edge names its holder, topic and way, and whom it is about is
// that edge's who-word resolved for this run. And the path landed a note of
// topic at all.
func checkNotes(t *testing.T, path string, landed []landedNote, predicted map[noteKey][]string, resolve whom, topic string) {
	t.Helper()
	require.NotEmpty(t, predicted, "%s: the read-back says this path", path)
	seen := false
	for _, n := range landed {
		words, ok := predicted[n.noteKey]
		require.True(t, ok, "%s: %s noted %s by %s, which no may-note edge of this path predicted", path, n.holder, n.topic, n.way)
		var want []string
		for _, w := range words {
			ids, ok := resolve[w]
			require.True(t, ok, "%s: the run resolves %s", path, w)
			for _, id := range ids {
				want = append(want, string(id))
			}
		}
		slices.Sort(want)
		require.Equal(t, want, n.subjects, "%s: %s noted %s about whom %v says", path, n.holder, n.topic, words)
		seen = seen || n.topic == topic
	}
	require.True(t, seen, "%s: precondition: the run landed a note of %s", path, topic)
}

func TestMadKingTheOutcomeAndTheVouchLandWhatTheyPredict(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	s := admittedWithAStraggler(t)
	before := s.landedNotes()
	// The guest claim admitted the hero alone: the straggler was on the road,
	// out of every guard's sight.
	checkNotes(t, "the outcome", before, predictedBy(rb, entering("admitted")),
		whom{string(encounter.RecognitionPartyInSight): {hero}}, "admitted")
	s.walkMember(straggler, s.at(hero), false)
	require.NoError(t, s.speakAs(hero, kingGate1, vouchLine, true))
	// The vouch, the straggler now up beside the hero: the party in sight.
	checkNotes(t, "the vouch", s.landedNotes()[len(before):],
		predictedBy(rb, readback.Connection{Kind: "option", ID: vouchLine, Role: "on", Is: "said"}),
		whom{string(encounter.RecognitionPartyInSight): {hero, straggler}}, "admitted")
}

// arrestWhom is whom the arrest's who-words name in a run: the taken and
// the wanted its record keeps.
func arrestWhom(r encounter.ArrestRecord) whom {
	var taken, gone []encounter.MemberID
	for _, m := range r.Taken {
		taken = append(taken, m.ID)
	}
	for _, m := range r.Wanted {
		gone = append(gone, m.ID)
	}
	return whom{encounter.WhoTheTaken: taken, encounter.WhoTheGone: gone}
}

func TestMadKingTheArrestLandsWhatItPredicts(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	s, _ := theArrest(t)
	var landed []landedNote
	for _, n := range s.landedNotes() {
		if n.topic == string(encounter.TopicArrested) || n.topic == string(encounter.TopicWanted) {
			landed = append(landed, n)
		}
	}
	checkNotes(t, "the arrest", landed,
		predictedBy(rb, whoEdge(string(encounter.RecognitionCarriersWhoWitnessed), "takes", "heard", string(encounter.TopicArrested))),
		arrestWhom(s.arrested()), string(encounter.TopicArrested))
}

func TestMadKingTheJailLandsWhatItPredicts(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	arrested, _ := theArrest(t)
	jailed := theJail(t)
	checkNotes(t, "the jail", jailed.landedNotes()[len(arrested.landedNotes()):], predictedBy(rb, entering("jailed")),
		whom{encounter.WhoTheTaken: jailed.arrested().Jailed}, string(encounter.TopicJailed))
}

// TestMadKingTheJailWantsTheGoneByJudgement: one taken at the arrest is gone
// when the hero goes quietly. Gate-2 never noted `arrested` about them (the
// run is loaded with that one note taken out, as if gate-2 had not caught
// the claim), so the jail's `wanted` lands about the one gone with the
// captain and gate-1 alone, each by its own judgement, as its statement
// says.
func TestMadKingTheJailWantsTheGoneByJudgement(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	var data map[string]any
	require.NoError(t, json.Unmarshal(arrestScene.blob, &data))
	holdings := data["perception"].(map[string]any)["intel"].(map[string]any)["holdings"].(map[string]any)
	about := holdings["member|"+string(kingGate2)].(map[string]any)["member|"+string(seenOutside)].(map[string]any)
	require.Contains(t, about, string(encounter.TopicArrested), "precondition: gate-2 noted it at the arrest")
	delete(about, string(encounter.TopicArrested))
	blob, err := json.Marshal(data)
	require.NoError(t, err)
	s := loadTheSceneOn(t, blob, arrestScene.compiled, arrestScene.host)

	before := len(s.landedNotes())
	_, err = s.enc.Exit(&encounter.ExitInput{Member: seenOutside})
	require.NoError(t, err)
	require.NoError(t, s.goQuietly(hero))
	landed := s.landedNotes()[before:]
	checkNotes(t, "the jail, one gone", landed, predictedBy(rb, entering("jailed")),
		whom{encounter.WhoTheTaken: s.arrested().Jailed, encounter.WhoTheGone: {seenOutside}}, string(encounter.TopicWanted))
	var wanting []string
	for _, n := range landed {
		if n.topic == string(encounter.TopicWanted) {
			wanting = append(wanting, n.holder)
		}
	}
	slices.Sort(wanting)
	require.Equal(t, []string{string(kingCaptain), string(kingGate1)}, wanting,
		"only an arresting carrier who has noted `arrested` about the one gone wants them")
}

func TestMadKingASeizureLandsWhatItPredicts(t *testing.T) {
	t.Parallel()
	rb := castleReadBack(t)
	s := theJail(t)
	before := len(s.landedNotes())
	s.seizeLeftTheGate()
	checkNotes(t, "a seizure", s.landedNotes()[before:],
		predictedBy(rb, whoEdge(encounter.WhoTheArresting, "takes", "judgement", string(encounter.TopicArrested))),
		whom{encounter.WhoTheTaken: {leftTheGate}}, string(encounter.TopicArrested))
}
