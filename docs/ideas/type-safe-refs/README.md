# Type-Safe Damage Source References

Replace loose string fields with type-safe enums and structured references for damage tracking.

## Status: Design Complete → Ready for Implementation

## Summary

`DamageComponent.source` in protos is a plain string receiving inconsistent values like `"weapon"`, `"ability"`, and `"dnd5e:conditions:raging"`. This prevents the UI from displaying meaningful damage breakdowns and creates type safety issues throughout the system.

## Trigger

Building the combat UI revealed that damage sources need to be displayable ("Warhammer +5, Rage +2"). The loose string field makes this impossible without fragile string parsing. The toolkit already has informal `DamageSourceType` constants - this formalizes them into type-safe protos.

## Key Decisions

| Question | Decision | Reason |
|----------|----------|--------|
| Flat enum vs oneof? | oneof in SourceRef | Reuses existing enums (Weapon, Ability), structure tells you category |
| Include homebrew escape hatch? | No | Premature abstraction - add when we have concrete use case |
| Separate FeatureId and ConditionId? | ConditionId only for now | Damage comes from active conditions, not passive features |
| Where does SourceRef live? | common.proto | Reusable across messages |

## Documents

| Phase | Document | Description |
|-------|----------|-------------|
| Use Cases | [use-cases.md](./use-cases.md) | Why we need this, UI consumption patterns |
| Brainstorm | [brainstorm.md](./brainstorm.md) | Flat enum vs oneof exploration |
| Design | [design-protos.md](./design-protos.md) | Proto changes (SourceRef, ConditionId) |

## Affected Projects

| Project | Key Changes |
|---------|-------------|
| rpg-api-protos | ConditionId enum, SourceRef message, DamageComponent update |
| rpg-toolkit | Map DamageSourceType constants to ConditionId |
| rpg-dnd5e-web | Enum display component for damage breakdowns |

## Implementation Order

1. **rpg-api-protos** - ConditionId enum, SourceRef message (issue #83)
2. **rpg-toolkit** - Update DamageSourceType to use refs
3. **rpg-dnd5e-web** - Build enum display components

## GitHub Issues

| Project | Issue | Status |
|---------|-------|--------|
| rpg-api-protos | [#83](https://github.com/KirkDiggler/rpg-api-protos/issues/83) | Open |

## Progress

See [progress.json](./progress.json) for detailed tracking.
