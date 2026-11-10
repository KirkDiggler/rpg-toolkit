// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MoveInput walks a member along a path of cells on the map.
//
// The path may leave the room the walker is standing in, and a doorway is how
// — see Path.
type MoveInput struct {
	// Session is the session to act in.
	Session string

	// Member is the member walking.
	Member string

	// DeclarationID is the opaque current Move selector returned by Afford.
	// It is required on the turn clock and must be empty on the world clock.
	DeclarationID string

	// Path is the cells to walk through, in order, each adjacent to the last
	// and the first adjacent to where the member currently stands.
	//
	// ADJACENT, not merely within one cell: a step onto the cell the walker has
	// just reached is refused with ErrBadPosition rather than recorded as a
	// movement of no distance. Revisiting a cell is fine — a there-and-back
	// walks genuinely both ways — so it is only the step that goes nowhere that
	// is refused.
	//
	// A path, not a destination. The caller says where to walk; what actually
	// happened comes back as Steps, which may be shorter. A single-cell path is
	// the ordinary case and entirely legal.
	//
	// Cells are DUNGEON-ABSOLUTE — the same coordinates the Atlas draws and
	// every other verb speaks (rpg-project#227).
	//
	// A WALK CROSSES A DOORWAY, and a crossing is written like any other step:
	// the far side is simply the next cell along (rpg-toolkit#1048, which is
	// also where the Traverse verb went). Absolute coordinates made a crossing
	// expressible, and that slice made it permitted.
	//
	// Adjacency is not permission, though. Two rooms may TOUCH without a door
	// between them, so a step into the next room with no doorway joining it to
	// the cell the walker stands on is refused with ErrNoCrossing — a refusal a
	// client reading only the Atlas's cells cannot predict, since the doorway is
	// in the doorway list or it is nowhere.
	Path []spatial.Position
}

// Step is one cell actually entered.
type Step struct {
	// Position is the cell entered, in dungeon-absolute space.
	Position spatial.Position `json:"position"`
}

// MovementStatus distinguishes a completed route from a resumable reaction
// pause or an early stop. Steps always contains only cells actually entered.
type MovementStatus string

const (
	MovementCompleted MovementStatus = "completed"
	MovementPaused    MovementStatus = "paused"
	MovementStopped   MovementStatus = "stopped"
	// MovementContinuing is an answer given before the walk ended: the next
	// step pays a world round, and that step and everything after it run
	// after the answer as a continuation ([MoveOutput.Continues]).
	MovementContinuing MovementStatus = "continuing"
)

// MoveOutput reports how far the member got and what it revealed.
type MoveOutput struct {
	// StopReason is the observation-safe explanation for an ordinary obstruction.
	StopReason string         `json:"stop_reason,omitempty"`
	Status     MovementStatus `json:"status"`
	// JoinedCombat reports entry from the current cell before the next step.
	JoinedCombat bool `json:"joined_combat,omitempty"`
	// Steps is what actually happened, in order.
	//
	// Shorter than the requested Path means the walk stopped early. The
	// reason is in Outcome for an ending underfoot, in Formed for a fight
	// starting, and in the story for the third case: a reaction to one of
	// these very steps felled the walker, who stops in the cell they were
	// leaving. This is deliberately not an error: the walk did what it could,
	// and where it got to is the answer.
	Steps []Step `json:"steps,omitempty"`

	// Discovered is what changed in each observer's perception across the whole
	// walk, keyed by observer.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Outcome is present if an ending fired underfoot, which is also why the
	// walk stopped.
	Outcome *Outcome `json:"outcome,omitempty"`

	// Formed is present if the walk put the walker in sight of the other side
	// and a fight started, which is also why the walk stopped. The remaining
	// cells were not attempted: a fight member does not free-roam.
	Formed *Formed `json:"formed,omitempty"`

	// Barred is present when a body stopped the walk (rpg-project#842, the
	// guard holds the gate R2): a member on the next cell, or a crossing a
	// post holds. The walker did not take that step, Status is
	// [MovementStopped], StopReason is Barred.Reason, and the remaining cells
	// were not attempted. It is an answer, never an error: the bump is a
	// `barred` beat the barrier heard at once, and what his answer made
	// (his line, an intel change, a fight in Formed, an ending in Outcome)
	// is in this output and the act stream like any other step's.
	Barred *Barred `json:"barred,omitempty"`

	// Addressed is present when a guard hailed the walker on a step
	// (rpg-project#923, the guard opens the conversation): a carrier of an
	// interaction saw her come inside its approach distance and now
	// addresses her. The step was taken and is in Steps; the remaining cells
	// were dropped, never attempted and never resumed, and Status is
	// [MovementStopped] unless the hail landed on the walk's last cell. It is
	// an answer, never an error: the guard's line, when his machine has one,
	// is a table answer under this Move's act, and the same address rides the
	// walker's Speak row ([Declaration.Address]) for as long as it stands.
	Addressed *Address `json:"addressed,omitempty"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`

	// Continues is true when the walk answered before it ended
	// (rpg-project#879 R2, "the verb answers before the world runs"): the
	// host configured [Config.Continuations] and the walker's next step pays
	// a world round. Status is then [MovementContinuing]; Steps and
	// Discovered are the steps taken before that one; Outcome, Formed,
	// Barred and JoinedCombat are empty because the walk has not ended; and
	// Saved and Delivery are empty because nothing is saved or delivered
	// until the rest has run. The remaining steps arrive on the stream as
	// the walker's own moved beats after Through, in the same order and
	// beside the same world beats a whole walk records; the continuation's
	// [ContinuationReport] is how the walk ended.
	//
	// False whenever no pace came due along the path, and always without a
	// runner: the answer is then the whole walk, saved and delivered.
	//
	// A CONTINUING ANSWER STANDS ONLY IF ITS REST LANDS. Nothing is saved
	// before the rest runs, so a continuation that fails persists none of
	// the walk, the answered steps included: Steps and Through then name
	// moves the record never got (rpg-project#879, §Where the instance
	// lives). The host learns it from the continuation's error and must
	// tell the client to re-read from before the move.
	Continues bool `json:"continues,omitempty"`

	// Through is the record id the answer covers up to when Continues is
	// true; the walker's own moved beats after it continue the walk. It is
	// the moved beat of the last step in Steps, or, for an answer with no
	// steps (the very first step pays the round), the last record entry at
	// the moment of the answer. Empty only when Continues is false, or when
	// the record holds no entry at all.
	Through string `json:"through,omitempty"`
}

