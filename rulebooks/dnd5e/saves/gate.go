// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package saves

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

// SaveEffect is what a successful save buys.
type SaveEffect string

const (
	// Negated means a successful save prevents the consequence entirely. A gate
	// that produces a condition uses this: you are knocked prone or you are not.
	Negated SaveEffect = "negated"

	// Half means a successful save halves the damage. Only meaningful for a gate
	// on a damage pool.
	Half SaveEffect = "half"
)

// Recurrence is WHEN the save happens.
type Recurrence string

const (
	// RecurrenceNone means the save happens once, when the consequence would
	// apply. Most gates.
	RecurrenceNone Recurrence = "none"

	// RecurrenceEndOfTurn means the saver may try again at the end of each of
	// their turns to end the effect — the ghoul's paralysis.
	RecurrenceEndOfTurn Recurrence = "end_of_turn"
)

// DCKind names one member of the closed DC formula set. It is the tag the wire
// form routes on, and the answer to "which rule is this number from".
type DCKind string

const (
	// DCKindStatic is a number written in the stat block.
	DCKindStatic DCKind = "static"

	// DCKindFivePlusDamageTaken is Undead Fortitude's 5 + damage taken.
	DCKindFivePlusDamageTaken DCKind = "five_plus_damage_taken"

	// DCKindHalfDamageFloorTen is concentration's max(10, damage / 2).
	DCKindHalfDamageFloorTen DCKind = "half_damage_floor_ten"
)

// DCInput is everything a derived DC formula is allowed to read: the damage of
// the blow that triggered the save.
//
// Deliberately a struct rather than a bare int, so that a formula needing
// another fact about the trigger extends this rather than changing every
// signature — but note that adding a field is not the cheap half. The
// expensive half is [DCSource]'s extension discipline.
type DCInput struct {
	// DamageTaken is the damage the saver just took, before any halving this
	// save might buy. Zero for a gate whose DC does not derive from damage.
	DamageTaken int
}

// DCSource is where a save's difficulty class comes from: a closed enum of
// named 5e formulas, per [ADR-0039].
//
// Closed, and closed on purpose. The ADR's discipline is worth quoting:
//
//	The extension discipline: a new DCSource case must cite a RAW rule — the
//	same price extending the step vocabulary pays (an ADR). This is the line
//	that keeps the enum from becoming a formula language: 5e's designers
//	already closed this set; we inherit their closure instead of inventing an
//	open one.
//
// The unexported method is what makes that discipline structural rather than
// aspirational: a fourth case cannot be declared outside this package, so
// adding one means editing this file, and editing this file means writing the
// rule down. The rejected alternative was a function arm (FromEvent(fn)) —
// total coverage, and the exact seam where a data model starts becoming a
// language.
//
// [ADR-0039]: https://github.com/KirkDiggler/rpg-toolkit/blob/main/docs/adr/0039-the-save-gate.md
type DCSource interface {
	// DC computes the difficulty class for one instance of this gate.
	DC(in DCInput) int

	// Kind names which formula this is — for the wire form, and for a UI that
	// wants to say "DC 11" rather than "some number".
	Kind() DCKind

	// isDCSource seals the set. See the type's godoc.
	isDCSource()
}

// DCStatic is a difficulty class written in the stat block: the wolf's 11.
func DCStatic(n int) DCSource {
	return staticDC{n: n}
}

// DCFivePlusDamageTaken is Undead Fortitude's DC: 5 + the damage taken.
//
// A function rather than a package-level variable, so nothing can reassign the
// meaning of a RAW rule at runtime — and so all three constructors read alike
// at a call site.
func DCFivePlusDamageTaken() DCSource {
	return fivePlusDamageTakenDC{}
}

// DCHalfDamageFloorTen is the concentration DC: max(10, damage taken / 2),
// rounded down. 21 damage is DC 10, not DC 10.5; 25 is DC 12.
func DCHalfDamageFloorTen() DCSource {
	return halfDamageFloorTenDC{}
}

type staticDC struct {
	n int
}

func (d staticDC) DC(_ DCInput) int { return d.n }
func (staticDC) Kind() DCKind       { return DCKindStatic }
func (staticDC) isDCSource()        {}

