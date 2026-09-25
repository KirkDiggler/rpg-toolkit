# World Builder v4: monster declarations and bindings

The single-room authoring schema evolves in place at version 4. There is no
compatibility alias for `room.room.monsters` or a declaration's `faction`.

```yaml
version: 4
# Other required document fields omitted from this excerpt.
factions:
  - id: watch
    table: watch-drill
tables:
  watch-drill:
    time:
      - { when: { enemy: reach }, attack: enemy }
      - { when: { enemy: none }, hold: {} }
room:
  # Scene and other room fields omitted.
  room:
    monsterDeclarations:
      - id: guard-1
        ref: dnd5e:monsters:thug
        startingCell:
          location: { q: 2, r: 0 }
          facing: ne
    monsterBindings:
      guard-1:
        faction: watch
```

## Consumer migration

- Rename `room.room.monsters` to `room.room.monsterDeclarations`.
- Move each declaration's `faction` to `room.room.monsterBindings.<id>.faction`.
  Preserve existing binding fields. A faction-only binding is valid.
- A declaration requires `id`, `ref`, and `startingCell.location`; facing remains
  optional. The declaration list is required but may be empty.
- Bindings remain optional. A monster without a faction binding keeps the same
  default side as before. Explicit empty or null faction references are refused.
- The Go source field is `RoomGameplaySource.MonsterDeclarations`. Compiled
  `Compiled.Monsters` and runtime persistence are unchanged.
- Update editor validation-path routing along with YAML serialization. Old keys
  are rejected with migration messages, not silently accepted.

A binding's table overlays its faction's table by trigger key; local `on`
overlays the named table. A nearer trigger replaces the whole row list.
Conditions only control row eligibility: `when.attacked` does not turn
`attack: enemy` into `attack: attacker`.

This change does not implement runtime faction rebinding, change faction minds,
remove engine-side social checks, or change the v2 region dialect. The older
single-room wrapper uses the same source type and therefore the same keys.
User composition files and downstream API/UI adoption are separate work.

Evidence: `encounter/dungeonspec/v4_declarations_test.go`, the migrated
`world-builder-*.yaml` fixtures, and their unchanged compiled golden files.
Design: `rpg-project/ideas/dungeon-authoring/world-builder/declarations-and-bindings.md`.
