// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SightPayload is the legacy untagged known-location shape retained for source
// compatibility. New encounter testimony is encoded with LocationKnowledge.
//
// It describes dungeon-absolute coordinates and is readable by
// DecodeLocationPayload as the legacy known form.
type SightPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// declaredEnding pairs an ending key with its trigger, in Setup order, plus
// the canvas cell a positional trigger was compiled to.
//
// The cell is computed once, at construction, from the authored room and the
// room-local position the trigger declares (rpg-toolkit#1106). Before the field
// became one canvas, an arrival was compared room-then-cell, in whatever frame
// the verb happened to hold; there is one frame now, so an ending is one cell,
// and the comparison is an equality. Meaningless — and never read — for a
// TriggerExternal, which carries no geometry.
type declaredEnding struct {
	key     string
	trigger Trigger
	cell    spatial.Position
}

// compileEndings resolves each declared ending's authored target into the
// single canvas cell an arrival is compared against. Every target named here
// has already been checked to be floor by validateEndingTriggers, which both
// construction seams run before this.
//
// Through the field's one conversion rather than an addition of its own
// (rpg-toolkit#1127). An ending is compared to a member's live cell by
// EQUALITY, so a second projection here would not be merely untidy: it lands
// the sanctuary tile somewhere no member can ever stand, and the encounter
// simply never ends — the liveness hole ErrNoEnding exists to prevent, arrived
// at from the inside.
func compileEndings(endings []EndingInput, f *field) []declaredEnding {
	out := make([]declaredEnding, 0, len(endings))
	for _, ei := range endings {
		de := declaredEnding{key: ei.Key, trigger: ei.Trigger}
		if t, ok := ei.Trigger.(TriggerReachedPosition); ok {
			de.cell = f.cellAt(t.Position)
		}
		out = append(out, de)
	}
	return out
}

// Encounter is the aggregate encounter composition: members, field, clock,
// intel, and record. Construct via NewEncounter or LoadEncounter; the zero
// value is unusable.
type Encounter struct {
	// canvas is THE MAP: one spatial room spanning the whole dungeon, in
	// dungeon-absolute cells, with every authored wall registered on it as an
	// absolute boundary edge (rpg-toolkit#1106).
	//
	// One, not N-plus-an-orchestrator. The composition AUTHORS regions —
	// named cell sets — and this is what they compile into (rpg-project#256).
	canvas *canvasRoom

	// field is the compiled field: the authored regions, props and walls
	// (deep-copied, what ToData writes back out), the declarations, and the
	// owner map every floor question is answered from. The canvas above was
	// built from it and holds the same pointer — see [canvasRoom].
	field *field

	// doors are every door in the field, sorted by ID (C8). A door's edges are
	// construction truth; its STATE is the one thing here a verb changes
	// mid-scene, and it is held once for however many edges the door has
	// (rpg-toolkit#1123).
	doors []*doorRecord

	// doorsByID indexes doors by name. The SAME pointers, not a second copy —
	// a door's state has one home however it is reached.
	doorsByID map[DoorID]*doorRecord

	clock *clock.Tick
	// bubbles are the localized initiative bubbles currently running. Zero or
	// more: a bubble exists only while a fight does, and an encounter with no
	// fight has none.
	//
	// A slice rather than a single pointer even though policy allows at most one
	// today, because a slice grows to N additively and a pointer does not. And
	// deliberately WITHOUT identity: a bubble is never named, it is always
	// reached through a member, which R6 makes a total function ("an entity
	// belongs to at most one clock"). Inventing an ID would create a second
	// thing to keep true.
	bubbles     []*clock.Turn
	intelLog    *intel.Intel
	story       *record.Log
	members     map[MemberID]*memberRecord
	everMembers map[MemberID]bool // Track all members who have ever joined (for Story access)
	deciders    map[MemberID]Decider

	// initiative rolls the order a bubble forms with. Nil until Setup is given
	// one; trigger detection refuses to start a fight without it rather than
	// dropping the fight silently.
	initiative InitiativeRoller
	// standing retains the source-compatible constructor capability shape.
	// The same concrete value is asserted once at construction and stored as
	// participation below; no play path falls back to the binary answer.
	standing Standing

	// participation is the richer half of standing. Required at both
	// constructors and never defaulted; see [StandingWithParticipation].
	participation Participation

	// sight reports how far each member can see. Required at both constructors
	// — see [Sight] for why it is asked at every refresh rather than held, and
	// why there is no default.
	sight Sight

	// turnDriver decides what a member with no player does when the clock
	// lands on their turn. Required at both constructors, for the same reason
	// standing and sight are, and — unlike deciders — never optional; see
	// [TurnDriver] and ADR-0043.
	turnDriver TurnDriver

	// striker resolves and records an [Attack] intent a TurnDriver returns.
	// Required at both constructors for the same reason turnDriver is; see
	// [Striker].
	striker Striker

	// announcer publishes the temporal boundaries a clock advance crossed.
	// Required at both constructors for the same reason striker is; see
	// [Announcer], whose doc explains why it is a capability rather than a
	// return value.
	announcer Announcer

	// world is the run's concealment knowledge — seeded from the field's
	// concealed structure at construction, its facts persisted on
	// EncounterData.World (rpg-toolkit#1371). NIL FOR A FIELD WITH NO
	// CONCEALMENT, deliberately: a plain dungeon builds no world machinery
	// at all, which is what makes zero-behavior-change structural rather
	// than promised.
	world *encounterWorld

	// holdings is WHO HAS WHAT: the run's append-only journal of holdings,
	// takings and drops (rpg-project#368, design §5). ALWAYS PRESENT, unlike
	// world above — holdings are not about concealment, and a dungeon with
	// no secret anywhere can still have a takeable idol on a plinth. An
	// encounter where nobody holds anything writes no facts and no bytes.
	holdings *holdings

	// checkResolver rolls a find check when a member searches. Required at
	// both constructors exactly when the field carries concealment; unread
	// otherwise. See [CheckResolver].
	checkResolver CheckResolver

	// witness answers who currently perceives an open concealed door.
	// Required under the same rule as checkResolver. See [Witness].
	witness Witness

	// driving is true for the duration of ONE driveMonsterTurns call, at
	// any depth of Go call stack — runtime state, never persisted (there is
	// no stack mid-verb for ToData to capture, and none is needed: a fresh
	// load always starts false).
	//
	// The load-bearing half of rpg-toolkit#1207's fix, alongside Transfer's
	// own active-member guard (see its own doc). A driven turn's own Strike
	// can splice a downed teammate out through Transfer, whose "the
	// departing member may have been active" rescue (rpg-toolkit#1162) is a
	// real need for a genuinely stalled slot — but nothing before this flag
	// stopped that rescue firing AGAIN for the SAME still-mid-turn monster
	// its own Strike call is nested inside, handing it a second,
	// undocumented turn under a second, fresh budget.
	//
	// driveMonsterTurns is the single owner of driving a bubble forward: a
	// driven turn runs to completion under one budget, and nothing that
	// happens inside it starts another. A nested call reaching it while one
	// is already running on this *Encounter is therefore a no-op — the
	// outer call already owns it and will finish it; see
	// driveMonsterTurns's own doc for the check itself.
	driving bool

	// endings holds declared endings in Setup order. Evaluation is
	// deterministic (law C8), but NOT globally "first-declared-wins":
	// for a single action (Step, Join) declaration order is
	// the only axis, so the first matching declared ending does win.
	// Pump can execute several monsters' actions in one tick, and there
	// evaluation walks them in DECISION order first — the action
	// decided earliest wins regardless of which of its matching endings
	// was declared later; declaration order is only the tiebreak within
	// one action's own scan. See Pump's ending-evaluation loop.
	endings []declaredEnding
	outcome *Outcome
	// retention is the story-beat window (see DefaultRetention). It persists
	// with the encounter so a reload keeps the policy it was built with.
	retention int
	// logFloor is the lowest Seq the story log still holds — everything below it
	// has been trimmed. Zero means nothing has been trimmed yet.
	//
	// Runtime state, deliberately NOT persisted: it is derived from the log
	// itself at load (logFloorOf), so it cannot drift out of agreement with the
	// entries it describes. A persisted copy could disagree with them after a
	// hand-edited blob and would then reject Story queries the log could
	// actually answer.
	logFloor uint64
}

// isIntegralHexCell reports whether pos names a whole axial cell —
// spatial's implicit integer-cube contract, upheld at this composition's
// boundary until tools/spatial#926 enforces it at ingress. AxialHexGrid
// bounds-checks Position.X/Y but does not integrality-check them, and all of
// its cube math truncates, so a fractional position like (0.5, 0.5) would
// otherwise persist as a distinct position that behaves exactly like (0,0) —
// an invisible collision with an unrelated, legitimately-placed cell.
//
// Asked wherever an ABSOLUTE cell first enters from a caller: Move's target
// (moveMember), a joiner's arrival cell (Join), a door's edges
// (validateDoorInputs), and a persisted member or outcome cell (Load).
// Authored offset pairs go through isAuthoredCell instead, which also bounds
// them.
func isIntegralHexCell(pos spatial.Position) bool {
	return pos.X == math.Trunc(pos.X) && pos.Y == math.Trunc(pos.Y)
}

// DefaultRetention is the number of story beats an encounter keeps when
// SetupInput.Retention is zero.
//
// Deliberately small. Under the event-stream contract the log is the truth and
// a live stream is only an optimisation over it: a client that misses beats
// notices a gap in Seq and re-queries Story from its last known sequence, and
// if it has been gone longer than the window it must resync from scratch
// instead. A generous window would make that resync path almost-never-taken and
// therefore almost-never-tested until a real player's connection dropped. A
// small one makes resync the ordinary case, so the expensive branch is the
// well-trodden one and the cheap delta is the optimisation rather than the
// assumption (#937).
//
// The window is a multiplayer-reconnect decision, not a storage one: it answers
// "how long can a client be gone and still rejoin cheaply," and the storage cost
// falls out of that rather than driving it.
const DefaultRetention = 32

// partyDefeatedEnding is the stable composition-owned outcome key used when a
// supplied participation assessment says the party is defeated.
const partyDefeatedEnding = "party_defeated"

