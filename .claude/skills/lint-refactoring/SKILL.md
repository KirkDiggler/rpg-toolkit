# Linter Issue Refactoring Specialist

## Purpose
Expert at refactoring Go code to resolve linter warnings while improving code quality and maintainability. Focuses on architectural improvements, not just suppressing warnings.

## Core Philosophy
- **Linter warnings are architectural signals** - they point to improvement opportunities
- **Question the linter, not just the code** - sometimes the linter config needs adjustment
- **Don't fix at any cost** - if a "fix" makes code worse, adjust the linter instead
- **Refactor, don't suppress** - use `//nolint` only as last resort for false positives
- **Improve while fixing** - use refactoring as opportunity to make code more modular
- **Test coverage first** - ensure tests pass before and after refactoring

### Decision Tree: Fix Code or Adjust Linter?

**Fix the code when:**
- ✅ Warning points to real architectural problem (duplication, complexity)
- ✅ Fix improves readability and maintainability
- ✅ Pattern should be consistent across codebase
- ✅ Industry best practice being violated

**Adjust the linter when:**
- ✅ Warning conflicts with project conventions (e.g., always use pointers)
- ✅ Threshold is too strict for this project (line length, complexity)
- ✅ Linter doesn't understand domain-specific patterns
- ✅ False positives common in legitimate code
- ✅ "Fix" would make code less readable or maintainable

**Example**: If project convention is "always use pointers for structs", disable `hugeParam` rather than mixing value/pointer types based on size.

## Common Linter Issues & Solutions

### 1. dupl (Duplicate Code)
**Signal**: Code is copied rather than abstracted
**Strategy**: Extract common patterns into helper functions or shared utilities

#### Test Duplication Pattern
**When to extract**:
- Setup/teardown code repeated across tests → Use testify suite SetupTest/SetupSubTest
- Assertion patterns repeated → Extract custom assertion helpers
- Test data construction repeated → Extract factory functions

**When duplication is OK**:
- Different tests testing different behavior (clarity > DRY)
- Test table entries (each case should be explicit)
- Small setup blocks unique to one test

**Example refactoring**:
```go
// Before: Duplicate test setup
func TestFeatureA(t *testing.T) {
    ctx := context.Background()
    bus := events.NewEventBus()
    entity := createTestEntity()
    // ... test A
}

func TestFeatureB(t *testing.T) {
    ctx := context.Background()
    bus := events.NewEventBus()
    entity := createTestEntity()
    // ... test B
}

// After: Suite with shared setup
type FeatureTestSuite struct {
    suite.Suite
    ctx    context.Context
    bus    events.EventBus
    entity *TestEntity
}

func (s *FeatureTestSuite) SetupTest() {
    s.ctx = context.Background()
    s.bus = events.NewEventBus()
    s.entity = createTestEntity()
}
```

#### Production Code Duplication
**Extract to**:
- **Helper functions** - same package, shared logic
- **Utility package** - cross-package, generic utilities
- **Base types** - embedded structs with shared methods

**Example**:
```go
// Before: Duplicate chain execution
finalEvent1, err := chain1.Execute(ctx, event1)
if err != nil {
    return nil, rpgerr.Wrap(err, "failed to execute chain1")
}

finalEvent2, err := chain2.Execute(ctx, event2)
if err != nil {
    return nil, rpgerr.Wrap(err, "failed to execute chain2")
}

// After: Extract helper
func executeChain[T any](ctx context.Context, chain chain.Chain[T], event T, errMsg string) (T, error) {
    final, err := chain.Execute(ctx, event)
    if err != nil {
        return final, rpgerr.Wrap(err, errMsg)
    }
    return final, nil
}
```

### 2. gocyclo (Cyclomatic Complexity)
**Signal**: Function is doing too much, hard to test
**Strategy**: Extract sub-functions, use table-driven approaches

**Common patterns**:
- Long if/else chains → Use switch or map dispatch
- Nested loops → Extract inner loop to function
- Multiple validation steps → Extract to validator function

### 3. lll (Line Length)
**Signal**: Expression too complex or deeply nested
**Strategy**: Extract to variables, simplify nesting

**Patterns**:
- Long function calls → Extract args to variables
- Chained method calls → Break into steps
- Complex conditions → Extract to named boolean variables

```go
// Before: 150 character line
err := c.Add(dnd5e.StageFeatures, "rage", func(_ context.Context, e combat.DamageChainEvent) (combat.DamageChainEvent, error) {

// After: Extract modifier function
rageModifier := func(_ context.Context, e combat.DamageChainEvent) (combat.DamageChainEvent, error) {
    e.DamageBonus += r.DamageBonus
    return e, nil
}
err := c.Add(dnd5e.StageFeatures, "rage", rageModifier)
```

### 4. goconst (Repeated Strings)
**Signal**: Magic strings should be constants
**Strategy**: Extract to typed constants in appropriate package

**Pattern**:
```go
// Before: Repeated strings
if conditionType == "raging" { ... }
if conditionType == "raging" { ... }

// After: Typed constants
type ConditionType string

const (
    ConditionTypeRaging      ConditionType = "raging"
    ConditionTypeUnconscious ConditionType = "unconscious"
)
```

### 5. errcheck (Unchecked Errors)
**Signal**: Error handling missing
**Strategy**: Always check errors, use appropriate handling

**Patterns**:
- Event publishing → Check and wrap with context
- Resource cleanup → Use defer with error check
- Optional operations → Explicitly ignore with `_ =` and comment why

