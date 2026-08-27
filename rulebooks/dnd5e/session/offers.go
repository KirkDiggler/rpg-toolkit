// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// compiledOffer is the internal, per-verb declaration Afford builds and each
// mutating verb regenerates before execution. It never crosses the host
// boundary: only its [Declaration] projection does.
//
// ONE COMPILED OFFER PER VERB/ACTION/SPEND VARIANT. v1 has exactly one spend
// variant per verb — the next swing's profile for Attack, SlotNone for Move
// and EndTurn — so full Afford compilation produces one offer per verb. The day
// a bonus-action strike lands, it arrives as a second Attack offer with a
// distinct slot, and the collision guard keys the two apart by selector
// material.
//
// The [targets] map is the shared, target-specific gate result Attack enforces
// against a chosen target before executing: it carries each candidate's
// reach verdict keyed by member ID, independent of the declaration's global
// turn/economy gate. It is populated for VerbAttack only.
type compiledOffer struct {
	// declaration is the public projection a host reads. Its ID is the
	// selector for this compiled offer, empty on a blocker.
	declaration Declaration
	// attack is the complete validated action definition for VerbAttack.
	// Non-nil for a compiled Attack, nil for Move/EndTurn and every blocker.
	attack *combatActions.Definition
	// targets is the per-candidate reach verdict for VerbAttack, keyed by
	// candidate member ID. Empty for Move/EndTurn and every blocker. Execution
	// looks a chosen target up here to re-enforce reach before resolving.
	targets map[string]targetPreflight
	// sheet is the already-loaded, turn-readied actor used to compile this
	// offer. Attack and Move execution reuse it rather than selecting one
	// definition and independently loading or pricing another. Nil for EndTurn
	// and blockers.
	sheet *character.Character
	// price is the exact Attack price compiled into attack.Cost and handed to
	// resolution. Nil for Move, EndTurn, and blockers.
	price *swingPrice
	// cast is the one raw resolution-participant snapshot gathered and strictly
	// preflighted while this Attack offer was compiled. Selected execution hands
	// these exact data pointers to resolution and never refetches a participant.
	// Empty for Move, EndTurn, and blockers.
	cast []resolution.Participant
	// verb and slot are the selector material the declaration ID is derived
	// from, kept here so the collision guard can compare offers by selector
	// material rather than candidate state.
	verb Verb
	slot Slot
	// variant is the canonical selector-variant bytes the declaration ID is
	// derived from — a sealed string for Move/EndTurn, the marshaled
	// definition for Attack. Equality compares these bytes so two offers
	// with the same selector material are recurrence, not a collision.
	variant json.RawMessage
}

// targetPreflight is the shared, target-specific gate result for one
// candidate. Reused by Attack's execution enforcement: the executor looks a
// chosen target up by member ID and refuses when available is false, echoing
// why verbatim. Never crosses the host boundary.
type targetPreflightFunc func(
	enc *encounter.Encounter,
	positions map[string]spatial.Position,
	holdings []intel.Holding,
	member string,
	maxRangeFeet int,
) ([]targetPreflight, error)

type targetPreflight struct {
	// member is the candidate member ID.
	member string
	// available is the target-specific reach verdict, independent of the
	// declaration's global turn/economy gate.
	available bool
	// why is present if and only if available is false. Carries a
	// candidate-level reason (ShortfallTargetOutOfReach today); global
	// turn/economy reasons are never copied here.
	why *Shortfall
}

