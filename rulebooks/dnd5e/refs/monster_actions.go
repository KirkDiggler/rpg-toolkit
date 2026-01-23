//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// TypeMonsterActions is the type identifier for monster actions
const TypeMonsterActions core.Type = "monster_actions"

// Monster action singletons - unexported for controlled access via methods
var (
	// Goblin actions
	monsterActionScimitar              = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "scimitar"}
	monsterActionShortbow              = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "shortbow"}
	monsterActionNimbleEscapeDisengage = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "nimble_escape_disengage"}
	monsterActionNimbleEscapeHide      = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "nimble_escape_hide"}

	// Generic actions
	monsterActionMelee       = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "melee"}
	monsterActionRanged      = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "ranged"}
	monsterActionMultiattack = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "multiattack"}
	monsterActionBite        = &core.Ref{Module: Module, Type: TypeMonsterActions, ID: "bite"}
)

// MonsterActions provides type-safe, discoverable references to D&D 5e monster actions.
// Use IDE autocomplete: refs.MonsterActions.<tab> to discover available actions.
// Methods return singleton pointers enabling identity comparison.
var MonsterActions = monsterActionsNS{}

type monsterActionsNS struct{}

// Scimitar returns the ref for a scimitar attack
func (n monsterActionsNS) Scimitar() *core.Ref { return monsterActionScimitar }

// Shortbow returns the ref for a shortbow attack
func (n monsterActionsNS) Shortbow() *core.Ref { return monsterActionShortbow }

// NimbleEscapeDisengage returns the ref for the Nimble Escape (Disengage) action
func (n monsterActionsNS) NimbleEscapeDisengage() *core.Ref {
	return monsterActionNimbleEscapeDisengage
}

// NimbleEscapeHide returns the ref for the Nimble Escape (Hide) action
func (n monsterActionsNS) NimbleEscapeHide() *core.Ref { return monsterActionNimbleEscapeHide }

// Melee returns the ref for a generic melee attack
func (n monsterActionsNS) Melee() *core.Ref { return monsterActionMelee }

// Ranged returns the ref for a generic ranged attack
func (n monsterActionsNS) Ranged() *core.Ref { return monsterActionRanged }

// Multiattack returns the ref for a multiattack action
func (n monsterActionsNS) Multiattack() *core.Ref { return monsterActionMultiattack }

// Bite returns the ref for a bite attack with knockdown
func (n monsterActionsNS) Bite() *core.Ref { return monsterActionBite }
