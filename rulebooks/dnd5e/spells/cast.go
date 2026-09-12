// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"encoding/json"
	"strconv"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
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

// BladeWardRangeFeet is Blade Ward's reach. The spell is Range: Self and
// touches nobody, but CastProfile.Validate requires a positive range because
// the range is what a UI draws, so this is the caster's own square.
const BladeWardRangeFeet = 5

// BladeWardTurnEnds is how many of the caster's own turn ends the ward
// survives: the end of the turn it was traced on, and the end of the next one.
//
// Two, and the first one is the reason -- the same arithmetic that makes
// TrueStrikeTurnEnds two. A cantrip costs an action, so Blade Ward is always
// cast DURING the caster's turn, and the very next turn end on the bus is that
// same turn's. Ending there would mean the ward never survived to see a swing.
//
// Counted on the condition rather than anchored to a round, because a
// TurnEndEvent may carry no round at all and Round 0 means "unknown" rather
// than "round zero" -- see conditions/raging.go, which paid for that once.
const BladeWardTurnEnds = 2

// bladeWardParameters is the ward's clock, declared by content and read by the
// condition factory. Duration is the spell's to state, not the factory's to
// default.
var bladeWardParameters = json.RawMessage(`{"turn_ends":2}`)

// ViciousMockeryRangeFeet is how far a caster may point Vicious Mockery
// (PHB p.285).
const ViciousMockeryRangeFeet = 60

// ViciousMockeryDamage is the psychic damage a failed save against Vicious
// Mockery takes at levels 1–4.
const ViciousMockeryDamage = "1d4"

// ThunderclapRadiusFeet is how far Thunderclap's burst reaches from the caster.
//
// It is the spell's RANGE and its AREA at once, which is why one constant fills
// both fields of the profile: the burst is centred on the caster, so how far it
// reaches and how far it can be aimed are the same number.
const ThunderclapRadiusFeet = 5

// ThunderclapDamage is the thunder damage a failed save takes at levels 1-4.
//
// Unscaled, as Sacred Flame ships unscaled: nothing in the cast path reads a
// character level, and higher-level scaling is out of this slice (see the note
// on cantrip damage above).
const ThunderclapDamage = "1d6"

// ThunderwaveCubeFeet is the edge of Thunderwave's cube, and its range.
//
// One constant fills both fields for Thunderclap's reason: the cube
// originates from the caster, so how far it reaches and how far it can be
// aimed are the same number. The cube is anchored on the caster's own edge
// rather than centred on them ([actions.AreaOriginCasterEdge]), so it extends
// this far AWAY and the caster never stands in their own wave.
const ThunderwaveCubeFeet = 15

// ThunderwaveDamage is the thunder damage a failed save takes when Thunderwave
// is cast from a first-level slot.
//
// Unscaled by slot level, as every cantrip here ships unscaled: nothing in the
// cast path reads the slot a cast was paid from, and upcasting is out of this
// slice.
const ThunderwaveDamage = "2d8"

// DissonantWhispersRangeFeet is how far a caster may point Dissonant Whispers.
// Sixty feet, the same reach as Vicious Mockery: the bard is whispering into
// one mind, not filling a room.
const DissonantWhispersRangeFeet = 60

// DissonantWhispersDamage is the psychic damage the whisper deals. A made save
// takes half of it, rounded down, which is what the gate's Half means.
const DissonantWhispersDamage = "3d6"

// ThunderwavePushCells is how far a failed save is shoved: ten feet, two cells.
//
// Cells rather than feet because it is a MOVE budget, and the thing that walks
// it counts cells. The footprint beside it is in feet because it is a shape
// content authors against no grid at all — the two units are the two questions.
const ThunderwavePushCells = 2

// CommandRangeFeet is how far a caster may point Command: sixty feet, the same
// reach as the two whispers. A word carries as far as a taunt does.
const CommandRangeFeet = 60

// CommandTurnEnds is how long the compulsion lasts, in the COMMANDED
// creature's own turn ends.
//
// One, and the reason is the opposite of True Strike's two. The spell says
// "on its next turn", and the condition lands on somebody whose turn has not
// begun — so the first turn end it will ever see with its own subject id is
// the end of the turn the word was meant for. A second count would carry the
// compulsion into a turn the spell never bought.
const CommandTurnEnds = 1

// The three words this slice ships, as the ids the request sends back and the
// Commanded condition stores. Constants because the layer that drives a
// compelled turn switches on them, and a string typed twice is a word that
// silently stops being obeyed.
const (
	// CommandWordApproach walks the creature to the caster and stops it there.
	CommandWordApproach = "approach"

	// CommandWordFlee walks the creature as far from the caster as its legs
	// carry it.
	CommandWordFlee = "flee"

	// CommandWordGrovel puts the creature prone and ends its turn.
	CommandWordGrovel = "grovel"
)

