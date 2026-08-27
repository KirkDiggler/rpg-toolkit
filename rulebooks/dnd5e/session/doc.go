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
//	    Characters: charRepo,
//	    Events:     stream,
//	    Dice:       roller,
//	})
//
//	out, err := mgr.Move(ctx, &session.MoveInput{Session: s, Member: m, Path: p})
//	// len(out.Steps) < len(p)  =>  something stopped the walk
//	// out.Outcome != nil       =>  and that something was an ending
//	// out.Formed  != nil       =>  and that something was a fight starting
//
// Every verb is load, act, save, return. There is no setup call, no teardown,
// and no ordering for the caller to get wrong.
//
// # What this package does not hold
//
// It holds no game rules. That is the charter, and it is checkable: read every
// file here and there is no die, no modifier, no threshold, and no decision
// about what any of it means. Verbs load, hand the world what the caller asked
// for, and report what the world says happened.
//
// It got there by giving one away. This package used to decide when an
// encounter begins: a walk read each step's perception delta, and a subject
// seen for the first time stopped the walk and opened a window for the walker
// to answer. That is a rule — and a wrong one twice over, since sight is not
// the only way a fight starts and a walker is not the only one who can see.
// The composition owns it now (rpg-toolkit#964), detecting contact wherever
// sight changes and starting the fight itself; the walk reads the result as
// news and stops because the walker is IN a fight.
//
// Dice are the same principle at the other end. A fight that starts by itself
// needs an order, so a host supplies the randomness (Config.Dice) and the
// RULEBOOK supplies the rule that turns it into an order. Neither the host nor
// this package writes initiative.
//
// # The one question this package answers for the world
//
// The composition asks who is standing, and this package is what answers
// (rpg-toolkit#1079). That looks at first like the charter breaking, and it is
// worth reading closely, because it is the shape every future capability will
// have.
//
// The composition cannot hold hit points — its go.mod cannot import the
// rulebook, by law — so defeat is a fact it can only be TOLD. It takes a
// capability: member IDs in, member IDs out. Something has to implement that,
// and the only layer that both knows what a sheet is and holds every sheet for
// the call in progress is this one.
//
// WHAT THIS PACKAGE CONTRIBUTES IS THE LOOKUP, NOT THE RULE. standing.go finds
// sheets — a monster's in the session record, a character's behind the host's
// repository — and hands each one to combat.IsDown, which is the rulebook's
// answer and the only place a hit point is ever compared to anything. When a
// monster's Undead Fortitude makes zero hit points survivable
// (rpg-toolkit#977), that function changes and nothing here moves.
//
// Read the file and the charter still holds: no die, no modifier, no threshold.
// A capability implemented here is this package doing what it has always done —
// wiring the pieces together — with the rule kept where the rules live.
//
// What it does DECIDE is which verbs a DOWNED member may still drive, and that
// is not a rules question but a seam question: Attack and Move refuse a downed
// actor (ErrDowned), the reads and the recording of outcomes do not.
//
// Downed is this seam's word for zero hit points, and its opposite is up. It is
// deliberately not "down", which also reads as PRONE — a posture condition the
// rulebook tracks and this package never gates on (Kirk's ruling,
// rpg-toolkit#1084). The composition underneath keeps its own "down", which is
// persisted in every stored world; kindOf is where the two meet. See ErrDowned
// and EventDowned.
//
// # Compiled declarations are the execution trust boundary
//
// Afford is the one current action surface on the turn clock. It compiles an
// Attack, Move and EndTurn offer plus one Activate offer per thing the member
// carries, projects only seam-owned values, and gives each compiled offer an
// opaque deterministic selector. Attack and EndTurn
// require that ID back; Move requires it on the turn clock and requires it
// empty on the world clock, where Afford deliberately returns no declarations.
//
// A mutating verb reloads current state, regenerates only its own offer, and
// selects the echoed ID before touching dice, movement, storage, or story.
// Turn-clock Move therefore never inherits Attack pricing, target-view, or
// participant-cast dependencies; Afford still compiles all verbs from one actor
// snapshot. Unknown,
// mismatched, now-unavailable, and target-invalid selection is
// ErrStaleDeclaration; omission is ErrNoDeclarationID. IDs are selectors, not
// authorization or idempotency tokens: the host still binds the acting member
// to its authenticated caller.
//
// The compiled object never crosses S2. For Attack it privately carries the
// complete priced definition, matching resolution cost/readied payer, shared
// candidate preflight, and one raw participant-data cast. Compilation strictly
// exercises that cast through the same public character/monster load-and-attach
// APIs resolution uses. Execution reuses those exact raw participant values and
// never refetches a participant after selection. EndTurn is intentionally
// clock-only; it does not inherit Attack/Move sheet, standing, or economy gates.
//
// # The suspension spine, and where it went
//
// S5 and S7 were laws here through v0.2.0: Pending was the one suspension
// vocabulary, a frozen resolution was data, and an open window froze every
// change verb until it was answered. They are gone, because their only
// producer was the rule above. Nothing opened a window once the walk stopped
// posing one, and a spine with no producer is a shape no caller can reach:
// Pending always empty, Answer always refusing, a freeze branch no test can
// enter.
//
// NOTHING ABOUT THE CUSTODY DESIGN WAS LOST. It lives in play/interrupt, which
// this wave did not touch by a single byte and whose own suite is that design's
// home: pose, audience, options, one-answer-per-window, and the ledger's
// persisted shape are all tested there, against the module that owns them.
// What retired was this package's ADAPTER to it — roughly 250 lines of walk-
// shaped plumbing.
//
// Wave 5 brings the first honest producer (a reaction: a real checkpoint that
// is not a perception rule) and re-creates four things, each of which existed
// and worked. The reference implementation is this module's git history at the
// rpg-toolkit#964 slice-2 commit, where all four are readable in full:
//
//   - the ledger load in openForWrite, and its reject-never-crash handling of a
//     stored ledger no version of this module wrote;
//   - the verb classification for the freeze — which verbs are refused while a
//     window is open and which are not, with reads deliberately exempt;
//   - restart survival: the frozen value written to the session aggregate, the
//     encounter-then-session write order, and the partial-save report;
//   - an Answer path, which will NOT be the old one — a reaction's window has a
//     different audience, different options and a different payload, so the
//     resume half was going to be rewritten whichever way this went.
//
// # Laws
//
// S1 — the manager holds no domain state. Each verb loads what it needs, acts,
// saves, and drops everything. The game is turn-based; this costs little and
// removes an entire class of stale-in-memory bugs, with no sticky sessions.
//
// S2 — no inner type crosses the boundary. Exported signatures reference types
// owned here plus stable value types (spatial.Position). Never an encounter,
// combat, clock, intel or record type. This is what allows the
// modules underneath to be replaced without the host changing a line, and it
// is enforced by a test rather than by good intentions.
//
// S3 — repositories trade in data, not domain objects. Hydration happens here,
// where the laws are; the host stays storage.
//
// S4 — every verb is load, act, save, return.
//
// S5 and S7 — retired with the spine above, not repealed. When a resolution can
// suspend again they are the laws it suspends under.
//
// S6 — failure names its pieces. A partial save is an error with a populated
// report, never a silent shrug.
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
