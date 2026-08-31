// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// AffordInput asks what one member can still declare this turn.
type AffordInput struct {
	// Session is the session to look inside.
	Session string

	// Member is whose budget is being asked about. Required.
	//
	// THE HOST MUST BIND THIS TO THE AUTHENTICATED CALLER, exactly as
	// [Manager.Where] requires and for the same reason: this package cannot
	// tell who is asking, and a client-supplied ID wired through unchecked
	// turns a caller-scoped budget read into one that answers about anybody.
	Member string
}

// Verb names a seam action [Manager.Afford] can price.
//
// A closed enum owned here rather than a reuse of the rulebook's own action
// vocabulary — a declaration is a question about what THIS SEAM lets a member
// declare, not the currency the rulebook happens to price it in. One verb may
// produce several declarations when several compiled variants exist.
type Verb string

const (
	// VerbAttack is [Manager.Attack]: swinging a weapon at another member.
	VerbAttack Verb = "attack"

	// VerbMove is [Manager.Move]: walking along a path while on the turn
	// clock. Free-roam movement is not priced and so never appears here —
	// see [Manager.Afford]'s own doc.
	VerbMove Verb = "move"

	// VerbActivate is [Manager.Activate]: using a combat ability or feature the
	// character already carries — Dodge, Dash, Disengage, Help, Hide, Rage,
	// Second Wind.
	//
	// One offer per carried ability. A level-1 barbarian gets several of these,
	// and [Slot] is what separates Rage on the bonus shape from Dodge on the
	// action one — they cannot share a row, because Slot is per declaration.
	//
	// Its selector variant is the ability's own ref rather than a sealed
	// string: one verb, seven offers, and the ref is what tells them apart.
	VerbActivate Verb = "activate"

	// VerbEndTurn is [Manager.EndTurn]: ending the member's turn. Like Move it
	// carries no authored action definition, so its declaration selector uses a
	// sealed variant string rather than a serialized [actions.Definition].
	// Spelled here so the declaration selector's verb byte is owned by the seam
	// that prices it, not borrowed from the rulebook's action vocabulary.
	VerbEndTurn Verb = "end_turn"
)

// Slot names which of a turn's three economy shapes a [Declaration] draws
// from, mirroring the vocabulary a UI already has buttons for — the same four
// values character.EconomySlot carries (action_economy_types.go), spelled
// here rather than imported across the seam (S2).
//
// It answers a narrower question than "what does this cost." A declaration
// that lights none of the three shapes is not thereby free — see SlotNone.
type Slot string

const (
	// SlotNone is a declaration that draws no per-turn slot: nothing to light
	// on an action, bonus action or reaction shape. NOT the same claim as
	// "costs nothing" — Extra Attack's second swing spends a banked attack
	// and lights no shape at all, and Affordable can still be false for it
	// once that bank runs out.
	SlotNone Slot = ""
	// SlotAction draws the standard action.
	SlotAction Slot = "action"
	// SlotBonus draws the bonus action.
	SlotBonus Slot = "bonus"
	// SlotReaction draws the reaction.
	SlotReaction Slot = "reaction"
)

