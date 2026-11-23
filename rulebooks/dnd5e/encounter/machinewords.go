// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// machinewords.go is THE MACHINE'S WORDS (rpg-project#997, #1001): one table
// of every word a machine's step may use — the beats it can be on, what its
// condition can read, what it can do — with where each is allowed and how
// each is said. The vocabulary the machine compiles against, the refusals
// that say where a word is allowed, the content compiler's own refusals and
// the read-back's sentences all read it, so none can say a word the others
// do not know.
//
// A WORD IS AS AUTHORED: "heard", "noted:" (a prefix whose open slot is a
// topic), "deed:" (a deed word). Where content is lowered before the engine
// sees it ("noted.anyone:" becomes the engine's "topic:"), the table keeps
// the engine's spelling to itself: a reader of [MachineWords] sees only the
// authored word.
//
// WHERE A WORD IS ALLOWED IS A LIST OF REAL BEATS. Inside the table a
// condition or an act may be allowed on a family of beats (those that name
// someone, those that happened somewhere, those a step may pick on), read
// from the beats' own columns; [MachineWords] publishes the family expanded,
// so a reader never meets a family's name.
//
// THE WORDS ARE SAID THREE WAYS: Long, the read-back's sentence template;
// Short, its folded form; Label, a few plain words for an arrow or a
// dropdown. A template's slots, in braces, are filled by the read-back:
// {who} the runner, {whose} its possessive, {id} the open slot's value
// (quoted), {value} a condition's value, {n} a count, {op} a comparison,
// {verb} a noting's polarity ("has noted" / "has not noted"), {about} the
// one a beat names, {raised} a loudness, {name} a fact's own word, {beat}
// the whole beat (quoted), {parts} a node's parts joined by its Join, and
// {inner} the guard a `not` wraps.

// OpenSlot is what fills a prefix word's open part, and from where.
type OpenSlot string

const (
	// SlotNone is a whole word: nothing follows it.
	SlotNone OpenSlot = ""
	// SlotDeed is a deed word this rulebook names ([WhenDeeds]): `deed:attacked`.
	SlotDeed OpenSlot = "deed"
	// SlotClock is a clock the document declares: `clock:bell:gong`.
	SlotClock OpenSlot = "clock"
	// SlotFact is a fact the document can set: `fact:gate-cowed`.
	SlotFact OpenSlot = "fact"
	// SlotTopic is a topic something in the document gives: `noted:admitted`.
	SlotTopic OpenSlot = "topic"
	// SlotLoudness is a loudness this rulebook names ([Loudnesses]):
	// `heard.loudness:shout`.
	SlotLoudness OpenSlot = "loudness"
	// SlotState is a state of this machine: `to`.
	SlotState OpenSlot = "state"
	// SlotList is a list (a key) this machine's table has: `pick`.
	SlotList OpenSlot = "list"
	// SlotFlag is a key this machine's bag declares: `flag`, `set`, `clear`.
	SlotFlag OpenSlot = "flag"
	// SlotOp is a comparison, one of the table's Ops: `rounds`.
	SlotOp OpenSlot = "op"
	// SlotCondition is a condition built from this table: `all`, `any`,
	// `not`.
	SlotCondition OpenSlot = "condition"
)

// Written is where a word is written in a step, as the author writes it in
// a native recipe (`on`, `when`, `do`), so a client places a word and its
// [Arg]s with no code of its own per word.
type Written struct {
	// In is the part of the step it is written in: "on" (the step's `on`
	// itself), "when" (a key of a condition's map), "do" (a key of an act's
	// map) or "step" (a key of the step itself).
	In string
	// Key is the key it is written under ("fact", "flag", "scope",
	// "rounds", "all", "any", "not", "pick", "wake_in", "set", "clear",
	// "anchor", "to"); empty for a beat, which is `on`'s value itself.
	Key string
	// Field is the field inside Key's map the word itself is spelled in
	// (`fact: { name: fact:gate-cowed }`); empty when the word is spelled
	// only by its Key, or, for a beat, as `on`'s value.
	Field string
}

