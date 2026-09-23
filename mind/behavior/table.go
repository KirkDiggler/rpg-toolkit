// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// table.go is THE CREATURE'S TABLE (rpg-project#465,
// ideas/creature-table/design.md) — the mind's POLICY primitive, and this
// module's half of it.
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
// verb that settled, and [KeyTime]'s pick is one turn's worth of doing —
// and a caller that has to tell them apart should be switching on a named
// type rather than comparing spellings.
type AnswerKey string

// Die is the smallest thing this module needs to make a pick: one roll of a
// die of a stated size.
//
// THE SMALLEST INTERFACE, NOT THE RULEBOOK'S. A rulebook's shared dice can do
// more than this — roll several, trace a calculation, name whose die it is —
// and none of that is this module's business. Declaring only what it uses is
// what lets a caller pass its own roller unchanged and a test pass three
// lines.
//
// THE DIE'S ENTITY IS THE CALLER'S TO SET. Every roll here belongs to
// somebody — a pick to the creature, a deal to the faction whose spread it is
// — and that is a fact about the game, recorded where the caller narrates it.
// This module rolls; it does not decide whose roll it was.
type Die interface {
	// Roll returns a number from 1 to sides.
	Roll(ctx context.Context, sides int) (int, error)
}

// The four deed verbs a [When] condition can ask about — what the deeds
// channel this module owns files them under.
//
// THE AUTHOR'S WORDS ARE PAST TENSE AND THESE ARE NOT, which is the whole of
// [DeedVerbFor]: a condition is written from the CREATURE's side ("I was
// attacked") and a deed is recorded from the WITNESS's ("somebody attacked").
// A rulebook that lands deeds under other verbs is welcome to; there is
// simply no `when` word for them yet, and adding one is adding it here.
const (
	// VerbAttack is a swing or a shot resolving against somebody.
	VerbAttack = "attack"

	// VerbIntimidate is a threat landing.
	VerbIntimidate = "intimidate"

	// VerbPersuade is an appeal landing.
	VerbPersuade = "persuade"

	// VerbFled is a creature's own memory of having been made to run. Its
	// actor is WHO CAUSED IT, the rule every verb above keeps.
	VerbFled = "fled"
)

// The two ways a table can be unrollable, both refused rather than guessed.
var (
	// ErrBadTable reports a table this module could not roll: every eligible
	// entry loaded to nothing, which is a temperament whose every factor is
	// zero. A caller validates the authored shape at its own door; this is
	// the evaluator refusing to roll a die of size zero.
	ErrBadTable = errors.New("behavior: table cannot be rolled")

	// ErrNoDie reports a pick or a deal with nothing to roll it on.
	// SUPPLIED, NEVER DEFAULTED: a silent default puts untestable randomness
	// into a result that looks fine, and every pick this module makes is
	// shown to the table.
	ErrNoDie = errors.New("behavior: no die supplied")

	// ErrBadMix reports a temperament mix that cannot be dealt: a share below
	// 1, which can never come up, or a word in the mix that no profile says
	// the meaning of.
	ErrBadMix = errors.New("behavior: temperament mix cannot be dealt")
)

// KeyTime is the one trigger this module names: the creature has time.
//
// A CREATURE HAVING TIME IS NOT A RULEBOOK'S IDEA. Every other key a table can
// carry is an event in some rulebook's vocabulary — a threat landing, an
// appeal failing — and those are the rulebook's to name and to seal. This one
// is the span of doing that every game has, and the evaluator has to know it
// because a creature given time and eligible for nothing HELD its turn, which
// is a different answer from the silence an unmatched event produces.
const KeyTime AnswerKey = "time"

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

// EnemyWord is a `when: { enemy: … }` condition's value: how close the
// nearest thing this creature is opposed to has got.
//
// FOUR BANDS, AND THEY ARE EXCLUSIVE BY DEFINITION (rpg-project#465, ruled on
// a session-builder finding). Exactly one of them holds for any creature at
// any moment, so an author writes one entry per band and knows exactly one is
// on the table — the same property `seen` and `remembered` already had, now
// carried through the whole ladder.
//
// # Why `reach` exists
//
// The shipped default could not close. `enemy: seen` fired `attack: enemy`,
// and an attack on somebody out of reach is a pass — so a thug that could see
// the party stood still and swung at nothing, forever. Sight and reach are
// two different questions and the table had a word for only one of them.
// With the band, the default reads: swing when they are in reach, WALK when
// they are merely in sight.
type EnemyWord string