// commandedParameters is the compulsion's clock, the one parameter content can
// fill in: the caster and the word both arrive by binding.
//
// Derived from [CommandTurnEnds] rather than typed out a second time beside
// it, so the constant the rest of the module reads and the JSON the factory
// reads cannot drift apart.
var commandedParameters = json.RawMessage(`{"turn_ends":` + strconv.Itoa(CommandTurnEnds) + `}`)

// CommandCasterParameter is the Commanded condition's parameter naming the
// caster who said the word. Bound by the cast effect's CounterpartKey, because
// Approach and Flee are measured from a creature rather than from a spell.
const CommandCasterParameter = "caster_id"

// CommandWordParameter is the Commanded condition's parameter naming which
// word was chosen. Bound by the cast effect's OptionKey from what the request
// carried, which is the first cast-time choice in this catalogue.
const CommandWordParameter = "word"

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

// BardCantrips is the cantrip list this build offers a bard: the 2014 PHB list
// in book order, then what we have added since.
//
// IT IS THIS BUILD'S LIST, NOT A TRANSCRIPTION OF ONE BOOK. It began as the
// latter and could then never hold anything we invented, which is the wrong
// shape for a toolkit whose published rulebooks are a supply of proof cases
// rather than a scope to finish. Thunderclap is the first entry that is not on
// the 2014 bard list, and it will not be the last.
//
// The idea worth keeping from the original is untouched: the list is what a
// bard's cantrips ARE; which of them have behavior is a separate question,
// answered by [Castable]. That separation is why the list has always been able
// to hold entries with no profile, and it is what lets this widen without
// lying about anything.
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
	Thunderclap,
}

// castProfileBuilder is one spell's compiled cast content and price, with the
// caster's own save DC supplied later because it is not a property of the spell.
type castProfileBuilder struct {
	casting combat.SpellCasting
	name    string
	cost    *combat.SpendProfile
	build   func(spellSaveDC int) actions.CastProfile
}

func cantripCost() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}
}

// slotCost is what a levelled spell costs: an action and one slot of the named
// pool.
//
// A PARAMETER RATHER THAN A CONSTANT because the pool is a property of the
// spell's level, and the two level-1 spells this build can cast would otherwise
// hand-write the same map twice and be free to drift apart. It was baneCost()
// while Bane was the only one; the second customer made the level the argument.
func slotCost(pool coreResources.ResourceKey) *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools: map[coreResources.ResourceKey]int{pool: 1},
	}
}

