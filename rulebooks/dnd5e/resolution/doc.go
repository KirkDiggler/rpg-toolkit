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
// Today is one inversion short of that. [Resolve] delegates each
// participant's load to character.LoadFromData and the monster three-call
// assembly, which still take a bus and run the condition loop themselves;
// the EffectScoper seam (#982) is how per-effect attribution survives the
// delegation. What actually keeps the bus in those signatures is not the
// conditions — it is the entity's own machinery: a character subscribes five
// handlers for itself (conditions applied to it, healing received, actions
// granted) and mutates its sheet when those events land. That is the sheet
// reacting to the world, which is real behaviour — it is just behaviour that
// belongs in an attachable of its own, attached and attributed like any
// effect, rather than wired invisibly inside a constructor.
//
// The remaining migration is therefore its own PR, not a side effect of any
// machine landing: the entity loaders drop their bus parameter and become
// data → sheet; this package runs the ref → loader → Apply loop itself; the
// self-subscriptions extract into a sheet-keeper attachable. Until then,
// those self-subscriptions are the zero-Ref entries in the registration
// list.
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
