# D&D 5e Character Calculator Design

## Core Design Principles

### 1. No Nil Without Error
If a method can return nil, it MUST return an error explaining why. This prevents silent failures and makes debugging easier.

### 2. Fail Fast Construction
Validate all required fields during construction. If a character needs a class, the constructor should fail if no class is provided.

### 3. Explicit Over Implicit
Rather than checking if methods return nil, use explicit feature detection methods.

### 4. Separation of Concerns
- **Data Layer**: Interfaces for accessing character data
- **Calculation Layer**: Pure functions that perform calculations
- **Integration Layer**: Adapters between your data model and the calculator

## Interface Design

### Core Interfaces (Required)

```go
// Character represents the minimum required data for calculations
type Character interface {
    // Identity - these never return errors
    ID() string
    Name() string
    
    // Core mechanics - these are REQUIRED and validated at construction
    Level() int
    ProficiencyBonus() int
    Class() Class                    // Never nil - character MUST have a class
    AbilityScores() AbilityScores    // Never nil - character MUST have abilities
}

// Class represents required class data
type Class interface {
    ID() string
    Name() string
    HitDice() int
    SpellcastingAbility() string     // empty string if not a spellcaster
    
    // For saves - returns empty slice if none
    SavingThrowProficiencies() []string
}

// AbilityScores are always required
type AbilityScores interface {
    Strength() int
    Dexterity() int
    Constitution() int
    Intelligence() int
    Wisdom() int
    Charisma() int
}
```

### Optional Features Interface

```go
// OptionalFeatures provides explicit checks for optional data
type OptionalFeatures interface {
    // Race is optional (some games start without choosing race)
    HasRace() bool
    Race() (Race, error) // Returns error if HasRace() is false
    
    // Background is optional
    HasBackground() bool
    Background() (Background, error)
    
    // Equipment might be empty
    HasEquipment() bool
    Equipment() ([]Equipment, error)
    
    // Proficiencies beyond class defaults
    HasAdditionalProficiencies() bool
    AdditionalProficiencies() (Proficiencies, error)
}

// FullCharacter combines required and optional
type FullCharacter interface {
    Character
    OptionalFeatures
}
```

## Calculation Context

Instead of passing just the character, pass a context that can hold temporary modifiers:

```go
type CalculationContext struct {
    Character Character
    
    // Optional features if available
    Features OptionalFeatures
    
    // Temporary effects
    Conditions []Condition
    
    // Advantage/disadvantage state
    Advantages map[string]bool
    
    // One-time bonuses (like Bardic Inspiration)
    TempBonuses map[string]int
}

// Calculations take context and return detailed results
func CalculateAC(ctx *CalculationContext) ACResult
func CalculateMaxHP(ctx *CalculationContext) HPResult
func CalculateAttackBonus(ctx *CalculationContext, attackType string) AttackResult
```

## Result Types

Results should be self-documenting:

```go
type ACResult struct {
    Total      int
    Base       int
    Breakdown  []Modifier
}

type Modifier struct {
    Source      string  // "Dexterity", "Armor", "Shield", etc.
    Value       int
    Type        string  // "base", "armor", "ability", "magic", etc.
    Description string  // Human-readable explanation
}
```

## Constructor Pattern

Use a builder or config pattern for construction with validation:

```go
type CharacterConfig struct {
    ID               string
    Name             string
    Level            int
    Class            Class         // Required
    AbilityScores    AbilityScores // Required
    
    // Optional
    Race             Race
    Background       Background
    Equipment        []Equipment
    Proficiencies    *Proficiencies
}

func (c *CharacterConfig) Validate() error {
    if c.Class == nil {
        return errors.New("character must have a class")
    }
    if c.AbilityScores == nil {
        return errors.New("character must have ability scores")
    }
    if c.Level < 1 || c.Level > 20 {
        return fmt.Errorf("invalid level %d: must be 1-20", c.Level)
    }
    return nil
}

func NewCharacter(cfg CharacterConfig) (FullCharacter, error) {
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid character config: %w", err)
    }
    
    // Calculate proficiency bonus from level
    if cfg.ProficiencyBonus == 0 {
        cfg.ProficiencyBonus = 2 + ((cfg.Level - 1) / 4)
    }
    
    return &character{
        config: cfg,
    }, nil
}
```

## Test Data Builder

For tests, use a fluent builder:

```go
type TestCharacterBuilder struct {
    cfg CharacterConfig
}

func TestCharacter() *TestCharacterBuilder {
    return &TestCharacterBuilder{
        cfg: CharacterConfig{
            ID:    "test-1",
            Name:  "Test Character",
            Level: 1,
            // Defaults that can be overridden
            Class:         TestClass("Fighter", 10),
            AbilityScores: TestAbilities(10, 10, 10, 10, 10, 10),
        },
    }
}

func (b *TestCharacterBuilder) WithClass(name string, hitDice int) *TestCharacterBuilder {
    b.cfg.Class = TestClass(name, hitDice)
    return b
}

func (b *TestCharacterBuilder) WithLevel(level int) *TestCharacterBuilder {
    b.cfg.Level = level
    return b
}

func (b *TestCharacterBuilder) WithAbilities(str, dex, con, int, wis, cha int) *TestCharacterBuilder {
    b.cfg.AbilityScores = TestAbilities(str, dex, con, int, wis, cha)
    return b
}

func (b *TestCharacterBuilder) Build() FullCharacter {
    char, err := NewCharacter(b.cfg)
    if err != nil {
        panic(fmt.Sprintf("test character build failed: %v", err))
    }
    return char
}
```

## Example Usage

```go
func TestArmorClassCalculation(t *testing.T) {
    // Build a test character
    char := TestCharacter().
        WithClass("Fighter", 10).
        WithAbilities(16, 14, 15, 10, 13, 8). // +3, +2, +2, +0, +1, -1
        WithLevel(5).
        Build()
    
    // Create context
    ctx := &CalculationContext{
        Character: char,
    }
    
    // Calculate AC
    result := CalculateAC(ctx)
    
    assert.Equal(t, 12, result.Total) // 10 + 2 (DEX)
    assert.Equal(t, "10 + 2 (Dexterity modifier)", result.String())
}
```

## Implementation Strategy

1. **Start with interfaces** - Define the contracts first
2. **Implement test builders** - Make it easy to create test data
3. **Write calculations** - Pure functions with the context pattern
4. **Add adapters last** - Bridge to your existing data model

## Migration Path

For existing code that might have nils:

```go
// Adapter that handles legacy data
type LegacyAdapter struct {
    data *OldCharacterData
}

func (a *LegacyAdapter) Class() Class {
    if a.data.Class == nil {
        // Return a default class rather than nil
        return DefaultClass()
    }
    return a.data.Class
}

func (a *LegacyAdapter) HasRace() bool {
    return a.data.Race != nil
}

func (a *LegacyAdapter) Race() (Race, error) {
    if a.data.Race == nil {
        return nil, errors.New("character has no race selected")
    }
    return a.data.Race, nil
}
```

## Next Steps

1. Create the interface definitions
2. Implement test builders
3. Write calculation functions with full result types
4. Create comprehensive tests
5. Build adapters for rpg-api integration