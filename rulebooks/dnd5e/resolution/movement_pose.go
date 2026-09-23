package resolution

import (
	"encoding/json"
	"fmt"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const frozenMovementKind = "movement.attack_reaction"

type frozenMovement struct {
	Kind        string                              `json:"kind"`
	Version     int                                 `json:"version"`
	Outcome     MovementOutcome                     `json:"outcome"`
	Folded      *dndEvents.MovementChainEvent       `json:"folded"`
	Triggers    []dndEvents.ReactionTriggerEvent    `json:"triggers"`
	Definitions map[string]combatActions.Definition `json:"definitions"`
	Index       int                                 `json:"index"`
	Inner       []byte                              `json:"inner"`
}
type storedReactionAttacks map[string]combatActions.Definition

func (a storedReactionAttacks) AttackFor(id string) (combatActions.Definition, bool) {
	d, ok := a[id]
	return d, ok
}
func (m *movementMachine) freezeMovement(index int, definition combatActions.Definition, pose Pose) (Step, error) {
	definitions := storedReactionAttacks{m.triggers[index].ReactorID: definition}
	for _, trigger := range m.triggers[index+1:] {
		if d, ok := m.in.Reactions.AttackFor(trigger.ReactorID); ok {
			definitions[trigger.ReactorID] = d
		}
	}
	outcome := m.outcome()
	raw, err := json.Marshal(frozenMovement{Kind: frozenMovementKind, Version: 1, Outcome: outcome, Folded: m.folded, Triggers: m.triggers, Definitions: definitions, Index: index, Inner: pose.Frozen})
	if err != nil {
		return nil, err
	}
	return Pose{Ask: pose.Ask, Frozen: raw, BeforeRoll: pose.BeforeRoll, Movement: &outcome}, nil
}

// NewMovementResumed continues the opportunity attacks on a frozen step without
// announcing the step again or repeating attacks which already resolved.
func NewMovementResumed(in *StrikeResumeInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	var f frozenMovement
	if err := json.Unmarshal(in.Frozen, &f); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if f.Kind != frozenMovementKind || f.Version != 1 || f.Outcome.Mover == "" || f.Folded == nil || f.Index < 0 || f.Index >= len(f.Triggers) {
		return nil, fmt.Errorf("%w: invalid movement continuation", ErrBadFrozen)
	}
	nested := *in
	nested.Frozen = f.Inner
	resumed, err := NewStrikeResumed(&nested)
	if err != nil {
		return nil, err
	}
	return &movementMachine{in: &MovementInput{Mover: encounter.MemberID(f.Outcome.Mover), From: f.Outcome.From, To: f.Outcome.To, Reactions: storedReactionAttacks(f.Definitions), Roller: in.Roller}, folded: f.Folded, triggers: f.Triggers, reactions: f.Outcome.Reactions, resumed: resumed, resumeIndex: f.Index}, nil
}
