# Action Economy History

Track what actions were taken during a turn, not just how many remain.

## Status: Issues Created → Ready for Implementation

## Summary

When a Monk uses Step of the Wind, they choose Disengage or Dash. Later game logic (like Opportunity Attack checks, speed calculations) needs to know which was chosen. Currently `ActionEconomy` only tracks budget, not history.

## Trigger

Implementing Monk levels 1-3 revealed that Step of Wind's choice needs to be queryable later in the turn. The `Action string` field attempted on the branch doesn't scale - no type safety, no validation, and per-feature event topics don't work for homebrew.

## Key Decisions

| Question | Decision | Reason |
|----------|----------|--------|
| Where does history live? | On ActionEconomy | Budget and history go together |
| ActionType location | Type in core, constants in dnd5e | Follows established pattern |
| Source field type | core.Ref | Typed, validated, no typo risk |
| Old API methods | Remove | Pre-release, no backwards compat needed |
| Pointer vs value in FeatureInput | Pointer | Accurate error messages |
| Feature choices | GetActivationChoices() method | Features self-describe |
| Event topics | One FeatureActivatedTopic | Per-feature topics don't scale |
| ActionType format | Enum | Type safety in protos and toolkit |
| Downstream effects | Turn-scoped conditions | Fits event-driven architecture |

## Documents

| Phase | Document | Description |
|-------|----------|-------------|
| Brainstorm | [brainstorm.md](./brainstorm.md) | Initial exploration and core design |
| Use Cases | [use-cases.md](./use-cases.md) | End-to-end flows, alternative approaches |
| Design | [design-protos.md](./design-protos.md) | Proto changes (ActionType, ActivationChoice, TurnState) |
| Design | [design-toolkit.md](./design-toolkit.md) | Toolkit changes (ActionEconomy, features, conditions) |
| Design | [design-api.md](./design-api.md) | Game server integration |

## Affected Projects

| Project | Key Changes |
|---------|-------------|
| rpg-api-protos | ActionType enum, ActivationChoice messages, TurnState with history |
| rpg-toolkit | ActionEconomy history, ActivationChoiceProvider, Disengaged condition |
| rpg-api | Pass ActionType through, include choices in GetCharacter, AoO events |

## Implementation Order

1. **rpg-api-protos** - ActionType enum, messages (API contract first)
2. **rpg-toolkit** - ActionEconomy, feature interface, conditions
3. **rpg-api** - Integration, conversion helpers

## GitHub Issues

| Project | Issue | Status |
|---------|-------|--------|
| rpg-api-protos | [#81](https://github.com/KirkDiggler/rpg-api-protos/issues/81) | Open |
| rpg-toolkit | [#399](https://github.com/KirkDiggler/rpg-toolkit/issues/399) | Open |
| rpg-api | [#266](https://github.com/KirkDiggler/rpg-api/issues/266) | Open |

## Progress

See [progress.json](./progress.json) for detailed tracking.
