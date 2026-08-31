// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package goal is the guild's needle: one condition over a whole region, with a
// clock on it.
//
// A quest is somebody's job. Somebody takes it, it is about a person or a
// place, and it is finished by whoever took it. A goal is nobody's job. Nobody
// claims it, it is about no one in particular, and it moves when the region
// moves — by whatever means, in whoever's hands. Three companies working three
// different problems by three different methods push one needle, because the
// needle is a fold and a fold does not ask how.
//
// # It never asks who did it
//
// [Condition] reads derived state and job censuses. It cannot see actors,
// claims, or attributions, and it has no way to weight one party's contribution
// against another's. That is not an omission — it is the whole claim. "The
// region is pacified" is true or false about the region; asking which company
// to thank is a different question, and one this package cannot even phrase.
//
// # The clock comes from outside
//
// [Clock] is injected and never defaulted. There is no world clock here and
// none is wanted: a deadline is arithmetic on a timestamp, not a simulation of
// time passing. A tracker built without a clock refuses to exist, because a
// deadline nobody can check is worse than no deadline at all.
//
// # A goal settles once
//
// [Tracker.Observe] moves a goal from open to met or missed, and terminal means
// terminal. Meeting the conditions after the deadline has passed does not
// retro-fire the unlock, and meeting them twice does not fire it twice. What
// the unlock *means* — a bonus stream, a title, three hundred experience points
// each — is the host's business. This package emits [Event] values and never
// learns what anybody did with them.
package goal
