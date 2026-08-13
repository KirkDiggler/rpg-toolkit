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
// The host implements repositories and then calls verbs with IDs. It never
// holds a domain object:
//
//	mgr, err := session.NewManager(&session.Config{
//	    Sessions:   sessRepo,
//	    Encounters: encRepo,
//	    Events:     stream, // optional
//	})
//
//	out, err := mgr.Move(ctx, &session.MoveInput{...})
//	// out.Pending != nil  =>  someone owes an answer
//
// Every verb is load, act, save, return. There is no setup call, no teardown,
// and no ordering for the caller to get wrong.
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
// it, surfaces in one shape and resolves through one Answer.
//
// S6 — failure names its pieces. A partial save is an error with a populated
// report, never a silent shrug.
//
// S7 — a frozen resolution is data. It survives a process restart because it
// was never anything else.
//
// S8 — construction is total. The manager refuses to exist without what it
// needs; there is no lazy discovery at call time.
//
// S12 — ports are key-value. Every operation is get-by-id or put-by-id, so the
// host's store can stay a key-value store forever.
//
// S13 — one repository per data type.
package session
