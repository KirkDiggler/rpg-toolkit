// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// table.go is THE CREATURE'S TABLE (rpg-project#465,
// ideas/creature-table/design.md).
//
// A creature decides by rolling on an authored weighted table, and four things
// load that die: the rulebook's default for its kind, the author's orders, its
// own temperament, and what it has seen and suffered. There is one evaluator
// and it is [pick]; a social verb's answer and a creature's turn are the same
// roll under two keys.
//
// # Why this replaced the mind
//
// Three surfaces claimed to say what a creature does: the mind ladder
// (presets in Go), the decider (wired to nothing), and this table. Only the
// table is a tool an author can hold, which is the product. The mind was a
// black box with three words on the lid and a streamer could not open it.
// Both are deleted rather than layered under (design §7).
//
// What the mind got right is kept and is read from here rather than
// reimplemented: the outcome of anything is TESTIMONY the creature holds
// (never a flag), and who a creature believes is where comes from perception,
// per observer. [Facts] is the projection of both.

// AnswerKey is one trigger of a creature's table: a social verb's verdict, or
// the creature having time.
//
// A STRING KIND rather than a bare string because the two kinds of trigger
// differ in WHEN the roll happens — an event key's pick executes inside the
// verb that settled, and [AnswerTime]'s pick is one turn's worth of doing —
// and a caller that has to tell them apart should be switching on a named
// type rather than comparing spellings.
type AnswerKey string

// The five triggers an author may write a table for: one per social verb per
// verdict, and one for the creature having time.
//
// THE SOCIAL KEY IS THE VERB AND THE VERDICT TOGETHER, which is why failure
// has a table of its own. "A failed Intimidate that raises the alarm makes
// attempting worse than not attempting. That is deliberate, and it is what
// gives the untrained rule teeth" (the shenanigans design). A verb with no
// table for the outcome that happened does nothing at all and writes no beat
// — absent means absent, not "the creature shrugged".
const (
	// AnswerIntimidated is what the creature does when a threat lands.
	AnswerIntimidated AnswerKey = "intimidated"

	// AnswerIntimidateFailed is what it does when a threat misses.
	AnswerIntimidateFailed AnswerKey = "intimidate_failed"

	// AnswerPersuaded is what it does when an appeal lands.
	AnswerPersuaded AnswerKey = "persuaded"

	// AnswerPersuadeFailed is what it does when an appeal misses.
	AnswerPersuadeFailed AnswerKey = "persuade_failed"

	// AnswerTime is what the creature does when it has time: its turn in a
	// fight, or a round of the world (worldtime.go).
	//
	// THE ONE TIME KEY, and the reason there is no `attacked` event key
	// beside the social four: what a creature does about being attacked is
	// decided when it next has time, reading `attacked: {within: N}` — which
	// is what the retired retaliator preset's grudge did in Go, moved into
	// the author's sight.
	AnswerTime AnswerKey = "time"
)

// AnswerKeys is every SOCIAL outcome key this build accepts, in authored
// order: each verb's success then its failure. EXPORTED because the authoring
// dialect refuses every other key by name and must list the ones it takes, and
// a second copy of the list there is the drift this variable exists to
// prevent.
var AnswerKeys = []AnswerKey{
	AnswerIntimidated,
	AnswerIntimidateFailed,
	AnswerPersuaded,
	AnswerPersuadeFailed,
}

// TableKeys is every key a table may name — the social four plus
// [AnswerTime]. [AnswerKeys]'s reason for being exported, one level up: the
// dialect refuses a key outside this list by name.
var TableKeys = append(append([]AnswerKey(nil), AnswerKeys...), AnswerTime)

// Table is one creature's whole policy: what it does, keyed by what happened.
//
// LAYERED, AND THE NEAREST LAYER THAT NAMES A KEY WINS WHOLESALE ([Layer],
// design §1). Three layers supply tables — the rulebook's default for the
// monster's kind, the author's orders on a faction, and the author's orders on
// the placement — and there is no merging of entry lists, so an author never
// has to reason about what was added to what. Overriding a key costs the
// entries the layer under it had, and that cost is visible in the file.
type Table map[AnswerKey][]Answer

