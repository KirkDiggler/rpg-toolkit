# Implementing Spells - Complete Guide

## The Magic: Spells Are Just Actions With Extra Rules

```go
// Spells are Actions with spell-specific patterns
fireball := &Fireball{
    id:        "fireball-spell",
    spellSlot: caster.GetSpellSlots(3), // 3rd level spell
}

// Cast it like any action
if err := fireball.CanActivate(ctx, caster, FireballInput{
    Center:    spatial.Position{X: 30, Y: 20},
    SlotLevel: 3,
}); err == nil {
    fireball.Activate(ctx, caster, input)
    // Boom! Events handle the rest
}
```

## What Makes Spells Special?

Spells have unique requirements beyond normal actions:

1. **Spell Slots**: Limited resource system by level
2. **Concentration**: Only one at a time
3. **Components**: Verbal, somatic, material requirements
4. **Upcasting**: Higher slots = stronger effects
5. **Saving Throws**: Targets can resist
6. **Areas of Effect**: Multiple targets in zones
7. **Rituals**: Cast without using slots (takes longer)

## Basic Spell Structure

```go
type Spell struct {
    // Identity
    id          string
    name        string
    level       int      // 0 for cantrips, 1-9 for spells
    school      string   // "evocation", "abjuration", etc.
    
    // Components
    verbal      bool
    somatic     bool
    material    string   // Description of materials
    
    // Casting
    castingTime string   // "1 action", "1 minute", "1 hour"
    range       int      // In feet, 0 for touch, -1 for self
    duration    string   // "instantaneous", "1 minute", "concentration, up to 10 minutes"
    ritual      bool     // Can be cast as ritual
    
    // Effects
    damage      string   // Dice expression, if damage spell
    saveType    string   // "dexterity", "wisdom", etc.
    saveDC      int      // Difficulty class for saves
    
    // Resources
    spellSlots  map[int]*resource.Resource // Slots by level
}
```

## Implementing Different Spell Types

### Damage Spell: Fireball

```go
type FireballInput struct {
    Center    spatial.Position
    SlotLevel int  // For upcasting
}

type Fireball struct {
    Spell
}

func NewFireball() *Fireball {
    return &Fireball{
        Spell: Spell{
            id:       uuid.New(),
            name:     "Fireball",
            level:    3,
            school:   "evocation",
            verbal:   true,
            somatic:  true,
            material: "tiny ball of bat guano and sulfur",
            range:    150,
            damage:   "8d6", // Base damage
            saveType: "dexterity",
        },
    }
}

func (f *Fireball) CanActivate(ctx context.Context, caster Entity, input FireballInput) error {
    // Check spell slot
    if input.SlotLevel < f.level {
        return fmt.Errorf("fireball requires at least level %d slot", f.level)
    }
    
    slot := f.spellSlots[input.SlotLevel]
    if !slot.CanConsume(1) {
        return fmt.Errorf("no level %d spell slots available", input.SlotLevel)
    }
    
    // Check range
    distance := spatial.Distance(caster.Position(), input.Center)
    if distance > float64(f.range) {
        return fmt.Errorf("target out of range (%.0f ft > %d ft)", distance, f.range)
    }
    
    // Check components
    if f.verbal && isSilenced(caster) {
        return errors.New("cannot cast verbal spells while silenced")
    }
    
    return nil
}

func (f *Fireball) Activate(ctx context.Context, caster Entity, input FireballInput) error {
    // Consume spell slot
    f.spellSlots[input.SlotLevel].Consume(1)
    
    // Calculate damage (upcasting adds 1d6 per level above 3rd)
    extraDice := input.SlotLevel - f.level
    damage := fmt.Sprintf("%dd6", 8 + extraDice)
    damageRoll := dice.New(damage).Roll()
    
    // Find all targets in 20ft radius
    targets := spatial.EntitiesWithin(input.Center, 20.0)
    
    // Apply damage to each target
    for _, target := range targets {
        // Publish save event
        bus.Publish(events.New("save.requested", map[string]any{
            "target":   target.GetID(),
            "type":     f.saveType,
            "dc":       f.saveDC,
            "source":   f.id,
            "onSuccess": "half_damage",
            "damage":   damageRoll,
        }))
    }
    
    return nil
}
```

