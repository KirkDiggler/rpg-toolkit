# Design: Proto Changes for Action Economy History

## Overview

Proto changes to support ActionType enums, feature activation choices, and turn state with action history.

## Changes to dnd5e/api/v1alpha1/enums.proto

Add ActionType enum:

```protobuf
// ActionType represents standard D&D 5e actions
enum ActionType {
  ACTION_TYPE_UNSPECIFIED = 0;
  ACTION_TYPE_ATTACK = 1;
  ACTION_TYPE_DASH = 2;
  ACTION_TYPE_DISENGAGE = 3;
  ACTION_TYPE_DODGE = 4;
  ACTION_TYPE_HELP = 5;
  ACTION_TYPE_HIDE = 6;
  ACTION_TYPE_READY = 7;
  ACTION_TYPE_SEARCH = 8;
  ACTION_TYPE_USE_OBJECT = 9;
}
```

## New file: dnd5e/api/v1alpha1/activation.proto

Feature activation choices:

```protobuf
syntax = "proto3";

package dnd5e.api.v1alpha1;

import "dnd5e/api/v1alpha1/enums.proto";

option go_package = "github.com/KirkDiggler/rpg-api-protos/gen/go/dnd5e/api/v1alpha1;v1alpha1";

// ActivationOption represents one selectable option for feature activation
message ActivationOption {
  // The action type this option grants
  ActionType action_type = 1;

  // Display label for UI (e.g., "Disengage")
  string label = 2;

  // Optional description (e.g., "Avoid opportunity attacks")
  string description = 3;
}

// ActivationChoice represents a choice required to activate a feature
message ActivationChoice {
  // Identifier for this choice (e.g., "action_type")
  string id = 1;

  // Description of what the user is choosing
  string description = 2;

  // Available options
  repeated ActivationOption options = 3;
}
```

## Changes to dnd5e/api/v1alpha1/character.proto

Add choices to Feature message:

```protobuf
// Feature represents a character feature (class ability, racial trait, etc.)
message Feature {
  string id = 1;
  Ref ref = 2;
  string name = 3;
  string description = 4;

  // Choices required for activation (empty if none needed)
  repeated ActivationChoice activation_choices = 5;
}
```

## Changes to dnd5e/api/v1alpha1/encounter.proto

### Update ActivateFeatureRequest

```protobuf
message ActivateFeatureRequest {
  string encounter_id = 1;
  string character_id = 2;
  string feature_id = 3;

  // Action type choice (for features that grant actions)
  ActionType action_type = 4;
}
```

### Add ActionRecord message

```protobuf
// ActionRecord tracks what used an action slot
message ActionRecord {
  // Feature/ability that consumed this action
  string source_ref = 1;

  // What action was taken
  ActionType action_type = 2;
}
```

### Update TurnState

Replace bools with counts and history:

```protobuf
message TurnState {
  string entity_id = 1;
  int32 movement_used = 2;
  int32 movement_max = 3;

  // Budget - how many remain
  int32 actions_remaining = 4;
  int32 bonus_actions_remaining = 5;
  int32 reactions_remaining = 6;

  // History - what was used
  repeated ActionRecord actions_taken = 7;
  repeated ActionRecord bonus_actions_taken = 8;
  repeated ActionRecord reactions_taken = 9;

  .api.v1alpha1.Position position = 10;
}
```

**Note:** This replaces the current bool fields (`action_used`, `bonus_action_used`, `reaction_available`). Existing clients will need updates.

### Update ActivateFeatureResponse

```protobuf
message ActivateFeatureResponse {
  bool success = 1;
  string message = 2;
  Character updated_character = 3;
  CombatState updated_combat_state = 4;  // Includes TurnState with history
}
```

## Migration Notes

### Breaking Changes

- `TurnState.action_used` (bool) → `actions_remaining` (int32) + `actions_taken` (repeated)
- Same for bonus_action and reaction

### Client Updates Required

Clients checking `turn_state.action_used` need to check `turn_state.actions_remaining == 0` or look at `actions_taken`.

## File Summary

| File | Changes |
|------|---------|
| enums.proto | Add ActionType enum |
| activation.proto | New file - ActivationChoice, ActivationOption |
| character.proto | Add activation_choices to Feature |
| encounter.proto | Update ActivateFeatureRequest, TurnState, add ActionRecord |