// compileOffersFor builds only the requested compiled offers for one member on
// the turn clock, applying the same per-verb blocker matrix. [Manager.Afford]
// requests all three verbs from one actor snapshot; Move and Attack each request
// only their own offer before selecting an echoed ID. EndTurn execution keeps
// its direct clock-only builder.
//
// BUILD ORDER FOLLOWS THE BLOCKER MATRIX. EndTurn is built first from the
// clock alone — it is governed solely by its clock/turn gate, so it stays
// compiled through Downed and an unreadable character. Move and Attack both
// use the one strict actorSheet supplied by the caller: combat.IsDown on that
// sheet blocks both, an unreadable character (ErrBadCharacter) blocks both, and
// a bad Attack compilation (ErrBadAttack) blocks Attack alone while Move
// continues off the readied sheet. Actor blocking and turn readying happen once
// regardless of how many verbs were requested. NotYourTurn is handled before
// actor loading.
//
// Every compiled Attack, turn-clock Move, and EndTurn receives a selector ID
// through [declarationID]; blockers carry an empty ID, no AttackRef, and an
// empty candidate slice. The collision guard fails closed when two
// non-identical compiled offers share an ID — offer equality compares selector
// material (verb, slot, variant), never candidate state.
func (m *Manager) compileOffersFor(
	ctx context.Context,
	enc *encounter.Encounter,
	data *SessionData,
	sessionID string,
	member string,
	clock *encounter.ClockOfOutput,
	actor actorSheet,
	verbs ...Verb,
) ([]compiledOffer, error) {
	requested := make(map[Verb]bool, len(verbs))
	for _, verb := range verbs {
		requested[verb] = true
	}

	var endTurn compiledOffer
	if requested[VerbEndTurn] {
		var err error
		endTurn, err = m.buildEndTurnOffer(sessionID, member)
		if err != nil {
			return nil, err
		}
	}

	// UNREADABLE CHARACTER: actor is the strict load Attack and Move execute
	// from. The caller performs that load exactly once, so standing and offer
	// compilation cannot observe different repository snapshots. ErrBadCharacter
	// blocks Attack and Move while EndTurn, already built from the clock alone,
	// continues.
	if actor.err != nil {
		why := Shortfall{
			Reason: ShortfallUnreadable,
			Text:   fmt.Errorf("member %q: %w: %v", member, ErrBadCharacter, actor.err).Error(),
		}
		return finishRequestedOffers(requested,
			blockedCompiledOffer(VerbAttack, TargetMember, why),
			blockedCompiledOffer(VerbMove, TargetPath, why),
			blockedCompiledOffer(VerbActivate, TargetNone, why),
			endTurn,
		)
	}

	sheet := actor.sheet
	if sheet == nil {
		why := Shortfall{
			Reason: ShortfallUnreadable,
			Text:   fmt.Errorf("member %q: %w", member, ErrBadCharacter).Error(),
		}
		return finishRequestedOffers(requested,
			blockedCompiledOffer(VerbAttack, TargetMember, why),
			blockedCompiledOffer(VerbMove, TargetPath, why),
			blockedCompiledOffer(VerbActivate, TargetNone, why),
			endTurn,
		)
	}

	if actor.downed {
		why := Shortfall{Reason: ShortfallDowned, Text: "member is downed"}
		return finishRequestedOffers(requested,
			blockedCompiledOffer(VerbAttack, TargetMember, why),
			blockedCompiledOffer(VerbMove, TargetPath, why),
			blockedCompiledOffer(VerbActivate, TargetNone, why),
			endTurn,
		)
	}

	// Ready the sheet for the turn: Move reads CapacityLeft off the readied
	// sheet, and Attack's price is the same readying's CostOfSwing. One
	// readying serves both — priceSwing below re-readies idempotently, a
	// no-op once the economy is filed under this turn.
	if err := readyForTurn(ctx, sheet, clock.Round); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadCost, err)
	}

	// ONE blocker row for Activate on the early paths above, and N compiled
	// rows here. That asymmetry is the same one Attack already has between a
	// blocker and a compiled offer, scaled by a verb that compiles more than
	// one: a member whose sheet will not load has no abilities to enumerate,
	// so there is nothing to emit N of.
	var activations []compiledOffer
	if requested[VerbActivate] {
		var err error
		activations, err = m.buildActivationOffers(sessionID, member, sheet)
		if err != nil {
			return nil, err
		}
	}

	var move compiledOffer
	if requested[VerbMove] {
		var err error
		move, err = buildMoveOffer(sessionID, member, sheet)
		if err != nil {
			return nil, err
		}
	}
	if !requested[VerbAttack] {
		return finishRequestedOffers(requested, append([]compiledOffer{move, endTurn}, activations...)...)
	}

	// Price BEFORE assembly. The complete Definition is selector material, so
	// compiling a costless weapon and merely pricing beside it would let two
	// different executable prices hash to the same offer. AssembleAttack clones
	// this actual profile into Definition.Cost; the resolution Cost receives a
	// second clone so selector material and execution data cannot alias.
	price, err := m.priceSwing(ctx, enc, member, sheet)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadCost, err)
	}
	// price.cost is never nil here: priceSwing returns a nil cost only on
	// the world clock, already ruled out by the caller.
	pricedProfile := combatActions.CloneSpendProfile(price.cost.Profile)

	// BAD ATTACK COMPILATION: a sheet that loads fine but names a weapon
	// this build cannot compile blocks Attack only — Move and EndTurn
	// continue, because Move reads the readied sheet's movement capacity and
	// EndTurn reads only the clock.
	definition, err := character.AssembleAttack(sheet, &character.AssembleAttackInput{
		Slot: character.SlotMainHand,
		Cost: pricedProfile,
	})
	if err != nil {
		why := Shortfall{
			Reason: ShortfallUnreadable,
			Text:   fmt.Errorf("member %q: %w: %v", member, ErrBadAttack, err).Error(),
		}
		return finishRequestedOffers(requested, append(
			[]compiledOffer{
				blockedCompiledOffer(VerbAttack, TargetMember, why),
				move,
				endTurn,
			}, activations...)...)
	}
	price.cost.Profile = combatActions.CloneSpendProfile(definition.Cost)
	slot := slotOf(definition.Cost)

	// Budget gate, asked non-mutatingly through the SAME check combat.Pay
	// runs to completion before touching anything. The sheet this call
	// loaded is shared with Move above, so the gate must not spend from it;
	// CanPay is exactly the check, and a failing check leaves the ledger
	// untouched for the move declaration already built.
	budgetOK := combat.CanPay(sheet, definition.Cost)
	var budgetWhy *Shortfall
	if !budgetOK {
		sf := shortfallForPay(sheet, definition.Cost, slot)
		budgetWhy = &sf
	}

	roster, err := enc.Members()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadCost, translate(err))
	}
	positions := rosterPositions(roster)

	holdings, err := enc.View(&encounter.ViewInput{Member: encounter.MemberID(member)})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadCost, translate(err))
	}

	candidates, err := m.targetPreflight(enc, positions, holdings, member, definition.Attack.Delivery.MaxRangeFeet())
	if err != nil {
		return nil, err
	}

	// Compile the raw resolution cast once, then strictly exercise the same
	// public character/monster load-and-attach APIs resolution uses. Candidate
	// failures stay on their rows; any unreadable participant also disables the
	// declaration globally because resolution's pass-everyone cast could not
	// start against any selected target. The ephemeral attach bus and loaded
	// runtime values die here; only the exact raw data snapshot is retained.
	cast, dependencyFailures := m.compileResolutionCast(ctx, data, roster, price.payer)
	var dependencyWhy *Shortfall
	for _, failure := range dependencyFailures {
		why := Shortfall{
			Reason: ShortfallUnreadable,
			Text:   fmt.Sprintf("resolution participant %q is unreadable: %v", failure.member, failure.err),
		}
		if dependencyWhy == nil {
			copied := why
			dependencyWhy = &copied
		}
		for i := range candidates {
			if candidates[i].member == failure.member {
				candidates[i].available = false
				candidateWhy := why
				candidates[i].why = &candidateWhy
				break
			}
		}
	}

	// Top Attack available requires the global dependency and budget gates AND at least one
	// candidate in reach. The global budget reason takes precedence over the
	// no-target-in-reach reason at the declaration level; if the budget
	// passes but no candidate is in reach, the declaration's Why is
	// NoTargetInReach while the candidate rows remain, each carrying its own
	// target-specific verdict.
	anyAvailable := false
	for _, c := range candidates {
		if c.available {
			anyAvailable = true
			break
		}
	}
	attackAvailable := dependencyWhy == nil && budgetOK && anyAvailable
	var attackWhy *Shortfall
	switch {
	case dependencyWhy != nil:
		attackWhy = dependencyWhy
	case !budgetOK:
		attackWhy = budgetWhy
	case !anyAvailable:
		noTarget := Shortfall{Reason: ShortfallNoTargetInReach, Text: "no target in reach"}
		attackWhy = &noTarget
	}

	attackRef := attackRefFor(definition)
	attackID, attackVariant, err := selectorIDFor(sessionID, member, VerbAttack, slot, &definition, "")
	if err != nil {
		return nil, err
	}

	targets := make(map[string]targetPreflight, len(candidates))
	for _, c := range candidates {
		targets[c.member] = c
	}

	attack := compiledOffer{
		declaration: Declaration{
			Verb:       VerbAttack,
			Slot:       slot,
			Available:  attackAvailable,
			Why:        attackWhy,
			ID:         attackID,
			Attack:     &attackRef,
			TargetKind: TargetMember,
			Candidates: projectCandidates(candidates),
		},
		attack:  &definition,
		targets: targets,
		sheet:   sheet,
		price:   price,
		cast:    cast,
		verb:    VerbAttack,
		slot:    slot,
		variant: attackVariant,
	}

	return finishRequestedOffers(requested, append([]compiledOffer{attack, move, endTurn}, activations...)...)
}

