# Character System Implementation Checklist

## ✅ Completed in rpg-toolkit/rulebooks/dnd5e

### Core Types
- [x] **Equipment Interface** (`/equipment/equipment.go`)
  - GetID(), GetType(), GetName(), GetWeight(), GetValue(), GetDescription()
  - Implemented by: Weapons, Armor, Tools, Packs

- [x] **Typed Constants**
  - `races.Race` - All race IDs
  - `classes.Class` - All class IDs
  - `skills.Skill` - All skill IDs
  - `fightingstyles.FightingStyle` - All fighting styles
  - `choices.ChoiceID` - All choice requirement IDs
  - `shared.EquipmentType` - Equipment categories

### Character Domain
- [x] **Character** (`character.go`) - Core domain entity
- [x] **Data** (`data.go`) - Serialization/persistence
- [x] **Draft** (`draft.go`) - Creation workflow
- [x] **Input Types** (`inputs.go`) - API input structures

### Choices System
- [x] **Requirements** (`choices/requirements.go`)
  - All requirement types with explicit ChoiceID
  - Equipment requirements include full Equipment objects
  - GetClassRequirements(), GetRaceRequirements()

- [x] **Submissions** (`choices/submissions.go`)
  - Typed ChoiceID references
  - Lightweight - just IDs of what was chosen

- [x] **Validation** (`choices/validation.go`)
  - Validates submissions against requirements
  - Returns structured ValidationResult

- [x] **Choice IDs** (`choices/choice_ids.go`)
  - Complete set of typed constants
  - Every possible choice has an ID

### Equipment System
- [x] **Weapons** - All weapons with properties
- [x] **Armor** - All armor with AC, requirements
- [x] **Tools** - Artisan tools, instruments, gaming sets
- [x] **Packs** - Equipment bundles
- [x] **Unified GetByID** - Single lookup function

### Documentation
- [x] **README.md** - Package overview
- [x] **CHOICES_SYSTEM.md** - Deep dive on choices

## 🔄 Ready for rpg-api Implementation

### Proto Definitions Needed

1. **Enums for all constants**:
   ```protobuf
   enum ChoiceID { ... }
   enum Race { ... }
   enum Class { ... }
   enum Skill { ... }
   enum FightingStyle { ... }
   enum EquipmentType { ... }
   ```

2. **Message types**:
   ```protobuf
   message Requirement {
     ChoiceID id = 1;
     string label = 2;
     oneof requirement {
       SkillRequirement skill = 3;
       EquipmentRequirement equipment = 4;
       // etc.
     }
   }
   
   message Submission {
     ChoiceID choice_id = 1;
     repeated string values = 2;
   }
   ```

3. **Equipment messages with full data**:
   ```protobuf
   message Equipment {
     string id = 1;
     EquipmentType type = 2;
     string name = 3;
     string description = 4;
     float weight = 5;
     int32 value = 6;
   }
   ```

### API Endpoints Needed

1. **GET /character/requirements**
   - Input: class, race, level
   - Output: Requirements with full Equipment data

2. **POST /character/draft**
   - Create new draft

3. **PUT /character/draft/{id}/class**
   - Set class with choices
   - Input includes ChoiceID -> values mapping

4. **PUT /character/draft/{id}/race**
   - Set race with choices

5. **GET /character/draft/{id}/validate**
   - Validate all choices made

6. **POST /character/draft/{id}/finalize**
   - Create final character from draft

### Converter Functions Needed

```go
// In rpg-api
func ChoiceIDToProto(id choices.ChoiceID) pb.ChoiceID
func ProtoToChoiceID(id pb.ChoiceID) choices.ChoiceID

func RequirementsToProto(reqs *choices.Requirements) *pb.Requirements
func SubmissionsFromProto(subs *pb.Submissions) *choices.Submissions

func EquipmentToProto(eq equipment.Equipment) *pb.Equipment
```

### Storage Considerations

1. **Draft Storage**
   - Store drafts temporarily (Redis? PostgreSQL?)
   - TTL for abandoned drafts

2. **Character Storage**
   - Use Data struct for serialization
   - Store as JSON or in normalized tables

3. **Choice Audit Trail**
   - Track what choices were made when
   - Useful for debugging and analytics

## ⚠️ Important Notes

1. **Equipment Items vs Equipment Interface**
   - Requirements send full Equipment objects
   - Submissions only send IDs
   - Storage only keeps IDs

2. **Validation Must Be Server-Side**
   - Never trust client validation
   - Use the Validator with typed constants

3. **Proto Enum Values**
   - Start from 1 (0 is UNSPECIFIED)
   - Keep stable - don't renumber
   - Add new values at the end

4. **API Versioning**
   - Consider how to handle rulebook updates
   - New choices, classes, races over time

## 🎯 Implementation Order for rpg-api

1. **Define proto enums** for all constants
2. **Create converter functions** between internal and proto types
3. **Implement requirements endpoint** - most complex, sends rich data
4. **Implement draft CRUD** operations
5. **Implement choice submission** endpoints
6. **Implement validation** endpoint
7. **Implement finalize** to create character
8. **Add storage layer** for persistence

## Testing Considerations

- Unit tests for converters
- Integration tests for full character creation flow
- Validation tests for invalid choices
- Performance tests for equipment lookups

## Security Considerations

- Validate all enum values from client
- Rate limit character creation
- Authenticate draft ownership
- Sanitize character names