### Buff Spell: Bless

```go
type BlessInput struct {
    Targets []core.Entity // Up to 3 creatures
}

type Bless struct {
    Spell
}

func (b *Bless) CanActivate(ctx context.Context, caster Entity, input BlessInput) error {
    // Validate target count
    if len(input.Targets) == 0 || len(input.Targets) > 3 {
        return fmt.Errorf("bless targets 1-3 creatures, got %d", len(input.Targets))
    }
    
    // Check spell slot
    if !b.spellSlots[1].CanConsume(1) {
        return errors.New("no 1st level spell slots available")
    }
    
    // Check concentration
    if hasConcentration(caster) {
        return errors.New("already concentrating on a spell")
    }
    
    // Check range (30 ft)
    for _, target := range input.Targets {
        if distance(caster, target) > 30 {
            return fmt.Errorf("%s is out of range", target.GetID())
        }
    }
    
    return nil
}

func (b *Bless) Activate(ctx context.Context, caster Entity, input BlessInput) error {
    b.spellSlots[1].Consume(1)
    
    // Create bless effect
    effect := &BlessEffect{
        id:       uuid.New(),
        source:   b.id,
        caster:   caster.GetID(),
        duration: 1 * time.Minute, // Concentration, up to 1 minute
    }
    
    // Apply to each target
    for _, target := range input.Targets {
        bus.Publish(events.New("effect.applied", map[string]any{
            "effect":   effect,
            "target":   target.GetID(),
            "duration": "1 minute",
            "concentration": true,
        }))
    }
    
    // Start concentration
    startConcentration(caster, b.id, effect.id)
    
    return nil
}

// Bless effect modifies rolls
type BlessEffect struct {
    id     string
    source string
    caster string
}

func (b *BlessEffect) ModifyAttackRoll(roll *dice.Roll) {
    roll.AddDice("1d4") // Bless adds 1d4 to attacks
}

func (b *BlessEffect) ModifySavingThrow(roll *dice.Roll) {
    roll.AddDice("1d4") // And to saves
}
```

### Utility Spell: Misty Step

```go
type MistyStepInput struct {
    Destination spatial.Position
}

type MistyStep struct {
    Spell
}

func (m *MistyStep) CanActivate(ctx context.Context, caster Entity, input MistyStepInput) error {
    // Check spell slot (2nd level)
    if !m.spellSlots[2].CanConsume(1) {
        return errors.New("no 2nd level spell slots available")
    }
    
    // Check range (30 ft teleport)
    if distance(caster.Position(), input.Destination) > 30 {
        return errors.New("destination out of range (30 ft)")
    }
    
    // Check destination is unoccupied
    if !spatial.IsUnoccupied(input.Destination) {
        return errors.New("destination is occupied")
    }
    
    // Check line of sight to destination
    if !hasLineOfSight(caster.Position(), input.Destination) {
        return errors.New("cannot see destination")
    }
    
    return nil
}

func (m *MistyStep) Activate(ctx context.Context, caster Entity, input MistyStepInput) error {
    m.spellSlots[2].Consume(1)
    
    oldPos := caster.Position()
    
    // Teleport (instant movement)
    bus.Publish(events.New("entity.teleported", map[string]any{
        "entity": caster.GetID(),
        "from":   oldPos,
        "to":     input.Destination,
        "spell":  m.id,
    }))
    
    // Visual effect
    bus.Publish(events.New("effect.visual", map[string]any{
        "type":     "mist",
        "position": oldPos,
        "duration": "instant",
    }))
    
    return nil
}
```

### Cantrip: Fire Bolt

