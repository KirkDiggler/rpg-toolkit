// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// PassiveInsight is the derived default DC for a shenanigan aimed at this
// monster — the number a character's Intimidation check beats when the
// placement authored none (rpg-project#454, ideas/shenanigans/intimidate.md).
//
// PASSIVE IS DERIVED, NEVER STORED (living-world §3). This is the rule that
// decides the shape: there is no `passive_insight` field on [SensesData] and
// there must not be one. [SensesData.PassivePerception] is a stored number
// because the SRD prints it on the stat block and a loader seeds it; nothing
// prints passive Insight, so the only honest source is the stat block's own
// Wisdom and proficiencies. A stored copy would be a second answer that goes
// stale the moment either moves.
//
// It reads 10 + the Wisdom modifier, plus the monster's proficiency bonus when
// the definition lists Insight among its proficiencies. A goblin (WIS 8)
// answers 9; a thug (WIS 10) answers 10.
//
// THE PROFICIENCY ENTRY IS A MEMBERSHIP TEST HERE, NOT A NUMBER.
// [ProficiencyData] carries a per-skill Bonus, and this derivation ignores it
// in favour of the creature's CR-based proficiency bonus, because that is what
// rpg-project#454 ruled. If a stat block ever needs a passive Insight that its
// own Wisdom and proficiency bonus cannot produce, that is the authored
// `intimidate:` DC on the placement, not a new field here.
func (d *Data) PassiveInsight() int {
	proficient := false
	for _, prof := range d.Proficiencies {
		if prof.Skill == string(skills.Insight) {
			proficient = true
			break
		}
	}

	return passiveInsight(d.AbilityScores, proficiencyBonusOf(d.ProficiencyBonus), proficient)
}

// PassiveInsight returns the loaded sheet's passive Insight — the same number
// [Data.PassiveInsight] derives from the blob it was loaded from.
//
// Both exist because both are read: a live sheet answers for anything holding
// a [Monster], and the blob answers for the session, which carries monster
// sheets as [Data] and would otherwise have to load a whole monster to ask one
// question of its Wisdom.
func (m *Monster) PassiveInsight() int {
	_, proficient := m.proficiencies[string(skills.Insight)]

	return passiveInsight(m.abilityScores, m.proficiencyBonus, proficient)
}

// passiveInsight is the one derivation both accessors answer with.
func passiveInsight(scores shared.AbilityScores, proficiencyBonus int, proficient bool) int {
	score := 10 + scores.Modifier(abilities.WIS)
	if proficient {
		score += proficiencyBonus
	}

	return score
}

// proficiencyBonusOf applies the same "absent means 2" rule loadMonster applies
// when it builds a sheet, so a blob and the sheet loaded from it never disagree
// about a proficient creature's passive Insight.
func proficiencyBonusOf(authored int) int {
	if authored == 0 {
		return 2
	}

	return authored
}