// There were TraverseInput and TraverseOutput here, and a Traverse verb that
// took a connection id and crossed it. All three retired with
// rpg-toolkit#1048: a doorway's two cells are adjacent on the map, so crossing
// one is a step, and Move takes steps. What the verb reported that a step does
// not — which rooms were left and entered — was the room dialect this seam
// stopped speaking two slices ago.

// Move walks an adjacent path, validating its requested budget before acting.
// Only completed steps consume movement. Reactions pause before the announced
// step, and resumed walks pay only for additional completed steps. A visible
// teammate in combat admits the walker before entering their occupied cell.
// Input, placement, participation, and persistence errors remain errors.
//
// With [Config.Continuations] set, a world-clock walk whose next step pays a
// round answers before that step ([MoveOutput.Continues]) and hands the rest
// to the runner with the session's guard still held; see
// [ContinuationRunner]. Without one the whole walk runs before the answer.
func (m *Manager) Move(ctx context.Context, in *MoveInput) (*MoveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("move: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	// THE GUARD IS THE CONTINUATION'S ONCE IT IS HANDED OFF: every return
	// before that releases it here, and a continued walk releases it when
	// its rest ends.
	guard := newHeldGuard(release)
	release = guard.verbRelease
	defer release()
	// A walk that may continue is opened under a context the hand-off can
	// re-parent: the encounter's host seams keep the context they are
	// built under, and the rest runs after the request's has ended.
	//
	// The runner is handed the REQUEST's context, never this one: a runner
	// continuing under a context derived from it would re-parent the verb's
	// context onto itself.
	request := ctx
	var vctx *verbContext
	if m.continuations != nil {
		vctx = newVerbContext(ctx)
		ctx = vctx
	}
	if in.Member == "" {
		return nil, fmt.Errorf("move: %w", ErrNoMemberID)
	}
	if len(in.Path) == 0 {
		return nil, fmt.Errorf("move: %w", ErrEmptyPath)
	}

	scope, err := m.openForChange(ctx, string(VerbMove), in.Session)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	if err = validateWalk(scope.enc, encounter.MemberID(in.Member), in.Path); err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	// Whose turn it is is asked FIRST among the fact-about-this-member
	// refusals — before downed, before anything is priced, before any sheet
	// is loaded at all (Copilot finding on #1171). If it is not your turn,
	// nothing else about you is this call's business yet: encounter.Step's
	// own gate would eventually refuse a non-active bubble member's first
	// step with ErrNotActive regardless, but by then refuseIfDown and offer
	// compilation would both already have loaded a sheet, and combat.Pay might
	// already have refused with a MISLEADING currency shortfall — a
	// non-active member low on movement would be told "movement: X ft
	// needed" instead of "not your turn", naming the wrong reason. This is
	// not a second copy of Step's rule: it is the same fact, ClockOf, that
	// Manager.Turn and offer compilation already read for their own purposes, read
	// once more here before anything else touches this member at all.
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("move: %w", translate(err))
	}
	if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) != in.Member {
		return nil, fmt.Errorf("move: %w", ErrNotYourTurn)
	}

	if err = scope.enc.ValidateWalkEnd(encounter.CellAtInput{Mover: encounter.MemberID(in.Member), Cell: in.Path[len(in.Path)-1]}); err != nil {
		return nil, fmt.Errorf("move destination: %w", translate(err))
	}

	var cost *walkCost
	if ClockKind(clock.Kind) == ClockTurn {
		// The turn path loads the actor strictly ONCE. Its downed verdict and
		// regenerated Move offer use this same sheet, preventing a sequenced
		// repository from changing the answer between the blocker and selector.
		// Only Move is regenerated: Attack pricing, assembly, target view and
		// participant preflight are unrelated dependencies of this execution.
		actor := m.loadActorSheet(ctx, in.Member)
		if actor.downed {
			return nil, fmt.Errorf("move: member %q: %w", in.Member, ErrDowned)
		}
		if in.DeclarationID == "" {
			return nil, fmt.Errorf("move: %w", ErrNoDeclarationID)
		}
		offers, compileErr := m.compileOffersFor(
			ctx, scope.enc, scope.data, scope.session, in.Member, clock, actor, VerbMove,
		)
		if compileErr != nil {
			return nil, fmt.Errorf("move: %w", compileErr)
		}
		selected, selectErr := selectCompiledOffer(offers, VerbMove, in.DeclarationID)
		if selectErr != nil {
			return nil, fmt.Errorf("move: %w", selectErr)
		}
		if selected.sheet == nil {
			return nil, fmt.Errorf("move: %w", ErrStaleDeclaration)
		}
		feet := 5 * len(in.Path)
		cost = &walkCost{
			profile: &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityMovement: feet}},
			sheet:   selected.sheet,
			feet:    feet,
		}
	} else {
		// World Move keeps its independent standing gate. It has no compiled
		// actor sheet or declaration, and free-roam movement semantics remain
		// separate from the turn economy.
		if err = refuseIfDown(scope, "member", in.Member); err != nil {
			return nil, fmt.Errorf("move: %w", err)
		}
		// Afford deliberately offers no world-clock Move row. Empty is the
		// only valid selector there; a non-empty ID left over from a dissolved
		// fight must not turn into a free move.
		if in.DeclarationID != "" {
			return nil, fmt.Errorf("move: %w", ErrStaleDeclaration)
		}
		cost = &walkCost{}
	}

	// Validate the requested budget without consuming it. Each completed
	// step pays below, so interruptions cannot spend the unwalked suffix.
	if cost.profile != nil {
		if !combat.CanPay(cost.sheet, cost.profile) {
			left := cost.sheet.CapacityLeft(combat.CapacityMovement)
			return nil, fmt.Errorf("move: %w: %s", ErrCannotAfford, movementShortfall(cost.feet, left))
		}
		walker, err := cost.sheet.ToData()
		if err != nil {
			return nil, fmt.Errorf("move: walker %q: %w: %v", in.Member, ErrBadCharacter, err)
		}
		scope.walker = walker
	}

	// runWalk asks per cell instead and this verb asks not at all.
	boundary := paceThrough
	if m.continuations != nil {
		boundary = paceStopBefore
	}
	res, rest, err := m.startWalk(ctx, scope, in.Member, in.Path, boundary)
	if err != nil {
		// A cell's opportunity attack can already have saved sheets before a
		// later cell refuses, so the refusal names what landed (S6).
		return nil, fmt.Errorf("move: %w", saveErrorAfterWrites(scope, "", err))
	}

	if rest != nil {
		// A PACE CAME DUE: the walker's next step pays a world round. The
		// answer is the steps so far; that step, the raise it fires inside,
		// the steps after, the save and the delivery are the continuation's,
		// in exactly the order a whole walk takes them.
		through, throughErr := answerThrough(scope, res)
		if throughErr != nil {
			return nil, fmt.Errorf("move: %w", throughErr)
		}
		answer := &MoveOutput{
			Status:     MovementContinuing,
			Steps:      res.steps,
			Discovered: nilIfEmpty(res.discovered),
			Continues:  true,
			Through:    through,
		}
		err = m.handOff(request, in.Session, VerbMove, vctx, guard,
			func(ctx context.Context) (*ContinuationReport, error) {
				return m.continueWalk(ctx, scope, in.Member, in.Path, rest)
			})
		if err != nil {
			return nil, fmt.Errorf("move: %w", err)
		}
		return answer, nil
	}

	if err = m.saveWalkProgress(ctx, scope); err != nil {
		return nil, err
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	return &MoveOutput{
		Status:       res.status,
		StopReason:   res.stopReason,
		Steps:        res.steps,
		JoinedCombat: res.joinedCombat,
		Discovered:   nilIfEmpty(res.discovered),
		Outcome:      res.outcome,
		Formed:       res.formed,
		Barred:       res.barred,
		Addressed:    res.addressed,
		Saved:        report,
		Delivery:     delivery,
	}, nil
}

