// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

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

	// cond and result are [EndingInput.If] and [EndingInput.Result].
	cond   Trigger
	result EndingResult
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
		de := declaredEnding{key: ei.Key, trigger: ei.Trigger, cond: ei.If, result: ei.Result}
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
	// holdEndings defers every close and every standing consult while a train
	// lands a unit's deed. The first ending asked for is parked in heldEnding
	// and the train closes with it after the last unit; a consult only answers
	// (noticeDown tells nothing). So neither an ending nor a fall is told
	// between two units' beats, while a fight the deed forms still forms in
	// place.
	// Unexported and never persisted: it is set and cleared inside one verb.
	holdEndings bool
	heldEnding  *heldEnding

	// acts is the stack of open acts (act.go): every beat appended carries
	// the top one as its correlation and its [ActTag] unless it names its
	// own correlation. Unexported and never persisted — an act lives inside
	// one call.
	acts []string

	// deferredDrives are the fights a held deed formed, whose first slot is
	// driven by the train's post-train pass instead of by the formation.
	deferredDrives []*clock.Turn

	// struckBy is who last landed an attack deed on each member during the
	// verb in flight ([Encounter.landAttack]): the cause a fall noticed in
	// that same verb names as its actor ([DeedFell]). Cleared by every
	// participation pass that tells its falls, so a blow never explains a
	// fall a later verb notices. Unexported and never persisted: it is set
	// and read inside one call.
	struckBy map[MemberID]MemberID

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

	// site is the compiled site the field is a shallow copy of (site.go,
	// rpg-project#879 S5): shared, read-only, the hash ToData writes in place
	// of the field, and the home of the raw visible-from answers. Nil only on
	// an Encounter built by hand in a test.
	site *Site

	// interactions are the run's authored interaction instances by id
	// (interaction.go, rpg-project#922): the entrance that remembers. Nil for
	// a run whose content declares none, which writes no key.
	interactions map[InteractionID]*interactionRecord

	// sharing is the party's sharing dial as authored (sharing.go,
	// rpg-project#935): empty when the content said nothing, read through
	// [Encounter.Sharing] as [DefaultSharing]. Persisted as written, so an
	// unmarked run writes no key. Inert: no audience is decided by it.
	sharing Sharing

	// granted is every topic the run can grant (recognition.go,
	// rpg-project#928, #929): its interactions' outcomes and its members'
	// lines' grants. A `recognized` requirement or pass on any other is one
	// nobody could ever meet, and refused. Kept so a joiner's grants add to it.
	granted map[TopicID]bool

	// doors are every door in the field, sorted by ID (C8). A door's edges are
	// construction truth; its STATE is the one thing here a verb changes
	// mid-scene, and it is held once for however many edges the door has
	// (rpg-toolkit#1123).
	doors []*doorRecord

	// doorsByID indexes doors by name. The SAME pointers, not a second copy —
	// a door's state has one home however it is reached.
	doorsByID map[DoorID]*doorRecord

	// partitionLinks is the wall-partition graph a sound travels through,
	// derived from the doors on first use and never persisted
	// ([Encounter.links]): its parts are construction truth, so a reload
	// derives the same graph.
	partitionLinks map[PartitionID][]partitionLink

	// partitionDoorways is, for each partition that is a footprint door's
	// own cells, the ways its door joins it to the rooms either side
	// ([Encounter.doorways]): derived with the links, never persisted.
	partitionDoorways map[PartitionID][]partitionLink

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
	intelLog    *perception.Perception
	story       *record.Log
	members     map[MemberID]*memberRecord
	everMembers map[MemberID]bool // Track all members who have ever joined (for Story access)

	// reserve is every member held back by an arrival predicate
	// (rpg-project#375, reserve.go): not on the roster, on no clock, in no
	// projection — its facts wait here and nowhere else until its predicate
	// holds, when it joins members through the one arrival path. Persisted
	// on EncounterData.Reserve; nil for a run with nothing waiting.
	reserve map[MemberID]*reservedMember

	// initiative rolls the order a bubble forms with. Nil until Setup is given
	// one; trigger detection refuses to start a fight without it rather than
	// dropping the fight silently.
	initiative InitiativeRoller
	// roller is THE WORLD'S DIE: the shared dice every pick this composition
	// makes is rolled through, and the one a faction's temperament mix is
	// dealt from (table.go, rpg-project#465).
	//
	// OPTIONAL, AND REFUSED LOUDLY AT THE ROLL when it is absent — the shape
	// [Encounter.initiative] already has, and for its reason. A scene with no
	// temperament mix and no `time` table rolls nothing, so requiring one at
	// every door would make every construction site declare a die it never
	// uses; a scene that DOES roll and was handed none gets [ErrNoRoller] by
	// name rather than a silent default.
	//
	// IT CANNOT BE PER VERB. The round site raises the world clock from inside
	// EndTurn, which takes no die, and a creature's `time` pick happens there
	// — so the die has to be the composition's, not the caller's.
	roller     dice.Roller
	sightAreas map[string]SightArea

	// landed is every beat landed for the listeners since the last raise
	// matched them (wakebeats.go, rpg-project#766 S4). Persisted
	// ([EncounterData.Landed]): a beat landed in a verb that raised nothing
	// waits for the raise that does.
	landed []wakeBeat

	// pickKey is the table key a world pick in progress rolls, set by
	// [Encounter.runPick] for the view it builds; empty is `time`.
	pickKey AnswerKey

	// opportunity identifies the first delivery in a world/reaction budget.
	// Reconsults use a fresh time/turn opportunity, never replay this occurrence.
	opportunity *behaviorOpportunity

	// wakeMatched is how many landed beats the pass in progress has matched,
	// against [wakeBeatBound]; reset by every [Encounter.wakePass].
	wakeMatched int

	// reactionPicked says a reaction in the pass in progress ran a pick, so a
	// pass that raised nothing still owes the sight refresh; reset by every
	// [Encounter.wakePass].
	reactionPicked bool

	// blockingTurned says a transition in the pass in progress took a body
	// into or out of a blocking state (rpg-project#836), so the pass's look
	// runs even on a fight round that raised nothing: what its observers
	// hold of that body's blocking is stale until it does.
	blockingTurned bool

	// stanceFormed is the last fight a stance turn formed
	// ([Encounter.formOnStance]), which no verb's look reports because the
	// turn formed it before the look ran. A bump ([Encounter.bump]) clears
	// it, matches, and reads it, so a fight the barrier's answer formed
	// reaches the bump's output. Nobody else reads it.
	stanceFormed *FormedBubble

	// reactAct is the act a reaction's pick reopens (act.go, Law 2): the act
	// of the verb whose bump the barrier is answering, set by
	// [Encounter.bump] for the match and empty otherwise, so a pick a clock
	// raised mints its own `time:` act as before.
	reactAct string

	// posted is every member admitted with posts (post.go), so the hold is
	// asked of them alone and of nobody on a field where nobody holds. A
	// member that has left is still listed and holds nothing.
	posted map[MemberID]bool

	// sightReads is each held sighting's decoded testimony, by observer,
	// subject and channel, beside the payload it was decoded from
	// (sightread.go, rpg-project#879 S9). Instance state: filled on first
	// read, never persisted.
	sightReads map[sightSlot]sightRead

	// pendingCells is every cell a change touched since the last look
	// (rpg-project#762): a step's two ends, a door's cells, a creature turning
	// where it stands, what a hold or an arrival moved. EVERY LOOK COVERS IT
	// ([Encounter.rebuildPercepts] adds the members within sight of these to
	// whatever observers it was asked for, then clears it), so a change is
	// seen at the first look after it, whichever verb's look that is — the
	// same moment the whole roster's look used to see it. Never persisted: a
	// verb ends with a look, so a saved encounter has nothing pending.
	pendingCells []spatial.Position

	// lastLook is the clock reading of the last look (rpg-project#762). Never
	// persisted: a verb ends with a look, so a loaded encounter's last look is
	// its saved clock high-water ([LoadEncounter]). Every sighting an observer
	// still holds as current was true at that reading — nothing within its
	// sight changed, or it would have been in that look — so a look
	// re-confirms an observer's older current sightings at it before
	// looking ([Encounter.reconfirm]): a memory formed now is stamped with
	// the last reading it was still seen at, as the whole roster's look
	// stamped it.
	lastLook uint64

	// everyoneLooks makes every look the whole roster's, as before
	// rpg-project#762. Set only by a test, as the oracle the bounded look is
	// held to; nothing in this module sets it.
	everyoneLooks bool

	// worldThinking guards [Encounter.wake] against re-entry: the outer pass
	// owns the world's round, and a nested one would step the same machines
	// twice for one raise. The same shape as `driving`, one clock over.
	worldThinking bool

	// due is who acts at which world round (machine.go, rpg-project#766 S3):
	// every member on the world clock whose machine wants time, keyed by
	// member so a round's members pop in seat order. Derived from the blobs
	// at construction and at load ([Encounter.rebuildWake]); never persisted.
	due machine.Due[MemberID]

	// dueAt is the round each member on the due heap is armed at, so a pop
	// knows the round a member steps in: a member due behind the high-water
	// steps at its own round and pops again at the next (the catch-up the
	// census's budget gave). Kept beside due by [Encounter.armAt] and
	// [Encounter.disarm] alone; never persisted.
	dueAt map[MemberID]uint64

	// listeners is who listens for which beat, from every machine's current
	// state ([machine.Program.Listens]). Derived like due and never
	// persisted. The default recipe listens for nothing but time, which the
	// heap answers, so it is empty until a recipe listens (S4).
	listeners machine.Listeners[MemberID]

	// participation answers who is down and who counts as present. Required at
	// both constructors and never defaulted; see [Participation].
	participation Participation

	// sight reports how far each member can see. Required at both constructors
	// — see [Sight] for why it is asked at every refresh rather than held, and
	// why there is no default.
	sight Sight

	// equipment reports what each member is holding. Required at both
	// constructors — see [Equipment] for why it answers truth rather than
	// testimony, and why a nil answer is a fact rather than a gap.
	equipment Equipment

	// conditions reports which conditions each member holds. Required at both
	// constructors — see [Conditions] for why a nil answer is a fact.
	conditions Conditions

	// sheets reports each member's speed, actions and targeting. Required at
	// both constructors — see [Sheets] for why it is asked at every use rather
	// than copied onto the member.
	sheets Sheets

	// turnDriver decides what a member with no player does when the clock
	// lands on their turn. Required at both constructors, for the same reason
	// standing and sight are, and never optional; see
	// [Driver] and ADR-0043.
	driver Driver

	// striker resolves and records an [Attack] intent a Driver returns.
	// Required at both constructors for the same reason turnDriver is; see
	// [Striker].
	striker Striker

	// disengager makes a nimble member Disengage before a walk out of reach.
	// OPTIONAL — nil skips the ask; see [Disengager].
	disengager Disengager

	// mover announces a [Move] intent's steps before this composition takes
	// them. Required at both constructors for the same reason striker is;
	// see [Mover], whose doc explains why announce-before-step is a contract.
	mover Mover

	// announcer publishes the temporal boundaries a clock advance crossed.
	// Required at both constructors for the same reason striker is; see
	// [Announcer], whose doc explains why it is a capability rather than a
	// return value.
	announcer Announcer

	// world is THE ONE WORLD (rpg-project#375, world.go): the one journal
	// every fact of the run lands in — knowledge, holdings — and the one
	// graph folded over it, rebuilt from the field and the roster at every
	// construction and roster change, its facts persisted on
	// EncounterData.Journal. ALWAYS PRESENT: a
	// plain dungeon has sides and knowledge too, and what it skips is the
	// concealment capabilities and the sweep's work, which keeps its blob
	// byte-identical to what it was before.
	world *encounterWorld
	// wakeParticipation is the one participation assessment a wake pass
	// reads (rpg-project#879 S3b): every woken creature asked the host anew,
	// a hundred assessments of the same roster per raise. Set by the first
	// ask inside [Encounter.wakePass], dropped when the pass ends, when an
	// intent other than a Pass has executed and when a member is noticed
	// down, the three moments inside a pass at which the answer can change.
	wakeParticipation *participationState
	discovery         map[MemberID]DiscoveryStateData

	// sneakVerdicts is every standing Stealth verdict, by sneaking mover then
	// by watcher (sneak.go, rpg-project#606 R10): discovery's attempt shape,
	// persisted on EncounterData.SneakVerdicts, dropped when the mover stops
	// sneaking.
	sneakVerdicts map[MemberID]map[MemberID]SneakVerdictData

	// sightSweeps is each observer's key at its last sight sweep
	// (sightsweep.go, rpg-project#664): an observer whose key is unchanged
	// skips the sweep, because it would learn nothing. Persisted on
	// EncounterData.SightSweeps; derived, never a second record of what
	// anybody knows — the journal is that.
	sightSweeps map[MemberID]string

	// sightSweepsRun counts the sweeps each observer actually ran in this
	// process, for the tests that prove the skip. Never persisted.
	sightSweepsRun map[MemberID]int

	// visibleCache is the floor seen from a cell, by sweep key (rpg-project#879
	// S2, R4): a bitset over the sweep disk in [field.sweepCells] order, kept
	// across passes. THE KEY IS THE SOUNDNESS: it fingerprints the observer's
	// cell, range and concealment view and the field's opacity (doors, placed
	// footprints, opaque entities, obscuring areas), every input seesCell can
	// see change; members never block sight. Since rpg-project#879 S5 it
	// holds only the CONCEALMENT LAYER: keys whose observer has a door or prop
	// hidden from it. A key with no hidden face is the site's raw answer and
	// lives on the site ([siteSight]), shared by every game on the content and
	// kept across loads. Bounded by visibleCacheEntries, never saved, rebuilt
	// by use after a load.
	visibleCache map[sightSweepKey]visibleBits
	// visibleCacheEntries is visibleCache's bound, [visibleCacheBound] from
	// every constructor. The encounter's own, so a test that shrinks it
	// shares it with no other encounter (rpg-toolkit#2317). An encounter
	// comes from [NewEncounter] or [LoadEncounter]: a zero-value Encounter
	// has a bound of 0 and drops its cache on every sweep.
	visibleCacheEntries int

	// sightEncoder replaces [EncodeSightTestimony] in [Encounter.encodeSighting]
	// so a test can count the calls (#1691). The whole point of moving the
	// loop into mind/perception is that the count is N for N members rather
	// than N² — identical bytes, encoded once, handed to every observer who
	// perceives them — and a cost nothing asserts is a cost that comes back
	// the next time somebody reaches for a nested loop. Nil outside a test.
	// A field, not a package variable, so a test's encoder is never read by
	// another encounter running beside it (rpg-toolkit#2317).
	sightEncoder func(SightTestimony) ([]byte, error)
	// rebuildProbe is told every observer list [Encounter.rebuildPercepts] is
	// asked to rebuild (rpg-project#762), so a test can say a member beyond
	// the sight bound of what changed was never looked for, for the same
	// reason as sightEncoder. Nil outside a test; the encounter's own for the
	// same reason (rpg-toolkit#2317).
	rebuildProbe func(observers []MemberID)
	// visibleEpoch is the opacity the cached entries were swept under; a
	// pass with a new opacity frees them, since their keys can no longer match.
	visibleEpoch string
	// visibleHits and visibleMisses count the cache's answers for the tests
	// and the harness.
	visibleHits, visibleMisses int

	// holdings is WHO HAS WHAT: a reader over the world's journal for the
	// holdings, takings and drops (rpg-project#368, design §5). An
	// encounter where nobody holds anything writes no such facts and no
	// bytes.
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

	// pause is the one walk stopped mid-route because a reactor is being
	// asked about a step — a driven turn ([PauseTurn]) or a directed walk
	// ([PauseDirective]); see [pause]. Nil whenever the fight is not waiting on
	// anybody, which is almost always.
	//
	// PERSISTED, unlike driving above, and that is the difference between
	// them: driving is a fact about a Go call stack, which a reload does not
	// have; a pause is a fact about the FIGHT, which survives rpg-api being
	// restarted between the question and the answer. It travels in the blob
	// as EncounterData.Pause.
	//
	// ONE SLOT. A fight waits on one answer at a time, so there is nowhere to
	// put a second pause and nothing to keep two of them exclusive.
	// [Encounter.Direct] refuses at the door while one is held, and while one
	// is set every drive entry is a no-op — there is exactly one way forward
	// and it is [Encounter.Resume].
	pause *pause

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

	// floodsComputed counts the distance fields this run has actually
	// flooded, memo hits excluded (rpg-toolkit#2097). It is a measure, read
	// by the package's own tests, and never decides anything.
	floodsComputed int

	// cellsSettled counts the cells the goal-directed search
	// ([cellQuery.searchToward]) has settled this run (rpg-project#762, R10):
	// the measure a route's reach is held to. Like floodsComputed, read by
	// the package's own tests and never deciding anything.
	cellsSettled int
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
//
// THE ACT ID IS STAMPED HERE (act.go): a beat that names no correlation of
// its own carries the open act's id as its correlation and as the [ActTag]
// tag, so no verb has to remember to.
func (e *Encounter) appendBeat(in *record.AppendInput) (*record.AppendOutput, error) {
	if in.Correlation == "" {
		act := e.CurrentAct()
		in.Correlation = act
		if act != "" {
			tags := make(map[string]string, len(in.Tags)+1)
			for k, v := range in.Tags {
				tags[k] = v
			}
			tags[ActTag] = act
			in.Tags = tags
		}
	}
	return e.story.Append(in)
}

