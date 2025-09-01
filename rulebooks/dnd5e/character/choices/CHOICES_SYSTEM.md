# The D&D 5e Choices System

## Overview

The choices system manages the complex decision tree of D&D 5e character creation. It ensures players make all required choices and that those choices are valid according to the rules.

## Core Concepts

### Requirements, Submissions, and Validation

```
Requirements: "You must choose 2 skills from this list"
     ↓
Submissions:  "I choose Athletics and History"  
     ↓
Validation:   "✓ Valid - both are allowed options and you chose exactly 2"
```

## The Choice ID System

Every requirement has a unique ID that links requirements to submissions:

```go
// Requirement defines what must be chosen
SkillRequirement{
    ID:    "fighter-skills",  // <-- Unique identifier
    Count: 2,
    Label: "Choose 2 skills",  // <-- Display text
}

// Submission references the requirement by ID
Submission{
    ChoiceID: "fighter-skills",  // <-- Links to requirement
    Values:   ["athletics", "history"],
}
```

### Why IDs Matter

1. **Unambiguous Reference** - Clear which requirement a submission satisfies
2. **Multiple Similar Choices** - Fighter has 3 equipment choice groups
3. **Validation** - Can match submissions to requirements precisely
4. **API Communication** - Client and server agree on choice identifiers

## ID Naming Conventions

### Class-based IDs
- Skills: `"{class}-skills"` (e.g., "fighter-skills")
- Fighting Style: `"{class}-fighting-style"`
- Expertise: `"{class}-expertise-{level}"` (e.g., "rogue-expertise-1")
- Equipment: `"{class}-{category}"` (e.g., "fighter-armor", "fighter-weapons-primary")

### Race-based IDs
- Skills: `"{race}-skills"` (e.g., "half-elf-skills")
- Languages: `"{race}-language"` or `"{race}-languages"`

### Background-based IDs
- Languages: `"{background}-language"`
- Tools: `"{background}-tools"`

## Equipment Choices Deep Dive

Equipment choices are the most complex, with nested structure:

```go
EquipmentRequirement{
    ID:     "fighter-armor",        // Requirement ID
    Choose: 1,                       // Pick one option
    Label:  "Choose your armor",     // Display text
    Options: []EquipmentOption{
        {
            ID:    "fighter-armor-a",  // Option ID
            Label: "Chain mail",
            Items: []EquipmentItem{
                {
                    Equipment: chainMail,  // Full Equipment object
                    Quantity:  1,
                },
            },
        },
        {
            ID:    "fighter-armor-b",  // Option ID
            Label: "Leather armor, longbow, and 20 arrows",
            Items: []EquipmentItem{
                {Equipment: leather, Quantity: 1},
                {Equipment: longbow, Quantity: 1},
                {Equipment: arrows, Quantity: 20},
            },
        },
    },
}
```

### Submission for Equipment

```go
Submission{
    ChoiceID: "fighter-armor",      // References requirement
    Values:   ["fighter-armor-b"],  // Selected option ID
}
```

## Full Character Creation Example

### 1. API Provides Requirements

```go
// GET /api/character/requirements?class=fighter&race=human
{
    "skills": {
        "id": "fighter-skills",
        "count": 2,
        "options": ["athletics", "history", ...],
        "label": "Choose 2 skills"
    },
    "fighting_style": {
        "id": "fighter-fighting-style",
        "options": [
            {
                "value": "defense",
                "name": "Defense",
                "description": "While wearing armor, +1 AC"
            }
        ],
        "label": "Choose a fighting style"
    },
    "equipment": [
        {
            "id": "fighter-armor",
            "choose": 1,
            "options": [...],
            "label": "Choose your armor"
        }
    ]
}
```

### 2. Client Collects Choices

```go
// POST /api/character/draft/class
{
    "class_id": "fighter",
    "choices": {
        "skills": ["athletics", "history"],
        "fighting_style": "defense",
        "equipment": {
            "fighter-armor": "fighter-armor-a",
            "fighter-weapons-primary": "fighter-weapon-b",
            "fighter-pack": "fighter-pack-a"
        }
    }
}
```

### 3. Server Validates

```go
validator := choices.NewValidator()
submissions := choices.NewSubmissions()

// Convert API input to submissions
submissions.Add(Submission{
    Category: ChoiceSkills,
    Source:   SourceClass,
    ChoiceID: "fighter-skills",
    Values:   ["athletics", "history"],
})

// Validate against requirements
result := validator.Validate(requirements, submissions)
if !result.Valid {
    // Return errors to client
}
```

## Sources and Categories

### Sources (Where choices come from)
- `SourceClass` - From your class (Fighter, Wizard, etc.)
- `SourceRace` - From your race (Elf, Dwarf, etc.)
- `SourceBackground` - From your background
- `SourceSubclass` - From your subclass
- `SourceFeat` - From feats

### Categories (What type of choice)
- `ChoiceSkills` - Skill proficiencies
- `ChoiceLanguages` - Language proficiencies  
- `ChoiceEquipment` - Starting equipment
- `ChoiceFightingStyle` - Combat style
- `ChoiceToolProficiency` - Tool proficiencies
- `ChoiceExpertise` - Expertise (double proficiency)

## Validation Rules

### Skills
- Must choose exact count
- Must be from allowed options
- No duplicates

### Equipment
- Must choose from each requirement group
- Must select exactly the "choose" count
- Option IDs must be valid

### Languages
- Must choose exact count
- Can be any language if options is nil
- No duplicates

## Best Practices

1. **Always use explicit IDs** - Never use labels as identifiers
2. **Include full data in requirements** - Send Equipment objects, not just IDs
3. **Submit only IDs** - Submissions should be lightweight
4. **Validate server-side** - Never trust client validation alone
5. **Use typed constants** - FightingStyle type, not strings

## Common Pitfalls

### Wrong: Using label as ID
```go
if submission.ChoiceID == requirement.Label {  // BAD
```

### Right: Using explicit ID
```go
if submission.ChoiceID == requirement.ID {     // GOOD
```

### Wrong: Sending only IDs in requirements
```go
Options: []string{"chain-mail", "leather"}     // BAD - no context
```

### Right: Sending full objects
```go
Items: []EquipmentItem{
    {Equipment: chainMail, Quantity: 1},       // GOOD - full data
}
```

## Testing Choices

```go
// Create requirements
reqs := GetClassRequirements(classes.Fighter)

// Create submissions
subs := NewSubmissions()
subs.Add(Submission{
    ChoiceID: "fighter-skills",
    Values:   []string{"athletics", "history"},
})

// Validate
validator := NewValidator()
result := validator.Validate(reqs, subs)

assert.True(t, result.Valid)
```

## Future Enhancements

- [ ] Conditional requirements (e.g., "If you chose X, also choose Y")
- [ ] Prerequisite validation (e.g., "Must have Str 13+ for heavy armor")
- [ ] Custom choice types for homebrew content
- [ ] Choice dependencies and exclusions