// RetentionUnbounded disables trimming entirely. Appropriate for
// verified-transcript scenes, which assert on the story itself rather than on
// the retention policy, and for any caller that genuinely needs the whole
// history in the blob.
const RetentionUnbounded = -1

// normalizeRetention maps a caller-supplied retention setting onto the value the
// encounter actually uses: zero (the unset zero value) selects DefaultRetention,
// and any negative value means unbounded. Negatives are folded to
// RetentionUnbounded rather than rejected because every negative expresses the
// same intent and there is no defect to report.
func normalizeRetention(r int) int {
	switch {
	case r == 0:
		return DefaultRetention
	case r < 0:
		return RetentionUnbounded
	default:
		return r
	}
}

// logFloorOf derives the lowest Seq a persisted log still holds.
//
// Derived rather than persisted so it cannot disagree with the entries it
// describes: a stored floor could be edited into conflict with the log body, and
// would then reject Story queries the log is perfectly able to answer.
//
// Three cases. A log with entries floors at its smallest Seq — scanned rather
// than read from index zero, because a hand-edited blob is not obliged to be in
// order and the trust boundary is here. An empty log that has already assigned
// sequences was trimmed to nothing and floors at NextSeq: every Seq below it is
// genuinely gone. A never-appended log floors at zero, which exempts nothing,
// because nothing has been lost.
func logFloorOf(data record.LogData) uint64 {
	if len(data.Entries) == 0 {
		if data.NextSeq > 1 {
			return data.NextSeq
		}
		return 0
	}
	floor := data.Entries[0].Seq
	for _, entry := range data.Entries[1:] {
		if entry.Seq < floor {
			floor = entry.Seq
		}
	}
	return floor
}

// appendBeat appends one story beat. Every beat in the composition goes
// through here rather than calling story.Append directly, so "all beats flow
// through one place" stays true and any future per-append concern has a
// single seam that cannot rot as verbs are added.
//
// Retention is deliberately NOT enforced here (#1381). A verb that mints more
// beats than the window would trim its own earliest beats mid-verb — entries
// no reader has been handed yet — so the live log holds everything appended
// since load and the window is enforced at the storage boundary, in ToData.
func (e *Encounter) appendBeat(in *record.AppendInput) (*record.AppendOutput, error) {
	return e.story.Append(in)
}

// NextStorySeq returns the global sequence the next successful story append
// will use. It is a read: it does not reserve, increment, or append anything.
// Encounter ingress is single-command in v1; this method makes no promise for a
// future host that allows concurrent writers between this read and Record.
func (e *Encounter) NextStorySeq() (uint64, error) {
	next, err := e.story.NextSeq()
	if err != nil {
		return 0, fmt.Errorf("next story seq: %w", err)
	}
	return next, nil
}

// enforceRetention trims the story log down to the retention window and advances
// logFloor to match. Called from ToData — the storage boundary — and nowhere
// else (#1381): retention is a fact about what is WRITTEN, applied at the one
// place writing happens, so it is structurally unable to touch a verb's own
// beats before that verb has returned them.
//
// No-op when unbounded, and no-op while the log is still shorter than the window
// — TrimBefore treats a bound at or below the oldest retained Seq as a no-op
// anyway, but returning early keeps logFloor from being written on every save.
func (e *Encounter) enforceRetention() error {
	if e.retention == RetentionUnbounded {
		return nil
	}

	nextSeq, err := e.story.NextSeq()
	if err != nil {
		return fmt.Errorf("retention next seq: %w", err)
	}

	// Seq is 1-based and gapless, so after N appends nextSeq == N+1 and the log
	// holds [1, N]. Retaining `retention` entries means dropping everything
	// below nextSeq-retention. The subtraction is guarded because nextSeq <=
	// window means the log has not yet reached the window at all, and computing
	// the floor anyway would wrap on uint64.
	window := uint64(e.retention)
	if nextSeq <= window {
		return nil
	}
	floor := nextSeq - window

	// Skip when the computed floor is at or below what the log already starts
	// at — there is nothing there to drop.
	//
	// Without this the log would be trimmed the moment it reached exactly the
	// window: floor would come out as the oldest retained Seq, TrimBefore would
	// do nothing, and logFloor would still be advanced to a value describing a
	// trim that never happened. Harmless today, because a floor equal to the
	// oldest entry rejects nothing that the log can still serve — which is
	// exactly why no test in this package can distinguish the two versions. It
	// is fixed anyway: the comment above claims the early return keeps logFloor
	// from being written on every save, and code that contradicts its own
	// documentation is a trap for whoever next reasons about the floor
	// (Copilot, PR #939).
	//
	// oldest is the lowest Seq the log currently holds: logFloor once anything
	// has been trimmed, and 1 before that, since the log's first assigned Seq
	// is 1 rather than 0.
	oldest := e.logFloor
	if oldest < 1 {
		oldest = 1
	}
	if floor <= oldest {
		return nil
	}

	if _, err := e.story.TrimBefore(&record.TrimBeforeInput{Seq: floor}); err != nil {
		return fmt.Errorf("retention trim: %w", err)
	}
	e.logFloor = floor
	return nil
}

// validateEndingTriggers rejects a TriggerReachedPosition ending whose target
// is malformed or is not floor: an ending that names a cell nobody can reach
// can never fire — "an encounter that cannot end is a liveness hole"
// (ErrNoEnding's doc comment) applies to a single dead ending exactly as it
// does to zero endings (#929 T3 Opus round F5). TriggerExternal endings carry
// no spatial data and are skipped.
//
// Checked identically at Setup and Load, against the compiled field, with no
// verb prefix — each caller wraps its own at the call site.
func validateEndingTriggers(f *field, endings []EndingInput) error {
	for _, ei := range endings {
		// A MemberDown ending must name a member — see TriggerMemberDown's
		// doc for why empty is refused rather than defaulted.
		if md, ok := ei.Trigger.(TriggerMemberDown); ok && md.Member == "" {
			return fmt.Errorf("ending %q names no member: %w", ei.Key, ErrNoEnding)
		}
		// An ExitedHolding ending must name an exit this field declares and
		// a prop it declares TAKEABLE. Either one missing is an ending that
		// can never fire — the same liveness hole an unreachable trigger
		// cell is, and refused here for the same reason
		// ([TriggerExitedHolding]).
		if eh, ok := ei.Trigger.(TriggerExitedHolding); ok {
			if _, declared := f.exitCells[eh.Exit]; !declared {
				return fmt.Errorf("ending %q names exit %q, which this field does not declare: %w",
					ei.Key, eh.Exit, ErrNoEnding)
			}
			if eh.Item == "" {
				return fmt.Errorf("ending %q names no item to be holding: %w", ei.Key, ErrNoEnding)
			}
			if _, takeable := f.takeable[eh.Item]; !takeable {
				return fmt.Errorf("ending %q waits for %q to be held, and no prop with that id is takeable: %w",
					ei.Key, eh.Item, ErrNoEnding)
			}
		}
		trigger, ok := ei.Trigger.(TriggerReachedPosition)
		if !ok {
			continue
		}
		if !isAuthoredCell(trigger.Position) {
			return fmt.Errorf("ending %q trigger position (%g,%g) is not an integral cell: %w",
				ei.Key, trigger.Position.X, trigger.Position.Y, ErrNoEnding)
		}
		if cell := f.cellAt(trigger.Position); !f.isStandable(cell) {
			return fmt.Errorf("ending %q trigger position [%g,%g] %s: %w",
				ei.Key, trigger.Position.X, trigger.Position.Y, f.notStandable(cell), ErrNoEnding)
		}
	}
	return nil
}

