## Toolkit advisory

**MVP toolkit code changes: NO.** `rulebooks/dnd5e` and `encounter` source are identical between `origin/main` and the current checkout; current APIs already cover draft finalization, authored dungeon compilation, and real monster seeding.

### Character choices
- **Protection Fighter:** query level-1 Fighter/Human requirements, then submit `fighting_style: "protection"` with the required skills/equipment. Protection is an allowed Fighter option (`rulebooks/dnd5e/character/choices/requirements.go:253-274`) and finalization maps it to `FightingStyleProtectionCondition` (`character/draft.go:1163-1222`).
- Choose the Fighter “martial weapon and shield” equipment option too (`requirements.go:928-947`). It puts a shield in inventory.
- **Important, medium:** a selected shield is not automatically equipped: finalization populates inventory but not equipment slots (`character/draft.go:600-633`, `1024-1056`). To exercise Protection mechanically, invoke the real existing equip API after finalization to equip `shield` off-hand; do not mutate `Character.Data`. Protection checks shield, reaction, adjacency, and melee at runtime (`conditions/fighting_style_protection.go:129-189`).
- **Barbarian:** submit ordinary level-1 skills/equipment only; no subclass, fighting-style, or Rage selection. Rage is a level-1 class grant (`classes/grant.go:125-155`); the requirements explicitly have no level-1 subclass/spells (`choices/requirements.go:277-288`).

Use the real CharacterService sequence: create draft → requirements → name/race/class/background/scores → finalize. Finalize is the authoritative validation point (`character/draft.go:535-563`). Do not rely on intermediate API validation alone: current rpg-api `SetClass` returns nil validation (`internal/orchestrators/character/orchestrator.go:348-354`).

### Dungeon and module boundary
- The existing `encounter/dungeonspec` compiler is the correct YAML boundary: it produces `DungeonParams` and encounter-owned `SpawnInstruction` without host conversion (`encounter/dungeonspec/compile.go:17-22,67-78`).
- Existing lobby flow already initializes the dungeon, adds players, then calls `SeedMonsters` (`rpg-api/internal/orchestrators/lobby/start_encounter.go:180-250`).
- **Second local override: NO** for the stated sandbox. rpg-api currently resolves `encounter/dungeonspec` from released `encounter@v0.53.0` and directly requires `rulebooks/dnd5e@v0.72.0`; a temporary main-module replacement of `rulebooks/dnd5e` applies transitively to encounter’s dnd5e imports.
- Add a temporary local `encounter` replacement **only** if the sandbox needs unpublished local changes under `encounter/` (including dungeonspec), not merely because dungeonspec imports dnd5e.
- **Medium scope limit:** authored YAML must use placed monster refs/boss coordinates. Count-based/rolled monsters are explicitly M2-only (`encounter/dungeonspec/validate.go:394-419`; `encounter/seed_monsters.go:183-190`).

### Future monster-owner expectations
A new monster is a future toolkit change: add constructor/typed ref and register it in `rulebooks/dnd5e/monster/monsters/registry.go:17-57`; dungeonspec validation and seeding both resolve through that registry (`encounter/dungeonspec/validate.go:359-377`, `encounter/seed_monsters.go:251-322`).

Run:
1. `cd rulebooks/dnd5e && go test ./...` (focused loop: `go test ./monster/...`)
2. `cd encounter && go test ./...`

### Boundary pushback
Do **not** extend the legacy direct-Redis `cmd/devseed` shape (`rpg-api/cmd/devseed/main.go:1-17`). The approved seed must drive CharacterService and lobby APIs. Do not construct `Character.Data`, `MonsterData`, or Redis records, and do not add a generic toolkit fixture framework. `SeedMonsters` already constructs, serializes, validates, and atomically commits runtime monsters; hosts should only pass authored refs through the production seams.