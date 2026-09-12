// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/act"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/stage"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

const (
	attacked  journal.Kind   = "attacked"
	attackerb world.VerbName = "attack"
)

// aWorld builds the smallest world that can record an attack: one verb, a
// resolver with no dice in it, and a witness that is the same senses the
// perception pass runs on.
func aWorld(t *testing.T, in projection.Input) *world.World {
	t.Helper()

	w, err := world.New(world.Config{
		Scenario: world.Scenario{
			// graph refuses a world that does not say how belonging works,
			// because audiences and allegiances would otherwise be guesses.
			// Nothing here needs groups, but it still has to be declared.
			Graph: graph.Config{Membership: "belongs-to"},
			Verbs: []world.Verb{{
				Name:       attackerb,
				Approach:   "force",
				Difficulty: 10,
				Otherwise: world.Emission{
					Kind:    attacked,
					Subject: world.SubjectTarget,
					Witness: world.WitnessTargetAndBystanders,
				},
			}},
		},
		Resolver: stage.Scripted{Succeeded: true, Margin: 3},
		Witness:  stage.Watcher{In: in},
	})
	require.NoError(t, err)

	return w
}

// standingInTheTunnels puts somebody on the truth surface where they can be
// seen and witnessed acting.
func standingInTheTunnels(source, as string) projection.Presence {
	return projection.Presence{
		Source: source,
		Where:  tunnels,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, as, "", tunnels)},
	}
}

// TestActingOnWhatYouHold is the loop end to end: a decider that sees only its
// own beliefs picks a target, the composition resolves that belief against what
// is really there, and the world records the deed.
func TestActingOnWhatYouHold(t *testing.T) {
	in := projection.Input{
		Presences: []projection.Presence{
			standingInTheTunnels(goblinBand, "goblin"),
			standingInTheTunnels(string(bram), "human"),
		},
		Senses: senses(sight, []string{tunnels}, bram, pip),
		At:     moment(1),
	}

	g := perception.NewGame()
	g.Mind(bram, reconcile.Woodwise{})

	_, err := g.Tick(in)
	require.NoError(t, err)

	seen := handle(sight, goblinBand)
	require.NoError(t, g.Identify(bram, seen, easternName, moment(1)))

	// The decider is handed one actor's holdings and nothing else.
	situation := g.Situation(bram, moment(1))
	intent, acting := act.Aggressive{}.Decide(situation)
	require.True(t, acting)
	assert.Equal(t, easternName, intent.Target, "he aimed at what he calls it")

	// The composition resolves the belief against the world.
	target := stage.Aim(in, situation, intent)
	require.Equal(t, journal.EntityID(goblinBand), target, "and there really was something there")

	w := aWorld(t, in)

	result, err := w.Act(context.Background(), world.Act{
		Verb:   attackerb,
		Actor:  asEntity(bram),
		Target: target,
	})
	require.NoError(t, err)

	assert.Equal(t, attacked, result.Fact.Kind)
	assert.Equal(t, journal.EntityID(goblinBand), result.Fact.Subject)
	assert.True(t, result.Fact.Outcome.Succeeded)

	// Pip's eyes reached the tunnels, so pip is in the audience — decided by
	// the same senses the perception pass ran on, not by anybody's beliefs.
	assert.True(t, result.Fact.Audience.Includes(asEntity(pip)),
		"pip was standing right there")
}

// TestActingOnALie is the one the whole design exists for. Pip swings at
// something that was never there, and nothing in the system had to be told that
// an illusion is special.
func TestActingOnALie(t *testing.T) {
	in := projection.Input{
		Presences: []projection.Presence{standingInTheTunnels(string(pip), "human")},
		Forgeries: []projection.Forgery{{
			Where:  tunnels,
			Source: silentImage,
			Says: map[testimony.Channel]projection.Says{
				sight: says(content.Creature, "dragon", "", tunnels),
			},
		}},
		Senses: senses(sight, []string{tunnels}, pip),
		At:     moment(1),
	}

	g := perception.NewGame()
	g.Mind(pip, reconcile.Oblivious{})

	_, err := g.Tick(in)
	require.NoError(t, err)

	theDragon := belief.Name("the dragon in the tunnels")
	require.NoError(t, g.Identify(pip, handle(sight, silentImage), theDragon, moment(1)))

	situation := g.Situation(pip, moment(1))
	intent, acting := act.Aggressive{}.Decide(situation)
	require.True(t, acting, "he is perfectly willing to attack it")
	assert.Equal(t, theDragon, intent.Target)

	// THE POINT. The belief was real, the name was real, the swing is real —
	// and the truth surface has nothing behind it.
	target := stage.Aim(in, situation, intent)
	assert.Empty(t, target, "he swung at empty air, and no rule had to say so")

	w := aWorld(t, in)

	result, err := w.Act(context.Background(), world.Act{
		Verb:   attackerb,
		Actor:  asEntity(pip),
		Target: target,
	})
	require.NoError(t, err)

	assert.Empty(t, result.Fact.Subject, "the world records an attack on nobody")

	for _, fact := range w.Journal().All() {
		assert.NotEqual(t, journal.EntityID(silentImage), fact.Subject,
			"the illusion is not in the ledger, because it was never in the world")
	}

	// And he still believes it, afterwards. Swinging taught him nothing.
	held, ok := g.Track(pip, handle(sight, silentImage))
	require.True(t, ok)
	assert.Equal(t, "dragon", mustRead(t, held.Latest().Payload).Of)
}

