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
// named with its id beside it, never a pronoun. A recognition says who comes
// to hold the topic, about whom, and who else hears it, in that order; a
// requirement says whose knowledge it reads. Stiff and true beats elegant.
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
