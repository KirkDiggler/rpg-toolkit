//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Action singletons - unexported for controlled access via methods
// These represent combat actions that consume capacity (attacks, movement, etc.)
var (
	actionMove          = &core.Ref{Module: Module, Type: TypeActions, ID: "move"}
	actionStrike        = &core.Ref{Module: Module, Type: TypeActions, ID: "strike"}
	actionOffHandStrike = &core.Ref{Module: Module, Type: TypeActions, ID: "off_hand_strike"}
	actionFlurryStrike  = &core.Ref{Module: Module, Type: TypeActions, ID: "flurry_strike"}
	actionUnarmedStrike = &core.Ref{Module: Module, Type: TypeActions, ID: "unarmed_strike"}
)

// Actions provides type-safe, discoverable references to D&D 5e combat actions.
// These are the "doing" actions that consume capacity granted by abilities/features.
// Use IDE autocomplete: refs.Actions.<tab> to discover available actions.
// Methods return singleton pointers enabling identity comparison.
var Actions = actionsNS{}

type actionsNS struct{}

// Move returns the ref for the Move action.
// Move consumes movement capacity to change position on the battlefield.
func (n actionsNS) Move() *core.Ref { return actionMove }

// Strike returns the ref for the Strike action.
// Strike consumes an attack from AttacksRemaining to make a weapon attack.
func (n actionsNS) Strike() *core.Ref { return actionStrike }

// OffHandStrike returns the ref for the OffHandStrike action.
// OffHandStrike is granted by two-weapon fighting and consumes a bonus action.
func (n actionsNS) OffHandStrike() *core.Ref { return actionOffHandStrike }

// FlurryStrike returns the ref for the FlurryStrike action.
// FlurryStrike is granted by Flurry of Blows and consumes FlurryStrikesRemaining.
func (n actionsNS) FlurryStrike() *core.Ref { return actionFlurryStrike }

// UnarmedStrike returns the ref for the UnarmedStrike action.
// UnarmedStrike is an attack made without a weapon.
func (n actionsNS) UnarmedStrike() *core.Ref { return actionUnarmedStrike }
