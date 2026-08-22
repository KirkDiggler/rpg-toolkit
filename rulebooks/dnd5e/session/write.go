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

	// Member is the joining player's character ID, which is also the ID they
	// are known by inside the encounter.
	//
	// There is no ref here, and its absence is the point. A ref names the
	// package that can load some data; no toolkit package can load a player
	// character, because the host owns it and only the host's repository can
	// produce it. A "dnd5e:characters:..." ref would claim otherwise.
	Member string

	// Position is the cell to place them on, in dungeon-absolute space —
	// the same coordinates the Atlas and every other verb speak. Which room
	// owns that cell is worked out here; a caller places somebody on the map,
	// not in a chamber (rpg-project#227).
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

	// Formed is present if arriving put the newcomer in sight of the other
	// side and a fight started around them.
	Formed *Formed

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

	// Position is the cell to place it on, in dungeon-absolute space. See
	// JoinInput.Position.
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

	// Formed is present if the spawned content arrived in sight of the party
	// and a fight started. This is the reason Formed is not a movement-only
	// field: nobody walked anywhere, and a fight started.
	Formed *Formed

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
// ErrNoEncounter, ErrNoCharacter, ErrBadCharacter, ErrBadPosition if no room
// owns the cell they were placed on, ErrClosed if the encounter has already
// ended, or ErrSaveFailed with a populated report.
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

	ch, err := m.loadCharacter(ctx, newCallBus(), in.Member)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	placed, err := place(scope, in.Member, KindPlayer, ch.GetName(), in.Position)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	down, err := discoveryStanding(scope)
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
		Discovered: projectDiscoveries(placed.IntelDeltas, down),
		Seq:        placed.Seq,
		Outcome:    projectOutcome(placed.Outcome),
		Formed:     projectFormed(placed.Formed),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// discoveryStanding batches a down-check over the WHOLE roster this scope's
