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
// prints passive Insight, so the only honest source is what the stat block
// does print. A stored copy would be a second answer that goes stale the
// moment the stat block moves.
//
// # 10 + the listed Insight, or 10 + the Wisdom modifier
//
// A LISTED SKILL IS A TOTAL, NOT A PROFICIENCY FLAG (ruled on
// rpg-project#454). An SRD stat block prints "Skills Stealth +6" and that
// +6 is the whole modifier — the goblin's is DEX +2 plus rather more than
// its +2 proficiency bonus, because Nimble Escape is in there too. So
// [ProficiencyData.Bonus] is the number, and the creature's CR-based
// proficiency bonus is NEVER added on top of one: doing that would double
// count the proficiency the listed number already includes.
//
// A stat block that lists no Insight has no printed number to use, and the
// answer falls back to 10 + its Wisdom modifier. A goblin (WIS 8) answers 9;
// a thug (WIS 10) answers 10.
//
// A listed Insight of +0 is a real answer and reads as one: passive 10, not
// "unlisted". The zero tells the truth because the list says whether the
// skill is there at all, separately from what it is worth.
func (d *Data) PassiveInsight() int {
	for _, prof := range d.Proficiencies {
		if prof.Skill == string(skills.Insight) {
			return passiveInsight(d.AbilityScores, prof.Bonus, true)
		}
	}

	return passiveInsight(d.AbilityScores, 0, false)
}

// PassiveInsight returns the loaded sheet's passive Insight — the same number
// [Data.PassiveInsight] derives from the blob it was loaded from.
//
// Both exist because both are read: a live sheet answers for anything holding
// a [Monster], and the blob answers for the session, which carries monster
// sheets as [Data] and would otherwise have to load a whole monster to ask one
// question of its stat block.
func (m *Monster) PassiveInsight() int {
	bonus, listed := m.proficiencies[string(skills.Insight)]

	return passiveInsight(m.abilityScores, bonus, listed)
}

// passiveInsight is the one derivation both accessors answer with: the listed
// total when the stat block prints one, and the bare Wisdom modifier when it
// does not.
func passiveInsight(scores shared.AbilityScores, listedBonus int, listed bool) int {
	if listed {
		return 10 + listedBonus
	}

	return 10 + scores.Modifier(abilities.WIS)
}
