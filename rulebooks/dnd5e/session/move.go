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

	// Seq is the story sequence of the recorded step.
	Seq uint64 `json:"seq"`
}

// MoveOutput reports how far the member got and what it revealed.
type MoveOutput struct {
	// Steps is what actually happened, in order.
	//
	// Shorter than the requested Path means the walk stopped early, and the
	// reason is in Outcome. This is deliberately not an error: the walk did
	// what it could, and where it got to is the answer.
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

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// There were TraverseInput and TraverseOutput here, and a Traverse verb that
// took a connection id and crossed it. All three retired with
// rpg-toolkit#1048: a doorway's two cells are adjacent on the map, so crossing
// one is a step, and Move takes steps. What the verb reported that a step does
// not — which rooms were left and entered — was the room dialect this seam
// stopped speaking two slices ago.

// Move walks a member along a path, one cell at a time.
//
// This is the first verb that does something the composition cannot do alone:
// the composition's own Step is a single hop, and walking is what makes the
// cells in between real. That matters beyond tidiness — anything that fires
// because a member *entered a particular cell* can only be noticed by something
// that visits each of them.
//
// The walk stops early when an ending fires underfoot. Remaining steps are not
// attempted, and `len(Steps) < len(Path)` is how the caller learns, with the
// reason in Outcome. Reporting that as an error would be wrong: the movement
// that happened, happened, and it is exactly what the caller asked for up to
// the point the world changed.
//
// # On the turn clock, a walk spends movement (rpg-toolkit#1169)
//
// A member IN A FIGHT may still walk — only when it is their turn, and only as
// far as their turn's movement reaches. The echoed Move offer supplies the
// already-loaded, turn-readied sheet; this verb compiles the whole requested
// path at 5 feet per cell and charges it through [combat.Pay] BEFORE any cell is
// entered, the same door [Manager.Attack] pays a swing through: a
// path that costs more than the turn has left is refused whole, naming the
// currency, and not one cell of it happens. Whose turn it is is not re-derived
// here — [encounter.Step] is the one place that gate lives, and its refusal
// (ErrNotActive) is translated to ErrNotYourTurn exactly as EndTurn's already
// is. Free roam is untouched: a member on the world clock pays nothing, exactly
// as [Manager.priceSwing] charges nothing there.
//
// WHAT IS VALIDATED WHOLE, AND WHAT IS NOT (R5, narrowed deliberately by
// rpg-toolkit#1059). A path that is not a WALK is rejected entire, before a
// single cell is entered: a gap in it, a step of no distance, a first cell
// nowhere near the walker. A caller who mis-computed a route wants none of it
// rather than an arbitrary prefix of it, and those are the mistakes a route
// computation actually makes. Movement's own price joins that list: it is
// charged against the whole path before the walk starts, not metered per cell
// as the walk goes.
//
// The two refusals that need the MAP — a cell no room owns, and a cell in the
// next room with no doorway joining it — are raised by the composition as each
// step is taken, because checking them in advance means the seam re-deciding
// what is crossable, which is the duplication this walk exists without. The
// caller sees no difference: a refused walk returns an error, and nothing is
// persisted or published, because the encounter that moved in memory is
// discarded unsaved. The same is true of a charged sheet: offer regeneration
// loads and readies a [character.Character] fresh for this call, and a walk
// that fails after paying simply never hands that sheet to [Manager.saveWalker] —
// nothing durable ever saw the spend.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrStaleDeclaration, ErrEmptyPath, ErrBrokenPath for a path with a gap in it,
// ErrNoSession, ErrNoEncounter, ErrNoMember, ErrClosed, ErrBadPosition for a
// cell no room owns OR a cell the walker is already standing on, ErrNoCrossing
// for a step into another room with no doorway joining it, ErrNotYourTurn for a
// bubble member asked to walk out of turn, ErrCannotAfford naming the movement
// that ran short, ErrBadCharacter or ErrBadCost if the walker's own sheet
// cannot be priced, or ErrSaveFailed with a populated report.
func (m *Manager) Move(ctx context.Context, in *MoveInput) (*MoveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("move: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("move: %w", ErrNoMemberID)
	}
	if len(in.Path) == 0 {
		return nil, fmt.Errorf("move: %w", ErrEmptyPath)
	}

	scope, err := m.openForWrite(ctx, in.Session)
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
		// Afford deliberately returns no world-clock declarations. Empty is the
		// only valid selector there; a non-empty ID left over from a dissolved
		// fight must not turn into a free move.
		if in.DeclarationID != "" {
			return nil, fmt.Errorf("move: %w", ErrStaleDeclaration)
		}
		cost = &walkCost{}
	}

	// THE ONE PLACE A SPEND GOES for this verb, mirroring Attack's own comment
	// at its call site: nil profile means FREE ROAM ONLY — turn-clock offer
	// compilation blocks a member whose sheet it cannot load rather than
	// silently pricing them as free. combat.Pay
	// treats a nil profile as a free action by its own contract (see
	// [combat.SpendProfile]), so this branch is a shortcut rather than a
	// second free-action path.
	if cost.profile != nil {
		if err := combat.Pay(cost.sheet, cost.profile); err != nil {
			left := cost.sheet.CapacityLeft(combat.CapacityMovement)
			return nil, fmt.Errorf("move: %w: %s", ErrCannotAfford, movementShortfall(cost.feet, left))
		}
	}

	// Standing, asked here and only NOW — after every gate that must refuse
	// without touching a sheet has already passed (Copilot's own precedence
	// finding on #1171, pinned by TestNotActiveWinsOverAffordability).
	//
	// THIS IS THE WALK'S FIRST ANSWER, NOT ITS ONLY ONE. It was batched once
	// for the whole walk (rpg-toolkit#1137) on the stated grounds that "a Move
	// cannot down or revive anyone, so the answer is stable across every
	// step's own Discovery". Announcing steps to the rules made that false: an
	// opportunity attack resolves inside a step and can drop the walker
	// mid-path (rpg-project#316, ruling R6). runWalk re-asks per step and
	// stops when the walker goes down; this call is what the first step reads.
	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	res, err := m.runWalk(ctx, scope, in.Member, in.Path, down)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	// The spend is durable only once the walk it paid for actually happened.
	// Charged for the WHOLE requested path regardless of whether the walk
	// stopped early on an Outcome or a Formed bubble (see runWalk) — v1 does
	// not refund an interrupted walk's unused cells, the same way a paid-for
	// swing is not refunded for missing.
	if cost.sheet != nil {
		if err := m.saveWalker(ctx, scope, cost.sheet); err != nil {
			return nil, fmt.Errorf("move: %w", err)
		}
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	return &MoveOutput{
		Steps:      res.steps,
		Discovered: nilIfEmpty(res.discovered),
		Outcome:    res.outcome,
		Formed:     res.formed,
		Saved:      report,
		Delivery:   delivery,
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
	data := sheet.ToData()
	if err := m.characters.SaveCharacter(ctx, data); err != nil {
		report := SaveReport{Written: scope.written, Failed: []string{"character:" + data.ID}}
		return &SaveError{Report: report, Err: fmt.Errorf("saving character: %w", err)}
	}

	// A second save of the SAME aggregate this call already reported — a
	// killing swing's own saveDirty, then this sheet readied and saved again
	// for something later in the same verb — writes the newer state (never
	// stale, always correct) but must not duplicate the NAME in the report:
	// a caller reading Written to know what landed should see one entry per
	// aggregate, not a count of how many times it was touched (Copilot's own
	// finding on PR #1222).
	aggregate := "character:" + data.ID
	for _, w := range scope.written {
		if w == aggregate {
			return nil
		}
	}
	scope.written = append(scope.written, aggregate)
	return nil
}

// walkResult is what one run of the walk produced.
type walkResult struct {
	steps      []Step
	discovered map[string]Discovery
	outcome    *Outcome
	formed     *Formed
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
	ctx context.Context, scope *writeScope, member string, path []spatial.Position, down map[string]bool,
) (*walkResult, error) {
	res := &walkResult{discovered: map[string]Discovery{}}

	for i, cell := range path {
		// Where the walker STILL stands, read off the composition rather than
		// carried forward from the previous cell — the same reason the step's
		// landing is read off the answer below instead of echoed from the
		// input.
		from, err := positionOf(scope.enc, member)
		if err != nil {
			return nil, err
		}

		// ANNOUNCE, THEN STEP. This is resolution.NewMovement's contract, not
		// an ordering preference: an opportunity attack fires because the
		// mover is LEAVING reach, and the reactor's swing enforces melee reach
		// against where the target IS. Announcing after the step would hand
		// the strike a departed target, the swing would refuse as out of
		// range, and a refused reaction fails the whole interaction.
		//
		// The same call the monster's path makes through moverSeam, so both
		// walks provoke the same things in the same order.
		if _, err := m.announceStep(
			ctx, scope, scope.enc, encounter.MemberID(member), string(KindPlayer), from, cell,
		); err != nil {
			return nil, err
		}

		// STANDING IS RE-ASKED PER STEP, and the walk stops here if a reaction
		// dropped the walker (rpg-project#316, ruling R6). Before this slice a
		// Move could not down anyone, so Manager.Move asked once for the whole
		// walk; announcing steps made that false. The batched answer above is
		// refreshed rather than kept, so the discoveries this step projects
		// are read against who is standing NOW.
		//
		// The walker falls in the cell they were LEAVING, never the one they
		// were entering — the reaction fired because they were leaving it, and
		// its strike was checked against the cell they still stood on. The
		// encounter's own Move case gives the identical answer on the driven
		// path; two paths, one rule.
		down, err = discoveryStanding(scope)
		if err != nil {
			return nil, err
		}
		if down[member] {
			return res, nil
		}

		stepped, err := scope.enc.Step(&encounter.StepInput{
			Member: encounter.MemberID(member),
			To:     cell,
		})
		if err != nil {
			// Nothing is saved on a mid-walk rejection. The member has really
			// moved in memory for the steps already taken, but that encounter is
			// discarded unsaved, so the persisted world is untouched.
			return nil, refusedStep(i, len(path), cell, err)
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
			Seq:      stepped.Seq,
		})
		mergeDiscoveries(res.discovered, projectDiscoveries(stepped.IntelDeltas, down))

		if stepped.Outcome != nil {
			// The encounter ended underfoot. Every remaining step is abandoned:
			// a closed encounter refuses movement anyway, and attempting them
			// would turn a clean stop into a rejection the caller must interpret.
			res.outcome = projectOutcome(stepped.Outcome)
			return res, nil
		}

		if stepped.Formed != nil {
			res.formed = projectFormed(stepped.Formed)
			return res, nil
		}
	}

	return res, nil
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
	out := &Formed{Seq: f.Seq}
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
			// recorded and persisted, sight refreshed, EventMoved fanned out to
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