// NewEncounter constructs and initializes an encounter from SetupInput.
// Validation order (first failure wins, R5 atomicity): nil input, no
// endings, empty-or-reserved ending key, duplicate ending key, empty member
// ID, duplicate member IDs, a player member carrying a Decider (design law
// C2), negative member facts, then the field (compileField: the canvas's
// declarations, region defects, props, walls), doors (validateDoorInputs),
// member seats (integral, on floor), ending trigger validity, spatial
// placement errors.
func NewEncounter(in *SetupInput) (*Encounter, error) {
	// Validation order: nil, no rooms, no endings, reserved ending, empty ID, duplicates
	if in == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNilInput)
	}

	if len(in.Endings) == 0 {
		return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
	}

	// Required, because construction is total (S8): trigger detection runs
	// from first light onward, so an encounter that can hold players and
	// monsters can start a fight before its caller does anything, and a fight
	// it cannot order is a misconfiguration. Refusing here rather than
	// mid-fight is the difference between a bug report and a bug — the
	// alternative, discovering it when two members finally see each other,
	// fails at the least convenient moment and looks like a rules bug.
	if in.Initiative == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoInitiative)
	}

	// Required for the same reason, one layer down: the standing consult runs
	// from first light too — a scene can open with a body already on the floor
	// — and an encounter that cannot ask would start fights with corpses and
	// walk them around the map. Refused at the door; never guarded at the use
	// site, and never defaulted (rpg-toolkit#1033).
	if in.Standing == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoStanding)
	}
	standingWithParticipation, ok := in.Standing.(StandingWithParticipation)
	if !ok {
		return nil, fmt.Errorf("newencounter: Standing does not implement Participation: %w", ErrNoParticipation)
	}

	// Required for the third time, at the same door and by the same law: the
	// sight consult runs at every refresh including first light, so an
	// encounter that cannot ask how far its members can see cannot build a
	// percept at all. Never defaulted — a number meaning "everyone sees this
	// far" is a rule 5e does not have, since sight is per-creature and
	// per-light-source (rpg-toolkit#1033, rpg-toolkit#1111).
	if in.Sight == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoSight)
	}

	// Required for the same reason again: a fight can form at first light
	// with an unplayed member first in the rolled order, so an encounter that
	// cannot answer "what does this member do" would stall before its caller
	// does anything (rpg-toolkit#1162). Never defaulted — see ADR-0043 for
	// why this differs from Decider, which is optional per member.
	if in.TurnDriver == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoTurnDriver)
	}

	// Required for the same reason again, one seam over: a TurnDriver can
	// decide to attack the moment a fight forms, so an encounter that
	// cannot resolve that swing would stall on it or silently drop it
	// (rpg-project#254). Never defaulted — see [Striker]'s own doc.
	if in.Striker == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoStriker)
	}

	// And once more, one seam further on: a fight forming starts round 1 and
	// somebody's first turn, so an encounter that cannot announce a boundary
	// would let every turn-scoped condition in it live forever — silently,
	// which is how this went unnoticed for months. Never defaulted — see
	// [Announcer]'s own doc.
	if in.Announcer == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNoAnnouncer)
	}

	// Check ending keys: empty/reserved, and duplicate (#929 hardening
	// round E — two endings sharing a key both used to load; End scans
	// in declaration order, so a reached_position twin declared FIRST
	// permanently shadowed a same-keyed external ending declared after
	// it, the exact liveness hole ErrNoEnding's doc comment already
	// names for zero endings and unreachable triggers, now closed for
	// this class too).
	seenEndingKeys := make(map[string]bool, len(in.Endings))
	for _, ending := range in.Endings {
		if ending.Key == "" || ending.Key == "abandoned" || ending.Key == partyDefeatedEnding {
			return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
		}
		if seenEndingKeys[ending.Key] {
			return nil, fmt.Errorf("newencounter: duplicate ending %q: %w", ending.Key, ErrNoEnding)
		}
		seenEndingKeys[ending.Key] = true
	}

	// Check member IDs: empty or duplicate; validate deciders
	seenIDs := make(map[MemberID]bool)
	for _, m := range in.Members {
		if m.ID == "" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("newencounter: duplicate member %s: %w", m.ID, ErrNoMember)
		}
		seenIDs[m.ID] = true

		// Players cannot carry deciders (design law C2)
		if m.Kind == KindPlayer && m.Decider != nil {
			return nil, fmt.Errorf("newencounter: player %s cannot carry a decider: %w", m.ID, ErrNoMember)
		}

		// Nor can a world NPC (rpg-toolkit#1404, design.md N4): a non-combatant
		// never acts on its own turn, and a decider would imply it does.
		if m.Kind == KindWorld && m.Decider != nil {
			return nil, fmt.Errorf("newencounter: world npc %s cannot carry a decider: %w", m.ID, ErrNoMember)
		}

		// SpeedFeet, SightFeet and each action's RangeFeet are feet-
		// denominated facts CellsFromFeet divides by FeetPerCell — a
		// negative one is not a shorter distance, it is a caller defect
		// (Copilot, PR #1187), and would otherwise produce a nonsense
		// budget or reach at the exact moment a monster's turn needs one.
		if err := validateMemberFacts(m.ID, m.SpeedFeet, m.SightFeet, m.Actions); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
	}

	// Compile the field: the canvas's two declarations, the regions that make
	// the floor, the props and walls on it — every rule about what a field
	// may be, stated once and shared with Load (compileField).
	f, err := compileField(in.Field)
	if err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Check doors: names, states, and edges that are real crossings on real
	// floor and belong to exactly one door (rpg-toolkit#1123).
	if err = validateDoorInputs(f, in.Field.Doors); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// The two concealment capabilities, required exactly when the field
	// carries concealed structure (rpg-toolkit#1371) — the same
	// supplied-never-defaulted law as the four above, scoped to the fields
	// that consult them: a plain dungeon needs neither, and a concealed one
	// refused here is a bug report instead of a secret nobody can ever
	// find. AFTER the field and door validation on purpose: a malformed
	// door is the author's earlier mistake, and refusing it as a missing
	// capability would send them to the wrong seam.
	if fieldHasConcealment(in.Field.Regions, in.Field.Doors) {
		if in.CheckResolver == nil {
			return nil, fmt.Errorf("newencounter: %w", ErrNoCheckResolver)
		}
		if in.Witness == nil {
			return nil, fmt.Errorf("newencounter: %w", ErrNoWitness)
		}
	}

	// Every authored seat is a whole offset cell that some region owns. Asked
	// here, before anything is built (R5), and named as itself rather than
	// as a placement spatial refused.
	for _, mi := range in.Members {
		if !isAuthoredCell(mi.Position) {
			return nil, fmt.Errorf("newencounter: member %q position (%g,%g) is not an integral cell: %w",
				mi.ID, mi.Position.X, mi.Position.Y, ErrBadPlacement)
		}
		if cell := f.cellAt(mi.Position); !f.isStandable(cell) {
			return nil, fmt.Errorf("newencounter: member %q position [%g,%g] %s: %w",
				mi.ID, mi.Position.X, mi.Position.Y, f.notStandable(cell), ErrBadPlacement)
		}
	}

	// A knowledge link must name a door this field declares (design P1).
	// Refused here, before anything is built (R5): a link to nothing is a
	// secret the author thinks they placed and did not. Whether that door is
	// CONCEALED is deliberately not asked — knowing an ordinary door is
	// inert, not an error ([MemberInput.Knows]).
	declaredDoors := make(map[DoorID]bool, len(in.Field.Doors))
	for _, d := range in.Field.Doors {
		declaredDoors[d.ID] = true
	}
	for _, mi := range in.Members {
		for _, id := range mi.Knows {
			if !declaredDoors[id] {
				return nil, fmt.Errorf("newencounter: member %q knows door %q: %w", mi.ID, id, ErrNoDoor)
			}
		}
	}

	// A TriggerReachedPosition ending must name a reachable cell (#929 T3
	// Opus round F5) — see validateEndingTriggers.
	if err = validateEndingTriggers(f, in.Endings); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// After validation passes, construct (R5: no observable state until success)
	e := &Encounter{
		members:       make(map[MemberID]*memberRecord),
		everMembers:   make(map[MemberID]bool),
		deciders:      make(map[MemberID]Decider),
		field:         f,
		initiative:    in.Initiative,
		standing:      standingWithParticipation,
		participation: standingWithParticipation,
		sight:         in.Sight,
		turnDriver:    in.TurnDriver,
		striker:       in.Striker,
		announcer:     in.Announcer,
		endings:       nil,
		retention:     normalizeRetention(in.Retention),
	}
	e.doors, e.doorsByID = doorRecordsFrom(in.Field.Doors)

	// The holdings journal, always. It starts empty and stays empty for a
	// field nobody carries anything in, which is what keeps such a field's
	// blob byte-identical to what it was before holdings existed.
	e.holdings = newHoldings()

	// The world, seeded from the concealed structure — and only then: a
	// field with none leaves e.world nil and every concealment path a
	// no-op (rpg-toolkit#1371).
	if fieldHasConcealment(in.Field.Regions, in.Field.Doors) {
		e.checkResolver = in.CheckResolver
		e.witness = in.Witness
		e.world, err = newEncounterWorld(f, e.doors)
		if err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
	}

	// Build clock and intel
	e.clock, err = clock.NewTick()
	if err != nil {
		return nil, fmt.Errorf("newencounter clock: %w", err)
	}

	e.intelLog, err = intel.NewIntel()
	if err != nil {
		return nil, fmt.Errorf("newencounter intel: %w", err)
	}

	e.story, err = record.NewLog()
	if err != nil {
		return nil, fmt.Errorf("newencounter story: %w", err)
	}

	// Compile the field into the one canvas this encounter runs on.
	e.canvas, err = f.compileCanvas(e.doors)
	if err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Place members and collect them
	memberIDs := make([]MemberID, 0, len(in.Members))
	for _, mi := range in.Members {
		memberIDs = append(memberIDs, mi.ID)

		entity := &memberEntity{
			id:             string(mi.ID),
			kind:           mi.Kind,
			blocksMovement: mi.BlocksMovement,
		}

		// Authored offset at the seat, absolute on the canvas: converted
		// through the field's one conversion, exactly as every region cell
		// was. Floor was checked above.
		if err = e.canvas.PlaceEntity(entity, f.cellAt(mi.Position)); err != nil {
			return nil, fmt.Errorf("newencounter member placement: %w: %w", ErrBadPlacement, err)
		}

		member := &memberRecord{
			ID:             mi.ID,
			Kind:           mi.Kind,
			Name:           mi.Name,
			SpeedFeet:      mi.SpeedFeet,
			SightFeet:      mi.SightFeet,
			Actions:        mi.Actions,
			Targeting:      mi.Targeting,
			BlocksMovement: mi.BlocksMovement,
		}
		e.members[mi.ID] = member
		e.everMembers[mi.ID] = true // Track in everMembers

		// Every member starts on the world clock. Free roam is not a mode, it
		// is simply where you are when no fight has pulled you elsewhere.
		if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(mi.ID)}); cerr != nil {
			return nil, fmt.Errorf("newencounter member %q world clock: %w", mi.ID, cerr)
		}

		// Store decider if present (monsters only, validated above)
		if mi.Decider != nil {
			e.deciders[mi.ID] = mi.Decider
		}

		// The author's knowledge links, seeded as the holdings they are
		// (design P1). SETUP ONLY — Load replays the journal instead, so
		// intel somebody already looted is not handed back to the body.
		if err = e.holdings.seedIntel(mi.ID, mi.Knows); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
	}

	// Store endings in declaration order (deterministic evaluation, C8), each
	// positional one compiled to the canvas cell it fires on.
	e.endings = compileEndings(in.Endings, f)

	// First light: build sight percepts for each member using refreshSight
	firstLight, err := e.rebuildPercepts(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("newencounter first light: %w", err)
	}

	// Opening record beat: all members hear "scene-opened". tableBeat,
	// subjects = memberIDs verbatim — this beat's audience has always been
	// declaration order (the order Members were given in), not the sorted
	// order every other beat in this module uses, and audienceFor's
	// tableBeat branch preserves exactly that (see its doc).
	beatPayload, _ := json.Marshal(map[string]string{"beat": "scene-opened"})
	_, err = e.appendBeat(&record.AppendInput{
		At:       0,
		Audience: e.audienceFor(tableBeat, memberIDs...),
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatPayload,
	})
	if err != nil {
		return nil, fmt.Errorf("newencounter append beat: %w", err)
	}

	// Concealment's own first light, AFTER the scene has opened and BEFORE
	// any fight it might start: presence pierces from the first frame — a
	// party start inside a concealed region is legal authoring, and the
	// occupants begin knowing — and a concealed door authored OPEN is
	// perceivable from frame one too. A no-op for a field with no
	// concealment.
	if serr := e.sweepConcealment(); serr != nil {
		return nil, fmt.Errorf("newencounter first light: %w", serr)
	}

	// Trigger detection at first light, AFTER the scene has opened. A scene
	// can open with a wolf already staring at the party, and a fight that
	// waited for somebody to take a step would let them stand there
	// indefinitely — but the story still has to read in the order it happened,
	// and a fight that starts before the scene opens is a story nobody can
	// follow. Setup ruled that first; it generalizes to every verb and the
	// law is stated at [Encounter.refreshSight].
	//
	// This is also what makes reading only the transition lists complete
	// everywhere else: every awareness that exists was created by some
	// refreshSight, and this is the first one, so no awareness predates
	// classification and no stale asymmetry can be missed.
	if _, _, terr := e.applyTrigger(firstLight); terr != nil {
		return nil, fmt.Errorf("newencounter first light: %w", terr)
	}

	return e, nil
}

