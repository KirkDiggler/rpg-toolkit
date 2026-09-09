// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// BaneRangeFeet is how far a caster may point Bane (PHB p.216).
const BaneRangeFeet = 30

// BaneTurnEnds is the ten subsequent caster turn ends in Bane's one-minute duration.
const BaneTurnEnds = 10

// BaneCasterParameter names the caster provenance required by Baned.
const BaneCasterParameter = "source_id"

// TrueStrikeRangeFeet is how far a caster may point True Strike (PHB p.283).
const TrueStrikeRangeFeet = 30

// ViciousMockeryRangeFeet is how far a caster may point Vicious Mockery
// (PHB p.285).
const ViciousMockeryRangeFeet = 60

// ViciousMockeryDamage is the psychic damage a failed save against Vicious
// Mockery takes at levels 1–4.
const ViciousMockeryDamage = "1d4"

// SacredFlameRangeFeet is Sacred Flame's range in the 2014 Basic Rules.
const SacredFlameRangeFeet = 60

// SacredFlameDamage is the radiant damage at character levels 1–4.
// Higher-level scaling is outside this level-one cast-content slice.
const SacredFlameDamage = "1d8"

// TrueStrikeTargetParameter is the True Strike condition's parameter naming
// the creature the advantage is good against.
const TrueStrikeTargetParameter = "target_id"

// TrueStrikeTurnEnds is how many of the caster's turn ends the concentration
// survives: the end of the turn it was cast on, and the end of the next one.
//
// Two, and the first one is the reason. A cantrip costs an action, so True
// Strike is always cast DURING the caster's turn — the very next turn end on
// the bus is that same turn's. Ending there would mean the advantage was never
// available on any attack, since the caster has already spent their action.
const TrueStrikeTurnEnds = 2

// ViciousMockeryCasterParameter is the Vicious Mockery condition's parameter
// naming the bard who imposed it.
const ViciousMockeryCasterParameter = "source_id"

// BardCantrips is the 2014 PHB bard cantrip list, in book order.
//
// ALL ELEVEN, even though this build can cast two of them. The list is what a
// bard's cantrips ARE; which of them have behavior is a separate question
// answered by [Castable], and the day a third grows a profile the two answers
// converge without this list moving.
var BardCantrips = []Spell{
	BladeWard,
	DancingLights,
	Friends,
	Light,
	MageHand,
	Mending,
	Message,
	MinorIllusion,
	Prestidigitation,
	TrueStrike,
	ViciousMockery,
}

// castProfileBuilder is one spell's compiled cast content and price, with the
// caster's own save DC supplied later because it is not a property of the spell.
type castProfileBuilder struct {
	name  string
	cost  *combat.SpendProfile
	build func(spellSaveDC int) actions.CastProfile
}

func cantripCost() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}
}

func baneCost() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools: map[coreResources.ResourceKey]int{resources.SpellSlotLevel1: 1},
	}
}

