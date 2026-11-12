// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

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
// speaker's room; of a shout, the next room as well; of a scream, two rooms.
// Catching the words is flat by level, no roll. Noticing that somebody spoke,
// without the words, stays on passive Perception against the voice's
// difficulty plus one step per room, as every sound.

// Loudness is how loudly a line is spoken (R10: the author's word).
type Loudness string

// The loudness words an author writes. Empty is [LoudnessSpeak].
const (
	// LoudnessWhisper reaches only the member addressed, from beside them.
	LoudnessWhisper Loudness = "whisper"

	// LoudnessSpeak reaches every member in the speaker's room.
	LoudnessSpeak Loudness = "speak"

	// LoudnessShout reaches the speaker's room and the next room over.
	LoudnessShout Loudness = "shout"

	// LoudnessScream reaches two rooms over.
	LoudnessScream Loudness = "scream"
)

// Loudnesses is every loudness word, in the order a refusal lists them.
var Loudnesses = []Loudness{LoudnessWhisper, LoudnessSpeak, LoudnessShout, LoudnessScream}

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
		return fmt.Errorf("%s: loudness %q is not one of whisper, speak, shout or scream", what, word)
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

// loudnessWord is the loudness a stored word means, empty and unknown both
// speak: every door already refused an unknown word, so this never guesses.
// SaidWithin is the farthest, in hexes, this line may be said from to whom
// it is said, when the host measures speech within reach: a whisper only
// from beside them (R10), any other line within reach. Reach zero is
// adjacent, as every reach door reads it, so the answer is never below one.
// THE ONE RULE the reply door and a host's offer both ask, so a line is
// never offered from where it would be refused.
func (o Option) SaidWithin(reach int) int {
	if loudnessWord(o.Loudness) == LoudnessWhisper || reach < 1 {
		return 1
	}
	return reach
}

func loudnessWord(word string) Loudness {
	l, _ := loudnessOf(word)
	if l == "" {
		return LoudnessSpeak
	}
	return l
}