// walkCost is what one walk costs its walker, and the sheet that will be
// charged for it — [swingPrice]'s own shape, carried over rather than
// reinvented: both fields nil is a price of nothing, never a missing one.
type walkCost struct {
	// profile is what the door charges for this walk, or nil to charge
	// nothing on the world clock. A turn-clock member without a loadable sheet
	// never produces a selectable Move offer.
	profile *combat.SpendProfile

	// sheet is the walker's own sheet with its turn readied, ready to be
	// charged and, after a successful walk, saved by [Manager.saveWalker].
	// Nil exactly when profile is.
	sheet *character.Character

	// feet is the requested path's price, named separately from profile so a
	// refusal can quote it without re-deriving it from a map lookup.
	feet int
}

// movementShortfall composes the "ft"-suffixed text a refused Move and
// Afford's own VerbMove declaration both carry — the SAME text in both
// places, never a paraphrase of it, the way ErrCannotAfford's own doc
// requires of every currency this seam names.
//
// It is presentation, not a second copy of the arithmetic: the YES/NO
// affordability answer comes from [combat.Pay]/[combat.CanPay] alone, exactly
// as it does for a swing. This exists only because [combat.SpendProfile]'s own
// refusal text has no unit to name ("movement: 20 needed, 15 left") and Kirk's
// brief wants one a client can say out loud without translating "movement" and
// a bare integer into feet itself.
func movementShortfall(needed, left int) string {
	return fmt.Sprintf("movement: %d ft needed, %d ft left", needed, left)
}

