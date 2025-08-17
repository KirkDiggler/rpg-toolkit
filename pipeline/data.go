package pipeline

import "fmt"

// HPData represents a change to an entity's hit points.
type HPData struct {
	EntityID  string
	Amount    int  // Negative for damage, positive for healing
	Temporary bool // Whether this is temporary HP
}

// GetEntityID returns the affected entity.
func (d *HPData) GetEntityID() string {
	return d.EntityID
}

// GetOperation returns the update operation.
func (d *HPData) GetOperation() DataOperation {
	return OpUpdate
}

// Apply applies the HP change to a store.
func (d *HPData) Apply(_ any) error {
	// Store implementation would handle this
	return nil
}

// String returns a string representation.
func (d *HPData) String() string {
	if d.Amount < 0 {
		return fmt.Sprintf("%s takes %d damage", d.EntityID, -d.Amount)
	}
	return fmt.Sprintf("%s heals %d HP", d.EntityID, d.Amount)
}

// LogData represents a combat log entry.
type LogData struct {
	Message string
}

// GetEntityID returns empty for log entries.
func (d *LogData) GetEntityID() string {
	return ""
}

// GetOperation returns append for logs.
func (d *LogData) GetOperation() DataOperation {
	return OpAppend
}

// Apply appends the log message.
func (d *LogData) Apply(_ any) error {
	// Store implementation would handle this
	return nil
}

// String returns the log message.
func (d *LogData) String() string {
	return d.Message
}

// ConditionData represents a condition change.
type ConditionData struct {
	EntityID  string
	Condition string
	Active    bool // true to add, false to remove
	Duration  int  // Duration in rounds (0 = permanent)
}

// GetEntityID returns the affected entity.
func (d *ConditionData) GetEntityID() string {
	return d.EntityID
}

// GetOperation returns the appropriate operation.
func (d *ConditionData) GetOperation() DataOperation {
	if d.Active {
		return OpUpdate
	}
	return OpRemove
}

// Apply applies the condition change.
func (d *ConditionData) Apply(_ any) error {
	// Store implementation would handle this
	return nil
}

// String returns a string representation.
func (d *ConditionData) String() string {
	if d.Active {
		return fmt.Sprintf("%s gains condition: %s", d.EntityID, d.Condition)
	}
	return fmt.Sprintf("%s loses condition: %s", d.EntityID, d.Condition)
}

// ResourceData represents a resource change (spell slots, etc).
type ResourceData struct {
	EntityID     string
	ResourceType string
	Amount       int // Change amount
	Level        int // For leveled resources like spell slots
}

// GetEntityID returns the affected entity.
func (d *ResourceData) GetEntityID() string {
	return d.EntityID
}

// GetOperation returns update for resources.
func (d *ResourceData) GetOperation() DataOperation {
	return OpUpdate
}

// Apply applies the resource change.
func (d *ResourceData) Apply(_ any) error {
	// Store implementation would handle this
	return nil
}

// String returns a string representation.
func (d *ResourceData) String() string {
	if d.Level > 0 {
		return fmt.Sprintf("%s uses level %d %s", d.EntityID, d.Level, d.ResourceType)
	}
	return fmt.Sprintf("%s %s: %+d", d.EntityID, d.ResourceType, d.Amount)
}
