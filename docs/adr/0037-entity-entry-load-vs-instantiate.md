# ADR-0037: Entity Entry Splits on Load-vs-Instantiate, Not Player-vs-Monster

Date: 2026-08-13

## Status

**Accepted** — decided by Kirk (architect/owner) on 2026-08-13, shipped in
`rulebooks/dnd5e/session/v0.4.0` (PR #952).

## Context

The session SDK needed a way for NPCs to enter a session. Characters already
entered through `Join`, which took a member ID and a `MemberKind`, and resolved
players through the host's `CharacterRepository`.

Monsters had no equivalent. `encounter.Member` is `{ID, Kind, Room, Position}` —
no hit points, no sheet, no actions — so a monster could be *placed* but had
nothing to fight with. Something had to supply the statblock.

The question looked small and was not. It shapes a public verb signature under a
`gorelease` promise, and the wave plan's own cutting rule says anything shaping a
public type lands early precisely because those decisions are the irreversible
ones. It was also, at that moment, **free**: rpg-api has not adopted the SDK and
nothing is pinned to the current tag. After the migration wave it would not be.

## Decision

**Two entry verbs, split by where the entity's data comes from.**

```go
// Join brings an entity that ALREADY EXISTS into the session.
Join(JoinInput{Session, Member, Room, Position})
// Member is the character ID; the host's CharacterRepository resolves it.

// Spawn instantiates content that lives in CODE as a new member.
Spawn(SpawnInput{Session, ID, Ref, Room, Position})
// Ref names the catalog entry — "dnd5e:monsters:skeleton".
```

**A player has no ref, and its absence is the decision rather than an omission.**

The reasoning that settled it, and the reusable part of this ADR: **a ref is
loader routing.** It names the package that can load some data — which is why
`homebrew:classes:artificer` can be dispatched to a homebrew package a game
server has loaded. Applied honestly:

- `dnd5e:monsters:skeleton` is **true**. `NewSkeleton` really does live in the
  dnd5e monsters package. The toolkit can produce this from the ref alone.
- `dnd5e:characters:alice` is **false**. No toolkit package can load Alice. The
  host owns her, and only the host's repository can produce her. The module
  segment would be claiming a capability that does not exist.

So the ref belongs exactly where data comes from code, and nowhere else. The
axis that follows — *load an existing instance* versus *instantiate from a
template* — is also the one that survives contact with the future:

- A **durable NPC** is a monster you *load*. It joins.
- **Homebrew content** can be either, and routes by `(Module, Type)` in both.

"Player or monster" survives neither.

**Supporting decisions**, each following from the above:

- `MemberKind` leaves `JoinInput` entirely — the verb implies it. It remains on
  the `Member` output and in the encounter's persistence, where ending triggers
  genuinely depend on it, and is set internally by each verb.
- `ID` and `Ref` are separate on `Spawn` because a template carries no identity:
  one catalog entry makes five skeletons.
- `SessionData.NPCs []monster.Data` is session-scoped with no repository until
  something durable exists — adding a `Config` field later is compatible,
  removing one a host implemented is not.
- **The stored NPC is the instance, never the ref it was built from.** Re-running
  the constructor on load would silently heal a wounded skeleton.
- Both verbs share ONE placement function, pinned by asserting the same sentinel
  through both doors. Same argument the anchoring wave settled by making Setup
  and Load share one validator.
- `Ref` is a **string**, not `*core.Ref`, keeping `core` off the boundary. A key
  is precisely the thing the project's Boundary Rule says should cross.

## Options considered

Recorded in full because the near-miss is the most useful part of this ADR.

**A. `Join` gains an optional `monster.Data` sheet.** The host supplies the
statblock inline for monsters, by ID for players.
*Rejected:* puts a full domain shape in a verb input, which is the one thing the
character design had just finished deciding against ("the seam takes IDs").

**B. `MonsterRepository`, symmetric with characters.** Both kinds resolve by ID.
*Rejected:* the plan explicitly defers a repository until a durable NPC exists,
and a required `Config` field is the direction that cannot be walked back.

**C. `Member{ID, Ref}` with ref-presence as the discriminator.** One verb; absent
ref means player.
*Rejected:* absence-means-something does not extend. The moment a durable NPC
repository exists, "no ref" stops uniquely meaning "player."

**D. Uniform refs for everything, including `dnd5e:characters:char-123`.**
*Rejected, and this is the one that nearly landed.* It was internally
consistent, removed every branch, and made the routing table beautifully
uniform — it **fit**. Two things killed it. First, `ID` and `Ref.ID` are the
same string for a character, so the shape was either redundant or an invitation
to give one entity two identities; the redundancy was the visible symptom.
Second, the deeper fault: the ref would have been *lying*, because the dnd5e
module cannot load a host-owned character. **Fitting the model is not the same
as being true.**

**E. Two verbs split on player-vs-monster.** The shape that shipped, but reasoned
from the wrong axis.
*Rejected as framing:* it produces the same code today and mispredicts
tomorrow — a durable NPC is a monster you load, and would have to break the
rule or distort a verb.

## Consequences

### Positive
- The host never constructs a statblock; it names content and gets an instance.
- Nothing is inferred at the seam. Each verb's fields are exactly what that case
  needs, with no field meaningful for only half its inputs.
- Homebrew has a place to land: a host-registered loader keyed by
  `(Module, Type)` is a new optional `Config` field, a compatible addition.
- Four distinct sentinels (`ErrNoRef`, `ErrBadRef`, `ErrNoLoader`,
  `ErrUnknownContent`) because the remedies differ — fix the call, ship a build
  with that content, or pick content that exists.

### Negative
- **A breaking change**: `JoinInput.Kind` removed. Free at the time and not
  later, which is why it was made immediately rather than deferred.
- Two placement paths exist in principle; mitigated by one shared `place()` and
  a pin through both doors, but the discipline has to be maintained.
- The loader table is internal. `homebrew:monsters:*` is an honest
  `ErrNoLoader` today, not an extension point.

### Neutral
- `monster.Data` crosses the boundary as a **persistence shape**, not a contract
  type beside `character.Data`. The distinguishing question: would the host build
  one field by field? For a character yes, so a change is announced. For a
  monster no, so the promise is replaceability.

## The process note, which generalises past this decision

Option D nearly shipped **because it fit**. Internal consistency is a seductive
signal and a weak one: a model can be perfectly uniform and still assert
something false about the world.

What broke it was not tightening the argument — it was a deliberately different
direction ("a ref is loader routing, for homebrew dispatch") thrown at the wall
to see what stuck. That reframing did not refine option D; it revealed that D's
foundation was wrong.

The rule worth keeping: **when a decision shapes a public type or a boundary,
stop and produce genuine alternatives — including deliberately odd ones — and put
the trade-offs in writing before choosing.** The cost is minutes. The cost of a
seam decided because it "fit" is a version promise built on a false claim.

## See also

- ADR-0022 (Loader Pattern) — the repository/loader split this extends to a
  ref-routed loader table.
- ADR-0023 (Core Provides Types, Rulebooks Provide Implementation) — why `core`
  is kept off this boundary.
- `docs/ideas/session-sdk/{design,plan}.md` — the wave plan and its amendments.