// Layer returns `over` laid on `base`: for each key, the nearer layer wins
// WHOLESALE (design §1).
//
// NEITHER INPUT IS MUTATED and neither is aliased into the result: a rulebook
// default table is one value shared by every creature of its kind, and a
// layering that wrote through to it would give one goblin's orders to every
// goblin in the game.
//
// A nil result for two empty inputs, the zero value telling the truth: a
// creature with no table anywhere answers nothing and rolls nothing.
func Layer(base, over Table) Table {
	if len(base) == 0 && len(over) == 0 {
		return nil
	}
	out := make(Table, len(base)+len(over))
	for key, entries := range base {
		out[key] = append([]Answer(nil), entries...)
	}
	for key, entries := range over {
		out[key] = append([]Answer(nil), entries...)
	}

	return out
}

// EnemyWord is a `when: { enemy: … }` condition's value: what this creature
// currently holds about anybody it is opposed to.
type EnemyWord string

const (
	// EnemySeen holds when an opposed member is in this creature's sight.
	EnemySeen EnemyWord = "seen"

	// EnemyRemembered holds when none is in sight but one is held from an
	// earlier sighting.
	EnemyRemembered EnemyWord = "remembered"

	// EnemyNone holds when neither.
	EnemyNone EnemyWord = "none"
)

// EnemyWords is every value `enemy:` takes, in the order the design lists
// them. Exported for the dialect's refusal, [AnswerKeys]'s reason.
var EnemyWords = []EnemyWord{EnemySeen, EnemyRemembered, EnemyNone}

// WhenDeeds is every word a `when: { <deed>: { within: N } }` condition may
// name. Exported for the dialect's refusal, [AnswerKeys]'s reason.
//
// THE AUTHOR'S WORDS ARE PAST TENSE, AND THE STORE'S ARE NOT. A condition is
// written from the CREATURE's side — "I was attacked" — while a deed is
// recorded from the WITNESS's — "somebody attacked". `attacked` and
// [DeedAttack] are the same event named from the two ends of it, and
// [DeedVerbFor] is the one place the two vocabularies meet.
var WhenDeeds = []string{"attacked", "intimidated", "persuaded", "fled"}

// DeedVerbFor is the deed a `when` word reads: the verb the deeds channel
// files it under, for the word the author wrote.
//
// ONE MAPPING, IN ONE PLACE, so the dialect's refusal list and the evaluator's
// comparison cannot come to disagree about which word means which deed.
// An unknown word answers empty, which matches no held deed — the refusal for
// one lives at the door the table came in through.
func DeedVerbFor(word string) string {
	switch word {
	case "attacked":
		return DeedAttack
	case "intimidated":
		return DeedIntimidate
	case "persuaded":
		return DeedPersuade
	case "fled":
		return DeedFled
	default:
		return ""
	}
}

// When is a condition on WHAT THIS CREATURE HOLDS, and an entry whose When
// does not hold is not on the table for this roll (design §2).
//
// ABSENT FROM THE ROLL, NOT WEIGHTED ZERO. A zero-weight entry would still
// show on the beat as a possibility nobody could have rolled; an ineligible
// entry is simply not a candidate, and the beat's candidate list is therefore
// the honest account of what the creature could have done.
//
// EXACTLY ONE CONDITION. Either Enemy is set or Deed is; a When with both or
// neither is refused by the dialect that built it and by [Table]'s own
// validation. Two conditions in one entry would be an `and` this design has
// not paid for, and reading it as one would be guessing which.
type When struct {
	// Enemy reads the two booleans on [Facts], or is empty when this
	// condition names a deed instead.
	Enemy EnemyWord

	// Deed is the word for what this creature must hold AGAINST ITSELF — one
	// of [WhenDeeds], in the author's own past tense — or empty when this
	// condition names an enemy instead. [DeedVerbFor] is what turns it into
	// the verb the deeds channel files under.
	Deed string

	// Within is how many rounds ago the deed may have landed and still
	// count, in the clock's own unit ([Encounter.clock]'s round). AT LEAST
	// 1: "a span is counted from 1", the same rule every other span in this
	// repo keeps — a `within: 0` would be a condition that can only hold on
	// the exact tick the deed landed, which is not what an author writing
	// "recently" means.
	//
	// This is the retired preset's "patience", moved out of Go and into the
	// author's sight.
	Within int
}

