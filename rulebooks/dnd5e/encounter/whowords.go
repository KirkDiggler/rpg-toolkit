// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "slices"

// WhoWord is one word that names who in a note (rpg-project#1001): who
// takes it, whom it is about, or whose note a read asks of — or who receives
// a status effect a line's row gives (rpg-project#990). `give`, a line's
// requirement, a post's pass, a machine's guard and `apply_status` name
// them; the dungeonspec grammar and the read-back both read this list, so
// neither can say a word the other does not know.
//
// ONE WORD MAY BE WRITTEN IN MORE THAN ONE PLACE, one entry per place:
// `speaker` is a word of `give.about` and of `apply_status.to`, and its
// label says what it means in each. A word is looked up WHERE IT IS WRITTEN
// ([WhoWordAt]), never by the word alone.
type WhoWord struct {
	// Word is the word as authored or as the read-back names it.
	Word string
	// Role is what it names: [WhoTaker] (who comes to note it), [WhoAbout]
	// (whom a given note is about), [WhoRead] (whom a read asks about) or
	// [WhoReceiver] (who receives a status effect).
	Role string
	// Label is a few plain words for it.
	Label string
	// Allowed is where it is written ([WhoInGiveTo], [WhoInGiveAbout],
	// [WhoInGiveHeardBy], [WhoInRequires], [WhoInPass], [WhoInNoted],
	// [WhoInNotedAnyone], [WhoInApplyStatusTo]), or [WhoInEngine] for a word only the engine's
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
	// WhoReceiver is who receives a status effect a line's row gives
	// (rpg-project#990). It notes nothing, so it has no Way.
	WhoReceiver = "receiver"
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
	// WhoInStateGiveTo and WhoInStateGiveAbout are a machine state's note:
	// who comes to hold it and whom it is about (rpg-project#989).
	WhoInStateGiveTo    = "state.give.to"
	WhoInStateGiveAbout = "state.give.about"
	// WhoInApplyStatusTo is who receives a status effect: a row's
	// `apply_status: { status, to }` (rpg-project#990).
	WhoInApplyStatusTo = "apply_status.to"
	WhoInEngine        = "engine"
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

// WhoItself is the creature whose machine stepped, at `state.give.to`
// (rpg-project#989).
const WhoItself = "itself"

var whoWords = []WhoWord{
	{Word: string(RecognitionSpokenTo), Role: WhoTaker, Label: "the one the line is said to", Allowed: WhoInGiveTo, Way: ChannelJudgement},
	{Word: string(RecognitionCarriersWhoWitnessed), Role: WhoTaker, Label: "the other carriers who caught the words", Allowed: WhoInGiveHeardBy, Way: ChannelHeard},
	{Word: WhoTheArresting, Role: WhoTaker, Label: "each one up now that carries the conversation", Allowed: WhoInEngine, Way: ChannelJudgement},
	{Word: string(RecognitionPartyInSight), Role: WhoAbout, Label: "the speaker's party it sees", Allowed: WhoInGiveAbout},
	{Word: string(RecognitionSpeaker), Role: WhoAbout, Label: "the speaker, if it sees them", Allowed: WhoInGiveAbout},
	{Word: WhoTheTaken, Role: WhoAbout, Label: "the ones taken", Allowed: WhoInEngine},
	{Word: WhoTheGone, Role: WhoAbout, Label: "the party members it saw who are not there now", Allowed: WhoInEngine},
	{Word: WhoItself, Role: WhoTaker, Label: "the creature whose machine stepped", Allowed: WhoInStateGiveTo, Way: ChannelJudgement},
	{Word: WhoTheOneNamed, Role: WhoAbout, Label: "the one the beat names", Allowed: WhoInStateGiveAbout},
	{Word: WhoTheSpeaker, Role: WhoRead, Label: "the speaker", Allowed: WhoInRequires},
	{Word: WhoTheWalker, Role: WhoRead, Label: "the walker", Allowed: WhoInPass},
	{Word: WhoTheOneNamed, Role: WhoRead, Label: "the one the beat names", Allowed: WhoInNoted},
	{Word: WhoAnyone, Role: WhoRead, Label: "anyone", Allowed: WhoInNotedAnyone},
	// WHO RECEIVES A STATUS EFFECT (rpg-project#990). `speaker` here is
	// UNCONDITIONAL — the speaker receives it whether or not anyone sees
	// them, on a thing's row as on a creature's — where `give.about`'s
	// `speaker` is the speaker only if the holder sees them.
	// `party_in_sight` and `spoken_to` mean exactly what `give` means by
	// them: the one spoken to, and the speaker's party it sees.
	{Word: string(RecognitionSpeaker), Role: WhoReceiver, Label: "the speaker, seen or not", Allowed: WhoInApplyStatusTo},
	{Word: string(RecognitionPartyInSight), Role: WhoReceiver, Label: "the speaker's party it sees", Allowed: WhoInApplyStatusTo},
	{Word: string(RecognitionSpokenTo), Role: WhoReceiver, Label: "the one the line is said to", Allowed: WhoInApplyStatusTo},
}

// WhoWords is every who-word, in order. The slice is a copy.
func WhoWords() []WhoWord { return slices.Clone(whoWords) }

// WhoWordAt is the who-word's entry where written; false for a word that
// place does not allow.
func WhoWordAt(allowed, word string) (WhoWord, bool) {
	for _, w := range whoWords {
		if w.Word == word && w.Allowed == allowed {
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