// View returns the member's current intel holdings.
// Returns ErrNotMember if the member is not part of this encounter.
func (e *Encounter) View(in *ViewInput) ([]intel.Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("view: %w", ErrNilInput)
	}

	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("view: %w", ErrNotMember)
	}

	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}

	return holdings, nil
}

// buildMemberOutcomes snapshots every current member's placement in
// sorted-ID order — deterministic output for outcomes and persistence
// (map iteration here was a latent nondeterminism, T6 review M1).
func (e *Encounter) buildMemberOutcomes() []MemberOutcome {
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	outcomes := make([]MemberOutcome, 0, len(ids))
	for _, id := range ids {
		m := e.members[id]
		cell, ok := e.canvas.GetEntityPosition(string(m.ID))
		if !ok {
			continue
		}
		region, _ := e.RegionAt(cell)
		outcomes = append(outcomes, MemberOutcome{ID: m.ID, Region: region, Position: cell})
	}
	return outcomes
}

// Members returns the current member roster in stable order, each with the
// dungeon-absolute cell they stand on.
//
// The position is why this exists in its current shape. Projecting a roster
// used to mean calling ToData — serializing clock, intel, log, field and
// endings to read two floats per member, once per frame in the worst case
// (rpg-toolkit#933). A roster read should cost a roster read.
//
// Returns ErrNoField if a member's cell cannot be resolved, which would mean
// the roster and the canvas disagree about who is placed — a defect worth
// surfacing rather than papering over with a zero position that reads like the
// map's origin. Their REGION cannot fail separately: it is derived from the
// cell (rpg-toolkit#1108), and a placed member's cell is always floor.
func (e *Encounter) Members() ([]Member, error) {
	// Sort by ID for stability
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	members := make([]Member, 0, len(ids))
	for _, id := range ids {
		member, err := e.placementOf(e.members[id])
		if err != nil {
			return nil, fmt.Errorf("members: %w", err)
		}
		members = append(members, member)
	}
	return members, nil
}

// placementOf builds a member's read shape: the stored record, the cell they
// actually stand on, and the region that holds it.
//
// ONE path, used by every read that reports a member — Members, MembersIn and
// Join alike. Two of them answering the same question differently is the kind
// of drift that stays invisible until two clients disagree about where
// somebody is.
func (e *Encounter) placementOf(record *memberRecord) (Member, error) {
	cell, err := e.cellOf(record)
	if err != nil {
		return Member{}, err
	}

	region, _ := e.RegionAt(cell)
	return Member{
		ID:             record.ID,
		Kind:           record.Kind,
		Name:           record.Name,
		Region:         region,
		Position:       cell,
		SpeedFeet:      record.SpeedFeet,
		SightFeet:      record.SightFeet,
		Actions:        record.Actions,
		Targeting:      record.Targeting,
		BlocksMovement: record.BlocksMovement,
	}, nil
}

// Distance reports the grid distance between two dungeon-absolute cells, in
// the same units this composition's own reach and sight checks use — cube
// distance on a hex field, Chebyshev on a square one (rpg-toolkit#1010).
//
// EXPOSED RATHER THAN LEFT INTERNAL, deliberately minimally: session needs to
// gate Attack on weapon reach and price Afford's per-target declarations the
// same way, and the alternative is a host re-deriving hex math from Atlas
// data it was never meant to carry grid semantics through (S2's spirit
// extended to arithmetic, not only types) — see refreshSight's own call to
// e.canvas.GetGrid().Distance for the internal precedent this mirrors. It
// takes cells rather than member IDs because every caller with a reach
// question already has both positions in hand (a roster read, a Sighting),
// and a second roster lookup here would be a redundant one.
func (e *Encounter) Distance(a, b spatial.Position) float64 {
	return e.canvas.GetGrid().Distance(a, b)
}

// cellOf reads a member's cell off the canvas, which is the only place that
// knows it — and it is already the dungeon-absolute one every report speaks.
func (e *Encounter) cellOf(record *memberRecord) (spatial.Position, error) {
	cell, ok := e.canvas.GetEntityPosition(string(record.ID))
	if !ok {
		return spatial.Position{}, fmt.Errorf("member %q: not placed on the map: %w", record.ID, ErrNoField)
	}
	return cell, nil
}

// Status returns the encounter's current state (Open or Closed with Outcome).
// Returns a deep copy of the outcome to prevent aliasing (MUTATION-PROOF).
func (e *Encounter) Status() (*Status, error) {
	if e.outcome != nil {
		// Deep-copy outcome and its Members slice to prevent aliasing
		// (mutation-proof: modifying returned outcome does not affect internal state)
		members := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			members[i] = MemberOutcome{
				ID:       m.ID,
				Region:   m.Region,
				Position: m.Position,
			}
		}
		return &Status{
			Open: false,
			Outcome: &Outcome{
				Ending:  e.outcome.Ending,
				At:      e.outcome.At,
				Members: members,
			},
		}, nil
	}
	return &Status{Open: true}, nil
}

// Story returns a member's story entries from the given sequence number
// onward, INCLUSIVE of it — AfterSeq is passed through as record.SliceFor's
// FromSeq, and its name is a misnomer kept for compatibility (see
// StoryInput.AfterSeq). To resume after entry N, pass N+1.
//
// Allows both current members and members who have exited (everMembers).
// Returns ErrNilInput if the input is nil, ErrNoMember if the member never
// joined, and ErrTrimmed if a non-zero AfterSeq names a sequence that has
// already aged out of the retention window — the caller must resync rather
// than resume, since a short answer would be indistinguishable from a complete
// one. AfterSeq == 0 is exempt and always answerable.
// Copy-out follows record's own conventions (returned entries are already copies
// per record's implementation).
func (e *Encounter) Story(in *StoryInput) ([]record.Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}

	if _, ok := e.everMembers[in.Audience]; !ok {
		return nil, fmt.Errorf("story: %w", ErrNoMember)
	}

	// A resume point below the retained floor cannot be honoured, and must be
	// REJECTED rather than partially answered. A caller passing a sequence is
	// asserting "I already hold everything below this"; returning only the
	// surviving tail would be indistinguishable from a complete answer and would
	// leave a silent, permanent hole in that caller's story. Rejecting tells it
	// to resync (#937).
	//
	// AfterSeq == 0 is exempt: zero means "I hold nothing, send what you have,"
	// which is always answerable. That is the difference between a first load
	// and a reconnect, and it is why trimming does not break the most common
	// call in the system.
	if in.AfterSeq > 0 && in.AfterSeq < e.logFloor {
		return nil, fmt.Errorf("story: seq %d below retained floor %d: %w",
			in.AfterSeq, e.logFloor, ErrTrimmed)
	}

	entries, err := e.story.SliceFor(&record.SliceForInput{
		Viewer:  in.Audience,
		FromSeq: in.AfterSeq,
	})
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}

	return entries, nil
}

// moveMember executes a spatial move for a member and returns the old position if successful,
// or an error if the spatial move was rejected. This is the shared managed seam for both
// player moves (Move verb) and monster moves (Pump). The member must exist and be in an
// open encounter; spatial rejection does not abort the operation (handled by caller).
func (e *Encounter) moveMember(member *memberRecord, to spatial.Position) (spatial.Position, error) {
	currentPos, ok := e.canvas.GetEntityPosition(string(member.ID))
	if !ok {
		return spatial.Position{}, fmt.Errorf("movemember: %w", ErrBadPlacement)
	}

	// The canvas refuses a move that crosses a movement-blocking boundary
	// (tools/spatial's MoveEntity checks every crossing on the canonical ray),
	// which is what a wall between two chambers now does and what a room
	// membership test used to stand in for.
	if err := e.canvas.MoveEntity(string(member.ID), to); err != nil {
		return spatial.Position{}, fmt.Errorf("movemember: %w: %w", ErrBadPlacement, err)
	}

	return currentPos, nil
}

