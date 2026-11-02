// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package replaytest replays an encounter from its own record
// (rpg-project#766 S5, design §5 "Reproducible, not seeded"): a roller that
// reads the record's rolls back in the order the log holds them, so a run
// opened again from round 0 and asked the same verbs lands the same beats.
//
// NO SEED (R6). The dice of a played run stay the host's, as random as they
// can be. What makes the run reproducible is that every roll the world made
// is already on the record — a temper dealt at the door is a `tempered`
// beat, a pick is an `answered` beat with its arithmetic — so the replay's
// die is the record itself, read back. A world roll the record does not
// carry is a defect, and [RecordRoller] finds it: the replay asks for a die
// the record never wrote and the roller says which beat it had reached and
// which verb was asking.
//
// WHAT IS THE WORLD'S AND WHAT IS THE HOST'S. This package reads back the
// world's rolls: the dice the composition and its table driver throw. The
// host's collaborators answer from outside the world's die, and a replay
// has to say, for each, where its answers come from (the tests' host,
// replay_test.go):
//
//   - initiative and the driven swing are host rolls the record carries
//     the result of (`bubble-formed`'s order; a swing's act-tagged
//     `struck`/`missed`), and the replay's host reads them back from it;
//   - a player's outcomes, falls and lines are verbs, read off the record;
//   - check verdicts and concealed-door perceivers are not exercised: the
//     routes ask none, and the tests' host refuses if asked;
//   - sight, sheets, equipment and conditions are the host's static facts,
//     the same content both times, not rolls;
//   - a sneak declaration and Speak's reach are not on the record at all.
package replaytest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

var (
	// ErrReplayExhausted is a replay asking for a die after the record's last
	// roll: a world roll the record does not carry.
	ErrReplayExhausted = errors.New("replay: the record holds no further roll")

	// ErrReplayMismatch is a replay asking for a die of a size other than the
	// one the record's next roll was thrown on: the replay has left the
	// record's road, and the face it would read back means nothing there.
	ErrReplayMismatch = errors.New("replay: the record's next roll was thrown on another die")

	// ErrBadBeat is a beat whose body cannot be read, or a roll-bearing beat
	// whose roll is not a face of a die.
	ErrBadBeat = errors.New("replay: a roll-bearing beat that cannot be read")
)

// Roll is one world roll as the record holds it: the beat that carries it,
// the face, and the die it was thrown on.
type Roll struct {
	// Seq is the carrying beat's place in the log.
	Seq uint64
	// Beat is the carrying beat's kind ([encounter.BeatTempered] or
	// [encounter.BeatAnswered]).
	Beat string
	// Who is the member the roll was thrown for: the dealt member of a
	// `tempered` beat, the creature (or the prop) of an `answered` one.
	Who string
	// Face is the roll.
	Face int
	// Of is the die's size.
	Of int
}

// String names the roll by its beat, for a failure message.
func (r Roll) String() string {
	return fmt.Sprintf("%d on a d%d (%s beat seq %d, %s)", r.Face, r.Of, r.Beat, r.Seq, r.Who)
}

// rollBody is the fields of a roll-bearing beat this package reads.
type rollBody struct {
	Beat     string        `json:"beat"`
	Member   string        `json:"member"`
	Creature string        `json:"creature"`
	Prop     string        `json:"prop"`
	Roll     int           `json:"roll"`
	Of       int           `json:"of"`
	Path     []int         `json:"path"`
	Rows     []rowVerdicts `json:"rows"`
}

// rowVerdicts is one row's verdict in an `answered` beat: a group row carries
// its own die and the verdicts of the rows inside it (rpg-project#670).
type rowVerdicts struct {
	Roll int           `json:"roll"`
	Of   int           `json:"of"`
	Rows []rowVerdicts `json:"rows"`
}

