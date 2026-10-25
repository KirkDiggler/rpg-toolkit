// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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
// [Encounter.Speak] lands a verdict. What the creature DOES about that verdict
// is not this composition's opinion and not the verb's: the author wrote a
// weighted list of entries for each verdict on the OPTION that was spoken
// (rpg-project#569 R2), and the world rolls one die to pick which entry fires.
// "The setup for the author is what I am most interested in" (Kirk).
//
// # This file is the spoken HALF of one evaluator
//
// The entries, the words, the weights and the roll all live in table.go,
// because a creature's turn rolls the SAME evaluator under [AnswerTime]
// (rpg-project#465). What is left here is what a SPOKEN verdict does with a
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
// [Encounter.Speak]'s law, one layer on: the verb is told whether the check
// was beaten and this file is told which entry the die chose. What a
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
// EXPORTED BECAUSE A DECODER READS IT, [BeatSpoke]'s reason exactly: the
// session-side decoder is written against this constant in the same wave, so a
// rename fails to compile there instead of quietly producing a beat nobody
// renders.
//
// IT IS THE ONLY ACCOUNT OF THE PICK, and the author's line reaches the table
// through it and nowhere else.
const BeatAnswered = "answered"

// BeatStayed is the "beat" value appended when a driven creature's word did
// nothing: a routed walk that moved nobody, an attack that could not land, or
// a `toward` whose target was already in reach.
//
// A SPENT ROUND THAT MOVED NOBODY MUST BE VISIBLE (Kirk's walk,
// rpg-project#465). The world had been spending the bandits' rounds and
// narrating nothing, so the log could not tell a creature that refused from
// one nobody asked from one sent somewhere it could not reach. It carries the
// cause that routed it and the route's own sentence about why it stopped.
//
// AND SO MUST A SWING OR A STEP THAT NEVER HAPPENED (rpg-toolkit#2000,
// #2001): `answered … attack` followed by `turn-ended` told the author
// nothing. A refused attack carries `encounter:intent:attack` and the reason
// ("target out of reach"); a `toward` already in reach carries the table's
// own cause and "already in reach".
const BeatStayed = "stayed"

// BeatTempered is the "beat" value appended when a faction's authored mix
// deals one member its temperament — "so the streamer sees which goblin came
// out the coward" (design §3).
//
// ONE PER DEALT MEMBER, AND NONE FOR AN AUTHORED WORD. A placement whose
// author wrote `temper: coward` rolled nothing, and a beat saying it did
// would be this composition inventing a die.
const BeatTempered = "tempered"

