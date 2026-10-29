// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
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

// The five deed verbs a [When] condition can ask about — what the deeds
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

	// VerbFell is a member going down, as its witnesses hold it
	// (rpg-project#716). Its actor is WHO CAUSED IT, the rule every verb
	// above keeps, and its target is WHO FELL: the one deed whose condition
	// may name its target, with `who` ([When.Who]). What going down means is
	// the rulebook's; this module knows only that it is a deed a witness can
	// hold. Spelled by the deeds channel ([deed.VerbFell]), which files a
	// fall by who fell as well as who caused it ([deed.FiledUnder]).
	VerbFell = deed.VerbFell
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

	// ErrBadWhen reports a `when` entry this table cannot read: one naming
	// more than one condition or none, a `heard` with no kind or a span
	// below 1, or a deed's `within` or scope on a condition that is not a
	// deed, or a [Name] that is both a literal and a pattern or whose
	// pattern does not compile. Returned by [When.Validate] and
	// [Answer.Validate], wrapped in a sentence written for whoever filled
	// the form.
	ErrBadWhen = errors.New("behavior: when cannot be read")

	// ErrBadSearch reports a `search` this table cannot read: a distance
	// below 1, a search that walks nowhere, or a heading that is not one of
	// [SearchHeadings]. Returned by [Answer.Validate],
	// wrapped in a sentence for the form.
	ErrBadSearch = errors.New("behavior: search cannot be read")

	// ErrBadSelector reports a selector this table cannot read: one naming
	// no target or more than one, a route on a word that does not walk,
	// `sound` on `attack`, or `sound` on a row that names no `heard`
	// condition. Returned by [Answer.Validate], wrapped in a sentence for
	// the form.
	ErrBadSelector = errors.New("behavior: selector cannot be read")

	// ErrBadGroup reports a group row this table cannot read: one carrying
	// a word, a search, a line or a fact of its own, one holding no rows,
	// one counting `as` a word no temperament multiplies, or one nested past
	// [MaxGroupDepth]. Returned by [Answer.Validate], wrapped in a sentence
	// for the form.
	ErrBadGroup = errors.New("behavior: group cannot be read")
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
// FIVE BANDS, AND THEY ARE EXCLUSIVE BY DEFINITION (rpg-project#465, ruled on
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
//
// # The second ladder: what a move buys (rpg-project#671, R18)
//
// `walk`, `dash` and `far` answer a different question from the five bands:
// not how the creature holds its foe, but what this turn's movement buys
// against it. They are exclusive among themselves and with `reach`, and they
// sit BESIDE `seen` and `remembered` rather than inside the ladder: a foe in
// sight that a walk reaches is both `seen` and `walk`. With no foe held
// placed none of the three holds. The host computes them from both
// creatures' speed and reach; this module never holds a distance, a speed or
// a number of cells (Kirk, 2026-10-26: "what it means to the NPC").
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
	// earlier sighting that still points somewhere worth walking to.
	EnemyRemembered EnemyWord = "remembered"

	// EnemyLost holds when none is in sight or remembered, but one is held
	// as LOST: the creature went to where it last saw them and found nobody
	// (rpg-project#668 R3). What turns a memory lost is the caller's rule;
	// this module reads the flag. `search` is the word that walks on from
	// here.
	EnemyLost EnemyWord = "lost"

	// EnemyNone holds when none of the four does.
	EnemyNone EnemyWord = "none"

	// EnemyWalk holds when no opposed member is in reach and a WALK this
	// turn puts the creature in reach of the one `enemy` names, so it can
	// still swing ([Facts.EnemyWalk]).
	EnemyWalk EnemyWord = "walk"

	// EnemyDash holds when a walk this turn does not reach the one `enemy`
	// names and a DASH does ([Facts.EnemyDash]). The dasher's row: `when: {
	// enemy: dash }` and `dash: enemy`.
	EnemyDash EnemyWord = "dash"

	// EnemyFar holds when not even a dash this turn reaches the one `enemy`
	// names ([Facts.EnemyFar]).
	EnemyFar EnemyWord = "far"
)

// EnemyWords is every value `enemy:` takes: the five bands nearest first,
// then the move words nearest first. Exported for the dialect's refusal,
// [AnswerKeys]'s reason.
var EnemyWords = []EnemyWord{
	EnemyReach, EnemySeen, EnemyRemembered, EnemyLost, EnemyNone,
	EnemyWalk, EnemyDash, EnemyFar,
}

// ThreatWord is a `when: { threat: … }` condition's value: who ELSE could
// reach the cell this row's move would land on, and swing, on their next
// turn (rpg-project#671, R18; Kirk, 2026-10-26: "I don't mind dashing so long
// as that doesn't put me in range after the other gets their move step").
//
// JUDGED PER ROW, AT THE ROW'S LANDING CELL. A `dash: enemy` row and a
// `toward: enemy` row land on different cells, so they can read different
// threats in one pick; a row that does not move is judged where the creature
// stands. The row's own target does not count: dashing into reach of
// somebody means they can hit back, and that is the point. `threat` asks who
// else can. The host computes it ([Facts.Threat]); this module holds no
// cell, no speed and no reach.
//
// THE CAREFUL DASHER is `when: [{ enemy: dash }, { threat: none }]` with
// `dash: enemy`; the reckless one drops the threat condition.
type ThreatWord string

const (
	// ThreatNone holds when no foe but the row's target could reach the
	// landing cell with a walk and a swing, nor with a dash.
	ThreatNone ThreatWord = "none"

	// ThreatWalk holds when at least one foe but the row's target could reach
	// the landing cell with a walk, and still swing.
	ThreatWalk ThreatWord = "walk"

	// ThreatDash holds when no foe but the row's target could reach the
	// landing cell with a walk, and at least one could by dashing.
	ThreatDash ThreatWord = "dash"
)

