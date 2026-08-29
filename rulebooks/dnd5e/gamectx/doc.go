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
//   - [WithReactionReadiness] / [IsReactionReady] — who is ready to spend a
//     reaction (free reactions are default-ready for everyone; costed ones
//     require opting in — see below).
//
// Resolution installs all three unconditionally on every resolve path — the
// room first, the cast beside it once the sheets are loaded, and a default
// readiness map derived from the cast (resolution/resolve.go; the #1252
// wiring landed in #1256, readiness in #1282). There is no "sometimes
// present" mode, because an ambient dependency that is sometimes present is
// exactly what went wrong here before —
// TestNoCodePathProducesACastlessInteraction and its room and readiness
// siblings hold that structurally rather than by example.
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
// # Reaction readiness fails closed, and that is the point
//
// [IsReactionReady] returns false when no map is installed, or when the map is
// silent about a member or a reaction. It survived the deletion above because
// that is not the same bug: not-ready is the CORRECT answer for a reaction
// nobody has opted into, and the code says so at the point of failure rather
// than leaving it to be discovered. Since rpg-toolkit#1282, resolution
// installs a default map on every path — free reactions readied for everyone,
// costed ones absent from the set (the Wave 2.11d ruling reused) — so the
// opportunity attack fires, while Shield keeps declining on purpose until
// opting into a costed reaction is something a player can be asked.
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
// An effect's OWN sheet comes from here too, and it comes as a question rather
// than as a thing. It looks itself up in the cast by its own ID and gets back
// a [github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat.Member] — the
// same surface it would get for the creature standing next to it, with no
// method on it that writes.
//
// That is the read law and the write law in one type. Reads come from the
// cast; a write is a request published on the bus, applied by whoever keeps
// the sheet. What ambient delivery of an OWNED thing would mean —
// rpg-toolkit#1178's walk-back, and the reason this paragraph used to say the
// opposite — is delivery of the sheet itself, mutable, to code that does not
// own it. A read surface is not that.
//
// The handle it replaced was events.OwnerAware, and it is gone
// (rpg-project#319, Phase 6). A condition reaches a bus three ways, and the
// handle was wired on ONE of them: the load-and-attach path, by
// character.Attach and monstertraits.AttachMonster. Draft.Finalize reaches
// the bus through SheetKeeper.subscribeSelf and never enters that loop, and
// each keeper's own runtime handler — the ordinary "a condition was applied
// mid-fight" path — calls Apply with no handoff at all. So the handle was
// structurally absent for every condition applied during play. That is the
// argument against it and not merely against its wiring: a mechanism
// installed on one path of three is a coincidence, not a guarantee.
//
// The four conditions that held one held it to WRITE — a dirty mark, a
// reaction spend — and every one of those is a published request now, applied
// by the keeper that owns the sheet.
package gamectx
