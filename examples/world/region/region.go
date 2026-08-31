// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package region is the guild's layer: two pieces of content, three companies,
// one needle.
//
// It is not a scenario and declares almost nothing of its own — no entities, no
// verbs, no jobs. What it does is compose the bandit camp and the hostage camp
// into one region sharing one journal, tie the three companies to the guild
// they all work for, and state the one thing the guild wants true of the whole
// place before the weekend.
//
// That division is the point of the package existing. Content declares a camp;
// a guild declares an ambition spanning camps. Neither piece of content is in a
// position to write the weekend goal, because neither knows the other exists.
package region

import (
	"context"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/scenarios/banditcamp"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scenarios/hostagecamp"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/goal"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// WeekendGoalID names the guild's standing ambition for the region.
const WeekendGoalID = "pacify-the-region"

// Scenario composes the two camps and ties the companies to the guild.
//
// The ties are the only declaration this package makes, and they are load
// bearing: they put the three hostage-camp companies inside the guild the
// bandit camp is hostile to. After that, a company that talks the camp round
// earns goodwill for the *guild* — because the fold points regard at the
// actor's faction, and the guild is now what the actor's faction belongs to —
// and the camp's declared hostility has something to convert.
//
// Nothing in either camp changed to make that work. Composition is edges.
func Scenario() (world.Scenario, error) {
	return world.Compose(
		banditcamp.Scenario(),
		hostagecamp.Scenario(),
		guildTies(),
	)
}

// Companies are the three outfits working the region.
func Companies() []journal.EntityID {
	return []journal.EntityID{hostagecamp.PartyA, hostagecamp.PartyB, hostagecamp.PartyC}
}

// guildTies makes the three companies part of the guild the bandit camp knows
// about.
func guildTies() world.Scenario {
	companies := Companies()
	edges := make([]graph.Edge, 0, len(companies))
	for _, company := range companies {
		edges = append(edges, graph.Edge{From: company, Rel: banditcamp.BelongsTo, To: banditcamp.Party})
	}

	return world.Ties(banditcamp.BelongsTo, edges...)
}

// WeekendGoal is the needle: the region pacified, before the weekend.
//
// Two conditions, and neither of them mentions a party, a method, or an act.
// The camp has to have stopped being hostile — by defeat, by conversion, by
// somebody wearing the chief's face, the condition does not ask. And nobody may
// still be in the hostage camp's cells or standing with its captors: every one
// of that population has to have ended up rescued, redeemed, or dead.
//
// Three companies can push this by three different routes at once, and nothing
// anywhere adds their contributions together, because there is nothing to add.
// The needle is a fold over one journal, and a fold does not ask how.
func WeekendGoal(deadline time.Time) goal.Goal {
	return goal.Goal{
		ID:       WeekendGoalID,
		Name:     "Pacify the Region",
		Deadline: deadline,
		Conditions: []goal.Condition{
			goal.Present{
				Observer: banditcamp.Camp,
				Predicate: quest.NoEdge{
					From: banditcamp.Camp, Rel: banditcamp.HostileTo, To: banditcamp.Party,
				},
			},
			goal.Population{
				Job: hostagecamp.RescueJob,
				Shape: quest.Every{
					quest.NoneIn{Bucket: hostagecamp.BucketCaptive},
					quest.NoneIn{Bucket: hostagecamp.BucketTurned},
				},
			},
		},
	}
}

// Crew loads every character working the region — both camps' casts, in one
// map, because one resolver serves one world.
func Crew(ctx context.Context) (map[journal.EntityID]*character.Character, error) {
	out := make(map[journal.EntityID]*character.Character)

	for _, load := range []func(context.Context) (map[journal.EntityID]*character.Character, error){
		banditcamp.Crew, hostagecamp.Crew,
	} {
		sheets, err := load(ctx)
		if err != nil {
			return nil, err
		}
		for id, sheet := range sheets {
			if _, seen := out[id]; seen {
				return nil, fmt.Errorf("two pieces of content both supply a character called %q", id)
			}
			out[id] = sheet
		}
	}

	return out, nil
}
