//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Condition singletons - unexported for controlled access via methods
var (
	// Class-based conditions
	conditionRaging            = &core.Ref{Module: Module, Type: TypeConditions, ID: "raging"}
	conditionBrutalCritical    = &core.Ref{Module: Module, Type: TypeConditions, ID: "brutal_critical"}
	conditionRecklessAttack    = &core.Ref{Module: Module, Type: TypeConditions, ID: "reckless_attack"}
	conditionUnarmoredDefense  = &core.Ref{Module: Module, Type: TypeConditions, ID: "unarmored_defense"}
	conditionImprovedCritical  = &core.Ref{Module: Module, Type: TypeConditions, ID: "improved_critical"}
	conditionMartialArts       = &core.Ref{Module: Module, Type: TypeConditions, ID: "martial_arts"}
	conditionUnarmoredMovement = &core.Ref{Module: Module, Type: TypeConditions, ID: "unarmored_movement"}
	conditionSneakAttack       = &core.Ref{Module: Module, Type: TypeConditions, ID: "sneak_attack"}

	// Fighting style conditions
	conditionFightingStyleArchery = &core.Ref{
		Module: Module, Type: TypeConditions, ID: "fighting_style_archery",
	}
	conditionFightingStyleDefense = &core.Ref{
		Module: Module, Type: TypeConditions, ID: "fighting_style_defense",
	}
	conditionFightingStyleDueling = &core.Ref{
		Module: Module, Type: TypeConditions, ID: "fighting_style_dueling",
	}
	conditionFightingStyleGreatWeaponFighting = &core.Ref{
		Module: Module, Type: TypeConditions, ID: "fighting_style_great_weapon_fighting",
	}
	conditionFightingStyleProtection = &core.Ref{
		Module: Module, Type: TypeConditions, ID: "fighting_style_protection",
	}
	conditionFightingStyleTwoWeaponFighting = &core.Ref{
		Module: Module, Type: TypeConditions, ID: "fighting_style_two_weapon_fighting",
	}

	// Turn-based conditions (from actions, last until start of next turn)
	conditionDodging     = &core.Ref{Module: Module, Type: TypeConditions, ID: "dodging"}
	conditionDisengaging = &core.Ref{Module: Module, Type: TypeConditions, ID: "disengaging"}

	// Combat-ability conditions (Beat 2: Help + Hide)
	conditionHidden = &core.Ref{Module: Module, Type: TypeConditions, ID: "hidden"}
	conditionHelped = &core.Ref{Module: Module, Type: TypeConditions, ID: "helped"}

	// Bard (rpg-project#397): the die an ally holds until they spend it.
	conditionInspired = &core.Ref{Module: Module, Type: TypeConditions, ID: "inspired"}

	// Bard cantrips (rpg-project#405): what the two cast cantrips deliver.
	conditionTrueStrike     = &core.Ref{Module: Module, Type: TypeConditions, ID: "true_strike"}
	conditionViciousMockery = &core.Ref{Module: Module, Type: TypeConditions, ID: "vicious_mockery"}
	conditionBaned          = &core.Ref{Module: Module, Type: TypeConditions, ID: "baned"}
	conditionBlessed        = &core.Ref{Module: Module, Type: TypeConditions, ID: "blessed"}
	conditionBladeWard      = &core.Ref{Module: Module, Type: TypeConditions, ID: "blade_ward"}

	// Command (rpg-project ideas/spells/command): the compulsion the spell
	// leaves on a creature that failed its save, holding the word it was
	// given and the caster the word is measured from.
	conditionCommanded = &core.Ref{Module: Module, Type: TypeConditions, ID: "commanded"}

	// Concentration (rpg-project#407): the owner on the caster's sheet that
	// holds what its spell left behind.
	conditionConcentrating = &core.Ref{Module: Module, Type: TypeConditions, ID: "concentrating"}

	// Guided (docs/ideas/cleric): the d4 Guidance leaves on the touched
	// creature, spendable on one later ability check.
	conditionGuided = &core.Ref{Module: Module, Type: TypeConditions, ID: "guided"}

	// Resistance (docs/ideas/cleric): the d4 Resistance leaves on the touched
	// creature, spendable on one later saving throw.
	conditionResistance = &core.Ref{Module: Module, Type: TypeConditions, ID: "resistance"}

	// Sanctuary (docs/ideas/cleric): the ward on the protected creature.
	// Checked directly by resolution at attack/cast declaration time rather
	// than offered or subscribed around — see [conditionsNS.Sanctuary].
	conditionSanctuary = &core.Ref{Module: Module, Type: TypeConditions, ID: "sanctuary"}

	// SanctuaryImmune (docs/ideas/cleric): the short immunity a creature earns
	// against one specific caster's Sanctuary after succeeding its ward save —
	// RAW's 24 hours, simplified to end at combat end or a rest rather than
	// tracked in real time (this rulebook has no real-time clock at all; see
	// [conditionsNS.SanctuaryImmune]).
	conditionSanctuaryImmune = &core.Ref{Module: Module, Type: TypeConditions, ID: "sanctuary_immune"}

	// Reaction conditions (Wave 2.11d) — universal-by-default reactions that
	// subscribe to the appropriate chain and publish ReactionTriggerEvents
	// when their predicate matches AND gamectx.IsReactionReady returns true.
	conditionOpportunityAttack = &core.Ref{Module: Module, Type: TypeConditions, ID: "opportunity_attack"}

	// Standard D&D 5e Conditions
	conditionBlinded       = &core.Ref{Module: Module, Type: TypeConditions, ID: "blinded"}
	conditionCharmed       = &core.Ref{Module: Module, Type: TypeConditions, ID: "charmed"}
	conditionDeafened      = &core.Ref{Module: Module, Type: TypeConditions, ID: "deafened"}
	conditionFrightened    = &core.Ref{Module: Module, Type: TypeConditions, ID: "frightened"}
	conditionGrappled      = &core.Ref{Module: Module, Type: TypeConditions, ID: "grappled"}
	conditionIncapacitated = &core.Ref{Module: Module, Type: TypeConditions, ID: "incapacitated"}
	conditionInvisible     = &core.Ref{Module: Module, Type: TypeConditions, ID: "invisible"}
	conditionParalyzed     = &core.Ref{Module: Module, Type: TypeConditions, ID: "paralyzed"}
	conditionPetrified     = &core.Ref{Module: Module, Type: TypeConditions, ID: "petrified"}
	conditionPoisoned      = &core.Ref{Module: Module, Type: TypeConditions, ID: "poisoned"}
	conditionProne         = &core.Ref{Module: Module, Type: TypeConditions, ID: "prone"}
	conditionRestrained    = &core.Ref{Module: Module, Type: TypeConditions, ID: "restrained"}
	conditionStunned       = &core.Ref{Module: Module, Type: TypeConditions, ID: "stunned"}
	conditionUnconscious   = &core.Ref{Module: Module, Type: TypeConditions, ID: "unconscious"}
	conditionExhaustion    = &core.Ref{Module: Module, Type: TypeConditions, ID: "exhaustion"}
)