// SelectorWord is a `<word>: <selector>` entry's target, in the author's own
// vocabulary.
type SelectorWord string

const (
	// SelectorEnemy is the nearest opposed member in sight, else the
	// nearest opposed member remembered.
	SelectorEnemy SelectorWord = "enemy"

	// SelectorAttacker is who last landed an attack on this creature.
	SelectorAttacker SelectorWord = "attacker"

	// SelectorActor is the actor of the deed this entry's `when` named, so
	// `fled` pairs with `away: actor`. Legal only in an entry whose When
	// names a deed: there is no actor otherwise, and an entry that named one
	// would be asking for somebody nobody identified.
	SelectorActor SelectorWord = "actor"
)

// SelectorWords is every selector word, in the design's own order. Exported
// for the dialect's refusal, [AnswerKeys]'s reason.
var SelectorWords = []SelectorWord{SelectorEnemy, SelectorAttacker, SelectorActor}

// Selector names what a `attack`/`toward`/`away` entry acts on: a member
// found by a word, or an authored cell.
//
// EXACTLY ONE OF THE TWO. A Selector with a Word names a member the encounter
// resolves against what this creature sees and remembers; one with At names a
// cell the author wrote, which is legal for `toward` only — walking away from
// a fixed cell is a direction rather than a flight, and nothing has paid for
// it.
type Selector struct {
	// Word is the selector word, or empty when At is set.
	Word SelectorWord

	// At is the authored DUNGEON-ABSOLUTE cell, or nil when Word is set.
	// A pointer rather than a value because the origin is a legal cell and
	// a zero Position could not be told from "no cell named".
	At *spatial.Position
}

// TemperProfile is a temperament: a multiplier per table word, IN PERCENT,
// and nothing else (design §3, R5).
//
// PERCENT RATHER THAN A FLOAT because the pick's whole arithmetic goes on the
// beat and a reader has to be able to add it up: `weight × percent` is the
// loaded share, the sum is the die, and every number in that sentence is an
// integer. A coward's half is 50 and its triple is 300.
//
// CONTENT, NOT GO. The numbers live in the rulebook beside the default tables
// and a walk tunes them; this module multiplies and never names a value.
//
// WHAT A TEMPERAMENT MAY NOT DO (Kirk, "I go with your lean"): it does not add
// entries where a table is silent, it has no triggers, and it holds no memory.
// The moment it grows one of those it is the mind again under a new name.
type TemperProfile struct {
	// Attack, Toward, Away, Flee and Hold are the percent multiplier for an
	// entry carrying that word. The ZERO PROFILE IS A SOLDIER — every factor
	// 100 — which is what [Temper.factor] answers for a creature nobody gave
	// a temperament, so "no temper" and "soldier" are one state rather than
	// two that have to agree.
	Attack int `json:"attack,omitempty"`
	Toward int `json:"toward,omitempty"`
	Away   int `json:"away,omitempty"`
	Flee   int `json:"flee,omitempty"`
	Hold   int `json:"hold,omitempty"`
}

