// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// prepareMonsterConditions loads every carried blob before recording any behavior.
// It takes no bus and marks no dirty state. Unknown data restores the original
// carried bytes, so even an attach caller that starts from monster.Load can retry.
func prepareMonsterConditions(m *monster.Monster) error {
	carried := m.TakeUnappliedConditions()
	prepared := make([]dndevents.ConditionBehavior, 0, len(carried))
	restore := func() {
		for _, blob := range carried {
			m.AddTraitData(blob)
		}
	}
	for i, blob := range carried {
		condition, err := conditions.LoadJSON(blob)
		if err != nil {
			restore()
			return rpgerr.Wrapf(err, "failed to load monster condition %d: %s", i, blob)
		}
		if err := requireNameableTrait(condition); err != nil {
			restore()
			return rpgerr.Wrapf(err, "failed to name monster condition %d: %s", i, blob)
		}
		prepared = append(prepared, condition)
	}
	// Nameability is AddLoadedCondition's only refusal, validated for every entry
	// above. A future additional admission rule must also be preflighted here.
	for _, condition := range prepared {
		if err := m.AddLoadedCondition(condition); err != nil {
			return rpgerr.Wrap(err, "failed to record prepared monster condition")
		}
	}
	return nil
}
