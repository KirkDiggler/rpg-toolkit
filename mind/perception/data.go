// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// Data is the persistent representation of Perception state. It wraps
// intel's own Data verbatim — perception adds no state of its own, so the
// JSON shape is free.
type Data struct {
	Intel intel.Data `json:"intel"`
}

// ToData returns a persistent snapshot of this Perception.
func (p *Perception) ToData() Data {
	return Data{Intel: p.intel.ToData()}
}

// Load reconstructs a Perception from persistent data. It rejects whatever
// intel.LoadIntel rejects, wrapped.
func Load(d Data) (*Perception, error) {
	i, err := intel.LoadIntel(d.Intel)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return &Perception{intel: i}, nil
}