```go
type FireBoltInput struct {
    Target core.Entity
}

type FireBolt struct {
    Spell
    characterLevel int // For scaling
}

func (f *FireBolt) CanActivate(ctx context.Context, caster Entity, input FireBoltInput) error {
    // Cantrips don't use spell slots!
    
    // Check range (120 ft)
    if distance(caster, input.Target) > 120 {
        return errors.New("target out of range")
    }
    
    // Check line of sight
    if !hasLineOfSight(caster, input.Target) {
        return errors.New("no line of sight to target")
    }
    
    return nil
}

func (f *FireBolt) Activate(ctx context.Context, caster Entity, input FireBoltInput) error {
    // Cantrip damage scales with character level
    numDice := 1
    if f.characterLevel >= 5 {
        numDice = 2
    }
    if f.characterLevel >= 11 {
        numDice = 3
    }
    if f.characterLevel >= 17 {
        numDice = 4
    }
    
    damage := dice.New(fmt.Sprintf("%dd10", numDice)).Roll()
    
    // Make spell attack
    bus.Publish(events.New("attack.spell", map[string]any{
        "attacker": caster.GetID(),
        "target":   input.Target.GetID(),
        "spell":    f.id,
        "damage":   damage,
        "type":     "fire",
    }))
    
    return nil
}
```

## Concentration Management

```go
type ConcentrationManager struct {
    concentrating map[string]*ConcentrationEffect // caster ID -> effect
}

func (c *ConcentrationManager) StartConcentration(caster Entity, spell, effect string) error {
    if existing := c.concentrating[caster.GetID()]; existing != nil {
        return fmt.Errorf("already concentrating on %s", existing.spell)
    }
    
    c.concentrating[caster.GetID()] = &ConcentrationEffect{
        spell:     spell,
        effect:    effect,
        startTime: time.Now(),
    }
    
    return nil
}

func (c *ConcentrationManager) OnDamageTaken(e events.Event) {
    caster := e.Data["target"].(string)
    damage := e.Data["amount"].(int)
    
    if effect := c.concentrating[caster]; effect != nil {
        // Constitution save to maintain concentration
        dc := max(10, damage/2)
        
        bus.Publish(events.New("save.concentration", map[string]any{
            "caster": caster,
            "dc":     dc,
            "spell":  effect.spell,
        }))
    }
}

func (c *ConcentrationManager) BreakConcentration(caster string) {
    if effect := c.concentrating[caster]; effect != nil {
        // Remove the concentrated effect
        bus.Publish(events.New("effect.removed", map[string]any{
            "effect": effect.effect,
            "reason": "concentration_broken",
        }))
        
        delete(c.concentrating, caster)
    }
}
```

## Spell Slots Management

```go
type SpellSlotManager struct {
    slots map[int]*resource.Resource // level -> resource
}

func NewSpellSlotManager(class string, level int) *SpellSlotManager {
    slots := make(map[int]*resource.Resource)
    
    // Calculate slots based on class and level
    // This is game-specific logic
    slotsPerLevel := getSpellSlotsForClass(class, level)
    
    for spellLevel, count := range slotsPerLevel {
        slots[spellLevel] = resource.New(resource.Config{
            ID:       fmt.Sprintf("spell-slots-%d", spellLevel),
            MaxValue: count,
            Current:  count,
        })
    }
    
    return &SpellSlotManager{slots: slots}
}

func (s *SpellSlotManager) CanCast(level int) bool {
    // Check if any slot of this level or higher is available
    for slotLevel := level; slotLevel <= 9; slotLevel++ {
        if slot, ok := s.slots[slotLevel]; ok && slot.CanConsume(1) {
            return true
        }
    }
    return false
}

func (s *SpellSlotManager) ConsumeSlot(level int) error {
    if slot, ok := s.slots[level]; ok {
        return slot.Consume(1)
    }
    return fmt.Errorf("no spell slot at level %d", level)
}

func (s *SpellSlotManager) OnLongRest() {
    // Restore all spell slots
    for _, slot := range s.slots {
        slot.SetCurrent(slot.Max())
    }
}
```

## Ritual Casting

```go
type RitualSpell interface {
    Action[any]
    CastAsRitual(ctx context.Context, caster Entity) error
}

type DetectMagic struct {
    Spell
}

func (d *DetectMagic) CastAsRitual(ctx context.Context, caster Entity) error {
    // Ritual casting takes 10 minutes longer
    bus.Publish(events.New("ritual.started", map[string]any{
        "caster":   caster.GetID(),
        "spell":    d.id,
        "duration": "10 minutes",
    }))
    
    // Schedule completion
    time.AfterFunc(10*time.Minute, func() {
        // No spell slot consumed!
        d.applyEffect(caster)
    })
    
    return nil
}

func (d *DetectMagic) applyEffect(caster Entity) {
    bus.Publish(events.New("effect.applied", map[string]any{
        "effect": "detect_magic",
        "target": caster.GetID(),
        "duration": "10 minutes",
    }))
}
```

