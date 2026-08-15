// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// InitiativeRoller turns the members entering a fight into the order they act
// in. The composition asks; the rulebook answers.
//
// Injected rather than held, exactly as [Decider] is. The composition owns no
// randomness (R7) — Form's own doc says the order "comes from OUTSIDE" — but
// trigger detection has to start a fight at the moment it detects one, which
// means asking for the order rather than being handed it. Holding the SEAM is
// not holding the randomness, and this is the module's existing pattern for
// exactly that distinction.
//
// Errors abort whatever verb was running, atomically (R5): a fight that cannot
// be ordered does not half-start.
type InitiativeRoller interface {
	// RollInitiative returns the given members in the order they act, first
	// to act first. The result must contain exactly the members it was given.
	RollInitiative(members []MemberID) ([]MemberID, error)
}

// contactKind is what one player-monster pair's awareness amounts to after a
// sight refresh.
//
// The four cases are the whole rule. 5e's surprise is an asymmetry — you are
// surprised if you did not notice your enemy — so a trigger built on who saw
// whom gets surprise as a consequence rather than as a later bolt-on.
type contactKind int

const (
	// contactNone: neither noticed the other this step.
	contactNone contactKind = iota

	// contactDrop: the player saw the monster and the monster did not see
	// back. The designed behavior is that the walk ends early and the player
	// decides in free roam — back away, sneak, or approach — with attacking
	// from here counting as initiation.
	//
	// UNREACHABLE TODAY, and deliberately kept. Sight is pure symmetric
	// geometry: refreshSight builds every percept from IsLineOfSightBlocked
	// with no facing, stealth, or perception input, so if the player sees the
	// wolf the wolf sees the player and this case has no producible input.
	// rpg-toolkit#1020 is the prerequisite that makes it real. The arm stays
	// because it is the documented design rather than dead ceremony — what is
	// NOT here is any behavior hanging off it, because unbuilt beats
	// built-and-dead.
	contactDrop

	// contactSpotted: the monster saw the player and the player did not see
	// back. The bubble forms; nobody had a choice to make.
	contactSpotted

	// contactMutual: both saw each other. The bubble forms.
	contactMutual
)

// trigger is what one classification pass concluded.
type trigger struct {
	// form names the members who should enter a bubble, sorted. Empty when
	// nothing forms.
	form []MemberID

	// surprised names the members who enter that bubble unaware, sorted.
	surprised []MemberID

	// joined names members pulled into a bubble that was ALREADY running —
	// the straggler case, sorted. Mutually exclusive with form: a fight
	// either starts or is joined.
	joined []MemberID
}