// Declaration is one server-compiled action/cost variant a member could
// still declare this turn, and whether every gate applicable to it currently
// passes. Mirrors the merged proto's Declaration (rpg-project#272/273).
//
// DECLARATIONS, NOT REMAINING CURRENCIES — Kirk's ruling on rpg-toolkit#1138:
// "backend tells dumb client what it can do." A read that answered
// {action:0, bonus:1} would still hand a client the raw ledger and trust it to
// know that a swing costs an action and that Extra Attack banks capacity —
// which is the D&D-in-the-client violation the economy exists to prevent, only
// moved one read earlier. A Declaration answers the question a client actually
// has, CAN I DO THIS, and keeps the arithmetic where it has always lived:
// server-side, behind [combat.SpendProfile]. See
// docs/adr/0042-afford-answers-in-declarations-not-currencies.md.
//
// ONE DECLARATION PER COMPILED OFFER — which is NOT one per verb, and was
// never quite the same claim.
//
// [VerbActivate] contributes one declaration per carried ability. [VerbAttack]
// contributes the main-hand swing and, after a qualifying Attack action, a
// bonus-slot off-hand swing. A consumer that indexes declarations BY VERB, or
// treats a second row for a verb as a producer defect, is reading a coincidence
// as a contract.
//
// It is still not one declaration per TARGET: each Attack declaration carries
// its own candidate universe, with target-specific availability on each row.
// The client renders availability, identity, target kind, and candidates
// verbatim and never derives game rules; every verb regenerates the selected
// offer before execution.
type Declaration struct {
	// Verb is which seam action this prices.
	Verb Verb `json:"verb"`

	// Slot is which economy shape this declaration would spend, or SlotNone.
	// Read off the SAME SpendProfile the door would charge — never a second
	// table asserting what a verb "usually" costs — so a class table changing
	// what an action buys changes this automatically.
	Slot Slot `json:"slot"`

	// Available is whether every gate applicable to this verb currently
	// passes. NO OMITEMPTY: false is an ANSWER, not an absence — the same
	// false-vs-absent law every other bool at this seam keeps (types.go).
	// Absence of the question is instead Declarations being empty, on the
	// world clock, where the economy does not apply at all.
	//
	// For Attack, Available requires the global budget gate AND at least one
	// candidate in reach; the global budget reason takes precedence over the
	// no-target-in-reach reason at the declaration level. For Move, Available
	// is whether any step at all is still possible. For EndTurn, Available is
	// the clock/turn gate alone.
	Available bool `json:"available"`

	// Remaining is how much of this verb's own currency is left, in the
	// currency's natural unit — feet, for Move (rpg-toolkit#1169).
	//
	// PRESENT ONLY FOR VERB_MOVE. Attack and EndTurn carry no such number — a
	// swing either happens or it does not, and EndTurn has no currency — so
	// the field is nil for them. A POINTER, not an omitted zero: false-vs-absent
	// for an int (types.go's law, generalised) — Remaining:0 is a real answer
	// (nothing left this turn) and must not collide with "this verb carries no
	// such number at all".
	Remaining *int `json:"remaining,omitempty"`

	// Why is the structured reason this declaration is unavailable, present
	// if and only if Available is false — the same presence law Remaining
	// keeps for "this verb carries no such number": absence beside Available
	// true is itself the answer ("nothing ran out"). Carries the reason and
	// the figures a UI acts on; the server owns refusal precedence. For
	// Attack, NoBudget takes precedence over NoTargetInReach at the
	// declaration level, and NoTargetInReach does not remove candidate rows.
	Why *Shortfall `json:"why,omitempty"`

	// ID is the opaque deterministic selector for this current compiled
	// offer. Non-empty on every compiled Attack, turn-clock Move, and EndTurn
	// declaration; empty on an early per-verb blocker. The client echoes it
	// and never parses it.
	ID string `json:"id"`

	// Attack is the sole public Attack identity. Present on every compiled
	// Attack declaration — including one disabled by budget or target gates,
	// which still carries its compiled ref — and absent for Move, EndTurn,
	// and early per-verb blockers.
	Attack *AttackRef `json:"attack,omitempty"`

	// Ability is the sole public activation identity, present on every
	// compiled Activate declaration — including one disabled by a budget,
	// charge or feature gate — and absent for Attack, Move, EndTurn and every
	// early per-verb blocker. The same presence law [Declaration.Attack]
	// keeps, for the same reason.
	Ability *AbilityRef `json:"ability,omitempty"`

	// TargetKind is fixed for every compiled or blocked declaration: Attack
	// -> TargetMember, Move -> TargetPath, EndTurn -> TargetNone. A blocker
	// keeps the fixed kind even with empty candidates, so a client always
	// knows which selector shape the verb carries.
	TargetKind TargetKind `json:"target_kind"`

	// Candidates is every member in the ruled candidate universe exactly
	// once, including unavailable targets and their server-authored
	// reasons. ShortfallNoTargetInReach does not remove these rows. Empty
	// (non-nil) for Move, EndTurn, and early per-verb blockers.
	Candidates []TargetCandidate `json:"candidates"`
}

