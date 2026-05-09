package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// Prompt-verb sentinel errors. Wrap with fmt.Errorf for context; the
// orchestrator inspects via errors.Is and maps to gRPC status codes.
var (
	// ErrDoorNotLocked is returned by AttemptUnlock when the target door
	// is not locked. Maps to gRPC FailedPrecondition.
	ErrDoorNotLocked = errors.New("door is not locked")
	// ErrPromptAlreadyPending is returned by AttemptUnlock when the player
	// already has an unresolved prompt (one prompt at a time per player).
	// Maps to gRPC FailedPrecondition.
	ErrPromptAlreadyPending = errors.New("player already has a pending prompt")
	// ErrNoPendingPrompt is returned by SubmitCheck when there is no
	// outstanding prompt for the player. Maps to gRPC FailedPrecondition.
	ErrNoPendingPrompt = errors.New("no pending prompt for player")
	// ErrInvalidRoll is returned by SubmitCheck when the supplied roll is
	// outside the d20 range [1,20]. Maps to gRPC InvalidArgument.
	ErrInvalidRoll = errors.New("roll out of range")
	// ErrUnsupportedPromptAction is returned by SubmitCheck when the
	// pending prompt's TriggeredAction is not one this wave knows how
	// to dispatch. Maps to gRPC Unimplemented.
	ErrUnsupportedPromptAction = errors.New("triggered action not supported")
	// ErrNoCharacterResolver is returned by SubmitCheck when the encounter
	// was constructed without a CharacterResolver — modifier resolution is
	// impossible without one. Maps to gRPC FailedPrecondition.
	ErrNoCharacterResolver = errors.New("encounter has no character resolver")
)

// PendingPromptKind discriminates the kinds of in-flight player prompts
// the encounter can surface. Wave 2.9 wires PromptKindSkillCheck only;
// dialogue / target-select land in later waves.
type PendingPromptKind int

const (
	// PromptKindUnspecified is the zero value and is never set deliberately.
	// Reading it back from data indicates a malformed prompt.
	PromptKindUnspecified PendingPromptKind = iota
	// PromptKindSkillCheck asks the player for an ability check (e.g. DEX
	// vs DC 15 with thieves' tools). Resolved via SubmitCheck.
	PromptKindSkillCheck
	// PromptKindDialogue asks the player to pick a dialogue option. Wave
	// 2.10+ — defined here so the resolver path is forward-compatible.
	PromptKindDialogue
	// PromptKindTargetSelect asks the player to pick an entity target.
	// Wave 2.10+.
	PromptKindTargetSelect
)

// PendingPrompt is the persisted shape of an in-flight player prompt.
//
// Invariant: at most one PendingPrompt per player on Data.PendingPrompts.
// Cleared on resolution (success or failure) and on encounter teardown.
//
// DC / Ability / Tool are only meaningful for PromptKindSkillCheck;
// they remain empty for other prompt kinds and are omitted from JSON.
//
// TriggeredBy identifies the entity that issued the prompt (door, trap,
// NPC); TriggeredAction names the side-effect to apply on success
// (Wave 2.9 only wires "open").
type PendingPrompt struct {
	Kind            PendingPromptKind `json:"kind"`
	DC              int               `json:"dc,omitempty"`
	Ability         string            `json:"ability,omitempty"`
	Tool            string            `json:"tool,omitempty"`
	TriggeredBy     core.EntityID     `json:"triggered_by"`
	TriggeredAction string            `json:"triggered_action"`
}

// CharacterResolver provides the ability and tool-proficiency modifiers
// the encounter needs to total a SubmitCheck roll. Implementations live
// outside the toolkit (rpg-api wires it against the character store; tests
// supply a stub) so the encounter package stays ruleset-agnostic.
//
// AbilityModifier returns the modifier for the named ability ("DEX",
// "STR", ...). ToolProficiencyBonus returns the proficiency bonus to add
// when the player is proficient with the given tool ref (empty tool means
// no tool bonus). Both return ok=false to signal the player or modifier
// is unknown; SubmitCheck treats unknowns as zero rather than erroring,
// since a missing proficiency is normal play (you may attempt a check
// without proficiency).
type CharacterResolver interface {
	AbilityModifier(playerID core.PlayerID, ability string) (mod int, ok bool)
	ToolProficiencyBonus(playerID core.PlayerID, tool string) (bonus int, ok bool)
}

// PromptIssued is the verb-return shape AttemptUnlock hands back to the
// orchestrator so the rpg-api translator can build the wire-shape
// InputRequired{skill_check} message. It mirrors the fields the
// translator needs without exposing the persisted PendingPrompt struct.
type PromptIssued struct {
	Kind            PendingPromptKind
	DC              int
	Ability         string
	Tool            string
	TriggeredBy     core.EntityID
	TriggeredAction string
}

// SubmitCheckResult is the verb-return shape SubmitCheck hands back to
// the orchestrator. Total is the resolved (roll + ability + tool) sum
// and Success is total >= prompt DC.
type SubmitCheckResult struct {
	Success bool
	Total   int
}