// NextStorySeq returns the global sequence the next successful story append
// will use. It is a read: it does not reserve, increment, or append anything.
// Encounter ingress is single-command in v1; this method makes no promise for a
// future host that allows concurrent writers between this read and RecordTrain.
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
		if err := f.validateLeaveEnding(ei); err != nil {
			return err
		}
		// A MemberDown ending must name a member — see TriggerMemberDown's
		// doc for why empty is refused rather than defaulted.
		if md, ok := ei.Trigger.(TriggerMemberDown); ok && md.Member == "" {
			return fmt.Errorf("ending %q names no member: %w", ei.Key, ErrNoEnding)
		}
		// An ExitedHolding ending must name an exit this field declares and
		// a prop it declares HOLDABLE. Either one missing is an ending that
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
			if !f.holdable[eh.Item] {
				return fmt.Errorf("ending %q waits for %q to be held, and no prop with that id is holdable: %w",
					ei.Key, eh.Item, ErrNoEnding)
			}
		}
		// The predicate grammar's own forms (rpg-project#375) — a round, a
		// fact, a stance — through the one validator dispositions use, so an
		// ending and an until refuse the same shapes in the same words.
		switch t := ei.Trigger.(type) {
		case TriggerNot:
			// A `not` of a `left` is a leave ending, judged by
			// validateLeaveEnding above; every other `not` is a waited-for
			// predicate like the forms below.
			if _, left := t.Inner.(TriggerLeft); left {
				break
			}
			if err := f.validatePredicate(fmt.Sprintf("ending %q", ei.Key), ei.Trigger, ErrNoEnding); err != nil {
				return err
			}
		case TriggerRound, TriggerFact, TriggerStance, TriggerHolds, TriggerInteraction, TriggerRecognized:
			if err := f.validatePredicate(fmt.Sprintf("ending %q", ei.Key), ei.Trigger, ErrNoEnding); err != nil {
				return err
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

// validateIntelTargets rejects a record whose target this field does not
// have: a record revealing a concealment nobody declared can never tell
// anybody anything, which is the same dead-declaration hole [ErrNoEnding]
// refuses for an ending naming an unreachable cell.
//
// Checked identically at Setup and Load, against the compiled field, with no
// verb prefix — each caller wraps its own at the call site.
//
// THE TARGET IS A CONCEALMENT NOW, NOT A DOOR (rpg-project#490, R7), and
// that turned an inert declaration into a refused one. A record revealing an
// UNCONCEALED door used to be legal and do nothing; there is no such record
// any more, because the only thing a record can reveal is a secret, and
// naming one this field does not have is the author's mistake rather than a
// harmless line.
func validateIntelTargets(f *field) error {
	for _, rec := range f.intel {
		if rec.Reveals.Concealment == "" {
			continue
		}
		if f.concealmentOf(rec.Reveals.Concealment) == nil {
			return fmt.Errorf("intel record %q reveals concealment %q, which this field does not declare: %w",
				rec.ID, rec.Reveals.Concealment, ErrNoConcealment)
		}
	}
	return nil
}

// NewEncounter constructs and initializes an encounter from SetupInput.
// Validation order (first failure wins, R5 atomicity): nil input, no
// endings, empty-or-reserved ending key, duplicate ending key, empty member
// ID, duplicate member IDs, a member fact that makes no sense (design law
// C2), negative member facts, then the field (compileField: the canvas's
// declarations, region defects, props, walls), doors (validateDoorInputs),
// member seats (integral, on floor), ending trigger validity, spatial
// placement errors.
func NewEncounter(in *SetupInput) (*Encounter, error) {
	return newEncounter(in, visibleCacheBound)
}

// newEncounter is [NewEncounter] with the visible cache's bound named, so a
// test builds its encounter with a small bound before any pass reads it
// (rpg-toolkit#2317). Every other caller passes [visibleCacheBound].
func newEncounter(in *SetupInput, visibleCacheEntries int) (*Encounter, error) {
	// Validation order: nil, no rooms, no endings, reserved ending, empty ID, duplicates
	if in == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNilInput)
	}

	if len(in.Endings) == 0 {
		return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
	}

	// Every capability is required, because construction is total (S8):
	// trigger detection and the standing, sight, equipment and sheet consults
	// run from first light onward, so an encounter that can hold players and
	// monsters can start a fight before its caller does anything. Refusing
	// here rather than mid-fight is the difference between a bug report and a
	// bug. Never defaulted (rpg-toolkit#1033); see [Capabilities] for each
	// member's reason and the order they are refused in.
	//nolint:staticcheck // QF1008: spelled out, because SetupInput has no Validate of its own and a promoted one would read as if it did
	if err := in.Capabilities.Validate(); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
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

	// Check member IDs: empty or duplicate; validate the member facts
	seenIDs := make(map[MemberID]bool)
	for _, m := range in.Members {
		if m.ID == "" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("newencounter: duplicate member %s: %w", m.ID, ErrDuplicateMember)
		}
		seenIDs[m.ID] = true

		if err := validateMemberFacts(memberFacts{
			ID:                 m.ID,
			AppearanceRef:      m.AppearanceRef,
			DefaultActivityRef: m.DefaultActivityRef,
			Facing:             m.Facing,
			Options:            m.Options, Table: m.Table, Machine: m.Machine, Behaviors: m.Behaviors,
			BlockingStates: m.BlockingStates, Posts: m.Posts,
		}); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
	}

	// THE SITE (site.go, rpg-project#879 S5): the field compiled once per
	// content and shared — the canvas's two declarations, the regions, the
	// props and walls, the doors' geometry, the partitions, the doors a
	// concealment names and the records' targets, every rule about what a
	// field may be stated once in [CompileSite] and shared with Load. The
	// encounter runs on its own shallow copy of the site's field.
	site, err := resolveSite(in.Sites, &SiteInput{Field: &in.Field})
	if err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}
	f := site.instanceField()

	// THE RUN'S INTERACTIONS (interaction.go, rpg-project#922): authored on
	// the field input, owned by the run, never site data.
	interactions, err := compileInteractions(in.Field.Interactions)
	if err != nil {
		return nil, fmt.Errorf("newencounter: %w: %w", err, ErrNoField)
	}

	// THE PARTY'S SHARING DIAL (sharing.go, rpg-project#935): the run's,
	// never site data. Any word of the dial is carried — shared knowledge
	// too, since building is also a preview's and a projection's; launching
	// a run with it is refused by the host, by [LaunchableSharing].
	if err := validateSharing(in.Field.Sharing); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// A line requiring a holding must name a prop this field lets somebody
	// pick up (option.go) — asked against the compiled field, so after it —
	// and a line naming an interaction must name one this run has.
	memberLines := make([][]Option, 0, len(in.Members))
	for _, m := range in.Members {
		memberLines = append(memberLines, m.Options)
	}
	granted := grantedTopics(interactions, memberLines...)
	for _, m := range in.Members {
		if err := f.validateOptionHolds(m.Options); err != nil {
			return nil, fmt.Errorf("newencounter: member %s: %w: %w", m.ID, err, ErrBadMember)
		}
		if err := validateOptionInteractions(interactions, granted, m.Options); err != nil {
			return nil, fmt.Errorf("newencounter: member %s: %w: %w", m.ID, err, ErrBadMember)
		}
	}
	if err := validatePropInteractions(interactions, granted, f); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}
	if err := validateArrests(interactions, f); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// The two concealment capabilities, required exactly when the field
	// declares a concealment (rpg-toolkit#1371) — the same
	// supplied-never-defaulted law as the four above, scoped to the fields
	// that consult them: a plain dungeon needs neither, and a concealing one
	// refused here is a bug report instead of a secret nobody can ever
	// find. AFTER the field and door validation on purpose: a malformed
	// door is the author's earlier mistake, and refusing it as a missing
	// capability would send them to the wrong seam.
	if fieldHasConcealment(in.Field.Concealments) {
		if err := in.validateConcealed(); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
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
		// STANDABLE INCLUDES UNCOVERED (issue #1753): the same standing fact
		// Join asks, at Setup — nobody spawns inside a table the author also
		// placed, whichever seam placed them.
		if prop, covered := f.standingBlocks(f.cellAt(mi.Position)); covered {
			return nil, fmt.Errorf("newencounter: member %q position [%g,%g] is covered by placed prop %q: %w",
				mi.ID, mi.Position.X, mi.Position.Y, prop, ErrBadPlacement)
		}
	}

	// A holder must name a record the field declares (design §2); a
	// record's own target was checked when the site compiled. Refused here,
	// before anything is built (R5): a dangling one is knowledge the author
	// thinks they placed and did not.
	for _, mi := range in.Members {
		for _, id := range mi.Holds {
			if _, declared := f.intelByID[id]; !declared {
				return nil, fmt.Errorf("newencounter: member %q holds intel %q: %w", mi.ID, id, ErrNoIntel)
			}
		}
	}
	// A PROP may hold records too (R6), and a dangling one is the same
	// mistake wherever it is written.
	for _, prop := range f.props {
		for _, id := range prop.Holds {
			if _, declared := f.intelByID[id]; !declared {
				return nil, fmt.Errorf("newencounter: prop %q holds intel %q: %w", prop.Ref, id, ErrNoIntel)
			}
		}
	}
	// AND SO MAY A PLACED FOOTPRINT (rpg-toolkit#1854). Named by its id
	// rather than by a ref, because a placement has none — the id is what an
	// author wrote under propBindings and what they are looking at.
	for i := range f.placed {
		for _, id := range f.placed[i].holds {
			if _, declared := f.intelByID[id]; !declared {
				return nil, fmt.Errorf("newencounter: prop %q holds intel %q: %w", f.placed[i].id, id, ErrNoIntel)
			}
		}
	}

	// A member's faction must be one this field has, and a faction's mind
	// must be in that faction (rpg-project#375) — asked here, before
	// anything is built (R5), and asked again by Join for the members that
	// arrive later, which is how every monster actually enters a run.
	for _, mi := range in.Members {
		if err = f.validateMemberFaction(mi.ID, mi.Kind, mi.Faction); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
		// And a member held in reserve must be one that can wait there and
		// one whose predicate can hold (reserve.go) — the same refusal Join
		// makes, at the same point: before the first mutation.
		if mi.Arrives != nil {
			if err = f.validateArrival(mi.ID, mi.Kind, mi.Arrives); err != nil {
				return nil, fmt.Errorf("newencounter: %w", err)
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
		visibleCacheEntries: visibleCacheEntries,
		members:             make(map[MemberID]*memberRecord),
		everMembers:         make(map[MemberID]bool),
		field:               f,
		site:                site,
		initiative:          in.Initiative,
		participation:       in.Participation,
		sight:               in.Sight,
		equipment:           in.Equipment,
		conditions:          in.Conditions,
		sheets:              in.Sheets,
		driver:              in.Driver,
		roller:              in.Roller,
		striker:             in.Striker,
		disengager:          in.Disengager,
		mover:               in.Mover,
		announcer:           in.Announcer,
		endings:             nil,
		retention:           normalizeRetention(in.Retention),
		interactions:        interactions,
		granted:             granted,
		sharing:             in.Field.Sharing,
	}
	e.doors, e.doorsByID = doorRecordsFrom(in.Field.Doors)

	// THE ONE WORLD (rpg-project#375, world.go): its journal now, empty, so
	// the seeds below have somewhere to land; its graph once the endings are
	// known, because the facts they wait for are part of its declaration.
	// Holdings are a reader over the same journal. Both start empty and stay
	// empty for a field nobody learns or carries anything in, which is what
	// keeps such a field's blob byte-identical to what it was before.
	e.world = newEncounterWorld()
	e.holdings = newHoldings(e.world.log)

	// AND THE FIELD IS GIVEN THE SAME READER (rpg-toolkit#1854). A placed
	// footprint can be picked up and can wait in reserve, so the four queries
	// that measure rectangles have to be able to ask where each one is right
	// now — the reader itself, never a snapshot, exactly as the doors are
	// held as their records (placed_props.go).
	f.attachHoldings(e.holdings)

	// The two concealment capabilities, REQUIRED exactly when the field
	// declares a concealment (rpg-toolkit#1371); the world exists either way.
	// The check resolver is HELD whenever it is supplied, because a sneaking
	// mover's Stealth roll asks it on any field (sneak.go, rpg-project#606);
	// a field that hides nothing and sees no sneaker never calls it.
	e.checkResolver = in.CheckResolver
	if fieldHasConcealment(in.Field.Concealments) {
		e.witness = in.Witness
	}

	// Build clock and intel
	e.clock, err = clock.NewTick()
	if err != nil {
		return nil, fmt.Errorf("newencounter clock: %w", err)
	}

	e.intelLog, err = perception.New()
	if err != nil {
		return nil, fmt.Errorf("newencounter intel: %w", err)
	}

	e.story, err = record.NewLog(record.WithIDSource(in.IDs))
	if err != nil {
		return nil, fmt.Errorf("newencounter story: %w", err)
	}

	// Compile the field into the one canvas this encounter runs on. Nothing
	// has arrived yet: a prop with a predicate stays off the canvas
	// (reserve.go).
	e.canvas, err = f.compileCanvas(e.doors, nil)
	if err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Place members and collect them
	memberIDs := make([]MemberID, 0, len(in.Members))
	for _, mi := range in.Members {
		member := &memberRecord{
			ID:                 mi.ID,
			Kind:               mi.Kind,
			Name:               mi.Name,
			AppearanceRef:      mi.AppearanceRef,
			DefaultActivityRef: mi.DefaultActivityRef,
			Facing:             seedFacing(mi.Facing),
			Options:            cloneOptions(mi.Options),
			Table:              cloneTable(mi.Table),
			Temper:             mi.Temper,
			BlocksMovement:     mi.BlocksMovement,
			Faction:            mi.Faction,
		}
		if err = e.useBehaviors(member, mi.Behaviors, nil, mi.BlockingStates); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
		if err = e.usePosts(member, mi.Posts, f.cellAt(mi.Position)); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}

		// A MEMBER WITH A PREDICATE WAITS IN RESERVE (rpg-project#375,
		// reserve.go): its facts are kept and nothing else happens — no
		// cell, no clock, no seed, no percept, not in the scene-opened
		// audience. Everything below is for the members who are here.
		if mi.Arrives != nil {
			e.reserveMember(&reservedMember{
				record: *member, at: f.cellAt(mi.Position), holds: mi.Holds,
				arrives: mi.Arrives,
			})
			continue
		}
		memberIDs = append(memberIDs, mi.ID)

		entity := e.newMemberEntity(mi.ID, mi.Kind)

		// Authored offset at the seat, absolute on the canvas: converted
		// through the field's one conversion, exactly as every region cell
		// was. Floor was checked above.
		if err = e.canvas.PlaceEntity(entity, f.cellAt(mi.Position)); err != nil {
			return nil, fmt.Errorf("newencounter member placement: %w: %w", ErrBadPlacement, err)
		}

		e.members[mi.ID] = member
		e.everMembers[mi.ID] = true // Track in everMembers

		// Every member starts on the world clock. Free roam is not a mode, it
		// is simply where you are when no fight has pulled you elsewhere.
		if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(mi.ID)}); cerr != nil {
			return nil, fmt.Errorf("newencounter member %q world clock: %w", mi.ID, cerr)
		}

		// The author's placed records, seeded as the holdings they are
		// (design §3). SETUP ONLY — Load replays the journal instead, so
		// intel somebody already looted is not handed back to the body.
		if err = e.holdings.seedIntel(mi.ID, mi.Holds); err != nil {
			return nil, fmt.Errorf("newencounter: %w", err)
		}
	}

	// Store endings in declaration order (deterministic evaluation, C8), each
	// positional one compiled to the canvas cell it fires on.
	e.endings = compileEndings(in.Endings, f)

	// The graph, declared from the field, the roster and the endings — the
	// sides, the belonging, the knowledge reducers, the flips (world.go).
	if err = e.buildWorld(); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	// Who is due, from every member's fresh machine (machine.go).
	if err = e.rebuildWake(); err != nil {
		return nil, fmt.Errorf("newencounter: %w", err)
	}

	for _, mi := range in.Members {
		if mi.Kind == KindPlayer {
			if err = e.initializeDiscovery(mi.ID, mi.PrivateDiscoveries); err != nil {
				return nil, err
			}
		}
	}

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
	beatPayload, _ := json.Marshal(map[string]string{"beat": BeatSceneOpened})
	_, err = e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(tableBeat, memberIDs...),
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatPayload,
	})
	if err != nil {
		return nil, fmt.Errorf("newencounter append beat: %w", err)
	}

	// First light's own sighting beats, AFTER the scene has opened for the
	// same reason everything else here is: a story in which members see each
	// other before the scene exists is one nobody can follow. Setup holds the
	// two halves of a refresh apart precisely so this beat and the
	// scene-opened beat land in the right order, which refreshSight's single
	// call cannot express.
	//
	// FIRST LIGHT EMITS, it does not skip. Every awareness that exists was
	// created by some refresh and this is the first one, so a recipient who
	// applies nothing but these beats has the whole picture from the opening
	// beat onward — which is the property the perception stream is for.
	if serr := e.appendSightedBeats(firstLight, nil, uint64(e.clock.ToData().HighWater)); serr != nil {
		return nil, fmt.Errorf("newencounter first light: %w", serr)
	}

	// THE FACTION'S MIXES ARE DEALT HERE, once each, and here is AFTER the
	// scene has opened (design §3, worldtime.go). Each deal is a beat — that
	// is its whole point, so the streamer sees which goblin came out the
	// coward — and a beat inside a scene that has not opened yet is one
	// nobody can follow, which is the law every other beat in this
	// constructor keeps. An authored word deals nothing and writes nothing.
	for _, id := range memberIDs {
		member := e.members[id]
		dealt, terr := e.dealTemperFor(id, member.Temper, 0)
		if terr != nil {
			return nil, fmt.Errorf("newencounter: %w", terr)
		}
		member.Temper = dealt
	}

	// Concealment's own first light, AFTER the scene has opened and BEFORE
	// any fight it might start: presence pierces from the first frame — a
	// party start on a concealment's floor is legal authoring, and the
	// occupants begin knowing — and a hidden door authored OPEN is
	// perceivable from frame one too. A no-op for a field that hides
	// nothing.
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