// AffordOutput is what one member can still declare this turn.
type AffordOutput struct {
	// Clock is which kind of time the member is in. ClockWorld means the
	// action economy does not apply to them at all: Declarations is empty,
	// and that IS the answer rather than a shorter way of asking again.
	Clock ClockKind `json:"clock"`

	// Declarations is one entry per compiled OFFER — one per current Attack
	// variant, one each for Move and EndTurn, and one per activatable thing the
	// member carries — empty on the world clock — where empty IS the answer rather than a shorter way
	// of asking again, so it is never omitted from the wire either: the same
	// false-vs-absent law types.go keeps for every bool at this seam applies
	// here to the list itself. A non-Go client must read "declarations": [],
	// not a missing key that reads as "the server didn't say".
	Declarations []Declaration `json:"declarations"`
}

// Afford reports the current compiled Attack, Move, Activate and EndTurn
// offers for one active turn member. Activate compiles one per thing the member
// carries; Attack compiles its main-hand variant and any granted off-hand
// variant. Each offer carries an opaque selector execution must echo.
// Move reports Remaining rather than a fixed price because a walk's cost
// depends on a path this read is never given. On the world clock declarations
// is the complete empty answer.
//
// # The gap this closes
//
// Without this read a turn UI has exactly two options: offer every button and
// discover refusals after clicks, or re-derive 5e's economy and target rules
// client-side. The latter is the Boundary Rule violation this seam exists to
// prevent. See rpg-toolkit#1138.
//
// # A declaration, not the remaining currencies
//
// Kirk's ruling: "backend tells dumb client what it can do." Reporting
// {action:0, bonus:1} would still make the client know a swing costs an
// action and that Extra Attack banks capacity — the rule would have leaked
// through the very read meant to keep it server-side. So this returns
// [Declaration]s — can-or-cannot, per verb — carrying [Slot] so a client can
// light the action/bonus/reaction shapes it already draws without knowing WHY
// any of them are lit. See docs/adr/0042-afford-answers-in-declarations-not-currencies.md.
//
// # The same price Attack pays, never a second copy of it
//
// [Manager.compileOffersFor] assembles and prices each current Attack variant,
// clones its actual SpendProfile into Definition.Cost before hashing it, and
// asks [combat.CanPay] against the same readied sheet execution will regenerate.
// It gathers and strictly preflights one raw resolution cast shared by those
// variants. Attack then selects and reuses the chosen definition, price, sheet,
// target preflight, and exact raw cast without refetching participants. Move likewise
// reuses its compiled readied sheet; EndTurn compiles from the clock alone.
//
// # The full current gate
//
// The clock-active comparison precedes every sheet load and blocks all three
// verbs. Downed and unreadable dependencies are then applied per verb: Attack
// and Move need a standing, readable actor; Attack additionally needs every
// resolution participant to load and attach strictly; EndTurn needs only its
// real clock gate. An unreadable candidate keeps its row with Unreadable, while
// an unreadable cast dependency disables Attack globally without erasing other
// candidate reach facts. Candidate and budget failures keep a compiled Attack
// with a selector and exact reasons, but mark it unavailable; execution treats
// an echoed now-unavailable selector as [ErrStaleDeclaration].
//
// # Reads, and reads only
//
// No new state, no new gate. [Manager.priceSwing] "readies" a cold sheet's
// turn as a side effect of pricing it — see its own doc — and here that
// mutation lands on a character this call loaded for itself and hands to
// nobody: never adopted, never passed to [Manager.saveDirty], never reaching a
// repository. A save happens exactly as often from this verb as from any
// other read: never. TestAffordSavesNothing pins it the way
// TestARefusedSwingWritesNothing pins the same claim for a refused swing.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember if the member is not in this encounter.
// Missing/unreadable actor sheets and attacks are projected as per-verb
// Unreadable blockers rather than failing the whole read. Missing/unreadable
// resolution participants make the compiled Attack unavailable; target
// candidates carry their own Unreadable reason and non-target dependencies
// produce the declaration-level one. Returns ErrBadCost if
// the rulebook cannot compile this member's own price. Returns a wrapped error
// when a live candidate holds no position in the roster: a live-sight holding
// whose subject the encounter no longer places is an internal inconsistency
// this read fails closed on rather than silently omitting the candidate.
func (m *Manager) Afford(ctx context.Context, in *AffordInput) (*AffordOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("afford: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("afford: %w", ErrNoMemberID)
	}

	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("afford: %w", err)
	}
	enc, err := m.loadWorld(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("afford: %w", err)
	}

	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("afford: %w", translate(err))
	}
	if ClockKind(clock.Kind) != ClockTurn {
		// A non-nil, empty slice: the world clock's Declarations marshals as
		// "[]", never "null" — the same reason above applies to the wire
		// shape as much as the tag.
		return &AffordOutput{Clock: ClockWorld, Declarations: []Declaration{}}, nil
	}

	// NOT YOUR TURN, checked FIRST and cheaply — clock.Active is already in
	// hand, no sheet touched — the same precedence Move's own gate keeps
	// (Copilot's finding on #1171: the clock is asked before anything is
	// loaded, so a refusal this early never reads a sheet at all). The
	// same clock-active comparison Attack's own gate makes
	// (rpg-toolkit#1010/#249) — announced here BEFORE a caller ever tries
	// the verb, which is Afford's whole point. NotYourTurn blocks ALL THREE
	// verbs the same way, so the sheet is never loaded for a member whose
	// turn it is not.
	if string(clock.Active) != in.Member {
		notYourTurn := Shortfall{Reason: ShortfallNotYourTurn, Text: "not your turn"}
		return &AffordOutput{Clock: ClockTurn, Declarations: []Declaration{
			blockedDeclaration(VerbAttack, TargetMember, notYourTurn),
			blockedDeclaration(VerbMove, TargetPath, notYourTurn),
			// ONE Activate row, not seven. A member whose turn it is not
			// cannot activate ANY of them, and the reason is identical for
			// every one — so seven rows would be seven copies of "not your
			// turn" and a panel that looks like it has choices.
			blockedDeclaration(VerbActivate, TargetNone, notYourTurn),
			blockedDeclaration(VerbEndTurn, TargetNone, notYourTurn),
		}}, nil
	}

	// Attack and Move compile from one strict actor snapshot. loadActorSheet
	// asks combat.IsDown on that loaded sheet before reading any offer
	// material, so the blocker and the declaration cannot disagree across two
	// repository reads. EndTurn remains clock-only when that actor is downed or
	// unreadable.
	actor := m.loadActorSheet(ctx, in.Member)
	offers, err := m.compileOffersFor(
		ctx, enc, data, in.Session, in.Member, clock, actor,
		VerbAttack, VerbMove, VerbActivate, VerbEndTurn,
	)
	if err != nil {
		return nil, fmt.Errorf("afford: %w", err)
	}

	declarations := make([]Declaration, 0, len(offers))
	for _, o := range offers {
		declarations = append(declarations, o.declaration)
	}
	sortDeclarations(declarations)
	return &AffordOutput{Clock: ClockTurn, Declarations: declarations}, nil
}