// RollsOf is every world roll a log holds, in the order the world threw
// them: log order, and inside one `answered` beat the top level's die first
// and then each group's die down the pick's path — the order
// behavior.Pick rolls them in.
//
// A pick with an empty path rolled nothing (a hold: nothing eligible), and
// contributes no roll.
func RollsOf(entries []record.EntryData) ([]Roll, error) {
	var out []Roll
	for _, entry := range entries {
		var body rollBody
		if err := json.Unmarshal(entry.Payload, &body); err != nil {
			// FAIL CLOSED: every beat body is JSON, and one that is not
			// might be a roll-bearing beat whose roll would drop silently.
			return nil, fmt.Errorf("beat seq %d: %v: %w", entry.Seq, err, ErrBadBeat)
		}
		switch body.Beat {
		case encounter.BeatTempered:
			if body.Of < 1 || body.Roll < 1 {
				return nil, fmt.Errorf("tempered beat seq %d: roll %d of %d: %w", entry.Seq, body.Roll, body.Of, ErrBadBeat)
			}
			out = append(out, Roll{Seq: entry.Seq, Beat: body.Beat, Who: body.Member, Face: body.Roll, Of: body.Of})
		case encounter.BeatAnswered:
			rolls, err := pickRolls(entry.Seq, body)
			if err != nil {
				return nil, err
			}
			out = append(out, rolls...)
		}
	}

	return out, nil
}

// pickRolls is one `answered` beat's dice, top level first, then down the
// path one group at a time.
func pickRolls(seq uint64, body rollBody) ([]Roll, error) {
	if len(body.Path) == 0 {
		return nil, nil
	}
	who := body.Creature
	if who == "" {
		who = body.Prop
	}
	if body.Of < 1 || body.Roll < 1 {
		return nil, fmt.Errorf("answered beat seq %d: roll %d of %d: %w", seq, body.Roll, body.Of, ErrBadBeat)
	}
	out := []Roll{{Seq: seq, Beat: body.Beat, Who: who, Face: body.Roll, Of: body.Of}}
	rows := body.Rows
	for depth, entry := range body.Path[:len(body.Path)-1] {
		if entry < 0 || entry >= len(rows) {
			return nil, fmt.Errorf("answered beat seq %d: path %v leaves the rows at depth %d: %w", seq, body.Path, depth, ErrBadBeat)
		}
		group := rows[entry]
		if group.Of < 1 || group.Roll < 1 {
			return nil, fmt.Errorf("answered beat seq %d: group at depth %d: roll %d of %d: %w",
				seq, depth, group.Roll, group.Of, ErrBadBeat)
		}
		out = append(out, Roll{Seq: seq, Beat: body.Beat, Who: who, Face: group.Roll, Of: group.Of})
		rows = group.Rows
	}

	return out, nil
}

// RecordRoller is a [dice.Roller] that throws nothing: each roll is the
// record's next, in log order. Safe for concurrent use, though a replay
// asks it one roll at a time.
type RecordRoller struct {
	mu    sync.Mutex
	rolls []Roll
	next  int
	verb  string
}

// NewRecordRoller is a roller over every world roll a saved record holds.
func NewRecordRoller(entries []record.EntryData) (*RecordRoller, error) {
	rolls, err := RollsOf(entries)
	if err != nil {
		return nil, err
	}

	return &RecordRoller{rolls: rolls}, nil
}

// During names the verb the replay is asking now, so a roll the record lacks
// names the verb that asked for it.
func (r *RecordRoller) During(verb string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verb = verb
}

// Remaining is how many of the record's rolls the replay has not read back.
func (r *RecordRoller) Remaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.rolls) - r.next
}

// Roll is the record's next roll, which must have been thrown on a die of
// this size.
func (r *RecordRoller) Roll(_ context.Context, size int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.take(size)
}

// RollN is the record's next count rolls, each of which must have been
// thrown on a die of this size.
func (r *RecordRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	if count < 0 {
		return nil, fmt.Errorf("replay: roll %d dice: a negative count", count)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		face, err := r.take(size)
		if err != nil {
			return nil, err
		}
		out = append(out, face)
	}

	return out, nil
}

// take reads back the next roll; the caller holds the lock.
func (r *RecordRoller) take(size int) (int, error) {
	if size <= 0 {
		return 0, fmt.Errorf("replay: a d%d", size)
	}
	if r.next >= len(r.rolls) {
		last := "no roll-bearing beat at all"
		if r.next > 0 {
			last = "the last was " + r.rolls[r.next-1].String()
		}
		return 0, fmt.Errorf("a d%d during %s, after %d rolls (%s): %w", size, r.verbName(), r.next, last, ErrReplayExhausted)
	}
	roll := r.rolls[r.next]
	if roll.Of != size {
		return 0, fmt.Errorf("a d%d during %s, but roll %d was %s: %w", size, r.verbName(), r.next, roll, ErrReplayMismatch)
	}
	r.next++

	return roll.Face, nil
}

// verbName is the verb being asked, or a word saying none was named.
func (r *RecordRoller) verbName() string {
	if r.verb == "" {
		return "an unnamed verb"
	}

	return r.verb
}
