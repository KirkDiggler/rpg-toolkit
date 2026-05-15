# ADR-0027: Attack Resolution and Reactions

Date: 2024-12-25 (proposed) / 2026-05-10 (accepted)

## Status

Accepted

## Confirmed by shipped code

The three-phase attack model and condition-as-chain-subscriber pattern this ADR
proposed are now load-bearing in shipped code. Promotion from Proposed to
Accepted reflects that the seams the ADR predicted are the seams the rulebook
actually uses:

- **Chain subscription pattern (Phase 1 — RESOLVE ATTACK)** —
  `rulebooks/dnd5e/combat/attack.go:225-235` builds a `StagedChain[AttackChainEvent]`
  and calls `attacks.PublishWithChain(ctx, attackEvent, attackChain)` followed
  by `Execute()`. Conditions subscribe via `AttackChain.On(bus).SubscribeWithChain(...)`
  and add modifiers at named stages — see `conditions/sneak_attack.go` (subscribes
  to `DamageChain`, marks eligibility, adds dice at `StageFeatures`) and
  `conditions/fighting_style_protection.go` (subscribes to `AttackChain`, adds
  disadvantage + records `ReactionsConsumed` at `StageFeatures`).
- **gamectx pattern (ADR-0025) is the state-query seam** — `sneak_attack.go:221`
  calls `gamectx.RequireRoom(ctx)` to query target position; `fighting_style_protection.go:130-152`
  calls `gamectx.RequireCharacters(ctx)` for shield + reaction-economy lookup
  and `gamectx.RequireRoom(ctx)` for adjacency. Conditions never bloat events;
  they query state through gamectx at handler time.
- **Movement chain + OA prevention primitive** —
  `rulebooks/dnd5e/combat/movement.go:138-250` ships `MoveEntity` with a
  `MovementChain` that conditions can modify before opportunity-attack
  triggering. `conditions/disengaging.go:131-157` is the reference subscriber:
  it appends to `MovementChainEvent.OAPreventionSources` at `StageConditions`,
  and `MoveEntity` checks `IsOAPrevented()` before invoking
  `triggerOpportunityAttack`. The OA-as-condition direction this ADR predicted
  is partially shipped — Disengage prevents OAs as a condition; the OA *trigger*
  itself still lives inline in `MoveEntity` rather than as a per-combatant
  subscriber. Wave 2.11 (`rpg-project/ideas/encounter/v1alpha2/plans/11-wave-2.11-condition-driven-reactions.md`)
  is the wave that ports the trigger to the same condition pattern.
- **Reaction consumption inside the chain** — `AttackChainEvent.ReactionsConsumed`
  (`rulebooks/dnd5e/events/events.go`) is a slice that subscribers append to
  during chain execution; `combat.ResolveAttack` (`combat/attack.go:296-306`)
  publishes one `ReactionUsedEvent` per entry after `Execute`. The "reactions
  are subscribers + record consumption back on the event" model from this ADR
  is the shipped shape.
- **AttackTypeOpportunity tag is shipped** —
  `rulebooks/dnd5e/events/events.go:181-182` defines `AttackTypeOpportunity`,
  consumed by `combat/movement.go:360-370` (`triggerOpportunityAttack` calls
  `combat.ResolveAttack` with `AttackType: AttackTypeOpportunity`). Conditions
  that gate on attack type (Sentinel, Polearm Master) have a stable hook.