// blockedDeclaration is the shape every early per-verb blocker emits:
// available false, why present, empty id, absent attack, empty candidates,
// and the fixed target kind for the verb. It never carries a selector id or an
// AttackRef — those belong to a compiled offer, and a blocker has not
// compiled one.
func blockedDeclaration(verb Verb, kind TargetKind, why Shortfall) Declaration {
	return Declaration{
		Verb:       verb,
		Slot:       SlotNone,
		Available:  false,
		Why:        &why,
		ID:         "",
		TargetKind: kind,
		Candidates: []TargetCandidate{},
	}
}

// currencyOfSlot maps a lit shape onto the ledger word a NoBudget shortfall
// names — the same three values Slot and Currency coincide on (types.go's
// own note on why they are still two enums).
func currencyOfSlot(slot Slot) Currency {
	switch slot {
	case SlotAction:
		return CurrencyAction
	case SlotBonus:
		return CurrencyBonus
	case SlotReaction:
		return CurrencyReaction
	default:
		return ""
	}
}

// shortfallForPay determines the structured reason a compiled price could
// not be paid, read off the SAME ledger state combat.Pay's own check
// consults — SlotsLeft/CapacityLeft, never by parsing the text Pay's error
// carries (rpg-toolkit#1010).
//
// SLOT FIRST, AND USUALLY ONLY. slot is the declaration's own answer to
// "which shape does this price light" (slotOf) — the same SpendProfile,
// asked the same way — and it covers the whole of v1's reachable economy:
// character.CostOfSwing's folding means a capacity shortfall
// always resurfaces as the action slot being spent already, so a profile
// that draws SlotNone (a purely banked swing) never actually runs out in a
// way this build can produce. That branch is still handled, defensively,
// rather than assumed away.
func shortfallForPay(sheet *character.Character, profile *combat.SpendProfile, slot Slot) Shortfall {
	if profile == nil {
		return Shortfall{Reason: ShortfallNoBudget, Text: "action cannot be paid for"}
	}

	switch slot {
	case SlotAction:
		return slotShortfall(sheet, coreCombat.ActionStandard, currencyOfSlot(slot), profile.Slots[coreCombat.ActionStandard])
	case SlotBonus:
		return slotShortfall(sheet, coreCombat.ActionBonus, currencyOfSlot(slot), profile.Slots[coreCombat.ActionBonus])
	case SlotReaction:
		return slotShortfall(sheet, coreCombat.ActionReaction, currencyOfSlot(slot), profile.Slots[coreCombat.ActionReaction])
	}

	for key, amount := range profile.Capacity {
		if left := sheet.CapacityLeft(key); left < amount {
			return Shortfall{
				Reason: ShortfallNoBudget, Needed: amount, Left: left,
				Text: fmt.Sprintf("%s: %d needed, %d left", key, amount, left),
			}
		}
	}
	return Shortfall{Reason: ShortfallNoBudget, Text: "action cannot be paid for"}
}