// classify reads one sight refresh and decides what it triggered.
//
// # v1, and how it evolves
//
// THE INVARIANT: upgrades change how percepts are PRODUCED, never how they are
// consumed. Everything below this comment — the four-case table, the
// precedence law, the transition-only induction, awareness-based surprise — is
// written against asymmetric sight and does not move again. That is what makes
// the next two versions cheap.
//
// v1, WHICH IS WHAT THIS IS: sight is symmetric line-of-sight, so any first
// contact forms the bubble and the drop case has no producible input. Surprise
// is computed correctly and is always empty, for the same reason — everyone
// sees everyone, so nobody is unaware. Honest option C under B-prime's design,
// not a compromise: the machinery is the permanent part and the percepts are
// the part that grows.
//
// v2 is asymmetric perception (rpg-toolkit#1020), where Stealth against
// passive Perception decides whether an observer's percept exists at all. The
// drop, real surprise, and initiating from unseen all come alive there with no
// change to this function. v3 is richer senses — blindsight and tremorsense
// defeating stealth — through the same production seam.
//
// The full evolution record, including the design questions reserved for v2's
// own moment, is issue #964's ratification comment.
//
// # Transitions, not states
//
// Only intel.SurveilOutput.FirstContact is read — never Refreshed. A subject
// already being watched is not news, so a player who spotted a wolf and keeps
// walking with it in view does not re-trigger on every step: the sneak-past
// works by construction rather than by a "have I already reported this" flag.
//
// Reading transitions is COMPLETE rather than merely cheap, and the induction
// is worth writing down because it is what makes the shortcut safe. This runs
// at every refreshSight from first light onward — setup, move, traverse, pump,
// join. Every awareness that exists was created by some step, and that step
// classified it. So a stale asymmetry — a monster that has "already been
// watching" without anything forming — cannot arise: the step that created the
// watching was itself classified, and either formed a bubble or was a drop.
//
// # Precedence
//
// Within one pass, any spotted or mutual pair forms the bubble, and the drop's
// walk-stop applies only when no pair does. A player who gets the drop on one
// wolf while a second wolf spots them is in a fight, not in a choice: the
// bubble is the strictly stronger outcome and the drop's freedom is gone.
//
// # Surprise is read from now, not from the trigger
//
// Classification uses the transition lists; surprise does NOT. A member is
// surprised iff its CURRENT view holds no opposing member at the moment the
// bubble forms.
//
// The walked-behind case is why. A player gets the drop on a wolf (no bubble,
// free roam), then circles behind it while keeping it in sight — the wolf is
// Refreshed on every step and never FirstContact again. Later the wolf turns
// and first-contacts the player: spotted, bubble forms, correct. But the
// PLAYER has been watching the whole time and is plainly not surprised, while
// FirstContact-only surprise would say they were, because the player's own
// first contact happened long ago and is not in this delta.
func (e *Encounter) classify(deltas map[MemberID]*intel.SurveilOutput) (*trigger, error) {
	sawFirst := func(observer, subject MemberID) bool {
		delta, ok := deltas[observer]
		if !ok || delta == nil {
			return false
		}
		for _, report := range delta.FirstContact {
			if MemberID(report.Subject) == subject {
				return true
			}
		}

		return false
	}

	players, monsters := e.sidesInContactOrder()

	var (
		engaged = map[MemberID]bool{}
		drops   []MemberID
		dropBy  MemberID
	)

	for _, player := range players {
		for _, monster := range monsters {
			switch contactBetween(sawFirst(player, monster), sawFirst(monster, player)) {
			case contactMutual, contactSpotted:
				// mutual or spotted — the bubble forms either way, and which
				// of the two it was only matters to surprise, which is read
				// from current awareness rather than from here.
				//
				// BOTH sides of the pair are engaged, and only they. A fight
				// is localized: the members who noticed each other are in it
				// and everyone else keeps free-roaming, which is the whole
				// point of a bubble being a bubble rather than a mode. Pulling
				// the entire roster in would delete the split party the
				// composition exists to support.
				engaged[player] = true
				engaged[monster] = true
			case contactDrop:
				drops = append(drops, monster)
				if dropBy == "" {
					dropBy = player
				}
			case contactNone:
			}
		}
	}

	if len(engaged) > 0 {
		return e.formation(engaged)
	}

	// The drop arm classifies and then does nothing, because nothing can reach
	// it (rpg-toolkit#1020) and a walk-stop built against an unreachable case
	// would be untestable behavior shipped on faith. When #1020 lands, this is
	// where the early stop attaches.
	_, _ = drops, dropBy

	return &trigger{}, nil
}

// formation names the members entering the bubble and who enters it unaware.
//
// Only the engaged: a fight is localized, so the members who noticed each
// other leave the world clock and everybody else keeps exploring. Stragglers
// join later through the same classification, which is what makes a fight
// grow rather than start twice.
func (e *Encounter) formation(engaged map[MemberID]bool) (*trigger, error) {
	form := make([]MemberID, 0, len(engaged))
	for id := range engaged {
		form = append(form, id)
	}
	sort.Slice(form, func(i, j int) bool { return form[i] < form[j] })

	// A fight already running is JOINED, not started again. This is 4.2's
	// straggler mechanic arriving automatically: a second wolf that rounds the
	// corner into a live fight belongs in it, and the alternative — refusing,
	// or forming a second bubble — is either a rule that stops working the
	// moment a fight exists or two fights nobody asked for.
	//
	// Everyone in contact who is not already on a turn clock transfers in at
	// the END of the order: they arrive after the creatures who were already
	// trading blows, which is the only ordering that does not silently give a
	// latecomer a turn ahead of somebody mid-fight.
	if len(e.bubbles) > 0 {
		var joined []MemberID
		for _, id := range form {
			bubble, err := e.bubbleFor(id)
			if err != nil {
				return nil, err
			}
			if bubble == nil {
				joined = append(joined, id)
			}
		}

		return &trigger{joined: joined}, nil
	}

	surprised := make([]MemberID, 0, len(form))
	for _, id := range form {
		unaware, err := e.unawareOfOpposition(id)
		if err != nil {
			return nil, err
		}
		if unaware {
			surprised = append(surprised, id)
		}
	}

	return &trigger{form: form, surprised: surprised}, nil
}