// castContent is the cast table, keyed by spell id. A spell absent from it has
// no cast behavior in this build, which is a fact about the build rather than a
// gap to paper over: nine of the bard's eleven cantrips are absent.
var castContent = map[Spell]castProfileBuilder{
	HealingWord: {
		name:    "Healing Word",
		casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction},
		cost: &combat.SpendProfile{
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
			Pools: map[coreResources.ResourceKey]int{resources.SpellSlotLevel1: 1},
		},
		build: func(_ int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet: 60, Target: actions.CastTargetOneCreature, MinTargets: 1, MaxTargets: 1,
				Healing: &healing.Declaration{Dice: "1d4"}, HealingExcludes: []string{"undead", "construct"},
			}
		},
	},
	CureWounds: {
		casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction},
		name:    "Cure Wounds",
		cost:    slotCost(resources.SpellSlotLevel1),
		build: func(_ int) actions.CastProfile {
			return actions.CastProfile{RangeFeet: 5, Target: actions.CastTargetTouch, MinTargets: 1, MaxTargets: 1,
				Healing: &healing.Declaration{Dice: "1d8"}, HealingExcludes: []string{"undead", "construct"}}
		},
	},
	Bane: {
		casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction},
		name:    "Bane",
		cost:    slotCost(resources.SpellSlotLevel1),
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
	Thunderclap: {
		casting: combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction},
		name:    "Thunderclap",
		cost:    cantripCost(),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				// The range and the radius are the same number: a burst
				// centred on the caster reaches exactly as far as it can be
				// aimed. RangeFeet is still declared because it is what a UI
				// draws, the way a self-targeted cast declares one.
				RangeFeet: ThunderclapRadiusFeet,
				Target:    actions.CastTargetArea,
				Area: &actions.CastArea{
					Footprint: actions.Footprint{
						Shape:    actions.AreaRadius,
						SizeFeet: ThunderclapRadiusFeet,
						Origin:   actions.AreaOriginCaster,
					},
					// "each creature other than you" — the caster stands in
					// their own burst and is not affected by it. A projection
					// of the footprint, never part of its shape.
					Catches: actions.AreaCatchesOthers,
				},
				Save: &saves.SaveGate{
					Abilities:  []abilities.Ability{abilities.CON},
					DC:         saves.DCStatic(spellSaveDC),
					OnSuccess:  saves.Negated,
					Recurrence: saves.RecurrenceNone,
				},
				Damage: []damage.Damage{{Dice: ThunderclapDamage, Type: damage.Thunder}},
			}
		},
	},
	Thunderwave: {
		casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction},
		name:    "Thunderwave",
		cost:    slotCost(resources.SpellSlotLevel1),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				// The range and the cube's edge are the same number: a wave
				// that originates from the caster reaches exactly as far as it
				// can be aimed. RangeFeet is declared rather than left at zero
				// because CastProfile.Validate requires a positive one — the
				// range is what a UI draws, which is also why Blade Ward
				// declares the caster's own square.
				RangeFeet: ThunderwaveCubeFeet,
				Target:    actions.CastTargetArea,
				Area: &actions.CastArea{
					Footprint: actions.Footprint{
						Shape:    actions.AreaBox,
						SizeFeet: ThunderwaveCubeFeet,
						// Anchored on the caster's EDGE, not centred on them.
						// A cube centred on the caster would put its first
						// five feet on the caster's own square; this one
						// stands entirely in front of them.
						Origin: actions.AreaOriginCasterEdge,
					},
					// "each creature in a 15-foot cube originating from you" —
					// the caster is not in the cube at all, and the projection
					// still says so for the same reason Thunderclap's does.
					Catches: actions.AreaCatchesOthers,
				},
				Save: &saves.SaveGate{
					Abilities: []abilities.Ability{abilities.CON},
					DC:        saves.DCStatic(spellSaveDC),
					// Half arrived with Dissonant Whispers, and this row
					// promised to flip the moment it did.
					OnSuccess:  saves.Half,
					Recurrence: saves.RecurrenceNone,
				},
				Damage: []damage.Damage{{Dice: ThunderwaveDamage, Type: damage.Thunder}},
				// The push, and every field that makes it the least permissive
				// directive there is comes from a zero value: it pays nothing
				// and provokes nothing. Content says "straight away from me,
				// two cells"; which cells those actually are is read from the
				// map by the layer that owns it, so a wave stops at a pillar
				// without this profile knowing a pillar exists.
				Move: &actions.CastMove{
					Policy: actions.MoveLine,
					Cells:  ThunderwavePushCells,
				},
			}
		},
	},
	DissonantWhispers: {
		casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction},
		name:    "Dissonant Whispers",
		cost:    slotCost(resources.SpellSlotLevel1),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet:  DissonantWhispersRangeFeet,
				Target:     actions.CastTargetOneCreature,
				MinTargets: 1,
				MaxTargets: 1,
				Save: &saves.SaveGate{
					Abilities: []abilities.Ability{abilities.WIS},
					DC:        saves.DCStatic(spellSaveDC),
					// HALF, and this is the spell half was built for. "On a
					// successful save, the creature takes half as much damage
					// and doesn't have to move away." One gate, two different
					// consequences on either side of it — which is only
					// legible because this cast delivers no condition.
					OnSuccess:  saves.Half,
					Recurrence: saves.RecurrenceNone,
				},
				Damage: []damage.Damage{{Dice: DissonantWhispersDamage, Type: damage.Psychic}},
				// THE FLEE, and it is the opposite lean from Thunderwave's
				// push in every field that one left at zero. The target is not
				// thrown: it turns and runs, so it pays its own reaction for
				// the movement and everyone whose reach it leaves gets a
				// swing. The budget is its own legs, which content cannot
				// know — the same whisper moves a dwarf and a horse different
				// distances. Where "away" actually lands is read off the map
				// by the layer that owns it.
				Move: &actions.CastMove{
					Policy:   actions.MoveAway,
					Speed:    true,
					Pays:     actions.PaysReaction,
					Provokes: true,
				},
			}
		},
	},
	Command: {
		casting: combat.SpellCasting{Level: 1, Time: combat.SpellCastingAction},
		name:    "Command",
		cost:    slotCost(resources.SpellSlotLevel1),
		build: func(spellSaveDC int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet:  CommandRangeFeet,
				Target:     actions.CastTargetOneCreature,
				MinTargets: 1,
				MaxTargets: 1,
				Save: &saves.SaveGate{
					Abilities: []abilities.Ability{abilities.WIS},
					DC:        saves.DCStatic(spellSaveDC),
					// NEGATED, and there is no other word for it. A made save
					// hears the command and ignores it; half of a compulsion
					// is not a shorter walk, it is nothing anybody can write
					// down.
					OnSuccess:  saves.Negated,
					Recurrence: saves.RecurrenceNone,
				},
				// THE MENU, and this spell is the reason the field exists. The
				// caster picks the word as they cast, the way Thunderwave's
				// caster picks a cell, and the id travels to the condition
				// under OptionKey below.
				//
				// Three words in the first slice, chosen because they are the
				// three different mechanisms: Approach walks toward, Flee
				// walks away, Grovel imposes a condition and stops.
				//
				// HALT is not here and is a ONE-ROW ADDITION whenever it is
				// wanted: it is the word with no route and no effect, so it
				// proves nothing the other three do not, and shipping it now
				// would be filler.
				//
				// DROP waits on three things in order, and the first of them
				// is not a spell problem: a monster that visibly HOLDS a
				// weapon, a holdable weapon prop delivered to its hand, and a
				// drop primitive distinct from unequip that puts an item on
				// the floor rather than into a bag. On every creature in the
				// sandbox today Drop would be Halt with extra words.
				//
				// UNDEAD are a DIVERGENCE, ruled rather than overlooked. The
				// letter exempts them; no creature type exists on a monster
				// definition, and every tomb monster is undead, so honouring
				// the letter would make this spell unwalkable. It works on
				// anything with a Wisdom save until goblins arrive and the
				// clause has something to spare.
				//
				// LANGUAGE is DEFERRED for a simpler reason: no language
				// exists anywhere in this stack, so "if it doesn't understand
				// your language" has nothing to read. No shelf is carved,
				// because nothing else wants one.
				Options: []actions.CastOption{
					{ID: CommandWordApproach, Label: "Approach"},
					{ID: CommandWordFlee, Label: "Flee"},
					{ID: CommandWordGrovel, Label: "Grovel"},
				},
				Effects: []actions.CastEffect{{
					Recipient: actions.CastRecipientTarget,
					Ref:       *refs.Conditions.Commanded(),
					// The clock is the only parameter content can fill. The
					// other two are bindings: the caster arrives under
					// CounterpartKey and the word under OptionKey, neither of
					// which a spell can know when it is written.
					Parameters:     commandedParameters,
					CounterpartKey: CommandCasterParameter,
					OptionKey:      CommandWordParameter,
				}},
			}
		},
	},
	SacredFlame: {
		casting: combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction},
		name:    "Sacred Flame",
		cost:    cantripCost(),
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
	BladeWard: {
		casting: combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction},
		name:    "Blade Ward",
		cost:    cantripCost(),
		build: func(_ int) actions.CastProfile {
			return actions.CastProfile{
				RangeFeet: BladeWardRangeFeet,
				Target:    actions.CastTargetSelf,
				// MinTargets and MaxTargets stay ZERO, which Validate requires
				// of a self-targeted profile: they bound what the CALLER may
				// name, and Blade Ward lets the caller name nobody. Who
				// receives the ward is a different question, and resolution
				// answers it with the caster.
				//
				// NO GATE, and NOT concentration. Nobody resists a ward traced
				// in front of yourself, and RAW's Blade Ward needs no
				// concentration -- which is not a detail. Declaring it would
				// drop whatever the bard was already holding, and would let the
				// first weapon hit the ward exists to soften provoke a
				// concentration check that could strip it: the spell cancelled
				// by the damage it resists.
				Effects: []actions.CastEffect{{
					Recipient: actions.CastRecipientCaster,
					Ref:       *refs.Conditions.BladeWard(),
					// NO CounterpartKey. A self-targeted cast has no other
					// party to bind, and CastEffect.validate refuses one.
					Parameters: bladeWardParameters,
				}},
			}
		},
	},
	TrueStrike: {
		casting: combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction},
		name:    "True Strike",
		cost:    cantripCost(),
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
		casting: combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction},
		name:    "Vicious Mockery",
		cost:    cantripCost(),
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
	HealingModifiers []healing.Modifier
	Spell            Spell
	SpellSaveDC      int
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
	casting := content.casting
	profile.Casting = &casting
	if profile.Healing != nil {
		profile.Healing.Modifiers = input.HealingModifiers
		declaration := profile.Healing.Clone()
		profile.Healing = &declaration
	}
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
