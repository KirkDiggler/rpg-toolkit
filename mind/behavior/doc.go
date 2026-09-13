// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package behavior is what a monster's mind has to be.
//
// A mind speaks in names. It is a few judgments and no state of its own:
// which holdings are one thing, what to call it, which to deal with first,
// how close to let it get. The ladder is fixed and not the mind's to change.
// You cannot aim at what you have not named.
//
// Perception is a tool behaviour holds. This package imports mind/perception
// and reads what an actor holds; perception never learns an intent exists.
// That is the whole of the layering, and the arrow points one way (R1).
//
// # The nouns
//
// A [Situation] is everything one actor has to go on: its [Contact] values as
// folded by its own mind, its [Self], and when. Nothing about anyone else
// that did not arrive through a channel.
//
// A [Mind] is four judgments. Judge is which holdings are one thing. Name is
// what the actor calls a contact. Rank is which contact it would rather deal
// with first. Keep is how close it lets things get. A behaviour author writes
// one type.
//
// [Decide] is the ladder, and it is not the mind's to change: step away from
// what is too close, attack a live named creature in reach, walk toward what
// the mind ranks first, flee what it may not approach, else pass. Every rung
// arrived with a use case that paid for it (R7).
//
// # What a payload means
//
// Perception carries payloads it never decodes. Behaviour decodes only one,
// the deeds channel it wrote itself. Everything else is read through the
// caller's [Reader], the way perception's physics is the caller's Reach:
// this package holds no vocabulary for what a sighting says (R2).
//
// # Inputs and outputs
//
// Every function that takes more than one thing takes one Input; every
// function that answers more than one thing answers one Output, and an error.
//
// Design contract: docs/ideas/mind/behavior/design.md (R1–R13). Composition
// module: depends on core and mind/perception.
package behavior