// Conditions provides type-safe, discoverable references to D&D 5e conditions.
// Use IDE autocomplete: refs.Conditions.<tab> to discover available conditions.
// Methods return singleton pointers enabling identity comparison (ref == refs.Conditions.Raging()).
var Conditions = conditionsNS{}

type conditionsNS struct{}

// Class-based conditions
func (n conditionsNS) Raging() *core.Ref            { return conditionRaging }
func (n conditionsNS) BrutalCritical() *core.Ref    { return conditionBrutalCritical }
func (n conditionsNS) RecklessAttack() *core.Ref    { return conditionRecklessAttack }
func (n conditionsNS) UnarmoredDefense() *core.Ref  { return conditionUnarmoredDefense }
func (n conditionsNS) ImprovedCritical() *core.Ref  { return conditionImprovedCritical }
func (n conditionsNS) MartialArts() *core.Ref       { return conditionMartialArts }
func (n conditionsNS) UnarmoredMovement() *core.Ref { return conditionUnarmoredMovement }
func (n conditionsNS) SneakAttack() *core.Ref       { return conditionSneakAttack }

// Fighting style conditions
func (n conditionsNS) FightingStyleArchery() *core.Ref { return conditionFightingStyleArchery }
func (n conditionsNS) FightingStyleDefense() *core.Ref { return conditionFightingStyleDefense }
func (n conditionsNS) FightingStyleDueling() *core.Ref { return conditionFightingStyleDueling }
func (n conditionsNS) FightingStyleGreatWeaponFighting() *core.Ref {
	return conditionFightingStyleGreatWeaponFighting
}
func (n conditionsNS) FightingStyleProtection() *core.Ref { return conditionFightingStyleProtection }
func (n conditionsNS) FightingStyleTwoWeaponFighting() *core.Ref {
	return conditionFightingStyleTwoWeaponFighting
}

// Turn-based conditions (from actions)
func (n conditionsNS) Dodging() *core.Ref     { return conditionDodging }
func (n conditionsNS) Disengaging() *core.Ref { return conditionDisengaging }

// Hidden returns the ref for the HiddenCondition, applied on a successful
// Hide stealth check.
func (n conditionsNS) Hidden() *core.Ref { return conditionHidden }

// Helped returns the ref for the HelpedCondition, applied to the ally
// targeted by the Help combat ability.
func (n conditionsNS) Helped() *core.Ref { return conditionHelped }

// OpportunityAttack returns the ref for the OpportunityAttackCondition
// applied by default to every melee combatant. The condition subscribes to
// MovementChain and publishes a ReactionTriggerEvent when an enemy leaves
// the holder's threatened reach AND the holder has the OA reaction readied.
func (n conditionsNS) OpportunityAttack() *core.Ref { return conditionOpportunityAttack }

