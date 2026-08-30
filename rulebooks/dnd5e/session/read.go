// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// AtlasInput asks for a session's static world map.
type AtlasInput struct {
	// Session is the session whose world to describe.
	Session string
}

// AtlasOfInput asks for the static map of an authored world that no session
// holds — the same shape [StartSessionInput.World] takes.
type AtlasOfInput struct {
	// World is the authored content to describe. Required.
	World *encounter.EncounterData
}

// StatusInput asks whether a session's encounter is still running.
type StatusInput struct {
	// Session is the session to report on.
	Session string
}

// WhereInput asks where a member stands.
type WhereInput struct {
	// Session is the session to look inside.
	Session string

	// Member is the member whose own placement is being asked for.
	Member string
}

// ViewInput asks what one member currently perceives.
type ViewInput struct {
	// Session is the session to look inside.
	Session string

	// Member is the observer.
	Member string
}

// StoryInput asks for what a member has witnessed.
type StoryInput struct {
	// Session is the session to read from.
	Session string

	// Member is the viewer whose story is requested.
	Member string

	// FromSeq is the INCLUSIVE lower bound: entries begin AT this sequence.
	// Zero means "I hold nothing, send what you have" and is always
	// answerable, however much has aged out of the retention window.
	//
	// To resume after entry N, pass N+1. Named for what it does, unlike the
	// composition's own field, whose name predates its behaviour.
	FromSeq uint64
}

// Atlas returns the session's static world map: one set of cells, the ones
// that block sight, the walls between them, and every doorway's kissing pair.
//
// Construction truth — unchanged by movement, joins, exits or endings — so a
// host should fetch it once per session rather than per frame.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, or ErrNoEncounter.
func (m *Manager) Atlas(ctx context.Context, in *AtlasInput) (*Atlas, error) {
	if in == nil {
		return nil, fmt.Errorf("atlas: %w", ErrNilInput)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("atlas: %w", err)
	}

	atlas, err := enc.Atlas()
	if err != nil {
		return nil, fmt.Errorf("atlas: %w", translate(err))
	}

	projected := projectAtlas(atlas)
	return &projected, nil
}

// AtlasOf projects the map of an authored world that no session holds.
//
// The same map [Manager.Atlas] answers for a started session — the same load
// ([Manager.loadAuthored], shared with StartSession's own validation), the
// same projection — for a world a host has only compiled. A dungeon registry
// answers "what does this dungeon look like" with it (rpg-api's
// PutDungeonResponse.atlas, rpg-project#256) without starting anything, and
// because the producer is shared, what a builder previews is what the game
// will play: one projection, one producer, no second geometry to keep in
// step.
//
// A Manager method rather than a package function, deliberately. A load
// needs the capabilities a Manager is built with — initiative, standing,
// sight, a turn driver — and the only construction-only stand-in the
// composition exports is its Striker. A free function would have to invent
// the other four, which is exactly the defaulted capability this stack
// forbids.
//
// Returns ErrNilInput for a nil input, ErrInvalidWorld for a nil world or one
// that will not load.
func (m *Manager) AtlasOf(ctx context.Context, in *AtlasOfInput) (*Atlas, error) {
	if in == nil {
		return nil, fmt.Errorf("atlasof: %w", ErrNilInput)
	}
	enc, err := m.loadAuthored(ctx, in.World)
	if err != nil {
		return nil, fmt.Errorf("atlasof: %w", err)
	}

	atlas, err := enc.Atlas()
	if err != nil {
		return nil, fmt.Errorf("atlasof: %w", translate(err))
	}

	projected := projectAtlas(atlas)
	return &projected, nil
}