type fivePlusDamageTakenDC struct{}

func (fivePlusDamageTakenDC) DC(in DCInput) int {
	return 5 + nonNegative(in.DamageTaken)
}
func (fivePlusDamageTakenDC) Kind() DCKind { return DCKindFivePlusDamageTaken }
func (fivePlusDamageTakenDC) isDCSource()  {}

type halfDamageFloorTenDC struct{}

func (halfDamageFloorTenDC) DC(in DCInput) int {
	half := nonNegative(in.DamageTaken) / 2 // damage is never negative, so this floors
	if half < 10 {
		return 10
	}

	return half
}
func (halfDamageFloorTenDC) Kind() DCKind { return DCKindHalfDamageFloorTen }
func (halfDamageFloorTenDC) isDCSource()  {}

// nonNegative clamps damage at zero. Negative damage is not a thing 5e has, and
// a formula that quietly produced a DC below its floor because someone passed
// -4 would be a worse bug than the clamp.
func nonNegative(damage int) int {
	if damage < 0 {
		return 0
	}

	return damage
}

// SaveGate is a declaration that a consequence can be contested with a saving
// throw. It is data — content carries it, nothing here executes it.
//
// The split of responsibility is [ADR-0039]'s: whatever wants to impose a
// consequence declares the gate, and resolution turns the declaration into a
// Request whose result decides whether the consequence lands. That separation
// is what makes "can this be resisted, and how?" answerable from a stat block,
// before anything runs — the founding complaint of rpg-toolkit#962 was that a
// stat block could carry a knockdown DC and be lying about whether anything
// read it.
//
// Because every save resolves through SavingThrowChain, a gate never needs to
// know that Raging or Bless exist: modifiers arrive by construction.
//
// [ADR-0039]: https://github.com/KirkDiggler/rpg-toolkit/blob/main/docs/adr/0039-the-save-gate.md
type SaveGate struct {
	// Abilities the saver may use. One entry for most gates; the wolf's
	// knockdown is STR where the monk's Flurry is DEX — same gate shape,
	// different ability, which is why the ability lives in the declaration and
	// not in a "knockdown" concept.
	Abilities []abilities.Ability

	// DC is where the number comes from.
	DC DCSource

	// OnSuccess is what a successful save buys.
	OnSuccess SaveEffect

	// Recurrence is when the save happens. Deliberately a separate axis from
	// OnSuccess: conflating "what success does" with "when you may try" was the
	// most likely modelling mistake here.
	Recurrence Recurrence
}

// NewSaveGate builds the common gate — one ability, a static DC, negated on a
// success, no recurrence — which is every knockdown in the roster.
//
// The struct literal is still there for the other shapes; this exists so that
// the common case does not have to spell out two fields whose value is "the
// usual one".
func NewSaveGate(ability abilities.Ability, dc int) *SaveGate {
	return &SaveGate{
		Abilities:  []abilities.Ability{ability},
		DC:         DCStatic(dc),
		OnSuccess:  Negated,
		Recurrence: RecurrenceNone,
	}
}

// Validate reports whether this gate describes a save anyone could actually
// make.
//
// A gate with no abilities is a save nobody can roll, and a static DC of zero
// or less is a save nobody can fail — both are almost certainly a field left
// empty rather than a rule, so they are refused here instead of resolving into
// a silent auto-success.
func (g *SaveGate) Validate() error {
	if g == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "save gate is nil")
	}
	if len(g.Abilities) == 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "save gate names no ability to save with")
	}
	if g.DC == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "save gate has no DC source")
	}
	if static, ok := g.DC.(staticDC); ok && static.n <= 0 {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, "static save DC must be positive, got %d", static.n)
	}
	if g.OnSuccess != Negated && g.OnSuccess != Half {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown save effect %q", g.OnSuccess)
	}
	if g.Recurrence != RecurrenceNone && g.Recurrence != RecurrenceEndOfTurn {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown recurrence %q", g.Recurrence)
	}

	return nil
}

