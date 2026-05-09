package core

// EncounterMode is the high-level interaction mode for an encounter.
//
// FREE_ROAM: no initiative; players act in any order; the encounter is in
// exploration / pre-combat. TURN_BASED: initiative order is fixed; one
// active actor at a time; verbs that mutate combat state require it.
// ENDED: terminal state; combat verbs reject with ErrEncounterEnded.
type EncounterMode int

// EncounterMode values.
const (
	// ModeUnspecified is the zero value; treat as ModeFreeRoam unless explicitly set.
	ModeUnspecified EncounterMode = iota
	// ModeFreeRoam is the exploration mode: no initiative, no active actor.
	ModeFreeRoam
	// ModeTurnBased is initiative-locked combat. Verbs gate on the active actor.
	ModeTurnBased
	// ModeEnded is the terminal state for an encounter — entered when the
	// encounter-end predicate first goes true (Wave 2.10: all hostiles
	// defeated). Combat verbs (TakeAction, EndTurn, NPCAct) reject with
	// ErrEncounterEnded; the orchestrator stops dispatching turns. The
	// encounter persists in storage with this mode so reconnects see the
	// terminal state via snapshot replay.
	//
	// The transition to ModeEnded happens inside the kill chain
	// (Encounter.checkEncounterEnd, in death.go), not via SetMode.
	// SetMode does NOT accept ModeEnded as a target — terminal state is
	// always orchestrator-driven through gameplay (last hostile dies),
	// never through an explicit "set the mode to ended" call. The
	// transition clears Initiative / ActiveIdx / Round.
	ModeEnded
)

// String returns a stable label for the mode (for logs and JSON-friendly debug).
func (m EncounterMode) String() string {
	switch m {
	case ModeFreeRoam:
		return "FREE_ROAM"
	case ModeTurnBased:
		return "TURN_BASED"
	case ModeEnded:
		return "ENDED"
	default:
		return "UNSPECIFIED"
	}
}