// Status reports whether the session's encounter is open, and how it ended if
// not.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, or ErrNoEncounter.
func (m *Manager) Status(ctx context.Context, in *StatusInput) (*Status, error) {
	if in == nil {
		return nil, fmt.Errorf("status: %w", ErrNilInput)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	status, err := enc.Status()
	if err != nil {
		return nil, fmt.Errorf("status: %w", translate(err))
	}

	return projectStatus(status), nil
}

// Where returns the cell a member stands on, in dungeon-absolute space.
//
// A client knows its own cell today only by remembering the last Move it made,
// which holds until the moment it matters: a reconnect, a fresh tab, a second
// device, a response it never received. This is the question asked directly.
//
// ONE MEMBER'S OWN PLACEMENT, and there is deliberately no read that returns
// everybody's. A roster of positions would hand a client the cells of members
// it has never perceived — around a corner, in a room it has not entered,
// behind a door it has not opened — which is the one thing perception exists to
// prevent. Where somebody ELSE is, is View's answer, and View gives it only for
// members the observer actually holds.
//
// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER. This package cannot
// know who is asking — a verb takes IDs, not identities — so the one check
// that keeps this read self-only is necessarily the host's: wire a
// client-supplied member ID through unchecked and this verb becomes the
// unperceived-position roster it refuses to be, one ID at a time, with
// monster IDs learnable from Story beats. The refusal above is only as good
// as the host's binding.
//
// It is a live read, not a stored one: the position comes from the composition's
// roster, which projects each member's cell through their room's anchor when
// asked. A member who has walked is reported where they are now.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember if the member is not in this encounter.
func (m *Manager) Where(ctx context.Context, in *WhereInput) (*WhereOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("where: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("where: %w", ErrNoMemberID)
	}

	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("where: %w", err)
	}

	// The same roster read the walk uses to find its starting cell. Converted
	// once, at the boundary, and compared as the newtype it is — the same
	// direction View and Story take a member id. Comparing the other way, by
	// stringifying the composition's ID, would work today and would stop being
	// checked the moment MemberID means anything more than a string.
	at, err := standsAt(enc, encounter.MemberID(in.Member))
	if err != nil {
		return nil, fmt.Errorf("where: %w", err)
	}
	return &WhereOutput{Position: at}, nil
}

// View returns what one member currently perceives.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember if the member is not in this encounter.
func (m *Manager) View(ctx context.Context, in *ViewInput) ([]Sighting, error) {
	if in == nil {
		return nil, fmt.Errorf("view: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("view: %w", ErrNoMemberID)
	}
	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}
	enc, err := m.loadWorld(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}

	holdings, err := enc.View(&encounter.ViewInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("view: %w", translate(err))
	}

	// Names and standing, batched over the whole roster rather than per
	// sighting (rpg-toolkit#1137) — the observer might hold a sighting for
	// anyone in it, live or memory.
	roster, err := enc.Members()
	if err != nil {
		return nil, fmt.Errorf("view: %w", translate(err))
	}
	down, err := standingSet(m.standingFor(ctx, data), rosterIDs(roster))
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}

	return projectSightings(holdings, rosterNames(roster), rosterKinds(roster), down), nil
}

// Story returns the beats a member has witnessed, from FromSeq onward
// inclusive, projected exactly as a live [EventStream] subscriber would have
// received them: same Kind, same typed Body, same Tags — one projection
// ([projectEntry]) for both paths, so a client that notices a gap and
// re-queries Story sees byte-equal entries for the same seq rather than a
// second, thinner shape it must decode differently (rpg-api-protos#239).
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, or ErrStoryTrimmed when the requested resume
// point has aged out of the retention window — in which case the caller must
// resync from zero rather than resume, since a short answer would be
// indistinguishable from a complete one.
func (m *Manager) Story(ctx context.Context, in *StoryInput) ([]Event, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("story: %w", ErrNoMemberID)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}

	entries, err := enc.Story(&encounter.StoryInput{
		Audience: encounter.MemberID(in.Member),
		// The composition's AfterSeq is an inclusive lower bound despite its
		// name; ours says so, and the value passes through unchanged.
		AfterSeq: in.FromSeq,
	})
	if err != nil {
		return nil, fmt.Errorf("story: %w", translate(err))
	}

	// projectEntry, not a second mapping: catch-up must be byte-equal to
	// what a live subscriber received for the same seq (rpg-api-protos#239).
	// The requesting member IS the recipient — Story asks after one's own
	// story, there is no other audience to address it to.
	events := make([]Event, 0, len(entries))
	for _, e := range entries {
		events = append(events, projectEntry(in.Session, in.Member, e))
	}
	return events, nil
}