// castContent is the cast table, keyed by spell id. A spell absent from it has
// no cast behavior in this build, which is a fact about the build rather than a
// gap to paper over: nine of the bard's eleven cantrips are absent.
var castContent = map[Spell]castProfileBuilder{
	Bane: {
		name: "Bane",
		cost: baneCost(),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet:  BaneRangeFeet,
				Target:     actions.CastTargetOneCreature,
				MinTargets: 1,
				MaxTargets: 3,
				Save: &saves.SaveGate{
					Abilities:  []abilities.Ability{abilities.CHA},
					DC:         saves.DCStatic(spellSaveDC),
					OnSuccess:  saves.Negated,
					Recurrence: saves.RecurrenceNone,
				},
				Effects: []actions.CastEffect{{
					Recipient:      actions.CastRecipientTarget,
					Ref:            *refs.Conditions.Baned(),
					CounterpartKey: BaneCasterParameter,
				}},
				Concentration: &actions.CastConcentration{
					TurnEnds: BaneTurnEnds, SkipFirstTurnEnd: true,
				},
			}
		},
	},
	SacredFlame: {
		name: "Sacred Flame",
		cost: cantripCost(),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet:  SacredFlameRangeFeet,
				Target:     actions.CastTargetOneCreature,
				MinTargets: 1,
				MaxTargets: 1,
				Save: &saves.SaveGate{
					Abilities:  []abilities.Ability{abilities.DEX},
					DC:         saves.DCStatic(spellSaveDC),
					OnSuccess:  saves.Negated,
					Recurrence: saves.RecurrenceNone,
				},
				Damage: []damage.Damage{{Dice: SacredFlameDamage, Type: damage.Radiant}},
			}
		},
	},
	TrueStrike: {
		name: "True Strike",
		cost: cantripCost(),
		build: func(_ int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet:  TrueStrikeRangeFeet,
				Target:     actions.CastTargetOneCreature,
				MinTargets: 1,
				MaxTargets: 1,
				// NO GATE. Nobody resists True Strike: it names a creature and
				// grants the caster something. This is the whole gateless half
				// of the cast door.
				//
				// RAW's True Strike is a concentration cantrip, and here it is
				// one: the caster holds it for two turn ends and the advantage
				// ends with the hold.
				Concentration: &actions.CastConcentration{TurnEnds: TrueStrikeTurnEnds},
				Effects: []actions.CastEffect{{
					Recipient:      actions.CastRecipientCaster,
					Ref:            *refs.Conditions.TrueStrike(),
					CounterpartKey: TrueStrikeTargetParameter,
				}},
			}
		},
	},
	ViciousMockery: {
		name: "Vicious Mockery",
		cost: cantripCost(),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet:  ViciousMockeryRangeFeet,
				Target:     actions.CastTargetOneCreature,
				MinTargets: 1,
				MaxTargets: 1,
				Save: &saves.SaveGate{
					Abilities: []abilities.Ability{abilities.WIS},
					DC:        saves.DCStatic(spellSaveDC),
					// A successful save negates BOTH halves — RAW 2014 is "on
					// a failed save it takes 1d4 psychic damage and has
					// disadvantage on its next attack roll", with nothing on a
					// success. Half-on-success arrives with a spell that has
					// one.
					OnSuccess:  saves.Negated,
					Recurrence: saves.RecurrenceNone,
				},
				Damage: []damage.Damage{{
					Dice: ViciousMockeryDamage,
					Type: damage.Psychic,
				}},
				Effects: []actions.CastEffect{{
					Recipient:      actions.CastRecipientTarget,
					Ref:            *refs.Conditions.ViciousMockery(),
					CounterpartKey: ViciousMockeryCasterParameter,
				}},
			}
		},
	},
}

// CastDefinitionInput contains the exact caster-specific facts needed to
// compile one spell definition. SpellSaveDC is a difficulty class, not a
// spell, slot, or character level.
type CastDefinitionInput struct {
	Spell       Spell
	SpellSaveDC int
}

// CastDefinition returns the action definition for one spell, with SpellSaveDC
// written into the gate of a spell that has one, or nil when this build has no
// cast content for the id.
//
// NIL RATHER THAN AN ERROR, because "this build cannot cast Mage Hand" is an
// ordinary answer rather than a failure: whoever mints Cast rows asks this
// about every cantrip a character knows and mints no row for a nil. A row that
// resolved to nothing would be the lie; a missing row is the truth.
//
// The DC is a parameter and not content because it belongs to the caster
// (8 + proficiency + spellcasting modifier), which is why the same Vicious
// Mockery is DC 13 for one bard and DC 12 for another.
func CastDefinition(input CastDefinitionInput) *actions.Definition {
	content, ok := castContent[input.Spell]
	if !ok {
		return nil
	}

	ref := refs.Spells.ByID(string(input.Spell))
	if ref == nil {
		// The table and the ref catalog disagreeing is a build defect rather
		// than a runtime condition, and returning nil says the same thing the
		// missing-content answer says: no row.
		return nil
	}

	profile := content.build(input.SpellSaveDC)
	return &actions.Definition{
		Ref:  *ref,
		Name: content.name,
		Cost: actions.CloneSpendProfile(content.cost),
		Cast: &profile,
	}
}

// HasCastProfile reports whether this build can cast the spell at all.
func HasCastProfile(id Spell) bool {
	_, ok := castContent[id]
	return ok
}

// Castable returns the subset of ids this build has cast content for,
// preserving the order given.
//
// This is the gate on a class's cantrip option list: a level-1 bard is offered
// the cantrips that do something, the way the class list offers the classes
// that do something. The cost is that "choose 2 of 2" is not a choice, which is
// honest about where the build is and disappears the moment a third cantrip
// gets a profile.
func Castable(ids []Spell) []Spell {
	out := make([]Spell, 0, len(ids))
	for _, id := range ids {
		if HasCastProfile(id) {
			out = append(out, id)
		}
	}
	return out
}
