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
// swing and the host's initiative.
type host struct {
	die        dice.Roller
	striker    encounter.Striker
	initiative encounter.InitiativeRoller
}

// rollingHost is a played run's host: the world's die given, and the host's
// own swings and initiative on a crypto die of their own.
func rollingHost(world dice.Roller) host {
	own := dice.NewRoller()

	return host{die: world, striker: rollingStriker{die: own}, initiative: shuffledInitiative{die: own}}
}

// recordHost is a replay's host: the world's die and the host's answers all
// read from the run's record.
func recordHost(t *testing.T, world dice.Roller, log []record.EntryData) host {
	t.Helper()

	return host{die: world, striker: newRecordStriker(t, log), initiative: newRecordInitiative(t, log)}
}

// recordInitiative answers each fight's initiative with the order the
// record's next `bubble-formed` beat holds, which must be the same members.
type recordInitiative struct {
	mu     sync.Mutex
	orders []formed
	next   int
}

// formed is one fight's formation on the record.
type formed struct {
	seq   uint64
	order []encounter.MemberID
}

func newRecordInitiative(t *testing.T, log []record.EntryData) *recordInitiative {
	t.Helper()
	out := &recordInitiative{}
	for _, e := range log {
		var b struct {
			Beat  string               `json:"beat"`
			Order []encounter.MemberID `json:"order"`
		}
		require.NoError(t, json.Unmarshal(e.Payload, &b), "beat seq %d", e.Seq)
		if b.Beat == encounter.BeatFightStarted {
			out.orders = append(out.orders, formed{e.Seq, b.Order})
		}
	}

	return out
}

func (r *recordInitiative) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next >= len(r.orders) {
		return nil, fmt.Errorf("initiative for %v: the record forms no further fight: %w", members, errNotOnRecord)
	}
	f := r.orders[r.next]
	asked, held := slices.Clone(members), slices.Clone(f.order)
	slices.Sort(asked)
	slices.Sort(held)
	if !slices.Equal(asked, held) {
		return nil, fmt.Errorf("initiative for %v: the record's fight (beat seq %d) formed %v: %w", members, f.seq, f.order, errNotOnRecord)
	}
	r.next++

	return slices.Clone(f.order), nil
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
func (h host) remaining() (swings, fights int) {
	if s, ok := h.striker.(*recordStriker); ok {
		swings = len(s.swing) - s.next
	}
	if i, ok := h.initiative.(*recordInitiative); ok {
		fights = len(i.orders) - i.next
	}

	return swings, fights
}
