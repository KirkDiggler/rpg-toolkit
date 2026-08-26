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
// [Input.Cost] is what an interaction costs the actor who declared it.
// [Machine.Start] runs first as pure preflight: it may only read and validate
// attached sheets and produce the first step; it must not roll, spend, publish,
// or mutate. The package charges the cost only after Start succeeds and before
// it drives that first step. A resolution nobody can pay for therefore executes
// no step and writes nothing; a nil cost is a free action, which is what every
// Resolve was before the door existed.
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
// from, which is rpg-toolkit#1095. Recorded here so nobody reads "the runner
// spends" as a compiler-checked claim it is not.
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
// Two game-context installers are populated, both unconditionally.
//
// This paragraph used to say "No game context is installed", and describe five
// gamectx registries waiting for a predicate to force them: the first predicate
// that needs one brings its installer with it, and populating a registry
// nothing reads would be building the wrong thing convincingly. The policy was
// right. It was not enforced, and the failure was invisible in exactly the
// direction nobody was looking.
//
// FOUR predicates arrived without their installer. UnarmoredDefense,
// MartialArts twice, and UnarmoredMovement all read
// gamectx.RequireCharacters — a registry with zero non-test call sites in the
// whole toolkit — and returned its "no game context" error into a chain fold.
// [character.Character.EffectiveAC] swallows fold errors, so a barbarian with
// Unarmored Defense attached was struck at 10+DEX in every real fight, every
// other contributor to that AC was discarded along with the one that failed,
// and nothing was logged. Every test of those rules passed throughout, because
// every one of them installed a registry by hand that production never
// installed (rpg-toolkit#1251).
//
// So the mirror of the warning above is the one that actually bit: defining
// registries nothing populates is the same mistake as populating registries
// nothing reads, and it fails silently instead of loudly.
//
// What is installed now, on every path and inside no condition:
//
//   - [gamectx.WithRoom] — the world the interaction happens on. ONE world,
//     installed EVERY time, and it is THE world rather than a copy of it. The
//     composition compiles the authored chambers into a single canvas in a
//     single absolute frame and hands that room over to be read
//     (rpg-toolkit#1105, rpg-toolkit#1114), so "which room describes this
//     interaction" stopped being a question anybody has to answer. It used to
//     be one, and the answer this package gave when the cast spanned two rooms
//     — install nothing — silently switched off every predicate that reads
//     positions the moment a party member wandered off, which in a dungeon is
//     most of the time (rpg-toolkit#1090).
//   - [gamectx.WithCast] — who the participants are to each other. This
//     package is the only one that can answer it: R3 passes everyone in, so
//     this is the only place that holds them all. See [castView].
//
// TestNoCodePathProducesARoomlessInteraction and
// TestNoCodePathProducesACastlessInteraction hold both structurally rather than
// by example, by reading this file's source. A behavioural suite cannot make
// the claim: the defect is that tests supply what production does not, so the
// tests are the last place it shows up.
//
// This package builds no geometry of its own. It held a room it assembled out
// of the encounter's persisted description, carrying a second copy of grid
// construction and no walls whatsoever; what the rules read and what the
// composition enforces are now the same object, which is a stronger guarantee
// than any test over two of them could be.
//
// The registries that were never populated are gone rather than waiting:
// CharacterRegistry (character-shaped, so it could not describe a monster at
// all), CombatantRegistry, CombatState, and combat.CombatantLookup, a sixth
// mechanism answering the same question from another package with its own
// context key. gamectx.ReactionReadiness stays — it has two live readers and
// its absent value is correct on purpose, which is a different thing from an
// installer nobody wired.
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
