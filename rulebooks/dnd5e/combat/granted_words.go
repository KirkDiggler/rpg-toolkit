// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"fmt"
	"strings"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
)

// GrantedWords says the turn slots a price gives, as a person reads them:
// "1 action", "2 actions", "1 bonus action", "1 reaction". Several are joined
// with " and " in the order action, bonus action, reaction. A nil price, a
// price that gives no slot, and a price that gives only capacity answer empty.
//
// It says nothing about [SpendProfile.Grants]. The words for banked capacity
// are the character package's, per ability (describeGrantedCapacity); they are
// neither moved nor repeated here.
func GrantedWords(p *SpendProfile) string {
	if p == nil {
		return ""
	}

	var parts []string
	for _, entry := range []struct {
		slot             coreCombat.ActionType
		singular, plural string
	}{
		{coreCombat.ActionStandard, "action", "actions"},
		{coreCombat.ActionBonus, "bonus action", "bonus actions"},
		{coreCombat.ActionReaction, "reaction", "reactions"},
	} {
		n := p.GrantSlots[entry.slot]
		switch {
		case n <= 0:
		case n == 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, entry.singular))
		default:
			parts = append(parts, fmt.Sprintf("%d %s", n, entry.plural))
		}
	}

	return strings.Join(parts, " and ")
}
