// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// answer.go is THE AUTHOR'S TABLE, AND THE WORLD'S DIE (rpg-project#458,
// rpg-project#465).
//
// AN ANSWER IS THE CREATURE'S AUTHORED ANSWER TO A CHECK, ROLLED BY THE WORLD
// — NOT THE React VERB. "Reaction" is D&D's own rules term and this engine
// already uses it for that: an interrupt window, a held reaction, the thing
// [Encounter.RecordRollWindow] and the session's React answer. A creature
// replying to somebody who spoke to it is a different thing on a different
// clock, and naming both "reaction" would have made every doc in this package
// ambiguous about which one it meant.
//
// A social verb lands a verdict. What the creature DOES about that verdict is
// not this composition's opinion and not the verb's: the author wrote a
// weighted list of entries per outcome on the placement, and the world rolls
// one die to pick which entry fires. "The setup for the author is what I am
// most interested in" (Kirk).
//
// # This file is the social HALF of one table
//
// The entries, the words, the weights and the roll all live in table.go now,
// because a creature's turn rolls the SAME table under [AnswerTime]
// (rpg-project#465). What is left here is what a SOCIAL verdict does with a
// pick: teach the fact, land the `fled` deed, and write the beat.
//
// # The die is the world's, and it is seen
//
// R1 (Kirk, 2026-09-17): the answer roll goes in the beat and the debug log.
// So the pick is one die of size sum(weight × temper factor) rolled through a
// supplied roller — never a local rand, never a hash of anything — and the
// face, the total, every candidate's arithmetic and the entry index all ride
// out on [BeatAnswered]. A table nobody can replay is a table nobody can
// trust.
//
// # Nothing here decides what an outcome MEANS
//
// [Encounter.Intimidate]'s law, one layer on: the verb is told whether the
// check was beaten and this file is told which entry the die chose. What a
// `fact` turns stays where it already lives — the disposition graph — and
// what a creature does about having fled is its own `time` table's business.

// The row, the words and the roll all live in mind/behavior now; [Answer] and
// its neighbours are named here through table.go's aliases.

// BeatAnswered is the "beat" value of the story beat appended when the world
// rolls a creature's answer — to a social verb, or to having time.
//
// "answered", NOT "reacted": the React verb owns that word at this seam, and a
// client switching on beat names must not have to disambiguate an interrupt
// window from a goblin replying to a threat.
//
// EXPORTED BECAUSE A DECODER READS IT, [BeatIntimidated]'s reason exactly: the
// session-side decoder is written against this constant in the same wave, so a
// rename fails to compile there instead of quietly producing a beat nobody
// renders.
//
// IT IS THE ONLY ACCOUNT OF THE PICK, and the author's line reaches the table
// through it and nowhere else.
const BeatAnswered = "answered"

// BeatTempered is the "beat" value appended when a faction's authored mix
// deals one member its temperament — "so the streamer sees which goblin came
// out the coward" (design §3).
//
// ONE PER DEALT MEMBER, AND NONE FOR AN AUTHORED WORD. A placement whose
// author wrote `temper: coward` rolled nothing, and a beat saying it did
// would be this composition inventing a die.
const BeatTempered = "tempered"

// answerInput is one settled verdict looking for the creature's answer to it.
type answerInput struct {
	// creature is whose placement carries the table — the member the verb
	// was aimed at.
	creature MemberID

	// actor is who spoke. The one a `flee` entry's deed names.
	actor MemberID

	// key is the outcome, one of [AnswerKeys].
	key AnswerKey

	// verb is the bare verb name carried onto the beat ("intimidate",
	// "persuade"), so a reader need not split the key apart to know which
	// verb produced it.
	verb string

	// beaten is the verdict, carried onto the beat for the same reason.
	beaten bool

	// witnesses is who saw it — who learns a fact, and the beat's audience.
	witnesses []MemberID

	// at is the clock's high-water mark, shared with the verb's own beat so
	// the answer is stamped with the moment that caused it.
	at uint64

	// roller is the world's die. Required by the verbs that call this.
	roller dice.Roller
}