## Area of Effect Patterns

### Sphere/Circle
```go
func targetsInSphere(center spatial.Position, radius float64) []Entity {
    return spatial.EntitiesWithin(center, radius)
}
```

### Cone
```go
func targetsInCone(origin spatial.Position, direction float64, length, angle float64) []Entity {
    var targets []Entity
    
    for _, entity := range spatial.AllEntities() {
        pos := entity.Position()
        
        // Check distance
        dist := distance(origin, pos)
        if dist > length {
            continue
        }
        
        // Check angle
        angleToTarget := angleBetween(origin, pos)
        angleDiff := abs(angleToTarget - direction)
        
        if angleDiff <= angle/2 {
            targets = append(targets, entity)
        }
    }
    
    return targets
}
```

### Line
```go
func targetsInLine(start, end spatial.Position, width float64) []Entity {
    var targets []Entity
    
    for _, entity := range spatial.AllEntities() {
        if distanceToLine(entity.Position(), start, end) <= width/2 {
            targets = append(targets, entity)
        }
    }
    
    return targets
}
```

## Metamagic (Sorcerer Features)

```go
type Metamagic interface {
    ModifySpell(spell Spell, input any) (Spell, any)
}

type TwinnedSpell struct {
    sorceryPoints *resource.Resource
}

func (t *TwinnedSpell) CanApply(spell Spell) error {
    if !spell.targetsSingle() {
        return errors.New("can only twin single-target spells")
    }
    
    cost := spell.level
    if cost == 0 {
        cost = 1 // Cantrips cost 1
    }
    
    if !t.sorceryPoints.CanConsume(cost) {
        return errors.New("insufficient sorcery points")
    }
    
    return nil
}

func (t *TwinnedSpell) ModifySpell(spell Spell, input any) (Spell, any) {
    // Double the targets
    switch i := input.(type) {
    case FireBoltInput:
        // Can't twin - need second target
        // This would be handled at input creation
    }
    
    return spell, input
}
```

## Testing Spells

### Unit Testing

```go
func TestFireball_Damage(t *testing.T) {
    fireball := NewFireball()
    caster := &MockCaster{position: spatial.Position{X: 0, Y: 0}}
    
    tests := []struct {
        name      string
        slotLevel int
        wantDice  string
    }{
        {"base level", 3, "8d6"},
        {"4th level", 4, "9d6"},
        {"5th level", 5, "10d6"},
        {"9th level", 9, "14d6"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            input := FireballInput{
                Center:    spatial.Position{X: 30, Y: 30},
                SlotLevel: tt.slotLevel,
            }
            
            // Mock dice to verify correct damage
            mockDice := dice.NewMock()
            mockDice.ExpectRoll(tt.wantDice)
            
            err := fireball.Activate(ctx, caster, input)
            require.NoError(t, err)
            
            mockDice.AssertExpectations(t)
        })
    }
}
```

### Integration Testing

```go
func TestConcentration_Breaking(t *testing.T) {
    bus := events.NewBus()
    concentrationMgr := NewConcentrationManager(bus)
    
    caster := &MockCaster{id: "wizard-1"}
    bless := NewBless()
    
    // Start concentration
    err := bless.Activate(ctx, caster, BlessInput{
        Targets: []Entity{target1, target2},
    })
    require.NoError(t, err)
    
    // Take damage
    bus.Publish(events.New("damage.taken", map[string]any{
        "target": caster.GetID(),
        "amount": 22, // DC 11 concentration save
    }))
    
    // Verify concentration save requested
    // Mock failed save
    bus.Publish(events.New("save.failed", map[string]any{
        "entity": caster.GetID(),
        "type":   "concentration",
    }))
    
    // Verify bless effect removed
    assert.False(t, hasEffect(target1, "bless"))
    assert.False(t, hasEffect(target2, "bless"))
}
```

## Common Spell Patterns

