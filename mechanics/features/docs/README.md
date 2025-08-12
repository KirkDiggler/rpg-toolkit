# Features Simplification

## Read These (in order):

1. **[The Journey](../../../docs/journey/008-features-conditions-refactor.md)** - Complete story with code examples
2. **[The Decision](../../../docs/adr/0020-features-conditions-simplification.md)** - Formal ADR

That's it. Everything you need is in those two documents.

## tl;dr

- Feature interface: 14 methods → ~8 methods
- Implementation: Embed effects.Core, focus on game logic
- Persistence: ToJSON/LoadFromJSON with simple switch
- Success metric: How simple is it to implement Rage?