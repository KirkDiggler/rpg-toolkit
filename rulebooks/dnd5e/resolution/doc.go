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
// # The door pays
//
// [Input.Cost] is what an interaction costs the actor who declared it, and this
// package charges it before [Machine.Start] runs. A resolution nobody can pay
// for never starts a machine and writes nothing; a nil cost is a free action,
// which is what every Resolve was before the door existed.
//
// The runner spends and machines yield. That is the same move R6 makes about
// the bus — a machine names what it wants and this package acts — so no machine
// here reads or writes an economy, and a machine cannot tell a swing that cost
// an action from one that cost nothing. What it cost was answered by whoever
// compiled the profile.
//
// It is a discipline rather than R6's guarantee, and the difference is worth
// being exact about. A machine cannot reach the bus BY CONSTRUCTION: [Gather]'s
// workings are unexported and a machine cannot build a step at all. The ledger
// has no such seal — [Participants] hands a machine the same
// [character.Character] values the gate spends from, and the gate is an
// ordinary exported function in a package anyone may import. Sealing it would
// mean changing what a machine is handed rather than where the debit is called
// from. Recorded here so nobody reads "the runner spends" as a compiler-checked
// claim it is not.
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
// # The bus-effect tally
//
// [Gather] is a step resolution runs with the bus in hand — folding a chain,
// first and mainly. Three of the four do exactly that and hand back the folded
// event: the saving throw, the attack chain, the damage chain. The fourth does
// something on the bus and hands back nothing but the next step — imposing a
// contest's consequence. All four are built by constructors here, all four run
// on resolution's bus, and each is named for what it does.
//
// ADR-0026's Notify would have been a second of the latter kind, and is
// deliberately absent — see strikeMachine.afterDamage. Publishing
// DamageReceivedEvent applies the damage a second time to a monster target,
// because that sheet's keeper treats the topic as an instruction, while a
// character has no such handler and the same event is inert. One event meaning
// two things is the classification slice 2 owes, and slice 1 records it rather
// than shipping a double-apply.
//
// Slice 2 settled it: the vocabulary does NOT grow a case. Three folds to one
// pure effect does not justify a fifth step kind, the odd one out is honest
// about itself, and its Name already reads correctly in a step log ("impose the
// prone condition" rather than a fold that folds nothing). Not extending a
// sealed set costs nothing; extending it against one example is the mistake
// [ADR-0007] exists to remember.
//
// The trigger for reopening it is recorded rather than left to memory: if wave
// 5's reactions multiply the pure-effect steps — several suspension windows
// that act on the bus without folding — the three-to-one ratio reverses and the
// argument reverses with it. At that point the question comes back as an ADR
// with the new tally as its evidence. Until then there is nothing to decide.
//
// All three folds are this package's own. The damage chain was the last one
// held elsewhere: slice 1 handed resolution's bus to combat.ResolveDamage,
// because every exported attack and damage entry point in that package required
// one and the multiplier arithmetic was unexported. Slice 2 exported that
// arithmetic bus-free ([combat.FinalDamage]) and moved the fold here, so
// custody now matches where the bus lives. The grep-able worklist that tracked
// it — every call site marked "divestment debt — #965 slice 2" — is empty.
//
// # What this package does not do yet
//
// The step vocabulary is [Gather], [Request] and [Done]. [ADR-0038] seals it as
// Gather | Pose | Request | Done, and each case lands with the caller that
// forces it rather than in advance: Request arrived with the contest machine,
// which needs a saving throw's answer before it knows whether to impose
// anything, and Pose waits for the walk machine. Sealing an enumeration against
// hypotheticals is the mistake [ADR-0007] exists to remember.
//
// [Request] runs its machine to Done inside the requester's own step loop —
// no suspension. That is enough for every consumer today, and the boundary it
// crosses is self-describing data (a machine, and a continuation taking its
// outcome), so the case where the answer comes from outside the process is a
// new step rather than a redesign of this one.
//
// No game context is installed. Effect predicates read world state through
// five separate installers in the gamectx package, and neither a saving throw's
// predicates nor a contest's read any of them — [conditions.RagingCondition]
// decides on the event alone. Resolution is where they will be populated,
// because it is the one place that holds both the world and the effects; the
// first predicate that needs one brings its installer with it. Populating a
// registry nothing reads would be building the wrong thing convincingly.
//
// One installer is now populated, by the machine that forced it. The strike
// folds the attack chain, and prone's predicate reads positions off that chain —
// advantage from within five feet, disadvantage beyond — so [Resolve] builds a
// spatial room out of [Input.World] and installs it for the interaction. The
// world it builds from is the persisted one the caller already handed over:
// members carry their room and position, rooms carry their grid and size. See
// interactionRoom for why one room rather than all of them, and why a
// participant list spanning rooms installs none.
//
// Nothing else is installed. The other four registries stay empty until a
// predicate that reads one arrives with its own consumer.
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
