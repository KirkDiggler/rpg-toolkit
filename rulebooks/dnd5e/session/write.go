// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// JoinInput places a member into a session's encounter.
type JoinInput struct {
	// Session is the session to join.
	Session string

	// Member is the joining member's identifier.
	Member string

	// Kind categorises the member.
	Kind MemberKind

	// Room is the room to place them in.
	Room string

	// Position is where within that room.
	Position spatial.Position
}

// JoinOutput reports the join and what it revealed.
type JoinOutput struct {
	// Member is the joined member's placement.
	Member Member

	// Discovered is what changed in each observer's perception, keyed by
	// observer. Absent observers saw nothing new.
	Discovered map[string]Discovery

	// Seq is the story sequence of the recorded join.
	Seq uint64

	// Outcome is present if an ending fired on the join.
	Outcome *Outcome

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// ExitInput removes a member from a session's encounter.
type ExitInput struct {
	// Session is the session to leave.
	Session string

	// Member is the departing member.
	Member string
}

// ExitOutput reports the departure and what the member took with them.
type ExitOutput struct {
	// Outcome is the member's final placement.
	Outcome MemberOutcome

	// Carry is what the member knew on the way out — the knowledge that
	// leaves with them rather than staying with the encounter.
	Carry []Sighting

	// Seq is the story sequence of the recorded exit.
	Seq uint64

	// Closed is present if the encounter auto-closed because the last member
	// left.
	Closed *Outcome

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// EndInput closes a session's encounter through a declared external ending.
type EndInput struct {
	// Session is the session to close.
	Session string

	// Ending is the key of the declared external ending to fire.
	Ending string
}

// EndOutput reports how the encounter closed.
type EndOutput struct {
	// Outcome is the final state: which ending fired and where everyone stood.
	Outcome Outcome

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// Join places a member into the session's encounter and reports what came into
// view as a result.
//
// A monster may be joined, but its decider is not supplied here: deciders are
// never persisted and are re-registered at load, so behaviour arrives with the
// wave that brings entities. A monster joined today is placed and perceived
// correctly; it simply does not act on its own.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrClosed if the encounter has already ended, or
// ErrSaveFailed with a populated report.
func (m *Manager) Join(ctx context.Context, in *JoinInput) (*JoinOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("join: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("join: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	joined, err := scope.enc.Join(&encounter.JoinInput{
		Member: encounter.MemberInput{
			ID:       encounter.MemberID(in.Member),
			Kind:     encounter.MemberKind(in.Kind),
			Room:     in.Room,
			Position: in.Position,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("join: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	return &JoinOutput{
		Member:     projectMember(joined.Member),
		Discovered: projectDiscoveries(joined.IntelDeltas),
		Seq:        joined.Seq,
		Outcome:    projectOutcome(joined.Outcome),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// Exit removes a member from the session's encounter, returning what they knew
// on the way out.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, ErrClosed, or ErrSaveFailed with a populated
// report.
func (m *Manager) Exit(ctx context.Context, in *ExitInput) (*ExitOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("exit: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("exit: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	left, err := scope.enc.Exit(&encounter.ExitInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("exit: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	return &ExitOutput{
		Outcome:  projectMemberOutcome(left.Outcome),
		Carry:    projectSightings(left.Carry),
		Seq:      left.Seq,
		Closed:   projectOutcome(left.Closed),
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// End closes the session's encounter through a declared external ending.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, ErrNoEncounter,
// ErrNoEnding if the key names no declared external ending, ErrClosed if the
// encounter has already ended, or ErrSaveFailed with a populated report.
func (m *Manager) End(ctx context.Context, in *EndInput) (*EndOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("end: %w", ErrNilInput)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("end: %w", err)
	}

	ended, err := scope.enc.End(&encounter.EndInput{Ending: in.Ending})
	if err != nil {
		return nil, fmt.Errorf("end: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("end: %w", err)
	}

	outcome := projectOutcome(&ended.Outcome)
	return &EndOutput{Outcome: *outcome, Saved: report, Delivery: delivery}, nil
}

// openForWrite loads a session's encounter and also returns the encounter's ID,
// which a write verb needs in order to save it back.
//
// A read verb has no use for the ID, so it uses open instead. Splitting them
// keeps a read from carrying a value it cannot act on, and keeps the "which ID
// do I save under" question answered in exactly one place.
func (m *Manager) openForWrite(ctx context.Context, sessionID string) (*writeScope, error) {
	if sessionID == "" {
		return nil, ErrNoSessionID
	}
	data, err := m.loadSessionData(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	enc, baseline, err := m.loadWorldWithBaseline(ctx, data.Encounter)
	if err != nil {
		return nil, err
	}
	return &writeScope{
		session:   sessionID,
		encounter: data.Encounter,
		enc:       enc,
		baseline:  baseline,
	}, nil
}

// writeScope is everything a write verb needs to act, save, and fan out: the
// live encounter, the IDs to save and address it under, and the sequence
// boundary separating what was already recorded from what this verb records.
type writeScope struct {
	session   string
	encounter string
	enc       *encounter.Encounter
	baseline  uint64
}

// persist writes the mutated encounter back and reports the result.
//
// Only the encounter changes on these verbs: a join, an exit, and an ending all
// mutate the world, while SessionData holds only an ID and the encounter it
// points at, neither of which any of them touches. So the report names one
// aggregate, and a partial save is not reachable here — the multi-write case
// lives in StartSession today and will return when entities do.
func (m *Manager) persist(ctx context.Context, scope *writeScope) (SaveReport, *encounter.EncounterData, error) {
	data := scope.enc.ToData()
	if err := m.encounters.SaveEncounter(ctx, scope.encounter, &data); err != nil {
		return SaveReport{Failed: []string{"encounter:" + scope.encounter}}, nil,
			fmt.Errorf("saving world: %w: %w", ErrSaveFailed, err)
	}
	return SaveReport{Written: []string{"encounter:" + scope.encounter}}, &data, nil
}

// commit saves the mutated world and then fans out what it recorded.
//
// The order is the law (S9): publish only after the save lands. Announcing a
// fact that failed to persist is the one mistake with no recovery — a client
// told the ogre died, a world in which it did not, and no sequence gap to
// betray the difference.
func (m *Manager) commit(ctx context.Context, scope *writeScope) (SaveReport, DeliveryReport, error) {
	report, snapshot, err := m.persist(ctx, scope)
	if err != nil {
		return report, DeliveryReport{}, err
	}
	return report, m.publish(ctx, scope, snapshot), nil
}
