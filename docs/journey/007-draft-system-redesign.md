# Journey 007: Draft System Redesign - Tracking Choice Sources

## The Problem We Discovered

We built a draft system for rpg-toolkit that seemed reasonable at the time, but when the game service tried to use it, we discovered a fundamental flaw: **we weren't tracking where choices came from**.

The irony? We already HAD rich choice tracking - but we put it on the `Character` instead of the `Draft` where it's actually needed!

### The Original Implementation

The toolkit's draft system made hard-coded assumptions about sources:

```go
// In draft.go - we just assumed all skill choices came from class
if len(d.SkillChoices) > 0 {
    charData.Choices = append(charData.Choices, ChoiceData{
        Category:  string(shared.ChoiceSkills),
        Source:    "class",  // WRONG! This is an assumption
        Selection: d.SkillChoices,
    })
}
```

### What Actually Happens in D&D 5e

Reality is much more complex than our assumptions:

1. **Skills can come from multiple sources:**
   - Race (Half-Orc gets Intimidation automatically)
   - Class (Fighter chooses 2 from their skill list)
   - Background (Soldier gets Athletics and Intimidation)

2. **The same skill can come from multiple sources:**
   - A Half-Orc Fighter with Soldier background could get Intimidation THREE times
   - Players need to know this to make informed choices
   - The UI needs to show "You already have Intimidation from your race"

3. **When rebuilding characters, we need perfect provenance:**
   - If switching from Half-Orc to Human, which skills do we remove?
   - If changing class, which choices were class-based vs racial?

## The Game Service Perspective

From the rpg-api ADR-007, we see what game services actually need:

> "**Unclear provenance**: When a player changes class, we can't distinguish what came from the class vs. what the player chose"

The game service needs to:
- Rebuild drafts from stored choices
- Show warnings about duplicate selections
- Handle race/class/background changes cleanly
- Present clear UI showing where features come from

## What We Tried (And Why It Failed)

### Attempt 1: Simple Source Tracking

We tried just adding a "source" field, but this failed because:
- We still mixed automatic grants with player choices
- Couldn't handle the same skill from multiple sources
- Lost information about which specific choice was made

### Attempt 2: Storing Everything in Choices

We tried storing all skills/languages as "choices", but:
- Automatic grants aren't choices
- Made validation complex
- Confused the mental model

## Solution Options We Considered

### Option 1: Enhanced Choice Tracking (Minimal Change)

Track the actual source with each choice:

```go
type TrackedChoice struct {
    Value    string // "intimidation"
    Source   string // "race:half_orc" or "class:fighter_choice_1"
    ChoiceID string // "fighter_skill_proficiency_1"
    Granted  bool   // true = automatic, false = player chose
}
```

**Why we rejected this:** Still mixes grants with choices, making the data model unclear.

### Option 2: Pure Choices (ADR Alignment)

Store only what the player chose:

```go
type Draft struct {
    // Only player choices
    ClassSkillChoices []SkillChoice
    // Race/background grants computed at compile time
}
```

**Why we rejected this:** Game service can't show duplicate warnings without having all race/class/background data.

### Option 3: Full Choice Manifest (Comprehensive)

Track grants and choices separately with complete metadata:

```go
type CharacterManifest struct {
    Grants  []Grant  // Automatic from race/class/background
    Choices []Choice // Player selections
}

type Grant struct {
    Source   string // "race:half_orc"
    Type     string // "skill"
    Value    string // "intimidation"
}

type Choice struct {
    ID       string   // "fighter_skill_proficiency_1"
    Source   string   // "class:fighter"
    Type     string   // "skill"
    Options  []string // What was available
    Selected []string // What was chosen
}
```

**Why we like this:** Complete information, clear mental model, supports all use cases.

### Option 4: Event Sourcing

Store draft as a series of events:

```go
type DraftEvent struct {
    Type      string    // "SELECT_RACE", "CHOOSE_SKILL"
    Timestamp time.Time
    Data      any
}
```

**Why we rejected this:** Overkill for our needs, adds complexity without enough benefit.

## The Realization

After analyzing all these options, we realized something crucial: **The Character already has the ChoiceData structure we need - it's just in the wrong place!**

```go
// Character has rich tracking
type Character struct {
    choices []ChoiceData  // Category, Source, ChoiceID, Selection
}

// But Draft has flat lists!
type Draft struct {
    SkillChoices []constants.Skill  // No source info!
}
```

The Character can tell you "this skill came from race" but by then it's too late - we needed that information DURING character creation, in the Draft!

## The Decision: Move Rich Choices to Draft

The fix is simpler than all our complex proposals - just use the ChoiceData structure in the Draft where it belongs:

```go
type Draft struct {
    // Instead of flat lists...
    // SkillChoices []constants.Skill
    
    // Use the same rich structure as Character
    Choices []ChoiceData
}
```

This way:
1. **During character creation** - Full source information available for UI
2. **No information loss** - Everything is tracked from the start  
3. **Clean conversion** - Draft.ToCharacter() just passes choices through
4. **Supports all use cases** - Duplicates, warnings, rebuilding all work
4. **Separates concerns** - Automatic grants vs player choices are clearly different

## Implementation Plan

### Phase 1: Add Manifest (Backward Compatible)
- Add `Manifest` field to draft
- Compute it during `ToCharacter()`
- Existing code continues working

### Phase 2: Manifest-First Design
- New builder methods use manifest
- Validation works on manifest
- Migration path for existing drafts

### Phase 3: Deprecate Old Structure
- Remove old choice tracking
- Clean up technical debt
- Simplified API

## Lessons Learned

1. **Don't assume sources** - Track where everything comes from explicitly
2. **Separate grants from choices** - They're fundamentally different concepts  
3. **Think about the full lifecycle** - Not just creation, but updates and rebuilds
4. **Consider the UI needs** - The data model should support user-facing features
5. **Put data where it's needed** - Rich tracking belongs in Draft, not just Character
6. **Check your assumptions** - We assumed Draft was tracking sources but it wasn't

## Connection to Registry System (ADR-0019)

While working on this problem, we discovered that ADR-0019's registry system could provide the foundation we need. The registry already plans to track class, race, and spell data with behavior. We could extend this pattern:

```go
type RaceEntry struct {
    Data   *RaceData
    Grants func() []Grant  // What this race automatically provides
}

type ClassEntry struct {
    Data    *ClassData
    Choices func() []ChoiceDefinition  // What choices this class offers
}
```

This would allow the draft system to:
1. Query the registry for what grants a race provides
2. Query the registry for what choices a class offers
3. Build the manifest dynamically based on current race/class/background

The registry becomes the single source of truth for both data AND the rules about what comes from where.

## What This Enables

With proper source tracking, we can now:
- Show duplicate warnings in the UI
- Handle race/class changes cleanly
- Support future features like multiclassing
- Provide clear provenance for every feature

The manifest approach gives us a foundation for complex character building scenarios while keeping the mental model clear and simple.

## Future Considerations

### Multiclassing
When we add multiclassing support, the manifest approach will shine:
- Each class level can track its own choices
- Clear separation of what came from Fighter 1 vs Wizard 2
- No confusion about skill proficiencies from different sources

### Custom Content
With the registry system, custom races/classes can define their own grants and choices:
- Homebrew races can specify their automatic skills
- Custom backgrounds can define their proficiencies
- Everything flows through the same manifest system