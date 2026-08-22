// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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
// declare, not the currency the rulebook happens to price it in. v1 has
// exactly one entry because v1 has exactly one gated verb.
type Verb string

const (
	// VerbAttack is [Manager.Attack]: swinging a weapon at another member.
	VerbAttack Verb = "attack"

	// VerbMove is [Manager.Move]: walking along a path while on the turn
	// clock. Free-roam movement is not priced and so never appears here —
	// see [Manager.Afford]'s own doc.
	VerbMove Verb = "move"
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

// Declaration is one verb this member could still declare this turn, and
// whether they can pay for it.
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
type Declaration struct {
	// Verb is which seam action this prices.
	Verb Verb `json:"verb"`

	// Slot is which economy shape this declaration would spend, or SlotNone.
	// Read off the SAME SpendProfile the door would charge — never a second
	// table asserting what a verb "usually" costs — so a class table changing
	// what an action buys changes this automatically.
	Slot Slot `json:"slot"`

	// Affordable is whether the member could pay for this verb right now.
	//
	// NO OMITEMPTY. False is an ANSWER, not an absence — the same
	// false-vs-absent law every other bool at this seam keeps (types.go).
	// Absence of information is instead Declarations being empty, on the
	// world clock, where the question does not apply at all.
	Affordable bool `json:"affordable"`

	// Shortfall names what ran out, in the SAME words a refused Attack would
	// use — "action: 1 needed, 0 left" — because a client that cannot repeat
	// a refusal in the player's own words has been handed a boolean and
	// nothing else. Empty when Affordable.
	//
	// NO OMITEMPTY, for the same reason Affordable has none: an empty string
	// beside affordable:true is itself the answer ("nothing ran out"), not
	// the absence of one, and a non-Go client reading a missing key cannot
	// tell that from "the server didn't say".
	//
	// KEPT FOR OLDER READERS; SUPERSEDED BY Why. This is the same text as
	// Why.Text — a producer that sets one sets both (rpg-toolkit#1010). A
	// new reader takes Why, the structured form it can branch on, and never
	// this.
	Shortfall string `json:"shortfall"`

	// Target is the candidate this declaration prices, for VerbAttack. ONE
	// DECLARATION PER TARGET IN REACH (rpg-project#249 §6, Kirk): Afford
	// gates each candidate through the same reach check Attack refuses
	// with (melee one cell, the reach property two; ranged stays refused
	// as today, rpg-toolkit#1010) and emits one declaration for each
	// target that passes. That list IS the client's "enemies in reach"
	// highlight — reach is never computed client-side, it is read off
	// these. When NO candidate is in reach the seam still answers, once: a
	// single ATTACK declaration with Affordable false, Why.Reason
	// ShortfallNoTargetInReach, and this field unset.
	//
	// A POINTER, not an omitted empty string: a MOVE declaration has no
	// target at all, and the no-target-in-reach ATTACK declaration has
	// none either — neither is a target whose id happens to be empty.
	//
	// Further strikes are FURTHER DECLARATIONS of this same shape, not new
	// fields: a monk's Martial Arts bonus strike is
	// {VerbAttack, SlotBonus, target}; an off-hand swing and a flurry
	// likewise. Nil for VerbMove.
	Target *string `json:"target,omitempty"`

	// Why is the structured reason this is unaffordable, present exactly
	// when Affordable is false — the same presence law Remaining keeps for
	// "this verb carries no such number": absence beside Affordable true is
	// itself the answer ("nothing ran out"). Carries the same text
	// Shortfall does, plus the reason and the figures a UI acts on.
	// Lands with rpg-toolkit#1010.
	Why *Shortfall `json:"why,omitempty"`

	// Remaining is how much of this verb's own currency is left, in the
	// currency's natural unit — feet, for Move (rpg-toolkit#1169).
	//
	// PRESENT ONLY WHERE THE NUMBER MEANS SOMETHING BEYOND CAN-OR-CANNOT.
	// Attack's declaration has never needed one — a swing either happens or
	// it does not — but a client walking Move wants to bound its own path
	// preview to the server's real number rather than re-deriving a
	// character's speed itself, which is exactly the calculation the
	// Boundary Rule keeps off the client. Nil for VerbAttack.
	//
	// A POINTER, not an omitted zero: false-vs-absent for an int rather than
	// a bool (types.go's law, generalised) — Remaining:0 is a real answer
	// (nothing left this turn) and must not collide with "this verb carries
	// no such number at all".
	Remaining *int `json:"remaining,omitempty"`
}

// AffordOutput is what one member can still declare this turn.
type AffordOutput struct {
	// Clock is which kind of time the member is in. ClockWorld means the
	// action economy does not apply to them at all: Declarations is empty,
	// and that IS the answer rather than a shorter way of asking again.
	Clock ClockKind `json:"clock"`

	// Declarations is one entry per verb the seam prices, empty on the world
	// clock — where empty IS the answer rather than a shorter way of asking
	// again, so it is never omitted from the wire either: the same
	// false-vs-absent law types.go keeps for every bool at this seam applies
	// here to the list itself. A non-Go client must read "declarations": [],
	// not a missing key that reads as "the server didn't say".
	Declarations []Declaration `json:"declarations"`
}

// Afford reports what one member can still declare this turn: which of the
// seam's gated verbs they could pay for right now, and — when they cannot —
// the same currency-naming text a refused [Manager.Attack] or [Manager.Move]
// would carry. Move's own declaration (VerbMove, rpg-toolkit#1169) reports
// Remaining rather than a fixed price, since a walk's cost depends on a path
// this read is never given — see [affordMove].
//
// # The gap this closes
//
// Attack refuses a swing nobody can pay for with [ErrCannotAfford], and the
// refusal is careful to name the currency that ran out. Nothing on the seam
// said so BEFORE the refusal. A turn UI built against that has exactly two
// options: offer the button and let the server say no, or re-derive 5e's
// economy client-side — a level-1 fighter gets one swing, Extra Attack lands
// at level 5 — which is the Boundary Rule violation this whole seam exists to
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
// This does not reprice the swing by a second route. It calls the identical
// [Manager.priceSwing] a real swing compiles from, then charges that price
// through [combat.Pay] — the SAME gate [Manager.Attack]'s door pays through —
// against a [character.Character] loaded fresh for this call and never saved.
// A payment that succeeds answers Affordable; one that fails hands back the
// exact text a refused Attack's door would have produced, because it IS that
// text: nothing here reimplements what a currency needs or what ran out. If
// Attack and Afford could ever disagree, one of them would be pricing wrong,
// and routing both through one gate call is what makes that impossible by
// construction rather than merely tested for.
//
// # The economy's answer, not the turn's
//
// Attack does not check whose turn it currently is — nothing on this seam
// does yet, [Manager.EndTurn] aside — so this does not either. Whether a swing
// would ALSO be refused for arriving out of turn is [Manager.Turn]'s question:
// folding "not your turn" into a Shortfall here would answer a question this
// read was never asked, in a currency it does not price. A future turn gate
// belongs beside Turn's own Active field, not inside a Declaration.
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
// Returns ErrNoCharacter or ErrBadCharacter if the member is on the turn clock
// and has no loadable sheet — reachable for a monster or a malformed record,
// since a character already in a fight has one by construction. Returns
// ErrBadCost if the rulebook cannot compile this member's own price, which
// [Manager.Attack] would also refuse the same way.
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
	// the verb, which is Afford's whole point.
	if string(clock.Active) != in.Member {
		return &AffordOutput{Clock: ClockTurn, Declarations: blockedDeclarations(Shortfall{
			Reason: ShortfallNotYourTurn, Text: "not your turn",
		})}, nil
	}

	// DOWNED, asked only once we know this member is even the one the
	// clock is waiting on: a downed member is spliced out of the turn
	// order and can never be active, so NotYourTurn already covers a
	// downed BYSTANDER — this is the specific fact a client renders
	// differently ("you are down" versus "wait your turn") for the member
	// who somehow is still active despite being down.
	standing := m.standingFor(ctx, data)
	down, err := standing.Standing([]encounter.MemberID{encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("afford: %w", err)
	}
	if len(down) > 0 {
		return &AffordOutput{Clock: ClockTurn, Declarations: blockedDeclarations(Shortfall{
			Reason: ShortfallDowned, Text: "member is downed",
		})}, nil
	}

	// UNREADABLE: the same compile Attack's own door runs, so the two
	// cannot disagree about whether a swing exists to price at all
	// (rpg-toolkit#1168's other half — ADR-0042 scoped this out, amended
	// here). ErrBadCharacter (no sheet, or bytes that will not
	// reconstitute) stays a hard failure, exactly as before: there is no
	// sheet to answer ANYTHING about, attack or movement. ErrBadAttack —
	// a sheet that loads fine but names a weapon this build cannot
	// compile — becomes a declaration instead of a failed read, so the
	// rest of a UI's turn panel still renders.
	sheet, profile, err := m.compileAttack(ctx, in.Member)
	if err != nil {
		if errors.Is(err, ErrBadAttack) {
			return &AffordOutput{Clock: ClockTurn, Declarations: blockedDeclarations(Shortfall{
				Reason: ShortfallUnreadable, Text: err.Error(),
			})}, nil
		}
		return nil, fmt.Errorf("afford: %w", err)
	}

	price, err := m.priceSwing(ctx, enc, in.Member, sheet)
	if err != nil {
		return nil, fmt.Errorf("afford: %w", err)
	}

	// price.cost is never nil here: priceSwing returns a nil cost only when
	// the member is on the world clock, already ruled out above.
	slot := slotOf(price.cost.Profile)

	// Charged ONCE against the sheet THIS CALL loaded, which is handed to
	// nobody else and never saved (see the doc comment above). combat.Pay is
	// the SAME gate Attack's door pays through, so a payment that succeeds
	// or fails here answers exactly as Attack's would — and it answers the
	// SAME way for every target below: affordability is an economy
	// question, not a geometry one, so it is asked once and shared rather
	// than re-paid per candidate (which would also double-spend the sheet
	// this call loaded).
	affordable := true
	var why *Shortfall
	if payErr := combat.Pay(sheet, price.cost.Profile); payErr != nil {
		affordable = false
		sf := shortfallForPay(sheet, price.cost.Profile, slot)
		why = &sf
	}

	roster, err := enc.Members()
	if err != nil {
		return nil, fmt.Errorf("afford: %w", translate(err))
	}
	positions := rosterPositions(roster)

	holdings, err := enc.View(&encounter.ViewInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("afford: %w", translate(err))
	}

	attackDecls := attackDeclarationsFor(enc, positions, holdings, in.Member, slot, profile.Reach, affordable, why)

	// affordMove reads the SAME sheet, already readied for this turn by
	// priceSwing's own call above — never a second ready, which would
	// re-seed a bank the attack declarations just read. Safe to share: an
	// attack's profile never names CapacityMovement, so paying it above
	// cannot have moved what affordMove is about to read.
	return &AffordOutput{
		Clock:        ClockTurn,
		Declarations: append(attackDecls, affordMove(sheet)),
	}, nil
}

// blockedDeclarations answers both of a turn's verbs the same way, for a
// reason that blocks the whole turn rather than one price: downed, not your
// turn, or a sheet this build cannot compile. Neither verb's own numbers
// mean anything against a member who cannot act at all, so both report the
// SAME Shortfall rather than one going on to compute a real (and
// misleading) answer for the other.
func blockedDeclarations(why Shortfall) []Declaration {
	return []Declaration{
		{Verb: VerbAttack, Slot: SlotNone, Affordable: false, Shortfall: why.Text, Why: &why},
		{Verb: VerbMove, Slot: SlotNone, Affordable: false, Shortfall: why.Text, Why: &why},
	}
}

// attackDeclarationsFor builds one ATTACK declaration per candidate the
// member currently, live, perceives (holdings whose CurrentVia is
// non-empty — a memory of somebody no longer in sight is not somebody this
// member could swing at right now) and who stands within reach. Every
// declaration shares the SAME affordable/why the economy already decided;
// what varies per declaration is only the target and whether reach passed.
//
// NO CANDIDATE IN REACH IS STILL AN ANSWER (rpg-toolkit#1010, rpg-project#249
// §6): a single declaration with no Target, Affordable false and
// Why.Reason ShortfallNoTargetInReach — never an empty list, which a client
// could mistake for "nothing to ask about yet" rather than "nothing is
// close enough."
func attackDeclarationsFor(
	enc *encounter.Encounter, positions map[string]spatial.Position, holdings []intel.Holding,
	member string, slot Slot, reach int, affordable bool, why *Shortfall,
) []Declaration {
	from := positions[member]

	var out []Declaration
	for _, h := range holdings {
		subject := string(h.Subject)
		if subject == member || len(h.CurrentVia) == 0 {
			continue
		}
		to, ok := positions[subject]
		if !ok || !inReach(enc, from, to, reach) {
			continue
		}
		target := subject
		out = append(out, Declaration{
			Verb: VerbAttack, Slot: slot, Target: &target, Affordable: affordable, Why: why,
			Shortfall: shortfallText(why),
		})
	}

	if len(out) == 0 {
		noTarget := Shortfall{Reason: ShortfallNoTargetInReach, Text: "no target in reach"}
		return []Declaration{{
			Verb: VerbAttack, Slot: slot, Affordable: false,
			Shortfall: noTarget.Text, Why: &noTarget,
		}}
	}
	return out
}

// shortfallText reads Why.Text, or the empty string for a nil Why — the
// same value Declaration.Shortfall has always carried, kept in step with
// Why by construction rather than set separately at each call site.
func shortfallText(why *Shortfall) string {
	if why == nil {
		return ""
	}
	return why.Text
}

// affordMove reports what this member could still spend on movement this
// turn, off the sheet [Manager.Afford] already loaded and readied above.
//
// UNLIKE ATTACK, movement has no fixed price to try paying: what a specific
// walk costs is a fact about the PATH ([Manager.priceWalk]), and Afford is
// asked before any path is chosen. So this answers the question Afford CAN
// answer without one. Remaining is the actual feet left — the number a
// client bounds its own path preview against — and Affordable answers only
// whether ANY movement at all is still possible: one cell, five feet, the
// smallest unit this grid has. Shortfall, when it applies, says so in the
// same "ft" words [movementShortfall] gives a refused Move, minus a "needed"
// side this read has no specific request to name.
func affordMove(sheet *character.Character) Declaration {
	left := sheet.CapacityLeft(combat.CapacityMovement)
	decl := Declaration{Verb: VerbMove, Slot: SlotNone, Remaining: &left}
	if left >= 5 {
		decl.Affordable = true
		return decl
	}
	why := Shortfall{
		Reason: ShortfallNoBudget, Currency: CurrencyMovement,
		// Needed names the smallest step this grid has (five feet, one
		// cell) rather than a specific request's cost: unlike Attack's
		// fixed action price, a walk's cost is a fact about a PATH this
		// read is never given (Declaration.Remaining's own doc), so
		// "needed" can only say how much the least possible move would
		// take.
		Needed: 5, Left: left,
		Text: fmt.Sprintf("movement: %d ft left", left),
	}
	decl.Shortfall = why.Text
	decl.Why = &why
	return decl
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
// costOfSwing's own folding (asOnePayment) means a capacity shortfall
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
