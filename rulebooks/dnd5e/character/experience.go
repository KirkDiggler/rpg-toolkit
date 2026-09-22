// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import "github.com/KirkDiggler/rpg-toolkit/rpgerr"

// experienceThresholds is the 2014 Character Advancement table (PHB p.15): the
// total experience a character must hold to be entitled to each level.
//
// Index 0 is level 1, which costs nothing — a character is entitled to its
// first level by existing. The table is the whole of the rule: entitlement is
// the highest level whose threshold this character's total has reached.
var experienceThresholds = []int{
	0,      // 1
	300,    // 2
	900,    // 3
	2700,   // 4
	6500,   // 5
	14000,  // 6
	23000,  // 7
	34000,  // 8
	48000,  // 9
	64000,  // 10
	85000,  // 11
	100000, // 12
	120000, // 13
	140000, // 14
	165000, // 15
	195000, // 16
	225000, // 17
	265000, // 18
	305000, // 19
	355000, // 20
}

// MaxCharacterLevel is the highest level the 2014 advancement table reaches.
// Experience past its threshold entitles a character to nothing further.
const MaxCharacterLevel = 20

// EntitledLevelForExperience is the highest level a total of experience has
// earned (design R4.9: "Level entitlement is derived from the total by a
// threshold table the toolkit owns... Entitlement is not level: it is what the
// character MAY take.").
//
// Never below 1 and never above [MaxCharacterLevel]: a character with no
// experience is still entitled to the level it was created at, and a character
// with more than the table describes has run out of table, not out of rule.
func EntitledLevelForExperience(experience int) int {
	entitled := 1
	for level, threshold := range experienceThresholds {
		if experience >= threshold {
			entitled = level + 1
		}
	}
	return entitled
}

// NextExperienceThreshold is the total a character needs for the level after
// the one it is entitled to, or 0 when there is no next level.
//
// Zero tells the truth here rather than hiding a level: a level-20 character
// has no next threshold, and "0 more experience needed" is not a reading anyone
// can arrive at from a table whose first threshold is 0 for level 1.
func NextExperienceThreshold(experience int) int {
	entitled := EntitledLevelForExperience(experience)
	if entitled >= MaxCharacterLevel {
		return 0
	}
	return experienceThresholds[entitled]
}

// ExperienceThresholdForLevel is the total experience a level requires. Levels
// outside the table return 0, which is what level 1 costs.
func ExperienceThresholdForLevel(level int) int {
	if level < 1 || level > MaxCharacterLevel {
		return 0
	}
	return experienceThresholds[level-1]
}

// Experience is the total experience this character holds.
//
// Cumulative and never debited (R4.8). The source R4.8 was waiting for has
// arrived and it is the fall of a monster: the monster's authored worth,
// divided among the players on the roster at that moment, reaches each sheet
// through [Character.AddExperience]. The session SDK is what applies it; the
// toolkit owns the worth, the split and the refusal, and nothing over the wire
// can write this total (R4.12).
//
// A fixture still seeds it by writing [Data.Experience].
func (c *Character) Experience() int {
	return c.experience
}

// AddExperience grants experience to this character. It is the only mutator
// the total has.
//
// The total ONLY GROWS (R4.8): zero and negative amounts are refused with an
// invalid-argument error naming the amount, rather than being clamped away.
// A grant of nothing is a caller that computed nothing — a share rounded to
// zero, a monster worth nothing — and the caller, not this sheet, is the place
// that decides whether that is worth reporting.
//
// Nothing else on the character moves. The level it is now entitled to is
// DERIVED ([Character.EntitledLevel]) and the level it holds is the record, so
// a grant that crosses a threshold takes no level: it opens one for Advance to
// take.
func (c *Character) AddExperience(amount int) error {
	if amount <= 0 {
		return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
			rpgerr.WithMeta("amount", amount),
		}, "experience granted must be positive, got %d", amount)
	}

	c.experience += amount

	// The total is persisted on the sheet, and a sheet that did not report
	// itself dirty would have the grant discarded by the next write-back
	// (rpg-toolkit#1087).
	c.dirty = true

	return nil
}

// ExperienceShare is one player's cut of a defeated monster's worth: the total
// divided equally among the party, rounded down (RAW, 2014 DMG p.82 "Creating
// Encounters" — the experience for a monster is divided evenly among the
// characters).
//
// The party is the players on the roster at the moment the monster falls; who
// swung and who was unconscious does not enter into it. A party of nobody and a
// monster worth nothing both share out zero, which is the honest answer to a
// division that has no one to divide among.
//
// The remainder is dropped rather than distributed: RAW divides evenly and says
// nothing about the change, and a rule that handed the odd point to somebody
// would have to pick who.
func ExperienceShare(total, partySize int) int {
	if partySize <= 0 || total <= 0 {
		return 0
	}

	return total / partySize
}

// EntitledLevel is the highest level this character's experience has earned.
//
// The gap between this and [Character.GetLevel] IS the "level up available"
// signal (R4.10). There is no flag and nothing stored: a character entitled to
// more than it has taken may keep playing, and the day it advances the gap
// closes itself.
func (c *Character) EntitledLevel() int {
	return EntitledLevelForExperience(c.experience)
}

// NextLevelThreshold is the experience total this character needs before it is
// entitled to another level, or 0 when it is entitled to the last one the table
// describes.
func (c *Character) NextLevelThreshold() int {
	return NextExperienceThreshold(c.experience)
}
