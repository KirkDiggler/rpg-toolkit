//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// FightingStyle singletons - unexported for controlled access via methods
var (
	fightingStyleArchery             = &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "archery"}
	fightingStyleDefense             = &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "defense"}
	fightingStyleDueling             = &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "dueling"}
	fightingStyleGreatWeaponFighting = &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "great_weapon_fighting"}
	fightingStyleProtection          = &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "protection"}
	fightingStyleTwoWeaponFighting   = &core.Ref{Module: Module, Type: TypeFightingStyles, ID: "two_weapon_fighting"}
)

// FightingStyles provides type-safe, discoverable references to D&D 5e fighting styles.
// Use IDE autocomplete: refs.FightingStyles.<tab> to discover available fighting styles.
// Methods return singleton pointers enabling identity comparison (ref == refs.FightingStyles.Dueling()).
var FightingStyles = fightingStylesNS{}

type fightingStylesNS struct{}

func (n fightingStylesNS) Archery() *core.Ref             { return fightingStyleArchery }
func (n fightingStylesNS) Defense() *core.Ref             { return fightingStyleDefense }
func (n fightingStylesNS) Dueling() *core.Ref             { return fightingStyleDueling }
func (n fightingStylesNS) GreatWeaponFighting() *core.Ref { return fightingStyleGreatWeaponFighting }
func (n fightingStylesNS) Protection() *core.Ref          { return fightingStyleProtection }
func (n fightingStylesNS) TwoWeaponFighting() *core.Ref   { return fightingStyleTwoWeaponFighting }
