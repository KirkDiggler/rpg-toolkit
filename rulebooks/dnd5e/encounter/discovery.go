// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DiscoveryCheckResolver is the supplied automatic-check capability. Older
// hosts implementing only CheckResolver retain their explicit-search contract;
// the game host supplies this capability and retires its Search action.
// Implementations prepare character data before the composition acts.
type DiscoveryCheckResolver interface {
	ResolveDiscoveryCheck(*ResolveCheckInput) (*ResolveCheckOutput, error)
}

// BeatDiscoveryChecked is an automatic roll without the hidden subject's identity.
const BeatDiscoveryChecked = "discovery_checked"

// DiscoveryAttemptData is the run's consumed allowance and re-arm state.
type DiscoveryAttemptData struct {
	Used  int  `json:"used"`
	Armed bool `json:"armed"`
}

// DiscoveryStateData holds a character's private preference and run attempts.
// Learned knowledge remains in the world's per-observer journal.
type DiscoveryStateData struct {
	Private  bool                                   `json:"private,omitempty"`
	Attempts map[ConcealmentID]DiscoveryAttemptData `json:"attempts,omitempty"`
}

// DiscoveryMemoryData is carry-forward testimony about one authored check.
type DiscoveryMemoryData struct {
	Used    int  `json:"used"`
	Learned bool `json:"learned,omitempty"`
}

// DiscoveryMemoryInput selects one member's retained check memory.
type DiscoveryMemoryInput struct{ Member MemberID }

// DiscoveryMemoryOutput contains only character-lifetime entries in this field.
type DiscoveryMemoryOutput struct {
	Checks map[ConcealmentID]DiscoveryMemoryData
}

// DiscoveryMemory returns a detached projection, never an additional authority.
func (e *Encounter) DiscoveryMemory(in *DiscoveryMemoryInput) (*DiscoveryMemoryOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, ErrNotMember
	}
	out := &DiscoveryMemoryOutput{Checks: map[ConcealmentID]DiscoveryMemoryData{}}
	for _, id := range e.world.concealments {
		c := e.field.concealmentOf(id)
		if c.attempts.Lifetime != DiscoveryLifetimeCharacter {
			continue
		}
		state := e.discovery[in.Member]
		used := state.Attempts[id].Used
		learned := e.world.knowsConcealment(in.Member, id)
		if used > 0 || learned {
			out.Checks[id] = DiscoveryMemoryData{Used: used, Learned: learned}
		}
	}
	return out, nil
}

// SetDiscoverySharingInput changes future result audiences for a placed player.
type SetDiscoverySharingInput struct {
	Member  MemberID
	Sharing bool
}

// SetDiscoverySharingOutput reports the effective preference.
type SetDiscoverySharingOutput struct{ Sharing bool }

// SetDiscoverySharing changes no past audience or learned fact and spends no time.
func (e *Encounter) SetDiscoverySharing(in *SetDiscoverySharingInput) (*SetDiscoverySharingOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	member, ok := e.members[in.Member]
	if !ok {
		return nil, ErrNotMember
	}
	if member.Kind != KindPlayer {
		return nil, fmt.Errorf("discovery sharing requires a player: %w", ErrNotMember)
	}
	if e.outcome != nil {
		return nil, ErrClosed
	}
	if e.discovery == nil {
		e.discovery = map[MemberID]DiscoveryStateData{}
	}
	state := e.discovery[in.Member]
	state.Private = !in.Sharing
	e.discovery[in.Member] = state
	return &SetDiscoverySharingOutput{Sharing: in.Sharing}, nil
}

// DiscoverySharing returns a member's effective preference without mutating it.
func (e *Encounter) DiscoverySharing(in *DiscoveryMemoryInput) (*SetDiscoverySharingOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, ErrNotMember
	}
	return &SetDiscoverySharingOutput{Sharing: !e.discovery[in.Member].Private}, nil
}

func (e *Encounter) initializeDiscovery(member MemberID, private bool, memory map[ConcealmentID]DiscoveryMemoryData) error {
	if e.discovery == nil {
		e.discovery = map[MemberID]DiscoveryStateData{}
	}
	state, exists := e.discovery[member]
	if !exists {
		state = DiscoveryStateData{Private: private, Attempts: map[ConcealmentID]DiscoveryAttemptData{}}
	}
	if state.Attempts == nil {
		state.Attempts = map[ConcealmentID]DiscoveryAttemptData{}
	}
	for _, id := range e.world.concealments {
		remembered, ok := memory[id]
		if !ok {
			continue
		}
		if remembered.Used < 0 {
			return fmt.Errorf("negative discovery count: %w", ErrInvalidData)
		}
		c := e.field.concealmentOf(id)
		if c.attempts.Lifetime != DiscoveryLifetimeCharacter {
			continue
		}
		current := state.Attempts[id]
		if remembered.Used > current.Used {
			current.Used = remembered.Used
		}
		// A new visit starts a fresh approach, not a fresh retained allowance.
		if !exists {
			current.Armed = true
		}
		if current.Used > 0 {
			state.Attempts[id] = current
		}
		if remembered.Learned && !e.world.knowsConcealment(member, id) {
			if err := e.world.learnConcealment(member, id, "retained discovery"); err != nil {
				return err
			}
		}
	}
	if state.Private || len(state.Attempts) > 0 {
		e.discovery[member] = state
	}
	return nil
}

