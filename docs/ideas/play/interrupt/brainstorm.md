# play/interrupt — Brainstorm (the WHY)

*2026-08-10. Axis three of the encounter reset (journey 051), designed in
dialogue between Kirk and the director session immediately after the
event-bus evaluation (journey 052) — which exists because this axis
forced it. Decisions recorded with their reasoning and rejected
alternatives. The normative WHAT is `design.md`; the HOW is `plan.md`.*

## What this axis is

Journey 051 called it **interrupt** — reactions, readied actions, and
every moment where a resolution must stop and wait for a decision that
lives outside it. The module is the **ledger of open windows**: when a
rulebook resolution reaches a point where someone may act — the wizard
deciding about Shield, the fighter offered an opportunity attack — a
*window* opens, control leaves the engine, and the game waits. The ledger
holds those open windows as pure data until an answer arrives.

The design's one sentence: **suspension is a value, not a callback.**

## The origin: why the bus never had this

Kirk, on the bus's design era: when it was built, the imagined ceiling of
complexity was **bless-with-concentration** — effect-*modification*
complexity. Journey 024 solved that beautifully (chains, stages,
publisher-executes). But suspension is complexity on a different axis —
*control-flow* complexity — and nobody was imagining a multiplayer
Discord activity where a resolution waits on a human's click. The census
in journey 052 shows the consequence: when reactions were attempted
(Opportunity Attack, Shield), the bus couldn't express "pause and ask,"
so a two-phase workaround grew — observe the chain, publish a
`ReactionTriggerEvent`, apply the modification later as a function
argument — and the capture-window disease it pushed into the old
encounter SDK became one of the reset's named diseases. Interrupt is the
first-class answer to the question the bus could only work around.

## Decisions and their reasoning

### 1. Store, not vocabulary

The fork: is the module a **store** (a container holding the open
windows, with verbs and queries and persistence, in the family mold of
clock/intel/record), or merely a **vocabulary** (envelope types and laws,
with composition holding the actual set)? Kirk: the store "seems like a
useful tool"; the vocabulary option "does not make much sense to me."
That instinct is the answer — the moment the envelope has real
invariants (one answer per window, no orphaned tokens, audience
validation), a vocabulary-with-invariants *is* a store, just an unnamed
one smeared into composition. Named beats smeared.

Kirk's scene sharpened what the store is *for*: "I am wizard in a room, I
can see the attack hit me, control is now waiting on me to click shield
or no shield." The open window is **player-facing game state**, not
internal machinery — the wizard's client renders the prompt from it, the
fighter's client renders "waiting on Aldric…" from the same state. The
store's queries are the host's projection surface.

### 2. Custody, not execution

Kirk, all in: "that is basically making it composable with deterministic
functions." The ledger is truth-blind the way intel is. `Pose` stores the
envelope; `Answer` validates (window open, answerer is the audience,
choice among the offered options), removes the window, and hands the
envelope back — including the frozen resolution as **opaque bytes it
never decodes**. The *rulebook* resumes. The module never runs game
logic, never interprets a choice, never knows what "shield" means. Every
verb is a pure state transition over the window set; the only
nondeterminism in the whole reaction cycle is *which answer arrives*, and
that comes from outside the leaf — a human, a die, a decider.

### 3. A set of windows; pausing is policy, not law

Does the world freeze while a window is open? In turn-based combat it
*feels* frozen ("the world seems paused at that moment" — Kirk), but the
freeze belongs to **composition policy, not the leaf**. The proof is
Kirk's own observation that OA "autofires": an auto-taken opportunity
attack is the *same shape at a different answer latency* — composition
poses the window and a policy decider answers it in the same host call,
so the window opens and closes with zero observable pause. One code
path, two latencies:

- **Shield**: the audience's decider is "ask the human" — the window
  stays open, persists, goes to the wire, control visibly waits.
- **Auto-OA**: the decider is policy or monster AI — posed and answered
  synchronously; the world never observes the wait.

The unification pays three ways: monsters get reactions for free through
behavior deciders; auto-vs-ask becomes a per-player decider swap with
zero engine change; and record sees every reaction identically. So: the
ledger holds a *set* of open windows and enforces nothing about what the
world does meanwhile. Combat's policy is "everything waits"; a future
free-roam mode may answer differently. The leaf never knows.

### 4. No timeout machinery in v0.1 — the shelf, labeled

The director proposed a default-answer law (every window declares its
timeout answer at Pose; expiry resolves to it) plus caller-stamped
deadlines. Kirk shelved the whole apparatus: "those are the things I
think future us can answer better than today us… if taking your turn
becomes problematic then default and timeout earns a place." The shelf
labels, verbatim from the dialogue: **skip-and-assume-the-answer-was-do-
nothing** (the default-answer idea, demoted from law to label), **kick
the character from the encounter**, **give them a wait timer**.

Why shelving is safe: every labeled item arrives *additively* — optional
fields and a new verb are minor-version changes the gorelease gate
classifies honestly. And "open until answered" is not dishonest for
v0.1: combat policy already waits, and a torn-down encounter discards
its ledger wholesale. Nothing welds the shelf shut.

