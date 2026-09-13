// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// This file is the content vocabulary the worked minds speak: what a sight
// or a hearing payload says. It belongs to the caller — here, the tests —
// and not to behaviour, which reads it only through the reader below (R2).
// The encounter has its own vocabulary and its own reader.

// kind is what a percept says is there.
type kind string

const (
	// creature is a live thing: something that can strike and be struck.
	creature kind = "creature"
	// noise is a sound: a thing on the hearing channel, of nothing in
	// particular until a mind decides.
	noise kind = "noise"
	// post is a place a guard was told to stand: a banner, a doorway, a
	// mark on the floor. It is perceived like anything else, so returning
	// to it is Toward a name and needs no verb.
	post kind = "post"
)

// percept is what one channel says about one subject.
type percept struct {
	Kind  kind   `json:"kind"`
	Of    string `json:"of,omitempty"`
	Note  string `json:"note,omitempty"`
	Where string `json:"where,omitempty"`
}

func encode(p percept) []byte {
	out, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}

	return out
}

func decode(payload []byte) (percept, bool) {
	var p percept
	if err := json.Unmarshal(payload, &p); err != nil || p.Kind == "" {
		return percept{}, false
	}

	return p, true
}

// reader is how behaviour reads this vocabulary: where the subject is, and
// whether it is a creature. Anything it cannot read is the zero reading.
type reader struct{}

func (reader) Read(h perception.Holding) (*behavior.Reading, error) {
	p, ok := decode(h.Payload)
	if !ok {
		return &behavior.Reading{}, nil
	}

	return &behavior.Reading{Where: p.Where, Creature: p.Kind == creature}, nil
}

// kindOf is what a mind would say a contact is: the kind of the first
// holding it can read. A contact made only of deeds has no kind.
func kindOf(c behavior.Contact) kind {
	for _, h := range c.Holdings {
		if p, ok := decode(h.Payload); ok {
			return p.Kind
		}
	}

	return ""
}

// saysNow reports whether a current holding in the contact says a kind and
// a note.
func saysNow(c behavior.Contact, k kind, note string) bool {
	for _, h := range c.Holdings {
		if len(h.CurrentVia) == 0 {
			continue
		}

		if p, ok := decode(h.Payload); ok && p.Kind == k && p.Note == note {
			return true
		}
	}

	return false
}