// saveWalker persists the walker's sheet after a walk has spent from it, and
// marks the write on the scope so persist's own report opens with it — the
// same contract [Manager.saveDirty] keeps for a swing's damaged sheets, sized
// down to the one sheet a walk can ever touch.
func (m *Manager) saveWalker(ctx context.Context, scope *writeScope, sheet *character.Character) error {
	data, err := sheet.ToData()
	if err != nil {
		// Nothing was written for this sheet; the report still names what
		// the verb already made durable (S6).
		return saveErrorAfterWrites(scope, "", fmt.Errorf("walker: %w: %v", ErrBadCharacter, err))
	}
	return m.sheetsFor(scope).save(ctx, data)
}

// continueWalk is the rest of a continued Move: the walk from the step that
// pays a round to its end, committed at each pace boundary (rpg-project#879
// R7): after every step that paid a world round and moved, the raise it fired
// is saved and delivered before the next step is taken, and the walk's
// remaining steps go in the next commit. The last commit is the final one,
// which closes the walk's act.
//
// A failure after a commit landed voids only what follows it. The landed
// commits stand and were delivered; nothing is retried (a retry would replay
// what another writer now owns, which R7 refuses). The report of what landed
// is returned beside the error.
func (m *Manager) continueWalk(
	ctx context.Context, scope *writeScope, member string, path []spatial.Position, rest *walkResume,
) (*ContinuationReport, error) {
	res := newWalkResult()
	landed := &ContinuationReport{}
	at := *rest
	for {
		if err := m.walk(ctx, scope, member, path, at, paceStopAfter, res); err != nil {
			return landed.after(fmt.Errorf("move: %w", saveErrorAfterWrites(scope, "", err)))
		}
		if err := m.saveWalkProgress(ctx, scope); err != nil {
			return landed.after(err)
		}
		if res.resume == nil {
			break
		}
		report, delivery, err := m.commitPace(ctx, scope)
		if err != nil {
			return landed.after(fmt.Errorf("move: %w", err))
		}
		landed.landed(report, delivery, scope)
		at = *res.resume
	}
	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return landed.after(fmt.Errorf("move: %w", err))
	}
	landed.landed(report, delivery, scope)
	landed.Move = &MoveRest{
		Status:       res.status,
		StopReason:   res.stopReason,
		JoinedCombat: res.joinedCombat,
		Steps:        res.steps,
		Discovered:   nilIfEmpty(res.discovered),
		Outcome:      res.outcome,
		Formed:       res.formed,
		Barred:       res.barred,
		Addressed:    res.addressed,
	}
	return landed, nil
}

// answerThrough is the record id a continuing answer covers up to
// ([MoveOutput.Through]): the last step's moved beat, or with no steps the
// newest entry the record holds, so a client always has a point to resume
// from. Empty only for an empty record.
func answerThrough(scope *writeScope, res *walkResult) (string, error) {
	if res.through != "" {
		return res.through, nil
	}
	read, err := scope.enc.ReadRecord(&encounter.ReadRecordInput{})
	if err != nil {
		return "", translate(err)
	}
	if len(read.Entries) == 0 {
		return "", nil
	}
	return read.Entries[len(read.Entries)-1].ID, nil
}

// walkResume is where a walk stopped before a step that pays a round: the
// index of that step in the path and the cell the walker stands on.
type walkResume struct {
	at   int
	from spatial.Position
}

// walkResult is what one run of the walk produced.
type walkResult struct {
	stopReason   string
	status       MovementStatus
	steps        []Step
	discovered   map[string]Discovery
	outcome      *Outcome
	formed       *Formed
	barred       *Barred
	addressed    *Address
	joinedCombat bool
	// through is the record id of the last moved step's beat.
	through string
	// resume is set when the walk stopped before a step that pays a world
	// round, or after one under paceStopAfter: the rest starts there.
	resume *walkResume
}

func newWalkResult() *walkResult {
	return &walkResult{discovered: map[string]Discovery{}, status: MovementStopped}
}

// Barred is a walk stopped by a body ([MoveOutput.Barred]): the
// composition's [encounter.StepStop], carried as written.
type Barred struct {
	// Barrier is the body in the way: the member on the cell, or for a held
	// crossing the nearest holder the walker sees, else the nearest holder.
	// It is the one member the `barred` beat landed on.
	Barrier string `json:"barrier"`

	// Cell is the cell the step was aimed at and refused into: the
	// barrier's own cell, or the far cell of a held crossing.
	Cell spatial.Position `json:"cell"`

	// Reason is the public sentence the walker is told: "<name> stands in
	// the way", "<name> bars the way", or "someone ..." for a body the
	// walker does not see.
	Reason string `json:"reason"`
}

// Address is an interaction addressing a member (rpg-project#923): the
// guard speaking to them and the interaction he speaks for. It is the
// composition's [encounter.Addressed], carried as written, on a Move a hail
// stopped ([MoveOutput.Addressed]) and on the addressed member's own Speak
// row ([Declaration.Address]).
type Address struct {
	// Guard is the member addressing them: the carrier who hailed.
	Guard string `json:"guard"`

	// Interaction is the interaction the guard speaks for.
	Interaction string `json:"interaction"`
}

// projectAddress is the composition's address in the seam's shape.
func projectAddress(a encounter.Addressed) *Address {
	return &Address{Guard: string(a.Guard), Interaction: string(a.Interaction)}
}

