# Session SDK — plan

**Design approved by Kirk 2026-08-12.** This cuts it into waves.

**The cutting rule:** anything that shapes a public type goes in wave 1;
anything purely internal waits. Under the `gorelease` promise the types fixed in
wave 1 are permanent, so the wave with the most trivial code carries the most
irreversible decisions. Everything else can be replaced behind them.

**Module:** `rulebooks/dnd5e/session`, own `go.mod`, own semver, `gorelease`
gate from the first tag.

---

## The waves

| Wave | Ships | Tag | Needs |
|---|---|---|---|
| **0** ✅ | Encounter log retention | `encounter/v0.4.0` | — |
| **1** ✅ | Shell, free-roam verbs, event stream | `session/v0.1.0` | — |
| **2** ✅ | The interrupt spine, proven by the perception pause | `session/v0.2.0` | 1 |
| **3** | Entities on the bus — characters, NPCs, conditions | from `session/v0.3.0` | 1 |
| **4** | Combat | — | 2, 3 |
| **5** | Reactions — opportunity attack (closes #916) | — | 4 |

**The tag column stops predicting after wave 3, which is a correction rather than
an omission.** It assumed one merge per wave. The repo **auto-tags on merge to
main**, deriving the bump from conventional-commit prefixes, so a wave landing in
two steps consumes two versions — wave 3's first step (characters load) took
`session/v0.3.0` by itself.

Nothing real depended on the numbering. Semver here tracks API change, not
project milestones, and the claim that matters — *after the migration wave, no
subsequent wave changes an rpg-api source file* — is about **which** wave migrates
(4, combat), not what number it carries. Writing a version beside a wave was
quietly promising that a wave is one PR. That is not a promise worth making, and
the auto-tagger was never going to keep it.

Practical consequence while planning a wave: a `feat:` commit takes the minor
bump the moment it lands on main, so the version a wave "ships at" is decided by
its first merge, not its last.

Waves 0 and 1 are independent and can run in parallel — different modules, and
the house rule is one in-flight PR per module.

**Wave 1 is drop-in for the current encounter work.** rpg-api imports *zero* of
the composition today (verified: no reference to `rulebooks/dnd5e/encounter`
anywhere in it), so adopting the SDK for free-roam costs them nothing they
already have. Their existing 6,700 lines of old-stack combat orchestration
migrate at wave 4, when the SDK finally covers what they use — that is when the
version-bump promise starts.

---

## Wave 0 — encounter log retention

Small, self-contained, and independently shippable. `TrimBefore` already exists
and `Seq` is explicitly never renumbered by it; the composition simply never
calls it, so every encounter save rewrites the whole story from its first beat.

**T0.1 — retention config, applied on append.** `NewEncounter`/`LoadEncounter`
take a retention setting; the log trims after each append. **Default extremely
small** so full resync is the common path and stays exercised.

**T0.2 — `Story` reports a trimmed range explicitly.** A request below the
retained floor returns a distinguishable "that range is gone, resync" signal,
never a short answer that looks complete.

**Pins:** an encounter that appends past the window retains exactly the window;
`Seq` values are unchanged by trimming; a `FromSeq` below the floor is
rejected as trimmed rather than silently truncated. Mutation: raise the
retention bound by one and the retention pin must fail.

---

## Wave 1 — the shell, free-roam, and the event stream

**Goal:** every public type in the SDK exists and is correct. The logic behind
them is mostly delegation.

**T1.1 — module, `Config`, `NewManager`.** The repositories (`Sessions`,
`Encounters`) plus the required `Events` stream, fail-fast construction naming
what is missing (S8), a distinguishable `NotFound` sentinel, and every
repository get-by-id / put-by-id only (S12).
*Pins:* construction refuses each component individually, by name, in a fixed
order; the full config succeeds.

**T1.2 — `SessionData`, `StartSession`, load/save.** The session aggregate, its
`ToData`/`LoadSession` discipline, the authored encounter handed in as a
parameter rather than fetched (no content repository).
*Pins:* round-trip is byte-identical; a hostile hand-edited blob is rejected,
not crashed (reject-never-crash); an unknown encounter reference is a clean
rejection.

**T1.3 — read verbs.** `View`, `Story`, `Status`, `Atlas` over SDK-owned output
types.
*Pins:* **the S2 boundary test** — a test that fails if any exported signature
in the package references a type from `encounter`, `combat`, `clock`, `intel`,
`record`, or `interrupt`. This is the single most valuable test in the wave: it
is what makes the version-bump promise mechanical rather than aspirational.
Stable value types (`spatial.Position`) are the allowed exception and the test
names them explicitly.

**T1.4 — `Join`, `Exit`, `End`, and `SaveReport`.** Multi-repo writes reporting
what landed and what did not (S6).
*Pins:* a repo that fails on save produces an error *and* a populated report
naming both the written and the failed; the verb never reports success on a
partial write.

**T1.5 — `Move` with a path, and `Traverse`.** The walk loop over the
composition's single-hop `Move`. Stops on ending, stops on rejection.
*Pins:* `len(Steps) < len(Path)` when an ending fires mid-path and the
remaining steps demonstrably never happened (scene 6); a one-cell path is legal;
a path with a non-adjacent jump is rejected before anything moves (R5
atomicity).

**T1.6 — the event stream.** SDK-owned `Event` envelope, proto-shaped: flat,
explicit kinds, nothing polymorphic. Derived from record beats. Published after
the save lands (S9). Publish failure reported but non-fatal (S10). **Projected
per audience, not filtered (S11).**
*Pins:* the scene-7 assertion — one beat, three payloads, and a fourth viewer
who receives nothing; **an observer's payload never contains the actor's
options** (the information-leak pin, and the reason this belongs in wave 1); a
failing stream leaves the verb successful and the report populated; nothing is
published when the save fails.

**T1.7 — gate.** `gorelease` clean, `./scripts/verify.sh`, the scenes as
runnable tests, and a workbench command that drives a session end to end.

---

## Wave 2 — the interrupt spine

**The headline, and it needs no combat.** Scene 2 and scene 3: Alice walks, step
three brings the ogre into view, the world freezes, we persist mid-path, a
different process answers, she resumes. Every part of that is reachable with the
composition's existing perception — which is why this lands before entities
rather than after.

**T2.1 — checkpoints in the walk.** The walk becomes a re-enterable phase
machine: explicit phase index, no Go stack held across a wait, in-between state
serializable (S7). Even here, where only one thing can suspend.

**T2.2 — freeze.** Every non-read verb rejects while a window is open, and the
rejection carries the window and its audience so the caller learns what to do
from the error.

**T2.3 — pose, persist, resume.** `interrupt.Ledger` in `SessionData`; frozen
resolution as opaque bytes. *Shipped without the session-level `Frozen` field:
the payload rides in the window that suspended it — see design.md.*

**T2.4 — the scene, as one continuous story with a pinned transcript**,
including a mid-story process restart. Its negative control: with the checkpoint
disabled, Alice walks past the ogre without stopping.

*Discipline:* enumeration order at a checkpoint is a function of persisted data,
never of subscription order (C8, the encounter's determinism law) — pinned by a
test that reloads and re-resolves
and gets the identical window order.

---

## Wave 3 — entities on the bus

Characters and NPCs load through their repositories, features and conditions
attach for the duration of a call, and durable condition state lives in the
entity's own blob. `Member.Kind` selects the repository. NPCs live in
`SessionData` and vanish with the session.

The bus carries **observation only** — journey 052's rule, enforced by the fact
that nothing in this wave can suspend.

**This is the wave the bus enters the composition at all.** Neither `encounter`
nor `session` uses `events.EventBus` today; both carry `events` as an indirect
dependency only. Everything so far has been bus-free, which is much of why
journey 052's rule has held so easily — there was nothing to put control flow
on. That changes here, which is why the determinism pin below is a wave-3
concern and not a wave-4 one.

**T3.1 — the bus, per call.** One bus per verb, shared by every entity loaded in
that call, discarded with the response. There is no session process: a verb
loads, attaches, acts, saves, and everything dies with the response. Shared
rather than per-entity, because a condition on one member must be able to
observe what happens to another — the prerequisite for reactions in wave 5.

**T3.2 — `CharacterRepository`, and `Member.Kind` routing.** Declared with
`character.Data`. Players resolve through the repository; NPCs through
`SessionData`. The seam takes **IDs, not characters** — `character.Data` appears
only on the repository the host implements, never in a verb input, and
`*character.Character` never crosses at all.

**T3.3 — NPCs in `SessionData`.** `NPCs []npc.Data`, session-scoped, no
repository until a durable NPC exists. Asymmetric reversibility again: adding a
`Config` field later is compatible; removing one a host implemented is not.

**T3.4 — attach and detach, across a suspension.** The load → attach → act →
`ToData()` → save loop, and that same loop re-entered by `Answer`. **Do not call
`character.Cleanup`** — see design.md; it nils the conditions that `ToData`
serializes, and its unsubscribe half is meaningless when the bus dies with the
response.

**T3.5 — the benchmark.** Load + attach at party scale (4–6 characters with a
realistic feature and condition load), measured per verb. This plan already
names *"stateless-per-call proves too slow once entities load on every verb"* as
something that would make it wrong; wave 3 is where that stops being
hypothetical. Measure before anyone designs a cache around a cost nobody has
seen — the fallback (a checkpointing repository on the host side) was already
the design's answer and changes no SDK signature.

**T3.6 — the scene**, continuing the story with a pinned transcript: Alice
raging when she is loaded, without the caller ever mentioning rage (scene 4).

**Pins:** a condition active before a verb is still active and still behaving
after save and reload — *mutation: `Cleanup` before `ToData`, which must fail,
because the failure it models is silent*; a condition active at the moment of
suspension survives a process restart and an `Answer` from a different process;
the boundary test still rejects `*character.Character` while admitting
`character.Data` as a contract type with a recorded reason; one shared bus per
call, proven by a condition on member A observing an event about member B
(*mutation: per-entity buses*); and enumeration order at a checkpoint stays a
function of persisted data rather than subscription order (C8) — the pin that
matters most now that a bus exists to make the other thing tempting.

---

## Wave 4 — combat

Where the real engineering is, and where "what would the ideal composable combat
interaction look like — what does it *simplify*?" gets asked rather than
assumed. Explicitly **not** a port of rpg-api's 6,700 lines; that code is
evidence about what a game server needs, not a specification.

Chains carry modification during resolution. Resolution is a re-enterable phase
machine **from its first line**, even though nothing suspends until wave 5 —
`ReactionTriggerEvent` happened because attack resolution was a straight-line
function, and this is the one lesson that must not be relearned.

This is the wave rpg-api migrates on.

---

## Wave 5 — reactions

Opportunity attack on the wave-2 spine, closing #916. Reaction economy, and the
cross-doorway threat question — which is blocked on the perception wave that
retires T3, and may push that wave ahead of this one.

---

## House disciplines, applying throughout

- Mutation-proof pins from the first task: a pin is not real until it has been
  shown to **fail** under the exact breakage it guards.
- One-defect fixtures with discriminating assertions.
- **Over-tightening defense**: a rejection table proves a validator rejects;
  only a rich positive control proves it does not over-reach.
- Scenes as one continuous story with exact pinned transcripts.
- `gorelease` green on every wave; a wave that reports *incompatible* does not
  ship without a deliberate, recorded decision.
- **Every required `Config` field lands at or before wave 4.** `gorelease` calls
  a new struct field compatible, and for a *required* one that verdict is wrong:
  it compiles everywhere and fails every existing host at its first
  `NewManager`, because construction is total (S8). Free until the migration
  wave, a silent runtime break wearing a green check after it. The gate cannot
  see this, so the discipline has to.
- No draft PRs; no `--no-verify`; no force-push; no `nolint`.

## What would make this plan wrong

- **The S2 boundary test proves unwritable or too coarse.** It is the mechanism
  behind the whole strategy; if it cannot be made precise, the version-bump
  promise degrades to a convention and wave 1 needs rethinking.
- **Stateless-per-call proves too slow** once entities load on every verb. The
  design's answer is that this is the server's problem behind a repository (an
  in-memory checkpointing encounter repository), but that answer is untested.
- **The perception pause turns out to need entity state** — if "should Alice
  stop?" depends on anything on her sheet, wave 2 gains a dependency on wave 3
  and the order flips.
