//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Features provides type-safe, discoverable references to D&D 5e features.
// Use IDE autocomplete: refs.Features.<tab> to discover available features.
var Features = featuresNS{ns{TypeFeatures}}

type featuresNS struct{ ns }

func (n featuresNS) Rage() *core.Ref        { return n.ref("rage") }
func (n featuresNS) SecondWind() *core.Ref  { return n.ref("second_wind") }
func (n featuresNS) ActionSurge() *core.Ref { return n.ref("action_surge") }
