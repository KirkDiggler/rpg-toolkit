// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package turntest is a fake of what the session does for real once the
// encounter reads a creature's turn from its sheet (rpg-project#1131): a host
// that keeps each member's turn, says what is left of it in every sheet it
// answers, and spends it. FOR THIS MODULE'S TESTS ONLY (it sits under
// internal/, and no production code imports it). The tests of package
// encounter itself cannot import it (it imports encounter) and keep their own
// copy, testturns_internal_test.go; the two must answer the same.
package turntest

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Host is the fake of what a session does for real once the encounter
// reads a creature's turn from its sheet (rpg-project#1131): it keeps each
// member's turn, says what is left of it in every sheet it answers, and is
// the host that spends it.
//
// IT WRAPS A TEST'S OWN HOST. The Sheets, Striker, optional Disengager and
// Announcer a scene already has stay what they were; the turn host answers
// the same facts with [encounter.SheetFacts.Turn] filled and passes every
// call through to the host it wraps. It is itself a [encounter.Dasher], and
// an [encounter.Activator] once a scene wires it as one ([Host.WithActivator]).
//
// A member has one action and one bonus action a turn unless a test says
// otherwise ([Host.AllowPerTurn]). A Strike that returned nil or
// [encounter.ErrStrikePaused] spends an action, a Dash spends an action, a
// Disengage that returned nil spends a bonus action, and an Announce that says
// a member's turn started gives that member a full turn again. A test can
// also set what is left by hand ([Host.Leave]).
type Host struct {
	sheets     encounter.Sheets
	Striker    encounter.Striker
	disengager encounter.Disengager
	announcer  encounter.Announcer

	// failDash, when set, is what every Dash answers with.
	FailDash error

	// FreeActivations, when set, is a host that does not charge an ability: it
	// is told and never spent, so the ability stays payable for ever. For the
	// test that holds the anti-spin bound.
	FreeActivations bool
	// FailActivate, when set, is what every Activate answers with.
	FailActivate error

	perTurn   map[encounter.MemberID]encounter.TurnFacts
	left      map[encounter.MemberID]encounter.TurnFacts
	abilities map[encounter.MemberID][]*ability
	hurt      map[encounter.MemberID][2]int

	// What it was asked, in order.
	Strikes     []encounter.MemberID
	Dashes      []encounter.MemberID
	Disengages  []encounter.MemberID
	Activations []Activation
	Announced   []encounter.Boundary
}

// New wraps a scene's own host. disengager and announcer may be nil:
// a nil disengager is not asked and the host does not claim to be one.
func New(
	sheets encounter.Sheets, striker encounter.Striker, announcer encounter.Announcer,
) *Host {
	if announcer == nil {
		announcer = noAnnouncer{}
	}

	return &Host{
		sheets: sheets, Striker: striker, announcer: announcer,
		perTurn: map[encounter.MemberID]encounter.TurnFacts{},
		left:    map[encounter.MemberID]encounter.TurnFacts{},

		abilities: map[encounter.MemberID][]*ability{},
		hurt:      map[encounter.MemberID][2]int{},
	}
}

// ability is one ability the host gave a member: it costs one bonus action and
// one use.
type ability struct {
	word, name string
	uses       int
}

// Activation is one ask of [Host.Activate], in the order asked.
type Activation struct {
	Member  encounter.MemberID
	Ability string
}

// Give gives member an ability that costs one bonus action and one use, with
// that many uses to spend. word is what a row says, name what the story calls
// it.
func (h *Host) Give(member encounter.MemberID, word, name string, uses int) {
	h.abilities[member] = append(h.abilities[member], &ability{word: word, name: name, uses: uses})
}

// Uses is how many uses of the ability member has left.
func (h *Host) Uses(member encounter.MemberID, word string) int {
	for _, a := range h.abilities[member] {
		if a.word == word {
			return a.uses
		}
	}

	return 0
}

// Hurt says member's hit points now and at most; every sheet the host answers
// for that member carries them. A member it was never told about reports what
// the host it wraps does (zero and zero for every existing test).
func (h *Host) Hurt(member encounter.MemberID, now, most int) {
	h.hurt[member] = [2]int{now, most}
}

// AllowPerTurn says how many actions and bonus actions member has on a full
// turn, and gives it that now.
func (h *Host) AllowPerTurn(member encounter.MemberID, actions, bonus int) {
	full := encounter.TurnFacts{ActionsLeft: actions, BonusActionsLeft: bonus}
	h.perTurn[member] = full
	h.left[member] = full
}

// Leave sets by hand what member has left of this turn.
func (h *Host) Leave(member encounter.MemberID, actions, bonus int) {
	h.left[member] = encounter.TurnFacts{ActionsLeft: actions, BonusActionsLeft: bonus}
}

// Left is what member has left of this turn.
func (h *Host) Left(member encounter.MemberID) encounter.TurnFacts {
	if t, ok := h.left[member]; ok {
		return t
	}
	if t, ok := h.perTurn[member]; ok {
		return t
	}

	return encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 1}
}

func (h *Host) spendAction(member encounter.MemberID) {
	t := h.Left(member)
	t.ActionsLeft = max(0, t.ActionsLeft-1)
	h.left[member] = t
}