// finishRequestedOffers filters candidate offers to the requested verbs and
// runs every compiled selector through the collision guard. Zero-value
// candidates are ignored; blockers are retained by their declaration verb.
func finishRequestedOffers(requested map[Verb]bool, candidates ...compiledOffer) ([]compiledOffer, error) {
	// len(requested) is a floor rather than a count now that VerbActivate
	// contributes N rows for one requested verb.
	offers := make([]compiledOffer, 0, len(requested))
	for _, offer := range candidates {
		if requested[offer.declaration.Verb] {
			offers = append(offers, offer)
		}
	}
	if err := guardOfferCollisions(offers); err != nil {
		return nil, err
	}
	return offers, nil
}

// actorSheet is one strict repository snapshot for Attack and turn-clock Move.
// Its error is carried into compileOffersFor so Afford can project an Unreadable
// blocker while execution can still select against exactly the same compiled
// shape. A successful result always has a non-nil sheet.
type actorSheet struct {
	sheet  *character.Character
	downed bool
	err    error
}

// loadActorSheet performs the one strict actor load shared by the downed gate,
// offer compilation, pricing, and execution. combat.IsDown is evaluated once
// on that sheet so callers and compileOffersFor reuse one verdict as well as one
// repository snapshot.
func (m *Manager) loadActorSheet(ctx context.Context, member string) actorSheet {
	sheet, err := m.loadAttackSheet(ctx, member)
	if err != nil || sheet == nil {
		return actorSheet{sheet: sheet, err: err}
	}
	return actorSheet{sheet: sheet, downed: combat.IsDown(sheet)}
}