// answerInput is one settled verdict looking for the answer to it.
type answerInput struct {
	// creature is the member the option was spoken to — whose temperament
	// loads the die and who a `flee` entry makes run. Empty when the option
	// is a prop's.
	creature MemberID

	// prop is the prop the option was spoken to, or empty when it is a
	// creature's. Exactly one of creature and prop is set.
	prop PropID

	// actor is who spoke. The one a `flee` entry's deed names.
	actor MemberID

	// option is the id of the option that was spoken, carried onto the beat
	// so a reader knows which line this answers.
	option string

	// key is the verdict, one of [OptionKeys] — the key the die is rolled
	// under and the beat reports.
	key AnswerKey

	// rows is the option's rows for that verdict: what the die picks from.
	rows []Answer

	// verb is the bare verb name carried onto the beat ("speak"), so a
	// reader switching on it need not infer it from the key.
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

// answerOutput is what one settled verdict's answer came to. A VALUE, ALWAYS:
// when no row fired, pick is nil and that is the answer rather than a missing
// one — this module never hands back (nil, nil).
type answerOutput struct {
	// pick is the row the world's die picked, with its arithmetic, or nil
	// when the option has no row for this verdict.
	pick *Pick
}

// answer rolls the answer to a settled verdict and carries it out, returning
// what it came to — [answerOutput.pick] nil when nothing fired.
//
// NO ELIGIBLE ENTRY, NO ROLL, NO BEAT. An option that authored nothing for
// this verdict produces silence, which is distinguishable from an entry that
// fired and did nothing: one writes no beat at all.
//
// ONE EVALUATOR FOR A CREATURE AND A PROP. The rows are the option's, so the
// pick is [behavior.Pick] over a one-key table of them. A creature's
// temperament loads the die and its board is the [Facts] the evaluator reads;
// a prop has neither, so its die is the soldier's flat one and it reads
// nothing — which is all an option's rows can ask of it, since a `when` is
// refused under a verdict.
//
// Order is fact, then deed, then beat — the opposite of the verb's own order
// and deliberately so. [Encounter.Speak] appends its beat first because the
// line is the CAUSE of everything after it; this beat is the RESULT, and it
// reports the entry, the roll and the line that only exist once the pick has
// been made.
func (e *Encounter) answer(ctx context.Context, in answerInput) (answerOutput, error) {
	var temper Temper
	var facts Facts
	if in.creature != "" {
		creature, ok := e.members[in.creature]
		if !ok {
			return answerOutput{}, fmt.Errorf("answer: creature %q: %w", in.creature, ErrNotMember)
		}
		temper = creature.Temper
		read, err := e.factsFor(in.creature)
		if err != nil {
			return answerOutput{}, fmt.Errorf("answer: %w", err)
		}
		facts = read
	}

	// THE VERB'S OWN DIE, and the target is whose roll it is — said on the
	// beat below, which is the only channel a shared roller leaves for it.
	chosen, err := behavior.Pick(ctx, &behavior.PickInput{
		Key: in.key, Table: Table{in.key: in.rows}, Temper: temper, Facts: facts, Die: in.roller,
	})
	if err != nil {
		return answerOutput{}, fmt.Errorf("answer: %w", err)
	}
	if chosen == nil {
		return answerOutput{}, nil
	}

	if chosen.Answer.Fact != "" {
		for _, id := range in.witnesses {
			if err := e.learnFact(id, chosen.Answer.Fact, BeatAnswered, in.at); err != nil {
				return answerOutput{}, fmt.Errorf("answer: %w", err)
			}
		}
	}

	// A prop's option cannot carry `flee` — refused at its door
	// ([validatePropOptions]) — so this is a creature's row whenever it runs.
	if chosen.Answer.Flee && in.creature != "" {
		if err := e.landFled(in.creature, in.actor, in.at); err != nil {
			return answerOutput{}, fmt.Errorf("answer: %w", err)
		}
	}

	if err := e.appendAnsweredBeat(in, chosen); err != nil {
		return answerOutput{}, err
	}

	// THE LINE IS A VOICE (rpg-project#569 R5): every `say` on any answer row
	// is heard beyond the room by whoever's ears beat it — after the beat
	// that carries the line, which is its cause. A prop has no voice: its
	// line is the author narrating what the speaker found, not a sound.
	if chosen.Answer.Say != "" && in.creature != "" {
		if err := e.makeSound(noiseSay, in.creature, in.at); err != nil {
			return answerOutput{}, fmt.Errorf("answer: %w", err)
		}
	}

	return answerOutput{pick: chosen}, nil
}

// answerTime carries out what a creature's `time` pick SAID: its line is a
// voice, and its fact is an alarm (rpg-project#569 R6).
//
// THE ALARM'S AUDIENCE IS WHOEVER SEES THE CRIER AND WHOEVER HEARS IT. A fact
// on a verdict teaches the witnesses of the line it answers; a creature
// raising the alarm on its own time is heard through the doors, so the
// hearers of a voice at its cell ([Encounter.hearersOf] at [SoundVoice]'s DC)
// learn it beside everyone who can see it ([Encounter.witnessesOf]). No task
// widens who learns beyond that (the plan's audience law).
//
// A FACT IS SAID ALOUD, so a row that teaches one makes a voice even when the
// author wrote no line for it: the hearers who learn it heard something, and
// a `heard: { kind: voice }` that did not hold for them would be the two
// readings of one shout disagreeing.
//
// Stamped with the clock's reading now, the one the pick's own beat carries.
func (e *Encounter) answerTime(member MemberID, chosen Answer) error {
	if chosen.Say == "" && chosen.Fact == "" {
		return nil
	}
	at := uint64(e.clock.ToData().HighWater)
	if err := e.makeSound(noiseSay, member, at); err != nil {
		return fmt.Errorf("time answer: %w", err)
	}
	if chosen.Fact == "" {
		return nil
	}

	where, witnesses, err := e.audienceOf(member)
	if err != nil {
		return fmt.Errorf("time answer: %w", err)
	}
	_, voiceDC, _ := soundOf(noiseSay)
	heard, err := e.hearersOf(where, voiceDC, member)
	if err != nil {
		return fmt.Errorf("time answer: %w", err)
	}
	audience := map[MemberID]bool{}
	for _, id := range witnesses {
		audience[id] = true
	}
	for _, h := range heard {
		audience[h.member] = true
	}
	learners := make([]MemberID, 0, len(audience))
	for id := range audience {
		learners = append(learners, id)
	}
	sort.Slice(learners, func(i, j int) bool { return learners[i] < learners[j] })
	for _, id := range learners {
		if err := e.learnFact(id, chosen.Fact, BeatAnswered, at); err != nil {
			return fmt.Errorf("time answer: %w", err)
		}
	}

	return nil
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
// EVERY FIELD IS WRITTEN UNCONDITIONALLY, the `spoke` beat's rule: a reader
// downstream must not get a third state out of an absent key. `word` is empty
// for an entry that only speaks; `verb`, `option` and `beaten` are empty and
// false for a `time` pick — nothing spoke — and `creature` is empty when a
// prop answered; all of those are answers rather than gaps.
func (e *Encounter) appendAnsweredBeat(in answerInput, chosen *Pick) error {
	// THE SYMBOLS FIRST (rpg-project#674): a creature's option rows are
	// described before the first answer that indexes them. A prop's rows are
	// not — a prop holds no member record to remember what it described —
	// and its answer is read by index.
	if in.creature != "" {
		if err := e.describeTable(
			in.creature, in.option, in.key, in.rows, e.members[in.creature].Temper, in.at,
		); err != nil {
			return fmt.Errorf("answer: %w", err)
		}
	}

	payload, err := json.Marshal(answeredBeatBody(answeredBy{
		creature: in.creature, prop: in.prop, verb: in.verb, option: in.option, beaten: in.beaten,
	}, chosen))
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

// answeredBy is who answered and to what: the creature (or the prop) whose
// pick it was, and — for a spoken answer — the verb, the option and the
// verdict. The zero value of every field but the creature is a `time` pick.
type answeredBy struct {
	creature MemberID
	prop     PropID
	verb     string
	option   string
	beaten   bool

	// target is the member a `time` pick's intent acts on ([intentTarget]),
	// empty for a spoken answer and for an intent that names nobody.
	target MemberID
}

// answeredBeatBody is the beat's payload for one pick — ONE BODY for a spoken
// answer and a `time` pick, so the two can never come to describe their
// shared arithmetic differently.
func answeredBeatBody(by answeredBy, chosen *Pick) map[string]interface{} {
	candidates := make([]map[string]interface{}, 0, len(chosen.Candidates))
	for _, c := range chosen.Candidates {
		candidates = append(candidates, map[string]interface{}{
			"entry": c.Entry, "weight": c.Weight, "percent": c.Percent, "loaded": c.Loaded,
		})
	}

	return map[string]interface{}{
		"beat":       BeatAnswered,
		"creature":   string(by.creature),
		"prop":       string(by.prop),
		"key":        string(chosen.Key),
		"verb":       by.verb,
		"option":     by.option,
		"beaten":     by.beaten,
		"roll":       chosen.Roll,
		"of":         chosen.Of,
		"entry":      chosen.Entry,
		"path":       pathOf(chosen),
		"candidates": candidates,
		"rows":       rowVerdicts(chosen),
		"temper":     chosen.Temper,
		"word":       answerWord(chosen.Answer),
		"selector":   selectorWord(chosen.Answer),
		"target":     string(by.target),
		"say":        chosen.Answer.Say,
		"fact":       string(chosen.Answer.Fact),
	}
}

// pathOf is the pick's path through the table, level by level
// (rpg-project#670): `[3]` for a word row, `[3, 1]` for the second row inside
// the group at entry 3. `entry` is its first element; a hold's path is
// empty, never null, the beat's every-field-written rule.
func pathOf(chosen *Pick) []int {
	if len(chosen.Path) == 0 {
		return []int{}
	}

	return append([]int(nil), chosen.Path...)
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
	case entry.Search != nil:
		return "search"
	default:
		return ""
	}
}

// intentTarget is the member an intent acts on: whom an attack strikes, whom a
// walk closes on, whom a run is measured from — what the selector resolved to
// (rpg-project#669), so the record says whom `attacker` named. Empty for an
// intent that names nobody: a hold, a walk to a cell, a selector that found
// no one.
func intentTarget(intent TurnIntent) MemberID {
	switch i := intent.(type) {
	case Attack:
		return i.Target
	case Move:
		return i.Target
	case Routed:
		return i.Anchor
	default:
		return ""
	}
}

// selectorWord is the entry's selector as the author wrote it — the word, or
// "at" for an authored cell, "path" for an authored route, or empty for a
// word that selects nothing.
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
	if len(sel.Path) > 0 {
		return "path"
	}

	return string(sel.Word)
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
		if IsRetiredTableKey(key) {
			return fmt.Errorf("`%s` is no longer a table key: author an option: %w", key, ErrBadAnswer)
		}
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
// key, and its conditions' and selector's shape. The key is [AnswerTime] for a
// table's row and one of [OptionKeys] for an option's — a verdict, where
// `fact` and `flee` are legal and the four time words and a `when` are not.
//
// `fact` IS LEGAL ON BOTH (rpg-project#569 R6): on a verdict it teaches the
// witnesses of the spoken line, and on `time` it is a creature raising an
// alarm — its audience is whoever HEARS the speaker as well as whoever sees
// them ([Encounter.answerTime]). `flee` stays a verdict's word: a creature
// runs because somebody made it, and that somebody is the actor of a verb.
//
// THE SHAPE RULE IS mind/behavior's. One condition per `when` entry, one
// target per selector, a `path` only on `toward`, `sound` never on `attack`:
// [behavior.Answer.Validate] is the one statement of those, and this door
// calls it rather than keeping a second copy that could come to disagree.
// What is left here is what only this composition knows — which enemy, deed
// and sound words this build reads, which key a word is legal under, and
// whether `actor` has a deed to name.
func validateEntry(key AnswerKey, i int, entry Answer) error {
	if entry.Weight < 1 {
		return fmt.Errorf("answer %q entry %d weighs %d: %w", key, i, entry.Weight, ErrBadAnswer)
	}
	if len(entry.Rows) > 0 || entry.As != "" || entry.From != "" || entry.Params != nil {
		return validateGroupEntry(key, i, entry)
	}

	social := key != AnswerTime
	word := answerWord(entry)
	switch word {
	case "flee":
		if !social {
			return fmt.Errorf("answer %q entry %d: `%s` answers a social verdict, not time: %w",
				key, i, word, ErrBadAnswer)
		}
	case "hold", "attack", "toward", "away", "search":
		if social {
			return fmt.Errorf("answer %q entry %d: `%s` is what a creature does with time: %w",
				key, i, word, ErrBadAnswer)
		}
	}
	if len(entry.When) > 0 && social {
		return fmt.Errorf("answer %q entry %d: a social verdict is already the condition: %w",
			key, i, ErrBadAnswer)
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("answer %q entry %d: %w: %w", key, i, err, ErrBadAnswer)
	}
	for j := range entry.When {
		if err := validateWhenWords(&entry.When[j]); err != nil {
			return fmt.Errorf("answer %q entry %d: %w", key, i, err)
		}
	}
	if sel := selectorOf(entry); sel != nil {
		if sel.Word == SelectorActor && deedWhen(entry.When) == nil {
			return fmt.Errorf("answer %q entry %d: `actor` names the actor of a deed and this entry names none: %w",
				key, i, ErrBadAnswer)
		}
		if sel.At != nil && entry.Toward == nil {
			return fmt.Errorf("answer %q entry %d: a cell is somewhere to walk toward, not %s: %w",
				key, i, word, ErrBadAnswer)
		}
		if cell, retraced := RetracedCell(sel.Path); retraced {
			return fmt.Errorf("answer %q entry %d: %s: %w", key, i, retraceRefusal(cell), ErrBadAnswer)
		}
	}

	return nil
}

// validateGroupEntry refuses a group row (rpg-project#670): its shape is
// mind/behavior's ([behavior.Answer.Validate]: no word of its own, at least
// one row, `as` a temperament word, the depth guard), its own `when` is
// judged by this build's vocabulary, and every row inside is judged as a row
// of the same key, so a word illegal under the key is as illegal one group
// down. A refusal inside names the path to the row ("entry 2 → 1").
func validateGroupEntry(key AnswerKey, i int, entry Answer) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("answer %q entry %d: %w: %w", key, i, err, ErrBadAnswer)
	}
	if len(entry.When) > 0 && key != AnswerTime {
		return fmt.Errorf("answer %q entry %d: a social verdict is already the condition: %w",
			key, i, ErrBadAnswer)
	}
	for j := range entry.When {
		if err := validateWhenWords(&entry.When[j]); err != nil {
			return fmt.Errorf("answer %q entry %d: %w", key, i, err)
		}
	}
	for j, row := range entry.Rows {
		if err := validateEntry(key, j, row); err != nil {
			return fmt.Errorf("answer %q entry %d, inside its group: %w", key, i, err)
		}
	}

	return nil
}

