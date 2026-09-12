//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Feature singletons - unexported for controlled access via methods
var (
	featureDiscipleOfLife = &core.Ref{Module: Module, Type: TypeFeatures, ID: "disciple_of_life"}
	// Barbarian
	featureRage           = &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
	featureBrutalCritical = &core.Ref{Module: Module, Type: TypeFeatures, ID: "brutal_critical"}
	featureRecklessAttack = &core.Ref{Module: Module, Type: TypeFeatures, ID: "reckless_attack"}

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

	// Bard
	featureBardicInspiration = &core.Ref{Module: Module, Type: TypeFeatures, ID: "bardic_inspiration"}
)

// Features provides type-safe, discoverable references to D&D 5e features.
// Use IDE autocomplete: refs.Features.<tab> to discover available features.
// Methods return singleton pointers enabling identity comparison (ref == refs.Features.Rage()).
var Features = featuresNS{}

type featuresNS struct{}

// DiscipleOfLife identifies the Life Domain's leveled-spell healing bonus.
func (n featuresNS) DiscipleOfLife() *core.Ref { return featureDiscipleOfLife }

// Barbarian
func (n featuresNS) Rage() *core.Ref           { return featureRage }
func (n featuresNS) BrutalCritical() *core.Ref { return featureBrutalCritical }
func (n featuresNS) RecklessAttack() *core.Ref { return featureRecklessAttack }

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

// Bard

// BardicInspiration returns the ref for the bard's level-1 feature: a bonus
// action that hands an ally a die they spend on a roll of their own.
func (n featuresNS) BardicInspiration() *core.Ref { return featureBardicInspiration }