- **Discrete-phase orchestration shipped (Wave 2.11d, 2026-05-14)** — the
  2026-05-10 amendment described the discrete RPC phase model conceptually;
  Wave 2.11d ships the load-bearing shape. Specifically:
  - **`combat.AttackContext` is JSON-serializable.** `eventBus` and `roller`
    are no longer carried inside the context; `abilityMod`/`abilityUsed`/
    `isOffHandAttack` are now exported as `AbilityMod`/`AbilityUsed`/
    `IsOffHandAttack`. The orchestrator persists the context across the
    player-reaction RPC gap (TakeAction → SubmitCheck{take_reaction} →
    CompleteTakeAction) without needing reattach helpers.
  - **`combat.ApplyAttackOutcomeInput` carries `EventBus` + `Roller` directly.**
    Symmetric with `ResolveAttackHitInput`. The orchestrator supplies these
    on the resume path from the encounter-scoped bus + cfg roller.
    `EventBus` is required at validation time; `Roller` is optional (defaults
    to `dice.NewRoller()`).
  - **`combat.PostAttackRollChain` is the post-roll subscription seam.**
    `ResolveAttackHit` publishes a `PostAttackRollEvent` after the d20 has
    been rolled and `wouldHit` computed but before the `AttackContext`
    returns. Implemented as a chained topic (rather than a typed topic) so
    the publish-time context (carrying `gamectx.WithReactionReadiness` etc.)
    propagates to subscribers. Shield's predicate runs here.
  - **`encounter.PhasedCombatResolver`** (optional extension to
    `CombatResolver`) exposes `ResolveAttackHit` + `ApplyAttackOutcome` to
    the encounter SDK. The SDK's `Encounter.TakeActionPhased` +
    `CompleteTakeAction` are the canonical orchestration entry points.
  - **`Encounter.Data.PendingReactionPrompts`** is the persistence shape
    between the two RPCs. Keyed by reactor `PlayerID`; carries
    `AttackContextJSON` as opaque bytes (the SDK stays rulebook-agnostic;
    the orchestrator marshals/unmarshals via `combat.AttackContext`).
  - **`encounter/events.InputRequiredDeliveredEvent`** is the metadata-only
    SDK event with single-viewer audience (only the reactor receives it).
    The wire-side translator reads `PendingReactionPrompts` to build the
    proto payload.
  - **NPC pause-on-reaction.** `Encounter.NPCAct` uses the encounter-scoped
    bus + phased path; when a player reactor has a triggered prompt the
    NPC's turn pauses via the `errNPCPausedForReaction` sentinel
    (`IsNPCPausedForReaction` helper for orchestrators).

  See `rulebooks/dnd5e/conditions/opportunity_attack.go` and
  `shield_spell.go` for the canonical reaction-condition implementations
  this surface enables.

## Amendments since proposal

The ADR's intent is correct. Two specific items in the original text are stale
relative to shipped code; they're noted here rather than rewritten so the
ADR's narrative remains the design that drove the implementation.

- **Phase 0 events (`AttackDeclaredEvent`) and Phase 2 events
  (`AttackRolledEvent`, `AttackResolvedEvent`) are not yet published as
  separate topics.** `combat.ResolveAttack` currently publishes one chain
  (`AttackChain`) before the roll and emits `DamageReceivedEvent` after damage
  is applied. The DECLARE / ROLL / DAMAGE windows the ADR describes exist
  conceptually inside `ResolveAttack` (chain publish → roll → hit determination
  → damage chain → damage publish) but are not externally observable as
  reaction-window events yet. **Wave 2.11 ships the missing publish points
  (Phase 0 declare + Phase 2 post-roll + Phase 3 pre-damage / post-damage) so
  Sentinel / Shield / Uncanny Dodge / Hellish Rebuke have somewhere to
  subscribe.** Until then, only Phase 1 (chain) reactions like Protection are
  expressible.
- **`OpportunityAttackCheckEvent` is not yet shipped as a typed topic.** The
  OA-prevention primitive is currently the `OAPreventionSources` slice on
  `MovementChainEvent`, mutated by chain subscribers (e.g., Disengaging). A
  separate `OpportunityAttackCheckTopic` was sketched in
  `docs/ideas/action-economy-history/design-toolkit.md` but did not land. The
  shipped chain-mutation approach is sufficient for prevention; it's
  insufficient for *triggering* OAs as conditions (the larger Wave 2.11 OA
  port). Wave 2.11 evaluates whether to add `OpportunityAttackCheckTopic` or
  keep the trigger inline in `MoveEntity` and just relocate the geometry +
  reaction-resource check into a per-combatant `OpportunityAttackCondition`
  subscriber.

