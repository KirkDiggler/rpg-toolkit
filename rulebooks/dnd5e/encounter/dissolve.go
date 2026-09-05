// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

// DissolveKind names why a fight ended, in the form the story carries it.
type DissolveKind string

const (
	// DissolveByDecision is a fight its members chose to leave.
	DissolveByDecision DissolveKind = "decision"

	// DissolveByDefeat is a fight that ran out of a side.
	DissolveByDefeat DissolveKind = "defeat"

	// DissolveByStance is a fight whose sides stopped being sides: the
	// stance between their factions turned (rpg-project#375, R1).
	DissolveByStance DissolveKind = "stance"
)

// DissolveCause is why a fight ended: a closed set, sealed the way
// [saves.DCSource] is and for the same reason.
//
// A fight can end for reasons of two different kinds, and the difference is the
// whole design. A party DECIDING to disengage is a choice somebody makes, so it
// reaches this module as a verb — [Encounter.Dissolve], and the verb IS the
// decision. Being DEFEATED is a fact the world NOTICES, so it arrives the way
// sight already starts fights: automatically, at a choke point, with no caller
// involved (rpg-toolkit#964's mirror).
//
// Those are different events, which is why they are not two mechanisms. Defeat
// is another CALLER of this shape rather than a second way to end a fight, and
// its case was earned by that caller — ruled fork (c) on rpg-toolkit#959 —
// rather than declared in advance against a future nobody had built.
//
// The unexported method is what makes the discipline structural rather than
// aspirational: a third case cannot be declared outside this package, so adding
// one means editing this file, and editing this file means having the caller in
// hand. A bare string would have let anything write a cause into the story that
// nothing ruled on.
//
// The cause travels in the story rather than in a return value, because a
// self-dissolve has no return value to travel in: nothing asked for it. See
// [Encounter.Dissolve] for the beat both endings share.
//
// The session package keeps its own [DissolveCause] for the wire, which this one
// translates into. Two sealed sets rather than one shared type is deliberate and
// unavoidable — this module's go.mod cannot import a package that imports it —
// and each is extended by the caller that forces it, at the layer that caller
// lives in.
//
// [saves.DCSource]: https://pkg.go.dev/github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves#DCSource
type DissolveCause interface {
	// Kind names which cause this is.
	Kind() DissolveKind

	// isDissolveCause seals the set. See the type's godoc.
	isDissolveCause()
}

// ByDecision is a fight its members chose to walk away from: the party breaks
// off to watch the goblin rather than trade blows with it.
//
// The only cause a CALLER can produce, because it is the only one that is a
// decision. It is a function rather than a package-level variable so nothing can
// reassign what it means at runtime, matching how the save gate's sources read
// at a call site.
func ByDecision() DissolveCause { return byDecision{} }

type byDecision struct{}

func (byDecision) Kind() DissolveKind { return DissolveByDecision }
func (byDecision) isDissolveCause()   {}

// ByDefeat is a fight whose supplied participation leaves no Contact member on
// one side and does not keep its turn order. The fight ends with nobody calling
// Dissolve.
//
// Ruled fork (c) on rpg-toolkit#959, including the distinction the old stack
// conflated. Its `all_hostiles_defeated` closed the whole ENCOUNTER, which in a
// multi-room dungeon means clearing room one ends the dungeon. What ends here is
// the BUBBLE. The encounter stays open, the bodies stay on the map and in the
// roster (ruled fork (a)), and the survivors go back to exploring.
//
// Unreachable from outside this package, and that is the point: defeat is
// something the composition NOTICES at [Encounter.noticeDown], never something a
// caller declares. A caller who could hand this in would be a second system
// deciding a fact the supplied Participation capability already owns.
func ByDefeat() DissolveCause { return byDefeat{} }

type byDefeat struct{}

func (byDefeat) Kind() DissolveKind { return DissolveByDefeat }
func (byDefeat) isDissolveCause()   {}

// ByStance is a fight whose two sides stopped being sides: the stance between
// their factions turned — the camp's chief came to know the party saved the
// Wiseman — and a fight between members who are opposed to nobody is not a
// fight (rpg-project#375, design §3.5, R1). The third case, earned by its
// caller the way ByDefeat was.
//
// Unreachable from outside this package, for ByDefeat's reason: a stance
// turning is something the world FOLDS at [Encounter.settleStances], never
// something a caller declares — a caller who could hand this in would be a
// second system deciding a fact the graph owns.
func ByStance() DissolveCause { return byStance{} }

type byStance struct{}