// View returns the member's current intel holdings, in the form this module
// delivers: sight testimony without the conditions it saw, which rules read
// through [Encounter.ObservedContext] and which never leave the toolkit as a
// payload (see [deliveredSightPayload]).
// Returns ErrNilInput, ErrNotMember if the member is not part of this
// encounter, or ErrInvalidData for stored testimony that cannot be delivered.
func (e *Encounter) View(in *ViewInput) ([]perception.Holding, error) {
	holdings, err := e.storedView(in)
	if err != nil {
		return nil, err
	}
	if holdings, err = deliveredHoldings(holdings); err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}
	return holdings, nil
}

// storedView is [Encounter.View] before delivery: the member's holdings as
// stored, conditions included, for readers inside this module.
func (e *Encounter) storedView(in *ViewInput) ([]perception.Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("view: %w", ErrNilInput)
	}

	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("view: %w", ErrNotMember)
	}

	holdings, err := e.memberIntel(in.Member)
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
		Facing:         record.Facing,
		Table:          cloneTable(record.Table),
		Temper:         record.Temper,
		BlocksMovement: e.memberBlocks(record.ID),
		Faction:        factionOf(record),
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
				Result:  e.outcome.Result,
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
// Returns ErrNilInput if the input is nil, ErrNoMember for an empty audience,
// ErrNotMember if the member never joined, and ErrTrimmed if a non-zero AfterSeq names a sequence that has
// already aged out of the retention window — the caller must resync rather
// than resume, since a short answer would be indistinguishable from a complete
// one. AfterSeq == 0 is exempt and always answerable.
// Copy-out follows record's own conventions (returned entries are already copies
// per record's implementation).
func (e *Encounter) Story(in *StoryInput) ([]record.Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}

	if in.Audience == "" {
		return nil, fmt.Errorf("story: %w", ErrNoMember)
	}
	if _, ok := e.everMembers[in.Audience]; !ok {
		return nil, fmt.Errorf("story %q: %w", in.Audience, ErrNotMember)
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
	if !in.WholeActs || in.AfterSeq == 0 || len(entries) == 0 {
		return entries, nil
	}

	from, err := e.wholeActsFrom(in.Audience, in.AfterSeq, entries)
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}
	if from >= in.AfterSeq {
		return entries, nil
	}
	entries, err = e.story.SliceFor(&record.SliceForInput{Viewer: in.Audience, FromSeq: from})
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}
	return entries, nil
}