// open loads a session and reconstitutes the encounter it points at.
//
// Every read verb begins here and none of them saves: S4's save step is simply
// empty for a read, and S1 holds — nothing is retained after the call.
func (m *Manager) open(ctx context.Context, sessionID string) (*encounter.Encounter, error) {
	if sessionID == "" {
		return nil, ErrNoSessionID
	}
	data, err := m.loadSessionData(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return m.loadWorld(ctx, data)
}

// loadSessionData fetches session state, translating the repository's
// ErrNotFound into this package's vocabulary so hosts match on one set of
// sentinels rather than two.
func (m *Manager) loadSessionData(ctx context.Context, sessionID string) (*SessionData, error) {
	data, err := m.sessions.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%q: %w", sessionID, ErrNoSession)
		}
		return nil, err
	}
	if data == nil {
		// A repository reporting success with no data has broken its contract.
		// Reported as that rather than as a miss, and certainly not
		// dereferenced: a nil here would panic several frames later with
		// nothing pointing back to its origin.
		return nil, fmt.Errorf(
			"%q: GetSession reported success with no data: %w", sessionID, ErrBadRepository)
	}
	if data.ID != sessionID {
		return nil, fmt.Errorf(
			"GetSession(%q) returned session %q: %w", sessionID, data.ID, ErrBadRepository)
	}
	return data, nil
}

// loadWorld fetches and reconstitutes the encounter a session points at.
//
// A construction-only Striker (rpg-project#254), exactly as StartSession's
// own validation load uses: every caller reaching here is a READ verb (open,
// and View directly) that never drives a turn, so a driven turn landing here
// at all would be this package's own bug rather than anything a caller did.
func (m *Manager) loadWorld(ctx context.Context, data *SessionData) (*encounter.Encounter, error) {
	enc, _, _, err := m.loadWorldWithBaseline(
		ctx, data, encounter.RefusingStriker{}, encounter.RefusingMover{},
		encounter.RefusingAnnouncer{}, &sightSeam{})
	return enc, err
}

