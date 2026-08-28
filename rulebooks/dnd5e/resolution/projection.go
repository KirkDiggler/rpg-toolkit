// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// ProjectCharacterInput is the one character a projection folds, on the way in.
type ProjectCharacterInput struct {
	// Character is the sheet as the host stored it. A RECORD, not a live sheet:
	// the seam fetches records and only records, and truth is built down here.
	// Handing a live *character.Character across this boundary instead would be
	// the caller holding the thing this entry exists to keep it from holding.
	Character *character.Data
}

// ProjectCharacterOutput is what the projection derived. All of it is data.
type ProjectCharacterOutput struct {
	// ArmorClass is the folded AC, carrying every contributor's component —
	// the same breakdown an interaction folds, because it is folded the same
	// way, on a bus with the same subscribers.
	ArmorClass *combat.ACBreakdown
}

// ProjectCharacter folds one character's armour class outside any interaction:
// attach the one character, install the truth, fold, tear down.
//
// FOLDS LIVE IN RESOLUTION. A fold needs game context, game context is only
// ever installed by the one door, and the door is in here — so when a
// computation needs truth, the computation comes to resolution rather than the
// truth being carried out to it. The caller this was written for is session's
// Join, which held an attached sheet and called Character.EffectiveAC where it
// stood: a fold outside resolution, reading whatever ambient context its caller
// happened to have, which was none at all. This is the entry that lets such a
// caller obey the law without being handed anything it should not hold.
//
// THE CAST STAYS SEALED. attachAll and Participants are internal and stay
// internal. What crosses this boundary is a record on the way in and numbers on
// the way out; a caller holding a cast would be a caller able to fold without a
// door, which is the disease rather than the cure.
//
// # It reads leniently, because refusing would be the wrong kind of loud
//
// A record carrying a condition this build cannot parse — homebrew, a body
// written by a newer version, a partial write — still projects. What parsed is
// folded; what did not is dropped, and the loader warns about it by name. This
// entry only reads, so a drop here cannot delete anything: nothing on this path
// writes a sheet back.
//
// [Resolve] does the opposite with the same record and is also right, because
// it hands back sheets to be persisted — a silently dropped condition there is
// a condition deleted by whatever verb happened to run (rpg-toolkit#948). One
// attach mechanism, policy per entry: the difference between the two entries is
// DropUnreadable on [attachAllInput], which this one sets and Resolve leaves at
// its zero value. TestTheProjectionReadsWhatResolveRefuses runs one record
// through both and pins the two answers side by side.
//
// That the drop is AUDIBLE rather than merely tolerated is the whole point of
// the ruling behind this — fail loudly means observable, not refused. Getting
// the drop out somewhere a player can see it, rather than into a log, is a
// named shelf in the design.
//
// # There is no world here, and that is an answer rather than a gap
//
// A character joining a session is not standing anywhere — a join is not an
// encounter and may never become one — so the room this installs is ABSENT.
// Absence is the honest value, not a placeholder: [gamectx.Room] answers
// (nil, false) and [gamectx.RequireRoom] answers ErrNoRoom, so a reader that
// needs the world fails closed with the reason readable where it failed. That
// is the tenant-admission rule about absent values doing what the author meant,
// and it is why this does not install some empty room to keep a shape tidy — an
// empty room would answer "you are nowhere, and there is nothing here", which
// is a different and false statement. Nothing on the AC chain asks in any case:
// Unarmored Defense and Fighting Style (Defense) both take their context as `_`.
//
// This is NOT the defect rpg-toolkit#1090 named, and the difference is worth
// stating precisely because that pin is still green over this code. #1090 was
// an interaction that HAD a world and, on a branch, chose not to install it.
// The install below is as unconditional as every other; what differs is the
// value, and the value tells the truth.
func ProjectCharacter(ctx context.Context, in *ProjectCharacterInput) (*ProjectCharacterOutput, error) {
	// One bus for this fold, created here and nowhere else — the same claim
	// Resolve makes one file over, made the same way.
	return projectCharacterOn(ctx, in, newSurface(events.NewEventBus()))
}

// projectCharacterOn is [ProjectCharacter] with the surface handed in, so a
// test can hold the bus underneath and check what is left on it afterwards.
//
// Unexported for the reason resolveOn is unexported: a caller supplying its own
// bus would be a caller keeping one alive, which is the thing this package
// exists to prevent.
func projectCharacterOn(
	ctx context.Context, in *ProjectCharacterInput, surf *surface,
) (*ProjectCharacterOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}

	// The same validation every participant gets, from the same function, for
	// the same reason: a sheet with no ID cannot be read back out of the cast
	// it was just put into.
	one := Participant{Character: in.Character}
	if err := one.validate(); err != nil {
		return nil, err
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: []Participant{one},
		Roller:       refusingRoller{},
		// Asked for in writing, because the default cannot destroy anything and
		// this is the entry that opts out of it. Safe here for a reason about
		// this entry rather than about loading — see the field's own comment.
		DropUnreadable: true,
	})
	if err != nil {
		// Tear down whatever did attach before giving up, exactly as the
		// interaction path does: a half-attached bus is garbage either way, and
		// leaving revocation to an error path's silence is how leaks become
		// normal.
		_ = surf.teardown(ctx)

		return nil, err
	}

	// THE SAME DOOR, with a cast of one and no world. Not a second installer
	// and not a lighter one — there is exactly one function that may do this,
	// and this is a caller of it.
	ctx = installTruth(ctx, nil, cast)

	ch, ok := cast.Character(one.ID())
	if !ok {
		// attachAll put it there one statement ago, under this exact ID, so
		// this is unreachable rather than unlikely. It refuses anyway: the day
		// that stops being true, a nil dereference is the worst available way
		// to find out.
		return nil, errors.Join(
			fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID()),
			surf.teardown(ctx),
		)
	}

	breakdown, foldErr := ch.EffectiveAC(ctx)

	// Revoked on every exit whether or not the fold worked, because a
	// subscription that outlives its interaction is the leak this package
	// exists to prevent, and a projection is a very short interaction.
	tearErr := surf.teardown(ctx)

	if foldErr != nil {
		return nil, errors.Join(
			fmt.Errorf("resolution: project character %q: %w", one.ID(), foldErr),
			tearErr,
		)
	}
	if tearErr != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", tearErr)
	}

	return &ProjectCharacterOutput{ArmorClass: breakdown}, nil
}

// refusingRoller is the roller handed to attachAll on a path where nothing may
// roll.
//
// The same argument RefusingStriker and RefusingAnnouncer make in resolveOn. A
// projection attaches exactly one CHARACTER, and the roller exists for the
// monster branch — traits like Undead Fortitude that roll when triggered. This
// path builds its own participant and never builds a monster one, so the
// capability is unreachable by construction. A nil would express the same
// belief and cash it out as a panic the moment somebody proved it wrong;
// refusing names the violation instead, which is the difference between
// failing closed and failing over.
type refusingRoller struct{}

// Ensure refusingRoller is a complete dice.Roller — an incomplete one would not
// compile at the attachAll call above, which is the point of stating it here.
var _ dice.Roller = refusingRoller{}

// Roll refuses, naming the path that reached it.
func (refusingRoller) Roll(_ context.Context, size int) (int, error) {
	return 0, fmt.Errorf("%w: a projection reached a d%d — projections fold, they do not roll",
		ErrNoRoller, size)
}

// RollN refuses, naming the path that reached it.
func (refusingRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	return nil, fmt.Errorf("%w: a projection reached %dd%d — projections fold, they do not roll",
		ErrNoRoller, count, size)
}
