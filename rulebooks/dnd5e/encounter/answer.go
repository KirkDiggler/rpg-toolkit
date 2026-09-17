// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// answer.go is THE AUTHOR'S TABLE, AND THE WORLD'S DIE (rpg-project#458,
// ideas/shenanigans/front-room-goblin.md).
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
// # It replaced a single fact, and the shape is the point
//
// `on: { intimidated: { fact: x } }` taught one fact on one outcome and had
// nowhere to put a second outcome, a line of speech, or a creature that runs.
// A LIST PER OUTCOME grows all three without breaking a file: a new outcome is
// a new key, a new possibility is a new entry, and a new thing a creature can
// do is a new field on an entry that every older file leaves at its zero
// value.
//
// # The die is the world's, and it is seen
//
// R1 (Kirk, 2026-09-17): the answer roll goes in the beat and the debug log.
// So the pick is one die of size sum(weights) rolled through the caller's
// supplied roller — never a local rand, never a hash of anything — and the
// face, the total and the entry index all ride out on [BeatAnswered]. A table
// nobody can replay is a table nobody can trust.
//
// # Nothing here decides what an outcome MEANS
//
// [Encounter.Intimidate]'s law, one layer on: the verb is told whether the
// check was beaten and this file is told which entry the die chose. What a
// `fact` turns, and what a fleeing creature's mind makes of the run, stay
// where they already live — the disposition graph and rulebooks/dnd5e/behavior.

// The four outcomes an author may write an answer table for: one per social
// verb per verdict.
//
// THE KEY IS THE VERB AND THE VERDICT TOGETHER, which is why failure has a
// table of its own. "A failed Intimidate that raises the alarm makes
// attempting worse than not attempting. That is deliberate, and it is what
// gives the untrained rule teeth" (the design). A verb with no table for the
// outcome that happened does nothing at all and writes no beat — absent means
// absent, not "the creature shrugged".
const (
	// AnswerIntimidated is what the creature does when a threat lands.
	AnswerIntimidated = "intimidated"

	// AnswerIntimidateFailed is what it does when a threat misses.
	AnswerIntimidateFailed = "intimidate_failed"

	// AnswerPersuaded is what it does when an appeal lands.
	AnswerPersuaded = "persuaded"

	// AnswerPersuadeFailed is what it does when an appeal misses.
	AnswerPersuadeFailed = "persuade_failed"
)

// AnswerKeys is every outcome key this build accepts, in authored order:
// each verb's success then its failure. EXPORTED because the authoring
// dialect refuses every other key by name and must list the ones it takes, and
// a second copy of the list there is the drift this constant exists to
// prevent.
var AnswerKeys = []string{
	AnswerIntimidated,
	AnswerIntimidateFailed,
	AnswerPersuaded,
	AnswerPersuadeFailed,
}

// Answer is ONE entry in an outcome's table: how likely it is, what the
// creature says, and the one thing it does.
//
// EXACTLY ONE OUTCOME WORD, OR NONE. An entry that both teaches a fact and
// sends the creature running is refused at the door rather than ordered here,
// so an author never has to guess which happens first (the design's rule of
// the table). An entry with neither is legal only when it has a line to say:
// a creature that answers and does nothing is a real outcome, and a creature
// that neither speaks nor acts is a row the author wrote for no reason.
type Answer struct {
	// Weight is this entry's share of the table. AT LEAST 1, refused
	// otherwise: a zero would be an entry that can never fire, sitting in a
	// file looking like a possibility. Weights are relative and summed by
	// the picker, so 3 and 1 and 75 and 25 both mean the same thing.
	//
	// The authoring dialect fills an omitted weight with 1 before it gets
	// here, so every entry that reaches this composition carries its own
	// number and nothing down here has to know what "omitted" meant.
	Weight int

	// Say is the creature's line, VERBATIM. The composition never composes
	// it, never templates it and never translates it — it is carried onto
	// [BeatAnswered] exactly as the author typed it. Empty means the creature
	// says nothing, which is the common case for a table that only turns a
	// disposition.
	Say string

	// Fact is the world fact every witness learns when this entry fires —
	// what `on: { intimidated: { fact: … } }` used to be, now one entry of
	// one outcome's table. Empty means this entry teaches nothing.
	//
	// THE PLAY RECORD AND THE WORLD JOURNAL ARE BOTH WRITTEN AND NEITHER IS
	// THE OTHER'S CACHE (Kirk, 2026-09-16). The deed the verb landed is the
	// creature's own memory of who leaned on it and goes with the run; this
	// is what the camp comes to know and carries out of it.
	Fact FactID

	// Flee sends the creature away from whoever just spoke to it, for its
	// own full speed, as a DIRECTED MOVE off anybody's turn (directive.go,
	// rpg-project#430) — not a flag, and not a driven turn. "A flee flag on
	// the member is state the driver would have to read live, which rule A2
	// forbids" (intimidate.md's first broken cut). The deed the verb already
	// landed is the fear; this is the running.
	//
	// A creature with a wall at its back stays where it is and the beat
	// still says this entry fired. Not going anywhere is an outcome.
	Flee bool
}

