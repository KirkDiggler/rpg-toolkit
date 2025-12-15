//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Condition singletons - unexported for controlled access via methods
var (
	// Class-based conditions
	conditionRaging            = &core.Ref{Module: Module, Type: TypeConditions, ID: "raging"}
	conditionBrutalCritical    = &core.Ref{Module: Module, Type: TypeConditions, ID: "brutal_critical"}
	conditionUnarmoredDefense  = &core.Ref{Module: Module, Type: TypeConditions, ID: "unarmored_defense"}
	conditionFightingStyle     = &core.Ref{Module: Module, Type: TypeConditions, ID: "fighting_style"}
	conditionImprovedCritical  = &core.Ref{Module: Module, Type: TypeConditions, ID: "improved_critical"}
	conditionMartialArts       = &core.Ref{Module: Module, Type: TypeConditions, ID: "martial_arts"}
	conditionUnarmoredMovement = &core.Ref{Module: Module, Type: TypeConditions, ID: "unarmored_movement"}

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
func (n conditionsNS) UnarmoredDefense() *core.Ref  { return conditionUnarmoredDefense }
func (n conditionsNS) FightingStyle() *core.Ref     { return conditionFightingStyle }
func (n conditionsNS) ImprovedCritical() *core.Ref  { return conditionImprovedCritical }
func (n conditionsNS) MartialArts() *core.Ref       { return conditionMartialArts }
func (n conditionsNS) UnarmoredMovement() *core.Ref { return conditionUnarmoredMovement }

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
