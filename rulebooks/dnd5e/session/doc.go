// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package session is the game server's single point of contact with the
// toolkit: a manager that composes composers.
//
// The encounter composition is the world — geometry, placement, perception,
// story, endings. This package is the table: it holds what is not the world,
// wires the pieces together, and gives the host one verb surface to call.
//
// # The loop
//
// The host implements a repository or two and then calls verbs with IDs. It
// never holds a domain object:
//
//	mgr, err := session.NewManager(&session.Config{
//	    Sessions:   sessRepo,
//	    Encounters: encRepo,
//	    Events:     stream,
//	})
//
//	out, err := mgr.Move(ctx, &session.MoveInput{Session: s, Member: m, Path: p})
//	// len(out.Steps) < len(p)  =>  something stopped the walk
//	// out.Outcome != nil       =>  and that something was an ending
//
// Every verb is load, act, save, return. There is no setup call, no teardown,
// and no ordering for the caller to get wrong.
//
// # What this version does and does not do
//
// Every law below is in force and exercised. S5 and S7 were commitments in
// v0.1.0 — nothing could suspend yet — and are descriptions as of v0.2.0: a
// walk stops mid-path, freezes as data, survives a process restart, and
// resumes. The loop did not change shape when they became real, which is why
// they were stated at the first tag rather than invented at the second.
//
// What is genuinely absent is scope, not law. Only one thing can suspend a
// resolution today: a member seeing something for the first time. Characters,
// NPCs and their conditions arrive with the entities wave; combat and
// reactions after that. Each is a new checkpoint kind, and a new checkpoint
// kind is additive by construction — Prompt names what the player is looking
// at and never why the resolution stopped, so no client learns a reason code.
//
// # Laws
//
// S1 — the manager holds no domain state. Each verb loads what it needs, acts,
// saves, and drops everything. The game is turn-based; this costs little and
// removes an entire class of stale-in-memory bugs, with no sticky sessions.
//
// S2 — no inner type crosses the boundary. Exported signatures reference types
// owned here plus stable value types (spatial.Position). Never an encounter,
// combat, clock, intel, record or interrupt type. This is what allows the
// modules underneath to be replaced without the host changing a line, and it
// is enforced by a test rather than by good intentions.
//
// S3 — repositories trade in data, not domain objects. Hydration happens here,
// where the laws are; the host stays storage.
//
// S4 — every verb is load, act, save, return.
//
// S5 — Pending is the only suspension vocabulary. Every pause, whatever caused
// it, surfaces in one shape and resolves through one Answer. While a window is
// open every verb that would change the world is refused; read verbs are not,
// because what is frozen is change, not observation.
//
// S6 — failure names its pieces. A partial save is an error with a populated
// report, never a silent shrug.
//
// S7 — a frozen resolution is data. The walk is a re-enterable phase machine
// holding nothing across a suspension, so a window survives the process that
// opened it: the answer may arrive days later, on another machine.
//
// S8 — construction is total. The manager refuses to exist without what it
// needs; there is no lazy discovery at call time.
//
// S12 — repositories are key-value. Every operation is get-by-id or put-by-id,
// so the host's store can stay a key-value store forever.
//
// The repositories point OUTWARD: this package calls them, the host implements
// them. That inversion is what lets storage be swapped, mocked, or held in
// memory without this package knowing, and it is the only structural idea here
// worth naming.
//
// S13 — one repository per data type.
package session