// BeatAnswered is the "beat" value of the story beat appended when the world
// rolls a creature's answer to a social verb.
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
// IT IS THE ONLY ACCOUNT OF THE ANSWER ROLL, and the author's line reaches
// the table through it and nowhere else.
const BeatAnswered = "answered"

// answerCauseFlee is the cause every beat of a fleeing creature's directed
// walk carries, so an observer can tell a rout from a step and from a shove
// ([DirectInput.Cause] is required for exactly this reason).
var answerCauseFlee = core.Ref{Module: "encounter", Type: "answer", ID: "flee"}

// answerInput is one settled verdict looking for the creature's answer to it.
type answerInput struct {
	// creature is whose placement carries the table — the member the verb
	// was aimed at.
	creature MemberID

	// actor is who spoke. The anchor a flee runs away from.
	actor MemberID

	// key is the outcome, one of [AnswerKeys].
	key string

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
// NO TABLE, NO ROLL, NO BEAT. A placement that authored nothing for this
// outcome produces silence, which is distinguishable from an entry that fired
// and did nothing: one writes no beat at all.
//
// Order is fact, then walk, then beat — the opposite of the verb's own
// order and deliberately so. [Encounter.Intimidate] appends its beat first
// because the threat is the CAUSE of everything after it; this beat is the
// RESULT, and it reports the entry, the roll and the line that only exist
// once the pick has been made.
func (e *Encounter) answer(ctx context.Context, in answerInput) error {
	creature, ok := e.members[in.creature]
	if !ok {
		return fmt.Errorf("react: creature %q: %w", in.creature, ErrNotMember)
	}
	entries := creature.Answers[in.key]
	if len(entries) == 0 {
		return nil
	}

	total := 0
	for _, entry := range entries {
		total += entry.Weight
	}
	if total < 1 {
		// Unreachable: every entry is validated to carry at least 1 at the
		// door it came in through. Refusing rather than rolling a d0 keeps
		// the day that stops being true from being a panic in dice.
		return fmt.Errorf("react: %q weighs %d: %w", in.key, total, ErrBadAnswer)
	}

	roll, err := in.roller.Roll(ctx, total)
	if err != nil {
		return fmt.Errorf("react: %q: %w", in.key, err)
	}

	index, entry := pickAnswer(entries, roll)

	if entry.Fact != "" {
		for _, id := range in.witnesses {
			if err := e.learnFact(id, entry.Fact, BeatAnswered, in.at); err != nil {
				return fmt.Errorf("react: %w", err)
			}
		}
	}

	if entry.Flee {
		if err := e.fleeFrom(ctx, in.creature, in.actor); err != nil {
			return fmt.Errorf("react: %w", err)
		}
	}

	return e.appendAnsweredBeat(in, index, entry, roll, total)
}

// pickAnswer walks the table accumulating weights and returns the entry the
// face landed in, with its index.
//
// AUTHORED ORDER, ACCUMULATED, so the same face always picks the same entry —
// a table whose answer depended on map iteration is one no transcript could
// compare (C8). The face is 1..total by the roller's own contract, so the
// final entry is reachable and the loop always returns inside itself; the
// fallback exists so a roller that breaks its contract picks the last entry
// rather than reading off the end.
func pickAnswer(entries []Answer, roll int) (int, Answer) {
	acc := 0
	for i, entry := range entries {
		acc += entry.Weight
		if roll <= acc {
			return i, entry
		}
	}
	last := len(entries) - 1

	return last, entries[last]
}

// fleeFrom routes a creature as far from the actor as its own speed pays for
// and walks it there.
//
// THE BUBBLE-FREE PAIR, which is the primitive this slice buys (R3): Route
// reads and Direct writes, neither asks whose turn it is, and neither needs a
// fight to exist. A neutral goblin in a front room with no initiative order
// can now run, which is the thing the engine could not do before
// (ideas/shenanigans/front-room-goblin.md, "Outside a fight").
//
// IT DOES NOT PROVOKE. A creature bolting because somebody frightened it is
// not taking its own move, and the one rule the composition could apply here
// — an opportunity attack — would be the fight's rule reaching into a room
// with no fight in it. Dissonant Whispers provokes because the spell says so;
// nothing says so here.
//
// NOWHERE TO GO IS NOT AN ERROR. A creature with a wall at its back stays
// where it is and the beat still reports the entry that fired: "if Route finds
// no step, the creature stays and the beat still says the entry fired".
func (e *Encounter) fleeFrom(ctx context.Context, creature, actor MemberID) error {
	from, ok := e.members[actor]
	if !ok {
		return fmt.Errorf("flee: actor %q: %w", actor, ErrNotMember)
	}
	anchor, err := e.cellOf(from)
	if err != nil {
		return fmt.Errorf("flee: actor %q: %w", actor, err)
	}

	runner, ok := e.members[creature]
	if !ok {
		return fmt.Errorf("flee: creature %q: %w", creature, ErrNotMember)
	}
	budget := CellsFromFeet(runner.SpeedFeet)
	if budget <= 0 {
		// A roster row that carried no speed. The entry still fired and the
		// beat still says so; there is simply nothing to walk, and guessing a
		// distance for a creature nobody said was fast would be this module
		// inventing a rule.
		return nil
	}

	route, err := e.Route(RouteInput{Mover: creature, Policy: MoveAway, Anchor: anchor, Budget: budget})
	if err != nil {
		return fmt.Errorf("flee: %w", err)
	}
	if len(route.Path) == 0 {
		return nil
	}

	if _, err := e.Direct(ctx, DirectInput{
		Mover: creature, Cause: answerCauseFlee, Route: route.Path, Provokes: false,
	}); err != nil {
		return fmt.Errorf("flee: %w", err)
	}

	return nil
}

// appendAnsweredBeat writes what the table saw: which creature answered, to
// which verb and verdict, the die and the weights it was rolled against, the
// entry that fired, the word it carried and the line the author wrote.
//
// EVERY FIELD IS WRITTEN UNCONDITIONALLY, the `intimidated` beat's rule: a
// reader downstream must not get a third state out of an absent key. `word` is
// empty for an entry that only speaks, and that is an answer rather than a
// gap.
func (e *Encounter) appendAnsweredBeat(
	in answerInput, index int, entry Answer, roll, of int,
) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":     BeatAnswered,
		"creature": string(in.creature),
		"verb":     in.verb,
		"beaten":   in.beaten,
		"roll":     roll,
		"of":       of,
		"entry":    index,
		"word":     answerWord(entry),
		"say":      entry.Say,
		"fact":     string(entry.Fact),
	})
	if err != nil {
		return fmt.Errorf("react: marshal beat: %w", err)
	}

	if _, err := e.appendBeat(&record.AppendInput{
		At:       in.at,
		Audience: in.witnesses,
		Tags:     map[string]string{"tag": BeatAnswered},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("react: %w", err)
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
	default:
		return ""
	}
}

