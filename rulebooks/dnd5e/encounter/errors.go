// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "errors"

var (
	// ErrNilInput is returned when a nil *XxxInput is passed to an operation.
	// Indicates a caller defect.
	ErrNilInput = errors.New("nil input")

	// ErrNoMember is returned when an input contains an empty or
	// duplicate member ID (Setup, Join, and Load — Load's identical
	// checks used to carry only ErrInvalidData, #929 hardening round F),
	// a member is declared a player while carrying a Decider — design
	// law C2 — at any of the three seams that accept one (NewEncounter,
	// LoadEncounter, Join), Join names a member ID already in the
	// encounter, Exit is called with an empty member ID, Story's
	// audience never joined, or — Load-only, since these are persisted-
	// state coherence checks with no Setup analogue — a persisted
	// abandoned outcome with non-empty membership (abandonment means the
	// membership emptied) or a current member missing from EverMembers
	// (#929 hardening round F).
	ErrNoMember = errors.New("empty member id")

	// ErrNotMember is returned when an entity is not a member of this encounter.
	ErrNotMember = errors.New("not a member")

	// ErrNoRegion is returned when [Encounter.MembersIn] is asked about a
	// region the field does not have. An EMPTY region is an ordinary answer —
	// "nobody has reached the tomb yet" is a fact worth reporting — so a
	// mistyped region name must not be able to say it (rpg-toolkit#1108).
	ErrNoRegion = errors.New("no such region")

	// ErrNoEnding is returned when Setup or Load is called with zero
	// endings, a declared ending's key is empty or one of the reserved
	// "abandoned"/"party_defeated" keys, a declared ending's key duplicates
	// an earlier one in
	// the same input (#929 hardening round E — an unvalidated duplicate
	// lets declaration order silently shadow a later ending, e.g. a
	// reached_position twin permanently hiding an external ending), End
	// is called with an undeclared key or one whose Trigger is not
	// TriggerExternal, or a TriggerReachedPosition ending names an
	// unknown room or an unreachable position (#929 T3 Opus round F5,
	// checked identically at Setup and Load — see
	// validateEndingTriggers), or — Load-only, since these are wire-
	// format/persisted-state concerns with no Setup analogue —
	// EndingData names an unrecognized Kind string, a reached_position
	// EndingData is missing its Room or Position, or a persisted Outcome
	// names an ending key that was never declared (these three used to
	// carry only ErrInvalidData, #929 hardening round F). An encounter
	// that cannot end — whether it declares zero endings or one that can
	// never fire — is a liveness hole.
	ErrNoEnding = errors.New("no such ending")

	// ErrClosed is returned when a mutating verb (action, event, exit, etc.)
	// is called on a closed encounter. A closed encounter has an Outcome.
	ErrClosed = errors.New("encounter closed")

	// ErrNoField is returned when the field as a whole cannot be built
	// (rpg-project#256): no regions at all; a region with an empty or
	// duplicate ID, a non-integral or out-of-range cell (maxAnchorCoord), or
	// a lighting intensity outside [0,1]; more cells than maxFieldCells across
	// the field; a prop with no ref, with either blocking answer left unsaid
	// (rpg-toolkit#1128 — [PropInput]), on a non-integral cell, or sharing a
	// cell with another prop; a wall listed twice; or — Load-only — a blob
	// carrying the retired `rooms` / `connections` keys (FieldData.Rooms),
	// or a region whose lighting block omits its intensity. The region
	// defects with a sentinel of their own (ErrRegionEmpty, ErrRegionOverlap,
	// ErrRegionArchetypeMissing, ErrRegionLightingMissing) and the edge
	// defects (ErrEdgeNotAdjacent, ErrEdgeOffFloor) carry that sentinel
	// instead.
	//
	// Checked identically at Setup and Load: LoadEncounter converts the blob
	// back into a [FieldInput] and routes it through the SAME compileField
	// Setup uses, so the two seams cannot drift on these checks by
	// construction, not just by convention. The shared validator's own
	// messages carry no verb prefix — NewEncounter and LoadEncounter each
	// wrap their own ("newencounter:" / "load encounter:") at the call site.
	//
	// And when the field does not say what its VOID is (rpg-toolkit#1116) or
	// which way its hexes point (rpg-toolkit#1127) — at Setup, a nil
	// [CanvasInput.Void] or [CanvasInput.Orientation]; at Load, a blob whose
	// canvas says nothing or carries a word this build does not know. Field
	// construction DATA, which is what this sentinel is for; the capability
	// sentinels (ErrNoSight, ErrNoStanding) are for injected behaviour, not
	// world facts. Both messages name the declaration and the two are worded
	// apart, because "never declared" and "declared something I do not know"
	// send whoever reads them to different places — see [Void] and
	// voidFromData.
	ErrNoField = errors.New("no field")

	// ErrBadPlacement is returned when a placement or position is bad in
	// a way runtime spatial state can catch (not declaration-time
	// validation — that's ErrNoField and the region/edge sentinels): a room or entity
	// lookup miss, a position out of bounds or (hex) non-integral, a
	// a prop placed on a cell no region owns, or an actual
	// underlying spatial-package call (PlaceEntity, MoveEntity,
	// RegisterBoundary, RemoveEntity) failing. Only the LAST class wraps an
	// underlying spatial error — most call sites (moveMember's own
	// integrality check, Pump's snapshot lookups, Join's and Exit's own
	// lookups) reject before ever reaching spatial, with nothing beneath
	// ErrBadPlacement to wrap.
	//
	// It is also what a WALL answers with. Since rpg-toolkit#1106 the thing
	// that stops a step between two chambers is a movement-blocking boundary
	// on the canvas, refused by spatial's own MoveEntity — where it used to be
	// the absence of a doorway in a connection list, refused with a sentinel
	// of its own (ErrNoCrossing, deleted with the concept). "There is no way
	// through" and "that is not a cell you can stand on" stopped being
	// different kinds of answer when walls became geometry.
	//
	// VOID IS NOT FLOOR carries the same error: the canvas spans the field's
	// bounding box, so a cell can be on the map and still belong to no
	// authored chamber, and stepping or joining onto one is refused here.
	//
	// Also covers LoadEncounter's member and outcome-member checks, which
	// mirror NewEncounter's identical member checks and used to carry only
	// ErrInvalidData (#929 hardening round F) — plus a MISSING cell on either,
	// which has no Setup analogue for the same structural reason
	// RoomData.Origin's absence does not: a cell is a plain value in memory
	// and a pointer on the wire, so only Load can tell a blob that omitted it
	// (the room-local dialects from before #1068 and #1106) from one that
	// declared (0,0).
	ErrBadPlacement = errors.New("bad placement")

	// ErrRegionEmpty is returned when a region declares no cells
	// (rpg-project#256). A region IS its cells — the floor is nothing but
	// their union — so one with none is not a small room, it is a name with
	// nothing under it, and an author who wrote it meant to paint something.
	ErrRegionEmpty = errors.New("region has no cells")

	// ErrRegionOverlap is returned when a cell is claimed twice: by two
	// regions, twice by one (W2 — regions never overlap), by both a region
	// and [FieldInput.Scenery], or twice by the scenery. Ownership has to be
	// unique for [Encounter.RegionAt] to be an answer rather than a guess,
	// and a cell painted twice is the one defect the builder's repaint can
	// produce without noticing.
	//
	// Scenery is the same defect at one remove (rpg-project#360): "nobody
	// owns this cell" is an ANSWER, so a cell that is both a region's and
	// nobody's is a cell with two of them.
	ErrRegionOverlap = errors.New("regions overlap")

	// ErrRegionArchetypeMissing is returned when a region carries no
	// archetype. An archetype is a presentation ref the assets resolve —
	// "crypt", "cavern" — carried unread by this composition and NEVER a
	// mechanic (rpg-project#256 ruling: v1's archetype chose the party's
	// start, which is the #1033 trap). It is still REQUIRED: a region with no
	// archetype is a region the assets cannot dress, and there is no default.
	ErrRegionArchetypeMissing = errors.New("region has no archetype")

	// ErrRegionLightingMissing is returned when a region carries no lighting
	// block. Lighting is a per-region world fact carried through the
	// composition unread (how an intensity becomes obscurement is a rule and
	// lives in the rulebook), and it is REQUIRED for rpg-toolkit#1033's
	// reason: a nil meaning "full light" or "no light" would be this module
	// deciding a fact about a world it is not allowed to know.
	ErrRegionLightingMissing = errors.New("region has no lighting")

	// ErrEdgeNotAdjacent is returned when an authored edge — a wall in
	// [FieldInput.Walls] or a door's [DoorEdge] — joins two cells that are
	// not neighbours under the field's orientation. An edge is a crossing
	// between adjacent cells and nothing else; "which cells are adjacent" is
	// spatial's answer under the declared layout, never a pair of offsets to
	// hardcode, and the same [col,row] pair is adjacent under one
	// orientation and not the other.
	ErrEdgeNotAdjacent = errors.New("edge joins cells that are not adjacent")

	// ErrEdgeOffFloor is returned when a wall's endpoint is a cell that is not
	// floor — neither a region's nor [FieldInput.Scenery]'s. The envelope is
	// implied, never written: a crossing from floor into void is a crossing
	// nobody can make, and [Void] already says whether sight crosses it, so a
	// wall drawn along the field's rim has nothing to stand on and is refused
	// rather than silently dropped.
	//
	// SCENERY IS SOMETHING TO STAND ON (rpg-project#360). A wall may run
	// along a strip nobody walks; what it may not do is stand in the void.
	ErrEdgeOffFloor = errors.New("edge endpoint is not floor")

	// ErrDoorEdgeOffFloor is returned when a door's endpoint is a cell that is
	// not floor — ErrEdgeOffFloor's rule for a door (#880: a door hanging in
	// the void is a wall drawn across nothing), scenery included as floor.
	// Wrapped together with ErrBadDoor, so a caller may match either.
	ErrDoorEdgeOffFloor = errors.New("door edge endpoint is not floor")

	// ErrInBubble is returned when a verb requires its member NOT be in a
	// running bubble and they are: Form names a member already in a fight
	// (rejected, never silently merged — merging is a Merge-verb decision
	// that arrives with multiple bubbles), or Form is called while this
	// encounter's one allowed bubble is already running (#963 policy: one
	// bubble per encounter, so fights stay linear and the party stays
	// together).
	//
	// NOT Step's own refusal for a mid-fight member any more
	// (rpg-toolkit#1169): the active member of a bubble moves through
	// Step exactly as a free-roaming member does. This package does not
	// price or spend movement — there is no CapacityMovement use
	// anywhere in encounter. session.Move is the caller responsible for
	// paying before it calls Step; that pricing is a follow-up PR, not
	// this one. What a bubble member still cannot do is free-roam OUT of
	// the fight — Transfer still refuses ClockTurn for a member already
	// in one — and what a NON-active bubble member cannot do is move at
	// all, which is ErrNotActive's refusal, not this one.
	ErrInBubble = errors.New("already in a bubble")

	// ErrNoBubble is returned when a verb requires a running bubble and
	// finds none: Transfer names ClockTurn while no fight is running, or
	// Transfer-to-world, EndTurn, or Dissolve names a member who is on the
	// world clock — there is no bubble to leave, act in, or dissolve.
	ErrNoBubble = errors.New("no bubble")

	// ErrNotActive is returned when a verb requires its member to be the
	// CURRENT active member of their bubble and they are not: EndTurn
	// naming someone whose turn it is not, or (rpg-toolkit#1169) Step
	// asked to move a bubble member who is not the one the clock is
	// waiting on.
	//
	// TWO VERBS SHARE THIS REFUSAL, which is why it is named at all. Before
	// Step could ever produce it, EndTurn's identical rejection reached a
	// caller unnamed — play/clock's own ErrNotActive, surfacing through
	// this package's exported signatures with no translation (a real S2
	// gap, closed here rather than left for a second caller to rediscover).
	// The leaf sentinel does not cross this boundary any more than any
	// other leaf's does: this is the one a host matches on.
	ErrNotActive = errors.New("not the active member")

	// ErrBadClock is returned when TransferInput.To names neither
	// ClockWorld nor ClockTurn. Direction is load-bearing on a transfer —
	// inferring it from the member's current clock would make the verb a
	// toggle, and a toggle applied to stale state silently moves somebody
	// the WRONG way instead of failing.
	ErrBadClock = errors.New("unknown clock kind")

	// ErrInvalidData is returned when LoadEncounter rejects the input data.
	ErrInvalidData = errors.New("invalid encounter data")

	// ErrTrimmed is returned by Story when the requested resume point has
	// already been trimmed out of the retention window — the caller asked to
	// continue from a sequence the log no longer holds.
	//
	// This is a REJECTION, never a short answer. A caller resuming from a
	// known sequence is asserting "I already have everything below this";
	// silently returning only what survives would look identical to a
	// complete answer and would leave a permanent hole in that caller's
	// view of the story. Rejecting tells it to resync from scratch.
	//
	// AfterSeq == 0 never produces this error: zero means "I have nothing,
	// send what you have," which is always answerable. The distinction is
	// between a first load and a reconnect (#937).
	ErrTrimmed = errors.New("story range trimmed")

	// ErrNoInitiative indicates Setup was given no InitiativeRoller. Trigger
	// detection runs from first light, so every encounter needs one — refused
	// at construction rather than at the moment a fight would have started.
	ErrNoInitiative = errors.New("encounter: no initiative roller")

	// ErrNoStanding indicates Setup or Load was given no Standing capability.
	// The consult runs from first light, so every encounter needs one —
	// refused at construction rather than at the moment a body would have
	// started a fight.
	ErrNoStanding = errors.New("encounter: no standing capability")

	// ErrNoParticipation indicates Setup or Load kept the source-compatible
	// Standing field but supplied a concrete capability without Participation.
	// There is no legacy binary fallback and no default assessment.
	ErrNoParticipation = errors.New("encounter: no participation capability")

	// ErrNoSight indicates this module was not told, usably, how far somebody
	// can see. Three ways to earn it: Setup or Load was given no Sight
	// capability; the capability answered without covering a member it was
	// asked about; or it answered with a negative distance. All three are the
	// same defect seen from different sides — a sight range this module would
	// have to invent, which rpg-toolkit#1033 forbids it to do.
	ErrNoSight = errors.New("encounter: no sight capability")

	// ErrNoCheckResolver indicates Setup or Load was given a field carrying
	// concealed structure and no CheckResolver capability. A concealed door
	// exists to be searched for, and this module refuses to roll the find
	// itself exactly as it refuses to know who is standing or how far anyone
	// sees (rpg-toolkit#1033). Refused at the door; never guarded at the use
	// site, and never defaulted. A field with NO concealment needs no
	// resolver, and supplying one there is harmless and unread.
	ErrNoCheckResolver = errors.New("encounter: no check resolver capability")

	// ErrNoWitness indicates Setup or Load was given a field carrying
	// concealed structure and no Witness capability. Perceiving a concealed
	// door standing open is what reveals it, and who perceives is the host's
	// light-and-sight truth — a rule this module may not invent
	// (rpg-toolkit#1033). Same door, same law as ErrNoCheckResolver.
	ErrNoWitness = errors.New("encounter: no witness capability")

	// ErrElsewhere indicates Search named a region the searcher does not
	// stand in — v1's rule is that presence is the host's truth and a member
	// sweeps the floor under their own feet. DELIBERATELY the same answer
	// for a region that does not exist at all: a distinct no-such-region
	// refusal would let a guessed ID probe for hidden rooms, which is the
	// probe law's concern arriving at the region vocabulary.
	ErrElsewhere = errors.New("encounter: not standing in that region")

	// ErrNoTurnDriver indicates Setup or Load was given no TurnDriver
	// capability. A member with no player can land on a fight's clock — a
	// turn ending, or a fight forming with an unplayed member first in
	// initiative — and this module refuses to guess what they do, exactly as
	// it refuses to guess who is standing or how far anyone sees
	// (rpg-toolkit#1033, rpg-toolkit#1162). Never defaulted: see ADR-0043 for
	// why this differs from Decider, which IS optional.
	ErrNoTurnDriver = errors.New("encounter: no turn driver capability")

	// ErrNoPlayerInBubble indicates a bubble was asked to drive its unplayed
	// members forward and found no KindPlayer member in it at all.
	//
	// Defensive rather than reachable through any documented path: a bubble
	// only forms on contact between a player and something else (rpg-
	// toolkit#964), so a player-free fight should not exist. Refused loudly
	// here rather than looping driveMonsterTurns forever or silently ending
	// every member's turn with nobody left to hand the clock to, in case that
	// invariant is ever broken elsewhere.
	ErrNoPlayerInBubble = errors.New("encounter: bubble has no player to end on")

	// ErrBadTurnOutcome indicates a TurnDriver returned a TurnIntent this
	// version of the module does not recognise — a value outside the sealed
	// Pass/Attack/Move vocabulary.
	//
	// Unreachable from any driver outside this package today: TurnIntent is
	// sealed on an unexported method, so nothing outside encounter can
	// construct a value satisfying it other than these three. It exists for
	// the same reason ErrBadRepository exists beside ErrNotFound — a defect
	// this module can detect should say so by name rather than proceed on a
	// value it does not understand, the day a fourth TurnIntent case is added
	// here and some call site is not updated to handle it.
	ErrBadTurnOutcome = errors.New("encounter: turn driver returned an unrecognised intent")

	// ErrNoStriker indicates Setup or Load was given no Striker capability.
	// A member with no player can be handed an Attack intent the moment a
	// fight forms — a turn ending, or a fight forming with an unplayed
	// member first in initiative — and this module refuses to guess how a
	// strike resolves, exactly as it refuses to guess who is standing, how
	// far anyone sees, or what an unplayed member does at all
	// (rpg-toolkit#1033, rpg-toolkit#1162, rpg-project#254). Never
	// defaulted: a Striker that silently did nothing would let a monster's
	// turn end without the swing its driver decided on ever landing.
	ErrNoStriker = errors.New("encounter: no striker capability")

	// ErrRefusingStriker is what [RefusingStriker.Strike] always returns:
	// a driven turn reached a Striker built for a construction-only world
	// (rpg-api's placement probes, a template's own acceptance test) — a
	// host bug, since nothing should ever call EndTurn/form against such a
	// world, not a legal outcome any caller is meant to recover from.
	ErrRefusingStriker = errors.New("encounter: RefusingStriker: a driven turn reached a construction-only world")

	// ErrNoAnnouncer is what both constructors return when no Announcer was
	// supplied. Every clock advance crosses boundaries — a turn ending, a
	// fight forming — and this module refuses to guess what one MEANS to a
	// condition, exactly as it refuses to guess how a strike resolves or
	// what an unplayed member does. Never defaulted: an Announcer that
	// silently did nothing would let every turn-scoped condition live
	// forever, which is precisely the bug this capability closed
	// (rpg-project#294).
	ErrNoAnnouncer = errors.New("encounter: no announcer capability")

	// ErrRefusingAnnouncer is what [RefusingAnnouncer.Announce] always
	// returns: a clock advanced on a world built only to be inspected. A
	// host bug — nothing should reach EndTurn or form against such a
	// world — not a legal outcome any caller is meant to recover from.
	ErrRefusingAnnouncer = errors.New("encounter: RefusingAnnouncer: a clock advanced on a construction-only world")

	// ErrNoDissolveMilestone is a leaf that reported dissolving a bubble and
	// then did not say so in its milestones. An INVARIANT of play/clock, not a
	// caller mistake and not recoverable: combatEndBoundaries reads the round a
	// fight ended on out of that milestone, and there is no honest value to
	// substitute for it.
	//
	// Loud rather than empty on purpose. Returning no boundaries would end
	// fights that expire nothing, which is indistinguishable from the state
	// combat end was already in before rpg-project#295 and is exactly how it
	// stayed there.
	ErrNoDissolveMilestone = errors.New("encounter: bubble dissolved without a dissolved milestone")

	// ErrBadIntent indicates a TurnDriver returned a syntactically valid
	// TurnIntent this composition cannot execute: an Attack naming a target
	// that is not currently Seen, not Standing, or out of the named
	// action's reach; an Attack naming an action Ref that is not among the
	// member's own [MonsterView.Actions]; or a Move whose path this member
	// cannot afford against its remaining movement budget.
	//
	// NOT A DRIVER MALFUNCTION (compare the plain Go error [TurnDriver.Act]
	// itself can return, which aborts the caller's whole verb). A bad
	// intent is a bad DECISION, not a broken driver: it simply ends this
	// member's turn exactly as [Pass] would, and the caller's own verb
	// (EndTurn, form) still succeeds — see [Encounter.driveMonsterTurns]'s
	// own doc for why this asymmetry is deliberate (rpg-project#254).
	ErrBadIntent = errors.New("encounter: turn driver returned an intent this composition cannot execute")

	// ErrReadOnly indicates an attempt to write to the map through the
	// read-only view [Encounter.Canvas] hands out.
	ErrReadOnly = errors.New("encounter: the canvas is read-only")

	// ErrNoDoor is returned when a verb names a door the field does not have,
	// or names none at all (rpg-toolkit#1123).
	//
	// Separate from ErrBadDoor the way ErrNoRegion is separate from
	// ErrNoField: this is a caller asking about something that is not there,
	// where ErrBadDoor is a field that could not be built.
	ErrNoDoor = errors.New("no such door")

	// ErrBadDoor is returned when a door cannot be part of a field
	// (rpg-toolkit#1123): an empty or duplicated ID, no edges at all, a
	// missing state (required, never defaulted — see [DoorState]), a lock
	// nothing has to beat, an edge whose cells are the same or not adjacent or
	// not integral, an edge endpoint that is not floor (#880: a door hanging
	// in the void is a wall drawn across nothing), the same crossing named
	// twice by one door, one crossing claimed by two doors — which could not
	// then have one state, which is the whole design — or a crossing where a
	// room already drew a wall.
	//
	// Also what a verb answers when a door is asked to do something its state
	// has already done: opening an open door, closing a shut one, or unlocking
	// one that is not locked. The honest answer to asking for something that
	// has happened is not silent success — [Encounter.Dissolve]'s ErrNoBubble
	// makes the same call.
	ErrBadDoor = errors.New("bad door")

	// ErrLocked is returned when [Encounter.OpenDoor] is asked to open a
	// locked door. The message names the door and the DC that has to be
	// beaten; [Encounter.Unlock] is the way through.
	//
	// A refusal rather than the old stack's silent success. Over there
	// OpenDoor deliberately does NOT gate on Locked, leaving the gate to the
	// orchestrator (encounter/data.go's DoorData doc) — which is the
	// looks-like-it-worked shape this composition has spent several slices
	// deleting.
	ErrLocked = errors.New("door is locked")

	// ErrDoorShut is returned when a step's travel crosses a door that is
	// merely closed — shut but not locked (a locked one refuses with
	// [ErrLocked], naming its DC). Distinct from [ErrBadPlacement] so a
	// fiction beat and a client bug stop sharing one sentinel
	// (rpg-toolkit#1135): the cell is real and the way is shut, and a
	// caller told "bad placement" for a shut door goes looking for
	// arithmetic that is fine.
	ErrDoorShut = errors.New("door is shut")

	// ErrOutOfRange is returned when Interact's target stands farther than
	// the configured range (default: adjacent, one cell) from the actor.
	ErrOutOfRange = errors.New("encounter: target out of range")

	// ErrNotVisible is returned when Interact's target is not in the
	// actor's current sight — a target once seen but not seen now refuses
	// identically to one never seen at all, the same "current, not held"
	// rule contactBetween and unawareOfOpposition already apply.
	ErrNotVisible = errors.New("encounter: target not visible")
)
