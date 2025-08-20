// Package conditions provides ref builders and constants for D&D 5e conditions.
package conditions

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a condition reference from an index
func Ref(index string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "condition",
		Value:  index,
	})
}

// Standard D&D 5e conditions as ref constants
var (
	// Standard conditions from Player's Handbook
	BlindedRef       = Ref("blinded")
	CharmedRef       = Ref("charmed")
	DeafenedRef      = Ref("deafened")
	ExhaustionRef    = Ref("exhaustion")
	FrightenedRef    = Ref("frightened")
	GrappledRef      = Ref("grappled")
	IncapacitatedRef = Ref("incapacitated")
	InvisibleRef     = Ref("invisible")
	ParalyzedRef     = Ref("paralyzed")
	PetrifiedRef     = Ref("petrified")
	PoisonedRef      = Ref("poisoned")
	ProneRef         = Ref("prone")
	RestrainedRef    = Ref("restrained")
	StunnedRef       = Ref("stunned")
	UnconsciousRef   = Ref("unconscious")

	// Feature-based conditions
	RagingRef        = Ref("raging")
	BlessedRef       = Ref("blessed")
	ConcentratingRef = Ref("concentrating")
	DodgingRef       = Ref("dodging")
	HidingRef        = Ref("hiding")
	
	// Spell effect conditions
	HasteRef     = Ref("hasted")
	SlowRef      = Ref("slowed")
	EnlargeRef   = Ref("enlarged")
	ReduceRef    = Ref("reduced")
	
	// Custom/temporary conditions
	SecondWindUsedRef  = Ref("second_wind_used")
	ActionSurgeUsedRef = Ref("action_surge_used")
)