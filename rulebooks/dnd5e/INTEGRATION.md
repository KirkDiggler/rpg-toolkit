# RPG-API Integration Guide

This guide shows how to integrate the D&D 5e rulebook character creation into rpg-api.

## What You Get

The rulebook provides:
- ✅ Complete character creation with validation
- ✅ Character draft management for multi-step creation
- ✅ Proper D&D 5e data structures
- ✅ Character persistence format (CharacterData)
- ✅ Progress tracking for character creation
- ✅ Race/class/background data structures

## Integration Steps

### 1. Add Dependency

In your rpg-api go.mod:
```bash
go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@latest
```

### 2. Character Creation Flow

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// In your character service
type CharacterService struct {
    repo         CharacterRepository
    draftRepo    CharacterDraftRepository
    dndAPI       DnD5eAPIClient
}

// Start character creation
func (s *CharacterService) StartCharacterCreation(playerID string) (*dnd5e.CharacterDraftData, error) {
    // Generate unique draft ID using your preferred method
    draftID := generateDraftID() // e.g., UUID, database sequence, etc.
    builder, err := dnd5e.NewCharacterBuilder(draftID)
    if err != nil {
        return err
    }
    
    // Save initial draft
    draftData := builder.ToData()
    draftData.PlayerID = playerID
    
    if err := s.draftRepo.Save(draftData); err != nil {
        return nil, err
    }
    
    return &draftData, nil
}

// Update draft with selections
func (s *CharacterService) UpdateCharacterDraft(draftID string, update CharacterDraftUpdate) (*dnd5e.CharacterDraftData, error) {
    // Load existing draft
    draftData, err := s.draftRepo.Get(draftID)
    if err != nil {
        return nil, err
    }
    
    // Recreate builder from saved draft
    builder, err := dnd5e.LoadDraft(*draftData)
    if err != nil {
        return nil, err
    }
    
    // Apply updates based on what was provided
    if update.Name != "" {
        builder.SetName(update.Name)
    }
    
    if update.RaceID != "" {
        // Load race data from your source
        raceData, err := s.loadRaceData(update.RaceID)
        if err != nil {
            return nil, err
        }
        builder.SetRaceData(*raceData, update.SubraceID)
    }
    
    if update.ClassID != "" {
        // Load class data
        classData, err := s.loadClassData(update.ClassID)
        if err != nil {
            return nil, err
        }
        builder.SetClassData(*classData)
    }
    
    if update.BackgroundID != "" {
        // Load background data
        backgroundData, err := s.loadBackgroundData(update.BackgroundID)
        if err != nil {
            return nil, err
        }
        builder.SetBackgroundData(*backgroundData)
    }
    
    if update.AbilityScores != nil {
        builder.SetAbilityScores(*update.AbilityScores)
    }
    
    if len(update.Skills) > 0 {
        builder.SelectSkills(update.Skills)
    }
    
    // Get progress
    progress := builder.Progress()
    
    // Save updated draft
    updatedDraft := builder.ToData()
    if err := s.draftRepo.Save(updatedDraft); err != nil {
        return nil, err
    }
    
    return &updatedDraft, nil
}

