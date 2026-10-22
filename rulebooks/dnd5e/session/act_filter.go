// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// act_filter.go is THE PERCEPTION FILTER on the act stream (rpg-project#631
// E6, ideas/event-stream R35, Law 10): a recipient receives only the acts
// they perceive. The record stream is untouched — it stays the whole truth
// (R35); only the act stream is narrowed, per recipient, here in the fold.
//
// THE FILTER READS ONLY THE RECIPIENT'S OWN ENTRIES (Law 10). Who the
// recipient sees is replayed from their own `sighted` beats' gained and lost
// lists; which fights they are in from `bubble-formed`'s order and
// `bubble-dissolved`'s members. Nothing is looked up live, so the answer is
// the recipient's testimony at that entry, rebuilt from the record alone.
//
// AN ACT IS DELIVERED when any of these holds ([viewState.perceives]):
//
//  1. It is the recipient's own: its subject is the recipient, or it sits
//     under an act the recipient opened (the act's first entry is theirs),
//     so a table's answer to their line arrives inside it.
//  2. It is addressed to the recipient: a swing or a line at them, a cast or
//     an activation touching them, a window or a roll window asking them, a
//     change of their own standing.
//  3. Its subject is in the recipient's seen set at that entry.
//  4. It is clock or roster, by kind: tick, scene_opened and ended always;
//     fight_formed and fight_dissolved when the recipient is in the fight;
//     turn_ended when its member is in the recipient's fight; joined,
//     exited and arrived under rule 3; a window under rule 3 by its mover,
//     and a roll window by its roller (the shared die is seen thrown); a
//     stance turn when the party's side is one of the two.
//  5. It is a perception, or a door changing: the encounter already
//     audiences those to the members who perceive them.
//
// A piece that folds into the segment already open for this recipient is
// part of an act they are receiving (a cast's saves, a line's answer), and
// arrives with it.
//
// WALKING INTO VIEW. In the record a step precedes the `sighted` beat its
// own sight refresh writes (a verb's beat precedes its consequences). So an
// unseen mover's step — and an arrival or a join — is HELD until the entries
// that follow under the same act, up to the mover's next step. If they hold
// this recipient's sighting that gains the mover, the sighting is delivered
// first and the held step after it, starting the walk from the cell that
// brought them into view. Otherwise the held step is dropped. The step that
// LEAVES view needs no rule: the mover was seen at its start, so it is
// delivered, then the `lost` sighting, and nothing after.
//
// A PROP ARRIVING is held the same way, and released by a sighting under its
// act that says the recipient's view of props changed (`knowledge_changed`):
// a sighting names members only, and that flag is how the record tells an
// observer they now see a different set of things. An arrival's own sight
// refresh writes it.
//
// A RELEASED STEP TAKES ITS SIGHTING'S ID. It is delivered after that
// sighting, so naming it by its own (earlier) entry would let a resume after
// the step's id hand back the sighting a second time. Under the sighting's
// id the two are one delivery unit (see [Act.ID]), and a resume after it
// skips both.
//
// A DROPPED ENTRY SPLITS NOTHING. R19 closes a segment on an entry
// "delivered to that recipient", so a filtered entry never interrupts a walk
// the recipient is watching.
//
// LIVE EQUALS CATCH-UP (#541 R2) by construction: the view is seeded by a
// prepass over the recipient's entries before the fold's start
// ([seedView]), and the fold then filters as it goes, so [actsAfter] — the
// one function both paths call — is still a pure function of the entries.

// viewState is what one recipient perceives at the fold's current entry.
type viewState struct {
	// seen is the members the recipient sees now (their `sighted` gained,
	// less their lost).
	seen map[string]bool
	// fights is the members of every fight the recipient is in
	// (`bubble-formed` order naming them, until `bubble-dissolved` does).
	fights map[string]bool
}

// newView is an empty view: the recipient sees no one and fights no one.
func newView() *viewState {
	return &viewState{seen: map[string]bool{}, fights: map[string]bool{}}
}