// runWalk steps a member along a path, stopping at the first fight, the first
// ending, or the last cell.
//
// IT DECIDES NOTHING. It used to decide two things and both were wrong to hold
// here. Until rpg-toolkit#964 it held a game rule: it read each step's
// perception delta, and a subject seen for the first time stopped the walk and
// opened a window for the walker to answer — the SDK deciding when an encounter
// begins, in the one package whose charter is to hold no rules. Until
// rpg-toolkit#1059 it held a mechanism: each step arrived pre-sorted into a
// same-room move or a doorway crossing, resolved here off a projected map, and
// executed through whichever of the composition's two verbs matched.
//
// Both now belong to the composition, which owns the data each answer is made
// of, and this loop reads what came back exactly the way it already read
// Outcome: as news. The walk stops on a formed bubble because the walker is IN
// a fight and a fight member does not free-roam — the composition would refuse
// the next step with ErrInBubble — so stopping is a fact about the world rather
// than a policy about perception.
func (m *Manager) runWalk(
	ctx context.Context, scope *writeScope, member string, path []spatial.Position,
) (*walkResult, error) {
	res, _, err := m.startWalk(ctx, scope, member, path, paceThrough)
	return res, err
}

// paceBoundary is where a walk stops at a world round
// ([encounter.Encounter.PaceDue]) so its caller can act there.
type paceBoundary int

const (
	// paceThrough walks every step without stopping at a round.
	paceThrough paceBoundary = iota
	// paceStopBefore stops BEFORE the first step that pays a round: the
	// answer of a continued Move (rpg-project#879 R2).
	paceStopBefore
	// paceStopAfter stops AFTER each step that paid a round and moved, so
	// the continuation commits the raise that step fired before walking on
	// (rpg-project#879 R7). The walk's last step never stops: the walk ends
	// there anyway.
	paceStopAfter
)

// startWalk is [Manager.runWalk] from the walker's own cell. paceStopBefore
// stops the walk before the first step that pays a world round
// ([encounter.Encounter.PaceDue]) and answers where it stopped; the steps
// taken so far are in the result.
func (m *Manager) startWalk(
	ctx context.Context, scope *writeScope, member string, path []spatial.Position, boundary paceBoundary,
) (*walkResult, *walkResume, error) {
	res := newWalkResult()

	// Where the walker STILL stands, advanced from each step's own answer
	// rather than from the cell that was asked for — the same reason the loop
	// below reads stepped.Stepped.To instead of cell.
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, nil, translate(err)
	}
	from, placed := rosterPositions(roster)[member]
	if !placed {
		return nil, nil, fmt.Errorf("member %q is not placed: %w", member, ErrNoMember)
	}
	if err := m.walk(ctx, scope, member, path, walkResume{from: from}, boundary, res); err != nil {
		return nil, nil, err
	}
	return res, res.resume, nil
}