// encounter now holds, for projecting Discovery/Sighting's Seen.Standing
// (rpg-toolkit#1137): a discovery's first-contact reports can name anyone
// the observer just perceived, so the safe set to ask about is everyone,
// asked once per verb rather than once per report.
//
// Fetched BEFORE commit, deliberately: a failure here must leave nothing
// persisted (R5 atomicity), the same discipline every other pre-commit
// check in this file already keeps.
func discoveryStanding(scope *writeScope) (map[string]bool, error) {
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, translate(err)
	}
	return standingSet(scope.standing, rosterIDs(roster))
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
// ErrBadPosition if no room owns the cell, ErrClosed, or ErrSaveFailed with a
// populated report.
func (m *Manager) Spawn(ctx context.Context, in *SpawnInput) (*SpawnOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("spawn: %w", ErrNilInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("spawn: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	// Built before the placement — a preference, NOT a correctness argument,
	// and worth stating plainly because the correctness version is tempting
	// and wrong.
	//
	// Swapping these two survives as a mutant. Under load-act-save (S4) the
	// in-memory encounter is discarded whenever a verb returns before commit,
	// so a bad ref cannot leave a member placed no matter which order these
	// run in. The rejection table's "a rejected spawn stores nothing" passes
	// either way, and it is honest about why.
	//
	// This is the second time the same shape has appeared here — the join's
	// load-versus-placement ordering had it too — which is the general lesson
	// rather than a coincidence: in a load-act-save verb, ordering with respect
	// to PERSISTENCE is never load-bearing before the commit. What is
	// load-bearing is that the error stops the verb, and that is pinned
	// separately.
	//
	// The lesson holds where the ordering is about persistence ALONE. It stops
	// holding the moment something READS BACK what was written inside the same
	// verb — which is what the exception below is, and what Attack's is too.
	//
	// The order is still chosen: there is no reason to touch the world when
	// the call is already doomed.
	sheet, err := instantiate(in.ID, in.Ref)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	// THE SHEET IS RECORDED BEFORE THE PLACEMENT, and here the ordering IS
	// load-bearing — an exception to the paragraph above, which is why it is
	// stated separately rather than folded into it.
	//
	// Arriving refreshes sight, and every sight refresh asks who is standing
	// (rpg-toolkit#1079). That consult reads the session's own sheets, so a
	// monster placed before its sheet was recorded is asked about while the
	// record still has nothing to say — answered "standing" for the right
	// reason by accident, and paying a pointless character-repository miss on
	// the way past. A member the world can see is a member the world can read.
	//
	// Nothing durable changes by moving it: a failed placement returns before
	// the commit and the whole scope is dropped, sheet and all.
	//
	// It stopped being the ONLY exception at rpg-toolkit#1083, and the second one
	// names what the two have in common. [Manager.Attack] must write its damaged
	// sheets BEFORE it records the outcome, because recording now runs the same
	// standing consult and it reads the same sheets — and there the write really
	// is durable, so that ordering has a cost the paragraph above says cannot
	// exist. The rule underneath both: ordering against persistence is free only
	// while nothing READS BACK what was written inside the same verb. A consult
	// is a read-back.
	scope.data.NPCs = append(scope.data.NPCs, *sheet)
	scope.touched = true

	placed, err := place(scope, in.ID, KindMonster, sheet.Name, in.Position)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	return &SpawnOutput{
		Member:     projectMember(placed.Member),
		NPC:        projectMonster(sheet),
		Discovered: projectDiscoveries(placed.IntelDeltas, down),
		Seq:        placed.Seq,
		Outcome:    projectOutcome(placed.Outcome),
		Formed:     projectFormed(placed.Formed),
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
	scope *writeScope, id string, kind MemberKind, name string, at spatial.Position,
) (*encounter.JoinOutput, error) {
	// This used to resolve the cell to a room first, because the composition's
	// verbs were room-local by law and somebody had to say which chamber owned
	// an absolute cell. That law is gone (rpg-toolkit#1059 and the world-model
	// wave): the composition speaks one map, so the cell goes straight in and
	// the chamber that owns it is nobody's business at this seam.
	//
	// The refusal moved with it. Join validates the cell itself — owned by no
	// region, off the grid, not an integral cell — and translate() turns that
	// into ErrBadPosition, so a caller matches on the same sentinel as before
	// and the composition's account still crosses as TEXT rather than as a
	// chain a host could match on (the S2 leak, rpg-toolkit#1058).
	placed, err := scope.enc.Join(&encounter.JoinInput{
		Member: encounter.MemberID(id),
		Kind:   encounter.MemberKind(kind),
		Name:   name,
		Cell:   at,
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

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	left, err := scope.enc.Exit(&encounter.ExitInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("exit: %w", translate(err))
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("exit: %w", translate(err))
	}
	down, err := standingSet(scope.standing, rosterIDs(roster))
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	return &ExitOutput{
		Outcome:  projectMemberOutcome(left.Outcome),
		Carry:    projectSightings(left.Carry, rosterNames(roster), down),
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
//
// It used to have a twin, openForChange, that additionally refused a verb while
// an interrupt window was open. Nothing in this module opens a window any more
// (rpg-toolkit#964 slice 2), so a freeze that could never be entered was a
// branch no test could reach — see doc.go on what retired with the walk's rule
// and what wave 5 re-creates.
func (m *Manager) openForWrite(ctx context.Context, sessionID string) (*writeScope, error) {
	if sessionID == "" {
		return nil, ErrNoSessionID
	}
	data, err := m.loadSessionData(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	enc, baseline, standing, err := m.loadWorldWithBaseline(ctx, data)
	if err != nil {
		return nil, err
	}
	return &writeScope{
		session:   sessionID,
		encounter: data.Encounter,
		data:      data,
		enc:       enc,
		baseline:  baseline,
		standing:  standing,
	}, nil
}

// writeScope is everything a write verb needs to act, save, and fan out: the
// live encounter, the session record, the IDs to save and address them under,
// and the sequence boundary separating what was already recorded from what this
// verb records.
type writeScope struct {
	session   string
	encounter string
	data      *SessionData
	enc       *encounter.Encounter
	baseline  uint64

	// standing is who-is-down, answered from the sheets this call holds.
	//
	// Kept on the scope because it is needed twice after the load: to rebuild
	// the world a resolution handed back (adopt), and to refuse a verb whose
	// actor has fallen (refuseIfDown). Rebuilding it at each use would compile
	// and would quietly allow two capabilities reading different sheets within
	// one verb.
	standing encounter.Standing

	// touched marks the session record as changed by this verb — a spawned
	// sheet, today — so a verb that changed only the world writes only the
	// world. Writes stay proportional to what actually changed.
	touched bool

	// written names what this verb made durable BEFORE reaching persist —
	// character sheets, today, which are the only aggregate a verb writes on
	// its own (see [Manager.saveDirty]).
	//
	// It rides the scope for the same reason touched does: it is a fact the
	// verb discovered on the way to the commit, and persist is the one place
	// that turns facts into a report. Threading it through commit as a
	// parameter would put a nil at every other call site — none of which
	// writes a sheet — which reads as ceremony rather than as a decision.
	//
	// A verb that writes nothing early leaves this empty and its report is
	// unchanged — the entries are added by whoever wrote, never by persist on
	// their behalf.
	written []string
}

// adopt replaces the scope's encounter with one loaded from a world that came
// back from somewhere else.
//
// ONE VERB NEEDS THIS AND MORE WILL. A resolution takes the world as data and
// returns a different world as data — that is the seam's whole design — so a
// verb that resolves through it is holding a stale encounter the moment Resolve
// returns. Recording the outcome or saving from the old one would drop
// everything the interaction did.
//
// THE INVARIANT: after adopt, every earlier reference to the encounter is
// DEAD. Read nothing from before it, and record only after it — the outcome is
// a consequence of the interaction, so its beat belongs on the world the
// interaction produced, in that order.
//
// It is a named method rather than an assignment inlined in the verb so the
// next verb that resolves through data reuses this instead of hand-rolling the
// swap, and so the novelty has exactly one home to document.
func (m *Manager) adopt(scope *writeScope, world encounter.EncounterData) error {
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       world,
		Initiative: m.initiative,
		Standing:   scope.standing,
		Sight:      sightSeam{},
		TurnDriver: m.turnDriver,
	})
	if err != nil {
		return fmt.Errorf("%q: %w: %v", scope.encounter, ErrInvalidWorld, err)
	}
	scope.enc = enc
	return nil
}

// persist writes the mutated aggregates back and reports the result.
//
// The encounter is written first and the session second, and the order is a
// correctness decision rather than a style one. Both orders can fail halfway;
// they fail differently:
//
//   - Encounter lands, session does not: the world holds what happened and the
//     session record does not know about it — a spawned monster standing in a
//     room with no sheet. Every persisted fact is true and the caller is told
//     which half failed, so the damage is bounded and describable.
//   - Session lands, encounter does not: the session holds a sheet for a
//     monster that is not in any room. A record that looks healthy, describing
//     a world that never happened.
//
// The first is visible wreckage; the second is corruption that looks like
// progress. So the aggregate that records what happened goes first, and the one
// that describes it goes second.
//
// NOTE WHAT THIS DOES NOT PROMISE. Retrying the verb does not repair the first
// case: Spawn is not idempotent, and a second attempt is refused because the
// member ID is already in the world (see TestADuplicateArrivalIsRejectedButMisreported,
// which pins that rejection including the part of it that is wrong). The caller
// is told exactly which aggregate is missing — that is S6's whole job — and
// repairing it needs a decision, not a retry. Making the entry verbs idempotent
// for this case is the fix, and it is not this wave's.
func (m *Manager) persist(ctx context.Context, scope *writeScope) (SaveReport, *encounter.EncounterData, error) {
	data := scope.enc.ToData()

	// The report opens with what the verb already made durable rather than
	// starting from nothing, which is the whole of rpg-toolkit#1056: a swing
	// writes the damaged sheet before it gets here, so a world save that fails
	// leaves a report claiming nothing landed while the damage is on disk. The
	// host reads that as "safe to retry", retries, and the damage applies a
	// second time. Copied rather than aliased so appending below cannot reach
	// back into the scope.
	report := SaveReport{Written: append([]string(nil), scope.written...)}

	if err := m.encounters.SaveEncounter(ctx, scope.encounter, &data); err != nil {
		report.Failed = []string{"encounter:" + scope.encounter}
		return report, nil, &SaveError{
			Report: report,
			Err:    fmt.Errorf("saving world: %w", err),
		}
	}
	report.Written = append(report.Written, "encounter:"+scope.encounter)

	if !scope.touched {
		return report, &data, nil
	}

	if err := m.sessions.SaveSession(ctx, scope.data); err != nil {
		// The encounter is already durable, so the report names both what landed
		// and what did not (S6). A bare error here would leave the caller unable
		// to tell a total failure from a half one, which is the difference
		// between "retry the verb" and "the world moved but its record did not".
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