func (byStance) Kind() DissolveKind { return DissolveByStance }
func (byStance) isDissolveCause()   {}

// DissolveInput names a member of the fight being ended. The bubble is
// reached through a member — never addressed by name — which R6 makes a
// total function.
type DissolveInput struct {
	// Member is anyone in the fight to dissolve.
	Member MemberID
}

// DissolveOutput reports who the fight held and where the story recorded
// its end.
type DissolveOutput struct {
	// Members is everyone the fight held, in bubble order, all re-homed to
	// the world clock at budget zero.
	Members []MemberID

	// Cause is why it ended. Always [ByDecision] here — the verb is the
	// decision — and the field exists so a caller reads the cause off the
	// answer rather than off the fact that it called.
	Cause DissolveCause

	// Seq is the story sequence of the dissolution beat.
	Seq uint64
}

// Dissolve ends the fight holding the named member: the bubble empties and
// every member re-homes to the world clock, at budget zero — time spent
// fighting was spent, not banked.
//
// This is the DECISION half of ending a fight, and since rpg-toolkit#1078 it is
// only half. The other half needs no verb: a fight with nobody left standing on
// one side ends itself in the pass that notices, with cause [ByDefeat] and no
// caller. So this verb can now be beaten to it — a Dissolve on a fight the world
// already ended returns ErrNoBubble, which is the honest answer to asking for
// something that has happened.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember, ErrNotMember, ErrNoBubble
// (the member is not in a fight). A failure while re-homing leaves members
// between clocks — the window ClockOf's on-no-clock check nets — and the
// caller's obligation is doc.go's: drop the encounter unsaved.
func (e *Encounter) Dissolve(in *DissolveInput) (*DissolveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("dissolve: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("dissolve: %w", ErrClosed)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("dissolve: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("dissolve %q: %w", in.Member, ErrNotMember)
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("dissolve %q: %w", in.Member, err)
	}
	if bubble == nil {
		return nil, fmt.Errorf("dissolve %q: %w", in.Member, ErrNoBubble)
	}

	return e.dissolveBubble(bubble, ByDecision())
}

// dissolveBubble is the ending itself, with the cause held apart from it.
//
// Both endings run exactly this: the same re-homing, the same prune, the same
// beat. Only the cause differs, which is what makes "a fight ended" one event
// with two accounts of it rather than two events a client has to reconcile.
//
// The beat carries the cause because it is the ONLY channel a self-dissolve
// has. Nothing asked for that ending, so there is nothing to return it to — the
// story is where a client, and D4's event stream, find out a fight is over and
// why.
func (e *Encounter) dissolveBubble(bubble *clock.Turn, cause DissolveCause) (*DissolveOutput, error) {
	out, err := bubble.Dissolve(&clock.DissolveInput{})
	if err != nil {
		return nil, fmt.Errorf("dissolve: %w", err)
	}
	for _, id := range out.Members {
		if _, jerr := e.clock.Join(&clock.JoinInput{ID: id}); jerr != nil {
			return nil, fmt.Errorf("dissolve member %q world clock: %w", id, jerr)
		}
	}
	if derr := e.dropBubbleIfIdle(bubble); derr != nil {
		return nil, fmt.Errorf("dissolve prune bubble: %w", derr)
	}

	seq, err := e.appendClockBeat(map[string]interface{}{
		"beat":    "bubble-dissolved",
		"members": out.Members,
		"cause":   string(cause.Kind()),
	}, out.Members...)
	if err != nil {
		return nil, fmt.Errorf("dissolve append beat: %w", err)
	}

	// AFTER the beat, and after the re-homing above: the story says the fight
	// ended before anything is told the fight ended, and the world an announcer
	// looks at is a settled one — everyone back on the world clock, the bubble
	// pruned. The same order driveOneMonsterTurn uses for a turn boundary.
	//
	// HERE rather than at the two callers, because this is the one place a
	// fight ends. An explicit [Encounter.Dissolve] and the composition noticing
	// defeat both arrive at this function, and announcing out there would be
	// two chances to forget — one of them on a path no host controls.
	crossed, cerr := combatEndBoundaries(out.Milestones, out.Members)
	if cerr != nil {
		return nil, fmt.Errorf("dissolve: %w", cerr)
	}
	if aerr := e.announceBoundaries(crossed); aerr != nil {
		return nil, fmt.Errorf("dissolve announce: %w", aerr)
	}

	return &DissolveOutput{Members: out.Members, Cause: cause, Seq: seq}, nil
}