// wholeActsFrom is where [StoryInput.WholeActs] begins a slice: the seq of
// the audience's first entry of the earliest act in delta that began before
// afterSeq, or afterSeq when none did. delta is the audience's entries at or
// after afterSeq, non-empty.
//
// Only a REOPENED act is looked up: one whose id sorts below the record's
// entry at afterSeq-1, whoever it was for. An act's id is minted from the
// record's increasing id space before its first beat, so an act whose id
// sorts above that entry was opened inside the delta and has no beat before
// it. A tag that is not `<verb>:<record id>` (a beat written before the v2
// ids) is looked up too, since its id says nothing about when it began.
func (e *Encounter) wholeActsFrom(audience MemberID, afterSeq uint64, delta []record.Entry) (uint64, error) {
	from := afterSeq
	before := ""
	beforeRead := false
	looked := map[string]bool{}
	for _, entry := range delta {
		act := entry.Tags[ActTag]
		if act == "" || looked[act] {
			continue
		}
		looked[act] = true
		if i := strings.IndexByte(act, ':'); i >= 0 && isRecordID(act[i+1:]) {
			if !beforeRead {
				id, err := e.recordIDAt(afterSeq - 1)
				if err != nil {
					return 0, err
				}
				before, beforeRead = id, true
			}
			if before == "" || act[i+1:] > before {
				continue
			}
		}
		beats, err := lookupActBeats(e, act)
		if err != nil {
			return 0, err
		}
		for _, beat := range beats {
			if beat.Seq >= from {
				break
			}
			if slices.Contains(beat.Audience, core.EntityID(audience)) {
				from = beat.Seq
				break
			}
		}
	}
	return from, nil
}

// recordIDAt is the id of the retained entry at seq, whoever it was for, or
// "" when none is retained. It scans by seq and copies only the entries from
// seq on — the verb's own, when seq is just below its baseline.
func (e *Encounter) recordIDAt(seq uint64) (string, error) {
	tail, err := e.story.All(&record.AllInput{FromSeq: seq})
	if err != nil {
		return "", err
	}
	if len(tail) == 0 || tail[0].Seq != seq {
		return "", nil
	}
	return tail[0].ID, nil
}

