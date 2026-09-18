// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// A class's spell and cantrip questions are DERIVED from its progression table,
// never authored as a row (design R4.6: "A spell or cantrip requirement is
// derived from the progression, never authored as a separate row... One fact,
// one home: the table says four then five, and 'choose one' follows.").
//
// Authoring "choose 1 spell" at level 2 beside a table that says 4 → 5 is two
// homes for one fact, and two homes can disagree. What IS authored here is the
// other half of the question: which spells this build can offer, which is a
// fact about the build and not about the class table.

// spellOptionList is what a class may choose from at one spell level, and the
// noun its label uses for them.
//
// The noun is authored rather than generated because the four caster classes
// that ask this question at level 1 each phrase it differently, and the phrasing
// is what a player reads. The COUNT in the label is always generated: a label
// carrying a count cannot be written once and be true at every level.
type spellOptionList struct {
	singular string
	plural   string
	options  []spells.Spell
}

// bardCantripOptions is the 2014 bard cantrip list GATED to the cantrips this
// build can cast, which is the one place a class list is filtered.
//
// spells.BardCantrips is what a bard's cantrips ARE; [spells.Castable] answers
// the separate question of which of them do anything here. Every other list in
// this file was hand-curated against that same question when it was written,
// so filtering them again would only remove options creation offers today.
var bardCantripOptions = spells.Castable(spells.BardCantrips)

// classCantripOptions is each class's cantrip list. A class absent from it has
// no cantrips at any level, which its progression table says too.
var classCantripOptions = map[classes.Class][]spells.Spell{
	classes.Bard: bardCantripOptions,

	classes.Cleric: {
		// Damage cantrips
		spells.SacredFlame,
		spells.TollTheDead,
		spells.WordOfRadiance,
		// Utility cantrips
		spells.Guidance,
		spells.Light,
		spells.Resistance,
		spells.SpareTheDying,
		spells.Thaumaturgy,
	},

	classes.Druid: {
		// Damage cantrips
		spells.Frostbite,
		spells.PrimalSavagery,
		spells.Thornwhip,
		spells.CreateBonfire,
		spells.Infestation,
		// Utility cantrips
		spells.Druidcraft,
		spells.Guidance,
		spells.MagicStone,
		spells.MoldEarth,
		spells.Resistance,
		spells.ShapeWater,
	},

	classes.Sorcerer: {
		// Damage cantrips
		spells.FireBolt,
		spells.RayOfFrost,
		spells.ShockingGrasp,
		spells.AcidSplash,
		spells.PoisonSpray,
		spells.ChillTouch,
		spells.BoomingBlade,
		spells.GreenFlameBlade,
		spells.SwordBurst,
		// Utility cantrips
		spells.MageHand,
		spells.MinorIllusion,
		spells.Prestidigitation,
		spells.Light,
		spells.DancingLights,
		spells.Friends,
		spells.Mending,
		spells.Message,
		spells.TrueStrike,
		spells.ControlFlames,
		spells.CreateBonfire,
	},

	classes.Warlock: {
		// Damage cantrips
		spells.EldritchBlast, // signature warlock cantrip
		spells.ChillTouch,
		spells.PoisonSpray,
		spells.SacredFlame,
		spells.TollTheDead,
		// Utility cantrips
		spells.MageHand,
		spells.MinorIllusion,
		spells.Prestidigitation,
		spells.Friends,
		spells.BladeWard,
		spells.CreateBonfire,
		spells.Infestation,
	},

	classes.Wizard: {
		// Damage cantrips
		spells.FireBolt,
		spells.RayOfFrost,
		spells.ShockingGrasp,
		spells.AcidSplash,
		spells.PoisonSpray,
		spells.ChillTouch,
		// Utility cantrips
		spells.MageHand,
		spells.MinorIllusion,
		spells.Prestidigitation,
		spells.Light,
	},
}

// clericSpellsLevel1 is the supported 1st-level cleric list. Creation takes the
// WHOLE list rather than choosing from it, which is what the cleric's
// progression placeholder counts and what its label says.
var clericSpellsLevel1 = []spells.Spell{
	spells.Bane, spells.Bless, spells.Command, spells.CureWounds, spells.HealingWord, spells.Sanctuary,
}

