// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package gamectx carries what is TRUE about an interaction, for effects that
// have to ask.
//
// An event says what is HAPPENING — who swung at whom, with what, for how
// much. That is the bus, and it is the channel effects already had. This
// package is the other one: where everybody is standing, who they are to each
// other, what they are. Things no single participant owns, scoped to one
// interaction, installed by whoever raised it.
//
// # What lives here, and why so little
//
//   - [WithRoom] / [Room] — the world the interaction happens on.
//   - [WithCast] / [CastOf] — the participants in it.
//   - [WithReactionReadiness] / [IsReactionReady] — who has opted in to spend
//     a reaction.
//
// Resolution installs [WithRoom] on every path today. [WithCast] is defined
// here but NOT yet installed anywhere — that wiring is rpg-toolkit#1252 — so
// [CastOf] currently returns ok=false and every consumer takes its "cannot
// answer" branch. When it lands it goes in unconditionally beside the room,
// because an ambient dependency that is sometimes present is exactly what went
// wrong here before.
//
// # What used to live here
//
// Five installers, one of them installed. GameContext/CharacterRegistry,
// CombatantRegistry, and CombatState were all defined against an imagined
// need, and combat.CombatantLookup was a sixth mechanism answering the same
// question from a different package with its own context key. Between them
// they had zero installs.
//
// That was not harmless. Three conditions READ CharacterRegistry —
// UnarmoredDefense, MartialArts, UnarmoredMovement — and returned its
// "no registry" error straight into a chain fold. Character.EffectiveAC
// swallows fold errors, so a barbarian with Unarmored Defense attached fought
// at 10+DEX, and every other contributor to that AC was dropped along with it.
// Nothing was logged, and every test of those conditions passed, because every
// test installed a registry by hand that production never installed.
//
// The lesson is not that ambient state is bad. The room is ambient and is the
// healthiest thing in this package. It is that an ambient dependency must be
// MANDATORY and SINGULAR, and its absent value must say what the author meant.
//
// # Reaction readiness is the counter-example, and it stays
//
// [IsReactionReady] returns false when no map is installed, and no map is ever
// installed today. It survived the deletion above because that is not the same
// bug: not-ready is the CORRECT answer while nothing in the stack lets a
// player ready a reaction, and the code says so at the point of failure rather
// than leaving it to be discovered. Opportunity Attack and Shield decline to
// fire, on purpose, and will keep declining until reactions are built.
//
// Fail-closed by design is not fail-silent by accident. The test for an
// ambient dependency is not "is it installed" — it is "does its absent value
// say what the author meant".
//
// # Reading anything here
//
// Defensively, and never as an error. Accessors return (value, ok); an effect
// that cannot answer its question leaves the chain unchanged. See
// conditions/prone.go's attackerIsWithinReach for the shape: "not within
// reach" and "nobody knows where these two are standing" are different
// answers, and a rule that conflates them is a rule invented out of missing
// data.
//
// An effect's OWN sheet does not come from here. That arrives by injection at
// attach time — see [github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events.OwnerAware]
// — because it has an owner, and ambient delivery of an owned thing is what
// rpg-toolkit#1178 already walked back once.
package gamectx
