// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package graph holds the world's structure and derives its present.
//
// Authors declare two things and never a third: structure — entities, typed
// edges, and roles as slots — and derivation, the folds that turn journal
// facts into present state. They never declare a method. There is no code
// path here for "the leader was assassinated" or "the camp was talked round";
// there are generic reducers and projections, parameterised by data, and the
// paths fall out of them.
//
// # Nothing present is stored
//
// A [World] holds only what was declared. Present state does not live in it and
// cannot be written to it. [World.StateFor] computes a [State] fresh from the
// declaration plus the facts an observer witnessed, every time it is called.
// Roll the journal back and the state rolls back with it, because the state is
// a pure function of the two.
//
// # The present is somebody's present
//
// [World.StateFor] takes an observer, and that is the whole knowledge model.
// The camp's present is the fold over what the camp witnessed; a lieutenant who
// saw one extra fact folds to a different present and acts on it. Audience
// reaches members through the declared membership relation, so a fact the camp
// witnessed is witnessed by everyone in the camp — group grain by default,
// individual grain when a fact is audienced to one member alone.
//
// [World.Truth] folds every fact regardless of audience. It is for tests and
// for a game master's seat, never for deciding how someone behaves.
//
// # Concealment has two grains
//
// An [Entity] or [Edge] may be declared [Entity.Concealed] or
// [Edge.Concealed]: hidden from [World.StateFor] until something says
// otherwise, present in [World.Truth] always. Two declarations say otherwise,
// on two different grains:
//
//   - [Pierce] folds on the audience grain, exactly like a reducer: the
//     observer whose own witnessed facts include the piercing fact sees the
//     structure, and nobody else does — a search that finds a hidden door
//     tells the searcher, and only the searcher.
//   - [Reveal] folds on the truth grain: once any fact has ever satisfied it,
//     every observer sees the structure from then on, including one with no
//     witnessed facts about it at all. Perceiving present state is not
//     witnessing past events, so a party that arrives after a door was
//     opened sees an open door.
//
// [State.Visible] answers for an entity; [State.HasEdge] and [State.Edges]
// already answer for an edge, concealment-aware without a second method.
//
// # The two stages of a fold
//
// Reducers run first, once per witnessed fact in Seq order: [Occupy] and
// [Vacate] move slot occupancy, [Count] moves a counter, [Raise] sets a flag.
// Projections run second, over the folded state in declared order: [FollowSlot]
// makes a group's stance follow whoever holds its slot, [AdoptStance] moves a
// flagged entity to another's stance, [Threshold] turns a counter crossing into
// an edge change, [Retire] strips edges from an entity carrying a flag,
// [Settle] rewrites both directions of a pair while one of them carries a
// flag, and [Label] names a derived posture.
//
// The split is what lets belief work. A reducer only knows what it witnessed;
// a projection only knows the folded result. Neither can consult the truth.
package graph