func (e *Encounter) discoveryAudience(actor MemberID) []MemberID {
	if e.discovery[actor].Private {
		return []MemberID{actor}
	}
	var audience []MemberID
	for _, id := range e.rosterIDs() {
		if e.members[id].Kind == KindPlayer {
			if _, placed := e.canvas.GetEntityPosition(string(id)); placed {
				audience = append(audience, id)
			}
		}
	}
	return audience
}

func (e *Encounter) discoveryDistance(c *concealment, at spatial.Position) float64 {
	distance := math.Inf(1)
	visit := func(cell spatial.Position) { distance = math.Min(distance, e.Distance(at, cell)) }
	for _, cell := range e.hiddenCellsOf(c) {
		visit(cell)
	}
	for _, cell := range e.memberPropCells(c) {
		visit(cell)
	}
	for _, id := range c.doors {
		d := e.doorsByID[id]
		if d == nil {
			continue
		}
		for _, edge := range d.edges {
			visit(edge.From)
			visit(edge.To)
		}
	}
	return distance
}

func (e *Encounter) sweepDiscoveryChecks() error {
	resolver, enabled := e.checkResolver.(DiscoveryCheckResolver)
	if !enabled {
		return nil
	}
	if e.outcome != nil || len(e.world.concealments) == 0 {
		return nil
	}
	at := uint64(e.clock.ToData().HighWater)
	for _, member := range e.rosterIDs() {
		if e.members[member].Kind != KindPlayer {
			continue
		}
		position, placed := e.canvas.GetEntityPosition(string(member))
		if !placed {
			continue
		}
		if err := e.initializeDiscovery(member, false, nil); err != nil {
			return err
		}
		state := e.discovery[member]
		if state.Attempts == nil {
			state.Attempts = map[ConcealmentID]DiscoveryAttemptData{}
		}
		for _, id := range e.world.concealments {
			c := e.field.concealmentOf(id)
			if e.world.knowsConcealment(member, id) {
				continue
			}
			attempt := state.Attempts[id]
			if attempt.Used >= c.attempts.MaxAttempts {
				continue
			}
			distance := e.discoveryDistance(c, position)
			if attempt.Used > 0 && distance >= float64(c.attempts.ResetHexes) {
				attempt.Armed = true
				state.Attempts[id] = attempt
			}
			if distance > discoveryReachHexes || (attempt.Used > 0 && !attempt.Armed) {
				continue
			}
			audience := e.discoveryAudience(member)
			verdict, err := resolver.ResolveDiscoveryCheck(&ResolveCheckInput{Member: member, Approaches: append([]CheckApproach(nil), c.checks...)})
			if err != nil {
				return fmt.Errorf("automatic discovery for %q: %w", member, err)
			}
			if verdict == nil {
				return fmt.Errorf("automatic discovery returned no verdict: %w", ErrBadConcealment)
			}
			attempt.Used++
			attempt.Armed = false
			state.Attempts[id] = attempt
			e.discovery[member] = state
			payload, err := json.Marshal(map[string]any{"beat": BeatDiscoveryChecked, "member": member, "ability": verdict.Applied.Ability, "beaten": verdict.Beaten, "total": verdict.Total, "calculation": verdict.Calculation})
			if err != nil {
				return err
			}
			if _, err = e.appendBeat(&record.AppendInput{Audience: e.audienceFor(tableBeat, audience...), At: at, Tags: map[string]string{"tag": "discovery"}, Payload: payload}); err != nil {
				return err
			}
			if verdict.Beaten {
				for _, recipient := range audience {
					if !e.world.knowsConcealment(recipient, id) {
						if err = e.revealConcealmentTo(recipient, c, "automatic discovery", at); err != nil {
							return err
						}
					}
				}
			}
		}
		if state.Private || len(state.Attempts) > 0 {
			e.discovery[member] = state
		}
	}
	return nil
}

func copyDiscoveryStates(in map[MemberID]DiscoveryStateData) map[MemberID]DiscoveryStateData {
	if len(in) == 0 {
		return nil
	}
	out := make(map[MemberID]DiscoveryStateData, len(in))
	for member, state := range in {
		copyState := DiscoveryStateData{Private: state.Private}
		if len(state.Attempts) > 0 {
			copyState.Attempts = map[ConcealmentID]DiscoveryAttemptData{}
			for id, attempt := range state.Attempts {
				copyState.Attempts[id] = attempt
			}
		}
		out[member] = copyState
	}
	return out
}

func (e *Encounter) validateDiscoveryStates() error {
	members := make([]MemberID, 0, len(e.discovery))
	for id := range e.discovery {
		members = append(members, id)
	}
	sort.Slice(members, func(i, j int) bool { return members[i] < members[j] })
	for _, member := range members {
		if !e.everMembers[member] {
			return fmt.Errorf("discovery state for unknown member %q: %w", member, ErrInvalidData)
		}
		for id, attempt := range e.discovery[member].Attempts {
			if e.field.concealmentOf(id) == nil || attempt.Used < 1 {
				return fmt.Errorf("invalid discovery attempt %q: %w", id, ErrInvalidData)
			}
		}
	}
	return nil
}