// seedView replays the recipient's entries before the fold's start into the
// view the fold begins from: their sightings and their fights.
func seedView(recipient string, entries []record.Entry) *viewState {
	v := newView()
	for _, e := range entries {
		v.observe(recipient, e.Payload)
	}
	return v
}

// viewKeys are the payload keys the view reads.
type viewKeys struct {
	Beat    string   `json:"beat"`
	Gained  []string `json:"gained"`
	Lost    []string `json:"lost"`
	Order   []string `json:"order"`
	Members []string `json:"members"`
}

// observe moves the view past one entry: a sighting gains and loses, a fight
// forming with the recipient in it joins them to its members, and one
// dissolving with them in it lets its members go.
func (v *viewState) observe(recipient string, payload []byte) {
	beat := peekBeat(payload)
	if beat != encounter.BeatSighted && beat != encounter.BeatFightStarted && beat != encounter.BeatFightEnded {
		return
	}
	var k viewKeys
	if json.Unmarshal(payload, &k) != nil {
		return // an unreadable entry moves nothing; its own piece says so
	}
	switch beat {
	case encounter.BeatSighted:
		for _, id := range k.Gained {
			v.seen[id] = true
		}
		for _, id := range k.Lost {
			delete(v.seen, id)
		}
	case encounter.BeatFightStarted:
		if contains(k.Order, recipient) {
			for _, id := range k.Order {
				v.fights[id] = true
			}
		}
	case encounter.BeatFightEnded:
		if contains(k.Members, recipient) {
			for _, id := range k.Members {
				delete(v.fights, id)
			}
		}
	}
}

