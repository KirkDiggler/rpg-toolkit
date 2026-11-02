// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// record_check_test.go is THE HOST'S STEALTH, READ BACK FROM THE RECORD. A
// sneaking mover crossing into a watcher's zone is rolled by the host's
// check seam (the session's rulebook), not the world's die, and the record
// does not carry the d20. It carries the verdict: each roll's `snuck` beat,
// with the DC faced, the total and whether it was beaten. The replay's host
// answers each sneak roll from those, in order, so a roll the record failed
// to write would show as a replay that cannot answer.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// stealthModifier is every sneaker's Stealth bonus on this host.
const stealthModifier = 7

// hostStealth is the played run's check seam: a sneak roll is the host's
// d20 plus [stealthModifier] against the DC handed in, on the host's own
// die (never the world's, so the world's rolls stay the record's `tempered`
// and `answered` beats alone). Any other check is not asked on these routes.
type hostStealth struct{ die dice.Roller }

func (hostStealth) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("check for %s: %w", in.Member, errNotAsked)
}

func (s hostStealth) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	if len(in.Approaches) != 1 || in.Approaches[0].Ability != "stealth" {
		return nil, fmt.Errorf("discovery check for %s %v: %w", in.Member, in.Approaches, errNotAsked)
	}
	d20, err := s.die.Roll(context.Background(), 20)
	if err != nil {
		return nil, err
	}
	applied := in.Approaches[0]
	total := d20 + stealthModifier

	return &encounter.ResolveCheckOutput{Beaten: total >= applied.DC, Applied: applied, Total: total}, nil
}

// recordStealth answers each sneak roll with the verdict the record's next
// `snuck` beat holds, which must be the same mover against the same DC.
type recordStealth struct {
	mu    sync.Mutex
	rolls []snuck
	next  int
}

// snuck is one sneak roll's verdict on the record.
type snuck struct {
	seq    uint64
	member encounter.MemberID
	dc     int
	total  int
	beaten bool
}

func newRecordStealth(t *testing.T, log []record.EntryData) *recordStealth {
	t.Helper()
	out := &recordStealth{}
	for _, e := range log {
		var b struct {
			Beat   string `json:"beat"`
			Member string `json:"member"`
			DC     int    `json:"dc"`
			Total  int    `json:"total"`
			Beaten bool   `json:"beaten"`
		}
		require.NoError(t, json.Unmarshal(e.Payload, &b), "beat seq %d", e.Seq)
		if b.Beat == encounter.BeatSnuck {
			out.rolls = append(out.rolls, snuck{e.Seq, encounter.MemberID(b.Member), b.DC, b.Total, b.Beaten})
		}
	}

	return out
}

func (*recordStealth) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("check for %s: %w", in.Member, errNotAsked)
}

func (r *recordStealth) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next >= len(r.rolls) {
		return nil, fmt.Errorf("sneak roll for %s: the record holds no further snuck beat: %w", in.Member, errNotOnRecord)
	}
	roll := r.rolls[r.next]
	if len(in.Approaches) != 1 || roll.member != in.Member || roll.dc != in.Approaches[0].DC {
		return nil, fmt.Errorf("sneak roll for %s %v: the record's next (beat seq %d) is %s at DC %d: %w",
			in.Member, in.Approaches, roll.seq, roll.member, roll.dc, errNotOnRecord)
	}
	r.next++

	return &encounter.ResolveCheckOutput{Beaten: roll.beaten, Applied: in.Approaches[0], Total: roll.total}, nil
}
