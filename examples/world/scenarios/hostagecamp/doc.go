// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package hostagecamp is UC-2: one job, three companies, three hostages, and
// whatever the dice make of it.
//
// It is the second scenario, and its job is to be a second one. Everything both
// it and banditcamp needed is one layer up now — the act loop, the verb
// vocabulary, the resolver seam, the scripted dice — and everything left in
// here is content: names, relationships, folds, verbs, a contract. That split
// is the result the package exists to demonstrate, and it is why this file
// declares no machinery at all.
//
// # A population, not a quest
//
// [Contract] has three subjects. A claim takes one off the board and mints the
// claiming company's own instance about that person, and nothing ever puts one
// back. Party Ember's failure turns Ember's hostage; Quill's and Thorn's are
// untouched, and no code compares parties to make that so — the instances were
// never about the same person.
//
// # The follow-up nobody branched on
//
// When the population settles into "nobody still captive, and everybody
// turned", a second job opens about exactly the people who ended up there. That
// is a [quest.Distribution] over a census of the world, not a rule somebody
// wrote: no line anywhere says "if all three fail, offer the reckoning". It
// opens once, and a population that later drifts back across the line does not
// offer it twice.
//
// # Flags only go up, so order carries the meaning
//
// A redeemed hostage is still carrying the flag that says they turned. Nothing
// clears it and nothing should — unwitnessing is not an event. Redemption wins
// because the projection that reads it is declared after the one that reads
// turning, and the bucket that asks about it is asked first. Precedence is the
// mechanism; see [Declaration] and the bucket list.
package hostagecamp