// ThreatWords is every value `threat:` takes, as the design names them.
// Exported for the dialect's refusal, [AnswerKeys]'s reason.
var ThreatWords = []ThreatWord{ThreatNone, ThreatWalk, ThreatDash}

// sealed reports whether t is one of [ThreatWords].
func (t ThreatWord) sealed() bool {
	return slices.Contains(ThreatWords, t)
}

// WhenDeeds is every word a `when: { <deed>: { within: N } }` condition may
// name. Exported for the dialect's refusal, [AnswerKeys]'s reason.
//
// THE AUTHOR'S WORDS ARE PAST TENSE, AND THE STORE'S ARE NOT. A condition is
// written from the CREATURE's side — "I was attacked" — while a deed is
// recorded from the WITNESS's — "somebody attacked". `attacked` and
// [VerbAttack] are the same event named from the two ends of it, and
// [DeedVerbFor] is the one place the two vocabularies meet.
var WhenDeeds = []string{"attacked", "intimidated", "persuaded", "fled", "fell"}

// WhoDeeds is every deed word whose condition may name WHO it was done to,
// with `who` ([When.Who]). Exported for the dialect's refusal, [WhenDeeds]'s
// reason.
//
// ONE WORD, AND IT IS `fell`: a fall's point is who fell, and a row that
// waits on the king's death and not the cook's has to say so. The other
// deeds are read from the creature's side (against me, my side, or by me),
// and no use case has asked which one of my side was struck. Widening this
// list is adding a word here, the way [WhenDeeds] grows.
var WhoDeeds = []string{"fell"}

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
	case "fell":
		return VerbFell
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
// EXACTLY ONE CONDITION PER WHEN: one of Enemy, Deed, Heard, Fact, Threat
// or Not.
// An entry that needs two writes two Whens in its list ([Answer.When], R8 of
// rpg-project#569): the list is the `and`, every entry in it must hold, and
// `not` wraps a negative. Two conditions in one When would be a second way to
// spell the same `and`, so it is refused rather than read.
//
// THE SHAPE RULE IS REFUSED HERE, AND NOWHERE ELSE OWNS IT. [When.Validate]
// and [Answer.Validate] are the one statement of "one condition per When"
// and "one target per Selector"; the dialect that decodes an author's file
// calls them at its door and adds what only it knows (which words are legal
// under which key, which deed and enemy words this build reads). [Pick]
// never validates: a struct built in Go that names two conditions reaches
// [When.holds] unrefused, and `holds` reads the first in the order it checks
// rather than guessing at an `and`.
type When struct {
	// Enemy reads the enemy booleans on [Facts], or is empty when this
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

	// Who narrows a deed condition to the deeds done to ONE member: the
	// deed's target ([HeldDeed.Target]) must be this id. Legal only on a
	// word in [WhoDeeds], which today is `fell`, so `{ fell: { who: king,
	// within: 3, on: ally } }` reads "I saw the king fall, recently" and not
	// the cook. Empty reads every deed of the word, as before `who` existed.
	//
	// THE CREATURE'S OWN TESTIMONY, never the world's: a witness that could
	// not see who fell holds a fall that names nobody, and a `who` row cannot
	// read it. Which ids are legal is the dialect's to seal.
	//
	// OMITTED WHEN EMPTY, Scope's rule for Scope's reason.
	Who core.EntityID `json:",omitempty"`

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

	// Heard is `heard: { kind, within }`: a sound of that kind this creature
	// holds, no older than the span. Nil when this condition names something
	// else. See [HeardWhen].
	//
	// OMITTED WHEN EMPTY, as Scope is and for Scope's reason: a document
	// that names none of the conditions added beside the first two must
	// marshal exactly as it did before they existed.
	Heard *HeardWhen `json:",omitempty"`

	// Fact is `fact: <id>`: this creature holds a world fact with that id,
	// or one the pattern matches ([Facts.Known], [Name]). Zero when this
	// condition names something else.
	//
	// OMITTED WHEN ZERO, which is what `omitempty` did for the plain string
	// it replaced: a literal marshals as that string, so no committed golden
	// moves.
	Fact Name `json:",omitzero"`

	// Not is `not: { … }`: it holds when the one condition inside it does
	// not. It wraps exactly one When — any of them, another `not` included —
	// and never a list: a negative of an `and` is an `or` this grammar has
	// not paid for.
	Not *When `json:",omitempty"`

	// Threat is `threat: none | walk | dash`: who else could reach the cell
	// this row's move lands on ([ThreatWord]), read off [Facts.Threat] for
	// the row being judged. Empty when this condition names something else.
	// One of [ThreatWords], refused otherwise ([When.Validate]).
	//
	// OMITTED WHEN EMPTY, for Scope's reason: a condition that names no
	// threat marshals exactly as it did before the word existed.
	Threat ThreatWord `json:",omitempty"`
}

// HeardWhen is a `heard` condition: which kind of sound, and how recently.
//
// A KIND AND A SPAN, NEVER A MAKER (R4 of rpg-project#569). A sound carries
// no identity ([sound.Sound]), so the condition cannot ask who made it.
type HeardWhen struct {
	// Kind is the sound's kind, in the caller's sealed vocabulary, or a
	// pattern over those kinds ([Name]). This module matches it and never
	// interprets it.
	Kind Name

	// Within is how many units ago the sound may have been made and still
	// count, in the caller's own unit — the deed condition's span, with the
	// deed condition's rule: AT LEAST 1, so `within: 2` heard at 10 holds at
	// 10, 11 and 12.
	Within int
}

// conditions names every condition this When sets, in the order a refusal
// lists them, each as the word an author wrote for it.
func (w *When) conditions() []string {
	var out []string
	if w.Enemy != "" {
		out = append(out, "`enemy`")
	}
	if w.Deed != "" {
		out = append(out, "`"+w.Deed+"`")
	}
	if w.Heard != nil {
		out = append(out, "`heard`")
	}
	if !w.Fact.IsZero() {
		out = append(out, "`fact`")
	}
	if w.Not != nil {
		out = append(out, "`not`")
	}
	if w.Threat != "" {
		out = append(out, "`threat`")
	}

	return out
}