// TestYouCannotAimAtWhatYouHaveNotNamed proves the constraint that makes the
// rest work. An actor reaches for a name, and a name is something they had to
// work out for themselves.
func TestYouCannotAimAtWhatYouHaveNotNamed(t *testing.T) {
	in := projection.Input{
		Presences: []projection.Presence{
			standingInTheTunnels(goblinBand, "goblin"),
			standingInTheTunnels(string(bram), "human"),
		},
		Senses: senses(sight, []string{tunnels}, bram),
		At:     moment(1),
	}

	g := perception.NewGame()

	_, err := g.Tick(in)
	require.NoError(t, err)

	// He can see them perfectly well. He has no word for them.
	situation := g.Situation(bram, moment(1))
	require.NotEmpty(t, situation.Current(), "he is looking right at them")
	assert.Empty(t, situation.Named(), "and cannot say what they are")

	_, acting := act.Aggressive{}.Decide(situation)
	assert.False(t, acting, "there is nothing he can aim at")

	// Naming it is the only thing that changes the answer.
	require.NoError(t, g.Identify(bram, handle(sight, goblinBand), easternName, moment(1)))

	_, acting = act.Aggressive{}.Decide(g.Situation(bram, moment(1)))
	assert.True(t, acting)
}

// TestBehaviourSeesOnlyBeliefs is the seam a behaviour author writes against.
// Two deciders, one situation, and the difference is entirely in what each makes
// of the same testimony — including which CHANNEL it arrived on.
func TestBehaviourSeesOnlyBeliefs(t *testing.T) {
	// Something is heard and not seen: sound reaches the tunnels, sight does not.
	in := projection.Input{
		Presences: []projection.Presence{{
			Source: goblinBand,
			Where:  tunnels,
			Says: map[testimony.Channel]projection.Says{
				hearing: says(content.Noise, "", "chanting", tunnels),
			},
		}},
		Senses: senses(hearing, []string{tunnels}, bram),
		At:     moment(1),
	}

	g := perception.NewGame()

	_, err := g.Tick(in)
	require.NoError(t, err)

	require.NoError(t, g.Identify(bram, handle(hearing, goblinBand), easternName, moment(1)))

	situation := g.Situation(bram, moment(1))

	// Aggressive does not care how it found out.
	_, acting := act.Aggressive{}.Decide(situation)
	assert.True(t, acting, "it will swing at a noise")

	// A behaviour that will only commit to what it has laid eyes on. Nothing
	// here imports world, or a roster, or the truth — only what bram holds.
	cautious := act.DeciderFunc(func(s act.Situation) (act.Intent, bool) {
		for _, held := range s.Named() {
			if held.Channel != sight || !held.Current {
				continue
			}

			return act.Intent{Verb: string(attackerb), Target: held.Name}, true
		}

		return act.Intent{}, false
	})

	_, acting = cautious.Decide(situation)
	assert.False(t, acting, "it has heard something and seen nothing, so it waits")
}

// TestAnActorNobodyCanSeeIsAWiringFault proves the witness fails closed. An
// empty audience is a perfect sneak, so guessing one would turn a broken setup
// into the best stealth in the game.
func TestAnActorNobodyCanSeeIsAWiringFault(t *testing.T) {
	in := projection.Input{
		Presences: []projection.Presence{standingInTheTunnels(goblinBand, "goblin")},
		Senses:    senses(sight, []string{tunnels}, bram),
		At:        moment(1),
	}

	_, err := stage.Watcher{In: in}.Bystanders(
		context.Background(), asEntity("nobody-is-standing-here"), "", world.WitnessBystanders)
	require.ErrorIs(t, err, stage.ErrActorNotPresent)
}