// rosterIDs is every member of this encounter, in stable ID order.
//
// The refresh scope for every verb that changes what can be seen, and the
// audience for every beat those verbs append. Sorted because determinism is
// module law (C8) and because a beat's audience is persisted: an unstable
// order would rewrite the blob on a save that changed nothing.
func (e *Encounter) rosterIDs() []MemberID {
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// frontierAudience is THE FRONTIER STOP (ruled on rpg-project#351, second
// round): a step whose destination lies inside a concealed region is
// delivered only to the members that region has been revealed to — the
// trail stops at the concealment boundary, the same knowledge scoping door
// beats already carry. The mover is always included: you saw yourself do
// it (their own presence fact lands in this same verb's sweep, but the
// beat precedes the sweep by the beat-order law, so the fold cannot answer
// for them yet).
//
// DELIBERATELY THE CONCEALMENT-FORCED MINIMUM. Steps on visible floor stay
// full-data to the whole roster — sight-scoped movement with last-known
// ghosts is the ruling's own named follow-up, not this. A recipient who
// gains the region reveal later simply starts receiving ordinary position
// updates from then on; the hidden trail is never backfilled. Applied in
// the ONE movement-beat writer, so a player's walk and a monster's pump
// step are scoped by the same line.
func (e *Encounter) frontierAudience(action executedAction, audience []MemberID) []MemberID {
	if e.world == nil {
		return audience
	}
	region, owned := e.field.regionOf(action.to)
	if !owned || !e.world.concealedRegions[region] {
		return audience
	}
	out := make([]MemberID, 0, len(audience))
	for _, id := range audience {
		if id == action.member.ID || e.world.knowsRegion(id, region) {
			out = append(out, id)
		}
	}
	return out
}

// beatClass names what a story beat is about — the audience question every
// append site used to answer alone, each in its own slightly different way.
// Recorded once per call, honestly, so rpg-toolkit#940's eventual policy
// split is a change to audienceFor's body, not a hunt through ten call
// sites for which ones need it.
type beatClass int

const (
	// subjectBeat concerns specific members: struck, missed, down, moved,
	// joined — and a door, whose own state change is a member-adjacent
	// fact too, not a table-wide one. subjects is the actor and targets
	// (moved: the mover; joined: the joiner; a door beat passes none —
	// see setDoorState's own doc, the shelf's one early adopter, which
	// already carries a precomputed, perception-shaped audience of its
	// own rather than one this function derives).
	subjectBeat beatClass = iota
	// bubbleBeat concerns a fight's clock: formed, turn-ended, transferred,
	// dissolved. subjects is the bubble's current members.
	bubbleBeat
	// tableBeat concerns the whole encounter, not any one member or fight:
	// scene-opened, tick, exited, ended. No PER-MEMBER subject — but a table
	// site that already computes its own audience (declaration order at
	// scene-opened; a roster snapshot taken before something else changes
	// it, at Pump's tick beat and Exit's own exit beat) passes it through
	// AS subjects so audienceFor doesn't overwrite it with a fresh, sorted
	// e.rosterIDs() that would tell a different, wrong-order story.
	tableBeat
)

// audienceFor is the one decision point every story beat's audience flows
// through — the shelf rpg-toolkit#940 sits on.
//
// Kirk's ruling (rpg-project#260 slice 4, 2026-08-24): "Until we get to
// v1.0 we intend on giving all the data down the combat log. Limiting what
// others see is a later concern — but we want a shelf that it can sit upon
// when needed." So v1's policy, for every class, is EVERYONE: the current
// roster, unconditionally.
//
// class is passed honestly at every call site — that classification is
// this slice's actual deliverable, reviewed once here. #940 is the flip,
// and it lands as a change to THIS function's body alone: subjects ∪
// current sight-holders for subject beats, bubble membership for bubble
// beats, table beats unchanged. No call site moves.
//
// subjects is ignored for subjectBeat and bubbleBeat today — v1 sends
// everyone regardless of who the beat is about, and subjects is only
// recorded for #940 to read later. tableBeat is the one exception: when a
// table site passes subjects, those ARE today's answer verbatim, because a
// table beat's own audience computation already varies in ways a fresh
// e.rosterIDs() call would not reproduce — scene-opened's declaration
// order (not sorted, unlike everything else here) chief among them. An
// empty tableBeat call (Pump's tick beat, Exit's exit beat, End's end beat)
// falls through to e.rosterIDs(), which already matches what those three
// compute by hand today.
func (e *Encounter) audienceFor(class beatClass, subjects ...MemberID) []MemberID {
	if class == tableBeat && len(subjects) > 0 {
		return subjects
	}
	return e.rosterIDs()
}

// appendMovementBeat records one executed step in the story and returns its
// sequence number.
//
// ONE narration path for every movement this composition performs — the Step
// verb, and every monster action inside a Pump. A movement is reported TWICE,
// once as a typed output and once as a beat, and a host reading both must be
// told the same cell; two copies of this arithmetic is how those two answers
// drift apart.
//
// Cells are DUNGEON-ABSOLUTE (#1040). A room-local coordinate with no room
// attached — which is exactly what the moved beat carried before — names
// nowhere in a multi-room field: two members in different rooms could report
// the same "position" and mean cells at opposite ends of the map.
//
// CALL THIS BEFORE refreshSight. A verb's own beat precedes any beat its
// consequences append — the law is stated at [Encounter.refreshSight].
//
// audience is computed by the CALLER, via audienceFor(subjectBeat, mover) —
// not here, because Pump needs its pre-Phase-1 snapshot reused across every
// movement beat in one tick rather than a fresh roster read per action (see
// Pump's own comment on why).
func (e *Encounter) appendMovementBeat(action executedAction, audience []MemberID, at uint64) (uint64, error) {
	audience = e.frontierAudience(action, audience)

	payload := map[string]interface{}{
		"beat":     "moved",
		"member":   string(action.member.ID),
		"position": action.to,
	}
	if len(action.doors) > 0 {
		// A step that went through a door names it. The BEAT does not
		// change — it is still "moved", because that is what happened
		// (rpg-toolkit#1106): a crossing stopped being a second kind of
		// movement when the field stopped being a set of rooms, and the
		// connection name that used to ride here went with the room chain
		// (rpg-project#256) — a doorway is the door standing in it.
		//
		// EXCEPT A CONCEALED ONE (rpg-toolkit#1371). This beat is
		// audienced to the whole roster, and one shared payload cannot
		// say a secret to knowers without saying it to everyone — so a
		// concealed door never rides it, found or not: the mover's own
		// crossing writes their recipient-scoped DOOR_REVEALED, and what
		// a door is doing reaches its knowers through its own beats. The
		// move itself stays narrated for everyone, doors or no doors.
		ids := make([]string, 0, len(action.doors))
		for _, d := range action.doors {
			if e.world != nil {
				if rec, ok := e.doorsByID[d.ID]; ok && rec.concealed != nil {
					continue
				}
			}
			ids = append(ids, d.ID)
		}
		if len(ids) > 0 {
			payload["doors"] = ids
		}
	}

	// PROPAGATED, not discarded. Every movement beat in this module used to
	// build its own payload and drop this error on the floor, which writes a
	// nil payload into the story and calls it a success — a beat a host reads
	// as an empty object rather than as a movement. It is unreachable in
	// practice (the payload is two strings and a position of finite float64s,
	// and construction refuses NaN and ±Inf), and it is one line here because
	// the four verbs now share one writer. clocks.go and outcome.go already
	// propagate theirs; this is the movement half catching up.
	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal moved beat: %w", err)
	}

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: audience,
		Tags:     map[string]string{"tag": "movement"},
		Payload:  beatBytes,
	})
	if err != nil {
		return 0, err
	}
	return appendOut.Seq, nil
}

// firedReachedPosition evaluates every declared ReachedPosition ending against
// where a member has just come to rest, closing the encounter if one fires.
//
// The cell is DUNGEON-ABSOLUTE, and so is the ending's, compiled from its
// authored room and local target once at construction (see compileEndings).
// One frame, one equality — where this used to compare a room label and then a
// coordinate, in whichever frame the calling verb happened to hold.
//
// The member filter carries a rule that reads backwards until you know it:
// EMPTY means any PLAYER member, not any member at all. A monster wandering
// onto the tomb's exit tile does not end the scene — the party leaving does.
// Naming a member explicitly overrides that, and then kind does not matter.
//
// Returns a DEEP COPY (mutation-proof): a caller holding the returned outcome
// cannot reach into this encounter's own.
//
// An ALREADY-CLOSED encounter short-circuits to its outcome: the sight
// refresh that ran just before this scan can itself close the scene
// (TriggerMemberDown, evaluated in noticeDown), and a second close here
// would overwrite the first and narrate a second ended beat. The verb still
// reports the close on its output — whichever trigger fired it.
func (e *Encounter) firedReachedPosition(member *memberRecord, cell spatial.Position, at uint64) (*Outcome, error) {
	if e.outcome != nil {
		members := make([]MemberOutcome, len(e.outcome.Members))
		copy(members, e.outcome.Members)
		return &Outcome{Ending: e.outcome.Ending, At: e.outcome.At, Members: members}, nil
	}

	for _, de := range e.endings {
		trigger, ok := de.trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}
		if de.cell.X != cell.X || de.cell.Y != cell.Y {
			continue // Different cell
		}
		if trigger.Member != "" && trigger.Member != member.ID {
			continue // Member filter doesn't match
		}
		if trigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only
		}

		return e.closeWith(de.key, at)
	}
	return nil, nil
}

// closeWith closes the encounter with the ending that fired: sets the
// outcome and appends the table-wide "ended" beat every close narrates.
//
// ONE PATH: External (End), ReachedPosition and MemberDown all close through
// here, so "what happens when an encounter ends" has a single answer — the
// same argument setDoorState makes for doors. Before this, a ReachedPosition
// close set the outcome and told nobody: the host learned from the verb's
// output while the story skipped a beat the External path wrote, and a
// client following the stream never heard the run end.
//
// Returns a DEEP COPY (mutation-proof), like every projection.
func (e *Encounter) closeWith(key string, at uint64, audience ...MemberID) (*Outcome, error) {
	e.outcome = &Outcome{
		Ending:  key,
		At:      at,
		Members: e.buildMemberOutcomes(),
	}

	// tableBeat. Callers that close mid-verb pass the audience they captured
	// BEFORE the verb changed the roster; everyone else passes none and a
	// fresh e.rosterIDs() is already "everyone".
	//
	// THE EXIT PATH IS WHY THIS IS A PARAMETER (rpg-project#368). A
	// TriggerExitedHolding ending fires after the departing member has been
	// removed from e.members, so a fresh roster read here would leave the
	// carrier out of the beat announcing the run they just won. Every other
	// close still runs with nobody removed, and for those the two are the
	// same list.
	beatBytes, _ := json.Marshal(map[string]interface{}{
		"beat":   "ended",
		"ending": key,
	})
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(tableBeat, audience...),
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatBytes,
	}); err != nil {
		return nil, fmt.Errorf("close append beat: %w", err)
	}

	members := make([]MemberOutcome, len(e.outcome.Members))
	copy(members, e.outcome.Members)
	return &Outcome{Ending: key, At: at, Members: members}, nil
}