// lookupActBeats is every retained entry of one act, whoever it was for: the
// whole-record scan [Encounter.wholeActsFrom] pays only for a reopened act.
// A variable so a test can count the scans.
var lookupActBeats = func(e *Encounter, act string) ([]record.Entry, error) {
	return e.story.All(&record.AllInput{Tags: map[string]string{ActTag: act}})
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

// observersOf is A REFRESH THAT COSTS ITS REACH (rpg-project#762, the law
// "a step costs its reach"): every placed member whose own sight, plus the
// one cell of a cover's face ([sightReach.seesCell]), reaches any of cells,
// with declared added, in ID order.
//
// cells are what changed: where a mover stood and stands, a door's cells, an
// arrival's cell. A member farther than its own sight from every one of them
// cannot see anything that changed — not the mover, not a ray the mover or
// the door now crosses, since every cell on a ray is nearer the observer than
// its far end — so its percept is the one it already holds, and rebuilding it
// would only restamp it. A fight forms the same: whoever came into view did so
// within the bound of a changed cell, by definition.
//
// ASKED OF THE CANVAS, not of the roster: the entities within the widest
// sight anybody has this refresh of each changed cell, each hit then held to
// its own ([Encounter.observersWithin] says what that costs). declared are
// added whether or not they are placed — [Encounter.rebuildPercepts] drops an
// unplaced observer itself. So is anybody [Encounter.addOutranged] names.
//
// The roster-wide look stays legal in one place: [Encounter.Board], the
// launch's one look, where everything changed at once.
func (e *Encounter) observersOf(cells []spatial.Position, declared []MemberID) ([]MemberID, error) {
	if len(e.members) == 0 {
		return []MemberID{}, nil
	}
	sight, err := e.sightNow()
	if err != nil {
		return nil, fmt.Errorf("observers of: %w", err)
	}

	return e.observersWithin(cells, declared, sight)
}

// observersWithin is [Encounter.observersOf] over a range reading the caller
// already took — a look's own, so it does not ask the capability twice.
//
// ONE DISTANCE PASS OVER THE CANVAS PER CHANGED CELL, not an index:
// spatial's GetEntitiesInRange walks every positioned entity, props included,
// so the cost is changed cells × canvas entities. Cheap beside a rebuild
// today (the refresh is ~10% of a keep Move, the patrols' route floods the
// rest); a spatial index on the canvas is the next thing to bound if it ever
// is not.
func (e *Encounter) observersWithin(
	cells []spatial.Position, declared []MemberID, sight map[MemberID]int,
) ([]MemberID, error) {
	in := make(map[MemberID]bool, len(declared))
	for _, id := range declared {
		if _, ok := e.members[id]; ok {
			in[id] = true
		}
	}
	if err := e.addOutranged(in, sight); err != nil {
		return nil, fmt.Errorf("observers of: %w", err)
	}
	widest := 0
	for _, cells := range sight {
		widest = max(widest, cells)
	}
	grid := e.canvas.GetGrid()
	asked := make(map[spatial.Position]bool, len(cells))
	for _, cell := range cells {
		if asked[cell] {
			continue
		}
		asked[cell] = true
		for _, entity := range e.canvas.GetEntitiesInRange(cell, float64(widest+1)) {
			id := MemberID(entity.GetID())
			if _, member := e.members[id]; !member || in[id] {
				continue
			}
			at, placed := e.canvas.GetEntityPosition(entity.GetID())
			if placed && grid.Distance(at, cell) <= float64(sight[id]+1) {
				in[id] = true
			}
		}
	}
	out := make([]MemberID, 0, len(in))
	for id := range in {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out, nil
}

// noteChanged records cells a change touched for the next look
// ([Encounter.pendingCells]).
func (e *Encounter) noteChanged(cells ...spatial.Position) {
	e.pendingCells = append(e.pendingCells, cells...)
}

// addOutranged adds to in every member holding a LIVE sighting of somebody
// now beyond its own sight (rpg-project#762): a range that shrank since the
// last look — the torch guttered — is a change at the observer, not at any
// cell, and the observer's own holdings are where it shows. The composition
// keeps no range to compare against ([Sight]: it is a pull, never stored), so
// the testimony it already holds is the witness. A range that GREW is seen
// at the next look that includes the observer: one within its new sight of
// anything that changed, or one a verb names (equip and recheck name theirs).
func (e *Encounter) addOutranged(in map[MemberID]bool, sight map[MemberID]int) error {
	grid := e.canvas.GetGrid()
	for id := range e.members {
		if in[id] {
			continue
		}
		at, placed := e.canvas.GetEntityPosition(string(id))
		if !placed {
			continue
		}
		// The store's own holdings, [Encounter.memberIntel]'s filter applied
		// in place: a sighting of anything but a member is not asked.
		observer := sightMember(id)
		held, err := e.intelLog.Held(observer)
		if err != nil {
			return err
		}
		for _, h := range held {
			if !h.CurrentOn(perception.Sight) {
				continue
			}
			if h.Channel == perception.Sight {
				if _, member := subjectID(h.Subject, memberSubjectKind); !member {
					continue
				}
			}
			seen, ok := e.heldSight(observer, h)
			if !ok || seen.State != LocationKnown {
				continue
			}
			if grid.Distance(at, seen.Position) > float64(sight[id]) {
				in[id] = true
				break
			}
		}
	}

	return nil
}

// refreshCostingReach is the refresh a verb runs for what it changed: the
// cells go to [Encounter.pendingCells], and the look asks the members within
// sight of them ([Encounter.rebuildPercepts]). declared here are extra
// OBSERVERS (the actor), never subjects whose facts changed — that is
// [Encounter.refreshSightDeclaring]'s list.
func (e *Encounter) refreshCostingReach(
	cells []spatial.Position, declared ...MemberID,
) (map[MemberID]*IntelDelta, *FormedBubble, error) {
	e.noteChanged(cells...)

	return e.refreshSight(declared)
}

// cellsOf is where each placed member of ids stands now; an unplaced one has
// no cell to add.
func (e *Encounter) cellsOf(ids ...MemberID) []spatial.Position {
	out := make([]spatial.Position, 0, len(ids))
	for _, id := range ids {
		if cell, placed := e.canvas.GetEntityPosition(string(id)); placed {
			out = append(out, cell)
		}
	}

	return out
}

// frontierAudience is THE FRONTIER STOP (ruled on rpg-project#351, second
// round): a step whose destination lies on floor a concealment hides is
// delivered only to the members that concealment has been revealed to — the
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
	c := e.concealmentOnCell(action.to)
	if c == nil {
		return audience
	}
	out := make([]MemberID, 0, len(audience))
	for _, id := range audience {
		if id == action.member.ID || e.world.knowsConcealment(id, c.id) {
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
func (e *Encounter) appendMovementBeat(action executedAction, audience []MemberID, at uint64) (*record.AppendOutput, error) {
	audience = e.frontierAudience(action, audience)
	// Both ends of a step are cells the next look is asked of
	// ([Encounter.pendingCells]), whoever's look that is.
	e.noteChanged(action.from, action.to)

	payload := map[string]interface{}{
		"beat":     BeatMoved,
		"member":   string(action.member.ID),
		"position": action.to,
	}
	// WHY THEY MOVED, when something moved them (rpg-project#430). A step a
	// creature chose carries no cause and the key is absent — the beat says
	// what it has always said. A directed one names the effect, reduced to its
	// canonical module:type:id string like every other ref this composition
	// persists (roll_trace.go's header), so a client reads a word rather than
	// an object it would have to know the shape of.
	//
	// FULL DATA, per the pre-v1 rule (rpg-project#260 slice 4): the ref goes
	// down the log to everyone. WHICH effect an observer is entitled to know
	// is a per-observer question and it is #940's, not this beat's.
	if cause := action.cause; cause.IsValid() == nil {
		payload["cause"] = cause.String()
	}
	// THE BEAT NAMES ITS KIND (rpg-project#825, design R1): `compelled` or
	// `displaced`, absent for own, so every own record keeps the shape it
	// always had and an absent kind on a stored beat means own.
	//
	// `forced` IS A DEPRECATED MIRROR, written exactly when the kind is
	// displaced (rpg-toolkit#2227's key), so a reader that does not know
	// `kind` keeps telling a push from a walk.
	if word := action.kind.beatWord(); word != "" {
		payload["kind"] = word
	}
	if action.kind.Displaced() {
		payload["forced"] = true
	}
	if len(action.doors) > 0 {
		// A step that went through a door names it. The BEAT does not
		// change — it is still "moved", because that is what happened
		// (rpg-toolkit#1106): a crossing stopped being a second kind of
		// movement when the field stopped being a set of rooms, and the
		// connection name that used to ride here went with the room chain
		// (rpg-project#256) — a doorway is the door standing in it.
		//
		// EXCEPT A HIDDEN ONE (rpg-toolkit#1371). This beat is audienced
		// to the whole roster, and one shared payload cannot say a secret
		// to knowers without saying it to everyone — so a door a
		// concealment holds never rides it, found or not: the mover's own
		// crossing writes their recipient-scoped CONCEALMENT_REVEALED, and
		// what a door is doing reaches its knowers through its own beats.
		// The move itself stays narrated for everyone, doors or no doors.
		ids := make([]string, 0, len(action.doors))
		for _, d := range action.doors {
			if _, hidden := e.hiddenDoorConcealment(d.ID); hidden {
				continue
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
		return nil, fmt.Errorf("marshal moved beat: %w", err)
	}

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: audience,
		Tags:     map[string]string{"tag": "movement"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, err
	}
	if err := e.appendStepAreaTransitions(action.member.ID, action.from, action.to); err != nil {
		return nil, err
	}
	return appendOut, nil
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
// errNoEnding is firedReachedPosition's news that no ending fired at this
// position: the member came to rest and nothing happened, which the caller
// reads as no outcome on its output (#2220: an error may carry the news, not
// the blame).
var errNoEnding = errors.New("no ending fired here")

func (e *Encounter) firedReachedPosition(member *memberRecord, cell spatial.Position, at uint64) (*Outcome, error) {
	if e.outcome != nil {
		members := make([]MemberOutcome, len(e.outcome.Members))
		copy(members, e.outcome.Members)
		return &Outcome{Ending: e.outcome.Ending, Result: e.outcome.Result, At: e.outcome.At, Members: members}, nil
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
	return nil, errNoEnding
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
	return e.closeWithEnded(key, at, nil, audience...)
}

// heldEnding is the ending a train's deed asked for, parked until the train's
// last unit has been told.
type heldEnding struct {
	key      string
	at       uint64
	ended    map[MemberID][]interface{}
	audience []MemberID
}

// closeWithEnded is [Encounter.closeWith] with what the ending took off each
// member carried on the ended beat (`ended`, omitted when nil).
func (e *Encounter) closeWithEnded(
	key string, at uint64, ended map[MemberID][]interface{}, audience ...MemberID,
) (*Outcome, error) {
	// A train is landing a deed: park the first ending and answer as if it had
	// fired. The train closes with it after its last unit (see
	// appendTrainAndNotice); nothing here is stored.
	if e.holdEndings {
		if e.heldEnding == nil {
			e.heldEnding = &heldEnding{key: key, at: at, ended: ended, audience: audience}
		}
		return &Outcome{Ending: key, Result: e.resultOf(key), At: at}, nil
	}
	// A reaction can finish the encounter while a turn or directed walk is
	// suspended. No continuation survives an ending, and closed persisted worlds
	// must never carry resumable work.
	e.pause = nil
	result := e.resultOf(key)
	e.outcome = &Outcome{
		Ending:  key,
		Result:  result,
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
	endedBeat := map[string]interface{}{
		"beat":   BeatEnded,
		"ending": key,
	}
	if len(ended) > 0 {
		endedBeat["ended"] = ended
	}
	// What the ending meant for the party, when it says (rpg-project#716).
	// Omitted for an ending that declares no result, so every beat written
	// before results existed reads exactly as it did.
	if result != "" {
		endedBeat["outcome"] = string(result)
	}
	beatBytes, _ := json.Marshal(endedBeat)
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
	return &Outcome{Ending: key, Result: result, At: at, Members: members}, nil
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
	return e.refreshSightDeclaring(observers, nil)
}

// refreshSightDeclaring is refreshSight with a list of members whose OBSERVABLE
// FACTS a caller says changed outside this composition — the equipment on a
// character sheet, today.
//
// It exists because a re-look on its own is silent. Everybody watching the
// subject simply refreshes, which is a state and not a transition, so the
// sighting beat says nothing and the watchers never learn their picture went
// stale. The declaration is what turns that refresh into news, and it is the
// CALLER's to make: this module cannot see a character sheet and must not
// guess that one moved.
//
// The declaration names the subject and never what about them changed. What a
// watcher may now perceive of them is their own testimony to re-read, which is
// the only shape in which an illusion can disagree with the truth.
//
// Every other caller passes nothing, which is refreshSight above.
func (e *Encounter) refreshSightDeclaring(
	observers []MemberID, declared []MemberID,
) (map[MemberID]*IntelDelta, *FormedBubble, error) {
	deltas, err := e.rebuildPercepts(observers)
	if err != nil {
		return nil, nil, err
	}

	// WHO SAW WHOM, before anything that seeing causes. A fight forms
	// because somebody came into view, so the view changing has to be
	// readable ahead of the fight — this function's own law, applied to the
	// beat that records the cause (see sightedbeat.go). Appended even on a
	// closed encounter: the percepts were rebuilt, so the transitions are
	// real whether or not a fight can follow them.
	if err := e.appendSightedBeats(deltas, declared, uint64(e.clock.ToData().HighWater)); err != nil {
		return nil, nil, err
	}
	// And a sighting wakes the observer whose state listens for one
	// (rpg-project#766 S4), at the next raise.
	e.landSightings(deltas, uint64(e.clock.ToData().HighWater))

	// A closed encounter has nothing to start a fight about, and form refuses
	// one anyway — checking here turns that refusal into a non-event.
	if e.outcome != nil {
		return deltas, nil, nil
	}

	// Concealment's trigger detection rides every sight refresh, for this
	// function's own reason: perceiving present state — an open hidden
	// door, hidden floor underfoot — is a rule about sight, and a rule
	// wired at the verbs is a rule some verb forgets (rpg-toolkit#1371).
	// Its reveal beats land after the verb's own beat (the law above) and
	// before any fight the refresh starts; a CLOSED encounter mints no new
	// knowledge, which the early return above already said.
	if err := e.sweepConcealment(); err != nil {
		return nil, nil, err
	}
	// An arresting guard who sees a jailed member out of the cell raises
	// the escape (jail.go), before the trigger asks who now fights.
	if err := e.sweepEscapes(); err != nil {
		return nil, nil, err
	}
	// An arresting guard who sees a wanted member seizes them (wanted.go).
	if err := e.sweepWanted(); err != nil {
		return nil, nil, err
	}

	formed, deltas, err := e.applyTrigger(deltas)
	if err != nil {
		return nil, nil, err
	}

	return deltas, formed, nil
}

// encodeSighting is the encoder one pass runs, once per placed member:
// [EncodeSightTestimony], unless a test handed this encounter a counting
// encoder in [Encounter.sightEncoder].
func (e *Encounter) encodeSighting(testimony SightTestimony) ([]byte, error) {
	if e.sightEncoder != nil {
		return e.sightEncoder(testimony)
	}
	return EncodeSightTestimony(testimony)
}

// rebuildPercepts rebuilds the complete percept for all given observers and
// returns a map of member IDs to encounter-owned intel deltas. The current
// clock reading is stamped on the pass.
//
// ONE PAYLOAD PER MEMBER, ONCE (rpg-toolkit#1691). This was a nested loop
// that encoded a subject's testimony once per observer who could see them —
// N² encodes for N members, of bytes that are identical every time, because
// what a member looks like is a fact about THEM and not about who is
// looking. mind/perception owns the loop now: this function assembles the
// presences, supplies the geometry as a [sightReach], and hands over one
// pass.
//
// The payload stays encoded HERE and stays opaque there, which is the whole
// reason an illusion is still expressible: perception carries bytes it
// cannot read, so this composition remains the only thing that knows what a
// sighting means (see [SightTestimony]).
func (e *Encounter) rebuildPercepts(observers []MemberID) (map[MemberID]*IntelDelta, error) {
	// One pass, one memo of the field's shared answers (rpg-toolkit#2097),
	// dropped when the pass returns: see [sightPassMemo].
	defer e.field.beginSightPass()()

	// Get current clock reading
	clockReadingInt := e.clock.ToData().HighWater
	clockReading := uint64(clockReadingInt)

	// Asked fresh for this refresh and never carried over from the last one —
	// see [Sight] for why remembering the answer would be the smallest
	// possible version of the dual state the capability exists to avoid.
	// Asked BEFORE the pass rather than inside it so that every observer in
	// one refresh is bounded by the same reading of the world (C8), and so
	// that a rulebook is consulted once for the whole roster rather than once
	// per member.
	reach, err := e.sightNow()
	if err != nil {
		return nil, err
	}

	// EVERY LOOK COVERS WHAT CHANGED SINCE THE LAST ONE (rpg-project#762):
	// the observers asked for, and every member within sight of a cell
	// changed since the last look ([Encounter.pendingCells]) or holding a
	// sighting now beyond its range ([Encounter.addOutranged]). So a change
	// is seen at the first look after it, which is when the whole roster's
	// look saw it.
	// The caller's observers keep the caller's order — first light asks in
	// declaration order, and the journal's facts follow it — and whoever
	// else the changes reach follows, in ID order.
	if e.everyoneLooks {
		observers = e.rosterIDs()
	} else {
		reached, err := e.observersWithin(e.pendingCells, nil, reach)
		if err != nil {
			return nil, err
		}
		asked := make(map[MemberID]bool, len(observers))
		for _, id := range observers {
			asked[id] = true
		}
		all := append(make([]MemberID, 0, len(observers)+len(reached)), observers...)
		for _, id := range reached {
			if !asked[id] {
				all = append(all, id)
			}
		}
		observers = all
	}
	e.pendingCells = e.pendingCells[:0]
	if e.rebuildProbe != nil {
		e.rebuildProbe(observers)
	}

	// Asked fresh for this refresh too, for the same C8 reason, and beside
	// sight rather than inside the pass so that one pass writes one
	// consistent reading of the world into every observer's testimony. What a
	// member holds is a fact an observer can be WRONG about later, which is
	// why it is snapshotted here rather than read when somebody asks — see
	// [SightTestimony].
	hands, err := e.equipmentNow()
	if err != nil {
		return nil, err
	}

	// Conditions too, beside equipment and for its reasons: what a member
	// holds is a fact an observer can be wrong about later, so it is
	// snapshotted into the one payload this pass encodes per member and
	// never read live (rpg-project#520 R16).
	held, err := e.conditionsNow()
	if err != nil {
		return nil, err
	}

	// Read before the pass and never carried into it, for the same reason as
	// sight and equipment above and beside them rather than inside the pass,
	// so that one pass writes one consistent reading of the world into every
	// subject's testimony.
	// Standing is a fact an observer can be WRONG about later, exactly like
	// equipment and position, which is why it is snapshotted here rather than
	// read live when somebody asks — see [SightTestimony].
	//
	// This choke point used to leave Down nil and cite C8 as the reason it
	// could not ask participation a second time in one pass. That reading of
	// C8 was wrong: C8 (design.md's composition laws) is "drivers may be
	// stochastic; the composition itself is deterministic" — a rule about
	// determinism, not a call budget, and "once per pass" appears nowhere in
	// [Standing] or [participationState] either. noticeDown already asks
	// participation again later in this same refresh (through applyTrigger),
	// so the second question this choke point refused was never actually
	// forbidden. Ruled 2026-09-12 (rpg-toolkit#1697): Assess is consistent,
	// so a caller may ask it when it needs it. Nil now means only what
	// testimony from a build that predates this field honestly says:
	// standing was not observed.
	down, err := e.downNow()
	if err != nil {
		return nil, err
	}

	// SORTED, because a roster read off a map has no order at all — and every
	// list downstream inherits whatever order it had. perception reports
	// FirstContact and Refreshed in percept order, those reach a host as
	// Discovered, and the sighting beat names them in a story that is supposed
	// to be a transcript. Two runs of one scene were producing
	// gained:["captain","alice"] and gained:["alice","captain"], which CI
	// caught and a local run did not. perception sorts its own presences by ID
	// as well (R6); this sort is what makes the ERRORS deterministic too — a
	// roster with two unencodable members must name the same one every run.
	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	// Every PLACED member is one presence, encoded once. An unplaced member is
	// nobody to see, exactly as the old loop's skip said.
	positions := make(map[MemberID]spatial.Position, len(roster))
	presences := make([]perception.Presence, 0, len(roster))
	// seenAs is each placed subject's index in presences, for the heading
	// variants headingPasses builds.
	seenAs := make(map[MemberID]int, len(roster))
	// seen is the testimony each placed subject's base payload was encoded
	// from, so a heading variant is built from it rather than decoded back.
	seen := make(map[MemberID]SightTestimony, len(roster))
	for _, subjectID := range roster {
		cell, ok := e.canvas.GetEntityPosition(string(subjectID))
		if !ok {
			continue // Not placed
		}
		positions[subjectID] = cell
		seenAs[subjectID] = len(presences)

		// Standing comes from the pass-scoped reading taken above, beside
		// sight and equipment — see the comment on [down] for why there was
		// never a rule against asking participation twice. isDown is its own
		// local so its address is a value for exactly this subject, not the
		// range variable.
		isDown := down[subjectID]
		// SEEN AS IT IS NOW: a posted body is seen holding its cell, and
		// one that stood aside is seen not to (rpg-project#836, B5).
		blocksMovement := e.memberBlocks(subjectID)
		// AND HOLDING HIS POST AS HE IS NOW (rpg-project#852, H5): the door
		// a seen holder bars, read from his state, his cell and his
		// standing in this pass.
		holdsDoor, _ := e.memberHolds(subjectID, down)
		facing := e.members[subjectID].Facing
		testimony := SightTestimony{
			State:              LocationKnown,
			Position:           cell,
			Equipment:          hands[subjectID],
			Conditions:         held[subjectID],
			Down:               &isDown,
			BlocksMovement:     &blocksMovement,
			HoldsDoor:          holdsDoor,
			AppearanceRef:      e.members[subjectID].AppearanceRef,
			DefaultActivityRef: e.members[subjectID].DefaultActivityRef,
			// FACING IS SEEN, beside where (design R13): snapshotted into
			// the one payload this subject is encoded into this pass, so a
			// memory keeps the facing it last saw.
			Facing: &facing,
		}
		payload, perr := e.encodeSighting(testimony)
		if perr != nil {
			return nil, fmt.Errorf("encode sight testimony: %w", perr)
		}
		presences = append(presences, perception.Presence{ID: sightMember(subjectID), Payload: payload})
		seen[subjectID] = testimony
	}

	// UNPLACED OBSERVERS AND NON-MEMBERS ARE FILTERED OUT, and this is the one
	// line of the adoption that could change ghost semantics in silence.
	//
	// The nested loop `continue`d on an observer it could not place: no
	// Surveil, so nothing of theirs faded. perception's rule 4 says an
	// observer that IS in the pass and reaches nothing lands a complete empty
	// percept and fades EVERYTHING — which is the right rule, and the exact
	// opposite behaviour. Rule 5 is the other half: an observer absent from
	// the pass is not touched at all. So "not placed" has to mean ABSENT, not
	// "present with no reach", and it has to mean it here rather than in
	// [sightReach.Reaches], which cannot tell the two apart.
	observerIDs := make([]core.EntityID, 0, len(observers))
	for _, observerID := range observers {
		if _, ok := e.members[observerID]; !ok {
			continue // Skip if not found
		}
		if _, placed := positions[observerID]; !placed {
			continue // Observer not placed
		}
		observerIDs = append(observerIDs, sightMember(observerID))
	}

	// Every OTHER member on the map, kept or dropped by GEOMETRY ALONE.
	//
	// There was a room-membership test in the loop this replaced, immediately
	// before the line of sight check, and it decided almost everything: two
	// members in different chambers never saw each other, however close, and
	// the check below never ran for them. It was not a range rule, it was the
	// ONLY visibility rule — standing in for the walls the composition could
	// not express (rpg-toolkit#1105/#1106). With one canvas and real walls it
	// has nothing left to say, so it is gone and the geometry answers.
	//
	// AND BOUNDED BY A DISTANCE THIS MODULE WAS TOLD (rpg-toolkit#1111). The
	// room label was quietly doing that job too: with it gone, the reference
	// tomb's longest unobstructed run — three doorways on one row, 27 cells,
	// 135 feet — was a sighting, and a sighting forms a fight. What was
	// missing there was never a number this module could pick. It was a LIGHT
	// model, and light is per-creature and per-light-source: the dwarf with
	// darkvision and the human holding her torch answer differently on the
	// same cell.
	//
	// So the term is SUPPLIED, and supplied per member. [Sight] is asked how
	// far each of them can see, this refresh, and the answer bounds what lands
	// in the percept. The light model 5e states arrives later as a better
	// ANSWER — with nothing here moving — which is exactly the promise
	// [Standing] makes about hit points. All of it lives in [sightReach] now,
	// which is the only thing in this pass that knows what a wall is.
	geometry := e.newSightReach(positions, reach)

	// THE STEALTH JUDGEMENT, BEFORE ANY COPY OF THE REACH (sneak.go, design
	// R10/R11). appendMutablePresences copies the geometry into the
	// observation reach the pass is handed, so a pair suppressed after that
	// line would be suppressed nowhere. Only this pass's observers are
	// judged.
	observerMembers := make([]MemberID, 0, len(observerIDs))
	for _, id := range observerIDs {
		member, _ := subjectID(id, memberSubjectKind)
		observerMembers = append(observerMembers, member)
	}
	snuck, err := e.judgeSneakers(&geometry, observerMembers)
	if err != nil {
		return nil, err
	}

	presences, observationGeometry, err := e.appendMutablePresences(presences, geometry)
	if err != nil {
		return nil, err
	}
	// What each observer still holds as current was true at the last look;
	// say so before this look fades any of it ([Encounter.reconfirm]).
	if err := e.reconfirm(observerIDs); err != nil {
		return nil, err
	}
	// THE WAY EACH OBSERVER SAW THEM GO (rpg-project#679 R10, #673 R8): one
	// pass per distinct set of headings, every observer in exactly one.
	passes, err := e.headingPasses(observerIDs, presences, positions, seenAs, seen)
	if err != nil {
		return nil, err
	}
	perceived := make(map[core.EntityID]*perception.Delta, len(observerIDs))
	for _, pass := range passes {
		out, oerr := e.intelLog.Observe(perception.Pass{
			At:        clockReading,
			Channel:   perception.Sight,
			Presences: pass.presences,
			Observers: pass.observers,
			Reach:     observationGeometry,
		})
		if oerr != nil {
			return nil, fmt.Errorf("refreshsight observe: %w", oerr)
		}
		for id, delta := range out {
			perceived[id] = delta
		}
	}

	// The rolls are told after the pass, so a beat names its watcher exactly
	// when the mover now holds it live (R12).
	if err := e.tellSnuck(snuck, clockReading); err != nil {
		return nil, err
	}

	opacity := e.sightOpacity()
	// A CHANGED OPACITY ORPHANS EVERY CACHED DISK: their keys carry the old
	// one and can never match again, so the memory is freed here rather than
	// at the bound. Soundness is the key's, not this check's (visible.go).
	if opacity != e.visibleEpoch {
		e.visibleCache = nil
		e.visibleEpoch = opacity
	}
	for _, observer := range observerIDs {
		member, _ := subjectID(observer, memberSubjectKind)
		if err := e.discoverCells(member, geometry, opacity); err != nil {
			return nil, err
		}
	}

	deltas := make(map[MemberID]*IntelDelta, len(perceived))
	for observerID, delta := range perceived {
		member, _ := subjectID(observerID, memberSubjectKind)
		projected, err := memberPerceptionDelta(delta)
		if err != nil {
			return nil, err
		}
		deltas[member] = projected
		corrected, err := e.correctEmptyProps(member, observationGeometry, clockReading)
		if err != nil {
			return nil, err
		}
		deltas[member].KnowledgeChanged = deltas[member].KnowledgeChanged || corrected
	}
	e.lastLook = clockReading

	return deltas, nil
}

// reconfirm re-lands, at [Encounter.lastLook], every current sighting an
// observer of this look last confirmed earlier (rpg-project#762).
//
// TRUE BY THE BOUND. An observer left out of a look is one within sight of
// nothing that changed, so every sighting it held as current stayed true
// through that look; the last look is the latest reading it is known true
// at. Re-landing the same testimony at that reading says exactly that, and
// moves nothing else: the payload is the one held, so Observed and every
// list but Confirmed stand. The look that follows then stamps what is still
// in view with now, and what leaves view keeps the last look's reading — the
// stamp the whole roster's look gave a memory, since it looked every time.
//
// The re-landing is a complete pass for the one observer: every current
// sighting it holds is in it, so nothing fades here (perception's rule 4),
// and its deltas are not news — they are dropped.
func (e *Encounter) reconfirm(observers []core.EntityID) error {
	at := e.lastLook
	if at == 0 {
		return nil
	}
	for _, observer := range observers {
		held, err := e.intelLog.Held(observer)
		if err != nil {
			return fmt.Errorf("reconfirm %q: %w", observer, err)
		}
		presences := make([]perception.Presence, 0, len(held))
		reach := make(heldReach, len(held))
		stale := false
		for _, h := range held {
			if !h.CurrentOn(perception.Sight) {
				continue
			}
			if h.Confirmed > at {
				// Looked at since: a current sighting is never older than
				// the observer's own last look, and this one is newer.
				stale = false
				break
			}
			stale = stale || h.Confirmed < at
			presences = append(presences, perception.Presence{ID: h.Subject, Payload: h.Payload})
			reach[h.Subject] = true
		}
		if !stale {
			continue
		}
		if _, err := e.intelLog.Observe(perception.Pass{
			At:        at,
			Channel:   perception.Sight,
			Presences: presences,
			Observers: []core.EntityID{observer},
			Reach:     reach,
		}); err != nil {
			return fmt.Errorf("reconfirm %q: %w", observer, err)
		}
	}

	return nil
}

// heldReach reaches exactly the subjects an observer already holds, for
// [Encounter.reconfirm]'s one-observer pass.
type heldReach map[core.EntityID]bool

// Reaches answers for the held subjects alone.
func (r heldReach) Reaches(_ perception.Channel, _, subject core.EntityID) bool { return r[subject] }

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
	if err := e.validateJoin(in); err != nil {
		return nil, err
	}
	member, out, err := e.admitMember(in)
	if err != nil {
		return nil, err
	}
	if out.Reserved {
		return out, nil
	}
	memberIDs := e.audienceFor(subjectBeat, in.Member)
	clockReadingForBeat := uint64(e.clock.ToData().HighWater)
	seqNum := out.Seq

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
	if err != nil && !errors.Is(err, errNoEnding) {
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

// validateJoin is every refusal [Encounter.Join] makes BEFORE its first
// mutation, shared with [Encounter.Board] so a board refuses exactly what a
// join refuses, and refuses it before anything is written.
func (e *Encounter) validateJoin(in *JoinInput) error {
	// Validation
	if in == nil {
		return fmt.Errorf("join: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return fmt.Errorf("join: %w", ErrClosed)
	}

	if in.Member == "" {
		return fmt.Errorf("join: %w", ErrNoMember)
	}

	// Check if already a member — on the roster, or waiting in reserve
	// (reserve.go): a reserved member is absent from every projection, but
	// it is in the encounter, and a second joiner under its id would arrive
	// twice.
	if _, exists := e.members[in.Member]; exists || e.inReserve(in.Member) {
		return fmt.Errorf("join: member %s is already in the encounter: %w", in.Member, ErrDuplicateMember)
	}

	// Nor can a world NPC (rpg-toolkit#1404, design.md N4) — see NewEncounter's
	// own check for why.

	// See NewEncounter's own call to validateMemberFacts for why — ASKED
	// BEFORE any mutation (canvas.PlaceEntity below is the first one),
	// unlike NewEncounter's construction-in-a-local-that-only-escapes-on-
	// success safety net: Join mutates a LIVE *Encounter, so an invalid
	// fact caught after PlaceEntity would need to roll a placement back
	// rather than simply never having made one (Copilot, PR #1187).
	if err := validateMemberFacts(memberFacts{
		ID:                 in.Member,
		AppearanceRef:      in.AppearanceRef,
		DefaultActivityRef: in.DefaultActivityRef,
		Facing:             in.Facing,
		Options:            in.Options, Table: in.Table, Machine: in.Machine, Behaviors: in.Behaviors,
		BlockingStates: in.BlockingStates, Posts: in.Posts,
	}); err != nil {
		return fmt.Errorf("join: %w", err)
	}
	if err := e.field.validateOptionHolds(in.Options); err != nil {
		return fmt.Errorf("join: member %s: %w: %w", in.Member, err, ErrBadMember)
	}
	joinGranted := grantedTopics(nil, in.Options)
	for topic := range e.granted {
		joinGranted[topic] = true
	}
	if err := validateOptionInteractions(e.interactions, joinGranted, in.Options); err != nil {
		return fmt.Errorf("join: member %s: %w: %w", in.Member, err, ErrBadMember)
	}

	// A holder must name a record this field declares — the SAME refusal
	// NewEncounter makes, and asked here for the reason the line above is:
	// Join mutates a live encounter, so every refusal happens before the
	// first mutation rather than needing to be rolled back
	// ([JoinInput.Holds]).
	for _, id := range in.Holds {
		if _, declared := e.field.intelByID[id]; !declared {
			return fmt.Errorf("join: member %q holds intel %q: %w", in.Member, id, ErrNoIntel)
		}
	}

	// The faction must be one this field has, and a member arriving under a
	// faction's mind id must arrive in that faction — the SAME refusal
	// NewEncounter makes, before the first mutation ([JoinInput.Faction]).
	if err := e.field.validateMemberFaction(in.Member, in.Kind, in.Faction); err != nil {
		return fmt.Errorf("join: %w", err)
	}

	// A joiner held in reserve must be one that can wait there and one whose
	// predicate can hold — the SAME refusal NewEncounter makes (reserve.go),
	// before the first mutation.
	if in.Arrives != nil {
		if err := e.field.validateArrival(in.Member, in.Kind, in.Arrives); err != nil {
			return fmt.Errorf("join: %w", err)
		}
	}

	// Hex fields require integral axial cells (interim tools/spatial#926
	// enforcement — see isIntegralHexCell). Asked first, for the reason
	// [Encounter.stepMember] asks it first: a fractional cell is an arithmetic
	// mistake and must not be reported as a map one.
	if !isIntegralHexCell(in.Cell) {
		return fmt.Errorf("join: position is not an integral axial cell: %w", ErrBadPlacement)
	}

	// The arrival cell must be STANDABLE — some authored region has to own
	// it. The canvas spans the field's whole bounding box, so "on the map"
	// and "somewhere a member can stand" are different questions, and this is
	// the one that matters (the same check [Encounter.stepMember] makes for a
	// step). Scenery is on the map and is not somewhere anybody stands.
	if !e.field.isStandable(in.Cell) {
		return fmt.Errorf("join: cell %v %s: %w", in.Cell, e.field.notStandable(in.Cell), ErrBadPlacement)
	}

	// STANDABLE INCLUDES UNCOVERED (issue #1753): a movement-blocking placed
	// footprint covering the cell's centre refuses the arrival by name — the
	// same fact [Encounter.CellAt] would give a step here, so a join and a
	// step cannot disagree about a table someone would land inside.
	if prop, covered := e.field.standingBlocks(in.Cell); covered {
		return fmt.Errorf("join: cell %v is covered by placed prop %q: %w", in.Cell, prop, ErrBadPlacement)
	}

	// THE REFUSAL THE ADMISSION WOULD OTHERWISE MAKE LATE, after the member is
	// placed and its join beat written: a temperament mix that cannot be
	// dealt. Asked here so Join and Board both refuse it before the first write.
	if err := e.validateTemper(in.Member, in.Temper); err != nil {
		return fmt.Errorf("join: %w", err)
	}

	return nil
}

// admitMember is everything [Encounter.Join] does to put one VALIDATED joiner
// into the run short of looking: the reserve hold, or the placement, the
// clock seat, the seeded holdings, the world rebuilt, the join beat, the
// discovery state and the dealt temperament. No sight refresh and no fight —
// the caller looks once, after everybody it is admitting is in
// ([Encounter.Join] after one, [Encounter.Board] after all of them).
//
// The returned output carries the reserve answer, or the join beat's Seq;
// the caller fills in what the look produced.
func (e *Encounter) admitMember(in *JoinInput) (*memberRecord, *JoinOutput, error) {
	member := &memberRecord{
		ID:                 in.Member,
		Kind:               in.Kind,
		Name:               in.Name,
		AppearanceRef:      in.AppearanceRef,
		DefaultActivityRef: in.DefaultActivityRef,
		Facing:             seedFacing(in.Facing),
		Options:            cloneOptions(in.Options),
		Table:              cloneTable(in.Table),
		Temper:             in.Temper,
		BlocksMovement:     in.BlocksMovement,
		Faction:            in.Faction,
	}
	if err := e.useBehaviors(member, in.Behaviors, nil, in.BlockingStates); err != nil {
		return nil, nil, fmt.Errorf("join: %w", err)
	}
	// A JOINER'S GRANTS JOIN THE RUN'S (rpg-project#929), before its own
	// posts are judged against them; a refused post puts them back.
	wasGranted := e.granted
	e.granted = grantedTopics(nil, in.Options)
	for topic := range wasGranted {
		e.granted[topic] = true
	}
	if err := e.usePosts(member, in.Posts, in.Cell); err != nil {
		e.granted = wasGranted
		return nil, nil, fmt.Errorf("join: %w", err)
	}

	// A JOINER WITH A PREDICATE GOES INTO RESERVE (rpg-project#375, design §3
	// Spawn; reserve.go): validated like any joiner, then held — no
	// placement, no clock, no seed, no beat, no refresh. The output says so,
	// and names the cell it will arrive at as its placement; nothing else in
	// the run does.
	if in.Arrives != nil {
		e.reserveMember(&reservedMember{
			record: *member, at: in.Cell, holds: in.Holds, arrives: in.Arrives,
		})
		region, _ := e.RegionAt(in.Cell)
		return member, &JoinOutput{
			Reserved: true,
			Member: Member{
				ID: in.Member, Kind: in.Kind, Name: in.Name, Region: region, Position: in.Cell,
				Table:          cloneTable(in.Table),
				Temper:         in.Temper,
				BlocksMovement: in.BlocksMovement, Faction: factionOf(member),
			},
		}, nil
	}

	entity := e.newMemberEntity(in.Member, in.Kind)

	if err := e.canvas.PlaceEntity(entity, in.Cell); err != nil {
		return nil, nil, fmt.Errorf("join placement: %w: %w", ErrBadPlacement, err)
	}

	// Register the member
	e.members[in.Member] = member
	e.everMembers[in.Member] = true // Track in everMembers

	// A joiner lands on the world clock, never mid-fight. Being pulled into a
	// running bubble is Transfer's job and is a separate decision from joining
	// the encounter at all.
	if cerr := e.seatOnWorldClock(in.Member); cerr != nil {
		return nil, nil, fmt.Errorf("join member %q world clock: %w", in.Member, cerr)
	}

	// Store decider if present (monsters only, validated above)

	// The joiner's placed records, seeded as the holdings they are — the
	// SAME call NewEncounter makes for an authored member, so intel enters a
	// run one way (design P5). Before the beat below, because a holding is
	// state the join establishes rather than something the join narrates:
	// nothing about it is ever narrated (design P3).
	if err := e.holdings.seedIntel(in.Member, in.Holds); err != nil {
		return nil, nil, fmt.Errorf("join: %w", err)
	}

	// A member is an entity in the graph, and the mind of a faction of one
	// is whoever is in it — so the declaration is rebuilt on arrival
	// (world.go).
	if err := e.buildWorld(); err != nil {
		return nil, nil, fmt.Errorf("join: %w", err)
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
	// THE KIND RIDES THE MEMBERSHIP ENTRIES (rpg-project#935): who was a
	// player in the run at an entry's time is read off the record, and a
	// member who has left is no longer on any roster to ask.
	beatPayload := map[string]interface{}{
		"beat":   BeatJoined,
		"member": string(in.Member),
		"kind":   string(member.Kind),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.appendBeat(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("join append beat: %w", err)
	}

	seqNum := appendOut.Seq
	if member.Kind == KindPlayer {
		if err := e.initializeDiscovery(in.Member, in.PrivateDiscoveries); err != nil {
			return nil, nil, err
		}
	}

	// The faction's mix, dealt at this door exactly as it is at Setup's
	// (design §3): a monster that arrives mid-run gets its own nerve rolled
	// for it, and the beat says which one it came out as.
	//
	// AFTER THE JOIN BEAT AND BEFORE THE SIGHT REFRESH, on both counts for a
	// reason. The deal's beat names a member the story introduces in the join
	// beat above, so leading with it would put a raw id on the line that
	// introduces the creature. And the refresh below can form a bubble, which
	// can consult a table — a creature consulted before its deal would roll
	// its orders as the soldier an empty temperament looks like.
	dealt, terr := e.dealTemperFor(in.Member, in.Temper, clockReadingForBeat)
	if terr != nil {
		return nil, nil, fmt.Errorf("join: %w", terr)
	}
	member.Temper = dealt

	return member, &JoinOutput{Seq: seqNum}, nil
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

	// WHAT THE DEPARTURE ENDED, validated before the first write below.
	ended, err := e.prepareEndedRemovals("exit", in.Ended)
	if err != nil {
		return nil, err
	}

	// Get the exiting member's final cell, and the region it falls in.
	finalCell, ok := e.canvas.GetEntityPosition(string(in.Member))
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}
	finalRegion, _ := e.RegionAt(finalCell)

	// Capture the exiting member's holdings (carry-forward)
	carry, err := e.memberIntel(in.Member)
	if err != nil {
		return nil, fmt.Errorf("exit held_by: %w", err)
	}
	// Carry leaves this module, so it leaves in the delivered form.
	if carry, err = deliveredHoldings(carry); err != nil {
		return nil, fmt.Errorf("exit held_by: %w", err)
	}

	// The exit beat's audience, captured HERE, before the exiter is removed
	// from e.members below: it is every member INCLUDING the exiter — they
	// witness their own departure (and can re-read it via Story).
	// audienceFor reads live membership, so calling it after the delete
	// would leave the exiter out (tableBeat, no subjects — see its doc).
	audience := e.audienceFor(tableBeat)

	// LEAVING ENDS SNEAKING (sneaking R7), and no verdict may name a member
	// who is gone, whichever side of the pair they stood on.
	e.endSneaking(in.Member)
	e.forgetWatcher(in.Member)

	// Remove from the map
	if err = e.canvas.RemoveEntity(string(in.Member)); err != nil {
		return nil, fmt.Errorf("exit remove entity: %w: %w", ErrBadPlacement, err)
	}

	// OFF THE ROSTER BEFORE OFF THE CLOCK. Leaving a bubble can hand the
	// active slot to a monster, and that monster is driven at once
	// (leaveAnyClock → driveIfStillRunning): its strike reads the roster,
	// and a roster read mid-drive must never meet a member who is off the
	// map and still on the list — placementOf refuses that as ErrNoField,
	// which surfaced as "invalid encounter data" on a player's own Exit
	// (Kirk's walk on the raider camp, 2026-09-05; pre-existing for any
	// active player leaving a fight with a striking monster next). The
	// roster changing is the graph's declaration changing too (world.go).
	leaverKind := e.members[in.Member].Kind
	delete(e.members, in.Member)
	// A LINE WAITING ON THE LEAVER'S ROLL IS DROPPED (rpg-project#924,
	// interaction_hold.go): nobody is left to decide on it, and the exit beat
	// tells it. A line said TO the leaver is released by the sweep below.
	e.dropHoldsOf(in.Member)
	if err = e.buildWorld(); err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	// Remove from whichever clock holds them — the world clock normally, a
	// bubble if they were in a fight when they left.
	clockDeltas, cerr := e.leaveAnyClock(in.Member)
	if cerr != nil {
		return nil, fmt.Errorf("exit member %q clock: %w", in.Member, cerr)
	}
	intelDeltas := mergeIntelDeltas(nil, clockDeltas)

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

	// DECIDE BEFORE NARRATING (rpg-toolkit#1507). The beat says what LEFT
	// THE RUN, so it cannot be written until it is known whether this
	// departure carried anything out or dropped it — and those are the same
	// question as whether an ending fired.
	//
	// This used to run the other way round: the beat was appended with
	// everything the member held, and the drop happened after. A carrier
	// leaving from a non-exit produced `exited holding=[chalice]` followed
	// by `dropped chalice` — two beats about one departure, disagreeing,
	// with a client patching from the first believing the chalice had left
	// the dungeon. Deciding first costs nothing and removes the
	// disagreement rather than asking every reader to reconcile it.
	//
	// The BEAT ORDER design §6 fixes is untouched: the departure beat still
	// precedes both `dropped` and `ended`. What moved is the decision, not
	// the narration.
	//
	// ONE PASS IN DECLARATION ORDER (rpg-project#716): an exited-holding
	// ending and a `left` ending at the same door are asked in the order the
	// author listed them, and the first that holds fires.
	match, err := e.departureEnding(leftThrough, carried)
	if err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}
	fired := match.fired

	// What actually walked out with them: everything they held when an
	// ending fired on this departure, and nothing otherwise. Never nil — an
	// empty list and an absent field are different facts to a reader, and
	// this is always the empty list.
	departing := []PropID{}
	if fired != nil {
		departing = carried
	}

	beatPayload := map[string]interface{}{
		"beat":   BeatExited,
		"member": string(in.Member),
		// The kind they left as: once gone they are on no roster, and a
		// reader asking whether they were a player then asks this entry
		// (rpg-project#935).
		"kind": string(leaverKind),
		// holding is what LEFT THE RUN with them, and exit is the authored
		// way they left by — empty when they left from anywhere else, which
		// is what a departure through the lobby looks like. Both always
		// present so a reader never has to tell "absent" from "none".
		"holding": departing,
		"exit":    string(leftThrough),
	}
	if len(ended) > 0 {
		beatPayload["ended"] = ended
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

	// LEAVING ENDS EVERY ADDRESS THAT NAMES THE LEAVER (rpg-project#942),
	// as the addressee or as the guard, after the departure that caused it
	// and before anything is saved: a run saved addressing a member it no
	// longer has does not load.
	if err := e.endAddressesOf(in.Member); err != nil {
		return nil, fmt.Errorf("exit: %w", err)
	}

	seqNum := appendOut.Seq

	// R9 — THEY DROP IT. A departure that did not end the run leaves
	// everything the member was carrying on the cell they stood on, as
	// holdable props anybody can pick up again. Otherwise a carrier who
	// leaves through the lobby — or simply disconnects — takes the only win
	// out of a run that is still going.
	var firedOutcome *Outcome
	if fired == nil {
		if err := e.dropCarried(in.Member, carried, finalCell, clockReadingForBeat, audience); err != nil {
			return nil, fmt.Errorf("exit: %w", err)
		}
	} else {
		// THE ENDING, AFTER THE DEPARTURE BEAT (design §6): the record reads
		// "left through the front gate with the heirloom" and then "ended".
		firedOutcome, err = e.closeWith(fired.key, clockReadingForBeat, audience...)
		if err != nil {
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

	// A HELD LINE SAID TO THE LEAVER IS RELEASED (rpg-project#924): its
	// settle could never be admitted, and a saved reservation naming a
	// creature the run lacks would not load.
	if err := e.sweepHolds(); err != nil {
		return nil, fmt.Errorf("exit: %w", err)
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
		Paused:      e.Paused(),
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

	// WHAT THE ENDING TOOK OFF EACH MEMBER, validated before the close.
	var endedByMember map[MemberID][]interface{}
	for member, results := range in.Ended {
		if _, ok := e.members[member]; !ok {
			return nil, fmt.Errorf("end: ended for %q, who is not a member: %w", member, ErrInvalidData)
		}
		// ONE MEMBER'S REMOVALS UNDER ONE MEMBER'S KEY: an entry filed under
		// alice that takes a condition off bob says something the beat's
		// shape says is about alice.
		for i, removed := range results {
			if removed.Address != nil && removed.Address.MemberID != member {
				return nil, fmt.Errorf("end: ended[%q][%d] names %q: %w", member, i, removed.Address.MemberID, ErrInvalidData)
			}
		}
		prepared, err := e.prepareEndedRemovals("end", results)
		if err != nil {
			return nil, err
		}
		if len(prepared) == 0 {
			continue
		}
		if endedByMember == nil {
			endedByMember = make(map[MemberID][]interface{}, len(in.Ended))
		}
		endedByMember[member] = prepared
	}

	closed, err := e.closeWithEnded(in.Ending, uint64(e.clock.ToData().HighWater), endedByMember)
	if err != nil {
		return nil, fmt.Errorf("end: %w", err)
	}

	return &EndOutput{Outcome: *closed}, nil
}

// memberEntity is an internal entity for members
type memberEntity struct {
	id   string
	kind MemberKind

	// blocks answers whether this member's body blocks NOW, for the canvas's
	// own occupancy seam (spatial's placement check). A READ THROUGH THE
	// ENCOUNTER ([Encounter.memberBlocks]), never a copy (rpg-project#836):
	// a posted body blocks by its machine state, and a stored flag here
	// would be a second answer the canvas and the fold could disagree on
	// the beat a transition fires.
	blocks func() bool
}

// newMemberEntity is a member's canvas entity, reading its blocking through
// the encounter ([Encounter.canvasBlocks]).
func (e *Encounter) newMemberEntity(id MemberID, kind MemberKind) *memberEntity {
	return &memberEntity{id: string(id), kind: kind, blocks: func() bool { return e.canvasBlocks(id) }}
}

// canvasBlocks is the canvas's answer for a member's body: the fold's
// occupant rule without stance, which the canvas cannot ask per mover. A
// body that blocks ([Encounter.memberBlocks]) blocks unless it is a downed
// monster, whose cell the fold calls standable whatever its state
// ([occupantPassage]), so a step the fold allows is never refused underneath
// it (rpg-project#836, B3).
//
// WHO IS DOWN IS ASKED ONLY HERE, AND ONLY OF A BLOCKING MONSTER: the canvas
// asks this of the bodies on a cell somebody is being placed on, so the
// participation read runs when a placement meets a body that would block, and
// never on open floor. It cannot be stored beside the body for the reason
// nothing about standing is (participation.go). A read that fails blocks: a
// world that cannot say who is down does not walk anyone onto a guess.
func (e *Encounter) canvasBlocks(id MemberID) bool {
	if !e.memberBlocks(id) {
		return false
	}
	if m := e.members[id]; m == nil || m.Kind != KindMonster {
		return true
	}
	down, err := e.downNow()

	return err != nil || !down[id]
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

// BlocksMovement reports whether the member's body blocks now: its current
// state when it has blocking states, else its authored BlocksMovement fact
// (rpg-toolkit#1434), false for a caller who never set it.
func (m *memberEntity) BlocksMovement() bool {
	return m.blocks != nil && m.blocks()
}

// prepareEndedRemovals validates and shapes the conditions a departure or an
// ending took off the board: each must be a [ResultConditionRemoved] naming a
// current member — a stranger is refused, never carried — and is otherwise
// validated as every removal is ([Encounter.prepareActivationResult]).
// Returns the beat's `ended` entries, nil when there are none. Writes
// nothing, so a caller asks it before its first write.
func (e *Encounter) prepareEndedRemovals(verb string, results []ActivationResult) ([]interface{}, error) {
	var out []interface{}
	for i, removed := range results {
		if removed.Kind != ResultConditionRemoved {
			return nil, fmt.Errorf("%s: ended[%d] kind %q: %w", verb, i, removed.Kind, ErrInvalidData)
		}
		if removed.Address == nil {
			return nil, fmt.Errorf("%s: ended[%d] names no member: %w", verb, i, ErrInvalidData)
		}
		if _, ok := e.members[removed.Address.MemberID]; !ok {
			return nil, fmt.Errorf("%s: ended[%d] names %q, who is not a member: %w",
				verb, i, removed.Address.MemberID, ErrInvalidData)
		}
		payload, err := e.prepareActivationResult(fmt.Sprintf("%s: ended", verb), i, removed)
		if err != nil {
			return nil, err
		}
		out = append(out, payload)
	}

	return out, nil
}
