//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Feature singletons - unexported for controlled access via methods
var (
	// Barbarian
	featureRage           = &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
	featureBrutalCritical = &core.Ref{Module: Module, Type: TypeFeatures, ID: "brutal_critical"}

	// Fighter
	featureSecondWind  = &core.Ref{Module: Module, Type: TypeFeatures, ID: "second_wind"}
	featureActionSurge = &core.Ref{Module: Module, Type: TypeFeatures, ID: "action_surge"}

	// Monk
	featureFlurryOfBlows   = &core.Ref{Module: Module, Type: TypeFeatures, ID: "flurry_of_blows"}
	featurePatientDefense  = &core.Ref{Module: Module, Type: TypeFeatures, ID: "patient_defense"}
	featureStepOfTheWind   = &core.Ref{Module: Module, Type: TypeFeatures, ID: "step_of_the_wind"}
	featureDeflectMissiles = &core.Ref{Module: Module, Type: TypeFeatures, ID: "deflect_missiles"}

	// Rogue
	featureSneakAttack = &core.Ref{Module: Module, Type: TypeFeatures, ID: "sneak_attack"}

	// Paladin
	featureDivineSmite = &core.Ref{Module: Module, Type: TypeFeatures, ID: "divine_smite"}
)

// Features provides type-safe, discoverable references to D&D 5e features.
// Use IDE autocomplete: refs.Features.<tab> to discover available features.
// Methods return singleton pointers enabling identity comparison (ref == refs.Features.Rage()).
var Features = featuresNS{}

type featuresNS struct{}

// Barbarian
func (n featuresNS) Rage() *core.Ref           { return featureRage }
func (n featuresNS) BrutalCritical() *core.Ref { return featureBrutalCritical }

// Fighter
func (n featuresNS) SecondWind() *core.Ref  { return featureSecondWind }
func (n featuresNS) ActionSurge() *core.Ref { return featureActionSurge }

// Monk
func (n featuresNS) FlurryOfBlows() *core.Ref   { return featureFlurryOfBlows }
func (n featuresNS) PatientDefense() *core.Ref  { return featurePatientDefense }
func (n featuresNS) StepOfTheWind() *core.Ref   { return featureStepOfTheWind }
func (n featuresNS) DeflectMissiles() *core.Ref { return featureDeflectMissiles }

// Rogue
func (n featuresNS) SneakAttack() *core.Ref { return featureSneakAttack }

// Paladin
func (n featuresNS) DivineSmite() *core.Ref { return featureDivineSmite }