// answerKeyFor is the outcome key a verb's verdict lands under. One
// function, so the verbs and the authoring dialect cannot spell the pairing
// differently.
func answerKeyFor(verb string, beaten bool) string {
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

// validateAnswers refuses a table this composition could not roll: an
// outcome key it does not know, and an entry that weighs less than 1.
//
// AT THE DOOR, NOT AT THE ROLL. A placement whose table can never fire is a
// misconfiguration, and the moment to find out is when the member joins —
// not when a player finally threatens it and the panel reports an internal
// error. The authoring dialect refuses the same two things in the author's own
// words; this is the composition refusing them for every other caller,
// including a persisted blob somebody edited.
func validateAnswers(answers map[string][]Answer) error {
	for key, entries := range answers {
		if !validAnswerKey(key) {
			return fmt.Errorf("answer %q is not an outcome this build lands: %w", key, ErrBadAnswer)
		}
		for i, entry := range entries {
			if entry.Weight < 1 {
				return fmt.Errorf("answer %q entry %d weighs %d: %w",
					key, i, entry.Weight, ErrBadAnswer)
			}
		}
	}

	return nil
}

// validAnswerKey reports whether a key is one of [AnswerKeys].
func validAnswerKey(key string) bool {
	for _, known := range AnswerKeys {
		if key == known {
			return true
		}
	}

	return false
}

// cloneAnswers deep-copies an answer table so a caller's map and the
// composition's cannot be the same one — [MemberInput]'s standing rule that
// nothing crossing this door stays aliased.
func cloneAnswers(answers map[string][]Answer) map[string][]Answer {
	if answers == nil {
		return nil
	}

	out := make(map[string][]Answer, len(answers))
	for key, entries := range answers {
		out[key] = append([]Answer(nil), entries...)
	}

	return out
}