// validateWhenWords refuses the words inside one condition that this build
// does not read: an enemy band, a deed, a scope or a deed span it has no
// meaning for, or a sound kind nothing here makes ([SoundKinds]). A `not`
// is judged by what it wraps.
//
// THE SHAPE IS ALREADY JUDGED ([behavior.When.Validate], asked first by
// [validateEntry]): exactly one condition, a `heard` with a kind and a span.
// This is the vocabulary, which only the rulebook's composition can seal.
func validateWhenWords(when *When) error {
	switch {
	case when.Not != nil:
		return validateWhenWords(when.Not)
	case when.Heard != nil:
		if !soundKindNamed(when.Heard.Kind) {
			if when.Heard.Kind.Pattern != "" {
				return fmt.Errorf("`heard: { kind: %s }` matches no sound this build makes: they are %s: %w",
					when.Heard.Kind, strings.Join(SoundKinds, ", "), ErrBadAnswer)
			}

			return fmt.Errorf("`heard: { kind: %s }` is not a sound this build makes: they are %s: %w",
				when.Heard.Kind, strings.Join(SoundKinds, ", "), ErrBadAnswer)
		}

		return nil
	case !when.Fact.IsZero():
		return nil
	case when.Enemy != "":
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
	// THE DEED SPAN IS THIS DOOR'S, NOT mind/behavior's (its review,
	// rpg-project#569): behavior refuses a `heard` span below 1 because it
	// owns that condition, and leaves the deed form's span to the rulebook
	// that seals the deed words — so "counted from 1" for a deed is refused
	// here and nowhere else.
	if when.Within < 1 {
		return fmt.Errorf("a span of %d rounds is counted from 1: %w", when.Within, ErrBadAnswer)
	}
	// THE SCOPE IS REFUSED HERE TOO, which is the one field of this condition
	// that was not (rpg-toolkit#1890 thread 1). This function is the door for
	// "every other caller, including a persisted blob somebody edited", so a
	// scope a future build wrote — or a hand-edit — must be refused by name
	// rather than reaching the evaluator, where an unknown scope matches
	// nothing and would silently revert an `on: ally` row to the self reading.
	if refusal := scopeRefusal(when.Scope); refusal != "" {
		return fmt.Errorf("%s: %w", refusal, ErrBadAnswer)
	}

	return nil
}

// retraceRefusal is the sentence a patrol route whose legs cross is refused
// with ([RetracedCell]), naming the cell — dungeon-absolute here; the dialect
// says the same thing in the author's own offset coordinates.
func retraceRefusal(cell spatial.Position) string {
	return fmt.Sprintf("this route retraces itself at [%g,%g]: a patrol walks a loop — "+
		"add a waypoint so no leg crosses another", cell.X, cell.Y)
}

// deedWhen is the condition in an entry's list that names a deed, or nil when
// none does — the deed `actor` names the actor of, and the scope `attacker`
// reads.
//
// THE LIST'S OWN ENTRIES ONLY, never one inside a `not`: `not: { attacked }`
// says nobody attacked, and there is no actor of a deed that did not happen.
// The first in authored order when several name one, which is the order an
// author reads them in.
func deedWhen(when []When) *When {
	for i := range when {
		if when[i].Deed != "" {
			return &when[i]
		}
	}

	return nil
}

// scopeRefusal names why a scope is not one this build reads, or empty when it
// is. The default is the empty string, so an unscoped condition — every
// document authored before scopes existed — passes.
func scopeRefusal(scope DeedScope) string {
	if scope == ScopeSelf {
		return ""
	}
	for _, known := range DeedScopes {
		if scope == known {
			return ""
		}
	}

	return fmt.Sprintf("`%s` is not a scope this build reads: they are ally, actor", scope)
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

// cloneAnswer deep-copies one entry's pointers and slices.
func cloneAnswer(entry Answer) Answer {
	out := entry
	if entry.When != nil {
		out.When = make([]When, 0, len(entry.When))
		for _, w := range entry.When {
			out.When = append(out.When, cloneWhen(w))
		}
	}
	out.Attack = cloneSelector(entry.Attack)
	out.Toward = cloneSelector(entry.Toward)
	out.Away = cloneSelector(entry.Away)
	if entry.Search != nil {
		search := *entry.Search
		out.Search = &search
	}
	if entry.Rows != nil {
		out.Rows = make([]Answer, 0, len(entry.Rows))
		for _, row := range entry.Rows {
			out.Rows = append(out.Rows, cloneAnswer(row))
		}
	}
	if entry.Params != nil {
		out.Params = make(map[string]string, len(entry.Params))
		for k, v := range entry.Params {
			out.Params[k] = v
		}
	}

	return out
}

// wordRows is every WORD row of a list, a group's rows in its place, in
// authored order: what a walk over "every row an author wrote" means once a
// row can be a table (rpg-project#670). A group itself teaches nothing and
// says nothing; its rows do.
func wordRows(rows []Answer) []Answer {
	var out []Answer
	for _, row := range rows {
		if len(row.Rows) > 0 {
			out = append(out, wordRows(row.Rows)...)
			continue
		}
		out = append(out, row)
	}

	return out
}

// cloneWhen deep-copies one condition, the `heard` body and a `not`'s inner
// condition included.
func cloneWhen(w When) When {
	out := w
	if w.Heard != nil {
		heard := *w.Heard
		out.Heard = &heard
	}
	if w.Not != nil {
		inner := cloneWhen(*w.Not)
		out.Not = &inner
	}

	return out
}

// cloneSelector deep-copies a selector, the cell it may name and the route
// it may walk.
func cloneSelector(sel *Selector) *Selector {
	if sel == nil {
		return nil
	}
	out := *sel
	if sel.At != nil {
		at := *sel.At
		out.At = &at
	}
	if sel.Path != nil {
		out.Path = append([]spatial.Position(nil), sel.Path...)
	}
	out.Facings = copyFacings(sel.Facings)
	if sel.Waits != nil {
		out.Waits = append([]int(nil), sel.Waits...)
	}

	return &out
}
