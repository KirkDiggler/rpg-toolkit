//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// FightingStyles provides type-safe, discoverable references to D&D 5e fighting styles.
// Use IDE autocomplete: refs.FightingStyles.<tab> to discover available fighting styles.
var FightingStyles = fightingStylesNS{ns{TypeFightingStyles}}

type fightingStylesNS struct{ ns }

func (n fightingStylesNS) Archery() *core.Ref             { return n.ref("archery") }
func (n fightingStylesNS) Defense() *core.Ref             { return n.ref("defense") }
func (n fightingStylesNS) Dueling() *core.Ref             { return n.ref("dueling") }
func (n fightingStylesNS) GreatWeaponFighting() *core.Ref { return n.ref("great_weapon_fighting") }
func (n fightingStylesNS) Protection() *core.Ref          { return n.ref("protection") }
func (n fightingStylesNS) TwoWeaponFighting() *core.Ref   { return n.ref("two_weapon_fighting") }