// loadWorldWithBaseline additionally reports the log's next sequence at load
// time — the boundary between what was already recorded and what this verb is
// about to record.
//
// Read out of the blob that was loaded anyway, so it costs nothing. Anything
// with a sequence at or above the baseline is news, which is how a verb knows
// which beats to fan out without the composition having to report them.
//
// It also returns the standing capability it built, because a verb needs the
// SAME one afterwards — to re-load a world that came back from a resolution
// ([Manager.adopt]), and to ask about one member before letting them act
// ([refuseIfDown]). Built once per verb and handed back rather than rebuilt at
// each use, so there is exactly one answer to "which sheets is this call
// reading" for the whole call.
//
// sight is PRE-ALLOCATED BY THE CALLER, empty, and populated here from the
// blob this function fetches — the same chicken-and-egg [strikerSeam] solves
// for Striker, one capability over (rpg-project#254). A write verb's caller
// keeps its own reference to the same pointer afterward, because [place]
// needs to add a member THIS SAME call is about to place before that
// member's own Join asks Sight about it; a read verb's caller passes a
// throwaway that nothing reaches again.
func (m *Manager) loadWorldWithBaseline(
	ctx context.Context, data *SessionData,
	striker encounter.Striker, mover encounter.Mover,
	announcer encounter.Announcer, sight *sightSeam,
) (*encounter.Encounter, uint64, encounter.Standing, error) {
	encID := data.Encounter

	world, err := m.encounters.GetEncounter(ctx, encID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, 0, nil, fmt.Errorf("%q: %w", encID, ErrNoEncounter)
		}
		return nil, 0, nil, err
	}
	if world == nil {
		return nil, 0, nil, fmt.Errorf(
			"%q: GetEncounter reported success with no data: %w", encID, ErrBadRepository)
	}

	sight.members = append(sight.members, world.Members...)

	standing := m.standingFor(ctx, data)
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       *world,
		Initiative: m.initiative,
		Standing:   standing,
		Sight:      sight,
		TurnDriver: m.turnDriver,
		// The caller says which: a real one bound to a write verb's own
		// scope, or RefusingStriker{} for a read that must never drive a
		// turn. See [Manager.loadWorld] and [Manager.openForWrite].
		Striker: striker,
		// The caller says which here too: a real one bound to a write verb's
		// own scope, or RefusingMover{} for a read that must never walk
		// anybody. A step announced on a read path is the same bug an
		// announced boundary would be.
		Mover: mover,
		// And the same, one capability over. A read verb cannot advance a
		// clock, so a boundary announced on a read path is a bug rather
		// than an event — RefusingAnnouncer says so at the point of
		// failure, where a silently-succeeding no-op would be
		// indistinguishable from the boundary that never got published.
		Announcer: announcer,
	})
	if err != nil {
		// The reason is kept as TEXT, not as a chain. A blob this seam cannot
		// reconstitute fails several modules deep — the composition's own
		// validation, or a leaf's (clock, intel, record) underneath it — and
		// every one of those is a module we intend to keep replaceable. %v
		// hands whoever debugs it the whole account and hands a host nothing to
		// match on but ours (S2).
		return nil, 0, nil, fmt.Errorf("%q: %w: %v", encID, ErrInvalidWorld, err)
	}
	return enc, world.Log.NextSeq, standing, nil
}

