// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// AbilityDefinition compiles the description of a feature this character has,
// with the character's own numbers. A ref the character does not have, and a
// feature that carries no description yet, answer false and no error.
func (c *Character) AbilityDefinition(ref core.Ref) (*actions.Definition, bool, error) {
	for _, feature := range c.features {
		held := feature.Ref()
		if held == nil || *held != ref {
			continue
		}
		described, ok := feature.(features.Described)
		if !ok {
			return nil, false, nil
		}
		definition, err := described.Definition(c)
		if err != nil {
			return nil, false, err
		}
		return definition, true, nil
	}
	return nil, false, nil
}
