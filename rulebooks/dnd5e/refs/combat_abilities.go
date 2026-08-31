//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// CombatAbility singletons - unexported for controlled access via methods
// These represent universal combat abilities available to all characters
var (
	combatAbilityAttack    = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "attack"}
	combatAbilityDash      = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "dash"}
	combatAbilityDodge     = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "dodge"}
	combatAbilityDisengage = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "disengage"}
	combatAbilityHelp      = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "help"}
	combatAbilityHide      = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "hide"}
	combatAbilityReady     = &core.Ref{Module: Module, Type: TypeCombatAbilities, ID: "ready"}
)

// CombatAbilities provides type-safe, discoverable references to D&D 5e combat abilities.
// These are universal actions available to all characters during combat (Attack, Dash, Dodge, etc).
// Use IDE autocomplete: refs.CombatAbilities.<tab> to discover available combat abilities.
// Methods return singleton pointers enabling identity comparison.
var CombatAbilities = combatAbilitiesNS{}

type combatAbilitiesNS struct{}

// Attack returns the ref for the Attack combat ability.
// Attack consumes 1 action to grant attack capacity based on Extra Attack.
func (n combatAbilitiesNS) Attack() *core.Ref { return combatAbilityAttack }

// Dash returns the ref for the Dash combat ability.
// Dash consumes 1 action to add character's speed to movement remaining.
func (n combatAbilitiesNS) Dash() *core.Ref { return combatAbilityDash }

// Dodge returns the ref for the Dodge combat ability.
// Dodge consumes 1 action to grant the Dodging condition.
func (n combatAbilitiesNS) Dodge() *core.Ref { return combatAbilityDodge }

// Disengage returns the ref for the Disengage combat ability.
// Disengage consumes 1 action to grant the Disengaging condition.
func (n combatAbilitiesNS) Disengage() *core.Ref { return combatAbilityDisengage }

// Help returns the ref for the Help combat ability.
// Help consumes 1 action to grant advantage to an ally's next attack or check.
func (n combatAbilitiesNS) Help() *core.Ref { return combatAbilityHelp }

// Hide returns the ref for the Hide combat ability.
// Hide consumes 1 action to attempt a stealth check.
func (n combatAbilitiesNS) Hide() *core.Ref { return combatAbilityHide }

// Ready returns the ref for the Ready combat ability.
// Ready consumes 1 action to prepare an action for a specified trigger.
func (n combatAbilitiesNS) Ready() *core.Ref { return combatAbilityReady }
