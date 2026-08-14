// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package resolution is the one place an event bus exists.
//
// A bus is created for a single interaction, everything passed in is attached
// to it, the interaction runs, and the bus dies with the call. Nothing above
// or below this package holds one: the encounter composition raises
// interactions and stays bus-free, rules packages are step machines over data,
// and the host seam names IDs. That is [ADR-0038], and this package is its
// implementation.
//
// # What a caller does
//
// Hand [Resolve] the world as data, every participant's sheet as data, and a
// machine describing the interaction. Get back the world as data, whichever
// participants came out dirty, the outcome, and the list of every hook that
// was attached on the way.
//
//	out, err := resolution.Resolve(ctx, &resolution.Input{
//	    World:        worldData,
//	    Participants: sheets,
//	    Machine:      resolution.NewSave(&resolution.SaveInput{...}),
//	})
//
// # The laws this package is bound by
//
//   - **R1 — One bus per interaction, shared by every participant in it.**
//     Shared rather than per-participant because an effect on one member must
//     be able to observe what happens to another; that is the whole reason a
//     bus exists here, and the prerequisite for reactions. A bus per
//     participant would compile, pass any single-participant test, and quietly
//     make cross-participant observation impossible.
//   - **R2 — Everything at the seam is data.** No runtime object crosses into
//     or out of [Resolve]. What goes in is what a repository holds; what comes
//     back is what a repository can store.
//   - **R3 — Pass everyone in.** Scope is the caller's business; applicability
//     is the effect's own predicate. Attaching a participant who turns out to
//     be irrelevant costs correctness nothing, and deciding relevance out here
//     would put a rule in the wiring. Room-scoping is an optimization, never a
//     correctness rule.
//   - **R4 — Attach in sorted participant order.** Two resolutions over
//     identical data must produce an identical registration list, or a
//     suspension cannot be resumed into the same world it left.
//   - **R5 — Resolution tears down every subscription it granted**, and trusts
//     no effect to clean up after itself.
//   - **R6 — A machine never sees the bus.** It yields steps; this package
//     folds chains on its own bus and hands back results. A machine that could
//     reach the bus could subscribe during its own resolution, and the
//     ordering of that is not something anyone should have to reason about.
//   - **R7 — Never call Cleanup on a participant.** [character.Character]'s
//     first statement there is to nil its conditions, and ToData serializes
//     them, so cleaning up before the snapshot persists a participant with no
//     conditions — silently. It buys nothing when the bus dies with the call
//     anyway.
//
// # Where this sits on the migration
//
// [ADR-0038]'s end state is one sentence of its Consequences: combat and
// character shed infrastructure and become what they are — rules vocabulary
// and data. In that world a character is a pure sheet. A condition is not
// part of it: the sheet merely carries the condition's persisted JSON, and
// the bus belongs to whoever runs the attach loop — route each blob's ref to
// its loader, call Apply. That loop is free-standing, and this package is
// where it ends up.
//
// That inversion has landed. [Resolve] loads each participant purely —
// character.Load, monstertraits.LoadMonster: data → sheet, no bus in the call —
// and then attaches it to the bus this package made, through
// character.Attach and monstertraits.AttachMonster. Each of those runs the
// loop this package used to delegate: the sheet's own keeper first, then every
// persisted effect through a view stamped with the ref its loader routed on.
// The entity's own machinery — the handlers a character keeps for itself, the
// two a monster does, the recoverable resources — is a sheet-keeper attachable,
// so it appears in the registration list under the participant that made it
// instead of as the zero-Ref entries this paragraph used to describe.
//
// Two behaviours ride in with it. Loading is strict: a persisted effect blob
// this build cannot route fails the resolution and names the blob, where the
// legacy path logged and carried on — and since [Resolve] hands sheets back to
// be persisted, an effect dropped on the way in is an effect deleted on the way
// out. And a failed attach is a no-op: whatever attached before the failure
// comes back off, by the entity's contract and again by this package's teardown
// on the error path, so a refused resolution leaves nothing on a bus and no
// half-written sheet.
//
// What remains is retirement rather than migration. character.LoadFromData and
// the monster three-call assembly still exist for their other callers, and a
// sheet still parks a bus for the verb methods that read one. Both go with
// #965 and #966.
//
// # What this package does not do yet
//
// The step vocabulary is [Gather] and [Done], and nothing else. [ADR-0038]
// seals it as Gather | Pose | Request | Done, but that set was derived from a
// table of six machines of which one was implemented; each remaining case
// lands with the caller that forces it — Pose with the walk machine, Request
// with concentration. Sealing an enumeration against hypotheticals is the
// mistake [ADR-0007] exists to remember.
//
// No game context is installed. Effect predicates read world state through
// five separate installers in the gamectx package, and a saving throw's
// predicates read none of them — [conditions.RagingCondition] decides on the
// event alone. Resolution is where they will be populated, because it is the
// one place that holds both the world and the effects; the first predicate
// that needs one brings its installer with it. Populating a registry nothing
// reads would be building the wrong thing convincingly.
//
// The machine for a saving throw lives here rather than in the rules package
// that owns saves, because `rulebooks/dnd5e` cannot import this module — this
// module imports it. When the second machine arrives (a strike, owned by
// `combat`), the step vocabulary has to move somewhere both can see. That is a
// known and deliberate debt, not an oversight.
//
// [ADR-0007]: https://github.com/KirkDiggler/rpg-toolkit/blob/main/docs/adr/0007-generic-trigger-system.md
// [ADR-0038]: https://github.com/KirkDiggler/rpg-toolkit/blob/main/docs/adr/0038-resolution-owns-the-bus.md
package resolution