// slotShortfall reads one per-turn slot's own state into a Shortfall —
// currencyOfSlot's figures, filled in.
func slotShortfall(sheet *character.Character, slotKey coreCombat.ActionType, currency Currency, needed int) Shortfall {
	left := sheet.SlotsLeft(slotKey)
	return Shortfall{
		Reason: ShortfallNoBudget, Currency: currency, Needed: needed, Left: left,
		Text: fmt.Sprintf("%s: %d needed, %d left", slotKey, needed, left),
	}
}

// slotOf reads which of a turn's three slots a compiled price draws from, if
// any — the same map [combat.Pay] would charge, never a second table
// asserting what a verb "usually" costs. A profile with none of the three
// keys a turn holds answers SlotNone: it may still draw capacity (a banked
// swing Extra Attack already bought) without lighting a shape a client draws
// for action, bonus or reaction.
func slotOf(p *combat.SpendProfile) Slot {
	if p == nil {
		return SlotNone
	}
	switch {
	case p.Slots[coreCombat.ActionStandard] > 0:
		return SlotAction
	case p.Slots[coreCombat.ActionBonus] > 0:
		return SlotBonus
	case p.Slots[coreCombat.ActionReaction] > 0:
		return SlotReaction
	default:
		return SlotNone
	}
}
