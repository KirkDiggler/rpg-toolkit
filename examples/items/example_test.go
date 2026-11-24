// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

func Example_itemComposition() {
	ctx, bus := context.Background(), events.NewEventBus()
	i, err := items.New(ringConfig("ring-17", 3, nil))
	if err != nil {
		panic(err)
	}
	if err := i.Attach(ctx, items.AttachInput{Bus: bus, HolderID: "rook"}); err != nil {
		panic(err)
	}
	economy := combat.NewActionEconomy()
	result, err := payRing(ctx, i, "rook", economy, &fixedRoller{face: 6})
	if err != nil {
		panic(err)
	}
	fmt.Printf("source: %s; healing calculation: %d\n", i.GetID(), result.Total)
	fmt.Printf("charges after use: %d\n", i.Data().Pools[chargePool])
	fmt.Printf("bonus actions left: %d\n", economy.BonusActionsRemaining)
	if err := i.Detach(ctx); err != nil {
		panic(err)
	}
	if err := i.Attach(ctx, items.AttachInput{Bus: bus, HolderID: "vale"}); err != nil {
		panic(err)
	}
	fmt.Printf("charges after transfer: %d\n", i.Data().Pools[chargePool])
	if err := items.RecoveryTopic.On(bus).Publish(ctx, items.RecoveryEvent{HolderID: "rook", Trigger: shortRest}); err != nil {
		panic(err)
	}
	fmt.Printf("charges after old holder rests: %d\n", i.Data().Pools[chargePool])
	if err := items.RecoveryTopic.On(bus).Publish(ctx, items.RecoveryEvent{HolderID: "vale", Trigger: shortRest}); err != nil {
		panic(err)
	}
	fmt.Printf("charges after current holder rests: %d\n", i.Data().Pools[chargePool])
	if err := i.Detach(ctx); err != nil {
		panic(err)
	}
	// Output:
	// source: ring-17; healing calculation: 9
	// charges after use: 0
	// bonus actions left: 0
	// charges after transfer: 0
	// charges after old holder rests: 0
	// charges after current holder rests: 1
}