const (
	// EnemyReach holds when an opposed member is within this creature's own
	// reach — the same reach an [Attack] intent is tested against
	// ([SeenMember.InReach]), so a table that says `attack` under this band
	// can always land it.
	EnemyReach EnemyWord = "reach"

	// EnemySeen holds when an opposed member is in this creature's sight AND
	// NONE IS IN REACH. The band is the gap between seeing and touching,
	// which is what `toward` is for.
	EnemySeen EnemyWord = "seen"

	// EnemyRemembered holds when none is in sight but one is held from an
	// earlier sighting.
	EnemyRemembered EnemyWord = "remembered"

	// EnemyNone holds when none of the three does.
	EnemyNone EnemyWord = "none"
)

// EnemyWords is every value `enemy:` takes, nearest band first. Exported for
// the dialect's refusal, [AnswerKeys]'s reason.
var EnemyWords = []EnemyWord{EnemyReach, EnemySeen, EnemyRemembered, EnemyNone}

// WhenDeeds is every word a `when: { <deed>: { within: N } }` condition may
// name. Exported for the dialect's refusal, [AnswerKeys]'s reason.
//
// THE AUTHOR'S WORDS ARE PAST TENSE, AND THE STORE'S ARE NOT. A condition is
// written from the CREATURE's side — "I was attacked" — while a deed is
// recorded from the WITNESS's — "somebody attacked". `attacked` and
// [VerbAttack] are the same event named from the two ends of it, and
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
		return VerbAttack
	case "intimidated":
		return VerbIntimidate
	case "persuaded":
		return VerbPersuade
	case "fled":
		return VerbFled
	default:
		return ""
	}
}

// DeedScope says whose deed a `when: { <word>: … }` condition is about. An
// empty scope is the reading this type was added beside — a deed AGAINST THE
// CREATURE ITSELF — so a document that authors no scope means exactly what it
// always meant and behaves identically.
//
// THE TWO NEW READINGS ARE DIFFERENT AXES, and neither is a new condition:
//
//   - [ScopeAlly] reads a deed against SOMEBODY ON THIS CREATURE'S SIDE. The
//     deed is already in hand — deeds land on every witness, not only on the
//     target — so this is a second reading of facts the creature holds, not a
//     second fact source.
//   - [ScopeActor] reads a deed the creature ITSELF DID. This is what a pause
//     is made of ("after I strike, stand still"), so the grammar needs no
//     separate pause concept.
type DeedScope string

const (
	// ScopeSelf is a deed against the creature itself — the default, and the
	// only reading this module knew before scopes existed. Spelled as the
	// empty string so an unauthored scope means this, and a document written
	// before scopes existed keeps its exact meaning.
	ScopeSelf DeedScope = ""

	// ScopeAlly is a deed against one of this creature's own side. WHICH
	// members those are is the caller's answer (the stance graph, in the
	// rulebook), asked when the condition is read — so a table keeps working
	// when a disposition changes, and a document never names a faction.
	ScopeAlly DeedScope = "ally"

	// ScopeActor is a deed this creature did itself, which is the fact a
	// pause reads.
	ScopeActor DeedScope = "actor"
)