These amendments do not change the architectural decision; they record the
shipped-code drift between the original proposal and today. The new ADR-0027
is the shape Wave 2.11 builds against; the amendments call out which pieces
Wave 2.11 has to ship to honor the full design.

## Amendments since acceptance

### 2026-05-10 — Discrete RPC phases instead of in-process chain pauses; opt-in reaction readiness

The chain pause primitive originally implied by the three-phase model is being
implemented as **discrete RPC phases orchestrated by the server (rpg-api)**, not
as in-process chain pauses inside `combat.ResolveAttack`. The toolkit chain
itself does not gain pause-resume capability; that complexity moves out of the
toolkit and lives at the RPC boundary, where the encounter snapshot already
persists naturally.

Concretely:

- `combat.ResolveAttack` is split into `ResolveAttackHit` (phase 1: chain runs
  end-to-end, returns an `AttackContext` carrying roll, originalAC, wouldHit,
  etc.) and `ApplyAttackOutcome` (phase 2: chain runs end-to-end with reaction
  modifiers baked in, recomputes effective AC, applies damage if still a hit,
  publishes outcome events).
- Each phase is atomic. No in-flight chain state is serialized. The "pause"
  lives between RPC calls — the server invokes phase 1, scans for
  `ReactionTrigger` events the chain published, pushes prompts to ready
  reactors, awaits responses via `SubmitCheck`, then invokes phase 2 with the
  player's choice baked in.
- Conditions still subscribe to the chain at the right window. When their
  predicate matches AND `gamectx.IsReactionReady(charID, reactionRef)` returns
  true, they publish a `ReactionTriggerEvent` on the encounter bus that the
  orchestrator reads after the chain returns. The chain itself runs to
  completion either way.
- NPC reactors auto-resolve inline during phase 1 (the modifier is applied
  directly to the in-flight chain event, no event published). Player reactors
  with readied reactions emit the trigger event for the server to translate
  into a prompt.

**Reactions are opt-in via per-character readiness state.** Default: no
prompts. The condition handler's readiness check gates the trigger event. This
trades RAW 5e flexibility ("cast Shield reactively without pre-declaring") for
"no constant prompt mashing" — the right call for asynchronous multiplayer
without a DM. Default readiness varies by reaction type: free-cost reactions
(OA) default-on for melee combatants; spell-cost reactions (Shield,
Counterspell) default-off; class-feature reactions per-feature. Spell-cost
reactions auto-clear after firing (one-shot); free reactions stay ready until
canceled.

**Why this beats the implied "chain pause-resume" primitive:**

- Serializing in-flight chain state means serializing a mutable Go struct with
  registered closures (chain stage handlers). Not JSON-round-trippable without
  significant refactoring.
- Shield's mechanic is retroactive — the hit decision already executed at the
  time Shield triggers. Implementing this with chain-pause-and-mutate means the
  chain has to be reversible, which is much harder than reversible RPC calls.
- The discrete-phase split doesn't pause anything inside the chain. Each phase
  runs end-to-end. The split is at the RPC boundary — where the snapshot
  persistence already lives.
- Opt-in readiness means the split rarely happens. The common case stays
  single-phase end-to-end. When a player has a readied reaction whose
  predicate matches, the server pays the prompt cost; otherwise the attack
  runs as today.

The wave-2.11d plan
(`rpg-project/ideas/encounter/v1alpha2/plans/11-wave-2.11-condition-driven-reactions.md`)
is the canonical implementation reference. The "Phase 0/1/2/3" framing in this
ADR remains the design that drove the implementation; the amendment is on
*how* the windows are surfaced (discrete RPC phases + ReactionTrigger event +
gamectx-readiness predicate, not in-process chain pause).

