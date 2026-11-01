// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// A host keeps one blob per creature, a Due heap for time and a Listeners
// index for everything else, and wakes only who is due or addressed.
func Example() {
	p, err := guardRecipe().Compile(testVocabulary())
	if err != nil {
		fmt.Println(err)
		return
	}
	blobs := map[string]*machine.Blob{}
	var due machine.Due[string]
	var listeners machine.Listeners[string]
	seat := func(id, state string) {
		b := p.NewBlob()
		b.State = state
		blobs[id] = &b
		if at, ok := p.DueAt(&b, 0); ok {
			due.Arm(id, at)
		}
		listeners.Set(id, p.Listens(&b))
	}
	seat("gate", "posted")
	seat("cellar", "asleep")

	// Round 1: only the posted guard is due.
	for _, id := range due.Pop(1) {
		got, _ := p.Step(blobs[id], "time", tick{Round: 1}, nil, 1)
		fmt.Println(id, got.Acts[0].Pick)
		if at, ok := p.DueAt(blobs[id], 1); ok {
			due.Arm(id, at)
		}
	}

	// A sound five hexes from each: the index answers who listens, the
	// scope and the guard answer who hears.
	for _, l := range listeners.For("heard") {
		got, _ := p.Step(blobs[l.Key], "heard", sound{Hexes: 5}, nil, 1)
		fmt.Printf("%s fired=%v to=%q\n", l.Key, got.Fired, got.To)
	}
	// Output:
	// gate time
	// cellar fired=false to=""
	// gate fired=true to="alert"
}
