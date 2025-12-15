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