// Temper is what a member was given: a word and the profile that word means,
// or a mix to deal one from.
//
// THE PROFILE IS THE CALLER'S TO FILL. The three words are rulebook content
// (rulebooks/dnd5e ships them beside the default tables) and this module
// cannot import the rulebook (C1), so the caller hands over the numbers the
// word means. What this module owns is the multiplication and the deal.
type Temper struct {
	// Word is the temperament this member has, or empty for none — which is
	// a soldier, and stores nothing.
	Word string `json:"word,omitempty"`

	// Profile is what Word means, filled by the caller from the rulebook.
	Profile TemperProfile `json:"profile,omitzero"`

	// Mix is a faction's authored spread, word → positive share
	// (`temper: { coward: 1, soldier: 2, aggressive: 1 }`). Non-nil only on
	// the way IN: the encounter deals one word from it at Join, through the
	// Roller, with the FACTION as the die's entity, and stores the dealt
	// Word and Profile. A member's stored Temper never carries a Mix.
	Mix map[string]int `json:"mix,omitempty"`

	// Profiles is what every word in Mix means, filled by the caller from
	// the rulebook for the same reason Profile is. A Mix naming a word this
	// map does not is refused at the door ([ErrBadTemper]) rather than
	// dealing a temperament nobody could apply.
	Profiles map[string]TemperProfile `json:"profiles,omitempty"`
}

// factor is the percent multiplier this temperament loads an entry's word
// with.
//
// 100 FOR A FACT, FOR A BARE LINE, AND FOR A SOLDIER. A temperament is a
// disposition toward ACTING — attacking, closing, running, standing still —
// and there is nothing for it to say about a creature teaching the room a
// fact or answering with a line. The zero profile is the soldier, so a member
// with no temperament at all takes this same path (design §3's "and absent").
func (t Temper) factor(a Answer) int {
	if t.Profile == (TemperProfile{}) {
		return 100
	}
	switch {
	case a.Hold:
		return t.Profile.Hold
	case a.Attack != nil:
		return t.Profile.Attack
	case a.Toward != nil:
		return t.Profile.Toward
	case a.Away != nil:
		return t.Profile.Away
	case a.Flee:
		return t.Profile.Flee
	default:
		return 100
	}
}

// HeldDeed is one deed this creature holds AGAINST ITSELF, projected out of
// its own holdings for the table to read: what happened, who did it, and
// when.
//
// THE CREATURE'S OWN TESTIMONY, never the encounter's live state. A deed is
// stamped when it happened and never restamped, so [Facts.Now] minus At is
// how old it is in rounds.
type HeldDeed struct {
	// Kind is the deed's verb — [DeedAttack], [DeedIntimidate],
	// [DeedPersuade], [DeedFled].
	Kind string

	// Actor is who did it. What `actor` selects.
	Actor MemberID

	// At is the clock reading the deed was landed at.
	At uint64
}

// Facts is everything a [When] condition reads: what this creature can see,
// what it remembers, and what has been done to it.
//
// A PROJECTION, BUILT PER PICK. There is no cache: a creature's facts are its
// holdings and the clock, both of which the encounter already holds, and a
// second copy kept between picks would be a second truth to keep in step.
type Facts struct {
	// EnemySeen is true when an opposed member is in this creature's sight.
	EnemySeen bool

	// EnemyRemembered is true when NONE is in sight and one is held from an
	// earlier sighting. The two are exclusive by construction, which is what
	// makes `enemy: remembered` mean "I have lost sight of them" rather than
	// "I have seen them at some point".
	EnemyRemembered bool

	// Deeds is every deed this creature holds against itself.
	Deeds []HeldDeed

	// Now is the clock's high-water when this pick was made — what a deed's
	// At is subtracted from to age it.
	Now uint64
}

// holds reports whether a condition is true of these facts. A nil When is a
// standing order and always holds (design §2: "an entry with no `when` is
// always eligible").
func (w *When) holds(f Facts) bool {
	if w == nil {
		return true
	}
	switch w.Enemy {
	case EnemySeen:
		return f.EnemySeen
	case EnemyRemembered:
		return f.EnemyRemembered
	case EnemyNone:
		return !f.EnemySeen && !f.EnemyRemembered
	}

	verb := DeedVerbFor(w.Deed)
	for _, d := range f.Deeds {
		if verb == "" || d.Kind != verb {
			continue
		}
		// A DEED STAMPED AHEAD OF NOW IS NOT FRESH, IT IS WRONG. Unsigned
		// subtraction the other way round would wrap to an enormous age and
		// read as "long ago", so a corrupt stamp would silently switch a
		// condition off instead of being visible. Skipped rather than
		// erroring: a pick is not the place to discover a bad blob, and the
		// loader refuses one.
		if d.At > f.Now {
			continue
		}
		if f.Now-d.At <= uint64(w.Within) {
			return true
		}
	}

	return false
}

