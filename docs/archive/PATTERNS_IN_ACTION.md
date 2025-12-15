# Patterns in Action: How Everything Works Together

This document shows concrete examples of how RPG Toolkit's architectural patterns combine to create complex game mechanics with elegant simplicity.

## Complete Combat Example: All Patterns Working Together

Let's walk through a complete combat scenario showing how typed topics, relationships, actions, and lazy evaluation all work in harmony.

### The Scenario
A party is in combat:
- **Cleric** has cast Bless on Fighter and Rogue (concentration)
- **Fighter** is raging (barbarian multiclass)
- **Wizard** has Mage Armor active
- **Rogue** is hidden (advantage on next attack)

The Fighter attacks a goblin. Here's how all the patterns orchestrate this single attack:

```go
// ========================================
// SETUP: Active Effects via Relationships
// ========================================

// Cleric's Bless spell (Action[T] pattern)
blessAction := cleric.GetAction("bless")
blessAction.Activate(ctx, cleric, BlessInput{
    Targets:   []Entity{fighter, rogue},
    SlotLevel: 1,
})

// This creates a Concentration Relationship:
// Cleric → [BlessFighter, BlessRogue]
// If cleric loses concentration, both effects vanish

// Fighter's Rage (Action[T] with resource consumption)
rageAction := fighter.GetAction("rage")
rageAction.Activate(ctx, fighter, EmptyInput{})
// Creates RageDamageBonus and RageResistance effects

// ========================================
// THE ATTACK: Typed Topics + Chains
// ========================================

// Fighter declares attack (simple action)
attackAction := fighter.GetAction("attack")
attackAction.Activate(ctx, fighter, AttackInput{
    Target: goblin,
    Weapon: "greataxe",
})

// ========================================
// INSIDE THE ATTACK ACTION
// ========================================

func (a *AttackAction) Activate(ctx context.Context, fighter Entity, input AttackInput) error {
    // Step 1: Create the attack event
    attackEvent := AttackEvent{
        Attacker: fighter.GetID(),
        Target:   input.Target.GetID(),
        Weapon:   input.Weapon,
    }

    // Step 2: Publish to typed topic with chain
    // AttackChain is defined in combat package as:
    // var AttackChain = events.DefineChainedTopic[AttackEvent]("combat.attack")
    attacks := combat.AttackChain.On(bus)  // Type-safe connection!

    // Step 3: Build the chain of modifiers
    chain := NewChain[AttackEvent]()

    // All active effects subscribe and modify the chain:

    // Bless Effect (via relationship from cleric)
    // Subscribes at StageConditions, adds +1d4 to hit

    // Rage Damage (from fighter's rage)
    // Subscribes at StageConditions, adds +2 damage

    // Weapon Enhancement (if any)
    // Subscribes at StageEquipment, adds magic bonus

    finalChain, _ := attacks.PublishWithChain(ctx, attackEvent, chain)

    // Step 4: Execute the chain in order
    modifiedAttack, _ := finalChain.Execute(ctx, attackEvent)

    // The chain executed in this order:
    // 1. StageBase (100):      Calculate base attack bonus
    // 2. StageFeatures (200):  Apply class features
    // 3. StageConditions (300): Apply Bless (+1d4) and Rage damage (+2)
    // 4. StageEquipment (400): Apply weapon bonuses
    // 5. StageFinal (500):     Apply advantage/disadvantage

    return nil
}

// ========================================
// LAZY DICE EVALUATION
// ========================================

// The attack roll is built lazily:
attackRoll := dice.D20(1)                    // Base d20
    .Plus(dice.Const(fighter.AttackBonus())) // +7 proficiency
    .Plus(dice.D4(1))                         // +1d4 from Bless (lazy!)
    .WithAdvantage()                          // Rogue's hidden advantage

// Nothing has rolled yet!
// When we need the result:
result := attackRoll.GetValue()  // NOW everything rolls at once

// If this was a critical hit:
damage := dice.D12(1)          // Greataxe base
    .Plus(dice.Const(3))        // Strength modifier
    .Plus(dice.Const(2))        // Rage bonus
    .WithCritical()             // Doubles the dice

damageDone := damage.GetValue()  // Rolls: (1d12 + 3 + 2) × 2

// ========================================
// CONCENTRATION CHECK (if cleric is hit)
// ========================================

// Goblin attacks cleric
goblinAttacks := combat.AttackTopic.On(bus)
goblinAttacks.Publish(ctx, AttackEvent{
    Attacker: goblin.GetID(),
    Target:   cleric.GetID(),
    Damage:   15,
})

// This triggers concentration check:
concentrationSaves := saves.ConcentrationTopic.On(bus)
concentrationSaves.Subscribe(ctx, func(ctx context.Context, e ConcentrationSaveEvent) error {
    if e.Success {
        // Concentration maintained
        return nil
    }

    // Failed! Break concentration relationship
    relationshipMgr.BreakAllRelationships(cleric)
    // This automatically removes Bless from Fighter AND Rogue!

    return nil
})
```

