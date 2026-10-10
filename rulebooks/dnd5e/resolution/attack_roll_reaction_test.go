package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/require"
)

func flareHero(t *testing.T) *character.Data {
	t.Helper()
	hero := actionHero()
	f, err := features.CreateFromRef(&features.CreateFromRefInput{Ref: refs.Features.WardingFlare().String(), CharacterID: heroID})
	require.NoError(t, err)
	raw, err := f.Feature.ToJSON()
	require.NoError(t, err)
	hero.Features = []json.RawMessage{raw}
	hero.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{resources.WardingFlare: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest}}
	hero.ActionEconomy = &character.ActionEconomyData{ReactionsRemaining: 1}
	return hero
}

func TestWardingFlarePausesBeforeDiceAndResumesOnlyOnce(t *testing.T) {
	for name, answer := range map[string]Answer{"take": Take(useChoice), "decline": Decline()} {
		t.Run(name, func(t *testing.T) {
			hero := flareHero(t)
			roller := &actionRoller{singles: []int{15}, pairs: [][]int{{15, 2}}, damage: [][]int{{3}}}
			folds := 0
			run := func(machine Machine) (*Output, error) {
				bus := events.NewEventBus()
				_, err := dndEvents.AttackChain.On(bus).SubscribeWithChain(context.Background(), func(_ context.Context, e dndEvents.AttackChainEvent, c chain.Chain[dndEvents.AttackChainEvent]) (chain.Chain[dndEvents.AttackChainEvent], error) {
					folds++
					return c, nil
				})
				require.NoError(t, err)
				return resolveOn(context.Background(), &Input{World: actionWorld(t, 2), Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: hero}}, Machine: machine, Capabilities: encounter.Capabilities{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Driver: passDriver{}, Roller: dice.NewRoller(), Actors: Actors}}, newSurface(bus))
			}
			out, err := run(NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: roller}))
			require.NoError(t, err)
			require.NotNil(t, out.Posed)
			require.Equal(t, PauseBeforeRoll, out.Posed.Kind)
			require.Nil(t, out.Outcome)
			require.Zero(t, roller.calls)
			require.Equal(t, heroID, out.Posed.Ask.Audience)
			require.Equal(t, reactionCost(heroID, resources.WardingFlare), out.Posed.Cost,
				"the price is stated on the pause: one reaction and one flare")
			resumed, err := Resume(&ResumeInput{Pause: *out.Posed, Answer: answer, Roller: roller})
			require.NoError(t, err)
			out, err = run(resumed)
			require.NoError(t, err)
			require.Nil(t, out.Posed)
			struck := out.Outcome.(StrikeOutcome)
			require.Equal(t, 1, folds, "the frozen attack chain must not be replayed")
			if answer.Taken() {
				require.Equal(t, 2, struck.Roll)
				require.False(t, struck.Hit)
				require.Len(t, struck.Folded.DisadvantageSources, 1)
				require.Equal(t, 1, out.DirtyCharacters[0].Resources[resources.WardingFlare].Current)
				require.Zero(t, out.DirtyCharacters[0].ActionEconomy.ReactionsRemaining)
			} else {
				require.Equal(t, 15, struck.Roll)
				require.True(t, struck.Hit)
				require.Empty(t, struck.Folded.DisadvantageSources)
				require.Equal(t, 2, out.DirtyCharacters[0].Resources[resources.WardingFlare].Current)
				require.Equal(t, 1, out.DirtyCharacters[0].ActionEconomy.ReactionsRemaining)
			}
		})
	}
}

func TestWardingFlareSequenceCanDeclineFirstAndSpendOnSecond(t *testing.T) {
	hero := flareHero(t)
	roller := &actionRoller{singles: []int{15}, pairs: [][]int{{17, 2}}, damage: [][]int{{3}}}
	out, err := resolveSequenceAgainst(t, twoClaws(), []combatActions.Definition{validMeleeDefinition()}, hero, roller)
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	require.Zero(t, roller.calls)
	resume := func(answer Answer) {
		machine, e := Resume(&ResumeInput{Pause: *out.Posed, Answer: answer, Roller: roller})
		require.NoError(t, e)
		if len(out.DirtyCharacters) > 0 {
			hero = out.DirtyCharacters[0]
		}
		out, e = Resolve(context.Background(), &Input{World: out.World, Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: hero}}, Machine: machine, Capabilities: encounter.Capabilities{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Driver: passDriver{}, Roller: dice.NewRoller(), Actors: Actors}})
		require.NoError(t, e)
	}
	resume(Decline())
	require.NotNil(t, out.Posed)
	first := out.Outcome.(SequenceOutcome)
	require.Equal(t, 0, first.From)
	require.Len(t, first.Steps, 1, "the first swing settled in this call and is told with the second pause")
	require.Equal(t, 15, first.Steps[0].Strike.Roll)
	require.Equal(t, 2, roller.calls)
	resume(Take(useChoice))
	require.Nil(t, out.Posed)
	sequence := out.Outcome.(SequenceOutcome)
	require.Equal(t, 1, sequence.From)
	require.Len(t, sequence.Steps, 1, "only the second swing settled after the pause")
	require.Equal(t, 2, sequence.Steps[0].Strike.Roll)
	require.Equal(t, 3, roller.calls, "first swing was not replayed")
	require.Equal(t, 1, out.DirtyCharacters[0].Resources[resources.WardingFlare].Current)
}