## Context

ADR-0026 established how damage flows through the event system. But attacks involve more than just damage - they require resolving modifiers (advantage, attack bonuses) before rolling, and reactions can interrupt at various points.

**Key challenges:**
- Sneak Attack needs to know about advantage BEFORE the roll to determine eligibility
- Shield spell triggers AFTER the roll but BEFORE hit determination, modifying AC
- Uncanny Dodge triggers AFTER hit determination but BEFORE damage
- Attack of Opportunity triggers on movement, consuming the reaction resource

We need a consistent model for:
1. Multi-phase attack resolution
2. Reaction windows at appropriate points
3. Reaction resource consumption

## Decision

**Attack resolution uses a three-phase event model with reaction windows between phases.**

### The Full Attack Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 0: DECLARE (optional reactions before anything happens)     │
│                                                                     │
│  AttackDeclared event published                                     │
│       ↓                                                             │
│  [Reaction Window]                                                  │
│       - Sentinel: "Enemy attacked my ally, I attack them first"    │
│       - Protection: "Ally attacked, I impose disadvantage"         │
│       - Consumes reactor's reaction                                 │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 1: RESOLVE ATTACK (gather modifiers before rolling)         │
│                                                                     │
│  ResolveAttack chain published                                      │
│       ↓                                                             │
│  [Stage: base] → attacker's base attack bonus                       │
│       ↓                                                             │
│  [Stage: features] → Sneak Attack marks eligibility                 │
│       ↓                                                             │
│  [Stage: conditions] → Poisoned? Restrained?                        │
│       ↓                                                             │
│  [Stage: equipment] → Magic weapon bonus                            │
│       ↓                                                             │
│  [Stage: situational] → Flanking advantage, cover penalties         │
│       ↓                                                             │
│  Chain.Execute() → returns ResolvedAttack                           │
│       - HasAdvantage, HasDisadvantage                               │
│       - AttackBonus                                                 │
│       - SneakAttackEligible                                         │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 2: ROLL AND EVALUATE                                         │
│                                                                     │
│  Roll d20 (with advantage/disadvantage)                             │
│       ↓                                                             │
│  Calculate total: roll + attack bonus                               │
│       ↓                                                             │
│  AttackRolled event published (contains roll result, target AC)     │
│       ↓                                                             │
│  [Reaction Window - CAN CHANGE OUTCOME]                             │
│       - Shield: "Add +5 to my AC"                                   │
│       - Cutting Words: "Subtract d6 from attack roll"               │
│       - Lucky: "Reroll that die"                                    │
│       - Consumes reactor's reaction (and spell slot if applicable)  │
│       ↓                                                             │
│  Determine hit/miss/crit with modified values                       │
│       ↓                                                             │
│  AttackResolved event published                                     │
│       - Hit: bool                                                   │
│       - Critical: bool                                              │
│       - AttackRoll, FinalAC                                         │
└─────────────────────────────────────────────────────────────────────┘
                              ↓ (if hit)
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 3: RESOLVE DAMAGE (see ADR-0026)                             │
│                                                                     │
│  [Reaction Window - BEFORE damage resolution]                       │
│       - Uncanny Dodge: "Halve incoming damage"                      │
│       - Consumes reactor's reaction                                 │
│       ↓                                                             │
│  ResolveDamage chain published                                      │
│       - Sneak Attack adds dice (if eligible AND hit)                │
│       - Rage adds flat bonus                                        │
│       - Critical doubles dice                                       │
│       ↓                                                             │
│  ApplyDamage (character mutated in gamectx)                         │
│       ↓                                                             │
│  DamageApplied notification                                         │
│       ↓                                                             │
│  [Reaction Window - AFTER damage]                                   │
│       - Hellish Rebuke: "You hurt me, take fire damage"             │
│       - Consumes reactor's reaction + spell slot                    │
└─────────────────────────────────────────────────────────────────────┘
```

### Reaction Resource

Reactions are tracked as a `RecoverableResource`:

```go
type ReactionResource struct {
    Available bool
    ResetsOn  ResetTrigger // TurnStart
}