## Example: Complex Spell Interaction

Here's how multiple patterns enable complex spell interactions:

```go
// ========================================
// SCENARIO: Wizard casts Haste on Fighter
// who is already Blessed and Raging
// ========================================

// Wizard casts Haste (concentration)
hasteAction := wizard.GetAction("haste")
hasteAction.Activate(ctx, wizard, SpellInput{
    Target:    fighter,
    SlotLevel: 3,
})

// This creates:
// 1. HasteEffect on Fighter (extra action, +2 AC, advantage on Dex saves)
// 2. Concentration relationship: Wizard → [HasteFighter]

// Fighter now has THREE active effects:
// - Bless (from Cleric's concentration)
// - Rage (self-maintained, uses rage resource)
// - Haste (from Wizard's concentration)

// ========================================
// FIGHTER'S ENHANCED ATTACK
// ========================================

// With Haste, fighter gets an extra action
for i := 0; i < 2; i++ {  // Two attacks!

    // Build attack with ALL modifiers
    attack := NewAttackEvent(fighter, goblin)

    // The typed topic system ensures ALL effects apply:
    attackChain := combat.AttackChain.On(bus)

    // Each effect adds its modifier at the right stage:
    // - Bless:    +1d4 to hit (StageConditions)
    // - Rage:     +2 damage (StageConditions)
    // - Haste:    Advantage on attacks (StageFinal)

    result, _ := attackChain.PublishWithChain(ctx, attack, chain)
}

// ========================================
// CATASTROPHIC FAILURE CASCADE
// ========================================

// Wizard loses concentration (failed save)
relationshipMgr.BreakAllRelationships(wizard)

// Haste ends, triggering lethargy:
lethargicEffect := NewLethargicCondition(fighter)
lethargicEffect.Apply(bus)  // Fighter loses next turn!

// But Bless remains (different concentration source)
// And Rage remains (not concentration-based)
```

## Example: Aura and Movement Integration

Showing how relationships handle dynamic positioning:

```go
// ========================================
// PALADIN'S AURA OF PROTECTION
// ========================================

// Paladin has a 10-foot aura giving +CHA to saves
paladinAura := NewAuraEffect(paladin, 10, paladin.CharismaModifier())

// Create aura relationships for nearby allies
func UpdatePaladinAura(paladin Entity) {
    nearbyAllies := spatial.FindAlliesWithin(paladin.Position(), 10.0)

    currentAuras := relationshipMgr.GetRelationshipsByType(paladin, RelationshipAura)

    // Remove auras for allies who moved away
    for _, rel := range currentAuras {
        for _, condition := range rel.Conditions {
            target := condition.GetTarget()
            if !nearbyAllies.Contains(target) {
                relationshipMgr.BreakRelationship(rel)
            }
        }
    }

    // Add auras for allies who moved closer
    for _, ally := range nearbyAllies {
        if !hasAuraFrom(ally, paladin) {
            auraBonus := NewAuraBonus(ally, paladin)
            auraBonus.Apply(bus)

            relationshipMgr.CreateRelationship(
                RelationshipAura,
                paladin,
                []Condition{auraBonus},
                map[string]any{"range": 10},
            )
        }
    }
}

// ========================================
// MOVEMENT TRIGGERS AURA UPDATES
// ========================================

// Subscribe to movement events
movement := spatial.MovementTopic.On(bus)
movement.Subscribe(ctx, func(ctx context.Context, e MovementEvent) error {
    // Check all aura sources
    for _, source := range getAuraSources() {
        UpdatePaladinAura(source)
    }
    return nil
})

// ========================================
// SAVING THROW WITH AURA
// ========================================

// Rogue (within 10 feet of Paladin) makes a save
saveEvent := SaveEvent{
    Entity:   rogue.GetID(),
    Type:     "dexterity",
    DC:       15,
}

// Publish to save chain
saves := mechanics.SaveChain.On(bus)
chain := NewChain[SaveEvent]()

// Paladin's aura effect subscribes and adds modifier
// Because Rogue has aura relationship from Paladin:
chain.Add(StageConditions, "paladin_aura", func(ctx context.Context, e SaveEvent) (SaveEvent, error) {
    e.Bonus += paladin.CharismaModifier()  // +3 from Paladin's CHA
    return e, nil
})

// Rogue also has Bless active:
chain.Add(StageConditions, "bless", func(ctx context.Context, e SaveEvent) (SaveEvent, error) {
    e.BonusDice = append(e.BonusDice, "1d4")  // +1d4 from Bless
    return e, nil
})

result, _ := saves.PublishWithChain(ctx, saveEvent, chain)
// Rogue rolls: 1d20 + DEX + 3 (aura) + 1d4 (bless)
```

