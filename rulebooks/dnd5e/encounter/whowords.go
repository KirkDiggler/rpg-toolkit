// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "slices"

// WhoWord is one word that names who in a note (rpg-project#1001): who
// takes it, whom it is about, or whose note a read asks of. `give`, a
// line's requirement, a post's pass and a machine's guard name them; the
// dungeonspec grammar and the read-back both read this list, so neither can
// say a word the other does not know.
type WhoWord struct {
	// Word is the word as authored or as the read-back names it.
	Word string
	// Role is what it names: [WhoTaker] (who comes to note it), [WhoAbout]
	// (whom a given note is about) or [WhoRead] (whom a read asks about).
	Role string
	// Label is a few plain words for it.
	Label string
	// Allowed is where it is written ([WhoInGiveTo], [WhoInGiveAbout],
	// [WhoInGiveHeardBy], [WhoInRequires], [WhoInPass], [WhoInNoted],
	// [WhoInNotedAnyone]), or [WhoInEngine] for a word only the engine's
	// own grants (an arrest, a jail, a seizure) use.
	Allowed string
	// Way is how a taker comes by the note, in the engine's own word
	// ([ChannelJudgement], [ChannelHeard]); empty for a word that takes
	// nothing. THE ONE PLACE this word is written: a ruling on what a
	// judged note is called changes this line.
	Way string
}

// The roles a who-word plays.
const (
	WhoTaker = "taker"
	WhoAbout = "about"
	WhoRead  = "read"
)

// Where a who-word is written.
const (
	WhoInGiveTo      = "give.to"
	WhoInGiveAbout   = "give.about"
	WhoInGiveHeardBy = "give.heard_by"
	WhoInRequires    = "requires"
	WhoInPass        = "pass"
	WhoInNoted       = "noted:"
	WhoInNotedAnyone = "noted.anyone:"
	WhoInEngine      = "engine"
)

// The who-words the engine's own grants name, beside the authored ones.
const (
	// WhoTheArresting is every guard who made an arrest: the
	// conversation's carriers. They note `jailed` and, on a seizure,
	// `arrested`, by judgement.
	WhoTheArresting = "the_arresting"
	// WhoTheTaken is the members an arrest took (and one seized after).
	WhoTheTaken = "the_taken"
	// WhoTheGone is the members an arrest made wanted: gone before it.
	WhoTheGone = "the_gone"
)

// The who-words a read names.
const (
	WhoTheSpeaker  = "the_speaker"
	WhoTheWalker   = "the_walker"
	WhoTheOneNamed = "the_one_named"
	WhoAnyone      = "anyone"
)

var whoWords = []WhoWord{
	{Word: string(RecognitionSpokenTo), Role: WhoTaker, Label: "the one the line is said to", Allowed: WhoInGiveTo, Way: ChannelJudgement},
	{Word: string(RecognitionCarriersWhoWitnessed), Role: WhoTaker, Label: "the other carriers who caught the words", Allowed: WhoInGiveHeardBy, Way: ChannelHeard},
	{Word: WhoTheArresting, Role: WhoTaker, Label: "every guard who made the arrest", Allowed: WhoInEngine, Way: ChannelJudgement},
	{Word: string(RecognitionPartyInSight), Role: WhoAbout, Label: "the speaker's party it sees", Allowed: WhoInGiveAbout},
	{Word: string(RecognitionSpeaker), Role: WhoAbout, Label: "the speaker, if it sees them", Allowed: WhoInGiveAbout},
	{Word: WhoTheTaken, Role: WhoAbout, Label: "the ones taken", Allowed: WhoInEngine},
	{Word: WhoTheGone, Role: WhoAbout, Label: "the ones gone before they were taken", Allowed: WhoInEngine},
	{Word: WhoTheSpeaker, Role: WhoRead, Label: "the speaker", Allowed: WhoInRequires},
	{Word: WhoTheWalker, Role: WhoRead, Label: "the walker", Allowed: WhoInPass},
	{Word: WhoTheOneNamed, Role: WhoRead, Label: "the one the beat names", Allowed: WhoInNoted},
	{Word: WhoAnyone, Role: WhoRead, Label: "anyone", Allowed: WhoInNotedAnyone},
}

// WhoWords is every who-word, in order. The slice is a copy.
func WhoWords() []WhoWord { return slices.Clone(whoWords) }

// WhoWordOf is the who-word's entry; false for a word the list does not have.
func WhoWordOf(word string) (WhoWord, bool) {
	for _, w := range whoWords {
		if w.Word == word {
			return w, true
		}
	}
	return WhoWord{}, false
}

// WhoWordsIn is every who-word allowed where given, in order.
func WhoWordsIn(allowed string) []string {
	var out []string
	for _, w := range whoWords {
		if w.Allowed == allowed {
			out = append(out, w.Word)
		}
	}
	return out
}