// answer rolls the creature's answer to a settled verdict and carries it out.
//
// NO ELIGIBLE ENTRY, NO ROLL, NO BEAT. A placement that authored nothing for
// this outcome — or whose every entry's `when` is false — produces silence,
// which is distinguishable from an entry that fired and did nothing: one
// writes no beat at all.
//
// Order is fact, then deed, then beat — the opposite of the verb's own order
// and deliberately so. [Encounter.Intimidate] appends its beat first because
// the threat is the CAUSE of everything after it; this beat is the RESULT,
// and it reports the entry, the roll and the line that only exist once the
// pick has been made.
func (e *Encounter) answer(ctx context.Context, in answerInput) error {
	creature, ok := e.members[in.creature]
	if !ok {
		return fmt.Errorf("answer: creature %q: %w", in.creature, ErrNotMember)
	}

	facts, err := e.factsFor(in.creature)
	if err != nil {
		return fmt.Errorf("answer: %w", err)
	}

	// THE VERB'S OWN DIE, and the creature is whose roll it is — said on the
	// beat below, which is the only channel a shared roller leaves for it.
	chosen, err := behavior.Pick(ctx, &behavior.PickInput{
		Key: in.key, Table: creature.Table, Temper: creature.Temper, Facts: facts, Die: in.roller,
	})
	if err != nil {
		return fmt.Errorf("answer: %w", err)
	}
	if chosen == nil {
		return nil
	}

	if chosen.Answer.Fact != "" {
		for _, id := range in.witnesses {
			if err := e.learnFact(id, chosen.Answer.Fact, BeatAnswered, in.at); err != nil {
				return fmt.Errorf("answer: %w", err)
			}
		}
	}

	if chosen.Answer.Flee {
		if err := e.landFled(in.creature, in.actor, in.at); err != nil {
			return fmt.Errorf("answer: %w", err)
		}
	}

	return e.appendAnsweredBeat(in, chosen)
}

// landFled puts [DeedFled] on the creature and nobody else: the memory of
// having been made to run, naming who did it.
//
// THE CREATURE'S OWN TESTIMONY. A deed lands on witnesses everywhere else in
// this module because a deed is what somebody SAW; this one is what the
// creature KNOWS about itself, and broadcasting it would tell the room a
// goblin's private state. The subject the store files it under is the actor's,
// exactly as every other deed's is, so `away: actor` reads the scarer's id
// straight off it.
//
// ACTOR IS WHO DID IT TO ME, which is the rule every `when` deed keeps:
// `attacked`, `intimidated` and `persuaded` all name the doer in Actor and
// this creature in Target, and `fled` names the one who caused the running.
func (e *Encounter) landFled(creature, actor MemberID, at uint64) error {
	record, ok := e.members[creature]
	if !ok {
		return fmt.Errorf("flee: creature %q: %w", creature, ErrNotMember)
	}
	where, err := e.cellOf(record)
	if err != nil {
		return fmt.Errorf("flee: creature %q: %w", creature, err)
	}

	return e.landDeedAt(DeedFled, actor, creature, where, []MemberID{creature}, at)
}

