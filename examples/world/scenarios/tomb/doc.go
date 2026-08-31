// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package tomb is UC-4: a boss room over a loot chest, a hidden room behind a
// secret door, and one artifact that two different routes can recover.
//
// It is the first scenario authored directly against the published `world`
// module rather than rewired onto it after the fact — content written exactly
// as a rulebook would write it. Its [Config] is the point: place the
// artifact, name the captain, set the door's two checks, and [New] refuses
// every absence in words meant for whoever is filling the form in, not for a
// debugger reading a stack trace.
//
// # The door is concealed, not gated
//
// The passage from the boss room to the hidden room is a plain declared
// [graph.Edge], real from the moment the world exists — and, along with the
// hidden room and the artifact inside it, declared [graph.Edge.Concealed].
// The kernel's own two-grain concealment (rpg-toolkit#1342) is what a
// stranger's structural view answers with now: a [graph.Pierce] un-hides it
// for whoever's own witnessed facts include a successful search or the
// captain's defeat, and a [graph.Reveal] un-hides it for everyone, arrival
// order be damned, once a knower gets the door open. [Knows] is a one-line
// wrapper over [graph.State.Visible] — this used to be a hand-rolled journal
// query before the kernel could answer the question itself.
//
// # Two writers of one fact
//
// [FactLocationKnown] can be written two ways, and both are declared as
// content, not as special cases in this package's logic:
//
//   - Defeating the captain writes it audienced to whoever was there. Their
//     knowledge becomes loot, and the boss-room chest is the fight's own
//     separate reward — a door-finder who never fights still has a reason to.
//   - The [Search] verb, margin-banded like any other attempt in this kernel,
//     writes the identical fact audienced to the searcher alone on success. A
//     failed search writes [FactSearchFailed] and reveals nothing; the world
//     never rewinds an attempt that did not pay off.
//
// Knowing is not entering. The [Open] verb carries its own check, and only a
// knower who SUCCEEDS it writes [FactDoorOpened] — audienced to whoever is
// there. That broadcast, not the knowing, is what the rest of the party ever
// learns from: a party-mate who never searched and never fought still sees
// nothing until somebody opens the door in front of them.
//
// # No passive detection
//
// Nothing here evaluates on room entry or proximity. [Search] is the one door
// into knowledge, by design — the sight seam this would otherwise reach for
// is reserved for a design round of its own.
package tomb
