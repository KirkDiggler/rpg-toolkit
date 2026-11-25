// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// record_host_test.go is THE HOST'S OWN ROLLS, READ BACK FROM THE RECORD.
// The world's die is not the only one a run throws: the host rolls
// initiative and resolves every driven swing (the session's diceSeam). Those
// are host rolls, not world rolls, and the record does not carry their dice —
// it carries what they decided: the order a fight formed in
// (`bubble-formed`'s `order`) and what each driven swing landed (its
// act-tagged `struck` or `missed` beat, with the amount). The replay's host
// answers from those, so a driven side the record failed to carry would show
// as a replay that cannot answer or lands another log.

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// host is the collaborators that roll: the world's die, the host's driven
// swing, the host's initiative and the host's Stealth.
type host struct {
	die        dice.Roller
	striker    encounter.Striker
	initiative encounter.InitiativeRoller
	check      encounter.CheckResolver
}

// rollingHost is a played run's host: the world's die given, and the host's
// own swings, initiative and Stealth on a crypto die of their own.
func rollingHost(world dice.Roller) host {
	own := dice.NewRoller()

	return host{
		die: world, striker: rollingStriker{die: own}, initiative: shuffledInitiative{die: own},
		check: hostStealth{die: own},
	}
}

// recordHost is a replay's host: the world's die and the host's answers all
// read from the run's record.
func recordHost(t *testing.T, world dice.Roller, log []record.EntryData) host {
	t.Helper()

	return host{
		die: world, striker: newRecordStriker(t, log), initiative: newRecordInitiative(t, log),
		check: newRecordStealth(t, log),
	}
}

// recordInitiative answers each fight's initiative with the order the
// record's next `bubble-formed` beat holds, which must be the same members.
type recordInitiative struct {
	mu     sync.Mutex
	orders []formed
	next   int
}

// formed is one ask the host answered: a fight's formation, or one creature's
// join of a fight under way (a `transferred` entry carrying `first_round`).
type formed struct {
	seq   uint64
	order []encounter.MemberID
	rolls []encounter.InitiativeRoll
	// join is true for a join: one item per joiner, answered one ask at a time
	// with the roll the entry carries, or a placed answer when it carries none.
	join bool
}

func newRecordInitiative(t *testing.T, log []record.EntryData) *recordInitiative {
	t.Helper()
	out := &recordInitiative{}
	for _, e := range log {
		var b struct {
			Beat       string                     `json:"beat"`
			Member     encounter.MemberID         `json:"member"`
			Order      []encounter.MemberID       `json:"order"`
			Rolls      []encounter.InitiativeRoll `json:"rolls"`
			FirstRound *int                       `json:"first_round"`
		}
		require.NoError(t, json.Unmarshal(e.Payload, &b), "beat seq %d", e.Seq)
		switch {
		case b.Beat == encounter.BeatFightStarted:
			out.orders = append(out.orders, formed{seq: e.Seq, order: b.Order, rolls: b.Rolls})
		case b.Beat == encounter.BeatTransferred && b.FirstRound != nil:
			out.orders = append(out.orders, formed{seq: e.Seq, order: []encounter.MemberID{b.Member}, rolls: b.Rolls, join: true})
		}
	}

	return out
}

func (r *recordInitiative) RollInitiative(members []encounter.MemberID) ([]encounter.InitiativeRoll, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next >= len(r.orders) {
		return nil, fmt.Errorf("initiative for %v: the record forms no further fight: %w", members, errNotOnRecord)
	}
	// A formation answers one ask for its whole order. Joins answer one ask
	// for however many joined in one verb: the next k join items, k asked.
	take := []formed{r.orders[r.next]}
	if take[0].join {
		take = take[:0]
		for i := r.next; i < len(r.orders) && r.orders[i].join && len(take) < len(members); i++ {
			take = append(take, r.orders[i])
		}
	}
	var held []encounter.MemberID
	for _, f := range take {
		held = append(held, f.order...)
	}
	asked := slices.Clone(members)
	slices.Sort(asked)
	slices.Sort(held)
	if !slices.Equal(asked, held) {
		return nil, fmt.Errorf("initiative for %v: the record's next ask (beat seq %d) holds %v: %w", members, take[0].seq, held, errNotOnRecord)
	}
	r.next += len(take)

	// A member the record holds a roll for gets that roll back, so a replay
	// writes the entry it read; everyone else is placed. Answered in the
	// recorded order.
	var out []encounter.InitiativeRoll
	for _, f := range take {
		recorded := make(map[encounter.MemberID]encounter.InitiativeRoll, len(f.rolls))
		for _, roll := range f.rolls {
			recorded[roll.Member] = roll
		}
		for _, id := range f.order {
			if roll, ok := recorded[id]; ok {
				out = append(out, roll)
				continue
			}
			out = append(out, encounter.InitiativeRoll{Member: id})
		}
	}

	return out, nil
}

// recordStriker lands each driven swing as the record's next act-tagged
// swing outcome landed, which must be the same attacker on the same target.
type recordStriker struct {
	mu    sync.Mutex
	swing []swing
	next  int
}

// swing is one driven swing's outcome on the record.
type swing struct {
	seq     uint64
	outcome encounter.RecordInput
}

func newRecordStriker(t *testing.T, log []record.EntryData) *recordStriker {
	t.Helper()
	out := &recordStriker{}
	for _, e := range log {
		if !strings.HasPrefix(e.Tags["act"], "attack:") {
			continue
		}
		var b verbBody
		require.NoError(t, json.Unmarshal(e.Payload, &b), "beat seq %d", e.Seq)
		if b.Beat == string(encounter.OutcomeStruck) || b.Beat == string(encounter.OutcomeMissed) {
			out.swing = append(out.swing, swing{e.Seq, outcomeOf(t, e.Seq, b)})
		}
	}

	return out
}