// walk steps from path[start.at], with the walker on start.from, into res.
// When boundary stops it at a step that pays a world round, res.resume is
// where the rest starts; every other end is in res too. res accumulates
// across calls (steps, discoveries), so a walk resumed into the same res
// reads as one walk.
func (m *Manager) walk(
	ctx context.Context, scope *writeScope, member string, path []spatial.Position,
	start walkResume, boundary paceBoundary, res *walkResult,
) error {
	from := start.from
	res.resume = nil
	for i := start.at; i < len(path); i++ {
		cell := path[i]
		// THE SPLIT COMES BEFORE THE ANNOUNCEMENT (rpg-project#879 R2): the
		// raise fires inside the Step that pays the round, and that step's
		// admission and announcement (and any reaction to it) belong with
		// it, so the continuation takes the due cell from the top exactly as
		// a whole walk would. PaceDue is a read; asking changes nothing.
		//
		// Asked here for paceStopAfter too, BEFORE the step: once the step
		// paid the round the pace is no longer due, so afterwards the
		// question has no answer left to give. The answer is the shared
		// rule's prediction (encounter's spendWorldPace pays by the same
		// rule); a step it got wrong costs one commit too many or one too
		// few, never a beat lost or told twice.
		due := false
		if boundary != paceThrough {
			paced, dueErr := scope.enc.PaceDue(&encounter.PaceDueInput{Member: encounter.MemberID(member)})
			if dueErr != nil {
				return translate(dueErr)
			}
			due = paced.Due
			if due && boundary == paceStopBefore {
				res.resume = &walkResume{at: i, from: from}
				return nil
			}
		}
		admission, admissionErr := scope.enc.AdmitWalk(&encounter.AdmitWalkInput{Member: encounter.MemberID(member)})
		if admissionErr != nil {
			return translate(admissionErr)
		}
		mergeDiscoveries(res.discovered, projectDiscoveries(admission.IntelDeltas))
		if admission.Joined {
			res.joinedCombat = true
			return nil
		}
		scope.walkContinuation = path[i:]
		// ANNOUNCED BEFORE IT IS TAKEN, which is [encounter.Mover]'s contract
		// and the reason an opportunity attack can fire at all: a reactor's
		// swing is checked for reach against where the walker IS, and a walk
		// that stepped first would hand the reaction a departed target.
		//
		// TWO CALLERS, ONE RULE. The composition's own monster loop announces
		// each cell of a driven walk exactly this way (encounter/clocks.go);
		// this is the player's half of the same rule, and a walk that skipped
		// it would mean a fighter takes the bite while a wolf never does.
		//
		// THE ZERO MoveStep, spelled out rather than assumed: a player's walk
		// is the ordinary case the struct's zero value describes — nobody
		// forced it and nothing caused it, so it provokes.
		if err := (moverSeam{m: m, scope: scope}).Move(
			ctx, scope.enc, encounter.MoveStep{
				Mover: encounter.MemberID(member), From: from, To: cell,
			},
		); err != nil {
			var paused *encounter.StepPausedError
			if errors.As(err, &paused) {
				res.status = MovementPaused
				return nil
			}
			return fmt.Errorf("step %d of %d to (%v,%v): %w", i+1, len(path), cell.X, cell.Y, err)
		}

		// A REACTION THAT DROPPED THE WALKER STOPS THE WALK, and the walker
		// falls in the cell they were LEAVING rather than the one they were
		// entering (rpg-project#316, ruling R6) — the same answer the
		// composition's driven walk gives, for the same reason: the swing was
		// checked against the cell the walker still stands on, so stepping
		// them afterwards would move a body out of the square it fell in.
		//
		// ASKED PER CELL, and the batched once-per-walk answer this loop was
		// handed no longer covers it. That batching rested on a move being
		// unable to down anyone; announcing steps is precisely what made that
		// false, so the answer is refreshed here — the walker's own standing,
		// nothing to do with what a Discovery reports, which now reads
		// Seen.Standing from the sight testimony rather than this consult.
		down, standingErr := discoveryStanding(scope)
		if standingErr != nil {
			return standingErr
		}
		if down[member] {
			// A STOP, NOT A FAILURE, reported the way every other early stop
			// is: the cells already walked are in res.steps, the ones after
			// this are not, and the blow that felled the walker is in the
			// story with its reaction named.
			return nil
		}

		clock, clockErr := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(member)})
		if clockErr != nil {
			return translate(clockErr)
		}
		if clock.Kind == encounter.ClockTurn {
			if string(clock.Active) != member {
				return nil
			}
			if scope.walker == nil {
				data, loadErr := m.sheetsFor(nil).load(ctx, "walker", member)
				if loadErr != nil {
					return loadErr
				}
				scope.walker = data
			}
		}
		if clock.Kind == encounter.ClockTurn {
			sheet, loadErr := character.Load(ctx, scope.walker)
			if loadErr != nil {
				return fmt.Errorf("walk sheet: %w", loadErr)
			}
			if !combat.CanPay(sheet, &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityMovement: 5}}) {
				return nil
			}
		}

		stepped, err := scope.enc.Step(&encounter.StepInput{
			Member:  encounter.MemberID(member),
			To:      cell,
			EndWalk: i == len(path)-1,
			// THE SESSION OWNS THE REACH (offers.go): a standing address is
			// measured at the distance Speak is, so an address never stands
			// where the walker could not answer it.
			Reach: InteractionReach,
		})
		if err != nil {
			var obstruction *encounter.StepObstructedError
			if errors.As(err, &obstruction) {
				res.stopReason = obstruction.PublicReason()
				return nil
			}
			// Nothing is saved on a mid-walk rejection. The member has really
			// moved in memory for the steps already taken, but that encounter is
			// discarded unsaved, so the persisted world is untouched.
			return refusedStep(i, len(path), cell, err)
		}

		// A BODY STOPPED THE STEP (rpg-project#842), READ BEFORE Stepped.To:
		// a stopped step names the walker's own cell at both ends, so reading
		// it as a step would walk her onto the cell she already stands on and
		// on to the next. Nothing moved and nothing is paid; what the
		// barrier's answer made is news like any step's.
		if stepped.Stopped != nil {
			res.barred = &Barred{
				Barrier: string(stepped.Stopped.Barrier),
				Cell:    stepped.Stopped.Cell,
				Reason:  stepped.Stopped.Reason,
			}
			res.stopReason = stepped.Stopped.Reason
			mergeDiscoveries(res.discovered, projectDiscoveries(stepped.IntelDeltas))
			res.outcome = projectOutcome(stepped.Outcome)
			res.formed = projectFormed(stepped.Formed)
			return nil
		}

		if clock.Kind == encounter.ClockTurn {
			sheet, loadErr := character.Load(ctx, scope.walker)
			if loadErr != nil {
				return fmt.Errorf("walk sheet: %w", loadErr)
			}
			if payErr := combat.Pay(sheet, &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityMovement: 5}}); payErr != nil {
				return fmt.Errorf("walk step: %w", payErr)
			}
			walker, dataErr := sheet.ToData()
			if dataErr != nil {
				return fmt.Errorf("walk sheet: %w: %v", ErrBadCharacter, dataErr)
			}
			scope.walker = walker
		}

		// Read off what the composition says happened rather than off the
		// input, and there is no projection left in between to get wrong: the
		// answer arrives on the map already.
		//
		// The two are the same value TODAY, and a mutation swapping this for
		// `cell` survives the whole suite because of it — a step lands exactly
		// where it was aimed or it is refused, so no fixture can tell them
		// apart. That is a statement about the composition's current contract,
		// not a guarantee this loop is entitled to assume: the day a step can
		// land somewhere other than where it was aimed (a shove, a slide, a
		// door that opens onto a different cell than the one named), echoing
		// the input reports a movement that did not happen, and reading the
		// answer keeps being right without anyone noticing it had to change.
		res.steps = append(res.steps, Step{
			Position: stepped.Stepped.To,
		})
		res.through = stepped.ID
		from = stepped.Stepped.To
		mergeDiscoveries(res.discovered, projectDiscoveries(stepped.IntelDeltas))

		// A HAIL STOPS THE WALK (rpg-project#923, ruled 2026-11-11: "I should
		// stop walking"): the step was taken, a guard now addresses the
		// walker, and the rest of the path is dropped, never resumed. Read
		// before Outcome and Formed so a hail whose answer also ended the run
		// or formed a fight is still reported.
		if len(stepped.Addressed) > 0 {
			res.addressed = projectAddress(stepped.Addressed[0])
		}

		if stepped.Outcome != nil {
			// The encounter ended underfoot. Every remaining step is abandoned:
			// a closed encounter refuses movement anyway, and attempting them
			// would turn a clean stop into a rejection the caller must interpret.
			res.outcome = projectOutcome(stepped.Outcome)
			return nil
		}

		if stepped.Formed != nil {
			res.formed = projectFormed(stepped.Formed)
			return nil
		}

		if res.addressed != nil && i < len(path)-1 {
			return nil
		}

		// THE PACE BOUNDARY (rpg-project#879 R7): this step paid a world
		// round and moved, so the raise it fired is in the record. The
		// continuation commits it before the next step. The walk's last
		// step never stops here: the walk ends with it and the final commit
		// takes the raise.
		if due && boundary == paceStopAfter && i < len(path)-1 {
			res.resume = &walkResume{at: i + 1, from: from}
			return nil
		}
	}

	res.status = MovementCompleted
	return nil
}

