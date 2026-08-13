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
// The laws below describe the whole design, and two of them are commitments
// rather than descriptions today. Stated as such deliberately: a reader
// deciding whether to depend on this needs to know which is which, and a
// package doc that quietly narrates the destination is worse than one that
// names the waypoint.
//
// In force now: S1, S2, S3, S4, S6, S8, S12, S13, and the event-stream laws.
//
// Not yet exercised: S5 (Pending as suspension vocabulary) and S7 (a frozen
// resolution is data) — nothing suspends in this version. The only pause a
// verb can report is an ending, and it is reported by returning fewer steps
// than were asked for. When resolutions become genuinely suspendable, the
// caller learns through the same channel: a verb that returns having stopped,
// plus a field naming who owes an answer. The loop does not change shape,
// which is why those laws are stated now rather than invented later.
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
// it, surfaces in one shape and resolves through one Answer. (Committed, not
// yet exercised: see above.)
//
// S6 — failure names its pieces. A partial save is an error with a populated
// report, never a silent shrug.
//
// S7 — a frozen resolution is data. It survives a process restart because it
// was never anything else. (Committed, not yet exercised: see above.)
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
