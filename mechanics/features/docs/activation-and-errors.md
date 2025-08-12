# Feature Activation and Error Communication

## Features Are Already Loaded!

```go
// Features are loaded at connection, not looked up each time
func (c *Character) ActivateFeature(featureRef string) error {
    // Features already in memory from LoadFromGameContext
    for _, feature := range c.features {
        if feature.Ref().String() == featureRef {
            return feature.Activate()
        }
    }
    return ErrFeatureNotFound
}
```

## Smart Error Types for Player Communication

```go
// mechanics/features/errors.go
package features

type ErrorType int

const (
    ErrorTypeInfo     ErrorType = iota  // "You must be raging to use this"
    ErrorTypeWarning                    // "Only 1 use remaining!"
    ErrorTypeCooldown                   // "Available in 2 turns"
    ErrorTypeResource                   // "Not enough spell slots"
    ErrorTypeInvalid                    // "Cannot use while stunned"
    ErrorTypeFatal                      // Something broke
)

// FeatureError communicates with players
type FeatureError struct {
    Type    ErrorType
    Message string      // Player-facing message
    Detail  string      // Technical detail for logs
    Retry   *RetryInfo  // When can they try again?
}

type RetryInfo struct {
    When     string  // "next_turn", "after_rest", "never"
    Duration int     // Turns/rounds/minutes
}

func (e *FeatureError) Error() string {
    return e.Message
}

// Helper constructors
func NoUsesError(remaining int, refreshOn string) *FeatureError {
    return &FeatureError{
        Type:    ErrorTypeResource,
        Message: "No uses remaining",
        Detail:  fmt.Sprintf("Feature has %d uses remaining", remaining),
        Retry:   &RetryInfo{When: refreshOn},
    }
}

func AlreadyActiveError(feature string) *FeatureError {
    return &FeatureError{
        Type:    ErrorTypeInfo,
        Message: fmt.Sprintf("%s is already active", feature),
    }
}

func CooldownError(turnsLeft int) *FeatureError {
    return &FeatureError{
        Type:    ErrorTypeCooldown,
        Message: fmt.Sprintf("Available in %d turns", turnsLeft),
        Retry:   &RetryInfo{When: "turns", Duration: turnsLeft},
    }
}
```

## Features Use Smart Errors

```go
func (r *RageFeature) Activate() error {
    // The feature knows its own name!
    if r.isActive {
        return AlreadyActiveError(r.Name())  // "Rage is already active"
    }
    
    // No uses - resource error
    if r.usesRemaining <= 0 {
        return NoUsesError(0, "long_rest")
    }
    
    // Character condition prevents it - feature checks this
    if r.owner.HasCondition("frightened") {
        return &FeatureError{
            Type:    ErrorTypeInvalid,
            Message: fmt.Sprintf("Cannot use %s while frightened", r.Name()),
            Detail:  "Frightened condition prevents activation",
        }
    }
    
    // Success!
    r.usesRemaining--
    r.isActive = true
    r.dirty = true
    
    // Fire activation event - rage publishes its own ref
    activateEvent := events.NewGameEvent("feature.activate", r.owner, nil)
    activateEvent.Context().Set("feature_ref", RageRef)  // Rage knows it's rage!
    
    return r.eventBus.Publish(context.Background(), activateEvent)
}

func (r *RageFeature) Name() string {
    return "Rage"  // Feature knows its own name
}
```

## Not Everything Can Be Activated

```go
// Some features are passive
type Darkvision struct {
    *features.SimpleFeature
}

func (d *Darkvision) Activate() error {
    return &FeatureError{
        Type:    ErrorTypeInfo,
        Message: "Darkvision is always active",
        Detail:  "Passive feature cannot be activated",
    }
}

func (d *Darkvision) CanActivate() bool {
    return false  // Never shows in action list
}

// Some features are triggered by events
type UncannyDodge struct {
    *features.SimpleFeature
    usedThisRound bool
}

func (u *UncannyDodge) Activate() error {
    return &FeatureError{
        Type:    ErrorTypeInfo,
        Message: "Uncanny Dodge triggers automatically when you take damage",
    }
}
```

## UI Handles Different Error Types

```go
// API response includes error type
type ActivationResponse struct {
    Success   bool        `json:"success"`
    Error     *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
    Type    string      `json:"type"`    // "cooldown", "resource", etc
    Message string      `json:"message"`
    Retry   *RetryInfo  `json:"retry,omitempty"`
}

// Server endpoint
func HandleActivateFeature(req ActivateRequest) ActivationResponse {
    err := character.ActivateFeature(req.FeatureRef)
    
    if err == nil {
        return ActivationResponse{Success: true}
    }
    
    // Check if it's a FeatureError with type info
    if featErr, ok := err.(*FeatureError); ok {
        return ActivationResponse{
            Success: false,
            Error: &ErrorInfo{
                Type:    featErr.Type.String(),
                Message: featErr.Message,
                Retry:   featErr.Retry,
            },
        }
    }
    
    // Generic error
    return ActivationResponse{
        Success: false,
        Error: &ErrorInfo{
            Type:    "error",
            Message: err.Error(),
        },
    }
}
```

## UI Can Show Appropriate Feedback

```javascript
// Client-side handling
async function activateFeature(featureRef) {
    const response = await api.activateFeature(featureRef);
    
    if (!response.success) {
        switch(response.error.type) {
            case 'cooldown':
                showToast(`⏱️ ${response.error.message}`, 'warning');
                break;
            
            case 'resource':
                showToast(`❌ ${response.error.message}`, 'error');
                updateResourceDisplay();
                break;
                
            case 'info':
                showToast(`ℹ️ ${response.error.message}`, 'info');
                break;
                
            case 'invalid':
                showToast(`⚠️ ${response.error.message}`, 'warning');
                break;
                
            default:
                showToast(`Error: ${response.error.message}`, 'error');
        }
        
        // Handle retry info
        if (response.error.retry) {
            scheduleRetryHint(featureRef, response.error.retry);
        }
    }
}
```

## The Complete Pattern

```go
type Feature interface {
    // Core
    Ref() *core.Ref
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    
    // Display
    Name() string
    Description() string
    
    // Activation (returns typed errors!)
    CanActivate() bool
    Activate() error  // Returns FeatureError with player-facing info
    IsActive() bool
    GetRemainingUses() string
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}
```

Now the server can:
1. Call Activate() on already-loaded features
2. Get typed errors that explain WHY it failed
3. Pass meaningful messages to players
4. Let UI show appropriate feedback

The spaghetti is sticking!