### 5. Sequential posing in initiative order — the two-wizards scene

Kirk's stress test: two wizards, both eligible to react to one trigger —
initiative order? both buttons? dismissal? double-shield? Every one of
those questions lands in **composition, none in the leaf** — the
boundary working as drawn. (Rules footnote that shrinks the problem: RAW
Shield is self-only; the true multi-reactor case is counterspell-class,
which is *rare*.)

The policy fork: **sequential** (pose one window at a time in initiative
order; the next is posed only if still relevant — A counterspells, the
spell dies, B's window is *never posed*) versus **parallel** (both
buttons light; answers apply in deterministic order; the mooted one is
withdrawn and refunded). Kirk chose sequential for v0.1: no races, no
Withdraw verb, no "I clicked and it counted for nothing," deterministic
and griefing-resistant, faithful to the tabletop rule that simultaneous
reactions resolve in a chosen order. Double-anything reduces to the
rulebook's ordinary stacking rules when a later window isn't mooted.
**Parallel posing + Withdraw + refund** goes on the shelf, labeled "if
serialized reaction waits ever feel bad in live play." The leaf is
identical under either policy — sequential-vs-parallel is entirely about
when composition calls Pose.

### 6. The seam: checkpoints, not bus — and resolution is the sibling axis

Ratified from journey 052: trigger points are **explicit checkpoints in
the resolution pipeline** — the attack resolution *knows* "a hit just
landed; a hit-reaction window may open here" — with eligibility at the
checkpoint coming from *enumeration* of the target's persisted effects
(and intel for perception: you can't react to what you can't see). No
discovery-by-eavesdropping anywhere in the path. Deliberate over
incidental.

The frozen state inside the envelope is the **rulebook's**: attacker,
target, rolls so far, phase reached — serialized with the same ToJSON
discipline conditions already use, opaque to interrupt and to the host.
`Resume(frozenBytes, answer)` is a rulebook function. The structural
price is that resolutions become **re-enterable phase machines** — no Go
stack held across the wait. That work is not scope creep on interrupt;
it is the **resolution axis** from journey 051's list, and interrupt is
its first customer. Sequencing follows the family's proven pattern:
interrupt ships now as a pure leaf (as clock shipped without a ticker
host and intel without a geometry engine), the resolution axis does the
rulebook-side work and wires the two together.

### 7. Anchors: Shield in the tests, OA-with-option in production

Product reality (Kirk): playable classes are **monk, fighter, rogue,
barbarian — level 1 only today** (level-up/experience is not implemented
yet, though most of the plumbing exists; the stated goal is reaching
level 3). No wizard in production. So:

- **Shield is the fixture scene** — it lives in unit tests, where it
  proves the two hard invariants: suspension surviving a process
  boundary, and an answer *retroactively modifying its own trigger*
  (re-judge the hit against +5 AC). The axis-two lesson says prove the
  hard thing first or the proof is theater.
- **OA-with-the-option is the first production consumer** — Kirk's call:
  "the OA can give the option, which takes care of the handing the
  control to the user." It is universal (all four classes), it already
  exists in-game as auto-fire, and promoting auto → ask is exactly the
  decider swap from decision 3: zero new machinery.
- **The production ladder** — each rung adds exactly one property, and
  the first two rungs need zero leveling work: OA-with-option
  (suspension + control handoff) → **Protection fighting style**
  (fighter — chosen at level 1, so available *today*; already in the
  codebase as a chain condition; adds ally-scoped triggers and true
  production retroactivity — Shield's hard property in martial
  clothes). Then, once leveling ships: **Deflect Missiles** (monk 3;
  adds a nested follow-up choice — catch and throw back) and Battle
  Master's **Riposte/Parry** (fighter 3) if that subclass enters
  scope. Rogue and barbarian are OA customers for now, which is fine.

### 8. Naming

The axis keeps journey 051's name: **interrupt** — the package is the
category of thing. Inside it, the nouns from the dialogue: the container
is the **Ledger** (the ledger of open windows — Kirk approved the words
as spoken), each entry is a **Window**, the verbs are **Pose** and
**Answer**. The container is deliberately *not* named `Interrupt`: each
window is an interrupt; the thing that holds them is a ledger (the same
instinct that named record's container `Log`, not `Record`). No namesake
collision, so the persistence pair is the family's plain
`ToData() LedgerData` / `LoadLedger`.

## Links

- `docs/journey/051-encounter-reset-application-to-toolkit.md` — the
  axes; this is axis three's leaf half (resolution is the rulebook half)
- `docs/journey/052-event-bus-evolution-broadcast-chains-enumeration.md`
  — the bus evaluation this axis forced; its stance (observation stays,
  discovery becomes enumeration) is ratified by decision 6
- `docs/ideas/play/clock/`, `docs/ideas/play/intel/`,
  `docs/ideas/play/record/` — the family precedent triplets
- Old-code evidence: `rulebooks/dnd5e/conditions/opportunity_attack.go`
  and `shield_spell.go` (the two-phase ReactionTrigger workaround);
  `encounter/combat_phased.go` `installTriggerBuffer` (the capture
  windows this design retires)