// Check and consume atomically
func (r *ReactionResource) TryUse() bool {
    if !r.Available {
        return false
    }
    r.Available = false
    return true
}
```

Features that use reactions check availability before triggering:

```go
// Shield spell subscribes to AttackRolled
AttackRolled.On(bus).Subscribe(func(ctx context.Context, event *AttackRolledEvent) {
    if event.TargetID != s.CasterID {
        return
    }

    // Would this hit without Shield?
    if event.AttackTotal < event.TargetAC {
        return // Already a miss, don't waste reaction
    }

    // Would Shield make it miss?
    if event.AttackTotal >= event.TargetAC + 5 {
        return // Still hits even with +5 AC, don't waste it
    }

    // Check reaction available
    caster, _ := gamectx.GetCharacter(ctx, s.CasterID)
    if !caster.Reaction.TryUse() {
        return // Already used reaction this round
    }

    // Check spell slot
    if !caster.Resources.TryConsume("spell_slot_1", 1) {
        return // No spell slots
    }

    // Modify the event - add AC bonus
    event.TargetAC += 5
})
```

### Attack of Opportunity

AoO triggers on movement, not attacks. It subscribes to movement events:

```go
// Entity movement is published step-by-step
MovementStep.On(bus).Subscribe(func(ctx context.Context, event *MovementStepEvent) {
    // Did entity leave a threatened square?
    if !s.ThreatenedSquares.Contains(event.FromPosition) {
        return
    }
    if s.ThreatenedSquares.Contains(event.ToPosition) {
        return // Still in threatened area
    }

    // Check reaction available
    threatener, _ := gamectx.GetCharacter(ctx, s.CharacterID)
    if !threatener.Reaction.TryUse() {
        return
    }

    // Make the attack - goes through full attack flow
    combat.ResolveAttack(ctx, bus, &ResolveAttackInput{
        AttackerID: s.CharacterID,
        TargetID:   event.EntityID,
        IsAoO:      true, // Sentinel checks this
    })
})
```

**Sentinel feat** modifies this:
- Subscribes to AttackResolved for AoO attacks
- If AoO hits, sets target's speed to 0 (event modifier)

### Key Design Elements

1. **`combat.ResolveAttack(ctx, bus, input)`** - Orchestrates phases 1-2
2. **`combat.DealDamage(ctx, bus, input)`** - Orchestrates phase 3 (ADR-0026)
3. **Reaction windows are events** - Not special infrastructure
4. **Events are mutable during chain** - Reactions modify AC, roll, etc.
5. **Reaction consumption is atomic** - TryUse returns false if already used

### Event Types

```go
// Phase 0
type AttackDeclaredEvent struct {
    AttackerID   string
    TargetID     string
    WeaponID     string
    AttackType   AttackType // Melee, Ranged, Spell
}

// Phase 1 (chain)
type ResolveAttackEvent struct {
    AttackerID        string
    TargetID          string
    AttackBonus       int
    HasAdvantage      bool
    HasDisadvantage   bool
    SneakAttackEligible bool
    // ... modifiers add to these
}

// Phase 2
type AttackRolledEvent struct {
    AttackerID   string
    TargetID     string
    NaturalRoll  int      // The d20 result
    AttackTotal  int      // Roll + modifiers
    TargetAC     int      // Can be modified by reactions
    IsCritical   bool
}

type AttackResolvedEvent struct {
    AttackerID   string
    TargetID     string
    Hit          bool
    Critical     bool
    AttackRoll   int
    FinalAC      int
}

