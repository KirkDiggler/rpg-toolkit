// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package machine is a state machine over a bag that fires configured events
// on transition.
//
// A condition such as Rage is a machine: explicit states, transitions that
// fire on beats under guards, trackers that note what happened, and one bag
// of state that the parts share and the condition persists. This package
// runs that machine. It knows nothing about chains, outputs or rulebooks:
// the composer (package effects) attaches what a state contributes, and the
// rulebook supplies the words a recipe may use. The design is design.md on
// the idea branch docs/idea-composable-conditions, in
// https://github.com/KirkDiggler/rpg-toolkit/tree/docs/idea-composable-conditions/docs/ideas/composable-conditions
//
// The pieces below are in the order a reader meets them in a recipe. The
// examples quote the Rage recipe the tests carry.
//
// # Recipe
//
// A Recipe is one condition type as data: its Ref, its Stacks identity, its
// Initial state, its declared Bag, its Trackers and its States. Every field
// is a value, so the Go literal and the JSON object are the same shape and
// round-trip to each other. A recipe is configuration. One recipe on one
// holder, with its bag filled in, is an instance: a Machine.
//
//	Ref: dnd5e:conditions:raging, Stacks: by_ref, Initial: "fresh"
//
// # Bag and Blob
//
// The Bag holds the keys a recipe declares, each with its zero value, and
// only bool, int and string values. Trackers and acts write it, and guards
// read it. The bag is also what the condition saves. Blob is that saved
// form: ref, member_id, source and state, with the bag's keys beside them in
// one flat JSON object. The bag is placed there by the `json:",embed"` tag,
// which is how Go 1.27's encoding/json/v2 spells an inline field. Because
// the bag is flat, today's Rage keys stay where consumers read them. The web
// client looks for turns_active at the top level, and it is there.
//
//	Bag: was_hit_this_turn, did_attack_this_turn, saw_turn_end (false);
//	     round_activated, turns_active (0)
//
// Whole JSON numbers decode as int, so a loaded bag compares equal to its Go
// literal. Program.NewBag fills an instance's bag from the applier's
// parameters, and refuses by name a parameter that names no declared key.
//
// # State and Terminal
//
// A State carries its Transitions and its Outputs. Outputs are opaque JSON:
// the machine stores them and never reads them, because they belong to the
// composer. A Terminal state is the one way out. Entering it ends the
// condition, after which the machine detaches.
//
//	"fresh" and "raging" carry Rage's four outputs; "ended" is Terminal.
//
// # Transition
//
// A Transition listens to one beat (On), when the event is about the holder
// (About). If its guard (When) applies, it runs its acts (Do) on the bag and
// enters the state named by To. An empty To means stay in the current state,
// and the acts still run. Reason is required when To is terminal, and it
// travels on the Transitioned event. On each beat the current state's
// transitions run in recipe order, and the first that fires wins. That
// order is how Rage checks for combat activity before it checks its
// duration.
//
//	raging, on turn_end: not(any(was_hit, did_attack)) -> ended "no_combat_activity"
//	                     rounds since anchor >= 9      -> ended "duration_expired"
//	                     round went backwards          -> ended "clock_reset"
//	                     otherwise stay: anchor_round, derive_turns_active, clear_flags
//
// # Tracker
//
// A Tracker sets one bool bag key when its beat fires about the holder. Its
// About selector names which id on the event must equal the holder, such as
// the attacker or the target. A tracker is idempotent within a turn: a flag
// that is already set stays set and reports no change.
//
//	post_attack_roll, holder_is_actor  -> did_attack_this_turn
//	damage_received,  holder_is_target -> was_hit_this_turn
//
// # Act
//
// An Act is one bag write that a transition performs. Exactly one field is
// set.
//
//   - anchor_round sets saw_turn_end true. It sets round_activated to the
//     beat's round only while round_activated is 0 and the beat carries a
//     round above zero, so a late anchor is possible and a re-anchor is not.
//   - derive_turns_active sets turns_active to round - anchor + 1 when the
//     bag is anchored and the round is at or after the anchor. The count is
//     derived, never incremented, so a missed turn end cannot skew it.
//   - clear_flags sets the named bool keys back to false, which is the
//     turn boundary for trackers.
//   - set writes one declared key.
//
// # Guard and its leaves
//
// A Guard is a tree with exactly one field set per node: All, Any or Not
// over child guards, or one leaf.
//
//   - fact compares a vocabulary fact on the frame or event with an Op.
//     Rage's damage bonus reads action.melee, action.weapon_pool and
//     action.ability.
//   - flag reads one bool in the bag.
//   - rounds compares the beat's round with the bag's anchor, either as
//     since_anchor (round - anchor, with an op) or as went_backwards.
//   - class_level compares an int fact named by the class with an op.
//
// The ops are eq, which works on bool, int and string, and gte, which works
// on int only. No other op exists until a recipe needs it.
//
// # Verdict: three-valued, by construction
//
// CompiledGuard.Answer returns Applies, DoesNotApply or Depends, and names
// the leaf that decided. Depends means unknown, never false. It comes from a
// fact the reader cannot see, a rounds leaf on a beat that carries no round,
// or a bag that is not yet anchored. A transition whose guard depends does
// not fire. An output whose guard depends cannot answer, and the composer
// reports that as a Depends row.
//
// all is DoesNotApply if any child is. Otherwise it is Depends if any child
// is, and Applies only when every child applies.
//
// any is Applies if any child is. Otherwise it is Depends if any child is,
// and DoesNotApply only when every child does not apply.
//
// not swaps Applies and DoesNotApply and leaves Depends as Depends.
//
// # Vocabulary
//
// The tool owns the grammar and the rulebook owns the words. A Vocabulary
// is the rulebook's plug. Facts maps each FactName to a FactSpec: a Kind
// (bool, int or string), and a Read function that returns the value from an
// opaque frame or event, plus whether it is known. Beats maps each Beat to a
// BeatSpec with four parts:
//
//   - Subscribe attaches a handler to the beat's typed topic.
//   - Round returns the beat's round, or nil for a beat that never carries one.
//   - About returns the id that a selector names on the event.
//   - Selectors lists the About values the beat supports.
//
// The machine never imports a rulebook, so every name a recipe uses has to
// come in this way. A word the vocabulary lacks is a finding, added with its
// test. It is never worked around.
//
// # Validate and Compile
//
// Validate runs when a recipe is loaded, not when it is played, and returns
// the first refusal by name, wrapping ErrInvalidRecipe. It refuses:
//
//   - a fact, beat or selector that is not in the vocabulary;
//   - a flag or a written key that the bag does not declare, or declares
//     with the wrong kind;
//   - a To that names no state, or an initial state that is terminal;
//   - a state that no transition reaches;
//   - a transition into a terminal state without a reason;
//   - a guard node, rounds leaf or act that sets other than exactly one field;
//   - a fact value or op that does not fit the fact's kind;
//   - a rounds leaf on a beat that carries no round;
//   - by_source stacking without a source_id key;
//   - a bag key that collides with the blob header;
//   - a bag key that nothing reads.
//
// The composer names the keys its outputs read as readElsewhere, because
// the machine cannot see inside outputs. Compile validates in the same way
// and binds the result into a Program, with fact readers and beat specs
// resolved once.
//
// # Machine lifecycle
//
// New makes a Machine for one holder from a Program, a bag and a state. An
// empty state means the initial state.
//
//   - Apply subscribes each tracker, plus one handler per beat that the
//     recipe's transitions name. If any subscription fails, it undoes the
//     ones already made.
//   - On each beat, the machine runs the current state's transitions in
//     recipe order. A transition that is not about the holder, or whose
//     guard does not apply, is skipped. The first that fires runs its acts
//     and may enter a new state.
//   - Entering a state fires Transitioned{From, To, Reason} to every
//     OnTransition listener. On a terminal state all listeners run first, so
//     the composer can publish the removal while still attached, and then
//     the machine detaches itself.
//   - OnChanged listeners hear about bag changes made by trackers and by
//     transitions that stay. The composer uses this to publish a state
//     change.
//   - Remove unsubscribes everything. ToBlob returns what to persist.
//
// # What the machine does not know
//
// The machine knows no chain, no output and no removal topic. It does not
// know what a condition contributes, how a removal is announced, or which
// rulebook it serves. Those belong to the composer and its OutputVocabulary,
// and to the rulebook that plugs both in.
//
// Contract (design E9): this package imports only the standard library,
// core, core/chain and events, and never a rulebook. TestImportsStayInsideTheSeam
// fails on any other import.
package machine