// Pump advances the world by one tick: the exploration clock advances,
// each monster member (in deterministic order) acts on its own intel via Decider,
// the complete sight refresh happens once, and the story accrues tick and
// movement beats. Errors from a decider abort the pump atomically (R5):
// no clock advance, no moves, no record entries.
//
// WHAT PUMP DRIVES IN v1, stated plainly because the answer narrowed: members
// on the WORLD clock. A bubble member is deliberately not pumped, and under
// v1's sight model (rpg-toolkit#964) any monster a player could see was in a
// bubble — so in practice this verb moved the monsters NOBODY HAD SEEN YET.
// Free-roam monster behaviour is offscreen behaviour.
//
// That was a narrowing rather than the intent, and it has started widening
// back exactly where classify's doc said it would: in how percepts are
// PRODUCED. rpg-toolkit#1111's per-member sight range makes the drop real, so
// a monster CAN now be watched by a player it cannot see back without a fight
// starting — and it keeps being pumped while that is true. #1020 widens it
// further, and a faction model lets a visible creature be non-hostile. All of
// them change what forms a bubble, not what Pump does — see classify's doc for
// the invariant. Pinned by
// TestPumpStopsMovingAMonsterOnceSeen.
//
// Semantics:
//   - Tick advances by exactly 1 (via clock.Advance with displacement 1).
//   - Monsters act in deterministic order (stable Members() order, filtered to KindMonster).
//   - Each decider receives exactly its own Snapshot: its own cell on the map and
//     its own holdings (anti-wall-hack contract C2 — placement included, not just
//     sight).
//   - IntentHold means do nothing; IntentMoveTo names a cell in dungeon-absolute
//     space and executes through stepTo — one step on the map, whether or not it
//     goes through a doorway.
//   - A refused step does NOT abort the pump — a cell no room owns, a wall in the
//     way, or any other spatial rejection all mean the monster simply fails to
//     act. Only a decider error aborts.
//   - After all monster actions: ONE refreshSight for all members, ONE tick beat
//     (stamped with the new clock reading), then movement beats in decision order
//     (the same order monsters were consulted in).
//   - Ending evaluation fires ReachedPosition triggers (only if the filter matches;
//     empty filter = players only, not monsters) against each action's resulting
//     cell, in the same decision order.
//   - Returns PumpOutput with the new Tick reading, the steps monsters took,
//     deltas, and beats.
func (e *Encounter) Pump(in *PumpInput) (*PumpOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("pump: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("pump: %w", ErrClosed)
	}

	// PHASE 1 — decide. Every decider is consulted BEFORE anything
	// mutates: a decider error aborts here with zero state touched
	// (R5 — no clock advance, no moves, no beats). This also means a
	// later monster's decider error cannot leave an earlier monster's
	// move half-applied.
	allMembers, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("pump members: %w", err)
	}

	// The tick beat's (and every movement beat's) audience, captured HERE
	// rather than at the append site: a contract-violating decider that
	// removes itself mid-Decide below still belongs in this tick's beat
	// audience, because an exited member keeps Story access to the beats
	// they were present for. tableBeat's policy is "everyone" either way,
	// but WHICH "everyone" — before or after Phase 1 mutates e.members — is
	// a real difference, and it is why Pump does not call audienceFor fresh
	// at each append site the way the other verbs do.
	audience := e.audienceFor(tableBeat)

	// Who is down, asked before anything is planned: a body has no action to
	// take, so its decider is not consulted at all rather than consulted and
	// discarded (a decider is behaviour, and running a corpse's behaviour is
	// the second census defect — Pump had no standing filter and dead monsters
	// kept patrolling).
	//
	// This is the SECOND consult in a Pump — refreshSight runs another at the
	// end, through noticeDown, which is what narrates and splices. Deliberate,
	// both ways round: the answer is not carried forward because carrying it
	// is a cache ([Standing]), and the narration cannot happen here because a
	// down beat appended before Pump's own tick beat would break the ordering
	// law refreshSight states.
	down, err := e.standingNow()
	if err != nil {
		return nil, fmt.Errorf("pump standing: %w", err)
	}

	type plannedAction struct {
		memberID MemberID
		intent   Intent
	}
	var planned []plannedAction

	for _, m := range allMembers {
		if m.Kind != KindMonster {
			continue
		}

		if down[m.ID] {
			continue
		}

		// A monster caught in a bubble is not the world's to think for: the
		// world thinks on the tick, and a fight thinks in turns. Skipped, not
		// rejected — being mid-fight is ordinary state, and Pump's job is
		// everyone else. Its budget entry is gone with it (Form removed it
		// from the tick), so the Advance below grants it nothing either.
		bubble, berr := e.bubbleFor(m.ID)
		if berr != nil {
			return nil, fmt.Errorf("pump bubble: %w", berr)
		}
		if bubble != nil {
			continue
		}

		decider, hasDecider := e.deciders[m.ID]
		if !hasDecider {
			continue // no decider = hold
		}

		// The monster's own placement, read fresh from the canvas — never
		// another member's. A decider that received anyone else's position
		// would be a wall hack extended to placement, not just sight (C2).
		ownCell, ok := e.canvas.GetEntityPosition(string(m.ID))
		if !ok {
			return nil, fmt.Errorf("pump snapshot position: %w", ErrBadPlacement)
		}

		// The monster's own holdings and nothing else (C2). HeldBy's
		// copy-out is intel's documented contract (pinned in play/intel)
		// — no redundant defensive copy here; the mutating-decider
		// integration test pins the composed guarantee.
		ownHoldings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: m.ID})
		if err != nil {
			return nil, fmt.Errorf("pump held_by: %w", err)
		}

		intent, err := decider.Decide(Snapshot{
			Position: ownCell,
			Holdings: ownHoldings,
		})
		if err != nil {
			return nil, fmt.Errorf("pump decide: %w", err)
		}

		switch intent.(type) {
		case IntentMoveTo:
			planned = append(planned, plannedAction{memberID: m.ID, intent: intent})
		}
	}

	// PHASE 2 — execute. Nothing below returns a decider-shaped error;
	// the world now advances.
	_, err = e.clock.Advance(&clock.AdvanceInput{
		Driver:       core.EntityID("world"),
		Displacement: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("pump advance: %w", err)
	}

	newTickReading := uint64(e.clock.ToData().HighWater)

	// executedAction (declared at package scope, beside stepTo which builds
	// one) is collected in PLANNED (decision) order regardless of kind, so
	// beats and ending evaluation below stay in the same deterministic
	// per-monster order the deciders were consulted in (C8) — not "all moves
	// then all crossings".
	var executed []executedAction

	for _, p := range planned {
		// The REAL member pointer, not a Members()-derived copy: an
		// executedAction carries it into beats and ending evaluation, and a
		// value copy of a member who left mid-tick would keep those alive.
		member := e.members[p.memberID]
		if member == nil {
			// A contract-violating decider removed itself from the
			// encounter (e.g. called Exit on its own member) during
			// phase 1's Decide. Its planned action has no live member
			// to execute against — same silent-skip contract as a
			// spatially-rejected move: absent from output and beats,
			// the pump otherwise proceeds normally.
			continue
		}
		if intent, ok := p.intent.(IntentMoveTo); ok {
			if action, stepped := e.stepTo(member, intent.To); stepped {
				executed = append(executed, action)
			}
		}
	}

	// Single refreshSight for all members after all monster actions, over
	// the SAME pre-Phase-1 audience captured above (its own comment there) —
	// movement beats below reuse it too, for the same reason.

	// Pump's own beats — the tick frame and every action inside it — are
	// recorded BEFORE sight refreshes: the monsters' walk is the cause,
	// anything trigger detection appends is its effect (see refreshSight).
	//
	// Record the tick beat first (the frame)
	tickBeatPayload := map[string]interface{}{
		"beat": "tick",
		"tick": newTickReading,
	}
	tickBeatBytes, _ := json.Marshal(tickBeatPayload)

	tickAppendOut, err := e.appendBeat(&record.AppendInput{
		At:       newTickReading,
		Audience: audience,
		Tags:     map[string]string{"tag": "clock"},
		Payload:  tickBeatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("pump append tick beat: %w", err)
	}

	seqs := []uint64{tickAppendOut.Seq}

	// Then record a beat for each successful action, in decision order.
	for _, action := range executed {
		actionSeq, err := e.appendMovementBeat(action, audience, newTickReading)
		if err != nil {
			return nil, fmt.Errorf("pump append movement beat: %w", err)
		}
		seqs = append(seqs, actionSeq)
	}

	intelDeltas, formed, err := e.refreshSight(audience)
	if err != nil {
		return nil, fmt.Errorf("pump refresh sight: %w", err)
	}

	// Evaluate ReachedPosition endings, in decision order, against the cell
	// each step landed on.
	//
	// A monster never fires an UNFILTERED ending: the empty filter means "any
	// player member", which firedReachedPosition enforces by kind, so a
	// wandering goblin cannot end the scene by standing on the exit.
	var firedOutcome *Outcome
	for _, action := range executed {
		fired, ferr := e.firedReachedPosition(action.member, action.to, newTickReading)
		if ferr != nil {
			return nil, fmt.Errorf("pump ending: %w", ferr)
		}
		if fired != nil {
			firedOutcome = fired
			break
		}
	}

	// Build the output. ONE list, in the order the steps were executed —
	// which is the order the deciders were consulted in (C8).
	//
	// It used to be two, split by which mechanism carried the step. The split
	// was never about the world: a crossing looked different because the
	// composition had two ways to move somebody, and it has one
	// (rpg-toolkit#1106). Every cell here is read straight off the canvas, so
	// what a host reads on this output and what it reads on the same movement's
	// beat cannot disagree — the drift rpg-toolkit#1062 chased was two
	// projections of one fact, and there are no projections left.
	var outputMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}
	for _, action := range executed {
		outputMoves = append(outputMoves, struct {
			Member MemberID
			From   spatial.Position
			To     spatial.Position
		}{
			Member: action.member.ID,
			From:   action.from,
			To:     action.to,
		})
	}

	return &PumpOutput{
		Tick:         newTickReading,
		MonsterMoves: outputMoves,
		IntelDeltas:  intelDeltas,
		Seqs:         seqs,
		Outcome:      firedOutcome,
		Formed:       formed,
	}, nil
}