func (h *Host) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	inner, err := h.sheets.Sheets(members)
	if err != nil {
		return nil, err
	}
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(inner))
	for id, facts := range inner {
		turn := h.Left(id)
		facts.Turn = &turn
		if hp, ok := h.hurt[id]; ok {
			facts.HitPoints, facts.MaxHitPoints = hp[0], hp[1]
		}
		for _, a := range h.abilities[id] {
			facts.Abilities = append(facts.Abilities, encounter.AbilityFacts{
				Word: a.word, Name: a.name, Payable: turn.BonusActionsLeft > 0 && a.uses > 0,
			})
		}
		out[id] = facts
	}

	return out, nil
}

func (h *Host) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	h.Strikes = append(h.Strikes, attacker)
	err := h.Striker.Strike(ctx, enc, attacker, target, action)
	if err == nil || errors.Is(err, encounter.ErrStrikePaused) {
		h.spendAction(attacker)
	}

	return err
}

// Disengage is the host's [encounter.Disengager]. Wire it only in a scene that
// names a Disengager (wrap one with [Host.WithDisengager]).
func (h *Host) Disengage(ctx context.Context, enc *encounter.Encounter, member encounter.MemberID) error {
	h.Disengages = append(h.Disengages, member)
	if h.disengager != nil {
		if err := h.disengager.Disengage(ctx, enc, member); err != nil {
			return err
		}
	}
	t := h.Left(member)
	t.BonusActionsLeft = max(0, t.BonusActionsLeft-1)
	h.left[member] = t

	return nil
}

// WithDisengager has Disengage pass through to d as well.
func (h *Host) WithDisengager(d encounter.Disengager) *Host {
	h.disengager = d

	return h
}

// Dash is the host's [encounter.Dasher]: a run spends an action.
func (h *Host) Dash(_ context.Context, _ *encounter.Encounter, member encounter.MemberID) error {
	h.Dashes = append(h.Dashes, member)
	if h.FailDash != nil {
		return h.FailDash
	}
	h.spendAction(member)

	return nil
}

// Activate is the host's [encounter.Activator]: the ability costs a bonus
// action and a use, and one activation is told, as a rules host tells one. The
// smallest input RecordActivation accepts is an actor and an ability's ref and
// name, and that is all this sends: no target, no result.
func (h *Host) Activate(_ context.Context, enc *encounter.Encounter, member encounter.MemberID, word string) error {
	h.Activations = append(h.Activations, Activation{Member: member, Ability: word})
	if h.FailActivate != nil {
		return h.FailActivate
	}
	for _, a := range h.abilities[member] {
		if a.word != word {
			continue
		}
		if !h.FreeActivations {
			t := h.Left(member)
			t.BonusActionsLeft = max(0, t.BonusActionsLeft-1)
			h.left[member] = t
			a.uses--
		}
		_, err := enc.RecordActivation(&encounter.RecordActivationInput{
			Actor:   member,
			Ability: encounter.ActivationIdentity{Ref: "test:abilities:" + a.word, Name: a.name},
		})

		return err
	}

	return fmt.Errorf("turntest: %q has no ability %q", member, word)
}

func (h *Host) Announce(ctx context.Context, enc *encounter.Encounter, crossed []encounter.Boundary) error {
	h.Announced = append(h.Announced, crossed...)
	for _, b := range crossed {
		if b.Kind == encounter.TurnStarted {
			delete(h.left, b.Subject)
			if full, ok := h.perTurn[b.Subject]; ok {
				h.left[b.Subject] = full
			}
		}
	}

	return h.announcer.Announce(ctx, enc, crossed)
}

// Actors is the host wired as every actor it can be. Disengager is the host
// only when asked for, because a scene with none must not be asked.
func (h *Host) Actors(mover encounter.Mover, withDisengage bool) encounter.Actors {
	a := encounter.Actors{Striker: h, Mover: mover, Announcer: h, Dasher: h}
	if withDisengage {
		a.Disengager = h
	}

	return a
}

// WithActivator wires the host as the scene's [encounter.Activator]. [With] and
// [Wrap] do not: a scene that names none must behave as it always did.
func WithActivator(c encounter.Capabilities, h *Host) encounter.Capabilities {
	c.Activator = h

	return c
}

// With puts a scene's own Sheets, Striker and Announcer (and its
// Disengager, when it wires one) behind a turn host, and wires the host as the
// scene's Dasher. The scene is otherwise exactly what it was.
func With(c encounter.Capabilities) (encounter.Capabilities, *Host) {
	h := New(c.Sheets, c.Striker, c.Announcer)
	h.disengager = c.Disengager
	c.Sheets, c.Striker, c.Announcer, c.Dasher = h, h, h, h
	if h.disengager != nil {
		c.Disengager = h
	}

	return c, h
}

// Wrap is [With] for a scene that has no use for the host itself.
func Wrap(c encounter.Capabilities) encounter.Capabilities {
	wrapped, _ := With(c)

	return wrapped
}

// noAnnouncer hears every boundary and does nothing with it.
type noAnnouncer struct{}

func (noAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}