// Phase 3 - see ADR-0026
```

## Consequences

### Positive

- **Clear phases** - Each step has defined inputs/outputs
- **Reactions are just subscribers** - No special reaction infrastructure
- **Composable** - Shield, Cutting Words, Lucky all work the same way
- **Sneak Attack works** - Eligibility determined before roll, damage added after hit
- **AoO is a real attack** - Goes through full flow, can crit, triggers reactions

### Negative

- **Multiple events per attack** - More ceremony than simple "roll to hit"
- **Mutable events** - Must be careful about modification order
- **Movement needs step-by-step events** - Can't just teleport entities

### Neutral

- **Reaction timing is explicit** - Each window is a distinct event
- **Some reactions are "automatic"** - Shield, Uncanny Dodge fire without player input in tactical AI

## Alternatives Considered

### A: Single ResolveAttack event with all phases

One big event that goes through all stages.

**Rejected because:**
- Reactions need to trigger at specific moments
- Shield can't modify AC after damage is already calculated
- Loses clarity about what happens when

### B: Reactions as interceptors, not subscribers

Special "interceptor" pattern that can halt/modify flow.

**Rejected because:**
- Unnecessary complexity - events can be mutable
- Subscribers already have the power to modify events
- Consistency with rest of event system

### C: No step-by-step movement

Movement is atomic - entity teleports from A to B.

**Rejected because:**
- AoO requires knowing when entity leaves threatened squares
- Difficult terrain, traps need step-by-step
- Movement already complex (dash, difficult terrain, flying)

## Example: Rogue Sneak Attack Flow

```go
// 1. Rogue attacks goblin
combat.ResolveAttack(ctx, bus, &ResolveAttackInput{
    AttackerID: rogueID,
    TargetID:   goblinID,
    WeaponID:   daggerID,
})

// 2. ResolveAttack chain executes
//    - SneakAttack subscriber checks: ally adjacent to goblin? YES
//    - Marks event.SneakAttackEligible = true

// 3. Roll happens: natural 18 + 7 = 25 vs AC 13 = HIT

// 4. AttackResolved published with Hit=true, Critical=false

// 5. DealDamage called (since hit)
combat.DealDamage(ctx, bus, &DealDamageInput{
    AttackerID: rogueID,
    TargetID:   goblinID,
    Source:     DamageSourceAttack,
    Instances:  []DamageInstance{{Type: Piercing, Dice: "1d4+4"}},
    Context: &AttackContext{
        SneakAttackEligible: true, // Passed from resolve phase
        Critical:            false,
    },
})

// 6. ResolveDamage chain executes
//    - SneakAttack subscriber sees eligible=true, adds 2d6
//    - Final instances: [{Piercing, "1d4+4"}, {Piercing, "2d6"}]

// 7. Damage applied, goblin HP reduced
```

## Example: Shield Spell Reaction

```go
// 1. Goblin attacks wizard, rolls 15 + 4 = 19

// 2. AttackRolled published: {AttackTotal: 19, TargetAC: 14}

// 3. Shield subscriber fires:
//    - 19 >= 14, would hit
//    - 19 < 14 + 5 = 19, Shield would make it miss!
//    - wizard.Reaction.TryUse() = true
//    - wizard.Resources.TryConsume("spell_slot_1", 1) = true
//    - event.TargetAC += 5 → now 19

// 4. Hit determination: 19 >= 19? NO (ties go to defender with Shield's wording)
//    Actually in 5e ties go to attacker, so 19 >= 19 = hit
//    Let me reconsider... Shield says AC becomes 19, attack is 19, that's a hit
//    Wizard might choose not to cast it! Need decision logic.

// Better: Shield subscriber checks if it would GUARANTEE a miss
func shouldCastShield(attackTotal, currentAC int) bool {
    return attackTotal >= currentAC && attackTotal < currentAC + 5
}
```

## Related ADRs

- **ADR-0024**: ChainedTopic event system
- **ADR-0025**: gamectx for entity lookup
- **ADR-0026**: Damage application via event chain