// buildEndTurnOffer builds the compiled EndTurn declaration from the clock
// alone. EndTurn is governed solely by its clock/turn gate; the caller has
// already ruled out NotYourTurn, so EndTurn is available here. Downed and an
// unreadable character do not reach it — it carries no sheet, no candidates,
// and no currency.
func (m *Manager) buildEndTurnOffer(session, member string) (compiledOffer, error) {
	id, variant, err := selectorIDFor(session, member, VerbEndTurn, SlotNone, nil, "")
	if err != nil {
		return compiledOffer{}, err
	}
	return compiledOffer{
		declaration: Declaration{
			Verb:       VerbEndTurn,
			Slot:       SlotNone,
			Available:  true,
			ID:         id,
			TargetKind: TargetNone,
			Candidates: []TargetCandidate{},
		},
		verb:    VerbEndTurn,
		slot:    SlotNone,
		variant: variant,
	}, nil
}

// buildMoveOffer builds the compiled Move declaration off the readied sheet.
// Move has no fixed price until a path is chosen, so Available answers only
// whether ANY step at all is still possible — one cell, five feet, the
// smallest unit this grid has — and Remaining is the actual feet left.
func buildMoveOffer(session, member string, sheet *character.Character) (compiledOffer, error) {
	id, variant, err := selectorIDFor(session, member, VerbMove, SlotNone, nil, "")
	if err != nil {
		return compiledOffer{}, err
	}
	left := sheet.CapacityLeft(combat.CapacityMovement)
	decl := Declaration{
		Verb:       VerbMove,
		Slot:       SlotNone,
		Remaining:  &left,
		ID:         id,
		TargetKind: TargetPath,
		Candidates: []TargetCandidate{},
	}
	if left >= 5 {
		decl.Available = true
		return compiledOffer{declaration: decl, sheet: sheet, verb: VerbMove, slot: SlotNone, variant: variant}, nil
	}
	why := Shortfall{
		Reason:   ShortfallNoBudget,
		Currency: CurrencyMovement,
		// Needed names the smallest step this grid has (five feet, one cell)
		// rather than a specific request's cost: unlike Attack's fixed
		// action price, a walk's cost is a fact about a PATH this read is
		// never given, so "needed" can only say how much the least possible
		// move would take.
		Needed: 5,
		Left:   left,
		Text:   fmt.Sprintf("movement: %d ft left", left),
	}
	decl.Why = &why
	return compiledOffer{declaration: decl, sheet: sheet, verb: VerbMove, slot: SlotNone, variant: variant}, nil
}

