// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ObservedContextOutput is detached context from one member's current knowledge.
// Members excludes the observer; Pairs ranges over the observer plus Members.
// This is an observed universe, never a claim that no unseen participant exists.
// It contains no target sheet and no answer about rule applicability. The
// conditions a member holds appear only as that member's sight testimony —
// what the observer saw at the moment of sighting (rpg-project#520 R16).
type ObservedContextOutput struct {
	// Observer identifies whose knowledge this answer describes.
	Observer MemberID
	// Position is the observer's own canonical placement.
	Position spatial.Position
	// Members contains other members currently sighted, sorted by ID.
	Members []ObservedContextMember
	// Pairs contains distinct ordered pairs, sorted by From then To.
	Pairs []ObservedContextPair
}

// ObservedContextMember is one current sight snapshot, not a live subject read.
// Missing observed facts remain nil; Down does not imply any other life-state or
// condition fact, Equipment describes observed hands, not an inventory, and
// Conditions is what was seen held at sight time, not the member's sheet now.
type ObservedContextMember struct {
	// ID names the sighted member.
	ID MemberID
	// Position is where this observer sees the member.
	Position spatial.Position
	// Down is nil when standing was not observed; false is observed standing.
	Down *bool
	// Equipment is nil when hands were not observed, not observed empty hands.
	Equipment *HeldEquipment
	// Conditions is nil when conditions were not observed; a non-nil empty set
	// is observed holding none. Entries keep the reported order. A detached
	// copy: it aliases neither the encounter nor the member's sheet.
	Conditions *ConditionSet
}

// ObservedContextPair carries spatial and relationship facts for two members
// in the bounded observed universe. Rules decide what these facts mean.
type ObservedContextPair struct {
	// From and To are distinct members, each the observer or a current sighting.
	From MemberID
	To   MemberID
	// DistanceCells is measured between the projected positions in grid cells.
	DistanceCells float64
	// Stance is always known: hostile, neutral or allied, or [StanceNone]
	// when either member is in no faction — a known "no side", never neutral
	// and never unknown (rpg-project#520, R5). Both members are always
	// members of this encounter (the observer is checked and every sighting
	// is validated), so the only absence left is a member with no side. It
	// follows encounter's believed-stance policy for the original observer.
	// The zero value "" is never produced.
	Stance Stance
}

// ObservedContext returns the member's own placement, current sight snapshots,
// and pair facts over exactly those subjects. It neither refreshes perception nor
// consults live Sight, Equipment, Conditions or Participation capabilities.
// Other members' positions and optional observed facts, conditions included,
// come only from their sight testimony.
// Relationships use the same owner as BelievedStance, with the original observer
// retained even when the pair names two other members.
//
// Memories and other channels are excluded. Returns ErrNilInput, ErrNotMember,
// placement errors for the observer, or ErrInvalidData for invalid current sight
// testimony, always with nil output on error. Returned values do not alias the
// encounter. Reading this context does not spend, roll, publish, or mutate state.
func (e *Encounter) ObservedContext(in *ViewInput) (*ObservedContextOutput, error) {
	holdings, err := e.storedView(in)
	if err != nil {
		return nil, fmt.Errorf("observed context: %w", err)
	}
	own, err := e.placementOf(e.members[in.Member])
	if err != nil {
		return nil, fmt.Errorf("observed context: %w", err)
	}
	out := &ObservedContextOutput{
		Observer: in.Member, Position: own.Position,
		Members: make([]ObservedContextMember, 0),
		Pairs:   make([]ObservedContextPair, 0),
	}
	// Sort the detached holding list before validation too, so an invalid read
	// names the same observed subject regardless of map iteration order.
	sort.Slice(holdings, func(i, j int) bool { return holdings[i].Subject < holdings[j].Subject })
	positions := map[MemberID]spatial.Position{in.Member: own.Position}
	for _, holding := range holdings {
		if holding.Subject == in.Member || holding.Channel != perception.Sight || !holding.CurrentOn(perception.Sight) {
			continue
		}
		seen, valid := DecodeSightTestimony(holding.Payload)
		if !valid || seen.State != LocationKnown {
			return nil, fmt.Errorf("observed context for %q: invalid current sight testimony: %w", holding.Subject, ErrInvalidData)
		}
		if _, exists := e.members[holding.Subject]; !exists {
			return nil, fmt.Errorf("observed context for %q: current subject is not a member: %w", holding.Subject, ErrInvalidData)
		}
		// Decoding allocates the optional facts from the observer's bytes; none
		// of these pointers points into a live participant or shared testimony.
		out.Members = append(out.Members, ObservedContextMember{
			ID: holding.Subject, Position: seen.Position, Down: seen.Down, Equipment: seen.Equipment,
			Conditions: seen.Conditions,
		})
		positions[holding.Subject] = seen.Position
	}

	ids := make([]MemberID, 0, len(positions))
	for id := range positions {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, from := range ids {
		for _, to := range ids {
			if from == to {
				continue
			}
			pair := ObservedContextPair{From: from, To: to, DistanceCells: e.Distance(positions[from], positions[to])}
			// Both ids are members (positions admits only the observer and
			// validated sightings), so a false here is StanceBetween's other
			// case: a member in no faction, which is a known no side.
			pair.Stance = StanceNone
			if stance, known := e.believedStanceBetween(in.Member, from, to); known {
				pair.Stance = stance
			}
			out.Pairs = append(out.Pairs, pair)
		}
	}
	return out, nil
}
