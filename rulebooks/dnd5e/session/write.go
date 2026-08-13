// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// JoinInput places a member into a session's encounter.
type JoinInput struct {
	// Session is the session to join.
	Session string

	// Member is the joining player's character ID, which is also the ID they
	// are known by inside the encounter.
	//
	// There is no ref here, and its absence is the point. A ref names the
	// package that can load some data; no toolkit package can load a player
	// character, because the host owns it and only the host's repository can
	// produce it. A "dnd5e:characters:..." ref would claim otherwise.
	Member string

	// Room is the room to place them in.
	Room string

	// Position is where within that room.
	Position spatial.Position
}

// JoinOutput reports the join and what it revealed.
type JoinOutput struct {
	// Member is the joined member's placement.
	Member Member

	// Character is the joined character's state, derived after loading.
	//
	// Always populated: Join is players only, and a player with no loadable
	// character does not join at all.
	Character *CharacterState

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

// SpawnInput instantiates content that lives in code as a new member.
type SpawnInput struct {
	// Session is the session to spawn into.
	Session string

	// ID is the ID the new member is known by inside the encounter.
	//
	// Separate from Ref because a template carries no identity: one catalog
	// entry makes five skeletons, and the encounter has to tell them apart.
	ID string

	// Ref names what to build — "dnd5e:monsters:skeleton".
	//
	// It routes on (Module, Type), which is what a ref is for: it says which
	// package can produce this data. A ref this build has no loader for is
	// rejected rather than guessed at.
	Ref string

	// Room is the room to place it in.
	Room string

	// Position is where within that room.
	Position spatial.Position
}

// SpawnOutput reports the spawn and what it revealed.
type SpawnOutput struct {
	// Member is the new member's placement.
	Member Member

	// NPC is the instantiated sheet's state.
	NPC *MonsterState

	// Discovered is what changed in each observer's perception, keyed by
	// observer. Absent observers saw nothing new.
	Discovered map[string]Discovery

	// Seq is the story sequence of the recorded arrival.
	Seq uint64

	// Outcome is present if an ending fired on the spawn.
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

// Join brings a PLAYER into the session's encounter and reports what came into
// view as a result.
//
// Players only. Content that lives in code — monsters — enters through Spawn,
// and the split is by where the data comes from rather than by what the thing
// is: a character is loaded from the host's repository, a monster is built from
// a ref. That distinction survives contact with the future, where "player or
// monster" does not — a durable NPC would be a monster you load.
//
// The character is loaded BEFORE the placement, which is the point of loading
// at join at all: a session that accepted a player with no character would look
// healthy until the first verb that needed a sheet, and would then fail
// somewhere with no visible connection to the join that caused it.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoCharacter, ErrBadCharacter, ErrMemberExists, ErrClosed
// if the encounter has already ended, or ErrSaveFailed with a populated report.
func (m *Manager) Join(ctx context.Context, in *JoinInput) (*JoinOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("join: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("join: %w", ErrNoMemberID)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	ch, err := m.loadCharacter(ctx, newCallBus(), in.Member)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	placed, err := place(scope, in.Member, KindPlayer, in.Room, in.Position)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	return &JoinOutput{
		Member:     projectMember(placed.Member),
		Character:  projectCharacter(ch),
		Discovered: projectDiscoveries(placed.IntelDeltas),
		Seq:        placed.Seq,
		Outcome:    projectOutcome(placed.Outcome),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// Spawn instantiates content that lives in code and places it as a new member.
//
// The ref names what to build — "dnd5e:monsters:skeleton" — and the ID names
// the member it becomes. They are separate because a template cannot carry
// identity: one catalog entry makes five skeletons, and each needs its own name
// in the encounter.
//
// The resulting sheet is stored in the session rather than behind a repository,
// because a spawned monster is session-scoped: it has no existence outside this
// fight and nothing durable to look up.
//
// A decider is not supplied here. Deciders are never persisted and are
// re-registered at load, so behaviour arrives with the wave that brings it. A
// monster spawned today is placed, perceived, and remembered correctly; it
// simply does not act on its own.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoRef, ErrBadRef,
// ErrNoLoader, ErrUnknownContent, ErrNoSession, ErrNoEncounter,
// ErrMemberExists, ErrClosed, or ErrSaveFailed with a populated report.
func (m *Manager) Spawn(ctx context.Context, in *SpawnInput) (*SpawnOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("spawn: %w", ErrNilInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("spawn: %w", ErrNoMemberID)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	// Built before the placement, for the same reason Join loads before it: a
	// session holding a member with no sheet is a problem discovered later, in
	// a place that does not name the call that caused it.
	sheet, err := instantiate(in.ID, in.Ref)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	placed, err := place(scope, in.ID, KindMonster, in.Room, in.Position)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	scope.data.NPCs = append(scope.data.NPCs, *sheet)
	scope.touched = true

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	return &SpawnOutput{
		Member:     projectMember(placed.Member),
		NPC:        projectMonster(sheet),
		Discovered: projectDiscoveries(placed.IntelDeltas),
		Seq:        placed.Seq,
		Outcome:    projectOutcome(placed.Outcome),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// place puts a member into the encounter.
//
// ONE placement path, shared by both entry verbs. Join and Spawn differ in
// where a sheet comes from and in nothing else — the same validation, the same
// adjacency rules, the same perception refresh, the same story beat. Two copies
// of this would be free to drift, and the drift would be invisible until a rule
// added to one silently failed to apply to the other.
//
// The composition already settled this argument in the anchoring wave, where
// Setup and Load were made to share one validator so that a single mutation
// kills the pins on both. Same reasoning, one layer up.
func place(
	scope *writeScope, id string, kind MemberKind, room string, at spatial.Position,
) (*encounter.JoinOutput, error) {
	placed, err := scope.enc.Join(&encounter.JoinInput{
		Member: encounter.MemberInput{
			ID:       encounter.MemberID(id),
			Kind:     encounter.MemberKind(kind),
			Room:     room,
			Position: at,
		},
	})
	if err != nil {
		return nil, translate(err)
	}
	return placed, nil
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

	scope, err := m.openForChange(ctx, in.Session)
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

	scope, err := m.openForChange(ctx, in.Session)
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
	ledger, err := interrupt.LoadLedger(data.Windows)
	if err != nil {
		// Reject, never crash. A stored ledger that LoadLedger refuses is a blob
		// no version of this module wrote, so the honest answer is that the
		// session record is unreadable — not to repair it into something
		// plausible and resume a resolution that never happened.
		return nil, fmt.Errorf("session %q: %w: %w", sessionID, ErrInvalidSession, err)
	}
	enc, baseline, err := m.loadWorldWithBaseline(ctx, data.Encounter)
	if err != nil {
		return nil, err
	}
	return &writeScope{
		session:   sessionID,
		encounter: data.Encounter,
		data:      data,
		enc:       enc,
		ledger:    ledger,
		baseline:  baseline,
	}, nil
}

// openForChange is openForWrite plus the freeze: a verb that would change the
// world is refused while a window is open.
//
// The split is deliberate and structural. Answer must reach a frozen session —
// it is the only thing that can unfreeze it — so the check cannot live inside
// openForWrite. Putting it in a differently named opener means a new verb picks
// its policy by picking its opener, and forgetting the freeze requires choosing
// the one Answer uses rather than merely omitting a line.
func (m *Manager) openForChange(ctx context.Context, sessionID string) (*writeScope, error) {
	scope, err := m.openForWrite(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := scope.frozen(); err != nil {
		return nil, err
	}
	return scope, nil
}

// frozen reports the oldest open window as an error, or nil if the world is
// running.
//
// The oldest rather than an arbitrary one: windows are ordered by pose, that
// order is persisted, and a caller who is told which window blocks them should
// be told the same one every time they ask.
func (s *writeScope) frozen() error {
	open, err := s.ledger.Open()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSession, err)
	}
	if len(open) == 0 {
		return nil
	}
	w := open[0]
	return &FrozenError{
		Window:   strconv.FormatUint(uint64(w.ID), 10),
		Audience: string(w.Audience),
	}
}

// writeScope is everything a write verb needs to act, save, and fan out: the
// live encounter, the session record and its window ledger, the IDs to save and
// address them under, and the sequence boundary separating what was already
// recorded from what this verb records.
type writeScope struct {
	session   string
	encounter string
	data      *SessionData
	enc       *encounter.Encounter
	ledger    *interrupt.Ledger
	baseline  uint64

	// touched marks the ledger as changed by this verb, so a walk that opened
	// or closed a window writes the session and one that did not leaves it
	// alone. Writes stay proportional to what actually changed: the common
	// case is a walk that suspends nothing and touches one aggregate, exactly
	// as it did before windows existed.
	touched bool
}

// persist writes the mutated aggregates back and reports the result.
//
// The encounter is written first and the session second, and the order is a
// correctness decision rather than a style one. Both orders can fail halfway;
// they fail differently:
//
//   - Encounter lands, session does not: the world holds the steps that were
//     taken and no window remembers the pause. The walk is stuck, but every
//     persisted fact is true, and the caller is told the save failed.
//   - Session lands, encounter does not: a window says "resume from step three"
//     over a world that still has the walker at step zero. Resuming would skip
//     three cells nobody walked, silently.
//
// The first is a stoppage; the second is corruption that looks like progress.
// So the aggregate that records what happened goes first, and the one that
// records what is owed goes second. Resume re-validates against the world it
// actually loads, which turns even the bad half of the first case into a clean
// rejection rather than a wrong walk.
func (m *Manager) persist(ctx context.Context, scope *writeScope) (SaveReport, *encounter.EncounterData, error) {
	data := scope.enc.ToData()
	if err := m.encounters.SaveEncounter(ctx, scope.encounter, &data); err != nil {
		report := SaveReport{Failed: []string{"encounter:" + scope.encounter}}
		return report, nil, &SaveError{
			Report: report,
			Err:    fmt.Errorf("saving world: %w", err),
		}
	}
	report := SaveReport{Written: []string{"encounter:" + scope.encounter}}

	if !scope.touched {
		return report, &data, nil
	}

	scope.data.Windows = scope.ledger.ToData()
	if err := m.sessions.SaveSession(ctx, scope.data); err != nil {
		// The encounter is already durable, so the report names both what landed
		// and what did not (S6). A bare error here would leave the caller unable
		// to tell a total failure from a half one, which is the difference
		// between "retry the verb" and "the world moved but the window is gone".
		report.Failed = append(report.Failed, "session:"+scope.session)
		return report, nil, &SaveError{
			Report: report,
			Err:    fmt.Errorf("saving session: %w", err),
		}
	}
	report.Written = append(report.Written, "session:"+scope.session)
	return report, &data, nil
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