func (r *recordStriker) Strike(
	_ context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, _ core.Ref,
) error {
	r.mu.Lock()
	if r.next >= len(r.swing) {
		r.mu.Unlock()
		return fmt.Errorf("%s swings at %s: the record holds no further driven swing: %w", attacker, target, errNotOnRecord)
	}
	s := r.swing[r.next]
	if s.outcome.Actor != attacker || !slices.Contains(s.outcome.Targets, target) {
		r.mu.Unlock()
		return fmt.Errorf("%s swings at %s: the record's next swing (beat seq %d) is %s at %v: %w",
			attacker, target, s.seq, s.outcome.Actor, s.outcome.Targets, errNotOnRecord)
	}
	r.next++
	r.mu.Unlock()
	outcome := s.outcome
	_, err := enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &outcome}}})

	return err
}

// remaining is how many of the record's host answers the replay did not use.
func (h host) remaining() (swings, fights, sneaks int) {
	if s, ok := h.striker.(*recordStriker); ok {
		swings = len(s.swing) - s.next
	}
	if i, ok := h.initiative.(*recordInitiative); ok {
		fights = len(i.orders) - i.next
	}
	if c, ok := h.check.(*recordStealth); ok {
		sneaks = len(c.rolls) - c.next
	}

	return swings, fights, sneaks
}

// TestRecordInitiativeReturnsRecordedRolls: a roll on the record's
// bubble-formed entry comes back for its member, and a member without one is
// placed, in the recorded order.
func TestRecordInitiativeReturnsRecordedRolls(t *testing.T) {
	modifier := 2
	payload, err := json.Marshal(map[string]any{
		"beat":  encounter.BeatFightStarted,
		"order": []encounter.MemberID{"alice", "goblin"},
		"rolls": []encounter.InitiativeRoll{{
			Member: "alice", Roll: 14, Total: 16,
			Calculation: &encounter.RollCalculation{
				Components: []encounter.RollComponent{
					{
						Source: encounter.RollSource{Ref: "dnd5e:dice:d20", Name: "d20", SourceID: "alice"},
						Dice:   &encounter.DiceTrace{Notation: "1d20", DieSize: 20, OriginalRolls: []int{14}, FinalRolls: []int{14}, Subtotal: 14},
					},
					{Source: encounter.RollSource{Ref: "dnd5e:modifiers:dex", Name: "Dexterity"}, Modifier: &modifier},
				},
				Total: 16,
			},
		}},
	})
	require.NoError(t, err)

	init := newRecordInitiative(t, []record.EntryData{{Seq: 7, Payload: payload}})
	got, err := init.RollInitiative([]encounter.MemberID{"goblin", "alice"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, encounter.MemberID("alice"), got[0].Member)
	require.NotNil(t, got[0].Calculation)
	require.Equal(t, 14, got[0].Roll)
	require.Equal(t, 16, got[0].Total)
	require.Equal(t, encounter.MemberID("goblin"), got[1].Member)
	require.Nil(t, got[1].Calculation)
}

// TestRecordInitiativeAnswersAJoinFromItsEntry: a `transferred` entry that
// carries `first_round` is one join on the record. Asked for the joiner, the
// host answers the roll the entry holds, or a placed answer when it holds
// none; asked for somebody the record did not join, it says so.
func TestRecordInitiativeAnswersAJoinFromItsEntry(t *testing.T) {
	rolled := encounter.InitiativeRoll{
		Member: "ogre", Roll: 9, Total: 10,
		Calculation: &encounter.RollCalculation{
			Components: []encounter.RollComponent{{
				Source: encounter.RollSource{Ref: "dnd5e:dice:d20", Name: "d20", SourceID: "ogre"},
				Dice:   &encounter.DiceTrace{Notation: "1d20", DieSize: 20, OriginalRolls: []int{9}, FinalRolls: []int{9}, Subtotal: 9},
			}},
			Total: 9,
		},
	}
	join := func(seq uint64, member string, rolls ...encounter.InitiativeRoll) record.EntryData {
		body := map[string]any{
			"beat": encounter.BeatTransferred, "member": member, "to": "turn", "round": 3, "first_round": 4,
		}
		if len(rolls) > 0 {
			body["rolls"] = rolls
		}
		payload, err := json.Marshal(body)
		require.NoError(t, err)

		return record.EntryData{Seq: seq, Payload: payload}
	}
	// A world-ward transfer carries no first_round and is no ask.
	toWorld, err := json.Marshal(map[string]any{"beat": encounter.BeatTransferred, "member": "bob", "to": "world"})
	require.NoError(t, err)

	init := newRecordInitiative(t, []record.EntryData{
		{Seq: 5, Payload: toWorld}, join(6, "ogre", rolled), join(7, "wolf"),
	})
	require.Len(t, init.orders, 2, "two joins, and the world-ward transfer is not one")

	got, err := init.RollInitiative([]encounter.MemberID{"ogre"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, rolled, got[0])

	_, err = init.RollInitiative([]encounter.MemberID{"goblin"})
	require.ErrorIs(t, err, errNotOnRecord)

	got, err = init.RollInitiative([]encounter.MemberID{"wolf"})
	require.NoError(t, err)
	require.Equal(t, []encounter.InitiativeRoll{{Member: "wolf"}}, got)
	_, fights, _ := host{initiative: init}.remaining()
	require.Zero(t, fights, "every ask on the record was answered")
}