// blockedCompiledOffer is the compiledOffer shape every early per-verb
// blocker emits: an empty selector ID, no AttackRef, an empty candidate
// slice, and the fixed target kind for the verb. It carries no selector
// material — a blocker has not compiled an offer — so it is skipped by the
// collision guard.
func blockedCompiledOffer(verb Verb, kind TargetKind, why Shortfall) compiledOffer {
	return compiledOffer{
		declaration: blockedDeclaration(verb, kind, why),
	}
}

// buildTargetPreflight enumerates the ruled candidate universe for one
// compiled Attack: every live CurrentVia-nonempty holding except the actor,
// exactly once, sorted by member ID. Stale memories (empty CurrentVia) and
// the actor are excluded. A live candidate whose position is missing from the
// roster is an internal inconsistency this read fails closed on rather than
// silently omitting.
//
// Each candidate's availability is the target-specific reach gate,
// independent of the declaration's global turn/economy gate: in range
// carries available true, out of range carries available false with a
// ShortfallTargetOutOfReach why. The declaration-level NoTargetInReach
// reason is decided by the caller from whether any candidate is available,
// not here.
func buildTargetPreflight(
	enc *encounter.Encounter,
	positions map[string]spatial.Position,
	holdings []intel.Holding,
	member string,
	maxRangeFeet int,
) ([]targetPreflight, error) {
	// Collect the live candidate universe first, sorted by member ID so the
	// projection is deterministic regardless of holding iteration order.
	candidateIDs := make([]string, 0, len(holdings))
	for _, h := range holdings {
		subject := string(h.Subject)
		if subject == member || len(h.CurrentVia) == 0 {
			continue
		}
		candidateIDs = append(candidateIDs, subject)
	}
	sort.Strings(candidateIDs)

	from, ok := positions[member]
	if !ok {
		// The actor's own position is missing: this is an internal
		// inconsistency the same law applies to — a live candidate without a
		// position fails closed, and so does the actor the candidates are
		// measured from.
		return nil, fmt.Errorf("attack offers: actor %q has no position in the roster", member)
	}

	out := make([]targetPreflight, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		to, ok := positions[id]
		if !ok {
			// A live candidate the encounter no longer places is never
			// silently omitted: it fails the read so the inconsistency is
			// surfaced rather than hidden as a missing row.
			return nil, fmt.Errorf("attack offers: live candidate %q has no position in the roster", id)
		}
		if inRange(enc, from, to, maxRangeFeet) {
			out = append(out, targetPreflight{member: id, available: true})
			continue
		}
		why := Shortfall{Reason: ShortfallTargetOutOfReach, Text: "target out of reach"}
		out = append(out, targetPreflight{member: id, available: false, why: &why})
	}
	return out, nil
}

// projectCandidates copies the internal preflight slice into the public
// candidate slice, preserving the sorted order buildTargetPreflight fixed.
func projectCandidates(candidates []targetPreflight) []TargetCandidate {
	out := make([]TargetCandidate, 0, len(candidates))
	for _, c := range candidates {
		var why *Shortfall
		if c.why != nil {
			copied := *c.why
			why = &copied
		}
		out = append(out, TargetCandidate{
			Member:    c.member,
			Available: c.available,
			Why:       why,
		})
	}
	return out
}