// saveGateData is the wire form. Unexported because nothing outside this
// package should build one: a caller holds a [SaveGate] and lets the marshaller
// decide what the bytes look like, which is what lets the bytes change without
// every content package changing with them.
type saveGateData struct {
	Abilities  []abilities.Ability `json:"abilities"`
	DC         dcSourceData        `json:"dc"`
	OnSuccess  SaveEffect          `json:"on_success"`
	Recurrence Recurrence          `json:"recurrence"`
}

// dcSourceData is a DC source as bytes: the kind it is, plus the number the
// static kind carries. Kind-tagged rather than a bare int, because the whole
// point of the enum is that a reader can tell which rule produced the number.
type dcSourceData struct {
	Kind DCKind `json:"kind"`
	N    int    `json:"n,omitempty"`
}

// MarshalJSON implements json.Marshaler.
//
// Always writes OnSuccess and Recurrence explicitly, even when they hold the
// common value, so a stored gate says what it means rather than relying on the
// reader to know the defaults.
func (g SaveGate) MarshalJSON() ([]byte, error) {
	gate := g
	gate.normalize()

	if err := gate.Validate(); err != nil {
		return nil, rpgerr.Wrap(err, "refusing to serialize an invalid save gate")
	}

	dc := dcSourceData{Kind: gate.DC.Kind()}
	if static, ok := gate.DC.(staticDC); ok {
		dc.N = static.n
	}

	return json.Marshal(saveGateData{
		Abilities:  gate.Abilities,
		DC:         dc,
		OnSuccess:  gate.OnSuccess,
		Recurrence: gate.Recurrence,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
//
// Strict about what it cannot make sense of and forgiving only about what it
// can: an omitted OnSuccess or Recurrence means the common value, while an
// unknown DC kind, save effect, or recurrence fails the load naming the value.
// A blob that named a fourth DC formula came from a build that knew a rule this
// one does not, and quietly dropping the gate would leave a consequence nobody
// could contest — with nothing anywhere saying so.
func (g *SaveGate) UnmarshalJSON(raw []byte) error {
	var data saveGateData
	if err := json.Unmarshal(raw, &data); err != nil {
		return rpgerr.Wrapf(err, "failed to unmarshal save gate: %s", raw)
	}

	dc, err := dcSourceFromData(data.DC)
	if err != nil {
		return err
	}

	g.Abilities = data.Abilities
	g.DC = dc
	g.OnSuccess = data.OnSuccess
	g.Recurrence = data.Recurrence
	g.normalize()

	return g.Validate()
}

// normalize fills the fields an author may leave out with the value they would
// almost always have written.
func (g *SaveGate) normalize() {
	if g.OnSuccess == "" {
		g.OnSuccess = Negated
	}
	if g.Recurrence == "" {
		g.Recurrence = RecurrenceNone
	}
}

// dcSourceFromData routes a wire kind back to its formula.
func dcSourceFromData(data dcSourceData) (DCSource, error) {
	switch data.Kind {
	case DCKindStatic:
		return DCStatic(data.N), nil
	case DCKindFivePlusDamageTaken:
		return DCFivePlusDamageTaken(), nil
	case DCKindHalfDamageFloorTen:
		return DCHalfDamageFloorTen(), nil
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unknown save DC kind %q: a new formula must cite a RAW rule (ADR-0039)", data.Kind)
	}
}

// String renders the gate the way a stat block would say it, so a log line or a
// UI tooltip has something to show without reaching into the fields.
func (g *SaveGate) String() string {
	if g == nil {
		return "no save"
	}

	dc := "DC ?"
	if g.DC != nil {
		if static, ok := g.DC.(staticDC); ok {
			dc = fmt.Sprintf("DC %d", static.n)
		} else {
			dc = string(g.DC.Kind())
		}
	}

	return fmt.Sprintf("%s %s save, %s on success, recurrence %s",
		dc, abilityList(g.Abilities), g.OnSuccess, g.Recurrence)
}

// abilityList joins the abilities a saver may choose between.
func abilityList(list []abilities.Ability) string {
	if len(list) == 0 {
		return "(no ability)"
	}

	out := string(list[0])
	for _, ability := range list[1:] {
		out += "/" + string(ability)
	}

	return out
}