// refusedStep says why a step the composition refused was refused, in this
// package's vocabulary and the caller's own coordinates.
//
// Our sentinel travels ALONE (S2, rpg-toolkit#1058): a host that could match on
// encounter.ErrBadPlacement would be coupled to the module this seam exists to
// keep replaceable, through the one channel the AST boundary test cannot see.
// The composition's account of WHY survives as TEXT, because "owned by no room"
// and "not an integral axial cell" are the difference between a typo and a
// fractional coordinate for whoever has to debug it.
//
// An error the mapping does not recognise is wrapped once and not repeated.
// Those are the HOST's own — a Standing or Initiative capability failing inside
// the step — and a host matching on its own error must keep being able to.
func refusedStep(i, n int, cell spatial.Position, err error) error {
	ours := translate(err)
	if errors.Is(ours, err) {
		return fmt.Errorf("step %d of %d to (%v,%v): %w", i+1, n, cell.X, cell.Y, err)
	}
	return fmt.Errorf("step %d of %d to (%v,%v): %w: %v", i+1, n, cell.X, cell.Y, ours, err)
}

// projectFormed turns the composition's report of a started fight into the
// SDK's own shape.
func projectFormed(f *encounter.FormedBubble) *Formed {
	if f == nil {
		return nil
	}
	out := &Formed{}
	for _, id := range f.Order {
		out.Order = append(out.Order, string(id))
	}
	for _, id := range f.Surprised {
		out.Surprised = append(out.Surprised, string(id))
	}
	return out
}

// validateWalk checks that a path IS a walk, whole, before a single cell is
// entered.
//
// What is left here after rpg-toolkit#1059 is exactly the part that is a rule
// about WALKING rather than a fact about the map. A walk is a run of adjacent
// cells starting next to the walker, and that can be decided from two things
// this seam is entitled to know: where the walker stands, and which coordinate
// family the field uses. Everything else a step needs — which room owns a cell,
// whether a doorway joins two of them — the composition decides as the step is
// taken, in the one place it is decided for monsters too.
//
// It used to decide all of it, and the cost was not only duplication. It
// fetched a whole Atlas per Move (documented O(total cells), measured ~128MB
// and ~50-65ms at the legal field budget, unmemoized) to read one room's grid
// and the doorway list; it located every cell of the path a second time; and it
// hard-coded "a doorway is exactly one adjacent cell pair" into the walk loop,
// where the first door that behaved differently would have had to be taught
// twice.
//
// Adjacency is delegated to spatial's own grid rather than hand-rolled, because
// the two families disagree about what "one step" means and substituting one
// for the other is a real, previously-shipped defect class: Chebyshev distance
// on axial hex coordinates agrees with cube distance everywhere except the
// diagonals, so a wrong formula passes almost every fixture. ONE grid answers
// for the whole path, including the step that changes rooms — a field has a
// single family by law (W1), and adjacency in both families survives
// translation, so an absolute pair is adjacent or not regardless of which
// room's grid is asked.
//
// Adjacency is also not sufficient, in the other direction: a cell is adjacent
// to ITSELF under every family's Distance <= 1, so the zero-distance step needs
// refusing by name rather than falling out of the distance check
// (rpg-toolkit#1060).
func validateWalk(enc *encounter.Encounter, member encounter.MemberID, path []spatial.Position) error {
	here, err := standsAt(enc, member)
	if err != nil {
		return err
	}

	grid, err := gridOf(enc)
	if err != nil {
		return err
	}

	for i, cell := range path {
		if cell == here {
			// A step of zero distance is not a step (rpg-toolkit#1060). Nothing
			// downstream would have caught it: every grid family reads
			// adjacency as Distance <= 1, so zero passes the check below, and
			// the composition's placement explicitly permits the mover's own
			// cell. The no-op then went the whole way — a genuine `moved` beat
			// recorded and persisted, sight refreshed, the walk fanned out to
			// every client — for a movement that never happened, and free-roam
			// has no movement budget to notice the discrepancy later.
			//
			// Compared against HERE, which advances with the walk, so [A,B,B]
			// is caught at its second B rather than only at the path's first
			// cell. [A,B,A] stays legal and must: a there-and-back moves
			// genuinely at every step, and a walker may retrace their route as
			// often as they like. It is zero DISTANCE that is the phantom, not
			// a repeated cell.
			return fmt.Errorf("step %d of %d: already standing on (%v,%v): %w",
				i+1, len(path), cell.X, cell.Y, ErrBadPosition)
		}

		if !grid.IsAdjacent(here, cell) {
			return fmt.Errorf("step %d from (%v,%v) to (%v,%v): %w",
				i+1, here.X, here.Y, cell.X, cell.Y, ErrBrokenPath)
		}

		here = cell
	}
	return nil
}

