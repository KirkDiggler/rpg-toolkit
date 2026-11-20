// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package readback says a content file back in plain sentences
// (rpg-project#940, the read-back; slice one is rpg-project#941).
//
// # What it says, and from what
//
// [Of] compiles a site document with the dialect's own loader and says the
// COMPILED result back: what will run, whoever wrote the file — a person,
// the studio or a generator. The authored spec is read beside it only for
// what the compiled form no longer carries: where in the file each thing is
// set (its path, the same form the compiler's refusals use), the display
// name a stat block gives a creature, and the cells an author wrote.
//
// # Two layers
//
// A [ReadBack] is structured: every [Statement] has a stable [Kind], the
// thing it is about ([Subject]), one plain sentence, the path where that
// thing is set, and what it connects to ([Connection]: the topics, facts,
// conversations, creatures and doors it reads or changes). [ReadBack.Text]
// renders it for a person, each sentence followed by its path.
// [ReadBack.Folded] groups the same statements for a person reading the
// whole file, naming what one creature or routine shares with another;
// [ReadBack.FoldedEach] groups them for one thing at a time, each said in
// full under its own heading. Both name an answer's loudness and say what it
// reaches once per speaker, and every folded entry carries the paths of the
// statements it stands for.
//
// # Hubs
//
// The last sections say each hub with its thread: every topic and fact, then
// every record and every secret (rpg-project#973) — the hub's own statement,
// then each statement that reaches it, repeated at its path under a kind
// that says how (`record.hold`, `record.tell`, `record.lift`;
// `secret.reveal`, `secret.find`, `secret.bar`, `secret.need`). A record
// connects what it reveals and a secret what it hides, so a secret passage is
// followed from the line that tells it to the prop it hides by connections
// alone. A WAY is a statement whose kind begins `place.way.`; a way repeated
// under a record or secret hub is a pointer to it, carrying only
// {way, <id>, is}, and is never itself a way. Only a way carries a `method`
// connection.
//
// # The sentences
//
// Mechanical, "this knows that": subject first, active voice, every actor
// named with its id beside it, never a pronoun. A topic given says who is
// given it, about whom they note it, and who else hears it, in that order; a
// requirement says whose knowledge it reads. Stiff and true beats elegant.
//
// # What a machine's step reads (rpg-project#1000, #1005, #997)
//
// Every condition in a step's `when` is one connection of kind `condition`
// ([ConnCondition]), role `asks`, so a reader rebuilds the whole `when`
// from connections alone:
//
//   - id: the condition's word as authored, its open part filled, from the
//     machine words table ([encounter.MachineWords]): `heard.claim`,
//     `noted:admitted`, `noted.anyone:admitted` (the engine's `topic:`),
//     `fact:gate-cowed`, `heard.kind`, `heard.loudness:shout`, `enemy`,
//     `flag:<name>`, `within`, `regions`, `rounds`.
//   - is: the value compared, as text ("failed", "true", "3").
//   - op: the comparison where it is not equality, as the engine spells it:
//     `gte` for `rounds`, `lte` for `within` and `regions` (at most); the
//     table's Ops give the plain words ("at least", "at most").
//   - not: this leaf sits under an odd number of `not`s. A hint, not the
//     structure: `at` is what makes it exact. Under `not: { all: [a, b] }`
//     both carry not, at `when.not.all[0]` and `when.not.all[1]`, and the
//     step needs NOT (a and b).
//   - at: where it sits in the `when` as the author wrote it: `when`,
//     `when.all[1]`, `when.any[0].not`. Conditions at one place are one
//     node, all of them needed; `all`, `any` and `not` are read from the path
//     and have no connection of their own.
//
// A condition that reads a thing in the file keeps that thing's connection
// beside it, which the hubs group by: `fact:<id>` {fact, <id>, wakes-on};
// `noted:<topic>` {topic, <topic>, asks}; `noted.anyone:<topic>` {topic,
// <topic>, wakes-on}. The sentence says "has noted" or "has not noted", and
// NOT (…) for a `not`; no sentence depends on the connections.
//
// The captain's step in `in-hall` on `heard` (the castle), whose `when` is
// an `all` of a failed claim and `noted:admitted`, carries:
//
//	{condition, heard.claim, asks, is: failed, at: when.all[0]}
//	{condition, noted:admitted, asks, is: true, at: when.all[1]}
//	{topic, admitted, asks}
//	{who, the_one_named, about, at: admitted}
//
// # Trace one note (rpg-project#1001)
//
// Every statement that gives a topic or reads one says who, through one
// function (noteedges.go), in the wire's own fields. A give carries
// {topic, <topic>, grants}; {who, <word>, about, at: <topic>}, whom it is
// about; {who, <word>, takes, is: <way>, at: <topic>} for each taker word,
// the rule, with its way (`judgement` or `heard`, the engine's words); and
// {creature, <id>, may-note, is: <way>, at: <topic>} for each creature that
// rule resolves to in the file. A may-note edge lists who MAY come to note
// it, not who will: the statement's sentence and conditions say when. What
// gave it follows: {option, <id>, on, is: said|win|miss} or {conversation,
// <id>, enters, is: <state>}. A read carries {topic, <topic>, asks} (or
// wakes-on), negated when asked false; {who, <word>, about, at: <topic>};
// and {creature, <id>, whose, at: <topic>} when it reads one creature's
// notes. The words, their labels and their ways are the engine's who-words
// list ([encounter.WhoWords]).
//
// # What a machine does and remembers (rpg-project#1001)
//
// Every act of a step is one connection of kind `act` ([ConnAct]), role
// `does`, in the order the author wrote them: id the act's word, then a
// colon and what it names where it names something; is its value or count;
// at its place.
//
//	{act, to:alert, does, at: to}
//	{act, pick:post, does, at: do[0]}
//	{act, set:warned, does, is: true, at: do[1]}
//	{act, clear:warned, does, at: do[0].clear[0]}
//	{act, wake_in, does, is: 3, at: do[2]}
//	{act, anchor, does, at: do[3]}
//
// A value is its JSON literal (true, 2, "idle"), so its kind reads off it.
// `to` and `pick` keep their {state, …, to} and {table, …, picks} edges.
//
// A routine's memory (its recipe's bag) is one statement per flag, kind
// `routine.flag`, at `behaviorDefinitions.<routine>.recipe.bag.<flag>`,
// with {flag, <flag>, declares, is: <its starting value>}. A step that
// reads the flag carries {flag, <flag>, asks} beside its `flag:<flag>`
// condition; one that sets or clears it carries {flag, <flag>, sets} or
// {flag, <flag>, clears} beside its act. A reader traces one flag by kind
// `flag` and its id within one routine (the statement's path names it).
//
// # Status effects (rpg-project#1024)
//
// Each status under the root's `statuses:` is one statement of kind
// `place.status` ([KindStatus]) about {status, <id>}, at `statuses.<id>`,
// naming it by id and name. A line's row that gives one is a `line.grant`
// statement at the row's `.apply_status`, connected {status, <id>, gives}
// ([ConnStatus], [RoleGives]). What a status changes and what ends it are
// the rules' words, judged when it is given; the read-back does not restate
// them.
//
// # Nothing skipped silently
//
// Whatever the file authors and this package cannot yet say is listed in
// [ReadBack.NotYetSaid] with its path and kind. Geometry is summarised:
// regions by count and name, walls and scene items by count.
//
// # Where the places join
//
// Under the place, one statement per join between two regions
// (rpg-project#971): at a door, on open floor (feet cross where their cells
// touch; the sentence says whether sight does too), or by a passage that is
// not physical (a row's `relocate`, an arrest and its jail). Feet decide a
// join: a seam only sight crosses (a window) is said as a sight seam, a door
// joining no two regions (within one, or off the floor) is said about the
// door, and a region in no join is said to be in none. After each join, its
// WAYS (rpg-project#981). A join is barred by zero or more bars (a lock,
// holders, a secret), each way lifts one bar, and a walker gets through when
// every bar is lifted: a lock approach, a line or a known fact or carried
// prop that meets every holder's pass, the fight, a search or a told record;
// a passage's carrier is how it is taken once its bars are lifted; what
// helps past some holders only is partial. Each way connects to its bar,
// method, needs and what a miss grants or steps, so a consumer counts them
// without reading a sentence. The geometry is the engine's own ([encounter.FloorTopology]); a
// join's state is the door's, its holders' and the secret's, reached through
// its connections and never restated.
//
// # Why its own package
//
// It reads only what dungeonspec and encounter export, adds nothing to
// either's surface, and sits in new files while the compiler beside it keeps
// changing. A later slice that draws "what is connected to what" reads the
// same statements.
package readback