## Example: Resource Management and Actions

How resources, actions, and effects interact:

```go
// ========================================
// RESOURCE-BASED ABILITIES
// ========================================

// Fighter's Action Surge (limited resource)
type ActionSurgeAction struct {
    uses *resource.Resource  // 1 use per short rest
}

func (a *ActionSurgeAction) CanActivate(ctx context.Context, fighter Entity, input EmptyInput) error {
    if !a.uses.CanConsume(1) {
        return errors.New("no action surge uses remaining")
    }
    return nil
}

func (a *ActionSurgeAction) Activate(ctx context.Context, fighter Entity, input EmptyInput) error {
    a.uses.Consume(1)

    // Grant extra action
    extraAction := NewExtraActionEffect(fighter)
    extraAction.Apply(bus)

    return nil
}

// ========================================
// SPELL SLOTS AS RESOURCES
// ========================================

// Wizard's spell slots
type SpellSlotManager struct {
    slots map[int]*resource.Resource  // Level → Resource
}

// Counterspell uses variable slot level
type CounterspellAction struct {
    slots *SpellSlotManager
}

func (c *CounterspellAction) Activate(ctx context.Context, wizard Entity, input CounterspellInput) error {
    // Use specified slot level (or minimum 3rd)
    slotLevel := max(3, input.SlotLevel)

    if !c.slots.CanUseSlot(slotLevel) {
        return errors.New("no spell slot available")
    }

    c.slots.ConsumeSlot(slotLevel)

    // Counter the spell if slot level >= spell level
    targetSpellLevel := getSpellLevel(input.TargetSpell)

    if slotLevel >= targetSpellLevel {
        // Automatic success!
        counterEvent := SpellCounteredEvent{
            Spell:   input.TargetSpell,
            Counter: wizard.GetID(),
        }

        counters := magic.CounterTopic.On(bus)
        counters.Publish(ctx, counterEvent)
    } else {
        // Need ability check
        // DC = 10 + spell level
        saveEvent := SaveEvent{
            Entity: wizard.GetID(),
            Type:   "spellcasting",
            DC:     10 + targetSpellLevel,
        }

        saves := mechanics.SaveTopic.On(bus)
        saves.Publish(ctx, saveEvent)
    }

    return nil
}

// ========================================
// RESOURCE REGENERATION
// ========================================

// On short rest
shortRest := rest.ShortRestTopic.On(bus)
shortRest.Subscribe(ctx, func(ctx context.Context, e RestEvent) error {
    // Restore action surge
    fighter.GetResource("action_surge").RestoreToMax()

    // Restore some spell slots (Wizard's Arcane Recovery)
    if wizard.HasFeature("arcane_recovery") {
        wizard.RestoreSpellSlots(wizard.Level() / 2)
    }

    return nil
})
```

## Example: Death and Cleanup

How the patterns handle entity death and effect cleanup:

```go
// ========================================
// DEATH TRIGGERS CLEANUP CASCADE
// ========================================

// When an entity dies
death := lifecycle.DeathTopic.On(bus)
death.Subscribe(ctx, func(ctx context.Context, e DeathEvent) error {
    entity := e.Entity

    // Step 1: Break all relationships where entity is source
    relationshipMgr.BreakAllRelationships(entity)
    // - If it was concentrating, those effects end
    // - If it had auras, those effects end
    // - If it was channeling, those effects end

    // Step 2: Check dependent relationships
    deps := relationshipMgr.GetDependentRelationships(entity)
    for _, dep := range deps {
        // E.g., Spiritual Weapon disappears when caster dies
        relationshipMgr.BreakRelationship(dep)
    }

    // Step 3: Trigger death saves for special cases
    if entity.HasFeature("death_ward") {
        // Death Ward prevents death once
        deathWard := entity.GetEffect("death_ward")
        deathWard.Remove(bus)

        entity.SetHP(1)  // Survive with 1 HP

        prevented := DeathPreventedEvent{
            Entity: entity.GetID(),
            Source: "death_ward",
        }

        preventions := lifecycle.DeathPreventedTopic.On(bus)
        preventions.Publish(ctx, prevented)

        return nil  // Don't actually die
    }

    return nil
})

// ========================================
// UNCONSCIOUS VS DEAD
// ========================================

// At 0 HP, character goes unconscious
unconscious := lifecycle.UnconsciousTopic.On(bus)
unconscious.Subscribe(ctx, func(ctx context.Context, e UnconsciousEvent) error {
    entity := e.Entity

    // Different from death - concentration breaks but not all effects
    concentrations := relationshipMgr.GetRelationshipsByType(entity, RelationshipConcentration)
    for _, conc := range concentrations {
        relationshipMgr.BreakRelationship(conc)
    }

    // But maintained effects might continue (DM discretion)
    // Auras typically end when unconscious
    auras := relationshipMgr.GetRelationshipsByType(entity, RelationshipAura)
    for _, aura := range auras {
        relationshipMgr.BreakRelationship(aura)
    }

    return nil
})
```

