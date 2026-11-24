// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
)

// Described is a feature that can say what it does as an action description,
// compiled with its owner's numbers. A feature without it carries no
// description yet, which is not a fault.
type Described interface {
	Definition(owner core.Entity) (*actions.Definition, error)
}

var _ Described = (*SecondWind)(nil)