// Standard D&D 5e Conditions
func (n conditionsNS) Blinded() *core.Ref       { return conditionBlinded }
func (n conditionsNS) Charmed() *core.Ref       { return conditionCharmed }
func (n conditionsNS) Deafened() *core.Ref      { return conditionDeafened }
func (n conditionsNS) Frightened() *core.Ref    { return conditionFrightened }
func (n conditionsNS) Grappled() *core.Ref      { return conditionGrappled }
func (n conditionsNS) Incapacitated() *core.Ref { return conditionIncapacitated }
func (n conditionsNS) Invisible() *core.Ref     { return conditionInvisible }
func (n conditionsNS) Paralyzed() *core.Ref     { return conditionParalyzed }
func (n conditionsNS) Petrified() *core.Ref     { return conditionPetrified }
func (n conditionsNS) Poisoned() *core.Ref      { return conditionPoisoned }
func (n conditionsNS) Prone() *core.Ref         { return conditionProne }
func (n conditionsNS) Restrained() *core.Ref    { return conditionRestrained }
func (n conditionsNS) Stunned() *core.Ref       { return conditionStunned }
func (n conditionsNS) Unconscious() *core.Ref   { return conditionUnconscious }
func (n conditionsNS) Exhaustion() *core.Ref    { return conditionExhaustion }

// Inspired returns the ref for the InspiredCondition, applied to the ally a
// bard grants a Bardic Inspiration die to.
func (n conditionsNS) Inspired() *core.Ref { return conditionInspired }

// TrueStrike returns the ref for the TrueStrikeCondition, applied to the
// CASTER of True Strike and keyed to the creature it was pointed at.
func (n conditionsNS) TrueStrike() *core.Ref { return conditionTrueStrike }

// ViciousMockery returns the ref for the ViciousMockeryCondition, applied to
// the creature that failed its save against Vicious Mockery.
func (n conditionsNS) ViciousMockery() *core.Ref { return conditionViciousMockery }

// Baned returns the ref for the source-qualified penalty imposed by Bane.
func (n conditionsNS) Baned() *core.Ref { return conditionBaned }

// Blessed returns the ref for the source-qualified bonus imposed by Bless.
func (n conditionsNS) Blessed() *core.Ref { return conditionBlessed }

// BladeWard returns the ref for the BladeWardCondition, applied to the caster
// by the Blade Ward cantrip and halving incoming weapon damage.
func (n conditionsNS) BladeWard() *core.Ref { return conditionBladeWard }

// Commanded returns the ref for the CommandedCondition, applied to the creature
// that failed its save against Command and driving its next turn.
func (n conditionsNS) Commanded() *core.Ref { return conditionCommanded }

// Concentrating returns the ref for the ConcentratingCondition, applied to the
// CASTER of a concentration spell and holding the addresses of the effects
// that spell left on the board.
func (n conditionsNS) Concentrating() *core.Ref { return conditionConcentrating }

// Guided returns the ref for the GuidedCondition, applied to the creature
// Guidance touches and holding the d4 it can spend on one later ability check.
func (n conditionsNS) Guided() *core.Ref { return conditionGuided }

// Resistance returns the ref for the ResistanceCondition, applied to the
// creature Resistance touches and holding the d4 it can spend on one later
// saving throw.
func (n conditionsNS) Resistance() *core.Ref { return conditionResistance }

// Sanctuary returns the ref for the SanctuaryCondition, applied to the
// creature Sanctuary wards. It offers nothing and modifies no roll of its
// own holder's — resolution reads its presence directly at the moment
// another creature targets the ward with an attack or a harmful spell.
func (n conditionsNS) Sanctuary() *core.Ref { return conditionSanctuary }

// SanctuaryImmune returns the ref for the SanctuaryImmuneCondition, applied
// to an ATTACKER who succeeded a ward save — named so its provenance is
// obvious wherever it shows up, not a generic immunity flag. Source-qualified
// per caster: it blocks only that same caster's future Sanctuary wards.
func (n conditionsNS) SanctuaryImmune() *core.Ref { return conditionSanctuaryImmune }

var conditionGuidingBolt = &core.Ref{Module: Module, Type: TypeConditions, ID: "guiding_bolt"}

// GuidingBolt returns the target-held light consumed by the next attack roll.
func (n conditionsNS) GuidingBolt() *core.Ref { return conditionGuidingBolt }

var conditionShieldOfFaith = &core.Ref{Module: Module, Type: TypeConditions, ID: "shield_of_faith"}

// ShieldOfFaith returns the recipient's concentration-owned AC protection.
func (n conditionsNS) ShieldOfFaith() *core.Ref { return conditionShieldOfFaith }

var conditionDivineFavor = &core.Ref{Module: Module, Type: TypeConditions, ID: "divine_favor"}

// DivineFavor returns the caster's concentration-owned radiant weapon bonus.
func (n conditionsNS) DivineFavor() *core.Ref { return conditionDivineFavor }