### Save-or-Suck Spells
```go
func (s *HoldPerson) Activate(ctx context.Context, caster Entity, input HoldPersonInput) error {
    bus.Publish(events.New("save.requested", map[string]any{
        "target":   input.Target.GetID(),
        "type":     "wisdom",
        "dc":       s.saveDC,
        "onFail":   "paralyzed",
        "duration": "1 minute",
        "concentration": true,
    }))
    
    return nil
}
```

### Summoning Spells
```go
func (s *SummonElemental) Activate(ctx context.Context, caster Entity, input SummonInput) error {
    // Create summoned entity
    elemental := &Elemental{
        id:       uuid.New(),
        summoner: caster.GetID(),
        duration: 1 * time.Hour,
    }
    
    // Place in world
    bus.Publish(events.New("entity.summoned", map[string]any{
        "entity":   elemental,
        "summoner": caster.GetID(),
        "position": input.Position,
        "duration": "1 hour",
    }))
    
    return nil
}
```

### Healing Spells
```go
func (c *CureWounds) Activate(ctx context.Context, caster Entity, input CureWoundsInput) error {
    // Healing scales with slot level
    dice := fmt.Sprintf("%dd8", input.SlotLevel)
    healing := dice.New(dice).Roll() + caster.SpellcastingModifier()
    
    bus.Publish(events.New("healing.applied", map[string]any{
        "source": c.id,
        "healer": caster.GetID(),
        "target": input.Target.GetID(),
        "amount": healing,
    }))
    
    return nil
}
```

## Spell Components Checking

```go
func checkComponents(spell Spell, caster Entity) error {
    if spell.verbal && hasCondition(caster, "silenced") {
        return errors.New("cannot cast verbal spells while silenced")
    }
    
    if spell.somatic && hasCondition(caster, "restrained") {
        return errors.New("cannot perform somatic components while restrained")
    }
    
    if spell.material != "" && !hasMaterials(caster, spell.material) {
        // Check for spell focus or component pouch
        if !hasSpellFocus(caster) {
            return fmt.Errorf("missing material components: %s", spell.material)
        }
    }
    
    return nil
}
```

## Counterspell Implementation

```go
type CounterspellInput struct {
    TargetSpell string // ID of spell being cast
    SlotLevel   int    // Level to cast counterspell at
}

type Counterspell struct {
    Spell
}

func (c *Counterspell) CanActivate(ctx context.Context, caster Entity, input CounterspellInput) error {
    // Must be a reaction to a spell being cast
    if !isSpellBeingCast(input.TargetSpell) {
        return errors.New("no spell to counter")
    }
    
    // Check range (60 ft)
    targetCaster := getSpellCaster(input.TargetSpell)
    if distance(caster, targetCaster) > 60 {
        return errors.New("target spell caster out of range")
    }
    
    return nil
}

func (c *Counterspell) Activate(ctx context.Context, caster Entity, input CounterspellInput) error {
    targetLevel := getSpellLevel(input.TargetSpell)
    
    if targetLevel <= input.SlotLevel {
        // Automatic success
        bus.Publish(events.New("spell.countered", map[string]any{
            "spell":    input.TargetSpell,
            "counter":  c.id,
            "caster":   caster.GetID(),
        }))
    } else {
        // Ability check required
        dc := 10 + targetLevel
        bus.Publish(events.New("check.requested", map[string]any{
            "entity":   caster.GetID(),
            "ability":  "spellcasting",
            "dc":       dc,
            "onSuccess": "counter_spell",
            "target":   input.TargetSpell,
        }))
    }
    
    return nil
}
```

## Checklist for New Spells

- [ ] Spell struct with all properties defined
- [ ] Input type with required parameters
- [ ] Spell level and school set correctly
- [ ] Components (V, S, M) specified
- [ ] Range validation in CanActivate
- [ ] Spell slot consumption (except cantrips)
- [ ] Concentration check if applicable
- [ ] Saving throw or attack roll logic
- [ ] Damage/effect calculation
- [ ] Upcasting logic if applicable
- [ ] Duration and cleanup for lasting effects
- [ ] Area of effect targeting if applicable
- [ ] Event publishing for all effects
- [ ] Unit tests for validation
- [ ] Integration tests with saves/concentration

## Remember

Spells are Actions with extra complexity. The toolkit provides the infrastructure (Actions, resources, events), while your game provides the specific spell implementations. Keep spells as self-contained Actions that communicate through events!