// translate maps the composition's sentinels onto this package's own.
//
// The boundary test cannot see this. It reads exported signatures, and a
// sentinel error is not a type in a signature — so an inner package can become
// part of the host's contract through a channel the AST never looks at. If
// hosts matched on encounter.ErrTrimmed, replacing the composition would break
// their error handling exactly as surely as leaking a struct would, and
// nothing in CI would have said a word.
//
// Unrecognised errors pass through UNCHANGED rather than being flattened. This
// function adds nothing to them; the calling verb wraps what comes back with
// its own prefix, which is where "view:" and "move:" come from.
//
// That limit is deliberate rather than an oversight. The default arm carries
// errors that ORIGINATED WITH THE HOST as well as composition ones we did not
// anticipate — a failing Roller comes back out through a composition verb — and
// flattening those would break the host's matching on its own errors to protect
// it from ours. So the guarantee here is that every sentinel this seam can
// reach has an arm, and the mechanical part of that guarantee lives in the
// tests: sentinels_test.go over the refusals a caller can drive, and
// translate_internal_test.go over every arm below (rpg-toolkit#1058).
//
// A translated error carries our sentinel ALONE. Wrapping both — fmt.Errorf(
// "%w: %w", ours, theirs) — reads like generosity and is the leak itself: a
// host can still match on theirs. Where the inner message is worth keeping, the
// call site keeps it with %v, which is text rather than a chain.
func translate(err error) error {
	switch {
	case errors.Is(err, encounter.ErrTrimmed):
		return fmt.Errorf("%w", ErrStoryTrimmed)
	case errors.Is(err, encounter.ErrNoMember), errors.Is(err, encounter.ErrNotMember):
		return fmt.Errorf("%w", ErrNoMember)
	case errors.Is(err, encounter.ErrClosed):
		return fmt.Errorf("%w", ErrClosed)
	case errors.Is(err, encounter.ErrNoEnding):
		return fmt.Errorf("%w", ErrNoEnding)
	case errors.Is(err, encounter.ErrNoDoor),
		errors.Is(err, encounter.ErrBadDoor):
		return fmt.Errorf("%w", ErrNoConnection)
	case errors.Is(err, encounter.ErrLocked):
		// A locked door — the door verb's refusal AND a walk into one, now
		// that the composition names both with its own sentinel
		// (rpg-toolkit#1135). Checked BEFORE ErrBadPlacement below: a locked
		// door is a fiction beat with a DC behind it, and a caller told "bad
		// position" would go looking for arithmetic that is fine.
		return fmt.Errorf("%w", ErrLocked)
	case errors.Is(err, encounter.ErrDoorShut):
		// A merely-shut door in a walk's path — the other half of the same
		// split: shut is a state a caller can change (OpenDoor), not a bad
		// coordinate.
		return fmt.Errorf("%w", ErrDoorShut)
	case errors.Is(err, encounter.ErrBadPlacement):
		return fmt.Errorf("%w", ErrBadPosition)
	case errors.Is(err, encounter.ErrNoField), errors.Is(err, encounter.ErrInvalidData):
		// Both mean the stored world cannot answer: a field that is defective
		// or does not hold the room somebody stands in, and a blob that cannot
		// be reconstituted at all. One sentinel because the host's remedy is
		// the same either way — this encounter's data is unusable, and the
		// repair is upstream of anything a caller can retry.
		return fmt.Errorf("%w", ErrInvalidWorld)
	case errors.Is(err, encounter.ErrInBubble):
		return fmt.Errorf("%w", ErrInBubble)
	case errors.Is(err, encounter.ErrNoBubble):
		return fmt.Errorf("%w", ErrNotInFight)
	case errors.Is(err, encounter.ErrNotActive):
		// Two producers, one arm (rpg-toolkit#1169): EndTurn's own refusal for
		// the wrong member, and now Step's — the active-member gate that lets
		// a fight member walk at all. Before this arm existed, EndTurn's
		// refusal reached a caller as the LEAF play/clock.ErrNotActive,
		// unnamed at every boundary it crossed; naming it here closes that
		// for both callers at once rather than leaving Move to rediscover it.
		return fmt.Errorf("%w", ErrNotYourTurn)
	default:
		return err
	}
}

// causeOf translates the composition's account of why a fight ended into this
// package's own sealed set.
//
// It exists because the answer must be DERIVED. A fight ends two ways now — a
// party breaking off, and a side running out of people standing — so the cause
// on a response has to come from what the world did rather than from what the
// caller said it was doing. Echoing the input back is indistinguishable from
// deriving it right up until the two can disagree, and as of rpg-toolkit#1078
// they can.
//
// Being TOTAL is the point, and it is what earns [ByDefeat] its case here. The
// composition keeps its own sealed set (it cannot import this one; this one
// imports it), so the two are extended at the layer their callers live in, and
// this is the seam between them. A translation that could not name defeat would
// have to report it as something else.
//
// The default arm is a composition NEWER than this build, carrying a cause this
// one has no name for. It is refused rather than flattened onto a cause we do
// know, which is the mistake kindOf's doc warns about one file over: a silent
// mapping onto the wrong value narrates the wrong thing forever, while a
// refusal is noticed the first time it happens. ErrInvalidWorld is the sentinel
// because the remedy is the host's and it is upstream of anything a caller can
// retry — this build cannot read that world.
func causeOf(cause encounter.DissolveCause) (DissolveCause, error) {
	switch cause.Kind() {
	case encounter.DissolveByDecision:
		return ByDecision(), nil
	case encounter.DissolveByDefeat:
		return ByDefeat(), nil
	default:
		return nil, fmt.Errorf("unknown dissolve cause %q: %w", cause.Kind(), ErrInvalidWorld)
	}
}