// Finalize character creation
func (s *CharacterService) FinalizeCharacter(draftID string) (*CharacterID, error) {
    // Load draft
    draftData, err := s.draftRepo.Get(draftID)
    if err != nil {
        return nil, err
    }
    
    // Recreate builder
    builder, err := dnd5e.LoadDraft(*draftData)
    if err != nil {
        return nil, err
    }
    
    // Check if ready to build
    progress := builder.Progress()
    if !progress.CanBuild {
        return nil, errors.New("character draft is incomplete")
    }
    
    // Need to reload the game data for building
    raceData, _ := s.loadRaceData(/* get from draft choices */)
    classData, _ := s.loadClassData(/* get from draft choices */)
    backgroundData, _ := s.loadBackgroundData(/* get from draft choices */)
    
    // Build the character
    character, err := builder.Build()
    if err != nil {
        return nil, err
    }
    
    // Convert to persistence format
    characterData := character.ToData()
    
    // Save to your character repository
    if err := s.repo.SaveCharacter(characterData); err != nil {
        return nil, err
    }
    
    // Clean up draft
    s.draftRepo.Delete(draftID)
    
    return &CharacterID{ID: characterData.ID}, nil
}
```

### 3. Data Loading Functions

```go
func (s *CharacterService) loadRaceData(raceID string) (*race.RaceData, error) {
    // Option 1: Load from D&D API
    apiRace, err := s.dndAPI.GetRace(raceID)
    if err != nil {
        return nil, err
    }
    
    // Convert API format to rulebook format
    return &race.RaceData{
        ID:          apiRace.Index,
        Name:        apiRace.Name,
        Size:        apiRace.Size,
        Speed:       apiRace.Speed,
        // ... map other fields
    }, nil
    
    // Option 2: Load from your database
    // return s.repo.GetRaceData(raceID)
}

func (s *CharacterService) loadClassData(classID string) (*class.ClassData, error) {
    // Similar pattern - load from API or database
}

func (s *CharacterService) loadBackgroundData(backgroundID string) (*shared.Background, error) {
    // Similar pattern
}
```

### 4. Repository Interface

```go
type CharacterRepository interface {
    SaveCharacter(data dnd5e.CharacterData) error
    GetCharacter(id string) (*dnd5e.CharacterData, error)
    ListCharactersByPlayer(playerID string) ([]dnd5e.CharacterData, error)
}

type CharacterDraftRepository interface {
    Save(draft dnd5e.CharacterDraftData) error
    Get(id string) (*dnd5e.CharacterDraftData, error)
    Delete(id string) error
    ListByPlayer(playerID string) ([]dnd5e.CharacterDraftData, error)
}
```

### 5. Discord Bot Integration

```go
// In your Discord handler
func (h *Handler) HandleCharacterCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Start creation
    draft, err := h.service.StartCharacterCreation(i.Member.User.ID)
    if err != nil {
        respondWithError(s, i, err)
        return
    }
    
    // Show first step (name)
    embed := &discordgo.MessageEmbed{
        Title:       "Character Creation - Step 1: Name",
        Description: "What is your character's name?",
        Fields: []*discordgo.MessageEmbedField{
            {
                Name:  "Progress",
                Value: "0% complete",
            },
        },
    }
    
    // Store draft ID in interaction data
    respondWithEmbed(s, i, embed, draft.ID)
}
```

## Key Benefits

1. **Validated Character Data**: All D&D 5e rules are enforced
2. **Progress Tracking**: Know exactly how far along character creation is
3. **Draft Management**: Save/resume character creation anytime
4. **Clean Data Structure**: CharacterData has everything you need
5. **Type Safety**: Strongly typed choices and validations

## What You Still Handle in rpg-api

- Combat mechanics (attacks, damage, initiative)
- Spell casting and spell management
- Equipment and inventory
- Character progression (leveling up)
- Game session management
- Discord UI/interactions

## Example Character Data Structure

```json
{
  "id": "char_1234567890",
  "name": "Ragnar",
  "level": 1,
  "race_id": "human",
  "class_id": "fighter",
  "background_id": "soldier",
  "ability_scores": {
    "strength": 16,
    "dexterity": 14,
    "constitution": 13,
    "intelligence": 12,
    "wisdom": 10,
    "charisma": 8
  },
  "hit_points": 11,
  "max_hit_points": 11,
  "armor_class": 12,
  "skills": {
    "athletics": "proficient",
    "intimidation": "proficient"
  },
  "languages": ["common", "orcish"],
  "features": ["fighting_style", "second_wind"],
  "conditions": [],
  "effects": [],
  "choices": [
    {
      "category": "skills",
      "source": "fighter",
      "selection": ["athletics", "intimidation"]
    }
  ]
}
```

This structure contains everything needed to recreate the character and continue gameplay.