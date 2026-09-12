// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// aCrowdedRoom is a pass the size of a real fight: everyone present, everyone
// perceiving everyone, on two channels.
func aCrowdedRoom(bodies int, at testimony.Stamp, jostle bool) (*perception.Game, projection.Input) {
	g := perception.NewGame()

	in := projection.Input{At: at}

	for i := range bodies {
		id := fmt.Sprintf("body-%02d", i)
		where := "room"

		// jostle moves everyone a little every pass, which is what a fight
		// actually looks like.
		if jostle {
			where = fmt.Sprintf("cell-%d", (uint64(i)+at.Tick)%8)
		}

		in.Presences = append(in.Presences, projection.Presence{
			Source: id,
			Where:  where,
			Says: map[testimony.Channel]projection.Says{
				testimony.Sight: says(content.Creature, "goblin", "", where),
				hearing:         says(content.Noise, "", "shouting", where),
			},
		})

		observer := testimony.Observer(id)
		g.Mind(observer, reconcile.Woodwise{})

		reach := []string{"room"}
		if jostle {
			reach = []string{"cell-0", "cell-1", "cell-2", "cell-3", "cell-4", "cell-5", "cell-6", "cell-7"}
		}

		in.Senses = append(in.Senses,
			projection.Sense{Observer: observer, Channel: testimony.Sight, Reach: reach},
			projection.Sense{Observer: observer, Channel: hearing, Reach: reach},
		)
	}

	return g, in
}

// TestWhatAPassActuallyCosts measures rather than assumes. Run with -v.
func TestWhatAPassActuallyCosts(t *testing.T) {
	for _, bodies := range []int{6, 12, 18} {
		for _, jostle := range []bool{false, true} {
			label := fmt.Sprintf("%d bodies, still", bodies)
			if jostle {
				label = fmt.Sprintf("%d bodies, moving", bodies)
			}

			t.Run(label, func(t *testing.T) {
				g, _ := aCrowdedRoom(bodies, testimony.Stamp{Tick: 0}, jostle)

				const passes = 50

				for tick := uint64(1); tick <= passes; tick++ {
					_, in := aCrowdedRoom(bodies, testimony.Stamp{Tick: tick}, jostle)

					_, err := g.Tick(in)
					require.NoError(t, err)
				}

				held := g.Held(testimony.Observer("body-00"))

				entries := 0
				for _, track := range held {
					entries += len(track.Entries)
				}

				t.Logf("after %d passes: %d tracks held, %d entries, %d claims",
					passes, len(held), entries, len(g.Contacts(testimony.Observer("body-00"))))
			})
		}
	}
}

func BenchmarkAPass(b *testing.B) {
	for _, bodies := range []int{6, 12, 18} {
		b.Run(fmt.Sprintf("%d-bodies-moving", bodies), func(b *testing.B) {
			g, _ := aCrowdedRoom(bodies, testimony.Stamp{Tick: 0}, true)

			b.ResetTimer()

			for i := 0; b.Loop(); i++ {
				_, in := aCrowdedRoom(bodies, testimony.Stamp{Tick: uint64(i + 1)}, true)
				if _, err := g.Tick(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
