// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
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

	// Corrected reports location-belief corrections made by driven turns.
	Corrected []IntelCorrection `json:"corrected,omitempty"`

	// Seq is the join beat's sequence IN THE JOINER'S OWN delivered
	// numbering (stream.go) — the same number their event for it carries.
	// The record's global sequence stays internal to the seam.
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

	// Corrected reports location-belief corrections made by driven turns.
	Corrected []IntelCorrection `json:"corrected,omitempty"`

	// Seq is the story sequence of the recorded arrival — the RECORD's own
	// numbering, because Spawn has no acting member to number for: the
	// caller is the host, and the host's view is the whole record. Every
	// member-driven verb reports in its actor's delivered numbering
	// instead (stream.go).
	Seq uint64

	// Outcome is present if an ending fired on the spawn.
	Outcome *Outcome

	// Formed is present if the spawned content arrived in sight of the party
	// and a fight started. This is the reason Formed is not a movement-only
	// field: nobody walked anywhere, and a fight started.
	//
	// Its Seq is the RECORD's numbering, like the sibling Seq above and for
	// the same reason: Spawn has no acting member to number for. THE HOST
	// MUST NEVER FORWARD SpawnOutput's Seq or Formed.Seq to a client beside
	// per-recipient events — a record number next to a member's own dense
	// stream is the gap oracle returning through a side door; clients hear
	// about the arrival through their own numbered beats.
	Formed *Formed

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// PlaceNPCInput places a caller-supplied world NPC into a session's
// encounter (rpg-toolkit#1404).
//
// NPC IS CALLER-SUPPLIED, NOT RESOLVED FROM A REF — the one place this
// verb's shape diverges from Spawn's, deliberately. instantiate() resolves
// a monster's ref through monsters.ByRef, a real toolkit-shipped catalog of
// code-built stat blocks; no NPC equivalent exists or is planned
// (docs/ideas/dnd5e-npcs/design.md already ruled out a NewBlacksmith-style
// toolkit archetype). So where Spawn takes a Ref string and builds the
// content itself, PlaceNPC takes the already-built content directly — the
// caller decided default-vs-explicit (npcs.NewMerchant's nil-vs-config
// signal) before this verb is ever called, and this verb does not
// re-interpret that decision a second time.
type PlaceNPCInput struct {
	// Session is the session to place into.
	Session string

	// Member is the ID the new member is known by inside the encounter.
	Member string

	// Position is the cell to place it on, in dungeon-absolute space. See
	// JoinInput.Position.
	Position spatial.Position

	// NPC is the already-built content this member is placed from. Required
	// — nil is refused with ErrNoRef, the same sentinel Spawn uses for an
	// empty Ref: the same shape of caller mistake, the same error.
	NPC *npc.Data
}