// The parts of a step a word is written in ([Written.In]).
const (
	// WrittenInOn is the step's `on` itself: a beat.
	WrittenInOn = "on"
	// WrittenInWhen is a key of a condition's map, in the step's `when`.
	WrittenInWhen = "when"
	// WrittenInDo is a key of an act's map, in the step's `do`.
	WrittenInDo = "do"
	// WrittenInStep is a key of the step itself (`to`).
	WrittenInStep = "step"
)

// The kinds an argument may be ([Arg.Kind]); a flag is declared with one of
// the first three.
const (
	ArgKindString    = "string"
	ArgKindInt       = "int"
	ArgKindBool      = "bool"
	ArgKindCondition = "condition" // a condition written as this table's words
	ArgKindOf        = "of"        // the kind the flag named by [Arg.KindOf] was declared with
)

// ArgPlace is where an argument goes in what the author writes. Three
// places cover every word:
//   - [PlaceOpen]: after the word's colon, where the word is spelled:
//     `on: deed:<id>`, `fact: { name: fact:<id> }`.
//   - [PlaceValue]: the value of the word's Key: `flag: <id>`,
//     `pick: <id>`, `wake_in: <n>`, `to: <id>`, `not: <inner>`,
//     `all: [<parts>]`, `clear: [<id>]`, `anchor: true`.
//   - [PlaceField]: a field inside the Key's map, named by [Arg.Field]:
//     `fact: { value: <value>, op: eq }`, `rounds: { op: <op>, n: <n> }`,
//     `scope: { within: <n> }`, `set: { key: <id>, value: <value> }`.
//   - [PlaceImplied]: written nowhere: what the word itself means, stated
//     in [Arg.Fixed] (`within: 2` is at most 2 cells: op `lte`).
type ArgPlace string

// The places an argument goes.
const (
	PlaceOpen  ArgPlace = "open"
	PlaceValue ArgPlace = "value"
	PlaceField ArgPlace = "field"
	// PlaceImplied is an argument the word means and nobody writes.
	PlaceImplied ArgPlace = "implied"
)

// Arg is one thing a word takes: what a studio offers for it, and where it
// goes in what the author writes. Its label slot is "{" + Name + "}".
type Arg struct {
	// Name is the argument's name: "id", "value", "n", "op", "parts",
	// "inner"; the word's label fills "{Name}".
	Name string
	// Goes is where it is written ([ArgPlace]).
	Goes ArgPlace
	// Field is the field it is written in when Goes is [PlaceField].
	Field string
	// Names is what it names: a state, a list, a flag, a fact, a topic, a
	// deed, a clock, a loudness, an op, a condition; SlotNone for a plain
	// value. It is an [OpenSlot] because what fills a word's open part and
	// what an argument names are one closed set: an open word's first
	// argument names exactly its Open.
	Names OpenSlot
	// Kind is its type: "string", "int", "bool", "condition" (a condition
	// written as this table's words), or "of": the kind the flag named by
	// the argument KindOf was declared with, one of Kinds.
	Kind string
	// KindOf names the argument whose flag gives this one its kind, when
	// Kind is "of".
	KindOf string
	// Kinds is the kinds an "of" argument may take: what a flag may be
	// declared with ("bool", "int", "string").
	Kinds []string
	// Values is the closed set it may be, in order; empty for an open one.
	Values []string
	// Many is true when it is written as a list of several (`clear`,
	// `all`, `any`).
	Many bool
	// Optional is true when it may be left out; Default is then what
	// leaving it out means (`rounds` with no `n` is n 0).
	Optional bool
	Default  string
	// Fixed is a value it always has: written for the author, offered to
	// no one (a fact leaf's `op: eq`, `anchor: true`). Empty for an
	// argument the author chooses.
	Fixed string
}

// cloneArgs is args with their lists copied.
func cloneArgs(args []Arg) []Arg {
	out := slices.Clone(args)
	for i := range out {
		out[i].Values = slices.Clone(out[i].Values)
		out[i].Kinds = slices.Clone(out[i].Kinds)
	}
	return out
}