// Validate refuses a When that does not name exactly one condition, a
// `threat` word not in [ThreatWords], a `heard` with no kind or a span below 1, a deed's `within` or scope set on
// a condition that is not a deed, a `who` on anything but a word in
// [WhoDeeds], a [Name] that sets both forms or whose
// pattern does not compile, and the same faults inside a `not`. ErrBadWhen,
// wrapped in a sentence for the form.
//
// It does NOT refuse an enemy or deed word this build does not read, an
// unknown scope, or a deed span below 1: those are the dialect's, where the
// sealed lists are refused by name, and this module's evaluator already
// treats every one of them as a dead row rather than a guess.
func (w *When) Validate() error {
	named := w.conditions()
	switch len(named) {
	case 0:
		words := make([]string, 0, len(WhenDeeds)+4)
		words = append(words, "`enemy`")
		for _, d := range WhenDeeds {
			words = append(words, "`"+d+"`")
		}
		words = append(words, "`heard`", "`fact`", "`threat`")

		return fmt.Errorf("a `when` entry names no condition; give it one of %s or `not`: %w",
			strings.Join(words, ", "), ErrBadWhen)
	case 1:
	default:
		return fmt.Errorf("a `when` entry names %s; give each its own entry in the list, "+
			"and every entry must hold: %w", andList(named), ErrBadWhen)
	}

	switch {
	case w.Deed == "" && (w.Within != 0 || w.Scope != ScopeSelf):
		return fmt.Errorf("a `when` entry names %s with a deed's `within` or scope; "+
			"those belong only beside a deed: %w", named[0], ErrBadWhen)
	case w.Deed == "" && w.Who != "":
		return fmt.Errorf("a `when` entry names %s with `who`; `who` belongs only beside `fell`: %w",
			named[0], ErrBadWhen)
	case w.Who != "" && !slices.Contains(WhoDeeds, w.Deed):
		return fmt.Errorf("`who` names the one who fell, and only `fell` takes it; `%s` does not: %w",
			w.Deed, ErrBadWhen)
	case w.Heard != nil && w.Heard.Kind.IsZero():
		return fmt.Errorf("`heard` needs a `kind`, the sound it listens for: %w", ErrBadWhen)
	case w.Heard != nil && w.Heard.Within < 1:
		return fmt.Errorf("`heard` within %d: a span is counted from 1: %w", w.Heard.Within, ErrBadWhen)
	case w.Heard != nil:
		return w.Heard.Kind.validate("`heard` kind")
	case !w.Fact.IsZero():
		return w.Fact.validate("`fact`")
	case w.Threat != "" && !w.Threat.sealed():
		return fmt.Errorf("`threat: %s`: a threat is none, walk or dash: %w", w.Threat, ErrBadWhen)
	case w.Not != nil:
		if err := w.Not.Validate(); err != nil {
			return fmt.Errorf("inside `not`: %w", err)
		}
	}

	return nil
}

// andList joins words the way a sentence does: "a and b", "a, b and c".
func andList(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}

	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// SelectorWord is a `<word>: <selector>` entry's target, in the author's own
// vocabulary.
type SelectorWord string

const (
	// SelectorEnemy is the nearest opposed member in sight, else the
	// nearest opposed member remembered.
	SelectorEnemy SelectorWord = "enemy"

	// SelectorAttacker is who last landed an attack on this creature, read
	// the way the row's deed condition reads ([Facts.Attacker]). The row is
	// on the table only while the creature holds them placed: in reach to
	// strike ([Facts.FoesInReach]), in sight or memory to walk
	// ([Facts.SeenFoes]). Otherwise it is absent, and a sibling row is the
	// author's fallback.
	SelectorAttacker SelectorWord = "attacker"

	// SelectorActor is the actor of the deed this entry's `when` named, so
	// `fled` pairs with `away: actor`. Legal only in an entry whose When
	// names a deed: there is no actor otherwise, and an entry that named one
	// would be asking for somebody nobody identified.
	SelectorActor SelectorWord = "actor"

	// SelectorSound is WHERE the freshest sound this creature holds was made
	// — any kind, because the row's own `when: { heard: { kind } }` already
	// narrowed what fired it ([Facts.Sounds]). A place, never a figure: a
	// sound names no maker (R4 of rpg-project#569), so the creature walks to
	// it, or from it, and finds out who by looking. Legal on `toward` and
	// `away` only, and only on a row whose `when` list names a `heard`
	// condition (R10), the way `actor` needs a deed; [Answer.Validate]
	// refuses it on `attack` and on a row that listened for nothing.
	// Resolving it to the heard kind is the caller's.
	SelectorSound SelectorWord = "sound"
)

// SelectorWords is every selector word, in the design's own order. Exported
// for the dialect's refusal, [AnswerKeys]'s reason.
var SelectorWords = []SelectorWord{SelectorEnemy, SelectorAttacker, SelectorActor, SelectorSound}

