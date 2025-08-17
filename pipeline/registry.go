package pipeline

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Registry manages pipeline factories.
// Since pipelines can have different input/output types, we store them as interface{}
// and require type assertions when retrieving.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]any // Stores Factory[I,O] as any
}

// NewRegistry creates a new pipeline registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]any),
	}
}

// Factory creates pipeline instances.
type Factory[I, O any] interface {
	// Create creates a new pipeline instance
	Create() Pipeline[I, O]
}

// Register registers a pipeline factory with a ref.
func (r *Registry) Register(ref *core.Ref, factory any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[ref.String()] = factory
}

// Get returns a pipeline by ref with type assertion.
func Get[I, O any](r *Registry, ref *core.Ref) (Pipeline[I, O], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[ref.String()]
	if !exists {
		return nil, fmt.Errorf("pipeline not found: %s", ref.String())
	}

	// Type assert to the expected factory type
	typedFactory, ok := factory.(Factory[I, O])
	if !ok {
		return nil, fmt.Errorf("pipeline factory type mismatch for: %s", ref.String())
	}

	return typedFactory.Create(), nil
}

// GetByString returns a pipeline by ref string with type assertion.
func GetByString[I, O any](r *Registry, refStr string) (Pipeline[I, O], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[refStr]
	if !exists {
		return nil, fmt.Errorf("pipeline not found: %s", refStr)
	}

	// Type assert to the expected factory type
	typedFactory, ok := factory.(Factory[I, O])
	if !ok {
		return nil, fmt.Errorf("pipeline factory type mismatch for: %s", refStr)
	}

	return typedFactory.Create(), nil
}