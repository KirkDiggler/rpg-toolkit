# World NPCs - Brainstorm

Issue: [rpg-toolkit#1280](https://github.com/KirkDiggler/rpg-toolkit/issues/1280)

## Problem

The current live-play stack has two placed creature categories:

- players, loaded from the host's character repository through `session.Join`;
- monsters, instantiated from rulebook content through `session.Spawn`.

Both become encounter members. Members are visible to sight, can hold sight
intel, can trigger player-monster contact, can enter fight bubbles, and are
candidates for attack and monster turn behavior according to their kind.

That is not the right shape for a villager, vendor, trainer, quest giver, or
rescued prisoner standing in the dungeon. Those are world entities: placed on
the map, visible and possibly blocking, but not monsters and not combatants in
the MVP.

## Current Vocabulary to Preserve

- The modern encounter field is one dungeon-absolute canvas with authored
  regions, props, walls, and doors. Do not bring back room-local placement.
- `PropInput` is for authored non-creature map contents. Props may block
  movement and sight, but they have no identity, no interaction surface, and no
  live state.
- `MemberInput` is for actors in the encounter. Members are currently players
  or monsters and participate in sight/contact/clock/fight behavior.
- `session` is the host seam. It exposes host-shaped types and keeps inner
  encounter types behind translators.
- Monster sheets are stored in `SessionData.NPCs`, but that word currently
  means session-scoped combat monster sheets. Reusing it for world NPCs would
  make the ambiguity worse.

## Package Shape

World NPCs need two homes, not one:

- a new toolkit-level NPC package should own reusable NPC definitions/profiles.
  Use `npc` as the preferred package name, unless implementation conventions
  make `npcs` read better beside existing top-level packages such as `items`.
- actual D&D vendor types/profiles live under `rulebooks/dnd5e`, where they can
  carry D&D item refs, stock defaults, pricing assumptions, and any vendor-type
  content added by #1275.
- `rulebooks/dnd5e/encounter` owns the placed runtime member behavior for the
  D&D live-play stack: where the NPC stands, whether it blocks movement,
  whether observers can learn its location, whether it can observe, interaction
  range checks, and exclusion from combat clocks.
- `rulebooks/dnd5e/session` owns the host seam that wires those placed NPCs into
  D&D session start/load/interact behavior.

The framework should not put vendor stock into either encounter or the generic
`npc` package. Stock and buy flow belong to #1275 and can point at a world NPC
by ID/ref.

## Options Considered

### Option A: Model world NPCs as props

Rejected for MVP. A prop can block movement and appear in the atlas, but it is
not a live entity with identity or per-entity interaction capabilities. Turning
props into live interactables would overload a content/rendering shape with
session behavior.

### Option B: Model world NPCs as monsters with no actions

Rejected. Monsters are combat participants. Even an actionless monster still
enters the player/monster taxonomy, contact detection, fight formation, standing
checks, and target candidate logic. That makes every combat system remember to
special-case "not really a monster", which is the leak this issue exists to
avoid.

### Option C: Add a third encounter member kind

Promising. World NPCs need live identity, placement, visibility, movement
blocking, optional observation, and story/interact beats. Those are
member-shaped facts. The trick is to make a world member kind, likely
`KindWorld`, a non-combat member kind with explicit combat, sight, and
interaction laws, rather than letting it inherit player/monster behavior by
accident. The runtime kind names how encounter treats the member; the reusable
content bucket remains `npc`.

### Option D: Add a separate encounter collection beside members

Possible, but likely heavier. It avoids touching member combat logic, but then
every spatial read, sight refresh, story beat, persistence path, and session
projection needs a parallel entity collection. Because these entities stand on
the same canvas and should be seen at the same world coordinates as members, a
separate collection risks reimplementing member placement without member
invariants.

## Working Direction

Use Option C with strict exclusions:

- world NPCs are encounter members for placement, visibility, movement blocking,
  story, and interaction;
- world NPCs are perceptible subjects, so players can hold intel about an NPC's
  location after they learn it through authored/loaded knowledge or sight;
- world NPCs may or may not be observers, chosen per NPC or NPC type;
- world NPCs are not on a combat side;
- world NPCs never form or join a fight bubble in the MVP;
- world NPCs are not attack candidates in the MVP;
- world NPCs are not driven by `Decider`, `TurnDriver`, or action economy;
- observer-capable world NPCs may hold intel, but no MVP behavior consumes that
  intel;
- interaction capabilities are reported as descriptors, not executed behavior.

The first concrete implementation that should sit on this framework is covered
by [rpg-toolkit#1275](https://github.com/KirkDiggler/rpg-toolkit/issues/1275):
vendor and NPC inventory. This framework should give that issue a placed,
interactable NPC to attach shop inventory to; #1275 owns stock, quotes, buys,
finite/infinite availability, and item transfer.

The first concrete profile this framework should support is therefore a
vendor-like NPC:

- authored/loaded so the party can know where they are from the start;
- exposes `vendor` as an interaction capability;
- non-hostile to players;
- non-hostile to monsters;
- never treated as prey, ally, enemy, or a combat participant by either side.
- carries common NPC facts we already know are useful: ref, display name,
  capabilities, combat policy, observation policy, movement blocking, and
  optional starting knowledge.

The generic package should ship the common NPC shape and only the first
capability we know we need now: `vendor`. Future capabilities can be added when
their first implementation arrives.

The model can grow later into vendors, trainers, quest actors, escorts, or
attackable NPCs by adding explicit policies. The MVP should not guess any of
those rules, and it should not pull #1275's buy flow into the world-NPC layer.
