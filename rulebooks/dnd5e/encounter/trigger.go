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
	// REACHABLE AS OF rpg-toolkit#1111, and it was not before — this comment
	// said UNREACHABLE for three waves and had earned it.
	//
	// A percept is built from TWO filters, and only one of them is mutual.
	// The geometric one is: IsLineOfSightBlocked takes no facing, stealth or
	// perception input, and spatial pins it as mutual over fuzzed rooms in
	// every grid family (rpg-toolkit#1022). It still is, and this slice did
	// not touch it. The other is REACH, and it is per observer as of
	// rpg-toolkit#1111 — see [Sight].
	//
	// While reach was the same for everybody, both filters were mutual, so
	// "the player sees the wolf" and "the wolf sees the player" were one
	// sentence and this arm had no producible input. They are two sentences
	// now: the player with the lantern sees the wolf that cannot see her back,
	// through geometry that is as mutual as it ever was.
	//
	// It was briefly reachable BY ACCIDENT before that, and nobody meant it to
	// be, which is why the geometric law exists: sight used to be one
	// rasterized line, and Bresenham chose different cells by direction on
	// square grids, so a wall corner could hide a player from a monster it did
	// not hide the monster from. A one-sided ambush earned by ray-rounding
	// rather than by anything in the fiction. Fixed in spatial v0.9.1, and
	// asymmetry now arrives only where it is designed to — which is what
	// rpg-toolkit#1111 is, and rpg-toolkit#1020 after it.
	//
	// WHAT IS STILL NOT HERE is any behaviour hanging off it. The
	// classification is correct and nothing acts on it: no bubble forms, which
	// is the right answer by itself — a creature that did not notice you starts
	// no fight. The walk-stop and the choice it offers stay unbuilt, because
	// the SHAPE of that offer is rpg-toolkit#1020's design and guessing it here
	// would be the built-and-wrong this comment has always preferred to avoid.
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
// v1 WAS symmetric line-of-sight, so any first contact formed the bubble, the
// drop case had no producible input, and surprise — computed correctly the
// whole time — was always empty, because everyone saw everyone and nobody could
// be unaware. Honest option C under B-prime's design, not a compromise: the
// machinery is the permanent part and the percepts are the part that grows.
//
// v1.5 IS WHAT THIS IS, and it is the invariant collecting. rpg-toolkit#1111
// gave sight a distance term supplied PER MEMBER, so two observers can answer
// differently: a monster with darkvision sees a player whose torch does not
// reach it. Spotted and drop have inputs, and 5e surprise is producible and
// real. NOT ONE LINE BELOW MOVED — the four-case table, the precedence law,
// transition-only induction and awareness-based surprise were all written
// against asymmetric sight, and they simply started firing. Pinned by
// TestTheFarSightedSpotTheShortSightedAndSurpriseThem.
//
// THAT CLAIM WAS FALSE WHEN IT WAS FIRST WRITTEN, and the correction is worth
// keeping rather than quietly editing away. Sight was symmetric by intent and
// not in fact: it was decided by one rasterized line, and the line differed by
// direction on square grids, so a wall corner produced real one-sided contact
// — Surprised populated, with no stealth anywhere in the module. Found by the
// session wave probing this very seam, filed as rpg-toolkit#1022, and fixed in
// spatial v0.9.1, which now pins symmetry as a LAW over fuzzed rooms in every
// grid family rather than leaving it as a property of an algorithm.
//
// The consumer needed no change, which is the part that mattered: Surprised
// populated, crossed the boundary and reported correctly the whole time. The
// invariant above held — what was wrong was the PRODUCTION of percepts, and
// fixing it there is exactly where the design said it would happen.
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
// Only IntelDelta.FirstContact is read — never Refreshed. A subject
// already being watched is not news, so a player who spotted a wolf and keeps
// walking with it in view does not re-trigger on every step: the sneak-past
// works by construction rather than by a "have I already reported this" flag.
//
// Reading transitions is COMPLETE rather than merely cheap, and the induction
// is worth writing down because it is what makes the shortcut safe. This runs
// at every refreshSight from first light onward — setup, step, pump,
// join. Every awareness that exists was created by some step, and that step
// classified it. So a stale asymmetry — a monster that has "already been
// watching" without anything forming — cannot arise: the step that created the
// watching was itself classified, and either formed a bubble or was a drop.
//
// STANDING IS THE ONE HOLE IN THAT INDUCTION, and it is worth stating rather
// than discovering. A down member is on no side (see sidesInContactOrder), so
// awareness created while they were down was classified as nothing — and if
// they were later reported standing again, that awareness is already Refreshed
// rather than FirstContact and will never trigger. A revived monster could
// stand in plain sight of a player and start no fight until somebody moves.
//
// rpg-toolkit#1078 gives that hole a second face rather than a second cause: a
// fight now ENDS when a side stops standing, so the monsters a party just
// dropped are the ones whose awareness has already gone stale. Stand one of them
// back up and the fight does not restart until somebody moves. Same hole, same
// fix, one step more visible.
//
// Left open on purpose. Ruled fork (b) on rpg-toolkit#959 is that zero hit
// points has no exit in v1 — death saves are deferred — so recovery is not a
// path anything can currently take, and building against it would be untestable
// behaviour shipped on faith. It is the same call contactDrop above documents
// for the same reason. When recovery becomes real, the fix belongs where the
// invariant says it does: in how percepts are PRODUCED, not in how they are
// consumed here.
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
func (e *Encounter) classify(deltas map[MemberID]*IntelDelta, down map[MemberID]bool) (*trigger, error) {
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

	players, monsters := e.sidesInContactOrder(down)

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
		return e.formation(engaged, down)
	}

	// The drop arm classifies and then does nothing. It is reachable now
	// (rpg-toolkit#1111 — see contactDrop), and doing nothing is still the
	// right amount: no bubble forms, which is correct on its own, and the
	// walk-stop's shape — where the walk ends and what the player is offered —
	// is rpg-toolkit#1020's design rather than something to guess here. When
	// #1020 lands, this is where the early stop attaches.
	_, _ = drops, dropBy

	return &trigger{}, nil
}