// sighting is what a sighting payload brings into view: the members it
// gains, and whether the observer's view of props and doors changed
// (`knowledge_changed`). Zero for any other beat.
func sighting(payload []byte) (gained []string, knowledge bool) {
	if peekBeat(payload) != encounter.BeatSighted {
		return nil, false
	}
	var k struct {
		Gained           []string `json:"gained"`
		KnowledgeChanged bool     `json:"knowledge_changed"`
	}
	if json.Unmarshal(payload, &k) != nil {
		return nil, false
	}
	return k.Gained, k.KnowledgeChanged
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// verdict is what the filter does with one piece.
type verdict int

const (
	// drop: the recipient does not perceive it.
	drop verdict = iota
	// deliver: the recipient perceives it.
	deliver
	// hold: its subject is unseen, but a sighting later in the same act may
	// bring them into view (walking into view).
	hold
)

// judge is the filter's verdict on one piece of an entry under act tag,
// given whether that act is the recipient's own.
func (v *viewState) judge(recipient string, own bool, p piece) verdict {
	if p.family == FamilyPerception || p.body == nil {
		// Perceptions are already the recipient's; an undecoded entry names
		// no subject and arrives marked, so a bad beat is seen, not lost.
		return deliver
	}
	if _, ok := p.body.(DoorAct); ok {
		// A door beat is already its witnesses' alone: the encounter
		// audiences it to the actor and whoever sees both the door and the
		// actor (mutableWitnesses), or the door alone for an unattributed
		// change. Holding it is perceiving it.
		return deliver
	}
	if own && p.family != FamilyClock {
		return deliver
	}
	if c, ok := p.body.(ClockEntry); ok {
		return v.judgeClock(recipient, c)
	}
	if s, ok := p.body.(StateChange); ok && s.Change == StateStance {
		return verdictOf(s.Stance != nil && betweenParty(s.Stance.Between))
	}
	if addressedTo(recipient, p.body) {
		return deliver
	}
	subject := subjectOf(p.body)
	if subject.Kind != PartyMember {
		return drop
	}
	if subject.ID == recipient || v.seen[subject.ID] {
		return deliver
	}
	if _, walk := p.body.(WalkAct); walk {
		return hold
	}
	return drop
}

// judgeClock is rule 4: the table's bookkeeping, by kind.
func (v *viewState) judgeClock(recipient string, c ClockEntry) verdict {
	switch c.Mark {
	case ClockTick, ClockSceneOpened, ClockEnded:
		return deliver
	case ClockFightFormed:
		return verdictOf(c.Fight != nil && containsParty(c.Fight.Order, recipient))
	case ClockFightDissolved:
		return verdictOf(c.Dissolved != nil && containsParty(c.Dissolved.Members, recipient))
	case ClockTurnEnded:
		return verdictOf(c.Turn != nil && (v.fights[c.Turn.Member.ID] || c.Turn.Member.ID == recipient))
	case ClockJoined:
		return v.judgeRoster(recipient, c.Member.ID)
	case ClockExited:
		if c.Departure == nil {
			return drop
		}
		return v.judgeRoster(recipient, c.Departure.Member.ID)
	case ClockArrived:
		if c.Arrival == nil {
			return drop
		}
		return v.judgeRoster(recipient, c.Arrival.ID)
	case ClockWindowOpened:
		if c.Window == nil {
			return drop
		}
		// The mover's own, one asking the recipient, or a step they watch.
		if mover := c.Window.Member.ID; mover == recipient || v.seen[mover] {
			return deliver
		}
		for _, w := range c.Window.Windows {
			if w.Audience.ID == recipient {
				return deliver
			}
		}
		return drop
	case ClockRollWindowOpened:
		// The roller's own, or a roll posed to someone they see: the shared
		// die is on the table for whoever watches it thrown.
		return verdictOf(c.RollWindow != nil && (c.RollWindow.Audience.ID == recipient || v.seen[c.RollWindow.Audience.ID]))
	default:
		return drop
	}
}

// judgeRoster is a join, an exit or an arrival: the recipient's own, one
// they see, or — for one they do not see yet — held for the sighting that
// may follow it.
func (v *viewState) judgeRoster(recipient, id string) verdict {
	if id == recipient || v.seen[id] {
		return deliver
	}
	return hold
}

func verdictOf(ok bool) verdict {
	if ok {
		return deliver
	}
	return drop
}

// betweenParty is whether the party's side is one of a stance turn's two.
func betweenParty(between []Party) bool {
	for _, f := range between {
		if f.ID == string(encounter.FactionParty) {
			return true
		}
	}
	return false
}

func containsParty(parties []Party, id string) bool {
	for _, p := range parties {
		if p.ID == id {
			return true
		}
	}
	return false
}

// subjectOf is who did what a body tells: the member a recipient must see
// for it to reach them.
//
//nolint:gocyclo // one arm per body
func subjectOf(body ActBody) Party {
	switch b := body.(type) {
	case WalkAct:
		return b.Mover
	case AttackAct:
		return b.By
	case CastAct:
		return b.By
	case ActivateAct:
		return b.By
	case SpeakAct:
		return b.By
	case DeathSaveAct:
		return b.By
	case LootAct:
		return b.By
	case HoldAct:
		return b.By
	case DropAct:
		return b.By
	case DoorAct:
		return b.By
	case EquipAct:
		return b.By
	case StayAct:
		return b.Member
	case SneakAct:
		return b.By
	case RestAct:
		return b.By
	case TableAnswer:
		return b.Who
	case StateChange:
		return b.Party
	default:
		return Party{}
	}
}

// addressedTo is rule 2: the body is done to the recipient.
func addressedTo(recipient string, body ActBody) bool {
	is := func(p Party) bool { return p.Kind == PartyMember && p.ID == recipient }
	switch b := body.(type) {
	case AttackAct:
		return is(b.To)
	case CastAct:
		for _, t := range b.Targets {
			if is(t) {
				return true
			}
		}
		for _, r := range b.Results {
			if is(r.Target) {
				return true
			}
		}
	case ActivateAct:
		if is(b.Target) {
			return true
		}
		for _, r := range b.Results {
			if is(r.Target) {
				return true
			}
		}
		for _, s := range b.Saves {
			if is(s.Saver) {
				return true
			}
		}
	case SpeakAct:
		return is(b.To)
	case StateChange:
		for _, g := range b.Experience {
			if g.Character == recipient {
				return true
			}
		}
	}
	return false
}