// refreshSight rebuilds every observer's percept AND runs trigger detection on
// what changed, returning the deltas and any fight that started.
//
// The two are ONE call on purpose. Trigger detection is a rule about sight, so
// it belongs wherever sight changes — and wiring it at the verbs instead left
// crossings and Join silently untriggered until review caught them
// (rpg-toolkit#964). A verb cannot refresh sight and forget the rule if
// refreshing sight IS running the rule; a future verb gets it by writing the
// obvious call.
//
// CALL THIS AFTER YOUR VERB HAS APPENDED ITS OWN BEAT. The law, stated once
// here because here is where every verb meets it: A VERB'S OWN BEAT PRECEDES
// ANY BEAT ITS CONSEQUENCES APPEND — cause before effect, in every story. A
// reader of Story must be able to see the walk that started the fight before
// the fight. Setup ruled this first (a scene records that it opened before it
// records a fight starting inside it); the same law holds for Step, Pump and
// Join, and each one pins its own half of it.
//
// [Encounter.rebuildPercepts] is the half without the rule, and Setup is its
// only caller — Setup needs the two halves separated so its scene-opened beat
// can land between them.
func (e *Encounter) refreshSight(observers []MemberID) (map[MemberID]*IntelDelta, *FormedBubble, error) {
	deltas, err := e.rebuildPercepts(observers)
	if err != nil {
		return nil, nil, err
	}

	// A closed encounter has nothing to start a fight about, and form refuses
	// one anyway — checking here turns that refusal into a non-event.
	if e.outcome != nil {
		return deltas, nil, nil
	}

	// Concealment's trigger detection rides every sight refresh, for this
	// function's own reason: perceiving present state — an open concealed
	// door, hidden floor underfoot — is a rule about sight, and a rule
	// wired at the verbs is a rule some verb forgets (rpg-toolkit#1371).
	// Its reveal beats land after the verb's own beat (the law above) and
	// before any fight the refresh starts; a CLOSED encounter mints no new
	// knowledge, which the early return above already said.
	if err := e.sweepConcealment(); err != nil {
		return nil, nil, err
	}

	formed, deltas, err := e.applyTrigger(deltas)
	if err != nil {
		return nil, nil, err
	}

	return deltas, formed, nil
}

// rebuildPercepts rebuilds the complete percept for all given observers,
// surveils each, and returns a map of member IDs to encounter-owned intel
// deltas.
// The current clock reading is stamped on each Surveil call.
func (e *Encounter) rebuildPercepts(observers []MemberID) (map[MemberID]*IntelDelta, error) {
	// Get current clock reading
	clockReadingInt := e.clock.ToData().HighWater
	clockReading := uint64(clockReadingInt)
	deltas := make(map[MemberID]*IntelDelta)

	// Asked ONCE per refresh and never carried between them — see [Sight] for
	// why remembering the answer would be the smallest possible version of the
	// dual state the capability exists to avoid. Asked BEFORE the loop rather
	// than inside it so that every observer in one refresh is bounded by the
	// same reading of the world (C8), and so that a rulebook is consulted once
	// per pass rather than once per member.
	reach, err := e.sightNow()
	if err != nil {
		return nil, err
	}

	for _, observerID := range observers {
		if _, ok := e.members[observerID]; !ok {
			continue // Skip if not found
		}

		observerCell, ok := e.canvas.GetEntityPosition(string(observerID))
		if !ok {
			continue // Observer not placed
		}

		// Every OTHER member on the map, kept or dropped by GEOMETRY ALONE.
		//
		// There was a room-membership test here, immediately before the line of
		// sight check, and it decided almost everything: two members in
		// different chambers never saw each other, however close, and the check
		// below never ran for them. It was not a range rule, it was the ONLY
		// visibility rule — standing in for the walls the composition could not
		// express (rpg-toolkit#1105/#1106). With one canvas and real walls it
		// has nothing left to say, so it is gone and the geometry answers.
		//
		// AND BOUNDED BY A DISTANCE THIS MODULE WAS TOLD (rpg-toolkit#1111).
		// The room label was quietly doing that job too: with it gone, the
		// reference tomb's longest unobstructed run — three doorways on one
		// row, 27 cells, 135 feet — was a sighting, and a sighting forms a
		// fight. What was missing there was never a number this module could
		// pick. It was a LIGHT model, and light is per-creature and
		// per-light-source: the dwarf with darkvision and the human holding
		// her torch answer differently on the same cell.
		//
		// So the term is SUPPLIED, and supplied per member. [Sight] is asked
		// how far each of them can see, this refresh, and the answer bounds
		// what lands in the percept. The light model 5e states arrives later
		// as a better ANSWER — with nothing here moving — which is exactly the
		// promise [Standing] makes about hit points.
		//
		// Two members can answer differently, so A may see B without B seeing
		// A. That asymmetry is real and it is NOT rpg-toolkit#1020: geometry
		// stays mutual (spatial v0.9.1 pins it), and what differs is reach.
		// What it does do is give [Encounter.classify]'s spotted and drop arms
		// their first producible input, and 5e surprise with them, without
		// changing a line of how percepts are CONSUMED.
		var percept []intel.Report
		for _, otherMember := range e.members {
			if otherMember.ID == observerID {
				continue // Skip self
			}

			otherCell, ok := e.canvas.GetEntityPosition(string(otherMember.ID))
			if !ok {
				continue // Not placed
			}

			// Too far BEFORE blocked: both filters are geometric, and this
			// one is arithmetic while the next one walks a ray. Order is a
			// cost decision, not a correctness one — either filter alone
			// drops the subject. Strictly greater, because a member exactly
			// at the edge of your sight is inside it.
			if e.canvas.GetGrid().Distance(observerCell, otherCell) > float64(reach[observerID]) {
				continue // Beyond how far this observer can see
			}

			if e.canvas.IsLineOfSightBlocked(observerCell, otherCell) {
				continue // A wall, or something standing in the way
			}

			payload, err := EncodeLocationPayload(LocationKnowledge{
				State: LocationKnown, Position: otherCell,
			})
			if err != nil {
				return nil, fmt.Errorf("encode sight location: %w", err)
			}
			percept = append(percept, intel.Report{
				Subject: intel.Subject(otherMember.ID),
				Payload: payload,
			})
		}

		// Surveil with the complete percept and current clock reading
		out, serr := e.intelLog.Surveil(&intel.SurveilInput{
			Observer: observerID,
			Channel:  intel.Sight,
			Percept:  percept,
			At:       clockReading,
		})
		if serr != nil {
			return nil, fmt.Errorf("refreshsight surveil: %w", serr)
		}
		deltas[observerID] = intelDeltaFromSurveil(out)
	}

	return deltas, nil
}

// propEntity is one authored [PropInput] as the canvas holds it.
//
// BOTH ANSWERS ARE CARRIED, NOT DECIDED (rpg-toolkit#1128). Its predecessor
// answered true to sight and false to movement unconditionally, which made
// every authored thing transparent to walk through and opaque to look through —
// the inverse of a pillar, a statue and a coffin alike. What a prop blocks is
// the author's to say, and this type's only job is to say it back to spatial.
type propEntity struct {
	id                string
	ref               string
	blocksMovement    bool
	blocksLineOfSight bool
}

// GetID returns the prop's index-derived entity ID — see compileCanvas for why
// it is not the ref.
func (p *propEntity) GetID() string {
	return p.id
}

// GetType returns "prop"
func (p *propEntity) GetType() core.EntityType {
	return core.EntityType("prop")
}

// GetSize returns 1 (single-cell entity)
func (p *propEntity) GetSize() int {
	return 1
}

// BlocksLineOfSight reports what the author declared. Subject to spatial's lane
// rule either way: one cell of it obstructs nothing on its own ([PropInput]).
func (p *propEntity) BlocksLineOfSight() bool {
	return p.blocksLineOfSight
}

// BlocksMovement reports what the author declared. True refuses an arrival on
// this cell, which is what a step is ([PropInput]).
func (p *propEntity) BlocksMovement() bool {
	return p.blocksMovement
}

