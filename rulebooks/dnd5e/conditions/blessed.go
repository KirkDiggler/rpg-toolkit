// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"

const (
	// BlessedName is the display name of the named additive roll composition.
	BlessedName = "Blessed"
	// BlessedContributionGroup selects one equal-potency Bless contribution.
	BlessedContributionGroup = "bless"
)

var blessedDescriptor = namedLinkedDescriptor(refs.Conditions.Blessed(),
	Display{Name: BlessedName, Detail: "Adds 1d4 to attack rolls and saving throws. Multiple Bless effects do not add extra dice to the same roll."},
	namedRollParts(BlessedContributionGroup, false))