// standsAt reports where a member is, on the map.
//
// A ROSTER READ AND NOTHING ELSE. It used to serialize the whole aggregate —
// clock, intel, log, field and endings — to read two floats, because the
// composition's roster reported a room and no position; toolkit#933 fixed that,
// and Members has carried each member's cell on the map ever since. What
// remained until rpg-toolkit#1059 was the other half of the round trip: this
// took the absolute cell the roster had just given it, Located it down to a
// room and a room-local cell, and handed that to a caller who immediately
// projected it back up to the identical absolute cell it started as — through a
// helper whose error path silently returned the unprojected value.
//
// The answer was always right there in the row.
func standsAt(enc *encounter.Encounter, member encounter.MemberID) (spatial.Position, error) {
	members, err := enc.Members()
	if err != nil {
		return spatial.Position{}, translate(err)
	}

	for _, m := range members {
		if m.ID == member {
			return m.Position, nil
		}
	}
	return spatial.Position{}, fmt.Errorf("%q: %w", member, ErrNoMember)
}

// adjacencySpan is the size the adjacency grid is built with, and it is
// ARBITRARY on purpose.
//
// A spatial.Grid is two things at once: a distance metric and a set of bounds.
// This seam wants only the metric — adjacency is Distance <= 1 in every family,
// and no family's Distance consults the grid's dimensions, so any span answers
// identically for cells anywhere on the map (pinned by
// TestTheAdjacencyGridIsSpanIndependent).
//
// The bounds are deliberately NOT this seam's business. Whether a cell exists
// is the composition's answer, given by the step itself: a cell no room owns is
// refused with ErrBadPosition when it is stepped on. Building this grid with a
// real room's width and height would have looked more careful while checking
// nothing — the walk crosses rooms, so the walker's own room's bounds are the
// wrong bounds for half the path anyway.
const adjacencySpan = 1

// gridOf builds a grid to test adjacency with, matching the field's own
// coordinate family.
//
// Asks the composition for the FAMILY and nothing else — an O(1) read (W1: one
// family per field). The Atlas this used to come from enumerates every cell of
// every room to answer, which is the per-Move cost rpg-toolkit#1059 finding 2
// measured.
func gridOf(enc *encounter.Encounter) (spatial.Grid, error) {
	family, err := enc.Grid()
	if err != nil {
		return nil, translate(err)
	}

	switch family {
	case spatial.GridShapeHex:
		return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
			SpanWidth:  adjacencySpan,
			SpanHeight: adjacencySpan,
		}), nil
	default:
		// Unreachable: hex is the only family a field has had since the
		// square one left with the room chain (rpg-project#256). Still asked
		// and still refused rather than assumed, because a family we do not
		// understand answered with one we do is a wrong answer dressed as a
		// working one — and the two families disagree on the diagonals.
		return nil, fmt.Errorf("unknown grid family: %w", ErrInvalidWorld)
	}
}

// mergeDiscoveries folds one step's perception deltas into the walk's running
// total.
//
// A walk produces a delta per step, and a caller wants what changed across the
// whole movement rather than a per-cell replay. First contact is appended;
// refreshed and faded subjects accumulate, deduplicated, because a subject that
// faded and returned should not appear twice in the same list.
func mergeDiscoveries(into map[string]Discovery, from map[string]Discovery) {
	for observer, delta := range from {
		running := into[observer]
		running.FirstContact = append(running.FirstContact, delta.FirstContact...)
		running.Refreshed = appendUnique(running.Refreshed, delta.Refreshed)
		running.Faded = appendUnique(running.Faded, delta.Faded)
		into[observer] = running
	}
}

func appendUnique(dst []string, src []string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		seen[s] = struct{}{}
	}
	for _, s := range src {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}

func nilIfEmpty(m map[string]Discovery) map[string]Discovery {
	if len(m) == 0 {
		return nil
	}
	return m
}

// saveWalkProgress saves the latest walking sheet, including reaction changes
// and only the movement actually consumed. Resumed walks use the same path.
func (m *Manager) saveWalkProgress(ctx context.Context, scope *writeScope) error {
	if scope.walker == nil {
		return nil
	}
	sheet, err := character.Load(ctx, scope.walker)
	if err != nil {
		return saveErrorAfterWrites(scope, "", fmt.Errorf("save walking sheet: %w", err))
	}
	return m.saveWalker(ctx, scope, sheet)
}
