package pipeline

// CompletedResult represents a pipeline that finished execution.
type CompletedResult struct {
	Output any    // The pipeline's output
	Data   []Data // State changes to apply
}

// IsComplete returns true for completed results.
func (r CompletedResult) IsComplete() bool {
	return true
}

// GetData returns the state changes.
func (r CompletedResult) GetData() []Data {
	return r.Data
}

// GetOutput returns the pipeline output.
func (r CompletedResult) GetOutput() any {
	return r.Output
}

// GetContinuation returns nil for completed results.
func (r CompletedResult) GetContinuation() *ContinuationData {
	return nil
}

// SuspendedResult represents a pipeline awaiting a decision.
type SuspendedResult struct {
	Continuation ContinuationData // State to resume from
	Decision     DecisionRequest  // What decision is needed
	Data         []Data           // Partial data to apply
}

// IsComplete returns false for suspended results.
func (r SuspendedResult) IsComplete() bool {
	return false
}

// GetData returns any partial data.
func (r SuspendedResult) GetData() []Data {
	return r.Data
}

// GetOutput returns nil for suspended results.
func (r SuspendedResult) GetOutput() any {
	return nil
}

// GetContinuation returns the continuation data.
func (r SuspendedResult) GetContinuation() *ContinuationData {
	return &r.Continuation
}

// DecisionRequest describes a decision needed from a player.
type DecisionRequest struct {
	Type     DecisionType     `json:"type"`
	EntityID string           `json:"entity_id"`
	Options  []DecisionOption `json:"options"`
	Context  map[string]any   `json:"context"`
}

// DecisionType categorizes the type of decision.
type DecisionType string

const (
	// DecisionReaction is for reactions like Shield
	DecisionReaction DecisionType = "reaction"

	// DecisionChoice is for general choices
	DecisionChoice DecisionType = "choice"
)

// DecisionOption represents one choice in a decision.
type DecisionOption struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
}
