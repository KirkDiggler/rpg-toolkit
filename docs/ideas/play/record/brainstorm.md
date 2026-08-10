# play/record — Brainstorm (the WHY)

*2026-08-10. Axis two of the encounter reset, lane 2 — the compact
mechanical lane run alongside `play/intel`'s deep-design lane. Most of
this module's why was settled inside the clock and intel dialogues; this
doc collects it. The normative WHAT is `design.md`.*

## What this axis is

Journey 051: "**Record** — the ordered, correlated, audience-projected
event log." The intel dialogue gave it its identity in one line of
Kirk's: *"I really just want to tell the story."* The family's answer
assigns three roles: **intel is the state of each mind, record is the
story as told, the host wire is the live performance.** Record is the
retained narrative — the thing the old module never actually had (it
persisted only a sequence counter; events lived and died on the wire, so
a reconnecting client's story was rebuilt by side-channels).

## Decisions and their reasoning

### 1. Storage and query, never delivery

The old broker coupled the log-shape to transport (encode/decode type
switches, Redis pub/sub, subscription lifecycles). Record refuses all of
it: entries are appended and queried as values; streaming the appends is
the host's business. This is the R4 return-values law applied to
history: nothing escapes by side effect.

### 2. The envelope is exactly what the leaves omit

Clock and intel deliberately return deltas with no sequence, no
correlation, no audience — facts undressed. Record is where facts get
dressed for the outside world: a **Seq** (the one thing record itself
assigns — the monotonic spine of the story), a caller-supplied **At**
(the family's opaque time-token convention), a caller-supplied
**Correlation** (the composition knows which action caused which
effects; record just files it), a materialized **Audience** (computed by
the composition — typically from intel/stage answers to "who could
perceive this"), and an opaque **Payload** (record never interprets;
story beats are the leaves' delta shapes, encoded by the composition —
the condition-blob discipline again).

Audience is a materialized `[]core.EntityID`, not a predicate —
predicates don't serialize (the old broker's AudienceSet got this
right). Deliberate rule over magic: an empty audience means *no viewer
sees it* (a GM-only or debug beat); "everyone" is spelled out by passing
the roster. Nothing is implicitly public.

### 3. Story beats are leaf deltas — by composition, not coupling

`Surveil`'s first-contact, clock's milestones, an attack's outcome: each
is a story beat *if the composition says so*. Record depends on core
only; it has never heard of clock or intel. The old per-player streams
(the door-opener's big reveal vs the follower's smaller one) fall out
naturally: each observer's intel deltas become entries with an
audience-of-one; shared beats carry shared audiences.

### 4. Retention is deliberate, not incidental

The honest problem with a log in a load-verbs-save host: it grows with
time and re-serializes every RPC. Options weighed: full persistence
(simple, unbounded), ring buffer (bounds by count, loses the story
arbitrarily), session-scoped (loses replay). Chosen: **full persistence
plus an explicit `TrimBefore` verb** — the composition owns retention
policy (trim what all clients have acknowledged, or keep everything for
a short encounter), and the demand on the data is a visible decision at
the call site instead of a hidden property of the module. Same
deliberate-over-incidental rule that reversed the nil-Input stance.
Event sourcing stays exactly where the intel dialogue left it: a host
that records complete testimony *may* rebuild intel by replay — an
option by composition, imposed on no one.

### 5. Tags — the question-surface (Kirk's customer probe)

Kirk tested the design against its actual customer: encounter records
intel deltas AND clock milestones — differently shaped — and "I might
want to ask questions about it." An opaque-payload-only envelope makes
the story streamable but not askable, and record can never decode
payloads (sibling import). The subjects doctrine from intel, applied to
the envelope: **tags** — caller-chosen key-value metadata the
composition flattens out of each beat (`kind`, `observer`, `subject`,
...), queryable by exact AND-match, never interpreted. The
question-surface is uniform even when payloads are not. Deliberately
bounded: AND-only; anything richer is the host projecting from `All`
into a real query engine — record is the story, not the database.

### 6. Runtime metadata — the structured-logging pattern (forward note)

Kirk's extension of tags: key-value runtime metadata as a general idiom
— "we could even load the game ctx with it." Record is structured
logging for the story, and the slog/zap precedent maps exactly: Append
is the line, tags are the fields, ambient scope-tags (`encounter_id`,
round, acting entity, correlation) are `logger.With(...)`. The layering
that keeps it honest: ambient tags live in the composition/host (the
future encounter's gameCtx — journey 048's pattern finally has a place
to land); the composition merges ambient + per-beat tags and hands
record the final map; leaves never read ambient anything — metadata
about the telling rides along, but any value a leaf needs is an
explicit Input field. A record-side `With(tags)` pre-merge handle is
anticipated as a five-line additive ergonomic, deferred per the
outside-in rule until encounter's call sites prove the repetition.

### 7. Plain vocabulary

Intel earned its register; record needs none. `Log`, `Entry`, `Append`,
`SliceFor`, `TrimBefore` — the words every reader already holds.

## Links

- `docs/journey/051-encounter-reset-application-to-toolkit.md` — axis
  definitions; the milestone-enrichment note that seeded this design
- `docs/ideas/play/intel/brainstorm.md` §7 — the fold/log split and the
  story framing this module exists to serve
- `docs/ideas/play/clock/design.md` — the family laws inherited here
- Old module evidence: `encounter/broker.go` (the 23-case encode/decode
  and transport coupling this design refuses; the AudienceSet idea it
  keeps), `encounter/data.go` (`Sequence` persisted, events not — the
  reconnect gap record closes)
