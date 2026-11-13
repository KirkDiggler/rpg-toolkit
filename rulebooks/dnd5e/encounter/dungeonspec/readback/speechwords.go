// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// speechwords.go says how loudly a line is spoken and what follows from it
// (rpg-project#933, perceived speech): whose words reach whom, who is told
// who spoke, and who learns a fact a spoken row teaches.

// loudnessOf is an authored loudness word, empty read as speak.
func loudnessOf(word string) string {
	if word == "" {
		return "speak"
	}
	return word
}

// speechVerb is the verb a creature says a row's line with, by loudness.
func speechVerb(word string) string {
	switch loudnessOf(word) {
	case "whisper":
		return "whispers"
	case "shout":
		return "shouts"
	case "scream":
		return "screams"
	}
	return "says"
}

// reachWords is where a line's words reach at this loudness, in rooms, said
// about who speaks it: a room is a wall partition, joined to the next by
// doors, and a closed door on the way counts as one room more.
func reachWords(word, who string) string {
	const addressed = ", and whoever it is said to, wherever they stand"
	switch loudnessOf(word) {
	case "whisper":
		return "only the one it is said to, and only from beside them"
	case "shout":
		return "everyone in " + who + "'s room and the next room over (a closed door on the way counts as a room)" + addressed
	case "scream":
		return "everyone up to two rooms beyond " + who + "'s (a closed door on the way counts as a room)" + addressed
	}
	return fmt.Sprintf("everyone in %s's room (the wall partition %s stands in), and anyone in the next room within %d feet"+
		" of %s by the way through an open doorway (a closed door stops them)", who, who, encounter.SpeechCarryFeet, who) + addressed
}

// loudnessSentence is a line's loudness and what it means, as one sentence.
func loudnessSentence(word string) string {
	l := loudnessOf(word)
	s := fmt.Sprintf("It is said at %s (%s): its words reach %s.", quote(l), loudnessSource(word), reachWords(word, "the speaker"))
	if l == "whisper" {
		s += " A whispered line is said only from beside the one it is said to, and nobody else notices it;" +
			" a whispered answer said to someone not beside the speaker reaches nobody."
	}
	return s
}

// raisedWords is a `heard` loudness as a clause after the sound it narrows
// (rpg-project#933): at or above the word, by whisper, speak, shout,
// scream; nothing when the condition names none, which hears any loudness.
func raisedWords(word string) string {
	switch word {
	case "":
		return ""
	case "scream":
		return " raised to a scream"
	}
	return fmt.Sprintf(" at %s or louder", quote(word))
}

// foldedLoudness is a line's loudness in the folded form, when it is authored
// as anything but the default `speak`: "shouted: its words reach the next
// room over; ". A spoken line says nothing here, as it always did.
func foldedLoudness(word string) string {
	switch loudnessOf(word) {
	case "whisper":
		return "whispered: only the one it is said to hears it, from beside them; "
	case "shout":
		return "shouted: its words reach the next room over; "
	case "scream":
		return "screamed: its words reach two rooms over; "
	}
	return ""
}

// loudnessSource says whether a loudness was authored or is the default.
func loudnessSource(word string) string {
	if word == "" {
		return "the default"
	}
	return "authored"
}

// perceivedSentence is how every member perceives a spoken line, the engine
// rule every line and answer follows (rpg-project#933, R1–R4).
const perceivedSentence = "Whoever sees the speaker is told who said it; whoever the words reach and does not see the " +
	"speaker hears the words — and which line it was, the roll, whether it worked and what it taught — from someone " +
	"unnamed; whoever only sees it said learns neither the words, which line it was, whether it worked, nor what it " +
	"taught; whoever the words do not reach may still notice a voice by passive Perception, never a whisper; the one " +
	"it is said to is named only to those who see them."

// learnersWords is who learns a fact a creature's spoken row teaches: whoever
// its words reach, seen or not (rpg-project#933, R9).
func learnersWords(word, who string) string {
	return fmt.Sprintf("every member %s's words reach (%s), whether or not they see %s, %s included,",
		who, reachWords(word, who), who, who)
}