// selectCompiledOffer returns the one current, available offer named by an
// echoed selector. Empty IDs are invalid input; every other miss — unknown ID,
// wrong verb, collision-shaped ambiguity, or an offer whose current global gate
// is false — is stale current-world state.
func selectCompiledOffer(offers []compiledOffer, verb Verb, id string) (compiledOffer, error) {
	if id == "" {
		return compiledOffer{}, ErrNoDeclarationID
	}

	var selected *compiledOffer
	for i := range offers {
		if offers[i].verb != verb || offers[i].declaration.ID != id {
			continue
		}
		if selected != nil {
			return compiledOffer{}, ErrStaleDeclaration
		}
		selected = &offers[i]
	}
	if selected == nil || !selected.declaration.Available {
		return compiledOffer{}, ErrStaleDeclaration
	}
	return *selected, nil
}

// selectorIDFor computes both the declaration selector ID and the canonical
// selector-variant bytes for one compiled offer. The variant is kept on the
// compiledOffer so the collision guard can compare offers by selector
// material rather than candidate state. For Move and EndTurn the variant is
// a sealed string; for Attack it is the marshaled, validated definition.
func selectorIDFor(
	session, member string, verb Verb, slot Slot,
	attack *combatActions.Definition, ability string,
) (id string, variant json.RawMessage, err error) {
	variant, err = selectorVariant(verb, attack, ability)
	if err != nil {
		return "", nil, err
	}
	variant, err = canonicalSelectorVariant(variant)
	if err != nil {
		return "", nil, err
	}
	id, err = declarationID(declarationIDInput{
		Session: session,
		Member:  member,
		Verb:    verb,
		Slot:    slot,
		Attack:  attack,
		Ability: ability,
	})
	if err != nil {
		return "", nil, err
	}
	return id, variant, nil
}

// guardOfferCollisions runs the compiled offers with non-empty selector IDs
// through the collision guard. Blockers carry empty IDs and are skipped —
// they are distinct by verb by construction and hold no selector material.
// Two non-identical compiled offers sharing an ID fail closed as an internal
// provider defect; a recurring offer (same selector material) is not a
// collision.
func guardOfferCollisions(offers []compiledOffer) error {
	idx := newIndexCompiledOffers(
		func(o compiledOffer) (string, error) { return o.declaration.ID, nil },
		offerSelectorEqual,
	)
	for _, o := range offers {
		if o.declaration.ID == "" {
			continue
		}
		if err := idx.add(o); err != nil {
			return err
		}
	}
	return nil
}

// offerSelectorEqual reports whether two compiled offers share selector
// material — verb, slot, and the RFC 8785 canonical variant bytes the
// declaration ID is derived from. Candidate and raw-cast state are deliberately
// excluded: two offers with the same selector but different current world data
// are the same offer recurring, not a collision.
func offerSelectorEqual(a, b compiledOffer) bool {
	if a.verb != b.verb || a.slot != b.slot {
		return false
	}
	aVariant, err := canonicalSelectorVariant(a.variant)
	if err != nil {
		return false
	}
	bVariant, err := canonicalSelectorVariant(b.variant)
	if err != nil {
		return false
	}
	return bytes.Equal(aVariant, bVariant)
}

// verbRank orders declarations in the deterministic output order the seam
// promises: Attack first, then Move, then EndTurn — the order a turn panel
// renders its action, movement and end-turn controls. Assertions may rely on
// this order; it never depends on candidate state.
func verbRank(v Verb) int {
	switch v {
	case VerbAttack:
		return 0
	case VerbMove:
		return 1
	case VerbActivate:
		return 2
	case VerbEndTurn:
		return 3
	default:
		return 4
	}
}

// sortDeclarations orders the projected declarations by the seam's
// documented verb rank, so [Manager.Afford]'s output is deterministic
// regardless of the build order compileOffersFor happens to use.
func sortDeclarations(decls []Declaration) {
	sort.SliceStable(decls, func(i, j int) bool {
		if verbRank(decls[i].Verb) != verbRank(decls[j].Verb) {
			return verbRank(decls[i].Verb) < verbRank(decls[j].Verb)
		}
		// WITHIN a verb, and only Activate ever has two. Verb rank alone was a
		// total order while every verb compiled one offer; with seven Activate
		// rows it leaves their order to whatever the caller appended, which is
		// stable today and stable by accident. Ability ref is a fact about the
		// character, so the panel's order is too.
		if decls[i].Ability != nil && decls[j].Ability != nil {
			return decls[i].Ability.Ref < decls[j].Ability.Ref
		}
		return false
	})
}