// classSpellOptions is each class's spell list by SPELL level — not class
// level. A class level decides how many spells are learned and at what spell
// level; this decides which ones exist to learn.
//
// Only spell level 1 is authored, because only 1st-level spells have cast
// content in this build. A class level that asks for a spell of a level absent
// here is refused rather than answered with an empty list; see
// [GetClassRequirementsGainedAtLevel].
var classSpellOptions = map[classes.Class]map[int]spellOptionList{
	classes.Bard: {
		1: {
			singular: "supported 1st-level spell",
			plural:   "supported 1st-level spells",
			// Each option has an executable cast profile. The level-one
			// known-spell count remains four as the supported catalog grows
			// (rpg-toolkit#1661).
			options: []spells.Spell{
				spells.Bane, spells.Thunderwave, spells.DissonantWhispers, spells.Command,
				spells.HealingWord,
			},
		},
	},

	classes.Cleric: {
		1: {
			singular: "supported 1st-level Cleric spell",
			plural:   "supported 1st-level Cleric spells",
			// Temporary spell access while preparation is deferred. These
			// supported class spells use the existing choice pipeline; this is
			// not a spellbook or a domain grant, and does not implement a
			// prepared-spell limit.
			options: clericSpellsLevel1,
		},
	},

	classes.Sorcerer: {
		1: {
			singular: "1st-level spell",
			plural:   "1st-level spells",
			options: []spells.Spell{
				spells.MagicMissile,
				spells.BurningHands,
				spells.ChromaticOrb,
				spells.Shield,
				spells.Sleep,
				spells.CharmPerson,
				spells.DisguiseSelf,
				spells.ExpeditiousRetreat,
				spells.FalseLife,
				spells.FogCloud,
				spells.RayOfSickness,
				spells.Thunderwave,
				spells.WitchBolt,
				spells.ColorSpray,
				spells.FeatherFall,
			},
		},
	},

	classes.Warlock: {
		1: {
			singular: "1st-level spell",
			plural:   "1st-level spells",
			options: []spells.Spell{
				spells.ArmsOfHadar,
				spells.CharmPerson,
				spells.ComprehendLanguages,
				spells.ExpeditiousRetreat,
				spells.HellishRebuke,
				spells.Hex,
				spells.ProtectionEvil,
				spells.UnseenServant,
			},
		},
	},

	classes.Wizard: {
		1: {
			singular: "1st-level spell for your spellbook",
			plural:   "1st-level spells for your spellbook",
			options: []spells.Spell{
				// Damage spells
				spells.MagicMissile,
				spells.BurningHands,
				spells.ChromaticOrb,
				spells.Thunderwave,
				spells.IceKnife,
				spells.WitchBolt,
				// Utility spells
				spells.Shield,
				spells.Sleep,
				spells.CharmPerson,
				spells.DetectMagic,
				spells.Identify,
				// Note: This is not the complete wizard spell list
			},
		},
	},
}

// CantripChoiceID is the identity of the cantrips a class learns at a class
// level (design R4.4a: "A requirement's identity includes the class level it is
// gained at: <class>-<kind>-<classLevel>").
//
// Level 1 composes to the same strings the package constants spell out —
// "bard-cantrips-1", "wizard-cantrips-1" — which is why the convention could be
// adopted without moving a single existing identifier.
func CantripChoiceID(classID classes.Class, classLevel int) ChoiceID {
	return ChoiceID(fmt.Sprintf("%s-cantrips-%d", classID, classLevel))
}

// SpellChoiceID is the identity of the spells a class learns at a class level
// (R4.4a). Level 1 composes to "bard-spells-1" and its siblings.
func SpellChoiceID(classID classes.Class, classLevel int) ChoiceID {
	return ChoiceID(fmt.Sprintf("%s-spells-%d", classID, classLevel))
}

// addDerivedSpellRequirements folds a class level's spell and cantrip questions
// into the requirements that level gained.
//
// It asks the progression table what the class knows at this level and at the
// one before it, and the difference IS the question: no difference, no
// question. A slot increase is not a question at all and is applied without
// asking (R4.6).
func addDerivedSpellRequirements(reqs *Requirements, classID classes.Class, classLevel int) {
	if reqs == nil || classLevel < 1 {
		return
	}

	row := classes.SpellProgressionAtLevel(classID, classLevel)
	previous := classes.SpellProgressionAtLevel(classID, classLevel-1)

	if gained := row.CantripsKnown - previous.CantripsKnown; gained > 0 {
		options := classCantripOptions[classID]
		reqs.Cantrips = &CantripRequirement{
			ID:      CantripChoiceID(classID, classLevel),
			Count:   gained,
			Options: options,
			Label:   spellChoiceLabel(gained, len(options), "cantrip", "cantrips"),
		}
	}

	if gained := row.SpellsKnown - previous.SpellsKnown; gained > 0 {
		spellLevel := row.HighestSpellLevel()
		list := spellOptionsFor(classID, spellLevel)
		reqs.Spellbook = &SpellbookRequirement{
			ID:         SpellChoiceID(classID, classLevel),
			Count:      gained,
			SpellLevel: spellLevel,
			Options:    list.options,
			Label:      spellChoiceLabel(gained, len(list.options), list.singular, list.plural),
		}
	}
}

// spellOptionsFor is what a class may learn at a spell level.
//
// A spell level this build has no list for returns an empty option list with a
// truthful noun. The requirement is still posed, because the class table really
// does say a spell is learned there, and an unanswerable requirement is refused
// loudly by advancement — quietly dropping the question would hand the player a
// level that silently taught them nothing.
func spellOptionsFor(classID classes.Class, spellLevel int) spellOptionList {
	if byLevel, ok := classSpellOptions[classID]; ok {
		if list, ok := byLevel[spellLevel]; ok {
			return list
		}
	}
	ordinal := spellLevelOrdinal(spellLevel)
	return spellOptionList{
		singular: ordinal + " spell",
		plural:   ordinal + " spells",
	}
}

// spellChoiceLabel writes what the player is being asked.
//
// A requirement whose count is its whole option list is not a choice, and says
// so: that is how the cleric's supported list reads, and it stays true of any
// list a class is handed entire.
func spellChoiceLabel(count, available int, singular, plural string) string {
	noun := plural
	if count == 1 {
		noun = singular
	}
	if available > 0 && count == available {
		return fmt.Sprintf("Select all %s", plural)
	}
	return fmt.Sprintf("Choose %d %s", count, noun)
}

// spellLevelOrdinal names a spell level the way the books do.
func spellLevelOrdinal(spellLevel int) string {
	switch spellLevel {
	case 1:
		return "1st-level"
	case 2:
		return "2nd-level"
	case 3:
		return "3rd-level"
	default:
		return fmt.Sprintf("%dth-level", spellLevel)
	}
}
