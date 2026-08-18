// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// everyoneStanding is the Standing capability these tests install by default.
//
// Nobody is ever down, which is what every scene written before death existed
// was already assuming. Installing it explicitly rather than letting a nil mean
// it is the whole point of the capability being required: a scene states what
// it believes about hit points out loud (rpg-toolkit#1033 — capabilities are
// supplied, never defaulted).
type everyoneStanding struct{}

func (everyoneStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// downList reports a caller-set roster down, and can be changed MID-SCENE —
// which is how a test drops somebody, or stands them back up, without
// rebuilding the encounter. That is the only way to test a pull: if the
// composition cached the answer, changing this would change nothing.
//
// It answers only about members it was ASKED about, because that is what a real
// implementation does — the composition hands over its roster and the rulebook
// reports on those. A name that has since left the encounter is not in the
// question and is not in the answer.
type downList struct {
	down []encounter.MemberID
}

func (d *downList) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	asked := make(map[encounter.MemberID]bool, len(members))
	for _, id := range members {
		asked[id] = true
	}

	var down []encounter.MemberID
	for _, id := range d.down {
		if asked[id] {
			down = append(down, id)
		}
	}

	return down, nil
}

// strangerWhenTold is a rulebook that starts well-behaved and then answers with
// somebody who is not in the encounter at all — the mis-wiring D4 could arrive
// as, held here so the composition's refusal can be asserted from a scene that
// was already running.
type strangerWhenTold struct {
	lying bool
}

func (s *strangerWhenTold) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	if s.lying {
		return []encounter.MemberID{"a-ghost"}, nil
	}

	return nil, nil
}
