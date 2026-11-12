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
// # Why its own package
//
// It reads only what dungeonspec and encounter export, adds nothing to
// either's surface, and sits in new files while the compiler beside it keeps
// changing. A later slice that draws "what is connected to what" reads the
// same statements.
package readback
