// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"slices"
	"strings"
)

// speech.go is PERCEIVED SPEECH (rpg-project#930; slice one is
// rpg-project#933): every spoken thing is one utterance with a loudness, and
// each observer is written exactly the parts their senses delivered.
//
// # Sight tells who; hearing tells what (R1)
//
// An observer is told who spoke only if they see the speaker, by the one
// sight answer every deed and line uses ([Encounter.audienceOf]). They are
// told the words only if the words reach them. An unseen speaker is heard and
// not named.
//
// # Loudness reaches in rooms (R2)
//
// A room is a wall partition joined by doors ([Encounter.links]); a closed
// door on the way costs one room more, the rule every sound follows. The
// words of a whisper reach only the member addressed; of a line spoken, the
// speaker's room, and a short way into the next room through an open doorway
// ([SpeechCarryFeet], rpg-project#953); of a shout, the next room as well; of
// a scream, two rooms.
// Catching the words is flat by level, no roll. Noticing that somebody spoke,
// without the words, stays on passive Perception against the voice's
// difficulty plus one step per room, as every sound.

// Loudness is how loudly a line is spoken (R10: the author's word).
type Loudness string

// The loudness words an author writes. Empty is [LoudnessSpeak].
const (
	// LoudnessWhisper reaches only the member addressed, from beside them.
	LoudnessWhisper Loudness = "whisper"

	// LoudnessSpeak reaches every member in the speaker's room, and every
	// member of the next room within [SpeechCarryFeet] of the speaker by a
	// way through an open doorway of the speaker's room.
	LoudnessSpeak Loudness = "speak"

	// LoudnessShout reaches the speaker's room and the next room over.
	LoudnessShout Loudness = "shout"

	// LoudnessScream reaches two rooms over.
	LoudnessScream Loudness = "scream"
)

// SpeechCarryFeet is how far a line spoken at [LoudnessSpeak] carries out of
// the speaker's room through an OPEN doorway into the next room
// (rpg-project#953, the operator: "ordinary speech should have a range"):
// thirty feet, six cells. THE ONE PLACE THE NUMBER LIVES.
//
// MEASURED THROUGH THE DOORWAY, NOT IN A STRAIGHT LINE: a member of the next
// room catches the words when, for some cell of an open door joining the
// speaker's room to theirs, the distance from the speaker to that cell plus
// the distance from that cell to them is at most this many feet, by
// [Encounter.Distance] in cells ([CellsFromFeet]). So a member just the other
// side of a wall with no open doorway near hears no words, however close in
// a straight line; a closed door stops the words, as it always did. Only the
// words carry: noticing a voice stays on passive Perception per room.
const SpeechCarryFeet = 30

// Loudnesses is every loudness word, in the order a refusal lists them.
var Loudnesses = []Loudness{LoudnessWhisper, LoudnessSpeak, LoudnessShout, LoudnessScream}

// LoudnessOrder is THE ONE ORDER a table's `heard` loudness and a held voice
// are compared by ([behavior.Facts.Loudness]): [Loudnesses] as words,
// quietest first. Every Facts this encounter builds carries it
// ([projectFacts]), so no two picks can judge a loudness by different
// orders. A fresh slice each call, so no caller can change it for another.
func LoudnessOrder() []string {
	out := make([]string, len(Loudnesses))
	for i, l := range Loudnesses {
		out[i] = string(l)
	}
	return out
}

// loudnessList is [LoudnessOrder] as a refusal says it: "whisper, speak,
// shout or scream".
func loudnessList() string {
	words := LoudnessOrder()
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// atOrAbove reports whether a loudness is at or above another, by
// [Loudnesses]; false when either is not a loudness this rulebook names.
func atOrAbove(l, least Loudness) bool {
	have, want := slices.Index(Loudnesses, l), slices.Index(Loudnesses, least)
	return have >= 0 && want >= 0 && have >= want
}

// loudnessOf is the loudness a stored word means: empty is speak. ok is
// false for a word this build does not know.
func loudnessOf(word string) (Loudness, bool) {
	if word == "" {
		return LoudnessSpeak, true
	}
	for _, l := range Loudnesses {
		if string(l) == word {
			return l, true
		}
	}
	return "", false
}

// validateLoudness refuses a loudness word this build does not know, at what.
func validateLoudness(what, word string) error {
	if _, ok := loudnessOf(word); !ok {
		return fmt.Errorf("%s: loudness %q is not one of %s", what, word, loudnessList())
	}
	return nil
}

// wordRooms is how many rooms the words of a line at this loudness reach,
// beyond the speaker's own (zero). A whisper reaches no room: only the member
// addressed, from beside them. ok is false for a whisper.
func wordRooms(l Loudness) (rooms int, ok bool) {
	switch l {
	case LoudnessShout:
		return 1, true
	case LoudnessScream:
		return 2, true
	case LoudnessWhisper:
		return 0, false
	}
	return 0, true
}

// noticeDC is the difficulty of noticing, without the words, that somebody
// spoke at this loudness, in the speaker's room ([dcPerHop] more per room).
// CONTENT, tuned on the walk (R8, the lead's call): speak 5, shout 0, scream
// 0. ok is false for a whisper, which nobody but its addressee notices.
func noticeDC(l Loudness) (dc int, ok bool) {
	switch l {
	case LoudnessWhisper:
		return 0, false
	case LoudnessShout, LoudnessScream:
		return 0, true
	}
	return soundDCs[SoundVoice], true
}

// SaidWithin is the farthest, in hexes, this line may be said from to whom
// it is said, when the host measures speech within reach: a whisper only
// from beside them (R10), any other line within reach. Reach zero is
// adjacent, as every reach door reads it, so the answer is never below one.
// For speech a reach below one, negative included, is judged as beside,
// where other reach verbs refuse a negative reach.
// THE ONE RULE the reply door and a host's offer both ask, so a line is
// never offered from where it would be refused.
func (o Option) SaidWithin(reach int) int {
	if loudnessWord(o.Loudness) == LoudnessWhisper || reach < 1 {
		return 1
	}
	return reach
}

// loudnessWord is the loudness a stored word means, empty and unknown both
// speak: every door already refused an unknown word, so this never guesses.
func loudnessWord(word string) Loudness {
	l, _ := loudnessOf(word)
	if l == "" {
		return LoudnessSpeak
	}
	return l
}