// DeedScopes is every scope a condition may name, in the order a refusal
// lists them: the default first, then the two readings beside it.
var DeedScopes = []DeedScope{ScopeSelf, ScopeAlly, ScopeActor}

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

	// Deed is the word for what this creature must hold — one of [WhenDeeds],
	// in the author's own past tense — or empty when this condition names an
	// enemy instead. [DeedVerbFor] is what turns it into the verb the deeds
	// channel files under.
	//
	// WHOSE DEED IT IS, IS [When.Scope]'s ANSWER, not this word's: the word
	// says WHAT happened and the scope says who it happened to or by.
	Deed string

	// Scope says WHOSE deed the Deed word is about, when Deed is set: the
	// empty string is "against me" (the reading this field was added
	// beside), [ScopeAlly] is "against one of mine", and [ScopeActor] is
	// "done by me". It is a refinement of the deed form, not a second
	// condition, so a When still names exactly one thing.
	//
	// ALLY IS A RELATIONSHIP, NOT A FACTION. Which members are "mine" is the
	// caller's answer — the stance graph's, in the rulebook — and it is asked
	// when the condition is read, never frozen into the document. That is
	// what makes a table keep working when a disposition changes.
	//
	// ACTOR IS THE PAUSE. "After I strike, stand still for two rounds" is
	// `{ attacked: { within: 2, as: actor } }` — a deed the creature DID,
	// recently — so a pause needs no separate concept in this grammar.
	//
	// OMITTED WHEN EMPTY, unlike its siblings here, and that is the design
	// claim made mechanical: the default reading IS the empty string, so a
	// document authored before scopes existed must compile to the same
	// picture it always did. A serializer that wrote `"Scope": ""` into every
	// condition would move every committed golden for a field whose meaning
	// is "nothing new".
	Scope DeedScope `json:",omitempty"`

	// Within is how many rounds ago the deed may have landed and still
	// count, in THE CALLER'S OWN UNIT — whatever [Facts.Now] and a deed's
	// At are counted in, which this module never interprets. AT LEAST
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
// EXACTLY ONE OF THE TWO. A Selector with a Word names somebody the CALLER
// resolves against what this creature sees and remembers; one with At names a
// cell the author wrote. Which words a caller resolves, and which of them it
// will accept beside which table word, is the caller's rule — this module
// carries the selector and resolves nothing.
//
// A CELL, AND ONLY BECAUSE IT IS CARRIED. tools/spatial is as
// rulebook-neutral as this module is, and a position is the one shape every
// board in this repo already speaks; nothing here reads it.
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
// THE PROFILE IS THE CALLER'S TO FILL, AND SO IS THE VOCABULARY. This module
// accepts ANY word that arrives with a profile: it multiplies, it deals, and
// it never asks what "coward" means or whether that is a word. Sealing the
// list is the rulebook's — it is the one that ships the numbers — and
// refusing an unsealed word on an author's form is the dialect's. A second
// sealed list here would be a second thing to keep in step with content.
type Temper struct {
	// Word is the temperament this member has, or empty for none — which is
	// a soldier, and stores nothing.
	Word string `json:"word,omitempty"`

	// Profile is what Word means, filled by the caller from the rulebook.
	Profile TemperProfile `json:"profile,omitzero"`

	// Mix is a faction's authored spread, word → positive share
	// (`temper: { coward: 1, soldier: 2, aggressive: 1 }`). Non-nil only on
	// the way IN: the caller deals one word from it at the door, through the
	// Roller, with the FACTION as the die's entity, and stores the dealt
	// Word and Profile. A member's stored Temper never carries a Mix.
	Mix map[string]int `json:"mix,omitempty"`

	// Profiles is what every word in Mix means, filled by the caller from
	// the rulebook for the same reason Profile is. A Mix naming a word this
	// map does not is refused at the door ([ErrBadMix]) rather than
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

// HeldDeed is one deed this creature HOLDS, projected out of its own holdings
// for the table to read: what happened, who did it, and when.
//
// WHICH DEEDS DEPENDS ON THE READING, and the three are the same shape because
// they are the same testimony read three ways (rpg-toolkit#1883): against the
// creature itself ([Facts.Deeds]), against its own side ([Facts.AllyDeeds]),
// or done by the creature ([Facts.OwnDeeds]). WHO it was done to is what
// separates them, which is why the shape does not carry it — the projection
// already answered that when it chose which list to append to.
//
// THE CREATURE'S OWN TESTIMONY, never anybody's live state. A deed is
// stamped when it happened and never restamped, so [Facts.Now] minus At is
// how old it is in rounds.
type HeldDeed struct {
	// Kind is the deed's verb — [VerbAttack], [VerbIntimidate],
	// [VerbPersuade], [VerbFled].
	Kind string

	// Actor is who did it. What `actor` selects.
	Actor core.EntityID

	// At is the clock reading the deed was landed at.
	At uint64
}

// Facts is everything a [When] condition reads: what this creature can see,
// what it remembers, what has been done to it, what has been done to its own
// side, and what it has itself done.
//
// A PROJECTION, BUILT PER PICK. There is no cache: a creature's facts are its
// holdings and the clock, both of which the caller already holds, and a
// second copy kept between picks would be a second truth to keep in step.
//
// THE THREE ENEMY FLAGS ARE EXCLUSIVE, and the projections enforce it rather
// than every reader re-deriving it: in reach clears seen, and seen clears
// remembered. That is what makes `enemy: remembered` mean "I have lost sight
// of them" rather than "I have seen them at some point", and `enemy: seen`
// mean "I can see them and cannot touch them" rather than "I can see them".
type Facts struct {
	// EnemyInReach is true when an opposed member is within this creature's
	// own reach — the nearest band, and the one that makes `attack` able to
	// land.
	EnemyInReach bool

	// EnemySeen is true when an opposed member is in this creature's sight
	// AND NONE IS IN REACH.
	EnemySeen bool

	// EnemyRemembered is true when NONE is in sight and one is held from an
	// earlier sighting.
	EnemyRemembered bool

	// CanAttack and CanMove are what the creature can still AFFORD this turn
	// — one attack left, one cell of movement left — and they are part of
	// ELIGIBILITY, exactly as a [When] is.
	//
	// A PICK THAT CANNOT ACT IS NOISE ON THE LOG (ruled on an api-builder
	// finding). A caller that asks again after a swing would otherwise be
	// handed `attack` a second time, publish a second identical account of a
	// roll, and watch nothing happen. An entry the creature cannot pay for is
	// not on the table, which is the same sentence an unmet condition earns
	// and for the same reason: the candidate list has to be the honest
	// account of what the creature could have done.
	//
	// THE ZERO VALUE IS "CANNOT", deliberately. A caller that has not thought
	// about a budget gets a creature that can only hold, which is visible on
	// the first beat; the inverse default would have let a forgotten field
	// quietly authorise a swing.
	CanAttack bool
	CanMove   bool

	// Deeds is every deed this creature holds against itself — the [ScopeSelf]
	// reading, and the only one this module knew before scopes existed.
	Deeds []HeldDeed

	// AllyDeeds is every deed this creature holds against ONE OF ITS OWN SIDE
	// — the [ScopeAlly] reading. The caller decides who "its own side" is
	// (the rulebook asks its stance graph), so this is the same deeds read a
	// second way rather than a second fact source: deeds land on every
	// witness, so a creature that watched an ally fall already holds it.
	//
	// EMPTY IS "I SAW NOTHING HAPPEN TO MY SIDE", and an unset field means
	// the same thing, so a caller that has not thought about allies hands a
	// table that authors no `on: ally` condition exactly what it had.
	AllyDeeds []HeldDeed

	// OwnDeeds is every deed this creature DID — the [ScopeActor] reading,
	// and what a pause is made of. The verb is the creature's own action, so
	// the actor of each is the creature itself.
	OwnDeeds []HeldDeed

	// Now is the clock's high-water when this pick was made — what a deed's
	// At is subtracted from to age it.
	Now uint64
}

// affords reports whether this creature can still pay for what an entry does.
//
// ONLY THE THREE BUDGETED WORDS ARE GATED. A `fact`, a `flee`, a bare line and
// a `hold` cost nothing a turn can run out of — and `hold` in particular must
// stay eligible whatever the budget says, because it is the word that lets a
// creature with nothing left to spend still have had its turn.
func (f Facts) affords(a Answer) bool {
	switch {
	case a.Attack != nil:
		return f.CanAttack
	case a.Toward != nil, a.Away != nil:
		return f.CanMove
	default:
		return true
	}
}

// deedsOf is the reading this condition's scope asks for: the deeds against
// the creature ([ScopeSelf], the default), the deeds against its side
// ([ScopeAlly]), or the deeds it did ([ScopeActor]).
//
// ONE PLACE THAT DECIDES, so the three readings cannot drift apart.
//
// AN UNKNOWN SCOPE MATCHES NOTHING, which is this module's policy for every
// other unknown word: [DeedVerbFor] answers empty so a typo'd deed matches no
// held deed, and `holds` refuses a span below 1 rather than reading a window
// that does not exist. Returning [Facts.Deeds] here instead would FAIL OPEN —
// a condition that meant `allie` would quietly read as "against me" and fire
// on the wrong facts, which is worse than a row that is visibly dead. The row
// is then absent from the candidates, the honest account of what the creature
// could have done.
//
// THE DOOR STILL REFUSES BY NAME. The dialect refuses an unknown scope at
// decode (rpg-toolkit#1885), so this arm is for a struct built in Go — a test,
// a future rulebook, a consumer pinned one tag behind — which never passes
// through that door.
func (w *When) deedsOf(f Facts) []HeldDeed {
	switch w.Scope {
	case ScopeSelf:
		return f.Deeds
	case ScopeAlly:
		return f.AllyDeeds
	case ScopeActor:
		return f.OwnDeeds
	default:
		return nil
	}
}

// holds reports whether a condition is true of these facts. A nil When is a
// standing order and always holds (design §2: "an entry with no `when` is
// always eligible").
func (w *When) holds(f Facts) bool {
	if w == nil {
		return true
	}
	switch w.Enemy {
	case EnemyReach:
		return f.EnemyInReach
	case EnemySeen:
		return f.EnemySeen
	case EnemyRemembered:
		return f.EnemyRemembered
	case EnemyNone:
		// THE BANDS ARE EXCLUSIVE, so `none` is simply none of them. The
		// projections that build [Facts] are what keep that true — they clear
		// the farther flags once a nearer one holds — and this reads the
		// result rather than re-deciding it.
		return !f.EnemyInReach && !f.EnemySeen && !f.EnemyRemembered
	}

	// A SPAN IS COUNTED FROM 1, so a span below it is not a span and holds
	// nothing. The caller refuses one at its own door; this is the evaluator
	// refusing to read a window that does not exist rather than quietly
	// treating it as "the exact moment it landed".
	if w.Within < 1 {
		return false
	}

	verb := DeedVerbFor(w.Deed)
	for _, d := range w.deedsOf(f) {
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

// PickOutput is one roll on one key: who was eligible, what the die said, and which
// entry fired.
//
// AN ENTRY OF -1 AND NO CANDIDATES IS A HOLD. That is the only shape with no
// roll in it: a `time` table with nothing eligible means the creature stands
// there, and the beat says so rather than saying nothing (design's "fail
// closed loudly").
type PickOutput struct {
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
	// set when nothing was eligible under [KeyTime].
	Answer Answer

	// Temper is the word the creature's temperament goes by, or empty for a
	// soldier. Carried so the beat can say WHY the shares are what they are.
	Temper string
}

// holdPick is the pick a `time` key with nothing eligible produces: the
// creature stands there, and the beat records that it was asked.
func holdPick(key AnswerKey, temper string) *PickOutput {
	return &PickOutput{Key: key, Entry: -1, Answer: Answer{Weight: 1, Hold: true}, Temper: temper}
}

// PickInput is one creature's whole situation, asked of its own table.
type PickInput struct {
	// Key is the trigger to roll: [KeyTime], or whatever the rulebook calls
	// the event that just happened to this creature.
	Key AnswerKey

	// Table is the creature's policy, ALREADY LAYERED by whoever owns the
	// layers ([Layer]). This module rolls the table it is handed.
	Table Table

	// Temper is what loads the die. The zero value is a soldier — every
	// factor 100 — so a caller with no temperament model passes nothing.
	Temper Temper

	// Facts is what the creature holds, projected by the caller against a
	// board this module does not have. See [Facts].
	Facts Facts

	// Die is what the roll is made on. REQUIRED once anything is eligible;
	// [ErrNoDie] otherwise.
	Die Die
}

// Pick is THE ONE EVALUATOR: eligibility, the loaded die, the roll, and the
// entry that fired.
//
// ONE FUNCTION FOR EVERY TRIGGER (design §2). An event's answer and a
// creature's turn differ in WHEN they are rolled and in what the caller does
// with the word — not in how the die is built — and two evaluators would be
// two answers to "what does a weight mean".
//
// THE DIE IS THE CREATURE'S, and saying so is the caller's: this module rolls
// and reports the arithmetic, and whoever narrates it names whose roll it was.
//
// A CONTEXT FIRST, AND ONLY FOR THE DIE. This module holds no cancellation of
// its own; [Die.Roll] takes one because a shared roller does, and there is
// nowhere else to put it that is not a context in a struct.
//
// Returns (nil, nil) for a key with nothing eligible on any trigger BUT
// [KeyTime] — silence, which is distinguishable from an entry that fired and
// did nothing: one writes no beat at all. [KeyTime] answers a hold instead,
// because a creature that was given time and did nothing still had its turn.
func Pick(ctx context.Context, in *PickInput) (*PickOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("pick: %w", ErrNilInput)
	}
	key, table, temper, facts, roller := in.Key, in.Table, in.Temper, in.Facts, in.Die
	entries := table[key]

	candidates := make([]Candidate, 0, len(entries))
	of := 0
	for i, entry := range entries {
		// TWO TESTS, AND THEY ARE THE SAME TEST. An entry is on the table
		// when its condition holds AND the creature can pay for it; either
		// way it is ABSENT rather than weighted zero, so the candidate list
		// stays the honest account of what could have happened.
		if !entry.When.holds(facts) || !facts.affords(entry) {
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
		if key == KeyTime {
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
		if key == KeyTime {
			return holdPick(key, temper.Word), nil
		}

		return nil, fmt.Errorf("pick: %q weighs %d: %w", key, of, ErrBadTable)
	}

	if roller == nil {
		return nil, fmt.Errorf("pick: %q: %w", key, ErrNoDie)
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

	return &PickOutput{
		Key:        key,
		Candidates: candidates,
		Roll:       roll,
		Of:         of,
		Entry:      chosen.Entry,
		Answer:     entries[chosen.Entry],
		Temper:     temper.Word,
	}, nil
}

// DealInput is a faction's authored spread, and the die to pick one out of it.
type DealInput struct {
	// Temper carries the Mix to deal from and the Profiles that say what each
	// word in it means. Its Word and Profile are ignored: what goes in is a
	// spread, what comes out is a creature.
	Temper Temper

	// Die is what the deal is made on. REQUIRED; [ErrNoDie] otherwise.
	Die Die
}

// DealOutput is the creature that came out of the spread, and the arithmetic
// that chose it.
type DealOutput struct {
	// Temper is the dealt word and the profile it means. The Mix is spent.
	Temper Temper

	// Roll is the face, 1..Of; Of is the sum of the authored shares. Both
	// travel so the caller can put the whole deal on a beat — the streamer
	// seeing which goblin came out the coward is the point of dealing at all.
	Roll int
	Of   int
}

// Deal picks one word out of an authored mix (design §3).
//
// AT THE DOOR, ONCE, AND THE RESULT IS THE CREATURE'S. "Four goblins, one
// table, four behaviours" only works if the deal happens when the goblin
// enters the run rather than on every roll it makes — a temperament re-dealt
// per pick would be noise, not a personality.
//
// THE DIE IS THE FACTION'S, and the caller is who says so: the spread belongs
// to the group, so the entity whose rule threw it is the side and not the
// creature that came out of it. This module rolls and reports; naming the
// thrower is the caller's, where it narrates.
func Deal(ctx context.Context, in *DealInput) (*DealOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("temper: %w", ErrNilInput)
	}
	temper, roller := in.Temper, in.Die

	return dealTemper(ctx, temper, roller)
}

// dealTemper is [Deal]'s body: one word out of an authored mix, through the
// caller's own die.
func dealTemper(ctx context.Context, temper Temper, roller Die) (*DealOutput, error) {
	words := make([]string, 0, len(temper.Mix))
	of := 0
	for word, share := range temper.Mix {
		if share < 1 {
			return nil, fmt.Errorf(
				"temper %q has a share of %d, which can never be dealt: %w", word, share, ErrBadMix)
		}
		if _, ok := temper.Profiles[word]; !ok {
			return nil, fmt.Errorf(
				"temper %q is in the mix and nothing says what it means: %w", word, ErrBadMix)
		}
		words = append(words, word)
		of += share
	}
	// C8: the same seeded roller must deal the same word every run, so the
	// mix is walked in sorted order rather than the map's own.
	sort.Strings(words)

	if roller == nil {
		return nil, fmt.Errorf("temper: %w", ErrNoDie)
	}
	roll, err := roller.Roll(ctx, of)
	if err != nil {
		return nil, fmt.Errorf("temper: %w", err)
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

	return &DealOutput{
		Temper: Temper{Word: dealt, Profile: temper.Profiles[dealt]},
		Roll:   roll,
		Of:     of,
	}, nil
}

// ErrNilInput reports a verb called with no input — the family's own first
// refusal, guarded before anything else.
var ErrNilInput = errors.New("behavior: nil input")