// unawareOfOpposition reports whether this member's CURRENT view holds nobody
// from the other side — which is exactly 5e's surprise condition, read at
// formation rather than inferred from whatever triggered it.
func (e *Encounter) unawareOfOpposition(id MemberID) (bool, error) {
	member, ok := e.members[id]
	if !ok {
		return false, nil
	}

	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: id})
	if err != nil {
		return false, err
	}

	for _, holding := range holdings {
		// Held, not Current, is a ghost — a subject remembered from before
		// rather than one being watched now. Remembering where a wolf used to
		// be does not stop it surprising you.
		if holding.Status != intel.Current {
			continue
		}
		other, ok := e.members[MemberID(holding.Subject)]
		if !ok {
			continue
		}
		if other.Kind != member.Kind {
			return false, nil
		}
	}

	return true, nil
}

// sidesInContactOrder returns the players and the monsters, each sorted.
//
// Sorted because e.members is a map and C8 requires that what a pass concludes
// be a function of persisted data rather than of iteration order — the same
// reason buildMemberOutcomes sorts.
func (e *Encounter) sidesInContactOrder() (players, monsters []MemberID) {
	for id, member := range e.members {
		switch member.Kind {
		case KindPlayer:
			players = append(players, id)
		case KindMonster:
			monsters = append(monsters, id)
		}
	}
	sort.Slice(players, func(i, j int) bool { return players[i] < players[j] })
	sort.Slice(monsters, func(i, j int) bool { return monsters[i] < monsters[j] })

	return players, monsters
}

// applyTrigger turns a classification into what actually happened: a fight
// starts, or nothing does.
//
// Called by every verb that refreshes sight, right after it does. Placement is
// the point: sight changes in more places than a player's walk, and the one
// that breaks a walk-only seam is Pump — a monster walking into view is first
// contact with nobody walking, and a trigger living in the walker's loop would
// never see it. The old stack learned this the expensive way and ended up
// calling its check from two call sites (encounter/combat.go's
// checkCombatEntry, reached from Move AND AddMonster); this sits at the one
// place all of them already pass through.
func (e *Encounter) applyTrigger(deltas map[MemberID]*intel.SurveilOutput) (*FormedBubble, error) {
	verdict, err := e.classify(deltas)
	if err != nil {
		return nil, err
	}

	if len(verdict.joined) > 0 {
		for _, id := range verdict.joined {
			if _, err := e.Transfer(&TransferInput{
				Member: id,
				To:     ClockTurn,
				Pos:    e.bubbleSize(),
			}); err != nil {
				return nil, fmt.Errorf("trigger transfer %q: %w", id, err)
			}
		}

		return nil, nil
	}

	if len(verdict.form) == 0 {
		return nil, nil
	}

	order, err := e.initiative.RollInitiative(append([]MemberID(nil), verdict.form...))
	if err != nil {
		return nil, fmt.Errorf("trigger roll initiative: %w", err)
	}

	formed, err := e.form(&FormInput{Order: order, Surprised: verdict.surprised})
	if err != nil {
		return nil, fmt.Errorf("trigger form: %w", err)
	}

	return &FormedBubble{
		Order:     order,
		Surprised: verdict.surprised,
		Seq:       formed.Seq,
	}, nil
}

// bubbleSize reports how many members the running bubble holds, which is the
// slot a straggler lands in: the end of the order.
func (e *Encounter) bubbleSize() int {
	if len(e.bubbles) == 0 {
		return 0
	}

	return len(e.bubbles[0].ToData().Order)
}

// contactBetween names what one pair's two first-contact answers amount to.
//
// Written as its own function so the four cases are stated once, in the order
// the rule reads, rather than inferred from the shape of an if.
func contactBetween(playerSaw, monsterSaw bool) contactKind {
	switch {
	case playerSaw && monsterSaw:
		return contactMutual
	case monsterSaw:
		return contactSpotted
	case playerSaw:
		return contactDrop
	default:
		return contactNone
	}
}
