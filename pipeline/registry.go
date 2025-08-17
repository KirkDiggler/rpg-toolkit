package pipeline

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Registry manages pipeline factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a new pipeline registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Factory creates pipeline instances.
type Factory interface {
	// Create creates a new pipeline instance
	Create() Pipeline
}

// Register registers a pipeline factory with a ref.
func (r *Registry) Register(ref *core.Ref, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[ref.String()] = factory
}

// Get returns a pipeline by ref.
func (r *Registry) Get(ref *core.Ref) (Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[ref.String()]
	if !exists {
		return nil, fmt.Errorf("pipeline not found: %s", ref.String())
	}

	return factory.Create(), nil
}

// GetByString returns a pipeline by ref string.
func (r *Registry) GetByString(refStr string) (Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[refStr]
	if !exists {
		return nil, fmt.Errorf("pipeline not found: %s", refStr)
	}

	return factory.Create(), nil
}