// Selector names what a `attack`/`toward`/`dash`/`away` entry acts on: a member
// found by a word, or an authored cell.
//
// EXACTLY ONE OF THE THREE. A Selector with a Word names somebody the CALLER
// resolves against what this creature sees and remembers; one with At names a
// cell the author wrote; one with Path names a route the author wrote, and
// only `toward` and `dash` walk one ([Answer.Validate] refuses the rest). Which words a
// caller resolves, and which of them it will accept beside which table word,
// is the caller's rule — this module carries the selector and resolves
// nothing.
//
// A CELL, AND ONLY BECAUSE IT IS CARRIED. tools/spatial is as
// rulebook-neutral as this module is, and a position is the one shape every
// board in this repo already speaks; nothing here reads it.
type Selector struct {
	// Word is the selector word, or empty when At or Path is set.
	Word SelectorWord

	// At is the authored DUNGEON-ABSOLUTE cell, or nil when Word or Path is
	// set. A pointer rather than a value because the origin is a legal cell
	// and a zero Position could not be told from "no cell named".
	At *spatial.Position

	// Path is an authored route of DUNGEON-ABSOLUTE cells, walked in order —
	// `toward: { path: [...] }`, a patrol (R7 of rpg-project#569). Empty when
	// Word or At is set. Legal only on `toward` and `dash`.
	//
	// THE ROUTE HOLDS NO STATE. Which waypoint is next is derived by the
	// caller from where the creature stands each time the row fires; nothing
	// here, and nothing on the creature, remembers how far along it got.
	//
	// Omitted when empty, so a selector that names no route marshals as it
	// did before routes existed.
	Path []spatial.Position `json:",omitempty"`

	// Facings is, per waypoint of Path, the bearing the walker faces while it
	// PAUSES on that waypoint, or nil where the author stated none
	// (rpg-project#624). Empty when no waypoint states one; otherwise exactly
	// as long as Path, index for index. A bearing is degrees in [0, 360);
	// which way zero points and which way it turns is the caller's frame.
	//
	// TAKEN ONLY AT THE PAUSE (Kirk, 2026-10-19): standing on the waypoint
	// the walker faces it; walking, and a walk that ends anywhere else, is
	// the caller's ordinary facing rule.
	//
	// Omitted when empty, so a route whose waypoints state no facing marshals
	// as it did before facings existed.
	Facings []*float64 `json:",omitempty"`

	// Waits is, per waypoint of Path, how many rounds the walker STANDS on
	// that waypoint before it walks on, 0 where the author stated none
	// (rpg-project#624; Kirk, 2026-10-19: a patrol walks to an area, stands
	// for N turns, then goes on). Empty when no waypoint states one;
	// otherwise exactly as long as Path. Never negative.
	//
	// THE ROUTE STILL HOLDS NO STATE. How long a creature has stood so far is
	// the caller's to keep on the creature, as which waypoint is next is the
	// caller's to derive; this module carries the number and reads it no more
	// than the cell.
	//
	// Omitted when empty, for Facings' reason.
	Waits []int `json:",omitempty"`
}

// targets names every target this selector sets, each as an author wrote it.
func (s *Selector) targets() []string {
	var out []string
	if s.Word != "" {
		out = append(out, "`"+string(s.Word)+"`")
	}
	if s.At != nil {
		out = append(out, "a cell (`at`)")
	}
	if len(s.Path) > 0 {
		out = append(out, "a route (`path`)")
	}

	return out
}

// validate refuses a selector naming no target or more than one, a route on
// a word that does not walk one, or `sound` on `attack`. word is the table
// word the selector is on.
func (s *Selector) validate(word string) error {
	named := s.targets()
	if len(named) == 0 {
		return fmt.Errorf("`%s` names no target; give it one of somebody, a cell (`at`) or a route (`path`): %w",
			word, ErrBadSelector)
	}
	if len(named) > 1 {
		return fmt.Errorf("`%s` names %s; a target is exactly one of somebody, a cell or a route: %w",
			word, andList(named), ErrBadSelector)
	}
	if s.Word == SelectorSound && word == "attack" {
		return fmt.Errorf("`attack` cannot take `sound`: a sound cannot be struck, only walked toward "+
			"or away from: %w", ErrBadSelector)
	}
	if err := s.validateWaypoints(word); err != nil {
		return err
	}
	if len(s.Path) > 0 && word != "toward" && word != "dash" {
		return fmt.Errorf("`%s` cannot take a `path`: a route is walked, and only `toward` and `dash` walk one: %w",
			word, ErrBadSelector)
	}

	return nil
}