// Candidate is one eligible entry as the die saw it: its authored weight, the
// temperament's factor, and their product.
//
// EVERY NUMBER THE ROLL WAS MADE OF (design §6, R7). A reader adds Loaded
// across the candidates and gets [Pick.Of]; the face lands in exactly one of
// them. "A table nobody can replay is a table nobody can trust."
type Candidate struct {
	// Entry is this candidate's index in the key's authored entry list, so a
	// reader can point at the line in the file.
	Entry int

	// Weight is the author's own number.
	Weight int

	// Percent is the temperament's multiplier for this entry's word — 100
	// for a soldier, for a fact and for a bare line ([Temper.factor]).
	Percent int

	// Loaded is Weight × Percent: this candidate's share of the die.
	Loaded int
}

// Pick is one roll on one key: who was eligible, what the die said, and which
// entry fired.
//
// AN ENTRY OF -1 AND NO CANDIDATES IS A HOLD. That is the only shape with no
// roll in it: a `time` table with nothing eligible means the creature stands
// there, and the beat says so rather than saying nothing (design's "fail
// closed loudly").
type Pick struct {
	// Key is the trigger this was rolled for.
	Key AnswerKey

	// Candidates is every eligible entry, in authored order.
	Candidates []Candidate

	// Roll is the face, 1..Of, or 0 when there was nothing to roll.
	Roll int

	// Of is the die's size: the sum of every candidate's Loaded.
	Of int

	// Entry is the index of the entry that fired in the key's authored
	// list, or -1 when nothing was eligible.
	Entry int

	// Answer is the entry that fired — the zero-valued [Answer] with Hold
	// set when nothing was eligible under [AnswerTime].
	Answer Answer

	// Temper is the word the creature's temperament goes by, or empty for a
	// soldier. Carried so the beat can say WHY the shares are what they are.
	Temper string
}

// holdPick is the pick a `time` key with nothing eligible produces: the
// creature stands there, and the beat records that it was asked.
func holdPick(key AnswerKey, temper string) *Pick {
	return &Pick{Key: key, Entry: -1, Answer: Answer{Weight: 1, Hold: true}, Temper: temper}
}

// pick is THE ONE EVALUATOR: eligibility, the loaded die, the roll, and the
// entry that fired.
//
// ONE FUNCTION FOR BOTH KINDS OF TRIGGER (design §2). A social verb's answer
// and a creature's turn differ in WHEN they are rolled and in what the caller
// does with the word — not in how the die is built — and two evaluators would
// be two answers to "what does a weight mean".
//
// THE DIE IS THE CREATURE'S (R7). The roller is the world's shared dice and
// the creature is who the die belongs to; [Encounter.appendAnsweredBeat]
// names it, which is the only channel [dice.Roller] leaves for saying whose a
// roll is.
//
// Returns (nil, nil) for a key with nothing eligible on a SOCIAL trigger —
// today's silence, which is distinguishable from an entry that fired and did
// nothing: one writes no beat at all. [AnswerTime] answers a hold instead,
// because a creature that was given time and did nothing still had its turn.
func pick(
	ctx context.Context, key AnswerKey, table Table, temper Temper, facts Facts, roller dice.Roller,
) (*Pick, error) {
	entries := table[key]

	candidates := make([]Candidate, 0, len(entries))
	of := 0
	for i, entry := range entries {
		if !entry.When.holds(facts) {
			continue
		}
		percent := temper.factor(entry)
		loaded := entry.Weight * percent
		candidates = append(candidates, Candidate{
			Entry: i, Weight: entry.Weight, Percent: percent, Loaded: loaded,
		})
		of += loaded
	}

	if len(candidates) == 0 {
		if key == AnswerTime {
			return holdPick(key, temper.Word), nil
		}

		return nil, nil
	}
	if of < 1 {
		// Every eligible entry loaded to nothing: an author's weights are
		// validated above 0, so this is a temperament whose every factor is
		// zero. A creature that can do nothing at all still had its time, so
		// `time` holds; a social table that cannot be rolled is the refusal
		// it always was rather than a silently skipped answer.
		if key == AnswerTime {
			return holdPick(key, temper.Word), nil
		}

		return nil, fmt.Errorf("pick: %q weighs %d: %w", key, of, ErrBadAnswer)
	}

	if roller == nil {
		return nil, fmt.Errorf("pick: %q: %w", key, ErrNoRoller)
	}
	roll, err := roller.Roll(ctx, of)
	if err != nil {
		return nil, fmt.Errorf("pick: %q: %w", key, err)
	}

	chosen := candidates[len(candidates)-1]
	acc := 0
	for _, c := range candidates {
		acc += c.Loaded
		if roll <= acc {
			chosen = c
			break
		}
	}

	return &Pick{
		Key:        key,
		Candidates: candidates,
		Roll:       roll,
		Of:         of,
		Entry:      chosen.Entry,
		Answer:     entries[chosen.Entry],
		Temper:     temper.Word,
	}, nil
}

