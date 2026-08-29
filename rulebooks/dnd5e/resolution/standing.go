// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// StandingInput is the cast to ask about.
type StandingInput struct {
	// Participants is who to ask about, as records. The same type [Resolve]
	// takes, because it is the same question about the same people — a seam
	// that had to build a second shape here would be describing a second cast.
	//
	// A member with no record is simply not in this list. NO SHEET, NO DEATH:
	// there is nothing to read hit points off, and that is an ordinary state
	// rather than a defect. Authored content placed straight into a world has
	// no sheet until something spawns it, and answering DOWNED for those would
	// kill every authored monster the moment anybody looked at one.
	Participants []Participant
}

// StandingOutput is who is down.
type StandingOutput struct {
	// Down names the members at zero hit points or below, in cast order.
	//
	// Empty is the ordinary answer and the safe one: nobody is down. A world
	// that could not find out does not appear here at all — it comes back as an
	// error, because neither available guess is safe. Reading an unreachable
	// sheet as UP runs the fight on against sheets nobody can read; reading it
	// as DOWNED kills a character because a database blinked.
	Down []string
}

// Standing reports which of these participants are down: at zero hit points, or
// below.
//
// # Why this is an entry rather than a comparison the caller makes
//
// The whole of the arithmetic is [combat.IsDown], and it is one line today. It
// will not stay one line: a monster with Undead Fortitude reaches zero and is
// NOT down, and when that answer is built it changes that function rather than
// its callers. A seam that compared hit points itself would have to be found
// and rewritten; a seam that asks does not change at all.
//
// So this exists for the same reason the projection does. The question needs
// sheets, sheets need attaching, attaching needs a bus, and a bus outside
// resolution is a fold waiting to happen. The caller hands over records and
// takes back member IDs.
//
// # It asks to read leniently, and that reaches CHARACTERS only
//
// A character record carrying a condition this build cannot parse still answers
// whether its owner is standing. What parsed is attached; what did not is
// dropped, and the loader warns about it by name — the same audible drop the
// projection makes, for the same reason: this entry only reads, so a drop here
// cannot delete anything. Nothing on this path writes a sheet back.
//
// Refusing would be the loud that is also wrong. A homebrew condition nobody is
// asking about would abort every verb in the session — a walk consults this
// once per step — over a question about hit points the unreadable blob has no
// bearing on.
//
// A MONSTER STILL REFUSES, and this entry does not pretend otherwise.
// DropUnreadable reaches the character branch of [attachAll] and stops there:
// monstertraits has one loader and no lenient half, so a monster carrying a
// trait blob this build cannot parse fails the whole call however leniently the
// caller asked to read.
//
// That is not a policy this entry chose, and it is not a change either. The
// path it replaces refuses the same record for the same reason — monster.Load
// parses those blobs and returns the same error — so routing the question here
// leaves the monster answer exactly where it was. Making it lenient means
// giving monstertraits a lenient loader, which is a change in a different
// module, and it is named here rather than quietly assumed away.
func Standing(ctx context.Context, in *StandingInput) (*StandingOutput, error) {
	return standingOn(ctx, in, newSurface(events.NewEventBus()))
}

// standingOn is [Standing] with the surface handed in, so a test can hold the
// bus underneath. Unexported for resolveOn's reason: a caller supplying its own
// bus would be a caller keeping one alive.
func standingOn(ctx context.Context, in *StandingInput, surf *surface) (*StandingOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}

	for _, p := range in.Participants {
		if err := p.validate(); err != nil {
			return nil, err
		}
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: in.Participants,
		Roller:       refusingRoller{},
		// Asked for in writing, as the projection asks for it: this entry only
		// reads, so a dropped condition cannot delete anything.
		DropUnreadable: true,
	})
	if err != nil {
		_ = surf.teardown(ctx)

		return nil, err
	}

	// The same door, with no world. Nothing on the standing question asks for
	// one — a member's hit points do not depend on where it is standing — but
	// the door is called unconditionally here as everywhere, because "install
	// on every path" is the property that makes a missing tenant impossible
	// rather than merely unlikely.
	ctx = installTruth(ctx, nil, cast)

	down := make([]string, 0, len(cast.IDs()))
	for _, id := range cast.IDs() {
		sheet, ok := castOfParticipants(cast, id)
		if !ok {
			// attachAll put every one of these in the cast under this exact ID.
			return nil, errors.Join(
				fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, id),
				surf.teardown(ctx),
			)
		}
		if combat.IsDown(sheet) {
			down = append(down, id)
		}
	}

	if err := surf.teardown(ctx); err != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", err)
	}

	return &StandingOutput{Down: down}, nil
}

// castOfParticipants reads one member's combat-facing sheet out of the cast,
// whichever kind it is. The cast's own view answers this for effects; this is
// the same question asked from inside the package, where the view is not built.
//
// The same question, so the same answer type: [combat.Member], exactly what
// [castView.Member] hands an effect. Its one caller asks [combat.IsDown] and
// nothing else, which is a read, and a helper that handed back the keeper's
// surface would be offering a write nobody here wants.
func castOfParticipants(cast *Participants, id string) (combat.Member, bool) {
	if ch, ok := cast.Character(id); ok {
		return ch, true
	}
	if m, ok := cast.Monster(id); ok {
		return m, true
	}

	return nil, false
}
