package core

// EncounterMode is the high-level interaction mode for an encounter.
//
// FREE_ROAM: no initiative; players act in any order; the encounter is in
// exploration / pre-combat. TURN_BASED: initiative order is fixed; one
// active actor at a time; verbs that mutate combat state require it.
type EncounterMode int

// EncounterMode values.
const (
	// ModeUnspecified is the zero value; treat as ModeFreeRoam unless explicitly set.
	ModeUnspecified EncounterMode = iota
	// ModeFreeRoam is the exploration mode: no initiative, no active actor.
	ModeFreeRoam
	// ModeTurnBased is initiative-locked combat. Verbs gate on the active actor.
	ModeTurnBased
)

// String returns a stable label for the mode (for logs and JSON-friendly debug).
func (m EncounterMode) String() string {
	switch m {
	case ModeFreeRoam:
		return "FREE_ROAM"
	case ModeTurnBased:
		return "TURN_BASED"
	default:
		return "UNSPECIFIED"
	}
}
