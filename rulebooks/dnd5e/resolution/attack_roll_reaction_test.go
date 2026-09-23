package resolution

import (
	"context"
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/require"
	"testing"
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
	for _, answer := range []OfferAnswer{OfferSpend, OfferKeep} {
		t.Run(string(answer), func(t *testing.T) {
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
				return resolveOn(context.Background(), &Input{World: actionWorld(t, 2), Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: hero}}, Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, TurnDriver: passDriver{}, Roller: dice.NewRoller()}, newSurface(bus))
			}
			out, err := run(NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: roller}))
			require.NoError(t, err)
			require.NotNil(t, out.Posed)
			require.True(t, out.Posed.BeforeRoll)
			require.Nil(t, out.Outcome)
			require.Zero(t, roller.calls)
			require.Equal(t, heroID, out.Posed.Ask.Audience)
			option := ""
			if answer == OfferSpend {
				option = ReactionUse
			}
			resumed, err := NewStrikeResumed(&StrikeResumeInput{Frozen: out.Posed.Frozen, Answer: answer, Option: option, Roller: roller})
			require.NoError(t, err)
			out, err = run(resumed)
			require.NoError(t, err)
			require.Nil(t, out.Posed)
			struck := out.Outcome.(StrikeOutcome)
			require.Equal(t, 1, folds, "the frozen attack chain must not be replayed")
			if answer == OfferSpend {
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
	resume := func(answer OfferAnswer, option string) {
		machine, e := NewAttackResumed(&StrikeResumeInput{Frozen: out.Posed.Frozen, Answer: answer, Option: option, Roller: roller})
		require.NoError(t, e)
		if len(out.DirtyCharacters) > 0 {
			hero = out.DirtyCharacters[0]
		}
		out, e = Resolve(context.Background(), &Input{World: out.World, Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: hero}}, Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, TurnDriver: passDriver{}, Roller: dice.NewRoller()})
		require.NoError(t, e)
	}
	resume(OfferKeep, "")
	require.NotNil(t, out.Posed)
	require.Len(t, out.Posed.Sequence.Steps, 1)
	require.Equal(t, 2, roller.calls)
	resume(OfferSpend, ReactionUse)
	require.Nil(t, out.Posed)
	sequence := out.Outcome.(SequenceOutcome)
	require.Len(t, sequence.Steps, 2)
	require.Equal(t, 15, sequence.Steps[0].Strike.Roll)
	require.Equal(t, 2, sequence.Steps[1].Strike.Roll)
	require.Equal(t, 3, roller.calls, "first swing was not replayed")
	require.Equal(t, 1, out.DirtyCharacters[0].Resources[resources.WardingFlare].Current)
}