// Join adds a new member to the encounter. The ambient field is always there
// to join. Validation order (R5 atomicity): nil input → closed → empty member ID →
// already a member → player-with-decider rejected → spatial placement rejection.
// On success, the joiner is placed, all members' sight is refreshed (the joiner's
// first percepts AND incumbents now seeing them), and a beat is recorded.
// ReachedPosition endings are evaluated (a player could join ON the stairs — fires YES).
func (e *Encounter) Join(in *JoinInput) (*JoinOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("join: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("join: %w", ErrClosed)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("join: %w", ErrNoMember)
	}

	// Check if already a member
	if _, exists := e.members[in.Member]; exists {
		return nil, fmt.Errorf("join: member %s is already in the encounter: %w", in.Member, ErrNoMember)
	}

	// Players cannot carry deciders (design law C2)
	if in.Kind == KindPlayer && in.Decider != nil {
		return nil, fmt.Errorf("join: player %s cannot carry a decider: %w", in.Member, ErrNoMember)
	}

	// Nor can a world NPC (rpg-toolkit#1404, design.md N4) — see NewEncounter's
	// own check for why.
	if in.Kind == KindWorld && in.Decider != nil {
		return nil, fmt.Errorf("join: world npc %s cannot carry a decider: %w", in.Member, ErrNoMember)
	}

	// See NewEncounter's own call to validateMemberFacts for why — ASKED
	// BEFORE any mutation (canvas.PlaceEntity below is the first one),
	// unlike NewEncounter's construction-in-a-local-that-only-escapes-on-
	// success safety net: Join mutates a LIVE *Encounter, so an invalid
	// fact caught after PlaceEntity would need to roll a placement back
	// rather than simply never having made one (Copilot, PR #1187).
	if err := validateMemberFacts(in.Member, in.SpeedFeet, in.SightFeet, in.Actions); err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	// Hex fields require integral axial cells (interim tools/spatial#926
	// enforcement — see isIntegralHexCell). Asked first, for the reason
	// [Encounter.stepMember] asks it first: a fractional cell is an arithmetic
	// mistake and must not be reported as a map one.
	if !isIntegralHexCell(in.Cell) {
		return nil, fmt.Errorf("join: position is not an integral axial cell: %w", ErrBadPlacement)
	}

	// The arrival cell must be STANDABLE — some authored region has to own
	// it. The canvas spans the field's whole bounding box, so "on the map"
	// and "somewhere a member can stand" are different questions, and this is
	// the one that matters (the same check [Encounter.stepMember] makes for a
	// step). Scenery is on the map and is not somewhere anybody stands.
	if !e.field.isStandable(in.Cell) {
		return nil, fmt.Errorf("join: cell %v %s: %w", in.Cell, e.field.notStandable(in.Cell), ErrBadPlacement)
	}

	entity := &memberEntity{
		id:             string(in.Member),
		kind:           in.Kind,
		blocksMovement: in.BlocksMovement,
	}

	if err := e.canvas.PlaceEntity(entity, in.Cell); err != nil {
		return nil, fmt.Errorf("join placement: %w: %w", ErrBadPlacement, err)
	}

	// Register the member
	member := &memberRecord{
		ID:             in.Member,
		Kind:           in.Kind,
		Name:           in.Name,
		SpeedFeet:      in.SpeedFeet,
		SightFeet:      in.SightFeet,
		Actions:        in.Actions,
		Targeting:      in.Targeting,
		BlocksMovement: in.BlocksMovement,
	}
	e.members[in.Member] = member
	e.everMembers[in.Member] = true // Track in everMembers

	// A joiner lands on the world clock, never mid-fight. Being pulled into a
	// running bubble is Transfer's job and is a separate decision from joining
	// the encounter at all.
	if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(in.Member)}); cerr != nil {
		return nil, fmt.Errorf("join member %q world clock: %w", in.Member, cerr)
	}

	// Store decider if present (monsters only, validated above)
	if in.Decider != nil {
		e.deciders[in.Member] = in.Decider
	}

	// Audience for both the join beat and the sight refresh: the joiner sees
	// incumbents, incumbents see the joiner. subjectBeat, subject is the
	// joiner — v1 still sends everyone (audienceFor's doc); rpg-toolkit#940
	// is where "everyone" might narrow to who can actually see them arrive.
	memberIDs := e.audienceFor(subjectBeat, in.Member)

	// Record the join beat BEFORE refreshing sight: arriving is the cause,
	// anything trigger detection appends is its effect (see refreshSight).
	// Audience = all members including the joiner.
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":   "joined",
		"member": string(in.Member),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("join append beat: %w", err)
	}

	seqNum := appendOut.Seq

	intelDeltas, formed, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("join refresh sight: %w", err)
	}

	// Evaluate ReachedPosition endings (a player could join ON the stairs),
	// through the SAME firedReachedPosition every other arrival uses.
	//
	// Join used to carry its own copy of that scan — the census's side-finding
	// 2, and the defect class rpg-toolkit#1059 spent two PRs eliminating for
	// movement. The copy existed because Join held a room and a local cell
	// while the shared path held the member's current room; with one frame
	// there is nothing left for a second implementation to differ about.
	firedOutcome, err := e.firedReachedPosition(member, in.Cell, clockReadingForBeat)
	if err != nil {
		return nil, fmt.Errorf("join ending: %w", err)
	}

	placement, err := e.placementOf(member)
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	return &JoinOutput{
		Formed:      formed,
		Member:      placement,
		IntelDeltas: intelDeltas,
		Seq:         seqNum,
		Outcome:     firedOutcome,
	}, nil
}

// Exit removes a member from the encounter. The member leaves with carry-forward:
// they are removed from the field, their final MemberOutcome is returned, and their
// intel holdings are copied out for the campaign. The encounter auto-closes with the
// reserved ending "abandoned" if the last member exits and no declared ending has fired.
// After exit, remaining members' views fade the departed naturally (their entity left,
// so next refreshSight removes them from new percepts; old holdings ghost).
func (e *Encounter) Exit(in *ExitInput) (*ExitOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("exit: %w", ErrNilInput)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("exit: %w", ErrNoMember)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("exit: %w", ErrClosed)
	}

	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("exit: %w", ErrNotMember)
	}

	// Get the exiting member's final cell, and the region it falls in.
	finalCell, ok := e.canvas.GetEntityPosition(string(in.Member))
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}
	finalRegion, _ := e.RegionAt(finalCell)

	// Capture the exiting member's holdings (carry-forward)
	carry, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("exit held_by: %w", err)
	}

	// Remove from the map
	if err = e.canvas.RemoveEntity(string(in.Member)); err != nil {
		return nil, fmt.Errorf("exit remove entity: %w: %w", ErrBadPlacement, err)
	}

	// Remove from whichever clock holds them — the world clock normally, a
	// bubble if they were in a fight when they left.
	clockDeltas, cerr := e.leaveAnyClock(in.Member)
	if cerr != nil {
		return nil, fmt.Errorf("exit member %q clock: %w", in.Member, cerr)
	}
	intelDeltas := mergeIntelDeltas(nil, clockDeltas)

	// The exit beat's audience, captured HERE, before the exiter is removed
	// from e.members below: it is every member INCLUDING the exiter — they
	// witness their own departure (and can re-read it via Story).
	// audienceFor reads live membership, so calling it after the delete
	// would leave the exiter out (tableBeat, no subjects — see its doc).
	audience := e.audienceFor(tableBeat)

	// Remove from member set (and deciders if present)
	delete(e.members, in.Member)
	delete(e.deciders, in.Member)

	// Get remaining member IDs for story and refresh
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)

	// WHERE THEY STOOD ON THE WAY OUT is already recorded (finalCell above),
	// and it is the whole of the exit-cell check: an authored exit compiled
	// to one absolute cell, compared by equality, exactly as a
	// ReachedPosition ending is (rpg-project#368, design §6).
	leftThrough := e.field.exitAt(finalCell)
	carried := e.heldPropsOf(in.Member)

	beatPayload := map[string]interface{}{
		"beat":   "exited",
		"member": string(in.Member),
		// holding is what they walked out carrying, and exit is the authored
		// way they left by — empty when they left from anywhere else, which
		// is what a departure through the lobby looks like. Both always
		// present so a reader never has to tell "absent" from "none".
		"holding": carried,
		"exit":    string(leftThrough),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: audience,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("exit append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// THE ENDING, AFTER THE DEPARTURE BEAT (design §6): the record reads
	// "left through the front gate with the heirloom" and then "ended".
	firedOutcome, err := e.firedExitedHolding(in.Member, leftThrough, carried, clockReadingForBeat, audience)
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	// R9 — THEY DROP IT. A departure that did not end the run leaves
	// everything the member was carrying on the cell they stood on, as
	// takeable props anybody can pick up again. Otherwise a carrier who
	// leaves through the lobby — or simply disconnects — takes the only win
	// out of a run that is still going.
	if firedOutcome == nil {
		if err := e.dropCarried(in.Member, carried, finalCell, clockReadingForBeat, audience); err != nil {
			return nil, fmt.Errorf("exit: %w", err)
		}
	}

	// refreshSight for REMAINING members only (the exiter's holdings remain in intel archive)
	if len(memberIDs) > 0 {
		refreshDeltas, _, err := e.refreshSight(memberIDs)
		if err != nil {
			return nil, fmt.Errorf("exit refresh sight: %w", err)
		}
		intelDeltas = mergeIntelDeltas(intelDeltas, refreshDeltas)
	}

	// Check if we need to auto-close (last member exited and no ending has fired)
	closedOutcome := firedOutcome
	if len(e.members) == 0 && e.outcome == nil {
		e.outcome = &Outcome{
			Ending:  "abandoned",
			At:      clockReadingForBeat,
			Members: []MemberOutcome{}, // No members remain
		}
		closedOutcome = &Outcome{
			Ending:  "abandoned",
			At:      clockReadingForBeat,
			Members: []MemberOutcome{},
		}
	}

	return &ExitOutput{
		Outcome: MemberOutcome{
			ID:       in.Member,
			Region:   finalRegion,
			Position: finalCell,
		},
		Carry:       carry,
		Seq:         seqNum,
		Closed:      closedOutcome,
		IntelDeltas: intelDeltas,
	}, nil
}

// End fires an externally-triggered ending. Validates the key was declared and
// has an External trigger, then closes the encounter with that Outcome.
func (e *Encounter) End(in *EndInput) (*EndOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("end: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("end: %w", ErrClosed)
	}

	if in.Ending == "" {
		return nil, fmt.Errorf("end: %w", ErrNoEnding)
	}

	// Find and validate the ending
	var foundEnding *declaredEnding
	for i := range e.endings {
		if e.endings[i].key == in.Ending {
			foundEnding = &e.endings[i]
			break
		}
	}

	if foundEnding == nil {
		return nil, fmt.Errorf("end: ending %s not declared: %w", in.Ending, ErrNoEnding)
	}

	// Verify it's an External trigger
	_, isExternal := foundEnding.trigger.(TriggerExternal)
	if !isExternal {
		return nil, fmt.Errorf("end: ending %s is not External: %w", in.Ending, ErrNoEnding)
	}

	closed, err := e.closeWith(in.Ending, uint64(e.clock.ToData().HighWater))
	if err != nil {
		return nil, fmt.Errorf("end: %w", err)
	}

	return &EndOutput{Outcome: *closed}, nil
}

// memberEntity is an internal entity for members
type memberEntity struct {
	id   string
	kind MemberKind

	// blocksMovement carries the member's authored BlocksMovement fact
	// (rpg-toolkit#1434) into the canvas's own occupancy seam. False for
	// every member before this field existed, and still false for any
	// caller who doesn't set it — see MemberInput.BlocksMovement's doc.
	blocksMovement bool
}

// GetID returns the member's ID
func (m *memberEntity) GetID() string {
	return m.id
}

// GetType returns the kind of member as an EntityType
func (m *memberEntity) GetType() core.EntityType {
	return core.EntityType(m.kind)
}

// GetSize returns 1 (single-cell entity)
func (m *memberEntity) GetSize() int {
	return 1
}

// BlocksLineOfSight returns false for members
func (m *memberEntity) BlocksLineOfSight() bool {
	return false
}

// BlocksMovement reports the member's authored BlocksMovement fact
// (rpg-toolkit#1434). False for a caller who never set it — the behavior
// every member had before this field existed.
func (m *memberEntity) BlocksMovement() bool {
	return m.blocksMovement
}