// appendAnsweredBeat writes what the table saw: which creature answered, to
// which trigger, the die and every candidate's arithmetic, the entry that
// fired, the word it carried and the line the author wrote.
//
// EVERY FIELD IS WRITTEN UNCONDITIONALLY, the `intimidated` beat's rule: a
// reader downstream must not get a third state out of an absent key. `word`
// is empty for an entry that only speaks, `verb` and `beaten` are empty and
// false for a `time` pick — nothing spoke — and both of those are answers
// rather than gaps.
func (e *Encounter) appendAnsweredBeat(in answerInput, chosen *Pick) error {
	payload, err := json.Marshal(answeredBeatBody(in.creature, in.verb, in.beaten, chosen))
	if err != nil {
		return fmt.Errorf("answer: marshal beat: %w", err)
	}

	if _, err := e.appendBeat(&record.AppendInput{
		At:       in.at,
		Audience: in.witnesses,
		Tags:     map[string]string{"tag": BeatAnswered},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("answer: %w", err)
	}

	return nil
}

// answeredBeatBody is the beat's payload for one pick — ONE BODY for a social
// answer and a `time` pick, so the two can never come to describe their
// shared arithmetic differently.
func answeredBeatBody(creature MemberID, verb string, beaten bool, chosen *Pick) map[string]interface{} {
	candidates := make([]map[string]interface{}, 0, len(chosen.Candidates))
	for _, c := range chosen.Candidates {
		candidates = append(candidates, map[string]interface{}{
			"entry": c.Entry, "weight": c.Weight, "percent": c.Percent, "loaded": c.Loaded,
		})
	}

	return map[string]interface{}{
		"beat":       BeatAnswered,
		"creature":   string(creature),
		"key":        string(chosen.Key),
		"verb":       verb,
		"beaten":     beaten,
		"roll":       chosen.Roll,
		"of":         chosen.Of,
		"entry":      chosen.Entry,
		"candidates": candidates,
		"temper":     chosen.Temper,
		"word":       answerWord(chosen.Answer),
		"selector":   selectorWord(chosen.Answer),
		"say":        chosen.Answer.Say,
		"fact":       string(chosen.Answer.Fact),
	}
}

// appendTemperedBeat writes which temperament a faction's mix dealt one
// member, with the faction the die belonged to, the face, and the die it was
// rolled on.
//
// THE FACTION IS THE DIE'S ENTITY (rpg-project#463, design §3): the spread is
// the FACTION's — "the instructions given to the group" — and the deal is one
// roll out of it, so the entity whose rule threw the die is the faction and
// not the creature that came out of it. Every other pick this composition
// makes names the creature; this one does not, and the beat has to say which,
// because a tray that draws every die in its owner's set cannot work it out
// from the member alone.
//
// THE DEAL SITE PASSES IT rather than this beat re-deriving it. The faction
// is what the deal was made FOR — [Encounter.dealTemperFor] has the member in
// hand and resolves it once — and a beat that looked it up again would be a
// second place the resolution happens, which is one more place for it to come
// to disagree.
func (e *Encounter) appendTemperedBeat(
	member MemberID, faction FactionID, word string, roll, of int, at uint64,
) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":    BeatTempered,
		"member":  string(member),
		"faction": string(faction),
		"temper":  word,
		"roll":    roll,
		"of":      of,
	})
	if err != nil {
		return fmt.Errorf("temper: marshal beat: %w", err)
	}

	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(tableBeat),
		Tags:     map[string]string{"tag": BeatTempered},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("temper: %w", err)
	}

	return nil
}

// answerWord is the entry's outcome word as the author wrote it, or empty
// for an entry that only speaks. One place, so the beat and the validator
// cannot disagree about what a word is called.
func answerWord(entry Answer) string {
	switch {
	case entry.Fact != "":
		return "fact"
	case entry.Flee:
		return "flee"
	case entry.Hold:
		return "hold"
	case entry.Attack != nil:
		return "attack"
	case entry.Toward != nil:
		return "toward"
	case entry.Away != nil:
		return "away"
	default:
		return ""
	}
}

// selectorWord is the entry's selector as the author wrote it — the word, or
// "at" for an authored cell, or empty for a word that selects nothing.
func selectorWord(entry Answer) string {
	sel := entry.Attack
	if sel == nil {
		sel = entry.Toward
	}
	if sel == nil {
		sel = entry.Away
	}
	if sel == nil {
		return ""
	}
	if sel.At != nil {
		return "at"
	}

	return string(sel.Word)
}

// answerKeyFor is the outcome key a verb's verdict lands under. One
// function, so the verbs and the authoring dialect cannot spell the pairing
// differently.
func answerKeyFor(verb string, beaten bool) AnswerKey {
	switch {
	case verb == DeedIntimidate && beaten:
		return AnswerIntimidated
	case verb == DeedIntimidate:
		return AnswerIntimidateFailed
	case verb == DeedPersuade && beaten:
		return AnswerPersuaded
	default:
		return AnswerPersuadeFailed
	}
}