// BeatWord is one beat a step can be on.
type BeatWord struct {
	// Word is the beat as authored: "heard", or a prefix "deed:", "clock:".
	Word string
	// Open is what follows a prefix word; SlotNone for a whole word.
	Open OpenSlot
	// Timed is a beat the clock raises (`time`, `turn`).
	Timed bool
	// Scoped is a beat that happened somewhere: `within` and `regions` may
	// be asked of it.
	Scoped bool
	// Names is a beat whose event names the one it is about: `noted:` may
	// be read on it.
	Names bool
	// Written is where the beat is written: the step's `on`.
	Written Written
	// Args is what the beat takes: an open beat's part (`deed:<word>`);
	// empty for a whole word.
	Args []Arg
	// About is the read-back's words for the one a naming beat is about;
	// empty exactly when Names is false.
	About string
	// Picks is a beat a step may act on with `pick`.
	Picks bool
	// Label is a few plain words for an arrow or a dropdown.
	Label string
	// LabelWith is Label with the thing the beat is about filled in, for a
	// beat that takes one ("learns {id}"); empty where it takes none.
	LabelWith string
	// Short is the read-back's folded form, about "it".
	Short string
	// Long is the read-back's sentence template.
	Long string
}

// ConditionWord is one thing a step's condition can read: an engine fact
// (Fact true) or a machine guard leaf.
type ConditionWord struct {
	// Word is as authored: "heard.claim", "noted:", "fact:", "within", "all".
	Word string
	// Open is what follows a prefix word.
	Open OpenSlot
	// Fact is true for an engine fact a `fact:` guard leaf names; false for
	// a machine guard leaf (`flag`, `within`, `regions`, `rounds`, `all`,
	// `any`, `not`).
	Fact bool
	// Kind is the value's kind: "bool", "string" or "int"; empty for a
	// leaf that carries none (`all`, `any`, `not`).
	Kind string
	// Values is the closed set of values it may ask, in the order a refusal
	// lists them; empty for an open value.
	Values []string
	// Written is where the condition is written: a key of a `when` map.
	Written Written
	// Args is what it takes, in order: its open part, the value it
	// compares, a comparison, a count, the conditions a node holds.
	Args []Arg
	// On is the beats it may be read on, as [BeatWord.Word] spells them, in
	// table order: a whole beat, or an open family's word (`deed:`) for
	// every beat of that family; empty for any beat.
	On []string
	// StatesForm is how the states dialect writes it in a `when`
	// ("{ enemy }"); empty where the dialect does not read it.
	StatesForm string
	// Join is how the read-back joins a node's parts: " and " for `all`,
	// ", or " for `any`; empty for a leaf.
	Join string
	// Label is a few plain words for an arrow; for a word with closed
	// values, the fallback for a value ValueLabels does not name.
	Label string
	// LabelNot is the label when the condition is asked to be false ("has
	// not learned {id}"); empty where Label's own negation reads right.
	LabelNot string
	// ValueLabels is, for a word with closed values, each value's own label
	// ("seen": "sees an enemy").
	ValueLabels map[string]string
	// Long is the read-back's sentence template.
	Long string

	// engine is the engine's spelling once content is lowered, where it
	// differs from Word ("topic:" for "noted.anyone:"). Never published.
	engine string
	// family is a family of beats it is allowed on besides On; published
	// expanded into On.
	family beatFamily
}

// beatFamily names a family of beats, read from the beats' own columns.
type beatFamily string

const (
	familyNone   beatFamily = ""
	familyNaming beatFamily = "naming" // [BeatWord.Names]
	familyScoped beatFamily = "scoped" // [BeatWord.Scoped]
	familyPicks  beatFamily = "picks"  // [BeatWord.Picks]
)

// beats is the family's beats, in table order, an open beat family by its
// table word (`deed:` stands for every `deed:<word>`).
func (f beatFamily) beats() []string {
	var keep func(BeatWord) bool
	switch f {
	case familyNaming:
		keep = func(b BeatWord) bool { return b.Names }
	case familyScoped:
		keep = func(b BeatWord) bool { return b.Scoped }
	case familyPicks:
		keep = func(b BeatWord) bool { return b.Picks }
	default:
		return nil
	}
	var out []string
	for _, b := range machineBeatWords {
		if keep(b) {
			out = append(out, b.Word)
		}
	}
	return out
}