// AttemptUnlock issues a skill-check prompt for a locked door. The
// player picks up the prompt via the next snapshot / event stream and
// must resolve it via SubmitCheck before issuing other verbs that would
// race the same player's input slot.
//
// Errors:
//   - door not in encounter: wrapped fmt.Errorf
//   - player not in encounter: wrapped fmt.Errorf
//   - door not locked: ErrDoorNotLocked
//   - player already has a pending prompt: ErrPromptAlreadyPending
//
// Does not emit any event; the prompt rides the next SnapshotFor /
// orchestrator response, not the broker.
func (e *Encounter) AttemptUnlock(playerID core.PlayerID, doorID core.EntityID) (PromptIssued, error) {
	if _, ok := e.data.Players[playerID]; !ok {
		return PromptIssued{}, fmt.Errorf("player %q not in encounter", playerID)
	}
	door, ok := e.data.Doors[doorID]
	if !ok {
		return PromptIssued{}, fmt.Errorf("door %q not in encounter", doorID)
	}
	if !door.Locked {
		return PromptIssued{}, fmt.Errorf("%w: %q", ErrDoorNotLocked, doorID)
	}
	if _, pending := e.data.PendingPrompts[playerID]; pending {
		return PromptIssued{}, fmt.Errorf("%w: player %q", ErrPromptAlreadyPending, playerID)
	}

	prompt := &PendingPrompt{
		Kind:            PromptKindSkillCheck,
		DC:              door.LockDC,
		Ability:         door.LockAbility,
		Tool:            door.LockTool,
		TriggeredBy:     doorID,
		TriggeredAction: promptActionOpen,
	}
	e.data.PendingPrompts[playerID] = prompt

	return PromptIssued{
		Kind:            prompt.Kind,
		DC:              prompt.DC,
		Ability:         prompt.Ability,
		Tool:            prompt.Tool,
		TriggeredBy:     prompt.TriggeredBy,
		TriggeredAction: prompt.TriggeredAction,
	}, nil
}

// SubmitCheck resolves the player's pending skill-check prompt against
// the supplied raw d20 roll. The encounter computes the total via the
// configured CharacterResolver, compares against the prompt DC, and on
// success dispatches the prompt's TriggeredAction (Wave 2.9 only wires
// "open" → calls OpenDoor internally, which emits DoorOpenedEvent +
// HexRevealedEvent through the broker).
//
// The pending prompt is cleared regardless of outcome.
//
// Errors:
//   - no resolver wired: ErrNoCharacterResolver
//   - no prompt outstanding for player: ErrNoPendingPrompt
//   - roll outside [1,20]: ErrInvalidRoll
//   - prompt's TriggeredAction not "open": ErrUnsupportedPromptAction
//   - downstream OpenDoor failure: wrapped through fmt.Errorf
func (e *Encounter) SubmitCheck(playerID core.PlayerID, roll int) (SubmitCheckResult, error) {
	if e.resolver == nil {
		return SubmitCheckResult{}, ErrNoCharacterResolver
	}
	prompt, ok := e.data.PendingPrompts[playerID]
	if !ok {
		return SubmitCheckResult{}, fmt.Errorf("%w: player %q", ErrNoPendingPrompt, playerID)
	}
	if roll < 1 || roll > 20 {
		return SubmitCheckResult{}, fmt.Errorf("%w: roll=%d", ErrInvalidRoll, roll)
	}

	abilityMod, _ := e.resolver.AbilityModifier(playerID, prompt.Ability)
	toolBonus := 0
	if prompt.Tool != "" {
		toolBonus, _ = e.resolver.ToolProficiencyBonus(playerID, prompt.Tool)
	}
	total := roll + abilityMod + toolBonus
	success := total >= prompt.DC

	// Capture what we need before clearing the prompt — dispatch happens
	// after clearing so an OpenDoor failure doesn't strand a stale prompt.
	triggeredBy := prompt.TriggeredBy
	triggeredAction := prompt.TriggeredAction
	delete(e.data.PendingPrompts, playerID)

	if !success {
		return SubmitCheckResult{Success: false, Total: total}, nil
	}

	if err := e.dispatchPromptAction(playerID, triggeredBy, triggeredAction); err != nil {
		return SubmitCheckResult{Success: true, Total: total}, err
	}
	return SubmitCheckResult{Success: true, Total: total}, nil
}

// promptActionOpen is the only TriggeredAction wired in Wave 2.9. Future
// waves extend the dispatch table.
const promptActionOpen = "open"

// dispatchPromptAction is the side-effect dispatch called by SubmitCheck
// on a successful check. Wave 2.9 only wires "open" → OpenDoor; any
// other action returns ErrUnsupportedPromptAction.
//
// OpenDoor itself emits DoorOpenedEvent + HexRevealedEvent through the
// broker and clears door.Locked-vs-Open state per the existing verb.
// We additionally clear the door's Locked flag here so a downstream
// OpenDoor never re-encounters a locked-but-open door.
func (e *Encounter) dispatchPromptAction(playerID core.PlayerID, target core.EntityID, action string) error {
	switch action {
	case promptActionOpen:
		door, ok := e.data.Doors[target]
		if !ok {
			return fmt.Errorf("door %q not in encounter", target)
		}
		// Clear the lock flag before OpenDoor so the door round-trips as
		// unlocked-and-open (not locked-and-open) for any subsequent
		// snapshot or verb.
		door.Locked = false
		if err := e.OpenDoor(playerID, target); err != nil {
			return fmt.Errorf("dispatch open door: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedPromptAction, action)
	}
}