// dealTemper picks one word out of a faction's authored mix, through the
// world's own dice, with the FACTION as the die's entity (design §3).
//
// AT JOIN, ONCE, AND THE RESULT IS THE MEMBER'S. "Four goblins, one table,
// four behaviours" is the whole point, and it only works if the deal happens
// when the goblin enters the run rather than on every roll it makes — a
// temperament re-dealt per pick would be noise, not a personality.
//
// Returns the resolved Temper (Word and Profile filled, Mix dropped), the
// face and the die's size, so the caller can put the whole arithmetic on the
// `tempered` beat.
func dealTemper(ctx context.Context, temper Temper, roller dice.Roller) (Temper, int, int, error) {
	words := make([]string, 0, len(temper.Mix))
	of := 0
	for word, share := range temper.Mix {
		if share < 1 {
			return Temper{}, 0, 0, fmt.Errorf(
				"temper %q has a share of %d, which can never be dealt: %w", word, share, ErrBadTemper)
		}
		if _, ok := temper.Profiles[word]; !ok {
			return Temper{}, 0, 0, fmt.Errorf(
				"temper %q is in the mix and nothing says what it means: %w", word, ErrBadTemper)
		}
		words = append(words, word)
		of += share
	}
	// C8: the same seeded roller must deal the same word every run, so the
	// mix is walked in sorted order rather than the map's own.
	sort.Strings(words)

	if roller == nil {
		return Temper{}, 0, 0, fmt.Errorf("temper: %w", ErrNoRoller)
	}
	roll, err := roller.Roll(ctx, of)
	if err != nil {
		return Temper{}, 0, 0, fmt.Errorf("temper: %w", err)
	}

	dealt := words[len(words)-1]
	acc := 0
	for _, word := range words {
		acc += temper.Mix[word]
		if roll <= acc {
			dealt = word
			break
		}
	}

	return Temper{Word: dealt, Profile: temper.Profiles[dealt]}, roll, of, nil
}

// TemperWords is the three temperaments this build multiplies by, sealed
// HERE as well as in the rulebook that ships their numbers.
//
// TWO LISTS, DELIBERATELY, AND A TEST IN THE SESSION MODULE PINS THAT THEY
// AGREE. The profiles are content and live in rulebooks/dnd5e; this module
// cannot import the rulebook (C1) and still has to refuse `temper: brave` on
// the author's form rather than at the table. So the words are sealed twice
// and the module that imports both is where the two lists are checked against
// each other.
var TemperWords = []string{"soldier", "coward", "aggressive"}

// ValidTemperWord reports whether a word is one of [TemperWords]. Exported
// because the authoring dialect refuses every other word by name.
func ValidTemperWord(word string) bool {
	for _, known := range TemperWords {
		if word == known {
			return true
		}
	}

	return false
}