// ActWord is one thing a step can do: move (`to`) or an act.
type ActWord struct {
	// Word is as authored: "to", "pick", "wake_in", "set", "clear", "anchor".
	Word string
	// Written is where the act is written: a key of a `do` item, or `to`
	// on the step itself.
	Written Written
	// Args is what it takes, in order.
	Args []Arg
	// On is the beats a step may do it on, spelled as [ConditionWord.On];
	// empty for any beat.
	On []string
	// StatesForm is how the states dialect writes it in `do`
	// ("{ pick: <key> }"); empty where the dialect does not take it.
	StatesForm string
	// Label is a few plain words for an arrow.
	Label string
	// Short is the read-back's folded form.
	Short string
	// Long is the read-back's sentence form.
	Long string

	// family is a family of beats it is allowed on besides On; published
	// expanded into On.
	family beatFamily
}

// MachineWordsTable is every word a machine's step may use, with where each
// is allowed and how each is said.
type MachineWordsTable struct {
	Beats      []BeatWord
	Conditions []ConditionWord
	Acts       []ActWord
	// Who is every word that names who in a note ([WhoWord]).
	Who []WhoWord
	// Ops is every comparison a condition can make, with its plain words,
	// so no reader maps the engine's `eq` or `gte` itself.
	Ops []OpWord
}

// OpWord is one comparison a condition can make.
type OpWord struct {
	// Op is the engine's spelling: "eq", "gte".
	Op string
	// Words is how a label or a sentence says it: "exactly", "at least".
	Words string
}

// OpAtMost is the comparison a `within` or `regions` leaf makes: the event
// was at most N cells or rooms away. The machine has no such op; the leaf
// itself means it.
const OpAtMost machine.Op = "lte"

// machineOpWords is every comparison a condition makes: the two the machine
// compiles ([machine.OpEq], [machine.OpGte]) and the bound a `within` or
// `regions` leaf means ([OpAtMost]).
var machineOpWords = []OpWord{
	{Op: string(machine.OpEq), Words: "exactly"},
	{Op: string(machine.OpGte), Words: "at least"},
	{Op: string(OpAtMost), Words: "at most"},
}

// OpWordsOf is the plain words for a comparison as the engine spells it;
// false for one the machine does not compile.
func OpWordsOf(op string) (string, bool) {
	for _, o := range machineOpWords {
		if o.Op == op {
			return o.Words, true
		}
	}
	return "", false
}

