// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Display is the rulebook-owned, display-ready projection of one condition
// ref: a never-empty display name and optional server-authored detail. It is
// the value a character StatusView (Task 4) looks up by ref so the projection
// never has to serialize a condition back to JSON to recover a human-readable
// label, and so the set of conditions a status view can name is bounded by
// this catalog rather than by whatever a live condition happens to carry.
type Display struct {
	// Name is the condition's display name. Never empty for a known ref.
	Name string

	// Detail is optional, server/toolkit-composed display text. May be empty.
	Detail string
}

// DisplayFor returns the display descriptor for the given condition ref, or
// (Display{}, false) when the ref is not in the rulebook-owned catalog. The
// character projection turns a false result into a hard error rather than
// silently dropping the condition, so an unknown ref fails loudly.
func DisplayFor(ref core.Ref) (Display, bool) {
	d, ok := displayCatalog[ref.String()]
	return d, ok
}

// displayCatalog maps the canonical ref string of every status-visible
// condition, including supported spell-delivered conditions, to its display
// descriptor.
// It is keyed by ref.String() because at least one condition — Sneak Attack —
// names itself by a feature ref (refs.Features.SneakAttack) rather than a
// condition ref, so the type alone is not enough to disambiguate.
//
// The existing Shield spell condition remains excluded because it is not
// promoted into status projection; Baned is explicitly status-visible.
var displayCatalog = map[string]Display{
	refs.Conditions.Shillelagh().String(): {Name: "Shillelagh", Detail: "Selected held weapon: 1d8 magical bludgeoning; uses the better of Strength or spellcasting ability."},
	refs.Conditions.InFog().String():      {Name: InFogName, Detail: "Inside Fog Cloud. Sight is blocked by the fog."},
	// Fighting styles (Fighter).
	refs.Conditions.FightingStyleArchery().String():             {Name: "Archery", Detail: "Adds 2 to attack rolls you make with ranged weapons."},
	refs.Conditions.FightingStyleDefense().String():             {Name: "Defense", Detail: "Adds 1 to your Armor Class while you wear armor."},
	refs.Conditions.FightingStyleDueling().String():             {Name: "Dueling", Detail: "Adds 2 damage when you wield a melee weapon in one hand and no other weapon."},
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): {Name: "Great Weapon Fighting", Detail: "Rerolls weapon damage dice that roll a 1 or 2 when you attack with a melee weapon held in both hands."},
	refs.Conditions.FightingStyleProtection().String():          {Name: "Protection"},
	refs.Conditions.FightingStyleTwoWeaponFighting().String():   {Name: "Two-Weapon Fighting", Detail: "Adds your ability modifier to the damage of your off-hand attack."},

	// Barbarian.
	refs.Conditions.Raging().String(): {
		Name:   "Raging",
		Detail: "Adds your rage damage bonus to melee weapon attacks that use Strength. Also grants advantage on Strength-based skill checks and Strength saving throws, and resistance to bludgeoning, piercing, and slashing damage.",
	},
	refs.Conditions.RecklessAttack().String(): {Name: "Reckless Attack", Detail: "Your melee weapon attacks using Strength have advantage until your next turn, and attacks against you have advantage."},
	refs.Conditions.BrutalCritical().String(): {Name: "Brutal Critical", Detail: "Adds extra weapon damage dice when you score a critical hit."},

	// Fighter (champion).
	refs.Conditions.ImprovedCritical().String(): {Name: "Improved Critical", Detail: "Your weapon attacks score a critical hit on a roll below 20 as well as on a 20."},

	// Monk.
	refs.Conditions.MartialArts().String():       {Name: "Martial Arts", Detail: "Unarmed strikes and monk weapons can use Dexterity, and unarmed strikes deal your Martial Arts die."},
	refs.Conditions.UnarmoredDefense().String():  {Name: "Unarmored Defense", Detail: "While you wear no armor, your Armor Class is 10 plus your Dexterity modifier plus your Constitution (barbarian) or Wisdom (monk) modifier. A shield still counts."},
	refs.Conditions.UnarmoredMovement().String(): {Name: "Unarmored Movement"},

	// Rogue. Sneak Attack names itself by a feature ref, not a condition ref.
	refs.Features.SneakAttack().String(): {
		Name:   "Sneak Attack",
		Detail: "Once per turn, adds extra damage to a qualifying attack when you have advantage or another enemy of the target is within 5 feet.",
	},

	// Turn-based / combat-ability conditions.
	refs.Conditions.Dodging().String():        {Name: "Dodging", Detail: "Attack rolls against you have disadvantage, and you have advantage on Dexterity saving throws."},
	refs.Conditions.Disengaging().String():    {Name: "Disengaging"},
	refs.Conditions.Hidden().String():         {Name: "Hidden", Detail: "Your attacks have advantage and attacks against you have disadvantage. Attacking ends it."},
	refs.Conditions.Helped().String():         {Name: "Helped", Detail: "Your next attack roll has advantage."},
	refs.Conditions.Inspired().String():       {Name: InspiredName, Detail: "Holds a Bardic Inspiration die. After seeing your attack roll you may add it; the die is spent only when you take it."},
	refs.Conditions.BladeWard().String():      {Name: BladeWardName, Detail: "Bludgeoning, piercing and slashing damage dealt to you by weapon attacks is halved."},
	refs.Conditions.GuidingBolt().String():    {Name: GuidingBoltName, Detail: "The next attack roll against you has advantage."},
	refs.Conditions.TrueStrike().String():     {Name: TrueStrikeName, Detail: "Your next attack against the chosen target has advantage."},
	refs.Conditions.ViciousMockery().String(): {Name: ViciousMockeryName, Detail: "Your next attack roll has disadvantage."},
	refs.Conditions.Commanded().String():      {Name: CommandedName},
	refs.Conditions.Concentrating().String():  {Name: ConcentratingName},
	refs.Conditions.Baned().String():          {Name: BanedName, Detail: "Subtracts 1d4 from attack rolls and saving throws. Multiple Bane effects do not subtract extra dice from the same roll."},
	refs.Conditions.Blessed().String(): {
		Name:   BlessedName,
		Detail: "Adds 1d4 to attack rolls and saving throws. Multiple Bless effects do not add extra dice to the same roll.",
	},
	refs.Conditions.Prone().String():             {Name: "Prone", Detail: "Your attacks have disadvantage. Attacks against you have advantage from within 5 feet and disadvantage from farther away."},
	refs.Conditions.OpportunityAttack().String(): {Name: "Opportunity Attack"},

	// Standard conditions reachable by the four builds.
	refs.Conditions.Unconscious().String(): {Name: "Unconscious"},

	// Cleric cantrips. Guided is a pre-existing gap found alongside
	// Resistance's own: Guidance shipped and merged without an entry here,
	// so any real Guidance cast whose activation gets recorded has hit this
	// same "no display catalog entry" error since it merged. Fixed here
	// rather than filed separately since it is a one-line addition to the
	// exact catalog Resistance's own entry touches, for the same reason.
	refs.Conditions.DivineFavor().String():     {Name: DivineFavorName, Detail: "Your weapon attacks deal an extra 1d4 radiant damage."},
	refs.Conditions.FaerieFire().String():      {Name: FaerieFireName, Detail: "Attack rolls against you have advantage if the attacker can see you."},
	refs.Conditions.ShieldOfFaith().String():   {Name: ShieldOfFaithName, Detail: "Adds 2 to your Armor Class."},
	refs.Conditions.Guided().String():          {Name: GuidedName},
	refs.Conditions.Resistance().String():      {Name: ResistanceName},
	refs.Conditions.Sanctuary().String():       {Name: SanctuaryName, Detail: "A creature that targets you must first succeed on a Wisdom saving throw. Making an attack ends the ward."},
	refs.Conditions.SanctuaryImmune().String(): {Name: SanctuaryImmuneName},
}