// validateTable refuses a table this composition could not roll: a trigger
// key it does not know, an entry that weighs less than 1, a word under a key
// it is not legal on, and a `when` that names nothing or two things.
//
// AT THE DOOR, NOT AT THE ROLL. A placement whose table can never fire is a
// misconfiguration, and the moment to find out is when the member joins —
// not when a player finally threatens it and the panel reports an internal
// error. The authoring dialect refuses the same things in the author's own
// words; this is the composition refusing them for every other caller,
// including a persisted blob somebody edited.
func validateTable(table Table) error {
	for key, entries := range table {
		if !validTableKey(key) {
			return fmt.Errorf("answer %q is not a trigger this build rolls: %w", key, ErrBadAnswer)
		}
		for i, entry := range entries {
			if err := validateEntry(key, i, entry); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateEntry refuses one row: its weight, its word's legality under this
// key, and its condition's shape.
func validateEntry(key AnswerKey, i int, entry Answer) error {
	if entry.Weight < 1 {
		return fmt.Errorf("answer %q entry %d weighs %d: %w", key, i, entry.Weight, ErrBadAnswer)
	}

	social := key != AnswerTime
	word := answerWord(entry)
	switch word {
	case "fact", "flee":
		if !social {
			return fmt.Errorf("answer %q entry %d: `%s` answers a social verdict, not time: %w",
				key, i, word, ErrBadAnswer)
		}
	case "hold", "attack", "toward", "away":
		if social {
			return fmt.Errorf("answer %q entry %d: `%s` is what a creature does with time: %w",
				key, i, word, ErrBadAnswer)
		}
	}
	if entry.When != nil {
		if social {
			return fmt.Errorf("answer %q entry %d: a social verdict is already the condition: %w",
				key, i, ErrBadAnswer)
		}
		if err := validateWhen(entry.When); err != nil {
			return fmt.Errorf("answer %q entry %d: %w", key, i, err)
		}
	}
	if sel := selectorOf(entry); sel != nil {
		if sel.Word == SelectorActor && (entry.When == nil || entry.When.Deed == "") {
			return fmt.Errorf("answer %q entry %d: `actor` names the actor of a deed and this entry names none: %w",
				key, i, ErrBadAnswer)
		}
		if sel.At != nil && entry.Toward == nil {
			return fmt.Errorf("answer %q entry %d: a cell is somewhere to walk toward, not %s: %w",
				key, i, word, ErrBadAnswer)
		}
	}

	return nil
}

// validateWhen refuses a condition that names neither an enemy nor a deed, or
// both, or a span counted from zero.
func validateWhen(when *When) error {
	hasEnemy := when.Enemy != ""
	hasDeed := when.Deed != ""
	switch {
	case hasEnemy && hasDeed:
		return fmt.Errorf("a `when` is one condition, and this is two: %w", ErrBadAnswer)
	case !hasEnemy && !hasDeed:
		return fmt.Errorf("a `when` with no condition is on the table always — omit it: %w", ErrBadAnswer)
	case hasEnemy:
		for _, known := range EnemyWords {
			if when.Enemy == known {
				return nil
			}
		}

		return fmt.Errorf("`enemy: %s` is not a condition this build reads: %w", when.Enemy, ErrBadAnswer)
	}

	if DeedVerbFor(when.Deed) == "" {
		return fmt.Errorf("`%s` is not a deed this build holds: %w", when.Deed, ErrBadAnswer)
	}
	if when.Within < 1 {
		return fmt.Errorf("a span of %d rounds is counted from 1: %w", when.Within, ErrBadAnswer)
	}

	return nil
}

// selectorOf is the entry's selector, whichever word carries it, or nil.
func selectorOf(entry Answer) *Selector {
	switch {
	case entry.Attack != nil:
		return entry.Attack
	case entry.Toward != nil:
		return entry.Toward
	case entry.Away != nil:
		return entry.Away
	default:
		return nil
	}
}

// validTableKey reports whether a key is one of [TableKeys].
func validTableKey(key AnswerKey) bool {
	for _, known := range TableKeys {
		if key == known {
			return true
		}
	}

	return false
}

// cloneTable deep-copies a table so a caller's map and the composition's
// cannot be the same one — [MemberInput]'s standing rule that nothing
// crossing this door stays aliased. The per-entry pointers (When, the three
// selectors) are copied too: a rulebook's default table is one value shared
// by every creature of its kind.
func cloneTable(table Table) Table {
	if table == nil {
		return nil
	}

	out := make(Table, len(table))
	for key, entries := range table {
		rows := make([]Answer, 0, len(entries))
		for _, entry := range entries {
			rows = append(rows, cloneAnswer(entry))
		}
		out[key] = rows
	}

	return out
}

// cloneAnswer deep-copies one entry's pointers.
func cloneAnswer(entry Answer) Answer {
	out := entry
	if entry.When != nil {
		when := *entry.When
		out.When = &when
	}
	out.Attack = cloneSelector(entry.Attack)
	out.Toward = cloneSelector(entry.Toward)
	out.Away = cloneSelector(entry.Away)

	return out
}

// cloneSelector deep-copies a selector and the cell it may name.
func cloneSelector(sel *Selector) *Selector {
	if sel == nil {
		return nil
	}
	out := *sel
	if sel.At != nil {
		at := *sel.At
		out.At = &at
	}

	return &out
}
