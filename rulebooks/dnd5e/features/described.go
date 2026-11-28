// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
)

// Described is a feature that can say what it does as an action description,
// compiled with its owner's numbers. A feature without it carries no
// description yet, which is not a fault.
type Described interface {
	Definition(owner core.Entity) (*actions.Definition, error)
}

// ErrUsedFromItsDescription is what a feature that carries a description
// answers when asked to activate itself: it is used from its description, not
// from its own code.
var ErrUsedFromItsDescription = errors.New(
	"this ability is used from its description: build the action with Definition and pay its cost at the gate")

var _ Described = (*SecondWind)(nil)
var _ Described = (*ActionSurge)(nil)