### 6. unparam (Unused Parameters)
**Signal**: Parameter not needed or always same value
**Strategy**: Remove if truly unused, or keep for interface compliance

**Keep parameter when**:
- Interface method signature requires it
- Future-proofing for likely extension
- Context parameter for cancellation support

**Remove when**:
- Internal function never uses it
- Always passed same value

## Refactoring Workflow

### Step 1: Understand the Warning
```bash
# Run linter with verbose output
golangci-lint run ./... --verbose

# Focus on specific file
golangci-lint run ./path/to/file.go

# Check specific linter
golangci-lint run --disable-all --enable=dupl ./...
```

### Step 2: Assess the Signal
- **Is this pointing to real architectural issue?** → Refactor
- **Is this a false positive?** → Suppress with `//nolint:lintername // reason`
- **Is this test clarity vs DRY tradeoff?** → Evaluate case-by-case

### Step 3: Plan the Refactoring
- Identify the pattern being repeated
- Determine appropriate abstraction level
- Consider impact on readability and testability
- Check if similar patterns exist elsewhere (fix all at once)

### Step 4: Execute with Safety
```bash
# Ensure tests pass before refactoring
go test ./...

# Make refactoring changes
# ... code changes ...

# Verify tests still pass
go test ./...

# Verify linter issue resolved
golangci-lint run ./...
```

### Step 5: Validate Improvement
- [ ] Linter warning resolved
- [ ] Tests still pass
- [ ] Code is more readable
- [ ] Code is more maintainable
- [ ] No new issues introduced

## RPG Toolkit Specific Patterns

### Event Chain Refactoring
Common duplication in event chain setup/execution:

```go
// Extract chain execution pattern
func publishAndExecuteChain[T any](
    ctx context.Context,
    topic *events.ChainedTopic[T],
    event T,
    stages []chain.Stage,
) (T, error) {
    c := events.NewStagedChain[T](stages)
    modifiedChain, err := topic.PublishWithChain(ctx, event, c)
    if err != nil {
        return event, rpgerr.Wrap(err, "failed to publish chain")
    }

    finalEvent, err := modifiedChain.Execute(ctx, event)
    if err != nil {
        return event, rpgerr.Wrap(err, "failed to execute chain")
    }

    return finalEvent, nil
}
```

### Test Suite Patterns
Use testify suite for shared test infrastructure:

```go
type MyTestSuite struct {
    suite.Suite
    ctx      context.Context
    bus      events.EventBus
    // ... shared test infrastructure
}

func (s *MyTestSuite) SetupTest() {
    // Fresh instances for each test
    s.ctx = context.Background()
    s.bus = events.NewEventBus()
}

func (s *MyTestSuite) SetupSubTest() {
    // Reset state for each s.Run() subtest
}
```

### Factory Functions for Test Data
Extract repeated test entity creation:

```go
// In test file
func newTestMonster(id string, overrides ...func(*monster.Config)) *monster.Monster {
    cfg := monster.Config{
        ID:   id,
        Name: "Test Monster",
        HP:   10,
        AC:   12,
        // ... defaults
    }

    for _, override := range overrides {
        override(&cfg)
    }

    return monster.New(cfg)
}

// Usage
attacker := newTestMonster("attacker-1", func(c *monster.Config) {
    c.HP = 50
    c.AC = 15
})
```

## When to Use //nolint

Only suppress linter warnings when:
1. **False positive** - linter is wrong about this specific case
2. **External constraint** - interface requires specific signature
3. **Intentional pattern** - design decision with good reason

**Always include reason**:
```go
//nolint:dupl // Tests intentionally similar for clarity
func TestFeatureA(t *testing.T) { ... }

//nolint:unparam // Interface requires context parameter
func (i *Implementation) Method(ctx context.Context) error { ... }
```

## Red Flags

Avoid these anti-patterns:
- ❌ Blanket `//nolint` without specific linter
- ❌ Suppressing without comment explaining why
- ❌ Ignoring dupl in production code (always refactor)
- ❌ Breaking up clear code just to satisfy line length
- ❌ Over-abstracting to eliminate all duplication (balance with readability)

## Success Metrics

Good refactoring achieves:
- ✅ Linter warnings eliminated
- ✅ Code is more modular
- ✅ Tests still pass
- ✅ Code is easier to understand
- ✅ Similar patterns throughout codebase
- ✅ Future changes are easier

## Integration with golang-architect Agent

This skill works alongside the `golang-architect` agent (`/home/kirk/personal/.claude/agents/golang-architect/`):

**When to use which:**
- **This skill**: Quick reference for common linter patterns and refactoring strategies
- **golang-architect agent**: Architectural review, debates about design decisions, Go convention enforcement

**Workflow:**
1. Linter reports issue
2. Consult this skill for common patterns
3. If architectural decision needed → spawn golang-architect agent
4. Agent reviews and provides guidance:
   - Fix code? → Apply refactoring from this skill
   - Adjust linter? → Update .golangci.yml
   - Suppress? → Add //nolint with reason

**Example:**
```
Linter: hugeParam warning on struct parameter

This skill: Could use pointer or adjust linter config

Spawn golang-architect agent for decision

Agent: Project convention is pointers for all structs - disable gocritic.hugeParam
```

## Project Context

This skill is part of the RPG Toolkit project:
- Multi-module Go repository
- Uses golangci-lint v2.5.0 (see `.golangci.yml`)
- Testify suite pattern for tests
- Event-driven architecture with modifier chains
- See `.claude/skills/rpg-toolkit-development/` for domain patterns
- See `/home/kirk/personal/.claude/agents/golang-architect/` for Go conventions
