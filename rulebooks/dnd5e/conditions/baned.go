// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"

const (
	// BanedName is the display name of the named subtractive roll composition.
	BanedName = "Baned"
	// BanedContributionGroup selects one equal-potency Bane contribution.
	BanedContributionGroup = "bane"
)

var banedDescriptor = namedLinkedDescriptor(refs.Conditions.Baned(),
	Display{Name: BanedName, Detail: "Subtracts 1d4 from attack rolls and saving throws. Multiple Bane effects do not subtract extra dice from the same roll."},
	namedRollParts(BanedContributionGroup, true))
