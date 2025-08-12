# Features/Conditions Simplification

## 📚 Start Here

1. **[The Problem & Solution](implementation-complete.md)** - Complete walkthrough with Rage example
2. **[Keep It Simple](keep-it-simple.md)** - Why features stay features (not generic actions)
3. **[Persistence Pattern](persistence-pattern.md)** - How ToJSON/LoadFromJSON works

## 📖 The Full Story

### Core Design Documents
- **[ADR-0020](../../../docs/adr/0020-features-conditions-simplification.md)** - Architecture Decision Record
- **[Journey 008](../../../docs/journey/008-features-conditions-refactor.md)** - How we got here (3-4 iterations)

### Key Concepts

#### What We're Building
- [Definitions](definitions-and-separation.md) - Features vs Spells vs Conditions vs Effects
- [Feature Interface](keep-it-simple.md#feature-interface---just-what-we-need) - The simplified interface

#### How It Works
- [Event Filtering](smart-event-subscriptions.md) - Smart event bus subscriptions
- [Targeting](targeting-and-event-filtering.md) - How features handle targets
- [Activation](activation-and-errors.md) - Player activation and error handling
- [Finding Features](finding-features.md) - Matching player input to features

#### Persistence & Loading
- [ToData vs ToJSON](patterns/todata-vs-tojson.md) - Internal vs external APIs
- [Game Server Flow](examples/game-server-example.md) - LoadFromGameContext example
- [Level Up Pattern](level-up-pattern.md) - How features change with level

### 📁 Archive

<details>
<summary>Earlier explorations (click to expand)</summary>

These documents explored various approaches before we settled on the current design:

#### Alternative Approaches
- [Actions Not Features](actions-not-features.md) - Explored generic action system (too abstract)
- [Feature Activation Pattern](feature-activation-pattern.md) - Earlier activation ideas
- [When to Use What](when-to-use-what.md) - NewRage() vs LoadFromJSON()
- [Features Level Up Too](features-level-up-too.md) - Feature progression ideas

#### Implementation Details
- [Data Loading Pattern](patterns/data-loading-pattern.md) - Registry pattern (abandoned for switch)
- [Data Persistence Pattern](patterns/data-persistence-pattern.md) - Earlier persistence ideas
- [Data Persistence Simple](patterns/data-persistence-simple.md) - JSON options explored
- [ToJSON Pattern](patterns/tojson-pattern.md) - What uses this pattern

#### Architecture Evolution
- [Architecture Proposal](architecture/architecture-proposal.md) - Initial thoughts
- [Architecture Clarification](architecture/architecture-clarification.md) - Ref location discussion
- [Usage Example](examples/usage-example.md) - Early usage patterns

</details>

## 🎯 Success Metric

**How simple is it to implement complex features like Rage?**

Before: 14 interface methods to implement
After: Just extend SimpleFeature and handle your game logic

## 💬 We Want Your Feedback!

This is a significant simplification. Please review:
1. Is the interface too simple? Too complex?
2. Does the persistence pattern make sense?
3. Are we missing any critical use cases?

Comment on [PR #192](https://github.com/KirkDiggler/rpg-toolkit/pull/192)