// PlaceNPCOutput reports the placement and what it revealed.
type PlaceNPCOutput struct {
	// Member is the new member's placement.
	Member Member

	// Discovered is what changed in each observer's perception, keyed by
	// observer. Absent observers saw nothing new.
	Discovered map[string]Discovery

	// Corrected reports location-belief corrections made by driven turns.
	Corrected []IntelCorrection `json:"corrected,omitempty"`

	// Seq is the story sequence of the recorded arrival — the RECORD's own
	// numbering, for the same reason SpawnOutput.Seq is (PlaceNPC has no
	// acting member to number for either).
	Seq uint64

	// Outcome is present if an ending fired on the placement.
	Outcome *Outcome

	// Formed is present if the placed NPC arrived in sight of both sides and
	// a fight started around it. In the MVP this should never actually
	// happen — KindWorld is on neither side of sidesInContactOrder,
	// structurally — but the field exists for the same reason SpawnOutput's
	// does: this verb reports what encounter.Join actually returned rather
	// than assuming a shape.
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

	// Discovered is what changed in remaining observers' perception.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Corrected reports location-belief corrections made by driven turns.
	Corrected []IntelCorrection `json:"corrected,omitempty"`

	// Seq is the exit beat's sequence IN THE DEPARTING MEMBER'S OWN
	// delivered numbering (stream.go).
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
// On the character's first-ever admission to this encounter, Join resolves a
// normal long rest from the persisted record and immediately saves it before
// projection or placement. That early write makes the repository authoritative
// for every standing, cast, and driven-turn callback placement may trigger. A
// later failure leaves the valid between-runs rest durable and reports it in a
// SaveError. EverMembers is the durable admission record, so a current
// duplicate and an exit/rejoin never rest or save again.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoCharacter, ErrBadCharacter, ErrBadAttack if the
// character's own main-hand weapon cannot be compiled into their member
// record's static Actions fact, ErrBadPosition if no room owns the cell they
// were placed on, ErrClosed if the encounter has already ended, or
// ErrSaveFailed with a populated report.
//
// ErrBadAttack WENT AWAY AND CAME BACK, which is worth a line because the
// record should not read as though it never left. When the compiling moved into
// resolution's projection, that entry reported the failure as a bad participant
// and this seam could translate it only one way, so a broken loadout arrived as
// a corrupt character. The projection reports the finer failure under its own
// sentinel now and [projectionSentinel] reads it to choose this package's word.
// A host can tell "this player's weapon is broken" from "this player's sheet is
// corrupt" again.
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

	// Read the durable admission record BEFORE encounter.Join can add this
	// member to it. Current membership is deliberately not the gate: Exit keeps
	// EverMembers, which is what makes a later Join a rejoin rather than another
	// launch rest.
	firstAdmission := true
	persisted := scope.enc.ToData()
	for _, member := range persisted.EverMembers {
		if string(member) == in.Member {
			firstAdmission = false
			break
		}
	}

	record, err := m.fetchCharacterData(ctx, "character", in.Member)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	if firstAdmission {
		resolved, restErr := resolution.LongRest(ctx, &resolution.LongRestInput{Character: record})
		if restErr != nil {
			// Resolution's vocabulary stays behind this seam just as it does for
			// projection: the host can act on ErrBadCharacter, while the inner
			// reason remains available as text for diagnosis.
			return nil, fmt.Errorf("join: character %q: %w: long rest: %v",
				in.Member, ErrBadCharacter, restErr)
		}
		if resolved == nil || resolved.Character == nil {
			return nil, fmt.Errorf("join: character %q: %w: long rest returned no character data",
				in.Member, ErrBadCharacter)
		}

		// Persist before ANY projection or placement callback. encounter.Join
		// can consult standing, form a fight, and drive a monster action; each
		// of those paths reads CharacterRepository. Saving here gives all of
		// them the same rested truth this Join projects, and any later driven
		// write is newer than this one rather than being overwritten by it.
		aggregate := "character:" + in.Member
		if err := m.characters.SaveCharacter(ctx, resolved.Character); err != nil {
			return nil, saveErrorAfterWrites(scope, aggregate,
				fmt.Errorf("saving character: %w", err))
		}
		scope.written = append(scope.written, aggregate)
		record = resolved.Character
	}

	// ONE QUESTION, ONE ANSWER. The record goes down and everything this verb
	// needs about the character comes back: the folded armour class, the static
	// facts, and the main-hand attack. Join used to load a sheet of its own and
	// read three of those off it; what it holds now is a record on the way in
	// and numbers on the way out.
	//
	// Asked BEFORE the placement because the placement needs the name and
	// speed. On first admission the rest is already durable, so every failure
	// from here through commit must carry scope.written in its SaveError rather
	// than masquerade as a no-write refusal.
	projected, err := projectCharacter(ctx, in.Member, record)
	if err != nil {
		return nil, fmt.Errorf("join: %w", saveErrorAfterWrites(scope, "", err))
	}

	actions, err := memberActionsFrom(projected.MainHand)
	if err != nil {
		return nil, fmt.Errorf("join: %w", saveErrorAfterWrites(scope, "", err))
	}

	placed, err := place(scope, in.Member, KindPlayer, projected.Sheet.Name, in.Position,
		projected.Sheet.SpeedFeet, defaultSightFeet, actions, "")
	if err != nil {
		return nil, fmt.Errorf("join: %w", saveErrorAfterWrites(scope, "", err))
	}

	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("join: %w", saveErrorAfterWrites(scope, "", err))
	}

	state := characterStateFrom(projected)

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	return &JoinOutput{
		Member:     projectMember(placed.Member),
		Character:  state,
		Discovered: projectDiscoveries(placed.IntelDeltas, down),
		Corrected:  projectIntelCorrections(placed.IntelDeltas),
		Seq:        scope.deliveredSeq(in.Member, placed.Seq),
		Outcome:    projectOutcome(placed.Outcome),
		Formed:     projectFormedFor(scope, in.Member, placed.Formed),
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
// Fetched BEFORE the encounter commit, deliberately: a failure here must not
// persist the local encounter mutation. A first-admission Join may already have
// persisted its independently valid rest; its caller wraps this failure with
// the durable character identity rather than claiming nothing was written.
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

	// The stat block's own authored Darkvision, UNCHANGED — zero included.
	// sightSeam is the one place that decides what a monster's silence
	// means: the same defaultSightFeet a character's own silence falls
	// back to (rpg-project#254 design §5) — see sight.go's doc for why
	// baking a decision in here, at spawn time, would be the wrong place
	// to make it.
	placed, err := place(scope, in.ID, KindMonster, sheet.Name, in.Position,
		sheet.Speed.Walk, sheet.Senses.Darkvision, memberActionsFromMonster(sheet.Actions), sheet.Targeting.String())
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
		Corrected:  projectIntelCorrections(placed.IntelDeltas),
		Seq:        placed.Seq,
		Outcome:    projectOutcome(placed.Outcome),
		Formed:     projectFormed(placed.Formed),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// PlaceNPC places a caller-supplied world NPC into the session's encounter
// (rpg-toolkit#1404). See PlaceNPCInput's own doc for why this takes
// already-built content rather than resolving a ref, the one place its
// shape diverges from Spawn's.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoRef (a nil NPC),
// ErrNoSession, ErrNoEncounter, ErrBadPosition if no room owns the cell,
// ErrClosed, or ErrSaveFailed with a populated report.
func (m *Manager) PlaceNPC(ctx context.Context, in *PlaceNPCInput) (*PlaceNPCOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("place npc: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("place npc: %w", ErrNoMemberID)
	}
	if in.NPC == nil {
		// ErrNoRef's own text is "empty ref" — accurate for Spawn's empty
		// Ref string, misleading here where the whole NPC pointer is nil,
		// not a Ref field within it. Kept as the wrapped sentinel (Copilot,
		// PR #1414 review) so errors.Is(err, ErrNoRef) still matches; only
		// the message changes.
		return nil, fmt.Errorf("place npc: NPC is required: %w", ErrNoRef)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("place npc: %w", err)
	}

	// THE CONTENT IS RECORDED BEFORE THE PLACEMENT — Spawn's own exception
	// to "ordering against persistence is free only before the commit"
	// (write.go's Spawn doc explains the general rule and this exception to
	// it in full). Arriving refreshes sight, and every sight refresh asks
	// who is standing; a member placed before its record exists is asked
	// about while the record still has nothing to say. A non-combatant
	// world NPC has no standing question to answer, but the ordering stays
	// identical to Spawn's on purpose — one placement shape, not two, for
	// the same reason `place()` itself is shared rather than duplicated.
	scope.data.WorldNPCs = append(scope.data.WorldNPCs, PlacedWorldNPC{
		MemberID: in.Member,
		NPC:      *in.NPC,
	})
	scope.touched = true

	// A world NPC is stationary and non-acting by construction (design.md
	// N4): zero speed/sight, no actions, no targeting strategy. encounter
	// enforces the one real rule (no decider) itself; place() never sets
	// one for either existing caller.
	placed, err := place(scope, in.Member, KindWorld, in.NPC.DisplayName, in.Position,
		0, 0, nil, "")
	if err != nil {
		return nil, fmt.Errorf("place npc: %w", err)
	}

	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("place npc: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("place npc: %w", err)
	}

	return &PlaceNPCOutput{
		Member:     projectMember(placed.Member),
		Discovered: projectDiscoveries(placed.IntelDeltas, down),
		Corrected:  projectIntelCorrections(placed.IntelDeltas),
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
	speedFeet, sightFeet int, actions []encounter.ActionView, targeting string,
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
	//
	// scope.sight learns about THIS member before Join does, not after:
	// arriving triggers its own sight refresh (does the newcomer see
	// anyone; is the newcomer seen), and that refresh asks about the
	// newcomer by ID — which the seam's snapshot cannot yet answer for
	// unless this runs first (sight.go's own doc explains why a pointer
	// makes that possible at all).
	scope.sight.add(encounter.MemberID(id), sightFeet)
	placed, err := scope.enc.Join(&encounter.JoinInput{
		Member: encounter.MemberID(id),
		Kind:   encounter.MemberKind(kind),
		Name:   name,
		Cell:   at,
		// SpeedFeet, SightFeet, Actions and Targeting are this member's
		// static facts (rpg-project#254) — what a TurnDriver reads through
		// MonsterView once the clock lands on this member with nobody
		// playing them. Both callers (Join, Spawn) compute these off the
		// sheet or catalog content they just loaded and hand them straight
		// through; see each verb's own doc for where its values come from.
		SpeedFeet: speedFeet,
		SightFeet: sightFeet,
		Actions:   actions,
		Targeting: targeting,
	})
	if err != nil {
		return nil, translate(err)
	}
	return placed, nil
}

// memberActionsFrom maps a joining player's main-hand attack onto the shared
// member record's static Actions fact (rpg-project#254).
//
// The compiling happens in resolution now, off the same sheet the armour class
// was folded on. What is left here is the mapping, which is the shape this seam
// is supposed to have: facts in, a member record out.
//
// FILLED FOR A PLAYER TOO, even though nothing reads it back today — a
// TurnDriver is only ever asked about an UNPLAYED member's turn. The same
// argument [memberRecord]'s own doc makes for Name: SpeedFeet, SightFeet,
// Actions and Targeting are member facts for every kind, not a monster-only
// extra, and the day a disconnected player's turn needs driving, the record
// already has what it needs.
//
// NO ATTACK IS AN ERROR HERE, and it stays one. A character the rules can build
// at all has a main hand — an empty one is an unarmed strike, which is why the
// entry never answers nil for a sheet it could load. So nil means something
// upstream stopped answering, and reporting it as "this member has no actions"
// would seat a player who cannot swing and say nothing.
func memberActionsFrom(attack *resolution.AttackFacts) ([]encounter.ActionView, error) {
	if attack == nil {
		return nil, fmt.Errorf("%w: no main-hand attack was compiled", ErrBadAttack)
	}

	return []encounter.ActionView{{
		Ref: attack.Ref, Name: attack.Name,
		RangeFeet: attack.RangeFeet,
		Kind:      attack.Kind,
	}}, nil
}

// memberActionsFromMonster projects every attack definition directly into the
// composition's opaque action view.
func memberActionsFromMonster(definitions []combatActions.Definition) []encounter.ActionView {
	views := make([]encounter.ActionView, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Attack == nil {
			continue
		}
		views = append(views, encounter.ActionView{
			Ref: definition.Ref, Name: definition.Name,
			RangeFeet: definition.Attack.Delivery.MaxRangeFeet(),
			Kind:      deliveryKind(definition.Attack.Delivery),
		})
	}
	return views
}

func deliveryKind(delivery combatActions.AttackDelivery) string {
	if delivery.IsMelee() {
		return "melee"
	}
	return "ranged"
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
		Outcome:    projectMemberOutcome(left.Outcome),
		Carry:      projectSightings(left.Carry, rosterNames(roster), rosterKinds(roster), down),
		Discovered: projectDiscoveries(left.IntelDeltas, down),
		Corrected:  projectIntelCorrections(left.IntelDeltas),
		Seq:        scope.deliveredSeq(in.Member, left.Seq),
		Closed:     projectOutcome(left.Closed),
		Saved:      report,
		Delivery:   delivery,
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

	// scope is allocated here, with its Session/Encounter/Data half filled,
	// BEFORE the load that needs to hand a Striker to the very *Encounter
	// this scope will go on to hold — the chicken-and-egg [strikerSeam]
	// exists to solve (rpg-project#254). strikerSeam reads scope.enc only
	// from inside Strike, which nothing can call until this function has
	// already returned it, so backfilling enc/baseline/standing below, after
	// the load, is safe: by the time a driven turn ever reaches Strike,
	// every field it reads is set.
	scope := &writeScope{
		session:   sessionID,
		encounter: data.Encounter,
		data:      data,
		sight:     &sightSeam{},
	}
	enc, baseline, standing, err := m.loadWorldWithBaseline(
		ctx, data, strikerSeam{m: m, scope: scope}, announcerSeam{m: m, scope: scope}, scope.sight,
		checkSeam{m: m, scope: scope}, witnessSeam{scope: scope})
	if err != nil {
		return nil, err
	}
	scope.enc = enc
	scope.baseline = baseline
	scope.standing = standing
	return scope, nil
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

	// sight is the SAME *sightSeam the live encounter holds — see the
	// type's own doc on why a pointer, not a value. place adds a member
	// being placed by THIS verb to it before that member's own Join asks
	// Sight about them; adopt rebuilds it from the world a resolution
	// handed back, the same way it rebuilds standing.
	sight *sightSeam

	// touched marks the session record as changed by this verb — a spawned
	// sheet, today — so a verb that changed only the world writes only the
	// world. Writes stay proportional to what actually changed.
	touched bool

	// checks is the sheets this verb staged for check resolution, keyed by
	// member — [stageCheck] writes it, [checkSeam] reads it at consult time.
	// Nil for the many verbs that roll no checks.
	checks map[string]*stagedCheck

	// numbers is this verb's per-recipient numbering, computed by commit
	// before the save and read afterwards by event projection and by the
	// verb's own output fields (stream.go). Nil until commit runs.
	numbers *streamNumbers

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

// deliveredSeq translates one recorded beat's global sequence into a member's
// own delivered numbering — the only numbering a verb's output may carry
// (stream.go). Zero for a beat that was never delivered to that member, which
// for a verb's OWN beat cannot happen: every verb's beat is audienced to its
// actor.
func (s *writeScope) deliveredSeq(member string, seq uint64) uint64 {
	return s.numbers.deliveredSeq(member, seq)
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
	scope.sight = &sightSeam{members: append([]encounter.MemberData(nil), world.Members...)}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       world,
		Initiative: m.initiative,
		Standing:   scope.standing,
		Sight:      scope.sight,
		TurnDriver: m.turnDriver,
		// Bound to the SAME scope, not rebuilt: this replaces scope.enc, and
		// strikerSeam only ever reads scope.enc from inside a later Strike
		// call, well after this assignment lands (rpg-project#254).
		Striker: strikerSeam{m: m, scope: scope},
		// Bound to the same scope for the same reason, and rebound here for
		// a sharper one: adopt REPLACES scope.enc, and the composition
		// announces from inside its own verbs, so the seam the new
		// encounter carries must be the one that reads this scope.
		Announcer: announcerSeam{m: m, scope: scope},
		// The concealment pair, bound to the same scope for the same
		// reason: the world coming back may carry concealed structure, and
		// the seams read scope.enc — which this assignment is about to
		// replace — only at consult time.
		CheckResolver: checkSeam{m: m, scope: scope},
		Witness:       witnessSeam{scope: scope},
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
//
// ONE MORE WEDGE, NAMED RATHER THAN PATCHED (the rebind review of
// rpg-toolkit#1377; the same admission is on rpg-project#351's record): a
// crash between the two saves loses the stream cursors the session save was
// carrying, and for a NORMAL verb the next load re-derives them from the
// persisted log — self-healing, by numberEntries' own arithmetic. For a verb
// that appended MORE BEATS THAN THE RETENTION WINDOW, the encounter save has
// already trimmed the blob's floor past every cursor, so re-derivation is
// impossible and every subsequent verb refuses at the trim-outran guard,
// permanently, as ErrInvalidWorld. That is fail-closed and TRUTHFUL — no
// beat was delivered that was not saved (delivery waits for both saves), the
// world never lies, and it is strictly better than the pre-fix behavior
// (which refused the big verb outright, every time, crash or no crash). It
// is also unhealable by retry, which is why it is admitted here in the
// ordering's own doc: a remediation path — reseeding cursors at the cost of
// a client resync, or journaling them beside the blob — is a named shelf,
// not slice work.
func (m *Manager) persist(
	ctx context.Context, scope *writeScope, data encounter.EncounterData,
) (SaveReport, error) {

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
		return report, &SaveError{
			Report: report,
			Err:    fmt.Errorf("saving world: %w", err),
		}
	}
	report.Written = append(report.Written, "encounter:"+scope.encounter)

	if !scope.touched {
		return report, nil
	}

	if err := m.sessions.SaveSession(ctx, scope.data); err != nil {
		// The encounter is already durable, so the report names both what landed
		// and what did not (S6). A bare error here would leave the caller unable
		// to tell a total failure from a half one, which is the difference
		// between "retry the verb" and "the world moved but its record did not".
		report.Failed = append(report.Failed, "session:"+scope.session)
		return report, &SaveError{
			Report: report,
			Err:    fmt.Errorf("saving session: %w", err),
		}
	}
	report.Written = append(report.Written, "session:"+scope.session)
	return report, nil
}

// commit saves the mutated world and then fans out what it recorded.
//
// The order is the law (S9): publish only after the save lands. Announcing a
// fact that failed to persist is the one mistake with no recovery — a client
// told the ogre died, a world in which it did not, and no sequence gap to
// betray the difference.
//
// exitDissolvedCombatants runs FIRST, before the save: a sheet it clears must
// land in the SAME persist this verb already makes, never a second write
// cycle a failure between the two could leave half-done.
func (m *Manager) commit(ctx context.Context, scope *writeScope) (SaveReport, DeliveryReport, error) {
	if err := m.exitDissolvedCombatants(ctx, scope); err != nil {
		report := SaveReport{Written: append([]string(nil), scope.written...)}
		return report, DeliveryReport{}, saveErrorAfterWrites(scope, "", err)
	}

	// Number every member's stream and BUILD the delivery batch BEFORE the
	// FINAL save-point ToData, from a pure WorldView and the live Story — the
	// ordering retention-at-the-storage-boundary makes load-bearing. Join's
	// earlier ToData reads only the already-persisted EverMembers before that
	// verb appends anything; it cannot trim the current verb's delta.
	// (encounter v0.43.0, #1381/#1385): ToData trims, so a verb whose delta
	// outgrew the retention window would lose its own early beats to any
	// read that came after it. Numbering failure fails the verb before
	// anything lands — R5's atomicity, and the fail-closed arm of the
	// numbering design. Delivery still WAITS for the save (S9): computed
	// here, handed to the stream only after persist reports the world
	// durable.
	view := scope.enc.WorldView()
	numbers, cursors, err := buildStreamNumbers(scope.enc, &view, scope.data.Streams)
	if err != nil {
		report := SaveReport{Written: append([]string(nil), scope.written...)}
		return report, DeliveryReport{}, saveErrorAfterWrites(scope, "", err)
	}
	scope.numbers = numbers
	if !cursorsEqual(scope.data.Streams, cursors) {
		scope.data.Streams = cursors
		scope.touched = true
	}
	events := m.projectEvents(scope, &view)

	// THE final storage boundary: this ToData runs after the verb's complete
	// delta has been numbered and projected. Everything before this line read
	// that whole delta; everything after reads only what storage keeps.
	world := scope.enc.ToData()

	report, err := m.persist(ctx, scope, world)
	if err != nil {
		return report, DeliveryReport{}, err
	}
	return report, m.deliver(ctx, events), nil
}

// exitDissolvedCombatants clears the action economy of every player whose
// fight THIS CALL just dissolved — the other half of [readyForTurn]'s own
// ignition (economy.go). StartTurn lights a cold sheet the first time an
// actor on the fight clock acts; nothing anywhere in this module ever put the
// light back out. grep -rn ExitCombat rulebooks/dnd5e/session
// rulebooks/dnd5e/encounter turns up only [character.Character.ExitCombat]'s
// own definition — no caller.
//
// Left unlit, [character.Character.InCombat] answers true forever after a
// member's first-ever combat turn in a session, so [readyForTurn]'s
// `!sheet.InCombat()` branch — the one that unconditionally reseeds via
// StartTurn — can never fire again for that character. Every later fight
// falls to RefreshForTurn instead, which is a deliberate no-op whenever the
// new fight's round happens to equal whatever TurnNumber was left on the
// sheet — and since every bubble's own round counter starts fresh at 1
// ([play/clock]'s Turn is per-bubble, never global to the session), a round-1
// collision with a stale round-1 economy from an earlier, unrelated fight is
// the common case, not a rare one. rpg-project#253 (Kirk, live, two
// browsers): a member recruited into a running fight started their own first
// turn in it with 5 of 30 feet left — exactly the number their PREVIOUS
// fight left on their sheet when it ended by defeat, mid-turn, with no
// EndTurn ever called (encounter/dissolve.go's ByDefeat: "the composition
// NOTICES defeat, never something a caller declares").
//
// Read off the SAME beats [Manager.projectEvents] already fans out from —
// every ever-member's own story since the scope's baseline — rather than
// re-deriving who dissolved from scratch: a "bubble-dissolved" beat already
// names everyone the fight held, in its `members` field
// (encounter/dissolve.go's dissolveBubble is the ONE place that beat is
// written, shared by an explicit [Manager.Dissolve] and the composition
// noticing defeat on its own — "Both endings run exactly this... Only the
// cause differs" — so this one read covers both without needing to know
// which produced it). TestTheLastOneDownedEndsTheFightByDefeat (death_test.go)
// pins the payload shape this reads.
//
// A member whose OWN story cannot be read is skipped — the same best-effort
// law [Manager.publish] states for delivery, and for the same reason: a
// read failure for one member's perception must not silence the rest of the
// table. WHAT IS NOT best-effort is a member the read DID name: a monster ID
// (no loadable character at all, [Manager.fetchCharacterData]'s own
// ErrNoCharacter) is skipped by design, but any OTHER error — a repository
// outage, a sheet that will not load, a failed save — is returned and
// FAILS THE VERB. The reason is durability, not caution: this call runs
// BEFORE [Manager.persist], so a failure here means nothing about this
// call's own dissolution has landed yet — a caller who retries the whole
// verb finds this cleanup still pending against the SAME baseline. Letting
// it fail silently instead would let the dissolve beat persist while the
// sheet it named stays stale, and — because the next call's own baseline
// moves past that beat — never retried again (Copilot's own finding on
// PR #1222).
func (m *Manager) exitDissolvedCombatants(ctx context.Context, scope *writeScope) error {
	// A pure view, not ToData: this is a mid-verb roster read after a verb may
	// have appended beats, so the final storage boundary still belongs to
	// commit (encounter v0.43.0, #1385). Join's admission check is the narrow
	// exception: it snapshots only before that verb mutates the encounter.
	data := scope.enc.WorldView()

	// Kind, from the ENCOUNTER's own authoritative roster — never inferred
	// from whether an ID happens to load out of the character repository
	// (Copilot's own finding on PR #1222). Spawn's only uniqueness guard is
	// "not a CURRENT member of this encounter"; nothing stops a monster ID
	// from colliding with a real character ID belonging to a different
	// session entirely. Asking the character store "does this load" would
	// answer yes for that collision and reset a stranger's sheet. Asking the
	// roster "is this a player" cannot be fooled by an ID that merely
	// resembles one.
	kindByID := make(map[string]encounter.MemberKind, len(data.Members))
	for _, member := range data.Members {
		kindByID[string(member.ID)] = member.Kind
	}

	seen := map[string]bool{}

	for _, member := range data.EverMembers {
		entries, err := scope.enc.Story(&encounter.StoryInput{
			Audience: member, AfterSeq: scope.baseline,
		})
		if err != nil {
			continue
		}

		for _, entry := range entries {
			var peek struct {
				Beat    string   `json:"beat"`
				Members []string `json:"members"`
			}
			if json.Unmarshal(entry.Payload, &peek) != nil || peek.Beat != "bubble-dissolved" {
				continue
			}

			for _, id := range peek.Members {
				if seen[id] || kindByID[id] != encounter.KindPlayer {
					continue
				}
				seen[id] = true
				if err := m.exitCombatIfPlayer(ctx, scope, id); err != nil {
					return fmt.Errorf("exit combat for dissolved member %q: %w", id, err)
				}
			}
		}
	}

	return nil
}

// exitCombatIfPlayer clears one member's action economy. The caller has
// already confirmed this ID is a PLAYER on the encounter's own roster
// (exitDissolvedCombatants) — a monster ID never reaches here at all, so an
// ErrNoCharacter from [Manager.fetchCharacterData] below would be a real
// inconsistency (a roster naming a player the character store does not
// hold), not the ordinary case it would be without that filter, and is
// returned rather than swallowed for the same reason every other fetch
// error is: a repository outage or corrupt data must not masquerade as
// nothing-to-clear (Copilot's own finding on PR #1222). A player whose
// sheet is already out of combat does nothing, but that is
// [character.Character.InCombat] answering false, not an error.
//
// Saved through [Manager.saveWalker] — move.go's own name for "save one
// sheet and mark the scope written", generic despite it, and the same
// mechanism a walk's own spend already uses. Its error is returned rather
// than discarded (Copilot's own finding on PR #1222): a save that silently
// failed here would report success while the stale economy it was
// supposed to clear stayed exactly as stale as it started.
func (m *Manager) exitCombatIfPlayer(ctx context.Context, scope *writeScope, id string) error {
	data, err := m.fetchCharacterData(ctx, "member", id)
	if err != nil {
		return err
	}
	sheet, err := character.Load(ctx, data)
	if err != nil {
		return fmt.Errorf("member %q: %w: %v", id, ErrBadCharacter, err)
	}
	if !sheet.InCombat() {
		return nil
	}
	if _, err := sheet.ExitCombat(ctx, &character.ExitCombatInput{}); err != nil {
		return err
	}
	return m.saveWalker(ctx, scope, sheet)
}