## The Complete Picture: A Full Combat Round

Here's how all patterns orchestrate a complete combat round:

```go
// ========================================
// INITIATIVE AND TURN ORDER
// ========================================

// Roll initiative (lazy dice with modifiers)
initiatives := make(map[Entity]int)
for _, combatant := range combatants {
    roll := dice.D20(1).Plus(dice.Const(combatant.DexModifier()))

    // Alert feat gives +5
    if combatant.HasFeature("alert") {
        roll = roll.Plus(dice.Const(5))
    }

    initiatives[combatant] = roll.GetValue()
}

// Sort by initiative
turnOrder := sortByInitiative(combatants, initiatives)

// ========================================
// PROCESS EACH TURN
// ========================================

for _, activeEntity := range turnOrder {
    // Start of turn events
    turnStart := combat.TurnStartTopic.On(bus)
    turnStart.Publish(ctx, TurnStartEvent{Entity: activeEntity})

    // This triggers:
    // - Regeneration effects
    // - Damage over time effects
    // - Save attempts for ongoing conditions
    // - Resource regeneration (like Legendary Resistance)

    // Check concentration saves if took damage
    if damageThisTurn[activeEntity] > 0 {
        dc := max(10, damageThisTurn[activeEntity]/2)

        save := SaveEvent{
            Entity: activeEntity.GetID(),
            Type:   "constitution",
            DC:     dc,
            Purpose: "concentration",
        }

        saves := mechanics.SaveChain.On(bus)

        // Build save chain with all modifiers
        chain := NewChain[SaveEvent]()

        // War Caster feat gives advantage
        if activeEntity.HasFeature("war_caster") {
            chain.Add(StageFinal, "war_caster", func(ctx context.Context, e SaveEvent) (SaveEvent, error) {
                e.Advantage = true
                return e, nil
            })
        }

        finalChain, _ := saves.PublishWithChain(ctx, save, chain)
        result, _ := finalChain.Execute(ctx, save)

        if !result.Success {
            // Break concentration
            relationshipMgr.BreakAllRelationships(activeEntity)
        }
    }

    // Process actions
    if !activeEntity.IsIncapacitated() {
        // Get available actions
        actions := activeEntity.GetAvailableActions()

        // AI or player chooses action
        chosen := chooseAction(activeEntity, actions)

        // Execute the action (Action[T] pattern)
        chosen.Activate(ctx, activeEntity, chosen.GetInput())
    }

    // End of turn
    turnEnd := combat.TurnEndTopic.On(bus)
    turnEnd.Publish(ctx, TurnEndEvent{Entity: activeEntity})

    // This triggers:
    // - Condition duration checks
    // - Effect expiration
    // - Temporary HP loss
    // - Save attempts for "end of turn" conditions
}

// ========================================
// END OF ROUND CLEANUP
// ========================================

roundEnd := combat.RoundEndTopic.On(bus)
roundEnd.Publish(ctx, RoundEndEvent{Round: currentRound})

// This triggers:
// - Duration countdowns for all effects
// - Lair actions (if applicable)
// - Environmental effects
// - Resource regeneration (like Legendary Actions)
```

## Key Insights from These Examples

1. **No Special Cases**: Everything follows the same patterns
   - Spells are Actions[T]
   - All modifiers go through chains
   - All connections use relationships

2. **Clean Separation**: Each system does one thing well
   - Topics handle events
   - Chains handle ordering
   - Relationships handle lifecycle
   - Actions handle activation

3. **Composition Over Configuration**: Complex behaviors emerge from simple patterns
   - Bless + Rage + Haste = each adds its modifier at the right stage
   - Death = relationships break = effects clean up automatically

4. **Type Safety Throughout**: No runtime surprises
   - Typed topics ensure event data is correct
   - Action[T] ensures inputs are valid
   - Chains preserve type through transformations

5. **Event-Driven Coordination**: Systems stay decoupled
   - Movement system doesn't know about auras
   - Damage system doesn't know about concentration
   - Death system doesn't know about specific effects

This is the power of RPG Toolkit - complex game mechanics emerge naturally from simple, composable patterns.