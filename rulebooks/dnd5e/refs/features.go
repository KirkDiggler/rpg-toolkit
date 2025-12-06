package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Features provides type-safe, discoverable references to D&D 5e features.
// Use IDE autocomplete: refs.Features.<tab> to discover available features.
var Features = featuresNS{}

type featuresNS struct{}

// Rage returns a reference to the Barbarian's Rage feature.
func (featuresNS) Rage() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
}

// SecondWind returns a reference to the Fighter's Second Wind feature.
func (featuresNS) SecondWind() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFeatures, ID: "second_wind"}
}

// ActionSurge returns a reference to the Fighter's Action Surge feature.
func (featuresNS) ActionSurge() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFeatures, ID: "action_surge"}
}

// FlurryOfBlows returns a reference to the Monk's Flurry of Blows feature.
func (featuresNS) FlurryOfBlows() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFeatures, ID: "flurry_of_blows"}
}

// PatientDefense returns a reference to the Monk's Patient Defense feature.
func (featuresNS) PatientDefense() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeFeatures, ID: "patient_defense"}
}
