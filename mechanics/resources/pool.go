// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

// Pool holds collections of resources and counters.
// It's just organized storage - no complex logic.
type Pool struct {
	Resources map[string]*Resource
	Counters  map[string]*Counter
}

// NewPool creates a new empty resource pool.
func NewPool() *Pool {
	return &Pool{
		Resources: make(map[string]*Resource),
		Counters:  make(map[string]*Counter),
	}
}

// AddResource adds a resource to the pool.
func (p *Pool) AddResource(r *Resource) {
	if r != nil {
		p.Resources[r.ID] = r
	}
}

// AddCounter adds a counter to the pool.
func (p *Pool) AddCounter(c *Counter) {
	if c != nil {
		p.Counters[c.ID] = c
	}
}

// GetResource retrieves a resource by ID.
func (p *Pool) GetResource(id string) (*Resource, bool) {
	r, ok := p.Resources[id]
	return r, ok
}

// GetCounter retrieves a counter by ID.
func (p *Pool) GetCounter(id string) (*Counter, bool) {
	c, ok := p.Counters[id]
	return c, ok
}

// RemoveResource removes a resource from the pool.
func (p *Pool) RemoveResource(id string) {
	delete(p.Resources, id)
}

// RemoveCounter removes a counter from the pool.
func (p *Pool) RemoveCounter(id string) {
	delete(p.Counters, id)
}

// Clear removes all resources and counters.
func (p *Pool) Clear() {
	p.Resources = make(map[string]*Resource)
	p.Counters = make(map[string]*Counter)
}

// RestoreAllResources restores all resources to full.
func (p *Pool) RestoreAllResources() {
	for _, r := range p.Resources {
		r.RestoreToFull()
	}
}

// ResetAllCounters resets all counters to zero.
func (p *Pool) ResetAllCounters() {
	for _, c := range p.Counters {
		c.Reset()
	}
}