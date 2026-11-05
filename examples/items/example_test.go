// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func Example_itemComposition() {
	ctx, bus := context.Background(), events.NewEventBus()
	out, err := composeItem(composeItemInput{
		Definition: ringDefinition(3), State: instanceData{ID: "ring-17", Remaining: 1},
	})
	if err != nil {
		panic(err)
	}
	i := out.Item
	if err := i.attach(ctx, bus, "rook"); err != nil {
		panic(err)
	}
	result, err := i.use(useInput{Holder: "rook", Roller: &fixedRoller{face: 6}})
	if err != nil {
		panic(err)
	}
	fmt.Printf("source: %s; healing calculation: %d\n", result.Calculation.Components[0].Source.SourceID, result.Calculation.Total)
	fmt.Printf("charges after use: %d\n", i.data().Remaining)
	if err := i.detach(ctx); err != nil {
		panic(err)
	}
	if err := i.attach(ctx, bus, "vale"); err != nil {
		panic(err)
	}
	fmt.Printf("charges after transfer: %d\n", i.data().Remaining)
	if err := restTopic.On(bus).Publish(ctx, restEvent{Holder: "rook", Kind: shortRest}); err != nil {
		panic(err)
	}
	fmt.Printf("charges after old holder rests: %d\n", i.data().Remaining)
	if err := restTopic.On(bus).Publish(ctx, restEvent{Holder: "vale", Kind: shortRest}); err != nil {
		panic(err)
	}
	fmt.Printf("charges after current holder rests: %d\n", i.data().Remaining)
	if err := i.detach(ctx); err != nil {
		panic(err)
	}
	// Output:
	// source: ring-17; healing calculation: 9
	// charges after use: 0
	// charges after transfer: 0
	// charges after old holder rests: 0
	// charges after current holder rests: 1
}