// validateWaypoints refuses a route's per-waypoint facings or waits that
// cannot be matched to its waypoints, a facing that is not a bearing in
// [0, 360), and a negative wait. word is the table word the selector is on.
func (s *Selector) validateWaypoints(word string) error {
	if len(s.Facings) > 0 && len(s.Facings) != len(s.Path) {
		return fmt.Errorf("`%s` states %d waypoint facings for a route of %d waypoints; a facing belongs "+
			"to one waypoint, so a route states one per waypoint or none: %w",
			word, len(s.Facings), len(s.Path), ErrBadSelector)
	}
	if len(s.Waits) > 0 && len(s.Waits) != len(s.Path) {
		return fmt.Errorf("`%s` states %d waypoint waits for a route of %d waypoints; a wait belongs "+
			"to one waypoint, so a route states one per waypoint or none: %w",
			word, len(s.Waits), len(s.Path), ErrBadSelector)
	}
	for i, f := range s.Facings {
		if f != nil && (math.IsNaN(*f) || *f < 0 || *f >= 360) {
			return fmt.Errorf("`%s` waypoint %d faces %v, which is not a bearing in [0, 360): %w",
				word, i+1, *f, ErrBadSelector)
		}
	}
	for i, w := range s.Waits {
		if w < 0 {
			return fmt.Errorf("`%s` waypoint %d waits %d rounds; a wait is never negative: %w",
				word, i+1, w, ErrBadSelector)
		}
	}

	return nil
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
//
// A GROUP LOADS BY THE WORD ITS `as` NAMES (R13), and at 100 when it names
// none: it has no word of its own, and a hidden rule reading its rows would
// be a number the author never wrote.
func (t Temper) factor(a Answer) int {
	if t.Profile == (TemperProfile{}) {
		return 100
	}
	if a.isGroup() {
		return t.factorAs(a.As)
	}
	switch {
	case a.Hold:
		return t.Profile.Hold
	case a.Attack != nil:
		return t.Profile.Attack
	case a.Toward != nil, a.Dash != nil, a.Search != nil:
		// A SEARCH IS A WALK TOWARD where the quarry went, and A DASH IS A
		// RUN TOWARD, so both carry the `toward` factor rather than a sixth
		// number every profile in content would have to grow.
		return t.Profile.Toward
	case a.Away != nil:
		return t.Profile.Away
	case a.Flee:
		return t.Profile.Flee
	default:
		return 100
	}
}

// factorAs is the percent for one of [AsWords], 100 for none.
func (t Temper) factorAs(word string) int {
	switch word {
	case "attack":
		return t.Profile.Attack
	case "toward":
		return t.Profile.Toward
	case "away":
		return t.Profile.Away
	case "flee":
		return t.Profile.Flee
	case "hold":
		return t.Profile.Hold
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
// or done by the creature ([Facts.OwnDeeds]). WHOSE SIDE it was done to is
// what separates them — the projection answered that when it chose which
// list to append to — and Target still names the one it was done to, as the
// witness could vouch for it, because a `fell` row asks which one ([When.Who]).
//
// THE CREATURE'S OWN TESTIMONY, never anybody's live state. A deed is
// stamped when it happened and never restamped, so [Facts.Now] minus At is
// how old it is in rounds.
type HeldDeed struct {
	// Kind is the deed's verb — [VerbAttack], [VerbIntimidate],
	// [VerbPersuade], [VerbFled], [VerbFell].
	Kind string

	// Actor is who did it. What `actor` selects.
	Actor core.EntityID

	// Target is who it was done to, as the witness could vouch for it:
	// empty when the witness could not see them. For [VerbFell] it is the
	// one who fell, which is what `who` reads ([When.Who]).
	Target core.EntityID

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
// THE FOUR ENEMY FLAGS ARE EXCLUSIVE, and the projections enforce it rather
// than every reader re-deriving it: in reach clears seen, seen clears
// remembered, and remembered clears lost. That is what makes `enemy: remembered` mean "I have lost sight
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

	// EnemyLost is true when NONE is in sight or remembered and one is held
	// as lost: looked for where it was last seen, and not there
	// ([EnemyLost]).
	EnemyLost bool

	// EnemyWalk, EnemyDash and EnemyFar are WHAT THIS TURN'S MOVEMENT BUYS
	// against the opposed member `enemy` names: the nearest in sight, else
	// the nearest remembered, at its believed position (rpg-project#671,
	// R18). The host fills them from both creatures' speed and reach:
	//
	//   - EnemyWalk: a walk this turn, at the creature's movement left, ends
	//     in its reach, so it can still swing.
	//   - EnemyDash: a walk does not, and a dash (that movement twice) does.
	//   - EnemyFar: not even a dash does.
	//
	// EXCLUSIVE, AND CLEARED BY REACH, as the bands are: when
	// [Facts.EnemyInReach] holds all three are false, and otherwise exactly
	// one is true whenever an opposed member is held placed (seen or
	// remembered). With none held placed all three are false. The
	// projection that builds Facts enforces it; this module reads the
	// result. THE ZERO VALUE IS "NONE OF THEM", so a host that has not
	// computed them gets `walk`, `dash` and `far` rows that never hold.
	EnemyWalk bool
	EnemyDash bool
	EnemyFar  bool

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

	// CanDash is whether this creature may still spend its action on a
	// `dash` this turn (rpg-project#671): the host sets it from its own
	// action economy — true while the swing is unspent on a turn, and true on
	// a clock with no swing to forfeit. A `dash` row needs it and CanMove
	// both ([Facts.affords]).
	//
	// A RUN THE CREATURE CANNOT PAY FOR IS ABSENT, NOT REFUSED: a picked run
	// the host then refused would end the turn where the table should have
	// rolled another row. The zero value is "cannot", the other Can fields'
	// rule, so a host that has not thought about the action gets no run.
	CanDash bool

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
	// or a sound's At is subtracted from to age it.
	Now uint64

	// Sounds is every sound this creature holds — what a `heard` condition
	// reads. The caller projects it from the creature's holdings on the
	// sounds channel ([sound.Held]); empty is "I heard nothing".
	Sounds []sound.Sound

	// SeenFoes is every opposed member this creature holds in its own sight
	// right now, standing; RememberedFoes is every opposed member it holds
	// placed from an earlier sighting and does not see now (rpg-project#669).
	// What `attacker` needs to name somebody: the one who hit it is on the
	// table only while it is among these ([Facts.Attacker]). The caller
	// projects both from the creature's own sight testimony, never from
	// where anybody really is.
	//
	// EMPTY IS "I HOLD NOBODY", so a caller that has not filled them gets an
	// `attacker` row that is never a candidate, the module's "cannot"
	// default for a forgotten field.
	SeenFoes       []core.EntityID
	RememberedFoes []core.EntityID

	// FoesInReach is every member of SeenFoes this creature can strike now,
	// with any of its attacks — the members `enemy: reach` is about. A strike
	// at the one who hit it is on the table only while they are here: one in
	// sight but out of reach is somebody to walk to, and an `attack` row that
	// named them would be a swing the caller can only refuse.
	FoesInReach []core.EntityID

	// Threat is who else could reach the cell a row's move would LAND on
	// ([ThreatWord]), asked by [Pick] for each row whose `when` names a
	// `threat` condition, with that row, and only then. The host computes:
	//
	//   - the landing cell: where the row's move ends this turn (the cut path
	//     of a `toward`, `dash`, `away` or `search`), or where the creature
	//     stands for a row that does not move, a group row included;
	//   - every opposed member the creature holds placed, at its believed
	//     position, BUT the row's own target (the one its selector names);
	//   - for each, from their speed and reach, whether a walk on their next
	//     turn brings that cell into their reach ([ThreatWalk]), only a dash
	//     does ([ThreatDash]), or neither for any of them ([ThreatNone]).
	//
	// A FUNCTION BECAUSE THE FACT IS PER ROW. Every other fact here is one
	// value per pick; this one depends on where each row would put the
	// creature, and only the host can walk a row's move.
	//
	// NIL HOLDS NOTHING, and so does an empty answer: a host that has not
	// computed threats gets `threat` rows that are never candidates, `none`
	// included, so a careful row cannot be talked into a run by a forgotten
	// field. Never marshalled.
	Threat func(row Answer) ThreatWord `json:"-"`

	// threat is [Facts.Threat]'s answer for the row being judged, filled by
	// [Facts.forRow]. Empty outside a row's judgement.
	threat ThreatWord

	// Known is the id of every world fact this creature holds — what a
	// `fact` condition reads. Whose facts these are (a witness's, a
	// hearer's, a faction's) is the caller's audience rule; this module
	// only asks whether an id is among them.
	Known []string
}

// affords reports whether this creature can still pay for what an entry does.
//
// ONLY THE BUDGETED WORDS ARE GATED. A `fact`, a `flee`, a bare line and
// a `hold` cost nothing a turn can run out of — and `hold` in particular must
// stay eligible whatever the budget says, because it is the word that lets a
// creature with nothing left to spend still have had its turn.
//
// A WALK TO A SOUND NEEDS A SOUND. `toward: sound` and `away: sound` walk
// relative to a sound the creature holds; with none held there is nowhere to
// go, so the row is absent from the roll exactly as an unmet `when` is. The
// row's own `heard` (R10) usually says the same, but [Pick] never validates,
// and eligibility is Pick's to get right either way.
func (f Facts) affords(a Answer) bool {
	switch {
	case a.Attack != nil:
		return f.CanAttack && f.locates(a.Attack, a.When, f.FoesInReach)
	case a.Toward != nil:
		return f.CanMove && f.locates(a.Toward, a.When, f.SeenFoes, f.RememberedFoes)
	case a.Dash != nil:
		return f.CanMove && f.CanDash && f.locates(a.Dash, a.When, f.SeenFoes, f.RememberedFoes) &&
			!f.inReachOf(a.Dash, a.When)
	case a.Away != nil:
		return f.CanMove && f.locates(a.Away, a.When, f.SeenFoes, f.RememberedFoes)
	case a.Search != nil:
		// A SEARCH WALKS ON FROM A LOST TRAIL, so with none held there is
		// nowhere to search from — the rule `sound` keeps for a walk with no
		// sound held, and for the same reason.
		return f.CanMove && f.EnemyLost
	default:
		return true
	}
}

// locates reports whether the facts hold something for this selector to
// name. `sound` names nothing with no sound held. `attacker` names nothing
// unless the one who hit the creature ([Facts.Attacker], read the way the
// row's own deed condition reads) is among the foes it holds placed: in
// reach to strike, in sight or memory to walk (rpg-project#669). The other
// words are resolved by the caller against a board this module does not
// have.
//
// ABSENT, NOT A PASS (R15). A row whose selector names nobody is not on the
// table, so the author's sibling row — `attack: enemy` beneath `attack:
// attacker` — is the fallback, through the roll, and no word hides one.
func (f Facts) locates(s *Selector, when []When, placed ...[]core.EntityID) bool {
	switch s.Word {
	case SelectorSound:
		return len(f.Sounds) > 0
	case SelectorAttacker:
		who, ok := f.Attacker(when)
		if !ok {
			return false
		}
		for _, foes := range placed {
			if slices.Contains(foes, who) {
				return true
			}
		}

		return false
	default:
		return true
	}
}

// inReachOf reports whether the foe a selector names is already in reach,
// which takes a `dash` at them off the table (rpg-project#671): a run at
// somebody the creature could strike is a turn spent for nothing. `enemy`
// is in reach while [Facts.EnemyInReach] holds; `attacker` while the
// attacker is in [Facts.FoesInReach]. A cell, a route and a sound are
// places, never foes, and `actor` is resolved by the caller off a reading
// this module does not hold, so none of them is in reach here.
func (f Facts) inReachOf(s *Selector, when []When) bool {
	switch s.Word {
	case SelectorEnemy:
		return f.EnemyInReach
	case SelectorAttacker:
		who, ok := f.Attacker(when)

		return ok && slices.Contains(f.FoesInReach, who)
	default:
		return false
	}
}

// Attacker is who `attacker` names on a row with this `when` list: the
// actor of the freshest attack in the reading the list's first deed
// condition asks for ([When.Scope]; the creature itself when it names none),
// ties on the stamp broken on the actor (C8). ok is false when that reading
// holds no attack.
//
// THE CREATURE'S OWN TESTIMONY (R8): a deed is held only by a creature that
// witnessed it, so an attack it never saw names nobody. Exported so the
// caller resolves the selector off the same reading this module judged it
// eligible by.
func (f Facts) Attacker(when []When) (core.EntityID, bool) {
	deeds := f.Deeds
	for i := range when {
		if when[i].Deed != "" {
			deeds = when[i].deedsOf(f)

			break
		}
	}
	var best HeldDeed
	found := false
	for _, d := range deeds {
		if d.Kind != VerbAttack || d.Actor == "" {
			continue
		}
		if !found || d.At > best.At || (d.At == best.At && d.Actor < best.Actor) {
			best, found = d, true
		}
	}

	return best.Actor, found
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

// forRow is these facts as one row's `when` reads them: [Facts.Threat]
// asked for this row, when its list names a threat anywhere, inside a `not`
// included. The host is asked once per row and never for a row that does
// not care.
func (f Facts) forRow(row Answer) Facts {
	f.threat = ""
	if f.Threat == nil {
		return f
	}
	for i := range row.When {
		if row.When[i].namesThreat() {
			f.threat = f.Threat(row)

			break
		}
	}

	return f
}

// namesThreat reports whether this condition, or the one a `not` wraps,
// names a threat.
func (w *When) namesThreat() bool {
	for c := w; c != nil; c = c.Not {
		if c.Threat != "" {
			return true
		}
	}

	return false
}

// allHold reports whether every condition in an entry's list holds (R8). An
// empty list holds: no conditions is a standing order (design §2: "an entry
// with no `when` is always eligible").
func allHold(when []When, f Facts) bool {
	for i := range when {
		if !when[i].holds(f) {
			return false
		}
	}

	return true
}

// holds reports whether one condition is true of these facts.
//
// The conditions added beside the first two are read first and each answers
// alone; a When that sets more than one is refused by [When.Validate] and
// read here by the first one this order meets, never as an `and`.
func (w *When) holds(f Facts) bool {
	switch {
	case w.Not != nil:
		return !w.Not.holds(f)
	case w.Heard != nil:
		return w.Heard.holds(f)
	case !w.Fact.IsZero():
		return slices.ContainsFunc(f.Known, w.Fact.Matches)
	case w.Threat != "":
		// AN UNSEALED OR UNCOMPUTED THREAT HOLDS NOTHING: the host's empty
		// answer is "I did not judge it", never "none".
		return w.Threat.sealed() && f.threat == w.Threat
	}

	switch w.Enemy {
	case EnemyReach:
		return f.EnemyInReach
	case EnemySeen:
		return f.EnemySeen
	case EnemyRemembered:
		return f.EnemyRemembered
	case EnemyLost:
		return f.EnemyLost
	case EnemyNone:
		// THE BANDS ARE EXCLUSIVE, so `none` is simply none of them. The
		// projections that build [Facts] are what keep that true — they clear
		// the farther flags once a nearer one holds — and this reads the
		// result rather than re-deciding it.
		return !f.EnemyInReach && !f.EnemySeen && !f.EnemyRemembered && !f.EnemyLost
	case EnemyWalk:
		return f.EnemyWalk
	case EnemyDash:
		return f.EnemyDash
	case EnemyFar:
		return f.EnemyFar
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
		// WHO NARROWS TO ONE TARGET, read off the testimony: a fall the
		// witness could not put a name to has an empty Target and does not
		// match, which is the honest answer to "did I see the king fall".
		if w.Who != "" && d.Target != w.Who {
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

// holds reports whether a sound of this kind is held and fresh: made no more
// than Within units before now.
func (h *HeardWhen) holds(f Facts) bool {
	// A SPAN IS COUNTED FROM 1, the deed condition's rule and for its reason.
	// Not only tidiness: [Pick] never validates, and a negative span would
	// wrap through uint64 to a window that never closes.
	if h.Within < 1 {
		return false
	}
	for _, s := range f.Sounds {
		// A sound stamped ahead of now is wrong, not fresh: the deed
		// condition's rule, and for the same reason.
		if !h.Kind.Matches(s.Kind) || s.At > f.Now {
			continue
		}
		if f.Now-s.At <= uint64(h.Within) {
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

// RowVerdict is one row of the key as the pick judged it, whether or not it
// was eligible (rpg-project#674).
//
// EVERY ROW, NOT ONLY THE CANDIDATES. [Candidate] is the arithmetic of the
// roll; a verdict is the account of the table, and a reader asking "why did
// it not attack?" needs the rows that were out as much as the ones that were
// in. Held and Affordable are judged independently, so a row can say both
// that its condition failed and that it could not have been paid for.
//
// Weight, Percent and Loaded are meaningful ONLY when Held && Affordable —
// they are then exactly the row's [Candidate] numbers — and are zero
// otherwise, because a row that was not on the table loaded nothing.
//
// A GROUP'S VERDICT HOLDS ITS ROWS' VERDICTS (rpg-project#670). For a group
// row ([Answer.Rows]), Held is its own `when` and Affordable is whether any
// row inside it was a candidate (a group pays with its rows), so a group out
// of the roll says which: its gate failed, or nothing inside could fire. Rows
// is then the verdict of every row inside it, judged whether or not the
// group was, and Roll and Of are the inner die when the pick descended into
// it.
type RowVerdict struct {
	// Entry is this row's index in the key's authored entry list.
	Entry int

	// Held reports whether every `when` condition of the row held.
	Held bool

	// Affordable reports whether the creature could pay for the row's word.
	Affordable bool

	// Weight is the author's number, when the row was a candidate.
	Weight int

	// Percent is the temperament's factor, when the row was a candidate.
	Percent int

	// Loaded is Weight × Percent, when the row was a candidate.
	Loaded int

	// Threat is the word the host answered for this row's landing cell
	// ([Facts.Threat]), so a careful row that did not hold says whether a
	// walker or a dasher kept it off the table. Set only for a row whose
	// `when` names a threat, and only when the host answered: an unjudged
	// row reads as absent, never as `none`. Omitted when empty, so a verdict
	// for a row that never asked marshals as it did before the word existed.
	Threat ThreatWord `json:",omitempty"`

	// Rows is a group's verdict for every row inside it, in authored order;
	// nil for a word row.
	Rows []RowVerdict `json:",omitempty"`

	// Roll and Of are the face and the size of the die rolled inside this
	// group, when the pick descended into it; zero otherwise, and always zero
	// for a word row.
	Roll int `json:",omitempty"`
	Of   int `json:",omitempty"`
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

	// Candidates is every eligible entry OF THE TOP LEVEL, in authored
	// order. A group is one candidate here; the candidates inside it are on
	// its verdict in Rows.
	Candidates []Candidate

	// Rows is a verdict for EVERY entry of the key, in authored order —
	// eligible or not — so the pick explains itself row by row. A hold
	// carries them too: a creature that did nothing is the one most in need
	// of saying why.
	Rows []RowVerdict

	// Roll is the top level's face, 1..Of, or 0 when there was nothing to
	// roll. A group's inner face is on its verdict.
	Roll int

	// Of is the top level's die: the sum of every candidate's Loaded.
	Of int

	// Entry is the index of the entry that fired in the key's authored
	// list, or -1 when nothing was eligible. With groups it is the top of
	// the path, Path[0]: the line in the key's own list the die landed on.
	Entry int

	// Path is the index of the row picked at every level, from the key's own
	// list down to the word row that fired: `[3]` for a word row, `[3, 1]`
	// for the second row inside the group at entry 3 (rpg-project#670).
	// Empty when nothing was eligible.
	Path []int

	// Answer is the WORD ROW that fired, at the end of the path — never a
	// group — or the zero-valued [Answer] with Hold set when nothing was
	// eligible under [KeyTime].
	Answer Answer

	// Temper is the word the creature's temperament goes by, or empty for a
	// soldier. Carried so the beat can say WHY the shares are what they are.
	Temper string
}

// holdPick is the pick a `time` key with nothing eligible produces: the
// creature stands there, and the beat records that it was asked.
func holdPick(key AnswerKey, temper string, rows []RowVerdict) *PickOutput {
	return &PickOutput{Key: key, Entry: -1, Answer: Answer{Weight: 1, Hold: true}, Temper: temper, Rows: rows}
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
	top := judge(entries, temper, facts, 0)

	if len(top.candidates) == 0 {
		if key == KeyTime {
			return holdPick(key, temper.Word, top.rows), nil
		}

		return nil, nil
	}
	if top.of < 1 {
		// Every eligible entry loaded to nothing: an author's weights are
		// validated above 0, so this is a temperament whose every factor is
		// zero. A creature that can do nothing at all still had its time, so
		// `time` holds; a social table that cannot be rolled is the refusal
		// it always was rather than a silently skipped answer.
		if key == KeyTime {
			return holdPick(key, temper.Word, top.rows), nil
		}

		return nil, fmt.Errorf("pick: %q weighs %d: %w", key, top.of, ErrBadTable)
	}

	if roller == nil {
		return nil, fmt.Errorf("pick: %q: %w", key, ErrNoDie)
	}

	// ONE DIE PER LEVEL (R0, R12). The top level's face is the output's
	// Roll; each group descended into records its own die on its verdict.
	// A group is a candidate only when something inside it is and its inner
	// die is at least 1, so every descent has something to land on.
	out := &PickOutput{Key: key, Candidates: top.candidates, Rows: top.rows, Of: top.of, Temper: temper.Word}
	level, rows, verdict := top, entries, (*RowVerdict)(nil)
	for {
		roll, err := roller.Roll(ctx, level.of)
		if err != nil {
			return nil, fmt.Errorf("pick: %q at %v: %w", key, out.Path, err)
		}
		if verdict == nil {
			out.Roll = roll
		} else {
			verdict.Roll, verdict.Of = roll, level.of
		}
		chosen := land(level.candidates, roll)
		out.Path = append(out.Path, chosen)
		inner, group := level.inner[chosen]
		if !group {
			out.Answer = rows[chosen]

			break
		}
		verdict = &level.rows[chosen]
		level, rows = inner, rows[chosen].Rows
	}
	out.Entry = out.Path[0]

	return out, nil
}

// level is one table as a pick judged it: its candidates and their die, a
// verdict for every row, and the judged level inside every group that is a
// candidate, by entry.
type level struct {
	candidates []Candidate
	rows       []RowVerdict
	of         int
	inner      map[int]*level
}

// judge is eligibility and load for one level, the same test at every depth.
// nest is how many groups enclose it.
//
// A WORD ROW is a candidate when its `when` all hold and the creature can pay
// for its word. A GROUP is a candidate when its own `when` hold AND something
// inside it is (R15: an empty group is absent, not a hold), loaded at its
// weight × the percent of its `as` (R13). Either way a row out of the roll is
// ABSENT rather than weighted zero, so the candidate list stays the honest
// account of what could have happened, and the verdict records both answers
// for every row.
func judge(entries []Answer, temper Temper, facts Facts, nest int) *level {
	lv := &level{
		candidates: make([]Candidate, 0, len(entries)),
		rows:       make([]RowVerdict, 0, len(entries)),
	}
	for i := range entries {
		entry := &entries[i]
		judged := facts.forRow(*entry)
		held := allHold(entry.When, judged)
		var inner *level
		var affordable bool
		if entry.isGroup() {
			// PAST THE GUARD A GROUP IS ABSENT: [Pick] never validates, and
			// a table built in Go can hold itself.
			if nest+1 <= MaxGroupDepth {
				inner = judge(entry.Rows, temper, facts, nest+1)
				affordable = inner.of >= 1
			}
		} else {
			affordable = facts.affords(*entry)
		}
		verdict := RowVerdict{Entry: i, Held: held, Affordable: affordable, Threat: judged.threat}
		if inner != nil {
			verdict.Rows = inner.rows
		}
		if !held || !affordable {
			lv.rows = append(lv.rows, verdict)
			continue
		}
		percent := temper.factor(*entry)
		loaded := entry.Weight * percent
		verdict.Weight, verdict.Percent, verdict.Loaded = entry.Weight, percent, loaded
		lv.candidates = append(lv.candidates, Candidate{
			Entry: i, Weight: entry.Weight, Percent: percent, Loaded: loaded,
		})
		lv.rows = append(lv.rows, verdict)
		lv.of += loaded
		if inner != nil {
			if lv.inner == nil {
				lv.inner = map[int]*level{}
			}
			lv.inner[i] = inner
		}
	}

	return lv
}

// land is the entry whose running total of loads first reaches the face:
// authored order maps faces to rows, and never decides priority (R0).
func land(candidates []Candidate, roll int) int {
	acc := 0
	for _, c := range candidates {
		acc += c.Loaded
		if roll <= acc {
			return c.Entry
		}
	}

	return candidates[len(candidates)-1].Entry
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