// argValues is a closed set as strings.
func argValues[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// flagKinds is every kind a flag may be declared with: a bag's values.
var flagKinds = []string{ArgKindBool, ArgKindInt, ArgKindString}

// The arguments the table's words share.
var (
	argOpen = func(names OpenSlot, values ...string) Arg {
		return Arg{Name: "id", Goes: PlaceOpen, Names: names, Kind: ArgKindString, Values: values}
	}
	argFactValue = func(kind string, values ...string) Arg {
		return Arg{Name: "value", Goes: PlaceField, Field: "value", Kind: kind, Values: values}
	}
	argFactEq    = Arg{Name: "op", Goes: PlaceField, Field: "op", Names: SlotOp, Kind: ArgKindString, Fixed: string(machine.OpEq)}
	argScopeN    = func(field string) Arg { return Arg{Name: "n", Goes: PlaceField, Field: field, Kind: ArgKindInt} }
	argAtMost    = Arg{Name: "op", Goes: PlaceImplied, Names: SlotOp, Kind: ArgKindString, Fixed: string(OpAtMost)}
	argValueN    = Arg{Name: "n", Goes: PlaceValue, Kind: ArgKindInt}
	argValueFlag = Arg{Name: "id", Goes: PlaceValue, Names: SlotFlag, Kind: ArgKindString}
	argInner     = Arg{Name: "inner", Goes: PlaceValue, Names: SlotCondition, Kind: ArgKindCondition}
	argParts     = Arg{Name: "parts", Goes: PlaceValue, Names: SlotCondition, Kind: ArgKindCondition, Many: true}
)

// Where the table's words are written.
var (
	writtenOn = Written{In: WrittenInOn}
	inFact    = Written{In: WrittenInWhen, Key: "fact", Field: "name"}
	inWhen    = func(key string) Written { return Written{In: WrittenInWhen, Key: key} }
	inDo      = func(key string) Written { return Written{In: WrittenInDo, Key: key} }
)

// The beats in the order every refusal lists them: the fixed words, then
// the open families.
var machineBeatWords = []BeatWord{
	{Word: "time", Written: writtenOn, Timed: true, Picks: true, Label: "time passes",
		Short: "on its time (out of a fight)",
		Long:  "on {whose} own time (each round {who} is due, out of a fight)"},
	{Word: "turn", Written: writtenOn, Timed: true, Picks: true, Label: "its turn in a fight",
		Short: "on its turn in a fight",
		Long:  "on {whose} turn in a fight"},
	{Word: "heard", Written: writtenOn, Scoped: true, Names: true, Label: "hears something",
		About: "the one {who} heard ({who} knows who only by seeing them)",
		Short: "on hearing a sound (a character's line at once)",
		Long:  "when {who} hears a sound (a line a character says is heard at once, as it is said)"},
	{Word: "deed", Written: writtenOn, Scoped: true, Label: "a deed is done",
		Short: "when a deed is done",
		Long:  "when a deed is done"},
	{Word: "seen", Written: writtenOn, Scoped: true, Names: true, Label: "sees someone opposed",
		About: "the one {who} saw",
		Short: "when an opposed creature comes into its sight",
		Long:  "when an opposed creature comes into {whose} sight"},
	{Word: "fact", Written: writtenOn, Label: "learns a fact", LabelWith: "learns {id}",
		Short: "on learning a fact",
		Long:  "when {who} learns a fact"},
	{Word: "topic", Written: writtenOn, Names: true, Label: "notes a topic about someone", LabelWith: "notes {id} about someone",
		About: "the one the note is about",
		Short: "on noting a topic about someone (judged or heard)",
		Long:  "when {who} notes a topic about someone, by its own judgement or by hearing it"},
	{Word: "clock:world", Written: writtenOn, Label: "a round of world time",
		Short: "on each round of world time",
		Long:  "on each round of world time (out of a fight)"},
	{Word: "clock:fight", Written: writtenOn, Label: "a round of its side's fight",
		Short: "on each round of a fight its side is in",
		Long:  "on each round of a fight {whose} side is in"},
	{Word: "barred", Written: writtenOn, Scoped: true, Names: true, Picks: true, Label: "someone walks into it",
		About: "the one who walked into {who}",
		Short: "when someone walks into it and its body stops them",
		Long:  "when someone walks into {who} and its body stops them"},
	{Word: "approached", Written: writtenOn, Scoped: true, Names: true, Picks: true, Label: "hails someone",
		About: "the one {who} hailed",
		Short: "when it is the carrier that hails a character (only under the conversation's hail rule)",
		Long: "when {who} hails a character, which happens only under the conversation's hail rule: the " +
			"character's own step ended within the approach distance, {who} sees them and they see {who}, {who} offers them a line " +
			"that steps the conversation, no nearer carrier qualifies, and the conversation is addressing no one"},
	{Word: deedPrefix, Written: writtenOn, Open: SlotDeed, Scoped: true, Label: "{id} is done",
		Args:  []Arg{argOpen(SlotDeed, WhenDeeds...)},
		Short: "when the deed {id} is done",
		Long:  "when the deed {id} is done"},
	{Word: beatClock, Written: writtenOn, Open: SlotClock, Label: "a round of {id}",
		Args:  []Arg{argOpen(SlotClock)},
		Short: "on the beat {beat}",
		Long:  "on the beat {beat}"},
}

// The conditions, engine facts first, then the machine's guard leaves.
var machineConditionWords = []ConditionWord{
	{Word: string(factEnemy), Fact: true, Kind: ArgKindString,
		Values: []string{string(EnemySeen), string(EnemyRemembered), string(EnemyNone)}, StatesForm: "{ enemy }",
		Written:     inFact,
		Args:        []Arg{argFactValue(ArgKindString, string(EnemySeen), string(EnemyRemembered), string(EnemyNone)), argFactEq},
		Label:       "an enemy is {value}",
		ValueLabels: map[string]string{string(EnemySeen): "sees an enemy", string(EnemyRemembered): "remembers an enemy", string(EnemyNone): "knows of no enemy"},
		Long:        "{whose} {name} is {op} {value}"},
	{Word: factPrefix, Open: SlotFact, Fact: true, Kind: ArgKindBool, StatesForm: "{ fact }",
		Written: inFact, Args: []Arg{argOpen(SlotFact), argFactValue(ArgKindBool), argFactEq},
		Label: "has learned {id}", LabelNot: "has not learned {id}",
		Long: "{who} knows the fact {id}"},
	{Word: "heard.kind", Fact: true, Kind: ArgKindString, Values: []string{SoundBoom, SoundCrash, SoundClang, SoundVoice},
		Written: inFact, Args: []Arg{argFactValue(ArgKindString, SoundBoom, SoundCrash, SoundClang, SoundVoice), argFactEq},
		Label: "it was a {value}",
		ValueLabels: map[string]string{SoundBoom: "it was a boom", SoundCrash: "it was a crash", SoundClang: "it was a clang",
			SoundVoice: "it was a voice"},
		Long: "what {who} heard was a {value}"},
	{Word: heardLoudnessPrefix, Open: SlotLoudness, Fact: true, Kind: ArgKindBool,
		Written: inFact, Args: []Arg{argOpen(SlotLoudness, argValues(Loudnesses)...), argFactValue(ArgKindBool), argFactEq},
		Label: "it was at least a {id}",
		Long:  "what {who} heard was a voice{raised} (a clang, a crash or a boom is at no loudness)"},
	{Word: string(factHeardClaim), Fact: true, Kind: ArgKindString, Values: []string{ClaimHeld, ClaimFailed}, On: []string{string(beatHeard)},
		Written: inFact, Args: []Arg{argFactValue(ArgKindString, ClaimHeld, ClaimFailed), argFactEq},
		Label:       "it was a claim that {value}",
		ValueLabels: map[string]string{ClaimHeld: "it was a claim that held", ClaimFailed: "it was a claim that failed"},
		Long:        "what {who} heard was a claim that {value}"},
	{Word: notedPrefix, Open: SlotTopic, Fact: true, Kind: ArgKindBool, family: familyNaming,
		Written: inFact, Args: []Arg{argOpen(SlotTopic), argFactValue(ArgKindBool), argFactEq},
		Label: "has noted {id} about them", LabelNot: "has not noted {id} about them",
		Long: "{who} {verb} {id} about {about}"},
	{Word: notedAsPrefix, Open: SlotTopic, Fact: true, Kind: ArgKindString, family: familyNaming,
		Written: inFact, Args: []Arg{argOpen(SlotTopic), argFactValue(ArgKindString), argFactEq},
		Label: "has noted {id} as {value} about them", LabelNot: "has not noted {id} as {value} about them",
		Long: "{who} {verb} {id} as {value} about {about}"},
	{Word: notedAnyonePrefix, Open: SlotTopic, engine: topicPrefix, Fact: true, Kind: ArgKindBool,
		Written: inFact, Args: []Arg{argOpen(SlotTopic), argFactValue(ArgKindBool), argFactEq},
		Label: "has noted {id} about anyone", LabelNot: "has not noted {id} about anyone",
		Long: "{who} {verb} {id} about anyone"},
	{Word: "flag:", Open: SlotFlag, Kind: ArgKindBool, Written: inWhen("flag"), Args: []Arg{argValueFlag},
		Label: "{id} is set",
		Long:  "{whose} flag {id} is set"},
	{Word: "within", Kind: ArgKindInt, family: familyScoped, Written: inWhen("scope"), Args: []Arg{argScopeN("within"), argAtMost},
		Label: "within {n} cells",
		Long:  "it happened within {n} cells of {who}"},
	{Word: "regions", Kind: ArgKindInt, family: familyScoped, Written: inWhen("scope"), Args: []Arg{argScopeN("regions"), argAtMost},
		Label: "within {n} rooms",
		Long:  "it happened within {n} rooms of {who}"},
	{Word: "rounds", Kind: ArgKindInt,
		Written: inWhen("rounds"),
		Args: []Arg{{Name: "op", Goes: PlaceField, Field: "op", Names: SlotOp, Kind: ArgKindString, Values: []string{string(machine.OpEq), string(machine.OpGte)}},
			// NO `n` IS n 0: with `eq`, the very round it marked the moment.
			{Name: "n", Goes: PlaceField, Field: "n", Kind: ArgKindInt, Optional: true, Default: "0"}},
		Label: "{op} {n} rounds since it marked the moment",
		Long:  "{op} {n} rounds have passed since {who} set its anchor"},
	{Word: "all", Join: " and ", Written: inWhen("all"), Args: []Arg{argParts}, Label: "all of these", Long: "{parts}"},
	{Word: "any", Join: ", or ", Written: inWhen("any"), Args: []Arg{argParts}, Label: "any of these", Long: "({parts})"},
	{Word: "not", StatesForm: "{ not }", Written: inWhen("not"), Args: []Arg{argInner}, Label: "not this", Long: "NOT ({inner})"},
}

// The acts, the move first.
var machineActWords = []ActWord{
	{Word: "to", Written: Written{In: WrittenInStep, Key: "to"}, Args: []Arg{{Name: "id", Goes: PlaceValue, Names: SlotState, Kind: ArgKindString}}, Label: "changes to {id}", Short: "to {id}", Long: "moves to the state {id}"},
	{Word: "pick", family: familyPicks, StatesForm: "{ pick: <key> }",
		Written: inDo("pick"), Args: []Arg{{Name: "id", Goes: PlaceValue, Names: SlotList, Kind: ArgKindString}},
		Label: "acts from the {id} list", Short: "acts from its {id} rows", Long: "acts from its own {id} rows"},
	{Word: "wake_in", StatesForm: "{ wake_in: N }", Written: inDo("wake_in"), Args: []Arg{argValueN},
		Label: "waits {n} rounds", Short: "wakes again in {n} rounds", Long: "wakes again in {n} rounds"},
	{Word: "set", Written: inDo("set"), Args: []Arg{
		{Name: "id", Goes: PlaceField, Field: "key", Names: SlotFlag, Kind: ArgKindString},
		{Name: "value", Goes: PlaceField, Field: "value", Kind: ArgKindOf, KindOf: "id", Kinds: flagKinds}}, Label: "sets {id} to {value}", Short: "sets {id} to {value}", Long: "sets {id} to {value}"},
	{Word: "clear", Written: inDo("clear"), Args: []Arg{{Name: "id", Goes: PlaceValue, Names: SlotFlag, Kind: ArgKindString, Many: true}}, Label: "forgets {id}", Short: "clears {id}", Long: "clears {id}"},
	{Word: "anchor", StatesForm: "{ anchor: true }", Written: inDo("anchor"),
		Args:  []Arg{{Name: "value", Goes: PlaceValue, Kind: ArgKindBool, Fixed: "true"}},
		Label: "marks the moment", Short: "sets its anchor", Long: "sets its anchor to this moment"},
}

// notedAnyonePrefix is the content word for a topic noted about anybody
// (rpg-project#1005), lowered to the engine's [topicPrefix].
const notedAnyonePrefix = "noted.anyone:"

// MachineWords is the table of every word a machine's step may use, as a
// reader sees it: every On a list of real beats, no engine spelling. The
// slices are copies: a caller's change reaches nothing.
func MachineWords() MachineWordsTable {
	out := MachineWordsTable{
		Beats:      slices.Clone(machineBeatWords),
		Conditions: slices.Clone(machineConditionWords),
		Acts:       slices.Clone(machineActWords),
		Ops:        slices.Clone(machineOpWords),
		Who:        WhoWords(),
	}
	for i := range out.Beats {
		out.Beats[i].Args = cloneArgs(out.Beats[i].Args)
	}
	for i := range out.Conditions {
		c := &out.Conditions[i]
		c.Args = cloneArgs(c.Args)
		c.Values = slices.Clone(c.Values)
		c.ValueLabels = maps.Clone(c.ValueLabels)
		c.On, c.engine, c.family = c.allowedOn(), "", familyNone
	}
	for i := range out.Acts {
		a := &out.Acts[i]
		a.Args = cloneArgs(a.Args)
		a.On, a.family = a.allowedOn(), familyNone
	}
	return out
}

// BeatWordOf is the table's entry for a beat as authored: its whole word,
// else its prefix family; false for a beat the table does not have.
func BeatWordOf(on string) (BeatWord, bool) {
	for _, b := range machineBeatWords {
		if b.Open == SlotNone && b.Word == on {
			return b, true
		}
	}
	for _, b := range machineBeatWords {
		if b.Open != SlotNone && strings.HasPrefix(on, b.Word) && len(on) > len(b.Word) {
			return b, true
		}
	}
	return BeatWord{}, false
}

// ConditionWordOf is the table's entry for a condition by its word (an open
// word bare, "flag:", or filled, "flag:lit") or its engine spelling
// ("noted.anyone:x" and "topic:x" both find `noted.anyone:`); false for a
// word the table does not have.
func ConditionWordOf(name string) (ConditionWord, bool) {
	for _, c := range machineConditionWords {
		if c.Word == name { // a whole word, or an open word named bare
			return c, true
		}
	}
	for _, c := range machineConditionWords {
		if c.Open == SlotNone {
			continue
		}
		for _, prefix := range []string{c.Word, c.engine} {
			if prefix != "" && strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
				return c, true
			}
		}
	}
	return ConditionWord{}, false
}