// formation names the members entering the bubble and who enters it unaware.
//
// Only the engaged: a fight is localized, so the members who noticed each
// other leave the world clock and everybody else keeps exploring. Stragglers
// join later through the same classification, which is what makes a fight
// grow rather than start twice.
func (e *Encounter) formation(engaged map[MemberID]bool, down map[MemberID]bool) (*trigger, error) {
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
		unaware, err := e.unawareOfOpposition(id, down)
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
func (e *Encounter) unawareOfOpposition(id MemberID, down map[MemberID]bool) (bool, error) {
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
		// A body in view is not opposition in view. Watching a corpse is not
		// what stops you being surprised by the thing that is still standing.
		if down[other.ID] {
			continue
		}
		if other.Kind != member.Kind {
			return false, nil
		}
	}

	return true, nil
}

// sidesInContactOrder returns the STANDING players and the STANDING monsters,
// each sorted.
//
// Down members are on neither side, which is the census defect this closes: the
// partition used to be by Kind alone, so a dead monster was still a monster and
// sight started a fight with it. A corpse can neither start a fight nor be
// pulled into one already running, from either side of the roster — a member
// who is not on a side is in no pair, and a pair is the only thing that forms
// or joins a bubble.
//
// Sorted because e.members is a map and C8 requires that what a pass concludes
// be a function of persisted data rather than of iteration order — the same
// reason buildMemberOutcomes sorts.
func (e *Encounter) sidesInContactOrder(down map[MemberID]bool) (players, monsters []MemberID) {
	for id, member := range e.members {
		if down[id] {
			continue
		}
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
func (e *Encounter) applyTrigger(deltas map[MemberID]*IntelDelta) (*FormedBubble, map[MemberID]*IntelDelta, error) {
	// The world notices who is down BEFORE it works out who is fighting whom.
	// Both halves of that order matter: a body must not be classified as an
	// enemy, and the beat saying so must not land after a bubble-formed beat
	// it contradicts (rpg-toolkit#1075).
	down, noticedDeltas, err := e.noticeDown()
	if err != nil {
		return nil, nil, err
	}
	// The deltas noticeDown returns were already classified by the nested
	// driven refresh that produced them. Compose them for the caller without
	// feeding them through classification a second time below.
	outputDeltas := mergeIntelDeltas(nil, deltas)
	outputDeltas = mergeIntelDeltas(outputDeltas, noticedDeltas)

	// The consult may have CLOSED the encounter (TriggerMemberDown fires
	// there). A closed scene forms no new fights: the ended beat is the
	// story's last word, and a classification after it would mutate a world
	// that is over — a bubble-formed after an ended is the contradiction
	// rpg-toolkit#1075 guards against, told forward.
	if e.outcome != nil {
		return nil, outputDeltas, nil
	}

	verdict, err := e.classify(deltas, down)
	if err != nil {
		return nil, nil, err
	}

	if len(verdict.joined) > 0 {
		for _, id := range verdict.joined {
			transferred, err := e.Transfer(&TransferInput{
				Member: id,
				To:     ClockTurn,
				Pos:    e.bubbleSize(),
			})
			if err != nil {
				return nil, nil, fmt.Errorf("trigger transfer %q: %w", id, err)
			}
			outputDeltas = mergeIntelDeltas(outputDeltas, transferred.IntelDeltas)
		}

		return nil, outputDeltas, nil
	}

	if len(verdict.form) == 0 {
		return nil, outputDeltas, nil
	}

	order, err := e.initiative.RollInitiative(append([]MemberID(nil), verdict.form...))
	if err != nil {
		return nil, nil, fmt.Errorf("trigger roll initiative: %w", err)
	}

	formed, err := e.form(&FormInput{Order: order, Surprised: verdict.surprised})
	if err != nil {
		return nil, nil, fmt.Errorf("trigger form: %w", err)
	}
	outputDeltas = mergeIntelDeltas(outputDeltas, formed.IntelDeltas)

	return &FormedBubble{
		Order:     order,
		Surprised: verdict.surprised,
		Seq:       formed.Seq,
	}, outputDeltas, nil
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