// ActWordOf is the table's entry for an act or the move.
func ActWordOf(word string) (ActWord, bool) {
	for _, a := range machineActWords {
		if a.Word == word {
			return a, true
		}
	}
	return ActWord{}, false
}

// beatsWhere is every fixed beat (and the deed family) for which keep
// holds, in table order: the list a refusal names.
func beatsWhere(keep func(BeatWord) bool) []string {
	var out []string
	for _, b := range machineBeatWords {
		if b.Open == SlotNone && keep(b) {
			out = append(out, b.Word)
		}
	}
	return out
}

// allowedOn is the beats a condition may be read on, its family expanded;
// nil for any beat.
func (c ConditionWord) allowedOn() []string {
	return expandOn(c.On, c.family)
}

// allowedOn is the beats an act may be done on, its family expanded; nil for
// any beat.
func (a ActWord) allowedOn() []string {
	return expandOn(a.On, a.family)
}

// expandOn is on with a family's beats after it.
func expandOn(on []string, family beatFamily) []string {
	if len(on) == 0 && family == familyNone {
		return nil
	}
	return append(slices.Clone(on), family.beats()...)
}

// SplitConditionWord is the table's entry for a condition as authored or
// as the engine spells it, with its open slot's value ("noted.anyone:x" and
// "topic:x" both answer `noted.anyone:` and "x"); false for a word the table
// does not have.
func SplitConditionWord(name string) (ConditionWord, string, bool) {
	c, ok := ConditionWordOf(name)
	if !ok || c.Open == SlotNone {
		return c, "", ok
	}
	for _, prefix := range []string{c.Word, c.engine} {
		if prefix != "" && strings.HasPrefix(name, prefix) {
			return c, strings.TrimPrefix(name, prefix), true
		}
	}
	return c, "", true
}

// beatAllowed reports whether a beat as authored is one of on's beats: a
// whole word by itself, a member of an open family by the family's table
// word ("deed:attacked" is on a list holding "deed:"). A reader of
// [MachineWords] matches a beat against On the same way.
func beatAllowed(on []string, beat machine.Beat) bool {
	if len(on) == 0 || slices.Contains(on, string(beat)) {
		return true
	}
	b, ok := BeatWordOf(string(beat))
	return ok && b.Open != SlotNone && slices.Contains(on, b.Word)
}

// pickWord is the table's `pick` act: the beats a step may pick on.
var pickWord, _ = ActWordOf("pick")
